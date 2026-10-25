package handlers

import (
	"strconv"
	"strings"
	"time"

	"nvs-server/aisafety"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== AI 内容安全闸（提示词 + 用户特征 + 蓝队模型 B） ====================

// countUserTurns 历史里用户发过几条消息（本轮轮次 = 这个数 + 1）。
func countUserTurns(history []models.AiChatMessage) int {
	n := 0
	for _, h := range history {
		if h.Role == "user" {
			n++
		}
	}
	return n
}

// userMessagesOnly 只取用户自己说的话。
//
// 不能把助手回复混进去：模型的回答里必然出现危险术语，
// 用户下一句「谢谢」也会被算成「承接追问」。
func userMessagesOnly(history []models.AiChatMessage) []string {
	out := make([]string, 0, len(history))
	for _, h := range history {
		if h.Role == "user" {
			out = append(out, h.Content)
		}
	}
	return out
}

// toJudgeTurns 转成蓝队检查用的对话轮（含助手回复，判定要看上下文）。
func toJudgeTurns(history []models.AiChatMessage) []aisafety.JudgeTurn {
	out := make([]aisafety.JudgeTurn, 0, len(history))
	for _, h := range history {
		role := h.Role
		if role != "assistant" {
			role = "user"
		}
		out = append(out, aisafety.JudgeTurn{Role: role, Content: h.Content})
	}
	return out
}

// serveSafetyRefusal 输入侧拦截后的响应：落库（用户要看到自己问了什么）+ 拒答 + 不扣费。
func serveSafetyRefusal(c *gin.Context, session *models.AiChatSession, storeMsg string, dec *aisafety.InputDecision) {
	models.AppendAiChatMessage(session.ID, "user", storeMsg, 0, 0, 0, 0)
	models.TouchAiChatSession(session.ID, storeMsg)
	text := dec.Refusal
	if strings.TrimSpace(text) == "" {
		text = "抱歉，这个请求我不能提供。"
	}
	am, _ := models.AppendAiChatMessage(session.ID, "assistant", text, 0, 0, 0, 0)
	models.TouchAiChatSession(session.ID, "")

	setupSSE(c)
	sseEvent(c, gin.H{
		"blocked": true, "replace": text, "reason": text,
		"action": dec.Action, "score": dec.Score, "categories": dec.Categories,
	})
	sseEvent(c, gin.H{
		"done": true, "session_id": session.ID, "message_id": am.ID,
		"blocked": true, "cost": 0, "tokens_in": 0, "tokens_out": 0, "free": false,
	})
}

// ---------- 配置 ----------

// GetAiSafetyConfig GET /api/admin/ai/safety/config
func GetAiSafetyConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfg, err := aisafety.GetConfig()
	if err != nil {
		utils.InternalError(c, "读取配置失败")
		return
	}
	utils.Success(c, gin.H{"config": cfg})
}

// SaveAiSafetyConfig PUT /api/admin/ai/safety/config
func SaveAiSafetyConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req aisafety.SafetyConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	// 防呆：策略数值为 0/负数在业务上无意义，负值一律纠正
	if req.InputBlockScore <= 0 {
		req.InputBlockScore = 65
	}
	if req.OutputBlockScore <= 0 {
		req.OutputBlockScore = 55
	}
	if req.ReviewScore < 0 {
		req.ReviewScore = 0
	}
	if req.WatchBlockScore <= 0 {
		req.WatchBlockScore = 30
	}
	if req.PrebufferChars < 40 {
		req.PrebufferChars = 40
	}
	if req.FlushChars < 20 {
		req.FlushChars = 20
	}
	if req.WatchHours <= 0 {
		req.WatchHours = 24
	}
	if req.CooldownMinutes <= 0 {
		req.CooldownMinutes = 60
	}
	if req.ExcerptChars <= 0 {
		req.ExcerptChars = 240
	}
	if req.JudgeEveryTurns <= 0 {
		req.JudgeEveryTurns = 3
	}
	if req.JudgeTimeoutSec <= 0 {
		req.JudgeTimeoutSec = 10
	}
	if req.JudgeMaxChars <= 0 {
		req.JudgeMaxChars = 3000
	}
	if req.JudgeMaxTurns <= 0 {
		req.JudgeMaxTurns = 6
	}
	if err := aisafety.SaveConfig(&req); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	cfg, _ := aisafety.GetConfig()
	utils.Success(c, gin.H{"message": "已保存", "config": cfg})
}

// ResetAiSafetyConfig POST /api/admin/ai/safety/config/reset
func ResetAiSafetyConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfg, err := aisafety.ResetConfig()
	if err != nil {
		utils.InternalError(c, "恢复失败")
		return
	}
	utils.Success(c, gin.H{"message": "已恢复默认", "config": cfg})
}

// ---------- 事件 / 画像 / 判定日志 ----------

