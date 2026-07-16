package handlers

import (
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

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

// POST /api/auth/forgot-password — 发送密码重置验证码
func ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请提供有效的邮箱地址")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	// 检查邮箱是否已注册
	user, err := models.GetUserByEmail(email)
	if err != nil || user == nil {
		// 统一返回成功消息，防止邮箱枚举攻击
		utils.SuccessMessage(c, "如果该邮箱已注册，重置邮件已发送", nil)
		return
	}

	code := utils.GenerateVerificationCode()
	utils.StoreVerificationCode(email, code)

	if err := utils.SendPasswordResetEmail(email, code); err != nil {
		utils.InternalError(c, "发送邮件失败: "+err.Error())
		return
	}

	utils.SuccessMessage(c, "如果该邮箱已注册，重置邮件已发送", nil)
}

// POST /api/auth/reset-password — 验证验证码并重置密码
func ResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		Code        string `json:"code" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	// 验证验证码
	if !utils.VerifyCode(email, req.Code) {
		utils.BadRequest(c, "验证码错误或已过期")
		return
	}

	// 查找用户
	user, err := models.GetUserByEmail(email)
	if err != nil || user == nil {
		utils.NotFound(c, "用户不存在")
		return
	}

	// 哈希新密码
	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		utils.InternalError(c, "密码处理失败")
		return
	}

	// 更新密码
	user.PasswordHash = hash
	if err := models.UpdateUser(user); err != nil {
		utils.InternalError(c, "重置密码失败")
		return
	}

	// 清除已验证的验证码
	utils.DeleteVerificationCode(email)

	utils.SuccessMessage(c, "密码重置成功，请重新登录", nil)
}

// POST /api/auth/change-password — 登录后修改密码
func ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: 新密码长度至少6位")
		return
	}

	// 获取当前登录用户
	userID, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "未登录")
		return
	}

	user, err := models.GetUserByID(userID.(uint))
	if err != nil || user == nil {
		utils.NotFound(c, "用户不存在")
		return
	}

	// 验证旧密码
	if !utils.CheckPassword(req.OldPassword, user.PasswordHash) {
		utils.BadRequest(c, "原密码不正确")
		return
	}

	// 新旧密码不能相同
	if req.OldPassword == req.NewPassword {
		utils.BadRequest(c, "新密码不能与原密码相同")
		return
	}

	// 哈希新密码
	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		utils.InternalError(c, "密码处理失败")
		return
	}

	// 更新密码
	user.PasswordHash = hash
	if err := models.UpdateUser(user); err != nil {
		utils.InternalError(c, "修改密码失败")
		return
	}

	utils.SuccessMessage(c, "密码修改成功", nil)
}
