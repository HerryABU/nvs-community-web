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
