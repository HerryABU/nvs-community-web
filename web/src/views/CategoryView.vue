<template>
  <div class="page-container category-page">
    <div class="category-header">
      <h1>{{ categoryName }}</h1>
      <span class="novel-count">共 {{ total }} 部作品</span>
    </div>

    <!-- 视图切换 -->
    <div class="category-toolbar">
      <el-radio-group v-model="sortBy" size="small" @change="onFilterChange">
        <el-radio-button value="featured">本周最佳</el-radio-button>
        <el-radio-button value="newest">新品上架</el-radio-button>
        <el-radio-button value="updated">最近更新</el-radio-button>
      </el-radio-group>
      <el-radio-group v-model="viewMode" size="small">
        <el-radio-button value="grid"><el-icon><Grid /></el-icon> 平铺</el-radio-button>
        <el-radio-button value="table"><el-icon><List /></el-icon> 表式</el-radio-button>
      </el-radio-group>
    </div>

    <!-- 网格视图 -->
    <div v-if="viewMode === 'grid'" class="card-grid" v-loading="loading">
      <NovelCard
        v-for="novel in novels"
        :key="novel.id"
        :novel="novel"
        @click="$router.push(`/novel/${novel.id}`)"
      />
      <el-empty v-if="!loading && novels.length === 0" description="暂无作品" />
    </div>

    <!-- 表式视图 -->
    <div v-else class="table-view" v-loading="loading">
      <el-table :data="novels" stripe @row-click="(row: any) => $router.push(`/novel/${row.id}`)" style="cursor:pointer">
        <el-table-column label="封面" width="70">
          <template #default="{ row }">
            <el-image v-if="row.cover_url" :src="row.cover_url" fit="cover" style="width:40px;height:54px;border-radius:4px" />
            <div v-else style="width:40px;height:54px;border-radius:4px;background:#e0e0e0;display:flex;align-items:center;justify-content:center;font-size:12px;color:#999">封</div>
          </template>
        </el-table-column>
        <el-table-column label="作品名称" min-width="200">
          <template #default="{ row }">
            <div>
              <div style="font-weight:500">{{ row.title }}</div>
              <div style="font-size:0.8rem;color:#999;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:300px">{{ row.summary }}</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="作者" width="100">
          <template #default="{ row }">
            <router-link :to="`/author/${row.author_id}`" @click.stop>{{ row.author_name || row.author?.nickname || '未知' }}</router-link>
          </template>
        </el-table-column>
        <el-table-column prop="total_words" label="字数" width="90" sortable />
        <el-table-column prop="total_chapters" label="章节" width="70" />
        <el-table-column label="更新" width="110">
          <template #default="{ row }">{{ formatDate(row.updated_at) }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && novels.length === 0" description="暂无作品" />
    </div>

    <!-- 分页 -->
    <div class="pagination" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="fetchNovels"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { novelApi, type Novel } from '@/api/novel';
import NovelCard from '@/components/NovelCard.vue';
import { Grid, List } from '@element-plus/icons-vue';

const route = useRoute();
const categoryName = ref(route.params.name as string);
const novels = ref<Novel[]>([]);
const loading = ref(false);
const currentPage = ref(1);
const pageSize = 12;
const total = ref(0);
const viewMode = ref<'grid' | 'table'>('grid');
const sortBy = ref<'featured' | 'newest' | 'updated'>('featured');

function formatDate(dateStr?: string) {
  if (!dateStr) return '';
  return new Date(dateStr).toLocaleDateString('zh-CN');
}

async function fetchNovels() {
  loading.value = true;
  try {
    const params: any = {
      page: currentPage.value,
      page_size: pageSize,
      category: categoryName.value,
    };
    if (sortBy.value === 'newest') params.sort_by = 'created_at';
    else if (sortBy.value === 'updated') params.sort_by = 'updated_at';
    else params.sort_by = 'featured';
    const res = await novelApi.getNovels(params);
    novels.value = res.data.data.list || [];
    total.value = res.data.data.total || 0;
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

function onFilterChange() {
  currentPage.value = 1;
  fetchNovels();
}

// 监听路由参数变化（切换分类时）
watch(
  () => route.params.name,
  (newName) => {
    categoryName.value = newName as string;
    currentPage.value = 1;
    viewMode.value = 'grid';
    fetchNovels();
  }
);

onMounted(() => fetchNovels());
</script>

<style scoped>
.category-header {
  display: flex;
  align-items: baseline;
  gap: 16px;
  margin-bottom: 20px;
}

.category-header h1 {
  font-size: 1.6rem;
  color: var(--primary-color);
}

.novel-count {
  color: var(--text-light);
  font-size: 0.9rem;
}

.category-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  flex-wrap: wrap;
  gap: 12px;
}

.table-view {
  margin-bottom: 24px;
}

.pagination {
  display: flex;
  justify-content: center;
  margin-top: 32px;
}

.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 20px;
}
</style>

<style>
[data-theme="dark"] .category-page {
  background: transparent;
}
[data-theme="dark"] .category-header h1 {
  color: #e2e8f0;
}
[data-theme="dark"] .category-toolbar {
  background: transparent;
}
[data-theme="dark"] .table-view .el-table__body tr:hover > td {
  background-color: #334155 !important;
}
[data-theme="dark"] .table-view [style*="background:#e0e0e0"] {
  background: #334155 !important;
  color: #94a3b8 !important;
}
</style>
