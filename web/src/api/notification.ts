import api from './index';

export interface NotificationItem {
  id: number;
  user_id: number;
  actor_id: number;
  actor_name: string;
  type: string; // thread_reply / comment_reply / novel_comment / blog_comment
  title: string;
  content: string;
  link: string;
  read: boolean;
  created_at: string;
}

export const notificationApi = {
  // 我的通知列表（未读优先，分页）
  list(page = 1, pageSize = 20) {
    return api.get('/notifications', { params: { page, page_size: pageSize } });
  },
  // 未读数（导航栏徽标轮询）
  unreadCount() {
    return api.get('/notifications/unread-count');
  },
  // 标记已读；id 缺省/0 = 全部已读
  markRead(id?: number) {
    return api.post('/notifications/read', { id: id || 0 });
  },
};
