package auth

import (
	"errors"
	"log"
	"nvs-server/utils/infra"
	"nvs-server/utils/secure"
	"strings"

	"nvs-server/gate"
	"nvs-server/handlers/admincommon"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/security/slidecaptcha"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	apikey "nvs-server/models/apikey"
	siteops "nvs-server/models/siteops"
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
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))

	if !input.AgreeToGuidelines {
		infra.BadRequest(c, "请同意平台指南")
		return
	}

	// 🔒 滑块验证码闸门：站长开启滑块时，注册必须携带有效的 pass token（先过滑块再注册），
	// 否则直接拒绝，堵死「裸调 register」绕过滑块的通道。
	if models.IsCaptchaEnabled() {
		if input.CaptchaToken == "" || !slidecaptcha.VerifyPassToken(input.CaptchaToken) {
			infra.Forbidden(c, "请先完成滑块验证后再注册")
			return
		}
	}

	// 注册门槛（gap-audit #14）：邀请码 / 设备指纹 / 蜜罐 IP / 临时邮箱
	deviceID, deviceSrc := gate.DeviceIDFrom(c)
	if gerr := gate.CanRegister(gate.RegisterInput{
		IP: c.ClientIP(), DeviceID: deviceID, Email: input.Email, Invite: input.InviteCode,
	}); gerr != nil {
		infra.Forbidden(c, gerr.Error())
		return
	}

	// 邮箱域名黑白名单校验（黑名单禁止、白名单放行）
	if allowed, reason := models.CheckEmailDomainAllowed(input.Email); !allowed {
		infra.BadRequest(c, reason)
		return
	}

	// 邮箱验证码校验（站长开启邮箱验证时强制）
	if models.GetPlatformConfig("email_verify") == "true" {
		if input.VerifyCode == "" {
			infra.BadRequest(c, "请输入邮箱验证码")
			return
		}
		if !infra.VerifyCode(input.Email, input.VerifyCode) {
			infra.BadRequest(c, "邮箱验证码错误或已过期")
			return
		}
		infra.DeleteVerificationCode(input.Email)
	}

	// 检查邮箱是否已注册
	existing, err := models.GetUserByEmail(input.Email)
	if err == nil && existing != nil {
		infra.BadRequest(c, "该邮箱已被注册")
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		infra.InternalError(c, "服务器错误")
		return
	}

	// 哈希密码
	hash, err := secure.HashPassword(input.Password)
	if err != nil {
		infra.InternalError(c, "密码处理失败")
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
			infra.BadRequest(c, "用户名或邮箱已被注册")
			return
		}
		infra.InternalError(c, "注册失败")
		return
	}

	// 邀请码消费（失败说明刚好被用尽——此时回滚用户，避免「无邀请码也能进」）
	if cfg := gate.Load(); cfg.Enabled && cfg.RequireInvite && input.InviteCode != "" {
		if uerr := gate.UseInvite(nil, input.InviteCode); uerr != nil {
			_ = models.DB.Unscoped().Delete(&models.User{}, user.ID).Error
			infra.BadRequest(c, "邀请码无效或已用尽")
			return
		}
	}
	// 设备指纹绑定（同设备账号数统计的基础；失败不阻断注册，只影响风控精度）
	_ = gate.Bind(nil, deviceID, user.ID, c.ClientIP(), c.GetHeader("User-Agent"), deviceSrc)

	// 合规审计：注册成功
	RecordUserAudit(c, user.ID, apikey.AuditRegister, "新用户注册，邮箱 "+user.Email, "success")

	// 生成访问令牌 + 刷新令牌，均写入 HttpOnly Cookie
	// 🔒 令牌绑定 + 刷新令牌 JTI 轮换（同登录流程，防 JWT/刷新令牌窃取盗用）
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
	if err := siteops.SetRefreshState(user.ID, jti); err != nil {
		log.Printf("[auth] 写入刷新状态失败（用户 %d）：%v", user.ID, err)
	}
	middleware.SetTokenCookie(c, accessToken)
	middleware.SetRefreshCookie(c, refreshToken)

	user.IsSuper = admincommon.IsSuperUser(user)
	infra.SuccessMessage(c, "注册成功", gin.H{
		"user": user,
	})
}
