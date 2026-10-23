package payment

import (
	"errors"
	"net/url"

	"nvs-server/config"
	"nvs-server/models"
)

// MockProvider 模拟支付 provider：不接真实网关。
//
// - CreateOrder 返回站内模拟收银台相对路径（前端 SPA 路由 /pay/mock/:orderNo，
//   knownSPARoots 已放行 "pay" 根段）；
// - VerifyNotify 不应被调用：mock 到账走受登录保护的 confirm 接口
//   （handlers.PaymentMockConfirm），与真实回调路径物理隔离。
type MockProvider struct{}

// Name 提供者标识
func (MockProvider) Name() string { return models.PayProviderMock }

// CreateOrder 返回模拟收银台地址（带部署前缀，如 /go-nvs/pay/mock/R2026...）
func (MockProvider) CreateOrder(p CreateParams, _ models.PaymentConfig) (string, error) {
	if p.OrderNo == "" {
		return "", errors.New("订单号为空")
	}
	return config.URLPrefix + "/pay/mock/" + p.OrderNo, nil
}

// VerifyNotify mock 无异步通知（返回错误防误用）
func (MockProvider) VerifyNotify(_ url.Values, _ models.PaymentConfig) (*NotifyResult, error) {
	return nil, errors.New("模拟支付无异步通知，请使用站内确认接口")
}

// NotifyAck 应答体
func (MockProvider) NotifyAck(ok bool) string {
	if ok {
		return "success"
	}
	return "fail"
}
