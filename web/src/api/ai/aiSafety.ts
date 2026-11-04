import api from '@/api/index';

/**
 * AI 内容安全闸（防越狱）。
 *
 * 三层结构：规则引擎（快、宁枉勿纵）+ 监管模型 B（蓝队检查师，批次判定、双向裁定）
 * + 用户特征（观察期 / 冷却）。
 */

export interface SafetyConfig {
  id: number;
  enabled: boolean;
  guard_input: boolean;
  guard_output: boolean;
  /** 拒绝后泄漏拦截：先声明「不能提供」再补「方向性背景」（催化剂族/温压/单元操作） */
  guard_leak: boolean;

  input_block_score: number;
  output_block_score: number;
  review_score: number;
  watch_block_score: number;
  watch_review_score: number;

  prebuffer_chars: number;
  flush_chars: number;

  watch_after_hits: number;
  watch_hours: number;
  cooldown_hits: number;
  cooldown_minutes: number;
  score_decay_hours: number;

  scan_context_turns: number;
  skip_first_turn: boolean;
  protect_system: boolean;

  allow_fiction_relief: boolean;
  allow_harm_relief: boolean;
  record_excerpt: boolean;
  excerpt_chars: number;

  system_guard_prompt: string;
  refusal_text: string;
  refusal_jailbreak_text: string;

  terminate_on_block: boolean;
  terminate_score: number;
  terminate_mode: 'strict' | 'moderate' | 'loose';
  session_terminated_text: string;
  contaminated_notice_text: string;
  moderate_blocked_text: string;
  cooldown_text: string;
  watch_notice_text: string;
  extra_terms: string;

  judge_enabled: boolean;
  judge_model_id: number;
  judge_every_turns: number;
  judge_on_suspect: boolean;
  judge_review_blocks: boolean;
  judge_block_score: number;
  judge_release_score: number;
  judge_max_chars: number;
  judge_timeout_sec: number;
  judge_fail_open: boolean;
  judge_max_turns: number;
  judge_prompt: string;

  // ── 未成年人心理危机防护（「赛博守护神」方案）──
  minor_crisis_enabled: boolean;
  minor_crisis_hotline: string;
  minor_crisis_comfort: string;
  minor_observation_hours: number;
  minor_crisis_prompt: string;

  // ── AI 绘画审核 ──
  /** 提示词初审（生成前）：命中高危词直接拒，不调上游、不扣费 */
  image_gen_guard: boolean;
  /** 出图后审（生成后，可选）：判定违规则撤回图片并退款 */
  image_output_guard: boolean;
  /** 出图后审的判定提示词（留空用内置默认） */
  image_output_prompt: string;

  // ── 绥靖模式（以疏代堵）──
  /** 终止模式：strict 锁会话 / moderate 污染标记 / loose 只拦当轮 / appease 绥靖（用户无感） */
  terminate_mode: 'strict' | 'moderate' | 'loose' | 'appease';
  /** 绥靖展示模式：replace 替代回答 / softcard 软警示卡片 / both 两者 */
  appease_display_mode: 'replace' | 'softcard' | 'both';
  /** 绥靖替代回答文案（科普/法律后果/求助引导） */
  appease_replace_text: string;
  /** 绥靖软警示卡片文案 */
  appease_soft_card_text: string;
  /** 监管模型 B 判定分 ≥ 此值时，可疑名单升级为恶意名单 */
  appease_escalate_score: number;
  /** 绥靖模式下前置给主模型的宽松安全准则（空则回退内置默认） */
  appease_system_prompt: string;

  schema_version: number;
  updated_at: string;
}

export interface SafetyEvent {
  id: number;
  user_id: number;
  username: string;
  direction: 'in' | 'out';
  action: string;
  level: string;
  score: number;
  categories: string;
  hits: string;
  excerpt: string;
  model_id: number;
  session_id: number;
  created_at: string;
}

export interface SafetyProfile {
  user_id: number;
  username: string;
  risk_score: number;
  block_count: number;
  hit_count: number;
  watch_until: string | null;
  cooldown_until: string | null;
  last_hit_at: string | null;
  updated_at: string;
}

export interface JudgeLog {
  id: number;
  user_id: number;
  username: string;
  session_id: number;
  turn: number;
  trigger: string;
  model_id: number;
  model_name: string;
  jailbroken: boolean;
  score: number;
  categories: string;
  reason: string;
  blocked: boolean;
  released: boolean;
  latency_ms: number;
  failed: boolean;
  raw_reply: string;
  created_at: string;
}

export interface SafetyStats {
  events: number;
  blocks: number;
  reviews: number;
  cooldowns: number;
  output_blocks: number;
  judged: number;
  jailbroken: number;
  judge_released: number;
  judge_failed: number;
  active_watch: number;
  active_cooldown: number;
}

/** 高危观察期用户（总览入口） */
export interface CrisisUser {
  user_id: number;
  username: string;
  nickname: string;
  event_count: number;
  observe_until: string;
  is_minor: boolean;
}

