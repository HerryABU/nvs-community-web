import api from './index';

// F12 用户自定义网站主题（alist 式）：CSS 一律走服务端白名单消毒，
// JS 走注入安全处理（剥离 </script> 等逃逸序列，详见后端 utils.SanitizeJS），
// 前端仅以 <style>/<script> textContent 注入，绝不使用 innerHTML。

// 主题保存结果：消毒后的 CSS/JS + 被服务端移除的规则列表（前端提示用户）
export interface ThemeSaveResult {
  css: string;
  js: string;
  removed: string[];
}

export interface ThemeData {
  css: string;
  js: string;
}

export const themeApi = {
  // 站点默认主题（公开）：管理员配置，对所有访客生效；
  // 登录用户的个人主题（/theme/me）优先级更高
  getSiteTheme() {
    return api.get<{ code: number; data: ThemeData }>('/theme/site');
  },
  // 保存站点默认主题（仅管理员；对全站访客生效）。
  // 注意：必须打 /admin/theme/site——该 PUT 注册在服务端 admin 组（带鉴权中间件），
  // public 组的 /theme/site 只有 GET；漏掉 /admin 前缀会 404（gin httprouter 无此方法路由）。
  updateSiteTheme(css: string, js: string) {
    return api.put<{ code: number; data: ThemeSaveResult }>('/admin/theme/site', { css, js });
  },
  // 我的个人主题（需登录；未登录返回 401，由 store 静默处理）
  getMyTheme() {
    return api.get<{ code: number; data: ThemeData }>('/theme/me');
  },
  // 保存个人主题（需登录）：返回消毒后的 CSS/JS 与被移除的规则列表
  updateMyTheme(css: string, js: string) {
    return api.put<{ code: number; data: ThemeSaveResult }>('/theme/me', { css, js });
  },
};
