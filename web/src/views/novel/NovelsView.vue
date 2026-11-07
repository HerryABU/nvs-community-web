<template>
  <div class="page-container novels-view">
    <!-- 页头：与首页「类型文学」区块同款标题，保证小说页与首页小说块视觉一致 -->
    <div class="nv-header">
      <div class="nv-title">
        <span class="nv-icon">📚</span>
        <div>
          <h1>类型文学</h1>
          <p class="nv-subtitle">探索全部作品 —— 与首页小说区块同一套卡片与栅格</p>
        </div>
      </div>

      <!-- 搜索：与导航栏搜索语义一致，回车或点按钮即筛选 -->
      <div class="nv-search">
        <el-input
          v-model="search"
          placeholder="搜索书名 / 简介 / 标签 / 作者…"
          clearable
          size="large"
          @keyup.enter="applySearch"
          @clear="applySearch"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
          <template #append>
            <el-button :icon="Search" @click="applySearch">搜索</el-button>
          </template>
        </el-input>
      </div>
    </div>

    <!-- 筛选行：分类 chips + 排序 -->
    <div class="nv-filters">
      <div class="nv-cats">
        <button
          class="nv-cat"
          :class="{ active: category === '' }"
          @click="setCategory('')"
        >全部</button>
        <button
          v-for="cat in categories"
          :key="cat"
          class="nv-cat"
          :class="{ active: category === cat }"
          @click="setCategory(cat)"
        >{{ cat }}</button>
      </div>

      <el-select v-model="sortBy" size="large" class="nv-sort" @change="applyFilters">
        <el-option label="推荐排序" value="featured" />
        <el-option label="最新发布" value="created_at" />
        <el-option label="最近更新" value="updated_at" />
      </el-select>
    </div>

    <!-- 联邦嵌入区块（站长开启「嵌入列表区块」时显示跨站作品） -->
    <FederatedBlock title="来自联邦站点的作品" :limit="8" />

    <!-- 结果区 -->
    <div v-loading="loading" class="nv-body">
      <!-- 骨架：与首页同款卡片占位 -->
      <div v-if="loading" class="nv-grid">
        <div v-for="i in pageSize" :key="i" class="nv-skeleton-card" />
      </div>

      <!-- 卡片栅格：复用 NovelCard，栅格间距与首页 HomeModuleSection.hms-grid 完全一致 -->
      <div v-else-if="novels.length" class="nv-grid">
        <NovelCard
          v-for="n in novels"
          :key="n.id"
          :novel="n"
          @click="router.push(`/novel/${n.id}`)"
        />
      </div>

      <!-- 空态 -->
      <div v-else class="nv-empty">
        <span class="nv-empty-icon">📭</span>
        <div class="nv-empty-text">
          <b>没有找到匹配的作品</b>
          <span>换个关键词或分类试试，或者去发布你的第一篇。</span>
        </div>
        <el-button type="primary" plain @click="router.push('/author/editor')">去发布作品</el-button>
      </div>
    </div>

    <!-- 分页 -->
    <div v-if="totalPages > 1" class="nv-pager">
      <el-pagination
        layout="prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="onPage"
        background
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { storeToRefs } from 'pinia';
import { Search } from '@element-plus/icons-vue';
import { novelApi, type Novel } from '@/api/novel';
import { useAppStore } from '@/stores/app';
import NovelCard from '@/components/novel/NovelCard.vue';
import FederatedBlock from '@/components/federation/FederatedBlock.vue';

const router = useRouter();
const route = useRoute();
const appStore = useAppStore();
const { categories } = storeToRefs(appStore);

const novels = ref<Novel[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(24);
const loading = ref(false);
const search = ref('');
const category = ref('');
const sortBy = ref<'featured' | 'created_at' | 'updated_at'>('featured');

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)));

async function load() {
  loading.value = true;
  try {
    const res = await novelApi.getNovels({
      page: page.value,
      page_size: pageSize.value,
      category: category.value || undefined,
      search: search.value || undefined,
      sort_by: sortBy.value,
    });
    novels.value = res.data?.data?.list || [];
    total.value = res.data?.data?.total || 0;
  } catch {
    novels.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
  }
}

