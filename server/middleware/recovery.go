package middleware

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// SecureRecovery 自定义 panic 恢复中间件
// 在生产环境中，不向客户端返回任何详细的错误信息（堆栈跟踪等）
func SecureRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 记录完整错误到服务器日志
				stack := debug.Stack()
				log.Printf("[PANIC RECOVERY] %v\n%s", err, string(stack))

				// 向客户端只返回通用错误
				if !c.Writer.Written() {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
						"code":    5,
						"message": "服务器内部错误，请稍后重试",
						"data":    nil,
					})
				} else {
					// 如果已经开始写入响应，只能中止连接
					c.Abort()
				}
			}
		}()

		c.Next()

		// 捕获处理过程中未预期的 500 错误
		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				log.Printf("[GIN ERROR] %v", e.Err)
			}
			// 仅当响应尚未写入时才返回通用错误
			if !c.Writer.Written() {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    5,
					"message": "服务器内部错误，请稍后重试",
					"data":    nil,
				})
			}
		}
	}
}

// ErrorResponse 生成标准错误响应的辅助函数
func ErrorResponse(code int, message string) gin.H {
	return gin.H{
		"code":    code,
		"message": message,
		"data":    nil,
	}
}

// NoRouteHandler 处理 404 的通用处理器
func NoRouteHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否是 API 请求
		path := c.Request.URL.Path
		if len(path) >= 4 && path[:4] == "/api" {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"code":    4,
				"message": fmt.Sprintf("接口 %s 不存在", path),
				"data":    nil,
			})
			return
		}
		c.AbortWithStatus(http.StatusNotFound)
	}
}
