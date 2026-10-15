import { defineStore } from 'pinia';
import { ref } from 'vue';
import { batchGet } from '@/api/batch';
import { appPrefix } from '@/utils/prefix';

// 首屏落地页判定：浏览器地址与站点前缀（如 /go-nvs）一致即视为「首页」。
// 仅对初始加载成立——这正是 bootstrap 的执行时机；SPA 内后续进入首页由 Home 走 ensureHomeLoaded 兜底。
function isLandingPath(): boolean {
  const p = window.location.pathname;
  const prefix = appPrefix || '';
  const withSlash = prefix.endsWith('/') ? prefix : prefix + '/';
  return p === prefix || p === withSlash || p === withSlash + 'index.html' || p === prefix + '/index.html';
}

interface HomeData {
  announcements: any[];
  algo: string;
  catStats: any[];
  novels: { list: any[]; total: number } | null;
}

// 应用级全局数据（导航栏 / 页脚等所有页面共享的"外壳"数据 + 首页内容）。
// 通过一次 /api/batch 并发拉取，避免每个页面/组件各自发独立请求。
export const useAppStore = defineStore('app', () => {
  const siteInfo = ref<any>(null); // /api/site-info 完整返回（含备案号等）
  const siteName = ref('星海文学');
  const categories = ref<string[]>([]);
  const homeModule = ref<string>('novels'); // 首页主模块（管理员后台可配置：novels/blogs/forums/combined）
  const federatedSites = ref<any[]>([]);
  const unreadCount = ref(0);
  const installed = ref(true);
  const homeData = ref<HomeData | null>(null);

  let bootstrapPromise: Promise<void> | null = null;
  let homePromise: Promise<void> | null = null;

  function homeRequests() {
    return [
      { id: 'ann', path: '/api/announcements' },
      { id: 'algo', path: '/api/algorithms/description' },
      { id: 'catStats', path: '/api/categories/stats' },
      { id: 'novels', path: '/api/novels', query: { page: '1', page_size: '60', sort_by: 'featured' } },
    ];
  }

  function applyHome(map: Record<string, any>) {
    homeData.value = {
      announcements: map.ann?.success && Array.isArray(map.ann.data) ? map.ann.data : [],
      algo: map.algo?.success && map.algo.data?.description ? map.algo.data.description : '',
      catStats: map.catStats?.success && Array.isArray(map.catStats.data) ? map.catStats.data : [],
      novels: map.novels?.success ? map.novels.data : null,
    };
  }

  // 一次性并发拉取外壳数据；若首屏为落地页则首页内容并入同一批 → 首屏严格 1 次请求。
  // 幂等（promise 缓存），可在 App / NavBar / SiteFooter 多处安全调用。
  function bootstrap(): Promise<void> {
    if (bootstrapPromise) return bootstrapPromise;
    const includeHome = isLandingPath();
    const requests: any[] = [
      { id: 'site', path: '/api/site-info' },
      { id: 'cats', path: '/api/categories' },
      { id: 'fed', path: '/api/federated/sites' },
      { id: 'notif', path: '/api/notifications/unread-count' },
      { id: 'setup', path: '/api/setup/status' },
    ];
    if (includeHome) requests.push(...homeRequests());
    bootstrapPromise = (async () => {
      try {
        const map = await batchGet(requests);
        if (map.site?.success && map.site.data) {
          siteInfo.value = map.site.data;
          if (map.site.data.site_name) siteName.value = map.site.data.site_name;
          if (map.site.data.home_module) homeModule.value = map.site.data.home_module;
        }
        if (map.cats?.success && Array.isArray(map.cats.data)) categories.value = map.cats.data;
        if (map.fed?.success && map.fed.data) federatedSites.value = map.fed.data || [];
        if (map.notif?.success && typeof map.notif.data?.count === 'number') {
          unreadCount.value = map.notif.data.count;
        }
        if (map.setup?.success && map.setup.data) installed.value = !!map.setup.data.installed;
        if (includeHome) applyHome(map);
      } catch (e) {
        console.error('[app bootstrap] failed', e);
      }
    })();
    return bootstrapPromise;
  }

  // 首页内容就绪：首屏由 bootstrap 一并带回；SPA 内后进入首页则自行拉取一次（幂等）。
  async function ensureHomeLoaded(): Promise<void> {
    if (homeData.value) return;
    if (bootstrapPromise) await bootstrapPromise;
    if (homeData.value) return; // bootstrap 已包含首页内容
    if (!homePromise) {
      homePromise = (async () => {
        try {
          applyHome(await batchGet(homeRequests()));
        } catch (e) {
          console.error('[home load] failed', e);
        }
      })();
    }
    await homePromise;
  }

  // 登录态变化 / 消息已读后刷新未读徽标（30s 轮询也调用）
  async function refreshUnread() {
    try {
      const map = await batchGet([{ id: 'notif', path: '/api/notifications/unread-count' }]);
      if (map.notif?.success && typeof map.notif.data?.count === 'number') {
        unreadCount.value = map.notif.data.count;
      }
    } catch {
      /* 忽略，保持旧值 */
    }
  }

  return {
    siteInfo, siteName, categories, homeModule, federatedSites, unreadCount, installed, homeData,
    bootstrap, ensureHomeLoaded, refreshUnread,
  };
});
