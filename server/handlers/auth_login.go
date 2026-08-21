package handlers

import (
	"errors"
	"strings"
	"time"

	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// POST /api/auth/login
func Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	input.Email = strings.TrimSpace(strings.ToLower(input.Email))

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

	if !utils.CheckPassword(input.Password, user.PasswordHash) {
		// 合规审计：登录失败（密码错误）
		recordUserAudit(c, user.ID, models.AuditLoginFail, "登录失败：密码错误 "+user.Username, "fail")
		utils.BadRequest(c, "邮箱或密码错误")
		return
	}

	// 合规审计：登录成功
	recordUserAudit(c, user.ID, models.AuditLoginSuccess, "登录成功 "+user.Username, "success")

	// 合规留存：记录最后登录 IP 与时间
	now := time.Now()
	models.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"last_login_ip": c.ClientIP(),
		"last_login_at": &now,
	})

	// 生成 JWT 并设置 Cookie
	token, err := utils.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		utils.InternalError(c, "Token 生成失败")
		return
	}
	middleware.SetTokenCookie(c, token)

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
	utils.SuccessMessage(c, "已退出登录", nil)
}
