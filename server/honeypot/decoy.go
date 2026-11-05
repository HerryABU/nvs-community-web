package honeypot

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ===== (原 decoy.go) =====

// ==================== 诱饵内容 ====================
// 原则（docs/honeypot-design.md）：凭据全部一次性生成、与真实系统零复用；
// 内容要「像运维疏忽泄露」，但不能诱导攻击者打真实服务。
// 每份 .env 带唯一 canary：一旦该字符串在别处出现，就能确认泄露来源。

// Decoy 一个诱饵
type Decoy struct {
	ID      string // env / git / admin / phpinfo / actuator / wp / backup
	Path    string
	Risk    int // 基础风险分（越高越像真实攻击）
	Content string
	Headers map[string]string
}

// randHex 随机十六进制（一次性凭据）
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(b)
}

// envDecoy 仿真 .env：JWT 密钥、DB/Redis 占位、运维备注。
//
// ⚠️ 这里**绝不能**出现任何自曝标记：
// 早期版本写了 `# jwt（泄漏示例 / 已轮换）`，还给金丝雀单独开了 `CANARY=` 一行 ——
// 攻击者扫到 .env 第一眼就知道是蜜罐，整份诱饵白做。
// 现在金丝雀伪装成一份普通密钥（默认挂 APP_KEY），
// 具体取值与文案由管理员在后台「蜜罐 → 诱饵内容」里配置（DecoyProfile）。
func envDecoy(p DecoyProfile) string {
	var b strings.Builder
	fmt.Fprintf(&b, `# ==== production ====
APP_NAME="%s"
APP_ENV=production
APP_DEBUG=false
APP_URL=https://%s

%s
JWT_SECRET=%s
JWT_EXPIRE_HOURS=72

# database
DB_CONNECTION=mysql
DB_HOST=%s
DB_PORT=%s
DB_DATABASE=%s
DB_USERNAME=%s
DB_PASSWORD=%s

# redis
REDIS_HOST=%s
REDIS_PORT=%s
REDIS_PASSWORD=%s
`,
		p.AppName, p.AppURL,
		p.canaryLine(), randHex(24),
		p.DBHost, p.DBPort, p.DBName, p.DBUser, randHex(12),
		p.RedisHost, p.RedisPort, randHex(12),
	)
	if p.OpsNote != "" {
		fmt.Fprintf(&b, "\n# %s\n", p.OpsNote)
	}
	if p.DebugTokenOn {
		fmt.Fprintf(&b, "DEBUG_TOKEN=%s\n", randHex(16))
	}
	return b.String()
}

// gitConfigDecoy 仿真 .git/config
func gitConfigDecoy() string {
	return `[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
[remote "origin"]
	url = git@gitlab.internal:nvs/nvs-web.git
	fetch = +refs/heads/*:refs/remotes/origin/*
[branch "main"]
	remote = origin
	merge = refs/heads/main
`
}

