package aicost

import (
	"time"

	"nvs-server/aiclient"
	"nvs-server/models"
)

// ==================== 渠道余额采样（把「消耗了多少货币单位」算出来） ====================
//
// 为什么需要采样：上游 API 不按请求返回「消耗了多少钱」，唯一权威值是该渠道账户的余额。
// 每次对话后都去查一次余额 = 每个请求两次上游调用，必然被限流、还会拖慢对话。
//
// 所以用「定时采样 + 窗口分摊」的口径：
//   1. 后台每 sampleInterval 采一次每个渠道的余额（只查有密钥的渠道）；
//   2. 相邻两次采样之间，用该渠道下模型的调用日志条数作为分母；
//   3. 期间消耗 / 期间请求数 ≈ 单次请求消耗的单位数。
//
// 这是**估算**，不是精确账单：采样窗口内并发/失败重试都会摊进去。
// 因此管理端必须标注「估算」，不能伪装成精确值。

// ChannelBalanceSample 一次渠道余额快照
type ChannelBalanceSample struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BaseURL   string    `gorm:"size:255;not null;index" json:"base_url"`
	Remaining float64   `json:"remaining"`
	Used      float64   `json:"used"`
	Granted   float64   `json:"granted"`
	Currency  string    `gorm:"size:16" json:"currency"`
	Supported bool      `json:"supported"`
	SampledAt time.Time `gorm:"index" json:"sampled_at"`
}

func (ChannelBalanceSample) TableName() string { return "channel_balance_samples" }

// sampleInterval 采样间隔。10 分钟是权衡：再短会打扰上游，再长则窗口里请求太多、
// 单次成本被摊平得失真。
const sampleInterval = 10 * time.Minute

// sampleRetention 采样保留时长（超过即清理，避免表无限增长）
const sampleRetention = 30 * 24 * time.Hour

func init() {
	go channelBalanceSampler()
}

func channelBalanceSampler() {
	// 首个 tick 刻意延后一轮：避免与启动时的迁移/初始化抢资源
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()
	for range ticker.C {
		SampleAllChannels()
	}
}

// SampleAllChannels 对每个有密钥的渠道采样一次（供后台定时调用，也可被管理端手动触发）。
// 单渠道失败只跳过，不影响其它渠道。
func SampleAllChannels() int {
	if models.DB == nil {
		return 0
	}
	var modelsList []models.AiModel
	if err := models.DB.Where("api_key_enc != ''").Order("base_url, id").Find(&modelsList).Error; err != nil {
		return 0
	}
	// 按 base_url 去重：同一渠道只采一次
	seen := map[string]bool{}
	ok := 0
	for i := range modelsList {
		m := &modelsList[i]
		if m.BaseURL == "" || seen[m.BaseURL] {
			continue
		}
		seen[m.BaseURL] = true
		key, err := m.DecryptApiKey()
		if err != nil {
			key = ""
		}
		b := aiclient.QueryChannelBalance(m.BaseURL, key, m.ProviderHint)
		if b.Error != "" && !b.Supported {
			// 无余额端点的渠道也记一条（supported=false），便于前端说明原因
			_ = models.DB.Create(&ChannelBalanceSample{
				BaseURL: m.BaseURL, Currency: b.Currency, Supported: false, SampledAt: time.Now(),
			}).Error
			continue
		}
		_ = models.DB.Create(&ChannelBalanceSample{
			BaseURL: m.BaseURL, Remaining: b.TotalRemaining, Used: b.TotalUsed,
			Granted: b.TotalGranted, Currency: b.Currency, Supported: b.Supported,
			SampledAt: time.Now(),
		}).Error
		ok++
	}
	// 顺手清理过期采样
	_ = models.DB.Where("sampled_at < ?", time.Now().Add(-sampleRetention)).
		Delete(&ChannelBalanceSample{}).Error
	return ok
}

// SaveChannelSample 记录一次渠道余额样本（后台定时采样与「管理端手动查询」共用）。
// 手动查询也会留样本，站长多点几次「渠道余额」就能快速攒出估算窗口。
func SaveChannelSample(baseURL string, b aiclient.ChannelBalance) {
	if models.DB == nil || baseURL == "" {
		return
	}
	_ = models.DB.Create(&ChannelBalanceSample{
		BaseURL: baseURL, Remaining: b.TotalRemaining, Used: b.TotalUsed,
		Granted: b.TotalGranted, Currency: b.Currency, Supported: b.Supported,
		SampledAt: time.Now(),
	}).Error
}

