package main

import (
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"nvs-server/chaptervcs"
	"nvs-server/collab"
	"nvs-server/config"
	"nvs-server/crypto"
	"nvs-server/gate"
	"nvs-server/gmcmd"
	"nvs-server/groups"
	"nvs-server/handlers"
	chaptervcshandler "nvs-server/handlers/chaptervcs"
	collabhandler "nvs-server/handlers/collab"
	grouphandler "nvs-server/handlers/group"
	honeypothandler "nvs-server/handlers/honeypot"
	ochandler "nvs-server/handlers/oc"
	prompthandler "nvs-server/handlers/prompts"
	"nvs-server/honeypot"
	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/oc"
	"nvs-server/police"
	"nvs-server/prompttpl"
	"nvs-server/risk"
	"nvs-server/scriptmode"
	"nvs-server/security"
	"nvs-server/security/sms"
	"nvs-server/tlsconfig"
	"nvs-server/utils"
	"nvs-server/version"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

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
	"reading": true, "topics": true, "user": true, "wallet": true,
	// 新增页面漏登记会直接 404（且只在「直接访问/刷新」时暴露，点链接进去是正常的），
	// 属于最难自查的一类回归。CI 里有 `make spa-roots-check` 做前后端对撞。
	"membership": true, "collab": true, "live": true,
}

// firstPathSegment 返回去前导斜杠后的路径首段，如 "novel/1"→"novel"，"admin"→"admin"
func firstPathSegment(cleanPath string) string {
	if i := strings.IndexByte(cleanPath, '/'); i >= 0 {
		return cleanPath[:i]
	}
	return cleanPath
}

