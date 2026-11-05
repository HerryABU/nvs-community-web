package ai

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/handlers/admin"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	aim "nvs-server/models/ai"
	aicost "nvs-server/models/aicost"
	aiops "nvs-server/models/aiops"
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
	if !admin.EnsureAdmin(c) {
		return
	}
	var ms []aim.AiModel
	if err := models.DB.Order("base_url, id").Find(&ms).Error; err != nil {
		utils.InternalError(c, "读取模型失败")
		return
	}

	// 按 base_url 去重，取该渠道下的第一个「有密钥」模型作为代表
	type chGroup struct {
		BaseURL     string
		ModelName   string
		ModelIDStr  string
		Hint        string
		TokenUserID string
		Count       int
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
			g.TokenUserID = m.TokenUserID
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

		var rep aim.AiModel
		b := aiclient.ChannelBalance{}
		if err := models.DB.Where("base_url = ?", g.BaseURL).Order("id").First(&rep).Error; err == nil {
			// 按该渠道代表模型配置的「余额查询方式」（官方/中转站/自定义）
			b = resolveModelBalance(&rep, key)
		} else {
			b = aiclient.QueryChannelBalanceWithUser(g.BaseURL, key, g.Hint, g.TokenUserID)
		}
		// 顺手留一个样本：站长多点几次「渠道余额」就能攒出估算窗口
		if b.Supported {
			aicost.SaveChannelSample(g.BaseURL, b)
		}
		item["provider"] = b.Provider
		item["supported"] = b.Supported
		item["total_granted"] = b.TotalGranted
		item["total_used"] = b.TotalUsed
		item["total_remaining"] = b.TotalRemaining
		item["currency"] = b.Currency
		item["fetched_at"] = b.FetchedAt
		acct, _ := decryptAccountTokenByBaseURL(g.BaseURL)
		if b.Error != "" {
			msg := b.Error
			// 中转站常见误配：只填了 sk_ 模型密钥就去查余额，接口返回 404/401。
			// 这里补一句可执行的引导，而不是把上游的 404 直接丢给站长。
			if strings.TrimSpace(acct) == "" &&
				(strings.Contains(msg, "404") || strings.Contains(msg, "余额")) {
				msg += "；若这是 New API / One API 中转站，需在模型里补「系统访问令牌(nap_)」与「令牌用户 ID」才能查账户余额"
			}
			item["error"] = msg
		}
		// 精确单次费用：New API 的 /api/log/self 每条日志的 quota 就是该次扣费，
		// 比「余额差值」精确得多（且不受"无限额度令牌"影响）。
		if strings.TrimSpace(acct) != "" && g.TokenUserID != "" {
			logs, lerr := aiclient.QueryNewAPIRecentLogs(g.BaseURL, acct, g.TokenUserID, 5, 15*time.Second)
			if lerr != "" {
				item["logs_error"] = lerr
			} else {
				item["recent_logs"] = logs
				if len(logs) > 0 {
					item["last_cost"] = logs[0].Quota
					item["last_model"] = logs[0].ModelName
				}
			}
		}

		// 窗口消耗 + 单次估算（估算值，前端必须标注）
		w := aicost.ResolveChannelWindowCost(g.BaseURL)
		item["window"] = w

		// ── 真实成本换算 ──
		// 上游给的「美元」在中转站里只是站内代币，必须按站长声明的充值比例
		// 才能折算成真金白银。未配置比例时**不给任何 real_* 字段**，
		// 前端据此显示「未设置比例」，而不是把虚拟金额当真实成本展示。
		rate, rateSrc := aicost.ResolveCostRate(g.BaseURL)
		item["rate_source"] = rateSrc
		if rate.Valid() {
			item["rate"] = gin.H{
				"real_amount":      rate.RealAmount,
				"virtual_amount":   rate.VirtualAmount,
				"real_currency":    rate.RealCurrency,
				"virtual_currency": rate.VirtualCurrency,
				"ratio":            rate.Ratio(),
				"describe":         rate.Describe(),
			}
			// ⚠️ 余额类字段只在「真的查到了」时换算：
			// 查不到余额却下发 real_remaining=0，界面会显示「真实 0 元」，
			// 让站长以为钱花光了 —— 比不显示更糟。
			if b.Supported {
				if v, ok := rate.ToReal(b.TotalRemaining); ok {
					item["real_remaining"] = v
				}
				if v, ok := rate.ToReal(b.TotalUsed); ok {
					item["real_used"] = v
				}
			}
			if lc, ok := item["last_cost"].(float64); ok {
				if v, ok2 := rate.ToReal(lc); ok2 {
					item["real_last_cost"] = v
				}
			}
			// 窗口统计本身带 HasData 标记，没有采样时不要编造 0 成本
			if w.HasData {
				if v, ok := rate.ToReal(w.CostUsed); ok {
					item["real_window_cost"] = v
				}
				if w.PerRequest > 0 {
					if pv, ok3 := rate.ToReal(w.PerRequest); ok3 {
						item["real_per_request"] = pv
					}
				}
			}
		}
		out = append(out, item)
	}

	utils.Success(c, gin.H{
		"channels": out,
		"probed":   probed,
		"total":    len(out),
	})
}

// decryptAccountTokenByBaseURL 找该渠道下第一个已配置的系统访问令牌并解密。
func decryptAccountTokenByBaseURL(baseURL string) (string, error) {
	var m aim.AiModel
	if err := models.DB.Where("base_url = ? AND account_token_enc != ''", baseURL).
		Order("id").First(&m).Error; err != nil {
		return "", err
	}
	return m.DecryptAccountToken()
}

// decryptKeyByBaseURL 找该渠道下第一个可用密钥并解密。
// 放在这里而不是遍历 handler 里，是为了让「按渠道查余额」的逻辑自洽。
func decryptKeyByBaseURL(baseURL string) string {
	var m aim.AiModel
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

// AdminAiUsageStats GET /api/admin/ai/usage-stats?dim=channel|model|vendor&days=30
//
// 模型池「模型统计」二级页的数据：同一份调用日志按三个维度聚合，
// 让站长能回答「这条渠道一共花了多少」「这个模型跑得稳不稳」「哪个厂商最贵」。
func AdminAiUsageStats(c *gin.Context) {
	if !admin.EnsureAdmin(c) {
		return
	}
	dim := c.DefaultQuery("dim", "model")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	rows, err := aiops.AggregateAiUsage(dim, days)
	if err != nil {
		utils.InternalError(c, "统计失败")
		return
	}
	// 汇总行：前端顶部直接显示合计，不必自己把数组再加一遍
	var totalReq, totalTok, totalCost int64
	for _, r := range rows {
		totalReq += r.Requests
		totalTok += r.TokensIn + r.TokensOut
		totalCost += r.Cost
	}
	utils.Success(c, gin.H{
		"dim":        dim,
		"days":       days,
		"rows":       rows,
		"total_req":  totalReq,
		"total_tok":  totalTok,
		"total_cost": totalCost,
	})
}
