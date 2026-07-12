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
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
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
		Username:     input.Username,
		Email:        input.Email,
		PasswordHash: hash,
		Nickname:     nickname,
		Role:         "reader",
	}

	if err := models.CreateUser(user); err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			utils.BadRequest(c, "用户名或邮箱已被注册")
			return
		}
		utils.InternalError(c, "注册失败")
		return
	}

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
			utils.BadRequest(c, "邮箱或密码错误")
			return
		}
		utils.InternalError(c, "服务器错误")
		return
	}

	if !utils.CheckPassword(input.Password, user.PasswordHash) {
		utils.BadRequest(c, "邮箱或密码错误")
		return
	}

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
	userID := c.GetUint("userID")
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

// ============ 邮箱验证码 ============

// POST /api/auth/send-code — 发送邮箱验证码
func SendVerificationCode(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请提供有效的邮箱地址")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	// 检查是否可以发送（60秒内只能发一次）
	code := utils.GenerateVerificationCode()
	utils.StoreVerificationCode(email, code)

	if err := utils.SendVerificationEmail(email, code); err != nil {
		utils.InternalError(c, "发送验证码失败: "+err.Error())
		return
	}

	utils.SuccessMessage(c, "验证码已发送，请查收邮件", nil)
}

// POST /api/auth/verify-code — 验证邮箱验证码
func VerifyEmailCode(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
		Code  string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	if !utils.VerifyCode(email, req.Code) {
		utils.BadRequest(c, "验证码错误或已过期")
		return
	}

	utils.DeleteVerificationCode(email)
	utils.SuccessMessage(c, "验证成功", gin.H{"email": email})
}

// POST /api/auth/logout
func Logout(c *gin.Context) {
	middleware.ClearTokenCookie(c)
	utils.SuccessMessage(c, "已退出登录", nil)
}
