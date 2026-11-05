package ai

// ==================== AI 成本闸门的管理接口（gap-audit #12 #15）====================
//   GET/PUT /api/admin/ai/budget    全站总预算 + 并发排队
//   GET/PUT /api/admin/ai/throttle  三维交叉限流
// 两个接口都顺带返回「现在用了多少 / 现在谁在刷」，避免管理员只能看配置不能看现状。

import (
	"strconv"
	"time"

	"nvs-server/handlers/admin"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	aim "nvs-server/models/ai"
	aiops "nvs-server/models/aiops"
)

// GetAiBudget GET /api/admin/ai/budget
func GetAiBudget(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	cfg := aim.LoadAiBudgetConfig()
	usage := aim.GetAiUsage()
	utils.Success(c, gin.H{
		"config": cfg, "usage": usage,
		"inflight":      aim.AiConcurrencyInUse(),
		"is_peak":       aim.IsPeakHour(cfg),
		"effective_max": aim.EffectiveConcurrency(cfg),
		"note": "HardStop 开启=到线直接拒绝；关闭=只记录告警继续跑（后台能看到已超）。" +
			"并发超出时排队，超时报「忙」而不是把上游打到 429 连坐所有人。",
	})
}

// SaveAiBudget PUT /api/admin/ai/budget
func SaveAiBudget(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	var in aim.AiBudgetConfig
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := aim.SaveAiBudgetConfig(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存并即时生效", aim.LoadAiBudgetConfig())
}

// GetAiThrottle GET /api/admin/ai/throttle
func GetAiThrottle(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"config": aiops.LoadAiThrottleConfig(),
		"top":    aiops.AiThrottleSnapshot(15),
		"note":   "三维：用户×模型 / 用户 / 模型，窗口 1 分钟。单实例内存计数，多实例部署时每实例各限一份（近似限流）。",
	})
}

// SaveAiThrottle PUT /api/admin/ai/throttle
func SaveAiThrottle(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	var in aiops.AiThrottleConfig
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := aiops.SaveAiThrottleConfig(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存并即时生效", aiops.LoadAiThrottleConfig())
}

// GetAiAgentUsage GET /api/admin/ai/agent-usage?days=30
// 按智能体(Agent)维度聚合 AI 调用量 / Token / 消耗。
// 维度取自 AiChatSession(agent_id/agent_name)，用量取自其消息(AiChatMessage: tokens/cost)，
// 因此无需给 AiCallLog 加冗余字段即可完成「按智能体」的统计。
func GetAiAgentUsage(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	d, err := strconv.Atoi(c.DefaultQuery("days", "30"))
	if err != nil || d <= 0 || d > 365 {
		d = 30
	}
	since := time.Now().AddDate(0, 0, -d)

	type agentRow struct {
		AgentID   uint   `json:"agent_id"`
		AgentName string `json:"agent_name"`
		Sessions  int64  `json:"sessions"`
		Calls     int64  `json:"calls"`
		TokensIn  int64  `json:"tokens_in"`
		TokensOut int64  `json:"tokens_out"`
		Cost      int64  `json:"cost"`
	}
	var rows []agentRow
	if err := models.DB.Raw(`
		SELECT s.agent_id AS agent_id, s.agent_name AS agent_name,
		       COUNT(DISTINCT m.session_id) AS sessions,
		       COUNT(m.id) AS calls,
		       COALESCE(SUM(m.tokens_in),0)  AS tokens_in,
		       COALESCE(SUM(m.tokens_out),0) AS tokens_out,
		       COALESCE(SUM(m.cost),0)       AS cost
		FROM ai_chat_messages m
		JOIN ai_chat_sessions s ON m.session_id = s.id
		WHERE m.created_at >= ?
		GROUP BY s.agent_id, s.agent_name
		ORDER BY cost DESC`, since).Scan(&rows).Error; err != nil {
		utils.InternalError(c, "统计失败")
		return
	}

	total := agentRow{}
	for _, r := range rows {
		total.Calls += r.Calls
		total.TokensIn += r.TokensIn
		total.TokensOut += r.TokensOut
		total.Cost += r.Cost
	}
	utils.Success(c, gin.H{"rows": rows, "total": total, "days": d})
}
