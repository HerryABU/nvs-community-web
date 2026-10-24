import api from '@/api/index';

/** 群组列表项（后端 Group 结构 + 运行时字段） */
export interface Group {
  id: number;
  owner_id: number;
  group_no: string;
  name: string;
  description: string;
  category: string;
  visibility: 'public' | 'semi' | 'private';
  join_mode: 'open' | 'apply' | 'invite';
  invite_code?: string;
  cover_url: string;
  member_limit: number;
  post_limit_per_day: number;
  review_first: boolean;
  status: 'active' | 'archived' | 'banned';
  member_count: number;
  post_count: number;
  announcement?: string;
  announcement_at?: string;
  allow_member_invite?: boolean;
  ai_enabled?: boolean;
  ai_agent_id?: number;
  created_at: string;
  owner_name?: string;
  my_role?: 'owner' | 'admin' | 'member' | '';
  my_status?: 'active' | 'pending' | '';
  can_read?: boolean;
  is_member?: boolean;
  is_admin?: boolean;
}

export interface GroupMember {
  id: number;
  group_id: number;
  user_id: number;
  role: 'owner' | 'admin' | 'member';
  status: 'active' | 'pending';
  muted_until?: string | null;
  note?: string;
  joined_at: string;
  username?: string;
  nickname?: string;
  avatar_url?: string;
}

export interface GroupPost {
  id: number;
  group_id: number;
  user_id: number;
  title: string;
  content: string;
  status: 'pending' | 'published' | 'removed';
  reply_count: number;
  ai_assisted: boolean;
  created_at: string;
  author_name?: string;
  author_avatar?: string;
}

export interface GroupFile {
  id: number;
  group_id: number;
  uploader_id: number;
  name: string;
  size: number;
  mime: string;
  status: string;
  created_at: string;
  uploader_name?: string;
}

export interface GroupSettingsPayload {
  name?: string;
  description?: string;
  category?: string;
  visibility?: string;
  join_mode?: string;
  cover_url?: string;
  member_limit?: number;
  post_limit_per_day?: number;
  review_first?: boolean;
  allow_member_invite?: boolean;
  ai_enabled?: boolean;
  ai_agent_id?: number;
}

