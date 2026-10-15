<template>
  <div class="menu-section">
    <h2 class="menu-section-title">管理功能</h2>
    <el-collapse v-model="openCats" class="amp-collapse">
      <el-collapse-item v-for="cat in categories" :key="cat.key" :name="cat.key">
        <template #title>
          <div class="amp-cat">
            <div class="amp-cat-icon" :style="{ background: cat.color }">
              <el-icon :size="18"><component :is="cat.icon" /></el-icon>
            </div>
            <span class="amp-cat-title">{{ cat.title }}</span>
            <span class="amp-cat-count">{{ cat.items.length }} 项</span>
          </div>
        </template>

        <div class="admin-menu-grid">
          <div
            v-for="m in cat.items"
            :key="m.path"
            :id="domId(m.path)"
            class="admin-menu-card"
            :class="{ 'amp-highlight': highlighted === m.path }"
            @click="$router.push(m.path)"
          >
            <div class="amc-icon" :style="{ background: m.color }">
              <el-icon :size="22"><component :is="m.icon" /></el-icon>
            </div>
            <div class="amc-body">
              <div class="amc-title">{{ m.title }}</div>
              <div class="amc-desc">{{ m.desc }}</div>
            </div>
            <el-icon class="amc-arrow"><ArrowRight /></el-icon>
          </div>
        </div>
      </el-collapse-item>
    </el-collapse>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { ArrowRight } from '@element-plus/icons-vue';
import { adminMenuCategories } from '@/views/admin/adminMenuData';

const route = useRoute();
const categories = adminMenuCategories;

const LS_KEY = 'nvs_admin_menu_open';
const allKeys = categories.map((c) => c.key);

// 展开的分类（默认全部展开；用户的折叠选择持久化到 localStorage）
const openCats = ref<string[]>([...allKeys]);
function loadOpen() {
  try {
    const raw = localStorage.getItem(LS_KEY);
    if (raw) {
      const arr = JSON.parse(raw);
      if (Array.isArray(arr)) {
        openCats.value = allKeys.filter((k) => arr.includes(k));
        return;
      }
    }
  } catch { /* 忽略，回退默认全部展开 */ }
  openCats.value = [...allKeys];
}
watch(
  openCats,
  (v) => {
    try { localStorage.setItem(LS_KEY, JSON.stringify(v)); } catch { /* 忽略 */ }
  },
  { deep: true },
);

// 深链高亮的目标 path（?item=），短暂高亮后自动取消
const highlighted = ref('');

// 将 path 转成合法 DOM id（/admin/log-redact → amp-admin-log-redact）
function domId(path: string) {
  return 'amp-' + path.replace(/[^a-zA-Z0-9]+/g, '-').replace(/^-+|-+$/g, '');
}

function locateItem(path: string) {
  // 找到所属分类并确保展开
  const cat = categories.find((c) => c.items.some((i) => i.path === path));
  if (cat && !openCats.value.includes(cat.key)) {
    openCats.value = [...openCats.value, cat.key];
  }
  highlighted.value = path;
  nextTick(() => {
    const el = document.getElementById(domId(path));
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
    // 3 秒后取消高亮
    window.setTimeout(() => {
      if (highlighted.value === path) highlighted.value = '';
    }, 3000);
  });
}

// 深链：/admin?category=<key>&item=<path>
function applyDeepLink() {
  const cat = route.query.category as string | undefined;
  const item = route.query.item as string | undefined;
  if (cat && allKeys.includes(cat) && !openCats.value.includes(cat)) {
    openCats.value = [...openCats.value, cat];
  }
  if (item) locateItem(item);
}

watch(
  () => [route.query.category, route.query.item],
  () => applyDeepLink(),
);

onMounted(() => {
  loadOpen();
  applyDeepLink();
});
</script>

<style scoped>
.menu-section { margin-top: 28px; }
.menu-section-title { font-size: 1.15rem; margin: 0 0 14px; color: var(--text-color, #333); }

.amp-collapse { border: none; background: transparent; }
.amp-collapse :deep(.el-collapse-item) {
  margin-bottom: 12px;
  border: 1px solid var(--border-color, #e8eaf0);
  border-radius: 12px;
  overflow: hidden;
  background: var(--bg-card, #fff);
}
.amp-collapse :deep(.el-collapse-item__header) {
  padding: 0 16px;
  height: auto;
  line-height: 1;
  background: transparent;
  border-bottom: none;
}
.amp-collapse :deep(.el-collapse-item__wrap) { background: transparent; }
.amp-collapse :deep(.el-collapse-item__content) { padding: 4px 16px 16px; }

.amp-cat { display: flex; align-items: center; gap: 10px; padding: 14px 0; width: 100%; }
.amp-cat-icon {
  width: 34px; height: 34px; border-radius: 9px;
  display: flex; align-items: center; justify-content: center; color: #fff; flex-shrink: 0;
}
.amp-cat-title { font-weight: 700; font-size: 1rem; color: var(--text-color, #333); }
.amp-cat-count { margin-left: auto; font-size: 0.78rem; color: var(--text-light, #999); }

.admin-menu-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 12px;
}
.admin-menu-card {
  display: flex; align-items: center; gap: 12px;
  padding: 14px; border-radius: 12px; cursor: pointer;
  background: var(--bg-subtle, #f8fafc);
  border: 1px solid transparent;
  transition: transform .15s, box-shadow .15s, border-color .15s;
}
.admin-menu-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 18px rgba(0,0,0,.08);
  border-color: var(--primary-color, #4f46e5);
}
.amc-icon {
  width: 42px; height: 42px; border-radius: 10px; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center; color: #fff;
}
.amc-body { flex: 1; min-width: 0; }
.amc-title { font-weight: 600; font-size: .95rem; color: var(--text-color, #333); }
.amc-desc { font-size: .78rem; color: var(--text-light, #888); margin-top: 2px; }
.amc-arrow { color: var(--text-light, #bbb); flex-shrink: 0; }

/* 深链定位高亮：脉冲边框，短暂吸引注意 */
.amp-highlight {
  border-color: var(--primary-color, #4f46e5) !important;
  animation: amp-pulse 1.2s ease-in-out 2;
}
@keyframes amp-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(79,70,229,.4); }
  50% { box-shadow: 0 0 0 6px rgba(79,70,229,0); }
}

[data-theme="dark"] .menu-section-title { color: #e2e8f0; }
[data-theme="dark"] .amp-cat-title { color: #e2e8f0; }
[data-theme="dark"] .admin-menu-card { background: rgba(255,255,255,.03); }
[data-theme="dark"] .amc-title { color: #e2e8f0; }
</style>
