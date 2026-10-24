import axios from 'axios';
import { appPrefix } from '@/utils/prefix';
import { reportAdminApiError, clearAdminApiError } from '@/utils/adminHealth';

// 为 AxiosRequestConfig 扩展内部标记字段（仅本拦截器使用，不影响外部类型）
declare module 'axios' {
  interface AxiosRequestConfig {
    _retry?: boolean;
  }
}

// 🔀 站点路径前缀支持（URL_PREFIX，如 /go-nvs——名字可任意配置）：
// 经 getAppPrefix() 智能解析：后端注入的 window.__NVS_PREFIX__ 优先，
// 自建反向代理只改写 <base href> 时自动从 document.baseURI 推导（无硬编码）。
// API 请求统一加前缀，保证经 nginx 只暴露 /{prefix}/ 前缀时也能正确到达后端。
// 导出供 frame.ts / window.open / <a :href> 等裸 URL 使用点拼接。
export const apiPrefix: string = appPrefix;

const api = axios.create({
  baseURL: apiPrefix ? `${apiPrefix}/api` : '/api',
  withCredentials: true,
  // 通用 postForm 方法：不手动设置 Content-Type，
  // 让浏览器自动生成带 boundary 的 multipart/form-data（手动设置会丢失 boundary 导致解析失败）
  postForm(url: string, formData: FormData) {
    return this.post(url, formData);
  },
  timeout: 15000,
});

// 请求拦截器：统一附加 CSRF 防护头（安全审计：配合后端 CSRFGuard 校验，
// 登录态写操作必须携带 X-Requested-With，跨站表单/脚本无法伪造）
api.interceptors.request.use(
  (config) => {
    config.headers = config.headers || {};
    config.headers['X-Requested-With'] = 'XMLHttpRequest';
    return config;
  },
  (error) => Promise.reject(error)
);

// axios 的 config.url 是**相对路径**（如 /admin/risk），baseURL（/api）不在里面。
// 判断「哪个 API 分组」时必须拼全，否则 /admin 与 /police 全被当成同一组、
// 任何成功都会清掉失败告警（这个坑实测让横幅时有时无）。
function fullUrl(cfg?: { baseURL?: string; url?: string }): string {
  if (!cfg) return '';
  return `${cfg.baseURL || ''}${cfg.url || ''}`;
}

// 响应拦截器：统一错误处理 + 访问令牌过期无感续期
// 🔒 安全设计：访问令牌（token Cookie）短期有效，过期返回 401 时，
// 拦截器静默调用 /api/auth/refresh（携带 HttpOnly 刷新令牌 Cookie）换发新令牌，
// 成功后自动重试原请求；仅当刷新也失败才跳转登录。全程不暴露令牌给 JS，防 XSS 窃取。
let isRefreshing = false;
// 并发 401 时排队，等刷新完成后统一重试，避免重复刷新
let refreshSubscribers: Array<(ok: boolean) => void> = [];

function subscribeTokenRefresh(cb: (ok: boolean) => void) {
  refreshSubscribers.push(cb);
}
function onRefreshed(ok: boolean) {
  refreshSubscribers.forEach((cb) => cb(ok));
  refreshSubscribers = [];
}

async function silentRefresh(): Promise<boolean> {
  if (isRefreshing) {
    // 已有刷新在途，等待其结果
    return new Promise<boolean>((resolve) => subscribeTokenRefresh(resolve));
  }
  isRefreshing = true;
  try {
    // 动态导入避免与 auth.ts（其又导入本文件）形成静态循环依赖
    const { authApi } = await import('@/api/auth');
    await authApi.refreshToken();
    isRefreshing = false;
    onRefreshed(true);
    return true;
  } catch {
    isRefreshing = false;
    onRefreshed(false);
    // 刷新失败：仅清理本地登录态，不强制整页跳登录（跳转决策交给路由守卫）。
    try {
      const { useAuthStore } = await import('@/stores/auth');
      useAuthStore().user = null;
    } catch {
      /* 忽略 store 异常 */
    }
    return false;
  }
}

api.interceptors.response.use(
  (response) => {
    // 同一 API 分组内成功才算恢复：后台页会并发请求 /api/setup/status 等公共接口，
    // 那些成功不该抹掉 admin 接口的失败告警（否则横幅一闪就没）。
    clearAdminApiError(fullUrl(response.config));
    return response;
  },
  async (error) => {
    const { config, response } = error;
    // 硬失败（网络错误 / 5xx）统一登记：后台页的 catch 里基本只打日志或弹 toast，
    // 用户实际看到的是"表格空白"。由 AdminHeader / AdminApiAlert 常驻提示 + 提供重试。
    //
    // ⚠️ 不要只匹配 /admin/：后台页也会调 /api/police/tickets（警方联动工单）、
    // /api/payments 等非 admin 前缀接口，限定前缀会漏掉整整一类页面（实测漏了工单页）。
    // 这条状态只在后台页面渲染，公共页面调这些接口失败不会弹给普通用户。
    const failedUrl = fullUrl(config);
    if (!response || response.status >= 500) {
      reportAdminApiError(
        failedUrl,
        response?.status || 0,
        response?.data?.message || error?.message || '网络异常',
      );
    }
    if (response) {
      const { status, data } = response;
      switch (status) {
        case 401: {
          // 刷新端点自身 401 不再递归刷新，直接放行到下方拒绝逻辑
          const url = config?.url || '';
          if (url.includes('/auth/refresh')) {
            break;
          }
          // 避免同一请求重复触发刷新
          if (config && !config._retry) {
            config._retry = true;
            const ok = await silentRefresh();
            if (ok && config) {
              // 续期成功，重试原请求
              return api.request(config);
            }
          }
          // 续期失败：仅拒绝该请求，不强制整页跳登录。
          // 整页跳转登录只应由路由守卫（requiresAuth 路由）决策，
          // 否则公共页面（论坛/帖子）上任意辅助接口 401（如 /api/theme/me）
          // 都会把用户劫持到登录页，导致「帖子/回复看不见」。
          break;
        }
        case 403:
          console.warn('无权限:', data?.message);
          break;
        case 429:
          console.warn('请求过于频繁，请稍后再试');
          break;
      }
    }
    return Promise.reject(error);
  }
);

export default api;