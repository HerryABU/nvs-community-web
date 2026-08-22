<template>
  <nav class="navbar">
    <div class="navbar-inner">
      <!-- 移动端汉堡菜单按钮 -->
      <button class="mobile-menu-btn" @click="mobileMenuOpen = true" aria-label="打开菜单">
        <el-icon :size="22"><Menu /></el-icon>
      </button>

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
      <router-link to="/forums" class="nav-link desktop-only">论坛</router-link>

      <!-- 博客入口 -->
      <router-link to="/blogs" class="nav-link desktop-only">博客</router-link>

      <!-- 扩展应用（管理+广场） -->
      <router-link to="/htmls" class="nav-link nav-link-create desktop-only">🔌 扩展应用</router-link>

      <!-- 自定义内容入口（需登录） -->
      <el-dropdown v-if="authStore.isLoggedIn" class="desktop-only" trigger="click">
        <span class="nav-link">
          <el-icon><Brush /></el-icon> 自定义
        </span>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item @click="$router.push('/frames')">
              🎨 UI模板管理
            </el-dropdown-item>
            <el-dropdown-item @click="$router.push('/wasm')">
              ⚙️ WASM 说明
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <!-- 书架入口（需登录） -->
      <router-link v-if="authStore.isLoggedIn" to="/bookshelf" class="nav-link desktop-only">📚 书架</router-link>

      <!-- 远程站点 -->
      <el-dropdown v-if="federatedSites.length > 0" class="desktop-only" trigger="click">
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
          <!-- 消息中心（新消息提醒：红点/红圈数字） -->
          <router-link to="/messages" class="nav-link msg-link" title="消息中心">
            <el-icon><Bell /></el-icon>
            <span class="msg-badge" v-if="unreadCount > 0">{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
          </router-link>

          <!-- 创作入口（移动端隐藏，收入抽屉） -->
          <el-dropdown class="desktop-only" trigger="click">
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
              <span class="user-name desktop-only">{{ authStore.user?.nickname || authStore.user?.username }}</span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="triggerAvatarUpload"><el-icon><Camera /></el-icon> 更换头像</el-dropdown-item>
                <el-dropdown-item @click="goAuthorHome">
                  <el-icon><User /></el-icon> 我的作者主页
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/author/templates')">
                  <el-icon><Brush /></el-icon> 模板设置
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/reports')">
                  <el-icon><Warning /></el-icon> 我的举报
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/docs')">
                  <el-icon><Document /></el-icon> 格式使用文档
                </el-dropdown-item>
                <el-dropdown-item v-if="authStore.user?.role === 'admin'" @click="$router.push('/admin')">
                  管理员面板
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/settings')">
                  <el-icon><Setting /></el-icon> 账号设置
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
  <input ref="avatarInput" type="file" accept="image/png,image/jpeg,image/gif" style="display:none" @change="onAvatarChange" />

  <!-- ====== 移动端导航抽屉 ====== -->
  <el-drawer
    v-model="mobileMenuOpen"
    direction="rtl"
    size="min(78vw, 360px)"
    :with-header="false"
    class="mobile-nav-drawer"
  >
    <div class="mnav">
      <!-- 搜索 -->
      <div class="mnav-search">
        <el-input
          v-model="searchText"
          placeholder="搜索作品、作者、博客..."
          clearable
          size="large"
          @keyup.enter="mnavSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
      </div>

      <!-- 浏览 -->
      <div class="mnav-group-title">浏览</div>
      <div class="mnav-item" @click="mnavGo('/')"><el-icon><HomeFilled /></el-icon> 首页</div>
      <div class="mnav-item" @click="mnavGo('/forums')"><el-icon><ChatDotRound /></el-icon> 论坛</div>
      <div class="mnav-item" @click="mnavGo('/blogs')"><el-icon><Document /></el-icon> 博客</div>
      <div class="mnav-item" @click="mnavGo('/htmls')"><span class="mnav-emoji">🔌</span> 扩展应用</div>
      <div class="mnav-item" v-if="federatedSites.length" @click="mnavFederated"><span class="mnav-emoji">🌐</span> 互联站点</div>

      <!-- 分类 -->
      <div class="mnav-group-title">分类</div>
      <div class="mnav-cats">
        <div v-for="cat in categories" :key="cat" class="mnav-cat" @click="mnavGo(`/category/${encodeURIComponent(cat)}`)">
          {{ cat }}
        </div>
      </div>

      <template v-if="authStore.isLoggedIn">
        <div class="mnav-group-title">我的</div>
        <div class="mnav-item" @click="mnavGo('/bookshelf')"><span class="mnav-emoji">📚</span> 书架</div>
        <div class="mnav-item" @click="mnavGo('/messages')"><el-icon><Bell /></el-icon> 消息中心
          <span v-if="unreadCount > 0" class="mnav-badge">{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
        </div>
        <div class="mnav-item" @click="mnavGo('/author/editor')"><el-icon><Plus /></el-icon> 发布作品</div>
        <div class="mnav-item" @click="mnavGo('/author')"><el-icon><HomeFilled /></el-icon> 作者工作台</div>
        <div class="mnav-item" @click="mnavGo('/reading')"><el-icon><Notebook /></el-icon> 精读笔记</div>

        <div class="mnav-group-title">账号</div>
        <div class="mnav-item" @click="mnavGoAuthorHome"><el-icon><User /></el-icon> 我的作者主页</div>
        <div class="mnav-item" @click="mnavGo('/author/templates')"><el-icon><Brush /></el-icon> 模板设置</div>
        <div class="mnav-item" @click="mnavGo('/reports')"><el-icon><Warning /></el-icon> 我的举报</div>
        <div class="mnav-item" @click="mnavGo('/docs')"><el-icon><Document /></el-icon> 格式使用文档</div>
        <div class="mnav-item" v-if="authStore.user?.role === 'admin'" @click="mnavGo('/admin')"><el-icon><Setting /></el-icon> 管理员面板</div>
        <div class="mnav-item" @click="mnavGo('/settings')"><el-icon><Setting /></el-icon> 账号设置</div>
        <div class="mnav-item mnav-logout" @click="mnavLogout"><el-icon><SwitchButton /></el-icon> 退出登录</div>
      </template>
      <template v-else>
        <div class="mnav-group-title">账号</div>
        <div class="mnav-auth-row">
          <el-button type="primary" size="large" style="flex:1" @click="mnavGo('/login')">登录</el-button>
          <el-button size="large" style="flex:1" @click="mnavGo('/register')">注册</el-button>
        </div>
      </template>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { useThemeStore } from '@/stores/theme';
import { publicApi } from '@/api/admin';
import { authApi } from '@/api/auth';
import { notificationApi } from '@/api/notification';
import { ElMessage } from 'element-plus';
import { Camera, Setting, Brush, Document, Warning, Bell, Menu, HomeFilled, ChatDotRound, Plus, Notebook, User, SwitchButton } from '@element-plus/icons-vue';

const avatarInput = ref<HTMLInputElement>();
const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const themeStore = useThemeStore();

const categories = ref<string[]>([]);
const searchText = ref('');
const siteName = ref('星海文学');
const federatedSites = ref<any[]>([]);

// ====== 移动端导航抽屉 ======
const mobileMenuOpen = ref(false);

function mnavGo(path: string) {
  mobileMenuOpen.value = false;
  router.push(path);
}

function mnavGoAuthorHome() {
  mobileMenuOpen.value = false;
  if (authStore.user?.id) router.push(`/author/${authStore.user.id}`);
}

function mnavSearch() {
  handleSearch();
  mobileMenuOpen.value = false;
}

function mnavFederated() {
  mobileMenuOpen.value = false;
  if (federatedSites.value.length > 0) openFederated(federatedSites.value[0]);
}

function mnavLogout() {
  mobileMenuOpen.value = false;
  authStore.logout();
}

// ====== 新消息提醒（红圈数字徽标，30s 轮询） ======
const unreadCount = ref(0);
let notifyTimer: ReturnType<typeof setInterval> | null = null;

async function refreshUnread() {
  if (!authStore.isLoggedIn) { unreadCount.value = 0; return; }
  try {
    const res = await notificationApi.unreadCount();
    if (res.data.code === 0) {
      unreadCount.value = Number(res.data.data?.count) || 0;
    }
  } catch { /* 忽略，保持旧值 */ }
}

// 登录态变化时启停轮询
watch(
  () => authStore.isLoggedIn,
  (v) => {
    if (v) {
      refreshUnread();
      if (!notifyTimer) notifyTimer = setInterval(refreshUnread, 30000);
    } else if (notifyTimer) {
      clearInterval(notifyTimer);
      notifyTimer = null;
      unreadCount.value = 0;
    }
  },
  { immediate: true }
);

// 消息中心标记已读后立即同步徽标（nvs:unread 事件）
function onUnreadEvent(e: Event) {
  const detail = (e as CustomEvent).detail;
  if (typeof detail === 'number') unreadCount.value = detail;
}

const isAuthor = computed(() => {
  const role = authStore.user?.role;
  return role === 'author' || role === 'vip_author' || role === 'admin';
});

function goAuthorHome() {
  if (authStore.user?.id) {
    router.push(`/author/${authStore.user.id}`);
  }
}

function triggerAvatarUpload() { avatarInput.value?.click(); }

async function onAvatarChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0];
  if (!file) return;
  const fd = new FormData();
  fd.append('avatar', file);
  try {
    const res = await authApi.uploadAvatar(fd);
    if (res.data.code === 0) {
      authStore.user!.avatar_url = res.data.data.avatar_url;
      ElMessage.success('头像已更新');
    } else {
      ElMessage.error(res.data.message || '上传失败');
    }
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.message || '上传失败，请使用 JPG/PNG/GIF/WebP 格式');
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
  window.addEventListener('nvs:unread', onUnreadEvent);
});

