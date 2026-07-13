package utils

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

// ============================================================
// SMTP 配置与发送基础设施
// ============================================================

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

// SendVerificationEmail 发送验证邮件
func SendVerificationEmail(to, code string) error {
	cfg := getSMTPConfig()
	return sendEmail(cfg, to, "【星海文学】邮箱验证码", fmt.Sprintf(verificationEmailTemplate, code))
}

// SendPasswordResetEmail 发送密码重置邮件
func SendPasswordResetEmail(to, code string) error {
	cfg := getSMTPConfig()
	return sendEmail(cfg, to, "【星海文学】密码重置", fmt.Sprintf(passwordResetEmailTemplate, code))
}

// sendEmail 发送邮件的底层函数
func sendEmail(cfg SMTPConfig, to, subject, body string) error {
	if cfg.User == "" || cfg.Password == "" {
		return fmt.Errorf("SMTP 未配置，请联系站长")
	}

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
