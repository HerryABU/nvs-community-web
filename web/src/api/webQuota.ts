import api from './index';

// Web 项目发布额度 + 超额审核
// 注意：baseURL 已含前缀（apiPrefix+/api），严禁在此硬编码 /go-nvs。
export const webQuotaApi = {
  // 用户侧：查看本人额度 / 已用 / 剩余 / 待审单
  myQuota() {
    return api.get('/web-quota/mine');
  },
  // 用户侧：把某 HTML 扩展设为公开（带额度判断）
  setHtmlPublic(id: number, isPublic: boolean) {
    return api.post('/web-quota/html-public', { id, is_public: isPublic });
  },
  // 用户侧：把某 UI 模板设为公开（带额度判断）
  setFramePublic(id: number, isPublic: boolean) {
    return api.post('/web-quota/frame-public', { id, is_public: isPublic });
  },

  // 管理端：超额待审列表（status 可选 pending/approved/rejected）
  listRequests(status?: string) {
    return api.get('/admin/web-quota/requests', { params: status ? { status } : {} });
  },
  // 管理端：批准超额公开
  approveRequest(id: number, note?: string) {
    return api.post(`/admin/web-quota/requests/${id}/approve`, { note: note || '' });
  },
  // 管理端：驳回超额公开
  rejectRequest(id: number, note?: string) {
    return api.post(`/admin/web-quota/requests/${id}/reject`, { note: note || '' });
  },

  // 管理端：额度配置（全局默认 + 按人覆盖）
  getConfig() {
    return api.get('/admin/web-quota/config');
  },
  // action: global / set_user / remove_user
  setConfig(data: {
    action: 'global' | 'set_user' | 'remove_user';
    max_per_user?: number;
    user_id?: number;
    quota?: number;
    note?: string;
  }) {
    return api.put('/admin/web-quota/config', data);
  },
};