func main() {
	// 确保工作目录为 exe 所在目录（双击启动时 Windows 工作目录可能不是 exe 目录）
	if exePath, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exePath); dir != "" {
			os.Chdir(dir)
		}
	}

	config.Init()

	// CLI 子命令（不启动 HTTP 服务）：gm-audit / credential-age
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		os.Exit(gmcmd.Run(os.Args[1:]))
	}

	// 国密（SM）双轨配置校验（fail-fast：轨道取值 / 实现路径 / 双证书四件套）
	if err := config.CryptoCfg.Validate(); err != nil {
		log.Fatalf("国密配置校验失败: %v", err)
	}
	// 握手观测（默认关闭；配置 NVS_TLS_HANDSHAKE_LOG 后启用 JSONL 落盘，N-04 仅本地）
	if err := tlsconfig.Configure(config.TLSHandshakeLogPath); err != nil {
		log.Fatalf("握手观测配置失败: %v", err)
	}

	// 确保数据目录存在
	os.MkdirAll(config.NovelDataDir, 0755)
	os.MkdirAll(config.UploadDir, 0755)
	os.MkdirAll(config.UserFrameDir, 0755)
	os.MkdirAll(config.UserHTMLDir, 0755)

	// 未完成建站向导 → 进入安装模式（不连接数据库，门禁中间件拦截业务 API）
	// 已完成安装 → 连接数据库 + 迁移 + 初始化默认数据
	if config.IsInstalled() {
		if err := initDatabase(); err != nil {
			log.Fatalf("数据库初始化失败: %v", err)
		}
	} else {
		setupKey := config.GetOrCreateSetupKey()
		log.Println("========================================")
		log.Println("  首次运行：进入建站向导模式（尚未连接数据库）")
		log.Printf("  向导地址: http://localhost:%s/install", config.ServerPort)
		log.Printf("  安装密钥（一次性，仅显示一次，切勿泄露）: %s", setupKey)
		log.Println("========================================")
	}

	// 设置 SMTP 配置提供者（从平台配置读取）
	utils.SMTPConfigProvider = func() utils.SMTPConfig {
		host, port, user, pass, from := models.GetSMTPConfigFromDB()
		portInt := 587
		if p, err := strconv.Atoi(port); err == nil {
			portInt = p
		}
		return utils.SMTPConfig{
			Host: host, Port: fmt.Sprintf("%d", portInt), User: user, Password: pass, From: from,
			// 兼容自签名/内网 SMTP：管理员在后台显式开启才跳过证书校验（默认校验证书，安全）
			InsecureSkipVerify: models.GetPlatformConfig("smtp_insecure_skip_verify") == "true",
		}
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
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
	handlers.BatchEngine = r

	// 健康检查：浏览器 → HTML 页面，API 请求 → 完整 JSON 健康报告
	r.GET("/health", func(c *gin.Context) {
		if strings.Contains(c.GetHeader("Accept"), "application/json") {
			c.JSON(http.StatusOK, handlers.CollectHealth())
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
		c.JSON(http.StatusOK, handlers.CollectHealth())
	})

	// 建站向导路由：status 始终可用（供前端判断是否已安装）；
	// 安装/环境检查/测试连接接口仅在未安装时挂载，安装完成后由 setupGuard 直接 404（路由级注销）。
	r.GET("/api/setup/status", handlers.SetupStatus)
	if !config.IsInstalled() {
		setupGroup := r.Group("/api/setup")
		// 双重防护：安装完成后 404 + 仅本机(localhost)可访问 + 接口级限流
		setupGroup.Use(setupGuard(), setupLocalOnlyGuard())
		setupGroup.GET("/env-check", handlers.SetupEnvCheck)
		setupGroup.POST("/db-test", security.RateLimit(30, 60), handlers.SetupDBTest)
		setupGroup.POST("/install", security.RateLimit(10, 60), handlers.SetupInstall)
	}

	// ==================== API 路由 ====================

	public := r.Group("/api")
	public.Use(middleware.OptionalAuth()) // 公开路由也识别登录态（管理员豁免/我的论坛等权限判断）
	{
		public.POST("/auth/register", security.RateLimit(10, 60), handlers.Register) // 安全审计 M2：注册限流防批量灌号
		// 注册门槛提示与预检（邀请码/设备指纹/账号年龄，gap-audit #14）
		public.GET("/auth/register-hint", handlers.RegisterGateHint)
		public.POST("/auth/register-check", security.RateLimit(20, 60), handlers.RegisterGateCheck)
		// 运营位（gap-audit #20 #23）：导航菜单 / 轮播 / 签到状态 / 访客设置
		public.GET("/nav-menus", handlers.PublicNavMenus)
		public.GET("/carousels", handlers.PublicCarousels)
		public.GET("/checkin/status", handlers.CheckInStatus)
		public.GET("/visit-setting", handlers.PublicVisitSetting)
		// 直播入口（gap-audit #23）：只做入口与嵌入，不提供推流
		public.GET("/live", handlers.PublicLive)
		public.POST("/auth/login", security.RateLimit(5, 60), handlers.Login)
		public.POST("/auth/logout", handlers.Logout)
		// 🔒 无感续期：校验 HttpOnly 刷新令牌 Cookie，签发新的访问令牌（不要求已登录）
		public.POST("/auth/refresh", handlers.Refresh)
		// 🔒 滑块验证码挑战（货真价实服务端校验）：先 issue 拿到拼图，再 verify 拿 pass token，
		// 后续 /send-code、/register 必须携带该 token，否则拒绝（防裸调/邮箱轰炸）。
		public.POST("/auth/captcha/issue", handlers.IssueCaptcha)
		public.POST("/auth/captcha/verify", handlers.VerifyCaptcha)
		public.POST("/auth/send-code", handlers.SendVerificationCode)
		// 手机号登录/注册（F10）：验证码经可插拔短信网关发送，存储支持 Redis/内存双模式
		public.POST("/auth/phone/send-code", security.RateLimit(5, 60), handlers.SendPhoneCode)
		public.POST("/auth/phone/login", security.RateLimit(10, 60), handlers.PhoneLogin)
		public.POST("/auth/phone/reset-password", security.RateLimit(5, 60), handlers.PhoneResetPassword)
		public.POST("/auth/verify-code", handlers.VerifyEmailCode)
		public.POST("/auth/forgot-password", security.RateLimit(3, 60), handlers.ForgotPassword)
		public.POST("/auth/reset-password", security.RateLimit(3, 60), handlers.ResetPassword)
		// 微信扫码登录（F21.2）：授权 URL 获取 + OAuth 回调（免鉴权，state 一次性防 CSRF）
		public.GET("/auth/wechat/url", security.RateLimit(10, 60), handlers.WeChatAuthStart)
		public.GET("/auth/wechat/callback", handlers.WeChatCallback)
		public.GET("/categories", handlers.ListCategories)
		public.GET("/categories/stats", handlers.ListCategoryStats)
		public.GET("/wall-zone/:zone", handlers.GetPublicZoneDetail)
		public.GET("/novels", security.RateLimit(100, 60), handlers.ListNovels)
		public.GET("/novels/:id", handlers.GetNovel)
		public.GET("/novels/:id/chapters", handlers.GetChapters)
		public.GET("/novels/:id/chapters/:num", handlers.GetChapterContent)
		public.GET("/novels/:id/chapters/:num/verify", handlers.VerifyChapter)
		public.GET("/comments", handlers.GetComments)
		// 楼加密（入口限制）：单楼整读（公开，楼内校验密码）；验证/设密需登录态
		public.GET("/comments/:id/detail", handlers.GetFloorDetail)
		public.GET("/reports/reasons", handlers.GetReportReasons)
		// API 聚合中间层（一次请求批量获取多个接口数据，向后兼容原有独立调用）
		public.POST("/batch", security.RateLimit(30, 60), handlers.BatchHandler)
		public.GET("/novels/:id/rating", handlers.GetNovelRating)
		public.GET("/forums", handlers.ListForums)
		// 群组（公开/半公开可浏览；私密仅成员）
		public.GET("/groups", grouphandler.GroupList)
		public.GET("/groups/:id", grouphandler.GroupDetail)
		public.GET("/group-lookup", grouphandler.GroupSearchByNo) // 群号/邀请码预览（独立路径，避开 /groups/:id 通配）
		public.GET("/forums/:id", handlers.GetForum)
		public.GET("/novels/:id/forum", handlers.GetNovelForum)
		public.GET("/threads/:id", handlers.GetThread)
		public.GET("/author/profile/:id", handlers.GetAuthorProfile)
		public.GET("/users/resolve", handlers.ResolveUserByKey) // 用户标识解析（id/用户名，数字是主键）
		public.GET("/author/forum/:id", handlers.GetAuthorForum)
		public.GET("/search/authors", handlers.SearchAuthors)
		public.GET("/userframes/public", handlers.ListPublicFrames)
		public.GET("/template/novel/:id", handlers.GetTemplateAPI)
		public.GET("/novels/:id/frames", handlers.GetNovelFrames)
		public.GET("/blogs/:id/frames", handlers.GetBlogFrames)
		public.GET("/author/:id/frames", handlers.GetAuthorFrames)
		public.GET("/novels/:id/htmls", handlers.GetNovelHTMLs)
		public.GET("/blogs/:id/htmls", handlers.GetBlogHTMLs)
		// 176号令合规：公开算法说明（第七条第九项 算法透明度）+ 站点公告栏
		public.GET("/algorithms/description", handlers.GetAlgoDescription)
		public.GET("/announcements", handlers.ListAnnouncements)
		// 自定义主页（F15）：公开区块列表 + 全站最新帖（论坛首页风格区块）
		public.GET("/home-blocks", handlers.ListPublicHomeBlocks)
		public.GET("/threads/latest", handlers.ListLatestThreads)
		// 狂暴模式（F19）：SSE 评论实时流 + 公开状态查询（管理员开关，默认关）
		public.GET("/live/stream", handlers.LiveStream)
		public.GET("/live/config", handlers.LiveConfig)

		// AI 会话分享：公开只读快照（令牌即凭据，无登录要求）
		public.GET("/ai/shared/:token", handlers.AiSharedSession)
		// 灵感广场：公开作品流（作者主动发布才可见）
		public.GET("/ai/gallery", handlers.AiGalleryList)
		// 风格字典（发布作品时下拉用）
		public.GET("/ai/styles", handlers.AiStyleList)
		// AI 生成内容标识（合规展示：只回模型名，不含内容）
		public.GET("/ai/marks", handlers.AiMarks)
		// 在线支付回调（§7.4）：网关异步通知（验签准入，无鉴权）+ 支付完成同步跳转
		public.POST("/payment/notify/:channel", security.RateLimit(120, 60), handlers.PaymentNotify)
		public.GET("/payment/return/:channel", handlers.PaymentReturn)
		// 自定义网站主题（F12：alist 式，服务端消毒后全局生效）
		public.GET("/theme/site", handlers.GetSiteTheme)
		// F12a 全站 opt-in iframe（管理员配置允许列表 URL，访客可读启用状态）
		public.GET("/site-iframe", handlers.GetSiteIframe)
		// F12b 作者主题（访客可见，仅 CSS，服务端消毒）
		public.GET("/author/:id/theme", handlers.GetAuthorTheme)
		// 自定义主页（作者公开页，访客可见已审核片段）
		public.GET("/author/:id/home-customize", handlers.GetAuthorHomeCustomize)
		// 沙盒虚拟文件系统
		public.GET("/sandbox-vfs/:htmlId/read", handlers.VFSRead)
		public.POST("/sandbox-vfs/:htmlId/write", handlers.VFSWrite)
		public.DELETE("/sandbox-vfs/:htmlId/delete", handlers.VFSDelete)
		public.GET("/sandbox-vfs/:htmlId/list", handlers.VFSList)
	}

	protected := r.Group("/api")
	protected.Use(middleware.AuthRequired())
	{
		protected.GET("/auth/me", handlers.GetCurrentUser)
		protected.POST("/auth/change-password", handlers.ChangePassword)
		// 实名制（F10/F21.1）：申请/状态/国家实名核验代理（姓名+身份证不落库明文）
		protected.POST("/auth/realname/apply", security.RateLimit(5, 60), handlers.ApplyRealName)
		protected.GET("/auth/realname", handlers.GetMyRealName)
		protected.POST("/auth/realname/national-check", security.RateLimit(5, 60), handlers.NationalRealNameCheck)
		// 未成年检测（F21.1）：出生日期自报 + 状态查询（strict 分级限制在发布入口门禁）
		protected.POST("/auth/birthdate", security.RateLimit(10, 60), handlers.SetBirthDate)
		protected.GET("/auth/minor-status", handlers.GetMinorStatus)
		// 微信账号绑定（F21.2）：绑定状态 / 绑定 / 解绑
		protected.GET("/auth/wechat/bind-status", handlers.WeChatBindStatus)
		protected.POST("/auth/wechat/bind", handlers.WeChatBind)
		protected.POST("/auth/wechat/unbind", handlers.WeChatUnbind)
		// 自定义网站主题（F12）：个人主题读取 / 保存（服务端消毒）
		protected.GET("/theme/me", handlers.GetMyTheme)
		protected.PUT("/theme/me", handlers.UpdateMyTheme)
		// F12b 作者主题（本人或管理员可写，访客可见）
		protected.PUT("/author/:id/theme", handlers.UpdateAuthorTheme)
		// 自定义主页（alist 式单框代码 + 双作用域 + 站长审核）
		protected.GET("/me/home-customize", handlers.GetMyHomeCustomize)
		protected.PUT("/me/home-customize", handlers.UpdateMyHomeCustomize)
		protected.PUT("/author/:id/home-customize", handlers.UpdateAuthorHomeCustomize)
		// web 项目额度（F17）：个人额度查询 + 设为公开（触发超额审核）
		protected.GET("/web-quota/mine", handlers.MyWebQuota)
		protected.POST("/web-quota/html-public", handlers.SetHTMLPublic)
		protected.POST("/web-quota/frame-public", handlers.SetFramePublic)
		// 每日发文限额（§13.4）：当前用户今日发文额度（发布表单展示剩余量）
		protected.GET("/daily-publish/status", handlers.GetMyPublishQuota)
		// 代币钱包（§7）：状态 / 流水 / 卡密兑换
		protected.GET("/token/status", handlers.TokenStatus)
		protected.GET("/token/ledger", handlers.TokenLedgerList)
		protected.POST("/token/redeem", handlers.RedeemCard)
		// 在线支付充值（§7.4）：档位下单 / 订单轮询 / 我的订单 / 模拟支付确认
		protected.GET("/payment/status", handlers.PaymentStatus)
		protected.POST("/payment/order", security.RateLimit(10, 60), handlers.PaymentCreateOrder)
		protected.GET("/payment/order/:no", handlers.PaymentOrderStatus)
		protected.GET("/payment/orders", handlers.PaymentMyOrders)
		protected.POST("/payment/mock/confirm", security.RateLimit(20, 60), handlers.PaymentMockConfirm)
		// 会员套餐：状态 + 在售套餐 + 支付可用性（会员页一次拉齐）
		protected.GET("/membership", handlers.MembershipInfo)
		// AI 调用（§5.3）：可用性/预估/补全计费（限流防刷）
		protected.GET("/ai/status", handlers.AiStatus)
		protected.POST("/ai/estimate", security.RateLimit(60, 60), handlers.AiEstimate)
		protected.POST("/ai/complete", security.RateLimit(20, 60), handlers.AiComplete)
		// AI 工作台（§5.4）：会话 / 流式对话 / 绘画 / 视频 / 智能体
		protected.GET("/ai/hub/status", handlers.AiHubStatus)
		protected.GET("/ai/agents", handlers.AiAgentList)
		protected.GET("/ai/free-quota", handlers.AiFreeQuotaStatus)

		// 用户自备 API Key（§5.5 BYOK）：CRUD + 默认（:id 通配与既有 /ai/video/:id 不同前缀段，无冲突）
		protected.GET("/ai/my-keys", handlers.AiMyKeys)
		protected.POST("/ai/my-keys", handlers.AiMyKeyCreate)
		protected.DELETE("/ai/my-keys/:id", handlers.AiMyKeyDelete)
		protected.POST("/ai/my-keys/:id/default", handlers.AiMyKeySetDefault)

		// 长篇 AI 记忆（§13.5）：大纲/章节摘要/上下文组装（:id 通配在不同前缀段，无冲突）
		protected.GET("/ai/memory", handlers.AiMemoryList)
		protected.PUT("/ai/memory/outline", handlers.AiMemorySaveOutline)
		protected.PUT("/ai/memory/characters", handlers.AiMemorySaveCharacters)
		protected.PUT("/ai/memory/summary", handlers.AiMemorySaveSummary)
		protected.DELETE("/ai/memory/summary", handlers.AiMemoryDeleteSummary)
		protected.POST("/ai/memory/context", handlers.AiMemoryContext)

		// @ai 评论区机器人（§5.6）：状态查询（配置在管理端）
		protected.GET("/ai/bot/status", handlers.AiBotStatus)
		protected.GET("/ai/sessions", handlers.AiHubSessions)
		protected.GET("/ai/sessions/:id/messages", handlers.AiHubSessionMessages)
		protected.DELETE("/ai/sessions/:id", handlers.AiHubDeleteSession)
		protected.PUT("/ai/sessions/:id", handlers.AiHubRenameSession)
		protected.POST("/ai/sessions/:id/share", handlers.AiHubShareSession)
		protected.DELETE("/ai/sessions/:id/share", handlers.AiHubUnshareSession)
		protected.POST("/ai/chat/stream", security.RateLimit(30, 60), handlers.AiHubChatStream)
		protected.POST("/ai/image", security.RateLimit(20, 60), handlers.AiHubImage)
		protected.POST("/ai/video", security.RateLimit(10, 60), handlers.AiHubVideoSubmit)
		protected.GET("/ai/video/:id", handlers.AiHubVideoStatus)
		// 取消异步视频任务：置终态停止等待 + 已扣款原路退回
		// 注意路径形态：POST 下已有 /ai/video 静态段，这里必须继续往下挂 :id 子节点，
		// 不能与静态段同级加通配（gin 会 panic: conflicts with existing wildcard）
		protected.POST("/ai/video/:id/cancel", handlers.AiHubVideoCancel)
		protected.GET("/ai/media", handlers.AiHubMediaHistory)
		// 出图附件上传 + 历史导出（gap-audit #17）
		// 注意：/ai/media 下不能加静态段（已有 /ai/media 精确路由 + /ai/media/:id/... 通配），
		// 所以历史导出放 /ai/export/media.zip 独立子树
		protected.POST("/ai/media/attach", security.RateLimit(30, 60), handlers.AttachMedia)
		protected.GET("/ai/media/:id/export", handlers.ExportMediaTask)
		protected.GET("/ai/export/media.zip", handlers.ExportMediaHistory)

		// AI 提示词库：平台预设（只读）+ 我的自定义（增删改）
		protected.GET("/ai/prompts", handlers.AiPromptList)
		protected.POST("/ai/prompts", handlers.AiPromptCreate)
		protected.PUT("/ai/prompts/:id", handlers.AiPromptUpdate)
		protected.DELETE("/ai/prompts/:id", handlers.AiPromptDelete)
		protected.POST("/ai/prompts/:id/use", handlers.AiPromptUse)

		// 灵感广场：发布/取消发布、点赞；思维导图生成
		protected.POST("/ai/gallery/:id/publish", handlers.AiGalleryPublish)
		protected.POST("/ai/gallery/:id/like", handlers.AiGalleryLike)
		protected.GET("/ai/media/:id/public", handlers.AiMyMediaPublic)
		protected.POST("/ai/mindmap", security.RateLimit(10, 60), handlers.AiMindmap)

		// 个人信息泄漏提示：只累计命中类型，不接收内容（检测不留痕）
		protected.POST("/security/leak-hint", handlers.RecordLeakHint)
		// 用户黑名单（F13：拉黑禁言/屏蔽内容）
		protected.POST("/blacklist", security.RateLimit(20, 60), handlers.AddBlacklist)
		protected.DELETE("/blacklist/:id", handlers.RemoveBlacklist)
		protected.GET("/blacklist", handlers.ListMyBlacklist)
		// 私信（F20：会话消息；互关才可发第 3 条，可在设置中关闭）
		protected.POST("/messages", security.RateLimit(30, 60), handlers.SendMessage)
		protected.GET("/messages/conversations", handlers.ListConversations)
		protected.GET("/messages/with/:userId", handlers.ListMessages)
		protected.POST("/messages/read", handlers.MarkMessageRead)
		protected.GET("/messages/unread-count", handlers.UnreadMessageCount)
		protected.GET("/messages/settings", handlers.GetMessageSettings)
		protected.PUT("/messages/settings", handlers.UpdateMessageSettings)
		protected.POST("/messages/recall", handlers.RecallMessage)
		protected.POST("/messages/conversation/delete", handlers.DeleteConversation)
		// 用户 API Key 管理（创建/查看/修改/重生成/删除，配额在管理员角色上限内自设）
		protected.GET("/apikeys", handlers.ListMyAPIKeys)
		protected.POST("/apikeys", handlers.CreateAPIKey)
		protected.PATCH("/apikeys/:id", handlers.UpdateAPIKey)
		protected.POST("/apikeys/:id/regenerate", handlers.RegenerateAPIKey)
		protected.DELETE("/apikeys/:id", handlers.DeleteAPIKey)
		// 站内消息通知
		protected.GET("/notifications", handlers.ListNotifications)
		protected.GET("/notifications/unread-count", handlers.UnreadNotificationCount)
		protected.POST("/notifications/read", handlers.MarkNotificationRead)
		// 精读：书签 + 批注
		protected.POST("/reading/notes", handlers.CreateReadingNote)
		protected.GET("/reading/notes", handlers.ListMyReadingNotes)
		protected.GET("/reading/notes/mine", handlers.ListAllMyNotes)
		protected.PATCH("/reading/notes/:id", handlers.UpdateReadingNote)
		protected.DELETE("/reading/notes/:id", handlers.DeleteReadingNote)
		protected.POST("/comments", security.RateLimit(20, 60), handlers.CreateComment)
		protected.DELETE("/comments/:id", handlers.DeleteComment)
		// 楼加密（入口限制）：验证访问密码 / 设置·修改·解除密码（限速防爆破）
		protected.POST("/comments/:id/verify-password", security.RateLimit(10, 60), handlers.VerifyFloorPassword)
		protected.POST("/comments/:id/password", security.RateLimit(10, 60), handlers.SetFloorPassword)
		// @ 提及来源设置 + 联想候选（站长/楼主可配置，评论框联想用）
		protected.GET("/mention-source", handlers.GetMentionSource)
		protected.PUT("/mention-source", handlers.SetMentionSource)
		protected.GET("/mention-candidates", handlers.GetMentionCandidates)
		// 评论网（社交图谱）分析：谁回复了谁的哪条评论 + 时间估计（可按 user_id 下钻）
		protected.GET("/comment-network", handlers.GetCommentNetwork)
		protected.POST("/novels", handlers.CreateNovel)
		protected.PUT("/novels/:id", handlers.UpdateNovel)
		protected.DELETE("/novels/:id", handlers.DeleteNovel)
		protected.POST("/novels/:id/chapters", handlers.CreateChapter)
		protected.PUT("/novels/:id/chapters/:num", handlers.UpdateChapter)
		protected.DELETE("/novels/:id/chapters/:num", handlers.DeleteChapter)
		// 章节工作流（多人协作）：提审 / 审核 / 直接发布 / 撤回
		protected.POST("/novels/:id/chapters/:num/submit", handlers.SubmitChapterReview)
		protected.POST("/novels/:id/chapters/:num/review", handlers.ReviewChapter)
		protected.POST("/novels/:id/chapters/:num/publish", handlers.PublishChapter)
		protected.POST("/novels/:id/chapters/:num/withdraw", handlers.WithdrawChapter)
		// 多人协作：成员管理 / 邀请处理 / 我参与的协作
		protected.GET("/novels/:id/collaborators", collabhandler.List)
		protected.POST("/novels/:id/collaborators", collabhandler.Invite)
		protected.PUT("/novels/:id/collaborators/:uid", collabhandler.Update)
		protected.DELETE("/novels/:id/collaborators/:uid", collabhandler.Remove)
		// 主笔转让（不可逆，前端二次确认输入作品标题）
		protected.POST("/novels/:id/transfer", collabhandler.Transfer)
		protected.GET("/collab/invites", collabhandler.Invites)
		protected.GET("/collab/works", collabhandler.Works)
		protected.POST("/collab/:novelId/respond", collabhandler.Respond)
		protected.POST("/collab/:novelId/leave", collabhandler.Leave)
		// 话本气泡体预览（不可挂 /novels 下，与 /novels/:id 通配段冲突）
		protected.POST("/script-preview", handlers.ScriptPreview)
		// ===== OC 模式（docs/oc-mode.md）=====
		// 角色卡 / 世界书（不挂 /groups 下，避免与 /groups/:id 通配冲突）
		protected.GET("/oc/characters", ochandler.ListCharacters)
		protected.POST("/oc/characters", ochandler.CreateCharacter)
		protected.GET("/oc/characters/:cid", ochandler.GetCharacter)
		protected.PUT("/oc/characters/:cid", ochandler.UpdateCharacter)
		protected.DELETE("/oc/characters/:cid", ochandler.DeleteCharacter)
		protected.GET("/oc/books", ochandler.ListBooks)
		protected.POST("/oc/books", ochandler.CreateBook)
		protected.GET("/oc/books/:bid", ochandler.GetBook)
		protected.PUT("/oc/books/:bid", ochandler.UpdateBook)
		protected.DELETE("/oc/books/:bid", ochandler.DeleteBook)
		protected.POST("/oc/books/:bid/entries", ochandler.SaveEntry)
		protected.DELETE("/oc/books/:bid/entries/:eid", ochandler.DeleteEntry)
		// 群级配置 / AI 角色 / 对话流
		protected.GET("/groups/:id/oc", ochandler.GetSetting)
		protected.PUT("/groups/:id/oc", ochandler.SaveSetting)
		protected.POST("/groups/:id/oc/agents", ochandler.AddAgent)
		protected.PUT("/groups/:id/oc/agents/:aid", ochandler.UpdateAgent)
		protected.DELETE("/groups/:id/oc/agents/:aid", ochandler.RemoveAgent)
		protected.GET("/groups/:id/oc/flows", ochandler.ListFlows)
		protected.POST("/groups/:id/oc/flows", ochandler.CreateFlow)
		protected.GET("/oc/flows/:fid/turns", ochandler.ListTurns)
		protected.POST("/oc/flows/:fid/turns", ochandler.HumanTurn)
		protected.POST("/oc/flows/:fid/step", ochandler.StepFlow)
		protected.POST("/oc/flows/:fid/status", ochandler.SetFlowStatus)
		protected.POST("/oc/flows/:fid/audit", ochandler.AuditFlow)
		protected.GET("/oc/flows/:fid/windows", ochandler.Windows)
		protected.GET("/oc/flows/:fid/report", ochandler.Report)
		protected.POST("/oc/flows/:fid/publish", ochandler.Publish)
		// 章节版本控制：历史 / 分支 / 合并（docs/novel-version-control.md）
		protected.GET("/novels/:id/chapters/:num/versions", chaptervcshandler.ListVersions)
		protected.GET("/novels/:id/chapters/:num/versions/:ver", chaptervcshandler.GetVersion)
		protected.GET("/novels/:id/chapters/:num/versions/:ver/diff", chaptervcshandler.DiffVersions)
		protected.POST("/novels/:id/chapters/:num/versions/:ver/revert", chaptervcshandler.RevertVersion)
		protected.DELETE("/novels/:id/chapters/:num/versions/:ver", chaptervcshandler.DeleteVersion)
		protected.GET("/novels/:id/chapters/:num/branches", chaptervcshandler.ListBranches)
		protected.POST("/novels/:id/chapters/:num/branches", chaptervcshandler.CreateBranch)
		protected.POST("/novels/:id/chapters/:num/merge", chaptervcshandler.MergeBranch)
		protected.GET("/author/novels", handlers.GetMyNovels)
		protected.GET("/author/novels/:id/stats", handlers.GetNovelStats)
		protected.GET("/author/banned-content", handlers.GetAuthorBannedContent)
		protected.GET("/author/recycle", handlers.ListAuthorRecycle)             // 回收站：用户自删内容
		protected.POST("/author/recycle/restore", handlers.RestoreAuthorRecycle) // 恢复自己删除的内容
		protected.GET("/author/dashboard", handlers.GetAuthorDashboard)
		protected.POST("/novels/:id/export", handlers.ExportNovel)
		protected.POST("/zone/enter", handlers.RecordZoneEnter) // 记录进入隔离墙（敏感分区）时间，176号令合规
		protected.POST("/ratings", handlers.UpsertRating)
		protected.GET("/ratings", handlers.GetUserRating)
		// 176号令合规：用户偏好设置（第七条第九项 个性化推荐关闭权）
		protected.GET("/users/me/preferences", handlers.GetUserPreferences)
		protected.PUT("/users/me/preferences", handlers.UpdateUserPreferences)
		protected.POST("/forums/:id/threads", handlers.CreateThread)
		protected.POST("/threads/:id/posts", handlers.CreatePost)
		protected.DELETE("/threads/:id", handlers.DeleteThreadByModerator)
		protected.DELETE("/posts/:id", handlers.DeletePostByModerator)
		// 群组：建群/改设置/解散/加入/按群号加入
		protected.POST("/groups", grouphandler.GroupCreate)
		protected.PUT("/groups/:id", grouphandler.GroupUpdate)
		protected.DELETE("/groups/:id", grouphandler.GroupDissolve)
		protected.POST("/groups/:id/join", grouphandler.GroupJoin)
		protected.POST("/group-join", grouphandler.GroupJoinByNo) // 按群号/邀请码加入（独立路径避通配冲突）
		// 群成员：拉人/审核/角色/禁言/踢退
		protected.GET("/groups/:id/members", grouphandler.GroupMembers)
		protected.POST("/groups/:id/invite", grouphandler.GroupInvite)
		protected.POST("/groups/:id/requests/:uid", grouphandler.GroupMemberApprove)
		protected.PUT("/groups/:id/members/:uid", grouphandler.GroupMemberSetRole)
		protected.PUT("/groups/:id/members/:uid/mute", grouphandler.GroupMemberMute)
		protected.DELETE("/groups/:id/members/:uid", grouphandler.GroupMemberRemove)
		protected.GET("/groups/:id/invite-code", grouphandler.GroupInviteCode)
		protected.PUT("/groups/:id/announcement", grouphandler.GroupAnnounce)
		// 群内帖子/回复
		protected.GET("/groups/:id/posts", grouphandler.GroupPostList)
		protected.POST("/groups/:id/posts", grouphandler.GroupPostCreate)
		protected.GET("/groups/:id/posts/:pid", grouphandler.GroupPostDetail)
		protected.POST("/groups/:id/posts/:pid/review", grouphandler.GroupPostReview)
		protected.DELETE("/groups/:id/posts/:pid", grouphandler.GroupPostDelete)
		protected.GET("/groups/:id/posts/:pid/replies", grouphandler.GroupReplyList)
		protected.POST("/groups/:id/posts/:pid/replies", grouphandler.GroupReplyCreate)
		protected.DELETE("/groups/:id/posts/:pid/replies/:rid", grouphandler.GroupReplyDelete)
		// 群文件（鉴权上传/下载，存储隔离在 data/groups）
		protected.GET("/groups/:id/files", grouphandler.GroupFileList)
		protected.POST("/groups/:id/files", grouphandler.GroupFileUpload)
		protected.GET("/groups/:id/files/:fid/download", grouphandler.GroupFileDownload)
		protected.DELETE("/groups/:id/files/:fid", grouphandler.GroupFileDelete)
		// 群内 AI 可用性
		protected.GET("/groups/:id/ai", grouphandler.AiStatus)
		// 群组实时消息（SSE：新帖/回复/公告/成员变动，复用 realtime Hub 批量下发）
		protected.GET("/groups/:id/stream", grouphandler.Stream)
		// 群消息已读（未读计数归零）
		protected.POST("/groups/:id/read", grouphandler.MarkRead)
		// 场景化提示词模板（编辑器「模板」下拉 + 变量渲染）
		protected.GET("/ai/templates", prompthandler.List)
		protected.POST("/ai/templates/:id/render", prompthandler.Render)
		// 带作品上下文的渲染：自动注入大纲/角色设定/前文摘要/前文/作者样本
		protected.POST("/ai/templates/:id/render-context", prompthandler.RenderContext)
		// 用户自建论坛
		protected.POST("/forums", handlers.CreateUserForum)
		protected.POST("/forums/enter-by-code", handlers.EnterForumByRoomCode) // 房间号进入不可见论坛
		protected.GET("/forums/mine", handlers.ListMyForums)
		protected.POST("/forums/:id/verify", handlers.VerifyForumPassword)
		protected.POST("/forums/:id/mute", handlers.ForumMuteUser)
		protected.POST("/forums/:id/unmute", handlers.ForumUnmuteUser)
		protected.GET("/forums/:id/mutes", handlers.ListForumMutes) // 论坛禁言列表（创建者/管理员）
		// VIP & 导入导出
		protected.POST("/author/apply-vip", handlers.ApplyVip)
		protected.POST("/novels/import/preview", handlers.ImportPreview)
		protected.POST("/novels/import", handlers.ImportNovel)
		protected.POST("/novels/import/batch", handlers.ImportBatch)
		protected.POST("/novels/:id/export/epub", handlers.ExportEPUB)
		protected.POST("/novels/:id/export/markdown", handlers.ExportMarkdown)
		protected.POST("/novels/:id/export/txt", handlers.ExportTXT)
		protected.POST("/novels/:id/export/docx", handlers.ExportDOCX)
		// 举报
		protected.POST("/reports", handlers.CreateReport)
		protected.GET("/reports/mine", handlers.ListMyReports)
		// 收益
		protected.GET("/author/earnings", handlers.GetEarnings)
		protected.POST("/author/withdraw", handlers.RequestWithdraw)
		// 作者头像上传
		protected.POST("/author/avatar", handlers.UploadAvatar)
		// 作者签名状态
		protected.GET("/author/signature-status", handlers.GetSignatureStatus)
		// 作者上传图片（重新编码防马）
		protected.POST("/author/image", handlers.UploadImage)
		// 作者更新资料（签名、隔离开关）
		protected.PUT("/author/profile", handlers.UpdateAuthorProfile)
		// 论坛置顶
		protected.POST("/threads/:id/pin", handlers.PinThread)
		protected.POST("/threads/:id/unpin", handlers.UnpinThread)
		// 关注作者
		protected.POST("/follow/:id", handlers.FollowAuthor)
		protected.DELETE("/follow/:id", handlers.UnfollowAuthor)
		protected.GET("/following", handlers.ListFollowing)
		protected.GET("/followers", handlers.ListFollowers)
		protected.GET("/follow/check/:id", handlers.CheckFollow)
		protected.GET("/follow/stats", handlers.GetFollowStats)
		// 特别关注 + 动态流（F14）
		protected.POST("/follows/:id/special", handlers.ToggleSpecialFollow)
		protected.GET("/follows/special", handlers.ListMySpecialFollows)
		protected.GET("/feed/special", handlers.GetSpecialFeed)
		// 作者博客（创建、编辑、删除需登录）
		protected.POST("/blogs", handlers.CreateBlog)
		protected.POST("/topics", handlers.CreateTopic) // 创建话题（登录）
		protected.POST("/blogs/import/batch", handlers.ImportBlogs)
		protected.POST("/blogs/batch-category", handlers.BatchSetBlogCategory) // 批量设置博客分类（CSDN 式）
		protected.PUT("/blogs/:id", handlers.UpdateBlog)
		protected.DELETE("/blogs/:id", handlers.DeleteBlog)
		protected.POST("/blogs/:id/like", handlers.ToggleBlogLike) // 博客点赞/取消
		protected.POST("/upload", handlers.UploadFile)             // 通用上传：博客图片/音视频
		// 博客分类管理（登录作者/管理员均可：新增/编辑/删除/挂靠，支持三级；普通作者创建=待审核）
		protected.GET("/blog-categories/manage", handlers.ListMyBlogCategories)
		protected.POST("/blog-categories", handlers.CreateBlogCategory)
		protected.PUT("/blog-categories/:id", handlers.UpdateBlogCategory)
		protected.DELETE("/blog-categories/:id", handlers.DeleteBlogCategory)
		// 用户自定义 UI-HTML 模板 (UserFrame)
		protected.GET("/userframes", handlers.ListFrames)
		// F16 站内内容相对路径引用选择器（CSDN 式）
		protected.GET("/frame/refs", handlers.ListContentRefs)
		protected.POST("/userframes", handlers.CreateFrame)
		protected.GET("/userframes/:id", handlers.GetFrame)
		protected.PUT("/userframes/:id", handlers.UpdateFrame)
		protected.DELETE("/userframes/:id", handlers.DeleteFrame)
		// 用户自定义 HTML (UserHTML)
		protected.GET("/userhtmls", handlers.ListHTMLs)
		protected.GET("/userhtmls/:id", handlers.GetHTML)
		protected.PUT("/userhtmls/:id", handlers.UpdateHTML)
		protected.DELETE("/userhtmls/:id", handlers.DeleteHTML)
		protected.POST("/userhtmls/upload", handlers.UploadZipHTML)
	}

	// 端口分配API
	port := r.Group("/api/port")
	port.Use(middleware.AuthRequired())
	{
		port.POST("/allocate", handlers.AllocatePort)
		port.POST("/release", handlers.ReleasePort)
		port.GET("/list", handlers.ListPorts)
	}

	// 书架路由
	bookshelf := r.Group("/api/bookshelf")
	bookshelf.Use(middleware.AuthRequired())
	{
		bookshelf.GET("", handlers.ListShelf)
		bookshelf.POST("", handlers.AddToShelf)
		bookshelf.DELETE("/:id", handlers.RemoveFromShelf)
		bookshelf.GET("/check/:id", handlers.CheckShelf)
		bookshelf.POST("/progress", handlers.UpdateShelfProgress)

		// ===== 运营位（gap-audit #20 #23）=====
		protected.POST("/checkin", handlers.DoCheckIn)
		// ===== 语音合成 / 音乐（gap-audit #18）=====
		protected.POST("/ai/speech", security.RateLimit(10, 60), handlers.Speech)
		// ===== OC 表情包 + 角色卡/世界书导入导出（gap-audit #22 #24）=====
		// 注意：/oc/characters/:cid 已是通配段，import 必须换前缀（gin 不允许同层静态+通配）
		protected.GET("/oc/emotes", ochandler.ListEmotes)
		protected.POST("/oc/emotes", ochandler.CreateEmote)
		protected.DELETE("/oc/emotes/:id", ochandler.DeleteEmote)
		protected.POST("/oc/import/character", ochandler.ImportCard)
		protected.POST("/oc/import/book", ochandler.ImportBook)
		protected.GET("/oc/characters/:cid/export", ochandler.ExportCard)
		protected.GET("/oc/books/:bid/export", ochandler.ExportBook)

		// ===== 警方联动工单（gap-audit #6）：角色校验在 handler 内 =====
		protected.GET("/police/tickets", handlers.ListTickets)
		protected.POST("/police/tickets", handlers.CreateTicket)
		protected.GET("/police/tickets/:id", handlers.GetTicket)
		protected.POST("/police/tickets/:id/assign", handlers.AssignTicket)
		protected.POST("/police/tickets/:id/status", handlers.SetTicketStatus)
		protected.POST("/police/tickets/:id/notes", handlers.AddTicketNote)
	}

	// 管理员路由
	admin := r.Group("/api/admin")
	// 🔒 纵深防御：组级强制 admin 角色（除登录校验外再叠加角色校验），
	// 防止「新增 handler 漏写 ensureAdmin」导致越权。具体写操作仍由各 handler 内部 ensureAdmin 兜底。
	admin.Use(middleware.AuthRequired(), middleware.AdminRequired())
	{
		admin.GET("/stats", handlers.GetAdminStats)
		admin.GET("/dashboard", handlers.GetDashboardStats)
		admin.GET("/community", handlers.GetCommunityDashboard)
		// 实名记录查询（F10：姓名解密仅在本响应返回，身份证仅脱敏展示）
		admin.GET("/realname", handlers.AdminListRealName)
		// 实名核验提供商配置（国家代理/腾讯/阿里/华为/网易）
		admin.GET("/realname-provider", handlers.AdminGetRealNameProvider)
		admin.PUT("/realname-provider", handlers.AdminSetRealNameProvider)
		admin.POST("/realname-provider/test", handlers.AdminTestRealNameProvider)
		admin.GET("/hotwords", handlers.AdminHotWords)                     // 热词统计（内容治理/舆情观测）
		admin.POST("/search-logs/cleanup", handlers.AdminSearchLogCleanup) // 维护搜索埋点日志（清空/保留期清理）
		admin.GET("/users", handlers.ListUsers)
		admin.GET("/users/audit", handlers.ListUserAuditLogs)
		admin.GET("/zone-access", handlers.ListZoneAccessLogs)           // 隔离墙进入记录（176号令专项，分页+筛选）
		admin.GET("/zone-access/stats", handlers.GetZoneAccessStats)     // 隔离墙进入记录统计（仪表盘卡片）
		admin.GET("/web-access", handlers.ListWebAccessLogs)             // 网络诊断 / 用户分布 / 攻击流量（分页+多维筛选）
		admin.GET("/web-access/stats", handlers.GetWebAccessStats)       // Web 访问日志聚合统计（诊断面板）
		admin.POST("/web-access/cleanup", handlers.CleanupWebAccessLogs) // 清空 Web 访问日志（合规证据由文件日志承担）
		admin.PUT("/users/:id", handlers.UpdateUser)
		admin.DELETE("/users/:id", handlers.DeleteUser)
		admin.POST("/users/:id/mute", handlers.MuteUser)
		admin.POST("/users/:id/unmute", handlers.UnmuteUser)
		admin.POST("/users/:id/promote-police", handlers.AdminPromoteToPolice) // 提权为警方（准管理员，仅 admin 可操作，留审计）
		admin.POST("/users/:id/revoke-police", handlers.AdminRevokePolice)     // 撤销警方权限（回退 reader）
		admin.GET("/vip-applications", handlers.ListVipApplications)
		admin.POST("/vip-applications/:id/approve", handlers.ApproveVip)
		admin.GET("/reports", handlers.ListReports)
		admin.POST("/reports/:id/handle", handlers.HandleReport)
		// HTML 内容审查（人工复核）
		admin.GET("/html-reviews", handlers.ListHTMLReviews)
		admin.GET("/html-reviews/:type/:id", handlers.GetHTMLReview)
		admin.POST("/html-reviews/:type/:id/review", handlers.ReviewHTML)
		admin.GET("/finance", handlers.GetFinanceOverview)
		// 站长配置
		admin.GET("/config", handlers.GetPlatformConfigs)
		admin.POST("/smtp/test", handlers.TestSMTP) // SMTP 测试与逐阶段诊断
		admin.PUT("/config", handlers.UpdatePlatformConfig)
		// 日志脱敏配置（176号令合规：生产环境日志敏感信息脱敏）
		admin.GET("/log-redact", handlers.GetLogRedactConfig)
		admin.PUT("/log-redact", handlers.UpdateLogRedactConfig)
		// 蜜罐等级 + 日志记录字段勾选（docs/honeypot-design.md）
		admin.GET("/honeypot/config", honeypothandler.GetConfig)
		admin.PUT("/honeypot/config", honeypothandler.SaveConfig)
		admin.GET("/honeypot/sessions", honeypothandler.ListSessions)
		admin.GET("/honeypot/sessions/:sid", honeypothandler.SessionDetail)
		admin.GET("/honeypot/export", honeypothandler.Export)
		admin.POST("/honeypot/cleanup", honeypothandler.Cleanup)
		admin.GET("/honeypot/archives", honeypothandler.Archives)
		admin.GET("/honeypot/db", honeypothandler.DBInfo)
		admin.GET("/honeypot/bans", honeypothandler.Bans)
		admin.POST("/honeypot/ban", honeypothandler.Ban)
		admin.POST("/honeypot/unban", honeypothandler.Unban)
		admin.GET("/honeypot/sessions/:sid/fingerprints", honeypothandler.Fingerprints)
		admin.GET("/honeypot/export.zip", honeypothandler.ExportZip)
		admin.GET("/honeypot/rules", honeypothandler.Rules)
		admin.POST("/honeypot/rules", honeypothandler.ApplyRules)
		admin.POST("/log-redact/preview", handlers.PreviewLogRedact)
		// 176号令合规补全：算法推荐管理（第七条第九项）
		admin.GET("/algorithms", handlers.GetAlgoConfig)
		admin.PUT("/algorithms", handlers.UpdateAlgoConfig)
		// 等保/关基识别记录（第七条第四/五项）
		admin.GET("/security-grade", handlers.GetSecurityGrade)
		admin.PUT("/security-grade", handlers.UpdateSecurityGrade)
		// 漏洞整改管理（第七条第六项）
		admin.GET("/vulnerabilities", handlers.ListVulnerabilities)
		admin.POST("/vulnerabilities", handlers.CreateVulnerability)
		admin.PUT("/vulnerabilities/:id", handlers.UpdateVulnerability)
		admin.DELETE("/vulnerabilities/:id", handlers.DeleteVulnerability)
		admin.GET("/vulnerabilities/export", handlers.ExportVulnerabilities)
		// 公告栏（管理端）
		admin.GET("/announcements", handlers.AdminListAnnouncements)
		admin.POST("/announcements", handlers.AdminCreateAnnouncement)
		admin.PUT("/announcements/:id", handlers.AdminUpdateAnnouncement)
		admin.DELETE("/announcements/:id", handlers.AdminDeleteAnnouncement)
		// 自定义主页区块（F15）
		// ⚠️ gin httprouter 铁律：静态段不能与同级通配段共存，
		// 故 reorder/presets 用独立前缀，避免与 /home-blocks/:id 冲突 panic
		admin.GET("/home-blocks", handlers.AdminListHomeBlocks)
		admin.POST("/home-blocks", handlers.AdminCreateHomeBlock)
		admin.PUT("/home-blocks/:id", handlers.AdminUpdateHomeBlock)
		admin.DELETE("/home-blocks/:id", handlers.AdminDeleteHomeBlock)
		admin.POST("/home-block-reorder", handlers.AdminReorderHomeBlocks)
		admin.GET("/home-block-presets", handlers.AdminListHomeBlockPresets)
		admin.POST("/home-block-presets/:key/apply", handlers.AdminApplyHomeBlockPreset)
		// 狂暴模式配置（F19：默认关；开启后评论区 SSE 实时推送）
		admin.GET("/rage-config", handlers.GetRageConfigAdmin)
		admin.PUT("/rage-config", handlers.UpdateRageConfigAdmin)
		// 自定义网站主题（F12：alist 式，管理员配置对全站生效）
		admin.PUT("/theme/site", handlers.UpdateSiteTheme)
		// F12a 全站 opt-in iframe 仅管理员可写
		admin.PUT("/site-iframe", handlers.UpdateSiteIframe)
		// web 项目额度与超额审核（F17）
		admin.GET("/web-quota/requests", handlers.AdminListWebQuotaRequests)
		admin.POST("/web-quota/requests/:id/approve", handlers.AdminApproveWebQuotaRequest)
		admin.POST("/web-quota/requests/:id/reject", handlers.AdminRejectWebQuotaRequest)
		admin.GET("/web-quota/config", handlers.AdminGetWebQuotaConfig)
		admin.PUT("/web-quota/config", handlers.AdminSetWebQuotaConfig)
		// 检查应对工具箱（176号令 5.2 检查材料清单）
		admin.GET("/compliance/report", handlers.GetComplianceReport)
		admin.GET("/compliance/export-logs", handlers.ExportComplianceLogs)
		admin.GET("/compliance/export-users", handlers.ExportComplianceUsers)
		// 一键导出检查应对材料包（zip + SHA256 清单，导出留痕）
		admin.GET("/compliance/bundle.zip", handlers.ExportComplianceBundle)
		// ===== 注册门槛 / 邀请码 / 设备指纹（gap-audit #14）=====
		admin.GET("/gate", handlers.AdminGateConfig)
		admin.PUT("/gate", handlers.SaveGateConfig)
		admin.GET("/gate/invites", handlers.AdminInvites)
		admin.POST("/gate/invites", handlers.CreateInvites)
		admin.POST("/gate/invites/:id/revoke", handlers.RevokeInvite)
		admin.GET("/gate/devices", handlers.AdminDeviceBindings)
		// ===== 充值风控（gap-audit #9）=====
		admin.GET("/risk", handlers.AdminRiskConfig)
		admin.PUT("/risk", handlers.SaveRiskConfig)
		admin.GET("/risk/events", handlers.AdminRiskEvents)
		admin.POST("/risk/events/:id/handle", handlers.HandleRiskEvent)
		// ===== API Key 审计告警（告警口径补齐）=====
		admin.GET("/api-key-alarms", handlers.AdminKeyAlarms)
		admin.POST("/api-key-alarms/:id/ack", handlers.AckKeyAlarm)
		// ===== 自定义导航菜单 / 轮播 / 签到 / 访客设置 =====
		admin.GET("/nav-menus", handlers.AdminNavMenus)
		admin.POST("/nav-menus", handlers.SaveNavMenu)
		admin.DELETE("/nav-menus/:id", handlers.DeleteNavMenu)
		admin.GET("/carousels", handlers.AdminCarousels)
		admin.POST("/carousels", handlers.SaveCarousel)
		admin.DELETE("/carousels/:id", handlers.DeleteCarousel)
		admin.GET("/checkin", handlers.AdminCheckIn)
		admin.PUT("/checkin", handlers.SaveCheckIn)
		admin.GET("/visit-setting", handlers.AdminVisitSetting)
		admin.PUT("/visit-setting", handlers.SaveVisitSetting)
		admin.GET("/live", handlers.AdminLive)
		admin.PUT("/live", handlers.SaveLive)
		// ===== 语音模型 / 转化漏斗 =====
		admin.GET("/ai/audio-models", handlers.AdminSpeechModels)
		admin.GET("/ai/funnel", handlers.AiFunnel)
		// 全站总预算 + 并发排队 / 三维限流
		admin.GET("/ai/budget", handlers.GetAiBudget)
		admin.PUT("/ai/budget", handlers.SaveAiBudget)
		admin.GET("/ai/throttle", handlers.GetAiThrottle)
		admin.PUT("/ai/throttle", handlers.SaveAiThrottle)
		// ===== 蜜罐：SQLi 命中与 TLS 采集率 =====
		admin.GET("/honeypot/sqli", honeypothandler.SQLiStats)
		admin.GET("/honeypot/tls", honeypothandler.TLSStats)
		// 隔离墙配置
		admin.GET("/wall-config", handlers.GetWallConfig)
		admin.PUT("/wall-config", handlers.UpdateWallConfig)
		// 远程站点互通
		admin.GET("/sites", handlers.ListFederatedSites)
		admin.GET("/sites/inbound", handlers.ListInboundSites)
		admin.POST("/sites", handlers.CreateFederatedSite)
		admin.PUT("/sites/:id", handlers.UpdateFederatedSite)
		admin.DELETE("/sites/:id", handlers.DeleteFederatedSite)
		admin.POST("/sites/:id/sync", handlers.SyncFederatedSite)
		// 联邦站点 API Key（双向可见：本站签发给对方 / 对方签发给本站）
		admin.GET("/sites/:id/keys", handlers.ListFederatedSiteKeys)
		admin.POST("/sites/:id/keys/inbound", handlers.CreateFederatedInboundKey)
		admin.POST("/sites/:id/keys/outbound", handlers.SaveFederatedOutboundKey)
		admin.POST("/sites/:id/keys/:kid/toggle", handlers.ToggleFederatedKey)
		admin.DELETE("/sites/:id/keys/:kid", handlers.DeleteFederatedKey)
		// 限流与 API Key 管理（全局配额 / 爬虫规则 / 日志统计 / 全站 Key）
		admin.GET("/ratelimit/config", handlers.GetRateLimitConfig)
		admin.PUT("/ratelimit/config", handlers.UpdateRateLimitConfig)
		admin.GET("/ratelimit/logs", handlers.ListRateLimitLogs)
		admin.GET("/ratelimit/stats", handlers.GetRateLimitStats)
		admin.POST("/ratelimit/clear", handlers.ClearRateLimits)
		admin.GET("/ratelimit/keys", handlers.AdminListAPIKeys)
		admin.POST("/ratelimit/keys/:id/regenerate", handlers.AdminRegenerateAPIKey)
		admin.PATCH("/ratelimit/keys/:id", handlers.AdminUpdateAPIKey)
		admin.DELETE("/ratelimit/keys/:id", handlers.AdminDeleteAPIKey)
		// 按把封禁 / 解封
		admin.POST("/ratelimit/keys/:id/ban", handlers.AdminBanAPIKey)
		admin.POST("/ratelimit/keys/:id/unban", handlers.AdminUnbanAPIKey)
		// 按人管控：封禁该用户全部 Key + 专属限流配额
		admin.GET("/ratelimit/users", handlers.AdminListUserAPIPolicies)
		admin.GET("/ratelimit/users/:id/quota", handlers.AdminGetUserAPIQuota)
		admin.PUT("/ratelimit/users/:id/quota", handlers.AdminSetUserAPIQuota)
		admin.DELETE("/ratelimit/users/:id/quota", handlers.AdminDeleteUserAPIPolicy)
		admin.POST("/ratelimit/users/:id/keys/ban", handlers.AdminBanUserAPIKeys)
		admin.POST("/ratelimit/users/:id/keys/unban", handlers.AdminUnbanUserAPIKeys)
		// 群组管理
		admin.GET("/groups", grouphandler.AdminListGroups)
		admin.POST("/groups/:id/ban", grouphandler.AdminBanGroup)
		admin.DELETE("/groups/:id", grouphandler.AdminDeleteGroup)
		// 群组额度：全局 + 按人（独立子树 /group-quota，避 /groups/:id 通配冲突）
		admin.GET("/group-quota", grouphandler.AdminGetGroupQuota)
		admin.PUT("/group-quota", grouphandler.AdminSetGroupQuota)
		admin.PUT("/group-quota/users/:id", grouphandler.AdminSetUserGroupQuota)
		// 论坛管理
		admin.GET("/forums", handlers.AdminListForums)
		admin.POST("/forums", handlers.AdminCreateForum)
		admin.PUT("/forums/:id", handlers.AdminUpdateForum)
		admin.DELETE("/forums/:id", handlers.AdminDeleteForum)
		admin.POST("/forums/:id/review", handlers.AdminReviewForum)
		// 自建论坛额度：全局默认 + 按人覆盖（削减/放开某人的建版数）
		// 独立子树 /forum-quota：避免与 /forums/:id 通配段冲突导致路由注册 panic
		admin.GET("/forum-quota", handlers.GetForumQuotaConfig)
		admin.PUT("/forum-quota", handlers.UpdateForumQuotaConfig)
		admin.GET("/forum-quota/users", handlers.ListUserForumQuotas)
		admin.PUT("/forum-quota/users/:id", handlers.SetUserForumQuota)
		admin.DELETE("/forum-quota/users/:id", handlers.DeleteUserForumQuota)
		// 每日发文限额 / 用户配额体系（§13.4）：等级×每日数量/字数 + 按人覆盖
		// 独立子树 /daily-publish：静态段，规避与既有 :id 通配段冲突
		admin.GET("/daily-publish/config", handlers.AdminGetDailyPublishConfig)
		admin.PUT("/daily-publish/config", handlers.AdminSetDailyPublishConfig)
		admin.GET("/daily-publish/overrides", handlers.AdminListPublishOverrides)
		admin.POST("/daily-publish/overrides", handlers.AdminManagePublishOverride)
		// 代币账本 / 卡密管理（§7）：配置 + 生成/作废卡密 + 授信扣减
		// 独立子树 /token：静态段，避让 :id 通配冲突
		admin.GET("/token/config", handlers.AdminGetTokenConfig)
		admin.PUT("/token/config", handlers.AdminSetTokenConfig)
		admin.GET("/token/cards", handlers.AdminListCards)
		admin.POST("/token/cards", handlers.AdminGenerateCards)
		admin.POST("/token/cards/:id/void", handlers.AdminVoidCard)
		admin.POST("/token/adjust", handlers.AdminAdjustToken)
		// AI 模型池（§5.3）：总开关 + 模型 CRUD（api_key 加密存储、只写不读）
		// 独立子树 /ai：静态段，避让 :id 通配冲突
		admin.GET("/ai/config", handlers.AdminGetAiConfig)
		admin.PUT("/ai/config", handlers.AdminSetAiConfig)
		admin.GET("/ai/models", handlers.AdminListAiModels)
		// AI 路由策略（最便宜/最快/场景）+ 失败降级开关
		admin.GET("/ai/route", handlers.GetAiRouteConfig)
		admin.PUT("/ai/route", handlers.UpdateAiRouteConfig)
		admin.GET("/ai/presets", handlers.AdminAiModelPresets)

		// API 卡池（多密钥轮询）
		admin.GET("/ai/models/:id/keys", handlers.AdminListAiModelKeys)
		admin.POST("/ai/models/:id/keys", handlers.AdminCreateAiModelKey)
		admin.PUT("/ai/models/:id/keys/:keyId", handlers.AdminUpdateAiModelKey)
		admin.DELETE("/ai/models/:id/keys/:keyId", handlers.AdminDeleteAiModelKey)
		admin.POST("/ai/models/:id/keys/test", handlers.AdminTestAiModelKey)

		// 风格字典 / 扣子默认配置
		admin.GET("/ai/styles", handlers.AdminListAiStyles)
		admin.POST("/ai/styles", handlers.AdminCreateAiStyle)
		admin.PUT("/ai/styles/:id", handlers.AdminUpdateAiStyle)
		admin.DELETE("/ai/styles/:id", handlers.AdminDeleteAiStyle)
		admin.GET("/ai/coze-default", handlers.AdminGetAiCozeDefault)
		admin.PUT("/ai/coze-default", handlers.AdminSetAiCozeDefault)

		// AI 功能开关 / 厂商字典 / 数据审计
		// 提示词模板引擎（CRUD / 版本 / 回滚 / 预览）
		admin.GET("/ai/templates", prompthandler.AdminList)
		admin.POST("/ai/templates", prompthandler.AdminCreate)
		admin.PUT("/ai/templates/:id", prompthandler.AdminUpdate)
		admin.DELETE("/ai/templates/:id", prompthandler.AdminDelete)
		admin.GET("/ai/templates/:id/versions", prompthandler.AdminVersions)
		admin.POST("/ai/templates/:id/restore", prompthandler.AdminRestore)
		// 预览用独立路径：/ai/templates/preview 会与 /ai/templates/:id 的通配段冲突（gin httprouter 限制）
		admin.POST("/ai/template-preview", prompthandler.AdminPreview)

		admin.GET("/ai/features", handlers.AdminGetAiFeatures)
		admin.PUT("/ai/features", handlers.AdminSetAiFeatures)
		admin.GET("/ai/vendors", handlers.AdminListAiVendors)
		admin.POST("/ai/vendors", handlers.AdminCreateAiVendor)
		admin.PUT("/ai/vendors/:id", handlers.AdminUpdateAiVendor)
		admin.DELETE("/ai/vendors/:id", handlers.AdminDeleteAiVendor)
		admin.GET("/ai/audit/stats", handlers.AdminAiAuditStats)
		admin.GET("/ai/audit/sessions", handlers.AdminAiAuditSessions)
		admin.GET("/ai/audit/sessions/:id/messages", handlers.AdminAiAuditSessionMessages)
		admin.GET("/ai/audit/media", handlers.AdminAiAuditMedia)
		admin.GET("/ai/audit/calls", handlers.AdminAiAuditCalls)

		// 个人信息泄漏提示统计（仅类型计数，无内容）
		admin.GET("/security/leak-hint/stats", handlers.AdminLeakHintStats)
		admin.POST("/ai/models", handlers.AdminCreateAiModel)
		admin.PUT("/ai/models/:id", handlers.AdminUpdateAiModel)
		admin.DELETE("/ai/models/:id", handlers.AdminDeleteAiModel)
		// AI 智能体管理（§5.4）：预设增删改
		admin.GET("/ai/agents", handlers.AdminListAiAgents)
		admin.POST("/ai/agents", handlers.AdminCreateAiAgent)
		admin.PUT("/ai/agents/:id", handlers.AdminUpdateAiAgent)
		admin.DELETE("/ai/agents/:id", handlers.AdminDeleteAiAgent)

		// 免费 AI 额度（§5.3）：平台默认 + 按人覆盖（静态子树避让 :id 通配）
		admin.GET("/ai/free-quota/config", handlers.AdminGetAiFreeQuotaConfig)
		admin.PUT("/ai/free-quota/config", handlers.AdminSetAiFreeQuotaConfig)
		admin.GET("/ai/free-quota/overrides", handlers.AdminListAiFreeQuotaOverrides)
		admin.POST("/ai/free-quota/overrides", handlers.AdminManageAiFreeQuotaOverride)

		// @ai 评论机器人配置（§5.6）
		admin.GET("/ai/bot/config", handlers.AdminGetAiBotConfig)
		admin.PUT("/ai/bot/config", handlers.AdminSetAiBotConfig)
		// 在线支付通道（§7.4）：配置（密钥只写不读）/ 档位 / 订单列表 / 手动补单
		admin.GET("/payment/config", handlers.AdminGetPaymentConfig)
		admin.PUT("/payment/config", handlers.AdminSetPaymentConfig)
		admin.PUT("/payment/tiers", handlers.AdminSetPaymentTiers)
		admin.GET("/payment/orders", handlers.AdminListPaymentOrders)
		admin.POST("/payment/orders/mark-paid", handlers.AdminMarkOrderPaid)
		// 应用扩展安全审核：预检未通过的 ZIP 原样封存（不解压），管理员放行/驳回
		admin.GET("/userhtmls", handlers.AdminListHTMLs)
		admin.GET("/userhtmls/:id/scan", handlers.AdminGetHTMLScan)
		admin.POST("/userhtmls/:id/review", handlers.AdminReviewHTML)
		// 自定义主页审核（作者公开页提交 → 站长/管理员复核原文）
		admin.GET("/home-customize/reviews", handlers.ListHomeCustomizeReviews)
		admin.POST("/home-customize/reviews/:id/review", handlers.ReviewHomeCustomize)
		// XSS 正则规则库（管理员维护）
		admin.GET("/xss-rules", handlers.ListXssRules)
		admin.POST("/xss-rules", handlers.CreateXssRule)
		admin.PUT("/xss-rules/:id", handlers.UpdateXssRule)
		admin.DELETE("/xss-rules/:id", handlers.DeleteXssRule)
		admin.POST("/xss-rules/test", handlers.TestXssRule)
		// 违禁词库（管理员维护：标记/删除/替换 + 关键词/正则双模式 + 可配置适用范围）
		admin.GET("/bad-words", handlers.ListBadWords)
		admin.POST("/bad-words", handlers.CreateBadWord)
		admin.PUT("/bad-words/:id", handlers.UpdateBadWord)
		admin.DELETE("/bad-words/:id", handlers.DeleteBadWord)
		admin.POST("/bad-words/test", handlers.TestBadWord)
		// 违禁词标记待审（mark 动作命中记录，管理员确认/放行）
		admin.GET("/bad-word-flags", handlers.ListBadWordFlags)
		admin.PUT("/bad-word-flags/:id", handlers.HandleBadWordFlag)
		// 内容抢救台（二级密码 + 应急预览/硬删除；unlock 加 IP 限流防爆破）
		admin.POST("/emergency/set-password", handlers.SetEmergencyPassword)
		admin.POST("/emergency/unlock", security.RateLimit(10, 60), handlers.UnlockEmergency)
		admin.POST("/emergency/preview", handlers.PreviewEmergencyContent)
		admin.POST("/emergency/content", handlers.DeleteEmergencyContent)
		admin.GET("/emergency/audit", handlers.ListEmergencyAudit)
		// 内容治理：删除/封禁任意内容（软删除 + softdel_ 留证）
		admin.GET("/softdel", handlers.AdminListSoftDel)
		admin.POST("/softdel", handlers.AdminModerateContent)
		admin.POST("/softdel/restore", handlers.AdminRestoreContent)
		// 回收站监管：管理员查看全站用户自删内容
		admin.GET("/recycle", handlers.AdminListRecycle)
		// 博客分类管理（一/二/三级；管理员审核视图 + 事后审核 + 创建/编辑/删除）
		admin.GET("/blog-categories", handlers.AdminListBlogCategories)
		admin.GET("/blog-categories/mode", handlers.AdminGetCategoryReviewMode)
		admin.PUT("/blog-categories/mode", handlers.AdminSetCategoryReviewMode)
		admin.POST("/blog-categories", handlers.CreateBlogCategory)
		admin.PUT("/blog-categories/:id", handlers.UpdateBlogCategory)
		admin.DELETE("/blog-categories/:id", handlers.DeleteBlogCategory)
		admin.POST("/blog-categories/:id/review", handlers.AdminReviewBlogCategory)
		// 小说/章节封禁管理（管理员视角列表 + 封禁 + 解封）
		admin.GET("/novels", handlers.AdminListNovels)
		admin.GET("/novels/:id/chapters", handlers.AdminListChapters)
		// 批量重算所有作品统计（修复 total_chapters / total_words 脏数据）
		// 注意：不可注册在 /novels 下，否则与 /novels/:id/chapters 的通配段冲突导致路由 panic。
		admin.POST("/recompute-novel-stats", handlers.AdminRecomputeNovelStats)
		// 数据库管理（健康检查 / 迁移 / 备份 / 恢复 / 定时备份）
		admin.GET("/db/health", handlers.AdminDBHealth)
		admin.GET("/db/current", handlers.AdminDBCurrent)
		admin.POST("/db/migrate", handlers.AdminDBMigrate)
		admin.POST("/db/backup", handlers.AdminDBBackup)
		admin.GET("/db/job/:id", handlers.AdminDBJob)
		admin.GET("/db/backups", handlers.AdminDBListBackups)
		admin.DELETE("/db/backups/:name", handlers.AdminDBDeleteBackup)
		admin.GET("/db/backups/:name/download", handlers.AdminDBDownloadBackup)
		admin.POST("/db/restore", handlers.AdminDBRestore)
		admin.GET("/db/backup-config", handlers.AdminDBBackupConfigGet)
		admin.PUT("/db/backup-config", handlers.AdminDBBackupConfigPut)
	}

	// 警方取管台（police）：准管理员角色，仅可取证 / 查看软删除，无普通管理权
	// 不可自注册，只能由管理员通过 /api/admin/users/:id/promote-police 提权。
	police := r.Group("/api/police")
	// 🔒 纵深防御：组级强制 police/admin 角色
	police.Use(middleware.AuthRequired(), middleware.PoliceRequired())
	{
		police.GET("/softdel", handlers.PoliceListSoftDel)       // 软删除留证列表（取证查看）
		police.GET("/forums", handlers.PoliceListForums)         // 论坛巡查：全部论坛（含封锁/加密/隐藏/软删除）
		police.GET("/evidence", handlers.PoliceGetEvidence)      // 单条取证凭证详情（?type=&id=）
		police.GET("/download", handlers.PoliceDownloadEvidence) // 下载单条取证凭证（?type=&id=）
		police.GET("/del-color", handlers.PoliceGetDelColor)     // 已删除内容高亮色（警方可读）
		police.PUT("/del-color", handlers.PoliceSetDelColor)     // 已删除内容高亮色（仅管理员可写）
		police.GET("/export", handlers.PoliceExportPackage)      // 导出全部取证包
	}

	// 远程站点公开接口
	{
		r.GET("/api/federated/novels", handlers.ListFederatedNovels)
		r.GET("/api/federated/sites", handlers.ListPublicSites)
		r.GET("/api/site-info", handlers.GetSiteInfo)
		// 精读：公开笔记（游客可看公开批注）
		r.GET("/api/reading/notes/public", handlers.ListPublicNotes)
		// 统一搜索：作品 / 作者 / 博客 / HTML 扩展
		r.GET("/api/search", handlers.UnifiedSearch)
		// 博客公开读取
		r.GET("/api/blogs", handlers.ListPublicBlogs) // 支持 category_id / tag 筛选
		r.GET("/api/blogs/:id", handlers.GetBlog)
		r.GET("/api/blogs/:id/likes", handlers.GetBlogLikes)       // 博客点赞数+当前用户状态
		r.GET("/api/blog-tags", handlers.ListBlogTags)             // 全站标签云
		r.GET("/api/topics", handlers.ListTopics)                  // 全站话题列表
		r.GET("/api/topics/:id", handlers.GetTopic)                // 话题详情
		r.GET("/api/topics/:id/blogs", handlers.ListTopicBlogs)    // 话题下博客列表
		r.GET("/api/blog-categories", handlers.ListBlogCategories) // 博客分类树（公开）
		r.GET("/api/author/:id/blogs/:blogId", handlers.GetAuthorBlog)
		r.GET("/api/author/:id/blogs", handlers.ListAuthorBlogs)
		r.GET("/api/author/:id/blog-categories", handlers.ListAuthorBlogCategories) // 作者个人专栏（已公开分类）

		// 沙盒预览 & WASM
		r.GET("/api/sandbox/info", handlers.GetSandboxInfo)
		r.GET("/api/userframes/:id/preview", handlers.GetFramePreview)
		r.GET("/api/userhtmls/:id/preview", handlers.GetHTMLPreview)
		r.GET("/api/userhtmls/:id/download", handlers.DownloadHTMLZip)
		r.GET("/api/htmls/public", handlers.ListPublicHTMLs)
		r.GET("/api/author/:id/htmls", handlers.GetAuthorPublicHTMLs)
	}

	// 🔀 小说路径重定向：旧格式 → 新格式 /author/:authorId/novel/:id
	r.GET("/novel/:id/read/:chapter", handlers.RedirectNovelReader)

	// 🔒 虚拟沙盒路径：隐藏真实存储路径，沙盒内无法探测 /api/userhtmls/
	r.GET("/app/:htmlId", handlers.ServeAppHTML)
	r.GET("/app/:htmlId/*filepath", handlers.ServeAppResource)

	// ==================== 静态文件服务 ====================

	// 注意：小说正文不再以 StaticFS 直链暴露。章节内容统一经 GetChapterContent
	// （/api/novels/:id/chapters/:num）按访客门禁（3.1/3.2/3.3）与签名校验后返回，
	// 避免匿名用户/爬虫绕过登录直接下载原始 .html 全文。沙盒代理对 /novels 的读取走
	// proxy.go 的文件处理器，与本静态路由无关，不受影响。

	sandboxGroup := r.Group("/sandbox")
	sandboxGroup.Use(
		middleware.SandboxFrameHeaders(),
		middleware.WasmContentType(),
	)
	sandboxGroup.StaticFS("/userframe", gin.Dir(config.UserFrameDir, false))
	sandboxGroup.StaticFS("/userhtml", gin.Dir(config.UserHTMLDir, false))
	sandboxGroup.GET("/frame/:userID/*filepath", handlers.ServeSandboxPreview)
	// 端口反向代理（/sandbox/proxy/:port/*path） + 严格安全
	// 命名代理（/sandbox/proxy/{authorId}/{projectName}/*path）— 隐藏端口号防打洞
	proxyGroup := r.Group("/sandbox/proxy")
	proxyGroup.Use(
		middleware.PortProxySecurity(),
		middleware.SandboxWriteProtect(),
	)
	proxyGroup.Any("/:authorId/:projectName/*path", middleware.NamedProxyHandler())

	// 上传资源静态文件
	r.StaticFS("/uploads", gin.Dir(config.UploadDir, false))

	// 嵌入的前端 (SPA)
	frontendFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Fatalf("前端资源加载失败（dist 目录不存在，请先运行 npm run build）: %v", err)
	}

	// 注入前端资源探针（供 /health 检查前端 SPA 是否可服务）
	handlers.FrontendProbe = func() (string, bool) {
		indexBytes, err := fs.ReadFile(frontendFS, "index.html")
		if err != nil {
			return "", false
		}
		re := frontendBundleRe.FindString(string(indexBytes))
		if re == "" {
			return "", false
		}
		if _, err := fs.Stat(frontendFS, "assets/"+re); err != nil {
			return re, false
		}
		return re, true
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// 蜜罐：可疑探路（.php/.env/admin/backup…）静默记录，不改变响应
		honeypot.ProbeNotFound(c)

		// 1) 未匹配路由的 /api/* 一律 404（JSON），绝不落入 SPA fallback。
		//    防止任意 /api/xxx（含被反向代理污染的前缀，如 /api/go-nvs/...，以及 ../ 穿越）被当作前端路由返回 200。
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": utils.CodeNotFound, "message": "接口不存在", "data": nil})
			return
		}

		// 2) 拒绝路径穿越：URL 中含 ".." 的请求一律 404（防止目录穿越/路径混淆）
		if strings.Contains(path, "..") {
			c.Status(http.StatusNotFound)
			return
		}

		// 3) 安装完成后：建站向导根段（install / setup，含任意子路径）一律 404。
		//    用首段匹配而非精确匹配，堵住 /install/xxx、/setup/xxx 等子路径穿越
		//    （此前 path=="/install" 精确匹配，/install/xxx 会绕过并落入 SPA 白名单返回 200）。
		if config.IsInstalled() {
			seg := firstPathSegment(strings.TrimPrefix(path, "/"))
			if seg == "install" || seg == "setup" {
				c.Status(http.StatusNotFound)
				return
			}
		}

		// robots.txt（站长可在后台自定义，兼容尾斜杠/大小写变体）
		normalizedPath := strings.ToLower(strings.TrimSuffix(path, "/"))
		if normalizedPath == "/robots.txt" || normalizedPath == "/robot.txt" {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(models.GetRobotsTxt()))
			return
		}

		// SPA 路由请求（Vue Router），检查是否匹配到前端静态资源
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// 根路径 / index.html：显式返回并注入站点路径前缀（供前端 router/API base 自适应）
		if cleanPath == "index.html" {
			indexBytes, err := fs.ReadFile(frontendFS, "index.html")
			if err != nil {
				c.String(http.StatusInternalServerError, "前端资源缺失")
				return
			}
			// index.html 不缓存：每次重新验证，保证发布新版（新 hash 资源）后浏览器立即拿到
			c.Header("Cache-Control", "no-cache")
			c.Data(http.StatusOK, "text/html; charset=utf-8",
				middleware.InjectPrefixIntoHTML(indexBytes, config.URLPrefix))
			return
		}

		// 尝试打开请求的文件（嵌入式文件系统中路径不带前导 /）
		f, err := frontendFS.Open(cleanPath)
		if err == nil {
			f.Close()
			// 静态资源缓存策略（解决浏览器每次刷新全量重复下载 js/css）：
			// - assets/*（Vite 产物，文件名带内容 hash）：immutable 永久缓存——
			//   文件名即指纹，内容不变 URL 不变，浏览器直接命中缓存不再发请求
			// - 其余静态文件（favicon 等无 hash 文件名）：短缓存 + Last-Modified/ETag 条件验证
			if strings.HasPrefix(cleanPath, "assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				c.Header("Cache-Control", "public, max-age=3600")
			}
			http.FileServer(http.FS(frontendFS)).ServeHTTP(c.Writer, c.Request)
			return
		}

		// 严格 SPA 回退白名单：仅允许已知前端路由根段回退到 index.html；
		// 其余（未配置前缀时的 /go-nvs/xxx、数字前缀 /123/xxx、以及任意未知前缀）一律 404，
		// 彻底禁止任何形式的代理/前缀造成的 /install 等组件穿越。
		if !knownSPARoots[firstPathSegment(cleanPath)] {
			c.Status(http.StatusNotFound)
			return
		}

		// 文件不存在 → SPA fallback（注入站点路径前缀）
		indexContent, err := fs.ReadFile(frontendFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "前端资源缺失")
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8",
			middleware.InjectPrefixIntoHTML(indexContent, config.URLPrefix))
	})

	// ==================== 启动服务器 ====================

	// 构建监听地址
	var listenAddr string
	if config.EnableIPv6 {
		// IPv6 双栈：[::] 在 Windows 上需要 syscall 设置才能同时接受 IPv4
		listenAddr = fmt.Sprintf("[%s]:%s", config.BindIPv6, config.ServerPort)
	} else {
		listenAddr = fmt.Sprintf("%s:%s", config.BindIPv4, config.ServerPort)
	}

	log.Println("========================================")
	log.Printf("  NVS Server v%s", version.AppVersion)
	log.Printf("  监听地址: %s", listenAddr)
	log.Println("========================================")

	// 列出所有可访问的 URL
	listAccessURLs(config.ServerPort)

	// 尝试配置 Windows 防火墙（静默进行，失败不影响启动）
	configureFirewall(config.ServerPort)

	// 启动定时备份调度器（按平台配置 interval 自动备份）
	models.StartBackupScheduler()

	// 启动访问日志归档调度器（每日 sha256 + gzip + 只读 + 6个月保留，176号令合规）
	models.StartLogArchiver()

	// 🔀 站点路径前缀（URL_PREFIX）在 gin 外层剥离：必须先于路由匹配执行。
	// 未配置前缀时该包装零开销直通。
	rootHandler := middleware.StripURLPrefixHandler(config.URLPrefix, r)
	srv := &http.Server{Addr: listenAddr, Handler: rootHandler}
	log.Println("前端已内嵌，无需额外启动 Nginx 或 npm run dev（支持路径前缀挂载，如 /go-nvs/login）")

	// 🔐 国密双轨监听装配：国际轨默认（零国密依赖）；gm/dual 轨经 crypto.Provider
	// 叠加 Tongsuo（CGO）或纯 Go gmtls 的 TLCP 能力（未配证书时保持纯 HTTP 现状）。
	listener, err := buildServiceListener(listenAddr)
	if err != nil {
		log.Fatalf("监听器构建失败: %v", err)
	}
	if err := srv.Serve(listener); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

