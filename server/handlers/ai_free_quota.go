package handlers

import (
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 免费 AI 额度（§5.3：用尽才扣代币） ====================

// AiFreeQuotaStatus GET /api/ai/free-quota — 当前用户今日免费额度状态
func AiFreeQuotaStatus(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	utils.Success(c, models.SummarizeAiFreeQuota(currentUserID(c)))
}

// AdminGetAiFreeQuotaConfig GET /api/admin/ai/free-quota/config — 读取配置
func AdminGetAiFreeQuotaConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"enabled": models.IsAiFreeQuotaEnabled(),
		"config":  models.GetAiFreeQuotaConfig(),
	})
}

// AdminSetAiFreeQuotaConfig PUT /api/admin/ai/free-quota/config — 保存配置（立即生效）
func AdminSetAiFreeQuotaConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Enabled         bool `json:"enabled"`
		CallsPerDay     int  `json:"calls_per_day"`
		MaxTokensPerDay int  `json:"max_tokens_per_day"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SetAiFreeQuotaConfig(models.AiFreeQuotaConfig{
		CallsPerDay: req.CallsPerDay, MaxTokensPerDay: req.MaxTokensPerDay,
	}); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	if err := models.SetAiFreeQuotaEnabled(req.Enabled); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "免费额度配置已保存（立即生效）"})
}

// AdminListAiFreeQuotaOverrides GET /api/admin/ai/free-quota/overrides — 按人覆盖列表
func AdminListAiFreeQuotaOverrides(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	list, err := models.ListUserAiFreeQuotas()
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, list)
}

// AdminManageAiFreeQuotaOverride POST /api/admin/ai/free-quota/overrides — 设置/撤销按人覆盖
// body: { action: "set"|"remove", user_id, daily_calls, daily_tokens, note }
func AdminManageAiFreeQuotaOverride(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Action      string `json:"action"`
		UserID      uint   `json:"user_id"`
		DailyCalls  int    `json:"daily_calls"`
		DailyTokens int    `json:"daily_tokens"`
		Note        string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if req.UserID == 0 {
		utils.BadRequest(c, "用户 ID 无效")
		return
	}
	if _, err := models.GetUserByID(req.UserID); err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}
	switch req.Action {
	case "set":
		if err := models.SetUserAiFreeQuotaOverride(req.UserID, req.DailyCalls, req.DailyTokens, req.Note); err != nil {
			utils.InternalError(c, "保存失败")
			return
		}
		utils.Success(c, gin.H{"message": "已保存该用户的免费额度（立即生效）"})
	case "remove":
		if err := models.DeleteUserAiFreeQuotaOverride(req.UserID); err != nil {
			utils.InternalError(c, "操作失败")
			return
		}
		utils.Success(c, gin.H{"message": "已撤销该用户的免费额度覆盖（恢复平台默认）"})
	default:
		utils.BadRequest(c, "未知的 action（支持 set / remove）")
	}
}
