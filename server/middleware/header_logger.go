package middleware

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

// HeaderLogger 打印所有请求头到日志，方便调试和审计。
// 敏感头（如 Cookie、Authorization）的值会被部分遮蔽。
func HeaderLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
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
