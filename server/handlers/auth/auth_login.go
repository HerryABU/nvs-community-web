package auth

import (
	"errors"
	"fmt"
	"log"
	"nvs-server/handlers/adminsite"
	"nvs-server/handlers/hctx"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/security/slidecaptcha"
	"nvs-server/utils/infra"
	"nvs-server/utils/secure"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	apikey "nvs-server/models/apikey"
	siteops "nvs-server/models/siteops"
)

type LoginInput struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required"`
	CaptchaToken string `json:"captcha_token"` // 滑块验证码 pass token（站长开启滑块时必填）
}

// POST /api/auth/login
func Login(c *gin.Context) {
	if !models.GetLoginMethods().Email {
		infra.Forbidden(c, "站长已关闭邮箱密码登录")
		return
	}
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	input.Email = strings.TrimSpace(strings.ToLower(input.Email))

	// 🔒 滑块验证码闸门：站长开启滑块时，登录前必须先过人机校验，阻断撞库/暴力破解。
	// 与注册、发码、找回密码一致；未通过滑块挑战直接拒绝，堵死「裸调 login」通道。
	if models.IsCaptchaEnabled() {
		if input.CaptchaToken == "" || !slidecaptcha.VerifyPassToken(input.CaptchaToken) {
			infra.Forbidden(c, "请先完成滑块验证后再登录")
			return
		}
	}

	user, err := models.GetUserByEmail(input.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 合规审计：登录失败（账号不存在）
			RecordUserAudit(c, 0, apikey.AuditLoginFail, "登录失败：邮箱不存在 "+input.Email, "fail")
			infra.BadRequest(c, "邮箱或密码错误")
			return
		}
		infra.InternalError(c, "服务器错误")
		return
	}

	// 🔒 安全审计 M1：账号级失败锁定（5 次失败锁定 15 分钟，防暴力破解；字段早已定义未启用）
	const maxLoginFails = 5
	const lockMinutes = 15
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		remaining := int(time.Until(*user.LockedUntil).Minutes()) + 1
		infra.BadRequest(c, fmt.Sprintf("登录失败次数过多，账号已锁定，请在 %d 分钟后重试", remaining))
		return
	}

	if !secure.CheckPassword(input.Password, user.PasswordHash) {
		// 合规审计：登录失败（密码错误）
		RecordUserAudit(c, user.ID, apikey.AuditLoginFail, "登录失败：密码错误 "+user.Username, "fail")
		// 递增失败计数；达到阈值锁定账号
		newCount := user.LoginFailCount + 1
		updates := map[string]interface{}{"login_fail_count": newCount}
		if newCount >= maxLoginFails {
			lockUntil := time.Now().Add(lockMinutes * time.Minute)
			updates["locked_until"] = &lockUntil
			updates["login_fail_count"] = 0
		}
		models.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates)
		infra.BadRequest(c, "邮箱或密码错误")
		return
	}

	// 登录成功：清零失败计数与锁定
	models.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"login_fail_count": 0,
		"locked_until":     nil,
	})

	// 合规审计：登录成功
	RecordUserAudit(c, user.ID, apikey.AuditLoginSuccess, "登录成功 "+user.Username, "success")

	// 合规留存：记录最后登录 IP 与时间
	now := time.Now()
	models.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"last_login_ip": c.ClientIP(),
		"last_login_at": &now,
	})

	// 生成访问令牌 + 刷新令牌，均写入 HttpOnly Cookie
	// 🔒 令牌绑定：把「设备指纹 + 登录地（省级）」加密进访问令牌，防 JWT 窃取后异地/异设备盗用；
	//    刷新令牌带 JTI 族 ID，用于刷新轮换（旧刷新令牌在受害者另设备刷新后立即失效）。
	fp := c.GetHeader(middleware.DeviceFingerprintHeader)
	region := secure.RegionForBinding(c.ClientIP())
	accessToken, err := secure.GenerateToken(user.ID, user.Email, user.Role, fp, region)
	if err != nil {
		infra.InternalError(c, "Token 生成失败")
		return
	}
	jti := secure.RandomJTI()
	refreshToken, err := secure.GenerateRefreshToken(user.ID, user.Email, user.Role, jti)
	if err != nil {
		infra.InternalError(c, "Refresh Token 生成失败")
		return
	}
	// 登记刷新令牌族 ID（刷新轮换/失效的基础；写入失败不阻断登录，仅降级为无轮换保护）
	if err := siteops.SetRefreshState(user.ID, jti); err != nil {
		log.Printf("[auth] 写入刷新状态失败（用户 %d）：%v", user.ID, err)
	}
	middleware.SetTokenCookie(c, accessToken)
	middleware.SetRefreshCookie(c, refreshToken)

	infra.SuccessMessage(c, "登录成功", gin.H{
		"user": user,
	})
}

