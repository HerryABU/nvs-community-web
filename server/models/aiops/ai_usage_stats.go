package aiops

import (
	"nvs-server/models"
	ai "nvs-server/models/ai"
	"sort"
	"strings"
	"time"
)

// ==================== AI 用量统计（按渠道 / 模型 / 厂商聚合） ====================
//
// 数据源是 ai.AiCallLog（每次调用一行，含 tokens / 实扣代币 / 状态 / 延迟）。
// 三个维度共用同一份聚合，只是最后的分组键不同：
//
//	model   —— 按 model_id（看单个模型跑得怎么样）
//	channel —— 按 base_url（看一条渠道整体消耗，与「渠道余额」对得上）
//	vendor  —— 按厂商（看 DeepSeek / OpenAI 各花了多少）
//
// 先按 model_id 做一次 group by（模型数是常量级），再在内存里按维度归并 ——
// 避免对大表做多列 group by，也避免 join（channel/vendor 存在 ai.AiModel 表里）。

// AiUsageRow 一个分组的用量
type AiUsageRow struct {
	Key         string  `json:"key"`         // model_id / base_url / vendor
	Name        string  `json:"name"`        // 展示名
	Requests    int64   `json:"requests"`    // 调用次数
	Success     int64   `json:"success"`     // 成功次数
	Fail        int64   `json:"fail"`        // 失败次数
	TokensIn    int64   `json:"tokens_in"`   // 输入 tokens
	TokensOut   int64   `json:"tokens_out"`  // 输出 tokens
	Cost        int64   `json:"cost"`        // 实扣代币（用户侧）
	AvgLatency  int     `json:"avg_latency"` // 平均延迟 ms（仅统计成功调用）
	ModelsCount int     `json:"model_count"` // 该组下涉及的模型数（渠道/厂商维度有意义）
	SuccessPct  float64 `json:"success_pct"` // 成功率百分比（保留 1 位）

	// LatencyCnt 内部累加用（加权平均的分母），不下发前端
	LatencyCnt int64 `json:"-"`
}

// AggregateAiUsage 按维度聚合近 N 天的 AI 调用。
// dim 取值 model / channel / vendor；非法值回退 model。
func AggregateAiUsage(dim string, days int) ([]AiUsageRow, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	dim = strings.ToLower(strings.TrimSpace(dim))
	if dim != "model" && dim != "channel" && dim != "vendor" {
		dim = "model"
	}
	since := time.Now().AddDate(0, 0, -days)

	// 第一层：按模型聚合（行数 = 用过的模型数）
	type modelAgg struct {
		ModelID    uint
		Requests   int64
		Success    int64
		Fail       int64
		TokensIn   int64
		TokensOut  int64
		Cost       int64
		LatencySum int64
		LatencyCnt int64
	}
	var aggs []modelAgg
	err := models.DB.Model(&ai.AiCallLog{}).
		Select(`model_id,
		        COUNT(*) as requests,
		        SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success,
		        SUM(CASE WHEN status = 'failed' OR status = 'error' THEN 1 ELSE 0 END) as fail,
		        COALESCE(SUM(tokens_in),0) as tokens_in,
		        COALESCE(SUM(tokens_out),0) as tokens_out,
		        COALESCE(SUM(cost),0) as cost,
		        COALESCE(SUM(CASE WHEN status = 'success' THEN latency_ms ELSE 0 END),0) as latency_sum,
		        SUM(CASE WHEN status = 'success' AND latency_ms > 0 THEN 1 ELSE 0 END) as latency_cnt`).
		Where("created_at >= ? AND model_id > 0", since).
		Group("model_id").
		Scan(&aggs).Error
	if err != nil {
		return nil, err
	}

	// 模型元信息（名称/渠道/厂商）——模型数少，一次全读进内存
	var modelRows []ai.AiModel
	models.DB.Select("id, name, base_url, vendor").Find(&modelRows)
	meta := make(map[uint]ai.AiModel, len(modelRows))
	for i := range modelRows {
		meta[modelRows[i].ID] = modelRows[i]
	}

	out := map[string]*AiUsageRow{}
	modelsPerGroup := map[string]map[uint]bool{}

	for _, a := range aggs {
		// 模型已被删除时仍要计入统计：用空元信息兜底（按其 ID 归组）
		m := meta[a.ModelID]
		var key, name string
		switch dim {
		case "channel":
			key = strings.TrimSpace(m.BaseURL)
			name = key
			if key == "" {
				key, name = "(未配置上游)", "(未配置上游)"
			}
		case "vendor":
			key = strings.TrimSpace(m.Vendor)
			name = key
			if key == "" {
				key, name = "(未归类)", "(未归类)"
			}
		default: // model
			key = itoaUint(a.ModelID)
			name = strings.TrimSpace(m.Name)
			if name == "" {
				name = "模型 #" + key
			}
		}

		row, exists := out[key]
		if !exists {
			row = &AiUsageRow{Key: key, Name: name}
			out[key] = row
			modelsPerGroup[key] = map[uint]bool{}
		}
		modelsPerGroup[key][a.ModelID] = true

		row.Requests += a.Requests
		row.Success += a.Success
		row.Fail += a.Fail
		row.TokensIn += a.TokensIn
		row.TokensOut += a.TokensOut
		row.Cost += a.Cost
		// 平均延迟按「成功且有延迟值的调用」加权，失败调用的 0 不拉低均值
		if a.LatencyCnt > 0 {
			row.AvgLatency = int((int64(row.AvgLatency)*row.LatencyCnt + a.LatencySum) / (row.LatencyCnt + a.LatencyCnt))
			row.LatencyCnt += a.LatencyCnt
		}
	}

	list := make([]AiUsageRow, 0, len(out))
	for k, row := range out {
		row.ModelsCount = len(modelsPerGroup[k])
		if row.Requests > 0 {
			row.SuccessPct = float64(int64(float64(row.Success)/float64(row.Requests)*1000+0.5)) / 10
		}
		list = append(list, *row)
	}
	// 按调用量降序（最活跃的排前面），同量按名称稳定排序
	sort.Slice(list, func(i, j int) bool {
		if list[i].Requests != list[j].Requests {
			return list[i].Requests > list[j].Requests
		}
		return list[i].Name < list[j].Name
	})
	return list, nil
}

func itoaUint(n uint) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