// buildServiceListener 构建服务监听器：按密码轨道叠加 TLS/TLCP 能力。
func buildServiceListener(listenAddr string) (net.Listener, error) {
	inner, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	provider, err := crypto.New(config.CryptoCfg)
	if err != nil {
		_ = inner.Close()
		return nil, err
	}
	ln, err := provider.BuildListener(inner, config.CryptoCfg)
	if err != nil {
		_ = inner.Close()
		return nil, err
	}
	if ln == net.Listener(inner) {
		log.Printf("  TLS: 未启用（纯 HTTP；反向代理终止 TLS 或未配置证书）")
	} else {
		log.Printf("  TLS: 已启用（provider=%s, track=%s）", provider.Name(), config.CryptoCfg.Track)
	}
	return ln, nil
}

// listAccessURLs 扫描所有网卡 IP 并输出可访问的 URL
func listAccessURLs(port string) {
	ips := collectNonLoopbackIPs()
	if len(ips) == 0 {
		return
	}

	log.Println("----------------------------------------")
	log.Println("  可访问地址：")
	log.Printf("    本机:   http://localhost:%s", port)
	log.Printf("    本机:   http://127.0.0.1:%s", port)
	for _, ip := range ips {
		isV6 := strings.Contains(ip, ":")
		if isV6 {
			log.Printf("    局域网: http://[%s]:%s", ip, port)
		} else {
			log.Printf("    局域网: http://%s:%s", ip, port)
		}
	}
	log.Println("----------------------------------------")
}

