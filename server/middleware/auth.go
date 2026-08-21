package middleware

import (
	"net/http"

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
	c.SetCookie(
		"token",
		token,
		72*3600, // 72 hours
		"/",
		"",
		false, // Secure: true in production
		true,  // HttpOnly
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
		false, // Secure: true in production
		true,  // HttpOnly
	)
	c.SetSameSite(http.SameSiteLaxMode)
}
