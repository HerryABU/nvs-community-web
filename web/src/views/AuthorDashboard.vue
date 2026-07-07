<template>
  <div class="dashboard-bg">
    <div class="dashboard-container">
      <!-- 顶部标题 -->
      <div class="dashboard-hero">
        <h1 class="dashboard-main-title">
          <span class="title-glow">创作数据大屏</span>
        </h1>
        <p class="dashboard-subtitle">跟踪创作数据 · 管理作品</p>
        <el-button class="create-btn" size="large" @click="$router.push('/author/editor')">
          <el-icon><EditPen /></el-icon>新建作品
        </el-button>
      </div>

      <!-- 第一行：统计卡片 -->
      <el-row :gutter="20" class="stats-row">
        <el-col :xs="24" :sm="12" :md="6" v-for="(card, idx) in statCardDefs" :key="idx">
          <div class="glass-stat-card">
            <div class="glass-stat-icon" :style="{ background: card.gradient }">
              <el-icon :size="24"><component :is="card.icon" /></el-icon>
            </div>
            <div class="glass-stat-body">
              <AnimatedNumber
                :target="card.value"
                :duration="1400"
                class="glass-stat-value"
              />
              <div class="glass-stat-label">{{ card.label }}</div>
            </div>
          </div>
        </el-col>
      </el-row>

      <!-- 第二行：折线图 + 柱状图 -->
      <el-row :gutter="20" class="chart-row">
        <el-col :xs="24" :md="12">
          <DashboardCharts :visitor-trend="chapterTrend" />
        </el-col>
        <el-col :xs="24" :md="12">
          <DashboardCharts :novel-bars="novelWordBars" />
        </el-col>
      </el-row>

      <!-- 第三行：仪表盘 + 仪表盘 + 评论趋势 -->
      <el-row :gutter="20" class="chart-row">
        <el-col :xs="24" :md="8">
          <DashboardCharts :completion-gauge="completionRate" />
        </el-col>
        <el-col :xs="24" :md="8">
          <DashboardCharts :rating-gauge="avgRating" />
        </el-col>
        <el-col :xs="24" :md="8">
          <DashboardCharts :visitor-trend="commentTrend" />
        </el-col>
      </el-row>

      <!-- 底部：作品管理卡片列表 -->
      <div class="novels-section">
        <h2 class="section-title-bar">
          <span class="title-bar-text">我的作品</span>
          <span class="title-bar-count">{{ novels.length }} 部</span>
        </h2>
        <div v-loading="loadingNovels">
          <el-empty v-if="!loadingNovels && novels.length === 0" description="还没有作品，快去创作吧！">
            <el-button type="primary" @click="$router.push('/author/editor')">新建作品</el-button>
          </el-empty>

          <div class="novel-card-grid" v-else>
            <div v-for="novel in novels" :key="novel.id" class="novel-card glass-novel-card">
              <div class="novel-card-cover">
                <el-image v-if="novel.cover_url" :src="novel.cover_url" fit="cover" class="cover-img">
                  <template #error>
                    <div class="cover-placeholder"><el-icon :size="32"><Document /></el-icon></div>
                  </template>
                </el-image>
                <div v-else class="cover-placeholder"><el-icon :size="32"><Document /></el-icon></div>
                <div class="novel-status-badge" :class="novel.status">
                  {{ novel.status === 'published' ? '已发布' : novel.status === 'draft' ? '草稿' : '已隐藏' }}
                </div>
              </div>
              <div class="novel-card-body">
                <h3 class="novel-card-title">{{ novel.title }}</h3>
                <div class="novel-card-tags">
                  <template v-if="novel.categories && novel.categories.length > 0">
                    <el-tag size="small" v-for="cat in novel.categories" :key="cat" class="cat-tag">{{ cat }}</el-tag>
                  </template>
                  <el-tag size="small" v-else class="cat-tag">{{ novel.category }}</el-tag>
                </div>
                <div class="novel-card-meta">
                  <span><el-icon><Collection /></el-icon>{{ novel.total_chapters || 0 }} 章</span>
                  <span><el-icon><MagicStick /></el-icon>{{ (novel.total_words || 0).toLocaleString() }} 字</span>
                </div>
              </div>
              <div class="novel-card-actions">
                <el-button size="small" type="primary" plain @click="$router.push(`/author/editor/${novel.id}`)">
                  <el-icon><Edit /></el-icon>编辑
                </el-button>
                <el-button size="small" plain @click="$router.push(`/novel/${novel.id}`)">
                  <el-icon><View /></el-icon>查看
                </el-button>
                <el-dropdown trigger="click" @command="(fmt: string) => handleExport(novel.id, fmt)">
                  <el-button size="small" type="warning" plain :loading="exportingId === novel.id">
                    <el-icon><Download /></el-icon>导出<el-icon class="el-icon--right"><ArrowDown /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="zip">ZIP 打包</el-dropdown-item>
                      <el-dropdown-item command="epub">EPUB 电子书</el-dropdown-item>
                      <el-dropdown-item command="md">合并 Markdown</el-dropdown-item>
                      <el-dropdown-item command="txt">纯文本 TXT</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
                <el-button size="small" type="danger" plain @click="handleDelete(novel)">
                  <el-icon><Delete /></el-icon>删除
                </el-button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue';
