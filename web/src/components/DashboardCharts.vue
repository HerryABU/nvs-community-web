<template>
  <div class="dashboard-charts">
    <!-- 折线图：访问/新增趋势 -->
    <div class="chart-card glass-chart-card" v-if="visitorTrend && visitorTrend.dates">
      <h4 class="chart-title">{{ visitorTrend.title || '趋势图' }}</h4>
      <v-chart :option="lineOption" autoresize class="chart-canvas" />
    </div>

    <!-- 柱状图：作品对比 -->
    <div class="chart-card glass-chart-card" v-if="novelBars && novelBars.labels">
      <h4 class="chart-title">{{ novelBars.title || '作品对比' }}</h4>
      <v-chart :option="barOption" autoresize class="chart-canvas" />
    </div>

    <!-- 饼图：分类分布 -->
    <div class="chart-card glass-chart-card" v-if="categoryPie && categoryPie.data">
      <h4 class="chart-title">{{ categoryPie.title || '分类分布' }}</h4>
      <v-chart :option="pieOption" autoresize class="chart-canvas" />
    </div>

    <!-- 仪表盘：完本率 -->
    <div class="chart-card glass-chart-card" v-if="completionGauge !== undefined && completionGauge !== null">
      <h4 class="chart-title">完本率</h4>
      <v-chart :option="completionGaugeOption" autoresize class="chart-canvas chart-gauge" />
    </div>

    <!-- 仪表盘：评分 -->
    <div class="chart-card glass-chart-card" v-if="ratingGauge !== undefined && ratingGauge !== null">
      <h4 class="chart-title">平均评分</h4>
      <v-chart :option="ratingGaugeOption" autoresize class="chart-canvas chart-gauge" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import VChart from 'vue-echarts';
import { useThemeStore } from '@/stores/theme';

const themeStore = useThemeStore();

interface TrendData {
  title?: string;
  dates: string[];
  values: number[];
  seriesName?: string;
  secondValues?: number[];
  secondName?: string;
}

interface BarData {
  title?: string;
  labels: string[];
  values: number[];
  seriesName?: string;
}

interface PieData {
  title?: string;
  data: { name: string; value: number }[];
}

const props = defineProps<{
  visitorTrend?: TrendData | null;
  novelBars?: BarData | null;
  categoryPie?: PieData | null;
  completionGauge?: number | null;
  ratingGauge?: number | null;
}>();

const baseTextColor = computed(() => themeStore.isDark ? '#cbd5e1' : '#475569');
const gridLineColor = computed(() => themeStore.isDark ? 'rgba(71,85,105,0.3)' : 'rgba(0,0,0,0.08)');
const tooltipBgColor = computed(() => themeStore.isDark ? 'rgba(15,23,42,0.9)' : 'rgba(255,255,255,0.95)');
const tooltipBorderColor = computed(() => themeStore.isDark ? '#475569' : '#e2e8f0');
const tooltipTextColor = computed(() => themeStore.isDark ? '#e2e8f0' : '#1e293b');
const axisLineColor = computed(() => themeStore.isDark ? '#475569' : '#cbd5e1');

function makeLineOption(trend: TrendData) {
  const series: any[] = [
    {
      name: trend.seriesName || '数值',
      type: 'line',
      smooth: true,
      symbol: 'circle',
      symbolSize: 8,
      lineStyle: { color: '#818cf8', width: 3 },
      itemStyle: { color: '#818cf8' },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: 'rgba(129, 140, 248, 0.35)' },
            { offset: 1, color: 'rgba(129, 140, 248, 0.02)' },
          ],
        },
      },
      data: trend.values,
      animationDuration: 1500,
    },
  ];

  if (trend.secondValues && trend.secondValues.length > 0) {
    series.push({
      name: trend.secondName || '数值2',
      type: 'line',
      smooth: true,
      symbol: 'circle',
      symbolSize: 8,
      lineStyle: { color: '#34d399', width: 3 },
      itemStyle: { color: '#34d399' },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: 'rgba(52, 211, 153, 0.3)' },
            { offset: 1, color: 'rgba(52, 211, 153, 0.02)' },
          ],
        },
      },
      data: trend.secondValues,
      animationDuration: 1500,
    });
  }

  return {
    backgroundColor: 'transparent',
    grid: { left: '3%', right: '4%', bottom: '3%', top: '12%', containLabel: true },
    tooltip: { trigger: 'axis', backgroundColor: tooltipBgColor.value, borderColor: tooltipBorderColor.value, textStyle: { color: tooltipTextColor.value } },
    legend: { textStyle: { color: baseTextColor.value }, top: 0 },
    xAxis: { type: 'category', data: trend.dates, axisLine: { lineStyle: { color: axisLineColor.value } }, axisLabel: { color: baseTextColor.value } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: gridLineColor.value } }, axisLabel: { color: baseTextColor.value } },
    series,
  };
}

