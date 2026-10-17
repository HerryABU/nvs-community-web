import api from '@/api/index';
import type { Comment } from './types';

// 楼加密解锁态：楼层 id → 访问密码（sessionStorage 持久化，关页即焚）。
// 读取加密楼评论/楼详情时以 X-Floor-Password 头携带（密码不进 URL）。
const FLOOR_PWD_KEY = 'nvs:floor-passwords';

export function getFloorPasswords(): Record<string, string> {
  try {
    return JSON.parse(sessionStorage.getItem(FLOOR_PWD_KEY) || '{}');
  } catch {
    return {};
  }
}
export function setFloorPassword(floorId: number, password: string) {
  const all = getFloorPasswords();
  if (password) all[String(floorId)] = password;
  else delete all[String(floorId)];
  sessionStorage.setItem(FLOOR_PWD_KEY, JSON.stringify(all));
}
export function clearFloorPassword(floorId: number) {
  setFloorPassword(floorId, '');
}

// 收集当前页面已解锁的加密楼密码 → X-Floor-Password 请求头。
// 多楼格式：`id:password,id:password`（后端按 id 匹配）；密码不含逗号/冒号（设置时已校验）。
export function floorPasswordHeaders(items: any[]): Record<string, string> {
  const all = getFloorPasswords();
  const parts: string[] = [];
  for (const it of items || []) {
    if (it?.has_password && all[String(it.id)]) {
      parts.push(`${it.id}:${all[String(it.id)]}`);
    }
  }
  return parts.length ? { 'X-Floor-Password': parts.join(',') } : {};
}

export const commentApi = {
  // 楼层模型：list = 本页楼层 + 这些楼层下的全部回复（平铺，前端按 root_id 分组成「楼」）
  getComments(params: { novel_id: number; blog_id?: number; chapter_number?: number; page?: number; items?: any[] }) {
    const { items, ...query } = params;
    return api.get<{
      code: number;
      data: { list: Comment[]; total: number; floor_total: number; has_more: boolean; page: number };
    }>('/comments', {
      params: query,
      headers: floorPasswordHeaders(items),
    });
  },

  // 楼加密：验证访问密码（前端 🔒 占位解锁）
  verifyFloorPassword(id: number, password: string) {
    return api.post<{ code: number; data: { ok: boolean; has_password: boolean } }>(
      `/comments/${id}/verify-password`,
      { password },
    );
  },
  // 楼加密：设置/修改/解除访问密码（空密码=解除；仅楼作者或管理员）
  setFloorPassword(id: number, password: string) {
    return api.post<{ code: number; data: { ok: boolean; has_password: boolean; message?: string } }>(
      `/comments/${id}/password`,
      { password },
    );
  },
  // 楼加密：单楼整读（楼主贴 + 全部楼内回复），凭 X-Floor-Password 头解锁
  getFloorDetail(id: number) {
    const pwd = getFloorPasswords()[String(id)];
    return api.get<{
      code: number;
      data: { floor: Comment; replies: Comment[] };
    }>(`/comments/${id}/detail`, {
      headers: pwd ? { 'X-Floor-Password': pwd } : {},
    });
  },

  // parent_id 留空 = 回复主内容（开新楼）；传值 = 回复该条（挂靠其所属楼）。
  // headers 用于加密楼内回复携带 X-Floor-Password（解锁密码）。
  createComment(data: {
    novel_id: number;
    blog_id?: number;
    chapter_number?: number;
    content: string;
    quote_text?: string;
    parent_id?: number;
  }, headers?: Record<string, string>) {
    return api.post('/comments', data, { headers: headers || {} });
  },

  deleteComment(id: number) {
    return api.delete(`/comments/${id}`);
  },
};
