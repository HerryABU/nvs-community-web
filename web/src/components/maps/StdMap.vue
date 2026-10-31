<template>
  <div class="std-map">
    <!-- 视野切换：中国 / 全球。中国站和国际访问都要能看，所以这里始终可切 -->
    <div class="sm-switch">
      <button
        v-for="s in scopes"
        :key="s.value"
        type="button"
        class="sm-tab"
        :class="{ active: viewScope === s.value }"
        @click="viewScope = s.value"
      >
        {{ s.label }}
      </button>
      <span class="sm-hint">
        {{ setting.provider === 'tianditu' && setting.key_ready ? '标准地图底图（天地图）' : '本地数据（未配置地图密钥）' }}
      </span>
    </div>

    <!-- ============ 中国视野 ============ -->
    <template v-if="viewScope === 'china'">
      <!-- 天地图：官方底图 + 省级气泡（不依赖任何第三方边界数据） -->
      <template v-if="setting.provider === 'tianditu' && setting.key_ready">
        <div ref="mapEl" class="sm-canvas" />
        <p class="sm-source">
          <template v-if="setting.review_no">审图号：{{ setting.review_no }}　</template>
          底图：天地图（国家地理信息公共服务平台）· 中国
          <span class="sm-dim">数据仅用于站内访问统计</span>
        </p>
      </template>
      <!-- 没有密钥时：用内置省级示意边界（明确标注非审图件），不影响看数据 -->
      <template v-else>
        <ChinaGeoMap :data="data" :note="note" />
        <p class="sm-source sm-warn">
          本地离线示意边界（非审图件，仅国内）—— 正式对外展示请到后台配置天地图密钥并标注审图号
        </p>
        <p v-if="setting.provider === 'tianditu' && !setting.key_ready" class="sm-source">
          已选择天地图但<span class="sm-warn">未填密钥</span>，因此暂用本地示意：
          请到 <code>tianditu.gov.cn</code> 申请「浏览器端」tk，填到后台「访客设置 → 地图服务」。
        </p>
      </template>
    </template>

    <!-- ============ 全球视野 ============ -->
    <template v-else>
      <!-- 有天地图密钥：官方全球底图 + 国家气泡点 -->
      <template v-if="setting.provider === 'tianditu' && setting.key_ready">
        <div ref="mapEl" class="sm-canvas" />
        <p class="sm-source">
          <template v-if="setting.review_no">审图号：{{ setting.review_no }}　</template>
          底图：天地图 · 全球
          <span class="sm-dim">数据仅用于站内访问统计</span>
        </p>
      </template>
      <!-- 没有密钥：**不画世界边界**（第三方世界边界既非审图件，也容易在国界表示上出错），
           改为按国家排行呈现同一份数据 —— 数据不缺，只是不画不合规的底图。 -->
      <template v-else>
        <div v-if="overseasRows.length" class="sm-ranks">
          <div class="sm-rank-head">
            <span>境外访问来源（近 30 天）</span>
            <span class="sm-dim">共 {{ overseasTotal }} 次 · {{ overseasRows.length }} 个国家/地区</span>
          </div>
          <div v-for="(r, i) in overseasRows" :key="r.code" class="sm-rank-row">
            <span class="sm-rank-no">{{ i + 1 }}</span>
            <span class="sm-rank-name">{{ r.name || r.code }}</span>
            <span class="sm-rank-bar">
              <i :style="{ width: barWidth(r.value) }" />
            </span>
            <span class="sm-rank-val">{{ r.value }}</span>
          </div>
        </div>
        <StateBlock
          v-else
          state="empty"
          title="近 30 天没有境外访问"
          description="境内访问请看「中国」视野；有了境外访问后这里会按国家排行显示。"
        />
        <p class="sm-source sm-warn">
          未配置天地图密钥，因此<b>不渲染世界边界底图</b>（第三方世界边界既不是审图件，
          也容易在国界表示上出错）。这里用同一份本地 ip2region 数据以国家排行呈现。
          需要真正的全球标准地图，请在后台配置天地图 tk（天地图支持全球视野）。
        </p>
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import api from '@/api';
import ChinaGeoMap from '@/components/ChinaGeoMap.vue';
import StateBlock from '@/components/common/StateBlock.vue';

interface MapSetting {
  provider: 'offline' | 'tianditu';
  scope: 'china' | 'world';
  key: string;
  review_no: string;
  key_ready: boolean;
}

interface CountryRow {
  code: string;
  name: string;
  value: number;
  cn?: boolean;
}

