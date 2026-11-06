package payment

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	wallet "nvs-server/models/wallet"
)

// #67 微信支付 APIv3 Native 扫码直连。
// 下单：POST /v3/pay/transactions/native，RSA-SHA256 商户私钥签名；返回 code_url。
// 回调：Wechatpay-* 头签名（平台证书验签）+ AEAD_AES_256_GCM 解密 resource。

// wxAPIBase 便于单测指向 httptest 上游，生产为官方地址。
var wxAPIBase = "https://api.mch.weixin.qq.com"
var wxHTTPClient = &http.Client{Timeout: 15 * time.Second}

const wxNativePath = "/v3/pay/transactions/native"

type wxNativeRequest struct {
	AppID       string       `json:"appid"`
	MchID       string       `json:"mchid"`
	Description string       `json:"description"`
	OutTradeNo  string       `json:"out_trade_no"`
	NotifyURL   string       `json:"notify_url"`
	Amount      wxAmount     `json:"amount"`
	SceneInfo   *wxSceneInfo `json:"scene_info,omitempty"`
}

type wxAmount struct {
	Total    int64  `json:"total"`
	Currency string `json:"currency"`
}

type wxSceneInfo struct {
	PayerClientIP string `json:"payer_client_ip"`
}

func wxCreateOrder(p CreateParams, cfg wallet.PaymentConfig) (string, error) {
	if strings.TrimSpace(cfg.WxAppID) == "" || strings.TrimSpace(cfg.WxMchID) == "" {
		return "", errors.New("微信直连未配置 AppID / 商户号")
	}
	if cfg.WxSerialNo == "" || cfg.WxPrivateKey == "" || cfg.WxAPIV3Key == "" {
		return "", errors.New("微信直连未配置证书序列号 / 商户私钥 / APIv3 密钥")
	}
	if p.AmountFen <= 0 {
		return "", errors.New("订单金额无效")
	}
	notifyURL := strings.TrimSpace(p.NotifyURL)
	if notifyURL == "" {
		return "", errors.New("微信直连需配置公网可达的异步通知地址")
	}
	// 渠道回调地址按通道分流，避免把聚合网关的通用地址直接拿给官方通道
	notifyURL = channelNotifyURL(notifyURL, wallet.PayChannelWeChat)

	reqBody := wxNativeRequest{
		AppID: cfg.WxAppID, MchID: cfg.WxMchID,
		Description: firstNonEmpty(strings.TrimSpace(p.Subject), "账户充值"),
		OutTradeNo:  p.OrderNo, NotifyURL: notifyURL,
		Amount: wxAmount{Total: p.AmountFen, Currency: "CNY"},
	}
	if strings.TrimSpace(p.ClientIP) != "" {
		reqBody.SceneInfo = &wxSceneInfo{PayerClientIP: p.ClientIP}
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	priv, err := parseRSAPrivateKey(cfg.WxPrivateKey)
	if err != nil {
		return "", err
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonceBytes := make([]byte, 16)
	_, _ = rand.Read(nonceBytes)
	nonce := hex.EncodeToString(nonceBytes)
	message := "POST\n" + wxNativePath + "\n" + ts + "\n" + nonce + "\n" + string(body) + "\n"
	sig, err := rsaSHA256Sign(priv, message)
	if err != nil {
		return "", fmt.Errorf("微信下单签名失败: %w", err)
	}
	auth := fmt.Sprintf(
		`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		cfg.WxMchID, nonce, sig, ts, cfg.WxSerialNo)

	req, err := http.NewRequest(http.MethodPost, wxAPIBase+wxNativePath, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "NVS-Server/1.0")
	resp, err := wxHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("微信下单请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("微信下单返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var out struct {
		CodeURL string `json:"code_url"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("微信下单响应解析失败: %w", err)
	}
	if out.CodeURL == "" {
		return "", fmt.Errorf("微信下单未返回二维码: %s", firstNonEmpty(out.Message, out.Code))
	}
	return out.CodeURL, nil
}

// wxVerifyNotifyRaw 校验 Wechatpay-Signature，并解密 resource 取出订单与金额。
func wxVerifyNotifyRaw(body []byte, hdr http.Header, cfg wallet.PaymentConfig) (*NotifyResult, error) {
	if cfg.WxPlatformCert == "" {
		return nil, errors.New("微信直连未配置平台证书/公钥，无法验签回调")
	}
	if cfg.WxAPIV3Key == "" {
		return nil, errors.New("微信直连未配置 APIv3 密钥，无法解密回调")
	}
	sig := strings.TrimSpace(hdr.Get("Wechatpay-Signature"))
	tsStr := strings.TrimSpace(hdr.Get("Wechatpay-Timestamp"))
	nonce := strings.TrimSpace(hdr.Get("Wechatpay-Nonce"))
	if sig == "" || tsStr == "" || nonce == "" {
		return nil, errors.New("微信回调缺少验签头")
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return nil, errors.New("微信回调时间戳非法")
	}
	if d := time.Now().Unix() - ts; d > 300 || d < -300 {
		return nil, errors.New("微信回调时间戳超出 5 分钟窗口（防重放）")
	}
	pub, err := parseRSAPublicKey(cfg.WxPlatformCert)
	if err != nil {
		return nil, err
	}
	message := tsStr + "\n" + nonce + "\n" + string(body) + "\n"
	if err := rsaSHA256Verify(pub, message, sig); err != nil {
		return nil, fmt.Errorf("微信回调验签失败: %w", err)
	}

	var envelope struct {
		EventType string `json:"event_type"`
		Resource  struct {
			Algorithm      string `json:"algorithm"`
			Ciphertext     string `json:"ciphertext"`
			Nonce          string `json:"nonce"`
			AssociatedData string `json:"associated_data"`
			OriginalType   string `json:"original_type"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("微信回调报文解析失败: %w", err)
	}
	if envelope.Resource.Algorithm != "AEAD_AES_256_GCM" {
		return nil, errors.New("微信回调加密算法不支持")
	}
	plain, err := wxDecryptResource(cfg.WxAPIV3Key, envelope.Resource.Ciphertext,
		envelope.Resource.Nonce, envelope.Resource.AssociatedData)
	if err != nil {
		return nil, err
	}
	var tx struct {
		OutTradeNo    string `json:"out_trade_no"`
		TransactionID string `json:"transaction_id"`
		TradeState    string `json:"trade_state"`
		Amount        struct {
			Total      int64 `json:"total"`
			PayerTotal int64 `json:"payer_total"`
		} `json:"amount"`
	}
	if err := json.Unmarshal(plain, &tx); err != nil {
		return nil, fmt.Errorf("微信回调交易明文解析失败: %w", err)
	}
	if tx.TradeState != "SUCCESS" {
		return nil, fmt.Errorf("微信交易状态非 SUCCESS: %s", tx.TradeState)
	}
	if tx.OutTradeNo == "" || tx.TransactionID == "" || tx.Amount.Total <= 0 {
		return nil, errors.New("微信回调交易字段不完整")
	}
	return &NotifyResult{
		OrderNo: tx.OutTradeNo, TxnID: tx.TransactionID,
		AmountFen: tx.Amount.Total, TradeStatus: tx.TradeState,
	}, nil
}

func wxDecryptResource(apiV3Key, ciphertext, nonce, aad string) ([]byte, error) {
	key := []byte(apiV3Key)
	if len(key) != 32 {
		return nil, errors.New("APIv3 密钥长度必须为 32 字节")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("微信回调密文 base64 非法: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("微信回调 nonce 长度非法")
	}
	return gcm.Open(nil, []byte(nonce), raw, []byte(aad))
}

// channelNotifyURL 把站长配置的通用通知地址规整成「当前通道」的回调地址：
// 已含 /payment/notify/<x> 时替换末尾通道，否则在站点根后补 /api/payment/notify/<channel>。
func channelNotifyURL(baseURL, channel string) string {
	baseURL = strings.TrimSpace(baseURL)
	if idx := strings.LastIndex(baseURL, "/payment/notify/"); idx >= 0 {
		return baseURL[:idx] + "/payment/notify/" + channel
	}
	return strings.TrimRight(baseURL, "/") + "/api/payment/notify/" + channel
}
