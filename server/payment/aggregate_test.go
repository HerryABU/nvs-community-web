package payment

import (
	"net/url"
	"testing"

	"nvs-server/models"
)

// testSecret 测试用商户密钥
const testSecret = "test-secret-123456"

func testAggregateConfig() models.PaymentConfig {
	cfg := models.DefaultPaymentConfig()
	cfg.Provider = models.PayProviderAggregate
	cfg.GatewayURL = "https://gateway.example.com/submit"
	cfg.AppID = "10001"
	cfg.Secret = testSecret
	return cfg
}

// buildNotifyForm 构造一份正确签名的异步通知表单
func buildNotifyForm(orderNo, txnID string, fen int64, tradeStatus string) url.Values {
	money := ""
	if fen >= 100 {
		money = itoa(fen/100) + "." + pad2(fen%100)
	} else {
		money = "0." + pad2(fen)
	}
	form := url.Values{}
	form.Set("pid", "10001")
	form.Set("trade_no", txnID)
	form.Set("out_trade_no", orderNo)
	form.Set("type", "alipay")
	form.Set("name", "星币充值")
	form.Set("money", money)
	form.Set("trade_status", tradeStatus)
	form.Set("sign_type", "MD5")
	params := map[string]string{}
	for k, v := range form {
		params[k] = v[0]
	}
	form.Set("sign", md5Sign(params, testSecret))
	return form
}

func TestAggregateVerifyNotifyOK(t *testing.T) {
	p := AggregateProvider{}
	cfg := testAggregateConfig()
	form := buildNotifyForm("R2026010100000001", "TXN-abc", 1000, "TRADE_SUCCESS")
	res, err := p.VerifyNotify(form, cfg)
	if err != nil {
		t.Fatalf("验签应通过: %v", err)
	}
	if res.OrderNo != "R2026010100000001" || res.TxnID != "TXN-abc" || res.AmountFen != 1000 {
		t.Fatalf("解析结果异常: %+v", res)
	}
	if p.NotifyAck(true) != "success" {
		t.Fatal("成功应答应为 success")
	}
}

func TestAggregateVerifyNotifyTampered(t *testing.T) {
	p := AggregateProvider{}
	cfg := testAggregateConfig()
	// 篡改金额：签名对不上
	form := buildNotifyForm("R2026010100000002", "TXN-xyz", 1000, "TRADE_SUCCESS")
	form.Set("money", "0.01")
	if _, err := p.VerifyNotify(form, cfg); err == nil {
		t.Fatal("篡改金额后验签应失败")
	}
	// 非成功交易状态
	form2 := buildNotifyForm("R2026010100000003", "TXN-xyz", 1000, "TRADE_CLOSED")
	if _, err := p.VerifyNotify(form2, cfg); err == nil {
		t.Fatal("非 TRADE_SUCCESS 应拒绝")
	}
	// 缺签名
	form3 := buildNotifyForm("R2026010100000004", "TXN-xyz", 1000, "TRADE_SUCCESS")
	form3.Del("sign")
	if _, err := p.VerifyNotify(form3, cfg); err == nil {
		t.Fatal("缺签名应拒绝")
	}
	// 错误密钥签的名
	form4 := buildNotifyForm("R2026010100000005", "TXN-xyz", 1000, "TRADE_SUCCESS")
	form4.Set("sign", md5Sign(map[string]string{"out_trade_no": "R2026010100000005", "money": "10.00"}, "wrong-secret"))
	if _, err := p.VerifyNotify(form4, cfg); err == nil {
		t.Fatal("错误密钥签名应拒绝")
	}
}

func TestParseMoneyFen(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0.01", 1}, {"1", 100}, {"10.00", 1000}, {"10.5", 1050},
		{"100.99", 10099}, {"0.1", 10}, {"1234.56", 123456},
	}
	for _, c := range cases {
		got, err := ParseMoneyFen(c.in)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("解析 %s 应为 %d，实际 %d", c.in, c.want, got)
		}
	}
	if _, err := ParseMoneyFen(""); err == nil {
		t.Fatal("空金额应报错")
	}
	if _, err := ParseMoneyFen("abc"); err == nil {
		t.Fatal("非数字金额应报错")
	}
}

func TestGetProvider(t *testing.T) {
	cfg := models.DefaultPaymentConfig()
	if GetProvider(cfg).Name() != models.PayProviderMock {
		t.Fatal("默认应返回 mock provider")
	}
	cfg.Provider = models.PayProviderAggregate
	if GetProvider(cfg).Name() != models.PayProviderAggregate {
		t.Fatal("aggregate 配置应返回 aggregate provider")
	}
}

// ---------- 小工具 ----------

func pad2(n int64) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
