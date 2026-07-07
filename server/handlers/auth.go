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

// POST /api/auth/logout
func Logout(c *gin.Context) {
	middleware.ClearTokenCookie(c)
	utils.SuccessMessage(c, "已退出登录", nil)
}
