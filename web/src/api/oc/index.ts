import api from '@/api/index';

// ==================== OC 模式（角色卡 / 世界书 / 群内 AI 角色 / 对话流） ====================
// 规则见 docs/oc-mode.md；字段划分借鉴 SillyTavern（酒馆）V2 卡片与 Lorebook 条目，
// 养成部分借鉴「小团子/咔伴」：互动改变 OC 状态（情绪/好感/精力）。

export interface OcCharacter {
  id: number;
  user_id: number;
  name: string;
  avatar_url: string;
  persona: string;
  appearance: string;
  speech: string;
  scenario: string;
  first_message: string;
  mes_example: string;
  system_prompt: string;
  post_history: string;
  creator_notes: string;
  greetings: string;
  book_id: number;
  mood: number;
  affinity: number;
  energy: number;
  emote_map: string;
  tags: string;
  visibility: 'private' | 'public';
  review: 'pending' | 'approved' | 'rejected';
  status: string;
  author_name?: string;
  created_at: string;
  updated_at: string;
}

export interface OcWorldEntry {
  id: number;
  book_id: number;
  kind: string;
  key: string;
  content: string;
  secondary_keys: string;
  selective: boolean;
  logic: number;
  constant: boolean;
  enabled: boolean;
  probability: number;
  case_sensitive: boolean;
  whole_word: boolean;
  insertion_order: number;
  priority: number;
  position: number;
  depth: number;
  group: string;
  sort_order: number;
}

export interface OcWorldBook {
  id: number;
  user_id: number;
  group_id: number;
  title: string;
  summary: string;
  version: number;
  scan_depth: number;
  token_budget: number;
  recursive_scan: boolean;
  is_official: boolean;
  visibility: 'private' | 'public';
  review: string;
  status: string;
  entries?: OcWorldEntry[];
}

export interface OcSetting {
  group_id: number;
  enabled: boolean;
  mode: 'mixed' | 'ai_only';
  audit_level: 'strict' | 'loose' | 'redteam';
  audit_timing: 'pre' | 'post';
  window_size: number;
  window_rounds: number;
  daily_budget: number;
  flow_budget: number;
  max_turns: number;
  auto_run: boolean;
  redteam_enabled: boolean;
  redteam_note: string;
}

export interface OcAgent {
  id: number;
  group_id: number;
  character_id: number;
  model_id: number;
  template_id: number;
  strategy: string;
  enabled: boolean;
  budget: number;
  character_name?: string;
  model_name?: string;
}

export interface OcFlow {
  id: number;
  group_id: number;
  starter_id: number;
  title: string;
  book_id: number;
  schedule: string;
  status: 'running' | 'paused' | 'stopped';
  turn_count: number;
  max_turns: number;
  stop_reason: string;
  created_at: string;
}

export interface OcTurn {
  id: number;
  flow_id: number;
  seq: number;
  character_id: number;
  agent_id: number;
  user_id: number;
  content: string;
  emotion: string;
  model_name: string;
  cost: number;
  review: string;
  character_name?: string;
  is_ai: boolean;
  created_at: string;
}

export interface OcWindow {
  id: number;
  flow_id: number;
  from_seq: number;
  to_seq: number;
  turn_count: number;
  level: string;
  result: string;
  hits: string;
  note: string;
  created_at: string;
}

export const ocApi = {
  // 群配置
  getSetting(groupId: number) {
    return api.get<{ code: number; data: { setting: OcSetting; agents: OcAgent[]; can_manage: boolean; daily_used: number } }>(
      `/groups/${groupId}/oc`,
    );
  },
  saveSetting(groupId: number, data: Partial<OcSetting>) {
    return api.put(`/groups/${groupId}/oc`, data);
  },

  // 角色卡
  listCharacters(params: { scope?: string; keyword?: string; page?: number; page_size?: number } = {}) {
    return api.get<{ code: number; data: { list: OcCharacter[]; total: number } }>('/oc/characters', { params });
  },
  getCharacter(id: number) {
    return api.get<{ code: number; data: OcCharacter }>(`/oc/characters/${id}`);
  },
  createCharacter(data: Partial<OcCharacter>) {
    return api.post('/oc/characters', data);
  },
  updateCharacter(id: number, data: Partial<OcCharacter>) {
    return api.put(`/oc/characters/${id}`, data);
  },
  deleteCharacter(id: number) {
    return api.delete(`/oc/characters/${id}`);
  },

  // 世界书
  listBooks(params: { scope?: string; group_id?: number } = {}) {
    return api.get<{ code: number; data: { list: OcWorldBook[] } }>('/oc/books', { params });
  },
  getBook(id: number) {
    return api.get<{ code: number; data: OcWorldBook }>(`/oc/books/${id}`);
  },
  createBook(data: { title: string; summary?: string; visibility?: string; group_id?: number }) {
    return api.post('/oc/books', data);
  },
  updateBook(id: number, data: { title?: string; summary?: string; visibility?: string }) {
    return api.put(`/oc/books/${id}`, data);
  },
  deleteBook(id: number) {
    return api.delete(`/oc/books/${id}`);
  },
  saveEntry(bookId: number, data: Partial<OcWorldEntry> & { id?: number }) {
    return api.post(`/oc/books/${bookId}/entries`, data);
  },
  deleteEntry(bookId: number, entryId: number) {
    return api.delete(`/oc/books/${bookId}/entries/${entryId}`);
  },

  // 群内 AI 角色
  addAgent(groupId: number, data: { character_id: number; model_id?: number; template_id?: number; strategy?: string }) {
    return api.post(`/groups/${groupId}/oc/agents`, data);
  },
  updateAgent(groupId: number, id: number, data: Record<string, any>) {
    return api.put(`/groups/${groupId}/oc/agents/${id}`, data);
  },
  removeAgent(groupId: number, id: number) {
    return api.delete(`/groups/${groupId}/oc/agents/${id}`);
  },

  // 对话流
  listFlows(groupId: number) {
    return api.get<{ code: number; data: { list: OcFlow[] } }>(`/groups/${groupId}/oc/flows`);
  },
  createFlow(groupId: number, data: { title?: string; schedule?: string; book_id?: number }) {
    return api.post(`/groups/${groupId}/oc/flows`, data);
  },
  listTurns(flowId: number, afterSeq = 0, limit = 100) {
    return api.get<{ code: number; data: { list: OcTurn[] } }>(`/oc/flows/${flowId}/turns`, {
      params: { after_seq: afterSeq, limit },
    });
  },
  humanTurn(flowId: number, data: { character_id: number; content: string }) {
    return api.post(`/oc/flows/${flowId}/turns`, data);
  },
  step(flowId: number) {
    return api.post<{ code: number; data: { turn?: OcTurn; stopped: boolean; reason?: string } }>(
      `/oc/flows/${flowId}/step`,
    );
  },
  setStatus(flowId: number, status: string, reason = '') {
    return api.post(`/oc/flows/${flowId}/status`, { status, reason });
  },
  audit(flowId: number, toSeq = 0) {
    return api.post(`/oc/flows/${flowId}/audit`, { to_seq: toSeq });
  },
  windows(flowId: number) {
    return api.get<{ code: number; data: { list: OcWindow[] } }>(`/oc/flows/${flowId}/windows`);
  },
  report(flowId: number) {
    return api.get(`/oc/flows/${flowId}/report`);
  },
  publish(flowId: number, data: Record<string, any>) {
    return api.post(`/oc/flows/${flowId}/publish`, data);
  },
};
