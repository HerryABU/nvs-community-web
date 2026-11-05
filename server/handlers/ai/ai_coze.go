package ai

import (
	"nvs-server/aiclient"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	aim "nvs-server/models/ai"
	aihub "nvs-server/models/aihub"
	wallet "nvs-server/models/wallet"
)

// runCozeChat 扣子（Coze）智能体对话：非流式整段返回，仍按 SSE 事件回吐（前端无感）
// 计费：与普通对话一致按 tokens 估算结算（免费额度命中则不扣代币）。
func runCozeChat(c *gin.Context, ag *aim.AiAgent, callLog *aim.AiCallLog, session *aim.AiChatSession, userID uint, question string) {
	token, terr := ag.DecryptCozeToken()
	if terr != nil {
		aim.FailAiCallLog(callLog.ID, "扣子令牌解密失败")
		sseEvent(c, gin.H{"error": "扣子令牌解密失败，请联系站长重新保存"})
		return
	}
	if token == "" {
		aim.FailAiCallLog(callLog.ID, "未配置扣子令牌")
		sseEvent(c, gin.H{"error": "该智能体未配置扣子访问令牌，请联系站长"})
		return
	}
	// 全局默认值兜底：智能体未填 API 域名 / 超时时继承「扣子默认配置」
	def := aihub.GetAiCozeDefault()
	base := strings.TrimSpace(ag.CozeBase)
	if base == "" {
		base = def.BaseURL
	}
	timeout := time.Duration(def.TimeoutSeconds) * time.Second
	client := aiclient.NewCozeClient(base, token, timeout)
	answer, convID, cerr := client.Chat(ag.BotID, "u"+strconv.FormatUint(uint64(userID), 10),
		session.ThirdConvID, question, timeout)
	if cerr != nil {
		aim.FailAiCallLog(callLog.ID, cerr.Error())
		sseEvent(c, gin.H{"error": "扣子调用失败：" + cerr.Error()})
		return
	}
	aim.SetSessionThirdConv(session.ID, convID)
	aim.TouchAiAgent(ag.ID)

	in := aim.EstimateTokens(question)
	out := aim.EstimateTokens(answer)
	if in <= 0 {
		in = 1
	}
	cost, serr := aim.SettleAiCallLog(callLog.ID, in, out)
	if serr != nil {
		aim.FailAiCallLog(callLog.ID, serr.Error())
	}
	aim.AppendAiChatMessage(session.ID, "assistant", answer, in, out, cost, callLog.ID)
	nb := wallet.GetTokenBalance(userID)

	// 一次性把整段内容推给前端（与流式协议保持一致：先 content 后 done）
	sseEvent(c, gin.H{"content": answer})
	sseEvent(c, gin.H{
		"done": true, "cost": cost, "balance_after": nb,
		"session_id": session.ID, "free": callLog.FreeQuota,
		"tokens_in": in, "tokens_out": out,
	})
}
