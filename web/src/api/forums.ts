import api from '@/api/index';

export const forumApi = {
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
  pinThread(id: number) {
    return api.post(`/threads/${id}/pin`);
  },
  unpinThread(id: number) {
    return api.post(`/threads/${id}/unpin`);
  },
};
