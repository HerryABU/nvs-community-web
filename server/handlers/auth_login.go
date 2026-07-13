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
