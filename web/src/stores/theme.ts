import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { themeApi, type ThemeJSMode, type ThemeSettings } from '@/api/home/theme';
import api from '@/api';
import { useAuthStore } from '@/stores/auth';
import { useAppStore } from '@/stores/app';

// alist 兼容主题 API：暴露给自定义 JS 调用的只读安全全局对象。
// 既挂 window.NVS（本平台原生），也挂 window.alist（兼容 alist 主题写法）。
interface NVSBus {
  on(evt: string, cb: (payload?: any) => void): void;
  off(evt: string, cb: (payload?: any) => void): void;
  emit(evt: string, payload?: any): void;
}
interface NVSGlobal {
  version: string;
  get siteName(): string;
  get dark(): boolean;
  get user(): { id: number; username: string; role: string } | null;
  get route(): { path: string; name: string | null };
  bus: NVSBus;
  on(evt: string, cb: (payload?: any) => void): void;
  off(evt: string, cb: (payload?: any) => void): void;
  emit(evt: string, payload?: any): void;
  api: {
    get: (path: string, cfg?: any) => Promise<any>;
    post: (path: string, body?: any, cfg?: any) => Promise<any>;
    put: (path: string, body?: any, cfg?: any) => Promise<any>;
    del: (path: string, cfg?: any) => Promise<any>;
  };
}
declare global {
  interface Window {
    NVS?: NVSGlobal;
    alist?: NVSGlobal;
  }
}

export const useThemeStore = defineStore('theme', () => {
  // ===== 明暗模式（既有功能：data-theme 属性切换，勿删） =====
  const isDark = ref(localStorage.getItem('nvs-theme') === 'dark');

  const applyTheme = () => {
    document.documentElement.setAttribute('data-theme', isDark.value ? 'dark' : 'light');
    window.NVS?.emit('theme', { dark: isDark.value });
  };

  const toggle = () => {
    isDark.value = !isDark.value;
    localStorage.setItem('nvs-theme', isDark.value ? 'dark' : 'light');
    applyTheme();
  };

  // 初始化时立即应用主题
  applyTheme();

  // ===== F12 用户自定义网站主题（alist 式，服务端消毒后全局生效） =====
  const siteCss = ref(''); // 站点默认主题（管理员配置，对全部访客生效）
  const myCss = ref(''); // 个人主题（登录用户配置，覆盖站点主题）
  const siteJs = ref(''); // 站点自定义 JS（管理员配置，全站访客执行）
  const myJs = ref(''); // 个人自定义 JS（仅本人登录态执行）
  // JS 加载模式（alist「延迟加载」）：immediate 立即 / idle 首屏后空闲时 / delayed 延迟 N 秒
  const siteJsMode = ref<ThemeJSMode>('immediate');
  const siteJsDelay = ref(0);
  const myJsMode = ref<ThemeJSMode>('immediate');
  const myJsDelay = ref(0);
  // F12a 全站 opt-in iframe（管理员配置允许列表 URL，访客可读启用状态）
  const globalIframe = ref<{ enabled: boolean; url: string }>({ enabled: false, url: '' });

  // 结构化主题设置（可视化控件生成，与原始 CSS 互补）：背景 / 毛玻璃 / 按钮 / 看板娘
  const siteSettings = ref<ThemeSettings>(defaultThemeSettings());

  // 生效 CSS：个人主题优先；两者均挂载时 my 后写入即覆盖 site
  const effectiveCss = computed(() => (myCss.value.trim() ? myCss.value : siteCss.value));

  // 拉取站点主题 +（已登录时）个人主题。
  async function loadTheme(): Promise<void> {
    const auth = useAuthStore();
    if (!auth.user) {
      try {
        await auth.fetchUser();
      } catch {
        /* 静默：按匿名处理 */
      }
    }
    const tasks: Promise<any>[] = [themeApi.getSiteTheme()];
    if (auth.isLoggedIn) tasks.push(themeApi.getMyTheme());
    const settled = await Promise.allSettled(tasks);
    const site = settled[0];
    if (site.status === 'fulfilled' && site.value?.data?.code === 0) {
      siteCss.value = site.value.data.data?.css || '';
      siteJs.value = site.value.data.data?.js || '';
      siteJsMode.value = normalizeJsMode(site.value.data.data?.js_mode);
      siteJsDelay.value = site.value.data.data?.js_delay || 0;
      siteSettings.value = parseSettings(site.value.data.data?.settings);
    }
    const me = settled[1];
    if (me && me.status === 'fulfilled' && me.value?.data?.code === 0) {
      myCss.value = me.value.data.data?.css || '';
      myJs.value = me.value.data.data?.js || '';
      myJsMode.value = normalizeJsMode(me.value.data.data?.js_mode);
      myJsDelay.value = me.value.data.data?.js_delay || 0;
    }
    // 先安装 alist 兼容全局对象，再注入用户 JS（主题可立即监听 bus 事件）
    installNvsGlobal();
    applyCustomTheme();
    // F12a 拉取全站 opt-in iframe 启用状态（访客可读）
    loadGlobalIframe();
  }

  // 写入 <style> 标签注入主题 CSS。优先级（后者覆盖前者）：
  // 结构化 CSS（nvs-theme-structured，由可视化控件生成）→ 站点原始 CSS → 个人 CSS。
  // 安全：textContent（CSS 不构 XSS）；真正防线在服务端白名单消毒（utils.SanitizeCSS）。
  function applyCustomTheme(): void {
    writeStyleTag('nvs-theme-structured', buildStructuredCSS(siteSettings.value));
    writeStyleTag('nvs-site-theme', siteCss.value);
    writeStyleTag('nvs-my-theme', myCss.value);
    scheduleScriptTag('nvs-site-js', siteJs.value, siteJsMode.value, siteJsDelay.value);
    scheduleScriptTag('nvs-my-js', myJs.value, myJsMode.value, myJsDelay.value);
    applyMascot(siteSettings.value);
  }

  // 重置个人主题（本地即时生效；持久化删除由页面调用 updateMyTheme('', '') 完成）
  function resetMyTheme(): void {
    myCss.value = '';
    writeStyleTag('nvs-my-theme', '');
    myJs.value = '';
    cancelScheduled('nvs-my-js');
    writeScriptTag('nvs-my-js', '');
  }

  // F12a 全站 opt-in iframe：拉取启用状态 + URL，App.vue 据此渲染
  async function loadGlobalIframe(): Promise<void> {
    try {
      const r = await api.get('/site-iframe');
      const d = r.data?.data || {};
      globalIframe.value = { enabled: !!d.enabled, url: d.url || '' };
    } catch {
      /* 静默：未配置则不渲染 */
    }
  }

  // F12b 作者主题（访客可见）：注入第三个 <style>，仅作者域页面生效
  function loadAuthorTheme(authorId: number | string): void {
    api.get(`/author/${authorId}/theme`).then((r: any) => {
      writeStyleTag(`nvs-author-theme-${authorId}`, r?.data?.data?.css || '');
    }).catch(() => {});
  }
  function clearAuthorTheme(authorId: number | string): void {
    const el = document.getElementById(`nvs-author-theme-${authorId}`);
    if (el) el.textContent = '';
  }

  return {
    isDark, toggle, applyTheme,
    siteCss, myCss, effectiveCss, loadTheme, applyCustomTheme, resetMyTheme,
    globalIframe, loadGlobalIframe, loadAuthorTheme, clearAuthorTheme,
    siteJs, myJs, siteJsMode, siteJsDelay, myJsMode, myJsDelay,
    siteSettings,
  };
});

