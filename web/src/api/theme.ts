import api from './index';

// F12 用户自定义网站主题（alist 式）：CSS 一律走服务端白名单消毒，
// 前端仅以 <style> textContent 注入，不使用 innerHTML。

// 主题保存结果：消毒后的 CSS + 被服务端移除的规则列表（前端提示用户）
export interface ThemeSaveResult {
  css: string;
  removed: string[];
}

export const themeApi = {
  // 站点默认主题（公开）：管理员配置，对所有访客生效；
  // 登录用户的个人主题（/theme/me）优先级更高
  getSiteTheme() {
    return api.get<{ code: number; data: { css: string } }>('/theme/site');
  },
  // 保存站点默认主题（仅管理员；对全站访客生效）
  updateSiteTheme(css: string) {
    return api.put<{ code: number; data: ThemeSaveResult }>('/theme/site', { css });
  },
  // 我的个人主题（需登录；未登录返回 401，由 store 静默处理）
  getMyTheme() {
    return api.get<{ code: number; data: { css: string } }>('/theme/me');
  },
  // 保存个人主题（需登录）：返回消毒后的 CSS 与被移除的规则列表
  updateMyTheme(css: string) {
    return api.put<{ code: number; data: ThemeSaveResult }>('/theme/me', { css });
  },
};
