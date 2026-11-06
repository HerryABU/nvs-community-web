package payment

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	wallet "nvs-server/models/wallet"
)

// #67 支付宝当面付 alipay.trade.precreate 直连（扫码）。
// 下单：应用私钥 RSA2 签名，gateway.do 返回 qr_code。
// 回调：支付宝公钥验 RSA2 签名，金额按「元」上报需换算成分。

const (
	aliGatewayProd    = "https://openapi.alipay.com/gateway.do"
	aliGatewaySandbox = "https://openapi.alipaydev.com/gateway.do"
)

var aliHTTPClient = &http.Client{Timeout: 15 * time.Second}
var aliGatewayBase = "" // 单测可覆盖；空则按 cfg.AliSandbox 选官方地址

func aliGateway(cfg wallet.PaymentConfig) string {
	if aliGatewayBase != "" {
		return aliGatewayBase
	}
	if cfg.AliSandbox {
		return aliGatewaySandbox
	}
	return aliGatewayProd
}

func aliCreateOrder(p CreateParams, cfg wallet.PaymentConfig) (string, error) {
	if strings.TrimSpace(cfg.AliAppID) == "" {
		return "", errors.New("支付宝直连未配置 AppID")
	}
	if cfg.AliPrivateKey == "" || cfg.AliPublicKey == "" {
		return "", errors.New("支付宝直连未配置应用私钥 / 支付宝公钥")
	}
	if p.AmountFen <= 0 {
		return "", errors.New("订单金额无效")
	}
	if strings.TrimSpace(p.NotifyURL) == "" {
		return "", errors.New("支付宝直连需配置公网可达的异步通知地址")
	}
	priv, err := parseRSAPrivateKey(cfg.AliPrivateKey)
	if err != nil {
		return "", err
	}
	biz, err := json.Marshal(map[string]interface{}{
		"out_trade_no":    p.OrderNo,
		"total_amount":    fenToYuan(p.AmountFen),
		"subject":         firstNonEmpty(strings.TrimSpace(p.Subject), "账户充值"),
		"store_id":        "NVS",
		"timeout_express": "30m",
	})
	if err != nil {
		return "", err
	}
	params := map[string]string{
		"app_id":      cfg.AliAppID,
		"method":      "alipay.trade.precreate",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"notify_url":  channelNotifyURL(p.NotifyURL, wallet.PayChannelAliPay),
		"biz_content": string(biz),
	}
	sig, err := rsaSHA256Sign(priv, canonicalQuery(params))
	if err != nil {
		return "", fmt.Errorf("支付宝下单签名失败: %w", err)
	}
	params["sign"] = sig

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequest(http.MethodPost, aliGateway(cfg), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	resp, err := aliHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("支付宝下单请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("支付宝下单返回 %d", resp.StatusCode)
	}
	var out struct {
		Response struct {
			Code       string `json:"code"`
			Msg        string `json:"msg"`
			SubCode    string `json:"sub_code"`
			SubMsg     string `json:"sub_msg"`
			QRCode     string `json:"qr_code"`
			OutTradeNo string `json:"out_trade_no"`
		} `json:"alipay_trade_precreate_response"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("支付宝下单响应解析失败: %w", err)
	}
	if out.Response.Code != "10000" || out.Response.QRCode == "" {
		return "", fmt.Errorf("支付宝下单失败: %s %s",
			firstNonEmpty(out.Response.SubMsg, out.Response.Msg),
			firstNonEmpty(out.Response.SubCode, out.Response.Code))
	}
	return out.Response.QRCode, nil
}

// aliVerifyNotifyForm 支付宝异步通知验签（RSA2）+ 金额/状态解析。
func aliVerifyNotifyForm(form url.Values, cfg wallet.PaymentConfig) (*NotifyResult, error) {
	if cfg.AliPublicKey == "" {
		return nil, errors.New("支付宝直连未配置支付宝公钥，无法验签回调")
	}
	if !strings.EqualFold(strings.TrimSpace(form.Get("sign_type")), "RSA2") {
		return nil, errors.New("支付宝回调签名类型仅支持 RSA2")
	}
	sig := strings.TrimSpace(form.Get("sign"))
	if sig == "" {
		return nil, errors.New("支付宝回调缺少签名")
	}
	params := map[string]string{}
	for k, v := range form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	pub, err := parseRSAPublicKey(cfg.AliPublicKey)
	if err != nil {
		return nil, err
	}
	if err := rsaSHA256Verify(pub, canonicalQuery(params, "sign", "sign_type"), sig); err != nil {
		return nil, fmt.Errorf("支付宝回调验签失败: %w", err)
	}
	status := strings.TrimSpace(form.Get("trade_status"))
	if status != "TRADE_SUCCESS" && status != "TRADE_FINISHED" {
		return nil, fmt.Errorf("支付宝交易状态未成功: %s", status)
	}
	orderNo := strings.TrimSpace(form.Get("out_trade_no"))
	txnID := strings.TrimSpace(form.Get("trade_no"))
	if orderNo == "" || txnID == "" {
		return nil, errors.New("支付宝回调缺少订单号或渠道流水号")
	}
	fen, err := yuanToFen(strings.TrimSpace(form.Get("total_amount")))
	if err != nil || fen <= 0 {
		return nil, fmt.Errorf("支付宝回调金额非法: %q", form.Get("total_amount"))
	}
	return &NotifyResult{
		OrderNo: orderNo, TxnID: txnID,
		AmountFen: fen, TradeStatus: status,
	}, nil
}

// fenToYuan 分 → 元字符串（两位小数，避免浮点展示误差）。
func fenToYuan(fen int64) string {
	return strconv.FormatInt(fen/100, 10) + "." + fmt.Sprintf("%02d", fen%100)
}

// yuanToFen 元字符串 → 分（四舍五入到分，支付宝金额最多两位小数）。
func yuanToFen(s string) (int64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(v * 100)), nil
}
