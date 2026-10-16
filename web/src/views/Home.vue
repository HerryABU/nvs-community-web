<template>
  <div class="home-page">
    <!-- ====== 极简 Hero ====== -->
    <div class="hero-section">
      <div class="hero-bg"></div>
      <div class="hero-glass">
        <h1>欢迎来到 {{ siteName || '星海文学' }}</h1>
        <p>发现好故事，连接创作者与读者的文学社区</p>
        <div class="hero-quick-links">
          <span v-for="cat in categories.slice(0, 4)" :key="cat"
            class="hero-cat-chip" @click="$router.push(`/category/${encodeURIComponent(cat)}`)">
            {{ cat }}
          </span>
          <span class="hero-cat-chip hero-cat-more" @click="$router.push('/forums')">论坛社区</span>
        </div>
      </div>
    </div>

    <!-- ====== 站点公告栏（多公告，置顶优先；176号令：合规通知/安全公告发布） ====== -->
    <div v-if="announcements.length" class="notice-bar">
      <span class="notice-icon">📢</span>
      <div class="notice-items">
        <span
          v-for="a in announcements.slice(0, 3)"
          :key="a.id"
          class="notice-item"
          @click="openNotice(a)"
        >
          <el-tag v-if="a.pinned" type="danger" size="small" effect="plain" class="notice-pin-tag">置顶</el-tag>
          <span class="notice-item-title">{{ a.title }}</span>
          <span class="notice-item-date">{{ formatDate(a.created_at) }}</span>
        </span>
      </div>
      <span class="notice-more" @click="showAllNotice = true">查看全部</span>
    </div>

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

      <!-- ====== 模块化首页：主模块（管理员后台可配置）+ 其余模块副区块 ====== -->
      <HomeModuleSection
        v-for="(m, i) in orderedModules"
        :key="m"
        :module="m"
        :featured="i === 0"
      />

      <el-empty v-if="!orderedModules.length" description="暂无可展示的模块" />
    </div>

    <div class="view-toggle-float">
      <el-tooltip :content="novelView==='grid'?'全部切换为表式':'全部切换为平铺'" placement="left">
        <el-button type="primary" circle size="large" @click="novelView=novelView==='grid'?'table':'grid'">
          <el-icon :size="22"><Grid v-if="novelView==='table'"/><List v-else/></el-icon>
        </el-button>
      </el-tooltip>
    </div>

    <!-- ====== 推荐说明弹窗（176号令 算法透明度） ====== -->
    <el-dialog v-model="showAlgo" title="📊 推荐说明" width="560px">
      <div class="algo-desc" style="white-space: pre-wrap; line-height: 1.8">{{ algoDescription }}</div>
      <template #footer><el-button type="primary" @click="showAlgo = false">知道了</el-button></template>
    </el-dialog>

    <!-- ====== 公告详情/列表弹窗（内容经 cherry-markdown 渲染，与评论区同款样式） ====== -->
    <el-dialog v-model="noticeDlg.visible" :title="noticeDlg.item?.title || '公告'" width="600px">
      <div class="notice-dlg-content cherry-markdown cherry-previewer">
        <div v-html="noticeHtml"></div>
      </div>
      <div class="notice-dlg-date">发布于 {{ formatDate(noticeDlg.item?.created_at) }}</div>
    </el-dialog>
    <el-dialog v-model="showAllNotice" title="📢 全部公告" width="600px">
      <div v-if="!announcements.length" style="color: var(--text-light)">暂无公告</div>
      <div v-for="a in announcements" :key="a.id" class="notice-list-item">
        <el-tag v-if="a.pinned" type="danger" size="small" effect="plain" style="margin-right:6px">置顶</el-tag>
        <span class="notice-list-title" @click="openNotice(a); showAllNotice = false">{{ a.title }}</span>
        <span class="notice-list-date">{{ formatDate(a.created_at) }}</span>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue';
