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
  // 站长可编辑项
  annotation?: string;
  custom_decoys?: string;
  // 诱饵画像 JSON（品牌名/域名/内网拓扑/运维备注/金丝雀），见 server/honeypot/decoy_profile.go
  decoy_profile?: string;
  updated_at?: string;
}

export interface HoneypotOverview {
  setting: HoneypotSetting;
  session_count: number;
  event_count: number;
  high_risk: number;
  last_24h: number;
  banned_count: number;
  key_b64?: string;
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
  /** 重新生成 / 手动修改蜜罐密钥（base64）。留空则随机新密钥；传入 key 则手动指定 */
  /**
   * 只保存诱饵内容（独立接口）。
   * 不走 saveConfig：那是全量覆盖，只提交诱饵字段会把「启用」等开关一并清成 false。
   */
  saveDecoyProfile(profile: string) {
    return api.put("/admin/honeypot/decoy-profile", { decoy_profile: profile });
  },

  regenerateKey(data: { key?: string } = {}) {
    return api.post('/admin/honeypot/regenerate-key', data);
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

/** 诱饵画像：管理员在「蜜罐 → 诱饵内容」里填，决定 .env 诱饵长什么样 */
export interface DecoyProfile {
  app_name: string;
  app_url: string;
  db_host: string;
  db_port: string;
  db_name: string;
  db_user: string;
  redis_host: string;
  redis_port: string;
  ops_note: string;
  debug_token_on: boolean;
  /** 金丝雀值：持久化，重启不变；清空 = 让后端重新生成 */
  canary: string;
  /** stealth（默认，伪装成普通密钥）/ labeled（带 NVS-CANARY- 前缀，便于自查） */
  canary_style: string;
  /** 金丝雀挂在哪一行，默认 APP_KEY */
  canary_key: string;
}
