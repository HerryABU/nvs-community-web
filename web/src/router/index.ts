import { createRouter, createWebHistory } from 'vue-router';
import Home from '@/views/Home.vue';
import { useAuthStore } from '@/stores/auth';
import api from '@/api';

const router = createRouter({
  // 🔀 站点路径前缀支持（URL_PREFIX，如 /go-nvs）：后端在 index.html 注入 window.__NVS_PREFIX__，
  // 此处作为 Vue Router base，使页面 URL 保持 /go-nvs/xxx 且刷新/跳转正确
  history: createWebHistory((window as any).__NVS_PREFIX__ || '/'),
  routes: [
    {
      path: '/',
      name: 'home',
      component: Home,
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/Login.vue'),
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/views/Register.vue'),
    },
    // 建站向导（仅未安装时可进入，安装后永久关闭；/install 为规范入口，/setup 兼容重定向）
    {
      path: '/install',
      name: 'install',
      component: () => import('@/views/SetupWizard.vue'),
    },
    {
      path: '/setup',
      redirect: '/install',
    },
    // 旧路径（兼容）
    {
      path: '/novel/:id',
      name: 'novel-detail',
      component: () => import('@/views/NovelDetail.vue'),
    },
    // 旧路径（兼容，Reader内部获取作者后自动替换URL）
    {
      path: '/novel/:id/read/:chapter',
      name: 'reader',
      component: () => import('@/views/Reader.vue'),
    },
    // 公开作者主页（新版，含统计图表）
    {
      path: '/author/:id',
      name: 'author-home',
      component: () => import('@/views/AuthorHome.vue'),
    },
    // 新路径（推荐）：/author/:authorId/novel/:id
    {
      path: '/author/:authorId/novel/:id',
      name: 'novel-detail-new',
      component: () => import('@/views/NovelDetail.vue'),
      alias: '/author/:authorId/novel/:id/',  // 尾斜杠兼容
    },
    // 新路径（推荐）：/author/:authorId/novel/:id/read/:chapter
    {
      path: '/author/:authorId/novel/:id/read/:chapter',
      name: 'reader-new',
      component: () => import('@/views/Reader.vue'),
    },
    {
      path: '/author',
      name: 'author-dashboard',
      component: () => import('@/views/AuthorDashboard.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/editor/:id?',
      name: 'editor',
      component: () => import('@/views/Editor.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/editor/:id/chapter/:num',
      name: 'chapter-editor',
      component: () => import('@/views/ChapterEditor.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/category/:name',
      name: 'category',
      component: () => import('@/views/CategoryView.vue'),
    },
    {
      path: '/author/:id/blog',
      redirect: to => ({ name: 'author-blogs', params: { id: to.params.id } }),
    },
    // 作者博客列表
    {
      path: '/author/:id/blogs',
      name: 'author-blogs',
      component: () => import('@/views/AuthorBlogs.vue'),
    },
    {
      path: '/author/:id/blogs/:blogId',
      name: 'author-blog-detail',
      component: () => import('@/views/BlogDetail.vue'),
    },
    {
      path: '/forums',
      name: 'forums',
      component: () => import('@/views/Forums.vue'),
    },
    {
      path: '/bookshelf',
      name: 'bookshelf',
      component: () => import('@/views/Bookshelf.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/forum/:id',
      name: 'forum-detail',
      component: () => import('@/views/ForumDetail.vue'),
    },
    {
      path: '/thread/:id',
      name: 'thread-detail',
      component: () => import('@/views/ThreadDetail.vue'),
    },
    {
      path: '/admin',
      name: 'admin-dashboard',
      component: () => import('@/views/AdminDashboard.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/users',
      name: 'admin-users',
      component: () => import('@/views/UserManagement.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/settings',
      name: 'admin-settings',
      component: () => import('@/views/admin/AdminSettings.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/categories',
      name: 'admin-categories',
      component: () => import('@/views/admin/AdminCategories.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/wall',
      name: 'admin-wall',
      component: () => import('@/views/admin/AdminWall.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/sites',
      name: 'admin-sites',
      component: () => import('@/views/admin/AdminSites.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/forums',
      name: 'admin-forums',
      component: () => import('@/views/admin/AdminForums.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/reports',
      name: 'admin-reports',
      component: () => import('@/views/admin/AdminReports.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/html-review',
      name: 'admin-html-review',
      component: () => import('@/views/admin/AdminHtmlReview.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/security',
      name: 'admin-security',
      component: () => import('@/views/admin/AdminSecurity.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/log-redact',
      name: 'admin-log-redact',
      component: () => import('@/views/admin/AdminLogRedact.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/algorithms',
      name: 'admin-algorithms',
      component: () => import('@/views/admin/AdminAlgorithms.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/vulnerabilities',
      name: 'admin-vulnerabilities',
      component: () => import('@/views/admin/AdminVulnerabilities.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/compliance',
      name: 'admin-compliance',
      component: () => import('@/views/admin/AdminCompliance.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/announcements',
      name: 'admin-announcements',
      component: () => import('@/views/admin/AdminAnnouncements.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/badwords',
      name: 'admin-badwords',
      component: () => import('@/views/admin/AdminBadWords.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/emergency',
      name: 'admin-emergency',
      component: () => import('@/views/admin/AdminEmergency.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/community',
      name: 'admin-community',
      component: () => import('@/views/admin/AdminCommunity.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/softdel',
      name: 'admin-softdel',
      component: () => import('@/views/admin/AdminSoftDel.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/blog-categories',
      name: 'admin-blog-categories',
      component: () => import('@/views/admin/AdminBlogCategories.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/ban',
      name: 'admin-ban',
      component: () => import('@/views/admin/AdminBan.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/database',
      name: 'admin-database',
      component: () => import('@/views/admin/AdminDatabase.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/admin/health',
      name: 'admin-health',
      component: () => import('@/views/admin/AdminHealth.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/blogs',
      name: 'blog-list',
      component: () => import('@/views/BlogList.vue'),
    },
    // 话题：/topics 列表 + /topics/:id 详情（参与博客）
    {
      path: '/topics',
      name: 'topics',
      component: () => import('@/views/Topics.vue'),
    },
    {
      path: '/topics/:id',
      name: 'topic-detail',
      component: () => import('@/views/Topics.vue'),
    },
    // 用户博客主页（/blog/{用户名} 或 /blog/{数字id}，数字是主键、用户名兼容访问）
    {
      path: '/blog/:key',
      name: 'blog-home',
      component: () => import('@/views/BlogHome.vue'),
    },
    // 作者博客管理（/author/blog/{用户名}，数字 id 兼容；仅本人可管理）
    // 注意：new / edit 必须定义在 :key 之前，避免被通配参数抢先匹配
    {
      path: '/author/blog/new',
      name: 'blog-new',
      component: () => import('@/views/BlogEditor.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/blog/edit/:id',
      name: 'blog-edit',
      component: () => import('@/views/BlogEditor.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/blog/:key',
      name: 'blog-manager',
      component: () => import('@/views/BlogManager.vue'),
      meta: { requiresAuth: true },
    },
    // 分类独立页：文章管理（CSDN 式批量操作）+ 分类设置
    {
      path: '/author/blog/:key/category/:id/settings',
      name: 'blog-category-settings',
      component: () => import('@/views/BlogCategorySettings.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/blog/:key/category/:id',
      name: 'blog-category-manage',
      component: () => import('@/views/BlogCategoryManage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/Settings.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/messages',
      name: 'messages',
      component: () => import('@/views/MessageCenter.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/reading',
      name: 'reading',
      component: () => import('@/views/ReadingNotes.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/reports',
      name: 'my-reports',
      component: () => import('@/views/MyReports.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/docs',
      name: 'docs',
      component: () => import('@/views/DocsPage.vue'),
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
      component: () => import('@/views/AuthorTemplateSettings.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/plaza',
      name: 'html-plaza',
      component: () => import('@/views/HtmlPlaza.vue'),
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
  if (to.meta.requiresAuth) {
    const authStore = useAuthStore();
    // 如果用户信息还没加载，先尝试恢复
    if (!authStore.user && !authStore.loading) {
      await authStore.fetchUser();
    }
    if (!authStore.isLoggedIn) {
      // 未登录，跳转到登录页，登录后返回原页面
      next({ path: '/login', query: { redirect: to.fullPath } });
      return;
    }
  }
  next();
});

export default router;