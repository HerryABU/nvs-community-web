package routes

// routes.go — 全部 HTTP 路由注册集中在此（从 main.go 原样搬出，未改动任何路由）。
//
// 为什么单独成包：main.go 此前 1852 行、近 1000 行都是路由注册，改一个接口要在
// 上千行里翻；而 main 真正的职责只有「初始化 → 装配 → 启动」。
// 搬出来后 main 只留一行 routes.Register(r)，路由的增删改都在这一个文件里。
//
// 依赖方向是单向的：main → routes → handlers/models/middleware，不存在回环。

import (
	"net"
	"net/http"
	"nvs-server/config"

	"github.com/gin-gonic/gin"
)

// 建站向导的两个守卫（仅 setup 路由使用）。
// setupGuard 建站向导路由守卫：安装完成后直接 404（路由级注销，不触达任何 handler）
func setupGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.IsInstalled() {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Next()
	}
}

// setupLocalOnlyGuard 建站向导仅本机访问守卫（反向代理越权防护）。
// 使用 RemoteIP（真实 TCP 对端地址，不受 X-Forwarded-For 伪造影响）判断是否回环，
// 默认禁止公网/局域网直连访问安装向导；可通过 .env 的 SETUP_LOCAL_ONLY=false 放开（需自担风险）。
func setupLocalOnlyGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.SetupLocalOnly {
			c.Next()
			return
		}
		ip := net.ParseIP(c.RemoteIP())
		if ip != nil && ip.IsLoopback() {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"code":    3,
			"message": "建站向导仅允许本机（localhost）访问，如需远程初始化请设置 SETUP_LOCAL_ONLY=false",
		})
	}
}
