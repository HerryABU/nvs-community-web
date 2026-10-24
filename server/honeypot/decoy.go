package honeypot

// ==================== 诱饵内容 ====================
// 原则（docs/honeypot-design.md）：凭据全部一次性生成、与真实系统零复用；
// 内容要「像运维疏忽泄露」，但不能诱导攻击者打真实服务。
// 每份 .env 带唯一 canary：一旦该字符串在别处出现，就能确认泄露来源。

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

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

// canary 本实例的唯一金丝雀
func canary() string {
	return "NVS-CANARY-" + strings.ToUpper(randHex(6))
}

// envDecoy 仿真 .env：JWT 密钥、DB/Redis 占位、运维备注
func envDecoy() string {
	return fmt.Sprintf(`# ==== production ====
APP_NAME="NVS"
APP_ENV=production
APP_DEBUG=false
APP_URL=https://%s.example.com

# jwt（泄漏示例 / 已轮换）
JWT_SECRET=%s
JWT_EXPIRE_HOURS=72

# database
DB_CONNECTION=mysql
DB_HOST=10.0.12.31
DB_PORT=3306
DB_DATABASE=nvs_prod
DB_USERNAME=nvs_ro
DB_PASSWORD=%s

# redis
REDIS_HOST=10.0.12.41
REDIS_PORT=6379
REDIS_PASSWORD=%s

# 运维备注：这台的密钥 3 月轮换过一次，脚本在 /opt/ops/rotate.sh
# TODO 2026-03-xx：临时开的调试接口记得关掉
DEBUG_TOKEN=%s
CANARY=%s
`,
		"nvs", randHex(24), randHex(12), randHex(12), randHex(16), canary())
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
func allDecoys() map[string]Decoy {
	list := []Decoy{
		{ID: "env", Path: "/.env", Risk: 70, Content: envDecoy(),
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
func decoysForLevel(level string) []Decoy {
	all := allDecoys()
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
