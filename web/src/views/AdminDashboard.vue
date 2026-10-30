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

      <!-- 第四行：热力图（中国省级访问热力 + 星期×小时 活跃热力） -->
      <el-row :gutter="20" class="chart-row">
        <el-col :xs="24" :lg="12">
          <div class="hm-card">
            <h4 class="chart-title">访问来源地域热力（近 30 天）</h4>
            <ChinaGeoMap
              :data="provinceHeat"
              note="基于 ip2region 离线库按省级着色；仅统计境内访问，内网/保留地址与境外不计入省份排名。"
            />
          </div>
        </el-col>
        <el-col :xs="24" :lg="12">
          <div class="hm-card">
            <AdminActiveHoursHeatmap :matrix="activeHours" />
          </div>
        </el-col>
      </el-row>

      <!-- 第五行：客户端画像（设备/OS/浏览器分布，结构化 UA 解析） -->
      <el-row :gutter="20" class="chart-row">
        <el-col :xs="24" :lg="8" v-for="(grp, gi) in clientGroups" :key="gi">
          <div class="hm-card">
            <h4 class="chart-title">{{ grp.title }}</h4>
            <div v-if="!grp.items || grp.items.length === 0" class="client-empty">近 30 天暂无数据</div>
            <div v-for="(it, i) in grp.items" :key="i" class="client-bar-row">
              <span class="client-bar-label">{{ grp.key === 'device' ? deviceName(it.name) : it.name }}</span>
              <div class="client-bar-track">
                <div class="client-bar-fill" :style="{ width: barPct(grp.items, it.cnt) }"></div>
              </div>
              <span class="client-bar-count">{{ it.cnt }}</span>
            </div>
          </div>
        </el-col>
      </el-row>
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
import ChinaGeoMap from '@/components/ChinaGeoMap.vue';
import AdminActiveHoursHeatmap from '@/components/admin/AdminActiveHoursHeatmap.vue';

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

// 热力图数据
const provinceHeat = ref<any[]>([]); // [{name:'广东省', value: N}] → ChinaGeoMap 省级着色
const activeHours = ref<number[][]>([]); // 7×24 星期×小时 互动矩阵

// 客户端画像（结构化 UA 解析：OS / 浏览器 / 设备类型）
const clientData = ref<{ os: any[]; browser: any[]; device: any[] }>({ os: [], browser: [], device: [] });
const clientGroups = computed(() => [
  { key: 'os', title: '操作系统分布（近30天）', items: clientData.value.os || [] },
  { key: 'browser', title: '浏览器分布（近30天）', items: clientData.value.browser || [] },
  { key: 'device', title: '设备类型分布（近30天）', items: clientData.value.device || [] },
]);
function deviceName(n: string) {
  return ({ desktop: '桌面端', phone: '手机端', tablet: '平板', unknown: '未知' } as Record<string, string>)[n] || n;
}
function barPct(items: any[], cnt: number) {
  const max = Math.max(1, ...items.map((x: any) => Number(x.cnt) || 0));
  return Math.round((Number(cnt) / max) * 100) + '%';
}

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
    const ph = d?.province_heat;
    if (Array.isArray(ph)) provinceHeat.value = ph;
    const ah = d?.active_hours;
    if (Array.isArray(ah)) activeHours.value = ah;
    if (d?.client) clientData.value = d.client;
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

/* ===== 热力图卡片 ===== */
.hm-card {
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, rgba(0,0,0,0.06));
  border-radius: 16px;
  padding: 18px 20px;
  box-shadow: 0 4px 20px rgba(0,0,0,0.04);
  height: 100%;
}
[data-theme="dark"] .hm-card {
  background: rgba(30,41,59,0.8);
  border-color: rgba(71,85,105,0.3);
}
.hm-card .chart-title { margin-bottom: 6px; }

/* ===== 客户端分布条形 ===== */
.client-empty { color: var(--text-light, #94a3b8); font-size: 0.82rem; padding: 8px 0; }
.client-bar-row { display: flex; align-items: center; gap: 8px; margin: 7px 0; font-size: 0.82rem; }
.client-bar-label { width: 56px; flex-shrink: 0; color: var(--text-color, #1e293b); text-align: right; }
.client-bar-track { flex: 1; height: 10px; background: rgba(99,102,241,0.12); border-radius: 6px; overflow: hidden; }
.client-bar-fill { height: 100%; background: linear-gradient(90deg, #6366f1, #8b5cf6); border-radius: 6px; transition: width 0.4s ease; }
.client-bar-count { width: 48px; flex-shrink: 0; color: var(--text-light, #94a3b8); text-align: left; }

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
