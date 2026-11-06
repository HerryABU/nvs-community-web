// Package payment 支付渠道提供者抽象。
//
// 两种实现：
//
//   - MockProvider：模拟支付（开发/演示），不接真实网关，到账由站内受登录
//     保护的「模拟支付成功」按钮触发，便于无资质站长与测试环境走通全链路；
//
//   - AggregateProvider：通用聚合支付（易支付/码支付类），微信/支付宝经网关
//     type 参数（wxpay/alipay）下发，MD5 签名，异步通知验签到账。
//
//   - DirectProvider（#67）：官方直连，微信支付 APIv3 Native 扫码 +
//     支付宝当面付 precreate，RSA-SHA256 签名、回调原始报文验签。
//     商户自行签约，不经第三方聚合；需公网可达的回调地址与商户资质。
package payment

import (
	"net/http"
	"net/url"

	wallet "nvs-server/models/wallet"
)

// CreateParams 下单参数（handler 从订单 + 配置组装）
type CreateParams struct {
	OrderNo   string // 平台订单号
	AmountFen int64  // 金额（分）
	Channel   string // wechat / alipay
	Subject   string // 订单标题（网关展示）
	NotifyURL string // 异步通知地址
	ReturnURL string // 支付完成跳转地址
	ClientIP  string // 下单用户 IP
}

// NotifyResult 验签通过的异步通知
type NotifyResult struct {
	OrderNo     string // 平台订单号
	TxnID       string // 渠道流水号
	AmountFen   int64  // 渠道实付金额（分）
	TradeStatus string // 交易状态（TRADE_SUCCESS 等）

	// #67 直连通道的应答格式与聚合网关不同（微信 v3 要求 JSON 且失败用 4xx/5xx，
	// 支付宝要求纯文本 success/fail）。留空则由 handler 走 Provider.NotifyAck 默认应答。
	AckStatus      int    `json:"-"`
	AckContentType string `json:"-"`
	AckBody        string `json:"-"`
}

// RawNotifyVerifier 可选接口：直接消费原始报文与 HTTP 头的回调验签器。
// 微信支付 APIv3 的回调是 JSON body + Wechatpay-* 头签名，无法用 url.Values 表达；
// 实现该接口的 provider（direct）由回调 handler 优先调用，未实现的沿用表单验签。
type RawNotifyVerifier interface {
	VerifyNotifyRaw(body []byte, hdr http.Header, cfg wallet.PaymentConfig) (*NotifyResult, error)
}

// Provider 支付渠道提供者
type Provider interface {
	// Name 提供者标识（mock / aggregate）
	Name() string
	// CreateOrder 向上游下单，返回收银台地址（用户跳转或扫码）
	CreateOrder(p CreateParams, cfg wallet.PaymentConfig) (payURL string, err error)
	// VerifyNotify 验签并解析异步通知表单；验签失败返回 error
	VerifyNotify(form url.Values, cfg wallet.PaymentConfig) (*NotifyResult, error)
	// NotifyAck 回调应答体（成功 success，失败 fail）
	NotifyAck(ok bool) string
}

// GetProvider 按配置返回 provider（缺省 mock）
func GetProvider(cfg wallet.PaymentConfig) Provider {
	switch cfg.Provider {
	case wallet.PayProviderAggregate:
		return AggregateProvider{}
	case wallet.PayProviderDirect:
		return DirectProvider{}
	default:
		return MockProvider{}
	}
}

// channelToGatewayType 渠道 → 聚合网关 type 参数
func channelToGatewayType(channel string) string {
	switch channel {
	case wallet.PayChannelWeChat:
		return "wxpay"
	case wallet.PayChannelAliPay:
		return "alipay"
	default:
		return ""
	}
}
