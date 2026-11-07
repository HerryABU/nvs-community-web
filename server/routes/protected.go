package routes

// routes.go — 全部 HTTP 路由注册集中在此（从 main.go 原样搬出，未改动任何路由）。
//
// 为什么单独成包：main.go 此前 1852 行、近 1000 行都是路由注册，改一个接口要在
// 上千行里翻；而 main 真正的职责只有「初始化 → 装配 → 启动」。
// 搬出来后 main 只留一行 routes.Register(r)，路由的增删改都在这一个文件里。
//
// 依赖方向是单向的：main → routes → handlers/models/middleware，不存在回环。

import (
	"nvs-server/handlers/account"
	"nvs-server/handlers/admingov"
	"nvs-server/handlers/adminsite"
	"nvs-server/handlers/aiadmin"
	"nvs-server/handlers/aimedia"
	"nvs-server/handlers/aiplatform"
	"nvs-server/handlers/aiworkbench"
	"nvs-server/handlers/apikey"
	"nvs-server/handlers/auth"
	"nvs-server/handlers/author"
	authoraih "nvs-server/handlers/authorai"
	"nvs-server/handlers/blacklist"
	"nvs-server/handlers/blog"
	"nvs-server/handlers/bookshelf"
	"nvs-server/handlers/comment"
	"nvs-server/handlers/compliance"
	"nvs-server/handlers/data"
	"nvs-server/handlers/export"
	"nvs-server/handlers/follow"
	"nvs-server/handlers/forum"
	"nvs-server/handlers/home"
	"nvs-server/handlers/media"
	"nvs-server/handlers/message"
	"nvs-server/handlers/notification"
	"nvs-server/handlers/novel"
	"nvs-server/handlers/ops"
	"nvs-server/handlers/payment"
	"nvs-server/handlers/police"
	"nvs-server/handlers/port"
	"nvs-server/handlers/publish"
	"nvs-server/handlers/rating"
	"nvs-server/handlers/reading"
	readpermh "nvs-server/handlers/readperm"
	"nvs-server/handlers/recycle"
	"nvs-server/handlers/scriptpreview"
	securityhandler "nvs-server/handlers/security"
	"nvs-server/handlers/theme"
	"nvs-server/handlers/token"
	"nvs-server/handlers/upload"
	vipcardh "nvs-server/handlers/vipcard"
	"nvs-server/handlers/web"
	"nvs-server/middleware"
	"nvs-server/security"

	chaptervcshandler "nvs-server/handlers/chaptervcs"
	collabhandler "nvs-server/handlers/collab"
	grouphandler "nvs-server/handlers/group"
	ochandler "nvs-server/handlers/oc"
	prompthandler "nvs-server/handlers/prompts"
	scripthandler "nvs-server/handlers/script"

	"github.com/gin-gonic/gin"
)

