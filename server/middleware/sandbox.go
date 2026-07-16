package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SandboxHeaders 为 iframe 沙盒内容添加安全响应头
// 用于 userframe、userhtml、wasm 等用户自定义内容的静态服务
func SandboxHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Content-Security-Policy: 限制脚本来源为自身，禁止内联脚本，禁止外部资源加载
		c.Header("Content-Security-Policy",
			"default-src 'self' 'unsafe-inline'; "+
				"script-src 'self' 'unsafe-eval' 'wasm-unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'self'; "+
				"base-uri 'self'; "+
				"form-action 'self'")

		// 禁止 MIME 类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")

		// 禁止页面被嵌入到非同源框架（防止点击劫持）
		c.Header("X-Frame-Options", "SAMEORIGIN")

		// 仅在 HTTPS 下启用 HSTS
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Referrer Policy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions Policy: 限制浏览器特性
		c.Header("Permissions-Policy",
			"camera=(), microphone=(), geolocation=(), "+
				"payment=(), usb=(), magnetometer=(), "+
				"gyroscope=(), speaker=(), vibrate=()")

		c.Next()
	}
}

// SandboxFrameHeaders 为 iframe 内容设置推荐的 sandbox 属性（作为 HTTP 头提示）
// 实际 sandbox 由前端 iframe 标签的 sandbox 属性控制，这里提供辅助安全头
func SandboxFrameHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// CSP: frame-ancestors 控制谁可以嵌入此页面
		c.Header("Content-Security-Policy",
			"default-src 'self' 'unsafe-inline'; "+
				"script-src 'self' 'unsafe-eval' 'wasm-unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'self'; "+
				"base-uri 'self'")

		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Header("Cross-Origin-Embedder-Policy", "require-corp")

		c.Next()
	}
}

// WasmContentType 确保 .wasm 文件返回正确的 MIME 类型
func WasmContentType() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasSuffix(c.Request.URL.Path, ".wasm") {
			c.Header("Content-Type", "application/wasm")
		}
		c.Next()
	}
}

// ServeSandboxedFile 创建带沙盒安全头的静态文件服务
func ServeSandboxedFile(dir string) gin.HandlerFunc {
	fs := http.FileServer(http.Dir(dir))
	return func(c *gin.Context) {
		// 设置安全头
		c.Header("Content-Security-Policy",
			"default-src 'self' 'unsafe-inline'; "+
				"script-src 'self' 'unsafe-eval' 'wasm-unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'self'; "+
				"base-uri 'self'")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// WASM MIME 类型
		if strings.HasSuffix(c.Request.URL.Path, ".wasm") {
			c.Header("Content-Type", "application/wasm")
		}

		// 禁用缓存以确保沙盒内容始终是最新的
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")

		fs.ServeHTTP(c.Writer, c.Request)
	}
}
