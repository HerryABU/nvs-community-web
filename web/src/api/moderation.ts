import api from './index';

// ==================== AI 内容审核（moderation 包） ====================
// 两个后台入口共用本文件：
//  - AI 审核（功能配置）：config / stats / batch / malicious
//  - 审核（待办处置）：records / decide
//
// 所有策略值（批次间隔、阈值、是否自动下架）都来自后端 moderation_config，
// 前端只做「选项」不写死默认值，避免前后端两套规则。

/** 内容作用域（与后端 moderation.Scope* 一致） */
export type ModScope = 'novel' | 'chapter' | 'blog' | 'forum' | 'thread' | 'post' | 'comment';

export const MOD_SCOPE_LABELS: Record<string, string> = {
  novel: '作品',
  chapter: '章节',
  blog: '博客',
  forum: '版块',
  thread: '帖子',
  post: '回帖',
  comment: '评论',
};

export const MOD_VERDICT_LABELS: Record<string, string> = {
  pass: '合规',
  fail: '违规',
  review: '存疑',
  skipped: '已跳过',
};

export const MOD_TRIGGER_LABELS: Record<string, string> = {
  write: '发布送检',
  report: '举报触发',
  batch: '定时批审',
  manual: '手动送检',
};

export const MOD_ACTION_LABELS: Record<string, string> = {
  pass: '放行',
  delete: '删除留证',
  ban: '封禁留证',
};

export interface ModerationConfig {
  id: number;
  enabled: boolean;
  model_id: number;
  novel_blog_on_write: boolean;
  forum_comment_batch: boolean;
  batch_interval_hours: number;
  auto_hide_on_fail: boolean;
  malicious_threshold: number;
  recheck_report_count: number;
  stop_review_after: number;
  hide_from_reporter: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface ModerationRecord {
  id: number;
  scope: string;
  target_type: string;
  target_id: number;
  trigger: string;
  model_id: number;
  model_name: string;
  verdict: string;
  compliant: boolean;
  reason: string;
  /** JSON 数组字符串，如 ["涉黄","暴力"] */
  categories: string;
  report_count: number;
  re_triggered: boolean;
  review_status: string;
  review_action: string;
  reviewer_id: number;
  review_note: string;
  reviewed_at: string | null;
  created_at: string;
}

export interface ModerationStats {
  total_records: number;
  pass_count: number;
  fail_count: number;
  review_count: number;
  skipped_count: number;
  pending_count: number;
  malicious_reporters: number;
  banned_reporters: number;
  hidden_count: number;
}

export interface MaliciousReporter {
  id: number;
  user_id: number;
  total_reports: number;
  malicious_count: number;
  reporting_banned: boolean;
  ban_reason: string;
  banned_at: string | null;
  updated_at: string;
}

export const moderationApi = {
  /** AI 审核配置 */
  getConfig() {
    return api.get<{ code: number; data: ModerationConfig }>('/admin/moderation/config');
  },
  saveConfig(data: Partial<ModerationConfig>) {
    return api.put<{ code: number; data: ModerationConfig }>('/admin/moderation/config', data);
  },

  /** 概览统计 */
  stats() {
    return api.get<{ code: number; data: ModerationStats }>('/admin/moderation/stats');
  },

  /** 审核记录（待办 / 全部） */
  records(params: {
    scope?: string;
    verdict?: string;
    trigger?: string;
    review_status?: string;
    /** 待办口径：违规/存疑且未处置（与概览的「待人工处置」一致） */
    todo?: string;
    page?: number;
  } = {}) {
    return api.get<{ code: number; data: { items: ModerationRecord[]; total: number; page: number } }>(
      '/admin/moderation/records',
      { params },
    );
  },

  /** 人工处置：pass / delete / ban */
  decide(id: number, action: 'pass' | 'delete' | 'ban', reason?: string) {
    return api.post<{ code: number; data: { message: string; action: string } }>(
      `/admin/moderation/records/${id}/decide`,
      { action, reason: reason || '' },
    );
  },

  /** 立即执行一次批量审核 */
  runBatch() {
    return api.post<{ code: number; data: { reviewed: number } }>('/admin/moderation/batch');
  },

  /** 恶意举报者列表 */
  malicious() {
    return api.get<{ code: number; data: MaliciousReporter[] }>('/admin/moderation/malicious');
  },

  /** 恢复某用户举报功能 */
  unbanReporter(id: number) {
    return api.post(`/admin/moderation/malicious/${id}/unban`);
  },
};
