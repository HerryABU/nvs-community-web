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
        <template v-if="!isCnViewer">境外访问：仅展示全球视野（中国省级明细不对境外开放）　</template>
        {{ setting.provider === 'tianditu' && setting.key_ready ? '标准地图底图（天地图）' : '本地数据（未配置地图密钥）' }}
      </span>
    </div>

    <!-- ============ 中国视野（只有中国，画面里没有别国） ============ -->
    <!-- 天地图只发布全球瓦片（GetCapabilities 里 layer 只有 vec / img / ter / ibo），
         没有「仅中国」的底图图层；要画面里物理上只有中国，只能用**官方政区边界数据**自绘。
         配了 tk 就拉天地图行政区划服务的 boundary（34 个省级行政区），没配则用内置省界。 -->
    <template v-if="viewScope === 'china'">
      <ChinaGeoMap
        :data="data"
        :note="chinaNote"
        :official="setting.provider === 'tianditu' && setting.key_ready"
        :tk="setting.key"
      />
    </template>

    <!-- ============ 全球视野 ============ -->
    <template v-else>
      <!-- 有天地图密钥：官方全球底图 + 国家气泡点 -->
      <template v-if="setting.provider === 'tianditu' && setting.key_ready">
        <div ref="mapEl" class="sm-canvas" />
        <div v-if="mapErr" class="sm-err">
          <b>底图没显示</b>
          <span class="sm-err-msg">{{ mapErr }}</span>
          <button type="button" class="sm-retry" @click="scheduleRender">重试</button>
          <span class="sm-err-tip">
            逐项核对：tk 是「浏览器端」应用 · Referer 白名单含本站域名 · 网络能访问
            <code>api.tianditu.gov.cn</code> 与 <code>t0.tianditu.gov.cn</code>
          </span>
        </div>
        <p v-else-if="mapLoading" class="sm-source">底图加载中…</p>
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
              <i :style="{ width: barWidth(r.value, maxOverseas) }" />
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
          需要真正的全球标准地图，请到后台「运营 → 地图服务」填天地图 tk（天地图支持全球视野）。
        </p>
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import api from '@/api';
import ChinaGeoMap from '@/components/maps/ChinaGeoMap.vue';
import StateBlock from '@/components/common/StateBlock.vue';

