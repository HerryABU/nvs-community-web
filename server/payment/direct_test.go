package payment

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	wallet "nvs-server/models/wallet"
)

const testAPIV3Key = "0123456789abcdef0123456789abcdef" // 32 bytes

func testKeyPair(t *testing.T) (*rsa.PrivateKey, string, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成测试密钥失败: %v", err)
	}
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(priv)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
	pubDER, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	return priv, privPEM, pubPEM
}

func testDirectCfg(privPEM, pubPEM string) wallet.PaymentConfig {
	return wallet.PaymentConfig{
		Provider:       wallet.PayProviderDirect,
		NotifyURL:      "https://example.com/go-nvs/api/payment/notify/alipay",
		WxAppID:        "wx8888888888888888",
		WxMchID:        "1900000109",
		WxSerialNo:     "SERIAL123",
		WxAPIV3Key:     testAPIV3Key,
		WxPrivateKey:   privPEM,
		WxPlatformCert: pubPEM,
		AliAppID:       "2021000000000000",
		AliPrivateKey:  privPEM,
		AliPublicKey:   pubPEM,
	}
}

// ─────────────── 微信 APIv3 Native ───────────────

func TestWxCreateOrderSignsRequest(t *testing.T) {
	priv, privPEM, pubPEM := testKeyPair(t)
	cfg := testDirectCfg(privPEM, pubPEM)

	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		buf, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		gotBody = string(buf)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code_url":"weixin://wxpay/bizpayurl?pr=TEST"}`))
	}))
	defer srv.Close()
	old := wxAPIBase
	wxAPIBase = srv.URL
	defer func() { wxAPIBase = old }()

	codeURL, err := wxCreateOrder(CreateParams{
		OrderNo: "R202611060001", AmountFen: 3800, Channel: wallet.PayChannelWeChat,
		Subject: "代币充值 月卡", NotifyURL: cfg.NotifyURL, ClientIP: "127.0.0.1",
	}, cfg)
	if err != nil {
		t.Fatalf("微信下单失败: %v", err)
	}
	if codeURL != "weixin://wxpay/bizpayurl?pr=TEST" {
		t.Fatalf("code_url 不符: %s", codeURL)
	}
	if !strings.HasPrefix(gotAuth, "WECHATPAY2-SHA256-RSA2048 ") ||
		!strings.Contains(gotAuth, `mchid="1900000109"`) ||
		!strings.Contains(gotAuth, `serial_no="SERIAL123"`) {
		t.Fatalf("Authorization 头格式不符: %s", gotAuth)
	}
	// 用商户公钥反验签名串（从 header 抽出各字段）
	ts := between(gotAuth, `timestamp="`, `"`)
	nonce := between(gotAuth, `nonce_str="`, `"`)
	sig := between(gotAuth, `signature="`, `"`)
	message := "POST\n" + wxNativePath + "\n" + ts + "\n" + nonce + "\n" + gotBody + "\n"
	if err := rsaSHA256Verify(&priv.PublicKey, message, sig); err != nil {
		t.Fatalf("下单签名校验失败: %v", err)
	}
	var req wxNativeRequest
	if err := json.Unmarshal([]byte(gotBody), &req); err != nil {
		t.Fatalf("下单 body 解析失败: %v", err)
	}
	if req.Amount.Total != 3800 || req.OutTradeNo != "R202611060001" ||
		!strings.HasSuffix(req.NotifyURL, "/payment/notify/wechat") {
		t.Fatalf("下单参数不符: %+v", req)
	}
}

func between(s, left, right string) string {
	i := strings.Index(s, left)
	if i < 0 {
		return ""
	}
	s = s[i+len(left):]
	j := strings.Index(s, right)
	if j < 0 {
		return ""
	}
	return s[:j]
}