const props = defineProps<{
  data: { name: string; value: number }[];
  note?: string;
  /** 按国家的聚合（境外为主，含中国一行）；来自后端 country_heat */
  countries?: CountryRow[];
}>();

const setting = ref<MapSetting>({ provider: 'offline', scope: 'china', key: '', review_no: '', key_ready: false });
const mapEl = ref<HTMLElement>();
let map: any = null;
let markers: any[] = [];

const scopes = [
  { label: '中国', value: 'china' as const },
  { label: '全球', value: 'world' as const },
];
// 初始视野跟随站长配置，用户可以临时切（切了不改后台配置）
const viewScope = ref<'china' | 'world'>('china');

// 境外国家排行（排除中国，中国由省级图负责）
const overseasRows = computed(() =>
  (props.countries || []).filter((c) => !c.cn && c.value > 0).slice(0, 12),
);
const overseasTotal = computed(() => overseasRows.value.reduce((s, r) => s + r.value, 0));
const maxOverseas = computed(() => Math.max(1, ...overseasRows.value.map((r) => r.value)));

function barWidth(v: number) {
  return `${Math.max(4, Math.round((v / maxOverseas.value) * 100))}%`;
}

// 省级中心点（把「省份计数」画到标准底图上，不依赖任何第三方边界数据）
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

// 国家/地区中心点（仅用于在标准底图上打气泡，不含任何边界数据）
const COUNTRY_CENTROIDS: Record<string, [number, number]> = {
  CN: [104.2, 35.8], US: [-98.5, 39.8], JP: [138.0, 36.2], KR: [127.8, 36.5], KP: [127.0, 40.0],
  GB: [-1.5, 52.5], DE: [10.4, 51.1], FR: [2.2, 46.6], RU: [96.0, 61.0], CA: [-106.0, 56.0],
  AU: [134.0, -25.0], IN: [79.0, 22.0], BR: [-52.0, -10.0], SG: [103.8, 1.4], MY: [102.0, 4.0],
  TH: [101.0, 15.0], VN: [106.0, 16.0], ID: [118.0, -2.0], PH: [122.0, 12.0], TW: [121.0, 23.7],
  HK: [114.2, 22.3], MO: [113.5, 22.2], NL: [5.5, 52.2], ES: [-3.7, 40.4], IT: [12.5, 42.8],
  SE: [15.0, 62.0], NO: [9.0, 61.0], FI: [26.0, 64.0], DK: [10.0, 56.0], PL: [19.0, 52.0],
  CH: [8.2, 46.8], AT: [14.0, 47.5], BE: [4.5, 50.8], IE: [-8.0, 53.0], PT: [-8.0, 39.5],
  GR: [22.0, 39.0], TR: [35.0, 39.0], IL: [35.0, 31.5], AE: [54.0, 24.0], SA: [45.0, 24.0],
  ZA: [24.0, -29.0], EG: [30.0, 27.0], NG: [8.0, 10.0], KE: [38.0, 0.0], AR: [-64.0, -34.0],
  CL: [-71.0, -35.0], MX: [-102.0, 23.0], PE: [-75.0, -10.0], CO: [-74.0, 4.0], NZ: [174.0, -41.0],
  PK: [69.0, 30.0], BD: [90.0, 24.0], LK: [81.0, 7.0], MM: [96.0, 21.0], KH: [105.0, 12.5],
  MN: [103.0, 46.0], KZ: [67.0, 48.0], UA: [31.0, 49.0], CZ: [15.0, 50.0], RO: [25.0, 46.0],
  HU: [19.0, 47.0], QA: [51.2, 25.3], KW: [47.5, 29.5], BH: [50.6, 26.0], OM: [56.0, 21.0],
};

