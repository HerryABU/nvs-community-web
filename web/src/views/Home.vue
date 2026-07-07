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

    <div class="page-container">
      <div class="home-top-bar" v-if="authStore.isLoggedIn">
        <el-button type="primary" size="large" @click="$router.push('/author/editor')">
          <el-icon><Plus /></el-icon> 发布作品
        </el-button>
      </div>

      <!-- ====== 类型文学分类区域 ====== -->
      <div class="category-zone">
        <div class="cz-header">
          <h2>类型文学分类</h2>
          <el-radio-group v-model="catStyle" size="small">
            <el-radio-button value="tags">标签横排</el-radio-button>
            <el-radio-button value="blocks">大方块</el-radio-button>
          </el-radio-group>
        </div>

        <!-- 模式1：标签横排 -->
        <div v-if="catStyle === 'tags'" class="cat-tags">
          <el-button v-for="cat in categories" :key="cat" size="default" round
            @click="$router.push(`/category/${encodeURIComponent(cat)}`)">
            {{ cat }}
            <span class="cat-count">{{ catStats[cat]?.novel_count || 0 }}</span>
          </el-button>
        </div>

        <!-- 模式2：大方块卡片 -->
        <div v-else class="cat-blocks">
          <div v-for="cat in categories" :key="cat" class="cat-block-card"
            @click="$router.push(`/category/${encodeURIComponent(cat)}`)">
            <div class="cbc-cover-row">
              <div v-for="(n,i) in (catStats[cat]?.novels || []).slice(0,3)" :key="n.id"
                class="cbc-cover" :style="{ zIndex: 3-i }">
                <el-image v-if="n.cover_url" :src="n.cover_url" fit="cover"
                  style="width:48px;height:64px;border-radius:3px;border:2px solid #fff;box-shadow:0 1px 3px rgba(0,0,0,.15)" />
                <div v-else class="cbc-cover-ph">封</div>
              </div>
              <div v-if="!catStats[cat]?.novels?.length" class="cbc-cover-empty">📚</div>
            </div>
            <div class="cbc-name">{{ cat }}</div>
            <div class="cbc-count">{{ catStats[cat]?.novel_count || 0 }} 部作品</div>
          </div>
        </div>
      </div>

      <!-- ====== 全局排序 + 小说视图切换 ====== -->
      <div class="global-toolbar">
        <el-radio-group v-model="sortBy" size="small" @change="reloadAll">
          <el-radio-button value="featured">本周最佳</el-radio-button>
          <el-radio-button value="newest">新品上架</el-radio-button>
          <el-radio-button value="updated">最近更新</el-radio-button>
        </el-radio-group>
        <el-radio-group v-model="novelView" size="small">
          <el-radio-button value="grid"><el-icon><Grid /></el-icon> 平铺</el-radio-button>
          <el-radio-button value="table"><el-icon><List /></el-icon> 表式</el-radio-button>
        </el-radio-group>
      </div>

      <!-- ====== 按分类展示小说 ====== -->
      <div v-loading="loading">
        <template v-for="cat in categories" :key="cat">
          <div v-if="categoryGroups[cat]?.length" class="novel-section">
            <div class="ns-header">
              <h3>{{ cat }}</h3>
              <el-button text type="primary" size="small"
                @click="$router.push(`/category/${encodeURIComponent(cat)}`)">
                查看全部 {{ (catStats[cat]?.novel_count || categoryGroups[cat].length) }} 部 <el-icon><ArrowRight /></el-icon>
              </el-button>
            </div>

            <div v-if="novelView === 'grid'" class="card-grid">
              <NovelCard v-for="novel in categoryGroups[cat].slice(0,3)" :key="novel.id"
                :novel="novel" @click="$router.push(`/novel/${novel.id}`)" />
            </div>

            <el-table v-else :data="categoryGroups[cat].slice(0,3)" stripe size="small"
              @row-click="(r:any)=>$router.push(`/novel/${r.id}`)" style="cursor:pointer">
              <el-table-column label="封面" width="60">
                <template #default="{row}">
                  <el-image v-if="row.cover_url" :src="row.cover_url" fit="cover" style="width:36px;height:48px;border-radius:3px"/>
                  <div v-else style="width:36px;height:48px;border-radius:3px;background:#e0e0e0;display:flex;align-items:center;justify-content:center;font-size:10px;color:#999">封</div>
                </template>
              </el-table-column>
              <el-table-column label="作品" min-width="180"><template #default="{row}"><span style="font-weight:500">{{row.title}}</span></template></el-table-column>
              <el-table-column label="作者" width="90"><template #default="{row}"><router-link :to="`/author/${row.author_id}`" @click.stop>{{row.author_name||'—'}}</router-link></template></el-table-column>
              <el-table-column prop="total_words" label="字数" width="80"/>
              <el-table-column prop="total_chapters" label="章节" width="60"/>
              <el-table-column label="更新" width="90"><template #default="{row}">{{formatDate(row.updated_at)}}</template></el-table-column>
            </el-table>
          </div>
        </template>
        <el-empty v-if="!loading && total===0" description="暂无作品"/>
      </div>
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
import { ref, onMounted } from 'vue';
import { novelApi, type Novel } from '@/api/novel';
import { publicApi } from '@/api/admin';
import { useAuthStore } from '@/stores/auth';
import NovelCard from '@/components/NovelCard.vue';
import { Grid, List, Plus, ArrowRight } from '@element-plus/icons-vue';

