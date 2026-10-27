<template>
  <div class="china-geo-map">
    <div v-if="note" class="cgm-note">{{ note }}</div>
    <v-chart v-if="ready" :option="option" autoresize class="cgm-canvas" />
    <StateBlock
      v-else-if="!ready"
      state="loading"
      title="正在准备地图数据"
      description="首次加载会解析行政区划边界。"
    />
    <p class="cgm-source">
      边界数据：DataV.GeoAtlas（基于高德行政区划，含南海诸岛/九段线）。仅供站内访问统计参考；
      正式对外展示请以自然资源部标准地图为准并标注审图号。
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { registerMap } from 'echarts/core';
import StateBlock from '@/components/common/StateBlock.vue';
import chinaGeo from '@/assets/china.json';

// 中国地图（省级分区着色）。
//
// 几个刻意的选择：
// 1) GeoJSON 作为模块资源打包，不走运行时 fetch —— 站点支持 /go-nvs/ 这类路径前缀挂载，
//    运行时拼 URL 很容易漏加前缀，模块化引入由构建器处理最稳。
// 2) 边界数据含南海诸岛（adcode 100000_JD），国界与台湾/港澳按国标显示；
//    但代码不替站长背书 —— 页脚写明来源与「以标准地图为准」，避免拿着统计图当审图件用。
// 3) 只对「中国省级」着色；境外与国家维度用外部排行榜展示，不硬塞进这张图的省名里。

const props = withDefaults(
  defineProps<{
    /** 省级数据：name 必须与 GeoJSON 的 properties.name 完全一致（如「广东省」） */
    data?: { name: string; value: number }[];
    /** 地图上方说明（可选） */
    note?: string;
  }>(),
  { data: () => [], note: '' },
);

// 注册只需一次；registerMap 幂等，重复调用不会报错但没必要
let registered = false;
if (!registered && chinaGeo) {
  registerMap('china', chinaGeo as any);
  registered = true;
}
const ready = ref(registered);

const maxValue = computed(() => {
  const vals = (props.data || []).map((d) => Number(d.value) || 0);
  return vals.length ? Math.max(...vals) : 0;
});

const option = computed(() => ({
  tooltip: {
    trigger: 'item',
    formatter: (p: any) => {
      const v = p.value == null || Number.isNaN(p.value) ? '—' : p.value;
      return `${p.name}<br/>访问次数：<b>${v}</b>`;
    },
  },
  visualMap: {
    min: 0,
    max: maxValue.value || 1,
    left: 12,
    bottom: 12,
    text: ['高', '低'],
    calculable: true,
    inRange: { color: ['#e8f1ff', '#93b8ff', '#4f6ef7', '#1e3a8a'] },
    textStyle: { color: 'var(--text-light, #64748b)', fontSize: 11 },
  },
  series: [
    {
      type: 'map',
      map: 'china',
      roam: false,
      // 南海诸岛（九段线）在数据里是一条独立要素，名称空 —— 不参与着色
      emphasis: {
        label: { show: false },
        itemStyle: { areaColor: '#f59e0b' },
      },
      itemStyle: {
        borderColor: 'rgba(148,163,184,0.55)',
        borderWidth: 0.6,
        areaColor: '#eef2f7',
      },
      label: { show: false },
      data: (props.data || []).map((d) => ({ name: d.name, value: d.value })),
    },
  ],
}));
</script>

<style scoped>
.china-geo-map {
  width: 100%;
}
.cgm-canvas {
  width: 100%;
  height: 420px;
}
.cgm-note {
  font-size: 0.82rem;
  color: var(--text-light, #8a92a6);
  margin-bottom: 6px;
}
.cgm-source {
  margin: 8px 0 0;
  font-size: 0.74rem;
  line-height: 1.6;
  color: var(--text-light, #9aa3b5);
}
</style>
