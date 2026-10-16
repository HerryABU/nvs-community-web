// auth_phone.go — 手机号验证码登录/注册 + 实名制 + 国家实名核验代理（F10 / F21.1）。
// 拆分为：
//   auth_phone.go      本文件：手机号验证码登录/注册
//   auth_realname.go   实名申请/状态/国家实名 API 代理（不存储姓名与身份证明文）
package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"time"

	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/security/slidecaptcha"
	"nvs-server/security/sms"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// POST /api/auth/phone/reset-password — 手机号验证码重置密码（F22）
func PhoneResetPassword(c *gin.Context) {
	var req struct {
		Phone       string `json:"phone" binding:"required"`
		Code        string `json:"code" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: 新密码长度至少6位")
		return
	}
	phone := sms.NormalizePhone(req.Phone)
	if !sms.IsValidCNMobile(phone) {
		utils.BadRequest(c, "手机号格式不正确")
		return
	}
	if err := utils.VerifyPhoneCode(phone, strings.TrimSpace(req.Code), utils.PhonePurposeReset); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	user, err := models.GetUserByPhone(phone)
	if err != nil || user == nil {
		// 防手机号枚举：统一话术
		utils.NotFound(c, "该手机号未注册或无法重置")
		return
	}
	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		utils.InternalError(c, "密码处理失败")
		return
	}
	if err := models.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("password_hash", hash).Error; err != nil {
		utils.InternalError(c, "重置密码失败")
		return
	}
	// 重置成功后作废该手机号全部未消费验证码
	utils.DeletePhoneCode(phone, utils.PhonePurposeReset)
	recordUserAudit(c, user.ID, models.AuditChangePassword, "通过手机号验证码重置密码", "success")
	utils.SuccessMessage(c, "密码重置成功，请重新登录", nil)
}

// POST /api/auth/phone/send-code — 发送手机验证码
// 防滥用：滑块验证码（开启时）+ 手机号 60s/日上限 + IP 日上限（见 utils/phonecode.go）。
func SendPhoneCode(c *gin.Context) {
	var req struct {
		Phone        string `json:"phone" binding:"required"`
		Purpose      string `json:"purpose"` // login=登录/注册 reset=找回密码
		CaptchaToken string `json:"captcha_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请提供有效的手机号")
		return
	}
	phone := sms.NormalizePhone(req.Phone)
	if !sms.IsValidCNMobile(phone) {
		utils.BadRequest(c, "手机号格式不正确")
		return
	}
	purpose := strings.TrimSpace(req.Purpose)
	if purpose == "" {
		purpose = utils.PhonePurposeLogin
	}
	if purpose != utils.PhonePurposeLogin && purpose != utils.PhonePurposeReset {
		utils.BadRequest(c, "不支持的验证码用途")
		return
	}

	// 🔒 与邮箱发码同策略：开启滑块验证码时必须先过人机校验（防短信轰炸/裸调）
	if models.IsCaptchaEnabled() {
		if req.CaptchaToken == "" || !slidecaptcha.VerifyPassToken(req.CaptchaToken) {
			utils.Forbidden(c, "请先完成滑块验证后再获取短信验证码")
			return
		}
	}

	code, err := utils.GeneratePhoneCode(phone, purpose, c.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrPhoneCodeTooOften):
			utils.BadRequest(c, err.Error())
		case errors.Is(err, utils.ErrPhoneCodeQuota):
			utils.BadRequest(c, err.Error())
		default:
			utils.InternalError(c, "验证码生成失败")
		}
		return
	}

	if err := sms.SendCode(c.Request.Context(), phone, code); err != nil {
		// 发送失败作废验证码，避免用户拿到收不到的码
		utils.DeletePhoneCode(phone, purpose)
		utils.InternalError(c, "短信发送失败: "+err.Error())
		return
	}

	utils.SuccessMessage(c, "验证码已发送，请查收短信", gin.H{
		"backend": utils.PhoneCodeBackend(),
	})
}