// adminDecoy 仿真后台登录页（不含任何可提交成功的端点）
func adminDecoy() string {
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<title>系统登录</title>
<style>body{font-family:system-ui;background:#0f172a;color:#e2e8f0;display:flex;align-items:center;justify-content:center;height:100vh;margin:0}
.card{background:#1e293b;padding:32px 40px;border-radius:12px;width:320px}
input{width:100%;padding:10px;margin:8px 0;border-radius:6px;border:1px solid #334155;background:#0f172a;color:#e2e8f0}
button{width:100%;padding:10px;margin-top:12px;border:0;border-radius:6px;background:#3b82f6;color:#fff;font-size:15px}
.hint{font-size:12px;color:#64748b;margin-top:12px}</style></head>
<body><div class="card"><h3>运维控制台</h3>
<form method="post" action="/admin/login"><input name="username" placeholder="账号" autocomplete="off">
<input name="password" type="password" placeholder="密码"><button>登录</button></form>
<div class="hint">仅限内网访问 · 登录行为已记录</div></div></body></html>`
}

// phpinfoDecoy 仿真 phpinfo（暴露「这是一台机器」的错觉）
func phpinfoDecoy() string {
	return `<html><head><title>phpinfo()</title></head><body>
<h1>PHP Version 8.2.18</h1>
<table><tr><td>System</td><td>Linux 5.15.0-101-generic x86_64</td></tr>
<tr><td>Server API</td><td>FPM/FastCGI</td></tr>
<tr><td>DOCUMENT_ROOT</td><td>/var/www/nvs/public</td></tr>
<tr><td>Server Software</td><td>nginx/1.24.0</td></tr></table>
</body></html>`
}

// actuatorDecoy 仿真 Spring Actuator
func actuatorDecoy() string {
	return `{"activeProfiles":["prod"],"propertySources":[{"name":"systemEnvironment","properties":{"DB_PASSWORD":{"value":"******"},"JWT_SECRET":{"value":"******"}}}]}`
}

// backupDecoy 仿真误放目录
func backupDecoy() string {
	return `Index of /backup
  ../ 
  db_2026-03-11.sql.gz
  nvs_uploads.tar.gz
  ops_notes.txt
`
}

// robotsDecoy 仿真「不想被收录」的路径清单（本身就是资产清单）
func robotsDecoy() string {
	return `User-agent: *
Disallow: /admin
Disallow: /internal
Disallow: /backup
Disallow: /api/internal
`
}

// allDecoys 全量诱饵（按路径索引）
func allDecoys(p DecoyProfile) map[string]Decoy {
	list := []Decoy{
		{ID: "env", Path: "/.env", Risk: 70, Content: envDecoy(p),
			Headers: map[string]string{"Content-Type": "text/plain; charset=utf-8"}},
		{ID: "git", Path: "/.git/config", Risk: 65, Content: gitConfigDecoy(),
			Headers: map[string]string{"Content-Type": "text/plain; charset=utf-8"}},
		{ID: "admin", Path: "/admin.php", Risk: 55, Content: adminDecoy(),
			Headers: map[string]string{"Content-Type": "text/html; charset=utf-8"}},
		{ID: "phpinfo", Path: "/phpinfo.php", Risk: 60, Content: phpinfoDecoy(),
			Headers: map[string]string{"Content-Type": "text/html; charset=utf-8"}},
		{ID: "actuator", Path: "/actuator/env", Risk: 75, Content: actuatorDecoy(),
			Headers: map[string]string{"Content-Type": "application/json"}},
		{ID: "wp", Path: "/wp-login.php", Risk: 45, Content: adminDecoy(),
			Headers: map[string]string{"Content-Type": "text/html; charset=utf-8"}},
		{ID: "backup", Path: "/backup/", Risk: 68, Content: backupDecoy(),
			Headers: map[string]string{"Content-Type": "text/plain; charset=utf-8"}},
		{ID: "robots", Path: "/robots-disallow.txt", Risk: 30, Content: robotsDecoy(),
			Headers: map[string]string{"Content-Type": "text/plain; charset=utf-8"}},
	}
	out := make(map[string]Decoy, len(list))
	for _, d := range list {
		out[d.Path] = d
	}
	return out
}

// decoysForLevel 按等级返回要挂的诱饵：低=1 个，中=4 个，高=全量
func decoysForLevel(level string, p DecoyProfile) []Decoy {
	all := allDecoys(p)
	pick := func(ids ...string) []Decoy {
		out := make([]Decoy, 0, len(ids))
		for _, id := range ids {
			for _, d := range all {
				if d.ID == id {
					out = append(out, d)
					break
				}
			}
		}
		return out
	}
	switch NormalizeLevel(level) {
	case LevelLow:
		return pick("env")
	case LevelMedium:
		return pick("env", "git", "admin", "robots")
	case LevelHigh:
		out := make([]Decoy, 0, len(all))
		for _, d := range all {
			out = append(out, d)
		}
		return out
	default:
		return nil
	}
}

// slowDelay 高等级命中后的延迟（毫秒），拖住自动化但不至于让自己超时栈溢出
func slowDelay(set Setting) time.Duration {
	ms := set.DelayMs
	if ms <= 0 {
		return 0
	}
	if ms > 5000 {
		ms = 5000
	}
	return time.Duration(ms) * time.Millisecond
}

// ==================== 自定义诱饵（站长可编辑内容） ====================
// 站长在后台填写的 JSON 数组；用途有二：
//   1. 覆盖内置诱饵（env/git/admin…）的返回内容 / 风险 / 头；
//   2. 为「额外诱饵路径」（extraBaitPaths，启动即注册路由）提供内容。
// 解析失败一律回落到默认：绝不因配置脏而让蜜罐静默失效。

// CustomDecoy 单条自定义诱饵
type CustomDecoy struct {
	ID      string            `json:"id"`
	Path    string            `json:"path"`
	Content string            `json:"content"`
	Risk    int               `json:"risk"`
	Headers map[string]string `json:"headers"`
}

// extraBaitPaths 额外诱饵路径：启动即注册静态路由（gin 路由表必须静态）。
// 未自定义时返回可信的「误泄露」默认内容；站长可在后台为任意一条填自定义内容。
var extraBaitPaths = []string{
	"/.git/credentials", "/.ssh/id_rsa", "/config.yml",
	"/.npmrc", "/id_rsa", "/secret.txt", "/.htpasswd", "/server.key",
	"/db_backup.sql", "/.env.bak", "/docker-compose.override.yml", "/.gitignore",
}

// parseCustomDecoys 解析 Setting 里的自定义诱饵（脏数据回落空）
func parseCustomDecoys(set Setting) []CustomDecoy {
	if strings.TrimSpace(set.CustomDecoys) == "" {
		return nil
	}
	var out []CustomDecoy
	if err := json.Unmarshal([]byte(set.CustomDecoys), &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}

// customByPath 按路径取自定义诱饵（覆盖优先）；CustomDecoys 为空时直接返回 nil
func customByPath(set Setting, path string) *CustomDecoy {
	if strings.TrimSpace(set.CustomDecoys) == "" {
		return nil
	}
	list := parseCustomDecoys(set)
	for i := range list {
		if list[i].Path == path {
			return &list[i]
		}
	}
	return nil
}

// defaultBaitContent 为未自定义的新诱饵路径生成可信泄露内容（按扩展名挑模板）
func defaultBaitContent(path string, p DecoyProfile) string {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "id_rsa") || strings.Contains(lower, "server.key") || strings.Contains(lower, ".pem"):
		return "-----BEGIN OPENSSH PRIVATE KEY-----\n" + randHex(48) + "\n-----END OPENSSH PRIVATE KEY-----\n"
	case strings.Contains(lower, "credentials") || strings.Contains(lower, ".npmrc") || strings.Contains(lower, ".htpasswd"):
		return "# 内部凭据（请勿提交）\n" + randHex(16) + ":" + randHex(12) + "\n"
	case strings.Contains(lower, "aws"):
		return "[default]\naws_access_key_id = AKIA" + strings.ToUpper(randHex(8)) +
			"\naws_secret_access_key = " + randHex(24) + "\n"
	case strings.Contains(lower, "sql") || strings.Contains(lower, "db_backup"):
		return "-- mysqldump 残留\nINSERT INTO users VALUES (1,'admin','" + randHex(12) + "',now());\n"
	case strings.Contains(lower, "yml") || strings.Contains(lower, "yaml") || strings.Contains(lower, "config"):
		return "APP_ENV=production\nDB_HOST=10.0.12.31\nDB_PASSWORD=" + randHex(12) + "\nJWT_SECRET=" + randHex(24) + "\n"
	default:
		return "# 部署参数（临时）\n" + p.canaryLine() + "\nDB_HOST=" + p.DBHost + "\nDB_PASSWORD=" + randHex(12) + "\n"
	}
}

// ===== 合并自 decoy_profile.go =====

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
