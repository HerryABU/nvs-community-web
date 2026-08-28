import { defineStore } from 'pinia';
import { ref } from 'vue';
import { batchGet } from '@/api/batch';

// 应用级全局数据（导航栏 / 页脚等所有页面共享的"外壳"数据）。
// 通过一次 /api/batch 并发拉取，避免每个页面/组件各自发独立请求。
export const useAppStore = defineStore('app', () => {
  const siteInfo = ref<any>(null); // /api/site-info 完整返回（含备案号等）
  const siteName = ref('星海文学');
  const categories = ref<string[]>([]);
  const federatedSites = ref<any[]>([]);
  const unreadCount = ref(0);
  const installed = ref(true);
  const loaded = ref(false);

  // 一次性并发拉取外壳数据；幂等（已加载则跳过），可在 App / NavBar 多处安全调用。
  async function bootstrap() {
    if (loaded.value) return;
    loaded.value = true;
    try {
      const map = await batchGet([
        { id: 'site', path: '/api/site-info' },
        { id: 'cats', path: '/api/categories' },
        { id: 'fed', path: '/api/federated/sites' },
        { id: 'notif', path: '/api/notifications/unread-count' },
        { id: 'setup', path: '/api/setup/status' },
      ]);
      if (map.site?.success && map.site.data) {
        siteInfo.value = map.site.data;
        if (map.site.data.site_name) siteName.value = map.site.data.site_name;
      }
      if (map.cats?.success && Array.isArray(map.cats.data)) categories.value = map.cats.data;
      if (map.fed?.success && map.fed.data) federatedSites.value = map.fed.data || [];
      if (map.notif?.success && typeof map.notif.data?.count === 'number') {
        unreadCount.value = map.notif.data.count;
      }
      if (map.setup?.success && map.setup.data) installed.value = !!map.setup.data.installed;
    } catch (e) {
      console.error('[app bootstrap] failed', e);
    }
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

  return { siteInfo, siteName, categories, federatedSites, unreadCount, installed, loaded, bootstrap, refreshUnread };
});