onUnmounted(() => {
  if (notifyTimer) {
    clearInterval(notifyTimer);
    notifyTimer = null;
  }
  window.removeEventListener('nvs:unread', onUnreadEvent);
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

/* 消息中心入口 + 未读徽标（红圈数字，>99 显示 99+） */
.msg-link {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 1.05rem;
}

.msg-badge {
  position: absolute;
  top: 0;
  right: 0;
  transform: translate(30%, -20%);
  min-width: 17px;
  height: 17px;
  padding: 0 4px;
  border-radius: 9px;
  background: #ef4444;
  color: #fff;
  font-size: 0.7rem;
  font-weight: 700;
  line-height: 17px;
  text-align: center;
  box-shadow: 0 0 0 2px var(--glass-bg, #fff);
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

/* ====== 移动端汉堡菜单按钮 ====== */
.mobile-menu-btn {
  display: none;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border: none;
  background: transparent;
  color: var(--text-color);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: background var(--transition-fast);
  padding: 0;
}

.mobile-menu-btn:hover {
  background: rgba(99, 102, 241, 0.08);
}

[data-theme="dark"] .mobile-menu-btn:hover {
  background: rgba(129, 140, 248, 0.12);
}

@media (max-width: 768px) {
  .mobile-menu-btn {
    display: flex;
  }
}

/* ====== 移动端导航抽屉 ====== */
.mobile-nav-drawer :deep(.el-drawer__body) {
  padding: 0;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

.mnav {
  padding: 16px 14px 32px;
}

.mnav-search {
  margin-bottom: 16px;
}

.mnav-group-title {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  color: var(--text-muted);
  text-transform: uppercase;
  margin: 20px 4px 8px;
}

.mnav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 12px;
  margin: 2px 0;
  border-radius: var(--radius-sm);
  font-size: 0.95rem;
  color: var(--text-color);
  cursor: pointer;
  transition: background var(--transition-fast), color var(--transition-fast);
  position: relative;
}

.mnav-item:hover,
.mnav-item:active {
  background: rgba(99, 102, 241, 0.08);
  color: var(--primary-color);
}

[data-theme="dark"] .mnav-item:hover,
[data-theme="dark"] .mnav-item:active {
  background: rgba(129, 140, 248, 0.12);
}

.mnav-emoji {
  font-size: 1.05rem;
  line-height: 1;
}

.mnav-badge {
  margin-left: auto;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 9px;
  background: #ef4444;
  color: #fff;
  font-size: 0.7rem;
  font-weight: 700;
  line-height: 18px;
  text-align: center;
}

.mnav-logout {
  color: var(--danger-color);
}

.mnav-cats {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.mnav-cat {
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--bg-soft);
  color: var(--text-color);
  font-size: 0.9rem;
  text-align: center;
  cursor: pointer;
  transition: all var(--transition-fast);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mnav-cat:hover,
.mnav-cat:active {
  background: rgba(99, 102, 241, 0.12);
  color: var(--primary-color);
}

[data-theme="dark"] .mnav-cat {
  background: #1e293b;
}

.mnav-auth-row {
  display: flex;
  gap: 10px;
  margin-top: 4px;
}
</style>