export const groupApi = {
  /** 发现页（公开 + 半公开） */
  list(params: { scope?: 'discover' | 'my'; category?: string; q?: string; page?: number; size?: number } = {}) {
    return api.get<{ code: number; data: { list: Group[]; total: number; page: number; size: number; categories: string[] } }>(
      '/groups',
      { params },
    );
  },
  detail(id: number) {
    return api.get<{
      code: number;
      data: {
        group: Group; can_read: boolean; my_role: string; my_status: string; is_admin: boolean;
        pending_requests?: number;
        allowed?: { invite: boolean; manage: boolean; owner: boolean };
      };
    }>(`/groups/${id}`);
  },
  create(data: GroupSettingsPayload) {
    return api.post<{ code: number; data: { group: Group } }>('/groups', data);
  },
  update(id: number, data: GroupSettingsPayload) {
    return api.put(`/groups/${id}`, data);
  },
  dissolve(id: number) {
    return api.delete(`/groups/${id}`);
  },
  join(id: number, code = '') {
    return api.post<{ code: number; data: { status: string } }>(`/groups/${id}/join`, { code });
  },
  /** 按群号 / 邀请码加入 */
  joinByNo(payload: { group_no: string; code?: string }) {
    return api.post<{ code: number; data: { status: string; group: Group } }>('/group-join', payload);
  },
  /** 群号 / 邀请码预览（扫码或输入群号后先看信息） */
  lookup(no: string) {
    return api.get<{ code: number; data: { group: Group; joined: boolean; my_status?: string; my_role?: string } }>(
      '/group-lookup',
      { params: { no } },
    );
  },

  // ---------- 成员与群管理 ----------
  members(id: number, params: { status?: string; page?: number; size?: number } = {}) {
    return api.get<{ code: number; data: { list: GroupMember[]; total: number } }>(`/groups/${id}/members`, { params });
  },
  invite(id: number, username: string) {
    return api.post(`/groups/${id}/invite`, { username });
  },
  approve(id: number, uid: number, approve: boolean) {
    return api.post(`/groups/${id}/requests/${uid}`, { approve });
  },
  setRole(id: number, uid: number, role: 'admin' | 'member') {
    return api.put(`/groups/${id}/members/${uid}`, { role });
  },
  mute(id: number, uid: number, minutes: number) {
    return api.put(`/groups/${id}/members/${uid}/mute`, { minutes });
  },
  removeMember(id: number, uid: number) {
    return api.delete(`/groups/${id}/members/${uid}`);
  },
  leave(id: number, uid: number) {
    return api.delete(`/groups/${id}/members/${uid}`);
  },
  inviteCode(id: number) {
    return api.get<{ code: number; data: { invite_code: string; group_no: string } }>(`/groups/${id}/invite-code`);
  },
  setAnnouncement(id: number, content: string) {
    return api.put(`/groups/${id}/announcement`, { content });
  },

  // ---------- 帖子与回复 ----------
  posts(id: number, params: { page?: number; size?: number; pending?: number } = {}) {
    return api.get<{ code: number; data: { list: GroupPost[]; total: number } }>(`/groups/${id}/posts`, { params });
  },
  createPost(id: number, data: { title: string; content: string; ai_assisted?: boolean }) {
    return api.post<{ code: number; data: { post: GroupPost } }>(`/groups/${id}/posts`, data);
  },
  postDetail(id: number, pid: number) {
    return api.get<{ code: number; data: { post: GroupPost; group: { id: number; name: string; group_no: string }; can_edit: boolean } }>(
      `/groups/${id}/posts/${pid}`,
    );
  },
  reviewPost(id: number, pid: number, approve: boolean) {
    return api.post(`/groups/${id}/posts/${pid}/review`, { approve });
  },
  deletePost(id: number, pid: number) {
    return api.delete(`/groups/${id}/posts/${pid}`);
  },
  replies(id: number, pid: number, params: { page?: number; size?: number } = {}) {
    return api.get<{ code: number; data: { list: any[]; total: number } }>(`/groups/${id}/posts/${pid}/replies`, { params });
  },
  createReply(id: number, pid: number, content: string) {
    return api.post(`/groups/${id}/posts/${pid}/replies`, { content });
  },

  // ---------- 群文件 ----------
  files(id: number, params: { page?: number; size?: number } = {}) {
    return api.get<{ code: number; data: { list: GroupFile[]; total: number; used: number; quota: number } }>(
      `/groups/${id}/files`,
      { params },
    );
  },
  uploadFile(id: number, file: File, onProgress?: (pct: number) => void) {
    const form = new FormData();
    form.append('file', file);
    return api.post<{ code: number; data: GroupFile }>(`/groups/${id}/files`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
      onUploadProgress: (e) => {
        if (onProgress && e.total) onProgress(Math.round((e.loaded / e.total) * 100));
      },
    });
  },
  deleteFile(id: number, fid: number) {
    return api.delete(`/groups/${id}/files/${fid}`);
  },
  /** 下载走鉴权接口：用 axios blob，避免直链绕过权限 */
  async downloadFile(id: number, fid: number, name: string) {
    const res = await api.get(`/groups/${id}/files/${fid}/download`, { responseType: 'blob' });
    const url = URL.createObjectURL(res.data as Blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = name;
    a.click();
    URL.revokeObjectURL(url);
  },

  aiStatus(id: number) {
    return api.get<{ code: number; data: { enabled: boolean; agent_id: number; agent_name: string } }>(`/groups/${id}/ai`);
  },

  // ---------- 管理端 ----------
  adminList(params: { status?: string; visibility?: string; q?: string; page?: number; size?: number } = {}) {
    return api.get<{ code: number; data: { list: Group[]; total: number } }>('/admin/groups', { params });
  },
  adminBan(id: number, banned: boolean, reason = '') {
    return api.post(`/admin/groups/${id}/ban`, { banned, reason });
  },
  adminDelete(id: number) {
    return api.delete(`/admin/groups/${id}`);
  },
  adminQuota() {
    return api.get<{ code: number; data: { config: any; overrides: any[] } }>('/admin/group-quota');
  },
  adminSetQuota(config: any) {
    return api.put('/admin/group-quota', config);
  },
  adminSetUserQuota(uid: number, payload: { max_groups?: number; note?: string; delete?: boolean }) {
    return api.put(`/admin/group-quota/users/${uid}`, payload);
  },
};
