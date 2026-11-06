package routes

// routes.go — 全部 HTTP 路由注册集中在此（从 main.go 原样搬出，未改动任何路由）。
//
// 为什么单独成包：main.go 此前 1852 行、近 1000 行都是路由注册，改一个接口要在
// 上千行里翻；而 main 真正的职责只有「初始化 → 装配 → 启动」。
// 搬出来后 main 只留一行 routes.Register(r)，路由的增删改都在这一个文件里。
//
// 依赖方向是单向的：main → routes → handlers/models/middleware，不存在回环。

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"nvs-server/config"
	"nvs-server/handlers/batch"
	"nvs-server/handlers/health"
	"nvs-server/handlers/setup"
	"nvs-server/honeypot"
	"nvs-server/middleware"
	"nvs-server/security"
	"nvs-server/version"
	"regexp"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// Register 装配顺序即路由注册顺序，与改造前完全一致。

// frontendBundleRe 匹配内嵌前端打包产物文件名（index-xxxx.js）
var frontendBundleRe = regexp.MustCompile(`index-[A-Za-z0-9_-]+\.js`)

// knownSPARoots 前端 Vue Router 根段白名单（首段路径）。
// 用于严格 SPA 回退：仅这些根段才回退到 index.html，其余（代理前缀 /go-nvs、数字前缀 /123、
// 以及任意未知前缀）一律 404，杜绝任何形式的代理/前缀造成的组件穿越。
var knownSPARoots = map[string]bool{
	"admin": true, "author": true, "blog": true, "blogs": true,
	"bookshelf": true, "category": true, "comment-network": true, "docs": true, "forum": true,
	"forums": true, "frames": true, "htmls": true, "install": true,
	"login": true, "novel": true, "plaza": true, "register": true,
	"reports": true, "run": true, "settings": true, "setup": true,
	"thread": true, "wasm": true, "pay": true,
	"groups": true, "group": true,
	// 以下根段此前遗漏，导致直接访问/刷新这些页面被 404（SPA 深链失效）
	"about": true, "ai": true, "api-spider": true, "feed": true,
	"me": true, "messages": true, "my-keys": true, "police-console": true,
	// 警方台独立成 /police/*（旧 /police-console 保留为前端重定向）
	"police":  true,
	"reading": true, "topics": true, "user": true, "wallet": true,
	// 新增页面漏登记会直接 404（且只在「直接访问/刷新」时暴露，点链接进去是正常的），
	// 属于最难自查的一类回归。CI 里有 `make spa-roots-check` 做前后端对撞。
	"membership": true, "collab": true, "live": true,
	"oc": true, // #78 独立 OC 工作台：此前遗漏，导致 /oc 直接访问/刷新 404（SPA 深链失效）
}

// firstPathSegment 返回去前导斜杠后的路径首段，如 "novel/1"→"novel"，"admin"→"admin"
func firstPathSegment(cleanPath string) string {
	if i := strings.IndexByte(cleanPath, '/'); i >= 0 {
		return cleanPath[:i]
	}
	return cleanPath
}

// FrontendFS / NodistBuild 由 main 注入。
//
// 为什么不直接调用：//go:embed 的路径相对**源文件所在目录**，且不允许 ..，
// 所以 embed.go 必须留在 server/ 根（与 dist 同级），只能属于 package main。
// routes 包因此拿不到它，改由 main 赋值 —— 行为与改造前一致。
var (
	FrontendFS  = func() (fs.FS, error) { return nil, errors.New("前端资源提供者未注入") }
	NodistBuild bool
)

