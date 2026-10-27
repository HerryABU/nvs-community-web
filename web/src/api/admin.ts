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

  // 日志脱敏配置（公安部令第176号合规：生产环境日志敏感信息脱敏）
  getLogRedactConfig() {
    return api.get('/admin/log-redact');
  },
  updateLogRedactConfig(data: { enabled?: boolean; mask?: string; sensitive_keys?: string[] }) {
    return api.put('/admin/log-redact', data);
  },
  previewLogRedact(data: { kind?: string; sample: string; enabled?: boolean; mask?: string; sensitive_keys?: string[] }) {
    return api.post('/admin/log-redact/preview', data);
  },

  // 算法推荐管理（176号令 第七条第九项）
  getAlgorithms() {
    return api.get('/admin/algorithms');
  },
  updateAlgorithms(data: { sort_rule?: string; rank_period?: string; personalized?: boolean; description?: string }) {
    return api.put('/admin/algorithms', data);
  },

  // 等保/关基识别记录（第七条第四/五项）
  getSecurityGrade() {
    return api.get('/admin/security-grade');
  },
  updateSecurityGrade(data: { grade?: string; note?: string }) {
    return api.put('/admin/security-grade', data);
  },

  // 漏洞整改管理（第七条第六项）
  getVulnerabilities(params?: { status?: string; severity?: string }) {
    return api.get('/admin/vulnerabilities', { params });
  },
  createVulnerability(data: { name: string; description?: string; severity: string; affected?: string; reported_by?: string }) {
    return api.post('/admin/vulnerabilities', data);
  },
  updateVulnerability(id: number, data: Record<string, any>) {
    return api.put(`/admin/vulnerabilities/${id}`, data);
  },
  deleteVulnerability(id: number) {
    return api.delete(`/admin/vulnerabilities/${id}`);
  },
  exportVulnerabilities() {
    return api.get('/admin/vulnerabilities/export', { responseType: 'blob' });
  },

  // 公告栏（管理端）
  getAnnouncements() {
    return api.get('/admin/announcements');
  },
  createAnnouncement(data: { title: string; content?: string; pinned?: boolean; published?: boolean }) {
    return api.post('/admin/announcements', data);
  },
  updateAnnouncement(id: number, data: Record<string, any>) {
    return api.put(`/admin/announcements/${id}`, data);
  },
  deleteAnnouncement(id: number) {
    return api.delete(`/admin/announcements/${id}`);
  },

  // 检查应对工具箱（176号令 5.2）
  getComplianceReport() {
    return api.get('/admin/compliance/report');
  },
  exportComplianceLogs(from: string, to: string) {
    return api.get('/admin/compliance/export-logs', { params: { from, to }, responseType: 'blob' });
  },
  // AI 路由策略（最便宜/最快/场景 + 失败降级）
  getAiRoute() {
    return api.get('/admin/ai/route');
  },
  updateAiRoute(data: Record<string, any>) {
    return api.put('/admin/ai/route', data);
  },
  exportComplianceUsers() {
    return api.get('/admin/compliance/export-users', { responseType: 'blob' });
  },
  /** 一键材料包：报告+算法说明+定级+漏洞+用户汇总+蜜罐摘要+日志清单，附 SHA256 清单 */
  exportComplianceBundle() {
    return api.get('/admin/compliance/bundle.zip', { responseType: 'blob' });
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

  // 用户管理（支持角色分类 + 状态多维筛选）
  getUsers(params: { page: number; search?: string; role?: string; status?: string }) {
    const q: Record<string, unknown> = { page: params.page };
    if (params.search) q.search = params.search;
    if (params.role) q.role = params.role;
    if (params.status) q.status = params.status;
    return api.get('/admin/users', { params: q });
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

  // 警方取管台（police role）：提权 / 撤销 / 取证查看 / 凭证下载
  promotePolice(id: number) {
    return api.post(`/admin/users/${id}/promote-police`);
  },
  revokePolice(id: number) {
    return api.post(`/admin/users/${id}/revoke-police`);
  },
  // 热词统计：内容热词(默认)/搜索热词/标签热词（内容治理与舆情观测）
  hotWords(params?: { days?: number; limit?: number; type?: 'content' | 'search' | 'tag' }) {
    return api.get<{
      code: number;
      message: string;
      data: {
        type: 'content' | 'search' | 'tag';
        words: { word: string; count: number }[];
        queries?: { word: string; count: number }[]; // type=search 时的原始搜索短语
        search_total?: number;                        // type=search 时的窗口内搜索总次数
        days: number;
        total: number;
      };
    }>('/admin/hotwords', { params });
  },
  // 维护搜索埋点日志：days>0 仅清理 N 天前，days=0/不传清空全部
  clearSearchLogs(days?: number) {
    return api.post<{ code: number; message: string; data: { removed: number } }>(
      '/admin/search-logs/cleanup',
      {},
      days && days > 0 ? { params: { days } } : undefined,
    );
  },
  // 实名记录（F10：姓名解密后仅在本响应返回，身份证仅脱敏展示）
  listRealName(params?: { page?: number; page_size?: number; keyword?: string; minor_only?: boolean }) {
    return api.get<{
      code: number; message: string; data: {
        list: {
          user_id: number; username: string; nickname: string; role: string; phone: string;
          real_name: string; id_card_masked: string; minor_protected: boolean; age: number;
          real_name_at: string | null;
        }[];
        total: number; minor_total: number; page: number; page_size: number;
      };
    }>('/admin/realname', { params });
  },
  // 实名核验提供商配置（国家代理/腾讯/阿里/华为/网易）
  getRealNameProvider() {
    return api.get<{
      code: number; message: string; data: {
        config: {
          provider: string;
          national: { url: string; token: string; name_field: string; card_field: string };
          tencent: { secret_id: string; secret_key: string; region: string };
          alibaba: { app_code: string; gateway: string; method: string; name_param: string; card_param: string };
          huawei: { ak: string; sk: string; endpoint: string; uri: string };
          netease: { business_id: string; secret_id: string; secret_key: string; url: string; algorithm: string };
        };
        has_secret: Record<string, boolean>;
        providers: string[];
      };
    }>('/admin/realname-provider');
  },
  setRealNameProvider(config: Record<string, unknown>) {
    return api.put('/admin/realname-provider', config);
  },
  testRealNameProvider(payload?: { real_name?: string; id_card?: string }) {
    return api.post<{ code: number; message: string; data: { ok: boolean; matched?: boolean; code?: string; message: string; hint?: string } }>(
      '/admin/realname-provider/test',
      payload || {},
    );
  },
  policeListSoftDel(params?: { type?: string; limit?: number }) {
    return api.get('/police/softdel', { params });
  },
  // 警方论坛巡查：全部论坛（含封锁/加密/隐藏/软删除），附帖子/回复统计
  policeListForums() {
    return api.get<{ code: number; message: string; data: { forums: any[]; total: number } }>('/police/forums');
  },
  policeGetEvidence(type: string, id: number) {
    return api.get(`/police/evidence/${type}/${id}`);
  },
  policeDownloadEvidence(type: string, id: number) {
    return api.get(`/police/evidence/${type}/${id}/download`, { responseType: 'blob' });
  },
  policeExportPackage() {
    return api.get('/police/export', { responseType: 'blob' });
  },

  // VIP 申请
  getVipApplications() {
    return api.get('/admin/vip-applications');
  },
  approveVip(id: number) {
    return api.post(`/admin/vip-applications/${id}/approve`);
  },

  // 举报
  getReports(status?: string, targetType?: string) {
    const params: Record<string, string> = {};
    if (status) params.status = status;
    if (targetType) params.target_type = targetType;
    return api.get('/admin/reports', { params });
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
  // 复核：publish=放行同时上广场；purge_file=驳回时连封存件一起删除
  reviewHtml(type: string, id: number, data: { action: string; note?: string; publish?: boolean; purge_file?: boolean }) {
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
  // 自建论坛额度：全局默认 + 按人覆盖
  getForumQuota() {
    return api.get('/admin/forum-quota');
  },
  updateForumQuota(maxPerUser: number) {
    return api.put('/admin/forum-quota', { max_per_user: maxPerUser });
  },
  listUserForumQuotas() {
    return api.get('/admin/forum-quota/users');
  },
  setUserForumQuota(userId: number, data: { max_forums: number; note?: string }) {
    return api.put(`/admin/forum-quota/users/${userId}`, data);
  },
  deleteUserForumQuota(userId: number) {
    return api.delete(`/admin/forum-quota/users/${userId}`);
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
  exportBadWords() {
    return api.get('/admin/bad-words/export');
  },
  importBadWords(data: any[]) {
    return api.post('/admin/bad-words/import', data);
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
  // 🔒 安全审计：应急 token 改走 POST body，不再进 URL query（防落入访问日志/浏览器历史）
  previewEmergencyContent(path: string, token: string) {
    return api.post('/admin/emergency/preview', { path, token });
  },
  deleteEmergencyContent(path: string, token: string) {
    return api.post('/admin/emergency/content', { path, token });
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

  // 隔离墙进入记录仪表盘
  getZoneAccessLogs(params?: { user_id?: number; zone?: string; from?: string; to?: string; page?: number; page_size?: number }) {
    return api.get('/admin/zone-access', { params });
  },
  getZoneAccessStats() {
    return api.get('/admin/zone-access/stats');
  },

  // 网络诊断 / 用户分布 / 攻击流量面板
  getWebAccessLogs(params?: {
    ip?: string; ua?: string; ua_family?: string; status?: number; is_bot?: boolean;
    method?: string; path?: string; identity?: string; attack?: boolean;
    from?: string; to?: string; page?: number; page_size?: number;
  }) {
    return api.get('/admin/web-access', { params });
  },
  getWebAccessStats() {
    return api.get('/admin/web-access/stats');
  },
  cleanupWebAccessLogs() {
    return api.post('/admin/web-access/cleanup');
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

  // 联邦站点 API Key（双向可见）
  getSiteKeys(id: number) {
    return api.get(`/admin/sites/${id}/keys`);
  },
  createInboundKey(id: number, data?: { quota_per_min?: number; allowed_ips?: string }) {
    return api.post(`/admin/sites/${id}/keys/inbound`, data || {});
  },
  saveOutboundKey(id: number, key: string) {
    return api.post(`/admin/sites/${id}/keys/outbound`, { key });
  },
  toggleSiteKey(id: number, kid: number) {
    return api.post(`/admin/sites/${id}/keys/${kid}/toggle`);
  },
  deleteSiteKey(id: number, kid: number) {
    return api.delete(`/admin/sites/${id}/keys/${kid}`);
  },

  // 限流与 API Key 管理
  getRateLimitConfig() {
    return api.get('/admin/ratelimit/config');
  },
  updateRateLimitConfig(data: Record<string, any>) {
    return api.put('/admin/ratelimit/config', data);
  },
  getRateLimitLogs(params?: { type?: string; page?: number; page_size?: number }) {
    return api.get('/admin/ratelimit/logs', { params });
  },
  getRateLimitStats() {
    return api.get('/admin/ratelimit/stats');
  },
  clearRateLimits() {
    return api.post('/admin/ratelimit/clear');
  },
  getAllApiKeys() {
    return api.get('/admin/ratelimit/keys');
  },
  // 管理员对全站任意 Key 的管理（绕过归属校验）
  regenerateApiKey(id: number) {
    return api.post(`/admin/ratelimit/keys/${id}/regenerate`);
  },
  updateApiKey(id: number, data: { name?: string; is_active?: boolean }) {
    return api.patch(`/admin/ratelimit/keys/${id}`, data);
  },
  deleteApiKey(id: number) {
    return api.delete(`/admin/ratelimit/keys/${id}`);
  },
  // 按把封禁 / 解封某一个人的某一把 Key
  banApiKey(id: number, reason?: string) {
    return api.post(`/admin/ratelimit/keys/${id}/ban`, { reason: reason || '' });
  },
  unbanApiKey(id: number) {
    return api.post(`/admin/ratelimit/keys/${id}/unban`);
  },
  // 按人管控：封禁该用户全部 API Key + 专属限流配额
  listUserApiPolicies() {
    return api.get('/admin/ratelimit/users');
  },
  getUserApiQuota(userId: number) {
    return api.get(`/admin/ratelimit/users/${userId}/quota`);
  },
  setUserApiQuota(userId: number, data: { quota_per_min?: number; window_secs?: number; max_keys?: number; note?: string }) {
    return api.put(`/admin/ratelimit/users/${userId}/quota`, data);
  },
  deleteUserApiQuota(userId: number) {
    return api.delete(`/admin/ratelimit/users/${userId}/quota`);
  },
  banUserApiKeys(userId: number, data: { reason?: string; duration_hours?: number }) {
    return api.post(`/admin/ratelimit/users/${userId}/keys/ban`, data);
  },
  unbanUserApiKeys(userId: number) {
    return api.post(`/admin/ratelimit/users/${userId}/keys/unban`);
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
  // 算法推荐说明（公开，176号令 算法透明度）
  getAlgoDescription() {
    return api.get('/algorithms/description');
  },
  // 站点公告（公开）
  getAnnouncements() {
    return api.get('/announcements');
  },  getFederatedSites() {
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