<template>
  <div class="stats-view">
    <!-- 统计卡片 -->
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

    <!-- 趋势图（多指标切换）+ 柱状图（多指标切换）-->
    <el-row :gutter="20" class="chart-row">
      <el-col :xs="24" :md="12">
        <DashboardCharts :metrics="authorTrendMetrics" />
      </el-col>
      <el-col :xs="24" :md="12">
        <DashboardCharts :metrics="novelBarMetrics" />
      </el-col>
    </el-row>

    <!-- 收藏/阅读（合并切换）+ 评分仪表盘 -->
    <el-row :gutter="20" class="chart-row">
      <el-col :xs="24" :md="12">
        <DashboardCharts :metrics="statBarMetrics" />
      </el-col>
      <el-col :xs="24" :md="12">
        <DashboardCharts :rating-gauge="avgRating" />
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { authorDataVersion } from './authorRefresh';
import { reactive, ref, computed, onMounted, watch } from 'vue';
import { Notebook, DataLine, Files, ChatLineRound } from '@element-plus/icons-vue';
import AnimatedNumber from '@/components/common/AnimatedNumber.vue';
import DashboardCharts from '@/components/admin/DashboardCharts.vue';
import { adminApi } from '@/api/admin';
import { novelApi, type Novel } from '@/api/novel';

// 收视率 / 数据总览视图：统计卡片 + 增长趋势 + 各作品对比 + 收藏阅读 + 评分仪表盘。
// 数据自取自「作者仪表盘」接口 + 我的作品列表（作品维度图表需要作品数据）。
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

const wordCountTrend = ref<any>({
  title: '字数增长趋势（近7天）',
  dates: [],
  values: [],
  seriesName: '新增字数',
});

const bookshelfCount = ref(0);
const totalViews = ref(0);
const avgRating = ref<number | null>(null);

const novels = ref<Novel[]>([]);

const bookshelfBarData = computed(() => ({
  title: '总收藏',
  labels: ['收藏'],
  values: [bookshelfCount.value],
  seriesName: '收藏数',
}));

const viewsBarData = computed(() => ({
  title: '总阅读量',
  labels: ['阅读'],
  values: [totalViews.value],
  seriesName: '阅读量',
}));

// 趋势图：章节增长 / 字数增长 / 评论趋势
const authorTrendMetrics = computed(() => [
  { label: '章节增长趋势', data: chapterTrend.value },
  { label: '字数增长趋势', data: wordCountTrend.value },
  { label: '评论趋势', data: commentTrend.value },
]);

// 柱状/趋势图：作品字数 / 作品章节（可切换折线/柱状）
const novelBarMetrics = computed(() => {
  const wordsData = { ...novelWordBars.value };
  const chaptersData = {
    title: '各作品章节数对比',
    labels: novelWordBars.value.labels,
    values: novels.value.map((n) => n.total_chapters || 0),
    seriesName: '章节数',
  };
  return [
    { label: '各作品字数', data: wordsData },
    { label: '各作品章节数', data: chaptersData },
  ];
});

// 柱状/趋势图：收藏 / 阅读
const statBarMetrics = computed(() => [
  { label: '总收藏', data: bookshelfBarData.value },
  { label: '总阅读', data: viewsBarData.value },
]);

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

      if (d.word_count_trend) {
        wordCountTrend.value = {
          title: '字数增长趋势（近7天）',
          dates: d.word_count_trend.dates || [],
          values: d.word_count_trend.counts || [],
          seriesName: '每日新增',
          secondValues: d.word_count_trend.cumulative || [],
          secondName: '累计总字数',
        };
      }

      bookshelfCount.value = d.bookshelf_count || 0;
      totalViews.value = d.total_views || 0;

      if (d.avg_rating !== undefined) {
        avgRating.value = d.avg_rating;
      }
    }
  } catch {
    // fallback：拿不到仪表盘接口时用作品列表兜底
  }
}

async function loadNovels() {
  try {
    const res = await novelApi.getMyNovels();
    novels.value = res.data.data || [];
    novelWordBars.value = {
      title: '各作品字数对比',
      labels: novels.value.map((n) => (n.title.length > 8 ? n.title.slice(0, 7) + '…' : n.title)),
      values: novels.value.map((n) => n.total_words || 0),
      seriesName: '字数',
    };
    // 仪表盘接口没给作品数时，用作品列表兜底
    if (statsCards[0].value === 0) {
      statsCards[0].value = novels.value.length;
      statsCards[1].value = novels.value.reduce((sum, n) => sum + (n.total_words || 0), 0);
      statsCards[2].value = novels.value.reduce((sum, n) => sum + (n.total_chapters || 0), 0);
    }
  } catch (e) {
    console.error(e);
  }
}

onMounted(() => {
  loadDashboard();
  loadNovels();
});

// 其它视图改动了数据（如回收站恢复作品）→ 重新拉取
watch(authorDataVersion, () => {
  loadDashboard();
  loadNovels();
});
</script>

<style scoped>
@import './statcard.css';

.chart-row {
  margin-bottom: 24px;
}

@media (max-width: 768px) {
  .stats-row .el-col,
  .chart-row .el-col {
    margin-bottom: 14px;
  }
  .stats-row .glass-stat-card {
    padding: 14px;
  }
}
</style>
