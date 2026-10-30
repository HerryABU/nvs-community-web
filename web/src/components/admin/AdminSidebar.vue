<template>
  <div class="admin-sidebar">
    <!-- 品牌区：点击回数据概览 -->
    <div class="as-brand" :class="{ collapsed }" @click="$router.push('/admin')">
      <div class="as-logo">N</div>
      <span v-show="!collapsed" class="as-brand-text">NVS 管理后台</span>
    </div>

    <el-scrollbar class="as-scroll">
      <el-menu
        :default-active="activeIndex"
        :collapse="collapsed"
        :collapse-transition="false"
        router
        class="as-menu"
      >
        <!-- 数据概览（仪表盘主页） -->
        <el-menu-item index="/admin">
          <el-icon><DataAnalysis /></el-icon>
          <template #title>数据概览</template>
        </el-menu-item>

        <!-- 分类分组 -->
        <el-sub-menu v-for="cat in visibleCats" :key="cat.key" :index="'cat-' + cat.key">
          <template #title>
            <el-icon><component :is="cat.icon" /></el-icon>
            <span>{{ cat.title }}</span>
          </template>
          <el-menu-item
            v-for="m in cat.items"
            :key="m.path"
            :index="m.path"
            :id="'sidem-' + m.path.replace(/\//g, '-')"
          >
            <el-icon><component :is="m.icon" /></el-icon>
            <template #title>{{ m.title }}</template>
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-scrollbar>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import { DataAnalysis } from '@element-plus/icons-vue';
import { adminMenuCategories } from '@/views/admin/adminMenuData';
import { useAuthStore } from '@/stores/auth';

defineProps<{ collapsed: boolean }>();
const route = useRoute();
const authStore = useAuthStore();

// 警方仅能看到「内容治理子集」：合规与安全、AI 配置、站点配置等对警方不可见。
// 后端已按路由逐项 403，此处只做前端收敛（避免展示不可达入口）。
const policePaths = new Set<string>([
  '/police',
  '/admin/moderation',
  '/admin/reports',
  '/admin/ban',
  '/admin/ai-moderation',
  '/admin/badwords',
  '/admin/softdel',
  '/admin/sentiment',
  '/admin/hotwords',
]);

const visibleCats = computed(() => {
  if (authStore.user?.role !== 'police') return adminMenuCategories;
  return adminMenuCategories
    .map((c) => ({ ...c, items: c.items.filter((it) => policePaths.has(it.path)) }))
    .filter((c) => c.items.length > 0);
});

// 深链 ?item=<path> 时高亮对应条目（即使当前显示的是仪表盘主页）
const activeIndex = computed(() => {
  const item = route.query.item as string | undefined;
  if (item && typeof item === 'string') return item;
  return route.path;
});
</script>

<style scoped>
.admin-sidebar {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--bg-card, #fff);
  border-right: 1px solid var(--border-color, #e8eaf0);
}

.as-brand {
  height: 56px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border-color, #e8eaf0);
  cursor: pointer;
  flex-shrink: 0;
  overflow: hidden;
  white-space: nowrap;
}
.as-brand.collapsed { justify-content: center; padding: 0; }
.as-logo {
  width: 32px; height: 32px; border-radius: 9px; flex-shrink: 0;
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
  color: #fff; font-weight: 800; font-size: 1.1rem;
  display: flex; align-items: center; justify-content: center;
}
.as-brand-text { font-weight: 700; font-size: 1.02rem; color: var(--text-color, #1e293b); }

.as-scroll { flex: 1; }
.as-menu { border-right: none !important; background: transparent !important; }

/* 折叠态：品牌区不换行、菜单只留图标 */
.as-menu:not(.el-menu--collapse) { width: 100%; }
.as-menu.el-menu--collapse { width: 64px; }

/* 菜单项视觉 */
.as-menu :deep(.el-menu-item),
.as-menu :deep(.el-sub-menu__title) {
  height: 46px;
  line-height: 46px;
  border-radius: 10px;
  margin: 4px 8px;
  width: calc(100% - 16px);
  color: var(--text-color, #334155);
  transition: background .15s, color .15s;
}
.as-menu :deep(.el-menu-item:hover),
.as-menu :deep(.el-sub-menu__title:hover) {
  background: var(--bg-subtle, #f1f5f9);
}
.as-menu :deep(.el-menu-item.is-active) {
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
  color: #fff;
  font-weight: 600;
}
.as-menu :deep(.el-sub-menu .el-menu-item) { margin-left: 4px; }

[data-theme="dark"] .as-brand { border-color: rgba(71,85,105,0.35); }
[data-theme="dark"] .admin-sidebar { background: rgba(20,28,42,0.92); border-color: rgba(71,85,105,0.35); }
[data-theme="dark"] .as-menu :deep(.el-menu-item),
[data-theme="dark"] .as-menu :deep(.el-sub-menu__title) { color: #cbd5e1; }
[data-theme="dark"] .as-menu :deep(.el-menu-item:hover),
[data-theme="dark"] .as-menu :deep(.el-sub-menu__title:hover) { background: rgba(255,255,255,0.06); }
[data-theme="dark"] .as-menu :deep(.el-sub-menu .el-menu-item) { background: transparent; }
</style>
