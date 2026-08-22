import api from './index';

// 类型定义从 types.ts 导入并重新导出
export type { Novel, Chapter, ChapterDetail, ChapterVerify, Comment, NovelListParams } from './novel/types';

// 子模块重新导出（保持向后兼容）
export { chapterApi } from './novel/chapters';
export { commentApi } from './novel/comments';
export { forumApi } from './forums';

export const novelApi = {
  // ============ 作品 CRUD ============

  getNovels(params?: { page?: number; page_size?: number; category?: string; search?: string; status?: string }) {
    return api.get<{ code: number; data: { list: import('./novel/types').Novel[]; total: number } }>('/novels', { params });
  },

  getNovel(id: number) {
    return api.get<{ code: number; data: import('./novel/types').Novel }>(`/novels/${id}`);
  },

  createNovel(data: Partial<import('./novel/types').Novel>) {
    return api.post('/novels', data);
  },

  updateNovel(id: number, data: Partial<import('./novel/types').Novel>) {
    return api.put(`/novels/${id}`, data);
  },

  deleteNovel(id: number) {
    return api.delete(`/novels/${id}`);
  },

  // ============ 作者面板 ============

  getMyNovels() {
    return api.get<{ code: number; data: import('./novel/types').Novel[] }>('/author/novels');
  },

  getNovelStats(id: number) {
    return api.get<{ code: number; data: Record<string, number> }>(`/author/novels/${id}/stats`);
  },

  // 获取作者被封禁（176号令）的小说/章节及原因
  getBannedContent() {
    return api.get<{ code: number; data: { novels: any[]; chapters: any[] } }>('/author/banned-content');
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

  // 批量导入多个 Word/Markdown 文件（合并为一部作品）
  // mode: file(默认，一个文件一章) | rule(自定义正则拆分) | auto(自动识别标题拆分)
  // splitRule: mode=rule 时的正则；titlePrefix: 章节标题加「第N章 」前缀
  importBatch(files: File[], title?: string, category?: string, mode?: string, splitRule?: string, titlePrefix?: boolean) {
    const formData = new FormData();
    for (const f of files) formData.append('files', f);
    if (title) formData.append('title', title);
    if (category) formData.append('category', category);
    if (mode) formData.append('mode', mode);
    if (splitRule) formData.append('split_rule', splitRule);
    if (titlePrefix) formData.append('title_prefix', 'on');
    return api.post<{ code: number; data: { novel_id: number; title: string; mode: string; files_count: number; chapters_count: number } }>(
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
