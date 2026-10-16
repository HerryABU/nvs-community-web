import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { themeApi } from '@/api/theme';
import api from '@/api';
import { useAuthStore } from '@/stores/auth';

export const useThemeStore = defineStore('theme', () => {
  // ===== 明暗模式（既有功能：data-theme 属性切换，勿删） =====
  const isDark = ref(localStorage.getItem('nvs-theme') === 'dark');

  const applyTheme = () => {
    document.documentElement.setAttribute('data-theme', isDark.value ? 'dark' : 'light');
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
  // F12a 全站 opt-in iframe（管理员配置允许列表 URL，访客可读启用状态）
  const globalIframe = ref<{ enabled: boolean; url: string }>({ enabled: false, url: '' });

  // 生效 CSS：个人主题优先；两者均挂载时 my 后写入即覆盖 site
  const effectiveCss = computed(() => (myCss.value.trim() ? myCss.value : siteCss.value));

  // 拉取站点主题 +（已登录时）个人主题。
  // 未登录时不请求 /theme/me：后端返回 401 会被全局拦截器带去刷新令牌并跳登录页，
  // 匿名访客访问首页不应被弹去登录（App.vue 接线只需调用本 action 一次）。
  async function loadTheme(): Promise<void> {
    const auth = useAuthStore();
    // 登录态未就绪时先恢复，保证已登录用户的个人主题不丢
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
    }
    const me = settled[1];
    if (me && me.status === 'fulfilled' && me.value?.data?.code === 0) {
      myCss.value = me.value.data.data?.css || '';
    }
    applyCustomTheme();
    // F12a 拉取全站 opt-in iframe 启用状态（访客可读）
    loadGlobalIframe();
  }

  // 写入两个 <style> 标签注入主题 CSS。
  // 安全说明：用 textContent 而非 innerHTML——CSS 文本不会构成 XSS，
  // 真正的安全防线在服务端白名单消毒（utils.SanitizeCSS）。
  function applyCustomTheme(): void {
    writeStyleTag('nvs-site-theme', siteCss.value);
    writeStyleTag('nvs-my-theme', myCss.value);
  }

  // 重置个人主题（本地即时生效；持久化删除由页面调用 updateMyTheme('') 完成）
  function resetMyTheme(): void {
    myCss.value = '';
    writeStyleTag('nvs-my-theme', '');
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
