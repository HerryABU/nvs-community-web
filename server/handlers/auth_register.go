package handlers

import (
	"errors"
	"strings"

	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RegisterInput struct {
	Username        string `json:"username" binding:"required,min=3,max=64"`
	Email           string `json:"email" binding:"required,email,max=128"`
	Password        string `json:"password" binding:"required,min=6,max=128"`
	Nickname        string `json:"nickname"`
	AgreeToGuidelines bool `json:"agree_to_guidelines"`
	VerifyCode      string `json:"verify_code"` // 邮箱验证码（站长开启邮箱验证时必填）
}

// POST /api/auth/register
func Register(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))

	if !input.AgreeToGuidelines {
		utils.BadRequest(c, "请同意平台指南")
		return
	}

	// 邮箱域名黑白名单校验（黑名单禁止、白名单放行）
	if allowed, reason := models.CheckEmailDomainAllowed(input.Email); !allowed {
		utils.BadRequest(c, reason)
		return
	}

	// 邮箱验证码校验（站长开启邮箱验证时强制）
	if models.GetPlatformConfig("email_verify") == "true" {
		if input.VerifyCode == "" {
			utils.BadRequest(c, "请输入邮箱验证码")
			return
		}
		if !utils.VerifyCode(input.Email, input.VerifyCode) {
			utils.BadRequest(c, "邮箱验证码错误或已过期")
			return
		}
		utils.DeleteVerificationCode(input.Email)
	}

	// 检查邮箱是否已注册
	existing, err := models.GetUserByEmail(input.Email)
	if err == nil && existing != nil {
		utils.BadRequest(c, "该邮箱已被注册")
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		utils.InternalError(c, "服务器错误")
		return
	}

	// 哈希密码
	hash, err := utils.HashPassword(input.Password)
	if err != nil {
		utils.InternalError(c, "密码处理失败")
		return
	}

	nickname := input.Nickname
	if nickname == "" {
		nickname = input.Username
	}

	// 新用户默认都是读者；首次发布作品时自动升级为作者
	user := &models.User{
		Username:       input.Username,
		Email:          input.Email,
		PasswordHash:   hash,
		Nickname:       nickname,
		Role:           "reader",
		RegistrationIP: c.ClientIP(), // 合规留存：注册 IP（支持 X-Forwarded-For / X-Real-Ip）
	}

	if err := models.CreateUser(user); err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			utils.BadRequest(c, "用户名或邮箱已被注册")
			return
		}
		utils.InternalError(c, "注册失败")
		return
	}

	// 合规审计：注册成功
	recordUserAudit(c, user.ID, models.AuditRegister, "新用户注册，邮箱 "+user.Email, "success")

	// 生成 JWT 并设置 Cookie
	token, err := utils.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		utils.InternalError(c, "Token 生成失败")
		return
	}
	middleware.SetTokenCookie(c, token)

	utils.SuccessMessage(c, "注册成功", gin.H{
		"user": user,
	})
}