const authStore = useAuthStore();
const categories = ref<string[]>([]);
const loading = ref(false);
const total = ref(0);
const catStyle = ref<'tags'|'blocks'>('tags');
const novelView = ref<'grid'|'table'>('grid');
const sortBy = ref<'featured'|'newest'|'updated'>('featured');
const categoryGroups = ref<Record<string,Novel[]>>({});
const catStats = ref<Record<string,{novel_count:number;novels:Novel[]}>>({});
const siteName = ref('星海文学');

function formatDate(d?:string){return d?new Date(d).toLocaleDateString('zh-CN'):''}

function groupByCategory(list:Novel[]):Record<string,Novel[]>{
  const g:Record<string,Novel[]>={};
  for(const n of list){
    const cs=(n.categories&&n.categories.length>0)?n.categories:(n.category?[n.category]:['其他']);
    for(const c of cs){if(!g[c])g[c]=[];g[c].push(n)}
  }
  return g;
}

async function fetchCatStats(){
  try{
    const res=await publicApi.getCategoryStats();
    if(res.data.code===0&&Array.isArray(res.data.data)){
      const map:Record<string,any>={};
      for(const item of res.data.data) map[item.name]=item;
      catStats.value=map;
    }
  }catch{/* */}
}

async function reloadAll(){
  loading.value=true;
  try{
    const params:any={page:1,page_size:60};
    if(sortBy.value==='newest')params.sort_by='created_at';
    else if(sortBy.value==='updated')params.sort_by='updated_at';
    else params.sort_by='featured';
    const res=await novelApi.getNovels(params);
    const list:Novel[]=res.data.data.list||[];
    total.value=res.data.data.total||0;
    categoryGroups.value=groupByCategory(list);
  }catch(e){console.error(e)}
  finally{loading.value=false}
}

async function loadCats(){
  try{
    const res=await publicApi.getCategories();
    if(res.data.code===0&&Array.isArray(res.data.data))categories.value=res.data.data;
  }catch{/* */}
}

async function loadSiteName(){
  try{
    const res=await publicApi.getSiteInfo();
    if(res.data.code===0&&res.data.data.site_name)siteName.value=res.data.data.site_name;
  }catch{/* */}
}

onMounted(async()=>{
  await loadCats();
  loadSiteName();
  await Promise.all([fetchCatStats(),reloadAll()]);
});
</script>

<style scoped>
/* ====== Hero Carousel — 2024 Modern ====== */
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
[data-theme="dark"] .hero-glass h1 {
  color: #fff;
}
[data-theme="dark"] .hero-glass p {
  color: rgba(255,255,255,.65);
}
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

.banner-carousel-wrapper {
  display: none;
}
.banner-carousel-wrapper {
  margin-bottom: 4px;
}
.banner-carousel-wrapper :deep(.el-carousel__indicators) {
  bottom: 8px;
}
.banner-carousel-wrapper :deep(.el-carousel__indicator .el-carousel__button) {
  background: rgba(255,255,255,.5);
  opacity: .5;
  width: 24px;
  height: 4px;
  border-radius: 2px;
  transition: all var(--transition-fast);
}
.banner-carousel-wrapper :deep(.el-carousel__indicator.is-active .el-carousel__button) {
  background: #fff;
  opacity: 1;
  width: 36px;
}