// 登录后路由（AuthRequired）。
func registerProtected(r *gin.Engine) {
	protected := r.Group("/api")
	protected.Use(middleware.AuthRequired())
	{
		protected.GET("/auth/me", auth.GetCurrentUser)
		protected.POST("/auth/change-password", auth.ChangePassword)
		// 实名制（F10/F21.1）：申请/状态/国家实名核验代理（姓名+身份证不落库明文）
		protected.POST("/auth/realname/apply", security.RateLimit(5, 60), auth.ApplyRealName)
		protected.GET("/auth/realname", auth.GetMyRealName)
		protected.POST("/auth/realname/national-check", security.RateLimit(5, 60), auth.NationalRealNameCheck)
		// 未成年检测（F21.1）：出生日期自报 + 状态查询（strict 分级限制在发布入口门禁）
		protected.POST("/auth/birthdate", security.RateLimit(10, 60), auth.SetBirthDate)
		protected.GET("/auth/minor-status", auth.GetMinorStatus)
		// 微信账号绑定（F21.2）：绑定状态 / 绑定 / 解绑
		protected.GET("/auth/wechat/bind-status", auth.WeChatBindStatus)
		protected.POST("/auth/wechat/bind", auth.WeChatBind)
		protected.POST("/auth/wechat/unbind", auth.WeChatUnbind)
		// 自定义网站主题（F12）：个人主题读取 / 保存（服务端消毒）
		protected.GET("/theme/me", theme.GetMyTheme)
		protected.PUT("/theme/me", theme.UpdateMyTheme)
		// F12b 作者主题（本人或管理员可写，访客可见）
		protected.PUT("/author/:id/theme", author.UpdateAuthorTheme)
		// 自定义主页（alist 式单框代码 + 双作用域 + 站长审核）
		protected.GET("/me/home-customize", home.GetMyHomeCustomize)
		protected.PUT("/me/home-customize", home.UpdateMyHomeCustomize)
		protected.PUT("/author/:id/home-customize", home.UpdateAuthorHomeCustomize)
		// web 项目额度（F17）：个人额度查询 + 设为公开（触发超额审核）
		protected.GET("/web-quota/mine", web.MyWebQuota)
		protected.POST("/web-quota/html-public", web.SetHTMLPublic)
		protected.POST("/web-quota/frame-public", web.SetFramePublic)
		// 每日发文限额（§13.4）：当前用户今日发文额度（发布表单展示剩余量）
		protected.GET("/daily-publish/status", publish.GetMyPublishQuota)
		// 代币钱包（§7）：状态 / 流水 / 卡密兑换
		protected.GET("/token/status", token.TokenStatus)
		protected.GET("/token/ledger", token.TokenLedgerList)
		protected.POST("/token/redeem", token.RedeemCard)
		// 在线支付充值（§7.4）：档位下单 / 订单轮询 / 我的订单 / 模拟支付确认
		protected.GET("/payment/status", payment.PaymentStatus)
		protected.POST("/payment/order", security.RateLimit(10, 60), payment.PaymentCreateOrder)
		protected.GET("/payment/order/:no", payment.PaymentOrderStatus)
		protected.GET("/payment/orders", payment.PaymentMyOrders)
		protected.POST("/payment/mock/confirm", security.RateLimit(20, 60), payment.PaymentMockConfirm)
		// 会员套餐：状态 + 在售套餐 + 支付可用性（会员页一次拉齐）
		protected.GET("/membership", payment.MembershipInfo)
		// AI 调用（§5.3）：可用性/预估/补全计费（限流防刷）
		protected.GET("/ai/status", aiworkbench.AiStatus)
		protected.POST("/ai/estimate", security.RateLimit(60, 60), aiworkbench.AiEstimate)
		protected.POST("/ai/complete", security.RateLimit(20, 60), aiworkbench.AiComplete)
		// AI 工作台（§5.4）：会话 / 流式对话 / 绘画 / 视频 / 智能体
		protected.GET("/ai/hub/status", aiworkbench.AiHubStatus)
		protected.GET("/ai/agents", aiadmin.AiAgentList)
		protected.GET("/ai/free-quota", aiplatform.AiFreeQuotaStatus)

		// 用户自备 API Key（§5.5 BYOK）：CRUD + 默认（:id 通配与既有 /ai/video/:id 不同前缀段，无冲突）
		protected.GET("/ai/my-keys", account.AiMyKeys)
		// 「我的」筛选：最近用过的模型（模型广场的第四个筛选维度）
		protected.GET("/ai/my-models", aiadmin.AiMyModels)
		// 智能体投稿：用户提交特殊提示词模板 → 站长审核 → 广场上架
		protected.POST("/ai/agents/submit", aiadmin.AiSubmitAgent)
		protected.GET("/ai/agents/my-submissions", aiadmin.AiMyAgentSubmissions)
		protected.POST("/ai/my-keys", account.AiMyKeyCreate)
		protected.DELETE("/ai/my-keys/:id", account.AiMyKeyDelete)
		protected.POST("/ai/my-keys/:id/default", account.AiMyKeySetDefault)

		// 长篇 AI 记忆（§13.5）：大纲/章节摘要/上下文组装（:id 通配在不同前缀段，无冲突）
		protected.GET("/ai/memory", aiworkbench.AiMemoryList)
		protected.PUT("/ai/memory/outline", aiworkbench.AiMemorySaveOutline)
		protected.PUT("/ai/memory/characters", aiworkbench.AiMemorySaveCharacters)
		protected.PUT("/ai/memory/summary", aiworkbench.AiMemorySaveSummary)
		protected.DELETE("/ai/memory/summary", aiworkbench.AiMemoryDeleteSummary)
		protected.POST("/ai/memory/context", aiworkbench.AiMemoryContext)

		// @ai 评论区机器人（§5.6）：状态查询（配置在管理端）
		protected.GET("/ai/bot/status", aiadmin.AiBotStatus)
		protected.GET("/ai/sessions", aiworkbench.AiHubSessions)
		protected.GET("/ai/sessions/:id/messages", aiworkbench.AiHubSessionMessages)
		// 对话分支：切到指定消息所在分支（不删任何消息，只改显示位置）
		protected.POST("/ai/sessions/:id/switch", aiworkbench.AiHubSwitchBranch)
		protected.DELETE("/ai/sessions/:id", aiworkbench.AiHubDeleteSession)
		protected.PUT("/ai/sessions/:id", aimedia.AiHubRenameSession)
		protected.POST("/ai/sessions/:id/share", aimedia.AiHubShareSession)
		protected.DELETE("/ai/sessions/:id/share", aimedia.AiHubUnshareSession)
		protected.POST("/ai/chat/stream", security.RateLimit(30, 60), aiworkbench.AiHubChatStream)
		protected.POST("/ai/image", security.RateLimit(20, 60), aimedia.AiHubImage)
		protected.POST("/ai/image/optimize-prompt", security.RateLimit(20, 60), aiworkbench.AiOptimizePrompt)
		protected.POST("/ai/video", security.RateLimit(10, 60), aimedia.AiHubVideoSubmit)
		protected.GET("/ai/video/:id", aimedia.AiHubVideoStatus)
		// 取消异步视频任务：置终态停止等待 + 已扣款原路退回
		// 注意路径形态：POST 下已有 /ai/video 静态段，这里必须继续往下挂 :id 子节点，
		// 不能与静态段同级加通配（gin 会 panic: conflicts with existing wildcard）
		protected.POST("/ai/video/:id/cancel", aimedia.AiHubVideoCancel)
		protected.GET("/ai/media", aimedia.AiHubMediaHistory)
		// 出图附件上传 + 历史导出（gap-audit #17）
		// 注意：/ai/media 下不能加静态段（已有 /ai/media 精确路由 + /ai/media/:id/... 通配），
		// 所以历史导出放 /ai/export/media.zip 独立子树
		protected.POST("/ai/media/attach", security.RateLimit(30, 60), aimedia.AttachMedia)
		protected.GET("/ai/media/:id/export", aimedia.ExportMediaTask)
		protected.GET("/ai/export/media.zip", aimedia.ExportMediaHistory)
		// AIGC 标识出口管道（GB 45438—2025）：
		// 用户下载必须走这里 —— 服务端合成、烧显式角标、写隐式 AIGC 字段，前端不碰像素。
		protected.GET("/ai/aigc/options", aimedia.AiAigcOptions)
		protected.POST("/ai/aigc/export", security.RateLimit(20, 60), aimedia.AiAigcExport)

		// AI 提示词库：平台预设（只读）+ 我的自定义（增删改）
		protected.GET("/ai/prompts", aiworkbench.AiPromptList)
		protected.POST("/ai/prompts", aiworkbench.AiPromptCreate)
		protected.PUT("/ai/prompts/:id", aiworkbench.AiPromptUpdate)
		protected.DELETE("/ai/prompts/:id", aiworkbench.AiPromptDelete)
		protected.POST("/ai/prompts/:id/use", aiworkbench.AiPromptUse)

		// 灵感广场：发布/取消发布、点赞；思维导图生成
		protected.POST("/ai/gallery/:id/publish", aimedia.AiGalleryPublish)
		protected.POST("/ai/gallery/:id/like", aimedia.AiGalleryLike)
		protected.GET("/ai/media/:id/public", aimedia.AiMyMediaPublic)
		protected.POST("/ai/mindmap", security.RateLimit(10, 60), aimedia.AiMindmap)

		// 个人信息泄漏提示：只累计命中类型，不接收内容（检测不留痕）
		protected.POST("/security/leak-hint", securityhandler.RecordLeakHint)
		// 用户黑名单（F13：拉黑禁言/屏蔽内容）
		protected.POST("/blacklist", security.RateLimit(20, 60), blacklist.AddBlacklist)
		protected.DELETE("/blacklist/:id", blacklist.RemoveBlacklist)
		protected.GET("/blacklist", blacklist.ListMyBlacklist)
		// 私信（F20：会话消息；互关才可发第 3 条，可在设置中关闭）
		protected.POST("/messages", security.RateLimit(30, 60), message.SendMessage)
		protected.GET("/messages/conversations", message.ListConversations)
		protected.GET("/messages/with/:userId", message.ListMessages)
		protected.POST("/messages/read", message.MarkMessageRead)
		protected.GET("/messages/unread-count", message.UnreadMessageCount)
		protected.GET("/messages/settings", message.GetMessageSettings)
		protected.PUT("/messages/settings", message.UpdateMessageSettings)
		protected.POST("/messages/recall", message.RecallMessage)
		protected.POST("/messages/conversation/delete", message.DeleteConversation)
		// 用户 API Key 管理（创建/查看/修改/重生成/删除，配额在管理员角色上限内自设）
		protected.GET("/apikeys", apikey.ListMyAPIKeys)
		protected.POST("/apikeys", apikey.CreateAPIKey)
		protected.PATCH("/apikeys/:id", apikey.UpdateAPIKey)
		protected.POST("/apikeys/:id/regenerate", apikey.RegenerateAPIKey)
		protected.DELETE("/apikeys/:id", apikey.DeleteAPIKey)
		// 站内消息通知
		protected.GET("/notifications", notification.ListNotifications)
		protected.GET("/notifications/unread-count", notification.UnreadNotificationCount)
		protected.POST("/notifications/read", notification.MarkNotificationRead)
		// 精读：书签 + 批注
		protected.POST("/reading/notes", reading.CreateReadingNote)
		protected.GET("/reading/notes", reading.ListMyReadingNotes)
		protected.GET("/reading/notes/mine", reading.ListAllMyNotes)
		protected.PATCH("/reading/notes/:id", reading.UpdateReadingNote)
		protected.DELETE("/reading/notes/:id", reading.DeleteReadingNote)
		protected.POST("/comments", security.RateLimit(20, 60), comment.CreateComment)
		protected.DELETE("/comments/:id", comment.DeleteComment)
		// 楼加密（入口限制）：验证访问密码 / 设置·修改·解除密码（限速防爆破）
		protected.POST("/comments/:id/verify-password", security.RateLimit(10, 60), forum.VerifyFloorPassword)
		protected.POST("/comments/:id/password", security.RateLimit(10, 60), forum.SetFloorPassword)
		// @ 提及来源设置 + 联想候选（站长/楼主可配置，评论框联想用）
		protected.GET("/mention-source", comment.GetMentionSource)
		protected.PUT("/mention-source", comment.SetMentionSource)
		protected.GET("/mention-candidates", comment.GetMentionCandidates)
		// 评论网（社交图谱）分析：谁回复了谁的哪条评论 + 时间估计（可按 user_id 下钻）
		protected.GET("/comment-network", comment.GetCommentNetwork)
		protected.POST("/novels", novel.CreateNovel)
		protected.PUT("/novels/:id", novel.UpdateNovel)
		protected.DELETE("/novels/:id", novel.DeleteNovel)
		protected.POST("/novels/:id/chapters", novel.CreateChapter)
		protected.PUT("/novels/:id/chapters/:num", novel.UpdateChapter)
		protected.DELETE("/novels/:id/chapters/:num", novel.DeleteChapter)
		// 章节工作流（多人协作）：提审 / 审核 / 直接发布 / 撤回
		protected.POST("/novels/:id/chapters/:num/submit", novel.SubmitChapterReview)
		protected.POST("/novels/:id/chapters/:num/review", novel.ReviewChapter)
		protected.POST("/novels/:id/chapters/:num/publish", novel.PublishChapter)
		protected.POST("/novels/:id/chapters/:num/withdraw", novel.WithdrawChapter)
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
		protected.POST("/script-preview", scriptpreview.ScriptPreview)
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
		protected.POST("/groups/:id/oc/invoke", ochandler.InvokeOcInGroup) // 群组召唤 OC 角色发言（强化群组 + 群组调用 OC）
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
		protected.GET("/author/novels", author.GetMyNovels)
		protected.GET("/author/novels/:id/stats", author.GetNovelStats)
		protected.GET("/author/banned-content", author.GetAuthorBannedContent)
		protected.GET("/author/recycle", recycle.ListAuthorRecycle)             // 回收站：用户自删内容
		protected.POST("/author/recycle/restore", recycle.RestoreAuthorRecycle) // 恢复自己删除的内容
		protected.GET("/author/dashboard", author.GetAuthorDashboard)
		protected.POST("/novels/:id/export", novel.ExportNovel)
		// #76 衍生同人列表（原著作者的粉丝/作者本人可见）
		protected.GET("/novels/:id/derivatives", novel.ListDerivatives)

		// 作者工作台内置 AI 写作流（与 AI 中台数据隔离）
		protected.GET("/author-ai/projects", authoraih.ListProjects)
		protected.POST("/author-ai/projects", authoraih.CreateProject)
		protected.GET("/author-ai/projects/:id", authoraih.GetProject)
		protected.PUT("/author-ai/projects/:id", authoraih.UpdateProject)
		protected.DELETE("/author-ai/projects/:id", authoraih.DeleteProject)
		protected.POST("/author-ai/projects/:id/generate", authoraih.GenerateComponent)
		protected.POST("/author-ai/projects/:id/generate-all", authoraih.GenerateAllMaterials)
		protected.GET("/author-ai/projects/:id/timeline-check", authoraih.TimelineCheck)
		// ADRO 创作向：引子 → 拆解成独立细节卡 → 逐细节多轮微调
		protected.GET("/author-ai/projects/:id/details", authoraih.ListDetails)
		protected.POST("/author-ai/projects/:id/details", authoraih.CreateDetail)
		protected.PUT("/author-ai/projects/:id/details/:did", authoraih.UpdateDetail)
		protected.DELETE("/author-ai/projects/:id/details/:did", authoraih.DeleteDetail)
		protected.POST("/author-ai/projects/:id/from-seed", authoraih.FromSeed)
		protected.POST("/author-ai/projects/:id/details/:did/refine", authoraih.RefineDetail)
		protected.GET("/author-ai/projects/:id/refine-modes", authoraih.RefineModes)
		protected.GET("/author-ai/projects/:id/chapters", authoraih.ListChapters)
		protected.POST("/author-ai/projects/:id/chapters", authoraih.CreateChapter)
		protected.PUT("/author-ai/projects/:id/chapters/:cid", authoraih.UpdateChapter)
		protected.DELETE("/author-ai/projects/:id/chapters/:cid", authoraih.DeleteChapter)
		protected.POST("/author-ai/projects/:id/chapters/:cid/generate", authoraih.GenerateChapter)
		protected.POST("/author-ai/projects/:id/publish", authoraih.PublishNovel)
		protected.POST("/zone/enter", auth.RecordZoneEnter) // 记录进入隔离墙（敏感分区）时间，176号令合规
		protected.POST("/ratings", rating.UpsertRating)
		protected.GET("/ratings", rating.GetUserRating)
		// 176号令合规：用户偏好设置（第七条第九项 个性化推荐关闭权）
		protected.GET("/users/me/preferences", compliance.GetUserPreferences)
		protected.PUT("/users/me/preferences", compliance.UpdateUserPreferences)
		protected.POST("/forums/:id/threads", forum.CreateThread)
		protected.POST("/threads/:id/posts", forum.CreatePost)
		protected.DELETE("/threads/:id", forum.DeleteThreadByModerator)
		protected.DELETE("/posts/:id", forum.DeletePostByModerator)
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
		protected.POST("/forums", forum.CreateUserForum)
		protected.POST("/forums/enter-by-code", forum.EnterForumByRoomCode) // 房间号进入不可见论坛
		protected.GET("/forums/mine", forum.ListMyForums)
		protected.POST("/forums/:id/verify", forum.VerifyForumPassword)
		protected.POST("/forums/:id/mute", forum.ForumMuteUser)
		protected.POST("/forums/:id/unmute", forum.ForumUnmuteUser)
		protected.GET("/forums/:id/mutes", forum.ListForumMutes) // 论坛禁言列表（创建者/管理员）
		// VIP & 导入导出
		protected.POST("/author/apply-vip", adminsite.ApplyVip)
		protected.POST("/novels/import/preview", data.ImportPreview)
		protected.POST("/novels/import", data.ImportNovel)
		protected.POST("/novels/import/batch", data.ImportBatch)
		protected.POST("/novels/:id/export/epub", export.ExportEPUB)
		protected.POST("/novels/:id/export/markdown", export.ExportMarkdown)
		protected.POST("/novels/:id/export/txt", export.ExportTXT)
		protected.POST("/novels/:id/export/docx", export.ExportDOCX)
		// 举报
		protected.POST("/reports", admingov.CreateReport)
		protected.GET("/reports/mine", admingov.ListMyReports)
		// 收益
		protected.GET("/author/earnings", adminsite.GetEarnings)
		protected.POST("/author/withdraw", adminsite.RequestWithdraw)
		// 作者头像上传
		protected.POST("/author/avatar", author.UploadAvatar)
		// 作者签名状态
		protected.GET("/author/signature-status", author.GetSignatureStatus)
		// 作者上传图片（重新编码防马）
		protected.POST("/author/image", media.UploadImage)
		// 作者更新资料（签名、隔离开关）
		protected.PUT("/author/profile", media.UpdateAuthorProfile)
		// 论坛置顶
		protected.POST("/threads/:id/pin", forum.PinThread)
		protected.POST("/threads/:id/unpin", forum.UnpinThread)
		// 关注作者
		protected.POST("/follow/:id", follow.FollowAuthor)
		protected.DELETE("/follow/:id", follow.UnfollowAuthor)
		protected.GET("/following", follow.ListFollowing)
		protected.GET("/followers", follow.ListFollowers)
		protected.GET("/follow/check/:id", follow.CheckFollow)
		protected.GET("/follow/stats", follow.GetFollowStats)
		// 特别关注 + 动态流（F14）
		protected.POST("/follows/:id/special", follow.ToggleSpecialFollow)
		protected.GET("/follows/special", follow.ListMySpecialFollows)
		protected.GET("/feed/special", follow.GetSpecialFeed)
		// 作者博客（创建、编辑、删除需登录）
		protected.POST("/blogs", blog.CreateBlog)
		protected.POST("/topics", blog.CreateTopic) // 创建话题（登录）
		protected.POST("/blogs/import/batch", blog.ImportBlogs)
		protected.POST("/blogs/batch-category", blog.BatchSetBlogCategory) // 批量设置博客分类（CSDN 式）
		protected.PUT("/blogs/:id", blog.UpdateBlog)
		protected.DELETE("/blogs/:id", blog.DeleteBlog)
		protected.POST("/blogs/:id/like", blog.ToggleBlogLike) // 博客点赞/取消
		protected.POST("/upload", upload.UploadFile)           // 通用上传：博客图片/音视频
		// AI 工作台附件：图片/文档上传（存盘不解析，发消息时原样内联给上游 API）
		protected.POST("/ai/attachment", aiworkbench.AiAttachmentUpload)
		// 博客分类管理（登录作者/管理员均可：新增/编辑/删除/挂靠，支持三级；普通作者创建=待审核）
		protected.GET("/blog-categories/manage", blog.ListMyBlogCategories)
		protected.POST("/blog-categories", blog.CreateBlogCategory)
		protected.PUT("/blog-categories/:id", blog.UpdateBlogCategory)
		protected.DELETE("/blog-categories/:id", blog.DeleteBlogCategory)
		// 用户自定义 UI-HTML 模板 (UserFrame)
		protected.GET("/userframes", web.ListFrames)
		// F16 站内内容相对路径引用选择器（CSDN 式）
		protected.GET("/frame/refs", web.ListContentRefs)
		protected.POST("/userframes", web.CreateFrame)
		protected.GET("/userframes/:id", web.GetFrame)
		protected.PUT("/userframes/:id", web.UpdateFrame)
		protected.DELETE("/userframes/:id", web.DeleteFrame)
		// 用户自定义 HTML (UserHTML)
		protected.GET("/userhtmls", web.ListHTMLs)
		protected.GET("/userhtmls/:id", web.GetHTML)
		protected.PUT("/userhtmls/:id", web.UpdateHTML)
		protected.DELETE("/userhtmls/:id", web.DeleteHTML)
		protected.POST("/userhtmls/upload", web.UploadZipHTML)
	}

	// 端口分配API
	portGrp := r.Group("/api/port")
	portGrp.Use(middleware.AuthRequired())
	{
		portGrp.POST("/allocate", port.AllocatePort)
		portGrp.POST("/release", port.ReleasePort)
		portGrp.GET("/list", port.ListPorts)
	}

	// 书架路由
	bookshelfGrp := r.Group("/api/bookshelf")
	bookshelfGrp.Use(middleware.AuthRequired())
	{
		bookshelfGrp.GET("", bookshelf.ListShelf)
		bookshelfGrp.POST("", bookshelf.AddToShelf)
		bookshelfGrp.DELETE("/:id", bookshelf.RemoveFromShelf)
		bookshelfGrp.GET("/check/:id", bookshelf.CheckShelf)
		bookshelfGrp.POST("/progress", bookshelf.UpdateShelfProgress)

		// ===== 运营位（gap-audit #20 #23）=====
		protected.POST("/checkin", ops.DoCheckIn)
		// ===== 语音合成 / 音乐（gap-audit #18）=====
		protected.POST("/ai/speech", security.RateLimit(10, 60), aimedia.Speech)
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
		protected.GET("/police/tickets", police.ListTickets)
		protected.POST("/police/tickets", police.CreateTicket)
		protected.GET("/police/tickets/:id", police.GetTicket)
		protected.POST("/police/tickets/:id/assign", police.AssignTicket)
		protected.POST("/police/tickets/:id/status", police.SetTicketStatus)
		protected.POST("/police/tickets/:id/notes", police.AddTicketNote)

		// #65 作者端阅读权限下拉（随站长配置实时变化）
		protected.GET("/read-permission/levels", readpermh.Levels)

		// #66 VIP 等级卡密：兑换（限流防爆破）与我的会员状态
		protected.POST("/vip-cards/redeem", security.RateLimit(10, 60), vipcardh.Redeem)
		protected.GET("/vip-cards/membership", vipcardh.MyMembership)

		// #67 付费内容解锁：代币购买 + 解锁状态查询
		protected.POST("/readperm/purchase", security.RateLimit(10, 60), readpermh.Purchase)
		protected.GET("/readperm/purchase/status", readpermh.PurchaseStatus)
	}

	// 管理员路由
}
