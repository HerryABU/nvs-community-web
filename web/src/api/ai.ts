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
  { value: 'mindmap', label: '思维导图', hint: '把内容整理成 Mermaid mindmap（可插入正文，阅读器可渲染）' },
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
  createAiVendor(data: { name: string; sort_order?: number; prefix?: string }) {
    return api.post('/admin/ai/vendors', data);
  },
  updateAiVendor(id: number, data: { name: string; sort_order?: number; enabled?: boolean; prefix?: string }) {
    return api.put(`/admin/ai/vendors/${id}`, data);
  },
  deleteAiVendor(id: number) {
    return api.delete(`/admin/ai/vendors/${id}`);
  },
  // 聚合分类定价（chat/image/video 默认单价）
  getCategoryPrices() {
    return api.get<{ code: number; data: { list: AiCategoryPrice[] } }>('/admin/ai/category-prices');
  },
  setCategoryPrice(data: { kind: string; price_in?: number; price_out?: number; price_call?: number }) {
    return api.put('/admin/ai/category-prices', data);
  },
  // 厂商→版本→亚版本矩阵（管理端专用，含 newest 标记）
  getVendorMatrix() {
    return api.get<{ code: number; data: { vendors: AiVendorMatrixEntry[] } }>('/admin/ai/vendors/matrix');
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

  // ---------- 从渠道一键拉取模型（L5：输入渠道地址 + key 拉出模型清单） ----------
  /** POST /api/admin/ai/models/pull —— 把渠道返回的原始模型清单 + 服务端推断分类返回 */
  pullModels(data: { base_url: string; api_key?: string }) {
    return api.post<{ code: number; data: { items: ProviderModelSuggestion[] } }>('/admin/ai/models/pull', data);
  },
  /** POST /api/admin/ai/models/import —— 批量导入勾选的模型（默认对用户隐藏，由站长按需开放） */
  importModels(data: { base_url: string; api_key?: string; channel: string; items: ProviderModelImportItem[] }) {
    return api.post<{ code: number; data: { created: number } }>('/admin/ai/models/import', data);
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

  // ---------- 模型统计（按渠道 / 模型 / 厂商聚合调用量，回答「谁在花钱、谁在失败」） ----------
  usageStats(dim: 'channel' | 'model' | 'vendor', days = 30) {
    return api.get<{
      code: number;
      data: { dim: string; days: number; rows: AiUsageRow[]; total_req: number; total_tok: number; total_cost: number };
    }>('/admin/ai/usage-stats', { params: { dim, days } });
  },

  // ---------- 渠道卡密池（一条渠道一个池，模型共用） ----------
  channelKeys(base_url: string) {
    return api.get<{ code: number; data: { list: AiChannelKey[]; total: number; max_fail: number } }>(
      '/admin/ai/channel-keys',
      { params: { base_url } },
    );
  },
  addChannelKeys(data: { base_url: string; keys: string; label?: string; note?: string; weight?: number }) {
    return api.post<{ code: number; data: { message: string; added: number; dups: number; failed: number } }>(
      '/admin/ai/channel-keys',
      data,
    );
  },
  updateChannelKey(id: number, data: { base_url: string; label?: string; weight?: number; enabled?: boolean }) {
    return api.patch(`/admin/ai/channel-keys/${id}`, data);
  },
  deleteChannelKey(id: number) {
    return api.delete(`/admin/ai/channel-keys/${id}`);
  },
  testChannelKey(id: number, base_url: string) {
    return api.post<{ code: number; data: { ok: boolean; model_count?: number; error?: string } }>(
      `/admin/ai/channel-keys/${id}/test`,
      {},
      { params: { base_url } },
    );
  },

  // ---------- 渠道监控：临时输入令牌 + 用户 ID 探一次（后端不落库） ----------
  channelMonitor(data: { base_url: string; token: string; user_id?: string; model_key?: string; kind?: string }) {
    return api.post<{ code: number; data: ChannelMonitorResult }>('/admin/ai/channel-monitor', data);
  },

  // ---------- 渠道余额一览（按 base_url 聚合，回答「每条渠道还剩多少钱」） ----------
  channelsBalance() {
    return api.get<{ code: number; data: { channels: AdminChannelRow[]; probed: number; total: number } }>(
      '/admin/ai/channels/balance',
    );
  },

  // ---------- 真实成本换算（虚拟记账单位 ≠ 真金白银） ----------
  // 中转站「quota÷500000 = 美元」只是站内代币；站长必须声明充值比例，
  // 才能把「还剩 5274」折算成真金白银的成本。
  getCostRate() {
    return api.get<{
      code: number;
      data: { rate: CostRateSetting; valid: boolean; ratio: number; describe: string; real_currencies: string[] };
    }>('/admin/cost-rate');
  },
  saveCostRate(data: CostRateSetting) {
    return api.put<{ code: number; data: { rate: CostRateSetting; valid: boolean; ratio: number; describe: string } }>(
      '/admin/cost-rate',
      data,
    );
  },
  previewCostRate(data: { virtual: number } & Partial<CostRateSetting>) {
    return api.post<{
      code: number;
      data: { virtual: number; real: number; ok: boolean; real_currency: string; describe: string };
    }>('/admin/cost-rate/preview', data);
  },

  // ---------- L5 渠道对账：拉取上游余额 / 已消费 / 剩余 + 真实连通性探针 ----------
  /** GET /api/admin/ai/models/:id/channel —— 余额（对账）与连通性 */
  modelChannel(id: number) {
    return api.get<{ code: number; data: AdminChannelReport }>(`/admin/ai/models/${id}/channel`);
  },
};

/** 上游渠道余额快照（仅 OpenAI 兼容渠道支持；其余 supported=false） */
export interface AdminChannelBalance {
  provider: string;
  total_granted: number;
  total_used: number;
  total_remaining: number;
  currency: string;
  period?: string;
  fetched_at: string;
  supported: boolean;
  error?: string;
}

/** 真实连通性探针（GET /models 或一次 1-token 补全） */
export interface AdminChannelProbe {
  reachable: boolean;
  latency_ms: number;
  status_code: number;
  models?: number;
  error?: string;
}

export interface AdminChannelRow {
  base_url: string;
  models: number;
  model_name: string;
  model_id_str: string;
  provider?: string;
  supported?: boolean;
  total_granted?: number;
  total_used?: number;
  total_remaining?: number;
  currency?: string;
  fetched_at?: string;
  error?: string;
  // 单次费用（虚拟单位）
  last_cost?: number;
  last_model?: string;
  logs_error?: string;
  // 窗口统计
  window?: {
    has_data: boolean;
    cost_used: number;
    currency?: string;
    requests: number;
    per_request: number;
    minutes: number;
    tokens_in?: number;
    tokens_out?: number;
    tokens_total?: number;
    per_request_tok?: number;
    cost_per_1k_tok?: number;
  };
  // 真实成本换算（比例未配置时后端不下发这些字段）
  rate_source?: 'channel' | 'global' | 'none';
  rate?: {
    real_amount: number;
    virtual_amount: number;
    real_currency: string;
    virtual_currency: string;
    ratio: number;
    describe: string;
  };
  real_remaining?: number;
  real_used?: number;
  real_last_cost?: number;
  real_window_cost?: number;
  real_per_request?: number;
}

// 虚拟记账单位 → 真实货币的换算比例
// 语义：real_amount 单位真实货币 ⇔ virtual_amount 单位虚拟代币
export interface CostRateSetting {
  real_amount: number;
  virtual_amount: number;
  real_currency: string;
  virtual_currency: string;
}

export interface AdminChannelReport {
  model_id: number;
  model_name: string;
  model_id_str: string;
  base_url: string;
  provider: string;
  balance: AdminChannelBalance;
  probe: AdminChannelProbe;
}

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
  /** 思考型模型：正文可能落在 reasoning 字段里 */
  thinking?: boolean;
  /** 上游提供 /search 检索接口（违禁词「联网补充」先检索再归纳） */
  search_endpoint?: boolean;
  /** 对话接口自带联网检索（请求里带 web_search_options / tools） */
  search_native?: boolean;
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
  /** 上游提供 /search 检索接口 */
  search_endpoint?: boolean;
  /** 对话接口自带联网检索 */
  search_native?: boolean;
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
  prefix: string;
  sort_order: number;
  enabled: boolean;
  models: number;
}

