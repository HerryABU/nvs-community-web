<template>
  <div class="page-container author-home" v-loading="loading">
    <template v-if="author">
      <!-- 作者信息头 -->
      <div class="author-header">
        <div class="author-avatar-wrap">
          <el-avatar :size="80" :src="author.avatar_url || ''">
            {{ author.nickname?.[0] || author.username?.[0] || 'A' }}
          </el-avatar>
        </div>
        <div class="author-info">
          <h1>{{ author.nickname || author.username }}</h1>
          <p class="author-bio" v-if="author.bio">{{ author.bio }}</p>
          <div class="author-stats">
            <span>作品：{{ data.total_novels || 0 }}</span>
            <span>总字数：{{ (data.total_words || 0).toLocaleString() }}</span>
            <span>总章节：{{ data.total_chapters || 0 }}</span>
            <span>评论：{{ (data.total_comments || 0).toLocaleString() }}</span>
            <el-tag v-if="author.role === 'vip_author'" type="warning" size="small">VIP作者</el-tag>
            <el-tag v-else-if="author.role === 'author'" size="small">作者</el-tag>
          </div>
          <!-- 评分 -->
          <div class="author-rating" v-if="data.avg_rating && data.avg_rating > 0">
            <el-rate v-model="avgRatingDisplay" disabled show-score text-color="#f59e0b" />
          </div>
          <!-- 进入作者大论坛 -->
          <el-button type="primary" text size="small" @click="goForum" style="margin-top:8px">
            进入 {{ author.nickname || author.username }} 的讨论区
          </el-button>
        </div>
      </div>

      <!-- 数据趋势面板（平滑折线图） -->
      <div class="author-charts" v-if="hasChartData">
        <el-row :gutter="20">
          <el-col :xs="24" :md="12">
            <div class="chart-card glass-chart-card">
              <h4 class="chart-title">章节增长趋势（近7天）</h4>
              <v-chart :option="chapterLineOption" autoresize class="chart-canvas" />
            </div>
          </el-col>
          <el-col :xs="24" :md="12">
            <div class="chart-card glass-chart-card">
              <h4 class="chart-title">评论趋势（近7天）</h4>
              <v-chart :option="commentLineOption" autoresize class="chart-canvas" />
            </div>
          </el-col>
        </el-row>
      </div>

      <!-- 作品列表 -->
      <h2 class="section-title">作品列表</h2>
      <div class="novel-list" v-if="data.novels && data.novels.length > 0">
        <div v-for="novel in data.novels" :key="novel.id" class="novel-item" @click="$router.push(`/novel/${novel.id}`)">
          <div class="novel-cover-sm">
            <el-image v-if="novel.cover_url" :src="novel.cover_url" fit="cover" style="width:64px;height:85px;border-radius:4px">
              <template #error><div class="cover-ph">封</div></template>
            </el-image>
            <div v-else class="cover-ph">封</div>
          </div>
          <div class="novel-body">
            <h3>{{ novel.title }}</h3>
            <div class="novel-meta">
              <el-tag v-if="novel.categories && novel.categories.length" v-for="c in novel.categories" :key="c" size="small" style="margin-right:4px">{{ c }}</el-tag>
              <el-tag v-else size="small">{{ novel.category }}</el-tag>
              <span>{{ novel.total_words?.toLocaleString() || 0 }} 字</span>
              <span>{{ novel.total_chapters || 0 }} 章</span>
              <el-tag size="small" :type="novel.status === 'published' ? 'success' : 'info'">{{ novel.status === 'published' ? '已发布' : '草稿' }}</el-tag>
              <el-tag v-if="novel.source_type === 'reprint'" size="small" type="warning">转载</el-tag>
              <el-tag v-else-if="novel.source_type === 'original'" size="small">原创</el-tag>
              <el-tag v-if="novel.creation_method === 'ai'" size="small" type="info">AI创作</el-tag>
              <el-tag v-else-if="novel.creation_method === 'human_ai_assisted'" size="small" type="info">AI辅助</el-tag>
            </div>
            <p class="novel-summary-line">{{ novel.summary?.slice(0, 120) || '暂无简介' }}</p>
          </div>
        </div>
      </div>
      <el-empty v-else description="暂无作品" />

      <!-- 作者评论区 -->
      <h2 class="section-title" style="margin-top:32px">读者留言</h2>
      <!-- 评论区：使用第一个公开作品的 ID -->
      <CommentSection :novel-id="data.novels && data.novels.length > 0 ? data.novels[0].id : 0" :novel-category="''" />
    </template>
    <el-empty v-else-if="!loading" description="作者不存在" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { publicApi } from '@/api/admin';
import CommentSection from '@/components/CommentSection.vue';
import VChart from 'vue-echarts';
import { useThemeStore } from '@/stores/theme';

const route = useRoute();
const router = useRouter();
const themeStore = useThemeStore();

const authorId = ref(Number(route.params.id));
const loading = ref(false);
const author = ref<any>(null);
const data = ref<any>({});

const baseTextColor = computed(() => themeStore.isDark ? '#cbd5e1' : '#475569');
const gridLineColor = computed(() => themeStore.isDark ? 'rgba(71,85,105,0.3)' : 'rgba(0,0,0,0.08)');
const tooltipBgColor = computed(() => themeStore.isDark ? 'rgba(15,23,42,0.9)' : 'rgba(255,255,255,0.95)');
const tooltipBorderColor = computed(() => themeStore.isDark ? '#475569' : '#e2e8f0');
const tooltipTextColor = computed(() => themeStore.isDark ? '#e2e8f0' : '#1e293b');
const axisLineColor = computed(() => themeStore.isDark ? '#475569' : '#cbd5e1');

