import api from '@/api/index';

// ==================== 充值风控 / 警方工单 / AI 成本与漏斗（管理端）====================

export interface RiskConfig {
  enabled: boolean;
  hold_score: number;
  block_score: number;
  rapid_window_min: number;
  rapid_count: number;
  refund_window_days: number;
  max_refund_rate: number;
  new_account_hours: number;
  big_amount_fen: number;
  link_honeypot: boolean;
  link_device_abuse: boolean;
}

export interface RiskEvent {
  id: number;
  order_no: string;
  user_id: number;
  score: number;
  decision: 'allow' | 'hold' | 'block';
  reasons: string;
  payload: string;
  status: 'open' | 'cleared' | 'confirmed';
  handled_by: number;
  note: string;
  created_at: string;
}

export const riskApi = {
  overview() {
    return api.get<{ code: number; data: { config: RiskConfig; stats: any; note: string } }>('/admin/risk');
  },
  saveConfig(data: RiskConfig) {
    return api.put('/admin/risk', data);
  },
  events(page = 1, size = 20, decision = '', status = '') {
    return api.get<{ code: number; data: { list: RiskEvent[]; total: number } }>('/admin/risk/events', {
      params: { page, size, decision, status },
    });
  },
  handle(id: number, status: 'cleared' | 'confirmed', note: string) {
    return api.post(`/admin/risk/events/${id}/handle`, { status, note });
  },
};

// ─────────────── 警方工单 ───────────────

export interface Ticket {
  id: number;
  no: string;
  kind: string;
  priority: string;
  status: string;
  title: string;
  summary: string;
  subject_user_id: number;
  subject_novel_id: number;
  subject_post_id: number;
  session_id: number;
  evidence: string;
  requester_id: number;
  handler_id: number;
  due_at?: string | null;
  closed_at?: string | null;
  result: string;
  created_at: string;
}

export interface TicketLog {
  id: number;
  ticket_id: number;
  actor_id: number;
  action: string;
  from_to: string;
  note: string;
  created_at: string;
}

export const ticketApi = {
  list(page = 1, size = 20, filters: Record<string, any> = {}) {
    return api.get<{ code: number; data: { list: Ticket[]; total: number; stats: any } }>('/police/tickets', {
      params: { page, size, ...filters },
    });
  },
  detail(id: number) {
    return api.get<{ code: number; data: { ticket: Ticket; logs: TicketLog[] } }>(`/police/tickets/${id}`);
  },
  create(payload: Record<string, any>) {
    return api.post<{ code: number; data: Ticket }>('/police/tickets', payload);
  },
  assign(id: number, handler_id: number) {
    return api.post(`/police/tickets/${id}/assign`, { handler_id });
  },
  setStatus(id: number, status: string, note: string) {
    return api.post(`/police/tickets/${id}/status`, { status, note });
  },
  addNote(id: number, note: string) {
    return api.post(`/police/tickets/${id}/notes`, { note });
  },
};

// ─────────────── AI 成本：预算 / 限流 / 漏斗 / 语音 ───────────────

export interface BudgetConfig {
  enabled: boolean;
  daily_cap: number;
  monthly_cap: number;
  hard_stop: boolean;
  queue_enabled: boolean;
  max_concurrent: number;
  queue_timeout_sec: number;
  peak_hours: number[];
  peak_concurrent: number;
  alert_at_ratio: number;
}

export const aiCostApi = {
  budget() {
    return api.get<{ code: number; data: any }>('/admin/ai/budget');
  },
  saveBudget(data: BudgetConfig) {
    return api.put('/admin/ai/budget', data);
  },
  throttle() {
    return api.get<{ code: number; data: any }>('/admin/ai/throttle');
  },
  saveThrottle(data: Record<string, any>) {
    return api.put('/admin/ai/throttle', data);
  },
  funnel(days = 30) {
    return api.get<{ code: number; data: any }>('/admin/ai/funnel', { params: { days } });
  },
  audioModels() {
    return api.get<{ code: number; data: any }>('/admin/ai/audio-models');
  },
  /** 语音合成 / 音乐：返回站内音频 URL */
  speech(payload: { text: string; voice?: string; kind?: 'tts' | 'music'; model_id?: number }) {
    return api.post<{ code: number; data: { url: string; mime: string; model: string } }>('/ai/speech', payload);
  },
};
