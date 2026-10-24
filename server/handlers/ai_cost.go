package handlers

// ==================== AI 成本闸门的管理接口（gap-audit #12 #15）====================
//   GET/PUT /api/admin/ai/budget    全站总预算 + 并发排队
//   GET/PUT /api/admin/ai/throttle  三维交叉限流
// 两个接口都顺带返回「现在用了多少 / 现在谁在刷」，避免管理员只能看配置不能看现状。

import (
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GetAiBudget GET /api/admin/ai/budget
func GetAiBudget(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfg := models.LoadAiBudgetConfig()
	usage := models.GetAiUsage()
	utils.Success(c, gin.H{
		"config": cfg, "usage": usage,
		"inflight":      models.AiConcurrencyInUse(),
		"is_peak":       models.IsPeakHour(cfg),
		"effective_max": models.EffectiveConcurrency(cfg),
		"note": "HardStop 开启=到线直接拒绝；关闭=只记录告警继续跑（后台能看到已超）。" +
			"并发超出时排队，超时报「忙」而不是把上游打到 429 连坐所有人。",
	})
}

// SaveAiBudget PUT /api/admin/ai/budget
func SaveAiBudget(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var in models.AiBudgetConfig
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SaveAiBudgetConfig(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存并即时生效", models.LoadAiBudgetConfig())
}

// GetAiThrottle GET /api/admin/ai/throttle
func GetAiThrottle(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"config": models.LoadAiThrottleConfig(),
		"top":    models.AiThrottleSnapshot(15),
		"note":   "三维：用户×模型 / 用户 / 模型，窗口 1 分钟。单实例内存计数，多实例部署时每实例各限一份（近似限流）。",
	})
}

// SaveAiThrottle PUT /api/admin/ai/throttle
func SaveAiThrottle(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var in models.AiThrottleConfig
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SaveAiThrottleConfig(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存并即时生效", models.LoadAiThrottleConfig())
}
