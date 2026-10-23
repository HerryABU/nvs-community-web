package payment

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
)

// AggregateProvider 通用聚合支付 provider（易支付 / 码支付类）。
//
// 协议要点（主流聚合网关事实标准）：
//   - 下单：POST application/x-www-form-urlencoded，字段含 pid/type/out_trade_no/
//     notify_url/return_url/name/money/clientip/sign/sign_type=MD5；
//   - 签名：参数按 key 升序拼 "k=v&..."（跳过 sign/sign_type 与空值），
//     末尾拼接商户密钥，取 MD5 十六进制小写；
//   - 异步通知：同规则验签，trade_status=TRADE_SUCCESS 表示成功；
//   - 应答：纯文本 "success"（其余任何应答都会触发网关重推）。
type AggregateProvider struct{}

// Name 提供者标识
func (AggregateProvider) Name() string { return models.PayProviderAggregate }

// md5Sign 聚合网关 MD5 签名
func md5Sign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(params[k])
		sb.WriteString("&")
	}
	sb.WriteString("key=")
	sb.WriteString(secret)
	sum := md5.Sum([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// CreateOrder 向聚合网关下单，返回收银台地址
func (AggregateProvider) CreateOrder(p CreateParams, cfg models.PaymentConfig) (string, error) {
	gwType := channelToGatewayType(p.Channel)
	if gwType == "" {
		return "", errors.New("不支持的支付渠道")
	}
	if cfg.GatewayURL == "" || cfg.AppID == "" || cfg.Secret == "" {
		return "", errors.New("支付通道未完整配置（网关地址/商户ID/密钥）")
	}
	money := fmt.Sprintf("%d.%02d", p.AmountFen/100, p.AmountFen%100)
	params := map[string]string{
		"pid":        cfg.AppID,
		"type":       gwType,
		"out_trade_no": p.OrderNo,
		"notify_url": cfg.NotifyURL,
		"return_url": cfg.ReturnURL,
		"name":       p.Subject,
		"money":      money,
		"clientip":   p.ClientIP,
		"sign_type":  "MD5",
	}
	params["sign"] = md5Sign(params, cfg.Secret)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.PostForm(cfg.GatewayURL, toURLValues(params))
	if err != nil {
		return "", fmt.Errorf("网关请求失败：%v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", fmt.Errorf("网关响应读取失败：%v", err)
	}
	var out struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		PayURL  string `json:"payurl"`
		QRCode  string `json:"qrcode"`
		CodeURL string `json:"codeurl"`
		URL     string `json:"url"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("网关响应解析失败：%v", err)
	}
	if out.Code != 1 {
		if out.Msg == "" {
			out.Msg = "未知错误"
		}
		return "", fmt.Errorf("网关拒绝下单：%s", out.Msg)
	}
	payURL := firstNonEmpty(out.PayURL, out.QRCode, out.CodeURL, out.URL)
	if payURL == "" {
		return "", errors.New("网关未返回收银台地址")
	}
	return payURL, nil
}

// VerifyNotify 验签并解析异步通知
func (AggregateProvider) VerifyNotify(form url.Values, cfg models.PaymentConfig) (*NotifyResult, error) {
	if cfg.Secret == "" {
		return nil, errors.New("商户密钥未配置")
	}
	if strings.ToUpper(form.Get("sign_type")) != "MD5" {
		return nil, errors.New("不支持的签名类型")
	}
	got := strings.ToLower(strings.TrimSpace(form.Get("sign")))
	if got == "" {
		return nil, errors.New("回调缺少签名")
	}
	params := map[string]string{}
	for k, v := range form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	want := md5Sign(params, cfg.Secret)
	if got != want {
		return nil, errors.New("验签失败")
	}
	tradeStatus := form.Get("trade_status")
	if tradeStatus != "TRADE_SUCCESS" {
		return nil, fmt.Errorf("交易状态非成功：%s", tradeStatus)
	}
	orderNo := strings.TrimSpace(form.Get("out_trade_no"))
	if orderNo == "" {
		return nil, errors.New("回调缺少商户订单号")
	}
	fen, err := ParseMoneyFen(form.Get("money"))
	if err != nil {
		return nil, fmt.Errorf("回调金额无法解析：%v", err)
	}
	return &NotifyResult{
		OrderNo:     orderNo,
		TxnID:       strings.TrimSpace(form.Get("trade_no")),
		AmountFen:   fen,
		TradeStatus: tradeStatus,
	}, nil
}

// NotifyAck 回调应答（网关要求纯文本 success）
func (AggregateProvider) NotifyAck(ok bool) string {
	if ok {
		return "success"
	}
	return "fail"
}

// ParseMoneyFen 解析 "12.34" 形式的元金额为分（不使用浮点，杜绝精度误差）。
// 兼容 "12"、"12.3"（视为 12.30 元）。
func ParseMoneyFen(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("金额为空")
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	parts := strings.SplitN(s, ".", 2)
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}
	cent := int64(0)
	if len(parts) == 2 && parts[1] != "" {
		frac := parts[1]
		if len(frac) > 2 {
			frac = frac[:2]
		}
		for len(frac) < 2 {
			frac += "0"
		}
		cent, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	fen := yuan*100 + cent
	if neg {
		fen = -fen
	}
	return fen, nil
}

// ---------- 小工具 ----------

func toURLValues(m map[string]string) url.Values {
	v := url.Values{}
	for k, val := range m {
		v.Set(k, val)
	}
	return v
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