// Register 把全部中间件与路由挂到引擎上。
func Register(r *gin.Engine) {
	// 🔒 反向代理越权防护：默认不信任任何代理头（X-Forwarded-For/X-Real-IP），
	// ClientIP 取真实 TCP 对端地址，防止攻击者伪造 X-Forwarded-For: 127.0.0.1 绕过限流/黑名单/建站保护。
	// 部署在 nginx 等反向代理后时，可通过 .env 的 TRUSTED_PROXIES 显式信任代理地址。
	_ = r.SetTrustedProxies(config.TrustedProxies)
	r.Use(gin.Logger(), middleware.HeaderLogger(), middleware.SecureRecovery())
	r.Use(middleware.AccessLog()) // 合规：访问日志关联 user_id（按天滚动写入 logs/）
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(middleware.SandboxWriteProtect()) // 🔒 允许数据文件，拦截脚本
	r.Use(middleware.CSRFGuard())           // 🔒 CSRF 防护：登录态写操作须带 X-Requested-With 头

	// 建站向导门禁：未完成安装时，除 /api/setup/* 与 /health 外，拦截所有业务 API（严防死守）
	r.Use(func(c *gin.Context) {
		if config.IsInstalled() {
			c.Next()
			return
		}
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/setup") || p == "/health" || p == "/api/health" {
			c.Next()
			return
		}
		if strings.HasPrefix(p, "/api/") {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code":    5,
				"message": "系统尚未完成初始化，请先完成建站向导",
				"data":    gin.H{"setup_required": true},
			})
			return
		}
		c.Next()
	})

	// 🔒 身份感知限流（API Key 体系）：四层身份 + 匿名/爬虫通道，统一配额管理。
	// 挂在全局中间件链（门禁之后）：仅处理 /api/* 路径；携带有效 Key 的读取请求
	// 按 Key 配额限流并跳过 route 级 IP 限流，无 Key 请求走匿名/爬虫低配额。
	r.Use(security.APIKeyGuard())

	// 蜜罐：诱饵 + 影子认证（/api/v2/*）+ 验证页与指纹采集（/hp/*）
	// 等级为 off 时这些路径与普通 404 无差别，不暴露蜜罐存在。
	honeypot.Register(r)
	// SQL 注入蜜罐（gap-audit #3）：假 SQL 入口 + 预生成响应库；off/low 档与普通 404 无差别
	honeypot.RegisterSQLi(r)
	honeypot.RegisterShadow(r)
	honeypot.RegisterVerify(r)

	// 注入 gin 引擎供 API 聚合中间层（/api/batch）内存内转发使用
	batch.BatchEngine = r

	// 健康检查：浏览器 → HTML 页面，API 请求 → 完整 JSON 健康报告
	r.GET("/health", func(c *gin.Context) {
		if strings.Contains(c.GetHeader("Accept"), "application/json") {
			c.JSON(http.StatusOK, health.CollectHealth())
			return
		}
		healthHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head><meta charset="UTF-8"><title>NVS 服务状态</title>
<style>
  *{margin:0;padding:0;box-sizing:border-box}
  body{font-family:system-ui,sans-serif;background:#f0f2f5;display:flex;justify-content:center;align-items:center;min-height:100vh}
  .card{background:#fff;border-radius:12px;padding:48px;text-align:center;box-shadow:0 2px 20px rgba(0,0,0,.08);max-width:420px}
  .dot{display:inline-block;width:14px;height:14px;border-radius:50%%;background:#22c55e;margin-right:8px;animation:pulse 2s infinite}
  @keyframes pulse{0%%,100%%{opacity:1}50%%{opacity:.5}}
  h1{font-size:1.5rem;color:#1a1a2e;margin-bottom:12px}
  p{color:#666;line-height:1.7}
  .tag{display:inline-block;background:#e8f0ff;color:#3366cc;padding:2px 10px;border-radius:20px;font-size:.85rem;margin:4px}
</style></head>
<body>
<div class="card">
  <h1><span class="dot"></span>NVS 服务运行中</h1>
  <p>网络小说平台 · 节点正常</p>
  <p><span class="tag">版本 %s</span></p>
  <p style="margin-top:16px;font-size:.85rem">联邦社区 · 内容即文件 · 哈希可验证</p>
</div>
</body></html>`, version.AppVersion)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(healthHTML))
	})

	// /api/health：始终返回 JSON 健康报告（供前端/监控调用）
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, health.CollectHealth())
	})

	// 建站向导路由：status 始终可用（供前端判断是否已安装）；
	// 安装/环境检查/测试连接接口仅在未安装时挂载，安装完成后由 setupGuard 直接 404（路由级注销）。
	r.GET("/api/setup/status", setup.SetupStatus)
	if !config.IsInstalled() {
		setupGroup := r.Group("/api/setup")
		// 双重防护：安装完成后 404 + 仅本机(localhost)可访问 + 接口级限流
		setupGroup.Use(setupGuard(), setupLocalOnlyGuard())
		setupGroup.GET("/env-check", setup.SetupEnvCheck)
		setupGroup.POST("/db-test", security.RateLimit(30, 60), setup.SetupDBTest)
		setupGroup.POST("/install", security.RateLimit(10, 60), setup.SetupInstall)
	}

	registerPublic(r)
	registerProtected(r)
	registerAdminArea(r)
	registerPolice(r)
	registerStaticAndSPA(r)
}