function makeBarOption(bars: BarData) {
  return {
    backgroundColor: 'transparent',
    grid: { left: '3%', right: '4%', bottom: '3%', top: '12%', containLabel: true },
    tooltip: { trigger: 'axis', backgroundColor: tooltipBgColor.value, borderColor: tooltipBorderColor.value, textStyle: { color: tooltipTextColor.value } },
    xAxis: { type: 'category', data: bars.labels, axisLine: { lineStyle: { color: axisLineColor.value } }, axisLabel: { color: baseTextColor.value, rotate: bars.labels.length > 4 ? 15 : 0 } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: gridLineColor.value } }, axisLabel: { color: baseTextColor.value } },
    series: [{
      name: bars.seriesName || '数值',
      type: 'bar',
      data: bars.values.map((v, i) => ({
        value: v,
        itemStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: i % 2 === 0 ? '#818cf8' : '#a78bfa' },
              { offset: 1, color: i % 2 === 0 ? '#6366f1' : '#8b5cf6' },
            ],
          },
          borderRadius: [8, 8, 0, 0],
        },
      })),
      barMaxWidth: 60,
      animationDuration: 1500,
    }],
  };
}

function makePieOption(pie: PieData) {
  const pieBorderColor = themeStore.isDark ? 'rgba(15,23,42,0.6)' : 'rgba(255,255,255,0.8)';
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', backgroundColor: tooltipBgColor.value, borderColor: tooltipBorderColor.value, textStyle: { color: tooltipTextColor.value } },
    legend: { orient: 'vertical', right: '5%', top: 'center', textStyle: { color: baseTextColor.value } },
    series: [{
      type: 'pie',
      radius: ['45%', '75%'],
      center: ['40%', '50%'],
      roseType: 'radius',
      itemStyle: { borderRadius: 6, borderColor: pieBorderColor, borderWidth: 3 },
      label: { color: baseTextColor.value },
      emphasis: { label: { fontSize: 16, fontWeight: 'bold' } },
      data: pie.data,
      animationDuration: 1500,
      animationType: 'scale',
    }],
  };
}

function makeGaugeOption(value: number, max: number, name: string, color: string) {
  return {
    backgroundColor: 'transparent',
    tooltip: { formatter: `{a} : {c}/${max === 1 ? '100%' : max}`, backgroundColor: tooltipBgColor.value, borderColor: tooltipBorderColor.value, textStyle: { color: tooltipTextColor.value } },
    series: [{
      name,
      type: 'gauge',
      startAngle: 210,
      endAngle: -30,
      center: ['50%', '55%'],
      radius: '85%',
      min: 0,
      max,
      splitNumber: max === 1 ? 5 : 5,
      axisLine: {
        show: true,
        lineStyle: {
          width: 18,
          color: [
            [value / max, color],
            [1, themeStore.isDark ? 'rgba(71,85,105,0.3)' : 'rgba(0,0,0,0.08)'],
          ],
        },
      },
      pointer: { icon: 'path://M12.8,0.7l12,40.1H0.7L12.8,0.7z', length: '60%', width: 6, itemStyle: { color: 'auto' } },
      axisTick: { distance: -18, length: 6, lineStyle: { color: baseTextColor.value, width: 1 } },
      splitLine: { distance: -22, length: 14, lineStyle: { color: baseTextColor.value, width: 2 } },
      axisLabel: { color: baseTextColor.value, distance: 28, fontSize: 10 },
      anchor: { show: true, showAbove: true, size: 18, itemStyle: { borderWidth: 2 } },
      title: { show: true, offsetCenter: [0, '75%'], color: baseTextColor.value, fontSize: 13 },
      detail: {
        valueAnimation: true,
        fontSize: 20,
        fontWeight: 'bold',
        color: themeStore.isDark ? '#e2e8f0' : '#1e293b',
        offsetCenter: [0, '55%'],
        formatter: max === 1 ? '{value}%' : '{value}',
      },
      data: [{ value: max === 1 ? +(value * 100).toFixed(1) : value }],
      animationDuration: 1500,
    }],
  };
}

const lineOption = computed(() => {
  if (!props.visitorTrend) return {};
  return makeLineOption(props.visitorTrend);
});

const barOption = computed(() => {
  if (!props.novelBars) return {};
  return makeBarOption(props.novelBars);
});

const pieOption = computed(() => {
  if (!props.categoryPie) return {};
  return makePieOption(props.categoryPie);
});

const completionGaugeOption = computed(() => {
  if (props.completionGauge === null || props.completionGauge === undefined) return {};
  return makeGaugeOption(props.completionGauge, 1, '完本率', '#34d399');
});

const ratingGaugeOption = computed(() => {
  if (props.ratingGauge === null || props.ratingGauge === undefined) return {};
  return makeGaugeOption(props.ratingGauge, 5, '评分', '#f59e0b');
});
</script>

<style scoped>
.dashboard-charts {
  display: contents;
}

.chart-card {
  border-radius: 16px;
  padding: 20px;
  overflow: hidden;
}

.glass-chart-card {
  background: rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

[data-theme="light"] .glass-chart-card {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}

.chart-title {
  font-size: 0.95rem;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.75);
  margin: 0 0 12px;
}

[data-theme="light"] .chart-title {
  color: var(--text-color);
}

.chart-canvas {
  width: 100%;
  height: 320px;
}

.chart-gauge {
  height: 260px;
}
</style>
