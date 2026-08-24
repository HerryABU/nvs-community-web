import axios from 'axios';
import { appPrefix } from '@/utils/prefix';

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

// 响应拦截器：统一错误处理
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response) {
      const { status, data } = error.response;
      // 登录页路径需带前缀（前缀挂载时 /login 不带 /go-nvs 会 404）
      const loginPath = apiPrefix + '/login';
      switch (status) {
        case 401:
          // 未认证，跳转登录
          if (window.location.pathname !== loginPath) {
            window.location.href = loginPath;
          }
          break;
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