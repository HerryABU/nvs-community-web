package middleware

import (
	"net/http"

	"nvs-server/config"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie("token")
		if err != nil {
			utils.Unauthorized(c, "未登录")
			c.Abort()
			return
		}

		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			utils.Unauthorized(c, "Token 无效或已过期")
			c.Abort()
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUserEmail, claims.Email)
		c.Set(CtxUserRole, claims.Role)
		c.Next()
	}
}

// OptionalAuth 可选登录中间件：解析 token 并设置用户上下文（user id/email/role），
// 解析失败不拦截请求。用于公开路由（如论坛详情）识别登录态，
// 使管理员/创建者等权限判断在公开接口中也能生效（如管理员进论坛免密码，176号令合规）。
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokenString, err := c.Cookie("token"); err == nil && tokenString != "" {
			if claims, perr := utils.ParseToken(tokenString); perr == nil {
				c.Set(CtxUserID, claims.UserID)
				c.Set(CtxUserEmail, claims.Email)
				c.Set(CtxUserRole, claims.Role)
			}
		}
		c.Next()
	}
}

// AuthorRequired 要求作者角色（author / vip_author）
func AuthorRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleStr := UserRole(c)
		if roleStr == "" {
			utils.Forbidden(c, "需要作者权限")
			c.Abort()
			return
		}

		if roleStr != "author" && roleStr != "vip_author" && roleStr != "admin" {
			utils.Forbidden(c, "需要作者权限")
			c.Abort()
			return
		}
		c.Next()
	}
}

// SetTokenCookie 设置 JWT Cookie
func SetTokenCookie(c *gin.Context, token string) {
	// 安全审计 L3：Secure 由 .env 的 COOKIE_SECURE 控制（生产 HTTPS 下应开启，防明文截获）
	c.SetCookie(
		"token",
		token,
		72*3600, // 72 hours
		"/",
		"",
		config.ForceCookieSecure, // Secure: 生产 HTTPS 下由 COOKIE_SECURE=true 开启
		true,                     // HttpOnly
	)
	c.SetSameSite(http.SameSiteLaxMode)
}

// ClearTokenCookie 清除 JWT Cookie
func ClearTokenCookie(c *gin.Context) {
	c.SetCookie(
		"token",
		"",
		-1,
		"/",
		"",
		config.ForceCookieSecure,
		true, // HttpOnly
	)
	c.SetSameSite(http.SameSiteLaxMode)
}
