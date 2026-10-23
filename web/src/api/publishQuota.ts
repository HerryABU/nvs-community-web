import api from './index';
import { adminApi } from './admin';

// 每日发文限额 / 用户配额体系（需求 §13.4）
// 注意：baseURL 已含前缀（apiPrefix+/api），严禁在此硬编码 /go-nvs。
export interface DailyPublishRoleLimit {
  count: number;    // 每日发布数量上限：-1=不限 / 0=禁止 / >0 上限
  word_cap: number; // 每日总字数上限：<=0=不限 / >0 上限
}
export interface DailyPublishConfig {
  enabled: boolean;
  roles: Record<string, DailyPublishRoleLimit>;
}
export interface MyPublishQuota {
  enabled: boolean;
  used_count: number;
  count_limit: number;
  used_words: number;
  word_limit: number;
  reset_at: string;
}

// 等级 → 展示名
export const PUBLISH_ROLE_LABELS: Record<string, string> = {
  reader: '普通读者',
  author: '普通作者',
  vip_author: 'VIP 作者',
  arbitrator: '仲裁者',
  police: '警方',
  admin: '管理员',
};

export const publishQuotaApi = {
  // 用户侧：今日发文额度（发布表单展示剩余量）
  myStatus() {
    return api.get<{ code: number; data: MyPublishQuota }>('/daily-publish/status');
  },

  // 管理端：读取配置
  getConfig() {
    return api.get<{ code: number; data: DailyPublishConfig }>('/admin/daily-publish/config');
  },
  // 管理端：保存配置（立即生效）
  setConfig(cfg: DailyPublishConfig) {
    return api.put('/admin/daily-publish/config', cfg);
  },
  // 管理端：按人覆盖列表
  listOverrides() {
    return api.get('/admin/daily-publish/overrides');
  },
  // 管理端：设置/撤销按人覆盖（action: set / remove）
  manageOverride(data: { action: 'set' | 'remove'; user_id: number; count?: number; word_cap?: number; note?: string }) {
    return api.post('/admin/daily-publish/overrides', data);
  },
  // 解析用户标识（数字=id，否则用户名）
  resolveUser(key: string) {
    return adminApi.resolveUser(key);
  },
};

// 数量/字数上限的友好文案
export function limitText(v: number, unit = '条'): string {
  if (v < 0) return '不限';
  if (v === 0) return '禁止';
  return `${v} ${unit}`;
}
