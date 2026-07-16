import { createRouter, createWebHistory } from 'vue-router';
import Home from '@/views/Home.vue';
import { useAuthStore } from '@/stores/auth';

const router = createRouter({
  history: createWebHistory(),
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
      path: '/blogs',
      name: 'blog-list',
      component: () => import('@/views/BlogList.vue'),
    },
    {
      path: '/blog/:id',
      name: 'blog-detail',
      component: () => import('@/views/BlogDetail.vue'),
    },
    {
      path: '/author/blog/new',
      name: 'blog-new',
      component: () => import('@/views/BlogEditor.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/author/blog/:id',
      name: 'blog-edit',
      component: () => import('@/views/BlogEditor.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/Settings.vue'),
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

// 路由守卫：需要登录的页面自动跳转
router.beforeEach(async (to, _from, next) => {
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