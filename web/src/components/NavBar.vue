<template>
  <nav class="navbar">
    <div class="navbar-inner">
      <!-- Logo -->
      <div class="navbar-brand" @click="$router.push('/')">
        <span class="brand-text">{{ siteName }}</span>
      </div>

      <!-- 分类下拉 -->
      <el-dropdown class="category-dropdown" trigger="click">
        <span class="category-trigger">
          分类 <el-icon><ArrowDown /></el-icon>
        </span>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item
              v-for="cat in categories"
              :key="cat"
              @click="$router.push(`/category/${encodeURIComponent(cat)}`)"
            >
              {{ cat }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <!-- 论坛入口 -->
      <router-link to="/forums" class="nav-link">论坛</router-link>

      <!-- 博客入口 -->
      <router-link to="/blogs" class="nav-link">博客</router-link>

      <!-- 书架入口（需登录） -->
      <router-link v-if="authStore.isLoggedIn" to="/bookshelf" class="nav-link">📚 书架</router-link>

      <!-- 远程站点 -->
      <el-dropdown v-if="federatedSites.length > 0" trigger="click">
        <span class="nav-link">
          互联站点 <el-icon><ArrowDown /></el-icon>
        </span>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item
              v-for="site in federatedSites"
              :key="site.id"
              @click="openFederated(site)"
            >
              {{ site.name }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <!-- 搜索框 -->
      <div class="search-box">
        <el-input
          v-model="searchText"
          placeholder="搜索作品..."
          size="small"
          clearable
          @keyup.enter="handleSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
      </div>

      <!-- 右侧 -->
      <div class="navbar-right">
        <!-- 主题切换按钮 -->
        <el-button
          circle
          size="small"
          class="theme-toggle-btn"
          @click="themeStore.toggle()"
          :title="themeStore.isDark ? '切换到白天模式' : '切换到黑夜模式'"
        >
          <span class="theme-icon">{{ themeStore.isDark ? '🌙' : '☀️' }}</span>
        </el-button>

        <template v-if="authStore.isLoggedIn">
          <!-- 创作入口 -->
          <el-dropdown trigger="click">
            <span class="nav-link nav-link-create">
              <el-icon><EditPen /></el-icon> 创作
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="$router.push('/author/editor')">
                  <el-icon><Plus /></el-icon> 发布作品
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/author')">
                  <el-icon><HomeFilled /></el-icon> 作者工作台
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>

          <el-dropdown trigger="click">
            <span class="user-info">
              <el-avatar :size="32" :src="authStore.user?.avatar_url">
                {{ authStore.user?.nickname?.[0] || authStore.user?.username?.[0] || 'U' }}
              </el-avatar>
              <span class="user-name">{{ authStore.user?.nickname || authStore.user?.username }}</span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-if="isAuthor" @click="goAuthorHome">
                  <el-icon><User /></el-icon> 我的作者主页
                </el-dropdown-item>
                <el-dropdown-item v-if="authStore.user?.role === 'admin'" @click="$router.push('/admin')">
                  管理员面板
                </el-dropdown-item>
                <el-dropdown-item @click="authStore.logout()">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
        <template v-else>
          <el-button size="small" text @click="$router.push('/login')">登录</el-button>
          <el-button size="small" type="primary" @click="$router.push('/register')">注册</el-button>
        </template>
      </div>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { useThemeStore } from '@/stores/theme';
import { publicApi } from '@/api/admin';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const themeStore = useThemeStore();

const categories = ref<string[]>([]);
const searchText = ref('');
const siteName = ref('星海文学');
const federatedSites = ref<any[]>([]);

const isAuthor = computed(() => {
  const role = authStore.user?.role;
  return role === 'author' || role === 'vip_author' || role === 'admin';
});

function goAuthorHome() {
  if (authStore.user?.id) {
    router.push(`/author/${authStore.user.id}`);
  }
}

function handleSearch() {
  const q = searchText.value.trim();
  if (q) {
    // 如果当前在首页，用 replace 避免多余历史记录；其他页面用 push
    const nav = route.path === '/' ? router.replace : router.push;
    nav({ path: '/', query: { search: q } });
  } else {
    // 空搜索：清除搜索参数
    if (route.path === '/') {
      router.replace({ path: '/', query: {} });
    }
  }
}

// 同步 URL 中的搜索参数到搜索框（例如从搜索结果页"清除搜索"后同步清空）
watch(
  () => route.query.search,
  (val) => {
    if (!val) {
      searchText.value = '';
    } else if (typeof val === 'string') {
      searchText.value = val;
    }
  },
  { immediate: true }
);

function openFederated(site: any) {
  // 通过本地代理路径访问外部站点内容: /{site-id}/...
  window.open(`/${site.id}`, '_blank');
}

async function loadSiteInfo() {
  try {
    const res = await publicApi.getSiteInfo();
    if (res.data.code === 0 && res.data.data.site_name) {
      siteName.value = res.data.data.site_name;
    }
  } catch (e) { /* use default */ }
}

async function loadCategories() {
  try {
    const res = await publicApi.getCategories();
    if (res.data.code === 0 && Array.isArray(res.data.data)) {
      categories.value = res.data.data;
    }
  } catch (e) { /* fallback: keep empty */ }
}

async function loadFederatedSites() {
  try {
    const res = await publicApi.getFederatedSites();
    if (res.data.code === 0) {
      federatedSites.value = res.data.data || [];
    }
  } catch (e) { /* ignore */ }
}

onMounted(() => {
  loadSiteInfo();
  loadCategories();
  loadFederatedSites();
});
</script>

<style scoped>
/* ====== NavBar — Glassmorphism 2024 ====== */
.navbar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 62px;
  background: var(--glass-bg);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-bottom: 1px solid var(--glass-border);
  z-index: 100;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
  transition: all var(--transition-base);
}

[data-theme="dark"] .navbar {
  background: rgba(15, 17, 23, 0.8);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
}

.navbar-inner {
  max-width: 1200px;
  margin: 0 auto;
  height: 100%;
  display: flex;
  align-items: center;
  gap: 22px;
  padding: 0 20px;
}

.navbar-brand {
  cursor: pointer;
  flex-shrink: 0;
}

.brand-text {
  font-size: 1.35rem;
  font-weight: 300;
  color: var(--text-color);
  letter-spacing: 0.04em;
  transition: color var(--transition-fast);
}

.brand-text:hover {
  color: var(--primary-color);
}

/* Category trigger */
.category-trigger {
  cursor: pointer;
  color: var(--text-color);
  font-size: 0.95rem;
  font-weight: 400;
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 6px 12px;
  border-radius: var(--radius-sm);
  transition: all var(--transition-fast);
  position: relative;
}

.category-trigger::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 12px;
  right: 12px;
  height: 2px;
  background: var(--primary-color);
  border-radius: 2px;
  transform: scaleX(0);
  transition: transform var(--transition-fast);
}

