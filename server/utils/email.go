package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"
)

// VerificationCode 邮箱验证码记录
type VerificationCode struct {
	Code      string
	ExpiresAt time.Time
}

var (
	codeStore   = make(map[string]VerificationCode)
	codeStoreMu sync.RWMutex
)

// SMTPConfig SMTP 配置
type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

// SMTPConfigProvider SMTP 配置提供者（由外部设置以避免循环依赖）
var SMTPConfigProvider func() SMTPConfig

// getSMTPConfig 获取 SMTP 配置：先查 provider（平台配置），再查环境变量
func getSMTPConfig() SMTPConfig {
	if SMTPConfigProvider != nil {
		cfg := SMTPConfigProvider()
		if cfg.Host != "" && cfg.User != "" {
			return cfg
		}
	}
	// 环境变量 fallback
	return SMTPConfig{
		Host:     getEnvOr("SMTP_HOST", "smtp.qq.com"),
		Port:     getEnvOr("SMTP_PORT", "587"),
		User:     os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     getEnvOr("SMTP_FROM", os.Getenv("SMTP_USER")),
	}
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GenerateVerificationCode 生成6位数字验证码
func GenerateVerificationCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

// SendVerificationEmail 发送验证邮件
func SendVerificationEmail(to, code string) error {
	cfg := getSMTPConfig()

	if cfg.User == "" || cfg.Password == "" {
		return fmt.Errorf("SMTP 未配置，请联系站长")
	}

	subject := "【星海文学】邮箱验证码"
	body := fmt.Sprintf(`
<html>
<body style="font-family: Arial, sans-serif; padding: 20px;">
  <div style="max-width: 480px; margin: 0 auto; background: #f9fafb; border-radius: 8px; padding: 24px;">
    <h2 style="color: #1a1a2e; margin-top: 0;">星海文学 · 邮箱验证</h2>
    <p>您的验证码是：</p>
    <div style="background: #fff; border: 1px solid #e5e7eb; border-radius: 6px; padding: 16px; text-align: center; margin: 16px 0;">
      <span style="font-size: 28px; font-weight: 700; letter-spacing: 6px; color: #2563eb;">%s</span>
    </div>
    <p style="color: #6b7280; font-size: 14px;">验证码 10 分钟内有效，请勿泄露给他人。</p>
    <p style="color: #9ca3af; font-size: 12px; margin-top: 24px;">如果这不是您本人的操作，请忽略此邮件。</p>
  </div>
</body>
</html>`, code)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		cfg.From, to, subject, body)

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)

	err := smtp.SendMail(addr, auth, cfg.From, []string{to}, []byte(msg))
	if err != nil {
		if strings.Contains(err.Error(), "unencrypted connection") {
			return smtp.SendMail(addr, nil, cfg.From, []string{to}, []byte(msg))
		}
		return err
	}
	return nil
}

// StoreVerificationCode 存储验证码（10分钟有效）
func StoreVerificationCode(email, code string) {
	codeStoreMu.Lock()
	defer codeStoreMu.Unlock()
	codeStore[email] = VerificationCode{
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
}

// VerifyCode 验证验证码
func VerifyCode(email, code string) bool {
	codeStoreMu.RLock()
	defer codeStoreMu.RUnlock()
	vc, ok := codeStore[email]
	if !ok {
		return false
	}
	if time.Now().After(vc.ExpiresAt) {
		delete(codeStore, email)
		return false
	}
	return vc.Code == code
}

// DeleteVerificationCode 删除验证码
func DeleteVerificationCode(email string) {
	codeStoreMu.Lock()
	defer codeStoreMu.Unlock()
	delete(codeStore, email)
}
