import api from '@/api/index';

// 关注作者
export const followApi = {
  follow(authorId: number) {
    return api.post(`/follow/${authorId}`);
  },
  unfollow(authorId: number) {
    return api.delete(`/follow/${authorId}`);
  },
  check(authorId: number) {
    return api.get(`/follow/check/${authorId}`);
  },
  listFollowing(page: number = 1) {
    return api.get('/following', { params: { page } });
  },
  listFollowers(page: number = 1) {
    return api.get('/followers', { params: { page } });
  },
  stats() {
    return api.get('/follow/stats');
  },
};

// 作者博客
export const blogApi = {
  listPublic(page: number = 1, pageSize: number = 12) {
    return api.get('/blogs', { params: { page, page_size: pageSize } });
  },
  getBlog(id: number) {
    return api.get(`/blogs/${id}`);
  },
  listByAuthor(authorId: number, page: number = 1) {
    return api.get(`/author/${authorId}/blogs`, { params: { page } });
  },
  create(data: { title: string; content: string; summary?: string }) {
    return api.post('/blogs', data);
  },
  update(id: number, data: { title?: string; content?: string; summary?: string; is_pinned?: boolean }) {
    return api.put(`/blogs/${id}`, data);
  },
  delete(id: number) {
    return api.delete(`/blogs/${id}`);
  },
};

export interface AuthorBlog {
  id: number;
  author_id: number;
  title: string;
  content: string;
  summary: string;
  is_pinned: boolean;
  view_count: number;
  created_at: string;
  updated_at: string;
  author?: {
    id: number;
    username: string;
    nickname: string;
    avatar_url: string;
  };
}
