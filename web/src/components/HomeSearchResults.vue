<template>
  <div class="search-results-section">
    <div class="search-header">
      <h2>
        <el-icon><Search /></el-icon>
        搜索结果：<span class="search-keyword">"{{ keyword }}"</span>
        <span class="search-count">共 {{ total }} 部作品</span>
      </h2>
      <el-button text type="primary" @click="$emit('clear')">✕ 清除搜索</el-button>
    </div>

    <div v-loading="loading">
      <!-- 作者搜索结果 -->
      <div v-if="authors.length > 0" style="margin-bottom:24px">
        <h3 style="margin-bottom:12px;font-size:1rem;color:var(--primary-color)">👤 找到 {{ authorTotal }} 位作者</h3>
        <div class="author-search-row">
          <div v-for="u in authors" :key="u.id" class="author-chip" @click="$router.push(`/author/${u.id}`)">
            <el-avatar :size="40" :src="u.avatar_url">{{ u.nickname?.[0] || u.username?.[0] }}</el-avatar>
            <span class="author-chip-name">{{ u.nickname || u.username }}</span>
            <el-tag v-if="u.role==='vip_author'" size="small" type="warning">VIP</el-tag>
          </div>
        </div>
      </div>

      <!-- 博客搜索结果（标题/关键词） -->
      <div v-if="blogs.length > 0" style="margin-bottom:24px">
        <h3 style="margin-bottom:12px;font-size:1rem;color:var(--primary-color)">✍️ 找到 {{ blogs.length }} 篇博客</h3>
        <div class="misc-search-list">
          <div
            v-for="b in blogs"
            :key="'b' + b.id"
            class="misc-search-item"
            @click="$router.push(`/author/${b.author_id}/blogs/${b.id}`)"
          >
            <span class="misc-title">{{ b.title }}</span>
            <span class="misc-summary">{{ b.summary }}</span>
            <span class="misc-meta">{{ b.author?.nickname || b.author?.username || '作者' }}</span>
          </div>
        </div>
      </div>

      <!-- 专栏搜索结果（博客分类） -->
      <div v-if="categories.length > 0" style="margin-bottom:24px">
        <h3 style="margin-bottom:12px;font-size:1rem;color:var(--primary-color)">📚 找到 {{ categories.length }} 个专栏</h3>
        <div class="misc-search-list">
          <div
            v-for="c in categories"
            :key="'c' + c.id"
            class="misc-search-item"
            @click="$router.push(`/blog/${c.created_by ? c.creator_name || c.created_by : c.id}`)"
          >
            <span class="misc-title">📚 {{ c.full_name || c.name }}</span>
            <span class="misc-summary">{{ c.blog_count }} 篇博客</span>
            <span class="misc-meta">创建者：{{ c.creator_name || '—' }}</span>
          </div>
        </div>
      </div>

      <!-- 话题搜索结果 -->
      <div v-if="topics.length > 0" style="margin-bottom:24px">
        <h3 style="margin-bottom:12px;font-size:1rem;color:var(--primary-color)">🔥 找到 {{ topics.length }} 个话题</h3>
        <div class="misc-search-list">
          <div
            v-for="t in topics"
            :key="'t' + t.id"
            class="misc-search-item"
            @click="$router.push(`/topics/${t.id}`)"
          >
            <span class="misc-title"># {{ t.name }}</span>
            <span class="misc-summary">{{ t.description }}</span>
            <span class="misc-meta">话题</span>
          </div>
        </div>
      </div>

      <!-- 论坛搜索结果 -->
      <div v-if="forums.length > 0" style="margin-bottom:24px">
        <h3 style="margin-bottom:12px;font-size:1rem;color:var(--primary-color)">💬 找到 {{ forums.length }} 个论坛</h3>
        <div class="misc-search-list">
          <div
            v-for="f in forums"
            :key="'f' + f.id"
            class="misc-search-item"
            @click="$router.push(`/forums/${f.id}`)"
          >
            <span class="misc-title">💬 {{ f.name }}</span>
            <span class="misc-summary">{{ f.description }}</span>
            <span class="misc-meta">{{ f.thread_count }} 帖</span>
          </div>
        </div>
      </div>

      <!-- HTML 扩展搜索结果（名称/描述/项目名） -->
      <div v-if="htmls.length > 0" style="margin-bottom:24px">
        <h3 style="margin-bottom:12px;font-size:1rem;color:var(--primary-color)">🔌 找到 {{ htmls.length }} 个扩展应用</h3>
        <div class="misc-search-list">
          <div
            v-for="h in htmls"
            :key="'h' + h.id"
            class="misc-search-item"
            @click="$router.push('/htmls')"
          >
            <span class="misc-title">{{ h.name }}</span>
            <span class="misc-summary">{{ h.description || h.project_name || '' }}</span>
            <span class="misc-meta">{{ h.user?.nickname || h.user?.username || '' }}</span>
          </div>
        </div>
      </div>

      <!-- 平铺视图 -->
      <div v-if="view === 'grid' && results.length > 0" class="card-grid">
        <NovelCard
          v-for="novel in results"
          :key="novel.id"
          :novel="novel"
          @click="$router.push(`/novel/${novel.id}`)"
        />
      </div>

      <!-- 表式视图 -->
      <el-table
        v-else-if="view === 'table' && results.length > 0"
        :data="results" stripe size="small"
        @row-click="(r:any)=>$router.push(`/novel/${r.id}`)" style="cursor:pointer"
      >
        <el-table-column label="封面" width="70">
          <template #default="{ row }">
            <el-image v-if="row.cover_url" :src="row.cover_url" fit="cover"
              style="width:40px;height:54px;border-radius:4px" />
            <div v-else style="width:40px;height:54px;border-radius:4px;background:#e0e0e0;display:flex;align-items:center;justify-content:center;font-size:12px;color:#999">封</div>
          </template>
        </el-table-column>
        <el-table-column label="作品" min-width="180">
          <template #default="{ row }">
            <div>
              <div style="font-weight:500">{{ row.title }}</div>
              <div style="font-size:0.8rem;color:#999;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:300px">{{ row.summary }}</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="作者" width="100">
          <template #default="{ row }">
            <router-link :to="`/author/${row.author_id}`" @click.stop>
              {{ row.author_name || row.author?.nickname || '未知' }}
            </router-link>
          </template>
        </el-table-column>
        <el-table-column label="分类" width="90">
          <template #default="{ row }">
            <span>{{ (row.categories && row.categories.length > 0) ? row.categories[0] : (row.category || '—') }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="total_words" label="字数" width="80" />
        <el-table-column prop="total_chapters" label="章节" width="60" />
        <el-table-column label="更新" width="110">
          <template #default="{ row }">{{ formatDate(row.updated_at) }}</template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && results.length === 0" description="没有找到匹配的作品，试试其他关键词" />
    </div>

    <!-- 搜索分页 -->
    <div class="pagination" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="$emit('page-change', $event)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { Search } from '@element-plus/icons-vue';
import NovelCard from '@/components/NovelCard.vue';

defineProps<{
  keyword: string;
  results: any[];
  authors: any[];
  authorTotal: number;
  total: number;
  loading: boolean;
  blogs: any[];
  htmls: any[];
  categories: any[];
  topics: any[];
  forums: any[];
  view: 'grid' | 'table';
  pageSize: number;
}>();

const currentPage = defineModel<number>('currentPage', { default: 1 });

defineEmits<{
  clear: [];
  'page-change': [page: number];
}>();

function formatDate(d?: string) { return d ? new Date(d).toLocaleDateString('zh-CN') : ''; }
</script>

<style scoped>
.misc-search-list { display: flex; flex-direction: column; gap: 8px; }
.misc-search-item {
  display: flex; align-items: center; gap: 12px; padding: 10px 14px;
  background: var(--card-bg, #fff); border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 10px; cursor: pointer; transition: all 0.2s ease;
}
.misc-search-item:hover { border-color: var(--primary-color, #6366f1); transform: translateY(-1px); }
.misc-title { font-weight: 600; color: var(--text-color, #333); font-size: 0.92rem; flex-shrink: 0; max-width: 40%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.misc-summary { flex: 1; min-width: 0; font-size: 0.82rem; color: var(--text-light, #888); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.misc-meta { font-size: 0.75rem; color: #aaa; flex-shrink: 0; }
[data-theme="dark"] .misc-search-item { background: #1a1f2e; border-color: rgba(255,255,255,0.08); }

/* ====== 移动端搜索结果适配 ====== */
@media (max-width: 768px) {
  /* 表式视图窄屏横向滚动 */
  .search-results-section .el-table {
    overflow-x: auto;
    min-width: 600px;
  }
  .misc-search-item {
    padding: 10px;
    gap: 8px;
  }
  .misc-title {
    max-width: 50%;
  }
  .search-header {
    flex-wrap: wrap;
    gap: 8px;
  }
}
</style>
