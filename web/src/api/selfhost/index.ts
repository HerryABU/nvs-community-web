import api, { apiPrefix } from '@/api/index';

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
  author_name?: string;
  route?: SelfHostRoute | null;
  runtime?: SelfHostRuntime | null;
  created_at: string;
  updated_at: string;
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
    return api.get<{ code: number; data: { project: SelfHostProject; logs: SelfHostReviewLog[]; config: Record<string, any> } }>(`/selfhost/projects/${id}`);
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
  fileUrl(id: number, kind: 'source' | 'binary') {
    return `${apiPrefix}/api/selfhost/projects/${id}/files/${kind}`;
  },
  plaza(keyword = '', lang = '') {
    return api.get<{ code: number; data: { projects: SelfHostProject[]; total: number } }>('/selfhost/plaza', { params: { keyword, lang } });
  },

  // ─── 管理端 ───
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
  adminConfig() {
    return api.get<{ code: number; data: { source_required: boolean; local_exec: boolean; note: string } }>('/admin/selfhost/config');
  },
  adminSaveConfig(sourceRequired: boolean) {
    return api.put<{ code: number }>('/admin/selfhost/config', { source_required: sourceRequired });
  },
};
