import api from '@/api/index';

// ===== 类型定义（与 server/models/message.go 对应）=====

// 私信消息（列表按 id 倒序返回，新消息在前）
export interface MessageItem {
  id: number;
  sender_id: number;
  receiver_id: number;
  content: string;
  read_at: string | null; // null=对方未读
  recalled: boolean; // 已撤回
  created_at: string;
}

// 会话列表项
export interface ConversationItem {
  peer_id: number;
  peer_username: string;
  peer_nickname: string;
  peer_avatar_url: string;
  last_message: MessageItem | null;
  unread: number;
  last_at: string;
}

// 私信设置
export interface MessageSettings {
  require_follow_to_message: boolean;
}

// ===== API =====
export const messageApi = {
  // 发送私信 body: { receiver_id, content }
  send(receiverId: number, content: string) {
    return api.post<{ code: number; data: MessageItem }>('/messages', {
      receiver_id: receiverId,
      content,
    });
  },

  // 撤回私信（仅发送者本人、2 分钟内）body: { id }
  recall(id: number) {
    return api.post<{ code: number; message: string }>('/messages/recall', { id });
  },

  // 单方删除会话 body: { peer_id }
  deleteConversation(peerId: number) {
    return api.post<{ code: number; message: string }>('/messages/conversation/delete', {
      peer_id: peerId,
    });
  },

  // 会话列表（对方信息 + 最后一条消息 + 未读数）
  conversations() {
    return api.get<{ code: number; data: { list: ConversationItem[]; total: number } }>(
      '/messages/conversations'
    );
  },

  // 与某人的聊天记录（分页，新消息在前）
  messagesWith(peerId: number, page = 1, pageSize = 30) {
    return api.get<{
      code: number;
      data: { list: MessageItem[]; total: number; page: number; has_more: boolean };
    }>('/messages/with/' + peerId, { params: { page, page_size: pageSize } });
  },

  // 把对方发给我的消息标记已读 body: { peer_id }
  markRead(peerId: number) {
    return api.post<{ code: number; message: string }>('/messages/read', { peer_id: peerId });
  },

  // 未读私信总数（导航栏徽标轮询）
  unreadCount() {
    return api.get<{ code: number; data: { unread: number } }>('/messages/unread-count');
  },

  // 我的私信设置
  getSettings() {
    return api.get<{ code: number; data: MessageSettings }>('/messages/settings');
  },

  // 更新私信设置
  updateSettings(requireFollowToMessage: boolean) {
    return api.put<{ code: number; data: MessageSettings }>('/messages/settings', {
      require_follow_to_message: requireFollowToMessage,
    });
  },
};

// 私信内容最大字符数（与后端 utils.ClampString 上限一致，前端 maxlength 同步）
export const MESSAGE_MAX_LENGTH = 2000;

// 撤回时间窗口（毫秒），与后端 models.recallWindow 一致
export const RECALL_WINDOW_MS = 2 * 60 * 1000;