import { useRoute } from 'vue-router';
import { novelApi, type Novel } from '@/api/novel';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  ArrowDown, EditPen, Document, Collection, MagicStick,
  Edit, View, Download, Delete, Notebook, DataLine, Files, ChatLineRound
} from '@element-plus/icons-vue';
import AnimatedNumber from '@/components/AnimatedNumber.vue';
import DashboardCharts from '@/components/DashboardCharts.vue';

const route = useRoute();

const novels = ref<Novel[]>([]);
const loadingNovels = ref(false);
const exportingId = ref(0);

// 统计数据
const statsCards = reactive([
  { label: '作品总数', value: 0 },
  { label: '总字数', value: 0 },
  { label: '总章节', value: 0 },
  { label: '总评论', value: 0 },
]);

const statCardDefs = computed(() => [
  { label: '作品总数', value: statsCards[0].value, icon: Notebook, gradient: 'var(--gradient-blue)' },
  { label: '总字数', value: statsCards[1].value, icon: DataLine, gradient: 'var(--gradient-purple)' },
  { label: '总章节', value: statsCards[2].value, icon: Files, gradient: 'var(--gradient-teal)' },
  { label: '总评论', value: statsCards[3].value, icon: ChatLineRound, gradient: 'var(--gradient-amber)' },
]);

// 图表数据
const chapterTrend = ref<any>({
  title: '章节增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增章节',
});

const novelWordBars = ref<any>({
  title: '各作品字数对比',
  labels: [],
  values: [],
  seriesName: '字数',
});

const commentTrend = ref<any>({
  title: '评论趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '评论数',
});

const completionRate = ref<number | null>(null);
const avgRating = ref<number | null>(null);

const exportFormats: Record<string, { label: string; ext: string; mime: string; fn: (id: number) => Promise<any> }> = {
  zip: { label: 'ZIP 打包', ext: 'zip', mime: 'application/zip', fn: novelApi.exportNovel },
  epub: { label: 'EPUB 电子书', ext: 'epub', mime: 'application/epub+zip', fn: novelApi.exportNovelEPUB },
  md: { label: '合并 Markdown', ext: 'md', mime: 'text/markdown', fn: novelApi.exportNovelMarkdown },
  txt: { label: '纯文本 TXT', ext: 'txt', mime: 'text/plain', fn: novelApi.exportNovelTXT },
};

async function loadDashboard() {
  try {
    const res = await adminApi.getAuthorDashboard();
    if (res.data.code === 0) {
      const d = res.data.data;

      if (d.stats) {
        statsCards[0].value = d.stats.novels || 0;
        statsCards[1].value = d.stats.total_words || 0;
        statsCards[2].value = d.stats.total_chapters || 0;
        statsCards[3].value = d.stats.total_comments || 0;
      }

      if (d.chapter_trend) {
        chapterTrend.value = {
          title: '章节增长趋势（近7天）',
          dates: d.chapter_trend.dates || [],
          values: d.chapter_trend.counts || [],
          seriesName: '新增章节',
        };
      }

      if (d.comment_trend) {
        commentTrend.value = {
          title: '评论趋势（近7天）',
          dates: d.comment_trend.dates || [],
          values: d.comment_trend.counts || [],
          seriesName: '评论数',
        };
      }

      if (d.completion_rate !== undefined) {
        completionRate.value = d.completion_rate;
      }

      if (d.avg_rating !== undefined) {
        avgRating.value = d.avg_rating;
      }
    }
  } catch {
    // fallback
  }
}

