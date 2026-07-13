import api from '@/api/index';
import type { Comment } from './types';

export const commentApi = {
  getComments(params: { novel_id: number; blog_id?: number; chapter_number?: number; page?: number }) {
    return api.get<{ code: number; data: { list: Comment[]; total: number } }>('/comments', {
      params,
    });
  },

  createComment(data: {
    novel_id: number;
    blog_id?: number;
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
};
