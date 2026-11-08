<template>
  <div class="admin-sidebar">
    <!-- 品牌区：点击回数据概览 -->
    <div class="as-brand" :class="{ collapsed }" @click="$router.push(homePath)">
      <div class="as-logo">N</div>
      <span v-show="!collapsed" class="as-brand-text">NVS 管理后台</span>
    </div>

    <el-scrollbar class="as-scroll">
      <el-menu
        ref="menuRef"
        :default-active="activeIndex"
        :default-openeds="defaultOpeneds"
        :collapse="collapsed"
        :collapse-transition="false"
        unique-opened
        router
        class="as-menu"
      >
        <!-- 数据概览（管理员）/ 警方取管台（警方） -->
        <el-menu-item :index="homePath">
          <el-icon><DataAnalysis /></el-icon>
          <template #title>{{ homeLabel }}</template>
        </el-menu-item>

        <!-- 分类分组：主分类用「渐变胶囊图标 + 加粗标题」，与下方子条目形成层级差异 -->
        <el-sub-menu v-for="cat in visibleCats" :key="cat.key" :index="'cat-' + cat.key">
          <template #title>
            <!-- ⚠️ 图标必须是 el-icon（渲染成 <i>），不能用 span 包一层：
                 Element Plus 折叠态规则 `.el-menu--collapse > .el-sub-menu__title > span
                 { visibility:hidden; width:0 }` 会把直接子 span 全部藏掉，
                 用 span 包裹会导致 64px 折叠时分类图标也消失、只剩空白行。 -->
            <el-icon class="cat-chip" :style="{ '--chip-bg': cat.color }">
              <component :is="cat.icon" />
            </el-icon>
            <span class="cat-title">{{ cat.title }}</span>
          </template>
          <!-- 子分类：图标统一次级灰，激活时用该条目自己的渐变色做左侧标记 -->
          <el-menu-item
            v-for="m in cat.items"
            :key="m.path"
            :index="m.path"
            :id="'sidem-' + m.path.replace(/\//g, '-')"
            :style="{ '--item-grad': m.color }"
          >
            <el-icon class="item-ico"><component :is="m.icon" /></el-icon>
            <template #title>{{ m.title }}</template>
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-scrollbar>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { DataAnalysis } from '@element-plus/icons-vue';
import { adminMenuCategories } from '@/views/admin/adminMenuData';
import { POLICE_ALLOWED_PATHS } from '@/constants/policeAccess';
import { useAuthStore } from '@/stores/auth';

const props = defineProps<{ collapsed: boolean }>();
const route = useRoute();
const authStore = useAuthStore();

// 警方仅能看到「内容治理子集」：合规与安全、AI 配置、站点配置等对警方不可见。
// 后端已按路由逐项 403，此处只做前端收敛（避免展示不可达入口）。
// 白名单与路由守卫共用 constants/policeAccess.ts，避免两处各写一份而漂移。
const visibleCats = computed(() => {
  // 超级管理员专属入口（如导航模块布局）仅超管可见
  const isSuper = authStore.isSuperAdmin;
  let cats = adminMenuCategories.map((c) => ({
    ...c,
    items: c.items.filter((it) => !it.superOnly || isSuper),
  }));
  if (authStore.user?.role !== 'police') return cats.filter((c) => c.items.length > 0);
  // 警方仅能看到「内容治理子集」
  return cats
    .map((c) => ({ ...c, items: c.items.filter((it) => POLICE_ALLOWED_PATHS.has(it.path)) }))
    .filter((c) => c.items.length > 0);
});

// 首页项：管理员进仪表盘，警方进警方台（/admin 对警方是禁入的）
const homePath = computed(() => (authStore.user?.role === 'police' ? '/police' : '/admin'));
const homeLabel = computed(() => (authStore.user?.role === 'police' ? '警方取管台' : '数据概览'));

// 深链 ?item=<path> 时高亮对应条目（即使当前显示的是仪表盘主页）
const activeIndex = computed(() => {
  const item = route.query.item as string | undefined;
  if (item && typeof item === 'string') return item;
  return route.path;
});

// 当前页所属分类（用长路径前缀匹配，子路由如 /admin/ai-pool 也算命中）
const activeCatIndex = computed(() => {
  const cur = activeIndex.value;
  const cat = visibleCats.value.find((c) =>
    c.items.some((it) => cur === it.path || cur.startsWith(it.path + '/')),
  );
  return cat ? 'cat-' + cat.key : '';
});

