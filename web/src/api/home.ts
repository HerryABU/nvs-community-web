import api, { apiPrefix } from './index';

// ==================== 自定义主页区块（F15） ====================

export interface HomeBlock {
  id: number;
  /** hero 头图 / notice 公告栏 / modules 模块区 / blogs 博客 / forums 版块 / threads 最新帖 / custom 自定义HTML / links 链接组 */
  type: string;
  title: string;
  /** 按 type 解析的 JSON 配置 */
  config: string;
  sort_order: number;
  enabled: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface HomeBlockPreset {
  key: string;
  name: string;
  desc: string;
}

/** 公开：首页区块（仅启用，按排序；空库自动播种默认布局） */
export const homeBlockApi = {
  list() {
    return api.get<{ code: number; message: string; data: { items: HomeBlock[] } }>('/home-blocks');
  },
  /** 全站最新帖（论坛首页风格区块；公开论坛） */
  latestThreads(limit = 8) {
    return api.get<{ code: number; message: string; data: { items: any[] } }>('/threads/latest', {
      params: { limit },
    });
  },
};

/** 管理端：区块 CRUD / 排序 / 布局预设 */
export const adminHomeBlockApi = {
  list() {
    return api.get<{ code: number; message: string; data: { items: HomeBlock[] } }>('/admin/home-blocks');
  },
  presets() {
    return api.get<{ code: number; message: string; data: { items: HomeBlockPreset[] } }>(
      '/admin/home-block-presets'
    );
  },
  create(data: { type: string; title?: string; config?: string; enabled?: boolean; sort_order?: number }) {
    return api.post<{ code: number; message: string; data: HomeBlock }>('/admin/home-blocks', data);
  },
  update(id: number, data: { type?: string; title?: string; config?: string; enabled?: boolean; sort_order?: number }) {
    return api.put<{ code: number; message: string; data: HomeBlock }>(`/admin/home-blocks/${id}`, data);
  },
  remove(id: number) {
    return api.delete<{ code: number; message: string }>(`/admin/home-blocks/${id}`);
  },
  /** 按 ids 顺序重排 */
  reorder(ids: number[]) {
    return api.post<{ code: number; message: string }>('/admin/home-block-reorder', { ids });
  },
  /** 一键套用布局预设 */
  applyPreset(key: string) {
    return api.post<{ code: number; message: string; data: { items: HomeBlock[] } }>(
      `/admin/home-block-presets/${encodeURIComponent(key)}/apply`
    );
  },
};

/** 区块类型的展示元信息（管理端与调试用） */
export const HOME_BLOCK_TYPES: { type: string; name: string; desc: string }[] = [
  { type: 'hero', name: '头图', desc: '站点欢迎头图（标题/副标题/分类快捷入口）' },
  { type: 'notice', name: '公告栏', desc: '滚动展示站点公告（置顶优先）' },
  { type: 'modules', name: '模块区', desc: '小说/博客/论坛模块顺序排列（主模块在前）' },
  { type: 'blogs', name: '最新博客', desc: '博客列表区块（可设条数与列表/网格视图）' },
  { type: 'forums', name: '版块大厅', desc: '论坛版块卡片列表' },
  { type: 'threads', name: '最新帖', desc: '全站公开论坛的最新讨论帖' },
  { type: 'custom', name: '自定义 HTML', desc: '管理员自定义 HTML 片段（服务端消毒）' },
  { type: 'links', name: '链接组', desc: '快捷链接按钮组' },
];

export { apiPrefix };
