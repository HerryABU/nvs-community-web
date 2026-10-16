<template>
  <div class="home-page">
    <!-- ====== 搜索结果区域 ====== -->
    <HomeSearchResults
      v-if="searchKeyword"
      :keyword="searchKeyword" :results="searchResults" :authors="searchAuthors"
      :author-total="searchAuthorTotal" :total="searchTotal" :loading="searchLoading"
      :blogs="searchBlogs" :htmls="searchHtmls"
      :categories="searchCategories" :topics="searchTopics" :forums="searchForums"
      :view="novelView" :page-size="searchPageSize"
      v-model:current-page="searchPage"
      @clear="clearSearch"
      @page-change="doSearch"
    />

    <div class="page-container">
      <div class="home-top-bar" v-if="authStore.isLoggedIn">
        <el-button type="primary" size="large" @click="$router.push('/author/editor')">
          <el-icon><Plus /></el-icon> 发布作品
        </el-button>
      </div>

      <!-- ====== 自定义主页区块（F15：管理员可编辑/排序/套用布局预设） ====== -->
      <HomeBlocks />
    </div>

    <div class="view-toggle-float">
      <el-tooltip :content="novelView==='grid'?'全部切换为表式':'全部切换为平铺'" placement="left">
        <el-button type="primary" circle size="large" @click="novelView=novelView==='grid'?'table':'grid'">
          <el-icon :size="22"><Grid v-if="novelView==='table'"/><List v-else/></el-icon>
        </el-button>
      </el-tooltip>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { appPrefix } from '@/utils/prefix';
import HomeSearchResults from '@/components/HomeSearchResults.vue';
import HomeBlocks from '@/components/home/HomeBlocks.vue';
import { Grid, List, Plus } from '@element-plus/icons-vue';

const authStore = useAuthStore();
const route = useRoute();
const router = useRouter();
const novelView = ref<'grid'|'table'>('grid');

// ====== 搜索模式 ======
const searchKeyword = ref('');
const searchResults = ref<any[]>([]);
const searchAuthors = ref<any[]>([]);
const searchAuthorTotal = ref(0);
const searchBlogs = ref<any[]>([]);
const searchHtmls = ref<any[]>([]);
const searchCategories = ref<any[]>([]);
const searchTopics = ref<any[]>([]);
const searchForums = ref<any[]>([]);
const searchLoading = ref(false);
const searchTotal = ref(0);
const searchPage = ref(1);
const searchPageSize = 12;

async function doSearch() {
  if (!searchKeyword.value) return;
  searchLoading.value = true;
  try {
    const res = await fetch(`${appPrefix}/api/search?q=${encodeURIComponent(searchKeyword.value)}`).then(r => r.json());
    if (res.code === 0) {
      const d = res.data;
      searchResults.value = d.novels || [];
      searchAuthors.value = d.authors || [];
      searchBlogs.value = d.blogs || [];
      searchHtmls.value = d.htmls || [];
      searchCategories.value = d.categories || [];
      searchTopics.value = d.topics || [];
      searchForums.value = d.forums || [];
      searchTotal.value = (d.novels || []).length;
      searchAuthorTotal.value = (d.authors || []).length;
    }
  } catch (e) {
    console.error(e);
  } finally {
    searchLoading.value = false;
  }
}

function clearSearch() {
  searchKeyword.value = '';
  searchResults.value = [];
  searchTotal.value = 0;
  searchPage.value = 1;
  searchBlogs.value = [];
  searchHtmls.value = [];
  router.replace({ query: {} });
}

watch(
  () => route.query.search,
  (newSearch) => {
    if (newSearch && typeof newSearch === 'string' && newSearch.trim()) {
      searchKeyword.value = newSearch.trim();
      searchPage.value = 1;
      doSearch();
    } else if (!newSearch) {
      searchKeyword.value = '';
      searchResults.value = [];
      searchTotal.value = 0;
    }
  },
  { immediate: true }
);
</script>

<style scoped>
/* ====== Page Layout ====== */
.page-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 24px 20px 60px;
}
.home-top-bar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 16px;
  gap: 10px;
}

/* Float Toggle */
.view-toggle-float {
  position: fixed;
  bottom: 36px;
  right: 36px;
  z-index: 50;
}
.view-toggle-float .el-button {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-full) !important;
  box-shadow: var(--shadow-lg);
  transition: transform var(--transition-fast), box-shadow var(--transition-fast);
}
.view-toggle-float .el-button:hover {
  transform: scale(1.12);
  box-shadow: var(--shadow-xl);
}

/* ====== 移动端首页适配 ====== */
@media (max-width: 768px) {
  .view-toggle-float { bottom: calc(16px + env(safe-area-inset-bottom)); right: 16px; }
  .view-toggle-float .el-button { width: 48px; height: 48px; }
}
</style>
