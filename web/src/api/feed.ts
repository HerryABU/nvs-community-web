import api from '@/api/index';

// ==================== 短视频 / 微文（视频号 + 贴图号） ====================

export interface FeedPost {
  id: number;
  user_id: number;
  kind: 'video' | 'micro';
  title: string;
  content: string;
  video_url: string;
  cover_url: string;
  images_raw?: string;
  images?: string[];
  tags: string;
  duration: number;
  status: string;
  like_count: number;
  comment_count: number;
  favorite_count: number;
  share_count: number;
  view_count: number;
  created_at: string;
  author_name?: string;
  avatar_url?: string;
  liked?: boolean;
  favorited?: boolean;
}

export interface FeedComment {
  id: number;
  post_id: number;
  user_id: number;
  parent_id: number;
  content: string;
  like_count: number;
  created_at: string;
  author_name?: string;
  avatar_url?: string;
}

export const feedApi = {
  /** 当前微文图片上限（后台可调，默认 20） */
  config() {
    return api.get<{ code: number; data: { max_micro_images: number } }>('/feed-config');
  },
  /** 后台调节图片上限（1~50） */
  setConfig(maxMicroImages: number) {
    return api.put<{ code: number; data: { max_micro_images: number } }>('/admin/feed-config', {
      max_micro_images: maxMicroImages,
    });
  },
  list(params: { kind?: string; scope?: string; author_id?: number; page?: number; page_size?: number } = {}) {
    return api.get<{ code: number; data: { list: FeedPost[]; total: number } }>('/feed', { params });
  },
  get(id: number) {
    return api.get<{ code: number; data: FeedPost }>(`/feed/${id}`);
  },
  create(data: {
    kind: string;
    title?: string;
    content?: string;
    video_url?: string;
    cover_url?: string;
    images?: string[];
    tags?: string;
    duration?: number;
    status?: string;
  }) {
    return api.post<{ code: number; data: FeedPost }>('/feed', data);
  },
  update(id: number, data: Partial<FeedPost>) {
    return api.put<{ code: number; data: FeedPost }>(`/feed/${id}`, data);
  },
  remove(id: number) {
    return api.delete<{ code: number }>(`/feed/${id}`);
  },
  like(id: number) {
    return api.post<{ code: number; data: { liked: boolean } }>(`/feed/${id}/like`);
  },
  favorite(id: number) {
    return api.post<{ code: number; data: { favorited: boolean } }>(`/feed/${id}/favorite`);
  },
  share(id: number) {
    return api.post<{ code: number }>(`/feed/${id}/share`);
  },
  comments(id: number, params: { page?: number; page_size?: number } = {}) {
    return api.get<{ code: number; data: { list: FeedComment[]; total: number } }>(`/feed/${id}/comments`, { params });
  },
  addComment(id: number, content: string, parentId = 0) {
    return api.post<{ code: number; data: FeedComment }>(`/feed/${id}/comments`, {
      content,
      parent_id: parentId,
    });
  },
  deleteComment(cid: number) {
    return api.delete<{ code: number }>(`/feed-comments/${cid}`);
  },
  likeComment(cid: number) {
    return api.post<{ code: number; data: { liked: boolean } }>(`/feed-comments/${cid}/like`);
  },
};
