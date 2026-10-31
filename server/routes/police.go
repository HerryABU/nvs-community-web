package routes

// routes.go — 全部 HTTP 路由注册集中在此（从 main.go 原样搬出，未改动任何路由）。
//
// 为什么单独成包：main.go 此前 1852 行、近 1000 行都是路由注册，改一个接口要在
// 上千行里翻；而 main 真正的职责只有「初始化 → 装配 → 启动」。
// 搬出来后 main 只留一行 routes.Register(r)，路由的增删改都在这一个文件里。
//
// 依赖方向是单向的：main → routes → handlers/models/middleware，不存在回环。

import (
	"nvs-server/handlers"
	"nvs-server/middleware"

	"github.com/gin-gonic/gin"
)

// 警方取管台（独立前缀 /api/police，治理白名单内）。
func registerPolice(r *gin.Engine) {
	police := r.Group("/api/police")
	// 🔒 纵深防御：组级强制 police/admin 角色
	police.Use(middleware.AuthRequired(), middleware.PoliceRequired())
	{
		police.GET("/softdel", handlers.PoliceListSoftDel)       // 软删除留证列表（取证查看）
		police.GET("/forums", handlers.PoliceListForums)         // 论坛巡查：全部论坛（含封锁/加密/隐藏/软删除）
		police.GET("/evidence", handlers.PoliceGetEvidence)      // 单条取证凭证详情（?type=&id=）
		police.GET("/download", handlers.PoliceDownloadEvidence) // 下载单条取证凭证（?type=&id=）
		police.GET("/del-color", handlers.PoliceGetDelColor)     // 已删除内容高亮色（警方可读）
		// 舆情面板：只给聚合计数（按天/按决策/按话题），不给被拦截内容正文
		police.GET("/sentiment/stats", handlers.PoliceSentimentStats)
		police.GET("/sentiment/trend", handlers.PoliceSentimentTrend)
		police.PUT("/del-color", handlers.PoliceSetDelColor) // 已删除内容高亮色（仅管理员可写）
		police.GET("/export", handlers.PoliceExportPackage)  // 导出全部取证包
		// 警方内容治理镜像（与站长内容治理板块同口径，留证仅含操作者用户名、不含 PII；
		// Snapshot 等敏感截面在 handler 内已置空，合规与安全敏感信息警方不可见）
		police.GET("/governance/overview", handlers.PoliceGovernanceOverview)
		police.GET("/governance/banned", handlers.PoliceGovernanceList)
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

}
