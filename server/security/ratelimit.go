package security

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

type rateLimiterEntry struct {
	count       int
	windowStart time.Time
}

var (
	rateLimitStore = make(map[string]*rateLimiterEntry)
	rateLimitMu    sync.Mutex
)

// ==================== 身份感知限流上下文 key ====================
// 供 security 中间件与 handlers 共享（middleware 已 import security，故常量放本包）

const (
	CtxKeyIdentity = "apiKeyIdentity" // *models.KeyIdentity：本次请求的身份识别结果
	CtxKeyOK       = "apiKeyOK"       // bool：是否携带并通过校验的有效 Key
	CtxKeySkipIP   = "apiKeySkipIP"   // bool：GET/HEAD 且带有效 Key → 跳过 route 级 IP 限流
)

// ==================== 通用滑动窗口限流（按任意 key） ====================

// KeyRateLimit 滑动窗口限流（内存存储）。
// limitKey: 限流维度（IP / Key 哈希 / bot:IP 等）
// 返回 true 表示放行，false 表示超出配额。
func KeyRateLimit(limitKey string, maxRequests int, windowSecs int) bool {
	now := time.Now()
	window := time.Duration(windowSecs) * time.Second

	rateLimitMu.Lock()
	entry, exists := rateLimitStore[limitKey]
	if !exists || now.Sub(entry.windowStart) > window {
		rateLimitStore[limitKey] = &rateLimiterEntry{
			count:       1,
			windowStart: now,
		}
		rateLimitMu.Unlock()
		return true
	}
	entry.count++
	currentCount := entry.count
	rateLimitMu.Unlock()
	return currentCount <= maxRequests
}

// RateLimit 基于 IP 的简单内存限流（route 级使用）。
// 携带有效 API Key 且请求为 GET/HEAD（非认证敏感接口）时放行 —— 按 Key 配额限流，不再按 IP 计数。
// maxRequests: 窗口内最大请求数；windowSecs: 时间窗口秒数
func RateLimit(maxRequests int, windowSecs int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetBool(CtxKeySkipIP) {
			c.Next()
			return
		}
		ip := c.ClientIP()
		now := time.Now()
		window := time.Duration(windowSecs) * time.Second

		rateLimitMu.Lock()
		entry, exists := rateLimitStore[ip]
		if !exists || now.Sub(entry.windowStart) > window {
			rateLimitStore[ip] = &rateLimiterEntry{
				count:       1,
				windowStart: now,
			}
			rateLimitMu.Unlock()
			c.Next()
			return
		}

		entry.count++
		currentCount := entry.count
		rateLimitMu.Unlock()

		if currentCount > maxRequests {
			utils.Error(c, 429, utils.CodeBadRequest, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}

		c.Next()
	}
}

// ==================== API Key 提取 ====================

