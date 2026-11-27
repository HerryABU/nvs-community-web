import api from '@/api/index';

// 自托管：运行时版本目录 / 项目（源码与编译产物）/ 审核 / 反向代理。
// 安全约定：平台不执行用户代码，作者自带服务；只有审核通过且路由启用才对外。

export interface SelfHostRuntime {
  id: number;
  language: string;
  version: string;
  label: string;
  kind: 'compiled' | 'interpreted';
  install_hint: string;
  download_url: string;
  checksum: string;
  exec_path?: string;
  enabled: boolean;
  sort: number;
}

export interface SelfHostRoute {
  id: number;
  project_id: number;
  prefix: string;
  target: string;
  strip_prefix: boolean;
  timeout: number;
  enabled: boolean;
  status: string;
  last_error: string;
}

export interface SelfHostProject {
  id: number;
  user_id: number;
  name: string;
  slug: string;
  description: string;
  language: string;
  runtime_id: number;
  mode: 'source' | 'compiled';
  source_file: string;
  binary_file: string;
  source_name: string;
  binary_name: string;
  build_command: string;
  start_command: string;
  upstream: string;
  visibility: 'private' | 'public';
  review_status: 'draft' | 'pending' | 'approved' | 'rejected' | 'suspended';
  review_note: string;
  reviewed_by: number;
  reviewed_at: string | null;
  source_waived: boolean;
  local_exec: boolean;
  source_shared?: boolean;
  binary_shared?: boolean;
  storage_bytes?: number;
  usage?: Pick<UsageRow, "requests" | "bytes_up" | "bytes_down" | "days">;
  // 服务端按当前登录用户计算，前端不要自行推断
  can_source?: boolean;
  can_binary?: boolean;
  author_name?: string;
  route?: SelfHostRoute | null;
  runtime?: SelfHostRuntime | null;
  created_at: string;
  updated_at: string;
}


export interface RuntimeCandidate {
  language: string;
  version: string;
  label: string;
  kind: 'compiled' | 'interpreted';
  exec_path: string;
  note: string;
}

export interface RuntimeRequest {
  id: number;
  user_id: number;
  project_id: number;
  language: string;
  version: string;
  note: string;
  status: 'pending' | 'approved' | 'rejected';
  final_language: string;
  final_version: string;
  admin_note: string;
  reviewed_by: number;
  reviewed_at: string | null;
  created_at: string;
}

export interface PlazaEntry {
  type: 'selfhost' | 'html';
  id: number;
  title: string;
  description: string;
  language: string;
  version: string;
  author: string;
  url: string;
  created_at: string;
  updated_at: string;
  can_source?: boolean;
  can_binary?: boolean;
  source_name?: string;
  binary_name?: string;
}

export interface DirectRoute {
  id: number;
  prefix: string;
  port: number;
  root_path: string;
  target_url?: string;
  cookie_hat?: string;
  drop_foreign_cookies?: boolean;
  enabled: boolean;
  description: string;
  created_at: string;
  updated_at: string;
}

export interface PortMap {
  id: number;
  listen_port: number;
  target_host: string;
  target_port: number;
  enabled: boolean;
  description: string;
  last_error: string;
  running: boolean;
  created_at: string;
  updated_at: string;
}

export interface IsoService {
  id: number;
  name: string;
  target: string;
  mode: 'pass-through' | 'strip' | 'isolated';
  timeout: number;
  enabled: boolean;
  description: string;
  cookie_hat?: string;
  html_rewrite?: boolean;
  known_abs_prefixes?: string;
  host_mode?: 'keep' | 'rewrite';
  no_full_compat?: boolean;
  max_rewrite_bytes?: number;
  drop_foreign_cookies?: boolean;
  passthrough_location_prefix?: boolean;
  created_at: string;
  updated_at: string;
}

export interface IsoSnapshot {
  name: string;
  prefix: string;
  target: string;
  mode: string;
  enabled: boolean;
  cookie_hat: string;
  html_rewrite: boolean;
  known_abs_prefixes?: string[];
  timeout: number;
  host_mode: string;
  no_full_compat: boolean;
  breaker: Record<string, unknown>;
  metrics: Record<string, unknown>;
}

export interface IsoAuditEntry {
  timestamp?: string;
  service?: string;
  kind?: string;
  path?: string;
  detail?: string;
  [key: string]: unknown;
}

export interface UsageRow {
  project_id: number;
  name: string;
  slug: string;
  owner_id: number;
  storage_bytes: number;
  requests: number;
  bytes_up: number;
  bytes_down: number;
  days: number;
}

export interface IsolationIssue {
  project_id: number;
  name: string;
  slug: string;
  upstream: string;
  reason: string;
  level: 'block' | 'warn';
}

export interface SelfHostGrant {
  id: number;
  project_id: number;
  user_id: number;
  kind: 'source' | 'binary' | 'all';
  status: 'pending' | 'approved' | 'rejected';
  note: string;
  reviewed_by: number;
  reviewed_at: string | null;
  created_at: string;
}

