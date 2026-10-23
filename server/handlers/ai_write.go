package handlers

import (
	"fmt"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== AI 调用（用户端，§5.3 / §13.3） ====================
// 流程：开关校验 → 模型校验 → 预估预检 → pending 流水 → 上游调用 →
// 按真实 usage 结算扣款（失败不扣款）。价格快照随代币流水落库。

// aiSystemPrompt 动作 → 系统提示词
func aiSystemPrompt(action string) string {
	switch action {
	case "continue":
		return "你是一位专业的中文小说写作助手。请根据用户提供的内容自然续写，保持人物、语气、设定一致，只输出续写正文，不要解释。"
	case "polish":
		return "你是一位专业的中文文字编辑。请润色用户提供的内容：修正语病、提升表达，保持原意与篇幅大致相当，只输出润色后的正文。"
	case "expand":
		return "你是一位专业的中文小说写作助手。请对用户提供的内容进行扩写：丰富细节、对话与环境描写，只输出扩写后的正文。"
	case "summarize":
		return "你是一位专业的内容摘要助手。请用简洁中文概括用户提供的内容，只输出摘要本身。"
	default:
		return "你是一位乐于助人的中文写作助手，请根据用户的要求完成创作，只输出结果正文。"
	}
}

// aiActionLabel 动作合法性（与 models.aiActionLabel 对应的白名单）
func aiActionValid(action string) bool {
	switch action {
	case "continue", "polish", "expand", "summarize", "custom":
		return true
	}
	return false
}

// AiStatus GET /api/ai/status — AI 可用性与我的余额/可用模型
func AiStatus(c *gin.Context) {
	aiEnabled := models.IsAiEnabled()
	tokenEnabled := models.IsTokenEnabled()
	userID := currentUserID(c)
	var modelList []gin.H
	if aiEnabled {
		list, err := models.ListAiModelsByKind(models.AiKindChat, true)
		if err == nil {
			for i := range list {
				m := &list[i]
				modelList = append(modelList, gin.H{
					"id": m.ID, "name": m.Name, "model_id": m.ModelID,
					"price_in": m.PriceIn, "price_out": m.PriceOut, "max_tokens": m.MaxTokens,
				})
			}
		}
	}
	utils.Success(c, gin.H{
		"ai_enabled":    aiEnabled,
		"token_enabled": tokenEnabled,
		"token_name":    models.GetTokenName(),
		"balance":       models.GetTokenBalance(userID),
		"models":        modelList,
		"free_quota":    models.SummarizeAiFreeQuota(userID),
	})
}

// AiEstimate POST /api/ai/estimate — 预估费用（不扣款）
func AiEstimate(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	userID := currentUserID(c)
	var req struct {
		ModelID   uint   `json:"model_id"`
		Text      string `json:"text"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	m, err := models.GetEnabledAiModel(req.ModelID)
	if err != nil || m == nil {
		utils.BadRequest(c, "模型不可用")
		return
	}
	tokensIn := models.EstimateTokens(req.Text)
	tokensOut := req.MaxTokens
	if tokensOut <= 0 {
		tokensOut = m.MaxTokens
	}
	cost := models.EstimateAiCost(m, tokensIn, tokensOut)
	freeOK, _, _ := models.CheckAiFreeQuota(userID, tokensIn+tokensOut)
	utils.Success(c, gin.H{
		"tokens_in":     tokensIn,
		"tokens_out":    tokensOut,
		"cost":          cost,
		"free_eligible": freeOK,
		"balance":       models.GetTokenBalance(userID),
		"affordable":    freeOK || cost <= 0 || models.GetTokenBalance(userID) >= cost,
	})
}

// AiComplete POST /api/ai/complete — 调用上游并按 usage 计费
func AiComplete(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	userID := currentUserID(c)
	var req struct {
		ModelID     uint   `json:"model_id"`
		Action      string `json:"action"`
		Text        string `json:"text"`
		MaxTokens   int    `json:"max_tokens"`
		Temperature *float64 `json:"temperature"`
		RefType     string `json:"ref_type"`
		RefID       uint   `json:"ref_id"`
		// 自备密钥（§5.5 BYOK）：byok=true 时用用户自带上游，不扣平台代币
		Byok      bool   `json:"byok"`
		ByokModel string `json:"byok_model"`
		ByokKeyID uint   `json:"byok_key_id"`
		// 长篇上下文（§13.5）：注入作品大纲 + 前文摘要 + 当前章尾部
		NovelID   uint `json:"novel_id"`
		ChapterID uint `json:"chapter_id"`
		UseMemory bool `json:"use_memory"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	action := strings.TrimSpace(req.Action)
	if !aiActionValid(action) {
		utils.BadRequest(c, "不支持的动作")
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		utils.BadRequest(c, "请输入内容")
		return
	}
	if len([]rune(text)) > 20000 {
		utils.BadRequest(c, "内容过长（上限 20000 字）")
		return
	}

	// ── 长篇上下文组装（§13.5，BYOK 与平台池均支持） ──
	memoryCtx := ""
	memoryTokens := 0
	if req.UseMemory && req.NovelID > 0 {
		memoryCtx, memoryTokens = models.BuildNovelContext(req.NovelID, req.ChapterID, text, models.AiContextBudget)
	}
	systemPrefix := aiContextBlock(memoryCtx)

	// ── 自备密钥分支（BYOK）：用户自带上游，不扣平台代币 ──
	if req.Byok {
		byokModel := strings.TrimSpace(req.ByokModel)
		if byokModel == "" {
			utils.BadRequest(c, "使用自备密钥时需填写上游模型标识")
			return
		}
		tokensIn := models.EstimateTokens(text) + memoryTokens
		tokensOut := req.MaxTokens
		if tokensOut <= 0 {
			tokensOut = 2048
		}
		if tokensOut > 8192 {
			tokensOut = 8192
		}
		uk, uerr := models.ResolveUserAiKey(userID, req.ByokKeyID)
		if uerr != nil {
			utils.InternalError(c, "读取自备密钥失败")
			return
		}
		if uk == nil {
			utils.BadRequest(c, "自备密钥不可用：请先在「我的钱包 → 上游密钥」中添加")
			return
		}
		apiKey, kerr := uk.DecryptApiKey()
		if kerr != nil {
			utils.InternalError(c, "自备密钥解密失败，请删除后重新添加")
			return
		}
		pseudo := &models.AiModel{ID: 0, Name: "自备密钥 · " + uk.Label}
		callLog, lerr := models.CreateAiCallLog(userID, pseudo, action, req.RefType, req.RefID, tokensIn, tokensOut, true)
		if lerr != nil {
			utils.InternalError(c, "记录调用失败")
			return
		}
		temp := 0.7
		if req.Temperature != nil {
			temp = *req.Temperature
		}
		client := aiclient.NewClient(uk.BaseURL, apiKey, 120*time.Second)
		content, realIn, realOut, cerr := client.Complete(aiclient.ChatRequest{
			Model: byokModel,
			Messages: []aiclient.Message{
				{Role: "system", Content: systemPrefix + aiSystemPrompt(action)},
				{Role: "user", Content: text},
			},
			MaxTokens:   tokensOut,
			Temperature: temp,
		})
		if cerr != nil {
			models.FailAiCallLog(callLog.ID, cerr.Error())
			utils.BadRequest(c, "AI 调用失败："+cerr.Error())
			return
		}
		if realIn <= 0 {
			realIn = tokensIn
		}
		if realOut <= 0 {
			realOut = models.EstimateTokens(content)
		}
		if _, serr := models.SettleAiCallLog(callLog.ID, realIn, realOut); serr != nil {
			utils.Error(c, 429, 42902, fmt.Sprintf("结算失败：%v", serr))
			return
		}
		models.TouchUserAiKey(uk.ID)
		utils.Success(c, gin.H{
			"content":       content,
			"tokens_in":     realIn,
			"tokens_out":    realOut,
			"cost":          0,
			"free":          false,
			"byok":          true,
			"balance_after": models.GetTokenBalance(userID),
		})
		return
	}

	m, err := models.GetEnabledAiModel(req.ModelID)
	if err != nil || m == nil {
		utils.BadRequest(c, "模型不可用")
		return
	}
	tokensIn := models.EstimateTokens(text) + memoryTokens
	tokensOut := req.MaxTokens
	if tokensOut <= 0 {
		tokensOut = m.MaxTokens
	}
	if tokensOut > 8192 {
		tokensOut = 8192
	}
	// 免费额度判定（优先）：命中则本次不扣代币
	freeOK, _, _ := models.CheckAiFreeQuota(userID, tokensIn+tokensOut)
	// 非免费路径：代币系统未开启或余额不足均拒绝（不发起上游）
	if !freeOK {
		if !models.IsTokenEnabled() {
			utils.BadRequest(c, "代币系统未开启，无法使用付费 AI")
			return
		}
		if _, ok := models.PrecheckAiBalance(userID, m, tokensIn, tokensOut); !ok {
			cost := models.EstimateAiCost(m, tokensIn, tokensOut)
			utils.Error(c, 429, 42902, fmt.Sprintf("%s不足：本次约需 %d，当前余额 %d，请先充值",
				models.GetTokenName(), cost, models.GetTokenBalance(userID)))
			return
		}
	}

	apiKey, kerr := m.DecryptApiKey()
	if kerr != nil {
		utils.InternalError(c, "模型密钥解密失败，请联系站长")
		return
	}
	// pending 流水（失败也不扣款的凭据）
	callLog, lerr := models.CreateAiCallLog(userID, m, action, req.RefType, req.RefID, tokensIn, tokensOut, freeOK)
	if lerr != nil {
		utils.InternalError(c, "记录调用失败")
		return
	}

	temp := m.Temperature
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 120*time.Second)
	content, realIn, realOut, cerr := client.Complete(aiclient.ChatRequest{
		Model: m.ModelID,
		Messages: []aiclient.Message{
			{Role: "system", Content: systemPrefix + aiSystemPrompt(action)},
			{Role: "user", Content: text},
		},
		MaxTokens:   tokensOut,
		Temperature: temp,
	})
	if cerr != nil {
		models.FailAiCallLog(callLog.ID, cerr.Error())
		utils.BadRequest(c, "AI 调用失败："+cerr.Error())
		return
	}
	if realIn <= 0 {
		realIn = tokensIn
	}
	if realOut <= 0 {
		realOut = models.EstimateTokens(content)
	}
	nb, serr := models.SettleAiCallLog(callLog.ID, realIn, realOut)
	if serr != nil {
		utils.Error(c, 429, 42902, fmt.Sprintf("结算失败：%v（未扣款）", serr))
		return
	}
	cost := models.EstimateAiCost(m, realIn, realOut)
	if freeOK {
		// 免费额度调用：记入当日免费用量（1 次 + 真实 tokens）
		_ = models.RecordAiFreeUsage(userID, 1, realIn+realOut)
		cost = 0
	}
	utils.Success(c, gin.H{
		"content":       content,
		"tokens_in":     realIn,
		"tokens_out":    realOut,
		"cost":          cost,
		"free":          freeOK,
		"balance_after": nb,
	})
}
