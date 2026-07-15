<template>
  <div class="dashboard-charts">
    <!-- ═══ 统一图表卡片：指标▾ + 折线/柱状▾ ═══ -->
    <div class="chart-card glass-chart-card" v-if="activeData">
      <div class="chart-card-header">
        <!-- 指标选择 -->
        <el-dropdown v-if="allMetrics.length > 1" trigger="click" @command="(i: number) => currentIdx = i">
          <h4 class="chart-title chart-title-clickable">
            {{ allMetrics[currentIdx]?.label || '数据' }}
            <span class="chart-arrow">▾</span>
          </h4>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item
                v-for="(m, i) in allMetrics"
                :key="i"
                :command="i"
                :class="{ active: currentIdx === i }"
              >
                {{ m.label }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <h4 v-else class="chart-title">{{ allMetrics[0]?.label || '数据' }}</h4>

        <!-- 图表类型切换 -->
        <el-dropdown v-if="customizable" trigger="click" @command="(t: string) => chartType = t">
          <span class="chart-dropdown-trigger">
            {{ chartType === 'line' ? '📈 折线 ▾' : '📊 柱状 ▾' }}
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="line" :class="{ active: chartType === 'line' }">📈 折线图</el-dropdown-item>
              <el-dropdown-item command="bar" :class="{ active: chartType === 'bar' }">📊 柱状图</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
      <v-chart :option="chartOption" autoresize class="chart-canvas" />
    </div>

    <!-- ═══ 饼图 ═══ -->
    <div class="chart-card glass-chart-card" v-if="categoryPie && categoryPie.data">
      <h4 class="chart-title">{{ categoryPie.title || '分类分布' }}</h4>
      <v-chart :option="pieOpt" autoresize class="chart-canvas" />
    </div>

    <!-- ═══ 仪表盘：完本率 ═══ -->
    <div class="chart-card glass-chart-card" v-if="completionGauge !== undefined && completionGauge !== null">
      <h4 class="chart-title">完本率</h4>
      <v-chart :option="completionOpt" autoresize class="chart-canvas chart-gauge" />
    </div>

    <!-- ═══ 仪表盘：评分 ═══ -->
    <div class="chart-card glass-chart-card" v-if="ratingGauge !== undefined && ratingGauge !== null">
      <h4 class="chart-title">平均评分</h4>
      <v-chart :option="ratingOpt" autoresize class="chart-canvas chart-gauge" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import VChart from 'vue-echarts';
import { useThemeStore } from '@/stores/theme';

const themeStore = useThemeStore();

// ── 类型 ──

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

interface MetricItem {
  label: string;
  data: TrendData | BarData;
}

function isTrend(d: TrendData | BarData): d is TrendData {
  return 'dates' in d && Array.isArray((d as TrendData).dates);
}

// ── Props ──

const props = withDefaults(defineProps<{
  metrics?: MetricItem[];
  visitorTrend?: TrendData | null;
  novelBars?: BarData | null;
  categoryPie?: PieData | null;
  completionGauge?: number | null;
  ratingGauge?: number | null;
  customizable?: boolean;
}>(), {
  customizable: true,
});

// ── 合并所有指标 ──

const allMetrics = computed<MetricItem[]>(() => {
  if (props.metrics && props.metrics.length > 0) return props.metrics;
  const result: MetricItem[] = [];
  if (props.visitorTrend?.dates?.length) result.push({ label: props.visitorTrend.title || '趋势', data: props.visitorTrend });
  if (props.novelBars?.labels?.length) result.push({ label: props.novelBars.title || '对比', data: props.novelBars });
  return result;
});

const currentIdx = ref(0);
const chartType = ref('line');

// 切换指标时自动选择合适图表类型
watch(currentIdx, (idx) => {
  const item = allMetrics.value[idx];
  if (item) {
    chartType.value = isTrend(item.data) ? 'line' : 'bar';
  }
});

const activeData = computed(() => allMetrics.value[currentIdx.value]?.data || null);

// ── 主题 ──

const baseTextColor = computed(() => themeStore.isDark ? '#cbd5e1' : '#475569');
const gridLineColor = computed(() => themeStore.isDark ? 'rgba(71,85,105,0.3)' : 'rgba(0,0,0,0.08)');
const tooltipBgColor = computed(() => themeStore.isDark ? 'rgba(15,23,42,0.9)' : 'rgba(255,255,255,0.95)');
const tooltipBorderColor = computed(() => themeStore.isDark ? '#475569' : '#e2e8f0');
const tooltipTextColor = computed(() => themeStore.isDark ? '#e2e8f0' : '#1e293b');
const axisLineColor = computed(() => themeStore.isDark ? '#475569' : '#cbd5e1');
const colors = ['#818cf8', '#34d399', '#fbbf24', '#f472b6', '#38bdf8', '#a78bfa'];

// ── 数据归一化 ──

interface Normalized {
  xLabels: string[];
  values: number[];
  seriesName: string;
  secondValues: number[];
  secondName: string;
}

function normalize(d: TrendData | BarData): Normalized {
  if (isTrend(d)) {
    return {
      xLabels: d.dates,
      values: d.values,
      seriesName: d.seriesName || '数值',
      secondValues: d.secondValues || [],
      secondName: d.secondName || '',
    };
  }
  return {
    xLabels: d.labels,
    values: d.values,
    seriesName: d.seriesName || '数值',
    secondValues: [],
    secondName: '',
  };
}

// ── 折线图 option ──

function lineOption(n: Normalized) {
  const s: any[] = [{
    name: n.seriesName, type: 'line', smooth: true,
    symbol: 'circle', symbolSize: 8,
    lineStyle: { color: colors[0], width: 3 },
    itemStyle: { color: colors[0] },
    areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
      colorStops: [{ offset: 0, color: 'rgba(129,140,248,0.35)' }, { offset: 1, color: 'rgba(129,140,248,0.02)' }] } },
    data: n.values,
    animationDuration: 1500,
  }];
  if (n.secondValues.length) {
    s.push({
      name: n.secondName || '数值2', type: 'line', smooth: true,
      symbol: 'circle', symbolSize: 8,
      lineStyle: { color: colors[1], width: 3 }, itemStyle: { color: colors[1] },
      areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
        colorStops: [{ offset: 0, color: 'rgba(52,211,153,0.3)' }, { offset: 1, color: 'rgba(52,211,153,0.02)' }] } },
      data: n.secondValues,
      animationDuration: 1500,
    });
  }
  return {
    backgroundColor: 'transparent',
    grid: { left: '3%', right: '4%', bottom: '3%', top: '12%', containLabel: true },
    tooltip: { trigger: 'axis', backgroundColor: tooltipBgColor.value, borderColor: tooltipBorderColor.value, textStyle: { color: tooltipTextColor.value } },
    legend: { textStyle: { color: baseTextColor.value }, top: 0 },
    xAxis: { type: 'category', data: n.xLabels, axisLine: { lineStyle: { color: axisLineColor.value } }, axisLabel: { color: baseTextColor.value } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: gridLineColor.value } }, axisLabel: { color: baseTextColor.value } },
    series: s,
  };
}

