// Package payment 支付渠道提供者抽象。
//
// 两种实现：
//   - MockProvider：模拟支付（开发/演示），不接真实网关，到账由站内受登录
//     保护的「模拟支付成功」按钮触发，便于无资质站长与测试环境走通全链路；
//   - AggregateProvider：通用聚合支付（易支付/码支付类），微信/支付宝经网关
//     type 参数（wxpay/alipay）下发，MD5 签名，异步通知验签到账。
//
// 真实微信/支付宝直连（需商户资质）不在此列——站长资质门槛高，聚合网关是
// 个人站长的现实路径；后续如需直连，实现同一 Provider 接口即可平滑扩展。
package payment

import (
	"net/url"

	"nvs-server/models"
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
}

// Provider 支付渠道提供者
type Provider interface {
	// Name 提供者标识（mock / aggregate）
	Name() string
	// CreateOrder 向上游下单，返回收银台地址（用户跳转或扫码）
	CreateOrder(p CreateParams, cfg models.PaymentConfig) (payURL string, err error)
	// VerifyNotify 验签并解析异步通知表单；验签失败返回 error
	VerifyNotify(form url.Values, cfg models.PaymentConfig) (*NotifyResult, error)
	// NotifyAck 回调应答体（成功 success，失败 fail）
	NotifyAck(ok bool) string
}

// GetProvider 按配置返回 provider（缺省 mock）
func GetProvider(cfg models.PaymentConfig) Provider {
	if cfg.Provider == models.PayProviderAggregate {
		return AggregateProvider{}
	}
	return MockProvider{}
}

// channelToGatewayType 渠道 → 聚合网关 type 参数
func channelToGatewayType(channel string) string {
	switch channel {
	case models.PayChannelWeChat:
		return "wxpay"
	case models.PayChannelAliPay:
		return "alipay"
	default:
		return ""
	}
}
