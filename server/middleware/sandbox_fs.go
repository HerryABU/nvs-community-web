package middleware

import (
	"net/http"
	"strings"

	"nvs-server/security"

	"github.com/gin-gonic/gin"
)

// SandboxWriteProtect 拦截对沙盒目录中脚本类文件的写入操作
// 防止侧链payload：已运行的沙盒代码不能在其目录中植入新的可执行文件
//
// ╔══════════════════════════════════════════════════════════╗
// ║  安全边界说明                                             ║
// ║  ✅ ALLOWED: /api/userframes /api/userhtmls/upload      ║
// ║             (通过认证的API端点 — 用户主动上传/修改)       ║
// ║  ❌ BLOCKED: /sandbox/* /userframe/* /userhtml/*         ║
// ║             (沙盒内运行时尝试写入脚本文件)                ║
// ╚══════════════════════════════════════════════════════════╝
func SandboxWriteProtect() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method

		// ⭐ API路径始终放行（用户通过认证的Web界面主动上传/修改是合法操作）
		if strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}

		// 仅拦截写入类请求
		if method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
			c.Next()
			return
		}

		// 检查是否为沙盒内容路径
		sandboxPrefixes := []string{"/sandbox/", "/userframe/", "/userhtml/"}
		isSandboxPath := false
		for _, prefix := range sandboxPrefixes {
			if strings.HasPrefix(path, prefix) {
				isSandboxPath = true
				break
			}
		}
		if !isSandboxPath {
			c.Next()
			return
		}

		// 检查是否尝试写入脚本类扩展名
		if security.IsBlockedExtension(path) {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    3,
				"message": "沙盒安全策略：禁止创建或修改脚本文件（" + filepathExt(path) + "）",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// 检查路径穿越
		if containsPathTraversal(path) {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    3,
				"message": "沙盒安全策略：路径穿越被阻止",
				"data":    nil,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// SandboxReadOnly 仅允许GET/HEAD/OPTIONS请求访问沙盒内容
func SandboxReadOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 仅对沙盒目录生效
		if !strings.HasPrefix(path, "/sandbox/") {
			c.Next()
			return
		}

		method := c.Request.Method
		if method != "GET" && method != "HEAD" && method != "OPTIONS" {
			c.JSON(http.StatusMethodNotAllowed, gin.H{
				"code":    3,
				"message": "沙盒安全策略：仅支持只读访问",
				"data":    nil,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// PortProxySecurity 端口代理专用安全头
// 比普通沙盒更严格，因为端口代理的后端是用户自己启动的进程
func PortProxySecurity() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if !strings.HasPrefix(path, "/sandbox/proxy/") {
			c.Next()
			return
		}

		// 严格CSP：禁止所有脚本和外部资源（仅静态HTML）
		c.Header("Content-Security-Policy",
			"default-src 'none'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'none'; "+
				"script-src 'none'; "+
				"frame-ancestors 'self'; "+
				"base-uri 'self'")

		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Cache-Control", "no-store, max-age=0")

		c.Next()
	}
}

func filepathExt(path string) string {
	// 从URL路径提取扩展名
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}

func containsPathTraversal(path string) bool {
	return strings.Contains(path, "..") ||
		strings.Contains(path, "./") ||
		strings.Contains(path, ".\\")
}