// ── 柱状图 option ──

function barOption(n: Normalized) {
  const s: any[] = [{
    name: n.seriesName, type: 'bar',
    data: n.values.map((v, i) => ({
      value: v,
      itemStyle: {
        color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [{ offset: 0, color: colors[i % colors.length] }, { offset: 1, color: colors[(i + 1) % colors.length] }] },
        borderRadius: [8, 8, 0, 0],
      },
    })),
    barMaxWidth: 50,
    animationDuration: 1500,
  }];
  if (n.secondValues.length) {
    s.push({
      name: n.secondName || '数值2', type: 'bar',
      data: n.secondValues.map((v, i) => ({
        value: v,
        itemStyle: {
          color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [{ offset: 0, color: colors[1] }, { offset: 1, color: colors[2] }] },
          borderRadius: [8, 8, 0, 0],
        },
      })),
      barMaxWidth: 50,
      animationDuration: 1500,
    });
  }
  return {
    backgroundColor: 'transparent',
    grid: { left: '3%', right: '4%', bottom: '3%', top: '12%', containLabel: true },
    tooltip: { trigger: 'axis', backgroundColor: tooltipBgColor.value, borderColor: tooltipBorderColor.value, textStyle: { color: tooltipTextColor.value } },
    legend: { textStyle: { color: baseTextColor.value }, top: 0 },
    xAxis: { type: 'category', data: n.xLabels, axisLine: { lineStyle: { color: axisLineColor.value } }, axisLabel: { color: baseTextColor.value, rotate: n.xLabels.length > 6 ? 15 : 0 } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: gridLineColor.value } }, axisLabel: { color: baseTextColor.value } },
    series: s,
  };
}

