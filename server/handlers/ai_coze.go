package handlers

import (
	"strconv"
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"

	"github.com/gin-gonic/gin"
)

// runCozeChat 扣子（Coze）智能体对话：非流式整段返回，仍按 SSE 事件回吐（前端无感）
// 计费：与普通对话一致按 tokens 估算结算（免费额度命中则不扣代币）。
func runCozeChat(c *gin.Context, ag *models.AiAgent, callLog *models.AiCallLog, session *models.AiChatSession, userID uint, question string) {
	token, terr := ag.DecryptCozeToken()
	if terr != nil {
		models.FailAiCallLog(callLog.ID, "扣子令牌解密失败")
		sseEvent(c, gin.H{"error": "扣子令牌解密失败，请联系站长重新保存"})
		return
	}
	if token == "" {
		models.FailAiCallLog(callLog.ID, "未配置扣子令牌")
		sseEvent(c, gin.H{"error": "该智能体未配置扣子访问令牌，请联系站长"})
		return
	}
	client := aiclient.NewCozeClient(ag.CozeBase, token, 150*time.Second)
	answer, convID, cerr := client.Chat(ag.BotID, "u"+strconv.FormatUint(uint64(userID), 10),
		session.ThirdConvID, question, 150*time.Second)
	if cerr != nil {
		models.FailAiCallLog(callLog.ID, cerr.Error())
		sseEvent(c, gin.H{"error": "扣子调用失败：" + cerr.Error()})
		return
	}
	models.SetSessionThirdConv(session.ID, convID)
	models.TouchAiAgent(ag.ID)

	in := models.EstimateTokens(question)
	out := models.EstimateTokens(answer)
	if in <= 0 {
		in = 1
	}
	cost, serr := models.SettleAiCallLog(callLog.ID, in, out)
	if serr != nil {
		models.FailAiCallLog(callLog.ID, serr.Error())
	}
	models.AppendAiChatMessage(session.ID, "assistant", answer, in, out, cost, callLog.ID)
	nb := models.GetTokenBalance(userID)

	// 一次性把整段内容推给前端（与流式协议保持一致：先 content 后 done）
	sseEvent(c, gin.H{"content": answer})
	sseEvent(c, gin.H{
		"done": true, "cost": cost, "balance_after": nb,
		"session_id": session.ID, "free": callLog.FreeQuota,
		"tokens_in": in, "tokens_out": out,
	})
}