// ===== 结构化主题设置（可视化控件 → 生成结构化 CSS，对齐 alist 的可视化美化能力）=====
function defaultThemeSettings(): ThemeSettings {
  return {
    background: { type: 'color', value: '' },
    glass: { enabled: true, blur: 20, opacity: 0.6, tint: '#ffffff' },
    button: { radius: 12, primary: '' },
    accent: { color: '' },
    surface: { color: '' },
    mascot: { enabled: false, script_url: '' },
  };
}

// parseSettings 合并默认值，容错非法 JSON（前端本地解析，服务端还会再次校验）
function parseSettings(raw?: string): ThemeSettings {
  const def = defaultThemeSettings();
  if (!raw) return def;
  try {
    const p = JSON.parse(raw);
    return {
      background: { ...def.background, ...(p.background || {}) },
      glass: { ...def.glass, ...(p.glass || {}) },
      button: { ...def.button, ...(p.button || {}) },
      accent: { ...def.accent, ...(p.accent || {}) },
      surface: { ...def.surface, ...(p.surface || {}) },
      mascot: { ...def.mascot, ...(p.mascot || {}) },
    };
  } catch {
    return def;
  }
}

// hexToRgb 将 HEX 颜色拆成 "r, g, b"，供 rgba(var(--primary-rgb), a) 这类旧式自定义 CSS 使用。
// 返回 null 表示非法颜色（调用方据此跳过输出，避免污染变量）。
function hexToRgb(hex: string): string | null {
  let h = (hex || '').replace('#', '');
  if (h.length === 3) h = h.split('').map((c) => c + c).join('');
  if (h.length !== 6) return null;
  const r = parseInt(h.slice(0, 2), 16);
  const g = parseInt(h.slice(2, 4), 16);
  const b = parseInt(h.slice(4, 6), 16);
  if ([r, g, b].some((n) => Number.isNaN(n))) return null;
  return `${r}, ${g}, ${b}`;
}

