package aiplatform

import (
	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
	aim "nvs-server/models/ai"
	aihub "nvs-server/models/aihub"
)

// ==================== 免费 AI 额度（§5.3：用尽才扣代币） ====================

// AiFreeQuotaStatus GET /api/ai/free-quota — 当前用户今日免费额度状态
func AiFreeQuotaStatus(c *gin.Context) {
	if !aim.IsAiEnabled() {
		infra.BadRequest(c, "AI 功能未开启")
		return
	}
	infra.Success(c, aihub.SummarizeAiFreeQuota(hctx.CurrentUserID(c)))
}

// AdminGetAiFreeQuotaConfig GET /api/admin/ai/free-quota/config — 读取配置
func AdminGetAiFreeQuotaConfig(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	infra.Success(c, gin.H{
		"enabled": aihub.IsAiFreeQuotaEnabled(),
		"config":  aihub.GetAiFreeQuotaConfig(),
	})
}

// AdminSetAiFreeQuotaConfig PUT /api/admin/ai/free-quota/config — 保存配置（立即生效）
func AdminSetAiFreeQuotaConfig(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var req struct {
		Enabled         bool `json:"enabled"`
		CallsPerDay     int  `json:"calls_per_day"`
		MaxTokensPerDay int  `json:"max_tokens_per_day"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if err := aihub.SetAiFreeQuotaConfig(aihub.AiFreeQuotaConfig{
		CallsPerDay: req.CallsPerDay, MaxTokensPerDay: req.MaxTokensPerDay,
	}); err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	if err := aihub.SetAiFreeQuotaEnabled(req.Enabled); err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	infra.Success(c, gin.H{"message": "免费额度配置已保存（立即生效）"})
}

// AdminListAiFreeQuotaOverrides GET /api/admin/ai/free-quota/overrides — 按人覆盖列表
func AdminListAiFreeQuotaOverrides(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	list, err := aihub.ListUserAiFreeQuotas()
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, list)
}

// AdminManageAiFreeQuotaOverride POST /api/admin/ai/free-quota/overrides — 设置/撤销按人覆盖
// body: { action: "set"|"remove", user_id, daily_calls, daily_tokens, note }
func AdminManageAiFreeQuotaOverride(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
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
		infra.BadRequest(c, "参数错误")
		return
	}
	if req.UserID == 0 {
		infra.BadRequest(c, "用户 ID 无效")
		return
	}
	if _, err := models.GetUserByID(req.UserID); err != nil {
		infra.NotFound(c, "用户不存在")
		return
	}
	switch req.Action {
	case "set":
		if err := aihub.SetUserAiFreeQuotaOverride(req.UserID, req.DailyCalls, req.DailyTokens, req.Note); err != nil {
			infra.InternalError(c, "保存失败")
			return
		}
		infra.Success(c, gin.H{"message": "已保存该用户的免费额度（立即生效）"})
	case "remove":
		if err := aihub.DeleteUserAiFreeQuotaOverride(req.UserID); err != nil {
			infra.InternalError(c, "操作失败")
			return
		}
		infra.Success(c, gin.H{"message": "已撤销该用户的免费额度覆盖（恢复平台默认）"})
	default:
		infra.BadRequest(c, "未知的 action（支持 set / remove）")
	}
}