// extractAPIKey 从请求头提取 API Key（优先级：X-API-Key > X-NVS-Key > Authorization: Bearer）
func extractAPIKey(c *gin.Context) string {
	if k := strings.TrimSpace(c.GetHeader("X-API-Key")); k != "" {
		return k
	}
	if k := strings.TrimSpace(c.GetHeader("X-NVS-Key")); k != "" {
		return k
	}
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

// ==================== 敏感接口白名单 ====================
// 认证/密码类接口：即使携带有效 Key 也保留 route 级 IP 限流（防撞库/爆破）。
// 这些接口的 route 级 RateLimit 始终按 IP 生效。

var sensitiveAuthPaths = map[string]bool{
	"/api/auth/login":            true,
	"/api/auth/register":         true,
	"/api/auth/forgot-password":  true,
	"/api/auth/reset-password":   true,
	"/api/auth/send-code":        true,
	"/api/auth/verify-code":      true,
	"/api/admin/emergency/unlock": true,
}

func isSensitivePath(path string) bool {
	return sensitiveAuthPaths[path]
}

// ==================== 访问日志与 last_used 节流 ====================

// logThrottle 每分钟每个维度最多写一条日志 / 更新一次 last_used_at，防写放大
type throttleEntry struct {
	minute int64
}

var (
	logThrottleMu   sync.Mutex
	logThrottle     = make(map[string]*throttleEntry) // 访问日志去重
	touchThrottle   = make(map[string]*throttleEntry) // last_used_at 去重
	botLogThrottle  = make(map[string]*throttleEntry) // bot 命中日志去重
)

func currentMinute() int64 { return time.Now().Unix() / 60 }

func shouldThrottle(m map[string]*throttleEntry, key string) bool {
	minute := currentMinute()
	logThrottleMu.Lock()
	defer logThrottleMu.Unlock()
	if e, ok := m[key]; ok && e.minute == minute {
		return false
	}
	m[key] = &throttleEntry{minute: minute}
	return true
}

// recordAccessLog 记录访问日志（带节流：每 key/IP 每分钟最多 1 条）
func recordAccessLog(identity *models.KeyIdentity, method, path, ip, ua string, statusCode int, botReason string) {
	dedupeKey := "anon:" + ip
	if identity != nil && identity.ValidKey {
		dedupeKey = "key:" + identity.KeyHash
	} else if identity != nil && identity.Type == "bot" {
		dedupeKey = "bot:" + ip
	}
	if !shouldThrottle(logThrottle, dedupeKey) {
		return
	}
	logType := "anonymous"
	identityID := uint(0)
	prefix := ""
	if identity != nil {
		logType = identity.Type
		identityID = identity.ID
		prefix = identity.KeyPrefix
	}
	_ = models.RecordAccessLog(&models.ApiAccessLog{
		IdentityType: logType,
		IdentityID:   identityID,
		KeyPrefix:    prefix,
		Method:       method,
		Path:         path,
		StatusCode:   statusCode,
		RequestIP:    ip,
		UserAgent:    truncate(ua, 500),
		BotReason:    botReason,
	})
}

// touchKeyLastUsed 节流更新 Key 的 last_used_at（每 key 每分钟最多写一次）
func touchKeyLastUsed(identity *models.KeyIdentity) {
	if identity == nil || !identity.ValidKey {
		return
	}
	if !shouldThrottle(touchThrottle, identity.KeyHash) {
		return
	}
	now := time.Now()
	if identity.Type == "federated" {
		models.DB.Model(&models.FederatedPeerKey{}).Where("id = ?", identity.ID).
			Update("last_used_at", now)
	} else {
		models.DB.Model(&models.APIKey{}).Where("id = ? AND user_id = ?", identity.ID, identity.UserID).
			Update("last_used_at", now)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ==================== 身份感知限流中间件 ====================

// APIKeyGuard 统一身份感知限流（挂在 /api 公共路由组）：
//   1. 携带有效 API Key → 按 Key 配额限流（Key 维度滑动窗口），识别身份供日志/联邦追溯；
//   2. 无 Key / 无效 Key → 匿名通道：先做爬虫识别（无头/伪装/高频），
//      爬虫走 Bot 配额（远低于匿名），其余走匿名配额（均按 IP 维度）。
// 所有判定只影响限流配额，不改变路由语义（公开接口仍公开）。
func APIKeyGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		path := c.Request.URL.Path
		method := c.Request.Method
		ua := c.Request.UserAgent()

		// 放行：健康检查与建站向导（未安装场景不连库，不能查 Key）
		if path == "/api/health" || path == "/health" || strings.HasPrefix(path, "/api/setup") {
			c.Next()
			return
		}

		cfg := models.LoadAPIRateLimitConfig()
		rawKey := extractAPIKey(c)

		if rawKey != "" {
			ident := models.ResolveAPIKey(rawKey)
			if ident != nil {
				// Key 有效
				if !models.MatchAllowedIP(ident.AllowedIPs, ip) {
					recordAccessLog(ident, method, path, ip, ua, http.StatusForbidden, "ip_not_allowed")
					utils.Error(c, http.StatusForbidden, utils.CodeForbidden, "该 API Key 不允许从当前 IP 访问")
					c.Abort()
					return
				}
				if !KeyRateLimit("key:"+ident.KeyHash, ident.QuotaPerMin, ident.WindowSecs) {
					recordAccessLog(ident, method, path, ip, ua, http.StatusTooManyRequests, "quota_exceeded")
					utils.Error(c, http.StatusTooManyRequests, utils.CodeBadRequest, "API Key 请求超限，请稍后再试")
					c.Abort()
					return
				}
				c.Set(CtxKeyIdentity, ident)
				c.Set(CtxKeyOK, true)
				// 读取类请求（GET/HEAD，非认证敏感接口）跳过 route 级 IP 限流，完全按 Key 配额
				if (method == "GET" || method == "HEAD") && !isSensitivePath(path) {
					c.Set(CtxKeySkipIP, true)
				}
				touchKeyLastUsed(ident)
				recordAccessLog(ident, method, path, ip, ua, 0, "")
				c.Next()
				return
			}
			// Key 无效 → 按匿名处理，记一条日志便于管理员排查伪造
			invalidIdent := &models.KeyIdentity{Type: "anonymous", KeyPrefix: shortPrefix(rawKey), ValidKey: false}
			recordAccessLog(invalidIdent, method, path, ip, ua, 0, "invalid_key")
		}

		// ===== 匿名 / 爬虫通道 =====
		// 0) 已知名头爬虫（AI 爬虫 / SEO 采集 / 管理员自定义）→ 直接 403 阻断，不消耗配额
		if blocked, blockReason := IsBlockedBot(ua); blocked {
			if shouldThrottle(botLogThrottle, "blocked:"+ip) {
				_ = models.RecordAccessLog(&models.ApiAccessLog{
					IdentityType: "blocked", Method: method, Path: path,
					RequestIP: ip, UserAgent: truncate(ua, 500), BotReason: blockReason,
				})
			}
			c.Header("X-NVS-Blocked", blockReason)
			utils.Error(c, http.StatusForbidden, utils.CodeForbidden, "已识别为已知名头爬虫，访问被拒绝")
			c.Abort()
			return
		}

		secFetch := HasSecFetchHeaders(
			c.GetHeader("Sec-Fetch-Site"), c.GetHeader("Sec-Fetch-Mode"), c.GetHeader("Sec-Fetch-Dest"))
		isBot, botReason := ClassifyBot(ua, secFetch, ip)

		identity := &models.KeyIdentity{Type: "anonymous", ValidKey: false}
		quota := cfg.Anonymous
		limitKey := "anon:" + ip
		if isBot {
			identity.Type = "bot"
			identity.BotReason = botReason
			quota = cfg.Bot
			limitKey = "bot:" + ip
		}

		if !KeyRateLimit(limitKey, quota.Req, quota.Window) {
			recordAccessLog(identity, method, path, ip, ua, http.StatusTooManyRequests, botReason)
			utils.Error(c, http.StatusTooManyRequests, utils.CodeBadRequest, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Set(CtxKeyIdentity, identity)

		// bot 命中记录（每 IP 每分钟去重）
		if isBot {
			if shouldThrottle(botLogThrottle, "bot:"+ip) {
				_ = models.RecordAccessLog(&models.ApiAccessLog{
					IdentityType: "bot", Method: method, Path: path,
					RequestIP: ip, UserAgent: truncate(ua, 500), BotReason: botReason,
				})
			}
		}
		c.Next()
	}
}

// shortPrefix 无效 Key 截取前 12 位用于日志识别（不存原文）
func shortPrefix(raw string) string {
	if len(raw) <= 12 {
		return raw
	}
	return raw[:12]
}

// ==================== 限流状态统计与管理（管理员面板） ====================

// RateLimitStats 返回当前内存限流状态摘要（条目数按维度分类）
func RateLimitStats() map[string]int {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	stats := map[string]int{"total": len(rateLimitStore)}
	for k := range rateLimitStore {
		switch {
		case strings.HasPrefix(k, "key:"):
			stats["by_key"]++
		case strings.HasPrefix(k, "bot:"):
			stats["by_bot"]++
		case strings.HasPrefix(k, "anon:"):
			stats["by_anon"]++
		default:
			stats["by_ip"]++
		}
	}
	return stats
}

// ClearRateLimits 清空全部内存限流计数（管理员误判解封；联邦 Key 配额等随之重置）
func ClearRateLimits() {
	rateLimitMu.Lock()
	rateLimitStore = make(map[string]*rateLimiterEntry)
	rateLimitMu.Unlock()
}

// 定期清理过期条目 (后台协程)
func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rateLimitMu.Lock()
			now := time.Now()
			for key, entry := range rateLimitStore {
				if now.Sub(entry.windowStart) > 10*time.Minute {
					delete(rateLimitStore, key)
				}
			}
			rateLimitMu.Unlock()

			// 节流表同样清理
			logThrottleMu.Lock()
			minute := currentMinute()
			for k, e := range logThrottle {
				if minute-e.minute > 10 {
					delete(logThrottle, k)
				}
			}
			for k, e := range touchThrottle {
				if minute-e.minute > 10 {
					delete(touchThrottle, k)
				}
			}
			for k, e := range botLogThrottle {
				if minute-e.minute > 10 {
					delete(botLogThrottle, k)
				}
			}
			logThrottleMu.Unlock()

			// IP 行为统计表
			botIPMu.Lock()
			for k, e := range botIPStore {
				if now.Sub(e.windowStart) > 30*time.Minute {
					delete(botIPStore, k)
				}
			}
			botIPMu.Unlock()
		}
	}()
}
