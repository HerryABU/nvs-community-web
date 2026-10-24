import api from '@/api/index';

// ==================== 多人协作（小说）前端接口层 ====================
// 规则基线见 docs/novel-collaboration.md：主笔 owner / 协作作者 co_author / 编辑 editor / 审稿 reviewer
// 收益分成用万分比 revenue_share_bp（0~10000），主笔拿剩余部分；署名顺序 byline_order 越小越前。

export const COLLAB_ROLE_LABEL: Record<string, string> = {
  owner: '主笔',
  co_author: '协作作者',
  editor: '编辑',
  reviewer: '审稿',
};

export const COLLAB_ROLE_OPTIONS = [
  { value: 'co_author', label: '协作作者' },
  { value: 'editor', label: '编辑' },
  { value: 'reviewer', label: '审稿' },
];

export type CollabRole = 'co_author' | 'editor' | 'reviewer';

export interface Collaborator {
  id: number;
  novel_id: number;
  user_id: number;
  role: string;
  status: 'pending' | 'active';
  can_publish: boolean;
  can_review: boolean;
  can_edit_others: boolean;
  revenue_share_bp: number;
  revenue_note: string;
  byline_order: number;
  invited_by: number;
  created_at: string;
  updated_at: string;
  // 运行时字段
  username?: string;
  nickname?: string;
  avatar_url?: string;
  novel_title?: string; // 邀请列表补齐的作品标题
}

/** 当前用户在该作品上的能力快照（后端 collab.Actor 计算，前端不重复判定） */
export interface CollabMe {
  is_owner: boolean;
  role: string;
  status: string;
  can_publish: boolean;
  can_review: boolean;
  can_edit_others: boolean;
  can_write: boolean;
}

export interface CollabListData {
  list: Collaborator[];
  owner_id: number;
  owner_name: string;
  byline: string[];
  me: CollabMe;
}

export interface CollabWork {
  novel_id: number;
  title: string;
  cover_url: string;
  role: string;
  role_label: string;
  revenue_share_bp: number;
  byline_order: number;
  owner_name: string;
  chapter_count: number;
}

export interface CollabUpdateInput {
  role?: CollabRole;
  can_publish?: boolean;
  can_review?: boolean;
  can_edit_others?: boolean;
  revenue_share_bp?: number;
  revenue_note?: string;
  byline_order?: number;
}

export const collabApi = {
  /** 主笔转让（不可逆）：受让人必须是已接受的协作者 */
  transferOwner(novelId: number, toUserId: number) {
    return api.post(`/novels/${novelId}/transfer`, { to_user_id: toUserId });
  },
  /** 协作成员 + 我的能力（主笔或成员可见） */
  list(novelId: number) {
    return api.get<{ code: number; data: CollabListData }>(`/novels/${novelId}/collaborators`);
  },

  /** 邀请协作者（按用户名） */
  invite(novelId: number, data: { username: string; role: CollabRole }) {
    return api.post(`/novels/${novelId}/collaborators`, data);
  },

  /** 调整角色 / 能力 / 分成 / 署名顺序 */
  update(novelId: number, userId: number, data: CollabUpdateInput) {
    return api.put(`/novels/${novelId}/collaborators/${userId}`, data);
  },

  /** 移除协作者（历史署名保留） */
  remove(novelId: number, userId: number) {
    return api.delete(`/novels/${novelId}/collaborators/${userId}`);
  },

  /** 我收到的待处理邀请 */
  invites() {
    return api.get<{ code: number; data: { list: Collaborator[] } }>('/collab/invites');
  },

  /** 我参与的协作作品 */
  works() {
    return api.get<{ code: number; data: { list: CollabWork[] } }>('/collab/works');
  },

  /** 接受 / 拒绝邀请 */
  respond(novelId: number, accept: boolean) {
    return api.post(`/collab/${novelId}/respond`, { accept });
  },

  /** 退出协作 */
  leave(novelId: number) {
    return api.post(`/collab/${novelId}/leave`);
  },
};

// ==================== 章节工作流（draft → pending → published / rejected） ====================

export const chapterWorkflowApi = {
  /** 提审：能编辑本章的人可提交 */
  submit(novelId: number, chapterNum: number) {
    return api.post(`/novels/${novelId}/chapters/${chapterNum}/submit`);
  },

  /** 审核：通过即发布，驳回回到 rejected */
  review(novelId: number, chapterNum: number, data: { approve: boolean; note?: string }) {
    return api.post(`/novels/${novelId}/chapters/${chapterNum}/review`, data);
  },

  /** 直接发布（需 can_publish） */
  publish(novelId: number, chapterNum: number) {
    return api.post(`/novels/${novelId}/chapters/${chapterNum}/publish`);
  },

  /** 撤回为草稿 */
  withdraw(novelId: number, chapterNum: number) {
    return api.post(`/novels/${novelId}/chapters/${chapterNum}/withdraw`);
  },
};

/** 章节状态展示文案与色型 */
export const CHAPTER_STATUS_META: Record<string, { label: string; type: 'info' | 'warning' | 'success' | 'danger' }> = {
  draft: { label: '草稿', type: 'info' },
  pending: { label: '待审核', type: 'warning' },
  published: { label: '已发布', type: 'success' },
  rejected: { label: '已驳回', type: 'danger' },
};

export function chapterStatusLabel(status: string): string {
  return CHAPTER_STATUS_META[status]?.label || '草稿';
}

export function chapterStatusType(status: string): 'info' | 'warning' | 'success' | 'danger' {
  return CHAPTER_STATUS_META[status]?.type || 'info';
}