async function loadSetting() {
  try {
    const res = await api.get<{ code: number; data: MapSetting }>('/map-setting');
    if (res.data?.data) {
      setting.value = res.data.data;
      viewScope.value = res.data.data.scope === 'world' ? 'world' : 'china';
    }
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

function bubbleIcon(T: any, size: number, color: string) {
  return new T.Icon({
    iconSize: new T.Point(size, size),
    iconUrl:
      'data:image/svg+xml;base64,' +
      btoa(
        `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}"><circle cx="${size / 2}" cy="${size / 2}" r="${size / 2 - 2}" fill="${color}" stroke="#409eff" stroke-width="1.5"/></svg>`,
      ),
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

  const world = viewScope.value === 'world';
  map = new T.Map(mapEl.value);
  map.centerAndZoom(new T.LngLat(world ? 20 : 104.2, world ? 15 : 35.8), world ? 2 : 5);

  markers.forEach((m) => map.removeOverLay(m));
  markers = [];

  // 全球视野画国家气泡，中国视野画省级气泡
  const points: { name: string; value: number; pos: [number, number]; cn?: boolean }[] = [];
  if (world) {
    for (const c of props.countries || []) {
      const pos = COUNTRY_CENTROIDS[c.code];
      if (!pos || !c.value) continue;
      points.push({ name: c.name || c.code, value: c.value, pos, cn: c.cn });
    }
  } else {
    for (const d of props.data) {
      const pos = CENTROIDS[d.name];
      if (!pos || !d.value) continue;
      points.push({ name: d.name, value: d.value, pos });
    }
  }

  const max = Math.max(1, ...points.map((p) => p.value));
  for (const p of points) {
    const size = 10 + Math.round((p.value / max) * 20);
    // 境外用青色、境内用蓝色：一眼区分「自家读者」和「海外读者」
    const color = world && !p.cn ? 'rgba(20,184,166,0.55)' : 'rgba(64,158,255,0.55)';
    const marker = new T.Marker(new T.LngLat(p.pos[0], p.pos[1]), { icon: bubbleIcon(T, size, color) });
    marker.addEventListener('click', () =>
      marker.openInfoWindow(new T.InfoWindow(`${p.name}：${p.value} 次`)),
    );
    map.addOverLay(marker);
    markers.push(marker);
  }
}

onMounted(async () => {
  await loadSetting();
  if (setting.value.provider === 'tianditu' && setting.key_ready) renderTianditu();
});

watch(viewScope, () => {
  if (setting.value.provider === 'tianditu' && setting.key_ready) renderTianditu();
});

watch(() => [props.data, props.countries], () => {
  if (setting.value.provider === 'tianditu' && setting.key_ready && map) renderTianditu();
});

onBeforeUnmount(() => {
  markers = [];
  map = null;
});
</script>

<style scoped>
.std-map { display: flex; flex-direction: column; gap: 10px; }
.sm-switch { display: flex; align-items: center; gap: 6px; }
.sm-tab {
  padding: 4px 14px; border-radius: 999px; cursor: pointer;
  border: 1px solid var(--border-color, #e5e7eb);
  background: var(--bg-card, #fff);
  font-size: 0.8rem; color: var(--text-light, #6b7280);
  transition: all 0.15s ease;
}
.sm-tab.active {
  background: var(--el-color-primary, #409eff);
  border-color: var(--el-color-primary, #409eff);
  color: #fff; font-weight: 600;
}
.sm-hint { margin-left: auto; font-size: 0.72rem; color: var(--text-light, #9ca3af); }

.sm-canvas { width: 100%; height: 360px; border-radius: 10px; overflow: hidden; }
.sm-source { font-size: 0.72rem; color: var(--text-light, #999); line-height: 1.6; margin: 0; }
.sm-warn { color: var(--el-color-warning, #e6a23c); }
.sm-dim { opacity: 0.75; }

/* 国家排行（无底图时的数据呈现） */
.sm-ranks {
  padding: 12px 14px; border-radius: 10px;
  border: 1px solid var(--border-color, #e5e7eb);
  background: var(--bg-soft, #fafbfc);
  display: flex; flex-direction: column; gap: 7px;
}
.sm-rank-head {
  display: flex; justify-content: space-between; align-items: baseline;
  font-size: 0.8rem; font-weight: 600; color: var(--text-main, #1f2937);
  margin-bottom: 2px;
}
.sm-rank-row { display: flex; align-items: center; gap: 8px; font-size: 0.78rem; }
.sm-rank-no {
  width: 18px; text-align: center; color: var(--text-light, #9ca3af);
  font-variant-numeric: tabular-nums;
}
.sm-rank-name { width: 130px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-main, #374151); }
.sm-rank-bar { flex: 1; height: 7px; border-radius: 999px; background: var(--bg-page, #eef2f7); overflow: hidden; }
.sm-rank-bar i { display: block; height: 100%; border-radius: 999px; background: linear-gradient(90deg, #14b8a6, #409eff); }
.sm-rank-val { width: 52px; text-align: right; font-variant-numeric: tabular-nums; color: var(--text-main, #374151); }

@media (max-width: 640px) {
  .sm-rank-name { width: 92px; }
  .sm-canvas { height: 280px; }
}
</style>
