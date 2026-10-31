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
	"nvs-server/security"

	"github.com/gin-gonic/gin"
	chaptervcshandler "nvs-server/handlers/chaptervcs"
	collabhandler "nvs-server/handlers/collab"
	grouphandler "nvs-server/handlers/group"
	ochandler "nvs-server/handlers/oc"
	prompthandler "nvs-server/handlers/prompts"
	scripthandler "nvs-server/handlers/script"
)

// 登录后路由（AuthRequired）。
func registerProtected(r *gin.Engine) {
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
		// 「我的」筛选：最近用过的模型（模型广场的第四个筛选维度）
		protected.GET("/ai/my-models", handlers.AiMyModels)
		// 智能体投稿：用户提交特殊提示词模板 → 站长审核 → 广场上架
		protected.POST("/ai/agents/submit", handlers.AiSubmitAgent)
		protected.GET("/ai/agents/my-submissions", handlers.AiMyAgentSubmissions)
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
		// 对话分支：切到指定消息所在分支（不删任何消息，只改显示位置）
		protected.POST("/ai/sessions/:id/switch", handlers.AiHubSwitchBranch)
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
		// AIGC 标识出口管道（GB 45438—2025）：
		// 用户下载必须走这里 —— 服务端合成、烧显式角标、写隐式 AIGC 字段，前端不碰像素。
		protected.GET("/ai/aigc/options", handlers.AiAigcOptions)
		protected.POST("/ai/aigc/export", security.RateLimit(20, 60), handlers.AiAigcExport)

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
		// 话本气泡体：作品角色管理（头像 / 主题色 / 默认左右）
		protected.GET("/novels/:id/characters", scripthandler.ListCharacters)
		protected.POST("/novels/:id/characters", scripthandler.CreateCharacter)
		protected.PUT("/novels/:id/characters/:cid", scripthandler.UpdateCharacter)
		protected.DELETE("/novels/:id/characters/:cid", scripthandler.DeleteCharacter)
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
		// AI 工作台附件：图片/文档上传（存盘不解析，发消息时原样内联给上游 API）
		protected.POST("/ai/attachment", handlers.AiAttachmentUpload)
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
}
