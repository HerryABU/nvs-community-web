import api from '@/api/index';

export const forumApi = {
  getForums(type?: string) {
    return api.get('/forums', { params: { type } });
  },
  getForum(id: number, page?: number, password?: string) {
    return api.get(`/forums/${id}`, { params: { page, password } });
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
  pinThread(id: number) {
    return api.post(`/threads/${id}/pin`);
  },
  unpinThread(id: number) {
    return api.post(`/threads/${id}/unpin`);
  },

  // ===== 用户自建论坛 =====
  createForum(data: { name: string; description?: string; is_public?: boolean; password?: string }) {
    return api.post('/forums', data);
  },
  listMyForums() {
    return api.get('/forums/mine');
  },
  verifyPassword(id: number, password: string) {
    return api.post(`/forums/${id}/verify`, { password });
  },
  // 论坛创建者/管理员：禁言、审核
  muteUser(forumId: number, data: { user_id: number; duration_hours?: number; reason?: string }) {
    return api.post(`/forums/${forumId}/mute`, data);
  },
  unmuteUser(forumId: number, userId: number) {
    return api.post(`/forums/${forumId}/unmute`, { user_id: userId });
  },
  deleteThread(threadId: number) {
    return api.delete(`/threads/${threadId}`);
  },
  deletePost(postId: number) {
    return api.delete(`/posts/${postId}`);
  },
};
