<template>
  <!-- 联邦条目按类型嵌入各自列表：小说放小说、论坛放论坛、博客放博客、微文放微文。
       点击走本站反代路径 /fed/<siteID>/...（proxycore 转发到源站并改写资源），
       不再直接跳外链。站长关闭嵌入模式时整块不渲染。 -->
  <section v-if="visible && items.length" class="fed-embed" :class="'fed-kind-' + kind">
    <div class="fed-head">
      <div class="fed-title">
        <span class="fed-icon">🌐</span>
        <span class="fed-label">{{ kindLabel }} · 来自互联站点</span>
      </div>
      <router-link to="/federation" class="fed-more">联邦广场 →</router-link>
    </div>
    <div class="fed-list">
      <a
        v-for="item in items"
        :key="item.site_id + '-' + item.remote_id + '-' + item.kind"
        class="fed-item"
        :href="item.proxy_path || item.source_url"
        :target="item.proxy_path ? '_self' : '_blank'"
        rel="noopener noreferrer"
      >
        <img v-if="item.cover_url" class="fed-cover" :src="item.cover_url" :alt="item.title" loading="lazy" />
        <span v-else class="fed-cover-ph">📄</span>
        <div class="fed-body">
          <div class="fed-item-title">{{ item.title }}</div>
          <div class="fed-meta">
            <span>{{ item.author || '佚名' }}</span>
            <span v-if="item.origin_site" class="fed-origin">{{ item.origin_site }}</span>
            <span v-if="item.hop_count > 1" class="fed-hop">{{ item.hop_count }} 跳</span>
          </div>
          <div v-if="item.summary" class="fed-summary">{{ item.summary }}</div>
        </div>
      </a>
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
    kind?: 'novel' | 'blog' | 'forum' | 'micro' | 'subculture';
    limit?: number;
  }>(),
  { kind: 'novel', limit: 6 },
);

const kindLabels: Record<string, string> = {
  novel: '小说', blog: '博客', forum: '论坛', micro: '微文', subculture: '亚文化',
};
const kindLabel = computed(() => kindLabels[props.kind] || '作品');

const appStore = useAppStore();
const { federationModes } = storeToRefs(appStore);
const items = ref<any[]>([]);
const visible = computed(() => !!federationModes.value.embed_enabled);

async function load() {
  if (!visible.value) return;
  try {
    const res = await publicApi.getFederatedItems({ page: 1, page_size: props.limit, kind: props.kind });
    items.value = (res.data?.data?.list || []).map((x: any) => ({
      ...x,
      site_id: x.site_id ?? x.site?.id ?? 0,
    }));
  } catch {
    items.value = [];
  }
}

watch(visible, (v) => { if (v) load(); }, { immediate: true });
watch(() => props.kind, () => load());
</script>

<style scoped>
.fed-embed { margin: 10px 0 22px; }
.fed-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 10px; }
.fed-title { display: flex; align-items: center; gap: 8px; font-size: 0.9rem; font-weight: 600; color: var(--text-color, #1a1a2e); }
.fed-icon { font-size: 1.1rem; }
.fed-more { font-size: 0.78rem; color: var(--primary-color, #6366f1); text-decoration: none; }
.fed-more:hover { text-decoration: underline; }
.fed-list { display: flex; flex-direction: column; gap: 8px; }
.fed-item {
  display: flex; align-items: center; gap: 12px; padding: 10px 12px;
  border: 1px solid var(--border-color, #e5e7eb); border-radius: 12px;
  background: var(--bg-card, #fff); text-decoration: none; color: inherit;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}
.fed-item:hover { transform: translateY(-2px); box-shadow: 0 6px 18px rgba(0,0,0,0.08); }
.fed-cover { width: 56px; height: 56px; object-fit: cover; border-radius: 8px; flex-shrink: 0; }
.fed-cover-ph { width: 56px; height: 56px; display: grid; place-items: center; background: var(--bg-soft, #f1f5f9); border-radius: 8px; font-size: 1.4rem; flex-shrink: 0; }
.fed-body { flex: 1; min-width: 0; }
.fed-item-title { font-size: 0.9rem; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fed-meta { display: flex; gap: 8px; font-size: 0.74rem; color: var(--text-light, #888); margin-top: 2px; }
.fed-origin { color: var(--primary-color, #6366f1); }
.fed-hop { color: #d97706; }
.fed-summary { font-size: 0.76rem; color: var(--text-light, #999); margin-top: 3px; display: -webkit-box; -webkit-line-clamp: 1; -webkit-box-orient: vertical; overflow: hidden; }
[data-theme="dark"] .fed-embed { background: transparent; }
[data-theme="dark"] .fed-item { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .fed-title { color: #e2e8f0; }
</style>