import { storeToRefs } from 'pinia';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { useAppStore } from '@/stores/app';
import { appPrefix } from '@/utils/prefix';
import { renderMarkdown } from '@/markdown/renderer';
import HomeSearchResults from '@/components/HomeSearchResults.vue';
import HomeModuleSection from '@/components/home/HomeModuleSection.vue';
import { Grid, List, Plus, ArrowRight, Search } from '@element-plus/icons-vue';

const authStore = useAuthStore();
const appStore = useAppStore();
const route = useRoute();
const router = useRouter();
// 分类列表与站点名来自应用级聚合 batch（stores/app.ts）；首页主模块也来自此处
const { categories, siteName, homeModule } = storeToRefs(appStore);
const novelView = ref<'grid'|'table'>('grid');

// ====== 模块化首页顺序：主模块在前，其余模块作为副区块（管理员后台可配置） ======
const MODULE_ALL = ['novels', 'blogs', 'forums'] as const;
const orderedModules = computed<string[]>(() => {
  const primary = homeModule.value === 'combined' ? 'novels' : (homeModule.value || 'novels');
  return [primary, ...MODULE_ALL.filter((m) => m !== primary)];
});

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

function formatDate(d?:string){return d?new Date(d).toLocaleDateString('zh-CN'):''}

// ====== 公告栏 + 算法说明（176号令合规） ======
const announcements = ref<any[]>([]);
const showAllNotice = ref(false);
const noticeDlg = reactive({ visible: false, item: null as any });
const algoDescription = ref('');
const showAlgo = ref(false);
const noticeHtml = computed(() => renderMarkdown(noticeDlg.item?.content || ''));

// 多公告排序：置顶优先，其余按发布时间倒序（后端 ListPublishedAnnouncements 已排序，前端二次保险）
function sortAnnouncements(list: any[]) {
  return [...list].sort((a, b) => {
    if (!!a.pinned !== !!b.pinned) return a.pinned ? -1 : 1;
    return new Date(b.created_at || 0).getTime() - new Date(a.created_at || 0).getTime();
  });
}

// 打开公告详情弹窗（标题 + 时间 + Markdown 全文）
function openNotice(a: any) {
  noticeDlg.item = a;
  noticeDlg.visible = true;
}

// 首屏由应用级 bootstrap 的 /api/batch 一并带回公告与算法说明
async function loadHomeInitial(){
  try{
    await appStore.ensureHomeLoaded();
    const d = appStore.homeData;
    if(!d) return;
    announcements.value = sortAnnouncements(d.announcements || []);
    if(d.algo) algoDescription.value = d.algo;
  }catch(e){ console.error(e); }
}
onMounted(async()=>{
  await loadHomeInitial();
});

