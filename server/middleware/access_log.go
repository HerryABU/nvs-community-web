package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/honeypot"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// AccessLog 请求日志中间件（176号令合规：上网日志关联用户标识）。
// 全量记录每一条 Web 请求（包括静态资源、404、OPTIONS 预检、鉴权失败等）：
// 时间戳、user_id、method、path、query、请求头、请求体（环境感知脱敏+64KB截断）、
// 响应状态码、响应大小、耗时、客户端 IP、User-Agent。
// 按小时写入 log/requests/<YYYY-MM-DD>/<HH>.log（request 一个文件夹，
// 每天内按小时切分；追加写，重启不覆盖）。
//
// 🔒 脱敏（公安部令第176号合规，见 redact.go）：通过 config.RedactMgr 读取运行时配置。
//   - 生产模式（LOG_REDACT_SENSITIVE=true）：Cookie/Authorization 令牌、body/query 敏感字段
//     写入前自动脱敏（脱敏在落盘之前，日志归档 SHA256 校验不受影响）；
//   - 开发模式（LOG_REDACT_SENSITIVE=false，默认）：保留原始数据，方便调试；
//   - 管理后台「系统设置 → 日志脱敏」可运行时修改，即时生效、无需重启。
//
// 注意：本中间件为全局中间件（在 r.Use 中注册，先于所有路由），
// 在 AuthRequired 之前执行；通过 c.Next() 后再取 user_id，
// 因此能拿到后续鉴权中间件写入的登录态。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 读取请求体（带上限保护，避免大文件如上传占用内存），并完整还原以便后续 handler 复用。
		// ⚠️ 历史 bug：曾用 64KB LimitReader 读取后再整体还原 → 超过 64KB 的 JSON 正文
		// （长章节/长博客）被截断成非法 JSON，导致 handler 的 ShouldBindJSON 报
		// 「请填写章节标题和内容 / 参数错误」（用户明明填了标题却保存失败）。
		// 修复：只缓冲前 maxBodyReadCap 字节用于日志取样，未读完部分保留在原 Body 流中，
		// 用 io.MultiReader 将「已读部分 + 剩余流」拼接还原 —— 任意大小的正文都能完整传给 handler。
		// ⚠️ multipart/form-data（文件上传）请求绝不读取 body——读取再还原会截断大文件，
		// 导致后续 gin 解析 multipart 报「boundary 缺失」（历史 bug：上传 >64KB 必失败）。
		const maxBodyReadCap = 4 * 1024 * 1024 // 4MB：日志取样上限（仅缓冲此大小）
		var bodySnippet string
		ct := c.Request.Header.Get("Content-Type")
		if strings.HasPrefix(strings.ToLower(ct), "multipart/form-data") {
			bodySnippet = "[multipart/form-data 文件上传]"
		} else if c.Request.Body != nil {
			raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyReadCap))
			if err == nil {
				// 完整还原：已读部分 + 未读剩余部分（MultiReader 先读完 raw 再续读原 Body）
				c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(raw), c.Request.Body))
				bodySnippet = string(raw)
			}
		}

		c.Next()

		userID := UserID(c)
		status := c.Writer.Status()
		size := c.Writer.Size()
		if size < 0 {
			size = 0
		}

		headers := map[string]string{}
		for k, v := range c.Request.Header {
			headers[k] = strings.Join(v, "; ")
		}

		// 🔒 环境感知脱敏（176号令合规）：生产环境自动脱敏，开发环境保留原始数据；
		// 脱敏在日志写入之前完成，保证落盘内容即最终合规内容（归档 SHA256 校验不受影响）。
		redactor := NewRedactor(config.RedactMgr.GetConfig())
		if redactor.IsEnabled() {
			headers = redactor.RedactHeaders(headers)
			bodySnippet = redactor.RedactBody(bodySnippet)
		}
		query := c.Request.URL.RawQuery
		if redactor.IsEnabled() {
			query = redactor.RedactQuery(query)
		}
		// 日志只保留前 64KB，防止超长正文撑爆日志
		if len(bodySnippet) > 64*1024 {
			bodySnippet = bodySnippet[:64*1024] + "...(truncated)"
		}

		entry := map[string]interface{}{
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			"user_id":   userID,
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
			"query":     query,
			"ip":        c.ClientIP(),
			"ua":        c.Request.UserAgent(),
			"status":    status,
			"size":      size,
			"latency":   time.Since(start).String(),
			"headers":   headers,
			"body":      bodySnippet,
		}

		if raw, err := json.Marshal(entry); err == nil {
			_ = utils.WriteDailyLog("requests", string(raw))
		}

		// —— 异步写入数据库 Web 访问日志（功能二：前台网络诊断 / 用户分布 / 攻击流量面板）——
		// 排除静态资源（/assets/* 及常见静态扩展名）以降噪控量；
		// bot 判定复用 security 包（仅在无 API Key 时判爬虫，带 Key 走高配额不判）。
		p := c.Request.URL.Path
		if !isStaticAsset(p) {
			ip := c.ClientIP()
			ua := c.Request.UserAgent()
			xff := c.Request.Header.Get("X-Forwarded-For")
			proxyHops := 0
			if xff != "" {
				proxyHops = len(strings.Split(xff, ","))
			}
			hasSecFetch := c.Request.Header.Get("Sec-Fetch-Site") != "" ||
				c.Request.Header.Get("Sec-Fetch-Mode") != "" ||
				c.Request.Header.Get("Sec-Fetch-Dest") != ""

			identityType := "anonymous"
			isBot := false
			botReason := ""
			uaFamily := classifyUAFamily(ua)
			if userID > 0 {
				identityType = "user"
			}
			if c.Request.Header.Get("X-API-Key") != "" ||
				strings.HasPrefix(c.Request.Header.Get("Authorization"), "Bearer nvs_") {
				identityType = "key"
			} else {
				if blocked, reason := security.IsBlockedBot(ua); blocked {
					isBot, botReason, uaFamily = true, reason, "bot"
				} else if b, reason := security.ClassifyBot(ua, hasSecFetch, ip); b {
					isBot, botReason, uaFamily = true, reason, "bot"
				}
			}
			query := c.Request.URL.RawQuery
			if len(query) > 512 {
				query = query[:512]
			}
			// 日志字段策略（后台可勾选）：关掉的字段一律不落库——
			// UA / Referer 之外，X-Forwarded-For 默认不记（含代理链路与内网信息）。
			hpSet := honeypot.Current()
			if !hpSet.RecordUA {
				ua, uaFamily = "", "unknown"
			}
			if !hpSet.RecordXFF {
				xff, proxyHops = "", 0
			}
			if !hpSet.RecordQuery {
				query = ""
			}
			models.EnqueueWebAccess(&models.WebAccessLog{
				UserID:        userID,
				IdentityType:  identityType,
				UAFamily:      uaFamily,
				Method:        c.Request.Method,
				Path:          p,
				Query:         query,
				IP:            ip,
				XForwardedFor: xff,
				ProxyHops:     proxyHops,
				UA:            ua,
				StatusCode:    status,
				Size:          size,
				LatencyMs:     int(time.Since(start).Milliseconds()),
				IsBot:         isBot,
				BotReason:     botReason,
				CreatedAt:     time.Now(),
			})
		}
	}
}

// classifyUAFamily 粗略归类 User-Agent 家族（用于诊断面板聚合；与 security 包 bot 判定互补）。
func classifyUAFamily(ua string) string {
	lower := strings.ToLower(ua)
	if lower == "" {
		return "unknown"
	}
	// 工具 / 脚本 / 下载库
	toolKw := []string{"curl/", "wget", "python", "go-http-client", "node-fetch", "axios",
		"okhttp", "java/", "libwww", "httpclient", "aiohttp", "httpx", "scrapy", "postmanruntime"}
	for _, kw := range toolKw {
		if strings.Contains(lower, kw) {
			return "tool"
		}
	}
	// 移动端浏览器
	if strings.Contains(lower, "android") || strings.Contains(lower, "iphone") ||
		strings.Contains(lower, "ipad") || strings.Contains(lower, "mobile") {
		return "mobile"
	}
	// 桌面浏览器
	browserKw := []string{"chrome", "firefox", "safari", "edg/", "opera", "msie", "trident", "webkit"}
	for _, kw := range browserKw {
		if strings.Contains(lower, kw) {
			return "browser"
		}
	}
	return "unknown"
}
