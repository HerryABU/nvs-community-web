// 天地图「官方省级边界」加载器
//
// 为什么需要它：内置的 china.json 是第三方派生数据（DataV.GeoAtlas），
// 不是审图件。天地图本身提供行政区划服务，`extensions=true` 会返回官方
// MULTIPOLYGON 边界 —— 用它画中国省份色阶，数据源就是自然资源部的。
//
// 两个限制（实测）：
//   1. 浏览器端 tk **只能在浏览器里调** —— 服务端请求返回 301012「权限类型错误」，
//      所以只能前端拉，不能后端代理；
//   2. keyword 必须用**名称**（`北京市`），用 9 位编码会 404。
//
// 34 个省级行政区各请求一次，结果缓存在 localStorage（默认 7 天）。
// 任何一省失败就用内置边界补那一个省，整体失败则调用方回退内置数据 —— 不让地图开天窗。

// v2：v1 会把 429 限流的失败结果也缓存 7 天，必须换 key 让旧缓存失效
const CACHE_PREFIX = 'nvs_cn_bounds_v2_';
const CACHE_TTL = 7 * 24 * 3600 * 1000;
const API = 'https://api.tianditu.gov.cn/v2/administrative';

/** WKT MULTIPOLYGON → GeoJSON MultiPolygon 坐标（[[[ring]]]） */
export function wktToCoords(wkt: string): number[][][][] {
  const s = String(wkt || '').replace(/^\s*MULTIPOLYGON\s*/i, '');
  const polys: number[][][][] = [];
  let depth = 0;
  let poly: number[][][] | null = null;
  let ring: number[][] | null = null;
  let buf = '';
  const flush = () => {
    const t = buf.trim();
    buf = '';
    if (!t || !ring) return;
    const parts = t.split(/\s+/);
    const x = Number(parts[0]);
    const y = Number(parts[1]);
    if (Number.isFinite(x) && Number.isFinite(y)) ring.push([x, y]);
  };
  for (const ch of s) {
    if (ch === '(') {
      depth++;
      if (depth === 2) {
        poly = [];
        polys.push(poly);
      } else if (depth === 3 && poly) {
        ring = [];
        poly.push(ring);
      }
    } else if (ch === ')') {
      if (depth === 3) flush();
      if (depth === 2) ring = null;
      depth--;
    } else if (ch === ',') {
      if (depth === 3) flush();
    } else if (depth === 3) {
      buf += ch;
    }
  }
  return polys;
}

interface OfficialResult {
  /** key = 省名，value = MultiPolygon 坐标 */
  shapes: Record<string, number[][][][]>;
  /** 实际从官方拿到的省数 */
  official: number;
  /** 回退内置的省数 */
  fallback: number;
}

function cacheKey(tk: string) {
  // tk 换了就换缓存（白名单/配额可能不同，边界理论上一样，但别把旧 tk 的数据当新 tk 的）
  let h = 0;
  for (let i = 0; i < tk.length; i++) h = (h * 31 + tk.charCodeAt(i)) & 0x7fffffff;
  return `${CACHE_PREFIX}${h.toString(36)}`;
}

/** 并发受限的 map */
async function mapLimit<T, R>(items: T[], limit: number, fn: (x: T) => Promise<R>): Promise<R[]> {
  const out: R[] = new Array(items.length);
  let idx = 0;
  const workers = new Array(Math.min(limit, items.length)).fill(0).map(async () => {
    for (;;) {
      const i = idx++;
      if (i >= items.length) return;
      out[i] = await fn(items[i]);
    }
  });
  await Promise.all(workers);
  return out;
}

/**
 * 拉取官方省级边界。
 * @param tk 站长配置的浏览器端天地图 tk
 * @param names 需要的省名（与内置 china.json 的 properties.name 对齐）
 * @param fallback 内置 GeoJSON，任一省失败时用它补
 */
export async function loadOfficialChinaBounds(
  tk: string,
  names: string[],
  fallback?: { features?: { properties?: { name?: string }; geometry?: any }[] },
): Promise<OfficialResult | null> {
  const key = tk.trim();
  if (!key) return null;

  // 缓存命中
  try {
    const raw = localStorage.getItem(cacheKey(key));
    if (raw) {
      const obj = JSON.parse(raw);
      if (obj && obj.ts && Date.now() - obj.ts < CACHE_TTL && obj.shapes) {
        return { shapes: obj.shapes, official: obj.official || 0, fallback: 0 };
      }
    }
  } catch {
    /* 缓存坏了就当没有 */
  }

  const shapes: Record<string, number[][][][]> = {};
  let official = 0;

  const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

  const fetchOne = async (name: string): Promise<string> => {
    const url = `${API}?keyword=${encodeURIComponent(name)}&childLevel=0&extensions=true&tk=${encodeURIComponent(key)}`;
    const r = await fetch(url);
    if (!r.ok) return ''; // 429/4xx 都算这次失败，由外层重试
    const j = await r.json();
    return String(j?.data?.district?.[0]?.boundary || '');
  };

  // 并发压到 2、每个间隔 200ms：天地图行政区划接口有 QPS 限制，
  // 实测并发 5 会大量 429（34 个省只成功 4 个）。
  const results = await mapLimit(names, 2, async (name) => {
    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        if (attempt) await sleep(500 * attempt);
        const wkt = await fetchOne(name);
        if (wkt) return { name, wkt };
      } catch {
        /* 继续重试 */
      }
      await sleep(200);
    }
    return { name, wkt: '' };
  });

  for (const { name, wkt } of results) {
    if (!wkt) continue;
    const coords = wktToCoords(wkt);
    if (!coords.length) continue;
    shapes[name] = coords;
    official++;
  }

  // 官方一个都没拿到：交给调用方回退内置
  if (!official) return null;

  // 缺的省用内置补，避免整张图缺一块
  let fallbackCount = 0;
  for (const f of fallback?.features || []) {
    const nm = f?.properties?.name;
    if (!nm || shapes[nm]) continue;
    const g = f?.geometry;
    if (!g) continue;
    const c = g.type === 'Polygon' ? [g.coordinates] : g.type === 'MultiPolygon' ? g.coordinates : [];
    if (!c.length) continue;
    shapes[nm] = c;
    fallbackCount++;
  }

  // 成功太少（多半是被限流）就别写缓存 —— 否则一周内都拿着半成品
  if (official >= Math.max(10, Math.floor(names.length * 0.8))) {
    try {
      localStorage.setItem(cacheKey(key), JSON.stringify({ ts: Date.now(), shapes, official }));
    } catch {
      /* 超配额就算了，下次重新拉 */
    }
  }

  return { shapes, official, fallback: fallbackCount };
}

/** 把 shapes 组装成 ECharts 能 registerMap 的 FeatureCollection */
export function toFeatureCollection(shapes: Record<string, number[][][][]>) {
  return {
    type: 'FeatureCollection',
    features: Object.entries(shapes).map(([name, coordinates]) => ({
      type: 'Feature',
      properties: { name },
      geometry: { type: 'MultiPolygon', coordinates },
    })),
  };
}
