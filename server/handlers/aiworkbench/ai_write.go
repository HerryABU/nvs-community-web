package aiworkbench

import (
	"fmt"
	"nvs-server/aiclient"
	"nvs-server/handlers/hctx"
	"nvs-server/utils/infra"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	aim "nvs-server/models/ai"
	aicost "nvs-server/models/aicost"
	aihub "nvs-server/models/aihub"
	aiops "nvs-server/models/aiops"
	wallet "nvs-server/models/wallet"
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
	case "mindmap":
		return "你是一位擅长把长文压缩成结构图的中文编辑。请把用户提供的内容整理成 Mermaid mindmap 语法，" +
			"只输出一个 ```mermaid 代码块，其它什么都不要写。要求：" +
			"1) 首行固定 `mindmap`，根节点用圆括号包裹主题；" +
			"2) 层级 2~4 层，节点文字不超过 12 字，不要标点堆叠；" +
			"3) 分支按原文逻辑（人物/事件/设定/因果）归并，不逐句罗列；" +
			"4) 不编造原文没有的内容，宁可少一层。"
	case "character":
		return "你是一位资深的中文小说角色设定师。请根据用户提供的线索设计角色卡，按以下小标题输出：" +
			"【姓名与称谓】【外貌（3 个可视觉化细节）】【性格与致命缺陷】【出身与当前处境】" +
			"【核心欲望 / 深层需求】【秘密（读者后知）】【与主角的关系张力】" +
			"【说话方式（口头禅 / 句长 / 用词层级）】【三个可推进情节的行动选项】。" +
			"要求：每条都能直接落到场景里，不要写成简历，不要评价文笔，只输出设定卡。"
	default:
		return "你是一位乐于助人的中文写作助手，请根据用户的要求完成创作，只输出结果正文。"
	}
}

// aiActionLabel 动作合法性（与 models.aiActionLabel 对应的白名单）
func aiActionValid(action string) bool {
	switch action {
	case "continue", "polish", "expand", "summarize", "character", "mindmap", "custom":
		return true
	}
	return false
}