/** 从路由 query 同步筛选状态（支持从标签链接 /novels?search=xxx 直达） */
function syncFromQuery() {
  const q = route.query;
  search.value = typeof q.search === 'string' ? q.search : '';
  category.value = typeof q.category === 'string' ? q.category : '';
  if (q.sort === 'created_at' || q.sort === 'updated_at') sortBy.value = q.sort;
  else sortBy.value = 'featured';
  page.value = 1;
  load();
}

/** 把当前筛选写回 URL（便于分享 / 浏览器前进后退） */
function pushQuery() {
  const query: Record<string, string> = {};
  if (search.value) query.search = search.value;
  if (category.value) query.category = category.value;
  if (sortBy.value !== 'featured') query.sort = sortBy.value;
  router.replace({ path: '/novels', query });
}

function applySearch() {
  page.value = 1;
  pushQuery(); // watch(fullPath) 会触发 syncFromQuery → load
}

function applyFilters() {
  page.value = 1;
  pushQuery();
}

function setCategory(cat: string) {
  category.value = cat;
  applyFilters();
}

function onPage(p: number) {
  page.value = p;
  load();
  window.scrollTo({ top: 0, behavior: 'smooth' });
}

onMounted(() => {
  appStore.bootstrap(); // 拉取分类等全局配置（幂等）
  syncFromQuery();
});

// 同路由仅 query 变化（如从标签链接跳入、浏览器前进后退）时重新拉取
watch(() => route.fullPath, syncFromQuery);
</script>

<style scoped>
.novels-view {
  max-width: 1280px;
}

/* 页头 */
.nv-header {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}
.nv-title {
  display: flex;
  align-items: center;
  gap: 14px;
}
.nv-icon { font-size: 2rem; line-height: 1; }
.nv-title h1 {
  font-size: 1.6rem;
  font-weight: 300;
  color: var(--text-color);
  margin: 0 0 4px;
  letter-spacing: -0.02em;
}
.nv-subtitle {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-light);
}
.nv-search {
  flex: 1 1 320px;
  max-width: 480px;
}

/* 筛选行 */
.nv-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 22px;
}
.nv-cats {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.nv-cat {
  border: 1px solid var(--border-color);
  background: var(--bg-card);
  color: var(--text-color);
  font-size: 0.85rem;
  padding: 7px 16px;
  border-radius: 999px;
  cursor: pointer;
  transition: all 0.18s ease;
  white-space: nowrap;
}
.nv-cat:hover {
  border-color: var(--primary-color);
  color: var(--primary-color);
}
.nv-cat.active {
  background: var(--primary-color);
  border-color: var(--primary-color);
  color: #fff;
}
.nv-sort { width: 150px; }

/* 卡片栅格：与首页 HomeModuleSection .hms-grid 同款 */
.nv-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 18px;
  min-height: 200px;
}
.nv-skeleton-card {
  height: 300px;
  border-radius: 16px;
  background: linear-gradient(90deg, var(--bg-page) 25%, var(--bg-soft) 37%, var(--bg-page) 63%);
  background-size: 400% 100%;
  animation: nv-shimmer 1.4s ease infinite;
}
@keyframes nv-shimmer {
  0% { background-position: 100% 50%; }
  100% { background-position: 0 50%; }
}
@media (prefers-reduced-motion: reduce) {
  .nv-skeleton-card { animation: none; }
}

/* 空态 */
.nv-empty {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 24px 18px;
  border: 1px dashed var(--border-color);
  border-radius: var(--radius-md);
  background: var(--bg-page);
  flex-wrap: wrap;
}
.nv-empty-icon { font-size: 26px; opacity: 0.7; }
.nv-empty-text { display: flex; flex-direction: column; gap: 2px; flex: 1; min-width: 200px; }
.nv-empty-text b { font-size: var(--text-sm); color: var(--text-color); }
.nv-empty-text span { font-size: var(--text-xs); color: var(--text-light); }

/* 分页 */
.nv-pager {
  display: flex;
  justify-content: center;
  margin-top: 30px;
}
</style>
