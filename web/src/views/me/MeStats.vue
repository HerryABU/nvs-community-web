<template>
  <div class="me-stats-page">
    <h2 class="section-title">创作数据趋势</h2>
    <div class="author-charts" v-if="hasChartData">
      <el-row :gutter="20">
        <el-col :xs="24" :md="12" v-for="(m, i) in metrics" :key="i">
          <DashboardCharts :metrics="[m]" />
        </el-col>
      </el-row>
    </div>
    <el-empty v-else description="暂无趋势数据" />
  </div>
</template>

<script setup lang="ts">
import { computed, inject, type Ref } from 'vue';
import DashboardCharts from '@/components/DashboardCharts.vue';

const profile = inject<Ref<any>>('meProfile');

const chapterTrend = computed(() => {
  const t = profile?.value?.chapter_trend;
  return t ? { title: '章节增长趋势', dates: t.dates || [], values: t.counts || [], seriesName: '新增章节' } : null;
});
const wordCountTrend = computed(() => {
  const t = profile?.value?.word_count_trend;
  return t ? { title: '字数增长趋势', dates: t.dates || [], values: t.counts || [], seriesName: '新增字数' } : null;
});
const commentTrend = computed(() => {
  const t = profile?.value?.comment_trend;
  return t ? { title: '评论趋势', dates: t.dates || [], values: t.counts || [], seriesName: '评论数' } : null;
});

const metrics = computed(() => {
  const opts: any[] = [];
  if (chapterTrend.value) opts.push({ label: '章节增长趋势', data: chapterTrend.value });
  if (wordCountTrend.value) opts.push({ label: '字数增长趋势', data: wordCountTrend.value });
  if (commentTrend.value) opts.push({ label: '评论趋势', data: commentTrend.value });
  return opts;
});

const hasChartData = computed(() => metrics.value.length > 0);
</script>

<style scoped>
.section-title { font-size: 1.15rem; margin-bottom: 12px; color: var(--text-color); }
.author-charts { margin-bottom: 24px; }
</style>
