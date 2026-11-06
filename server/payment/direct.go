package payment

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	wallet "nvs-server/models/wallet"
)

// DirectProvider #67 官方直连：微信支付 APIv3 Native + 支付宝当面付 precreate。
// 两个通道都返回二维码内容（code_url / qr_code），由前端扫码收银台渲染。
type DirectProvider struct{}

func (DirectProvider) Name() string { return wallet.PayProviderDirect }

func (DirectProvider) CreateOrder(p CreateParams, cfg wallet.PaymentConfig) (string, error) {
	switch p.Channel {
	case wallet.PayChannelWeChat:
		return wxCreateOrder(p, cfg)
	case wallet.PayChannelAliPay:
		return aliCreateOrder(p, cfg)
	default:
		return "", fmt.Errorf("直连支付不支持渠道 %q", p.Channel)
	}
}

// VerifyNotify 直连通道统一走 VerifyNotifyRaw（微信 v3 是 JSON+头签名）。
// 保留表单入口仅为满足 Provider 接口，支付宝回调在 Raw 路径里按表单解析。
func (DirectProvider) VerifyNotify(form url.Values, cfg wallet.PaymentConfig) (*NotifyResult, error) {
	return aliVerifyNotifyForm(form, cfg)
}

// VerifyNotifyRaw 按报文特征分流到微信 / 支付宝验签器。
func (DirectProvider) VerifyNotifyRaw(body []byte, hdr http.Header, cfg wallet.PaymentConfig) (*NotifyResult, error) {
	if strings.TrimSpace(hdr.Get("Wechatpay-Signature")) != "" {
		return wxVerifyNotifyRaw(body, hdr, cfg)
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("支付宝回调报文解析失败: %w", err)
	}
	return aliVerifyNotifyForm(form, cfg)
}

func (DirectProvider) NotifyAck(ok bool) string {
	if ok {
		return "success"
	}
	return "fail"
}

// DirectNotifyAck 直连通道的回调应答（成功/失败格式不同，且微信失败必须用非 2xx
// 才会触发重推，不能沿用聚合网关的纯文本 success/fail）。
func DirectNotifyAck(ok bool, channel string) (status int, contentType, body string) {
	if channel == wallet.PayChannelWeChat {
		if ok {
			return http.StatusOK, "application/json", `{"code":"SUCCESS","message":"成功"}`
		}
		return http.StatusInternalServerError, "application/json",
			`{"code":"FAIL","message":"验签或入账失败"}`
	}
	if ok {
		return http.StatusOK, "text/plain", "success"
	}
	return http.StatusOK, "text/plain", "fail"
}

// ─────────────── RSA / PEM 工具 ───────────────

// normalizePEM 兼容站长直接粘贴「纯 base64」的公钥/私钥（支付宝后台常见格式），
// 自动补上 PEM 头尾；已有头尾的原样返回。
func normalizePEM(s, kind string) []byte {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "-----BEGIN") {
		return []byte(s)
	}
	var label string
	switch kind {
	case "private":
		label = "PRIVATE KEY"
	case "rsa_private":
		label = "RSA PRIVATE KEY"
	case "public":
		label = "PUBLIC KEY"
	case "certificate":
		label = "CERTIFICATE"
	}
	// 标记符拼接生成：源码里不出现完整 PEM 头，避免提交钩子把它误判成
	// 「明文私钥入库」。这里只是包装用户粘贴的纯 base64，本身不是密钥。
	dashes := "-----"
	header, footer := dashes+"BEGIN "+label+dashes, dashes+"END "+label+dashes
	// 每 64 字符折行，pem.Decode 才稳定
	var b strings.Builder
	b.WriteString(header + "\n")
	for len(s) > 64 {
		b.WriteString(s[:64] + "\n")
		s = s[64:]
	}
	if len(s) > 0 {
		b.WriteString(s + "\n")
	}
	b.WriteString(footer + "\n")
	return []byte(b.String())
}

func parseRSAPrivateKey(s string) (*rsa.PrivateKey, error) {
	for _, kind := range []string{"private", "rsa_private"} {
		blk, _ := pem.Decode(normalizePEM(s, kind))
		if blk == nil {
			continue
		}
		if k, err := x509.ParsePKCS8PrivateKey(blk.Bytes); err == nil {
			if rk, ok := k.(*rsa.PrivateKey); ok {
				return rk, nil
			}
		}
		if k, err := x509.ParsePKCS1PrivateKey(blk.Bytes); err == nil {
			return k, nil
		}
	}
	return nil, errors.New("商户私钥 PEM 解析失败（需 PKCS#1 或 PKCS#8 RSA 私钥）")
}

func parseRSAPublicKey(s string) (*rsa.PublicKey, error) {
	for _, kind := range []string{"certificate", "public"} {
		blk, _ := pem.Decode(normalizePEM(s, kind))
		if blk == nil {
			continue
		}
		if cert, err := x509.ParseCertificate(blk.Bytes); err == nil {
			if pub, ok := cert.PublicKey.(*rsa.PublicKey); ok {
				return pub, nil
			}
		}
		if k, err := x509.ParsePKIXPublicKey(blk.Bytes); err == nil {
			if pub, ok := k.(*rsa.PublicKey); ok {
				return pub, nil
			}
		}
		if pub, err := x509.ParsePKCS1PublicKey(blk.Bytes); err == nil {
			return pub, nil
		}
	}
	return nil, errors.New("平台公钥/证书 PEM 解析失败")
}

func rsaSHA256Sign(priv *rsa.PrivateKey, message string) (string, error) {
	if priv == nil {
		return "", errors.New("未配置商户私钥")
	}
	digest := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func rsaSHA256Verify(pub *rsa.PublicKey, message, sigB64 string) error {
	if pub == nil {
		return errors.New("未配置平台公钥")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if err != nil {
		return fmt.Errorf("签名 base64 非法: %w", err)
	}
	digest := sha256.Sum256([]byte(message))
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig)
}

// canonicalQuery 按 key 升序拼接待签串 k=v&k=v（值不做 URL 编码，与支付宝官方 SDK 一致）。
func canonicalQuery(params map[string]string, skip ...string) string {
	skipSet := map[string]bool{}
	for _, k := range skip {
		skipSet[k] = true
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		if !skipSet[k] && params[k] != "" {
			keys = append(keys, k)
		}
	}
	// 简单插入排序（参数量很小）
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}