// AiStatus GET /api/ai/status — AI 可用性与我的余额/可用模型
func AiStatus(c *gin.Context) {
	aiEnabled := aim.IsAiEnabled()
	tokenEnabled := wallet.IsTokenEnabled()
	userID := hctx.CurrentUserID(c)
	var modelList []gin.H
	if aiEnabled {
		list, err := aim.ListAiModelsByKind(aim.AiKindChat, true)
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
	infra.Success(c, gin.H{
		"ai_enabled":    aiEnabled,
		"token_enabled": tokenEnabled,
		"token_name":    wallet.GetTokenName(),
		"balance":       wallet.GetTokenBalance(userID),
		"models":        modelList,
		"free_quota":    aihub.SummarizeAiFreeQuota(userID),
	})
}

// AiEstimate POST /api/ai/estimate — 预估费用（不扣款）
func AiEstimate(c *gin.Context) {
	if !aim.IsAiEnabled() {
		infra.BadRequest(c, "AI 功能未开启")
		return
	}
	userID := hctx.CurrentUserID(c)
	var req struct {
		ModelID   uint   `json:"model_id"`
		Text      string `json:"text"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	m, err := aim.GetEnabledAiModel(req.ModelID)
	if err != nil || m == nil {
		infra.BadRequest(c, "模型不可用")
		return
	}
	tokensIn := aim.EstimateTokens(req.Text)
	tokensOut := req.MaxTokens
	if tokensOut <= 0 {
		tokensOut = m.MaxTokens
	}
	cost := aim.EstimateAiCost(m, tokensIn, tokensOut)
	freeOK, _, _ := aihub.CheckAiFreeQuota(userID, tokensIn+tokensOut)
	infra.Success(c, gin.H{
		"tokens_in":     tokensIn,
		"tokens_out":    tokensOut,
		"cost":          cost,
		"free_eligible": freeOK,
		"balance":       wallet.GetTokenBalance(userID),
		"affordable":    freeOK || cost <= 0 || wallet.GetTokenBalance(userID) >= cost,
	})
}

// AiComplete POST /api/ai/complete — 调用上游并按 usage 计费
func AiComplete(c *gin.Context) {
	if !aim.IsAiEnabled() {
		infra.BadRequest(c, "AI 功能未开启")
		return
	}
	userID := hctx.CurrentUserID(c)
	var req struct {
		ModelID     uint     `json:"model_id"`
		Action      string   `json:"action"`
		Text        string   `json:"text"`
		MaxTokens   int      `json:"max_tokens"`
		Temperature *float64 `json:"temperature"`
		RefType     string   `json:"ref_type"`
		RefID       uint     `json:"ref_id"`
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
		infra.BadRequest(c, "参数错误")
		return
	}
	action := strings.TrimSpace(req.Action)
	if !aiActionValid(action) {
		infra.BadRequest(c, "不支持的动作")
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		infra.BadRequest(c, "请输入内容")
		return
	}
	if len([]rune(text)) > 20000 {
		infra.BadRequest(c, "内容过长（上限 20000 字）")
		return
	}

	// ── 长篇上下文组装（§13.5，BYOK 与平台池均支持） ──
	memoryCtx := ""
	memoryTokens := 0
	if req.UseMemory && req.NovelID > 0 {
		memoryCtx, memoryTokens = aiops.BuildNovelContext(req.NovelID, req.ChapterID, text, aiops.AiContextBudget)
	}
	systemPrefix := aiContextBlock(memoryCtx)

	// ── 自备密钥分支（BYOK）：用户自带上游，不扣平台代币 ──
	if req.Byok {
		byokModel := strings.TrimSpace(req.ByokModel)
		if byokModel == "" {
			infra.BadRequest(c, "使用自备密钥时需填写上游模型标识")
			return
		}
		tokensIn := aim.EstimateTokens(text) + memoryTokens
		tokensOut := req.MaxTokens
		if tokensOut <= 0 {
			tokensOut = 2048
		}
		if tokensOut > 8192 {
			tokensOut = 8192
		}
		uk, uerr := aicost.ResolveUserAiKey(userID, req.ByokKeyID)
		if uerr != nil {
			infra.InternalError(c, "读取自备密钥失败")
			return
		}
		if uk == nil {
			infra.BadRequest(c, "自备密钥不可用：请先在「我的钱包 → 上游密钥」中添加")
			return
		}
		apiKey, kerr := uk.DecryptApiKey()
		if kerr != nil {
			infra.InternalError(c, "自备密钥解密失败，请删除后重新添加")
			return
		}
		pseudo := &aim.AiModel{ID: 0, Name: "自备密钥 · " + uk.Label}
		callLog, lerr := aim.CreateAiCallLog(userID, pseudo, action, req.RefType, req.RefID, tokensIn, tokensOut, true)
		if lerr != nil {
			infra.InternalError(c, "记录调用失败")
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
			aim.FailAiCallLog(callLog.ID, cerr.Error())
			infra.BadRequest(c, "AI 调用失败："+cerr.Error())
			return
		}
		if realIn <= 0 {
			realIn = tokensIn
		}
		if realOut <= 0 {
			realOut = aim.EstimateTokens(content)
		}
		if _, serr := aim.SettleAiCallLog(callLog.ID, realIn, realOut); serr != nil {
			infra.Error(c, 429, 42902, fmt.Sprintf("结算失败：%v", serr))
			return
		}
		aicost.TouchUserAiKey(uk.ID)
		infra.Success(c, gin.H{
			"content":       content,
			"tokens_in":     realIn,
			"tokens_out":    realOut,
			"cost":          0,
			"free":          false,
			"byok":          true,
			"balance_after": wallet.GetTokenBalance(userID),
		})
		return
	}

	m, err := aim.GetEnabledAiModel(req.ModelID)
	if err != nil || m == nil {
		infra.BadRequest(c, "模型不可用")
		return
	}
	tokensIn := aim.EstimateTokens(text) + memoryTokens
	tokensOut := req.MaxTokens
	if tokensOut <= 0 {
		tokensOut = m.MaxTokens
	}
	if tokensOut > 8192 {
		tokensOut = 8192
	}
	// 免费额度判定（优先）：命中则本次不扣代币
	freeOK, _, _ := aihub.CheckAiFreeQuota(userID, tokensIn+tokensOut)
	// 非免费路径：代币系统未开启或余额不足均拒绝（不发起上游）
	if !freeOK {
		if !wallet.IsTokenEnabled() {
			infra.BadRequest(c, "代币系统未开启，无法使用付费 AI")
			return
		}
		if _, ok := aim.PrecheckAiBalance(userID, m, tokensIn, tokensOut); !ok {
			cost := aim.EstimateAiCost(m, tokensIn, tokensOut)
			infra.Error(c, 429, 42902, fmt.Sprintf("%s不足：本次约需 %d，当前余额 %d，请先充值",
				wallet.GetTokenName(), cost, wallet.GetTokenBalance(userID)))
			return
		}
	}

	apiKey, kerr := m.DecryptApiKey()
	if kerr != nil {
		infra.InternalError(c, "模型密钥解密失败，请联系站长")
		return
	}
	// pending 流水（失败也不扣款的凭据）
	callLog, lerr := aim.CreateAiCallLog(userID, m, action, req.RefType, req.RefID, tokensIn, tokensOut, freeOK)
	if lerr != nil {
		infra.InternalError(c, "记录调用失败")
		return
	}

	temp := m.Temperature
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 120*time.Second)
	completeFn := client.Complete
	if aim.NormalizeAiApiKind(m.ApiKind) == aim.AiApiClaude {
		completeFn = client.CompleteClaude // Anthropic Messages API
	}
	content, realIn, realOut, cerr := completeFn(aiclient.ChatRequest{
		Model: m.ModelID,
		Messages: []aiclient.Message{
			{Role: "system", Content: systemPrefix + aiSystemPrompt(action)},
			{Role: "user", Content: text},
		},
		MaxTokens:   tokensOut,
		Temperature: temp,
	})
	if cerr != nil {
		aim.FailAiCallLog(callLog.ID, cerr.Error())
		infra.BadRequest(c, "AI 调用失败："+cerr.Error())
		return
	}
	if realIn <= 0 {
		realIn = tokensIn
	}
	if realOut <= 0 {
		realOut = aim.EstimateTokens(content)
	}
	nb, serr := aim.SettleAiCallLog(callLog.ID, realIn, realOut)
	if serr != nil {
		infra.Error(c, 429, 42902, fmt.Sprintf("结算失败：%v（未扣款）", serr))
		return
	}
	cost := aim.EstimateAiCost(m, realIn, realOut)
	if freeOK {
		// 免费额度调用：记入当日免费用量（1 次 + 真实 tokens）
		_ = aihub.RecordAiFreeUsage(userID, 1, realIn+realOut)
		cost = 0
	}
	infra.Success(c, gin.H{
		"content":       content,
		"tokens_in":     realIn,
		"tokens_out":    realOut,
		"cost":          cost,
		"free":          freeOK,
		"balance_after": nb,
	})
}
