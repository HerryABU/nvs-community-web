package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== AI 工作台：会话 + 流式对话（§5.4） ====================

// AiHubStatus GET /api/ai/hub/status — 工作台总状态（分类型模型 + 余额）
func AiHubStatus(c *gin.Context) {
	aiEnabled := models.IsAiEnabled()
	tokenEnabled := models.IsTokenEnabled()
	userID := currentUserID(c)
	out := gin.H{
		"ai_enabled":    aiEnabled,
		"token_enabled": tokenEnabled,
		"token_name":    models.GetTokenName(),
		"balance":       models.GetTokenBalance(userID),
		"chat_models":   []gin.H{},
		"image_models":  []gin.H{},
		"video_models":  []gin.H{},
	}
	if aiEnabled {
		collect := func(kind string) []gin.H {
			items := []gin.H{}
			list, err := models.ListAiModelsByKind(kind, true)
			if err != nil {
				return items
			}
			for i := range list {
				m := &list[i]
				items = append(items, gin.H{
					"id": m.ID, "name": m.Name, "model_id": m.ModelID,
					"price_in": m.PriceIn, "price_out": m.PriceOut,
					"price_call": m.PriceCall, "max_tokens": m.MaxTokens,
				})
			}
			return items
		}
		out["chat_models"] = collect(models.AiKindChat)
		out["image_models"] = collect(models.AiKindImage)
		out["video_models"] = collect(models.AiKindVideo)
	}
	utils.Success(c, out)
}

// AiHubSessions GET /api/ai/sessions — 我的会话列表
func AiHubSessions(c *gin.Context) {
	userID := currentUserID(c)
	list, err := models.ListAiChatSessions(userID, 100)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// AiHubSessionMessages GET /api/ai/sessions/:id/messages — 会话消息
func AiHubSessionMessages(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	s, err := models.GetAiChatSession(uint(id), userID)
	if err != nil {
		utils.NotFound(c, "会话不存在")
		return
	}
	list, err := models.ListAiChatMessages(s.ID, 200)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"session": s, "messages": list})
}

// AiHubDeleteSession DELETE /api/ai/sessions/:id — 删除会话
func AiHubDeleteSession(c *gin.Context) {
	userID := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := models.DeleteAiChatSession(uint(id), userID); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.Success(c, gin.H{"message": "已删除"})
}

// sseEvent 写一条 SSE 事件
func sseEvent(c *gin.Context, v interface{}) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	c.Writer.Flush()
}

