package routes

// routes.go — 全部 HTTP 路由注册集中在此（从 main.go 原样搬出，未改动任何路由）。
//
// 为什么单独成包：main.go 此前 1852 行、近 1000 行都是路由注册，改一个接口要在
// 上千行里翻；而 main 真正的职责只有「初始化 → 装配 → 启动」。
// 搬出来后 main 只留一行 routes.Register(r)，路由的增删改都在这一个文件里。
//
// 依赖方向是单向的：main → routes → handlers/models/middleware，不存在回环。

import (
	"nvs-server/handlers/admin"
	"nvs-server/handlers/aiadmin"
	"nvs-server/handlers/aimedia"
	"nvs-server/handlers/auth"
	"nvs-server/handlers/author"
	"nvs-server/handlers/batch"
	"nvs-server/handlers/captcha"
	"nvs-server/handlers/comment"
	"nvs-server/handlers/compliance"
	"nvs-server/handlers/follow"
	"nvs-server/handlers/forum"
	"nvs-server/handlers/gate"
	"nvs-server/handlers/geo"
	"nvs-server/handlers/home"
	"nvs-server/handlers/live"
	"nvs-server/handlers/novel"
	"nvs-server/handlers/ops"
	"nvs-server/handlers/payment"
	"nvs-server/handlers/rating"
	"nvs-server/handlers/sandbox"
	"nvs-server/handlers/site"
	"nvs-server/handlers/theme"
	"nvs-server/handlers/web"
	"nvs-server/middleware"
	"nvs-server/security"

	grouphandler "nvs-server/handlers/group"

	"github.com/gin-gonic/gin"
)

