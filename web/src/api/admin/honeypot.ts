import api from '@/api/index';

// ==================== 蜜罐 / 日志记录策略（管理端） ====================

export type HoneypotLevel = 'off' | 'low' | 'medium' | 'high';

export interface HoneypotSetting {
  id?: number;
  enabled: boolean;
  level: HoneypotLevel;
  // 日志记录字段（勾选哪项才落库）
  record_ua: boolean;
  record_referer: boolean;
  record_xff: boolean;
  record_query: boolean;
  record_headers: boolean;
  record_body_snippet: boolean;
  body_snippet_limit: number;
  header_allowlist: string;
  // 处置与留存
  auto_ban: boolean;
  ban_minutes: number;
  delay_ms: number;
  retention_days: number;
  updated_at?: string;
}

export interface HoneypotOverview {
  setting: HoneypotSetting;
  session_count: number;
  event_count: number;
  high_risk: number;
  last_24h: number;
  banned_count: number;
}

export interface HoneypotSession {
  id: number;
  hid: string;
  src_ip: string;
  ua_norm: string;
  referer: string;
  xff: string;
  event_count: number;
  max_risk: number;
  banned: boolean;
  first_seen: string;
  last_seen: string;
}

export interface HoneypotEvent {
  id: number;
  session_id: number;
  kind: string;
  decoy: string;
  method: string;
  path: string;
  query: string;
  ua: string;
  referer: string;
  xff: string;
  headers: string;
  body_snippet: string;
  risk: number;
  action: string;
  created_at: string;
}

export const honeypotApi = {
  getConfig() {
    return api.get<{ code: number; data: HoneypotOverview }>('/admin/honeypot/config');
  },
  saveConfig(data: Partial<HoneypotSetting>) {
    return api.put('/admin/honeypot/config', data);
  },
  listSessions(params: { page?: number; page_size?: number; keyword?: string; min_risk?: number } = {}) {
    return api.get<{ code: number; data: { list: HoneypotSession[]; total: number } }>('/admin/honeypot/sessions', { params });
  },
  sessionDetail(id: number) {
    return api.get<{ code: number; data: { session: HoneypotSession; events: HoneypotEvent[] } }>(
      `/admin/honeypot/sessions/${id}`,
    );
  },
  export(sessionId = 0) {
    return api.get('/admin/honeypot/export', { params: { session_id: sessionId } });
  },
  cleanup() {
    return api.post('/admin/honeypot/cleanup');
  },
  bans() {
    return api.get<{ code: number; data: { list: Record<string, string> } }>('/admin/honeypot/bans');
  },
  ban(ip: string, minutes = 60) {
    return api.post('/admin/honeypot/ban', { ip, minutes });
  },
  /** SQL 注入蜜罐命中统计（gap-audit #3） */
  sqliStats() {
    return api.get('/admin/honeypot/sqli');
  },
  /** JA3/JA4 采集率与反代配置模板（gap-audit #4） */
  tlsStats() {
    return api.get('/admin/honeypot/tls');
  },
  unban(ip: string) {
    return api.post('/admin/honeypot/unban', { ip });
  },
  /** 会话指纹（验证页前置采集） */
  fingerprints(sessionId: number) {
    return api.get<{ code: number; data: { list: HoneypotFingerprint[] } }>(
      `/admin/honeypot/sessions/${sessionId}/fingerprints`,
    );
  },
  /** 取证包 zip（含 SHA256 清单；导出动作留痕） */
  exportZip(sessionId = 0) {
    return api.get('/admin/honeypot/export.zip', { params: { session_id: sessionId }, responseType: 'blob' });
  },
  /** 规则反哺候选 */
  rules() {
    return api.get<{ code: number; data: { list: HoneypotRuleSuggestion[] } }>('/admin/honeypot/rules');
  },
  /** 应用规则（写入爬虫阻断名单并即时生效） */
  applyRules(keywords: string[]) {
    return api.post('/admin/honeypot/rules', { keywords });
  },
};

export interface HoneypotFingerprint {
  id: number;
  session_id: number;
  canvas: string;
  webgl: string;
  fonts: string;
  tz: string;
  lang: string;
  screen: string;
  dpr: string;
  cores: string;
  memory: string;
  client_hints: string;
  headless_flags: string;
  raw: string;
  created_at: string;
}

export interface HoneypotRuleSuggestion {
  keyword: string;
  reason: string;
  hits: number;
  kind: 'ua' | 'path';
}
