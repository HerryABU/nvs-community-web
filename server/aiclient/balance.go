package aiclient

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// ════════════════ L5 级能力：上游渠道余额 / 消费查询（用于 API 对账） ════════════════
// 仅在「OpenAI 兼容」渠道上具备标准余额端点；Anthropic 等无公开余额 API 时优雅返回 unsupported。
// 余额数据归站长做成本核算 / 渠道对账使用，与「用户在站内的 token 余额」是两回事。

// ChannelBalance 上游渠道余额 / 消费快照
type ChannelBalance struct {
	Provider       string         `json:"provider"`         // openai / azure / anthropic / unknown
	TotalGranted   float64        `json:"total_granted"`    // 总授予 / 充值
	TotalUsed      float64        `json:"total_used"`       // 已消费
	TotalRemaining float64        `json:"total_remaining"`  // 剩余
	Currency       string         `json:"currency"`         // 币种，如 USD
	Period         string         `json:"period,omitempty"` // 计费周期说明
	Raw            map[string]any `json:"raw,omitempty"`    // 上游原始响应（便于排查）
	FetchedAt      string         `json:"fetched_at"`       // 查询时间（RFC3339）
	Supported      bool           `json:"supported"`        // 该渠道是否支持余额查询
	Error          string         `json:"error,omitempty"`  // 查询失败原因
}

// creditGrantsFlat OpenAI 旧版 /dashboard/billing/credit_grants 结构
type creditGrantsFlat struct {
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
	TotalAvailable float64 `json:"total_available"`
	Currency       string  `json:"currency"`
}

// creditGrantsV2 新版（嵌套 value/currency）
type creditGrantsV2 struct {
	TotalGranted struct {
		Value    float64 `json:"value"`
		Currency string  `json:"currency"`
	} `json:"total_granted"`
	TotalUsed struct {
		Value    float64 `json:"value"`
		Currency string  `json:"currency"`
	} `json:"total_used"`
	TotalAvailable struct {
		Value    float64 `json:"value"`
		Currency string  `json:"currency"`
	} `json:"total_available"`
}

// detectProvider 由 providerHint / baseURL 推断上游类型
func detectProvider(baseURL, hint string) string {
	h := strings.ToLower(strings.TrimSpace(hint))
	if h != "" {
		switch {
		case strings.Contains(h, "anthropic"), strings.Contains(h, "claude"):
			return "anthropic"
		case strings.Contains(h, "azure"):
			return "azure"
		case strings.Contains(h, "openai"):
			return "openai"
		}
	}
	u := strings.ToLower(baseURL)
	switch {
	case strings.Contains(u, "anthropic"), strings.Contains(u, "claude"):
		return "anthropic"
	case strings.Contains(u, "azure"):
		return "azure"
	default:
		return "openai"
	}
}