func makeWxNotify(t *testing.T, priv *rsa.PrivateKey, orderNo string, fen int64, ts string, nonce string, key string) ([]byte, http.Header, string) {
	t.Helper()
	tx := map[string]interface{}{
		"out_trade_no": orderNo, "transaction_id": "TXN-WX-001", "trade_state": "SUCCESS",
		"amount": map[string]interface{}{"total": fen, "payer_total": fen, "currency": "CNY"},
	}
	plain, _ := json.Marshal(tx)
	block, _ := aes.NewCipher([]byte(key))
	gcm, _ := cipher.NewGCM(block)
	ct := gcm.Seal(nil, []byte(nonce), plain, []byte(""))
	envelope := map[string]interface{}{
		"event_type": "TRANSACTION.SUCCESS",
		"resource": map[string]interface{}{
			"algorithm":  "AEAD_AES_256_GCM",
			"ciphertext": base64.StdEncoding.EncodeToString(ct),
			"nonce":      nonce, "associated_data": "", "original_type": "transaction",
		},
	}
	body, _ := json.Marshal(envelope)
	message := ts + "\n" + nonce + "\n" + string(body) + "\n"
	sig, err := rsaSHA256Sign(priv, message)
	if err != nil {
		t.Fatal(err)
	}
	hdr := http.Header{}
	hdr.Set("Wechatpay-Signature", sig)
	hdr.Set("Wechatpay-Timestamp", ts)
	hdr.Set("Wechatpay-Nonce", nonce)
	hdr.Set("Wechatpay-Serial", "PLATFORM-SERIAL")
	return body, hdr, sig
}

func TestWxVerifyNotify(t *testing.T) {
	priv, privPEM, pubPEM := testKeyPair(t)
	cfg := testDirectCfg(privPEM, pubPEM)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "abcdefghijkl" // 12 bytes

	body, hdr, sig := makeWxNotify(t, priv, "R202611060001", 3800, ts, nonce, testAPIV3Key)
	res, err := (DirectProvider{}).VerifyNotifyRaw(body, hdr, cfg)
	if err != nil {
		t.Fatalf("微信回调验签应通过: %v", err)
	}
	if res.OrderNo != "R202611060001" || res.TxnID != "TXN-WX-001" || res.AmountFen != 3800 {
		t.Fatalf("解析结果不符: %+v", res)
	}

	// 篡改签名
	bad := hdr.Clone()
	bad.Set("Wechatpay-Signature", sig+"x")
	if _, err := (DirectProvider{}).VerifyNotifyRaw(body, bad, cfg); err == nil {
		t.Fatal("签名被篡改应失败")
	}
	// 时间戳超窗（防重放）
	old := strconv.FormatInt(time.Now().Unix()-600, 10)
	oldBody, oldHdr, _ := makeWxNotify(t, priv, "R202611060001", 3800, old, nonce, testAPIV3Key)
	if _, err := (DirectProvider{}).VerifyNotifyRaw(oldBody, oldHdr, cfg); err == nil {
		t.Fatal("过期时间戳应失败")
	}
	// 错误 APIv3Key
	wrong := cfg
	wrong.WxAPIV3Key = "ffffffffffffffffffffffffffffffff"
	if _, err := (DirectProvider{}).VerifyNotifyRaw(body, hdr, wrong); err == nil {
		t.Fatal("错误 APIv3Key 应解密失败")
	}
}

// ─────────────── 支付宝当面付 ───────────────

