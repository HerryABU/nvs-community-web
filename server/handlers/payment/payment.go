package payment

import (
	"nvs-server/utils/infra"
	"strconv"
	"strings"
	"time"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/payment"

	"github.com/gin-gonic/gin"
	aihub "nvs-server/models/aihub"
	wallet "nvs-server/models/wallet"
)

// ==================== 在线支付充值（§7.4 微信/支付宝聚合通道） ====================

// PaymentStatus GET /api/payment/status — 充值页初始化数据（钱包页用）
func PaymentStatus(c *gin.Context) {
	cfg := wallet.GetPaymentConfig()
	type chanItem struct {
		Value string `json:"value"`
		Label string `json:"label"`
	}
	channels := []chanItem{}
	if cfg.Provider == wallet.PayProviderAggregate {
		channels = append(channels,
			chanItem{Value: wallet.PayChannelWeChat, Label: wallet.PayChannelLabels[wallet.PayChannelWeChat]},
			chanItem{Value: wallet.PayChannelAliPay, Label: wallet.PayChannelLabels[wallet.PayChannelAliPay]},
		)
	} else {
		channels = append(channels,
			chanItem{Value: wallet.PayChannelMock, Label: wallet.PayChannelLabels[wallet.PayChannelMock]},
		)
	}
	infra.Success(c, gin.H{
		"enabled":       wallet.IsPaymentEnabled(),
		"token_enabled": wallet.IsTokenEnabled(),
		"token_name":    wallet.GetTokenName(),
		"provider":      cfg.Provider,
		"channels":      channels,
		"tiers":         wallet.GetPaymentTiers(),
		"token_per_fen": cfg.TokenPerFen,
		"order_ttl_min": cfg.OrderTTLMin,
	})
}

