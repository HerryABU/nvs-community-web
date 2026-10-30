<template>
  <div class="std-map">
    <!-- 天地图（自然资源部官方）：国内 + 全球都支持 -->
    <template v-if="setting.provider === 'tianditu'">
      <div v-if="!setting.key_ready" class="sm-guide">
        <p class="sm-guide-title">尚未配置天地图密钥</p>
        <p class="sm-guide-text">
          标准地图请使用<b>天地图</b>（国家地理信息公共服务平台）。请到
          <code>tianditu.gov.cn</code> 注册开发者、创建「浏览器端」应用拿到 tk，
          再到本站后台「地图服务」填入；并在地图控制台把本站域名加入 <b>Referer 白名单</b>，防止密钥被盗用。
        </p>
        <p class="sm-guide-text">
          未配置前不会渲染任何示意边界 —— 宁可留空，也不拿非标准地图充当对外展示。
        </p>
      </div>
      <template v-else>
        <div ref="mapEl" class="sm-canvas" />
        <p class="sm-source">
          <template v-if="setting.review_no">审图号：{{ setting.review_no }}　</template>
          底图：天地图（国家地理信息公共服务平台）· {{ setting.scope === 'world' ? '全球' : '中国' }}
          <span class="sm-dim">数据仅用于站内访问统计</span>
        </p>
      </template>
    </template>

    <!-- 离线示意：保留历史行为，但明确标注「非标准地图」 -->
    <template v-else>
      <ChinaGeoMap :data="data" :note="note" />
      <p class="sm-source sm-warn">
        离线示意边界（非审图件，仅国内）—— 正式对外展示请切换到天地图并标注审图号
      </p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import api from '@/api';
import ChinaGeoMap from '@/components/ChinaGeoMap.vue';

interface MapSetting {
  provider: 'offline' | 'tianditu';
  scope: 'china' | 'world';
  key: string;
  review_no: string;
  key_ready: boolean;
}

const props = defineProps<{ data: { name: string; value: number }[]; note?: string }>();

const setting = ref<MapSetting>({ provider: 'offline', scope: 'china', key: '', review_no: '', key_ready: false });
const mapEl = ref<HTMLElement>();
let map: any = null;
let markers: any[] = [];

// 省级中心点（用于把「省份计数」画到标准底图上，避免依赖任何第三方边界数据）
const CENTROIDS: Record<string, [number, number]> = {
  北京市: [116.4, 39.9], 天津市: [117.2, 39.1], 河北省: [114.5, 38.0], 山西省: [112.5, 37.9],
  内蒙古自治区: [111.7, 40.8], 辽宁省: [123.4, 41.8], 吉林省: [125.3, 43.9], 黑龙江省: [126.6, 45.8],
  上海市: [121.5, 31.2], 江苏省: [118.8, 32.1], 浙江省: [120.2, 30.3], 安徽省: [117.3, 31.9],
  福建省: [119.3, 26.1], 江西省: [115.9, 28.7], 山东省: [117.0, 36.7], 河南省: [113.7, 34.8],
  湖北省: [114.3, 30.6], 湖南省: [113.0, 28.2], 广东省: [113.3, 23.1], 广西壮族自治区: [108.3, 22.8],
  海南省: [110.3, 20.0], 重庆市: [106.5, 29.6], 四川省: [104.1, 30.7], 贵州省: [106.7, 26.6],
  云南省: [102.7, 25.0], 西藏自治区: [91.1, 29.6], 陕西省: [108.9, 34.3], 甘肃省: [103.8, 36.1],
  青海省: [101.8, 36.6], 宁夏回族自治区: [106.3, 38.5], 新疆维吾尔自治区: [87.6, 43.8],
  台湾省: [121.5, 25.0], 香港特别行政区: [114.2, 22.3], 澳门特别行政区: [113.5, 22.2],
};

async function loadSetting() {
  try {
    const res = await api.get<{ code: number; data: MapSetting }>('/map-setting');
    if (res.data?.data) setting.value = res.data.data;
  } catch {
    /* 读不到就保持离线示意 */
  }
}

/** 动态加载天地图 SDK（key 由站长在后台配置，代码里不内置任何密钥） */
function loadTianditu(key: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if ((window as any).T) return resolve();
    const s = document.createElement('script');
    s.src = `https://api.tianditu.gov.cn/api?v=4.0&tk=${encodeURIComponent(key)}`;
    s.onload = () => resolve();
    s.onerror = () => reject(new Error('天地图 SDK 加载失败'));
    document.head.appendChild(s);
  });
}

async function renderTianditu() {
  if (setting.value.provider !== 'tianditu' || !setting.value.key_ready) return;
  await nextTick();
  if (!mapEl.value) return;
  try {
    await loadTianditu(setting.value.key);
  } catch {
    return; // SDK 拉不到就静默保持空白，不画假图
  }
  const T = (window as any).T;
  if (!T || !mapEl.value) return;

  const world = setting.value.scope === 'world';
  map = new T.Map(mapEl.value);
  map.centerAndZoom(new T.LngLat(world ? 20 : 104.2, world ? 20 : 35.8), world ? 2 : 5);

  // 清旧标记
  markers.forEach((m) => map.removeOverLay(m));
  markers = [];

  const max = Math.max(1, ...props.data.map((d) => d.value));
  for (const d of props.data) {
    const c = CENTROIDS[d.name];
    if (!c || !d.value) continue;
    const size = 10 + Math.round((d.value / max) * 18);
    const marker = new T.Marker(new T.LngLat(c[0], c[1]), {
      icon: new T.Icon({
        iconSize: new T.Point(size, size),
        iconUrl:
          'data:image/svg+xml;base64,' +
          btoa(
            `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}"><circle cx="${size / 2}" cy="${size / 2}" r="${size / 2 - 2}" fill="rgba(64,158,255,0.55)" stroke="#409eff" stroke-width="1.5"/></svg>`,
          ),
      }),
    });
    marker.addEventListener('click', () => marker.openInfoWindow(new T.InfoWindow(`${d.name}：${d.value} 次`)));
    map.addOverLay(marker);
    markers.push(marker);
  }
}

onMounted(async () => {
  await loadSetting();
  if (setting.value.provider === 'tianditu') renderTianditu();
});

watch(() => props.data, () => {
  if (setting.value.provider === 'tianditu' && map) renderTianditu();
});

onBeforeUnmount(() => {
  markers = [];
  map = null;
});
</script>

<style scoped>
.std-map { display: flex; flex-direction: column; gap: 8px; }
.sm-canvas { width: 100%; height: 360px; border-radius: 10px; overflow: hidden; }
.sm-source { font-size: 0.72rem; color: var(--text-light, #999); line-height: 1.6; margin: 0; }
.sm-warn { color: var(--el-color-warning, #e6a23c); }
.sm-dim { opacity: 0.75; }
.sm-guide {
  padding: 18px 20px; border-radius: 10px;
  border: 1px dashed var(--border-color, #dcdfe6);
  background: var(--bg-soft, #fafbfc);
}
.sm-guide-title { margin: 0 0 8px; font-weight: 600; font-size: 0.9rem; color: var(--text-main, #1f2937); }
.sm-guide-text { margin: 0 0 6px; font-size: 0.8rem; line-height: 1.7; color: var(--text-light, #666); }
.sm-guide code { padding: 1px 5px; border-radius: 4px; background: rgba(0, 0, 0, 0.06); }
</style>
