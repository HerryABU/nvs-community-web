import { createRouter, createWebHistory } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { canAccessPath, POLICE_ALLOWED_PATHS } from '@/constants/policeAccess';
import api from '@/api';
import { appPrefix } from '@/utils/prefix';

const router = createRouter({
  // 🔀 站点路径前缀支持（URL_PREFIX，如 /go-nvs）：经 getAppPrefix() 智能解析——
  // 后端注入的 window.__NVS_PREFIX__ 优先；自建反向代理仅改写 <base href> 时
  // 自动从 document.baseURI 推导。作为 Vue Router base，使页面 URL 保持
  // /go-nvs/xxx 且刷新/跳转正确
  history: createWebHistory(appPrefix || '/'),
  routes: [
    {
      path: '/',
      name: 'home',
      // 首页形态由后台配置决定（作品 / 博客 / 论坛 / 混合流），
      // 由 HomeSwitch 按 /api/home-view 分发，默认作品首页（历史行为）
      component: () => import('@/views/home/HomeSwitch.vue'),
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/auth/Login.vue'),
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/views/auth/Register.vue'),
    },
    // 建站向导（仅未安装时可进入，安装后永久关闭；/install 为规范入口，/setup 兼容重定向）
    {
      path: '/install',
      name: 'install',
      component: () => import('@/views/auth/SetupWizard.vue'),
    },
    {
      path: '/setup',
      redirect: '/install',
    },
    // 旧路径（兼容）
    {
      path: '/novel/:id',
      name: 'novel-detail',
      component: () => import('@/views/novel/NovelDetail.vue'),
    },
    // 旧路径（兼容，Reader内部获取作者后自动替换URL）
    {
      path: '/novel/:id/read/:chapter',
      name: 'reader',
      component: () => import('@/views/novel/Reader.vue'),
    },
    // 公开作者主页（新版，含统计图表）
    {
      path: '/author/:id',
      name: 'author-home',
      component: () => import('@/views/author/AuthorHome.vue'),
    },
    // 新路径（推荐）：/author/:authorId/novel/:id
    {
      path: '/author/:authorId/novel/:id',
      name: 'novel-detail-new',
      component: () => import('@/views/novel/NovelDetail.vue'),
      alias: '/author/:authorId/novel/:id/',  // 尾斜杠兼容
    },
    // 新路径（推荐）：/author/:authorId/novel/:id/read/:chapter
    {
      path: '/author/:authorId/novel/:id/read/:chapter',
      name: 'reader-new',
      component: () => import('@/views/novel/Reader.vue'),
    },
    {
      path: '/author',
      name: 'author-dashboard',
      component: () => import('@/views/author/AuthorDashboard.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/editor/:id?',
      name: 'editor',
      component: () => import('@/views/novel/Editor.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/editor/:id/chapter/:num',
      name: 'chapter-editor',
      component: () => import('@/views/novel/ChapterEditor.vue'),
      meta: { requiresAuth: true },
    },
    // 多人协作：我参与的协作作品 + 待处理邀请
    {
      path: '/collab',
      name: 'collab-works',
      component: () => import('@/views/me/CollabWorks.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/category/:name',
      name: 'category',
      component: () => import('@/views/novel/CategoryView.vue'),
    },
    {
      path: '/author/:id/blog',
      redirect: to => ({ name: 'author-blogs', params: { id: to.params.id } }),
    },
    // 作者博客列表
    {
      path: '/author/:id/blogs',
      name: 'author-blogs',
      component: () => import('@/views/blog/AuthorBlogs.vue'),
    },
    {
      path: '/author/:id/blogs/:blogId',
      name: 'author-blog-detail',
      component: () => import('@/views/blog/BlogDetail.vue'),
    },
    {
      path: '/live',
      name: 'live',
      component: () => import('@/views/site/Live.vue'),
    },
    {
      path: '/groups',
      name: 'groups',
      component: () => import('@/views/group/index.vue'),
    },
    {
      path: '/group/:id',
      name: 'group-detail',
      component: () => import('@/views/group/detail.vue'),
    },
    // OC 模式群组：角色卡 / 世界书 / 群内 AI 角色 / 对话流 / 一键发布（docs/oc-mode.md）
    {
      path: '/group/:id/oc',
      name: 'group-oc',
      component: () => import('@/views/group/OcPanel.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/forums',
      name: 'forums',
      component: () => import('@/views/forum/Forums.vue'),
    },
    {
      path: '/comment-network',
      name: 'comment-network',
      component: () => import('@/views/comment/CommentNetworkPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/bookshelf',
      name: 'bookshelf',
      component: () => import('@/views/me/Bookshelf.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/forum/:id',
      name: 'forum-detail',
      component: () => import('@/views/forum/ForumDetail.vue'),
    },
    {
      path: '/thread/:id',
      name: 'thread-detail',
      component: () => import('@/views/forum/ThreadDetail.vue'),
    },
    {
      path: '/wallet',
      name: 'wallet',
      component: () => import('@/views/payment/Wallet.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/membership',
      name: 'membership',
      component: () => import('@/views/payment/Membership.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/pay/mock/:orderNo',
      name: 'pay-mock',
      component: () => import('@/views/payment/PayMock.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/pay/result/:orderNo',
      name: 'pay-result',
      component: () => import('@/views/payment/PayResult.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/ai',
      name: 'ai-hub',
      component: () => import('@/views/ai/AiHub.vue'),
      meta: { requiresAuth: true },
    },
    {
      // 模型价目表：公开可看，价格数据来自后台「模型收费」页的配置
      path: '/ai/pricing',
      name: 'ai-pricing',
      component: () => import('@/views/ai/AiPricing.vue'),
    },
    {
      // 灵感广场独立页（作品流公开可浏览）
      path: '/ai/gallery',
      name: 'ai-gallery',
      component: () => import('@/views/ai/AiGallery.vue'),
    },
    {
      // 公开只读分享页（令牌即凭据，无需登录）
      path: '/ai/share/:token',
      name: 'ai-shared',
      component: () => import('@/views/ai/AiShared.vue'),
    },
    {
      path: '/admin',
      name: 'admin-dashboard',
      component: () => import('@/views/admin/AdminDashboard.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/users',
      name: 'admin-users',
      component: () => import('@/views/admin/business/UserManagement.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/settings',
      name: 'admin-settings',
      component: () => import('@/views/admin/site/AdminSettings.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/categories',
      name: 'admin-categories',
      component: () => import('@/views/admin/content/AdminCategories.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/aigc',
      name: 'admin-aigc',
      component: () => import('@/views/admin/security/AdminAigc.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/wall',
      name: 'admin-wall',
      component: () => import('@/views/admin/security/AdminWall.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/wall-access',
      name: 'admin-wall-access',
      component: () => import('@/views/admin/security/AdminWallAccess.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/net-diag',
      name: 'admin-net-diag',
      component: () => import('@/views/admin/security/AdminNetDiag.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/hotwords',
      name: 'admin-hotwords',
      component: () => import('@/views/admin/content/AdminHotWords.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/realname',
      name: 'admin-realname',
      component: () => import('@/views/admin/security/AdminRealName.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/login-methods',
      name: 'admin-login-methods',
      component: () => import('@/views/admin/site/AdminLoginMethods.vue'),
      meta: { requiresAuth: true },
    },
    {
      // 旧地址兼容：历史书签、站内老链接仍指向 /police-console
      path: '/police-console',
      redirect: '/police',
    },
    {
      path: '/police',
      name: 'police',
      component: () => import('@/views/admin/police/AdminPoliceConsole.vue'),
      meta: { requiresAuth: true },
    },
    // ── 警方：内容治理子集（复用后台视图，但整段 /police/* 强制 requiresPolice）──
    // 之所以能复用：这些页面只依赖「内容治理」白名单内的接口，
    // 越权路径（合规与安全、AI 配置、用户管理）后端会直接 403 并记审计。
    {
      path: '/police/moderation',
      name: 'police-moderation',
      component: () => import('@/views/admin/content/AdminModerationQueue.vue'),
      meta: { requiresPolice: true },
    },
    {
      path: '/police/reports',
      name: 'police-reports',
      component: () => import('@/views/admin/content/AdminReports.vue'),
      meta: { requiresPolice: true },
    },
    {
      path: '/police/ban',
      name: 'police-ban',
      component: () => import('@/views/admin/content/AdminBan.vue'),
      meta: { requiresPolice: true },
    },
    {
      path: '/admin/sites',
      name: 'admin-sites',
      component: () => import('@/views/admin/site/AdminSites.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ratelimit',
      name: 'admin-ratelimit',
      component: () => import('@/views/admin/security/AdminRateLimit.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/forums',
      name: 'admin-forums',
      component: () => import('@/views/admin/content/AdminForums.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/reports',
      name: 'admin-reports',
      component: () => import('@/views/admin/content/AdminReports.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/html-review',
      name: 'admin-html-review',
      component: () => import('@/views/admin/content/AdminHtmlReview.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/home-customize-review',
      name: 'admin-home-customize-review',
      component: () => import('@/views/admin/site/AdminHomeCustomizeReview.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/security',
      name: 'admin-security',
      component: () => import('@/views/admin/security/AdminSecurity.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/log-redact',
      name: 'admin-log-redact',
      component: () => import('@/views/admin/security/AdminLogRedact.vue'),
      meta: { requiresAuth: true },
    },
    // 密码轨道：国际轨 / 国密轨 / 双轨（只读状态 + 配置指引）
    {
      path: '/admin/crypto',
      name: 'admin-crypto',
      component: () => import('@/views/admin/security/AdminCrypto.vue'),
      meta: { requiresAuth: true },
    },
    // 蜜罐等级 + 日志记录字段勾选
    {
      path: '/admin/honeypot',
      name: 'admin-honeypot',
      component: () => import('@/views/admin/security/AdminHoneypot.vue'),
      meta: { requiresAuth: true },
    },
    // 注册门槛与防刷 / 充值风控 / 警方工单 / 运营位 / AI 成本与转化
    {
      path: '/admin/gate',
      name: 'admin-gate',
      component: () => import('@/views/admin/security/AdminGate.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/risk',
      name: 'admin-risk',
      component: () => import('@/views/admin/security/AdminRisk.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/tickets',
      name: 'admin-tickets',
      component: () => import('@/views/admin/police/AdminTickets.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ops',
      name: 'admin-ops',
      component: () => import('@/views/admin/site/AdminOps.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ai-cost',
      name: 'admin-ai-cost',
      component: () => import('@/views/admin/ai/AdminAiCost.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/algorithms',
      name: 'admin-algorithms',
      component: () => import('@/views/admin/ai/AdminAlgorithms.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/vulnerabilities',
      name: 'admin-vulnerabilities',
      component: () => import('@/views/admin/security/AdminVulnerabilities.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/compliance',
      name: 'admin-compliance',
      component: () => import('@/views/admin/security/AdminCompliance.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/announcements',
      name: 'admin-announcements',
      component: () => import('@/views/admin/site/AdminAnnouncements.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/rage',
      name: 'admin-rage',
      component: () => import('@/views/admin/site/AdminRage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/comment-policy',
      name: 'admin-comment-policy',
      component: () => import('@/views/admin/content/AdminCommentPolicy.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/theme',
      name: 'admin-theme',
      component: () => import('@/views/admin/site/AdminTheme.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/home-blocks',
      name: 'admin-home-blocks',
      component: () => import('@/views/admin/site/AdminHomeBlocks.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/web-quota',
      name: 'admin-web-quota',
      component: () => import('@/views/admin/business/AdminWebQuota.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/daily-publish',
      name: 'admin-daily-publish',
      component: () => import('@/views/admin/business/AdminPublishQuota.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/token',
      name: 'admin-token',
      component: () => import('@/views/admin/business/AdminToken.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ai-pool',
      name: 'admin-ai-pool',
      component: () => import('@/views/admin/ai/AdminAiPool.vue'),
      meta: { requiresAuth: true },
    },
    // 内容治理：AI 审核（功能配置） / 审核（待办处置）——两个独立入口
    {
      path: '/admin/ai-moderation',
      name: 'admin-ai-moderation',
      component: () => import('@/views/admin/ai/AdminAiModeration.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ai-safety',
      name: 'admin-ai-safety',
      component: () => import('@/views/admin/ai/AdminAiSafety.vue'),
      meta: { requiresAuth: true },
    },
    {
      // 心理危机干预：独立模块页（原为 AI 防越狱里的一个 tab，运营侧要求独立入口）
      path: '/admin/crisis',
      name: 'admin-crisis',
      component: () => import('@/views/admin/content/AdminCrisis.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/sentiment',
      name: 'admin-sentiment',
      component: () => import('@/views/admin/content/AdminSentiment.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/moderation',
      name: 'admin-moderation',
      component: () => import('@/views/admin/content/AdminModerationQueue.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ai-free-quota',
      name: 'admin-ai-free-quota',
      component: () => import('@/views/admin/ai/AdminAiFreeQuota.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/groups',
      name: 'admin-groups',
      component: () => import('@/views/admin/group/index.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ai-logs',
      name: 'admin-ai-logs',
      component: () => import('@/views/admin/ai/AdminAiLogs.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ai-templates',
      name: 'admin-ai-templates',
      component: () => import('@/views/admin/ai/templates.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ai-features',
      name: 'admin-ai-features',
      component: () => import('@/views/admin/ai/AdminAiFeatures.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ai-bot',
      name: 'admin-ai-bot',
      component: () => import('@/views/admin/ai/AdminAiBot.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/payment',
      name: 'admin-payment',
      component: () => import('@/views/admin/business/AdminPayment.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ai-agents',
      name: 'admin-ai-agents',
      component: () => import('@/views/admin/ai/AdminAiAgents.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/agent-review',
      name: 'admin-agent-review',
      component: () => import('@/views/admin/ai/AdminAgentReview.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ai-pricing',
      name: 'admin-ai-pricing',
      component: () => import('@/views/admin/ai/AdminAiPricing.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ai-menu',
      name: 'admin-ai-menu',
      component: () => import('@/views/admin/ai/AdminAiMenu.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/badwords',
      name: 'admin-badwords',
      component: () => import('@/views/admin/content/AdminBadWords.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/emergency',
      name: 'admin-emergency',
      component: () => import('@/views/admin/content/AdminEmergency.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/community',
      name: 'admin-community',
      component: () => import('@/views/admin/content/AdminCommunity.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/softdel',
      name: 'admin-softdel',
      component: () => import('@/views/admin/content/AdminSoftDel.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/blog-categories',
      name: 'admin-blog-categories',
      component: () => import('@/views/admin/content/AdminBlogCategories.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ban',
      name: 'admin-ban',
      component: () => import('@/views/admin/content/AdminBan.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/database',
      name: 'admin-database',
      component: () => import('@/views/admin/site/AdminDatabase.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/health',
      name: 'admin-health',
      component: () => import('@/views/admin/site/AdminHealth.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/blogs',
      name: 'blog-list',
      component: () => import('@/views/blog/BlogList.vue'),
    },
    // 话题：/topics 列表 + /topics/:id 详情（参与博客）
    {
      path: '/topics',
      name: 'topics',
      component: () => import('@/views/forum/Topics.vue'),
    },
    {
      path: '/topics/:id',
      name: 'topic-detail',
      component: () => import('@/views/forum/Topics.vue'),
    },
    // 用户博客主页（/blog/{用户名} 或 /blog/{数字id}，数字是主键、用户名兼容访问）
    {
      path: '/blog/:key',
      name: 'blog-home',
      component: () => import('@/views/blog/BlogHome.vue'),
    },
    // 作者博客管理（/author/blog/{用户名}，数字 id 兼容；仅本人可管理）
    // 注意：new / edit 必须定义在 :key 之前，避免被通配参数抢先匹配
    {
      path: '/author/blog/new',
      name: 'blog-new',
      component: () => import('@/views/blog/BlogEditor.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/blog/edit/:id',
      name: 'blog-edit',
      component: () => import('@/views/blog/BlogEditor.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/blog/:key',
      name: 'blog-manager',
      component: () => import('@/views/blog/BlogManager.vue'),
      meta: { requiresAuth: true },
    },
    // 分类独立页：文章管理（CSDN 式批量操作）+ 分类设置
    {
      path: '/author/blog/:key/category/:id/settings',
      name: 'blog-category-settings',
      component: () => import('@/views/blog/BlogCategorySettings.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/blog/:key/category/:id',
      name: 'blog-category-manage',
      component: () => import('@/views/blog/BlogCategoryManage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/site/Settings.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/settings/theme',
      name: 'settings-theme',
      component: () => import('@/views/site/ThemeSettings.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/settings/web-quota',
      name: 'settings-web-quota',
      component: () => import('@/views/user/WebQuota.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/about',
      name: 'about',
      component: () => import('@/views/site/About.vue'),
    },
    {
      path: '/user/blacklist',
      name: 'user-blacklist',
      component: () => import('@/views/user/Blacklist.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/user/special-follows',
      name: 'user-special-follows',
      component: () => import('@/views/user/SpecialFollows.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/user/wechat-bind',
      name: 'user-wechat-bind',
      component: () => import('@/views/user/WeChatBind.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/feed',
      name: 'feed',
      component: () => import('@/views/home/Feed.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/user/messages',
      name: 'user-messages',
      component: () => import('@/views/user/Messages.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/messages',
      name: 'messages',
      component: () => import('@/views/me/MessageCenter.vue'),
      meta: { requiresAuth: true },
    },
    // ========== 个人中心（/me 路由，按功能类型懒加载子页，消除点击进入的长拉取尾巴） ==========
    {
      path: '/me',
      component: () => import('@/views/me/MeLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', name: 'me', component: () => import('@/views/me/MeOverview.vue') },
        { path: 'works', name: 'me-works', component: () => import('@/views/me/MeWorks.vue') },
        { path: 'blogs', name: 'me-blogs', component: () => import('@/views/me/MeBlogs.vue') },
        { path: 'comments', name: 'me-comments', component: () => import('@/views/me/MeComments.vue') },
        { path: 'stats', name: 'me-stats', component: () => import('@/views/me/MeStats.vue') },
        { path: 'decorate', name: 'me-decorate', component: () => import('@/views/me/MeDecorate.vue') },
      ],
    },
    {
      path: '/reading',
      name: 'reading',
      component: () => import('@/views/me/ReadingNotes.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/reports',
      name: 'my-reports',
      component: () => import('@/views/me/MyReports.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/docs',
      name: 'docs',
      component: () => import('@/views/site/DocsPage.vue'),
    },
    {
      path: '/api-spider',
      name: 'api-spider',
      component: () => import('@/views/site/APIKeySpider.vue'),
    },
    {
      path: '/my-keys',
      name: 'my-keys',
      component: () => import('@/views/me/MyApiKeys.vue'),
      meta: { requiresAuth: true },
    },
    // ========== 用户自定义内容（frame/html/wasm 沙盒） ==========
    {
      path: '/frames',
      name: 'frame-manager',
      component: () => import('@/views/frame/FrameManager.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/templates',
      name: 'author-templates',
      component: () => import('@/views/author/AuthorTemplateSettings.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/plaza',
      name: 'html-plaza',
      component: () => import('@/views/site/HtmlPlaza.vue'),
    },
    {
      path: '/htmls',
      name: 'html-manager',
      component: () => import('@/views/frame/HTMLManager.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/run/:htmlId',
      name: 'html-runner',
      component: () => import('@/views/frame/HtmlRunner.vue'),
    },
    {
      path: '/wasm',
      name: 'wasm-runner',
      component: () => import('@/views/frame/WasmRunner.vue'),
      meta: { requiresAuth: true },
    },
  ],
});

// 🔒 组级角色守卫：/admin/* 默认强制 admin，/police/* 强制 police。
// 但「内容治理子集」必须对警方开放（侧栏也确实给警方链到 /admin/moderation 等），
// 与后端 AdminOrPoliceGovernance 同口径 —— 白名单见 constants/policeAccess.ts。
// 早先这里一律 requiresAdmin，结果警方点侧栏治理入口被守卫踢回首页。
router.getRoutes().forEach((r) => {
  if (r.path.startsWith('/admin')) {
    const policeOK = POLICE_ALLOWED_PATHS.has(r.path);
    r.meta = {
      ...(r.meta || {}),
      requiresAdmin: !policeOK,
      ...(policeOK ? { requiresPolice: true } : {}),
    };
  }
  // 警方台独立成 /police/*：整段都要 requiresPolice，
  // 只守入口页的话，将来加子页面（/police/xxx）会漏掉权限
  if (r.path === '/police' || r.path.startsWith('/police/')) {
    r.meta = { ...(r.meta || {}), requiresPolice: true };
  }
});

// 建站向导状态缓存（进程内只查询一次 /api/setup/status）
const setupState = { checked: false, installed: true };
let setupPromise: Promise<void> | null = null;

async function ensureSetupState() {
  if (setupState.checked) return;
  if (!setupPromise) {
    setupPromise = (async () => {
      try {
        const res = await api.get('/setup/status');
        setupState.installed = !!res.data?.data?.installed;
      } catch {
        setupState.installed = true; // 网络异常时放行，避免误伤
      } finally {
        setupState.checked = true;
      }
    })();
  }
  return setupPromise;
}

// 路由守卫：未安装 → 强制进入建站向导；需要登录的页面自动跳转
router.beforeEach(async (to, _from, next) => {
  await ensureSetupState();
  if (!setupState.installed && to.path !== '/install') {
    next('/install');
    return;
  }
  if (to.meta.requiresAuth || to.meta.requiresAdmin || to.meta.requiresPolice) {
    const authStore = useAuthStore();
    // ⚠️ 必须 await 到底：早期写成 `if (!user && !loading) await fetchUser()`，
    // 而页面初始化也会触发 fetchUser —— 守卫撞上 loading=true 就跳过等待，
    // 此时 user 还没写入 → isLoggedIn=false → 已登录的管理员直接打开 /admin/users
    // 会被踢到 /login?redirect=...（后台所有深链都受影响）。
    // fetchUser 现在是幂等的：已恢复直接返回，正在请求则复用同一个 promise。
    await authStore.fetchUser();
  }

  if (to.meta.requiresAuth) {
    const authStore = useAuthStore();
    if (!authStore.isLoggedIn) {
      // 未登录，跳转到登录页，登录后返回原页面
      next({ path: '/login', query: { redirect: to.fullPath } });
      return;
    }
  }

  // 🔒 角色守卫：admin / police 专属路由拦截非对应角色（即使已登录也不放行）
  if (to.meta.requiresAdmin || to.meta.requiresPolice) {
    const authStore = useAuthStore();
    const role = authStore.user?.role;
    if (to.meta.requiresAdmin && role !== 'admin') {
      next('/');
      return;
    }
    if (to.meta.requiresPolice && role !== 'admin' && role !== 'police') {
      next('/');
      return;
    }
  }

  // 🔒 前缀兜底（防漏写 meta）：所有后台路由历史上都是「平铺 + 只写 requiresAuth」，
  // 90 个 /admin/* 里只有 5 个真正声明了 requiresAdmin —— 于是任何已登录普通用户
  // 都能直接打开 /admin/settings 之类页面（接口会 403，但模板与菜单全渲染出来了）。
  // 这里按路径前缀统一收紧，白名单与后台侧栏共用（constants/policeAccess.ts），
  // 新增路由即使忘了写 meta 也不会漏放行。
  {
    const role = useAuthStore().user?.role;
    if (!canAccessPath(role, to.path)) {
      next('/');
      return;
    }
  }

  next();
});

// 暴露给自定义主题 JS（alist 兼容 window.NVS.route）读取当前路由
(window as any).__nvsRouter__ = router;

export default router;