// collectNonLoopbackIPs 收集所有非回环的网卡 IP 地址
func collectNonLoopbackIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var ips []string
	for _, iface := range ifaces {
		// 跳过未启用的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		// 跳过回环接口
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			// 优先私有地址
			if ip.To4() != nil && ip.IsPrivate() {
				ips = append(ips, ip.String())
			} else if ip.IsGlobalUnicast() {
				ips = append(ips, ip.String())
			}
		}
	}
	return ips
}

// configureFirewall 尝试在 Windows 防火墙上为 nvs-server.exe 开放端口
func configureFirewall(port string) {
	if runtime.GOOS != "windows" {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		return
	}

	ruleName := "NVS Server (nvs-server.exe)"

	// 检查规则是否已存在
	checkCmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule",
		"name="+ruleName)
	if err := checkCmd.Run(); err == nil {
		return // 规则已存在，跳过
	}

	// 尝试添加入站规则（需要管理员权限）
	addCmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+ruleName,
		"dir=in",
		"action=allow",
		"program="+exePath,
		"protocol=TCP",
		"localport="+port,
		"enable=yes",
		"profile=any",
	)

	output, err := addCmd.CombinedOutput()
	if err != nil {
		// 没有管理员权限，给出友好提示
		log.Println("----------------------------------------")
		log.Println("  ⚠ 未能自动配置 Windows 防火墙（可能需要管理员权限）")
		log.Println("  请以管理员身份运行以下命令以允许外部访问：")
		log.Printf("    netsh advfirewall firewall add rule name=\"NVS Server\" dir=in action=allow program=\"%s\" protocol=TCP localport=%s enable=yes profile=any", exePath, port)
		log.Println("----------------------------------------")
		return
	}
	_ = output
	log.Println("[firewall] Windows 防火墙规则已添加")
}

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

