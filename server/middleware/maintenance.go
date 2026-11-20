package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"nvs-server/maintenance"
)

// maintenanceMessage 前端 response interceptor 也认这段文案，改这里要同步改 api/index.ts
const maintenanceMessage = "上传功能正在维护中，请稍后"

// MaintenanceWriteGuard 热迁移期间拦截所有写操作（HTTP 423 Locked）。
//
// 白名单：登录/刷新/验证码（否则用户无法建立会话看到提示）、
// /api/admin/db/*（迁移任务本身与进度查询）、/api/maintenance（前端轮询状态）。
func MaintenanceWriteGuard() gin.HandlerFunc {
	writeMethods := map[string]bool{"POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	return func(c *gin.Context) {
		if !maintenance.Enabled() || !writeMethods[c.Request.Method] {
			c.Next()
			return
		}
		p := c.Request.URL.Path
		for _, prefix := range []string{"/api/admin/db/", "/api/auth/login", "/api/auth/refresh", "/api/auth/captcha"} {
			if strings.HasPrefix(p, prefix) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusLocked, gin.H{
			"code": 423, "message": maintenanceMessage, "data": nil,
		})
	}
}
