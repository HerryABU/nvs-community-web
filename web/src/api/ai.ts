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
  { value: 'character', label: '角色设定', hint: '按线索生成角色卡（外貌/缺陷/秘密/说话方式）' },
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
    byok?: boolean;
    byok_model?: string;
  }) {
    return api.post<{ code: number; data: { content: string; tokens_in: number; tokens_out: number; cost: number; free?: boolean; byok?: boolean; balance_after: number } }>('/ai/complete', data);
  },

  // 免费 AI 额度（§5.3）
  freeQuotaStatus() {
    return api.get<{ code: number; data: AiFreeQuota }>('/ai/free-quota');
  },

  // 用户自备 API Key（§5.5 BYOK）
  myKeys() {
    return api.get<{ code: number; data: { list: MyAiKey[]; max: number } }>('/ai/my-keys');
  },
  createMyKey(data: { label?: string; base_url: string; api_key: string; model_id?: string }) {
    return api.post<{ code: number; data: { message: string; id: number } }>('/ai/my-keys', data);
  },
  deleteMyKey(id: number) {
    return api.delete(`/ai/my-keys/${id}`);
  },
  setDefaultMyKey(id: number) {
    return api.post(`/ai/my-keys/${id}/default`);
  },

  // 长篇 AI 记忆（§13.5）
  memoryList(novelId: number) {
    return api.get<{ code: number; data: { list: AiNovelMemory[]; outline: boolean; summaries: number; budget: number; recent: number } }>('/ai/memory', { params: { novel_id: novelId } });
  },
  saveOutline(data: { novel_id: number; content: string }) {
    return api.put('/ai/memory/outline', data);
  },
  saveSummary(data: { novel_id: number; chapter_id: number; content: string }) {
    return api.put('/ai/memory/summary', data);
  },
  memorySaveCharacters(data: { novel_id: number; content: string }) {
    return api.put('/ai/memory/characters', data);
  },
  deleteSummary(data: { novel_id: number; chapter_id: number }) {
    return api.delete('/ai/memory/summary', { data });
  },
  previewContext(data: { novel_id: number; chapter_id?: number; text?: string; budget?: number }) {
    return api.post<{ code: number; data: { context: string; tokens: number } }>('/ai/memory/context', data);
  },

  // @ai 评论区机器人（§5.6）
  botStatus() {
    return api.get<{ code: number; data: { enabled: boolean; used: number; daily_left: number } }>('/ai/bot/status');
  },
  adminGetBotConfig() {
    return api.get<{ code: number; data: { enabled: boolean; config: { daily_limit: number; agent_id: number; signature: string } } }>('/admin/ai/bot/config');
  },
  adminSetBotConfig(data: { enabled: boolean; daily_limit: number; agent_id: number; signature: string }) {
    return api.put('/admin/ai/bot/config', data);
  },

  // 管理端
  getConfig() {
    return api.get<{ code: number; data: { enabled: boolean } }>('/admin/ai/config');
  },
  setConfig(data: { enabled: boolean }) {
    return api.put('/admin/ai/config', data);
  },
  // ---------- 数据审计 / 功能开关 / 厂商字典（管理端） ----------
  aiAuditStats() {
    return api.get<{ code: number; data: Record<string, number> }>('/admin/ai/audit/stats');
  },
  aiAuditSessions(params: { user_id?: number; page?: number } = {}) {
    return api.get<{ code: number; data: { list: any[]; total: number; page: number } }>('/admin/ai/audit/sessions', { params });
  },
  aiAuditSessionMessages(id: number) {
    return api.get<{ code: number; data: { session: any; messages: any[] } }>(`/admin/ai/audit/sessions/${id}/messages`);
  },
  aiAuditMedia(params: { kind?: string; status?: string; page?: number } = {}) {
    return api.get<{ code: number; data: { list: any[]; total: number; page: number } }>('/admin/ai/audit/media', { params });
  },
  aiAuditCalls(params: { status?: string; page?: number } = {}) {
    return api.get<{ code: number; data: { list: any[]; total: number; page: number } }>('/admin/ai/audit/calls', { params });
  },
  getAiFeatures() {
    return api.get<{ code: number; data: AiFeatureFlags }>('/admin/ai/features');
  },
  setAiFeatures(data: AiFeatureFlags) {
    return api.put('/admin/ai/features', data);
  },
  listAiVendors() {
    return api.get<{ code: number; data: { list: AiVendor[] } }>('/admin/ai/vendors');
  },
  createAiVendor(data: { name: string; sort_order?: number }) {
    return api.post('/admin/ai/vendors', data);
  },
  updateAiVendor(id: number, data: { name: string; sort_order?: number; enabled?: boolean }) {
    return api.put(`/admin/ai/vendors/${id}`, data);
  },
  deleteAiVendor(id: number) {
    return api.delete(`/admin/ai/vendors/${id}`);
  },
  // ---------- API 卡池（多密钥轮询） ----------
  aiModelKeys(modelId: number) {
    return api.get<{ code: number; data: { list: any[]; total: number; max_fail: number } }>(`/admin/ai/models/${modelId}/keys`);
  },
  aiModelKeyCreate(modelId: number, data: { label?: string; key?: string; keys?: string; weight?: number }) {
    return api.post(`/admin/ai/models/${modelId}/keys`, data);
  },
  aiModelKeyUpdate(modelId: number, keyId: number, data: { label?: string; weight?: number; enabled?: boolean; reset?: boolean }) {
    return api.put(`/admin/ai/models/${modelId}/keys/${keyId}`, data);
  },
  aiModelKeyDelete(modelId: number, keyId: number) {
    return api.delete(`/admin/ai/models/${modelId}/keys/${keyId}`);
  },
  aiModelKeyTest(modelId: number, data: { key_id?: number; key?: string }) {
    return api.post<{ code: number; data: { ok: boolean; message: string } }>(`/admin/ai/models/${modelId}/keys/test`, data);
  },

  // ---------- 风格字典 / 扣子默认配置 ----------
  listAiStyles(params: { kind?: string } = {}) {
    return api.get<{ code: number; data: { list: AiStyle[] } }>('/admin/ai/styles', { params });
  },
  createAiStyle(data: { kind: string; name: string; sort_order?: number }) {
    return api.post('/admin/ai/styles', data);
  },
  updateAiStyle(id: number, data: { name: string; sort_order?: number; enabled?: boolean }) {
    return api.put(`/admin/ai/styles/${id}`, data);
  },
  deleteAiStyle(id: number) {
    return api.delete(`/admin/ai/styles/${id}`);
  },
  getAiCozeDefault() {
    return api.get<{ code: number; data: AiCozeDefault }>('/admin/ai/coze-default');
  },
  setAiCozeDefault(data: AiCozeDefault) {
    return api.put('/admin/ai/coze-default', data);
  },
  aiModelPresets(params: { kind?: string } = {}) {
    return api.get<{ code: number; data: { list: AiModelPreset[] } }>('/admin/ai/presets', { params });
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

export interface MyAiKey {
  id: number;
  label: string;
  base_url: string;
  model_id: string;
  enabled: boolean;
  is_default: boolean;
  has_key: boolean;
  last_used_at: string | null;
  created_at: string;
}

export interface AiNovelMemory {
  id: number;
  novel_id: number;
  kind: 'outline' | 'chapter_summary';
  chapter_id: number;
  content: string;
  tokens: number;
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
  api_kind?: string;
  provider_hint?: string;
  vendor?: string;
  note: string;
  created_at: string;
  updated_at: string;
}

/** AI 模型预设（绘画/视频/对话，一键填充上游参数） */
export interface AiModelPreset {
  key: string;
  name: string;
  kind: string;
  api_kind: string;
  model_id: string;
  base_url: string;
  price_call: number;
  provider_hint: string;
  vendor?: string;
  note: string;
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
  api_kind?: string;
  provider_hint?: string;
  vendor?: string;
  note?: string;
}

/** AI 功能开关（平台级） */
export interface AiFeatureFlags {
  allow_byok: boolean;
  allow_i2i: boolean;
  allow_faceswap: boolean;
  context_msgs: number;
  daily_chat_limit: number;
  public_gallery: boolean;
  max_cost_per_call: number;
  confirm_threshold: number;
}

/** AI 厂商（含引用计数，便于判断能否删除） */
export interface AiVendor {
  id: number;
  name: string;
  sort_order: number;
  enabled: boolean;
  models: number;
}

/** 作品风格字典 */
export interface AiStyle {
  id: number;
  kind: string;
  name: string;
  sort_order: number;
  enabled: boolean;
}

/** 扣子全局默认配置 */
export interface AiCozeDefault {
  base_url: string;
  timeout_seconds: number;
}
