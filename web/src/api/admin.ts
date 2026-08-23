import api from './index';

export const adminApi = {
  // 平台配置
  getConfig() {
    return api.get('/admin/config');
  },
  updateConfig(data: Record<string, string>) {
    return api.put('/admin/config', data);
  },

  // SMTP 测试与诊断
  testSmtp(payload: { to: string; host?: string; port?: string; user?: string; password?: string; from?: string }) {
    return api.post('/admin/smtp/test', payload);
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
  getUserAuditLogs(params?: { user_id?: number; action?: string }) {
    return api.get('/admin/users/audit', { params });
  },
  updateUser(id: number, data: { role?: string; nickname?: string; email?: string }) {
    return api.put(`/admin/users/${id}`, data);
  },
  deleteUser(id: number) {
    return api.delete(`/admin/users/${id}`);
  },
  muteUser(id: number, data: { duration_hours?: number; reason?: string }) {
    return api.post(`/admin/users/${id}/mute`, data);
  },
  unmuteUser(id: number) {
    return api.post(`/admin/users/${id}/unmute`);
  },

  // VIP 申请
  getVipApplications() {
    return api.get('/admin/vip-applications');
  },
  approveVip(id: number) {
    return api.post(`/admin/vip-applications/${id}/approve`);
  },

  // 举报
  getReports(status?: string) {
    return api.get('/admin/reports', { params: status ? { status } : {} });
  },
  handleReport(id: number, data: { status: string; verdict: string; delete_target?: boolean }) {
    return api.post(`/admin/reports/${id}/handle`, data);
  },

  // HTML 内容审查
  getHtmlReviews(params?: { status?: string; type?: string }) {
    return api.get('/admin/html-reviews', { params });
  },
  getHtmlReview(type: string, id: number) {
    return api.get(`/admin/html-reviews/${type}/${id}`);
  },
  reviewHtml(type: string, id: number, data: { action: string; note?: string }) {
    return api.post(`/admin/html-reviews/${type}/${id}/review`, data);
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
  reviewForum(id: number, data: { action: string; note?: string }) {
    return api.post(`/admin/forums/${id}/review`, data);
  },

  // XSS 正则规则库
  getXssRules() {
    return api.get('/admin/xss-rules');
  },
  createXssRule(data: { name: string; pattern: string; action?: string; replacement?: string; enabled?: boolean; description?: string }) {
    return api.post('/admin/xss-rules', data);
  },
  updateXssRule(id: number, data: { name?: string; pattern?: string; action?: string; replacement?: string; enabled?: boolean; description?: string }) {
    return api.put(`/admin/xss-rules/${id}`, data);
  },
  deleteXssRule(id: number) {
    return api.delete(`/admin/xss-rules/${id}`);
  },
  testXssRule(data: { pattern: string; action?: string; replacement?: string; text: string }) {
    return api.post('/admin/xss-rules/test', data);
  },

  // 违禁词库（标记/删除/替换 + 关键词/正则双模式 + 可配置适用范围）
  getBadWords() {
    return api.get('/admin/bad-words');
  },
  createBadWord(data: { name: string; match_type?: string; pattern: string; action?: string; replacement?: string; scope?: string; enabled?: boolean; description?: string }) {
    return api.post('/admin/bad-words', data);
  },
  updateBadWord(id: number, data: { name?: string; match_type?: string; pattern?: string; action?: string; replacement?: string; scope?: string; enabled?: boolean; description?: string }) {
    return api.put(`/admin/bad-words/${id}`, data);
  },
  deleteBadWord(id: number) {
    return api.delete(`/admin/bad-words/${id}`);
  },
  testBadWord(data: { match_type?: string; pattern: string; action?: string; replacement?: string; text: string }) {
    return api.post('/admin/bad-words/test', data);
  },
  // 违禁词标记待审
  getBadWordFlags(params: { status?: string; page?: number; page_size?: number }) {
    return api.get('/admin/bad-word-flags', { params });
  },
  handleBadWordFlag(id: number, data: { status: string; note?: string; delete_content?: boolean }) {
    return api.put(`/admin/bad-word-flags/${id}`, data);
  },

  // 内容抢救台（二级密码 + 应急预览/软删除留证）
  setEmergencyPassword(data: { old_password?: string; new_password: string }) {
    return api.post('/admin/emergency/set-password', data);
  },
  unlockEmergency(data: { password: string }) {
    return api.post('/admin/emergency/unlock', data);
  },
  previewEmergencyContent(path: string, token: string) {
    return api.get('/admin/emergency/preview', { params: { path, token } });
  },
  deleteEmergencyContent(path: string, token: string) {
    return api.delete('/admin/emergency/content', { params: { path, token } });
  },
  getEmergencyAudit() {
    return api.get('/admin/emergency/audit');
  },

  // 内容治理（软删除 + softdel_ 留证）
  getSoftDel(type?: string) {
    return api.get('/admin/softdel', { params: type ? { type } : {} });
  },
  moderateContent(data: { type: string; id: number; reason?: string; action?: string }) {
    return api.post('/admin/softdel', data);
  },
  restoreContent(data: { type: string; id: number }) {
    return api.post('/admin/softdel/restore', data);
  },

  // 回收站监管（管理员查看全站用户自删内容）
  getAdminRecycle(type?: string) {
    return api.get('/admin/recycle', { params: type ? { type } : {} });
  },

  // 博客分类管理（一级/二级）
  getBlogCategories() {
    return api.get('/admin/blog-categories');
  },
  createBlogCategory(data: { name: string; parent_id?: number; sort_order?: number }) {
    return api.post('/admin/blog-categories', data);
  },
  updateBlogCategory(id: number, data: { name?: string; parent_id?: number; sort_order?: number }) {
    return api.put(`/admin/blog-categories/${id}`, data);
  },
  deleteBlogCategory(id: number) {
    return api.delete(`/admin/blog-categories/${id}`);
  },
  // 分类审核（通过/拒绝；拒绝即封禁下架，可填原因）
  reviewBlogCategory(id: number, action: 'approve' | 'reject', reason?: string) {
    return api.post(`/admin/blog-categories/${id}/review`, { action, reason });
  },
  // 审核模式：post_review（事后审核，通过后公开）/ pre_public（先公开，事后封禁）
  getCategoryReviewMode() {
    return api.get('/admin/blog-categories/mode');
  },
  setCategoryReviewMode(mode: 'post_review' | 'pre_public') {
    return api.put('/admin/blog-categories/mode', { mode });
  },

  // 小说/章节封禁管理（管理员视角，含封禁状态）
  getAdminNovels(params?: { search?: string; page?: number; page_size?: number }) {
    return api.get('/admin/novels', { params });
  },
  getAdminChapters(novelId: number) {
    return api.get(`/admin/novels/${novelId}/chapters`);
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
  getInboundSites() {
    return api.get('/admin/sites/inbound');
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

  // 数据库管理（健康检查 / 迁移 / 备份 / 恢复 / 定时备份）
  getDbHealth() {
    return api.get('/admin/db/health');
  },
  getDbCurrent() {
    return api.get('/admin/db/current');
  },
  migrateDb(data: { driver: string; host?: string; port?: string; user?: string; password?: string; name?: string; path?: string }) {
    return api.post('/admin/db/migrate', data);
  },
  backupDb() {
    return api.post('/admin/db/backup');
  },
  getDbJob(id: string) {
    return api.get(`/admin/db/job/${id}`);
  },
  getBackups() {
    return api.get('/admin/db/backups');
  },
  deleteBackup(name: string) {
    return api.delete(`/admin/db/backups/${encodeURIComponent(name)}`);
  },
  restoreDb(name: string) {
    return api.post('/admin/db/restore', { name });
  },
  getBackupConfig() {
    return api.get('/admin/db/backup-config');
  },
  updateBackupConfig(data: { enabled?: boolean; interval?: number }) {
    return api.put('/admin/db/backup-config', data);
  },
};

// 公开接口
export const publicApi = {
  // 全面健康检查（服务器/数据库/Redis/网络/业务API/前端）
  getHealth() {
    return api.get('/health');
  },
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
  getAuthorProfile(id: number | string) {
    // :id 兼容数字主键 或 用户名（username）
    return api.get(`/author/profile/${id}`);
  },
  // 用户标识解析：数字=主键 id，非数字=用户名（大小写不敏感）
  resolveUser(key: string) {
    return api.get<{ code: number; data: { id: number; username: string; nickname: string; role: string } }>('/users/resolve', { params: { key } });
  },
  getAuthorForum(id: number) {
    return api.get(`/author/forum/${id}`);
  },
};