package honeypot

// ==================== 诱饵画像（管理员可配置） ====================
//
// 诱饵的唯一目的是「像一台真的、运维有点疏忽的机器」。
// 所以这里的每一条内容都必须经得起攻击者反复推敲：
//   1. 不能出现任何自曝标记 —— 变量名里不能有 CANARY / HONEY / DECOY / EXAMPLE，
//      注释里不能有「示例 / 模拟 / 已轮换 / 测试」这类词。攻击者看到任意一个，
//      立刻就知道这是蜜罐，诱饵价值归零。
//   2. 同一次运行内必须**稳定** —— 刷新两次内容就变，等于自己招供。
//      所以金丝雀值持久化在 Setting.DecoyProfile 里，不是每次请求/重启重新随机。
//   3. 内部要自洽 —— APP_URL 的域名、DB_HOST 的网段、运维备注里提到的机器，
//      都应该像同一套环境里出来的。
//
// 金丝雀怎么做到「既能溯源、又不像标记」：
//   值本身是高熵随机串，伪装成一份常见的密钥（默认挂成 Laravel 风格的 APP_KEY），
//   变量名和注释都不提它。站长在外部（仓库 / 论坛 / 社工库）看到这个值，
//   回后台「蜜罐 → 诱饵内容」复制本实例的金丝雀比对即可确认泄露来源。
//   想图省事直接搜 "NVS-CANARY-" 的话，可以在后台把 CanaryStyle 切成 labeled。

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// DecoyProfile 诱饵画像：管理员在后台填，未填项走默认值。
type DecoyProfile struct {
	// AppName 站点品牌名（出现在 APP_NAME 与页面标题里）
	AppName string `json:"app_name"`
	// AppURL 假装的生产域名（不带 https://）
	AppURL string `json:"app_url"`

	// 内网拓扑：攻击者会据此判断「这机器在内网什么位置」
	DBHost    string `json:"db_host"`
	DBPort    string `json:"db_port"`
	DBName    string `json:"db_name"`
	DBUser    string `json:"db_user"`
	RedisHost string `json:"redis_host"`
	RedisPort string `json:"redis_port"`

	// OpsNote 运维备注（越像真人越可信，但不要提蜜罐/测试）
	OpsNote string `json:"ops_note"`
	// DebugTokenOn 是否在诱饵里留一个「临时调试令牌」
	DebugTokenOn bool `json:"debug_token_on"`

	// Canary 本实例的金丝雀值（持久化，重启不变；空 = 首次自动生成后写回）
	Canary string `json:"canary"`
	// CanaryStyle 金丝雀取值风格：stealth（默认，伪装成普通密钥）/ labeled（带可搜索前缀）
	CanaryStyle string `json:"canary_style"`
	// CanaryKey 金丝雀挂在哪一行（默认 APP_KEY）
	CanaryKey string `json:"canary_key"`
}

// DefaultDecoyProfile 默认画像（与旧版内置内容保持同一套「假环境」，只是去掉了自曝标记）
func DefaultDecoyProfile() DecoyProfile {
	return DecoyProfile{
		AppName:      "NVS",
		AppURL:       "app.nvs-prod.cn",
		DBHost:       "10.0.12.31",
		DBPort:       "3306",
		DBName:       "nvs_prod",
		DBUser:       "nvs_ro",
		RedisHost:    "10.0.12.41",
		RedisPort:    "6379",
		OpsNote:      "这台的密钥 3 月轮换过一次，脚本在 /opt/ops/rotate.sh",
		DebugTokenOn: true,
		CanaryStyle:  "stealth",
		CanaryKey:    "APP_KEY",
	}
}

// ParseProfile 从 Setting.DecoyProfile（JSON）解析，空/损坏时回落默认值并补齐缺失项。
func ParseProfile(raw string) DecoyProfile {
	p := DefaultDecoyProfile()
	if strings.TrimSpace(raw) == "" {
		return p
	}
	var in DecoyProfile
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return p
	}
	merge := func(dst *string, v string) {
		if strings.TrimSpace(v) != "" {
			*dst = v
		}
	}
	merge(&p.AppName, in.AppName)
	merge(&p.AppURL, in.AppURL)
	merge(&p.DBHost, in.DBHost)
	merge(&p.DBPort, in.DBPort)
	merge(&p.DBName, in.DBName)
	merge(&p.DBUser, in.DBUser)
	merge(&p.RedisHost, in.RedisHost)
	merge(&p.RedisPort, in.RedisPort)
	merge(&p.OpsNote, in.OpsNote)
	merge(&p.Canary, in.Canary)
	merge(&p.CanaryStyle, in.CanaryStyle)
	merge(&p.CanaryKey, in.CanaryKey)
	// DebugTokenOn 是布尔，用指针语义区分「没填」不现实，交给调用方在保存时兜住
	p.DebugTokenOn = in.DebugTokenOn || raw == "" || strings.Contains(raw, `"debug_token_on":true`)
	if p.Canary == "" {
		p.Canary = newCanary(p.CanaryStyle)
	}
	return p
}

// newCanary 生成金丝雀值。
//
// stealth：伪装成一份常见的密钥（默认形如 Laravel 的 APP_KEY=base64:...），
//
//	值里不含任何可被识别为蜜罐的固定片段 —— 靠后台登记 + 比对来溯源。
//
// labeled：带前缀，便于站长直接在日志/外网全文搜「NVS-CANARY-」，
//
//	代价是攻击者也可能因此识破。默认不用。
func newCanary(style string) string {
	if strings.EqualFold(style, "labeled") {
		return "NVS-CANARY-" + strings.ToUpper(randHex(6))
	}
	return "base64:" + randHex(24)
}

// canaryValue 取出本实例的金丝雀值（空则生成一个，调用方负责持久化）
func (p DecoyProfile) canaryValue() string {
	if p.Canary != "" {
		return p.Canary
	}
	return newCanary(p.CanaryStyle)
}

// canaryLine 金丝雀所在的整行（变量名可配，默认 APP_KEY）
func (p DecoyProfile) canaryLine() string {
	key := strings.TrimSpace(p.CanaryKey)
	if key == "" {
		key = "APP_KEY"
	}
	return key + "=" + p.canaryValue()
}

// newRandHex 供本文件内联使用（避免与 decoy.go 的 randHex 重名混淆，语义相同）
func newRandHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(b)
}

// mustProfileJSON 序列化画像（字段都是字符串/布尔，不会失败）
func mustProfileJSON(p DecoyProfile) string {
	b, err := json.Marshal(p)
	if err != nil {
		return "{}"
	}
	return string(b)
}
