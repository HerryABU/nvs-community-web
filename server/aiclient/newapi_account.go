package aiclient

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// NewAPI 体系的**账户级**额度查询。
//
// 关键区别（很多站长都会踩）：
//   - sk- 开头的 API Key 只能查「这个令牌自己」的用量（且常被设成"无限额度"），
//     永远看不到账户还剩多少钱；
//   - 账户余额必须用**系统访问令牌**（nap_ 开头）+ 用户 ID 调 GET /api/user/self，
//     返回的 quota 是整数单位，需 ÷ quotaPerUnit（默认 500000）换算成美元等值。
//
// 本文件只做「拿账户额度」这一件事，不做猜测：没有访问令牌就明确返回未配置。

// newAPIQuotaPerUnit New API 默认兑换比例：500000 quota = 1 美元等值
const newAPIQuotaPerUnit = 500000

// NewAPIAccount New API 账户额度快照
type NewAPIAccount struct {
	OK        bool           `json:"ok"`
	Quota     float64        `json:"quota"`      // 剩余（已换算）
	UsedQuota float64        `json:"used_quota"` // 累计已用（已换算）
	Currency  string         `json:"currency"`
	Error     string         `json:"error,omitempty"`
	Raw       map[string]any `json:"raw,omitempty"`
}

// QueryNewAPIAccount 用系统访问令牌查账户额度。
// accessToken 必须是 nap_ 这类**非模型调用**令牌；userID 为站点用户数字 ID。
func QueryNewAPIAccount(baseURL, accessToken, userID string, timeout time.Duration) NewAPIAccount {
	out := NewAPIAccount{Currency: "USD"}
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	accessToken = strings.TrimSpace(accessToken)
	userID = strings.TrimSpace(userID)
	if base == "" {
		out.Error = "上游地址未配置"
		return out
	}
	if accessToken == "" {
		out.Error = "未配置系统访问令牌（nap_ 开头）；sk_ 模型密钥查不到账户余额"
		return out
	}
	if userID == "" {
		out.Error = "未配置用户 ID（New-Api-User 头需要）"
		return out
	}

	req, err := http.NewRequest(http.MethodGet, base+"/api/user/self", nil)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("New-Api-User", userID)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		out.Error = "请求失败：" + err.Error()
		return out
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		out.Error = "系统访问令牌无效或无权限（需 nap_ 访问令牌 + 正确用户 ID）"
		return out
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Quota     json.Number `json:"quota"`
			UsedQuota json.Number `json:"used_quota"`
		} `json:"data"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &body)
	if !body.Success && body.Message != "" {
		out.Error = body.Message
		return out
	}
	q, _ := body.Data.Quota.Float64()
	u, _ := body.Data.UsedQuota.Float64()
	if q == 0 && u == 0 {
		out.Error = "返回中没有 quota 字段，可能不是 New API 站点"
		return out
	}
	out.OK = true
	out.Quota = q / newAPIQuotaPerUnit
	out.UsedQuota = u / newAPIQuotaPerUnit
	return out
}

// NewAPILog 一条使用日志（New API /api/log/self）
type NewAPILog struct {
	ModelName string  `json:"model_name"`
	Quota     float64 `json:"quota"`      // 已换算成金额（原值 ÷ 500000）
	QuotaRaw  float64 `json:"quota_raw"`  // 原始 quota 整数
	CreatedAt int64   `json:"created_at"` // Unix 秒
	TokensIn  int64   `json:"tokens_in"`
	TokensOut int64   `json:"tokens_out"`
}

// QueryNewAPIRecentLogs 拉取账户最近的使用日志。
//
// 这是「本次请求花了多少」的**精确**答案来源：
// New API 每次调用都会记一条 log，字段 quota 就是本次扣费（÷500000 = 美元等值），
// 不需要在对话前后各查一次余额做差值，也不会被"无限额度令牌"干扰。
//
// 需要系统访问令牌（nap_）+ 用户 ID；且令牌必须勾选「查看」权限（使用日志/统计）。
func QueryNewAPIRecentLogs(baseURL, accessToken, userID string, pageSize int, timeout time.Duration) ([]NewAPILog, string) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	accessToken = strings.TrimSpace(accessToken)
	userID = strings.TrimSpace(userID)
	if base == "" {
		return nil, "上游地址未配置"
	}
	if accessToken == "" {
		return nil, "未配置系统访问令牌（nap_ 开头）"
	}
	if userID == "" {
		return nil, "未配置用户 ID（New-Api-User 头需要）"
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}

	url := base + "/api/log/self?page_size=" + itoa(pageSize) + "&p=1"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("New-Api-User", userID)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "请求失败：" + err.Error()
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, "令牌无「查看日志」权限：请到中转站权限页勾选 Usage/Logs 读取后换新令牌"
	}

	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    []struct {
			ModelName        string      `json:"model_name"`
			Quota            json.Number `json:"quota"`
			CreatedAt        int64       `json:"created_at"`
			PromptTokens     int64       `json:"prompt_tokens"`
			CompletionTokens int64       `json:"completion_tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, "响应解析失败（可能不是 New API 站点）"
	}
	if !body.Success && body.Message != "" {
		return nil, body.Message
	}
	out := make([]NewAPILog, 0, len(body.Data))
	for _, l := range body.Data {
		q, _ := l.Quota.Float64()
		out = append(out, NewAPILog{
			ModelName: l.ModelName,
			Quota:     q / newAPIQuotaPerUnit,
			QuotaRaw:  q,
			CreatedAt: l.CreatedAt,
			TokensIn:  l.PromptTokens,
			TokensOut: l.CompletionTokens,
		})
	}
	return out, ""
}

// itoa 小工具（避免为一次整数转换引入 strconv 依赖到本文件顶部 import 里）
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