.banner-slide {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
}

/* Flowing gradient background animation */
.banner-slide::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  opacity: 0.3;
  background: linear-gradient(
    270deg,
    rgba(255,255,255,0.06) 0%,
    rgba(255,255,255,0.15) 25%,
    rgba(255,255,255,0.06) 50%,
    rgba(255,255,255,0.15) 75%,
    rgba(255,255,255,0.06) 100%
  );
  background-size: 400% 100%;
  animation: bannerFlow 8s ease-in-out infinite;
}

@keyframes bannerFlow {
  0%, 100% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
}

/* Glow orb */
.banner-glow {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  animation: bannerGlowPulse 5s ease-in-out infinite alternate;
}

@keyframes bannerGlowPulse {
  0% { opacity: 0.5; transform: scale(1); }
  100% { opacity: 1; transform: scale(1.1); }
}

/* Glass card */
.banner-glass-card {
  position: relative;
  z-index: 2;
  background: rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 24px;
  padding: 36px 56px;
  text-align: center;
  color: #fff;
  box-shadow: 0 8px 40px rgba(0, 0, 0, 0.25), inset 0 1px 0 rgba(255, 255, 255, 0.12);
  animation: bannerCardFloat 6s ease-in-out infinite;
  max-width: 560px;
  cursor: default;
  transition: all var(--transition-base);
}

.banner-glass-card:hover {
  background: rgba(255, 255, 255, 0.16);
  border-color: rgba(255, 255, 255, 0.3);
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.3), inset 0 1px 0 rgba(255, 255, 255, 0.18);
}

@keyframes bannerCardFloat {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-10px); }
}

.banner-icon {
  font-size: 3.2rem;
  margin-bottom: 16px;
  filter: drop-shadow(0 2px 12px rgba(0,0,0,.35));
}

.banner-glass-card h2 {
  font-size: 2.4rem;
  font-weight: 200;
  letter-spacing: -0.03em;
  margin-bottom: 10px;
  text-shadow: 0 2px 12px rgba(0, 0, 0, 0.35);
  line-height: 1.2;
}

.banner-glass-card p {
  font-size: 1.1rem;
  font-weight: 300;
  opacity: 0.85;
  margin-bottom: 20px;
  text-shadow: 0 1px 6px rgba(0, 0, 0, 0.25);
  letter-spacing: 0.01em;
}

.banner-action {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: rgba(255, 255, 255, 0.18);
  border: 1px solid rgba(255, 255, 255, 0.3);
  border-radius: 9999px;
  padding: 10px 28px;
  font-size: 0.95rem;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-fast);
  backdrop-filter: blur(6px);
  letter-spacing: 0.02em;
}

.banner-action:hover {
  background: rgba(255, 255, 255, 0.3);
  transform: translateY(-2px);
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.25);
}

.banner-arrow {
  transition: transform var(--transition-fast);
}

.banner-action:hover .banner-arrow {
  transform: translateX(4px);
}

/* Floating particles */
.banner-particles {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 1;
}

.banner-particle {
  position: absolute;
  bottom: -10px;
  width: 5px;
  height: 5px;
  background: rgba(255, 255, 255, 0.55);
  border-radius: 50%;
  box-shadow: 0 0 6px rgba(255,255,255,0.3);
  animation: particleRise linear infinite;
}

@keyframes particleRise {
  0% {
    bottom: -10px;
    opacity: 0;
    transform: translateX(0) scale(0.3);
  }
  15% {
    opacity: 1;
  }
  85% {
    opacity: 0.4;
  }
  100% {
    bottom: 115%;
    opacity: 0;
    transform: translateX(30px) scale(1.4);
  }
}

/* ====== Dark Mode Carousel ====== */
[data-theme="dark"] .banner-glass-card {
  background: rgba(15, 17, 23, 0.55);
  border-color: rgba(255, 255, 255, 0.08);
  box-shadow: 0 8px 40px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(255, 255, 255, 0.05);
  color: #e2e8f0;
}

[data-theme="dark"] .banner-glass-card:hover {
  background: rgba(15, 17, 23, 0.65);
}

