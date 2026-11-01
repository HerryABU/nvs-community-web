<template>
  <el-container class="admin-shell">
    <el-aside :width="effectiveCollapsed ? '64px' : '232px'" class="admin-aside">
      <AdminSidebar :collapsed="effectiveCollapsed" />
    </el-aside>

    <el-container class="admin-right">
      <el-header class="admin-topbar">
        <div class="at-left">
          <el-button text class="at-collapse" @click="onToggle">
            <el-icon size="20"><component :is="collapsed ? Expand : Fold" /></el-icon>
          </el-button>
          <el-breadcrumb separator="/" class="at-crumb">
            <el-breadcrumb-item>管理后台</el-breadcrumb-item>
            <el-breadcrumb-item>{{ pageTitle }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>

        <div class="at-right">
          <el-link class="at-health" :underline="false" @click="$router.push('/admin/health')">
            <el-icon><Monitor /></el-icon><span class="at-health-text">系统健康</span>
          </el-link>
          <el-dropdown @command="onUserCommand">
            <span class="at-user">
              <el-avatar :size="32" class="at-avatar">{{ avatarText }}</el-avatar>
              <span class="at-username">{{ authStore.user?.nickname || '管理员' }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="site">返回站点首页</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="admin-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { Expand, Fold, Monitor, ArrowDown } from '@element-plus/icons-vue';
import AdminSidebar from '@/components/admin/AdminSidebar.vue';
import { useAuthStore } from '@/stores/auth';
import { adminMenuCategories } from '@/views/admin/adminMenuData';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

// 折叠态持久化到 localStorage（企业级：刷新后保持）
const LS_KEY = 'nvs_admin_sidebar_collapsed';
const collapsed = ref(localStorage.getItem(LS_KEY) === '1');
// 窄屏强制折叠：侧栏展开时内容区只剩 ~143px，手机上完全没法用。
// 用 effectiveCollapsed 而不是直接改 collapsed，避免和用户的手动折叠互相打架。
const isNarrow = ref(typeof window !== 'undefined' && window.innerWidth < 768);
if (typeof window !== 'undefined') {
  window.addEventListener('resize', () => {
    isNarrow.value = window.innerWidth < 768;
  });
}
const effectiveCollapsed = computed(() => isNarrow.value || collapsed.value);
function onToggle() {
  collapsed.value = !collapsed.value;
  localStorage.setItem(LS_KEY, collapsed.value ? '1' : '0');
}

// 当前页标题（从菜单数据反查）
const pageTitle = computed(() => {
  if (route.path === '/admin') return '数据概览';
  for (const c of adminMenuCategories) {
    const m = c.items.find((i) => i.path === route.path);
    if (m) return m.title;
  }
  if (route.path.startsWith('/police')) return '警方取管台';
  return '管理后台';
});

const avatarText = computed(() => (authStore.user?.nickname || 'A').slice(0, 1).toUpperCase());

function onUserCommand(cmd: string) {
  if (cmd === 'site') {
    router.push('/');
  } else if (cmd === 'logout') {
    authStore.logout();
    router.push('/login');
  }
}
</script>

<style scoped>
.admin-shell {
  height: 100vh;
  background: var(--bg-color, #f5f7fb);
}
.admin-aside {
  transition: width .2s ease;
  overflow: hidden;
}
.admin-right { height: 100vh; display: flex; flex-direction: column; }
.admin-topbar {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  background: var(--bg-card, #fff);
  border-bottom: 1px solid var(--border-color, #e8eaf0);
  flex-shrink: 0;
  position: sticky;
  top: 0;
  z-index: 10;
}
.at-left { display: flex; align-items: center; gap: 8px; min-width: 0; }
.at-collapse { color: var(--text-light, #64748b); }
.at-crumb { margin-left: 4px; }
.at-right { display: flex; align-items: center; gap: 18px; }
.at-health {
  display: inline-flex; align-items: center; gap: 5px;
  color: var(--text-light, #64748b);
  cursor: pointer;
}
.at-health:hover { color: var(--primary-color, #6366f1); }
.at-user {
  display: inline-flex; align-items: center; gap: 8px; cursor: pointer;
  color: var(--text-color, #334155);
}
.at-avatar {
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
  color: #fff; font-weight: 700;
}
.at-username { font-size: 0.9rem; max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.admin-main { padding: 20px; overflow-y: auto; flex: 1; }

[data-theme="dark"] .admin-topbar { background: rgba(20,28,42,0.92); border-color: rgba(71,85,105,0.35); }
[data-theme="dark"] .admin-shell { background: #0f172a; }
[data-theme="dark"] .at-user { color: #e2e8f0; }
[data-theme="dark"] .at-health { color: #94a3b8; }

/* 窄屏：顶栏塞不下会横向溢出（实测 375px 溢出 35px）。
   处理：只留图标/头像、收紧间距、面包屑截断。 */
@media (max-width: 768px) {
  .admin-topbar { padding: 0 10px; }
  .at-left { flex: 1; min-width: 0; overflow: hidden; }
  .at-crumb { overflow: hidden; white-space: nowrap; }
  .at-right { gap: 10px; min-width: 0; }
  .at-health-text,
  .at-username { display: none; }
}
</style>