const avgRatingDisplay = computed(() => {
  const v = data.value.avg_rating;
  if (!v || v <= 0) return 0;
  return Math.min(5, Math.round(v * 2) / 2); // 换算成 0.5 步长
});

const hasChartData = computed(() => {
  return data.value.chapter_trend?.dates?.length > 0 || data.value.comment_trend?.dates?.length > 0;
});

const chapterLineOption = computed(() => {
  const trend = data.value.chapter_trend;
  if (!trend) return {};
  return makeSmoothLineOption(trend.dates || [], trend.counts || [], '新增章节', '#818cf8');
});

const commentLineOption = computed(() => {
  const trend = data.value.comment_trend;
  if (!trend) return {};
  return makeSmoothLineOption(trend.dates || [], trend.counts || [], '评论数', '#34d399');
});

function makeSmoothLineOption(dates: string[], values: number[], seriesName: string, color: string) {
  const rgbaColors: Record<string, [string, string]> = {
    '#818cf8': ['rgba(129,140,248,0.35)', 'rgba(129,140,248,0.02)'],
    '#34d399': ['rgba(52,211,153,0.35)', 'rgba(52,211,153,0.02)'],
  };
  const [areaTop, areaBottom] = rgbaColors[color] || ['rgba(129,140,248,0.35)', 'rgba(129,140,248,0.02)'];
  return {
    backgroundColor: 'transparent',
    grid: { left: '3%', right: '4%', bottom: '3%', top: '12%', containLabel: true },
    tooltip: {
      trigger: 'axis',
      backgroundColor: tooltipBgColor.value,
      borderColor: tooltipBorderColor.value,
      textStyle: { color: tooltipTextColor.value },
    },
    xAxis: {
      type: 'category',
      data: dates,
      axisLine: { lineStyle: { color: axisLineColor.value } },
      axisLabel: { color: baseTextColor.value },
    },
    yAxis: {
      type: 'value',
      splitLine: { lineStyle: { color: gridLineColor.value } },
      axisLabel: { color: baseTextColor.value },
    },
    series: [{
      name: seriesName,
      type: 'line',
      smooth: true,
      symbol: 'circle',
      symbolSize: 8,
      lineStyle: { color, width: 3 },
      itemStyle: { color },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: areaTop },
            { offset: 1, color: areaBottom },
          ],
        },
      },
      data: values,
      animationDuration: 1500,
    }],
  };
}

async function load() {
  loading.value = true;
  try {
    const res = await publicApi.getAuthorProfile(authorId.value);
    if (res.data.code === 0) {
      const d = res.data.data;
      author.value = d.author;
      data.value = d;
    }
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

async function goForum() {
  try {
    const res = await publicApi.getAuthorForum(authorId.value);
    const forumId = res.data.data?.forum?.id;
    if (forumId) router.push(`/forum/${forumId}`);
  } catch {
    // ignore
  }
}

onMounted(load);
</script>

<style scoped>
.author-header {
  display: flex;
  gap: 24px;
  align-items: flex-start;
  margin-bottom: 32px;
  padding: 24px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 1px 6px rgba(0,0,0,0.06);
}

[data-theme="dark"] .author-header {
  background: #1e293b;
  box-shadow: 0 1px 6px rgba(0,0,0,0.25);
}

.author-info h1 {
  font-size: 1.5rem;
  color: var(--primary-color);
  margin-bottom: 6px;
}

.author-bio {
  color: #666;
  margin-bottom: 6px;
  max-width: 500px;
}

[data-theme="dark"] .author-bio {
  color: #94a3b8;
}

.author-stats {
  display: flex;
  gap: 16px;
  font-size: 0.85rem;
  color: var(--text-light);
  flex-wrap: wrap;
  align-items: center;
  margin-bottom: 4px;
}

.author-rating {
  display: flex;
  align-items: center;
  margin-top: 4px;
}

/* 图表区域 */
.author-charts {
  margin-bottom: 24px;
}

.chart-card {
  background: #fff;
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 1px 6px rgba(0,0,0,0.06);
}

[data-theme="dark"] .chart-card {
  background: #1e293b;
  box-shadow: 0 1px 6px rgba(0,0,0,0.25);
}

.chart-title {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-color);
  margin: 0 0 12px 0;
}

.chart-canvas {
  width: 100%;
  height: 280px;
}

.novel-item {
  display: flex;
  gap: 16px;
  padding: 16px;
  background: #fff;
  border-radius: 8px;
  margin-bottom: 8px;
  cursor: pointer;
  transition: background 0.2s;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}

[data-theme="dark"] .novel-item {
  background: #1e293b;
  box-shadow: 0 1px 4px rgba(0,0,0,0.25);
}

.novel-item:hover {
  background: #f5f7fa;
}

[data-theme="dark"] .novel-item:hover {
  background: #334155;
}

.novel-body {
  flex: 1;
}

.novel-body h3 {
  font-size: 1.05rem;
  margin-bottom: 6px;
}

.novel-meta {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
  font-size: 0.8rem;
  color: var(--text-light);
  margin-bottom: 4px;
}

.novel-summary-line {
  font-size: 0.85rem;
  color: #888;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

[data-theme="dark"] .novel-summary-line {
  color: #64748b;
}

.cover-ph {
  width: 64px;
  height: 85px;
  background: #e2e8f0;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #94a3b8;
  font-size: 1.2rem;
}

[data-theme="dark"] .cover-ph {
  background: #334155;
  color: #64748b;
}
</style>
