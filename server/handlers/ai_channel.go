package handlers

import (
	"strconv"

	"nvs-server/aiclient"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ════════════════ AI 渠道对账 / 连通性（L5：拉取上游余额与消费，用于 API 对账） ════════════════

// AdminAiModelChannel GET /api/admin/ai/models/:id/channel
// 返回该上游渠道的余额 / 消费快照（对账用）与真实连通性探针。
// 余额查询仅对 OpenAI 兼容渠道有效；Anthropic / Azure 等无公开端点时明确标记 unsupported。
func AdminAiModelChannel(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	m, err := models.GetAiModel(uint(id))
	if err != nil {
		utils.NotFound(c, "模型不存在")
		return
	}
	key, kerr := m.DecryptApiKey()
	if kerr != nil {
		key = "" // 解密失败仍尝试（部分渠道可能无需密钥）
	}
	balance := aiclient.QueryChannelBalance(m.BaseURL, key, m.ProviderHint)
	probe := aiclient.ProbeChannel(m.BaseURL, key, m.ModelID)
	utils.Success(c, gin.H{
		"model_id":     m.ID,
		"model_name":   m.Name,
		"model_id_str": m.ModelID,
		"base_url":     m.BaseURL,
		"provider":     balance.Provider,
		"balance":      balance,
		"probe":        probe,
	})
}
