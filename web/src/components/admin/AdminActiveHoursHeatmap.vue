<template>
  <div class="active-hours-heatmap">
    <v-chart :option="option" autoresize class="ahh-canvas" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import VChart from 'vue-echarts';
import { useThemeStore } from '@/stores/theme';

// 星期×小时 活跃热力图（7 行 × 24 列）。matrix[weekday][hour] = 互动次数。
// weekday: 0=周日 … 6=周六（与 SQLite strftime('%w') 对齐）。
const props = withDefaults(
  defineProps<{
    matrix?: number[][];
    title?: string;
  }>(),
  { matrix: () => [], title: '活跃时段热力（星期 × 小时）' },
);

const themeStore = useThemeStore();
const days = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];

const option = computed(() => {
  const m = props.matrix || [];
  const data: [number, number, number][] = [];
  let max = 0;
  for (let d = 0; d < 7; d++) {
    const row = m[d] || [];
    for (let h = 0; h < 24; h++) {
      const v = Number(row[h]) || 0;
      if (v > max) max = v;
      data.push([h, d, v]);
    }
  }
  const textColor = themeStore.isDark ? '#cbd5e1' : '#475569';
  const splitColor = themeStore.isDark ? 'rgba(71,85,105,0.3)' : 'rgba(0,0,0,0.06)';
  return {
    backgroundColor: 'transparent',
    title: { text: props.title, left: 0, top: 0, textStyle: { color: textColor, fontSize: 14, fontWeight: 600 } },
    tooltip: {
      position: 'top',
      backgroundColor: themeStore.isDark ? 'rgba(15,23,42,0.92)' : 'rgba(255,255,255,0.96)',
      borderColor: themeStore.isDark ? '#475569' : '#e2e8f0',
      textStyle: { color: themeStore.isDark ? '#e2e8f0' : '#1e293b' },
      formatter: (p: any) => `${days[p.value[1]]} ${p.value[0]}:00<br/>互动次数：<b>${p.value[2]}</b>`,
    },
    grid: { left: 48, right: 16, top: 40, bottom: 56, containLabel: false },
    xAxis: {
      type: 'category',
      data: Array.from({ length: 24 }, (_, i) => `${i}`),
      splitArea: { show: true },
      axisLine: { lineStyle: { color: splitColor } },
      axisLabel: { color: textColor, fontSize: 11 },
      name: '时',
      nameTextStyle: { color: textColor },
    },
    yAxis: {
      type: 'category',
      data: days,
      splitArea: { show: true },
      axisLine: { lineStyle: { color: splitColor } },
      axisLabel: { color: textColor, fontSize: 11 },
    },
    visualMap: {
      min: 0,
      max: max || 1,
      calculable: true,
      orient: 'horizontal',
      left: 'center',
      bottom: 4,
      itemWidth: 12,
      itemHeight: 90,
      textStyle: { color: textColor, fontSize: 11 },
      inRange: { color: ['#eef2ff', '#a5b4fc', '#6366f1', '#4338ca'] },
    },
    series: [
      {
        name: '互动次数',
        type: 'heatmap',
        data,
        label: { show: false },
        emphasis: { itemStyle: { shadowBlur: 8, shadowColor: 'rgba(0,0,0,0.3)' } },
        itemStyle: { borderColor: themeStore.isDark ? '#1e293b' : '#fff', borderWidth: 1 },
      },
    ],
  };
});
</script>

<style scoped>
.active-hours-heatmap { width: 100%; }
.ahh-canvas { width: 100%; height: 320px; }
</style>