// QueryChannelBalance 查询上游渠道余额 / 消费。
// baseURL 为上游地址（不含 /chat/completions），apiKey 为上游密钥，providerHint 可选。
func QueryChannelBalance(baseURL, apiKey, providerHint string) ChannelBalance {
	res := ChannelBalance{
		Provider:  detectProvider(baseURL, providerHint),
		FetchedAt: time.Now().Format(time.RFC3339),
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		res.Error = "上游地址未配置"
		return res
	}

	switch res.Provider {
	case "anthropic":
		// Anthropic 不提供公开余额查询 API
		res.Supported = false
		res.Error = "Anthropic 不提供公开余额查询接口"
		return res
	case "azure":
		// Azure OpenAI 的用量需走 Azure 门户 / 订阅 API，资源密钥无法直接查
		res.Supported = false
		res.Error = "Azure OpenAI 余额需通过 Azure 订阅查询，渠道密钥无法拉取"
		return res
	}

	// OpenAI 兼容：尝试 /dashboard/billing/credit_grants
	url := baseURL + "/dashboard/billing/credit_grants"
	body, status, err := httpGetJSON(url, apiKey, 10*time.Second)
	if err != nil {
		res.Error = "请求失败：" + err.Error()
		return res
	}
	if status == 401 || status == 403 {
		res.Supported = false
		res.Error = "密钥无效或无权限（" + http.StatusText(status) + "）"
		return res
	}
	if status == 404 {
		res.Supported = false
		res.Error = "该渠道未提供余额查询端点（404）"
		return res
	}
	if status < 200 || status >= 300 {
		res.Supported = false
		res.Error = "上游返回 " + http.StatusText(status)
		return res
	}

	res.Supported = true
	res.Raw = body

	// 兼容两种结构
	var flat creditGrantsFlat
	if json.Unmarshal(rawOf(body), &flat) == nil && (flat.TotalGranted != 0 || flat.TotalUsed != 0 || flat.TotalAvailable != 0) {
		res.TotalGranted = flat.TotalGranted
		res.TotalUsed = flat.TotalUsed
		res.TotalRemaining = flat.TotalAvailable
		res.Currency = flat.Currency
	} else {
		var v2 creditGrantsV2
		if json.Unmarshal(rawOf(body), &v2) == nil {
			res.TotalGranted = v2.TotalGranted.Value
			res.TotalUsed = v2.TotalUsed.Value
			res.TotalRemaining = v2.TotalAvailable.Value
			res.Currency = firstNonEmpty(v2.TotalGranted.Currency, v2.TotalUsed.Currency, v2.TotalAvailable.Currency)
		}
	}
	if res.Currency == "" {
		res.Currency = "USD"
	}
	return res
}

// ════════════════ 真实连通性探针（F2：确保真的能连上内容） ════════════════

// ChannelProbe 上游真实连通性探针结果
type ChannelProbe struct {
	Reachable  bool   `json:"reachable"`
	LatencyMs  int64  `json:"latency_ms"`
	StatusCode int    `json:"status_code"`
	Models     int    `json:"models,omitempty"` // GET /models 返回的模型数（若有）
	Error      string `json:"error,omitempty"`
}

// ProbeChannel 对上游做一次真实请求验证连通性：
// 优先 GET /models（不消耗生成额度即可验证密钥 + 可达性），失败再退化为一次 1-token 补全。
func ProbeChannel(baseURL, apiKey, modelID string) ChannelProbe {
	p := ChannelProbe{}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		p.Error = "上游地址未配置"
		return p
	}
	start := time.Now()

	// 1) GET /models
	body, status, err := httpGetJSON(baseURL+"/models", apiKey, 12*time.Second)
	if err == nil && status >= 200 && status < 300 {
		p.Reachable = true
		p.StatusCode = status
		p.LatencyMs = time.Since(start).Milliseconds()
		if arr, ok := body["data"].([]any); ok {
			p.Models = len(arr)
		}
		return p
	}

	// 2) 退化：1-token 补全（需要 modelID）
	if modelID != "" {
		c := NewClient(baseURL, apiKey, 15*time.Second)
		_, _, _, e := c.Complete(ChatRequest{
			Model:       modelID,
			Messages:    []Message{{Role: "user", Content: "ping"}},
			MaxTokens:   1,
			Temperature: 0,
		})
		p.LatencyMs = time.Since(start).Milliseconds()
		if e == nil {
			p.Reachable = true
			p.StatusCode = 200
			return p
		}
		p.Error = e.Error()
	} else if err != nil {
		p.Error = err.Error()
	} else {
		p.StatusCode = status
		p.Error = "上游返回 " + http.StatusText(status)
	}
	return p
}

// ───────────────────────── 内部工具 ─────────────────────────

// httpGetJSON 发起带 Bearer 的 GET，返回解析后的 JSON map、状态码、错误
func httpGetJSON(url, apiKey string, timeout time.Duration) (map[string]any, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: timeout}
	resp, derr := client.Do(req)
	if derr != nil {
		return nil, 0, derr
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out, resp.StatusCode, nil
}

func rawOf(m map[string]any) []byte {
	b, _ := json.Marshal(m)
	return b
}