// ListAiSafetyEvents GET /api/admin/ai/safety/events
func ListAiSafetyEvents(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	if size < 1 || size > 200 {
		size = 30
	}
	q := models.DB.Model(&aisafety.SafetyEvent{})
	if v := c.Query("direction"); v != "" {
		q = q.Where("direction = ?", v)
	}
	if v := c.Query("action"); v != "" {
		q = q.Where("action = ?", v)
	}
	if v := c.Query("user_id"); v != "" {
		q = q.Where("user_id = ?", v)
	}
	var total int64
	q.Count(&total)
	var list []aisafety.SafetyEvent
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list)
	utils.Success(c, gin.H{"items": list, "total": total, "page": page})
}

// ListAiSafetyProfiles GET /api/admin/ai/safety/profiles
func ListAiSafetyProfiles(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	if size < 1 || size > 200 {
		size = 30
	}
	q := models.DB.Model(&aisafety.SafetyProfile{})
	if c.Query("only_active") == "1" {
		now := time.Now()
		q = q.Where("(cooldown_until IS NOT NULL AND cooldown_until > ?) OR (watch_until IS NOT NULL AND watch_until > ?)", now, now)
	}
	var total int64
	q.Count(&total)
	var list []aisafety.SafetyProfile
	q.Order("risk_score DESC, updated_at DESC").Offset((page - 1) * size).Limit(size).Find(&list)
	utils.Success(c, gin.H{"items": list, "total": total, "page": page})
}

// ClearAiSafetyCooldown POST /api/admin/ai/safety/profiles/:uid/clear
// 解除冷却 + 观察期（误判申诉时用）。风险分保留，便于仍能看出历史。
func ClearAiSafetyCooldown(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	uid, _ := strconv.ParseUint(c.Param("uid"), 10, 64)
	if uid == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := aisafety.ClearCooldown(uint(uid)); err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	utils.Success(c, gin.H{"message": "已解除"})
}

// ResetAiSafetyProfile POST /api/admin/ai/safety/profiles/:uid/reset
func ResetAiSafetyProfile(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	uid, _ := strconv.ParseUint(c.Param("uid"), 10, 64)
	if uid == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := aisafety.ResetProfile(uint(uid)); err != nil {
		utils.InternalError(c, "操作失败")
		return
	}
	utils.Success(c, gin.H{"message": "已重置"})
}

// ListJudgeLogs GET /api/admin/ai/safety/judge-logs
func ListJudgeLogs(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
	if size < 1 || size > 200 {
		size = 30
	}
	q := models.DB.Model(&aisafety.JudgeLog{})
	if c.Query("only_jailbreak") == "1" {
		q = q.Where("jailbroken = ?", true)
	}
	var total int64
	q.Count(&total)
	var list []aisafety.JudgeLog
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list)
	utils.Success(c, gin.H{"items": list, "total": total, "page": page})
}

// AiSafetyStats GET /api/admin/ai/safety/stats
func AiSafetyStats(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var (
		events, blocks, reviews, released, cooldowns   int64
		judged, jailbroken, judgeReleased, judgeFailed int64
		activeWatch, activeCooldown                    int64
	)
	models.DB.Model(&aisafety.SafetyEvent{}).Count(&events)
	models.DB.Model(&aisafety.SafetyEvent{}).Where("action = ?", aisafety.ActionBlock).Count(&blocks)
	models.DB.Model(&aisafety.SafetyEvent{}).Where("action = ?", aisafety.ActionReview).Count(&reviews)
	models.DB.Model(&aisafety.SafetyEvent{}).Where("action = ?", aisafety.ActionCooldown).Count(&cooldowns)
	models.DB.Model(&aisafety.SafetyEvent{}).Where("direction = ? AND action = ?", aisafety.DirOut, aisafety.ActionBlock).Count(&released)

	models.DB.Model(&aisafety.JudgeLog{}).Count(&judged)
	models.DB.Model(&aisafety.JudgeLog{}).Where("jailbroken = ?", true).Count(&jailbroken)
	models.DB.Model(&aisafety.JudgeLog{}).Where("released = ?", true).Count(&judgeReleased)
	models.DB.Model(&aisafety.JudgeLog{}).Where("failed = ?", true).Count(&judgeFailed)

	now := time.Now()
	models.DB.Model(&aisafety.SafetyProfile{}).
		Where("watch_until IS NOT NULL AND watch_until > ?", now).Count(&activeWatch)
	models.DB.Model(&aisafety.SafetyProfile{}).
		Where("cooldown_until IS NOT NULL AND cooldown_until > ?", now).Count(&activeCooldown)

	utils.Success(c, gin.H{
		"events": events, "blocks": blocks, "reviews": reviews,
		"cooldowns": cooldowns, "output_blocks": released,
		"judged": judged, "jailbroken": jailbroken,
		"judge_released": judgeReleased, "judge_failed": judgeFailed,
		"active_watch": activeWatch, "active_cooldown": activeCooldown,
	})
}