async function loadNovels() {
  loadingNovels.value = true;
  try {
    const res = await novelApi.getMyNovels();
    novels.value = res.data.data || [];

    // Build chart data from novels
    novelWordBars.value = {
      title: '各作品字数对比',
      labels: novels.value.map(n => n.title.length > 8 ? n.title.slice(0, 7) + '…' : n.title),
      values: novels.value.map(n => n.total_words || 0),
      seriesName: '字数',
    };

    // If dashboard didn't provide stats, compute from novels
    if (statsCards[0].value === 0) {
      statsCards[0].value = novels.value.length;
      statsCards[1].value = novels.value.reduce((sum, n) => sum + (n.total_words || 0), 0);
      statsCards[2].value = novels.value.reduce((sum, n) => sum + (n.total_chapters || 0), 0);
    }

    // Compute completion rate if not provided
    if (completionRate.value === null && novels.value.length > 0) {
      const completed = novels.value.filter(n => n.status === 'published').length;
      completionRate.value = completed / novels.value.length;
    }
  } catch (e) {
    console.error(e);
  } finally {
    loadingNovels.value = false;
  }
}

async function handleExport(id: number, format: string = 'zip') {
  const fmt = exportFormats[format] || exportFormats.zip;
  exportingId.value = id;
  try {
    const res = await fmt.fn.call(novelApi, id);
    const blob = new Blob([res.data], { type: fmt.mime });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `novel_${id}.${fmt.ext}`;
    a.click();
    URL.revokeObjectURL(url);
    ElMessage.success(`${fmt.label}导出成功`);
  } catch {
    ElMessage.error(`${fmt.label}导出失败`);
  } finally {
    exportingId.value = 0;
  }
}

async function handleDelete(novel: Novel) {
  try {
    await ElMessageBox.confirm(`确定要删除《${novel.title}》吗？此操作不可恢复！`, '删除确认', {
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
      type: 'warning',
    });
    await novelApi.deleteNovel(novel.id);
    ElMessage.success('已删除');
    await loadNovels();
  } catch { /* cancelled */ }
}

onMounted(() => {
  loadDashboard();
  loadNovels();
});

// 从编辑页面返回时自动刷新
let lastPath = route.fullPath;
import { watch } from 'vue';
watch(
  () => route.fullPath,
  (newPath) => {
    if (newPath === '/author' && lastPath !== '/author') {
      loadDashboard();
      loadNovels();
    }
    lastPath = newPath;
  }
);
</script>

