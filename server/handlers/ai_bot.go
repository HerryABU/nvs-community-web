package handlers

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
	"nvs-server/realtime"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== @ai 评论区机器人（§5.6） ====================
// 评论中 @ai / @智能体名 → 异步生成 AI 回复（机器人身份发评论）。
// 计费：发起人付费（免费额度优先 → 代币预检）；每日限流防刷。

// aiBotRecent 同楼去重窗口（内存态，重启清零）
var (
	aiBotMu     sync.Mutex
	aiBotRecent = map[string]time.Time{} // key: floorKey(parent/root) → 最近触发时间
)

const aiBotDedupWindow = 60 * time.Second

// maybeTriggerAiBot 评论创建成功后调用：检测 @ai 召唤并异步触发
func maybeTriggerAiBot(c *gin.Context, comment *models.Comment) {
	if !models.IsAiBotEnabled() || !models.IsAiEnabled() {
		return
	}
	agentName, prompt, ok := models.ParseAiBotInvocation(comment.Content)
	if !ok || prompt == "" {
		return
	}
	cfg := models.GetAiBotConfig()
	userID := comment.UserID
	if !models.CheckAiBotQuota(userID, cfg.DailyLimit) {
		return // 超限静默跳过（前端 status 展示剩余次数）
	}

	// 同楼去重：60 秒内同一父/楼只触发一次
	floorKey := fmt.Sprintf("%d:%d:%d", comment.NovelID, comment.BlogID, floorKeyOf(comment))
	aiBotMu.Lock()
	if last, seen := aiBotRecent[floorKey]; seen && time.Since(last) < aiBotDedupWindow {
		aiBotMu.Unlock()
		return
	}
	aiBotRecent[floorKey] = time.Now()
	aiBotMu.Unlock()

	// 快照触发上下文（goroutine 内不依赖 gin.Context）
	parentContent := ""
	if comment.ParentID != nil {
		if parent, perr := models.GetCommentByID(*comment.ParentID); perr == nil {
			parentContent = parent.Content
		}
	}
	triggerContent := comment.Content
	triggerExcerpt := prompt
	if len([]rune(triggerExcerpt)) > 500 {
		triggerExcerpt = string([]rune(triggerExcerpt)[:500]) + "…"
	}

	go func() {
		defer func() { _ = recover() }() // 机器人故障绝不拖垮评论主流程
		generateAiBotReply(userID, comment, cfg, agentName, parentContent, triggerContent, triggerExcerpt)
	}()
}

func floorKeyOf(comment *models.Comment) uint {
	if comment.RootID != nil {
		return *comment.RootID
	}
	if comment.ParentID != nil {
		return *comment.ParentID
	}
	return comment.ID
}