// hexToRgba 将 HEX 颜色转为带透明度的 rgba()（毛玻璃底色）
function hexToRgba(hex: string, alpha: number): string {
  const rgb = hexToRgb(hex);
  if (!rgb) return `rgba(255,255,255,${alpha})`;
  return `rgba(${rgb},${alpha})`;
}

// buildStructuredCSS 由结构化设置生成 CSS：覆盖站点级 CSS 变量（背景 / 玻璃 / 主色 / 强调色 / 表面色 / 圆角）。
// 原始 CSS 文本仍可覆盖这些变量，两者互补而非互斥。
function buildStructuredCSS(s: ThemeSettings): string {
  const rules: string[] = [];
  if (s.background?.value) {
    let bg = s.background.value;
    if (s.background.type === 'image') bg = `url("${bg}") center / cover no-repeat fixed`;
    rules.push(`html, body { background: ${bg} !important; }`);
  }
  const blur = s.glass?.enabled ? Math.max(0, s.glass.blur || 20) : 0;
  const tint = s.glass?.tint || '#ffffff';
  const opacity = s.glass?.enabled ? Math.min(1, Math.max(0, s.glass.opacity ?? 0.6)) : 0;
  rules.push(`:root { --glass-blur: ${blur}px; --glass-bg: ${hexToRgba(tint, opacity)}; }`);
  // 主色：除 --primary-color 外同步输出 --primary-rgb，兼容旧式 rgba(var(--primary-rgb), a) 自定义 CSS。
  // 其余主色系（渐变 / Element Plus 光晕 / 浅底）已在 global.css 中用 color-mix 派生，自动跟随。
  if (s.button?.primary) {
    const rgb = hexToRgb(s.button.primary);
    rules.push(`:root { --primary-color: ${s.button.primary};${rgb ? ` --primary-rgb: ${rgb};` : ''} }`);
  }
  if (s.accent?.color) {
    rules.push(`:root { --accent-color: ${s.accent.color}; }`);
    // 强调色同时驱动 AI 写作可视化主节点与回环色，做到「换个强调色，图表随之变色」
    rules.push(`:root { --aw-node: ${s.accent.color}; --aw-return: ${s.accent.color}; }`);
    // 循环弧三色也由强调色派生，避免换了强调色后图表仍残留红/橙/绿
    rules.push(`:root { --aw-loop: ${s.accent.color}; --aw-loop-2: color-mix(in srgb, ${s.accent.color} 65%, #f59e0b); --aw-loop-3: color-mix(in srgb, ${s.accent.color} 55%, #10b981); }`);
  }
  if (s.surface?.color) rules.push(`:root { --surface-color: ${s.surface.color}; }`);
  if (s.button?.radius != null && s.button.radius >= 0) {
    rules.push(`:root { --radius-md: ${s.button.radius}px; --radius-lg: ${s.button.radius + 4}px; }`);
  }
  return rules.join('\n');
}

// applyMascot 看板娘（兼容 alist/live2d 等任意外部脚本）：加载管理员配置的 script_url，
// 挂载点 #nvs-mascot 由 App.vue 提供；未启用或无 URL 时隐藏挂载点。
function applyMascot(s: ThemeSettings): void {
  const existing = document.getElementById('nvs-mascot-script');
  if (existing) existing.remove();
  const slot = document.getElementById('nvs-mascot');
  const enabled = !!s.mascot?.enabled && !!s.mascot?.script_url;
  if (slot) slot.style.display = enabled ? '' : 'none';
  if (!enabled) return;
  const el = document.createElement('script');
  el.id = 'nvs-mascot-script';
  el.src = s.mascot.script_url;
  el.async = true;
  document.head.appendChild(el);
}

// writeStyleTag 确保 <style id> 存在并以 textContent 写入 CSS
function writeStyleTag(id: string, css: string): void {
  let el = document.getElementById(id) as HTMLStyleElement | null;
  if (!el) {
    el = document.createElement('style');
    el.id = id;
    document.head.appendChild(el);
  }
  el.textContent = css;
}

// writeScriptTag 确保 <script id> 存在并以 textContent 写入 JS。
// 用 textContent（非 innerHTML）由浏览器执行；后端已剥离 </script> 防止逃逸。
function writeScriptTag(id: string, js: string): void {
  const existing = document.getElementById(id);
  if (!js || !js.trim()) {
    if (existing) existing.remove();
    return;
  }
  const el = document.createElement('script');
  el.id = id;
  el.textContent = js; // 注入后浏览器执行（textContent，非 innerHTML）
  if (existing) existing.remove();
  document.head.appendChild(el);
}