/** 聚合分类定价（chat/image/video 默认单价） */
export interface AiCategoryPrice {
  kind: string;
  price_in: number;
  price_out: number;
  price_call: number;
  updated_at?: string;
}

/** 厂商→版本→亚版本矩阵（管理端专用） */
export interface AiVendorMatrixEntry {
  id: number;
  name: string;
  prefix: string;
  enabled: boolean;
  models: number;
  versions: {
    version: string;
    newest_sub_version: string;
    count: number;
    items: {
      id: number;
      name: string;
      model_id: string;
      sub_version: string;
      newest: boolean;
      enabled: boolean;
      user_visible: boolean;
    }[];
  }[];
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

/** 从渠道拉取的模型条目（服务端已做厂商/渠道/类别推断与后缀剥离） */
export interface ProviderModelSuggestion {
  raw_id: string;
  alias: string;
  vendor: string;
  channel: string;
  category: string;
  prefix: string;     // 模型大名 / 家族（deepseek）
  version: string;    // 版本号（v4.1）
  variant: string;    // 类型名称（flash / pro / thinking）
  build: string;      // 子号（1017 / 2024-08-06）
  sub_version: string; // 兼容字段：版本之后的原始尾段
}

/** 导入时每个模型的可覆盖字段 */
export interface ProviderModelImportItem {
  raw_id: string;
  alias: string;
  vendor: string;
  category: string;
  user_visible: boolean;
}

// AI 用量统计行（按维度聚合）
export interface AiUsageRow {
  key: string;
  name: string;
  requests: number;
  success: number;
  fail: number;
  tokens_in: number;
  tokens_out: number;
  cost: number;
  avg_latency: number;
  model_count: number;
  success_pct: number;
}

// 渠道监控返回（一次请求回答：通不通 / 剩多少 / 最近花了多少）
export interface ChannelMonitorResult {
  kind: string;
  base_url: string;
  probe_ok: boolean;
  probe_skipped?: boolean;
  probe_error?: string;
  model_count?: number;
  balance_ok: boolean;
  balance_error?: string;
  remaining?: number;
  used?: number;
  currency?: string;
  logs_error?: string;
  recent_logs?: { model_name: string; quota: number; tokens_in: number; tokens_out: number; created_at: number }[];
}

// 渠道卡密（上游 API Key，按渠道归属；明文永不回传，只有尾 4 位提示）
export interface AiChannelKey {
  id: number;
  base_url: string;
  label: string;
  weight: number;
  enabled: boolean;
  fail_count: number;
  success_total: number;
  fail_total: number;
  last_fail_at?: string;
  last_used_at?: string;
  note: string;
  hint: string;
}

// ==================== 渠道账户凭据（加密入库） ====================
// 站长把「渠道地址 + nap_ + 用户 ID + sk_」保存一次，之后监控 / 拉取都不用再粘贴。
// 密文只在服务端，前端只看得到「有没有配」与尾号。

export interface AiChannelAccount {
  id: number;
  base_url: string;
  label: string;
  user_id: string;
  enabled: boolean;
  has_token: boolean;
  token_tail: string;
  has_model_key: boolean;
  model_key_tail: string;
  last_checked_at?: string | null;
  last_ok: boolean;
  last_models: number;
  last_balance: number;
  last_used: number;
  last_currency: string;
  last_error: string;
  updated_at: string;
}

export const channelAccountApi = {
  /** 取某条渠道已保存的凭据（不存在时 exists=false） */
  get: (baseUrl: string) =>
    api.get<{ code: number; data: { exists: boolean; account?: AiChannelAccount } }>(
      '/admin/ai/channel-account',
      { params: { base_url: baseUrl } },
    ),
  /** 保存/更新：access_token / model_key 留空=不改，传 "-"=清空 */
  save: (data: { base_url: string; label?: string; user_id?: string; access_token?: string; model_key?: string }) =>
    api.post<{ code: number; data: AiChannelAccount }>('/admin/ai/channel-account', data),
  list: () => api.get<{ code: number; data: { list: AiChannelAccount[] } }>('/admin/ai/channel-accounts'),
  remove: (id: number) => api.delete(`/admin/ai/channel-account/${id}`),
};
