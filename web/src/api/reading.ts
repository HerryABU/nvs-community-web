import api from './index';

export interface ReadingNote {
  id: number;
  user_id: number;
  novel_id: number;
  chapter_number: number;
  type: 'bookmark' | 'annotation';
  content: string;
  selected_text: string;
  anchor: number;
  visibility: 'private' | 'public';
  created_at: string;
  updated_at: string;
  username?: string;
  novel_title?: string;
  chapter_title?: string;
}

export const readingApi = {
  // 新建书签/批注
  create(data: {
    novel_id: number;
    chapter_number?: number;
    type: 'bookmark' | 'annotation';
    content?: string;
    selected_text?: string;
    anchor?: number;
    visibility?: 'private' | 'public';
  }) {
    return api.post('/reading/notes', data);
  },
  // 我的笔记（按作品/章节/类型筛选）
  listMine(params: { novel_id?: number; chapter_number?: number; type?: string }) {
    return api.get('/reading/notes', { params });
  },
  // 公开笔记（游客可读）
  listPublic(params: { novel_id: number; chapter_number?: number }) {
    return api.get('/reading/notes/public', { params });
  },
  // 我的全部笔记（精读页）
  listAllMine(page = 1, pageSize = 20) {
    return api.get('/reading/notes/mine', { params: { page, page_size: pageSize } });
  },
  update(id: number, data: { content?: string; visibility?: 'private' | 'public' }) {
    return api.patch(`/reading/notes/${id}`, data);
  },
  remove(id: number) {
    return api.delete(`/reading/notes/${id}`);
  },
};
