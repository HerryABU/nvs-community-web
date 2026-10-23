import api from './index';

// AI 调用计费（§5.3 / §13.3）
// 注意：baseURL 已含前缀（apiPrefix+/api），严禁在此硬编码 /go-nvs。

export interface AiModelItem {
  id: number;
  name: string;
  model_id: string;
  price_in: number;
  price_out: number;
  max_tokens: number;
}

export interface AiFreeQuota {
  enabled: boolean;
  calls_left: number; // -1=不限
  tokens_left: number; // -1=不限
  used_calls: number;
  used_tokens: number;
  calls_per_day: number; // 生效上限（-1 不限 / 0 不提供）
  max_tokens_per_day: number; // 生效上限（-1 不限 / 0 不限）
}

export interface AiStatus {
  ai_enabled: boolean;
  token_enabled: boolean;
  token_name: string;
  balance: number;
  models: AiModelItem[];
  free_quota?: AiFreeQuota;
}

export interface AiEstimateResult {
  tokens_in: number;
  tokens_out: number;
  cost: number;
  free_eligible?: boolean;
  balance: number;
  affordable: boolean;
}

/** 编辑区 AI 动作（与后端白名单一致） */
export const AI_ACTIONS = [
  { value: 'continue', label: '续写', hint: '基于现有内容继续向下写' },
  { value: 'polish', label: '润色', hint: '修正语病、提升表达，篇幅大致不变' },
  { value: 'expand', label: '扩写', hint: '丰富细节、对话与环境描写' },
  { value: 'summarize', label: '摘要', hint: '概括内容要点' },
  { value: 'custom', label: '自定义', hint: '把输入当作完整指令执行' },
] as const;

export const aiApi = {
  // 用户侧
  status() {
    return api.get<{ code: number; data: AiStatus }>('/ai/status');
  },
  estimate(data: { model_id: number; text: string; max_tokens?: number }) {
    return api.post<{ code: number; data: AiEstimateResult }>('/ai/estimate', data);
  },
  complete(data: {
    model_id: number;
    action: string;
    text: string;
    max_tokens?: number;
    temperature?: number;
    ref_type?: string;
    ref_id?: number;
  }) {
    return api.post<{ code: number; data: { content: string; tokens_in: number; tokens_out: number; cost: number; free?: boolean; balance_after: number } }>('/ai/complete', data);
  },

  // 免费 AI 额度（§5.3）
  freeQuotaStatus() {
    return api.get<{ code: number; data: AiFreeQuota }>('/ai/free-quota');
  },

  // 管理端
  getConfig() {
    return api.get<{ code: number; data: { enabled: boolean } }>('/admin/ai/config');
  },
  setConfig(data: { enabled: boolean }) {
    return api.put('/admin/ai/config', data);
  },
  listModels() {
    return api.get<{ code: number; data: { list: AdminAiModel[]; total: number } }>('/admin/ai/models');
  },
  createModel(data: AdminAiModelInput) {
    return api.post('/admin/ai/models', data);
  },
  updateModel(id: number, data: AdminAiModelInput) {
    return api.put(`/admin/ai/models/${id}`, data);
  },
  deleteModel(id: number) {
    return api.delete(`/admin/ai/models/${id}`);
  },

  // 管理端：免费 AI 额度（平台默认 + 按人覆盖）
  getFreeQuotaConfig() {
    return api.get<{ code: number; data: { enabled: boolean; config: { calls_per_day: number; max_tokens_per_day: number } } }>('/admin/ai/free-quota/config');
  },
  setFreeQuotaConfig(data: { enabled: boolean; calls_per_day: number; max_tokens_per_day: number }) {
    return api.put('/admin/ai/free-quota/config', data);
  },
  listFreeQuotaOverrides() {
    return api.get<{ code: number; data: AdminAiFreeQuotaOverride[] }>('/admin/ai/free-quota/overrides');
  },
  manageFreeQuotaOverride(data: { action: 'set' | 'remove'; user_id: number; daily_calls?: number; daily_tokens?: number; note?: string }) {
    return api.post('/admin/ai/free-quota/overrides', data);
  },
};

export interface AdminAiFreeQuotaOverride {
  id: number;
  user_id: number;
  daily_calls: number;
  daily_tokens: number;
  note: string;
  username?: string;
  created_at: string;
  updated_at: string;
}

export interface AdminAiModel {
  id: number;
  name: string;
  kind: string;
  model_id: string;
  base_url: string;
  has_api_key: boolean;
  price_in: number;
  price_out: number;
  price_call: number;
  max_tokens: number;
  temperature: number;
  enabled: boolean;
  priority: number;
  note: string;
  created_at: string;
  updated_at: string;
}

export interface AdminAiModelInput {
  name: string;
  kind?: string;
  model_id: string;
  base_url: string;
  api_key?: string;
  price_in?: number;
  price_out?: number;
  price_call?: number;
  max_tokens?: number;
  temperature?: number;
  enabled?: boolean;
  priority?: number;
  note?: string;
}
