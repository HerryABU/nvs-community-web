package handlers

import (
	"strconv"
	"strings"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 支付通道管理端（§7.4） ====================
// 密钥纪律：secret 只写不读——GET 仅返回 secret_set 布尔；
// PUT 用指针语义：nil=保持原值，""=清空，非空=更新。

// AdminGetPaymentConfig GET /api/admin/payment/config — 读取配置（密钥掩码）
func AdminGetPaymentConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfg := models.GetPaymentConfig()
	utils.Success(c, gin.H{
		"enabled":       models.IsPaymentEnabled(),
		"provider":      cfg.Provider,
		"gateway_url":   cfg.GatewayURL,
		"app_id":        cfg.AppID,
		"secret_set":    cfg.Secret != "",
		"notify_url":    cfg.NotifyURL,
		"return_url":    cfg.ReturnURL,
		"token_per_fen": cfg.TokenPerFen,
		"min_fen":       cfg.MinFen,
		"max_fen":       cfg.MaxFen,
		"order_ttl_min": cfg.OrderTTLMin,
		"tiers":         models.GetPaymentTiers(),
		"stats":         models.PaymentStats(),
	})
}

// AdminSetPaymentConfig PUT /api/admin/payment/config — 保存配置（立即生效）
func AdminSetPaymentConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Enabled     *bool   `json:"enabled"`
		Provider    string  `json:"provider"`
		GatewayURL  string  `json:"gateway_url"`
		AppID       string  `json:"app_id"`
		Secret      *string `json:"secret"` // nil=保持，""=清空
		NotifyURL   string  `json:"notify_url"`
		ReturnURL   string  `json:"return_url"`
		TokenPerFen int64   `json:"token_per_fen"`
		MinFen      int64   `json:"min_fen"`
		MaxFen      int64   `json:"max_fen"`
		OrderTTLMin int     `json:"order_ttl_min"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	cfg := models.GetPaymentConfig() // 以现值起步，未传字段保持
	cfg.Provider = req.Provider
	cfg.GatewayURL = req.GatewayURL
	cfg.AppID = req.AppID
	if req.Secret != nil {
		cfg.Secret = *req.Secret
	}
	cfg.NotifyURL = req.NotifyURL
	cfg.ReturnURL = req.ReturnURL
	if req.TokenPerFen > 0 {
		cfg.TokenPerFen = req.TokenPerFen
	}
	if req.MinFen > 0 {
		cfg.MinFen = req.MinFen
	}
	if req.MaxFen > 0 {
		cfg.MaxFen = req.MaxFen
	}
	if req.OrderTTLMin > 0 {
		cfg.OrderTTLMin = req.OrderTTLMin
	}
	if err := models.SetPaymentConfig(cfg); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	if req.Enabled != nil {
		if err := models.SetPaymentEnabled(*req.Enabled); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
	}
	utils.Success(c, gin.H{"message": "支付配置已保存（立即生效）"})
}

// AdminSetPaymentTiers PUT /api/admin/payment/tiers — 保存充值档位
func AdminSetPaymentTiers(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Tiers []models.PaymentTier `json:"tiers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SetPaymentTiers(req.Tiers); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "档位已保存（立即生效，仅影响新订单）"})
}

// AdminListPaymentOrders GET /api/admin/payment/orders — 订单列表（status/user_id 筛选）
func AdminListPaymentOrders(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	userID64, _ := strconv.ParseUint(strings.TrimSpace(c.Query("user_id")), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := models.AdminListRechargeOrders(c.Query("status"), uint(userID64), page, pageSize)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// AdminMarkOrderPaid POST /api/admin/payment/orders/mark-paid — 手动补单（留痕）
func AdminMarkOrderPaid(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		OrderNo       string `json:"order_no"`
		TransactionID string `json:"transaction_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	req.OrderNo = strings.TrimSpace(req.OrderNo)
	if req.OrderNo == "" {
		utils.BadRequest(c, "订单号不能为空")
		return
	}
	if req.TransactionID == "" {
		req.TransactionID = "ADMIN-" + req.OrderNo
	}
	order, credited, err := models.MarkOrderPaid(req.OrderNo, strings.TrimSpace(req.TransactionID), "manual mark-paid by admin")
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	msg := "补单完成，代币已到账"
	if !credited {
		msg = "该订单此前已到账（幂等，未重复入账）"
	}
	utils.Success(c, gin.H{
		"message":      msg,
		"credited":     credited,
		"token_amount": order.TokenAmount,
		"balance":      models.GetTokenBalance(order.UserID),
	})
}
