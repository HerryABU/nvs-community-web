package auth

import (
	"nvs-server/utils/infra"
	"nvs-server/utils/secure"
	"strings"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/security/slidecaptcha"

	"github.com/gin-gonic/gin"
	apikey "nvs-server/models/apikey"
)

// ============ 邮箱验证码 ============

// POST /api/auth/send-code — 发送邮箱验证码
func SendVerificationCode(c *gin.Context) {
	var req struct {
		Email        string `json:"email" binding:"required,email"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "请提供有效的邮箱地址")
		return
	}

	// 🔒 必须先通过滑块验证码，否则禁止发送（防邮箱轰炸 / 探测 / 裸调接口）。
	// 仅当站长开启滑块验证码（captcha_enabled）时强制；关闭时为兼容旧链路放行。
	if models.IsCaptchaEnabled() {
		if req.CaptchaToken == "" || !slidecaptcha.VerifyPassToken(req.CaptchaToken) {
			infra.Forbidden(c, "请先完成滑块验证后再获取邮箱验证码")
			return
		}
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	// 60 秒内只能发一次（防邮箱轰炸 / 验证码被频繁覆盖）
	code := infra.GenerateVerificationCode()
	if err := infra.StoreVerificationCode(email, code); err != nil {
		infra.BadRequest(c, err.Error())
		return
	}

	if err := infra.SendVerificationEmail(email, code); err != nil {
		// 发送失败则回滚已写入的验证码，避免用户拿到无效码
		infra.DeleteVerificationCode(email)
		infra.InternalError(c, "发送验证码失败: "+err.Error())
		return
	}

	infra.SuccessMessage(c, "验证码已发送，请查收邮件", nil)
}

// POST /api/auth/verify-code — 验证邮箱验证码
func VerifyEmailCode(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
		Code  string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	if !infra.VerifyCode(email, req.Code) {
		infra.BadRequest(c, "验证码错误或已过期")
		return
	}

	infra.DeleteVerificationCode(email)
	infra.SuccessMessage(c, "验证成功", gin.H{"email": email})
}

// POST /api/auth/forgot-password — 发送密码重置验证码
func ForgotPassword(c *gin.Context) {
	var req struct {
		Email        string `json:"email" binding:"required,email"`
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "请提供有效的邮箱地址")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	// 🔒 滑块验证码闸门：站长开启滑块时，必须先通过滑块才能发送重置邮件，防邮箱轰炸 / 枚举。
	// 与 /api/auth/send-code、/api/auth/register 一致，堵死「裸调 forgot-password」通道。
	if models.IsCaptchaEnabled() {
		if req.CaptchaToken == "" || !slidecaptcha.VerifyPassToken(req.CaptchaToken) {
			infra.Forbidden(c, "请先完成滑块验证后再请求密码重置")
			return
		}
	}

	// 检查邮箱是否已注册
	user, err := models.GetUserByEmail(email)
	if err != nil || user == nil {
		// 统一返回成功消息，防止邮箱枚举攻击
		infra.SuccessMessage(c, "如果该邮箱已注册，重置邮件已发送", nil)
		return
	}

	code := infra.GenerateVerificationCode()
	infra.StoreVerificationCode(email, code)

	if err := infra.SendPasswordResetEmail(email, code); err != nil {
		infra.InternalError(c, "发送邮件失败: "+err.Error())
		return
	}

	infra.SuccessMessage(c, "如果该邮箱已注册，重置邮件已发送", nil)
}

// POST /api/auth/reset-password — 验证验证码并重置密码
func ResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		Code        string `json:"code" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	// 验证验证码
	if !infra.VerifyCode(email, req.Code) {
		infra.BadRequest(c, "验证码错误或已过期")
		return
	}

	// 查找用户
	user, err := models.GetUserByEmail(email)
	if err != nil || user == nil {
		infra.NotFound(c, "用户不存在")
		return
	}

	// 哈希新密码
	hash, err := secure.HashPassword(req.NewPassword)
	if err != nil {
		infra.InternalError(c, "密码处理失败")
		return
	}

	// 更新密码
	user.PasswordHash = hash
	if err := models.UpdateUser(user); err != nil {
		infra.InternalError(c, "重置密码失败")
		return
	}

	// 清除已验证的验证码
	infra.DeleteVerificationCode(email)

	// 合规审计：重置密码
	RecordUserAudit(c, user.ID, apikey.AuditChangePassword, "通过邮箱验证码重置密码", "success")

	infra.SuccessMessage(c, "密码重置成功，请重新登录", nil)
}

// POST /api/auth/change-password — 登录后修改密码
func ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误: 新密码长度至少6位")
		return
	}

	// 获取当前登录用户
	userID := hctx.CurrentUserID(c)
	if userID == 0 {
		infra.Unauthorized(c, "未登录")
		return
	}

	user, err := models.GetUserByID(userID)
	if err != nil || user == nil {
		infra.NotFound(c, "用户不存在")
		return
	}

	// 验证旧密码
	if !secure.CheckPassword(req.OldPassword, user.PasswordHash) {
		infra.BadRequest(c, "原密码不正确")
		return
	}

	// 新旧密码不能相同
	if req.OldPassword == req.NewPassword {
		infra.BadRequest(c, "新密码不能与原密码相同")
		return
	}

	// 哈希新密码
	hash, err := secure.HashPassword(req.NewPassword)
	if err != nil {
		infra.InternalError(c, "密码处理失败")
		return
	}

	// 更新密码
	user.PasswordHash = hash
	if err := models.UpdateUser(user); err != nil {
		infra.InternalError(c, "修改密码失败")
		return
	}

	// 合规审计：修改密码
	RecordUserAudit(c, user.ID, apikey.AuditChangePassword, "修改登录密码", "success")

	infra.SuccessMessage(c, "密码修改成功", nil)
}