// PaymentCreateOrder POST /api/payment/order — 选档位下单，返回收银台地址
func PaymentCreateOrder(c *gin.Context) {
	if !wallet.IsPaymentEnabled() {
		infra.BadRequest(c, "在线充值未开启")
		return
	}
	userID := hctx.CurrentUserID(c)
	var req struct {
		Channel string `json:"channel"`
		Fen     int64  `json:"fen"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	order, err := wallet.CreateRechargeOrder(userID, strings.TrimSpace(req.Channel), req.Fen)
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	cfg := wallet.GetPaymentConfig()
	tier, _ := wallet.ResolvePaymentTier(order.AmountFen)
	subject := wallet.GetTokenName() + "充值 " + tier.Label
	if subject == "" {
		subject = wallet.GetTokenName() + "充值"
	}
	params := payment.CreateParams{
		OrderNo: order.OrderNo, AmountFen: order.AmountFen, Channel: order.Channel,
		Subject: subject, NotifyURL: cfg.NotifyURL, ReturnURL: cfg.ReturnURL,
		ClientIP: c.ClientIP(),
	}
	payURL, perr := payment.GetProvider(cfg).CreateOrder(params, cfg)
	if perr != nil {
		// 下单失败：订单留在 pending 由超时关单回收，前端可重试
		infra.BadRequest(c, "创建支付单失败："+perr.Error())
		return
	}
	if err := wallet.UpdateOrderPayURL(order.OrderNo, payURL); err != nil {
		infra.InternalError(c, "订单保存失败")
		return
	}
	infra.Success(c, gin.H{
		"order_no":     order.OrderNo,
		"pay_url":      payURL,
		"channel":      order.Channel,
		"provider":     order.Provider,
		"amount_fen":   order.AmountFen,
		"token_amount": order.TokenAmount,
		"expires_at":   order.ExpiresAt,
	})
}

// PaymentOrderStatus GET /api/payment/order/:no — 轮询订单状态（仅本人订单）
func PaymentOrderStatus(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	orderNo := strings.TrimSpace(c.Param("no"))
	order, err := wallet.GetRechargeOrderByNo(orderNo)
	if err != nil {
		infra.NotFound(c, "订单不存在")
		return
	}
	if order.UserID != userID {
		infra.NotFound(c, "订单不存在")
		return
	}
	// 惰性关单：待支付且已过期 → 就地关闭，前端轮询立刻看到 closed
	if order.Status == wallet.PayStatusPending && time.Now().After(order.ExpiresAt) {
		_ = models.DB.Model(&wallet.RechargeOrder{}).
			Where("order_no = ? AND status = ?", order.OrderNo, wallet.PayStatusPending).
			Updates(map[string]interface{}{"status": wallet.PayStatusClosed, "updated_at": time.Now()}).Error
		order.Status = wallet.PayStatusClosed
	}
	infra.Success(c, gin.H{
		"order_no":     order.OrderNo,
		"status":       order.Status,
		"channel":      order.Channel,
		"provider":     order.Provider,
		"amount_fen":   order.AmountFen,
		"token_amount": order.TokenAmount,
		"expires_at":   order.ExpiresAt,
		"paid_at":      order.PaidAt,
		"balance":      wallet.GetTokenBalance(userID),
	})
}

// PaymentMyOrders GET /api/payment/orders — 我的充值订单（分页）
func PaymentMyOrders(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := wallet.ListUserRechargeOrders(userID, page, pageSize)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// PaymentMockConfirm POST /api/payment/mock/confirm — 模拟支付确认（仅 mock provider）。
// 与真实回调路径物理隔离：要求登录、本人订单、provider==mock。
func PaymentMockConfirm(c *gin.Context) {
	if !wallet.IsPaymentEnabled() {
		infra.BadRequest(c, "在线充值未开启")
		return
	}
	cfg := wallet.GetPaymentConfig()
	if cfg.Provider != wallet.PayProviderMock {
		infra.BadRequest(c, "当前通道不是模拟支付")
		return
	}
	userID := hctx.CurrentUserID(c)
	var req struct {
		OrderNo string `json:"order_no"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	order, err := wallet.GetRechargeOrderByNo(strings.TrimSpace(req.OrderNo))
	if err != nil {
		infra.NotFound(c, "订单不存在")
		return
	}
	if order.UserID != userID {
		infra.NotFound(c, "订单不存在")
		return
	}
	paid, credited, err := wallet.MarkOrderPaid(order.OrderNo, "MOCK-"+order.OrderNo, "mock-confirm by user "+strconv.FormatUint(uint64(userID), 10))
	if err != nil {
		infra.BadRequest(c, err.Error())
		return
	}
	infra.Success(c, gin.H{
		"credited":     credited,
		"status":       paid.Status,
		"token_amount": paid.TokenAmount,
		"balance":      wallet.GetTokenBalance(userID),
	})
}

// ==================== 会员套餐 ====================

// MembershipInfo GET /api/membership — 会员页初始化数据
//
// 一次请求返回「我的会员状态 + 在售套餐 + 支付可用性」：
// 会员页是首屏，分成三次请求会出现"卡片先出来、按钮后变灰"的跳变。
func MembershipInfo(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	cfg := wallet.GetPaymentConfig()
	// 权益对比表要显示真实数字，不能写"平台配置"这种空话
	quota := aihub.GetAiFreeQuotaConfig()
	infra.Success(c, gin.H{
		"status":          wallet.GetMembershipStatus(uid),
		"tiers":           wallet.GetPaymentTiers(),
		"token_per_fen":   cfg.TokenPerFen,
		"payment_enabled": wallet.IsPaymentEnabled(),
		"token_enabled":   wallet.IsTokenEnabled(),
		"free_quota": gin.H{
			"enabled":       aihub.IsAiFreeQuotaEnabled(),
			"calls_per_day": quota.CallsPerDay,
			"max_tokens":    quota.MaxTokensPerDay,
			// 会员倍数：与 models.memberFreeQuotaMultiplier 保持一致
			"member_multiplier": 2,
		},
	})
}