<style scoped>
/* ===== 全局背景 ===== */
.dashboard-bg {
  min-height: 100vh;
  background: linear-gradient(135deg, #0f172a 0%, #1e293b 40%, #0f172a 100%);
  position: relative;
}

[data-theme="light"] .dashboard-bg {
  background: linear-gradient(135deg, #e2e8f0 0%, #f1f5f9 40%, #e2e8f0 100%);
}

.dashboard-bg::before {
  content: '';
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background:
    radial-gradient(ellipse at 20% 20%, rgba(139, 92, 246, 0.08) 0%, transparent 50%),
    radial-gradient(ellipse at 80% 60%, rgba(99, 102, 241, 0.06) 0%, transparent 50%),
    radial-gradient(ellipse at 50% 80%, rgba(245, 158, 11, 0.04) 0%, transparent 50%);
  pointer-events: none;
  z-index: 0;
}

[data-theme="light"] .dashboard-bg::before {
  background:
    radial-gradient(ellipse at 20% 20%, rgba(139, 92, 246, 0.04) 0%, transparent 50%),
    radial-gradient(ellipse at 80% 60%, rgba(99, 102, 241, 0.03) 0%, transparent 50%),
    radial-gradient(ellipse at 50% 80%, rgba(245, 158, 11, 0.02) 0%, transparent 50%);
}

.dashboard-container {
  max-width: 1280px;
  margin: 0 auto;
  padding: 40px 24px 60px;
  position: relative;
  z-index: 1;
}

/* ===== Hero ===== */
.dashboard-hero {
  text-align: center;
  margin-bottom: 40px;
}

.dashboard-main-title {
  font-size: 2.4rem;
  font-weight: 800;
  margin: 0 0 8px;
  line-height: 1.3;
}

.title-glow {
  background: linear-gradient(135deg, #a78bfa, #f59e0b, #34d399);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  filter: drop-shadow(0 0 18px rgba(167, 139, 250, 0.35));
}

.dashboard-subtitle {
  color: var(--text-light);
  font-size: 0.95rem;
  margin: 0 0 16px;
}

.create-btn {
  background: linear-gradient(135deg, #818cf8, #6366f1) !important;
  border: none !important;
  color: #fff !important;
  font-weight: 600 !important;
  padding: 12px 28px !important;
  border-radius: 12px !important;
  box-shadow: 0 4px 20px rgba(99, 102, 241, 0.35);
}

.create-btn:hover {
  box-shadow: 0 6px 28px rgba(99, 102, 241, 0.5) !important;
  transform: translateY(-2px);
}

/* ===== 统计卡片 ===== */
.stats-row {
  margin-bottom: 24px;
}

.glass-stat-card {
  background: rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 16px;
  padding: 24px 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  transition: transform 0.3s ease, box-shadow 0.3s ease, border-color 0.3s ease;
}

[data-theme="light"] .glass-stat-card {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}

.glass-stat-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
  border-color: rgba(255, 255, 255, 0.2);
}

[data-theme="light"] .glass-stat-card:hover {
  box-shadow: var(--shadow-lg);
  border-color: var(--border-color);
}

.glass-stat-icon {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  flex-shrink: 0;
}

.glass-stat-body {
  flex: 1;
  min-width: 0;
}

.glass-stat-value :deep(.animated-number-value) {
  background: linear-gradient(135deg, #e2e8f0, #f1f5f9);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-size: 2rem;
}

[data-theme="light"] .glass-stat-value :deep(.animated-number-value) {
  background: var(--gradient-primary);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.glass-stat-label {
  font-size: 0.82rem;
  color: rgba(255, 255, 255, 0.5);
  margin-top: 2px;
}

[data-theme="light"] .glass-stat-label {
  color: var(--text-light);
}

/* ===== 图表行 ===== */
.chart-row {
  margin-bottom: 24px;
}

/* ===== 作品列表 ===== */
.novels-section {
  margin-top: 8px;
}

.section-title-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

[data-theme="light"] .section-title-bar {
  border-bottom-color: var(--border-color);
}

.title-bar-text {
  font-size: 1.25rem;
  font-weight: 700;
  color: #e2e8f0;
}

[data-theme="light"] .title-bar-text {
  color: var(--text-color);
}

.title-bar-count {
  font-size: 0.85rem;
  color: rgba(255, 255, 255, 0.4);
  background: rgba(255, 255, 255, 0.06);
  padding: 2px 10px;
  border-radius: 12px;
}

[data-theme="light"] .title-bar-count {
  color: var(--text-light);
  background: var(--bg-color);
}

/* ===== 作品卡片 ===== */
.novel-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}

.glass-novel-card {
  border-radius: 16px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: transform 0.3s ease, box-shadow 0.3s ease, border-color 0.3s ease;
}

[data-theme="light"] .glass-novel-card {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}

.glass-novel-card:hover {
  transform: translateY(-6px);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.35);
  border-color: rgba(255, 255, 255, 0.15);
}

[data-theme="light"] .glass-novel-card:hover {
  box-shadow: var(--shadow-lg);
  border-color: var(--border-color);
}

.novel-card-cover {
  position: relative;
  height: 200px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.15), rgba(139, 92, 246, 0.1));
  overflow: hidden;
}

.cover-img {
  width: 100%;
  height: 100%;
}

.cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.2);
}

[data-theme="light"] .cover-placeholder {
  color: rgba(0, 0, 0, 0.15);
}

.novel-status-badge {
  position: absolute;
  top: 12px;
  right: 12px;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 0.75rem;
  font-weight: 600;
  color: #fff;
  backdrop-filter: blur(6px);
}

.novel-status-badge.published {
  background: rgba(39, 174, 96, 0.85);
}

.novel-status-badge.draft {
  background: rgba(149, 165, 166, 0.85);
}

.novel-status-badge.hidden {
  background: rgba(231, 76, 60, 0.85);
}

.novel-card-body {
  padding: 16px;
  flex: 1;
}

.novel-card-title {
  font-size: 1.1rem;
  font-weight: 600;
  color: #e2e8f0;
  margin: 0 0 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

[data-theme="light"] .novel-card-title {
  color: var(--text-color);
}

.novel-card-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 10px;
}

.cat-tag {
  font-size: 0.75rem;
}

.novel-card-meta {
  display: flex;
  gap: 16px;
  font-size: 0.85rem;
  color: rgba(255, 255, 255, 0.45);
}

[data-theme="light"] .novel-card-meta {
  color: var(--text-light);
}

.novel-card-meta span {
  display: flex;
  align-items: center;
  gap: 4px;
}

.novel-card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 12px 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

[data-theme="light"] .novel-card-actions {
  border-top-color: var(--border-color);
}
</style>