func TestAliCreateOrderSignsRequest(t *testing.T) {
	_, privPEM, pubPEM := testKeyPair(t)
	cfg := testDirectCfg(privPEM, pubPEM)

	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"alipay_trade_precreate_response":{"code":"10000","msg":"Success","out_trade_no":"R202611060001","qr_code":"https://qr.alipay.com/bax00001"},"sign":"x"}`))
	}))
	defer srv.Close()
	old := aliGatewayBase
	aliGatewayBase = srv.URL
	defer func() { aliGatewayBase = old }()

	qr, err := aliCreateOrder(CreateParams{
		OrderNo: "R202611060001", AmountFen: 3800, Channel: wallet.PayChannelAliPay,
		Subject: "代币充值", NotifyURL: cfg.NotifyURL,
	}, cfg)
	if err != nil {
		t.Fatalf("支付宝下单失败: %v", err)
	}
	if !strings.HasPrefix(qr, "https://qr.alipay.com/") {
		t.Fatalf("qr_code 不符: %s", qr)
	}
	if gotForm.Get("method") != "alipay.trade.precreate" || gotForm.Get("sign_type") != "RSA2" {
		t.Fatalf("公共参数不符: %v", gotForm)
	}
	params := map[string]string{}
	for k, v := range gotForm {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	// 用配置里的公钥验 sign（签名串为不含 sign 的规范化参数）
	pub, err := parseRSAPublicKey(cfg.AliPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	// 下单签名保留 sign_type（仅排除 sign），与回调验签规则不同
	if err := rsaSHA256Verify(pub, canonicalQuery(params, "sign"), gotForm.Get("sign")); err != nil {
		t.Fatalf("支付宝下单签名校验失败: %v", err)
	}
	var biz map[string]interface{}
	_ = json.Unmarshal([]byte(gotForm.Get("biz_content")), &biz)
	if biz["total_amount"] != "38.00" || biz["out_trade_no"] != "R202611060001" {
		t.Fatalf("biz_content 金额/订单号不符: %v", biz)
	}
}

func TestAliVerifyNotify(t *testing.T) {
	priv, privPEM, pubPEM := testKeyPair(t)
	cfg := testDirectCfg(privPEM, pubPEM)
	// 支付宝官方规则：回调待签串排除 sign 与 sign_type
	params := map[string]string{
		"out_trade_no": "R202611060001", "trade_no": "2026110622001",
		"total_amount": "38.00", "trade_status": "TRADE_SUCCESS",
		"app_id": cfg.AliAppID, "gmt_create": "2026-11-06 17:00:00",
	}
	sig, err := rsaSHA256Sign(priv, canonicalQuery(params))
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	form.Set("sign_type", "RSA2")
	form.Set("sign", sig)

	res, err := aliVerifyNotifyForm(form, cfg)
	if err != nil {
		t.Fatalf("支付宝验签应通过: %v", err)
	}
	if res.OrderNo != "R202611060001" || res.AmountFen != 3800 || res.TxnID != "2026110622001" {
		t.Fatalf("解析结果不符: %+v", res)
	}
	// 篡改金额 → 验签失败
	bad := url.Values{}
	for k, v := range form {
		bad[k] = v
	}
	bad.Set("total_amount", "0.01")
	if _, err := aliVerifyNotifyForm(bad, cfg); err == nil {
		t.Fatal("金额被篡改应验签失败")
	}
	// 缺签名
	noSign := url.Values{}
	for k, v := range form {
		noSign[k] = v
	}
	noSign.Del("sign")
	if _, err := aliVerifyNotifyForm(noSign, cfg); err == nil {
		t.Fatal("缺签名应失败")
	}
}

// ─────────────── 应答与地址 ───────────────

func TestDirectNotifyAck(t *testing.T) {
	st, ct, body := DirectNotifyAck(true, wallet.PayChannelWeChat)
	if st != http.StatusOK || ct != "application/json" || !strings.Contains(body, "SUCCESS") {
		t.Fatalf("微信成功应答不符: %d %s %s", st, ct, body)
	}
	st, _, body = DirectNotifyAck(false, wallet.PayChannelWeChat)
	if st < 400 || !strings.Contains(body, "FAIL") {
		t.Fatalf("微信失败应答必须非 2xx 且 code=FAIL: %d %s", st, body)
	}
	st, _, body = DirectNotifyAck(true, wallet.PayChannelAliPay)
	if st != http.StatusOK || body != "success" {
		t.Fatalf("支付宝成功应答不符: %d %s", st, body)
	}
}

func TestChannelNotifyURLAndAmount(t *testing.T) {
	if got := channelNotifyURL("https://x/go-nvs/api/payment/notify/alipay", "wechat"); !strings.HasSuffix(got, "/notify/wechat") {
		t.Fatalf("通道替换失败: %s", got)
	}
	if got := channelNotifyURL("https://x", "alipay"); !strings.HasSuffix(got, "/api/payment/notify/alipay") {
		t.Fatalf("通道补全失败: %s", got)
	}
	for _, fen := range []int64{1, 99, 100, 3800, 1000000} {
		got, err := yuanToFen(fenToYuan(fen))
		if err != nil || got != fen {
			t.Fatalf("金额往返失败 fen=%d got=%d err=%v", fen, got, err)
		}
	}
}