// ── 饼图 ──

function pieOption(pie: PieData) {
  const b = themeStore.isDark ? 'rgba(15,23,42,0.6)' : 'rgba(255,255,255,0.8)';
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', backgroundColor: tooltipBgColor.value, borderColor: tooltipBorderColor.value, textStyle: { color: tooltipTextColor.value } },
    legend: { orient: 'vertical', right: '5%', top: 'center', textStyle: { color: baseTextColor.value } },
    series: [{
      type: 'pie', radius: ['45%', '75%'], center: ['40%', '50%'], roseType: 'radius',
      itemStyle: { borderRadius: 6, borderColor: b, borderWidth: 3 },
      label: { color: baseTextColor.value },
      emphasis: { label: { fontSize: 16, fontWeight: 'bold' } },
      data: pie.data,
      animationDuration: 1500, animationType: 'scale',
    }],
  };
}

// ── 仪表盘 ──

function gaugeOption(value: number, max: number, name: string, color: string) {
  return {
    backgroundColor: 'transparent',
    tooltip: { formatter: `{a} : {c}/${max === 1 ? '100%' : max}`, backgroundColor: tooltipBgColor.value, borderColor: tooltipBorderColor.value, textStyle: { color: tooltipTextColor.value } },
    series: [{
      name, type: 'gauge', startAngle: 210, endAngle: -30, center: ['50%', '55%'], radius: '85%',
      min: 0, max, splitNumber: 10,
      axisLine: { lineStyle: { width: 18, color: [[value / max, color], [1, themeStore.isDark ? '#334155' : '#e5e7eb']] } },
      pointer: { length: '70%', width: 6, itemStyle: { color: 'auto' } },
      axisTick: { distance: -18, length: 8, lineStyle: { width: 2 } },
      splitLine: { distance: -22, length: 20, lineStyle: { width: 4 } },
      axisLabel: { distance: 32, color: baseTextColor.value, fontSize: 11 },
      detail: {
        valueAnimation: true,
        formatter: max === 1 ? '{value}%' : '{value}',
        color: themeStore.isDark ? '#e2e8f0' : '#1e293b',
        fontSize: 28, fontWeight: 'bold', offsetCenter: [0, '80%'],
      },
      data: [{ value, name }],
    }],
  };
}

// ── 计算 ──

const chartOption = computed(() => {
  const d = activeData.value;
  if (!d) return {};
  const n = normalize(d);
  return chartType.value === 'line' ? lineOption(n) : barOption(n);
});

const pieOpt = computed(() => props.categoryPie ? pieOption(props.categoryPie) : {});
const completionOpt = computed(() => gaugeOption(props.completionGauge!, 1, '完本率', '#10b981'));
const ratingOpt = computed(() => gaugeOption(props.ratingGauge!, 5, '评分', '#f59e0b'));
</script>

<style scoped>
.dashboard-charts { display: contents; }

.chart-card {
  background: #fff; border-radius: 16px; padding: 20px;
  border: 1px solid rgba(0,0,0,0.06);
  box-shadow: 0 4px 20px rgba(0,0,0,0.04);
  transition: box-shadow 0.2s; margin-bottom: 20px; height: 100%;
}
.chart-card:hover { box-shadow: 0 8px 30px rgba(0,0,0,0.08); }
[data-theme="dark"] .chart-card {
  background: rgba(30,41,59,0.8); border-color: rgba(71,85,105,0.3);
}

.chart-card-header {
  display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;
}

.chart-title {
  font-size: 1rem; font-weight: 700; margin: 0;
  color: var(--text-primary, #1e293b);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}

.chart-title-clickable {
  cursor: pointer; user-select: none;
  display: flex; align-items: center; gap: 4px;
  transition: color 0.2s;
}
.chart-title-clickable:hover { color: var(--primary-color, #409eff); }
.chart-arrow { font-size: 0.7rem; opacity: 0.5; margin-left: 2px; }

.chart-dropdown-trigger {
  font-size: 0.8rem; color: var(--primary-color, #409eff);
  cursor: pointer; user-select: none;
  padding: 2px 10px; border: 1px solid var(--primary-color, #409eff);
  border-radius: 12px; white-space: nowrap; font-weight: 500;
  transition: all 0.2s;
}
.chart-dropdown-trigger:hover { background: var(--primary-color, #409eff); color: #fff; }

.chart-canvas { width: 100%; height: 280px; }
.chart-gauge { height: 240px; }
</style>
