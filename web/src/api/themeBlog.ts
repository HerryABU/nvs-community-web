import api from '@/api/index';

// ==================== 高级博客主题（用户可选） ====================

export interface ThemeBlogAuthor {
  id: number;
  username: string;
  nickname: string;
  avatar_url: string;
  signature?: string;
  bio?: string;
}

export interface ThemeBlogPost {
  id: number;
  title: string;
  summary: string;
  category_id: number;
  category: string;
  cover_url: string;
  tags: string[];
  view_count: number;
  like_count: number;
  favorite_count: number;
  comment_count: number;
  is_pinned: boolean;
  created_at: string;
  month: string;
}

export interface ThemeBlogOverview {
  author: ThemeBlogAuthor;
  stats: {
    posts: number; categories: number; tags: number;
    views: number; likes: number; favorites: number; comments: number;
  };
  categories: { id: number; parent_id: number; name: string; count: number }[];
  tags: { name: string; count: number }[];
  archives: { month: string; count: number }[];
  posts: ThemeBlogPost[];
  viewer_id: number;
}

export interface FeatureTheme {
  user_id: number;
  username: string;
  nickname: string;
  avatar_url: string;
  signature: string;
  post_count: number;
  recent: { id: number; title: string; cover_url: string }[];
}

export const themeBlogApi = {
  overview(key: string) {
    return api.get<{ code: number; data: ThemeBlogOverview }>(`/theme-blog/${encodeURIComponent(key)}/overview`);
  },
  featureThemes() {
    return api.get<{ code: number; data: { list: FeatureTheme[] } }>('/feature-themes');
  },
};
