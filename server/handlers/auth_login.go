package handlers

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/security/slidecaptcha"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LoginInput struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required"`
	CaptchaToken string `json:"captcha_token"` // 滑块验证码 pass token（站长开启滑块时必填）
}

// POST /api/auth/login
func Login(c *gin.Context) {
	if !models.GetLoginMethods().Email {
		utils.Forbidden(c, "站长已关闭邮箱密码登录")
		return
	}
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	input.Email = strings.TrimSpace(strings.ToLower(input.Email))

	// 🔒 滑块验证码闸门：站长开启滑块时，登录前必须先过人机校验，阻断撞库/暴力破解。
	// 与注册、发码、找回密码一致；未通过滑块挑战直接拒绝，堵死「裸调 login」通道。
	if models.IsCaptchaEnabled() {
		if input.CaptchaToken == "" || !slidecaptcha.VerifyPassToken(input.CaptchaToken) {
			utils.Forbidden(c, "请先完成滑块验证后再登录")
			return
		}
	}

	user, err := models.GetUserByEmail(input.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 合规审计：登录失败（账号不存在）
			recordUserAudit(c, 0, models.AuditLoginFail, "登录失败：邮箱不存在 "+input.Email, "fail")
			utils.BadRequest(c, "邮箱或密码错误")
			return
		}
		utils.InternalError(c, "服务器错误")
		return
	}

	// 🔒 安全审计 M1：账号级失败锁定（5 次失败锁定 15 分钟，防暴力破解；字段早已定义未启用）
	const maxLoginFails = 5
	const lockMinutes = 15
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		remaining := int(time.Until(*user.LockedUntil).Minutes()) + 1
		utils.BadRequest(c, fmt.Sprintf("登录失败次数过多，账号已锁定，请在 %d 分钟后重试", remaining))
		return
	}

	if !utils.CheckPassword(input.Password, user.PasswordHash) {
		// 合规审计：登录失败（密码错误）
		recordUserAudit(c, user.ID, models.AuditLoginFail, "登录失败：密码错误 "+user.Username, "fail")
		// 递增失败计数；达到阈值锁定账号
		newCount := user.LoginFailCount + 1
		updates := map[string]interface{}{"login_fail_count": newCount}
		if newCount >= maxLoginFails {
			lockUntil := time.Now().Add(lockMinutes * time.Minute)
			updates["locked_until"] = &lockUntil
			updates["login_fail_count"] = 0
		}
		models.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates)
		utils.BadRequest(c, "邮箱或密码错误")
		return
	}

	// 登录成功：清零失败计数与锁定
	models.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"login_fail_count": 0,
		"locked_until":     nil,
	})

	// 合规审计：登录成功
	recordUserAudit(c, user.ID, models.AuditLoginSuccess, "登录成功 "+user.Username, "success")

	// 合规留存：记录最后登录 IP 与时间
	now := time.Now()
	models.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"last_login_ip": c.ClientIP(),
		"last_login_at": &now,
	})

	// 生成访问令牌 + 刷新令牌，均写入 HttpOnly Cookie
	accessToken, err := utils.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		utils.InternalError(c, "Token 生成失败")
		return
	}
	refreshToken, err := utils.GenerateRefreshToken(user.ID, user.Email, user.Role)
	if err != nil {
		utils.InternalError(c, "Refresh Token 生成失败")
		return
	}
	middleware.SetTokenCookie(c, accessToken)
	middleware.SetRefreshCookie(c, refreshToken)

	utils.SuccessMessage(c, "登录成功", gin.H{
		"user": user,
	})
}

// GET /api/auth/me
func GetCurrentUser(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "未登录")
		return
	}

	user, err := models.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "用户不存在")
			return
		}
		utils.InternalError(c, "服务器错误")
		return
	}

	utils.Success(c, user)
}

// POST /api/auth/logout
func Logout(c *gin.Context) {
	// 合规审计：记录登出（从 Cookie 解析用户身份）
	if token, err := c.Cookie("token"); err == nil && token != "" {
		if claims, e := utils.ParseToken(token); e == nil {
			recordUserAudit(c, claims.UserID, models.AuditLogout, "用户登出", "success")
		}
	}
	middleware.ClearTokenCookie(c)
	middleware.ClearRefreshCookie(c)
	utils.SuccessMessage(c, "已退出登录", nil)
}

// POST /api/auth/refresh
// 无感续期：校验 HttpOnly 刷新令牌 Cookie，签发新的访问令牌（并轮换刷新令牌），
// 失败（令牌缺失/过期/非法）则清除全部会话 Cookie 并返回 401，由前端引导重新登录。
func Refresh(c *gin.Context) {
	recordInbound(c, "auth-refresh")
	rt, err := c.Cookie("refresh_token")
	if err != nil || rt == "" {
		middleware.ClearTokenCookie(c)
		middleware.ClearRefreshCookie(c)
		utils.Unauthorized(c, "会话已过期，请重新登录")
		return
	}
	claims, err := utils.ParseRefreshToken(rt)
	if err != nil {
		middleware.ClearTokenCookie(c)
		middleware.ClearRefreshCookie(c)
		utils.Unauthorized(c, "刷新令牌无效或已过期")
		return
	}

	// 校验用户仍然存在（防止已注销账号的刷新令牌被继续利用）
	if _, uerr := models.GetUserByID(claims.UserID); uerr != nil {
		middleware.ClearTokenCookie(c)
		middleware.ClearRefreshCookie(c)
		utils.Unauthorized(c, "账号不存在或已注销")
		return
	}

	accessToken, err := utils.GenerateToken(claims.UserID, claims.Email, claims.Role)
	if err != nil {
		utils.InternalError(c, "Token 生成失败")
		return
	}
	newRefresh, err := utils.GenerateRefreshToken(claims.UserID, claims.Email, claims.Role)
	if err != nil {
		utils.InternalError(c, "Refresh Token 生成失败")
		return
	}
	middleware.SetTokenCookie(c, accessToken)
	middleware.SetRefreshCookie(c, newRefresh)
	utils.SuccessMessage(c, "已续期", nil)
}
