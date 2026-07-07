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
    {
      path: '/novel/:id',
      name: 'novel-detail',
      component: () => import('@/views/NovelDetail.vue'),
    },
    {
      path: '/novel/:id/read/:chapter',
      name: 'reader',
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
      path: '/author/:id',
      name: 'author-home',
      component: () => import('@/views/AuthorHome.vue'),
    },
    {
      path: '/forums',
      name: 'forums',
      component: () => import('@/views/Forums.vue'),
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
