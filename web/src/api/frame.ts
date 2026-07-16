import api from './index';

// ==================== UserFrame API ====================
export const frameApi = {
  list() { return api.get('/userframes'); },
  get(id: number) { return api.get(`/userframes/${id}`); },
  create(data: {
    name: string; description?: string; novel_id?: number;
    content: string; is_public?: boolean; tags?: string;
    has_controls?: boolean; uses_novel_api?: boolean; sandbox_level?: string;
    frame_type?: string;
  }) { return api.post('/userframes', data); },
  update(id: number, data: Record<string, unknown>) { return api.put(`/userframes/${id}`, data); },
  delete(id: number) { return api.delete(`/userframes/${id}`); },
  listPublic() { return api.get('/userframes/public'); },
  getPreview(id: number) { return `/api/userframes/${id}/preview`; },
  getByNovel(novelId: number) { return api.get(`/novels/${novelId}/frames`); },
  getByAuthor(authorId: number) { return api.get(`/author/${authorId}/frames`); },
};

// ==================== UserHTML API (ZIP上传) ====================
export const htmlApi = {
  list() { return api.get('/userhtmls'); },
  get(id: number) { return api.get(`/userhtmls/${id}`); },
  upload(formData: FormData) {
    return api.post('/userhtmls/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 60000,
    });
  },
  update(id: number, data: Record<string, unknown>) { return api.put(`/userhtmls/${id}`, data); },
  delete(id: number) { return api.delete(`/userhtmls/${id}`); },
  getByNovel(novelId: number) { return api.get(`/novels/${novelId}/htmls`); },
  getPreview(id: number) { return `/api/userhtmls/${id}/preview`; },
};

// ==================== 模板API ====================
export const templateApi = {
  getNovelData(novelId: number) { return api.get(`/template/novel/${novelId}`); },
};

// ==================== 沙盒信息 ====================
export const sandboxApi = {
  getInfo() { return api.get('/sandbox/info'); },
};
