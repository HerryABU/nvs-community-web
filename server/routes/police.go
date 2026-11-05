package routes

// routes.go — 全部 HTTP 路由注册集中在此（从 main.go 原样搬出，未改动任何路由）。
//
// 为什么单独成包：main.go 此前 1852 行、近 1000 行都是路由注册，改一个接口要在
// 上千行里翻；而 main 真正的职责只有「初始化 → 装配 → 启动」。
// 搬出来后 main 只留一行 routes.Register(r)，路由的增删改都在这一个文件里。
//
// 依赖方向是单向的：main → routes → handlers/models/middleware，不存在回环。

import (
	"nvs-server/handlers/adminsite"
	"nvs-server/handlers/blog"
	"nvs-server/handlers/novel"
	"nvs-server/handlers/police"
	"nvs-server/handlers/reading"
	"nvs-server/handlers/wasm"
	"nvs-server/handlers/web"
	"nvs-server/middleware"

	"github.com/gin-gonic/gin"
)

// 警方取管台（独立前缀 /api/police，治理白名单内）。
func registerPolice(r *gin.Engine) {
	policeGrp := r.Group("/api/police")
	// 🔒 纵深防御：组级强制 police/admin 角色
	policeGrp.Use(middleware.AuthRequired(), middleware.PoliceRequired())
	{
		policeGrp.GET("/softdel", police.PoliceListSoftDel)       // 软删除留证列表（取证查看）
		policeGrp.GET("/forums", police.PoliceListForums)         // 论坛巡查：全部论坛（含封锁/加密/隐藏/软删除）
		policeGrp.GET("/evidence", police.PoliceGetEvidence)      // 单条取证凭证详情（?type=&id=）
		policeGrp.GET("/download", police.PoliceDownloadEvidence) // 下载单条取证凭证（?type=&id=）
		policeGrp.GET("/del-color", police.PoliceGetDelColor)     // 已删除内容高亮色（警方可读）
		// 舆情面板：只给聚合计数（按天/按决策/按话题），不给被拦截内容正文
		policeGrp.GET("/sentiment/stats", police.PoliceSentimentStats)
		policeGrp.GET("/sentiment/trend", police.PoliceSentimentTrend)
		policeGrp.PUT("/del-color", police.PoliceSetDelColor) // 已删除内容高亮色（仅管理员可写）
		policeGrp.GET("/export", police.PoliceExportPackage)  // 导出全部取证包（警方须先获批）
		// 取证导出申请 / 审批流（以疏代堵对接警方：警方须先申请、站长批准才能导出）
		policeGrp.POST("/evidence-requests", police.PoliceCreateEvidenceRequest) // 警方提交申请
		policeGrp.GET("/evidence-requests", police.PoliceListMyEvidenceRequests) // 警方看自己的申请
		policeGrp.GET("/export-user", police.PoliceExportUserForensics)          // 导出某入档用户取证包
		// 警方内容治理镜像（与站长内容治理板块同口径，留证仅含操作者用户名、不含 PII；
		// Snapshot 等敏感截面在 handler 内已置空，合规与安全敏感信息警方不可见）
		policeGrp.GET("/governance/overview", police.PoliceGovernanceOverview)
		policeGrp.GET("/governance/banned", police.PoliceGovernanceList)
	}

	// 远程站点公开接口
	{
		r.GET("/api/federated/novels", adminsite.ListFederatedNovels)
		r.GET("/api/federated/sites", adminsite.ListPublicSites)
		r.GET("/api/site-info", adminsite.GetSiteInfo)
		// 精读：公开笔记（游客可看公开批注）
		r.GET("/api/reading/notes/public", reading.ListPublicNotes)
		// 统一搜索：作品 / 作者 / 博客 / HTML 扩展
		r.GET("/api/search", novel.UnifiedSearch)
		// 博客公开读取
		r.GET("/api/blogs", blog.ListPublicBlogs) // 支持 category_id / tag 筛选
		r.GET("/api/blogs/:id", blog.GetBlog)
		r.GET("/api/blogs/:id/likes", blog.GetBlogLikes)       // 博客点赞数+当前用户状态
		r.GET("/api/blog-tags", blog.ListBlogTags)             // 全站标签云
		r.GET("/api/topics", blog.ListTopics)                  // 全站话题列表
		r.GET("/api/topics/:id", blog.GetTopic)                // 话题详情
		r.GET("/api/topics/:id/blogs", blog.ListTopicBlogs)    // 话题下博客列表
		r.GET("/api/blog-categories", blog.ListBlogCategories) // 博客分类树（公开）
		r.GET("/api/author/:id/blogs/:blogId", blog.GetAuthorBlog)
		r.GET("/api/author/:id/blogs", blog.ListAuthorBlogs)
		r.GET("/api/author/:id/blog-categories", blog.ListAuthorBlogCategories) // 作者个人专栏（已公开分类）

		// 沙盒预览 & WASM
		r.GET("/api/sandbox/info", wasm.GetSandboxInfo)
		r.GET("/api/userframes/:id/preview", wasm.GetFramePreview)
		r.GET("/api/userhtmls/:id/preview", wasm.GetHTMLPreview)
		r.GET("/api/userhtmls/:id/download", web.DownloadHTMLZip)
		r.GET("/api/htmls/public", web.ListPublicHTMLs)
		r.GET("/api/author/:id/htmls", web.GetAuthorPublicHTMLs)
	}

	// 🔀 小说路径重定向：旧格式 → 新格式 /author/:authorId/novel/:id
	r.GET("/novel/:id/read/:chapter", novel.RedirectNovelReader)

	// 🔒 虚拟沙盒路径：隐藏真实存储路径，沙盒内无法探测 /api/userhtmls/
	r.GET("/app/:htmlId", wasm.ServeAppHTML)
	r.GET("/app/:htmlId/*filepath", wasm.ServeAppResource)

}
