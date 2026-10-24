package handlers

// ==================== AI 思维导图：需求 → 层级大纲 ====================
import (
	"strconv"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

func AiMindmap(c *gin.Context) {
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	userID := currentUserID(c)
	var req struct {
		Topic   string `json:"topic"`
		Depth   int    `json:"depth"`
		ModelID uint   `json:"model_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		utils.BadRequest(c, "请输入需求描述")
		return
	}
	if len([]rune(topic)) > 2000 {
		utils.BadRequest(c, "描述过长（上限 2000 字）")
		return
	}
	depth := req.Depth
	if depth < 2 || depth > 4 {
		depth = 3
	}
	var m *models.AiModel
	if req.ModelID > 0 {
		mm, err := models.GetEnabledAiModelOfKind(req.ModelID, models.AiKindChat)
		if err != nil || mm == nil {
			utils.BadRequest(c, "模型不可用")
			return
		}
		m = mm
	} else {
		list, err := models.ListAiModelsByKind(models.AiKindChat, true)
		if err != nil || len(list) == 0 {
			utils.BadRequest(c, "暂无可用对话模型")
			return
		}
		m = &list[0]
	}
	systemPrompt := "你是思维导图结构专家。请把用户需求整理为层级化 Markdown 大纲：" +
		"第一行用 `# ` 作为中心主题（不超过 20 字），随后用 `## ` 表示一级分支、" +
		"`### ` 表示二级、`- ` 表示叶子要点。层级不超过 " + strconv.Itoa(depth) +
		" 级，每个叶子要点不超过 18 字，只输出大纲，不要任何解释或代码块围栏。"
	messages := []aiclient.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: topic},
	}
	tokensIn := models.EstimateTokens(systemPrompt) + models.EstimateTokens(topic)
	tokensOut := 1200
	freeOK, _, _ := models.CheckAiFreeQuota(userID, tokensIn+tokensOut)
	if !freeOK {
		if !models.IsTokenEnabled() {
			utils.BadRequest(c, "代币系统未开启，无法使用付费 AI")
			return
		}
		if _, ok := models.PrecheckAiBalance(userID, m, tokensIn, tokensOut); !ok {
			utils.Error(c, 429, 42902, "余额不足，请先充值")
			return
		}
	}
	callLog, lerr := models.CreateAiCallLog(userID, m, "chat", "mindmap", 0, tokensIn, tokensOut, freeOK)
	if lerr != nil {
		utils.InternalError(c, "记录调用失败")
		return
	}
	lease, kerr := models.AcquireModelKey(m)
	apiKey := ""
	if lease != nil {
		apiKey = lease.Key
	}
	if kerr != nil {
		models.FailAiCallLog(callLog.ID, "密钥解密失败")
		utils.InternalError(c, "模型密钥解密失败，请联系站长")
		return
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 120*time.Second)
	chatReq := aiclient.ChatRequest{Model: m.ModelID, Messages: messages, MaxTokens: tokensOut, Temperature: 0.4}
	var out string
	var realIn, realOut int
	if models.NormalizeAiApiKind(m.ApiKind) == models.AiApiClaude {
		out, realIn, realOut, kerr = client.CompleteClaude(chatReq)
	} else {
		out, realIn, realOut, kerr = client.Complete(chatReq)
	}
	if kerr != nil {
		models.FailAiCallLog(callLog.ID, kerr.Error())
		utils.BadRequest(c, "生成失败："+kerr.Error()+"（未扣款）")
		return
	}
	if realIn <= 0 {
		realIn = tokensIn
	}
	if realOut <= 0 {
		realOut = models.EstimateTokens(out)
	}
	cost, serr := models.SettleAiCallLog(callLog.ID, realIn, realOut)
	if serr != nil {
		models.FailAiCallLog(callLog.ID, serr.Error())
	}
	nb := models.GetTokenBalance(userID)
	utils.Success(c, gin.H{
		"outline": strings.TrimSpace(out),
		"cost":    cost, "balance_after": nb, "free": freeOK,
		"tokens_in": realIn, "tokens_out": realOut,
	})
}