// POST /api/auth/phone/login — 手机号验证码登录（不存在则自动注册）
func PhoneLogin(c *gin.Context) {
	if !models.GetLoginMethods().Phone {
		utils.Forbidden(c, "站长已关闭手机号登录")
		return
	}
	var req struct {
		Phone string `json:"phone" binding:"required"`
		Code  string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	phone := sms.NormalizePhone(req.Phone)
	code := strings.TrimSpace(req.Code)
	if !sms.IsValidCNMobile(phone) {
		utils.BadRequest(c, "手机号格式不正确")
		return
	}

	if err := utils.VerifyPhoneCode(phone, code, utils.PhonePurposeLogin); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	user, err := models.GetUserByPhone(phone)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			utils.InternalError(c, "服务器错误")
			return
		}
		// 首次手机号登录 → 自动注册（176号令合规：记录注册 IP）
		user, err = createPhoneUser(c, phone)
		if err != nil {
			utils.InternalError(c, "账号创建失败: "+err.Error())
			return
		}
		recordUserAudit(c, user.ID, models.AuditRegister, "手机号注册 "+phone, "success")
	} else {
		// 账号锁定检查（与邮箱登录一致）
		if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
			remaining := int(time.Until(*user.LockedUntil).Minutes()) + 1
			utils.BadRequest(c, "登录失败次数过多，账号已锁定，请在 "+itoa(remaining)+" 分钟后重试")
			return
		}
	}

	issueSession(c, user, "手机号登录 "+phone)
	utils.SuccessMessage(c, "登录成功", gin.H{"user": user})
}

// createPhoneUser 为手机号创建账号：占位邮箱（唯一）+ 随机密码哈希 + 默认读者角色。
func createPhoneUser(c *gin.Context, phone string) (*models.User, error) {
	username, err := generateUniqueUsername(phone)
	if err != nil {
		return nil, err
	}
	// 随机密码哈希：手机号用户不经密码登录，但 PasswordHash 为 not null 列
	randPass := make([]byte, 24)
	if _, err := rand.Read(randPass); err != nil {
		return nil, err
	}
	hash, err := utils.HashPassword("ph!" + hex.EncodeToString(randPass))
	if err != nil {
		return nil, err
	}
	user := &models.User{
		Username:         username,
		Email:            "p" + phone + "@phone.nvs.local", // 占位邮箱（唯一，不可用于登录）
		PasswordHash:     hash,
		Nickname:         "手机用户" + phone[len(phone)-4:],
		Role:             "reader",
		Phone:            phone,
		RegistrationIP:   c.ClientIP(),
	}
	if err := models.CreateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

// generateUniqueUsername 生成唯一用户名：手机号后 4 位 + 随机后缀，冲突时重试。
func generateUniqueUsername(phone string) (string, error) {
	base := "u" + phone[len(phone)-4:]
	for i := 0; i < 8; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(100000))
		if err != nil {
			n = big.NewInt(int64(time.Now().UnixNano() % 100000))
		}
		candidate := base + int64ToStr(n.Int64())
		var count int64
		models.DB.Model(&models.User{}).Where("username = ?", candidate).Count(&count)
		if count == 0 {
			return candidate, nil
		}
	}
	return "", errors.New("用户名生成冲突，请重试")
}

// issueSession 签发访问/刷新令牌 Cookie 并记录登录审计与最后登录信息。
func issueSession(c *gin.Context, user *models.User, auditDesc string) {
	now := time.Now()
	models.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"login_fail_count": 0,
		"locked_until":     nil,
		"last_login_ip":    c.ClientIP(),
		"last_login_at":    &now,
	})
	recordUserAudit(c, user.ID, models.AuditLoginSuccess, auditDesc, "success")

	accessToken, err := utils.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return
	}
	refreshToken, err := utils.GenerateRefreshToken(user.ID, user.Email, user.Role)
	if err != nil {
		return
	}
	middleware.SetTokenCookie(c, accessToken)
	middleware.SetRefreshCookie(c, refreshToken)
}

func intToStr(n int) string    { return int64ToStr(int64(n)) }
func int64ToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [24]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// sha256Hex 小工具（实名查重）
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