// 初次渲染就展开当前分组：否则刷新子页面时侧栏是收起的，
// 高亮项藏在一个关着的分组里，用户会以为"菜单没定位到我"。
const defaultOpeneds = ref<string[]>(activeCatIndex.value ? [activeCatIndex.value] : []);

// 路由变化时跟着展开（unique-opened 会自动收起其它分组，形成手风琴）
const menuRef = ref<any>(null);
watch(
  () => activeCatIndex.value,
  (idx) => {
    if (!idx || !menuRef.value) return;
    // 已展开就不用管，避免每次跳转都触发一次无谓的收起/展开
    const opened: string[] = menuRef.value.openedMenus || [];
    if (opened.includes(idx)) return;
    menuRef.value.open(idx);
  },
);

// 折回去（展开侧栏）时补一次展开当前分组
watch(
  () => props.collapsed,
  (collapsedNow) => {
    if (!collapsedNow && activeCatIndex.value && menuRef.value) {
      menuRef.value.open(activeCatIndex.value);
    }
  },
);
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

/* ── 主分类 vs 子分类：用「颜色形态」区分层级 ──
   主分类：渐变胶囊图标 + 加粗深色标题（彩色、块状）
   子分类：图标统一灰、文字次级；激活时用该条目自己的渐变做左侧标记
   这样一眼能看出「这是分组」还是「这是可点的功能」。 */
/* 分类图标本体就是 el-icon（<i>），背景色走 --chip-bg 由模板注入 */
.cat-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  margin-right: 9px;
  border-radius: 7px;
  background: var(--chip-bg, var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6)));
  color: #fff;
  flex: none;
  font-size: 13px;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.16);
}
.cat-title {
  font-weight: 700;
  letter-spacing: 0.2px;
  color: var(--text-main, #1e293b);
  font-size: 0.9rem;
}
/* 子条目图标：比主分类弱一档，形成层级 */
.as-menu :deep(.el-menu-item) .item-ico {
  color: var(--text-light, #94a3b8);
  transition: color .15s;
}
.as-menu :deep(.el-menu-item:hover) .item-ico { color: var(--text-color, #475569); }

.as-menu :deep(.el-menu-item.is-active) {
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
  color: #fff;
  font-weight: 600;
}
/* 激活项左侧色条：用该功能自己的主题色（--item-grad 由模板按条目注入） */
.as-menu :deep(.el-menu-item.is-active)::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 18px;
  border-radius: 0 3px 3px 0;
  background: var(--item-grad, #fff);
}
.as-menu :deep(.el-menu-item) { position: relative; }
.as-menu :deep(.el-menu-item.is-active) .item-ico { color: #fff; }

/* 子条目比主分类多缩进一档，形成"从属"的视觉关系；
   只靠颜色区分层级不够，缩进才是最直接的从属信号。 */
.as-menu :deep(.el-sub-menu .el-menu-item) {
  margin: 3px 8px 3px 20px;
  width: calc(100% - 28px);
  height: 42px;
  line-height: 42px;
  font-size: 0.86rem;
}
/* 层级引导线：从分组标题下方垂到子项区域，明确"这几个属于上面那组" */
.as-menu :deep(.el-sub-menu .el-menu) {
  position: relative;
  padding-left: 0;
}
.as-menu :deep(.el-sub-menu .el-menu)::before {
  content: '';
  position: absolute;
  left: 17px;
  top: 2px;
  bottom: 6px;
  width: 1px;
  background: var(--border-color, #e8eaf0);
}

[data-theme="dark"] .as-brand { border-color: rgba(71,85,105,0.35); }
[data-theme="dark"] .admin-sidebar { background: rgba(20,28,42,0.92); border-color: rgba(71,85,105,0.35); }
[data-theme="dark"] .as-menu :deep(.el-menu-item),
[data-theme="dark"] .as-menu :deep(.el-sub-menu__title) { color: #cbd5e1; }
[data-theme="dark"] .as-menu :deep(.el-menu-item:hover),
[data-theme="dark"] .as-menu :deep(.el-sub-menu__title:hover) { background: rgba(255,255,255,0.06); }
[data-theme="dark"] .as-menu :deep(.el-sub-menu .el-menu-item) { background: transparent; }
</style>
