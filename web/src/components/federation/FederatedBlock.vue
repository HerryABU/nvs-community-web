<template>
  <!-- 联邦嵌入区块（嵌入模式）：仅在站长开启「嵌入列表区块」且确有跨站作品时渲染。
       卡片跳转到联邦站点原文（source_url，外链新标签页打开）。 -->
  <section v-if="visible" class="fed-block">
    <div class="fed-head">
      <div class="fed-title">
        <span class="fed-icon">🌐</span>
        <div>
          <h3>{{ title }}</h3>
          <p class="fed-sub">来自其它互联 NVS 站点的作品 · 点击前往源站阅读</p>
        </div>
      </div>
      <router-link to="/federation" class="fed-more">查看全部友站 →</router-link>
    </div>

    <!-- 按类别分板块展示：不同类别出现在各自分区，而非一股脑混排 -->
    <div v-loading="loading" class="fed-groups">
      <section v-for="g in groups" :key="g.name" class="fed-group">
        <h4 class="fed-group-title">
          <span class="fed-group-name">{{ g.name }}</span>
          <span class="fed-group-count">{{ g.items.length }} 部</span>
        </h4>
        <div class="fed-grid">
          <a
            v-for="item in g.items"
            :key="item.site_id + '-' + item.remote_id"
            class="fed-card"
            :href="item.source_url"
            target="_blank"
            rel="noopener noreferrer"
          >
            <div class="fed-cover">
              <img v-if="item.cover_url" :src="item.cover_url" :alt="item.title" loading="lazy" />
              <span v-else class="fed-cover-ph">📖</span>
              <span v-if="item.site" class="fed-site-badge">{{ item.site.name }}</span>
            </div>
            <div class="fed-card-body">
              <h4 class="fed-card-title">{{ item.title }}</h4>
              <p class="fed-card-author">{{ item.author || '佚名' }}</p>
              <p v-if="item.summary" class="fed-card-summary">{{ item.summary }}</p>
            </div>
          </a>
        </div>
      </section>
    </div>

    <div v-if="!loading && novels.length === 0" class="fed-empty">
      暂无跨站作品缓存，请站长在「远程站点互通」中同步后展示。
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { storeToRefs } from 'pinia';
import { useAppStore } from '@/stores/app';
import { publicApi } from '@/api/admin';

const props = withDefaults(
  defineProps<{
    title?: string;
    limit?: number;
  }>(),
  { title: '来自联邦站点', limit: 8 },
);

const appStore = useAppStore();
const { federationModes } = storeToRefs(appStore);

const novels = ref<any[]>([]);
const loading = ref(false);

// 按类别分组（同类聚合、保持出现顺序）；无类别归入「其他」
const groups = computed<{ name: string; items: any[] }[]>(() => {
  const map = new Map<string, any[]>();
  for (const it of novels.value) {
    const name = (it.category || '').trim() || '其他';
    if (!map.has(name)) map.set(name, []);
    map.get(name)!.push(it);
  }
  return Array.from(map.entries()).map(([name, items]) => ({ name, items }));
});

// 嵌入模式：仅在站长开启「嵌入列表区块」时渲染（独立页入口另由 independent_enabled 控制）
const visible = computed(() => !!federationModes.value.embed_enabled);

async function load() {
  if (!visible.value) return;
  loading.value = true;
  try {
    const res = await publicApi.getFederatedNovels({ page: 1, page_size: props.limit });
    novels.value = res.data?.data?.list || [];
  } catch {
    novels.value = [];
  } finally {
    loading.value = false;
  }
}

// ⚠️ embed_enabled 由 bootstrap 异步回填：挂载瞬间可能仍是默认 false，直接 load 会被
// 守卫跳过且不再重跑。改为 watch（immediate 覆盖「已就绪」场景）。
watch(visible, (v) => {
  if (v) load();
}, { immediate: true });
</script>

<style scoped>
.fed-block {
  margin: 8px 0 28px;
  padding: 18px 18px 20px;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-lg, 16px);
  background: var(--bg-card, #fff);
}
.fed-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.fed-title { display: flex; align-items: center; gap: 12px; }
.fed-icon { font-size: 1.6rem; line-height: 1; }
.fed-title h3 {
  margin: 0 0 2px;
  font-size: 1.15rem;
  font-weight: 600;
  color: var(--text-color, #1a1a2e);
}
.fed-sub { margin: 0; font-size: 0.78rem; color: var(--text-light, #888); }
.fed-more {
  font-size: 0.82rem;
  color: var(--primary-color, #6366f1);
  text-decoration: none;
  white-space: nowrap;
}
.fed-more:hover { text-decoration: underline; }

.fed-groups {
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-height: 80px;
}
.fed-group-title {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin: 0 0 10px;
  font-size: 0.92rem;
  color: var(--text-color, #1a1a2e);
  border-left: 3px solid var(--primary-color, #6366f1);
  padding-left: 8px;
}
.fed-group-count {
  font-size: 0.72rem;
  font-weight: 400;
  color: var(--text-light, #999);
}
.fed-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
}
.fed-card {
  display: flex;
  flex-direction: column;
  border-radius: var(--radius-md, 12px);
  overflow: hidden;
  background: var(--bg-soft, #f7f8fa);
  border: 1px solid var(--border-color, #eee);
  text-decoration: none;
  color: inherit;
  transition: transform 0.18s ease, box-shadow 0.18s ease;
}
.fed-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}
.fed-cover {
  position: relative;
  aspect-ratio: 3 / 4;
  background: linear-gradient(135deg, #e0e7ff, #f5f3ff);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.fed-cover img { width: 100%; height: 100%; object-fit: cover; }
.fed-cover-ph { font-size: 2.4rem; opacity: 0.5; }
.fed-site-badge {
  position: absolute;
  left: 8px;
  top: 8px;
  font-size: 0.68rem;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(17, 24, 39, 0.78);
  color: #fff;
  backdrop-filter: blur(4px);
}
.fed-card-body { padding: 10px 12px 12px; display: flex; flex-direction: column; gap: 4px; }
.fed-card-title {
  margin: 0;
  font-size: 0.92rem;
  font-weight: 600;
  color: var(--text-color, #1a1a2e);
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.fed-card-author { margin: 0; font-size: 0.76rem; color: var(--text-light, #888); }
.fed-card-summary {
  margin: 0;
  font-size: 0.76rem;
  color: var(--text-light, #999);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.fed-empty {
  padding: 18px;
  text-align: center;
  font-size: 0.85rem;
  color: var(--text-light, #999);
  border: 1px dashed var(--border-color, #e5e7eb);
  border-radius: var(--radius-md, 12px);
}
[data-theme="dark"] .fed-block { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .fed-card { background: #0f172a; border-color: #334155; }
[data-theme="dark"] .fed-card-title { color: #e2e8f0; }
[data-theme="dark"] .fed-cover { background: linear-gradient(135deg, #1e293b, #312e81); }
[data-theme="dark"] .fed-empty { border-color: #334155; }
</style>
