import api from '@/api/index';
import type { Comment } from './types';

export const commentApi = {
  // 楼层模型：list = 本页楼层 + 这些楼层下的全部回复（平铺，前端按 root_id 分组成「楼」）
  getComments(params: { novel_id: number; blog_id?: number; chapter_number?: number; page?: number }) {
    return api.get<{
      code: number;
      data: { list: Comment[]; total: number; floor_total: number; has_more: boolean; page: number };
    }>('/comments', {
      params,
    });
  },

  // parent_id 留空 = 回复主内容（开新楼）；传值 = 回复该条（挂靠其所属楼）
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
