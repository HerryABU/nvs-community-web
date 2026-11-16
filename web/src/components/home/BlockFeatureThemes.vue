<template>
  <section v-if="list.length" class="ft-wrap">
    <div class="ft-head">
      <h2>{{ title || '精选博客主题' }}</h2>
      <span class="ft-sub">作者自选的高级博客 · 点击进入专属主页</span>
    </div>
    <div class="ft-grid">
      <router-link v-for="t in list" :key="t.user_id" :to="`/blog-theme/${t.username}`" class="ft-card">
        <div class="ft-covers">
          <template v-if="t.recent?.length">
            <img v-for="p in t.recent.slice(0, 3)" :key="p.id" :src="media(p.cover_url)" :alt="p.title" loading="lazy" />
          </template>
          <div v-else class="ft-cover-empty">✍️</div>
        </div>
        <div class="ft-body">
          <el-avatar :size="36" :src="media(t.avatar_url)">{{ (t.nickname || '?')[0] }}</el-avatar>
          <div class="ft-info">
            <b>{{ t.nickname || t.username }}</b>
            <span class="ft-sign">{{ t.signature || '这位作者的自定义博客' }}</span>
          </div>
          <span class="ft-count">{{ t.post_count }} 篇</span>
        </div>
      </router-link>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { themeBlogApi, type FeatureTheme } from '@/api/themeBlog';
import { getAppPrefix } from '@/utils/prefix';

defineProps<{ title?: string; config?: Record<string, unknown> }>();
const prefix = getAppPrefix() || '';
function media(u?: string) {
  if (!u) return '';
  if (/^https?:\/\//i.test(u)) return u;
  return prefix + (u.startsWith('/') ? u : '/' + u);
}

const list = ref<FeatureTheme[]>([]);
onMounted(async () => {
  try {
    const r = await themeBlogApi.featureThemes();
    list.value = r.data?.data?.list || [];
  } catch { list.value = []; }
});
</script>

<style scoped>
.ft-wrap { max-width: 1240px; margin: 22px auto; padding: 0 20px; }
.ft-head { display: flex; align-items: baseline; gap: 12px; margin-bottom: 12px; }
.ft-head h2 { margin: 0; font-size: 1.15rem; }
.ft-sub { font-size: .78rem; color: var(--text-muted, #888); }
.ft-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 14px; }
.ft-card { background: var(--bg-card, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 10px; overflow: hidden; text-decoration: none; color: inherit; transition: transform .15s, box-shadow .15s; }
.ft-card:hover { transform: translateY(-2px); box-shadow: 0 6px 18px rgba(0,0,0,.08); }
.ft-covers { height: 96px; display: grid; grid-template-columns: repeat(3, 1fr); gap: 2px; background: var(--fill-color, #f0f2f5); }
.ft-covers img { width: 100%; height: 96px; object-fit: cover; }
.ft-cover-empty { grid-column: 1/-1; display: flex; align-items: center; justify-content: center; font-size: 2rem; }
.ft-body { display: flex; align-items: center; gap: 10px; padding: 12px; }
.ft-info { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.ft-sign { font-size: .75rem; color: var(--text-muted, #888); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ft-count { font-size: .75rem; color: var(--text-muted, #888); }
</style>
