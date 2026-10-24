import api from '@/api/index';

// ==================== 注册门槛 / 邀请码 / 设备指纹 / API Key 告警（gap-audit #14 + 告警口径）====================

export interface GateConfig {
  enabled: boolean;
  require_invite: boolean;
  max_accounts_per_device: number;
  block_honeypot_ip: boolean;
  block_disposable_email: boolean;
  post_min_age_hours: number;
  comment_min_age_hours: number;
  new_user_daily_posts: number;
  new_user_days: number;
}

export interface Invite {
  id: number;
  code: string;
  created_by: number;
  max_uses: number;
  used: number;
  note: string;
  expires_at?: string | null;
  created_at: string;
}

export interface DeviceRow {
  device_id: string;
  accounts: number;
  last_ip: string;
  last_at: string;
}

export interface KeyAlarm {
  id: number;
  kind: string;
  key_id: number;
  user_id: number;
  detail: string;
  acked: boolean;
  acked_by: number;
  note: string;
  created_at: string;
}

export const gateApi = {
  // 注册前提示（无需登录）
  registerHint() {
    return api.get<{ code: number; data: Partial<GateConfig> }>('/auth/register-hint');
  },
  registerCheck(payload: { email: string; invite: string }) {
    return api.post('/auth/register-check', payload);
  },

  adminConfig() {
    return api.get<{ code: number; data: { config: GateConfig; note: string } }>('/admin/gate');
  },
  saveConfig(data: GateConfig) {
    return api.put('/admin/gate', data);
  },
  invites(page = 1, size = 20) {
    return api.get<{ code: number; data: { list: Invite[]; total: number } }>('/admin/gate/invites', {
      params: { page, size },
    });
  },
  createInvites(payload: { count: number; max_uses: number; note: string; ttl_hours: number }) {
    return api.post<{ code: number; data: { codes: string[] } }>('/admin/gate/invites', payload);
  },
  revokeInvite(id: number) {
    return api.post(`/admin/gate/invites/${id}/revoke`);
  },
  devices(keyword = '') {
    return api.get<{ code: number; data: { list: DeviceRow[]; max_accounts_per_device: number; note: string } }>(
      '/admin/gate/devices',
      { params: { keyword } },
    );
  },

  // API Key 审计告警
  alarms(page = 1, size = 20, kind = '', acked?: boolean) {
    return api.get<{ code: number; data: { list: KeyAlarm[]; total: number } }>('/admin/api-key-alarms', {
      params: { page, size, kind, acked: acked === undefined ? undefined : acked ? 1 : 0 },
    });
  },
  ackAlarm(id: number, note: string) {
    return api.post(`/admin/api-key-alarms/${id}/ack`, { note });
  },
};
