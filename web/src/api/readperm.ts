import api from '@/api/index';

// #65 作品级阅读权限：等级集合、VIP 门槛、付费最低价全部由站长在后台配置。
// 前端只消费「当前可用等级」，不硬编码策略，避免与后台配置漂移。

export interface ReadPermLevel {
  key: string;
  label: string;
}

export interface ReadPermConfig {
  enabled_levels: string[];
  labels: Record<string, string>;
  vip_min_level: number;
  paid_min_price: number;
}

export interface ReadPermDeny {
  reason: string;
  read_permission: string;
}

interface LevelsResp {
  code: number;
  data: {
    levels: ReadPermLevel[];
    vip_min_level: number;
    paid_min_price: number;
  };
}

interface AdminResp {
  code: number;
  data: {
    config: ReadPermConfig;
    levels: ReadPermLevel[];
    all_levels: string[];
    default_labels: Record<string, string>;
  };
}

export const readPermApi = {
  // 作者端下拉（登录可访问）
  levels: () => api.get<LevelsResp>('/read-permission/levels'),
  // 站长配置
  adminGet: () => api.get<AdminResp>('/admin/read-permission'),
  adminSave: (config: ReadPermConfig) => api.put('/admin/read-permission', config),
};

// 后端权限拒绝响应里的 data.reason，供读者端决定引导动作。
export const READ_PERM_REASONS = {
  login: 'login_required',
  follower: 'follower_only',
  vip: 'vip_required',
  paid: 'paid_required',
  private: 'private',
} as const;

/** 从 axios 错误里提取权限拒绝信息；非权限错误返回 null。 */
export function parseReadPermDeny(e: any): { message: string; reason: string } | null {
  const status = e?.response?.status;
  const reason = e?.response?.data?.data?.reason;
  if ((status !== 401 && status !== 403) || !reason) return null;
  return {
    reason: String(reason),
    message: e?.response?.data?.message || '当前账号无权阅读该内容',
  };
}
