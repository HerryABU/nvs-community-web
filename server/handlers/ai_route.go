package handlers

// AI 路由策略的管理接口（gap-audit #10/#11）
//   GET  /api/admin/ai/route   —— 当前策略 + 候选排序预览（让人看见「现在会选谁」）
//   PUT  /api/admin/ai/route   —— 保存
//
// 排序预览很重要：策略是「自动」的，不给预览就没人知道它到底选了什么。

import (
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GetAiRouteConfig GET /api/admin/ai/route
func GetAiRouteConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfg := models.LoadAiRouteConfig()
	utils.Success(c, gin.H{
		"config": cfg,
		"preview": gin.H{
			// 各场景当前会选中谁（按当前策略实时算）
			"cheapest": previewPick(models.RouteCheapest),
			"fastest":  previewPick(models.RouteFastest),
			"manual":   previewPick(models.RouteManual),
			"scenes":   previewScenes(cfg),
		},
		"note": "降级发生时会在输出里明确标注「模型已降级：A → B（原因）」，不会静默换模型",
	})
}

// UpdateAiRouteConfig PUT /api/admin/ai/route
func UpdateAiRouteConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var in models.AiRouteConfig
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SaveAiRouteConfig(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存并即时生效", models.LoadAiRouteConfig())
}

// previewPick 按策略预览会选中的模型（含价格，便于核对「最便宜」到底多便宜）
func previewPick(strategy string) []gin.H {
	list, err := models.ListAiModelsByKind(models.AiKindChat, true)
	if err != nil || len(list) == 0 {
		return []gin.H{}
	}
	out := make([]gin.H, 0, len(list))
	for _, m := range list {
		if !m.Enabled {
			continue
		}
		out = append(out, gin.H{
			"id": m.ID, "name": m.Name, "model_id": m.ModelID,
			"price_in": m.PriceIn, "price_out": m.PriceOut,
			"priority": m.Priority, "provider": m.ProviderHint, "vendor": m.Vendor,
		})
	}
	return out
}

// previewScenes 各场景实际会用的策略
func previewScenes(cfg models.AiRouteConfig) map[string]string {
	out := map[string]string{}
	for _, scene := range []string{"ai_hub", "oc_turn", "oc_publish", "group_bot", "chapter"} {
		out[scene] = models.EffectiveStrategy(cfg, scene)
	}
	return out
}
