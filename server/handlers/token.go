package handlers

import (
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 代币账本 / 卡密充值（需求 §7） ====================
// 代币为平台内消费积分：不可提现、不可转账、不可兑换现金；与作者收益账分离。

// TokenStatus GET /api/token/status — 当前用户代币状态（钱包页用）
func TokenStatus(c *gin.Context) {
	userID := currentUserID(c)
	utils.Success(c, gin.H{
		"enabled": models.IsTokenEnabled(),
		"name":    models.GetTokenName(),
		"balance": models.GetTokenBalance(userID),
	})
}

// TokenLedgerList GET /api/token/ledger — 我的代币流水（分页）
func TokenLedgerList(c *gin.Context) {
	userID := currentUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := models.ListTokenLedger(userID, page, pageSize)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list, "total": total, "page": page, "page_size": pageSize})
}

// RedeemCard POST /api/token/redeem — 兑换卡密
func RedeemCard(c *gin.Context) {
	if !models.IsTokenEnabled() {
		utils.BadRequest(c, "代币系统未开启")
		return
	}
	userID := currentUserID(c)
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请输入卡密")
		return
	}
	nb, err := models.RedeemCardKey(userID, req.Code)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "兑换成功", "balance": nb})
}

// ---------- 管理端 ----------

// AdminGetTokenConfig GET /api/admin/token/config
func AdminGetTokenConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"enabled": models.IsTokenEnabled(),
		"name":    models.GetTokenName(),
	})
}

// AdminSetTokenConfig PUT /api/admin/token/config
func AdminSetTokenConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Enabled *bool  `json:"enabled"`
		Name    string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.Enabled != nil {
		if err := models.SetTokenEnabled(*req.Enabled); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
	}
	if strings.TrimSpace(req.Name) != "" {
		if err := models.SetTokenName(req.Name); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
	}
	utils.Success(c, gin.H{"message": "已保存（立即生效）"})
}

// AdminGenerateCards POST /api/admin/token/cards — 批量生成卡密（明文仅此一次返回）
func AdminGenerateCards(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	adminID := currentUserID(c)
	var req struct {
		Count     int    `json:"count"`
		Value     int    `json:"value"`
		ExpiresIn int    `json:"expires_in"` // 有效天数，0 = 永久
		Note      string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * 24 * time.Hour)
		expiresAt = &t
	}
	codes, err := models.GenerateCardKeys(adminID, req.Count, req.Value, expiresAt, strings.TrimSpace(req.Note))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{
		"message": "已生成，请立即保存以下卡密（仅本次可见，刷新后无法再次查看）",
		"codes":   codes,
		"count":   len(codes),
	})
}

// AdminListCards GET /api/admin/token/cards — 卡密列表（status/batch 筛选）
func AdminListCards(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	list, err := models.ListCardKeys(c.Query("status"), c.Query("batch"), 200)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, list)
}

// AdminVoidCard POST /api/admin/token/cards/:id/void — 作废卡密
func AdminVoidCard(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := models.VoidCardKey(uint(id)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已作废"})
}

// AdminAdjustToken POST /api/admin/token/adjust — 管理员增发/扣减（授信）
func AdminAdjustToken(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	adminID := currentUserID(c)
	var req struct {
		UserID uint   `json:"user_id"`
		Amount int64  `json:"amount"` // 正=增发，负=扣减
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.UserID == 0 || req.Amount == 0 {
		utils.BadRequest(c, "用户 ID 与金额不能为空/0")
		return
	}
	if _, err := models.GetUserByID(req.UserID); err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}
	nb, err := models.AdminAdjustToken(adminID, req.UserID, req.Amount, strings.TrimSpace(req.Remark))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已调整", "balance": nb})
}
