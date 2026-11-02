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
	"nvs-server/handlers/ai"
	"nvs-server/middleware"
	"nvs-server/security"

	"github.com/gin-gonic/gin"
	grouphandler "nvs-server/handlers/group"
	honeypothandler "nvs-server/handlers/honeypot"
	prompthandler "nvs-server/handlers/prompts"
)

// 端口/书架/后台管理路由。
func registerAdminArea(r *gin.Engine) {
	admin := r.Group("/api/admin")
	// 🔒 纵深防御：组级强制 admin 角色（除登录校验外再叠加角色校验），
	// 防止「新增 handler 漏写 ensureAdmin」导致越权。具体写操作仍由各 handler 内部 ensureAdmin 兜底。
	// 警方不再进 /admin：这里改成「admin 全放行 + police 仅白名单（内容治理子集）」。
	// 白名单见 middleware/governance.go，非白名单路径对 police 一律 403 并记审计。
	admin.Use(middleware.AuthRequired(), middleware.AdminOrPoliceGovernance())
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
		admin.GET("/web-access/geo", handlers.GetWebAccessGeo)           // 访问来源地域分布（ip2region 离线解析，供地图/排行）
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
		// AI 内容审核中枢
		admin.GET("/moderation/config", handlers.GetModerationConfig)
		admin.PUT("/moderation/config", handlers.SaveModerationConfig)
		admin.GET("/moderation/records", handlers.ListModerationRecords)
		admin.POST("/moderation/records/:id/decide", handlers.DecideModerationRecord)
		admin.POST("/moderation/batch", handlers.RunModerationBatch)
		admin.GET("/moderation/stats", handlers.GetModerationStats)
		admin.GET("/moderation/malicious", handlers.ListMaliciousReporters)
		admin.POST("/moderation/malicious/:id/unban", handlers.UnbanReporter)
		// HTML 内容审查（人工复核）
		admin.GET("/html-reviews", handlers.ListHTMLReviews)
		admin.GET("/html-reviews/:type/:id", handlers.GetHTMLReview)
		admin.POST("/html-reviews/:type/:id/review", handlers.ReviewHTML)
		// HTML/WASM 托管额度（每人项目数 + 累计空间；支持按人覆盖）
		admin.GET("/html-quota", handlers.AdminGetHTMLQuota)
		admin.PUT("/html-quota", handlers.AdminSaveHTMLQuota)
		admin.POST("/html-quota/overrides", handlers.AdminSetHTMLQuotaOverride)
		admin.GET("/html-quota/usage", handlers.AdminHTMLQuotaUsage)
		// 评论区策略（楼加密是否连带隐藏楼内回复）
		admin.GET("/comment-policy", handlers.AdminGetCommentPolicy)
		admin.PUT("/comment-policy", handlers.AdminSaveCommentPolicy)
		admin.GET("/finance", handlers.GetFinanceOverview)
		// 站长配置
		admin.GET("/config", handlers.GetPlatformConfigs)
		admin.POST("/smtp/test", handlers.TestSMTP) // SMTP 测试与逐阶段诊断
		admin.PUT("/config", handlers.UpdatePlatformConfig)
		// 日志脱敏配置（176号令合规：生产环境日志敏感信息脱敏）
		admin.GET("/log-redact", handlers.GetLogRedactConfig)
		admin.PUT("/log-redact", handlers.UpdateLogRedactConfig)
		// 密码轨道状态（国际轨 / 国密轨 / 双轨）：只读，配置项在 .env，改了要重启
		admin.GET("/crypto/status", handlers.AdminCryptoStatus)
		// 蜜罐等级 + 日志记录字段勾选（docs/honeypot-design.md）
		admin.GET("/honeypot/config", honeypothandler.GetConfig)
		admin.PUT("/honeypot/config", honeypothandler.SaveConfig)
		admin.POST("/honeypot/regenerate-key", honeypothandler.RegenerateKey)
		// 诱饵内容独立保存：只改文案，不碰启用/等级（避免全量覆盖把蜜罐顺手关掉）
		admin.PUT("/honeypot/decoy-profile", honeypothandler.SaveDecoyProfile)
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
		// 首页形态配置（管理员可把博客/论坛/混合流设为首页）
		admin.GET("/home-view", handlers.AdminGetHomeView)
		admin.PUT("/home-view", handlers.AdminSaveHomeView)
		// 地图服务（标准地图合规配置）
		admin.GET("/map-setting", handlers.AdminGetMapSetting)
		admin.PUT("/map-setting", handlers.AdminSaveMapSetting)
		admin.POST("/map-setting/test", handlers.AdminTestMapSetting)
		// 虚拟记账单位 → 真实货币的换算比例（中转站的「美元」只是站内代币）
		admin.GET("/cost-rate", handlers.AdminGetCostRate)
		admin.PUT("/cost-rate", handlers.AdminSaveCostRate)
		admin.POST("/cost-rate/preview", handlers.AdminCostRatePreview)
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
		admin.GET("/ai/audio-models", ai.AdminSpeechModels)
		admin.GET("/ai/funnel", ai.AiFunnel)
		// 全站总预算 + 并发排队 / 三维限流
		admin.GET("/ai/budget", ai.GetAiBudget)
		admin.PUT("/ai/budget", ai.SaveAiBudget)
		admin.GET("/ai/throttle", ai.GetAiThrottle)
		admin.PUT("/ai/throttle", ai.SaveAiThrottle)
		admin.GET("/ai/agent-usage", ai.GetAiAgentUsage)
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
		admin.POST("/ratelimit/keys/:id/test", handlers.AdminTestAPIKey) // 平台 Key 连通性/有效性体检
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
		admin.GET("/ai/config", ai.AdminGetAiConfig)
		// AIGC 内容标识（合规）
		admin.GET("/aigc/config", ai.AdminGetAigcConfig)
		admin.PUT("/aigc/config", ai.AdminSaveAigcConfig)
		admin.GET("/aigc/exports", ai.AdminListAigcExports)
		admin.GET("/aigc/waivers", ai.AdminListAigcWaivers)
		admin.GET("/aigc/verify/:produce_id", ai.AdminAigcVerify)
		admin.POST("/aigc/font", ai.AdminUploadAigcFont)
		admin.PUT("/ai/config", ai.AdminSetAiConfig)
		admin.GET("/ai/models", ai.AdminListAiModels)
		// AI 路由策略（最便宜/最快/场景）+ 失败降级开关
		admin.GET("/ai/route", ai.GetAiRouteConfig)
		admin.PUT("/ai/route", ai.UpdateAiRouteConfig)
		admin.GET("/ai/presets", ai.AdminAiModelPresets)
		// L5 渠道对账：拉取上游余额/消费 + 真实连通性探针
		admin.GET("/ai/models/:id/channel", ai.AdminAiModelChannel)
		// 按渠道聚合的余额一览（每条渠道还剩多少钱）
		admin.GET("/ai/channels/balance", ai.AdminChannelBalanceOverview)
		// 模型统计：按渠道 / 模型 / 厂商聚合调用量、tokens、代扣与成功率
		admin.GET("/ai/usage-stats", ai.AdminAiUsageStats)
		// 渠道监控：临时输入令牌 + 用户 ID 探一次余额/连通性/最近扣费（不落库）
		admin.POST("/ai/channel-monitor", ai.AdminAiChannelMonitor)
		// 渠道账户凭据（nap_/sk_ 加密入库）：保存一次，之后监控与「拉取模型」都不用再粘贴
		admin.GET("/ai/channel-accounts", ai.AdminListChannelAccounts)
		admin.GET("/ai/channel-account", ai.AdminGetChannelAccount)
		admin.POST("/ai/channel-account", ai.AdminSaveChannelAccount)
		admin.DELETE("/ai/channel-account/:id", ai.AdminDeleteChannelAccount)
		// 渠道卡密池：一条渠道一个池，池内多把按权重轮询、连败自动摘除
		admin.GET("/ai/channel-keys", ai.AdminListAiChannelKeys)
		admin.POST("/ai/channel-keys", ai.AdminCreateAiChannelKey)
		admin.PATCH("/ai/channel-keys/:id", ai.AdminUpdateAiChannelKey)
		admin.DELETE("/ai/channel-keys/:id", ai.AdminDeleteAiChannelKey)
		admin.POST("/ai/channel-keys/:id/test", ai.AdminTestAiChannelKey)

		// API 卡池（多密钥轮询）
		admin.GET("/ai/models/:id/keys", handlers.AdminListAiModelKeys)
		admin.POST("/ai/models/:id/keys", handlers.AdminCreateAiModelKey)
		admin.PUT("/ai/models/:id/keys/:keyId", handlers.AdminUpdateAiModelKey)
		admin.DELETE("/ai/models/:id/keys/:keyId", handlers.AdminDeleteAiModelKey)
		admin.POST("/ai/models/:id/keys/test", handlers.AdminTestAiModelKey)

		// 风格字典 / 扣子默认配置
		admin.GET("/ai/styles", ai.AdminListAiStyles)
		admin.POST("/ai/styles", ai.AdminCreateAiStyle)
		admin.PUT("/ai/styles/:id", ai.AdminUpdateAiStyle)
		admin.DELETE("/ai/styles/:id", ai.AdminDeleteAiStyle)
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

		// AI 内容安全闸（提示词 + 用户特征 + 蓝队模型 B）
		admin.GET("/ai/safety/config", ai.GetAiSafetyConfig)
		admin.PUT("/ai/safety/config", ai.SaveAiSafetyConfig)
		admin.POST("/ai/safety/config/reset", ai.ResetAiSafetyConfig)
		admin.GET("/ai/safety/events", ai.ListAiSafetyEvents)
		admin.GET("/ai/safety/profiles", ai.ListAiSafetyProfiles)
		admin.POST("/ai/safety/profiles/:uid/clear", ai.ClearAiSafetyCooldown)
		admin.POST("/ai/safety/profiles/:uid/reset", ai.ResetAiSafetyProfile)
		admin.GET("/ai/safety/judge-logs", ai.ListJudgeLogs)
		admin.GET("/ai/safety/stats", ai.AiSafetyStats)

		// 社区舆情守卫（话题驱动条件拦截 + 词表 + 命中留证）
		admin.GET("/sentiment/config", handlers.GetSentimentConfig)
		admin.PUT("/sentiment/config", handlers.SaveSentimentConfig)
		admin.GET("/sentiment/terms", handlers.ListSentimentTerms)
		admin.POST("/sentiment/terms", handlers.CreateSentimentTerm)
		admin.PUT("/sentiment/terms/:id", handlers.UpdateSentimentTerm)
		admin.DELETE("/sentiment/terms/:id", handlers.DeleteSentimentTerm)
		admin.POST("/sentiment/terms/:id/review", handlers.ReviewSentimentTerm)
		admin.GET("/sentiment/audits", handlers.ListTermAudits)
		admin.GET("/sentiment/hits", handlers.ListSentimentHits)
		admin.GET("/sentiment/topics", handlers.GetContentTopic)
		admin.PUT("/sentiment/topics", handlers.SetContentTopic)
		admin.POST("/sentiment/run-update", handlers.RunSentimentUpdate)
		admin.GET("/ai/safety/locked-sessions", ai.ListLockedSessions)
		admin.POST("/ai/safety/sessions/:sid/unlock", ai.UnlockChatSession)
		admin.GET("/ai/vendors", handlers.AdminListAiVendors)
		admin.POST("/ai/vendors", handlers.AdminCreateAiVendor)
		admin.PUT("/ai/vendors/:id", handlers.AdminUpdateAiVendor)
		admin.DELETE("/ai/vendors/:id", handlers.AdminDeleteAiVendor)
		// 聚合分类定价（chat/image/video 默认单价，导入时未显式给价则套用）
		admin.GET("/ai/category-prices", handlers.AdminGetCategoryPrices)
		admin.PUT("/ai/category-prices", handlers.AdminSetCategoryPrice)
		// 厂商→版本→亚版本矩阵（管理端专用，含 newest 标记，用户侧不可见）
		admin.GET("/ai/vendors/matrix", handlers.AdminAiVendorMatrix)
		admin.GET("/ai/audit/stats", handlers.AdminAiAuditStats)
		admin.GET("/ai/audit/sessions", handlers.AdminAiAuditSessions)
		admin.GET("/ai/audit/sessions/:id/messages", handlers.AdminAiAuditSessionMessages)
		admin.GET("/ai/audit/media", handlers.AdminAiAuditMedia)
		admin.GET("/ai/audit/calls", handlers.AdminAiAuditCalls)

		// 个人信息泄漏提示统计（仅类型计数，无内容）
		admin.GET("/security/leak-hint/stats", handlers.AdminLeakHintStats)
		admin.POST("/ai/models", ai.AdminCreateAiModel)
		admin.PUT("/ai/models/:id", ai.AdminUpdateAiModel)
		admin.DELETE("/ai/models/:id", ai.AdminDeleteAiModel)
		// AI 模型资产管理：渠道拉取 / 批量导入 / 可见性 / 按渠道合并拆分
		admin.POST("/ai/models/pull", ai.AdminPullAiModels)
		admin.POST("/ai/models/import", ai.AdminImportAiModels)
		admin.POST("/ai/models/:id/visibility", ai.AdminSetModelVisibility)
		admin.POST("/ai/models/merge", ai.AdminMergeModels)
		admin.POST("/ai/models/split", ai.AdminSplitModels)
		// AI 智能体管理（§5.4）：预设增删改
		admin.GET("/ai/agents", ai.AdminListAiAgents)
		admin.POST("/ai/agents", ai.AdminCreateAiAgent)
		admin.PUT("/ai/agents/:id", ai.AdminUpdateAiAgent)
		admin.DELETE("/ai/agents/:id", ai.AdminDeleteAiAgent)
		// 智能体投稿审核（用户提交 → 站长审 → 广场上架）
		admin.GET("/agent-submissions", ai.AdminListAgentSubmissions)
		admin.POST("/agent-submissions/:id/review", ai.AdminReviewAgentSubmission)
		admin.GET("/ai-nav-items", ai.AdminGetAiNavItems)
		admin.PUT("/ai-nav-items", ai.AdminSaveAiNavItems)
		admin.GET("/agent-submit-config", ai.AdminGetAgentSubmitConfig)
		admin.PUT("/agent-submit-config", ai.AdminSaveAgentSubmitConfig)

		// 免费 AI 额度（§5.3）：平台默认 + 按人覆盖（静态子树避让 :id 通配）
		admin.GET("/ai/free-quota/config", ai.AdminGetAiFreeQuotaConfig)
		admin.PUT("/ai/free-quota/config", ai.AdminSetAiFreeQuotaConfig)
		admin.GET("/ai/free-quota/overrides", ai.AdminListAiFreeQuotaOverrides)
		admin.POST("/ai/free-quota/overrides", ai.AdminManageAiFreeQuotaOverride)

		// @ai 评论机器人配置（§5.6）
		admin.GET("/ai/bot/config", ai.AdminGetAiBotConfig)
		admin.PUT("/ai/bot/config", ai.AdminSetAiBotConfig)
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
		admin.GET("/bad-words/export", handlers.ExportBadWords)
		admin.POST("/bad-words/import", handlers.ImportBadWords)
		admin.POST("/bad-words/search-augment", handlers.SearchAugmentBadWords)
		// 违禁词标记待审（mark 动作命中记录，管理员确认/放行）
		admin.GET("/bad-word-flags", handlers.ListBadWordFlags)
		admin.PUT("/bad-word-flags/:id", handlers.HandleBadWordFlag)
		// 未成年人心理危机：高危观察期用户总览 +《高危用户心理干预报告》
		admin.GET("/crisis-report", handlers.GetCrisisReport)
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
}
