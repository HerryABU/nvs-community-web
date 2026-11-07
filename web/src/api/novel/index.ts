import api from '@/api/index';

// 类型定义从 types.ts 导入并重新导出
export type { Novel, Chapter, ChapterDetail, ChapterVerify, Comment, NovelListParams } from './types';

// 子模块重新导出（保持向后兼容）
export { chapterApi } from './chapters';
export { commentApi } from './comments';
export { forumApi } from '@/api/social/forums';

export const novelApi = {
  // ============ 作品 CRUD ============

  getNovels(params?: { page?: number; page_size?: number; category?: string; search?: string; status?: string }) {
    return api.get<{ code: number; data: { list: import('./types').Novel[]; total: number } }>('/novels', { params });
  },

  getNovel(id: number) {
    return api.get<{ code: number; data: import('./types').Novel }>(`/novels/${id}`);
  },

  // #76 衍生同人列表（原著作者的粉丝/作者本人可见）
  getDerivatives(id: number) {
    return api.get<{ code: number; data: { visible: boolean; items: import('./types').Novel[]; origin: any } }>(`/novels/${id}/derivatives`);
  },

  createNovel(data: Partial<import('./types').Novel>) {
    return api.post('/novels', data);
  },

  updateNovel(id: number, data: Partial<import('./types').Novel>) {
    return api.put(`/novels/${id}`, data);
  },

  deleteNovel(id: number) {
    return api.delete(`/novels/${id}`);
  },

  // ============ 作者面板 ============

  getMyNovels() {
    return api.get<{ code: number; data: import('./types').Novel[] }>('/author/novels');
  },

  getNovelStats(id: number) {
    return api.get<{ code: number; data: Record<string, number> }>(`/author/novels/${id}/stats`);
  },

  // 作品点赞/取消点赞（toggle）
  toggleNovelLike(id: number) {
    return api.post<{ code: number; data: { liked: boolean; count: number } }>(`/novels/${id}/like`);
  },
  // 作品点赞数 + 当前用户是否已赞
  getNovelLikes(id: number) {
    return api.get<{ code: number; data: { count: number; liked: boolean } }>(`/novels/${id}/likes`);
  },

  // 获取作者被封禁（176号令）的小说/章节及原因
  getBannedContent() {
    return api.get<{ code: number; data: { novels: any[]; chapters: any[] } }>('/author/banned-content');
  },

  // 回收站：用户自删内容（小说/博客）+ 恢复
  getRecycle() {
    return api.get<{ code: number; data: { novels: any[]; novel_total: number; blogs: any[]; blog_total: number } }>('/author/recycle');
  },
  restoreRecycle(data: { type: string; id: number }) {
    return api.post('/author/recycle/restore', data);
  },

  // ============ 导入导出 ============

  exportNovel(id: number) {
    return api.post(`/novels/${id}/export`, {}, { responseType: 'blob' });
  },

  exportNovelEPUB(id: number) {
    return api.post(`/novels/${id}/export/epub`, {}, { responseType: 'blob' });
  },

  exportNovelMarkdown(id: number) {
    return api.post(`/novels/${id}/export/markdown`, {}, { responseType: 'blob' });
  },

  exportNovelTXT(id: number) {
    return api.post(`/novels/${id}/export/txt`, {}, { responseType: 'blob' });
  },

  exportNovelDOCX(id: number) {
    return api.post(`/novels/${id}/export/docx`, {}, { responseType: 'blob' });
  },

  importNovel(file: File, title?: string, category?: string, novelId?: number, splitRule?: string) {
    const formData = new FormData();
    formData.append('file', file);
    if (title) formData.append('title', title);
    if (category) formData.append('category', category);
    if (novelId) formData.append('novel_id', String(novelId));
    if (splitRule) formData.append('split_rule', splitRule);
    return api.post<{ code: number; data: { novel_id: number; title: string; chapters_count: number; chapters: { num: number; title: string }[] } }>(
      '/novels/import',
      formData,
    );
  },

  importPreview(file: File, splitRule?: string) {
    const formData = new FormData();
    formData.append('file', file);
    if (splitRule) formData.append('split_rule', splitRule);
    return api.post<{ code: number; data: { total: number; chapters: { num: number; title: string; preview: string; words: number }[] } }>(
      '/novels/import/preview',
      formData,
    );
  },

  // 批量导入多个 Word/Markdown 文件（合并为一部作品；传 novelId 则追加到已有小说）
  // mode: file(默认，一个文件一章) | rule(自定义正则拆分) | auto(自动识别标题拆分)
  // splitRule: mode=rule 时的正则；titlePrefix: 章节标题加「第N章 」前缀
  importBatch(files: File[], title?: string, category?: string, mode?: string, splitRule?: string, titlePrefix?: boolean, novelId?: number) {
    const formData = new FormData();
    for (const f of files) formData.append('files', f);
    if (title) formData.append('title', title);
    if (category) formData.append('category', category);
    if (mode) formData.append('mode', mode);
    if (splitRule) formData.append('split_rule', splitRule);
    if (titlePrefix) formData.append('title_prefix', 'on');
    if (novelId) formData.append('novel_id', String(novelId));
    return api.post<{ code: number; data: { novel_id: number; title: string; mode: string; files_count: number; chapters_count: number; is_append?: boolean } }>(
      '/novels/import/batch',
      formData,
    );
  },

  // ============ 评分 ============

  getNovelRating(id: number) {
    return api.get<{ code: number; data: { dimensions: Record<string, number>; overall: number; count: number } }>(`/novels/${id}/rating`);
  },

  submitRating(data: { novel_id: number; type_completion: number; narrative_quality: number; thought_depth: number; community_reputation: number; update_stability: number }) {
    return api.post('/ratings', data);
  },

  getUserRating(novelId: number) {
    return api.get('/ratings', { params: { novel_id: novelId } });
  },
};
