<template>
  <div class="dashboard-bg">
    <div class="dashboard-container">
      <!-- 顶部标题 -->
      <div class="dashboard-hero">
        <h1 class="dashboard-main-title">
          <span class="title-glow">管理员数据大屏</span>
        </h1>
        <p class="dashboard-subtitle">实时监控平台数据 · 管理站点配置</p>
        <div class="hero-buttons">
          <el-button class="refresh-btn" @click="refreshAll" :loading="refreshing" size="default">
            <el-icon><Refresh /></el-icon>刷新数据
          </el-button>
          <el-button type="success" size="default" @click="$router.push('/admin/users')">
            <el-icon><User /></el-icon>用户管理
          </el-button>
        </div>
      </div>

      <!-- 备案状态提醒（176号令 第七条第一项，F7） -->
      <el-alert
        v-if="!filingReady"
        type="warning"
        show-icon
        :closable="false"
        style="margin-bottom: 20px"
        title="⚠️ 请尽快完成工信部ICP备案和公安联网备案，并在此配置备案号"
      >
        <template #default>
          <span style="font-size: 0.85rem">
            未完成备案的网站可能面临行政处罚。前往
            <el-link type="primary" @click="$router.push('/admin/settings')">站点设置 → 联网备案</el-link>
            填写备案号（填好后将自动显示在站点页脚）。
          </span>
        </template>
      </el-alert>

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

      <!-- 第二行：趋势图（多指标切换） -->
      <el-row :gutter="20" class="chart-row">
        <el-col :xs="24" :md="12">
          <DashboardCharts :metrics="adminTrendMetrics" />
        </el-col>
        <el-col :xs="24" :md="12">
          <DashboardCharts :metrics="adminTrendMetrics" />
        </el-col>
      </el-row>

      <!-- 第三行：柱状图 + 饼图 -->
      <el-row :gutter="20" class="chart-row">
        <el-col :xs="24" :md="12">
          <DashboardCharts :novel-bars="topNovelBars" />
        </el-col>
        <el-col :xs="24" :md="12">
          <DashboardCharts :category-pie="categoryPieData" />
        </el-col>
      </el-row>

      <!-- 管理功能入口（F8：分类折叠 + ?category=/?item= 深链定位） -->
      <AdminMenuPanel />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue';
import { adminApi } from '@/api/admin';
import {
  User, Document, ChatLineRound, Connection,
  Refresh
} from '@element-plus/icons-vue';
import AnimatedNumber from '@/components/AnimatedNumber.vue';
import DashboardCharts from '@/components/DashboardCharts.vue';
import AdminMenuPanel from '@/components/admin/AdminMenuPanel.vue';

const refreshing = ref(false);

const stats = reactive({ users: 0, novels: 0, comments: 0, forums: 0 });

// 备案状态（176号令 第七条第一项，F7 首页提醒）
const filingReady = ref(true);
async function checkFiling() {
  try {
    const res = await adminApi.getConfig();
    const d = res.data.data || {};
    filingReady.value = !!(d.site_icp && d.site_psb);
  } catch { /* 保持默认（视为已配置，避免误报） */ }
}

// 统计卡片
const statCardDefs = computed(() => [
  { label: '用户数', value: stats.users, icon: User, gradient: 'var(--gradient-blue)' },
  { label: '作品数', value: stats.novels, icon: Document, gradient: 'var(--gradient-purple)' },
  { label: '评论数', value: stats.comments, icon: ChatLineRound, gradient: 'var(--gradient-teal)' },
  { label: '论坛数', value: stats.forums, icon: Connection, gradient: 'var(--gradient-accent)' },
]);

// 管理功能入口已抽离至 @/components/admin/AdminMenuPanel.vue（分类折叠 + 深链定位）

// 折线图数据
const userTrend = ref<any>({
  title: '用户增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增用户',
  secondValues: [],
  secondName: '活跃用户',
});

const chapterTrend = ref<any>({
  title: '章节增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增章节',
});

// 柱状图数据
const topNovelBars = ref<any>({
  title: 'Top 6 作品字数对比',
  labels: [] as string[],
  values: [] as number[],
  seriesName: '字数',
});