// 公开路由（/api 下无需登录即可访问，但会识别登录态）。
func registerPublic(r *gin.Engine) {
	// ==================== API 路由 ====================

	public := r.Group("/api")
	public.Use(middleware.OptionalAuth()) // 公开路由也识别登录态（管理员豁免/我的论坛等权限判断）
	{
		public.POST("/auth/register", security.RateLimit(10, 60), auth.Register) // 安全审计 M2：注册限流防批量灌号
		// 注册门槛提示与预检（邀请码/设备指纹/账号年龄，gap-audit #14）
		public.GET("/auth/register-hint", gate.RegisterGateHint)
		public.POST("/auth/register-check", security.RateLimit(20, 60), gate.RegisterGateCheck)
		// 运营位（gap-audit #20 #23）：导航菜单 / 轮播 / 签到状态 / 访客设置
		public.GET("/nav-menus", ops.PublicNavMenus)
		public.GET("/carousels", ops.PublicCarousels)
		public.GET("/checkin/status", ops.CheckInStatus)
		public.GET("/visit-setting", ops.PublicVisitSetting)
		// 直播入口（gap-audit #23）：只做入口与嵌入，不提供推流
		public.GET("/live", ops.PublicLive)
		public.POST("/auth/login", security.RateLimit(5, 60), auth.Login)
		public.POST("/auth/logout", auth.Logout)
		// 🔒 无感续期：校验 HttpOnly 刷新令牌 Cookie，签发新的访问令牌（不要求已登录）
		public.POST("/auth/refresh", auth.Refresh)
		// 🔒 滑块验证码挑战（货真价实服务端校验）：先 issue 拿到拼图，再 verify 拿 pass token，
		// 后续 /send-code、/register 必须携带该 token，否则拒绝（防裸调/邮箱轰炸）。
		public.POST("/auth/captcha/issue", captcha.IssueCaptcha)
		public.POST("/auth/captcha/verify", captcha.VerifyCaptcha)
		public.POST("/auth/send-code", auth.SendVerificationCode)
		// 手机号登录/注册（F10）：验证码经可插拔短信网关发送，存储支持 Redis/内存双模式
		public.POST("/auth/phone/send-code", security.RateLimit(5, 60), auth.SendPhoneCode)
		public.POST("/auth/phone/login", security.RateLimit(10, 60), auth.PhoneLogin)
		public.POST("/auth/phone/reset-password", security.RateLimit(5, 60), auth.PhoneResetPassword)
		public.POST("/auth/verify-code", auth.VerifyEmailCode)
		public.POST("/auth/forgot-password", security.RateLimit(3, 60), auth.ForgotPassword)
		public.POST("/auth/reset-password", security.RateLimit(3, 60), auth.ResetPassword)
		// 微信扫码登录（F21.2）：授权 URL 获取 + OAuth 回调（免鉴权，state 一次性防 CSRF）
		public.GET("/auth/wechat/url", security.RateLimit(10, 60), auth.WeChatAuthStart)
		public.GET("/auth/wechat/callback", auth.WeChatCallback)
		public.GET("/categories", novel.ListCategories)
		public.GET("/categories/stats", novel.ListCategoryStats)
		public.GET("/wall-zone/:zone", admin.GetPublicZoneDetail)
		public.GET("/novels", security.RateLimit(100, 60), novel.ListNovels)
		public.GET("/novels/:id", novel.GetNovel)
		public.GET("/novels/:id/chapters", novel.GetChapters)
		public.GET("/novels/:id/chapters/:num", novel.GetChapterContent)
		public.GET("/novels/:id/chapters/:num/verify", novel.VerifyChapter)
		public.GET("/comments", comment.GetComments)
		// 楼加密（入口限制）：单楼整读（公开，楼内校验密码）；验证/设密需登录态
		public.GET("/comments/:id/detail", forum.GetFloorDetail)
		public.GET("/reports/reasons", admin.GetReportReasons)
		// API 聚合中间层（一次请求批量获取多个接口数据，向后兼容原有独立调用）
		public.POST("/batch", security.RateLimit(30, 60), batch.BatchHandler)
		public.GET("/novels/:id/rating", rating.GetNovelRating)
		public.GET("/forums", forum.ListForums)
		// 群组（公开/半公开可浏览；私密仅成员）
		public.GET("/groups", grouphandler.GroupList)
		public.GET("/groups/:id", grouphandler.GroupDetail)
		public.GET("/group-lookup", grouphandler.GroupSearchByNo) // 群号/邀请码预览（独立路径，避开 /groups/:id 通配）
		public.GET("/forums/:id", forum.GetForum)
		public.GET("/novels/:id/forum", forum.GetNovelForum)
		public.GET("/threads/:id", forum.GetThread)
		public.GET("/author/profile/:id", author.GetAuthorProfile)
		public.GET("/users/resolve", author.ResolveUserByKey) // 用户标识解析（id/用户名，数字是主键）
		public.GET("/author/forum/:id", author.GetAuthorForum)
		// 公开的「被访问者粉丝/关注数」（注意与 protected 的 /follow/stats 区分：后者是浏览者自己的口径）
		public.GET("/author/:id/follow-stats", follow.GetAuthorFollowStats)
		public.GET("/search/authors", novel.SearchAuthors)
		public.GET("/userframes/public", web.ListPublicFrames)
		public.GET("/template/novel/:id", web.GetTemplateAPI)
		public.GET("/novels/:id/frames", web.GetNovelFrames)
		public.GET("/blogs/:id/frames", web.GetBlogFrames)
		public.GET("/author/:id/frames", web.GetAuthorFrames)
		public.GET("/novels/:id/htmls", web.GetNovelHTMLs)
		public.GET("/blogs/:id/htmls", web.GetBlogHTMLs)
		// 176号令合规：公开算法说明（第七条第九项 算法透明度）+ 站点公告栏
		public.GET("/algorithms/description", compliance.GetAlgoDescription)
		public.GET("/announcements", admin.ListAnnouncements)
		// 自定义主页（F15）：公开区块列表 + 全站最新帖（论坛首页风格区块）
		public.GET("/home-blocks", home.ListPublicHomeBlocks)
		public.GET("/threads/latest", home.ListLatestThreads)
		// 狂暴模式（F19）：SSE 评论实时流 + 公开状态查询（管理员开关，默认关）
		public.GET("/live/stream", live.LiveStream)
		public.GET("/live/config", live.LiveConfig)

		// AI 会话分享：公开只读快照（令牌即凭据，无登录要求）
		public.GET("/ai/shared/:token", aimedia.AiSharedSession)
		// 灵感广场：公开作品流（作者主动发布才可见）
		public.GET("/ai/gallery", aimedia.AiGalleryList)
		// 风格字典（发布作品时下拉用）
		public.GET("/ai/styles", aimedia.AiStyleList)
		// AI 生成内容标识（合规展示：只回模型名，不含内容）
		public.GET("/ai/marks", aimedia.AiMarks)
		// 在线支付回调（§7.4）：网关异步通知（验签准入，无鉴权）+ 支付完成同步跳转
		public.POST("/payment/notify/:channel", security.RateLimit(120, 60), payment.PaymentNotify)
		public.GET("/payment/return/:channel", payment.PaymentReturn)
		// 自定义网站主题（F12：alist 式，服务端消毒后全局生效）
		public.GET("/theme/site", theme.GetSiteTheme)
		// F12a 全站 opt-in iframe（管理员配置允许列表 URL，访客可读启用状态）
		public.GET("/site-iframe", site.GetSiteIframe)
		// 首页形态（novels/blogs/forum/feed）：前端据此决定首页渲染哪套视图
		public.GET("/home-view", home.GetHomeView)
		// 给用户看的模型价目表（只含启用中模型与单价，不含上游地址/密钥）
		public.GET("/ai/pricing", aiadmin.AiPublicPricing)
		// 智能体投稿开关（前台据此决定是否显示「投稿」入口；投稿本身需登录）
		public.GET("/ai/agents/submit-config", aiadmin.AiAgentSubmitConfig)
		// AI 工作台左栏自定义菜单（站长设置，前台只取已启用项）
		public.GET("/ai/nav-items", aimedia.AiNavItems)
		// 地图服务配置（标准地图合规：offline 示意 / 天地图 国内+全球）
		public.GET("/map-setting", geo.GetMapSetting)
		// F12b 作者主题（访客可见，仅 CSS，服务端消毒）
		public.GET("/author/:id/theme", author.GetAuthorTheme)
		// 自定义主页（作者公开页，访客可见已审核片段）
		public.GET("/author/:id/home-customize", home.GetAuthorHomeCustomize)
		// 沙盒虚拟文件系统
		public.GET("/sandbox-vfs/:htmlId/read", sandbox.VFSRead)
		public.POST("/sandbox-vfs/:htmlId/write", sandbox.VFSWrite)
		public.DELETE("/sandbox-vfs/:htmlId/delete", sandbox.VFSDelete)
		public.GET("/sandbox-vfs/:htmlId/list", sandbox.VFSList)
	}

}