// initDatabase 连接数据库、执行迁移、初始化默认数据与默认管理员
func initDatabase() error {
	var dialector gorm.Dialector
	switch config.DBDriver {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			config.DBUser, config.DBPassword, config.DBHost, config.DBPort, config.DBName)
		dialector = mysql.Open(dsn)
		log.Println("使用 MySQL 数据库")
	case "sqlite":
		dbPath := config.DBPath
		if dbPath == "" {
			dbPath = "data/nvs.db"
		}
		if config.DBMemory {
			dbPath = ":memory:"
			log.Println("使用 SQLite 内存数据库（重启数据丢失）")
		} else {
			os.MkdirAll(filepath.Dir(dbPath), 0755)
			log.Printf("使用 SQLite 数据库: %s", dbPath)
		}
		dialector = sqlite.Open(dbPath)
	default:
		os.MkdirAll("data", 0755)
		dialector = sqlite.Open("data/nvs.db")
		log.Println("使用 SQLite 数据库: data/nvs.db")
	}

	var err error
	models.DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: utils.NewDBLogger(),
	})
	if err != nil {
		return err
	}

	if config.DBDriver == "mysql" {
		sqlDB, _ := models.DB.DB()
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
		// MySQL 模式 → 自动启用 Redis 缓存
		utils.ConnectRedis(config.RedisHost, config.RedisPort, config.RedisPassword, config.RedisDB)
	}

	if config.DBDriver == "sqlite" && config.DBMemory {
		log.Println("[SQLite] 内存模式：验证码和限流使用进程内存储")
	}

	// 短信网关（可插拔：log/http）与手机验证码存储后端（Redis/内存，随 ConnectRedis 决定）
	sms.Init()
	log.Printf("[sms] 验证码存储后端: %s", utils.PhoneCodeBackend())

	if err := models.AutoMigrateAll(models.DB); err != nil {
		return err
	}
	// 群组模块（独立功能包）建表：紧随主迁移，避免 models 包反向依赖子包
	if err := groups.Migrate(models.DB); err != nil {
		return err
	}
	// 提示词模板引擎（独立功能包）：建表 + 类型文学默认框架种子（幂等）
	if err := prompttpl.Migrate(models.DB); err != nil {
		return err
	}
	prompttpl.InitSeeds()
	// 多人协作（独立功能包）：协作成员 + 协作审计
	if err := collab.Migrate(models.DB); err != nil {
		return err
	}
	// 章节版本控制（独立功能包）：提交历史 + 分支指针
	if err := chaptervcs.Migrate(models.DB); err != nil {
		return err
	}
	// OC 模式（独立功能包）：角色卡 / 世界书 / 群内 AI 角色 / 对话流 / 审核窗口 / 预算
	if err := oc.Migrate(models.DB); err != nil {
		return err
	}
	// 蜜罐（独立功能包）：等级与日志策略 + 会话/事件/指纹
	// 设 NVS_HONEYPOT_DB_PATH 则使用独立库（与业务库物理隔离，后台仍可读）
	if err := honeypot.Bootstrap(models.DB); err != nil {
		return err
	}
	// 注册门槛（邀请码/设备指纹/账号年龄）+ 充值风控 + 警方工单 + OC 表情
	if err := gate.Migrate(models.DB); err != nil {
		return err
	}
	if err := risk.Migrate(models.DB); err != nil {
		return err
	}
	if err := police.Migrate(models.DB); err != nil {
		return err
	}
	if err := oc.MigrateEmote(models.DB); err != nil {
		return err
	}
	// 自定义表情注册到渲染器（气泡由服务端渲染，需全局表；失败不影响启动）
	oc.ReloadCustomEmotes()
	scriptmode.RegisterCustomEmotes(oc.CustomEmoteMap())
	// 版本控制后端选择：NVS_VCS_BACKEND=git 时启用真实 git 仓库（需 -tags gitvcs 构建），
	// 失败或不可用自动回落自研版本链，不阻断启动
	chaptervcs.Init()
	models.InitDefaultForums()
	models.InitPlatformConfigs()
	models.SeedDefaultAiAgents() // AI 智能体内置种子（表为空时写入）
	models.InitAiPrompts()       // AI 提示词库平台预设（幂等）
	models.SeedAiVendors()       // AI 厂商字典（幂等）
	models.SeedAiStyles()        // 作品风格字典（幂等）
	// 管理端配置（DB）覆盖 .env 初始值：微信扫码凭据热更新进 config 包级变量
	// （sms 网关由 resolveGateway 每次发送前读 DB，无需在此处理）
	if wxID, wxSecret, wxCallback, fromDB := models.GetWeChatConfigFromDB(); fromDB {
		config.WeChatAppID = wxID
		config.WeChatAppSecret = wxSecret
		config.WeChatCallback = wxCallback
		log.Println("[wechat] 已从数据库恢复管理端保存的微信扫码配置")
	}
	// 限流与 API Key：连库后重载管理员配置（含自定义爬虫规则），并清理过期访问日志
	security.ReloadBotRules()
	models.CleanupAccessLogs(models.LoadAPIRateLimitConfig().LogRetentionDays)
	// 日志脱敏：注入数据库持久化钩子并加载已保存的配置（后台从未保存过则使用 .env 默认）
	config.RedactMgr.SetPersistence(models.LoadRedactConfig, models.StoreRedactConfig)
	config.RedactMgr.LoadFromDB()
	handlers.ReloadXssRules() // 加载管理员配置的 XSS 正则规则
	handlers.ReloadBadWords() // 加载管理员配置的违禁词库规则
	initDefaultAdmin()
	log.Println("数据库迁移完成")
	return nil
}

// initDefaultAdmin 如果数据库中没有管理员，自动创建默认管理员账号
func initDefaultAdmin() {
	var count int64
	models.DB.Model(&models.User{}).Where("role = ?", "admin").Count(&count)
	if count > 0 {
		return
	}

	hash, err := utils.HashPassword("admin123")
	if err != nil {
		log.Println("警告: 默认管理员密码哈希失败")
		return
	}

	admin := &models.User{
		Username:     "admin",
		Email:        "admin@nvs.local",
		PasswordHash: hash,
		Nickname:     "管理员",
		Role:         "admin",
	}

	if err := models.DB.Create(admin).Error; err != nil {
		log.Printf("警告: 默认管理员创建失败: %v", err)
		return
	}

	log.Println("========================================")
	log.Println("  默认管理员账号已创建")
	log.Println("  邮箱: admin@nvs.local")
	log.Println("  密码: admin123")
	log.Println("  请登录后立即修改密码！")
	log.Println("========================================")
}
