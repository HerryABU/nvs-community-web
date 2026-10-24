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

      <!-- 主页入口 -->
      <router-link to="/" class="nav-link desktop-only">主页</router-link>

      <!-- 小说入口（作品信息流） -->
      <router-link to="/feed" class="nav-link desktop-only">小说</router-link>

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

      <!-- 群组入口（半私密协作空间：群号/二维码加入、群公告、群文件、群内 @ai） -->
      <router-link to="/groups" class="nav-link desktop-only">群组</router-link>

      <!-- 博客入口 -->
      <router-link to="/blogs" class="nav-link desktop-only">博客</router-link>

      <!-- AI 工作台（全 AI 台：对话/绘画/视频/智能体，与编辑器内「AI 辅助创作台」区分） -->
      <router-link to="/ai" class="nav-link nav-link-ai desktop-only">✨ AI 工作台</router-link>

      <!-- 扩展应用（管理+广场） -->
      <router-link to="/htmls" class="nav-link nav-link-create desktop-only">🔌 扩展应用</router-link>

      <!-- 站长自定义菜单（gap-audit #20）：站内路由用 router-link，外链用 a + noopener -->
      <template v-for="m in dynamicMenus" :key="m.id">
        <el-dropdown v-if="m.children && m.children.length" class="desktop-only" trigger="hover">
          <span class="nav-link">
            <span v-if="m.icon" class="nav-emoji">{{ m.icon }}</span>{{ m.title }}
            <span v-if="m.badge" class="nav-badge">{{ m.badge }}</span>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-for="c in m.children" :key="c.id" @click="goMenu(c)">
                <span v-if="c.icon" class="nav-emoji">{{ c.icon }}</span>{{ c.title }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <a v-else-if="isExternal(m.url)" :href="m.url" target="_blank" rel="noopener noreferrer"
           class="nav-link desktop-only">
          <span v-if="m.icon" class="nav-emoji">{{ m.icon }}</span>{{ m.title }}
          <span v-if="m.badge" class="nav-badge">{{ m.badge }}</span>
        </a>
        <router-link v-else :to="m.url" class="nav-link desktop-only">
          <span v-if="m.icon" class="nav-emoji">{{ m.icon }}</span>{{ m.title }}
          <span v-if="m.badge" class="nav-badge">{{ m.badge }}</span>
        </router-link>
      </template>

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

      <!-- 直播入口（站长开启才显示，gap-audit #23） -->
      <router-link v-if="liveEnabled" to="/live" class="nav-link desktop-only">
        <span v-if="liveStatus === 'live'" class="live-dot" />📺 直播
      </router-link>

      <!-- 每日签到（站长开启才显示） -->
      <button v-if="authStore.isLoggedIn && checkinEnabled" class="nav-link nav-checkin desktop-only"
        :disabled="checkinLoading" :title="checkinDone ? '今天已签到' : '每日签到领代币'"
        @click="doCheckin">
        {{ checkinDone ? '✅ 已签到' : '📅 签到' }}
      </button>

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
                <el-dropdown-item @click="$router.push('/me')">
                  <el-icon><User /></el-icon> 个人中心
                </el-dropdown-item>
                <el-dropdown-item @click="triggerAvatarUpload"><el-icon><Camera /></el-icon> 更换头像</el-dropdown-item>
                <el-dropdown-item @click="$router.push('/author/templates')">
                  <el-icon><Brush /></el-icon> 模板设置
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/reports')">
                  <el-icon><Warning /></el-icon> 我的举报
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/docs')">
                  <el-icon><Document /></el-icon> 格式使用文档
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/api-spider')">
                  <el-icon><Connection /></el-icon> API Key 爬虫指南
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/my-keys')">
                  <el-icon><Key /></el-icon> 我的 API Key
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/wallet')">
                  <el-icon><Wallet /></el-icon> 我的钱包
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/ai')">
                  <el-icon><MagicStick /></el-icon> AI 工作台
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/user/blacklist')">
                  <el-icon><CircleClose /></el-icon> 黑名单管理
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/user/special-follows')">
                  <el-icon><Star /></el-icon> 特别关注
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/feed')">
                  <el-icon><Bell /></el-icon> 特别关注动态
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/user/messages')">
                  <el-icon><ChatLineRound /></el-icon> 我的私信
                </el-dropdown-item>
                <el-dropdown-item @click="$router.push('/settings')">
                  <el-icon><Setting /></el-icon> 账号设置
                </el-dropdown-item>
                <el-dropdown-item v-if="authStore.user?.role === 'admin'" divided @click="$router.push('/admin')">
                  <el-icon><Setting /></el-icon> 管理员面板
                </el-dropdown-item>
                <el-dropdown-item
                  v-if="authStore.user?.role === 'admin' || authStore.user?.role === 'police'"
                  :divided="authStore.user?.role !== 'admin'"
                  @click="$router.push('/police-console')"
                >
                  <el-icon><Stamp /></el-icon> 警方取管台
                </el-dropdown-item>
                <el-dropdown-item divided @click="authStore.logout()">退出登录</el-dropdown-item>
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
        <div class="mnav-item" @click="mnavGo('/ai')"><el-icon><MagicStick /></el-icon> AI 工作台</div>
        <div class="mnav-item" @click="mnavGo('/reading')"><el-icon><Notebook /></el-icon> 精读笔记</div>

        <div class="mnav-group-title">账号</div>
        <div class="mnav-item" @click="mnavGo('/me')"><el-icon><User /></el-icon> 个人中心</div>
        <div class="mnav-item" @click="mnavGo('/author/templates')"><el-icon><Brush /></el-icon> 模板设置</div>
        <div class="mnav-item" @click="mnavGo('/reports')"><el-icon><Warning /></el-icon> 我的举报</div>
        <div class="mnav-item" @click="mnavGo('/docs')"><el-icon><Document /></el-icon> 格式使用文档</div>
        <div class="mnav-item" @click="mnavGo('/api-spider')"><el-icon><Connection /></el-icon> API Key 爬虫指南</div>
        <div class="mnav-item" @click="mnavGo('/my-keys')"><el-icon><Key /></el-icon> 我的 API Key</div>
        <div class="mnav-item" @click="mnavGo('/wallet')"><el-icon><Wallet /></el-icon> 我的钱包</div>
        <div class="mnav-item" @click="mnavGo('/ai')"><el-icon><MagicStick /></el-icon> AI 工作台</div>
        <div class="mnav-item" v-if="authStore.user?.role === 'admin'" @click="mnavGo('/admin')"><el-icon><Setting /></el-icon> 管理员面板</div>
        <div class="mnav-item" v-if="authStore.user?.role === 'admin' || authStore.user?.role === 'police'" @click="mnavGo('/police-console')"><el-icon><Stamp /></el-icon> 警方取管台</div>
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
import { storeToRefs } from 'pinia';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { useThemeStore } from '@/stores/theme';
import { useAppStore } from '@/stores/app';
import { authApi } from '@/api/auth';
import { notificationApi } from '@/api/notification';
import api from '@/api/index';
import { opsApi, type NavMenuItem } from '@/api/ops';
import { appPrefix } from '@/utils/prefix';
import { ElMessage } from 'element-plus';
import { Camera, Setting, Brush, Document, Warning, Bell, Menu, HomeFilled, ChatDotRound, Plus, Notebook, User, SwitchButton, Connection, Key, Stamp, CircleClose, ChatLineRound, Star, Wallet, MagicStick } from '@element-plus/icons-vue';

const avatarInput = ref<HTMLInputElement>();
const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const themeStore = useThemeStore();
const appStore = useAppStore();
const { categories, siteName, federatedSites, unreadCount } = storeToRefs(appStore);
const searchText = ref('');

// ====== 移动端导航抽屉 ======
const mobileMenuOpen = ref(false);

function mnavGo(path: string) {
  mobileMenuOpen.value = false;
  router.push(path);
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
// unreadCount 来自应用级 store（stores/app.ts 聚合 batch 初始化）
let notifyTimer: ReturnType<typeof setInterval> | null = null;

async function refreshUnread() {
  if (!authStore.isLoggedIn) { appStore.unreadCount = 0; return; }
  try {
    const res = await notificationApi.unreadCount();
    if (res.data.code === 0) {
      appStore.unreadCount = Number(res.data.data?.count) || 0;
    }
  } catch { /* 忽略，保持旧值 */ }
}

// 登录态变化时启停轮询
// 注：首屏不必再单独拉未读数 —— 它由应用级 bootstrap 的聚合 batch 一并带回。
// 仅在会话中真实发生"登录切换"（挂载 3 秒之后）才立即刷新，避免首屏多一次直连请求。
const navMountedAt = Date.now();
watch(
  () => authStore.isLoggedIn,
  (v) => {
    if (v) {
      if (Date.now() - navMountedAt > 3000) refreshUnread();
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
  // 🔀 前缀挂载时需带 /go-nvs，否则代理规则不命中
  window.open(`${appPrefix}/${site.id}`, '_blank');
}

// ===== 站长自定义菜单（gap-audit #20）=====
const dynamicMenus = ref<NavMenuItem[]>([]);

function isExternal(u: string) {
  return /^https?:\/\//i.test(u || '');
}
function goMenu(m: NavMenuItem) {
  if (isExternal(m.url)) {
    window.open(m.url, '_blank', 'noopener,noreferrer');
  } else {
    router.push(m.url);
  }
}
async function loadMenus() {
  try {
    const res = await opsApi.navMenus();
    dynamicMenus.value = res.data.data?.items || [];
  } catch { /* 菜单不可用不影响导航主体 */ }
}

// ===== 直播入口（gap-audit #23）=====
const liveEnabled = ref(false);
const liveStatus = ref('');
async function loadLive() {
  try {
    const res = await api.get('/live');
    liveEnabled.value = !!res.data?.data?.enabled;
    liveStatus.value = res.data?.data?.status || '';
  } catch { /* 未开启或接口不可用，静默 */ }
}

// ===== 每日签到（gap-audit #23）=====
const checkinEnabled = ref(false);
const checkinDone = ref(false);
const checkinLoading = ref(false);
async function loadCheckin() {
  try {
    const res = await opsApi.checkInStatus();
    checkinEnabled.value = !!res.data.data?.enabled;
    checkinDone.value = !!res.data.data?.checked;
  } catch { /* 未开启或未登录，静默 */ }
}
async function doCheckin() {
  if (checkinDone.value) {
    ElMessage.info('今天已经签到过了');
    return;
  }
  checkinLoading.value = true;
  try {
    const res: any = await opsApi.checkIn();
    const d = res.data.data;
    if (d?.already) {
      ElMessage.info('今天已经签到过了');
    } else {
      ElMessage.success(`签到成功：+${d?.reward ?? 0}（连签 ${d?.streak ?? 1} 天）`);
    }
    checkinDone.value = true;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '签到失败');
  } finally {
    checkinLoading.value = false;
  }
}

onMounted(() => {
  appStore.bootstrap(); // 幂等：与 App.vue 重复调用只会发一次 /api/batch
  window.addEventListener('nvs:unread', onUnreadEvent);
  loadMenus();
  loadLive();
  if (authStore.isLoggedIn) loadCheckin();
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
/* 自定义菜单与签到（gap-audit #20 #23） */
.nav-emoji { margin-right: 4px; }
.nav-badge {
  margin-left: 4px; font-size: 0.6rem; padding: 0 4px; border-radius: 6px;
  background: var(--el-color-danger); color: #fff; vertical-align: super;
}
.live-dot {
  display: inline-block; width: 6px; height: 6px; border-radius: 50%;
  background: var(--el-color-danger); margin-right: 4px; animation: live-pulse 1.4s infinite;
}
@keyframes live-pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.25; } }
.nav-checkin {
  background: none; border: 0; cursor: pointer; font: inherit; color: inherit;
  display: inline-flex; align-items: center;
}
.nav-checkin:disabled { opacity: 0.6; cursor: default; }

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
  /* 防止窄屏下栏目被挤压成竖排：超宽时横向滚动兜底（隐藏滚动条） */
  overflow-x: auto;
  scrollbar-width: none;
  -ms-overflow-style: none;
}
.navbar-inner::-webkit-scrollbar { display: none; }

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
.category-dropdown { flex-shrink: 0; }
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
  white-space: nowrap;
  flex-shrink: 0;
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
  white-space: nowrap;
  flex-shrink: 0;
}

/* AI 工作台入口（前页可见，与编辑器内「AI 辅助创作台」区分） */
.nav-link-ai {
  color: var(--primary-color);
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.nav-link-ai.router-link-active { color: var(--primary-color); }

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
  white-space: nowrap;
  flex-shrink: 0;
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
  flex-shrink: 0;
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

/* 中窄屏：收紧间距与内边距，优先保证栏目完整单行展示 */
@media (max-width: 1240px) {
  .navbar-inner { gap: 14px; }
  .nav-link, .category-trigger { padding: 6px 9px; }
  .search-box { max-width: 220px; }
}

@media (max-width: 1080px) {
  .navbar-inner { gap: 10px; padding: 0 14px; }
  .brand-text { font-size: 1.15rem; }
  .nav-link, .category-trigger { padding: 6px 7px; font-size: 0.9rem; }
  .nav-link-create { padding: 6px 10px; }
  .search-box { max-width: 150px; }
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