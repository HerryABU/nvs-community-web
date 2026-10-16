// Package sms 提供可插拔短信网关：log（开发日志网关）与 http（通用 Webhook 网关）。
// 通过 config.SMSGateway 选择实现；新增云厂商网关只需实现 Gateway 接口并注册。
package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"nvs-server/config"
)

// Gateway 短信网关接口：发送验证码短信。
// phone 为 E.164 或国内号码（调用方已校验），code 为纯数字验证码。
type Gateway interface {
	Name() string
	Send(ctx context.Context, phone, code string) error
}

// LogGateway 开发网关：不真实发送，把验证码写入服务日志（生产禁用）。
type LogGateway struct{}

func (LogGateway) Name() string { return "log" }

func (LogGateway) Send(_ context.Context, phone, code string) error {
	log.Printf("[sms][log-gateway] 验证码短信（未真实发送）phone=%s code=%s", maskPhone(phone), code)
	return nil
}

// HTTPGateway 通用 HTTP 网关：POST JSON {phone,code,sign_name,template_code} 到配置 URL，
// 请求头 Authorization: Bearer <token>。适配自建短信网关或云厂商 API 网关封装。
type HTTPGateway struct {
	url      string
	token    string
	signName string
	template string
	client   *http.Client
}

func NewHTTPGateway() *HTTPGateway {
	timeout := time.Duration(config.SMSTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPGateway{
		url:      config.SMSHTTPURL,
		token:    config.SMSHTTPToken,
		signName: config.SMSSignName,
		template: config.SMSTemplateCode,
		client:   &http.Client{Timeout: timeout},
	}
}

func (g *HTTPGateway) Name() string { return "http" }

func (g *HTTPGateway) Send(ctx context.Context, phone, code string) error {
	if g.url == "" {
		return fmt.Errorf("短信网关未配置 SMS_HTTP_URL")
	}
	payload := map[string]string{
		"phone":         phone,
		"code":          code,
		"sign_name":     g.signName,
		"template_code": g.template,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("短信网关请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("短信网关返回状态 %d", resp.StatusCode)
	}
	return nil
}

// current 当前网关实例（进程级，配置在 Init 时确定）
var current Gateway = LogGateway{}

// Init 按配置选择网关（main 启动时调用一次）。
func Init() {
	switch config.SMSGateway {
	case "http":
		current = NewHTTPGateway()
	default:
		current = LogGateway{}
	}
	log.Printf("[sms] 短信网关: %s", current.Name())
}

// SendCode 发送验证码短信（进程级入口）。
func SendCode(ctx context.Context, phone, code string) error {
	return current.Send(ctx, phone, code)
}

// CurrentName 当前网关名（供健康检查/管理端展示）。
func CurrentName() string { return current.Name() }

// maskPhone 日志脱敏：138****5678
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return "***"
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// NormalizePhone 规范化国内手机号：去空格/横线/+86/86 前缀。
func NormalizePhone(phone string) string {
	p := strings.TrimSpace(phone)
	p = strings.ReplaceAll(p, "-", "")
	p = strings.ReplaceAll(p, " ", "")
	p = strings.TrimPrefix(p, "+86")
	p = strings.TrimPrefix(p, "86")
	return p
}

// IsValidCNMobile 大陆手机号格式校验（1 开头 11 位，第二位 3-9）。
func IsValidCNMobile(phone string) bool {
	p := NormalizePhone(phone)
	if len(p) != 11 {
		return false
	}
	if p[0] != '1' || p[1] < '3' || p[1] > '9' {
		return false
	}
	for i := 0; i < 11; i++ {
		if p[i] < '0' || p[i] > '9' {
			return false
		}
	}
	return true
}
