package handlers

import (
	"errors"
	"strings"

	"nvs-server/gate"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/security/slidecaptcha"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RegisterInput struct {
	Username          string `json:"username" binding:"required,min=3,max=64"`
	Email             string `json:"email" binding:"required,email,max=128"`
	Password          string `json:"password" binding:"required,min=6,max=128"`
	Nickname          string `json:"nickname"`
	AgreeToGuidelines bool   `json:"agree_to_guidelines"`
	VerifyCode        string `json:"verify_code"`   // 邮箱验证码（站长开启邮箱验证时必填）
	CaptchaToken      string `json:"captcha_token"` // 滑块验证码 pass token（站长开启滑块时必填）
	InviteCode        string `json:"invite_code"`   // 邀请码（站长开启邀请制时必填）
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

	// 🔒 滑块验证码闸门：站长开启滑块时，注册必须携带有效的 pass token（先过滑块再注册），
	// 否则直接拒绝，堵死「裸调 register」绕过滑块的通道。
	if models.IsCaptchaEnabled() {
		if input.CaptchaToken == "" || !slidecaptcha.VerifyPassToken(input.CaptchaToken) {
			utils.Forbidden(c, "请先完成滑块验证后再注册")
			return
		}
	}

	// 注册门槛（gap-audit #14）：邀请码 / 设备指纹 / 蜜罐 IP / 临时邮箱
	deviceID, deviceSrc := gate.DeviceIDFrom(c)
	if gerr := gate.CanRegister(gate.RegisterInput{
		IP: c.ClientIP(), DeviceID: deviceID, Email: input.Email, Invite: input.InviteCode,
	}); gerr != nil {
		utils.Forbidden(c, gerr.Error())
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

	// 邀请码消费（失败说明刚好被用尽——此时回滚用户，避免「无邀请码也能进」）
	if cfg := gate.Load(); cfg.Enabled && cfg.RequireInvite && input.InviteCode != "" {
		if uerr := gate.UseInvite(nil, input.InviteCode); uerr != nil {
			_ = models.DB.Unscoped().Delete(&models.User{}, user.ID).Error
			utils.BadRequest(c, "邀请码无效或已用尽")
			return
		}
	}
	// 设备指纹绑定（同设备账号数统计的基础；失败不阻断注册，只影响风控精度）
	_ = gate.Bind(nil, deviceID, user.ID, c.ClientIP(), c.GetHeader("User-Agent"), deviceSrc)

	// 合规审计：注册成功
	recordUserAudit(c, user.ID, models.AuditRegister, "新用户注册，邮箱 "+user.Email, "success")

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

	utils.SuccessMessage(c, "注册成功", gin.H{
		"user": user,
	})
}