export interface SelfHostReviewLog {
  id: number;
  project_id: number;
  operator_id: number;
  action: string;
  note: string;
  created_at: string;
}

export const selfhostApi = {
  listRuntimes() {
    return api.get<{ code: number; data: { runtimes: SelfHostRuntime[] } }>('/selfhost/runtimes');
  },
  listMyProjects(keyword = '') {
    return api.get<{ code: number; data: { projects: SelfHostProject[]; total: number } }>('/selfhost/projects', { params: { keyword } });
  },
  getProject(id: number) {
    return api.get<{ code: number; data: {
      project: SelfHostProject; logs: SelfHostReviewLog[]; config: Record<string, any>;
      can_source: boolean; can_binary: boolean;
      my_grants: SelfHostGrant[]; grants?: SelfHostGrant[];
    } }>(`/selfhost/projects/${id}`);
  },
  createProject(payload: Partial<SelfHostProject>) {
    return api.post<{ code: number; data: SelfHostProject }>('/selfhost/projects', payload);
  },
  updateProject(id: number, payload: Partial<SelfHostProject>) {
    return api.put<{ code: number; data: SelfHostProject }>(`/selfhost/projects/${id}`, payload);
  },
  deleteProject(id: number) {
    return api.delete<{ code: number }>(`/selfhost/projects/${id}`);
  },
  submit(id: number) {
    return api.post<{ code: number }>(`/selfhost/projects/${id}/submit`);
  },
  setVisibility(id: number, visibility: 'private' | 'public') {
    return api.put<{ code: number }>(`/selfhost/projects/${id}/visibility`, { visibility });
  },
  uploadFile(id: number, kind: 'source' | 'binary', name: string, file: File) {
    const fd = new FormData();
    fd.append('file', file);
    return api.postForm(`/selfhost/projects/${id}/files/${kind}?name=${encodeURIComponent(name)}`, fd);
  },
  // 下载走 axios：JWT 绑定设备指纹，普通 <a href> 不带指纹头会被 401。
  async downloadFile(id: number, kind: 'source' | 'binary', fileName: string) {
    const res = await api.get(`/selfhost/projects/${id}/files/${kind}`, { responseType: 'blob' });
    const blob = new Blob([res.data as BlobPart]);
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = fileName || (kind === 'source' ? 'source.zip' : 'binary');
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  },
  requestAccess(id: number, kind: 'source' | 'binary' | 'all', note: string) {
    return api.post<{ code: number; data: SelfHostGrant }>(`/selfhost/projects/${id}/access`, { kind, note });
  },
  setSharing(id: number, sourceShared: boolean, binaryShared: boolean) {
    return api.put<{ code: number }>(`/selfhost/projects/${id}/sharing`, { source_shared: sourceShared, binary_shared: binaryShared });
  },
  myGrants() {
    return api.get<{ code: number; data: { grants: SelfHostGrant[] } }>('/selfhost/access-requests');
  },
  reviewGrant(id: number, approve: boolean) {
    return api.post<{ code: number }>(`/selfhost/grants/${id}/review`, { approve });
  },
  revokeGrant(id: number) {
    return api.delete<{ code: number }>(`/selfhost/grants/${id}`);
  },
  plaza(keyword = '', lang = '') {
    return api.get<{ code: number; data: { projects: SelfHostProject[]; total: number } }>('/selfhost/plaza', { params: { keyword, lang } });
  },

  // ─── 管理端 ───
  scanRuntimes() {
    return api.post<{ code: number; data: { candidates: RuntimeCandidate[] } }>('/admin/selfhost/runtimes/scan');
  },
  createRuntimeRequest(payload: Partial<RuntimeRequest>) {
    return api.post<{ code: number; data: RuntimeRequest }>('/selfhost/runtime-requests', payload);
  },
  myRuntimeRequests() {
    return api.get<{ code: number; data: { requests: RuntimeRequest[] } }>('/selfhost/runtime-requests');
  },
  adminRuntimeRequests(status = '') {
    return api.get<{ code: number; data: { requests: RuntimeRequest[] } }>('/admin/selfhost/runtime-requests', { params: { status } });
  },
  adminReviewRuntimeRequest(id: number, payload: Partial<RuntimeRequest> & { approve: boolean; exec_path?: string }) {
    return api.post<{ code: number; data: unknown }>(`/admin/selfhost/runtime-requests/${id}/review`, payload);
  },
  adminListRuntimes() {
    return api.get<{ code: number; data: { runtimes: SelfHostRuntime[] } }>('/admin/selfhost/runtimes');
  },
  adminSaveRuntime(payload: Partial<SelfHostRuntime>, id?: number) {
    return id
      ? api.put<{ code: number }>(`/admin/selfhost/runtimes/${id}`, payload)
      : api.post<{ code: number }>('/admin/selfhost/runtimes', payload);
  },
  adminDeleteRuntime(id: number) {
    return api.delete<{ code: number }>(`/admin/selfhost/runtimes/${id}`);
  },
  adminListProjects(status = '', keyword = '') {
    return api.get<{ code: number; data: { projects: SelfHostProject[]; total: number } }>('/admin/selfhost/projects', { params: { status, keyword } });
  },
  adminReview(id: number, approve: boolean, note: string, waiveSource = false) {
    return api.post<{ code: number }>(`/admin/selfhost/projects/${id}/review`, { approve, note, waive_source: waiveSource });
  },
  adminSuspend(id: number, suspend: boolean, note = '') {
    return api.post<{ code: number }>(`/admin/selfhost/projects/${id}/suspend`, { suspend, note });
  },
  adminListRoutes(keyword = '') {
    return api.get<{ code: number; data: { routes: SelfHostRoute[]; total: number; active: number; base: string } }>('/admin/selfhost/routes', { params: { keyword } });
  },
  adminSaveRoute(payload: Partial<SelfHostRoute>) {
    return api.post<{ code: number; data: SelfHostRoute }>('/admin/selfhost/routes', payload);
  },
  adminToggleRoute(id: number, enabled: boolean) {
    return api.put<{ code: number }>(`/admin/selfhost/routes/${id}/toggle`, { enabled });
  },
  adminDeleteRoute(id: number) {
    return api.delete<{ code: number }>(`/admin/selfhost/routes/${id}`);
  },
  plazaAll(keyword = '', type = 'all') {
    return api.get<{ code: number; data: { entries: PlazaEntry[]; total: number } }>('/selfhost/plaza/all', { params: { keyword, type } });
  },
  adminDirectRoutes() {
    return api.get<{ code: number; data: { routes: DirectRoute[]; base: string } }>('/admin/selfhost/direct-routes');
  },
  adminSaveDirectRoute(payload: Partial<DirectRoute>) {
    return api.post<{ code: number; data: DirectRoute }>('/admin/selfhost/direct-routes', payload);
  },
  adminToggleDirectRoute(id: number, enabled: boolean) {
    return api.put<{ code: number }>(`/admin/selfhost/direct-routes/${id}/toggle`, { enabled });
  },
  adminDeleteDirectRoute(id: number) {
    return api.delete<{ code: number }>(`/admin/selfhost/direct-routes/${id}`);
  },
  adminIsoServices() {
    return api.get<{ code: number; data: { services: IsoService[]; snapshot: IsoSnapshot[]; enabled: boolean; count: number; warnings: string[]; audit_file: string } }>('/admin/selfhost/iso-services');
  },
  adminProbeIsoMode(target: string, service = '') {
    return api.post<{ code: number; data: { mode: 'pass-through' | 'strip' | 'isolated'; reason: string; signals: string[]; confidence: string } }>('/admin/selfhost/iso-probe', { target, service });
  },
  adminIsoMetrics() {
    return api.get<{ code: number; data: { metrics: Record<string, unknown>[] } }>('/admin/selfhost/iso-metrics');
  },
  adminIsoAudit(limit = 200) {
    return api.get<{ code: number; data: { entries: IsoAuditEntry[] } }>('/admin/selfhost/iso-audit', { params: { limit } });
  },
  adminSaveIsoService(payload: Partial<IsoService>) {
    return api.post<{ code: number; data: IsoService }>('/admin/selfhost/iso-services', payload);
  },
  adminToggleIsoService(id: number, enabled: boolean) {
    return api.put<{ code: number }>(`/admin/selfhost/iso-services/${id}/toggle`, { enabled });
  },
  adminDeleteIsoService(id: number) {
    return api.delete<{ code: number }>(`/admin/selfhost/iso-services/${id}`);
  },
  adminPortMaps() {
    return api.get<{ code: number; data: { maps: PortMap[] } }>('/admin/selfhost/port-maps');
  },
  adminSavePortMap(payload: Partial<PortMap>) {
    return api.post<{ code: number; data: PortMap }>('/admin/selfhost/port-maps', payload);
  },
  adminTogglePortMap(id: number, enabled: boolean) {
    return api.put<{ code: number }>(`/admin/selfhost/port-maps/${id}/toggle`, { enabled });
  },
  adminDeletePortMap(id: number) {
    return api.delete<{ code: number }>(`/admin/selfhost/port-maps/${id}`);
  },
  adminUsage() {
    return api.get<{ code: number; data: { usage: UsageRow[] } }>('/admin/selfhost/usage');
  },
  adminIsolation() {
    return api.get<{ code: number; data: { issues: IsolationIssue[] } }>('/admin/selfhost/isolation');
  },
  adminConfig() {
    return api.get<{ code: number; data: { source_required: boolean; local_exec: boolean; note: string } }>('/admin/selfhost/config');
  },
  adminSaveConfig(sourceRequired: boolean) {
    return api.put<{ code: number }>('/admin/selfhost/config', { source_required: sourceRequired });
  },
};
