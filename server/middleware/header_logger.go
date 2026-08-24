package middleware

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

// HeaderLogger 打印所有请求头到日志，方便调试和审计。
// 敏感头（如 Cookie、Authorization）的值会被部分遮蔽。
// 静态资源（/assets/* 及带扩展名的静态文件）只打一行摘要，不展开请求头——
// 避免每个 js/css 刷十几行日志（浏览器并发加载十几个资源时日志爆炸）。
func HeaderLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if isStaticAsset(p) {
			log.Printf("[REQ] %s %s (static)", c.Request.Method, p)
			c.Next()
			return
		}

		log.Printf("[REQ] %s %s", c.Request.Method, c.Request.URL.String())
		log.Println("[REQ] ── Headers ──")

		for key, values := range c.Request.Header {
			for _, v := range values {
				if isSensitive(key) {
					v = maskValue(v)
				}
				log.Printf("[REQ]   %s: %s", key, v)
			}
		}

		log.Println("[REQ] ── End ──")
		c.Next()
	}
}

// isStaticAsset 判断是否为静态资源请求（前缀 assets/ 或带常见静态扩展名）。
// 仅影响终端调试日志的详细度，不影响访问日志（AccessLog）留存与合规。
func isStaticAsset(p string) bool {
	if strings.HasPrefix(p, "/assets/") {
		return true
	}
	dot := strings.LastIndex(p, ".")
	if dot < 0 || strings.Contains(p[dot:], "/") {
		return false
	}
	switch strings.ToLower(p[dot:]) {
	case ".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico",
		".woff", ".woff2", ".ttf", ".eot", ".map", ".wasm", ".mp3", ".mp4", ".webm":
		return true
	}
	return false
}

// isSensitive 判断是否为敏感请求头
func isSensitive(key string) bool {
	lower := strings.ToLower(key)
	switch lower {
	case "cookie", "authorization", "x-api-key", "x-auth-token":
		return true
	default:
		return false
	}
}

// maskValue 对敏感值只显示前4个和后4个字符，中间用 *** 代替
func maskValue(v string) string {
	if len(v) <= 8 {
		return "***"
	}
	return v[:4] + "***" + v[len(v)-4:]
}
