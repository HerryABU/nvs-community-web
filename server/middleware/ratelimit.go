package middleware

import (
	"sync"
	"time"

	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

type rateLimiterEntry struct {
	count    int
	windowStart time.Time
}

var (
	rateLimitStore = make(map[string]*rateLimiterEntry)
	rateLimitMu    sync.Mutex
)

// RateLimit 基于 IP 的简单内存限流
// maxRequests: 窗口内最大请求数
// windowSecs: 时间窗口秒数
func RateLimit(maxRequests int, windowSecs int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		window := time.Duration(windowSecs) * time.Second

		rateLimitMu.Lock()
		entry, exists := rateLimitStore[ip]
		if !exists || now.Sub(entry.windowStart) > window {
			// 新窗口
			rateLimitStore[ip] = &rateLimiterEntry{
				count:    1,
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

// 定期清理过期条目 (后台协程)
func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rateLimitMu.Lock()
			now := time.Now()
			for ip, entry := range rateLimitStore {
				if now.Sub(entry.windowStart) > 10*time.Minute {
					delete(rateLimitStore, ip)
				}
			}
			rateLimitMu.Unlock()
		}
	}()
}