// GET /api/auth/me
func GetCurrentUser(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	if userID == 0 {
		infra.Unauthorized(c, "未登录")
		return
	}

	user, err := models.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			infra.NotFound(c, "用户不存在")
			return
		}
		infra.InternalError(c, "服务器错误")
		return
	}

	infra.Success(c, user)
}

// POST /api/auth/logout
func Logout(c *gin.Context) {
	// 合规审计：记录登出（从 Cookie 解析用户身份）
	if token, err := c.Cookie("token"); err == nil && token != "" {
		if claims, e := secure.ParseToken(token); e == nil {
			RecordUserAudit(c, claims.UserID, apikey.AuditLogout, "用户登出", "success")
			// 清除刷新状态：使仍在有效期内的刷新令牌立即失效（防登出后令牌被续用）
			if derr := siteops.DeleteRefreshState(claims.UserID); derr != nil {
				log.Printf("[auth] 清除刷新状态失败（用户 %d）：%v", claims.UserID, derr)
			}
		}
	}
	middleware.ClearTokenCookie(c)
	middleware.ClearRefreshCookie(c)
	infra.SuccessMessage(c, "已退出登录", nil)
}

// POST /api/auth/refresh
// 无感续期：校验 HttpOnly 刷新令牌 Cookie，签发新的访问令牌（并轮换刷新令牌），
// 失败（令牌缺失/过期/非法）则清除全部会话 Cookie 并返回 401，由前端引导重新登录。
func Refresh(c *gin.Context) {
	adminsite.RecordInbound(c, "auth-refresh")
	rt, err := c.Cookie("refresh_token")
	if err != nil || rt == "" {
		middleware.ClearTokenCookie(c)
		middleware.ClearRefreshCookie(c)
		infra.Unauthorized(c, "会话已过期，请重新登录")
		return
	}
	claims, err := secure.ParseRefreshToken(rt)
	if err != nil {
		middleware.ClearTokenCookie(c)
		middleware.ClearRefreshCookie(c)
		infra.Unauthorized(c, "刷新令牌无效或已过期")
		return
	}

	// 校验用户仍然存在（防止已注销账号的刷新令牌被继续利用）
	if _, uerr := models.GetUserByID(claims.UserID); uerr != nil {
		middleware.ClearTokenCookie(c)
		middleware.ClearRefreshCookie(c)
		infra.Unauthorized(c, "账号不存在或已注销")
		return
	}

	// 🔒 刷新令牌轮换 / 失效：校验刷新令牌族 ID 是否仍为当前有效值。
	// 旧刷新令牌（被窃取或已在别处刷新）因 JTI 不匹配而立即失效，防止刷新令牌被盗用后长期可用。
	// 升级前签发的旧刷新令牌无 JTI（claims.JTI==""），首刷时直接认作有效并初始化状态（过渡兼容）。
	if claims.JTI != "" {
		if state, serr := siteops.GetRefreshState(claims.UserID); serr == nil {
			if state.ActiveJTI != claims.JTI {
				middleware.ClearTokenCookie(c)
				middleware.ClearRefreshCookie(c)
				infra.Unauthorized(c, "会话已在其他设备刷新，请重新登录")
				return
			}
		} else if !errors.Is(serr, gorm.ErrRecordNotFound) {
			// DB 异常：降级放行（避免误锁在线用户），仅记录告警
			log.Printf("[auth] 查询刷新状态失败（用户 %d），降级放行：%v", claims.UserID, serr)
		}
	}

	// 轮换出新 JTI 并登记，使本次使用的刷新令牌作废（下次刷新必须携带新 JTI）
	fp := c.GetHeader(middleware.DeviceFingerprintHeader)
	region := secure.RegionForBinding(c.ClientIP())
	accessToken, err := secure.GenerateToken(claims.UserID, claims.Email, claims.Role, fp, region)
	if err != nil {
		infra.InternalError(c, "Token 生成失败")
		return
	}
	newJTI := secure.RandomJTI()
	newRefresh, err := secure.GenerateRefreshToken(claims.UserID, claims.Email, claims.Role, newJTI)
	if err != nil {
		infra.InternalError(c, "Refresh Token 生成失败")
		return
	}
	if err := siteops.SetRefreshState(claims.UserID, newJTI); err != nil {
		log.Printf("[auth] 写入刷新状态失败（用户 %d）：%v", claims.UserID, err)
	}
	middleware.SetTokenCookie(c, accessToken)
	middleware.SetRefreshCookie(c, newRefresh)
	infra.SuccessMessage(c, "已续期", nil)
}
