package handlers

import (
	"sort"

	"nvs-server/aiclient"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// AdminChannelBalanceOverview GET /api/admin/ai/channels/balance
//
// 按**渠道**（上游 base_url）聚合，逐个查一次余额，回答站长最关心的问题：
// 「每一条渠道上还剩多少钱」。
//
// 为什么不逐模型查：同一渠道下挂着几十个模型，逐个查等于对上游连打几十次，
// 既慢又容易被限流；余额是账户级的，按渠道去重后查一次即可。
//
// 安全与稳定性：
//   - 只统计「有密钥」的模型（无密钥的渠道查不了余额）；
//   - 单次最多查 maxChannelProbe 条，避免自建了几十条渠道时把请求拖死；
//   - 逐条串行 + balance.go 内部超时，单条失败只标记 error，不影响其它渠道。
const maxChannelProbe = 12

func AdminChannelBalanceOverview(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var ms []models.AiModel
	if err := models.DB.Order("base_url, id").Find(&ms).Error; err != nil {
		utils.InternalError(c, "读取模型失败")
		return
	}

	// 按 base_url 去重，取该渠道下的第一个「有密钥」模型作为代表
	type chGroup struct {
		BaseURL    string
		ModelName  string
		ModelIDStr string
		Hint       string
		Count      int
	}
	groups := map[string]*chGroup{}
	for i := range ms {
		m := &ms[i]
		base := m.BaseURL
		if base == "" {
			continue
		}
		g, ok := groups[base]
		if !ok {
			g = &chGroup{BaseURL: base, Count: 0}
			groups[base] = g
		}
		g.Count++
		if g.ModelName == "" && m.ApiKeyEnc != "" {
			g.ModelName = m.Name
			g.ModelIDStr = m.ModelID
			g.Hint = m.ProviderHint
		}
	}

	list := make([]*chGroup, 0, len(groups))
	for _, g := range groups {
		list = append(list, g)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].BaseURL < list[j].BaseURL })

	out := make([]gin.H, 0, len(list))
	probed := 0
	for _, g := range list {
		item := gin.H{
			"base_url":     g.BaseURL,
			"models":       g.Count,
			"model_name":   g.ModelName,
			"model_id_str": g.ModelIDStr,
		}
		if g.ModelName == "" {
			item["supported"] = false
			item["error"] = "该渠道下没有已配置密钥的模型，无法查询余额"
			out = append(out, item)
			continue
		}
		if probed >= maxChannelProbe {
			item["supported"] = false
			item["error"] = "本次已达查询上限，稍后重试可查剩余渠道"
			out = append(out, item)
			continue
		}
		probed++

		// 取该渠道下第一个可用密钥（解密失败仍尝试，部分渠道无需密钥）
		key := decryptKeyByBaseURL(g.BaseURL)

		b := aiclient.QueryChannelBalance(g.BaseURL, key, g.Hint)
		// 顺手留一个样本：站长多点几次「渠道余额」就能攒出估算窗口
		if b.Supported {
			models.SaveChannelSample(g.BaseURL, b)
		}
		item["provider"] = b.Provider
		item["supported"] = b.Supported
		item["total_granted"] = b.TotalGranted
		item["total_used"] = b.TotalUsed
		item["total_remaining"] = b.TotalRemaining
		item["currency"] = b.Currency
		item["fetched_at"] = b.FetchedAt
		if b.Error != "" {
			item["error"] = b.Error
		}
		// 窗口消耗 + 单次估算（估算值，前端必须标注）
		w := models.ResolveChannelWindowCost(g.BaseURL)
		item["window"] = w
		out = append(out, item)
	}

	utils.Success(c, gin.H{
		"channels": out,
		"probed":   probed,
		"total":    len(out),
	})
}

// decryptKeyByBaseURL 找该渠道下第一个可用密钥并解密。
// 放在这里而不是遍历 handler 里，是为了让「按渠道查余额」的逻辑自洽。
func decryptKeyByBaseURL(baseURL string) string {
	var m models.AiModel
	if err := models.DB.Where("base_url = ? AND api_key_enc != ''", baseURL).
		Order("id").First(&m).Error; err != nil {
		return ""
	}
	key, err := m.DecryptApiKey()
	if err != nil {
		return ""
	}
	return key
}
