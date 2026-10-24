package handlers

import (
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/payment"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 在线支付充值（§7.4 微信/支付宝聚合通道） ====================

// PaymentStatus GET /api/payment/status — 充值页初始化数据（钱包页用）
func PaymentStatus(c *gin.Context) {
	cfg := models.GetPaymentConfig()
	type chanItem struct {
		Value string `json:"value"`
		Label string `json:"label"`
	}
	channels := []chanItem{}
	if cfg.Provider == models.PayProviderAggregate {
		channels = append(channels,
			chanItem{Value: models.PayChannelWeChat, Label: models.PayChannelLabels[models.PayChannelWeChat]},
			chanItem{Value: models.PayChannelAliPay, Label: models.PayChannelLabels[models.PayChannelAliPay]},
		)
	} else {
		channels = append(channels,
			chanItem{Value: models.PayChannelMock, Label: models.PayChannelLabels[models.PayChannelMock]},
		)
	}
	utils.Success(c, gin.H{
		"enabled":       models.IsPaymentEnabled(),
		"token_enabled": models.IsTokenEnabled(),
		"token_name":    models.GetTokenName(),
		"provider":      cfg.Provider,
		"channels":      channels,
		"tiers":         models.GetPaymentTiers(),
		"token_per_fen": cfg.TokenPerFen,
		"order_ttl_min": cfg.OrderTTLMin,
	})
}

// PaymentCreateOrder POST /api/payment/order — 选档位下单，返回收银台地址
func PaymentCreateOrder(c *gin.Context) {
	if !models.IsPaymentEnabled() {
		utils.BadRequest(c, "在线充值未开启")
		return
	}
	userID := currentUserID(c)
	var req struct {
		Channel string `json:"channel"`
		Fen     int64  `json:"fen"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	order, err := models.CreateRechargeOrder(userID, strings.TrimSpace(req.Channel), req.Fen)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	cfg := models.GetPaymentConfig()
	tier, _ := models.ResolvePaymentTier(order.AmountFen)
	subject := models.GetTokenName() + "充值 " + tier.Label
	if subject == "" {
		subject = models.GetTokenName() + "充值"
	}
	params := payment.CreateParams{
		OrderNo: order.OrderNo, AmountFen: order.AmountFen, Channel: order.Channel,
		Subject: subject, NotifyURL: cfg.NotifyURL, ReturnURL: cfg.ReturnURL,
		ClientIP: c.ClientIP(),
	}
	payURL, perr := payment.GetProvider(cfg).CreateOrder(params, cfg)
	if perr != nil {
		// 下单失败：订单留在 pending 由超时关单回收，前端可重试
		utils.BadRequest(c, "创建支付单失败："+perr.Error())
		return
	}
	if err := models.UpdateOrderPayURL(order.OrderNo, payURL); err != nil {
		utils.InternalError(c, "订单保存失败")
		return
	}
	utils.Success(c, gin.H{
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
	userID := currentUserID(c)
	orderNo := strings.TrimSpace(c.Param("no"))
	order, err := models.GetRechargeOrderByNo(orderNo)
	if err != nil {
		utils.NotFound(c, "订单不存在")
		return
	}
	if order.UserID != userID {
		utils.NotFound(c, "订单不存在")
		return
	}
	// 惰性关单：待支付且已过期 → 就地关闭，前端轮询立刻看到 closed
	if order.Status == models.PayStatusPending && time.Now().After(order.ExpiresAt) {
		_ = models.DB.Model(&models.RechargeOrder{}).
			Where("order_no = ? AND status = ?", order.OrderNo, models.PayStatusPending).
			Updates(map[string]interface{}{"status": models.PayStatusClosed, "updated_at": time.Now()}).Error
		order.Status = models.PayStatusClosed
	}
	utils.Success(c, gin.H{
		"order_no":     order.OrderNo,
		"status":       order.Status,
		"channel":      order.Channel,
		"provider":     order.Provider,
		"amount_fen":   order.AmountFen,
		"token_amount": order.TokenAmount,
		"expires_at":   order.ExpiresAt,
		"paid_at":      order.PaidAt,
		"balance":      models.GetTokenBalance(userID),
	})
}

// PaymentMyOrders GET /api/payment/orders — 我的充值订单（分页）
func PaymentMyOrders(c *gin.Context) {
	userID := currentUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := models.ListUserRechargeOrders(userID, page, pageSize)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// PaymentMockConfirm POST /api/payment/mock/confirm — 模拟支付确认（仅 mock provider）。
// 与真实回调路径物理隔离：要求登录、本人订单、provider==mock。
func PaymentMockConfirm(c *gin.Context) {
	if !models.IsPaymentEnabled() {
		utils.BadRequest(c, "在线充值未开启")
		return
	}
	cfg := models.GetPaymentConfig()
	if cfg.Provider != models.PayProviderMock {
		utils.BadRequest(c, "当前通道不是模拟支付")
		return
	}
	userID := currentUserID(c)
	var req struct {
		OrderNo string `json:"order_no"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	order, err := models.GetRechargeOrderByNo(strings.TrimSpace(req.OrderNo))
	if err != nil {
		utils.NotFound(c, "订单不存在")
		return
	}
	if order.UserID != userID {
		utils.NotFound(c, "订单不存在")
		return
	}
	paid, credited, err := models.MarkOrderPaid(order.OrderNo, "MOCK-"+order.OrderNo, "mock-confirm by user "+strconv.FormatUint(uint64(userID), 10))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{
		"credited":     credited,
		"status":       paid.Status,
		"token_amount": paid.TokenAmount,
		"balance":      models.GetTokenBalance(userID),
	})
}

// ==================== 会员套餐 ====================

// MembershipInfo GET /api/membership — 会员页初始化数据
//
// 一次请求返回「我的会员状态 + 在售套餐 + 支付可用性」：
// 会员页是首屏，分成三次请求会出现"卡片先出来、按钮后变灰"的跳变。
func MembershipInfo(c *gin.Context) {
	uid := currentUserID(c)
	cfg := models.GetPaymentConfig()
	utils.Success(c, gin.H{
		"status":          models.GetMembershipStatus(uid),
		"tiers":           models.GetPaymentTiers(),
		"token_per_fen":   cfg.TokenPerFen,
		"payment_enabled": models.IsPaymentEnabled(),
		"token_enabled":   models.IsTokenEnabled(),
	})
}