.category-trigger:hover {
  color: var(--primary-color);
}

.category-trigger:hover::after {
  transform: scaleX(1);
}

/* Nav link */
.nav-link {
  color: var(--text-color);
  font-size: 0.95rem;
  font-weight: 400;
  padding: 6px 12px;
  border-radius: var(--radius-sm);
  transition: all var(--transition-fast);
  position: relative;
  cursor: pointer;
  text-decoration: none;
}

.nav-link::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 12px;
  right: 12px;
  height: 2px;
  background: var(--primary-color);
  border-radius: 2px;
  transform: scaleX(0);
  transition: transform var(--transition-fast);
}

.nav-link:hover {
  color: var(--primary-color);
}

.nav-link:hover::after {
  transform: scaleX(1);
}

/* Create link */
.nav-link-create {
  cursor: pointer;
  color: var(--primary-color);
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 6px 14px;
  border-radius: var(--radius-full);
  border: 1.5px dashed var(--primary-color);
  transition: all var(--transition-fast);
  font-size: 0.9rem;
}

.nav-link-create:hover {
  background: rgba(99, 102, 241, 0.08);
  border-style: solid;
  transform: translateY(-1px);
}

[data-theme="dark"] .nav-link-create:hover {
  background: rgba(129, 140, 248, 0.1);
}

/* Theme toggle */
.theme-toggle-btn {
  border: none !important;
  background: transparent !important;
  font-size: 1.1rem;
  transition: transform 0.3s ease;
}

.theme-toggle-btn:hover {
  transform: rotate(20deg);
  background: rgba(99, 102, 241, 0.06) !important;
}

.theme-icon {
  font-size: 1rem;
  line-height: 1;
}

[data-theme="dark"] .theme-toggle-btn:hover {
  background: rgba(129, 140, 248, 0.08) !important;
}

/* Search box */
.search-box {
  flex: 1;
  max-width: 360px;
}

/* Right section */
.navbar-right {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-left: auto;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 2px 4px;
  border-radius: var(--radius-sm);
  transition: background var(--transition-fast);
}

.user-info:hover {
  background: rgba(99, 102, 241, 0.06);
}

.user-name {
  font-size: 0.9rem;
  font-weight: 400;
  color: var(--text-color);
}

@media (max-width: 768px) {
  .search-box {
    display: none;
  }

  .category-dropdown {
    display: none;
  }
}
</style>