[data-theme="dark"] .banner-glass-card h2 {
  text-shadow: 0 2px 12px rgba(0, 0, 0, 0.6);
}

[data-theme="dark"] .banner-glass-card p {
  text-shadow: 0 1px 6px rgba(0, 0, 0, 0.5);
}

[data-theme="dark"] .banner-action {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.12);
}

[data-theme="dark"] .banner-action:hover {
  background: rgba(255, 255, 255, 0.14);
}

[data-theme="dark"] .banner-particle {
  background: rgba(255, 255, 255, 0.35);
}

[data-theme="dark"] .banner-carousel-wrapper :deep(.el-carousel__indicator .el-carousel__button) {
  background: rgba(255,255,255,.25);
}

/* ====== Page Layout ====== */
.home-top-bar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 16px;
  gap: 10px;
}

/* Category Zone */
.category-zone {
  background: #fff;
  border-radius: var(--radius-lg);
  padding: 24px 28px;
  margin-bottom: 28px;
  box-shadow: var(--shadow-card);
}
[data-theme="dark"] .category-zone {
  background: #1a1d27;
  box-shadow: var(--shadow-card);
}

.cz-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.cz-header h2 {
  font-size: 1.3rem;
  color: var(--text-color);
  font-weight: 300;
  letter-spacing: -0.02em;
}

.cat-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.cat-tags .el-button {
  font-size: 0.9rem;
  border-radius: var(--radius-full) !important;
}
.cat-count {
  display: inline-block;
  background: rgba(99,102,241,.12);
  color: #6366f1;
  padding: 0 7px;
  border-radius: var(--radius-full);
  margin-left: 4px;
  font-size: 0.72rem;
  font-weight: 600;
  min-width: 22px;
  text-align: center;
}

.cat-blocks {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(155px, 1fr));
  gap: 14px;
}
.cat-block-card {
  cursor: pointer;
  background: var(--bg-soft);
  border-radius: var(--radius-md);
  padding: 18px;
  text-align: center;
  transition: all var(--transition-base);
  border: 2px solid transparent;
}
.cat-block-card:hover {
  background: #eef2ff;
  border-color: var(--primary-color);
  transform: translateY(-3px);
  box-shadow: var(--shadow-md);
}
[data-theme="dark"] .cat-block-card {
  background: #1a1d27;
}
[data-theme="dark"] .cat-block-card:hover {
  background: #1e293b;
  box-shadow: var(--shadow-md);
}
.cbc-cover-row {
  display: flex;
  justify-content: center;
  align-items: flex-end;
  gap: 2px;
  height: 72px;
  margin-bottom: 12px;
}
.cbc-cover {
  position: relative;
}
.cbc-cover-ph {
  width: 48px;
  height: 64px;
  border-radius: 4px;
  border: 2px solid #fff;
  box-shadow: 0 1px 3px rgba(0,0,0,.12);
  background: #e2e8f0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #94a3b8;
  font-size: 11px;
}
[data-theme="dark"] .cbc-cover-ph {
  background: #334155;
  border-color: #475569;
  color: #64748b;
  box-shadow: 0 1px 3px rgba(0,0,0,.3);
}
.cbc-cover-empty {
  font-size: 2rem;
  opacity: .4;
}
.cbc-name {
  font-weight: 500;
  color: var(--text-color);
  font-size: 0.95rem;
  margin-bottom: 4px;
}
.cbc-count {
  font-size: 0.75rem;
  color: var(--text-light);
}

/* Novel Sections */
.global-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  flex-wrap: wrap;
  gap: 12px;
  background: #fff;
  padding: 14px 20px;
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
}
[data-theme="dark"] .global-toolbar {
  background: #1a1d27;
  box-shadow: var(--shadow-sm);
}
.novel-section {
  margin-bottom: 32px;
}
.ns-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 14px;
}
.ns-header h3 {
  font-size: 1.15rem;
  font-weight: 300;
  color: var(--text-color);
  letter-spacing: -0.01em;
  border-left: 3px solid var(--primary-color);
  padding-left: 12px;
}

.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
  gap: 20px;
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

/* Dark: table placeholder fix */
[data-theme="dark"] .el-table .cbc-cover-ph,
[data-theme="dark"] .novel-section [style*="background:#e0e0e0"] {
  background: #334155 !important;
  color: #64748b !important;
}
</style>