// 饼图数据
const categoryPieData = ref<any>({
  title: '分类分布',
  data: [],
});

const adminTrendMetrics = computed(() => {
  const opts: any[] = [
    { label: '用户增长趋势', data: userTrend.value },
    { label: '章节增长趋势', data: chapterTrend.value },
  ];
  if (wordCountTrend.value?.dates?.length) {
    opts.push({ label: '字数增长趋势', data: wordCountTrend.value });
  }
  if (commentTrend.value?.dates?.length) {
    opts.push({ label: '评论增长趋势', data: commentTrend.value });
  }
  return opts;
});

const commentTrend = ref<any>({
  title: '评论增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增评论',
});

const wordCountTrend = ref<any>({
  title: '字数增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增字数',
});

// ===== 仪表盘数据 =====
async function loadDashboard() {
  try {
    const res = await adminApi.getDashboardStats();
    const d = res.data.data;
    if (d?.stats) {
      stats.users = d.stats.users || 0;
      stats.novels = d.stats.novels || 0;
      stats.comments = d.stats.comments || 0;
      stats.forums = d.stats.forums || 0;
    }
    const ut = d?.user_trend;
    if (ut) {
      userTrend.value = { title: '用户增长趋势（近7天）', dates: ut.dates || [], values: ut.new_users || [], seriesName: '新增用户' };
    }
    const ct = d?.chapter_trend;
    if (ct) {
      chapterTrend.value = { title: '章节增长趋势（近7天）', dates: ct.dates || [], values: ct.counts || [], seriesName: '新增章节' };
    }
    const cmt = d?.comment_trend;
    if (cmt) {
      commentTrend.value = { title: '评论增长趋势（近7天）', dates: cmt.dates || [], values: cmt.counts || [], seriesName: '新增评论' };
    }
    const wct = d?.word_count_trend;
    if (wct) {
      wordCountTrend.value = { title: '字数增长趋势（近7天）', dates: wct.dates || [], values: wct.counts || [], seriesName: '每日新增', secondValues: wct.cumulative || [], secondName: '累计总字数' };
    }
    const tn = d?.top_novels;
    if (tn) {
      topNovelBars.value = { title: 'Top 6 作品字数对比', labels: tn.map((n: any) => n.title), values: tn.map((n: any) => n.total_words), seriesName: '字数' };
    }
    const cd = d?.category_distribution;
    if (cd) {
      categoryPieData.value = { title: '分类分布', data: cd.map((c: any) => ({ name: c.name, value: c.count })) };
    }
  } catch { /* ignore */ }
}

async function refreshAll() {
  refreshing.value = true;
  await loadDashboard();
  refreshing.value = false;
}

onMounted(() => { loadDashboard(); checkFiling(); });
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
    radial-gradient(ellipse at 20% 20%, rgba(99, 102, 241, 0.08) 0%, transparent 50%),
    radial-gradient(ellipse at 80% 60%, rgba(139, 92, 246, 0.06) 0%, transparent 50%),
    radial-gradient(ellipse at 50% 80%, rgba(52, 211, 153, 0.04) 0%, transparent 50%);
  pointer-events: none;
  z-index: 0;
}

