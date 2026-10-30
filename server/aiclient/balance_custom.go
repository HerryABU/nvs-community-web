package aiclient

import (
	"strings"
	"time"
)

// ==================== 自定义余额规则（陌生渠道兜底） ====================
//
// 现实：上游五花八门，官方（DeepSeek 有专用接口）、New API 中转站（quota÷500000）、
// 自建代理（可能只回一个裸数字）各不相同。与其在代码里不断加 if，
// 不如让站长自己描述「怎么取、怎么算」：
//
//	URL      https://api.example.com/stat/credit
//	字段路径 data.credit.available      （支持 a.b.c 逐层取值）
//	除以     500000                      （quota 类单位换算）
//	乘以     1                           （多数是 1）
//	币种     USD
//
// 计算规则固定为 **(原始值 ÷ 除数) × 乘数**，够覆盖真实场景，且不会变成任意表达式注入。

// CustomBalanceSpec 自定义余额取值规则
type CustomBalanceSpec struct {
	URL        string  `json:"url"`        // 余额接口完整地址（含路径）
	Field      string  `json:"field"`      // 字段路径，如 data.quota；留空表示响应本身就是数字
	Divisor    float64 `json:"divisor"`    // 除数（0 视为 1）
	Multiplier float64 `json:"multiplier"` // 乘数（0 视为 1）
	Currency   string  `json:"currency"`   // 币种/单位，如 USD / CNY / 积分
}

// QueryCustomBalance 按站长配置的规则取余额。
func QueryCustomBalance(spec CustomBalanceSpec, apiKey, tokenUserID string, timeout time.Duration) ChannelBalance {
	res := ChannelBalance{FetchedAt: time.Now().Format(time.RFC3339), Provider: "custom"}
	url := strings.TrimSpace(spec.URL)
	if url == "" {
		res.Error = "未配置余额接口地址"
		return res
	}
	body, status, err := httpGetJSONWithUser(url, apiKey, tokenUserID, timeout)
	if err != nil {
		res.Error = "请求失败：" + err.Error()
		return res
	}
	if status == 401 || status == 403 {
		res.Error = "密钥无效或无权限：上游返回 " + itoa(status)
		return res
	}
	if status < 200 || status >= 300 {
		res.Error = "上游返回 HTTP " + itoa(status)
		return res
	}

	raw, ok := pickPath(body, spec.Field)
	if !ok {
		res.Error = "响应里找不到字段 " + spec.Field
		return res
	}
	val := numOf(raw)

	div := spec.Divisor
	if div == 0 {
		div = 1
	}
	mul := spec.Multiplier
	if mul == 0 {
		mul = 1
	}
	res.Supported = true
	res.TotalRemaining = round4Float(val / div * mul)
	res.Currency = strings.ToUpper(strings.TrimSpace(spec.Currency))
	if res.Currency == "" {
		res.Currency = "—"
	}
	return res
}

// pickPath 按 "a.b.c" 逐层取值；支持数组下标 a.0.b。
func pickPath(root map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}
	var cur any = root
	for _, seg := range strings.Split(path, ".") {
		seg = strings.TrimSpace(seg)
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			idx := 0
			for _, ch := range seg {
				if ch < '0' || ch > '9' {
					return nil, false
				}
				idx = idx*10 + int(ch-'0')
			}
			if idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// round4Float 保留 4 位小数（余额单位可能很小）
func round4Float(v float64) float64 {
	return float64(int64(v*10000+0.5)) / 10000
}