// ====== 搜索功能 ======
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
/* ====== 公告栏（176号令：合规通知/安全公告） ====== */
.notice-bar {
  max-width: 1200px; margin: 14px auto 0; padding: 8px 16px;
  display: flex; align-items: center; gap: 12px;
  background: linear-gradient(90deg, rgba(250,204,21,0.12), rgba(250,204,21,0.03));
  border: 1px solid rgba(250,204,21,0.35); border-radius: 10px;
  font-size: 0.9rem;
}
.notice-icon { flex-shrink: 0; }
.notice-items { flex: 1; display: flex; flex-wrap: wrap; gap: 6px 18px; overflow: hidden; }
.notice-item { cursor: pointer; display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 360px; }
.notice-item-title { overflow: hidden; text-overflow: ellipsis; }
.notice-item-date { flex-shrink: 0; font-size: 0.75rem; color: var(--text-light, #aaa); }
.notice-item:hover { text-decoration: underline; }
.notice-pin-tag { flex-shrink: 0; }
.notice-more { flex-shrink: 0; cursor: pointer; color: var(--el-color-primary); font-size: 0.8rem; }
/* 公告详情弹窗：内容经 cherry-markdown 渲染，长文限高滚动 */
.notice-dlg-content { max-height: 60vh; overflow-y: auto; }
.notice-dlg-date { color: var(--text-light, #999); font-size: 0.8rem; margin-top: 8px; }
.notice-list-item { display: flex; align-items: center; padding: 8px 0; border-bottom: 1px dashed var(--el-border-color); }
.notice-list-title { flex: 1; cursor: pointer; }
.notice-list-title:hover { color: var(--el-color-primary); }
.notice-list-date { color: var(--text-light, #999); font-size: 0.8rem; }

/* ====== Hero Section ====== */
.hero-section {
  position: relative;
  height: 240px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  margin-bottom: 32px;
}
.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, #e8ecf1 0%, #dce3ed 50%, #cfd9e8 100%);
  z-index: 0;
}
.hero-bg::after {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse at 50% 80%, rgba(99,102,241,.12) 0%, transparent 60%),
              radial-gradient(ellipse at 20% 20%, rgba(139,92,246,.08) 0%, transparent 50%);
}
.hero-glass {
  position: relative;
  z-index: 1;
  text-align: center;
  padding: 32px 40px;
  background: rgba(255,255,255,.55);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(0,0,0,.06);
  border-radius: var(--radius-lg);
  max-width: 640px;
  width: 90%;
  box-shadow: 0 4px 24px rgba(0,0,0,.04);
}
.hero-glass h1 {
  font-size: 2rem;
  font-weight: 300;
  color: var(--primary-color);
  margin-bottom: 6px;
  letter-spacing: -.5px;
}
.hero-glass p {
  font-size: .95rem;
  color: var(--text-light);
  margin-bottom: 16px;
  font-weight: 300;
}
.hero-quick-links {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: center;
}
.hero-cat-chip {
  padding: 5px 16px;
  border-radius: 20px;
  font-size: .82rem;
  font-weight: 500;
  cursor: pointer;
  transition: all .2s;
  background: rgba(99,102,241,.08);
  color: #4f46e5;
  border: 1px solid rgba(99,102,241,.2);
}
.hero-cat-chip:hover {
  background: rgba(99,102,241,.16);
  border-color: rgba(99,102,241,.35);
}
.hero-cat-more {
  background: rgba(99,102,241,.16);
  border-color: rgba(99,102,241,.35);
  color: #4338ca;
}

/* ====== Dark Mode Hero ====== */
[data-theme="dark"] .hero-bg {
  background: linear-gradient(135deg, #0f1117 0%, #131620 50%, #0f172a 100%);
}
[data-theme="dark"] .hero-bg::after {
  background: radial-gradient(ellipse at 50% 80%, rgba(99,102,241,.18) 0%, transparent 60%),
              radial-gradient(ellipse at 20% 20%, rgba(139,92,246,.10) 0%, transparent 50%);
}
[data-theme="dark"] .hero-glass {
  background: rgba(255,255,255,.04);
  border-color: rgba(255,255,255,.08);
  box-shadow: none;
}
[data-theme="dark"] .hero-glass h1 { color: #fff; }
[data-theme="dark"] .hero-glass p { color: rgba(255,255,255,.65); }
[data-theme="dark"] .hero-cat-chip {
  background: rgba(255,255,255,.1);
  color: rgba(255,255,255,.85);
  border-color: rgba(255,255,255,.15);
}
[data-theme="dark"] .hero-cat-chip:hover {
  background: rgba(255,255,255,.22);
  border-color: rgba(255,255,255,.35);
}
[data-theme="dark"] .hero-cat-more {
  background: rgba(99,102,241,.25);
  border-color: rgba(99,102,241,.4);
  color: rgba(255,255,255,.85);
}

/* ====== Page Layout ====== */
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
  .hero-section { height: 150px; margin-bottom: 16px; }
  .hero-glass { padding: 18px 16px; width: calc(100% - 24px); }
  .hero-glass h1 { font-size: 1.3rem; margin-bottom: 4px; }
  .hero-glass p { font-size: 0.8rem; margin-bottom: 10px; }
  .hero-cat-chip { padding: 4px 12px; font-size: 0.75rem; }
  .view-toggle-float { bottom: calc(16px + env(safe-area-inset-bottom)); right: 16px; }
  .view-toggle-float .el-button { width: 48px; height: 48px; }
}
</style>