// ===== 自定义 JS 的延迟加载（对齐 alist 的「延迟加载」）=====
//
// immediate：随主题立即注入（既有行为）
// idle     ：等首屏渲染完成（window load）后，在浏览器空闲时注入
// delayed  ：延迟 N 秒后注入
//
// 为什么要延迟：自定义 JS 往往要遍历 DOM、挂埋点、加载第三方挂件，
// 跟首屏渲染抢主线程会拖慢开屏；延迟注入则两者互不干扰。

type PendingTimer = { kind: 'timeout'; id: number } | { kind: 'listener'; fn: () => void };
const pendingScripts = new Map<string, PendingTimer[]>();

function normalizeJsMode(v: unknown): ThemeJSMode {
  const s = String(v || '');
  return s === 'idle' || s === 'delayed' ? (s as ThemeJSMode) : 'immediate';
}

function cancelScheduled(id: string): void {
  const list = pendingScripts.get(id) || [];
  for (const t of list) {
    if (t.kind === 'timeout') window.clearTimeout(t.id);
    else window.removeEventListener('load', t.fn);
  }
  pendingScripts.delete(id);
}

function scheduleScriptTag(id: string, js: string, mode: ThemeJSMode, delaySec: number): void {
  cancelScheduled(id);
  const existing = document.getElementById(id);
  if (!js || !js.trim()) {
    if (existing) existing.remove();
    return;
  }
  if (mode === 'immediate') {
    writeScriptTag(id, js);
    return;
  }
  // 延迟模式：先把旧的移除，等调度点再注入新的
  if (existing) existing.remove();
  const pending: PendingTimer[] = [];
  const inject = () => {
    writeScriptTag(id, js);
    pendingScripts.delete(id);
  };
  if (mode === 'idle') {
    const run = () => {
      const ric = (window as any).requestIdleCallback;
      if (typeof ric === 'function') ric(() => inject(), { timeout: 2000 });
      else window.setTimeout(inject, 200);
    };
    if (document.readyState === 'complete') {
      pending.push({ kind: 'timeout', id: window.setTimeout(run, 0) });
    } else {
      pending.push({ kind: 'listener', fn: run });
      window.addEventListener('load', run, { once: true });
    }
  } else {
    const ms = Math.max(1, delaySec > 0 ? delaySec : 3) * 1000;
    pending.push({ kind: 'timeout', id: window.setTimeout(inject, ms) });
  }
  pendingScripts.set(id, pending);
}

// ===== alist 兼容全局对象（window.NVS / window.alist）=====
let nvsInstalled = false;
function installNvsGlobal(): void {
  if (nvsInstalled) return;
  nvsInstalled = true;

  const listeners = new Map<string, Set<(p?: any) => void>>();
  const bus: NVSBus = {
    on(evt, cb) {
      if (!listeners.has(evt)) listeners.set(evt, new Set());
      listeners.get(evt)!.add(cb);
    },
    off(evt, cb) {
      listeners.get(evt)?.delete(cb);
    },
    emit(evt, payload) {
      listeners.get(evt)?.forEach((cb) => {
        try { cb(payload); } catch (e) { console.error('[NVS.bus]', evt, e); }
      });
    },
  };

  const g: NVSGlobal = {
    version: '3.10.0',
    get siteName() {
      try { return useAppStore().siteInfo?.site_name || document.title || ''; } catch { return document.title || ''; }
    },
    get dark() {
      return document.documentElement.getAttribute('data-theme') === 'dark';
    },
    get user() {
      try {
        const u = useAuthStore().user;
        return u ? { id: u.id, username: u.username, role: u.role } : null;
      } catch { return null; }
    },
    get route() {
      try {
        const r = (window as any).__nvsRouter__?.currentRoute?.value;
        return { path: r?.path || location.pathname, name: r?.name || null };
      } catch { return { path: location.pathname, name: null }; }
    },
    bus,
    on: bus.on,
    off: bus.off,
    emit: bus.emit,
    api: {
      get: (path, cfg) => api.get(path, cfg),
      post: (path, body, cfg) => api.post(path, body, cfg),
      put: (path, body, cfg) => api.put(path, body, cfg),
      del: (path, cfg) => api.delete(path, cfg),
    },
  };

  window.NVS = g;
  // alist 主题兼容 shim：alist 主题用 window.alist 读取站点状态/事件
  window.alist = g;
}
