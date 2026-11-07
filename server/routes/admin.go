package routes

// routes.go — 全部 HTTP 路由注册集中在此（从 main.go 原样搬出，未改动任何路由）。
//
// 为什么单独成包：main.go 此前 1852 行、近 1000 行都是路由注册，改一个接口要在
// 上千行里翻；而 main 真正的职责只有「初始化 → 装配 → 启动」。
// 搬出来后 main 只留一行 routes.Register(r)，路由的增删改都在这一个文件里。
//
// 依赖方向是单向的：main → routes → handlers/models/middleware，不存在回环。

import (
	"nvs-server/handlers/admingov"
	"nvs-server/handlers/adminsite"
	"nvs-server/handlers/aiadmin"
	"nvs-server/handlers/aimedia"
	"nvs-server/handlers/aiplatform"
	"nvs-server/handlers/aiworkbench"
	"nvs-server/handlers/apikey"
	"nvs-server/handlers/auth"
	"nvs-server/handlers/badword"
	"nvs-server/handlers/blog"
	"nvs-server/handlers/comment"
	"nvs-server/handlers/community"
	"nvs-server/handlers/costrate"
	"nvs-server/handlers/dbadmin"
	"nvs-server/handlers/emergency"
	"nvs-server/handlers/gate"
	"nvs-server/handlers/geo"
	"nvs-server/handlers/home"
	"nvs-server/handlers/live"
	"nvs-server/handlers/moderation"
	"nvs-server/handlers/ops"
	"nvs-server/handlers/police"
	"nvs-server/handlers/publish"
	readpermh "nvs-server/handlers/readperm"
	"nvs-server/handlers/recycle"
	securityhandler "nvs-server/handlers/security"
	"nvs-server/handlers/sentiment"
	"nvs-server/handlers/site"
	"nvs-server/handlers/theme"
	"nvs-server/handlers/token"
	vipcardh "nvs-server/handlers/vipcard"
	"nvs-server/handlers/web"
	"nvs-server/middleware"
	"nvs-server/security"

	grouphandler "nvs-server/handlers/group"
	honeypothandler "nvs-server/handlers/honeypot"
	prompthandler "nvs-server/handlers/prompts"

	"github.com/gin-gonic/gin"
)