export const aiSafetyApi = {
  getConfig() {
    return api.get<{ code: number; data: { config: SafetyConfig } }>('/admin/ai/safety/config');
  },
  saveConfig(data: Partial<SafetyConfig>) {
    return api.put<{ code: number; data: { config: SafetyConfig } }>('/admin/ai/safety/config', data);
  },
  resetConfig() {
    return api.post<{ code: number; data: { config: SafetyConfig } }>('/admin/ai/safety/config/reset');
  },
  events(params: { page?: number; page_size?: number; direction?: string; action?: string; user_id?: number } = {}) {
    return api.get<{ code: number; data: { items: SafetyEvent[]; total: number; page: number } }>(
      '/admin/ai/safety/events',
      { params },
    );
  },
  profiles(params: { page?: number; page_size?: number; only_active?: string } = {}) {
    return api.get<{ code: number; data: { items: SafetyProfile[]; total: number; page: number } }>(
      '/admin/ai/safety/profiles',
      { params },
    );
  },
  clearCooldown(uid: number) {
    return api.post(`/admin/ai/safety/profiles/${uid}/clear`);
  },
  resetProfile(uid: number) {
    return api.post(`/admin/ai/safety/profiles/${uid}/reset`);
  },
  judgeLogs(params: { page?: number; page_size?: number; only_jailbreak?: string } = {}) {
    return api.get<{ code: number; data: { items: JudgeLog[]; total: number; page: number } }>(
      '/admin/ai/safety/judge-logs',
      { params },
    );
  },
  lockedSessions(params: { page?: number; page_size?: number } = {}) {
    return api.get<{ code: number; data: { items: any[]; total: number; page: number } }>(
      '/admin/ai/safety/locked-sessions',
      { params },
    );
  },
  unlockSession(sid: number) {
    return api.post(`/admin/ai/safety/sessions/${sid}/unlock`);
  },
  stats() {
    return api.get<{ code: number; data: SafetyStats }>('/admin/ai/safety/stats');
  },

  // 心理危机：高危观察期用户总览（不传 user_id）
  crisisUsers() {
    return api.get<{ code: number; data: { users: CrisisUser[] } }>('/admin/crisis-report');
  },
  // 心理危机：《高危用户心理干预报告》（传 user_id）
  crisisReport(userId: number) {
    return api.get<{ code: number; data: { user_id: number; report: string } }>(
      '/admin/crisis-report',
      { params: { user_id: userId } },
    );
  },

  // ───────────── 名单子系统（以疏代堵）─────────────

  superviseList(params: { page?: number; page_size?: number; tier?: string; status?: string } = {}) {
    return api.get<{ code: number; data: { items: SupervisedUser[]; total: number; page: number } }>(
      '/admin/supervise/list',
      { params },
    );
  },
  superviseGet(id: number) {
    return api.get<{ code: number; data: { record: SupervisedUser; logs: SuperviseLog[] } }>(
      `/admin/supervise/${id}`,
    );
  },
  superviseSetTier(id: number, tier: string, note: string) {
    return api.post(`/admin/supervise/${id}/set-tier`, { tier, note });
  },
  superviseClear(id: number, note: string) {
    return api.post(`/admin/supervise/${id}/clear`, { note });
  },
  superviseAddNote(id: number, note: string) {
    return api.post(`/admin/supervise/${id}/note`, { note });
  },
  superviseStats() {
    return api.get<{ code: number; data: SuperviseStats }>('/admin/supervise/stats');
  },

  // ───────────── 警方取证申请 / 审批流 ─────────────

  // 警方可提交申请、看自己的申请
  evidenceRequestCreate(data: { target_user_id: number; reason: string }) {
    return api.post<{ code: number; data: { request: EvidenceRequest } }>(
      '/police/evidence-requests',
      data,
    );
  },
  evidenceRequestMy(params: { page?: number; page_size?: number } = {}) {
    return api.get<{ code: number; data: { items: EvidenceRequest[]; total: number; page: number } }>(
      '/police/evidence-requests',
      { params },
    );
  },
  // 警方导出某入档用户取证包（须先获批）
  exportUserForensics(userId: number) {
    return api.get(`/police/export-user`, { params: { user_id: userId }, responseType: 'blob' });
  },
  // 站长审批
  evidenceRequestList(params: { page?: number; page_size?: number; status?: string } = {}) {
    return api.get<{ code: number; data: { items: EvidenceRequest[]; total: number; page: number } }>(
      '/admin/police/evidence-requests',
      { params },
    );
  },
  evidenceRequestReview(id: number, approve: boolean, note: string) {
    return api.post(`/admin/police/evidence-requests/${id}/review`, { approve, note });
  },
};

// 名单记录
export interface SupervisedUser {
  id: number;
  user_id: number;
  username: string;
  tier: 'suspect' | 'malicious'; // suspect 可疑 / malicious 恶意
  score: number;
  source: string;
  reasons: string; // JSON 数组
  evidence_refs: string; // JSON 数组
  status: 'active' | 'cleared';
  handled_by: number;
  note: string;
  first_seen: string;
  last_event: string;
  created_at: string;
  updated_at: string;
}

export interface SuperviseLog {
  id: number;
  supervised_id: number;
  user_id: number;
  actor_id: number;
  action: string; // upsert / escalate / clear / set_tier / note
  from_to: string;
  note: string;
  created_at: string;
  actor_name?: string;
}

export interface SuperviseStats {
  by_tier: Record<string, number>;
  cleared: number;
}

export interface EvidenceRequest {
  id: number;
  no: string;
  requester_id: number;
  target_user_id: number;
  target_name: string;
  reason: string;
  status: 'pending' | 'approved' | 'rejected';
  reviewer_id: number;
  review_note: string;
  reviewed_at: string | null;
  created_at: string;
  updated_at: string;
}
