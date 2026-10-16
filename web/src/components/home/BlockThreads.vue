<template>
  <!-- ====== 最新帖区块（F15：论坛首页风格的讨论流；服务端已排除加密/待审论坛） ====== -->
  <section class="home-block block-threads">
    <header class="bt-header">
      <h2>{{ title || '最新讨论' }}</h2>
      <el-button text type="primary" @click="$router.push('/forums')">
        全部版块 <el-icon><ArrowRight /></el-icon>
      </el-button>
    </header>
    <div v-loading="loading" class="bt-list">
      <template v-if="!loading && items.length">
        <article v-for="t in items" :key="t.id" class="bt-item" @click="$router.push(`/thread/${t.id}`)">
          <el-tag v-if="t.is_pinned" type="danger" size="small" effect="plain">置顶</el-tag>
          <span class="bt-title">{{ t.title }}</span>
          <span class="bt-forum">[{{ t.forum?.name || '版块' }}]</span>
          <span class="bt-meta">
            {{ t.user?.nickname || t.user?.username || '匿名' }} · {{ formatDate(t.created_at) }} ·
            {{ t.post_count || 0 }} 回复
          </span>
        </article>
      </template>
      <el-empty v-else-if="!loading" description="暂无讨论" :image-size="60" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ArrowRight } from '@element-plus/icons-vue';
import { homeBlockApi } from '@/api/home';

const props = defineProps<{
  title?: string;
  /** {limit: 10} */
  config: string;
}>();

const items = ref<any[]>([]);
const loading = ref(true);

const parsed = JSON.parse(props.config || '{}');
const limit = Math.min(Math.max(Number(parsed.limit) || 10, 1), 20);

function formatDate(d?: string) { return d ? new Date(d).toLocaleDateString('zh-CN') : ''; }

onMounted(async () => {
  try {
    const res = await homeBlockApi.latestThreads(limit);
    items.value = res.data?.data?.items || [];
  } catch (e) {
    console.error('[BlockThreads] load failed', e);
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.home-block {
  background: #fff;
  border-radius: var(--radius-lg);
  padding: 22px 26px;
  margin-bottom: 26px;
  box-shadow: var(--shadow-card);
}
[data-theme="dark"] .home-block { background: #1a1d27; }

.bt-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.bt-header h2 { margin: 0; font-size: 1.15rem; font-weight: 600; }

.bt-list { display: flex; flex-direction: column; }
.bt-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 11px 4px;
  border-bottom: 1px dashed var(--el-border-color-lighter);
  cursor: pointer;
  font-size: .92rem;
}
.bt-item:last-child { border-bottom: none; }
.bt-item:hover .bt-title { color: var(--el-color-primary); }
.bt-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color .2s;
}
.bt-forum { flex-shrink: 0; font-size: .78rem; color: var(--el-color-primary); }
.bt-meta { flex-shrink: 0; font-size: .76rem; color: var(--text-light); }
</style>
