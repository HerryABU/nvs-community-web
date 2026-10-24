<template>
  <!-- ====== 最新博客区块（F15：博客首页风格的主角区块） ====== -->
  <section class="home-block block-blogs">
    <header class="bb-header">
      <h2>{{ title || '最新博客' }}</h2>
      <el-button text type="primary" @click="$router.push('/blogs')">
        更多 <el-icon><ArrowRight /></el-icon>
      </el-button>
    </header>
    <div v-loading="loading" :class="['bb-body', view === 'grid' ? 'bb-grid' : 'bb-list']">
      <template v-if="!loading && items.length">
        <article v-for="b in items" :key="b.id" class="bb-card" @click="$router.push(`/blog/${b.id}`)">
          <div class="bb-cover" v-if="b.cover_url">
            <img :src="b.cover_url" :alt="b.title" loading="lazy" />
          </div>
          <div class="bb-main">
            <h3 class="bb-title">{{ b.title }}</h3>
            <p class="bb-summary">{{ b.summary || stripHtml(b.content).slice(0, 90) }}</p>
            <div class="bb-meta">
              <span>{{ b.author_name || b.nickname || b.username || '匿名' }}</span>
              <span>{{ formatDate(b.created_at) }}</span>
            </div>
          </div>
        </article>
      </template>
      <p v-else-if="!loading" class="home-block-empty">还没有博客。发布后会显示在这里。</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ArrowRight } from '@element-plus/icons-vue';
import api from '@/api/index';

const props = defineProps<{
  title?: string;
  /** {limit: 8, view: "list"|"grid"} */
  config: string;
}>();

const items = ref<any[]>([]);
const loading = ref(true);

const parsed = JSON.parse(props.config || '{}');
const limit = Math.min(Math.max(Number(parsed.limit) || 6, 1), 20);
const view = parsed.view === 'grid' ? 'grid' : 'list';

function formatDate(d?: string) { return d ? new Date(d).toLocaleDateString('zh-CN') : ''; }
function stripHtml(s?: string) { return (s || '').replace(/<[^>]*>/g, '').trim(); }

onMounted(async () => {
  try {
    const res = await api.get('/blogs', { params: { page: 1, page_size: limit } });
    items.value = res.data?.data?.list || [];
  } catch (e) {
    console.error('[BlockBlogs] load failed', e);
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.home-block {
  background: var(--bg-card);
  border-radius: var(--radius-lg);
  padding: 22px 26px;
  margin-bottom: 26px;
  box-shadow: var(--shadow-card);
}

.bb-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.bb-header h2 { margin: 0; font-size: 1.15rem; font-weight: 600; }

.bb-list { display: flex; flex-direction: column; gap: 12px; }
.bb-card {
  display: flex;
  gap: 14px;
  padding: 14px;
  border-radius: var(--radius-md);
  border: 1px solid var(--el-border-color-lighter);
  cursor: pointer;
  transition: border-color .2s, transform .2s;
}
.bb-card:hover { border-color: var(--el-color-primary); transform: translateY(-1px); }
.bb-cover { flex-shrink: 0; width: 120px; height: 80px; border-radius: 8px; overflow: hidden; }
.bb-cover img { width: 100%; height: 100%; object-fit: cover; }
.bb-main { flex: 1; min-width: 0; }
.bb-title { margin: 0 0 6px; font-size: 1rem; }
.bb-summary {
  margin: 0 0 8px;
  font-size: .85rem;
  color: var(--text-light);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.bb-meta { display: flex; gap: 14px; font-size: .78rem; color: var(--text-light); }

.bb-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 14px; }
.bb-grid .bb-card { flex-direction: column; }
.bb-grid .bb-cover { width: 100%; height: 130px; }
</style>