[data-theme="light"] .dashboard-bg::before {
  background:
    radial-gradient(ellipse at 20% 20%, rgba(99, 102, 241, 0.04) 0%, transparent 50%),
    radial-gradient(ellipse at 80% 60%, rgba(139, 92, 246, 0.03) 0%, transparent 50%),
    radial-gradient(ellipse at 50% 80%, rgba(52, 211, 153, 0.02) 0%, transparent 50%);
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
  background: linear-gradient(135deg, #818cf8, #c084fc, #34d399);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  text-shadow: none;
  filter: drop-shadow(0 0 18px rgba(129, 140, 248, 0.35));
}

.dashboard-subtitle {
  color: var(--text-light);
  font-size: 0.95rem;
  margin: 0 0 16px;
}

.refresh-btn {
  background: rgba(255, 255, 255, 0.08) !important;
  border: 1px solid rgba(255, 255, 255, 0.15) !important;
  color: #e2e8f0 !important;
  backdrop-filter: blur(8px);
}

[data-theme="light"] .refresh-btn {
  background: rgba(0, 0, 0, 0.04) !important;
  border-color: rgba(0, 0, 0, 0.1) !important;
  color: var(--text-color) !important;
}

.refresh-btn:hover {
  background: rgba(255, 255, 255, 0.14) !important;
  border-color: rgba(255, 255, 255, 0.25) !important;
}

[data-theme="light"] .refresh-btn:hover {
  background: rgba(0, 0, 0, 0.08) !important;
  border-color: rgba(0, 0, 0, 0.15) !important;
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

/* ===== 管理功能入口 ===== */
.menu-section {
  margin-top: 8px;
}

.menu-section-title {
  font-size: 1.2rem;
  font-weight: 700;
  color: #e2e8f0;
  margin: 0 0 16px;
  text-align: center; /* 与下方居中排布的卡片网格保持同一视觉中轴 */
}

[data-theme="light"] .menu-section-title {
  color: var(--text-color);
}

.admin-menu-grid {
  /* flex 弹性换行 + 居中：菜单数量任意（如 21 项）时，末行不足一列也整体居中，
     避免左侧堆叠的视觉失衡（早期为左对齐，现按需求改为居中） */
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 16px;
}

@media (max-width: 1000px) {
  .admin-menu-grid { gap: 14px; }
}
@media (max-width: 760px) {
  .admin-menu-grid { gap: 12px; }
}
@media (max-width: 480px) {
  .admin-menu-grid { gap: 10px; }
}

.admin-menu-card {
  display: flex;
  align-items: center;
  gap: 14px;
  width: calc(25% - 12px); /* 4 列（gap 16×3=48，(100%-48)/4=25%-12px） */
  box-sizing: border-box;
  padding: 18px 16px;
  background: rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 14px;
  cursor: pointer;
  transition: transform 0.25s ease, box-shadow 0.25s ease, border-color 0.25s ease;
}

@media (max-width: 1000px) {
  .admin-menu-card { width: calc(33.333% - 10px); } /* 3 列 */
}
@media (max-width: 760px) {
  .admin-menu-card { width: calc(50% - 6px); } /* 2 列 */
}
@media (max-width: 480px) {
  .admin-menu-card { width: 100%; } /* 1 列 */
}

[data-theme="light"] .admin-menu-card {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}

.admin-menu-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.25);
  border-color: rgba(255, 255, 255, 0.2);
}

[data-theme="light"] .admin-menu-card:hover {
  box-shadow: var(--shadow-lg);
  border-color: var(--border-color);
}

.amc-icon {
  width: 46px;
  height: 46px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  flex-shrink: 0;
}

.amc-body {
  flex: 1;
  min-width: 0;
}

.amc-title {
  font-weight: 600;
  font-size: 0.98rem;
  color: #e2e8f0;
}

[data-theme="light"] .amc-title {
  color: var(--text-color);
}

.amc-desc {
  font-size: 0.78rem;
  color: rgba(255, 255, 255, 0.45);
  margin-top: 3px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

[data-theme="light"] .amc-desc {
  color: var(--text-light);
}

.amc-arrow {
  color: rgba(255, 255, 255, 0.35);
  flex-shrink: 0;
}

[data-theme="light"] .amc-arrow {
  color: var(--text-light);
}

/* ====== 移动端管理员仪表盘适配 ====== */
@media (max-width: 768px) {
  .dashboard-container {
    padding: 20px 12px 40px;
  }
  .dashboard-hero {
    margin-bottom: 24px;
  }
  .dashboard-main-title {
    font-size: 1.5rem;
  }
  .dashboard-subtitle {
    font-size: 0.85rem;
  }
  .hero-buttons {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
  }
  .hero-buttons .el-button {
    flex: 1;
    min-width: 120px;
    margin-left: 0 !important;
  }
  .stats-row .el-col,
  .chart-row .el-col {
    margin-bottom: 14px;
  }
  .stats-row .glass-stat-card {
    padding: 14px;
  }
  .menu-section {
    padding: 0 2px;
  }
}
</style>
