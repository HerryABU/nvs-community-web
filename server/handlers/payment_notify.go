package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/payment"

	"github.com/gin-gonic/gin"
)

// ==================== 支付网关回调（公开，无鉴权，验签准入） ====================
// 安全模型：不依赖登录态，以「MD5 验签 + 金额核对 + 幂等到账」三道闸门准入；
// 任何验签失败/状态不符一律按失败应答，网关会按策略重推。

// PaymentNotify POST /api/payment/notify/:channel — 聚合网关异步通知
func PaymentNotify(c *gin.Context) {
	fail := func(reason string) {
		// 记录原始报文便于对账（不记录密钥；form 本身不含密钥明文）
		if reason != "" {
			log.Printf("[payment] 回调被拒绝: %s", reason)
		}
		c.String(http.StatusOK, "fail")
	}
	if !models.IsPaymentEnabled() {
		fail("payment disabled")
		return
	}
	cfg := models.GetPaymentConfig()
	if cfg.Provider != models.PayProviderAggregate {
		fail("provider not aggregate")
		return
	}
	form, raw := parseNotifyForm(c)
	if form == nil {
		fail("bad body")
		return
	}
	provider := payment.GetProvider(cfg)
	result, err := provider.VerifyNotify(form, cfg)
	if err != nil {
		fail("verify: " + err.Error())
		return
	}
	order, err := models.GetRechargeOrderByNo(result.OrderNo)
	if err != nil {
		fail("order not found: " + result.OrderNo)
		return
	}
	// 金额核对：渠道实付必须与订单面额一致（分毫不差），防篡改下游金额
	if order.AmountFen != result.AmountFen {
		fail("amount mismatch")
		return
	}
	if _, _, err := models.MarkOrderPaid(order.OrderNo, result.TxnID, raw); err != nil {
		fail("mark paid: " + err.Error())
		return
	}
	c.String(http.StatusOK, provider.NotifyAck(true))
}

// PaymentReturn GET /api/payment/return/:channel — 支付完成同步跳转。
// 跳转前端结果页（带部署前缀，反向代理下不丢前缀）；实际到账以异步通知为准。
func PaymentReturn(c *gin.Context) {
	orderNo := strings.TrimSpace(c.Query("out_trade_no"))
	target := config.URLPrefix + "/pay/result"
	if orderNo != "" {
		target += "/" + url.QueryEscape(orderNo)
	}
	c.Redirect(http.StatusFound, target)
}

// parseNotifyForm 解析回调体：优先 application/x-www-form-urlencoded，
// 兼容 JSON 报文（部分网关支持 POST JSON）。返回 (表单, 原始报文)。
func parseNotifyForm(c *gin.Context) (url.Values, string) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 64<<10))
	if err != nil || len(body) == 0 {
		return nil, ""
	}
	raw := string(body)
	ct := strings.ToLower(c.ContentType())
	if strings.Contains(ct, "application/json") {
		var m map[string]interface{}
		if err := json.Unmarshal(body, &m); err != nil {
			return nil, raw
		}
		form := url.Values{}
		for k, v := range m {
			switch val := v.(type) {
			case string:
				form.Set(k, val)
			case float64:
				form.Set(k, strconv.FormatFloat(val, 'f', -1, 64))
			case bool:
				if val {
					form.Set(k, "true")
				} else {
					form.Set(k, "false")
				}
			}
		}
		return form, raw
	}
	form, err := url.ParseQuery(raw)
	if err != nil {
		return nil, raw
	}
	return form, raw
}
