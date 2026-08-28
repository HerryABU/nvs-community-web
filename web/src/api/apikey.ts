import api from './index';

// 用户侧 API Key 管理（个人设置页）
export const apiKeyApi = {
  // 我的 Key 列表 + 当前角色配额上限
  list() {
    return api.get('/apikeys');
  },
  // 创建（完整 Key 仅返回一次）
  create(data: { name?: string; quota_per_min?: number; allowed_ips?: string; expires_at?: string }) {
    return api.post('/apikeys', data);
  },
  update(id: number, data: { name?: string; quota_per_min?: number; allowed_ips?: string; expires_at?: string; is_active?: boolean }) {
    return api.patch(`/apikeys/${id}`, data);
  },
  regenerate(id: number) {
    return api.post(`/apikeys/${id}/regenerate`);
  },
  remove(id: number) {
    return api.delete(`/apikeys/${id}`);
  },
};