// AiHubChatStream POST /api/ai/chat/stream — 流式对话（SSE）
// 事件序列：{"content":"..."} × N → {"done":true,"cost":..,"balance_after":..,"session_id":..}
// 失败：{"error":"..."}（未扣款）
func AiHubChatStream(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	if !models.IsTokenEnabled() {
		utils.BadRequest(c, "代币系统未开启，无法使用付费 AI")
		return
	}
	userID := currentUserID(c)
	var req struct {
		SessionID uint   `json:"session_id"`
		ModelID   uint   `json:"model_id"`
		AgentID   uint   `json:"agent_id"`
		Message   string `json:"message"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		utils.BadRequest(c, "请输入内容")
		return
	}
	if len([]rune(msg)) > 20000 {
		utils.BadRequest(c, "内容过长（上限 20000 字）")
		return
	}
	m, err := models.GetEnabledAiModelOfKind(req.ModelID, models.AiKindChat)
	if err != nil || m == nil {
		utils.BadRequest(c, "模型不可用")
		return
	}

	// ── 会话 ──
	var session *models.AiChatSession
	if req.SessionID > 0 {
		s, e := models.GetAiChatSession(req.SessionID, userID)
		if e != nil {
			utils.NotFound(c, "会话不存在")
			return
		}
		session = s
	} else {
		var agentName string
		if req.AgentID > 0 {
			if a, e := models.GetAiAgent(req.AgentID); e == nil && a.Enabled {
				agentName = a.Name
			}
		}
		s, e := models.CreateAiChatSession(userID, req.AgentID, agentName, m.ID)
		if e != nil {
			utils.InternalError(c, "创建会话失败")
			return
		}
		session = s
	}

	// ── 历史 + 系统提示词 ──
	history, _ := models.ListAiChatMessages(session.ID, 50)
	systemPrompt := "你是一位乐于助人的中文写作助手。"
	if session.AgentID > 0 {
		if a, e := models.GetAiAgent(session.AgentID); e == nil {
			systemPrompt = a.SystemPrompt
		}
	}
	messages := []aiclient.Message{{Role: "system", Content: systemPrompt}}
	// 仅带最近 20 条进上下文，控制成本
	start := 0
	if len(history) > 20 {
		start = len(history) - 20
	}
	for _, h := range history[start:] {
		messages = append(messages, aiclient.Message{Role: h.Role, Content: h.Content})
	}
	messages = append(messages, aiclient.Message{Role: "user", Content: msg})

	// ── 预检（不足直接 429，不开流） ──
	tokensIn := models.EstimateTokens(msg)
	for _, h := range history[start:] {
		tokensIn += models.EstimateTokens(h.Content)
	}
	tokensOut := req.MaxTokens
	if tokensOut <= 0 {
		tokensOut = m.MaxTokens
	}
	if tokensOut > 8192 {
		tokensOut = 8192
	}
	if _, ok := models.PrecheckAiBalance(userID, m, tokensIn, tokensOut); !ok {
		cost := models.EstimateAiCost(m, tokensIn, tokensOut)
		utils.Error(c, 429, 42902, fmt.Sprintf("%s不足：本次约需 %d，当前余额 %d，请先充值",
			models.GetTokenName(), cost, models.GetTokenBalance(userID)))
		return
	}

	// ── 落用户消息 + pending 流水 ──
	models.AppendAiChatMessage(session.ID, "user", msg, 0, 0, 0, 0)
	models.TouchAiChatSession(session.ID, msg)
	callLog, lerr := models.CreateAiCallLog(userID, m, "chat", "session", session.ID, tokensIn, tokensOut)
	if lerr != nil {
		utils.InternalError(c, "记录调用失败")
		return
	}

	// ── 开流 ──
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 反代关闭缓冲
	c.Status(200)

	apiKey, kerr := m.DecryptApiKey()
	if kerr != nil {
		models.FailAiCallLog(callLog.ID, "密钥解密失败")
		sseEvent(c, gin.H{"error": "模型密钥解密失败，请联系站长"})
		return
	}
	temp := m.Temperature
	client := aiclient.NewClient(m.BaseURL, apiKey, 180*time.Second)
	var sb string
	var realIn, realOut int
	var streamErr string
	for chunk := range client.CompleteStream(aiclient.ChatRequest{
		Model:       m.ModelID,
		Messages:    messages,
		MaxTokens:   tokensOut,
		Temperature: temp,
	}) {
		if chunk.Err != "" {
			streamErr = chunk.Err
			break
		}
		if chunk.Content != "" {
			sb += chunk.Content
			sseEvent(c, gin.H{"content": chunk.Content})
		}
		if chunk.Done {
			realIn = chunk.TokensIn
			realOut = chunk.TokensOut
		}
	}
	if streamErr != "" {
		models.FailAiCallLog(callLog.ID, streamErr)
		sseEvent(c, gin.H{"error": "AI 调用失败：" + streamErr})
		return
	}
	if sb == "" {
		models.FailAiCallLog(callLog.ID, "上游未返回内容")
		sseEvent(c, gin.H{"error": "AI 未返回内容，未扣款"})
		return
	}
	if realIn <= 0 {
		realIn = tokensIn
	}
	if realOut <= 0 {
		realOut = models.EstimateTokens(sb)
	}
	nb, serr := models.SettleAiCallLog(callLog.ID, realIn, realOut)
	if serr != nil {
		sseEvent(c, gin.H{"error": fmt.Sprintf("结算失败：%v（未扣款）", serr)})
		return
	}
	cost := models.EstimateAiCost(m, realIn, realOut)
	am, _ := models.AppendAiChatMessage(session.ID, "assistant", sb, realIn, realOut, cost, callLog.ID)
	models.TouchAiChatSession(session.ID, "")
	sseEvent(c, gin.H{
		"done":          true,
		"session_id":    session.ID,
		"message_id":    am.ID,
		"tokens_in":     realIn,
		"tokens_out":    realOut,
		"cost":          cost,
		"balance_after": nb,
	})
}
