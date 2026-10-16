import api from '@/api/index';

// 特别关注（F14）：关注关系的加强态，动态流的数据源

// 特别关注用户简要信息（列表项）
export interface SpecialFollowUser {
  id: number;
  username: string;
  nickname: string;
  avatar_url: string;
  bio: string;
}

// 动态流条目类型：novel=小说 / blog=博客 / thread=论坛帖子
export type FeedItemType = 'novel' | 'blog' | 'thread';

// 动态流统一条目（聚合小说更新/博客发布/论坛发帖，时间倒序）
export interface FeedItem {
  type: FeedItemType;
  id: number;
  title: string;
  summary: string;
  user_id: number;
  user_name: string;
  created_at: string;
  url: string;
}

export const specialApi = {
  // 设置/取消特别关注（special=true 时未关注会自动建立关注）
  toggleSpecialFollow(userId: number, special: boolean) {
    return api.post<{ code: number; data: { message: string; is_special: boolean } }>(
      `/follows/${userId}/special`,
      { special },
    );
  },
  // 我的特别关注列表
  listSpecialFollows(page: number = 1, size: number = 20) {
    return api.get<{ code: number; data: { list: SpecialFollowUser[]; total: number } }>(
      '/follows/special',
      { params: { page, size } },
    );
  },
  // 特别关注动态流（聚合小说/博客/论坛，时间倒序分页）
  getSpecialFeed(page: number = 1, size: number = 20) {
    return api.get<{ code: number; data: { list: FeedItem[]; total: number } }>(
      '/feed/special',
      { params: { page, size } },
    );
  },
};
