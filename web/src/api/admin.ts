import api from './index';

export const adminApi = {
  // 平台配置
  getConfig() {
    return api.get('/admin/config');
  },
  updateConfig(data: Record<string, string>) {
    return api.put('/admin/config', data);
  },

  // 统计数据
  getStats() {
    return api.get('/admin/stats');
  },

  // 仪表盘数据大屏
  getDashboardStats() {
    return api.get('/admin/dashboard');
  },
  getCommunity() {
    return api.get('/admin/community');
  },

  // 作者仪表盘数据
  getAuthorDashboard() {
    return api.get('/author/dashboard');
  },

  // 用户管理
  getUsers(page: number = 1, search?: string) {
    return api.get('/admin/users', { params: search ? { page, search } : { page } });
  },
  updateUser(id: number, data: { role?: string; nickname?: string; email?: string }) {
    return api.put(`/admin/users/${id}`, data);
  },
  deleteUser(id: number) {
    return api.delete(`/admin/users/${id}`);
  },

  // VIP 申请
  getVipApplications() {
    return api.get('/admin/vip-applications');
  },
  approveVip(id: number) {
    return api.post(`/admin/vip-applications/${id}/approve`);
  },

  // 举报
  getReports() {
    return api.get('/admin/reports');
  },
  handleReport(id: number, data: { status: string; verdict: string }) {
    return api.post(`/admin/reports/${id}/handle`, data);
  },

  // 财务
  getFinance() {
    return api.get('/admin/finance');
  },

  // 论坛管理
  getForums(type?: string) {
    return api.get('/admin/forums', { params: { type } });
  },
  createForum(data: { name: string; description: string; type: string; zone?: string; ref_id?: string; sort_order?: number }) {
    return api.post('/admin/forums', data);
  },
  updateForum(id: number, data: Record<string, any>) {
    return api.put(`/admin/forums/${id}`, data);
  },
  deleteForum(id: number) {
    return api.delete(`/admin/forums/${id}`);
  },

  // 隔离墙配置
  getWallConfig() {
    return api.get('/admin/wall-config');
  },
  updateWallConfig(data: { zones: string[]; zone_details?: any[]; enabled: boolean; cross_domain_warning: boolean }) {
    return api.put('/admin/wall-config', data);
  },

  // 远程站点互通
  getSites() {
    return api.get('/admin/sites');
  },
  createSite(data: { name: string; url: string; api_url: string; description?: string }) {
    return api.post('/admin/sites', data);
  },
  updateSite(id: number, data: Record<string, string>) {
    return api.put(`/admin/sites/${id}`, data);
  },
  deleteSite(id: number) {
    return api.delete(`/admin/sites/${id}`);
  },
  syncSite(id: number) {
    return api.post(`/admin/sites/${id}/sync`);
  },
};

// 公开接口
export const publicApi = {
  getSiteInfo() {
    return api.get('/site-info');
  },
  getFederatedSites() {
    return api.get('/federated/sites');
  },
  getFederatedNovels(params?: { page?: number; page_size?: number; site_id?: number }) {
    return api.get('/federated/novels', { params });
  },
  getCategories() {
    return api.get<string[]>('/categories');
  },
  getCategoryStats() {
    return api.get('/categories/stats');
  },
  getAuthorProfile(id: number) {
    return api.get(`/author/profile/${id}`);
  },
  getAuthorForum(id: number) {
    return api.get(`/author/forum/${id}`);
  },
};