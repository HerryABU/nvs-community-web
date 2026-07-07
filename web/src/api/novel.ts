import api from './index';

export interface Novel {
  id: number;
  author_id: number;
  title: string;
  category: string;
  categories?: string[];
  tags: string[];
  summary: string;
  cover_url: string;
  price_per_chapter: number;
  status: string;
  total_words: number;
  total_chapters: number;
  created_at: string;
  updated_at: string;
  author_name?: string;
  author?: {
    id: number;
    username: string;
    nickname: string;
    avatar_url: string;
  };
}

export interface Chapter {
  id: number;
  novel_id: number;
  chapter_number: number;
  title: string;
  content?: string;
  content_hash?: string;
  word_count: number;
  status: string;
  created_at: string;
}

export interface ChapterDetail {
  chapter: Chapter;
  html_content?: string;
  hash: string;
  hash_match: boolean;
  signature_verified?: boolean;
  file_size: number;
  modified_at: number;
}

export interface ChapterVerify {
  novel_id: number;
  chapter_number: number;
  title: string;
  hash_db: string;
  hash_current: string;
  hash_verified: boolean;
  file_size: number;
  modified_at: number;
  message: string;
}

export interface Comment {
  id: number;
  user_id: number;
  novel_id: number;
  chapter_number: number;
  content: string;
  quote_text: string;
  parent_id: number;
  username?: string;
  created_at: string;
}

export interface NovelListParams {
  page?: number;
  page_size?: number;
  category?: string;
  search?: string;
  status?: string;
}

export const novelApi = {
  // 作品
  getNovels(params?: NovelListParams) {
    return api.get<{ code: number; data: { list: Novel[]; total: number } }>('/novels', {
      params,
    });
  },

  getNovel(id: number) {
    return api.get<{ code: number; data: Novel }>(`/novels/${id}`);
  },

  createNovel(data: Partial<Novel>) {
    return api.post('/novels', data);
  },

  updateNovel(id: number, data: Partial<Novel>) {
    return api.put(`/novels/${id}`, data);
  },

  deleteNovel(id: number) {
    return api.delete(`/novels/${id}`);
  },

  // 章节
  getChapters(novelId: number) {
    return api.get<{ code: number; data: Chapter[] }>(`/novels/${novelId}/chapters`);
  },

  getChapter(novelId: number, chapterNum: number) {
    return api.get<{ code: number; data: ChapterDetail }>(
      `/novels/${novelId}/chapters/${chapterNum}`
    );
  },

  verifyChapter(novelId: number, chapterNum: number) {
    return api.get<{ code: number; data: ChapterVerify }>(
      `/novels/${novelId}/chapters/${chapterNum}/verify`
    );
  },

  createChapter(novelId: number, data: { title: string; content: string }) {
    return api.post(`/novels/${novelId}/chapters`, data);
  },

  updateChapter(novelId: number, chapterNum: number, data: Partial<Chapter>) {
    return api.put(`/novels/${novelId}/chapters/${chapterNum}`, data);
  },

  deleteChapter(novelId: number, chapterNum: number) {
    return api.delete(`/novels/${novelId}/chapters/${chapterNum}`);
  },

  // 评论
  getComments(params: { novel_id: number; chapter_number?: number; page?: number }) {
    return api.get<{ code: number; data: { list: Comment[]; total: number } }>('/comments', {
      params,
    });
  },

  createComment(data: {
    novel_id: number;
    chapter_number?: number;
    content: string;
    quote_text?: string;
    parent_id?: number;
  }) {
    return api.post('/comments', data);
  },

  deleteComment(id: number) {
    return api.delete(`/comments/${id}`);
  },

  // 作者面板
  getMyNovels() {
    return api.get<{ code: number; data: Novel[] }>('/author/novels');
  },

  getNovelStats(id: number) {
    return api.get<{ code: number; data: Record<string, number> }>(`/author/novels/${id}/stats`);
  },

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

  importNovel(file: File, title?: string, category?: string, novelId?: number) {
    const formData = new FormData();
    formData.append('file', file);
    if (title) formData.append('title', title);
    if (category) formData.append('category', category);
    if (novelId) formData.append('novel_id', String(novelId));
    return api.post<{ code: number; data: { novel_id: number; title: string; chapters_count: number; chapters: { num: number; title: string }[] } }>(
      '/novels/import',
      formData,
      { headers: { 'Content-Type': 'multipart/form-data' } },
    );
  },

  importPreview(file: File, splitRule?: string) {
    const formData = new FormData();
    formData.append('file', file);
    if (splitRule) formData.append('split_rule', splitRule);
    return api.post<{ code: number; data: { total: number; chapters: { num: number; title: string; preview: string; words: number }[] } }>(
      '/novels/import/preview',
      formData,
      { headers: { 'Content-Type': 'multipart/form-data' } },
    );
  },

  // 评分
  getNovelRating(id: number) {
    return api.get<{ code: number; data: { dimensions: Record<string, number>; overall: number; count: number } }>(`/novels/${id}/rating`);
  },
  submitRating(data: { novel_id: number; type_completion: number; narrative_quality: number; thought_depth: number; community_reputation: number; update_stability: number }) {
    return api.post('/ratings', data);
  },
  getUserRating(novelId: number) {
    return api.get('/ratings', { params: { novel_id: novelId } });
  },

  // 论坛
  getForums(type?: string) {
    return api.get('/forums', { params: { type } });
  },
  getForum(id: number, page?: number) {
    return api.get(`/forums/${id}`, { params: { page } });
  },
  getNovelForum(novelId: number) {
    return api.get(`/novels/${novelId}/forum`);
  },
  createThread(forumId: number, data: { title: string; content: string }) {
    return api.post(`/forums/${forumId}/threads`, data);
  },
  getThread(id: number, page?: number) {
    return api.get(`/threads/${id}`, { params: { page } });
  },
  createPost(threadId: number, data: { content: string }) {
    return api.post(`/threads/${threadId}/posts`, data);
  },
};
