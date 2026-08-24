package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CSRFGuard CSRF 防护（安全审计：此前仅依赖 SameSite=Lax）。
// 规则：对「携带登录 Cookie 的非安全方法请求（POST/PUT/DELETE/PATCH）」，
// 强制要求 X-Requested-With: XMLHttpRequest 自定义头——跨站表单/脚本无法伪造该头。
// 匿名请求（无 token Cookie，含沙盒跨源 fetch 场景）不校验，不影响公开接口。
func CSRFGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if _, err := c.Cookie("token"); err != nil {
			c.Next() // 未登录：无会话可劫持
			return
		}
		if !strings.EqualFold(c.GetHeader("X-Requested-With"), "XMLHttpRequest") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    3,
				"message": "请求被拒绝：缺少 CSRF 防护头（X-Requested-With）",
			})
			return
		}
		c.Next()
	}
}