// generateAiBotReply 异步生成并落库机器人评论
func generateAiBotReply(triggerUser uint, trigger *models.Comment, cfg models.AiBotConfig,
	agentName, parentContent, triggerContent, prompt string) {

	// 模型：平台启用的第一个 chat 模型
	list, err := models.ListAiModelsByKind(models.AiKindChat, true)
	if err != nil || len(list) == 0 {
		return
	}
	m := &list[0]

	// 计费预检：免费额度优先，否则代币
	estTokens := models.EstimateTokens(prompt) + models.EstimateTokens(parentContent) + 512
	freeOK, _, _ := models.CheckAiFreeQuota(triggerUser, estTokens)
	if !freeOK {
		if !models.IsTokenEnabled() {
			return
		}
		if _, ok := models.PrecheckAiBalance(triggerUser, m, estTokens, m.MaxTokens); !ok {
			return // 余额不足：静默（评论区不打扰）
		}
	}

	// 人设：@智能体名 优先，否则配置默认
	systemPrompt := "你是评论区里的 AI 助手，回答请简洁有趣、贴合讨论上下文，直接输出回复内容，不要解释。"
	if agentName != "" {
		if agents, aerr := models.ListAiAgents(true); aerr == nil {
			for _, a := range agents {
				if a.Name == agentName {
					systemPrompt = a.SystemPrompt + "\n（请以该人设简洁回复评论，直接输出内容）"
					break
				}
			}
		}
	} else if cfg.AgentID > 0 {
		if a, aerr := models.GetAiAgent(cfg.AgentID); aerr == nil && a.Enabled {
			systemPrompt = a.SystemPrompt + "\n（请以该人设简洁回复评论，直接输出内容）"
		}
	}

	var sb strings.Builder
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n【讨论上下文】\n")
	if parentContent != "" {
		sb.WriteString("被回复的评论：" + parentContent + "\n")
	}
	sb.WriteString("召唤者说：" + triggerContent + "\n")
	sb.WriteString("请针对以上内容给出回复。")

	apiKey, kerr := m.DecryptApiKey()
	if kerr != nil {
		return
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 60*time.Second)
	callLog, lerr := models.CreateAiCallLog(triggerUser, m, "bot", "comment", trigger.ID,
		models.EstimateTokens(sb.String()), m.MaxTokens, freeOK)
	if lerr != nil {
		return
	}
	content, realIn, realOut, cerr := client.Complete(aiclient.ChatRequest{
		Model: m.ModelID,
		Messages: []aiclient.Message{
			{Role: "system", Content: sb.String()},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   m.MaxTokens,
		Temperature: 0.7,
	})
	if cerr != nil {
		models.FailAiCallLog(callLog.ID, cerr.Error())
		return
	}
	if strings.TrimSpace(content) == "" {
		models.FailAiCallLog(callLog.ID, "上游未返回内容")
		return
	}
	if realIn <= 0 {
		realIn = estTokens
	}
	if realOut <= 0 {
		realOut = models.EstimateTokens(content)
	}
	cost := models.EstimateAiCost(m, realIn, realOut)
	if freeOK {
		_ = models.RecordAiFreeUsage(triggerUser, 1, realIn+realOut)
		cost = 0
	} else if _, serr := models.SettleAiCallLog(callLog.ID, realIn, realOut); serr != nil {
		return // 结算失败（余额被并发耗尽）：不发布
	}
	models.RecordAiBotUsage(triggerUser)

	// 机器人身份发评论：挂靠触发评论同楼
	botID, berr := models.EnsureAiBotUser()
	if berr != nil {
		return
	}
	body := content
	if cfg.Signature != "" {
		body += "\n\n" + cfg.Signature
	}
	if !freeOK && cost > 0 {
		body += fmt.Sprintf("\n\n<small>本次召唤消耗 %d %s</small>", cost, models.GetTokenName())
	}
	reply := &models.Comment{
		UserID:        botID,
		NovelID:       trigger.NovelID,
		BlogID:        trigger.BlogID,
		ChapterNumber: trigger.ChapterNumber,
		Content:       body,
		ParentID:      trigger.ParentID,
		RootID:        trigger.RootID,
		ReplyToUserID: trigger.UserID,
		IsMarkdown:    true,
		CreatedMicro:  time.Now().UnixMicro(),
	}
	if reply.ParentID == nil {
		reply.ParentID = &trigger.ID // 楼中楼：挂到触发评论下
	}
	if reply.RootID == nil {
		reply.RootID = &trigger.ID
	}
	if err := models.CreateComment(reply); err != nil {
		return
	}
	_ = models.PushNotification(botID, trigger.UserID, "ai_bot_reply",
		"AI 助手回复了你的召唤", "AI 助手已回应你的 @ 召唤", models.LinkForComment(reply))
	if trigger.NovelID > 0 {
		realtime.PublishCommentEvent("novel", trigger.NovelID, reply)
	} else if trigger.BlogID > 0 {
		realtime.PublishCommentEvent("blog", trigger.BlogID, reply)
	}
}

// ---------- 用户端 / 管理端 ----------

// AiBotStatus GET /api/ai/bot/status — 召唤状态与今日剩余次数
func AiBotStatus(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	cfg := models.GetAiBotConfig()
	used := models.CountTodayAiBotUsage(currentUserID(c))
	left := cfg.DailyLimit
	if left > 0 {
		left = left - int(used)
		if left < 0 {
			left = 0
		}
	}
	utils.Success(c, gin.H{
		"enabled":    models.IsAiBotEnabled(),
		"used":       int(used),
		"daily_left": left, // -1=不限
	})
}

// AdminGetAiBotConfig GET /api/admin/ai/bot/config
func AdminGetAiBotConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"enabled": models.IsAiBotEnabled(),
		"config":  models.GetAiBotConfig(),
	})
}

// AdminSetAiBotConfig PUT /api/admin/ai/bot/config
func AdminSetAiBotConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Enabled   bool   `json:"enabled"`
		DailyLimit int   `json:"daily_limit"`
		AgentID   uint   `json:"agent_id"`
		Signature string `json:"signature"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SetAiBotConfig(models.AiBotConfig{
		DailyLimit: req.DailyLimit, AgentID: req.AgentID, Signature: req.Signature,
	}); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	if err := models.SetAiBotEnabled(req.Enabled); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.Success(c, gin.H{"message": "AI 评论机器人配置已保存（立即生效）"})
}
