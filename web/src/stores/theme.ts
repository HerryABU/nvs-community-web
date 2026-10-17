import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { themeApi } from '@/api/theme';
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
  // F12a 全站 opt-in iframe（管理员配置允许列表 URL，访客可读启用状态）
  const globalIframe = ref<{ enabled: boolean; url: string }>({ enabled: false, url: '' });

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
    }
    const me = settled[1];
    if (me && me.status === 'fulfilled' && me.value?.data?.code === 0) {
      myCss.value = me.value.data.data?.css || '';
      myJs.value = me.value.data.data?.js || '';
    }
    // 先安装 alist 兼容全局对象，再注入用户 JS（主题可立即监听 bus 事件）
    installNvsGlobal();
    applyCustomTheme();
    // F12a 拉取全站 opt-in iframe 启用状态（访客可读）
    loadGlobalIframe();
  }

  // 写入两个 <style> 标签注入主题 CSS。安全：textContent（CSS 不构 XSS），
  // 真正防线在服务端白名单消毒（utils.SanitizeCSS）。
  function applyCustomTheme(): void {
    writeStyleTag('nvs-site-theme', siteCss.value);
    writeStyleTag('nvs-my-theme', myCss.value);
    writeScriptTag('nvs-site-js', siteJs.value);
    writeScriptTag('nvs-my-js', myJs.value);
  }

  // 重置个人主题（本地即时生效；持久化删除由页面调用 updateMyTheme('', '') 完成）
  function resetMyTheme(): void {
    myCss.value = '';
    writeStyleTag('nvs-my-theme', '');
    myJs.value = '';
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
    siteJs, myJs,
  };
});

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
    version: '3.6.0',
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