// ChannelWindowCost 两次采样之间的消耗与请求数（用于估算单次成本）
type ChannelWindowCost struct {
	HasData    bool    `json:"has_data"`
	From       string  `json:"from"`
	To         string  `json:"to"`
	CostUsed   float64 `json:"cost_used"`   // 窗口内消耗（单位：渠道币种）
	Currency   string  `json:"currency"`    // 单位（USD / CNY …）
	Requests   int64   `json:"requests"`    // 窗口内该渠道的请求数
	PerRequest float64 `json:"per_request"` // 估算单次消耗（货币单位）
	Minutes    int     `json:"minutes"`     // 窗口时长（分钟）

	// token 口径：只看金额分不清「便宜」和「省 token」，两个口径要一起看
	TokensIn      int64   `json:"tokens_in"`       // 窗口内输入 token 合计
	TokensOut     int64   `json:"tokens_out"`      // 窗口内输出 token 合计
	TokensTotal   int64   `json:"tokens_total"`    // 合计
	PerRequestTok int64   `json:"per_request_tok"` // 估算单次 token
	CostPer1KTok  float64 `json:"cost_per_1k_tok"` // 每千 token 的单位成本（估算）
}

// ResolveChannelWindowCost 取某渠道最近两次采样，算出窗口内的消耗与单次估算。
func ResolveChannelWindowCost(baseURL string) ChannelWindowCost {
	var out ChannelWindowCost
	if models.DB == nil {
		return out
	}
	var samples []ChannelBalanceSample
	if err := models.DB.Where("base_url = ? AND supported = ?", baseURL, true).
		Order("sampled_at DESC").Limit(2).Find(&samples).Error; err != nil || len(samples) < 2 {
		return out
	}
	latest, prev := samples[0], samples[1]
	usedDelta := latest.Used - prev.Used
	// 上游偶尔会重置累计值（比如换计费口径），负值不可信 → 退回用余额差值
	if usedDelta < 0 {
		usedDelta = prev.Remaining - latest.Remaining
	}
	if usedDelta < 0 {
		usedDelta = 0
	}
	// 该渠道下所有模型的请求数
	var modelIDs []uint
	models.DB.Model(&models.AiModel{}).Where("base_url = ?", baseURL).Pluck("id", &modelIDs)
	if len(modelIDs) > 0 {
		var agg struct {
			N    int64
			Tin  int64
			Tout int64
		}
		models.DB.Model(&models.AiCallLog{}).
			Select("COUNT(*) as n, COALESCE(SUM(tokens_in),0) as tin, COALESCE(SUM(tokens_out),0) as tout").
			Where("model_id IN ? AND created_at >= ? AND created_at <= ?", modelIDs, prev.SampledAt, latest.SampledAt).
			Scan(&agg)
		out.Requests = agg.N
		out.TokensIn = agg.Tin
		out.TokensOut = agg.Tout
		out.TokensTotal = agg.Tin + agg.Tout
	}
	out.HasData = true
	out.From = prev.SampledAt.Format(time.RFC3339)
	out.To = latest.SampledAt.Format(time.RFC3339)
	out.CostUsed = round4(usedDelta)
	out.Currency = latest.Currency
	out.Minutes = int(latest.SampledAt.Sub(prev.SampledAt).Minutes())
	if out.Requests > 0 {
		out.PerRequest = round4(usedDelta / float64(out.Requests))
		out.PerRequestTok = out.TokensTotal / out.Requests
	}
	if out.TokensTotal > 0 {
		// 每千 token 的单位成本：把「贵」和「用得多」拆开看
		out.CostPer1KTok = round4(usedDelta / (float64(out.TokensTotal) / 1000))
	}
	return out
}

// round4 保留 4 位小数（金额单位通常很敏感，太少小数会全变 0）
func round4(v float64) float64 {
	return float64(int64(v*10000+0.5)) / 10000
}