interface MapSetting {
  provider: 'offline' | 'tianditu';
  scope: 'china' | 'world';
  key: string;
  review_no: string;
  key_ready: boolean;
  /** 看的人在哪儿：中国省级明细只对境内展示 */
  viewer?: { cn: boolean; country?: string; code?: string };
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
/** 底图加载失败必须说出来：API 4.0 在容器尺寸为 0 或瓦片被拒时会静默留白，
 *  站长只会看到「底图：天地图」下面一片空，无从判断是 key 还是网络的问题。 */
const mapErr = ref('');
const mapLoading = ref(false);
let renderTimer: any = null;
let tileTimer: any = null;

// 境外访客只给「全球」：省份/城市粒度的境内读者分布不对外。
const isCnViewer = computed(() => setting.value.viewer?.cn !== false);
const scopes = computed(() =>
  isCnViewer.value
    ? [
        { label: '中国', value: 'china' as const },
        { label: '全球', value: 'world' as const },
      ]
    : [{ label: '全球', value: 'world' as const }],
);
// 初始视野跟随站长配置，用户可以临时切（切了不改后台配置）
const viewScope = ref<'china' | 'world'>('china');

// 境外国家排行（排除中国，中国由省级图负责）
const overseasRows = computed(() =>
  (props.countries || []).filter((c) => !c.cn && c.value > 0).slice(0, 12),
);
const overseasTotal = computed(() => overseasRows.value.reduce((s, r) => s + r.value, 0));
const maxOverseas = computed(() => Math.max(1, ...overseasRows.value.map((r) => r.value)));

// 境内省级排行（未配天地图时的替代呈现：不画边界，但数据一条不少）
const cnRows = computed(() =>
  (props.data || [])
    .filter((d) => d.value > 0)
    .slice()
    .sort((a, b) => b.value - a.value)
    .slice(0, 12),
);
const cnTotal = computed(() => (props.data || []).reduce((s, d) => s + d.value, 0));
const maxCn = computed(() => Math.max(1, ...cnRows.value.map((r) => r.value)));

function barWidth(v: number, max: number) {
  return `${Math.max(4, Math.round((v / Math.max(1, max)) * 100))}%`;
}

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

// 中国视野的说明文字（审图号由站长在后台填；边界来源由 ChinaGeoMap 自行声明）
const chinaNote = computed(() => {
  const base = '省份按访问次数着色（蓝越深 = 占比越高）';
  return setting.value.review_no ? `${base} · 审图号：${setting.value.review_no}` : base;
});

async function loadSetting() {
  try {
    const res = await api.get<{ code: number; data: MapSetting }>('/map-setting');
    if (res.data?.data) {
      setting.value = res.data.data;
      // 境外访客即便站长默认设置是中国，也只给全球视野
      const cnViewer = res.data.data.viewer?.cn !== false;
      viewScope.value = !cnViewer ? 'world' : res.data.data.scope === 'world' ? 'world' : 'china';
    }
  } catch {
    /* 读不到就保持离线示意 */
  }
}

/** 动态加载天地图 SDK（key 由站长在后台配置，代码里不内置任何密钥）。
 *  必须带超时：脚本被网络/WAF 拦时 onerror 不一定触发，否则会永远「转圈什么都不显示」。 */
function loadTianditu(key: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const T = (window as any).T;
    if (T) return resolve();
    const s = document.createElement('script');
    s.src = `https://api.tianditu.gov.cn/api?v=4.0&tk=${encodeURIComponent(key)}`;
    const timer = setTimeout(() => reject(new Error('天地图 SDK 加载超时（10 秒）：网络可能被拦截')), 10000);
    s.onload = () => {
      clearTimeout(timer);
      if ((window as any).T) resolve();
      else reject(new Error('SDK 已下载但没有初始化 window.T，tk 可能不被接受'));
    };
    s.onerror = () => {
      clearTimeout(timer);
      reject(new Error('天地图 SDK 脚本加载失败：可能被网络/WAF 拦截'));
    };
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
  // 中国视野用官方边界自绘（ChinaGeoMap），没有天地图容器，别在这里白跑
  if (viewScope.value !== 'world') return;
  await nextTick();
  if (!mapEl.value) return;
  mapErr.value = '';
  mapLoading.value = true;
  try {
    await loadTianditu(setting.value.key);
  } catch (e: any) {
    mapLoading.value = false;
    mapErr.value = e?.message || '天地图 SDK 加载失败';
    return;
  }
  // 注：天地图 4.0 的 SDK 自身会抛一次 "Cannot read properties of undefined (reading 'extend')"
  // （它内部加载 v4.0 扩展脚本时的 bug），与出图无关 —— 实测底图瓦片正常加载，不要误判成本项目的错。
  const T = (window as any).T;
  if (!T || !mapEl.value) {
    mapLoading.value = false;
    mapErr.value = '天地图 SDK 未就绪（window.T 缺失）';
    return;
  }

  // 重建前先清空：同一容器重复 new T.Map 会叠出多层画布
  mapEl.value.innerHTML = '';
  map = new T.Map(mapEl.value);
  // 容器已清空，旧实例的 overlay 随之消失；这里只重置引用。
  markers = [];
  // 只有全球视野会走到这里（中国视野在函数开头就返回了）
  map.centerAndZoom(new T.LngLat(20, 15), 2);
  mapLoading.value = false;

  {
    // ── 全球：国家气泡（没有国界数据，气泡最诚实，也不会画错国界）──
    const points: { name: string; value: number; pos: [number, number]; cn?: boolean }[] = [];
    for (const c of props.countries || []) {
      const pos = COUNTRY_CENTROIDS[c.code];
      if (!pos || !c.value) continue;
      points.push({ name: c.name || c.code, value: c.value, pos, cn: c.cn });
    }
    const max = Math.max(1, ...points.map((p) => p.value));
    for (const p of points) {
      const size = 10 + Math.round((p.value / max) * 20);
      const color = !p.cn ? 'rgba(20,184,166,0.55)' : 'rgba(64,158,255,0.55)';
      const marker = new T.Marker(new T.LngLat(p.pos[0], p.pos[1]), { icon: bubbleIcon(T, size, color) });
      marker.addEventListener('click', () =>
        marker.openInfoWindow(new T.InfoWindow(`${p.name}：${p.value} 次`)),
      );
      map.addOverLay(marker);
      markers.push(marker);
    }
  }

  // 最后一道自检：8 秒后容器里一张瓦片都没有 = 只出了灰底，说明请求被拒（tk/白名单）或被拦
  clearTimeout(tileTimer);
  tileTimer = window.setTimeout(() => {
    const n = mapEl.value?.querySelectorAll('img').length || 0;
    if (n === 0) {
      mapErr.value =
        '底图瓦片一张都没加载出来：tk 可能没通过 Referer 白名单，或网络拦截了 t0.tianditu.gov.cn';
    }
  }, 8000);
}

onMounted(async () => {
  await loadSetting();
  scheduleRender();
});

/**
 * 调度一次渲染：去抖 + 容器缺失时重试。
 * 为什么需要：StdMap 挂在父页面的 v-if 分幕里，配置又是异步拉回来的，
 * onMounted 里直接取 mapEl 有可能还没被 patch 上去（拿不到 DOM 就永久留白）。
 */
function scheduleRender() {
  clearTimeout(renderTimer);
  renderTimer = window.setTimeout(() => {
    let tries = 0;
    const step = () => {
      if (setting.value.provider !== 'tianditu' || !setting.value.key_ready) return;
      // 中国视野用官方边界自绘，没有天地图容器 —— 不去找 mapEl，否则会误报「容器没渲染出来」
      if (viewScope.value !== 'world') return;
      if (mapEl.value) {
        renderTianditu();
        return;
      }
      if (++tries > 30) {
        mapErr.value = '地图容器没有渲染出来（页面可能还处于隐藏状态）';
        return;
      }
      renderTimer = window.setTimeout(step, 100);
    };
    step();
  }, 60);
}

// 配置变化（含首次从接口拿回）后重建；flush:'post' 保证 DOM 已更新
watch(setting, () => scheduleRender(), { deep: true, flush: 'post' });

watch(viewScope, () => scheduleRender(), { flush: 'post' });

watch(
  () => [props.data, props.countries],
  () => {
    if (map) renderTianditu();
  },
);

onBeforeUnmount(() => {
  clearTimeout(renderTimer);
  clearTimeout(tileTimer);
  markers = [];
  map = null;
});
</script>

<style scoped>
.std-map { position: relative; display: flex; flex-direction: column; gap: 10px; }
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
/* 色阶图例（叠在画布右上角，避开天地图右下版权条） */
.sm-legend {
  position: absolute; right: 12px; top: 44px; z-index: 2;
  display: flex; align-items: center; gap: 6px;
  padding: 6px 9px; border-radius: 8px;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.08);
  font-size: 0.68rem; color: var(--text-light, #6b7280);
}
.sm-legend-bar {
  display: block; width: 84px; height: 8px; border-radius: 999px;
  background: linear-gradient(90deg, #e8f1ff, #93b8ff, #4f6ef7, #1e3a8a);
}
.sm-legend-cap { font-variant-numeric: tabular-nums; }
.sm-legend-note { margin-left: 4px; padding-left: 8px; border-left: 1px solid var(--border-color, #e5e7eb); line-height: 1.45; }

/* 底图失败提示：不静默留白，直接说清「没出图」和下一步核对什么 */
.sm-err {
  display: flex; flex-wrap: wrap; align-items: baseline; gap: 6px 10px;
  padding: 10px 12px; border-radius: 8px;
  border: 1px solid var(--el-color-warning-light-5, #f5dab1);
  background: var(--el-color-warning-light-9, #fdf6ec);
  font-size: 0.78rem; color: var(--text-main, #6b4a1b);
}
.sm-err-msg { color: var(--el-color-warning, #e6a23c); font-weight: 500; }
.sm-retry {
  padding: 2px 10px; border-radius: 6px; cursor: pointer;
  border: 1px solid var(--el-color-primary, #409eff);
  background: transparent; color: var(--el-color-primary, #409eff); font-size: 0.74rem;
}
.sm-retry:hover { background: var(--el-color-primary-light-9, #ecf5ff); }
.sm-err-tip { flex: 1 1 100%; font-size: 0.72rem; color: var(--text-light, #9ca3af); line-height: 1.7; }
.sm-err-tip code { padding: 0 4px; border-radius: 3px; background: var(--bg-page, #eef2f7); }
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
