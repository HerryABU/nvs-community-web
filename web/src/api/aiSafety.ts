import api from './index';

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
};