// 端口/书架/后台管理路由。
func registerAdminArea(r *gin.Engine) {
	adminGrp := r.Group("/api/admin")
	// 🔒 纵深防御：组级强制 admin 角色（除登录校验外再叠加角色校验），
	// 防止「新增 handler 漏写 ensureAdmin」导致越权。具体写操作仍由各 handler 内部 ensureAdmin 兜底。
	// 警方不再进 /admin：这里改成「admin 全放行 + police 仅白名单（内容治理子集）」。
	// 白名单见 middleware/governance.go，非白名单路径对 police 一律 403 并记审计。
	adminGrp.Use(middleware.AuthRequired(), middleware.AdminOrPoliceGovernance())
	{
		adminGrp.GET("/stats", adminsite.GetAdminStats)
		adminGrp.GET("/dashboard", adminsite.GetDashboardStats)
		adminGrp.GET("/community", community.GetCommunityDashboard)
		// 实名记录查询（F10：姓名解密仅在本响应返回，身份证仅脱敏展示）
		adminGrp.GET("/realname", admingov.AdminListRealName)
		// 实名核验提供商配置（国家代理/腾讯/阿里/华为/网易）
		adminGrp.GET("/realname-provider", admingov.AdminGetRealNameProvider)
		adminGrp.PUT("/realname-provider", admingov.AdminSetRealNameProvider)
		adminGrp.POST("/realname-provider/test", admingov.AdminTestRealNameProvider)
		adminGrp.GET("/hotwords", adminsite.AdminHotWords)                     // 热词统计（内容治理/舆情观测）
		adminGrp.POST("/search-logs/cleanup", adminsite.AdminSearchLogCleanup) // 维护搜索埋点日志（清空/保留期清理）
		adminGrp.GET("/users", adminsite.ListUsers)
		adminGrp.GET("/users/audit", adminsite.ListUserAuditLogs)
		adminGrp.GET("/zone-access", auth.ListZoneAccessLogs)          // 隔离墙进入记录（176号令专项，分页+筛选）
		adminGrp.GET("/zone-access/stats", auth.GetZoneAccessStats)    // 隔离墙进入记录统计（仪表盘卡片）
		adminGrp.GET("/web-access", web.ListWebAccessLogs)             // 网络诊断 / 用户分布 / 攻击流量（分页+多维筛选）
		adminGrp.GET("/web-access/stats", web.GetWebAccessStats)       // Web 访问日志聚合统计（诊断面板）
		adminGrp.GET("/web-access/geo", web.GetWebAccessGeo)           // 访问来源地域分布（ip2region 离线解析，供地图/排行）
		adminGrp.POST("/web-access/cleanup", web.CleanupWebAccessLogs) // 清空 Web 访问日志（合规证据由文件日志承担）
		adminGrp.PUT("/users/:id", adminsite.UpdateUser)
		adminGrp.DELETE("/users/:id", adminsite.DeleteUser)
		adminGrp.POST("/users/:id/mute", adminsite.MuteUser)
		adminGrp.POST("/users/:id/unmute", adminsite.UnmuteUser)
		adminGrp.POST("/users/:id/promote-police", adminsite.AdminPromoteToPolice) // 提权为警方（准管理员，仅 admin 可操作，留审计）
		adminGrp.POST("/users/:id/revoke-police", adminsite.AdminRevokePolice)     // 撤销警方权限（回退 reader）
		adminGrp.GET("/vip-applications", adminsite.ListVipApplications)
		adminGrp.POST("/vip-applications/:id/approve", adminsite.ApproveVip)
		adminGrp.GET("/reports", admingov.ListReports)
		adminGrp.POST("/reports/:id/handle", admingov.HandleReport)
		// AI 内容审核中枢
		adminGrp.GET("/moderation/config", moderation.GetModerationConfig)
		adminGrp.PUT("/moderation/config", moderation.SaveModerationConfig)
		adminGrp.GET("/moderation/records", moderation.ListModerationRecords)
		adminGrp.POST("/moderation/records/:id/decide", moderation.DecideModerationRecord)
		adminGrp.POST("/moderation/batch", moderation.RunModerationBatch)
		adminGrp.GET("/moderation/stats", moderation.GetModerationStats)
		adminGrp.GET("/moderation/malicious", moderation.ListMaliciousReporters)
		adminGrp.POST("/moderation/malicious/:id/unban", moderation.UnbanReporter)
		// HTML 内容审查（人工复核）
		adminGrp.GET("/html-reviews", web.ListHTMLReviews)
		adminGrp.GET("/html-reviews/:type/:id", web.GetHTMLReview)
		adminGrp.POST("/html-reviews/:type/:id/review", web.ReviewHTML)
		// HTML/WASM 托管额度（每人项目数 + 累计空间；支持按人覆盖）
		adminGrp.GET("/html-quota", adminsite.AdminGetHTMLQuota)
		adminGrp.PUT("/html-quota", adminsite.AdminSaveHTMLQuota)
		adminGrp.POST("/html-quota/overrides", adminsite.AdminSetHTMLQuotaOverride)
		adminGrp.GET("/html-quota/usage", adminsite.AdminHTMLQuotaUsage)
		// 评论区策略（楼加密是否连带隐藏楼内回复）
		adminGrp.GET("/comment-policy", adminsite.AdminGetCommentPolicy)
		adminGrp.PUT("/comment-policy", adminsite.AdminSaveCommentPolicy)
		adminGrp.GET("/finance", adminsite.GetFinanceOverview)
		// 站长配置
		adminGrp.GET("/config", adminsite.GetPlatformConfigs)
		adminGrp.POST("/smtp/test", adminsite.TestSMTP) // SMTP 测试与逐阶段诊断
		adminGrp.PUT("/config", adminsite.UpdatePlatformConfig)
		// 日志脱敏配置（176号令合规：生产环境日志敏感信息脱敏）
		adminGrp.GET("/log-redact", admingov.GetLogRedactConfig)
		adminGrp.PUT("/log-redact", admingov.UpdateLogRedactConfig)
		// 密码轨道状态（国际轨 / 国密轨 / 双轨）：只读，配置项在 .env，改了要重启
		adminGrp.GET("/crypto/status", adminsite.AdminCryptoStatus)
		// 蜜罐等级 + 日志记录字段勾选（docs/honeypot-design.md）
		adminGrp.GET("/honeypot/config", honeypothandler.GetConfig)
		adminGrp.PUT("/honeypot/config", honeypothandler.SaveConfig)
		adminGrp.POST("/honeypot/regenerate-key", honeypothandler.RegenerateKey)
		// 诱饵内容独立保存：只改文案，不碰启用/等级（避免全量覆盖把蜜罐顺手关掉）
		adminGrp.PUT("/honeypot/decoy-profile", honeypothandler.SaveDecoyProfile)
		adminGrp.GET("/honeypot/sessions", honeypothandler.ListSessions)
		adminGrp.GET("/honeypot/sessions/:sid", honeypothandler.SessionDetail)
		adminGrp.GET("/honeypot/export", honeypothandler.Export)
		adminGrp.POST("/honeypot/cleanup", honeypothandler.Cleanup)
		adminGrp.GET("/honeypot/archives", honeypothandler.Archives)
		adminGrp.GET("/honeypot/db", honeypothandler.DBInfo)
		adminGrp.GET("/honeypot/bans", honeypothandler.Bans)
		adminGrp.POST("/honeypot/ban", honeypothandler.Ban)
		adminGrp.POST("/honeypot/unban", honeypothandler.Unban)
		adminGrp.GET("/honeypot/sessions/:sid/fingerprints", honeypothandler.Fingerprints)
		adminGrp.GET("/honeypot/export.zip", honeypothandler.ExportZip)
		adminGrp.GET("/honeypot/rules", honeypothandler.Rules)
		adminGrp.POST("/honeypot/rules", honeypothandler.ApplyRules)
		adminGrp.POST("/log-redact/preview", admingov.PreviewLogRedact)
		// 176号令合规补全：算法推荐管理（第七条第九项）
		adminGrp.GET("/algorithms", admingov.GetAlgoConfig)
		adminGrp.PUT("/algorithms", admingov.UpdateAlgoConfig)
		// 等保/关基识别记录（第七条第四/五项）
		adminGrp.GET("/security-grade", admingov.GetSecurityGrade)
		adminGrp.PUT("/security-grade", admingov.UpdateSecurityGrade)
		// 漏洞整改管理（第七条第六项）
		adminGrp.GET("/vulnerabilities", admingov.ListVulnerabilities)
		adminGrp.POST("/vulnerabilities", admingov.CreateVulnerability)
		adminGrp.PUT("/vulnerabilities/:id", admingov.UpdateVulnerability)
		adminGrp.DELETE("/vulnerabilities/:id", admingov.DeleteVulnerability)
		adminGrp.GET("/vulnerabilities/export", admingov.ExportVulnerabilities)
		// 公告栏（管理端）
		adminGrp.GET("/announcements", adminsite.AdminListAnnouncements)
		adminGrp.POST("/announcements", adminsite.AdminCreateAnnouncement)
		adminGrp.PUT("/announcements/:id", adminsite.AdminUpdateAnnouncement)
		adminGrp.DELETE("/announcements/:id", adminsite.AdminDeleteAnnouncement)
		// 自定义主页区块（F15）
		// ⚠️ gin httprouter 铁律：静态段不能与同级通配段共存，
		// 故 reorder/presets 用独立前缀，避免与 /home-blocks/:id 冲突 panic
		adminGrp.GET("/home-blocks", home.AdminListHomeBlocks)
		adminGrp.POST("/home-blocks", home.AdminCreateHomeBlock)
		adminGrp.PUT("/home-blocks/:id", home.AdminUpdateHomeBlock)
		adminGrp.DELETE("/home-blocks/:id", home.AdminDeleteHomeBlock)
		adminGrp.POST("/home-block-reorder", home.AdminReorderHomeBlocks)
		adminGrp.GET("/home-block-presets", home.AdminListHomeBlockPresets)
		adminGrp.POST("/home-block-presets/:key/apply", home.AdminApplyHomeBlockPreset)
		// 狂暴模式配置（F19：默认关；开启后评论区 SSE 实时推送）
		adminGrp.GET("/rage-config", live.GetRageConfigAdmin)
		adminGrp.PUT("/rage-config", live.UpdateRageConfigAdmin)
		// 自定义网站主题（F12：alist 式，管理员配置对全站生效）
		adminGrp.PUT("/theme/site", theme.UpdateSiteTheme)
		// F12a 全站 opt-in iframe 仅管理员可写
		adminGrp.PUT("/site-iframe", site.UpdateSiteIframe)
		// 首页形态配置（管理员可把博客/论坛/混合流设为首页）
		adminGrp.GET("/home-view", home.AdminGetHomeView)
		adminGrp.PUT("/home-view", home.AdminSaveHomeView)
		// 地图服务（标准地图合规配置）
		adminGrp.GET("/map-setting", geo.AdminGetMapSetting)
		adminGrp.PUT("/map-setting", geo.AdminSaveMapSetting)
		adminGrp.POST("/map-setting/test", geo.AdminTestMapSetting)
		// 虚拟记账单位 → 真实货币的换算比例（中转站的「美元」只是站内代币）
		adminGrp.GET("/cost-rate", costrate.AdminGetCostRate)
		adminGrp.PUT("/cost-rate", costrate.AdminSaveCostRate)
		adminGrp.POST("/cost-rate/preview", costrate.AdminCostRatePreview)
		// web 项目额度与超额审核（F17）
		adminGrp.GET("/web-quota/requests", web.AdminListWebQuotaRequests)
		adminGrp.POST("/web-quota/requests/:id/approve", web.AdminApproveWebQuotaRequest)
		adminGrp.POST("/web-quota/requests/:id/reject", web.AdminRejectWebQuotaRequest)
		adminGrp.GET("/web-quota/config", web.AdminGetWebQuotaConfig)
		adminGrp.PUT("/web-quota/config", web.AdminSetWebQuotaConfig)
		// 检查应对工具箱（176号令 5.2 检查材料清单）
		adminGrp.GET("/compliance/report", admingov.GetComplianceReport)
		adminGrp.GET("/compliance/export-logs", admingov.ExportComplianceLogs)
		adminGrp.GET("/compliance/export-users", admingov.ExportComplianceUsers)
		// 一键导出检查应对材料包（zip + SHA256 清单，导出留痕）
		adminGrp.GET("/compliance/bundle.zip", admingov.ExportComplianceBundle)
		// ===== 注册门槛 / 邀请码 / 设备指纹（gap-audit #14）=====
		adminGrp.GET("/gate", gate.AdminGateConfig)
		adminGrp.PUT("/gate", gate.SaveGateConfig)
		adminGrp.GET("/gate/invites", gate.AdminInvites)
		adminGrp.POST("/gate/invites", gate.CreateInvites)
		adminGrp.POST("/gate/invites/:id/revoke", gate.RevokeInvite)
		adminGrp.GET("/gate/devices", gate.AdminDeviceBindings)
		// ===== 充值风控（gap-audit #9）=====
		adminGrp.GET("/risk", police.AdminRiskConfig)
		adminGrp.PUT("/risk", police.SaveRiskConfig)
		adminGrp.GET("/risk/events", police.AdminRiskEvents)
		adminGrp.POST("/risk/events/:id/handle", police.HandleRiskEvent)
		// ===== API Key 审计告警（告警口径补齐）=====
		adminGrp.GET("/api-key-alarms", police.AdminKeyAlarms)
		adminGrp.POST("/api-key-alarms/:id/ack", police.AckKeyAlarm)
		// ===== 自定义导航菜单 / 轮播 / 签到 / 访客设置 =====
		adminGrp.GET("/nav-menus", ops.AdminNavMenus)
		adminGrp.POST("/nav-menus", ops.SaveNavMenu)
		adminGrp.DELETE("/nav-menus/:id", ops.DeleteNavMenu)
		adminGrp.GET("/carousels", ops.AdminCarousels)
		adminGrp.POST("/carousels", ops.SaveCarousel)
		adminGrp.DELETE("/carousels/:id", ops.DeleteCarousel)
		adminGrp.GET("/checkin", ops.AdminCheckIn)
		adminGrp.PUT("/checkin", ops.SaveCheckIn)
		adminGrp.GET("/visit-setting", ops.AdminVisitSetting)
		adminGrp.PUT("/visit-setting", ops.SaveVisitSetting)
		// #65 阅读权限：站长配置可用等级 / VIP 门槛 / 付费最低价
		adminGrp.GET("/read-permission", readpermh.AdminGetConfig)
		adminGrp.PUT("/read-permission", readpermh.AdminSaveConfig)
		// #66 VIP 等级卡密：生成 / 列表 / 作废
		adminGrp.GET("/vip-cards", vipcardh.AdminList)
		adminGrp.POST("/vip-cards", vipcardh.AdminGenerate)
		adminGrp.POST("/vip-cards/:id/void", vipcardh.AdminVoid)
		adminGrp.GET("/live", ops.AdminLive)
		adminGrp.PUT("/live", ops.SaveLive)
		// ===== 语音模型 / 转化漏斗 =====
		adminGrp.GET("/ai/audio-models", aimedia.AdminSpeechModels)
		adminGrp.GET("/ai/funnel", aiadmin.AiFunnel)
		// 全站总预算 + 并发排队 / 三维限流
		adminGrp.GET("/ai/budget", aiplatform.GetAiBudget)
		adminGrp.PUT("/ai/budget", aiplatform.SaveAiBudget)
		adminGrp.GET("/ai/throttle", aiplatform.GetAiThrottle)
		adminGrp.PUT("/ai/throttle", aiplatform.SaveAiThrottle)
		adminGrp.GET("/ai/agent-usage", aiplatform.GetAiAgentUsage)
		// ===== 蜜罐：SQLi 命中与 TLS 采集率 =====
		adminGrp.GET("/honeypot/sqli", honeypothandler.SQLiStats)
		adminGrp.GET("/honeypot/tls", honeypothandler.TLSStats)
		// 隔离墙配置
		adminGrp.GET("/wall-config", adminsite.GetWallConfig)
		adminGrp.PUT("/wall-config", adminsite.UpdateWallConfig)
		// 远程站点互通
		adminGrp.GET("/sites", adminsite.ListFederatedSites)
		adminGrp.GET("/sites/inbound", adminsite.ListInboundSites)
		adminGrp.POST("/sites", adminsite.CreateFederatedSite)
		adminGrp.PUT("/sites/:id", adminsite.UpdateFederatedSite)
		adminGrp.DELETE("/sites/:id", adminsite.DeleteFederatedSite)
		adminGrp.POST("/sites/:id/sync", adminsite.SyncFederatedSite)
		// 联邦站点 API Key（双向可见：本站签发给对方 / 对方签发给本站）
		adminGrp.GET("/sites/:id/keys", adminsite.ListFederatedSiteKeys)
		adminGrp.POST("/sites/:id/keys/inbound", adminsite.CreateFederatedInboundKey)
		adminGrp.POST("/sites/:id/keys/outbound", adminsite.SaveFederatedOutboundKey)
		adminGrp.POST("/sites/:id/keys/:kid/toggle", adminsite.ToggleFederatedKey)
		adminGrp.DELETE("/sites/:id/keys/:kid", adminsite.DeleteFederatedKey)
		// 限流与 API Key 管理（全局配额 / 爬虫规则 / 日志统计 / 全站 Key）
		adminGrp.GET("/ratelimit/config", apikey.GetRateLimitConfig)
		adminGrp.PUT("/ratelimit/config", apikey.UpdateRateLimitConfig)
		adminGrp.GET("/ratelimit/logs", apikey.ListRateLimitLogs)
		adminGrp.GET("/ratelimit/stats", apikey.GetRateLimitStats)
		adminGrp.POST("/ratelimit/clear", apikey.ClearRateLimits)
		adminGrp.GET("/ratelimit/keys", apikey.AdminListAPIKeys)
		adminGrp.POST("/ratelimit/keys/:id/regenerate", apikey.AdminRegenerateAPIKey)
		adminGrp.PATCH("/ratelimit/keys/:id", apikey.AdminUpdateAPIKey)
		adminGrp.DELETE("/ratelimit/keys/:id", apikey.AdminDeleteAPIKey)
		// 按把封禁 / 解封
		adminGrp.POST("/ratelimit/keys/:id/ban", adminsite.AdminBanAPIKey)
		adminGrp.POST("/ratelimit/keys/:id/unban", adminsite.AdminUnbanAPIKey)
		adminGrp.POST("/ratelimit/keys/:id/test", apikey.AdminTestAPIKey) // 平台 Key 连通性/有效性体检
		// 按人管控：封禁该用户全部 Key + 专属限流配额
		adminGrp.GET("/ratelimit/users", adminsite.AdminListUserAPIPolicies)
		adminGrp.GET("/ratelimit/users/:id/quota", adminsite.AdminGetUserAPIQuota)
		adminGrp.PUT("/ratelimit/users/:id/quota", adminsite.AdminSetUserAPIQuota)
		adminGrp.DELETE("/ratelimit/users/:id/quota", adminsite.AdminDeleteUserAPIPolicy)
		adminGrp.POST("/ratelimit/users/:id/keys/ban", adminsite.AdminBanUserAPIKeys)
		adminGrp.POST("/ratelimit/users/:id/keys/unban", adminsite.AdminUnbanUserAPIKeys)
		// 群组管理
		adminGrp.GET("/groups", grouphandler.AdminListGroups)
		adminGrp.POST("/groups/:id/ban", grouphandler.AdminBanGroup)
		adminGrp.DELETE("/groups/:id", grouphandler.AdminDeleteGroup)
		// 群组额度：全局 + 按人（独立子树 /group-quota，避 /groups/:id 通配冲突）
		adminGrp.GET("/group-quota", grouphandler.AdminGetGroupQuota)
		adminGrp.PUT("/group-quota", grouphandler.AdminSetGroupQuota)
		adminGrp.PUT("/group-quota/users/:id", grouphandler.AdminSetUserGroupQuota)
		// 论坛管理
		adminGrp.GET("/forums", adminsite.AdminListForums)
		adminGrp.POST("/forums", adminsite.AdminCreateForum)
		adminGrp.PUT("/forums/:id", adminsite.AdminUpdateForum)
		adminGrp.DELETE("/forums/:id", adminsite.AdminDeleteForum)
		adminGrp.POST("/forums/:id/review", adminsite.AdminReviewForum)
		// 自建论坛额度：全局默认 + 按人覆盖（削减/放开某人的建版数）
		// 独立子树 /forum-quota：避免与 /forums/:id 通配段冲突导致路由注册 panic
		adminGrp.GET("/forum-quota", adminsite.GetForumQuotaConfig)
		adminGrp.PUT("/forum-quota", adminsite.UpdateForumQuotaConfig)
		adminGrp.GET("/forum-quota/users", adminsite.ListUserForumQuotas)
		adminGrp.PUT("/forum-quota/users/:id", adminsite.SetUserForumQuota)
		adminGrp.DELETE("/forum-quota/users/:id", adminsite.DeleteUserForumQuota)
		// 每日发文限额 / 用户配额体系（§13.4）：等级×每日数量/字数 + 按人覆盖
		// 独立子树 /daily-publish：静态段，规避与既有 :id 通配段冲突
		adminGrp.GET("/daily-publish/config", publish.AdminGetDailyPublishConfig)
		adminGrp.PUT("/daily-publish/config", publish.AdminSetDailyPublishConfig)
		adminGrp.GET("/daily-publish/overrides", publish.AdminListPublishOverrides)
		adminGrp.POST("/daily-publish/overrides", publish.AdminManagePublishOverride)
		// 代币账本 / 卡密管理（§7）：配置 + 生成/作废卡密 + 授信扣减
		// 独立子树 /token：静态段，避让 :id 通配冲突
		adminGrp.GET("/token/config", token.AdminGetTokenConfig)
		adminGrp.PUT("/token/config", token.AdminSetTokenConfig)
		adminGrp.GET("/token/cards", token.AdminListCards)
		adminGrp.POST("/token/cards", token.AdminGenerateCards)
		adminGrp.POST("/token/cards/:id/void", token.AdminVoidCard)
		adminGrp.POST("/token/adjust", token.AdminAdjustToken)
		// AI 模型池（§5.3）：总开关 + 模型 CRUD（api_key 加密存储、只写不读）
		// 独立子树 /ai：静态段，避让 :id 通配冲突
		adminGrp.GET("/ai/config", aiadmin.AdminGetAiConfig)
		// AIGC 内容标识（合规）
		adminGrp.GET("/aigc/config", aimedia.AdminGetAigcConfig)
		adminGrp.PUT("/aigc/config", aimedia.AdminSaveAigcConfig)
		adminGrp.GET("/aigc/exports", aimedia.AdminListAigcExports)
		adminGrp.GET("/aigc/waivers", aimedia.AdminListAigcWaivers)
		adminGrp.GET("/aigc/verify/:produce_id", aimedia.AdminAigcVerify)
		adminGrp.POST("/aigc/font", aimedia.AdminUploadAigcFont)
		adminGrp.PUT("/ai/config", aiadmin.AdminSetAiConfig)
		adminGrp.GET("/ai/models", aiadmin.AdminListAiModels)
		// AI 路由策略（最便宜/最快/场景）+ 失败降级开关
		adminGrp.GET("/ai/route", aiplatform.GetAiRouteConfig)
		adminGrp.PUT("/ai/route", aiplatform.UpdateAiRouteConfig)
		adminGrp.GET("/ai/presets", aiadmin.AdminAiModelPresets)
		// L5 渠道对账：拉取上游余额/消费 + 真实连通性探针
		adminGrp.GET("/ai/models/:id/channel", aiadmin.AdminAiModelChannel)
		// 按渠道聚合的余额一览（每条渠道还剩多少钱）
		adminGrp.GET("/ai/channels/balance", aiadmin.AdminChannelBalanceOverview)
		// 模型统计：按渠道 / 模型 / 厂商聚合调用量、tokens、代扣与成功率
		adminGrp.GET("/ai/usage-stats", aiadmin.AdminAiUsageStats)
		// 渠道监控：临时输入令牌 + 用户 ID 探一次余额/连通性/最近扣费（不落库）
		adminGrp.POST("/ai/channel-monitor", aiadmin.AdminAiChannelMonitor)
		// 渠道账户凭据（nap_/sk_ 加密入库）：保存一次，之后监控与「拉取模型」都不用再粘贴
		adminGrp.GET("/ai/channel-accounts", aiadmin.AdminListChannelAccounts)
		adminGrp.GET("/ai/channel-account", aiadmin.AdminGetChannelAccount)
		adminGrp.POST("/ai/channel-account", aiadmin.AdminSaveChannelAccount)
		adminGrp.DELETE("/ai/channel-account/:id", aiadmin.AdminDeleteChannelAccount)
		// 渠道卡密池：一条渠道一个池，池内多把按权重轮询、连败自动摘除
		adminGrp.GET("/ai/channel-keys", aiadmin.AdminListAiChannelKeys)
		adminGrp.POST("/ai/channel-keys", aiadmin.AdminCreateAiChannelKey)
		adminGrp.PATCH("/ai/channel-keys/:id", aiadmin.AdminUpdateAiChannelKey)
		adminGrp.DELETE("/ai/channel-keys/:id", aiadmin.AdminDeleteAiChannelKey)
		adminGrp.POST("/ai/channel-keys/:id/test", aiadmin.AdminTestAiChannelKey)

		// API 卡池（多密钥轮询）
		adminGrp.GET("/ai/models/:id/keys", adminsite.AdminListAiModelKeys)
		adminGrp.POST("/ai/models/:id/keys", adminsite.AdminCreateAiModelKey)
		adminGrp.PUT("/ai/models/:id/keys/:keyId", adminsite.AdminUpdateAiModelKey)
		adminGrp.DELETE("/ai/models/:id/keys/:keyId", adminsite.AdminDeleteAiModelKey)
		adminGrp.POST("/ai/models/:id/keys/test", adminsite.AdminTestAiModelKey)

		// 风格字典 / 扣子默认配置
		adminGrp.GET("/ai/styles", aimedia.AdminListAiStyles)
		adminGrp.POST("/ai/styles", aimedia.AdminCreateAiStyle)
		adminGrp.PUT("/ai/styles/:id", aimedia.AdminUpdateAiStyle)
		adminGrp.DELETE("/ai/styles/:id", aimedia.AdminDeleteAiStyle)
		adminGrp.GET("/ai/coze-default", adminsite.AdminGetAiCozeDefault)
		adminGrp.PUT("/ai/coze-default", adminsite.AdminSetAiCozeDefault)

		// AI 功能开关 / 厂商字典 / 数据审计
		// 提示词模板引擎（CRUD / 版本 / 回滚 / 预览）
		adminGrp.GET("/ai/templates", prompthandler.AdminList)
		adminGrp.POST("/ai/templates", prompthandler.AdminCreate)
		adminGrp.PUT("/ai/templates/:id", prompthandler.AdminUpdate)
		adminGrp.DELETE("/ai/templates/:id", prompthandler.AdminDelete)
		adminGrp.GET("/ai/templates/:id/versions", prompthandler.AdminVersions)
		adminGrp.POST("/ai/templates/:id/restore", prompthandler.AdminRestore)
		// 预览用独立路径：/ai/templates/preview 会与 /ai/templates/:id 的通配段冲突（gin httprouter 限制）
		adminGrp.POST("/ai/template-preview", prompthandler.AdminPreview)

		adminGrp.GET("/ai/features", admingov.AdminGetAiFeatures)
		adminGrp.PUT("/ai/features", admingov.AdminSetAiFeatures)

		// AI 内容安全闸（提示词 + 用户特征 + 蓝队模型 B）
		adminGrp.GET("/ai/safety/config", aiworkbench.GetAiSafetyConfig)
		adminGrp.PUT("/ai/safety/config", aiworkbench.SaveAiSafetyConfig)
		adminGrp.POST("/ai/safety/config/reset", aiworkbench.ResetAiSafetyConfig)
		adminGrp.GET("/ai/safety/events", aiworkbench.ListAiSafetyEvents)
		adminGrp.GET("/ai/safety/profiles", aiworkbench.ListAiSafetyProfiles)
		adminGrp.POST("/ai/safety/profiles/:uid/clear", aiworkbench.ClearAiSafetyCooldown)
		adminGrp.POST("/ai/safety/profiles/:uid/reset", aiworkbench.ResetAiSafetyProfile)
		adminGrp.GET("/ai/safety/judge-logs", aiworkbench.ListJudgeLogs)
		adminGrp.GET("/ai/safety/stats", aiworkbench.AiSafetyStats)

		// 社区舆情守卫（话题驱动条件拦截 + 词表 + 命中留证）
		adminGrp.GET("/sentiment/config", sentiment.GetSentimentConfig)
		adminGrp.PUT("/sentiment/config", sentiment.SaveSentimentConfig)
		adminGrp.GET("/sentiment/terms", sentiment.ListSentimentTerms)
		adminGrp.POST("/sentiment/terms", sentiment.CreateSentimentTerm)
		adminGrp.PUT("/sentiment/terms/:id", sentiment.UpdateSentimentTerm)
		adminGrp.DELETE("/sentiment/terms/:id", sentiment.DeleteSentimentTerm)
		adminGrp.POST("/sentiment/terms/:id/review", sentiment.ReviewSentimentTerm)
		adminGrp.GET("/sentiment/audits", sentiment.ListTermAudits)
		adminGrp.GET("/sentiment/hits", sentiment.ListSentimentHits)
		adminGrp.GET("/sentiment/topics", sentiment.GetContentTopic)
		adminGrp.PUT("/sentiment/topics", sentiment.SetContentTopic)
		adminGrp.POST("/sentiment/run-update", sentiment.RunSentimentUpdate)
		// 阵营中立标记 / 语言格式 / 图谱 / 删除申请审批（超级管理员分层）
		adminGrp.GET("/sentiment/neutral-prompt", sentiment.GetNeutralityPrompt)
		adminGrp.GET("/sentiment/chat-models", sentiment.ListSentimentChatModels)
		adminGrp.POST("/sentiment/camp-analyze", sentiment.AnalyzeCamp)
		adminGrp.POST("/sentiment/camp-marks", sentiment.CreateCampMark)
		adminGrp.GET("/sentiment/camp-marks", sentiment.ListCampMarks)
		adminGrp.GET("/sentiment/camp-graph", sentiment.GetCampGraph)
		adminGrp.POST("/sentiment/camp-marks/:id/approve", sentiment.ApproveCampMark)
		adminGrp.POST("/sentiment/camp-marks/:id/reject", sentiment.RejectCampMark)
		adminGrp.GET("/sentiment/topics/list", sentiment.ListTopicTags)
		adminGrp.POST("/sentiment/camp-marks/:id/request-update", sentiment.RequestUpdateCampMark)
		adminGrp.POST("/sentiment/camp-marks/:id/request-delete", sentiment.RequestDeleteCampMark)
		adminGrp.PUT("/sentiment/camp-marks/:id", sentiment.UpdateCampMark)
		adminGrp.DELETE("/sentiment/camp-marks/:id", sentiment.DeleteCampMark)
		adminGrp.GET("/sentiment/delete-requests", sentiment.ListDeleteRequests)
		adminGrp.POST("/sentiment/delete-requests/:id/approve", sentiment.ApproveDeleteRequest)
		adminGrp.POST("/sentiment/delete-requests/:id/reject", sentiment.RejectDeleteRequest)
		adminGrp.GET("/ai/safety/locked-sessions", aiworkbench.ListLockedSessions)
		adminGrp.POST("/ai/safety/sessions/:sid/unlock", aiworkbench.UnlockChatSession)
		// 名单子系统（以疏代堵）：可疑/恶意名单 + 警方取证申请审批
		adminGrp.GET("/supervise/list", aiadmin.AdminListSupervised)
		adminGrp.GET("/supervise/:id", aiadmin.AdminGetSupervised)
		adminGrp.POST("/supervise/:id/set-tier", aiadmin.AdminSetTier)
		adminGrp.POST("/supervise/:id/clear", aiadmin.AdminClearSupervised)
		adminGrp.POST("/supervise/:id/note", aiadmin.AdminAddSuperviseNote)
		adminGrp.GET("/supervise/stats", aiadmin.AdminSuperviseStats)
		// 警方取证导出申请（站长审批流）
		adminGrp.GET("/police/evidence-requests", admingov.AdminListEvidenceRequests)
		adminGrp.POST("/police/evidence-requests/:id/review", admingov.AdminReviewEvidenceRequest)
		adminGrp.GET("/ai/vendors", admingov.AdminListAiVendors)
		adminGrp.POST("/ai/vendors", admingov.AdminCreateAiVendor)
		adminGrp.PUT("/ai/vendors/:id", admingov.AdminUpdateAiVendor)
		adminGrp.DELETE("/ai/vendors/:id", admingov.AdminDeleteAiVendor)
		// 聚合分类定价（chat/image/video 默认单价，导入时未显式给价则套用）
		adminGrp.GET("/ai/category-prices", admingov.AdminGetCategoryPrices)
		adminGrp.PUT("/ai/category-prices", admingov.AdminSetCategoryPrice)
		// 厂商→版本→亚版本矩阵（管理端专用，含 newest 标记，用户侧不可见）
		adminGrp.GET("/ai/vendors/matrix", admingov.AdminAiVendorMatrix)
		adminGrp.GET("/ai/audit/stats", admingov.AdminAiAuditStats)
		adminGrp.GET("/ai/audit/sessions", admingov.AdminAiAuditSessions)
		adminGrp.GET("/ai/audit/sessions/:id/messages", admingov.AdminAiAuditSessionMessages)
		adminGrp.GET("/ai/audit/media", admingov.AdminAiAuditMedia)
		adminGrp.GET("/ai/audit/calls", admingov.AdminAiAuditCalls)

		// 个人信息泄漏提示统计（仅类型计数，无内容）
		adminGrp.GET("/security/leak-hint/stats", securityhandler.AdminLeakHintStats)
		adminGrp.POST("/ai/models", aiadmin.AdminCreateAiModel)
		adminGrp.PUT("/ai/models/:id", aiadmin.AdminUpdateAiModel)
		adminGrp.DELETE("/ai/models/:id", aiadmin.AdminDeleteAiModel)
		// AI 模型资产管理：渠道拉取 / 批量导入 / 可见性 / 按渠道合并拆分
		adminGrp.POST("/ai/models/pull", aiadmin.AdminPullAiModels)
		adminGrp.POST("/ai/models/import", aiadmin.AdminImportAiModels)
		adminGrp.POST("/ai/models/:id/visibility", aiadmin.AdminSetModelVisibility)
		adminGrp.POST("/ai/models/merge", aiadmin.AdminMergeModels)
		adminGrp.POST("/ai/models/split", aiadmin.AdminSplitModels)
		// AI 智能体管理（§5.4）：预设增删改
		adminGrp.GET("/ai/agents", aiadmin.AdminListAiAgents)
		adminGrp.POST("/ai/agents", aiadmin.AdminCreateAiAgent)
		adminGrp.PUT("/ai/agents/:id", aiadmin.AdminUpdateAiAgent)
		adminGrp.DELETE("/ai/agents/:id", aiadmin.AdminDeleteAiAgent)
		// 智能体投稿审核（用户提交 → 站长审 → 广场上架）
		adminGrp.GET("/agent-submissions", aiadmin.AdminListAgentSubmissions)
		adminGrp.POST("/agent-submissions/:id/review", aiadmin.AdminReviewAgentSubmission)
		adminGrp.GET("/ai-nav-items", aimedia.AdminGetAiNavItems)
		adminGrp.PUT("/ai-nav-items", aimedia.AdminSaveAiNavItems)
		adminGrp.GET("/agent-submit-config", aiadmin.AdminGetAgentSubmitConfig)
		adminGrp.PUT("/agent-submit-config", aiadmin.AdminSaveAgentSubmitConfig)

		// 免费 AI 额度（§5.3）：平台默认 + 按人覆盖（静态子树避让 :id 通配）
		adminGrp.GET("/ai/free-quota/config", aiplatform.AdminGetAiFreeQuotaConfig)
		adminGrp.PUT("/ai/free-quota/config", aiplatform.AdminSetAiFreeQuotaConfig)
		adminGrp.GET("/ai/free-quota/overrides", aiplatform.AdminListAiFreeQuotaOverrides)
		adminGrp.POST("/ai/free-quota/overrides", aiplatform.AdminManageAiFreeQuotaOverride)

		// @ai 评论机器人配置（§5.6）
		adminGrp.GET("/ai/bot/config", aiadmin.AdminGetAiBotConfig)
		adminGrp.PUT("/ai/bot/config", aiadmin.AdminSetAiBotConfig)
		// 在线支付通道（§7.4）：配置（密钥只写不读）/ 档位 / 订单列表 / 手动补单
		adminGrp.GET("/payment/config", adminsite.AdminGetPaymentConfig)
		adminGrp.PUT("/payment/config", adminsite.AdminSetPaymentConfig)
		adminGrp.PUT("/payment/tiers", adminsite.AdminSetPaymentTiers)
		adminGrp.GET("/payment/orders", adminsite.AdminListPaymentOrders)
		adminGrp.POST("/payment/orders/mark-paid", adminsite.AdminMarkOrderPaid)
		// 应用扩展安全审核：预检未通过的 ZIP 原样封存（不解压），管理员放行/驳回
		adminGrp.GET("/userhtmls", adminsite.AdminListHTMLs)
		adminGrp.GET("/userhtmls/:id/scan", adminsite.AdminGetHTMLScan)
		adminGrp.POST("/userhtmls/:id/review", adminsite.AdminReviewHTML)
		// 自定义主页审核（作者公开页提交 → 站长/管理员复核原文）
		adminGrp.GET("/home-customize/reviews", home.ListHomeCustomizeReviews)
		adminGrp.POST("/home-customize/reviews/:id/review", home.ReviewHomeCustomize)
		// XSS 正则规则库（管理员维护）
		adminGrp.GET("/xss-rules", moderation.ListXssRules)
		adminGrp.POST("/xss-rules", moderation.CreateXssRule)
		adminGrp.PUT("/xss-rules/:id", moderation.UpdateXssRule)
		adminGrp.DELETE("/xss-rules/:id", moderation.DeleteXssRule)
		adminGrp.POST("/xss-rules/test", moderation.TestXssRule)
		// 违禁词库（管理员维护：标记/删除/替换 + 关键词/正则双模式 + 可配置适用范围）
		adminGrp.GET("/bad-words", badword.ListBadWords)
		adminGrp.POST("/bad-words", badword.CreateBadWord)
		adminGrp.PUT("/bad-words/:id", badword.UpdateBadWord)
		adminGrp.DELETE("/bad-words/:id", badword.DeleteBadWord)
		adminGrp.POST("/bad-words/test", badword.TestBadWord)
		adminGrp.GET("/bad-words/export", badword.ExportBadWords)
		adminGrp.POST("/bad-words/import", badword.ImportBadWords)
		adminGrp.POST("/bad-words/search-augment", badword.SearchAugmentBadWords)
		// 违禁词标记待审（mark 动作命中记录，管理员确认/放行）
		adminGrp.GET("/bad-word-flags", badword.ListBadWordFlags)
		adminGrp.PUT("/bad-word-flags/:id", badword.HandleBadWordFlag)
		// 未成年人心理危机：高危观察期用户总览 +《高危用户心理干预报告》
		adminGrp.GET("/crisis-report", comment.GetCrisisReport)
		// 内容抢救台（二级密码 + 应急预览/硬删除；unlock 加 IP 限流防爆破）
		adminGrp.POST("/emergency/set-password", emergency.SetEmergencyPassword)
		adminGrp.POST("/emergency/unlock", security.RateLimit(10, 60), emergency.UnlockEmergency)
		adminGrp.POST("/emergency/preview", emergency.PreviewEmergencyContent)
		adminGrp.POST("/emergency/content", emergency.DeleteEmergencyContent)
		adminGrp.GET("/emergency/audit", emergency.ListEmergencyAudit)
		// 内容治理：删除/封禁任意内容（软删除 + softdel_ 留证）
		adminGrp.GET("/softdel", admingov.AdminListSoftDel)
		adminGrp.POST("/softdel", admingov.AdminModerateContent)
		adminGrp.POST("/softdel/restore", admingov.AdminRestoreContent)
		// 回收站监管：管理员查看全站用户自删内容
		adminGrp.GET("/recycle", recycle.AdminListRecycle)
		// 博客分类管理（一/二/三级；管理员审核视图 + 事后审核 + 创建/编辑/删除）
		adminGrp.GET("/blog-categories", blog.AdminListBlogCategories)
		adminGrp.GET("/blog-categories/mode", blog.AdminGetCategoryReviewMode)
		adminGrp.PUT("/blog-categories/mode", blog.AdminSetCategoryReviewMode)
		adminGrp.POST("/blog-categories", blog.CreateBlogCategory)
		adminGrp.PUT("/blog-categories/:id", blog.UpdateBlogCategory)
		adminGrp.DELETE("/blog-categories/:id", blog.DeleteBlogCategory)
		adminGrp.POST("/blog-categories/:id/review", blog.AdminReviewBlogCategory)
		// 小说/章节封禁管理（管理员视角列表 + 封禁 + 解封）
		adminGrp.GET("/novels", admingov.AdminListNovels)
		adminGrp.GET("/novels/:id/chapters", admingov.AdminListChapters)
		// 批量重算所有作品统计（修复 total_chapters / total_words 脏数据）
		// 注意：不可注册在 /novels 下，否则与 /novels/:id/chapters 的通配段冲突导致路由 panic。
		adminGrp.POST("/recompute-novel-stats", admingov.AdminRecomputeNovelStats)
		// 数据库管理（健康检查 / 迁移 / 备份 / 恢复 / 定时备份）
		adminGrp.GET("/db/health", dbadmin.AdminDBHealth)
		adminGrp.GET("/db/current", dbadmin.AdminDBCurrent)
		adminGrp.POST("/db/migrate", dbadmin.AdminDBMigrate)
		adminGrp.POST("/db/backup", dbadmin.AdminDBBackup)
		adminGrp.GET("/db/job/:id", dbadmin.AdminDBJob)
		adminGrp.GET("/db/backups", dbadmin.AdminDBListBackups)
		adminGrp.DELETE("/db/backups/:name", dbadmin.AdminDBDeleteBackup)
		adminGrp.GET("/db/backups/:name/download", dbadmin.AdminDBDownloadBackup)
		adminGrp.POST("/db/restore", dbadmin.AdminDBRestore)
		adminGrp.GET("/db/backup-config", dbadmin.AdminDBBackupConfigGet)
		adminGrp.PUT("/db/backup-config", dbadmin.AdminDBBackupConfigPut)
	}

	// 警方取管台（police）：准管理员角色，仅可取证 / 查看软删除，无普通管理权
	// 不可自注册，只能由管理员通过 /api/admin/users/:id/promote-police 提权。
}
