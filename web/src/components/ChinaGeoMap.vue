<template>
  <div class="china-geo-map">
    <div v-if="note" class="cgm-note">{{ note }}</div>
    <v-chart v-if="ready" :option="option" autoresize class="cgm-canvas" />
    <StateBlock
      v-else-if="!ready"
      state="loading"
      :title="official && !ready ? '正在拉取天地图官方边界' : '正在准备地图数据'"
      :description="official && !ready ? '首次会向天地图行政区划服务请求 34 个省级边界，之后走本地缓存。' : '首次加载会解析行政区划边界。'"
    />
    <p class="cgm-source">{{ sourceNote }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { registerMap } from 'echarts/core';
import StateBlock from '@/components/common/StateBlock.vue';
import chinaGeo from '@/assets/china.json';
import { loadOfficialChinaBounds, toFeatureCollection } from '@/components/maps/chinaOfficial';

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
    /** 用天地图官方省级边界（需要 tk）；失败自动回退内置边界 */
    official?: boolean;
    /** 站长的天地图浏览器端 tk */
    tk?: string;
  }>(),
  { data: () => [], note: '', official: false, tk: '' },
);

// 注册只需一次；registerMap 幂等，重复调用不会报错但没必要
const BASE_NOTE =
  '边界数据：DataV.GeoAtlas（基于高德行政区划，含南海诸岛/九段线）。仅供站内访问统计参考；' +
  '正式对外展示请以自然资源部标准地图为准并标注审图号。';

let baseRegistered = false;
if (!baseRegistered && chinaGeo) {
  registerMap('china', chinaGeo as any);
  baseRegistered = true;
}

// mapName 可在内置/官方之间切换；official=true 时先等官方边界再渲染
const mapName = ref('china');
const ready = ref(props.official ? false : baseRegistered);
const sourceNote = ref(BASE_NOTE);

let officialLoading = false;
let officialDone = false;

/**
 * 拉官方边界。
 * 注意不能用 onMounted 一次性判断：本组件在父级地图卡里比「地图配置」先挂载，
 * 首次渲染时 official 还是 false，等配置回来后必须重新触发。
 */
async function tryLoadOfficial() {
  if (officialDone || officialLoading) return;
  if (!props.official || !props.tk) {
    ready.value = baseRegistered;
    return;
  }
  officialLoading = true;
  const names = ((chinaGeo as any)?.features || [])
    .map((f: any) => f?.properties?.name)
    .filter((n: any) => !!n);
  try {
    const res = await loadOfficialChinaBounds(props.tk, names, chinaGeo as any);
    if (!res) {
      sourceNote.value = BASE_NOTE + '（天地图官方边界拉取失败，已回退内置数据）';
      ready.value = baseRegistered;
      return;
    }
    registerMap('china-official', toFeatureCollection(res.shapes) as any);
    mapName.value = 'china-official';
    sourceNote.value =
      `边界数据：天地图行政区划服务（自然资源部官方，${res.official} 个省级行政区` +
      (res.fallback ? `，另 ${res.fallback} 个回退内置` : '') +
      '）。数据仅用于站内访问统计；对外展示请标注审图号。';
    ready.value = true;
    officialDone = true;
  } catch {
    sourceNote.value = BASE_NOTE + '（天地图官方边界拉取失败，已回退内置数据）';
    ready.value = baseRegistered;
  } finally {
    officialLoading = false;
  }
}

watch([() => props.official, () => props.tk], () => tryLoadOfficial(), { immediate: true });

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
      map: mapName.value,
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
