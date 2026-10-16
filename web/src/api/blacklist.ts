import api from './index';

// 黑名单模式：block=禁止在其评论区评论 / hide=屏蔽其内容不看
export type BlacklistMode = 'block' | 'hide';

export interface BlacklistItem {
  id: number;
  user_id: number;
  blocked_user_id: number;
  mode: BlacklistMode;
  reason: string;
  created_at: string;
  // 运行时由后端 JOIN 带出的对方用户信息
  blocked_username: string;
  blocked_nickname: string;
  blocked_avatar_url: string;
}

export interface BlacklistListResult {
  list: BlacklistItem[];
  total: number;
  page: number;
}

export const blacklistApi = {
  // 添加/更新黑名单（user_key 支持用户名或数字 id；已拉黑则更新模式）
  add(data: { user_key?: string; user_id?: number; mode?: BlacklistMode; reason?: string }) {
    return api.post<{ code: number; data: { message: string; id: number; mode: BlacklistMode } }>('/blacklist', data);
  },
  // 解除拉黑
  remove(id: number) {
    return api.delete(`/blacklist/${id}`);
  },
  // 我的黑名单分页列表
  list(params?: { page?: number; page_size?: number }) {
    return api.get<{ code: number; data: BlacklistListResult }>('/blacklist', { params });
  },
};
