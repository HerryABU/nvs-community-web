<template>
  <div class="page-container">
    <div class="feed-header">
      <h2>特别关注动态</h2>
      <div class="filter-bar">
        <el-radio-group v-model="activeType" size="default">
          <el-radio-button label="all">全部</el-radio-button>
          <el-radio-button label="novel">小说</el-radio-button>
          <el-radio-button label="blog">博客</el-radio-button>
          <el-radio-button label="thread">论坛</el-radio-button>
        </el-radio-group>
      </div>
    </div>

    <div class="feed-list" v-loading="loading">
      <div v-if="!loading && visibleItems.length === 0" class="empty-hint">
        <template v-if="items.length === 0">
          还没有特别关注动态<br />
          去作者主页设置特别关注，这里会汇总他的小说更新、博客和论坛发帖
        </template>
        <template v-else>当前类型下暂无动态</template>
      </div>
      <div
        v-for="item in visibleItems"
        :key="`${item.type}-${item.id}`"
        class="feed-card"
        @click="goDetail(item)"
      >
        <div class="card-top">
          <el-tag size="small" :type="typeTag(item.type)" effect="light" round>{{ typeLabel(item.type) }}</el-tag>
          <span class="card-title">{{ item.title }}</span>
        </div>
        <div class="card-summary" v-if="item.summary">{{ item.summary }}</div>
        <div class="card-meta">
          <span class="author">{{ item.user_name }}</span>
          <span class="time">{{ formatTime(item.created_at) }}</span>
        </div>
      </div>
    </div>

    <div class="pagination" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="loadData"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { specialApi, type FeedItem, type FeedItemType } from '@/api/social/special';

const router = useRouter();

const items = ref<FeedItem[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;
const loading = ref(false);
const activeType = ref<'all' | FeedItemType>('all');

// 类型筛选为当前页内客户端过滤（后端按时间倒序分页）
const visibleItems = computed(() =>
  activeType.value === 'all' ? items.value : items.value.filter((it) => it.type === activeType.value),
);

function typeLabel(type: FeedItemType): string {
  switch (type) {
    case 'novel': return '小说';
    case 'blog': return '博客';
    case 'thread': return '论坛';
    default: return '动态';
  }
}

function typeTag(type: FeedItemType): 'primary' | 'success' | 'warning' | 'info' {
  switch (type) {
    case 'novel': return 'primary';
    case 'blog': return 'success';
    case 'thread': return 'warning';
    default: return 'info';
  }
}

// 时间格式化：YYYY-MM-DD HH:mm
function formatTime(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// 跳转详情（后端已给前端路由路径，如 /author/2/novel/1、/thread/3）
function goDetail(item: FeedItem) {
  router.push(item.url);
}

async function loadData() {
  loading.value = true;
  try {
    const res = await specialApi.getSpecialFeed(page.value, pageSize);
    if (res.data.code === 0) {
      items.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
    }
  } catch { /* ignore */ }
  finally { loading.value = false; }
}

onMounted(() => { loadData(); });
</script>

<style scoped>
.page-container { max-width: 760px; margin: 0 auto; padding: 24px; }
.feed-header { margin-bottom: 16px; }
.feed-header h2 { margin: 0 0 12px 0; }
.filter-bar { display: flex; justify-content: flex-start; }
.feed-list { display: flex; flex-direction: column; gap: 10px; min-height: 120px; }
.feed-card {
  padding: 14px 16px; background: var(--el-bg-color-overlay, #fff); border-radius: 10px;
  border: 1px solid var(--border-color, #eee); cursor: pointer; transition: box-shadow 0.2s;
}
.feed-card:hover { box-shadow: 0 2px 10px rgba(0,0,0,0.08); }
.card-top { display: flex; align-items: center; gap: 8px; }
.card-title {
  font-weight: 600; font-size: 0.95rem;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.card-summary {
  color: var(--text-light, #666); font-size: 0.82rem; margin-top: 6px; line-height: 1.6;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.card-meta {
  display: flex; justify-content: space-between; gap: 12px;
  margin-top: 8px; color: var(--text-light, #999); font-size: 0.75rem;
}
.empty-hint { text-align: center; padding: 48px 16px; color: var(--text-light, #999); line-height: 1.8; }
.pagination { margin-top: 20px; display: flex; justify-content: center; }

/* ====== 移动端适配 ====== */
@media (max-width: 640px) {
  .page-container { padding: 16px; }
  .filter-bar { overflow-x: auto; }
  .pagination .el-pagination {
    flex-wrap: wrap;
    justify-content: center;
  }
}
</style>
