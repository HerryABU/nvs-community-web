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
  // #77 OC 分类与 furry 兼容
  oc_kind?: string;        // native 原生 OC / furry 兽人 OC
  species?: string;        // 物种
  species_traits?: string; // 动物特征
  anthro_type?: string;    // 拟人类型 full/anthro/feral
  visibility: 'private' | 'public';
  review: 'pending' | 'approved' | 'rejected';
  status: string;
  author_name?: string;
  created_at: string;
  updated_at: string;
}

// OC 立项制：项目（世界观/企划）
export interface OcProject {
  id: number;
  user_id: number;
  title: string;
  summary: string;
  cover_url: string;
  category: string;
  visibility: 'private' | 'public' | 'password';
  world_count: number;
  member_count: number;
  review: 'pending' | 'approved' | 'rejected';
  status: string;
  owner_name?: string;
  created_at: string;
  updated_at: string;
}

export interface OcProjectMember {
  id: number;
  project_id: number;
  user_id: number;
  role: 'owner' | 'editor' | 'viewer';
  username?: string;
  nickname?: string;
  avatar_url?: string;
}

export interface OcWorldLink {
  id: number;
  from_book_id: number;
  to_book_id: number;
  label: string;
  from_title?: string;
  to_title?: string;
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
  listBooks(params: { scope?: string; group_id?: number; project_id?: number; page_size?: number } = {}) {
    return api.get<{ code: number; data: { list: OcWorldBook[] } }>('/oc/books', { params });
  },
  getBook(id: number, token?: string) {
    return api.get<{ code: number; data: OcWorldBook }>(
      `/oc/books/${id}`,
      token ? { headers: { 'X-Oc-Project-Token': token } } : undefined,
    );
  },
  createBook(data: { title: string; summary?: string; visibility?: string; group_id?: number; project_id?: number }) {
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
  // 群组召唤 OC 角色在群里发言（强化群组 + 群组调用 OC）
  invokeOc(groupId: number, data: { agent_id?: number; prompt?: string }) {
    return api.post<{ code: number; data: { post: any; character: string } }>(
      `/groups/${groupId}/oc/invoke`,
      data,
    );
  },
  // 群组召唤整队 OC 组（反向：群组调用某个 OC 组）
  invokeOcGroup(groupId: number, data: { oc_group_id: number; prompt?: string }) {
    return api.post<{ code: number; data: { count: number; posts: any[] } }>(
      `/groups/${groupId}/oc/invoke-group`,
      data,
    );
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

// ==================== OC 组（阵容）：用户个人创作资产，与群组解耦 ====================

export interface OcGroup {
  id: number;
  user_id: number;
  name: string;
  summary: string;
  cover_url: string;
  tags: string;
  member_count: number;
  visibility: 'private' | 'public';
  review: string;
  status: string;
  owner_name?: string;
  created_at: string;
  updated_at: string;
}

export interface OcGroupMember {
  id: number;
  group_id: number;
  character_id: number;
  sort_order: number;
  character?: OcCharacter;
}

export const ocGroupApi = {
  list(params: { scope?: string; keyword?: string; page?: number; page_size?: number } = {}) {
    return api.get<{ code: number; data: { list: OcGroup[]; total: number } }>('/oc/groups', { params });
  },
  get(id: number) {
    return api.get<{ code: number; data: { group: OcGroup; members: OcGroupMember[] } }>(`/oc/groups/${id}`);
  },
  create(data: { name: string; summary?: string; cover_url?: string; tags?: string; visibility?: string; character_ids?: number[] }) {
    return api.post<{ code: number; data: OcGroup }>('/oc/groups', data);
  },
  update(id: number, data: { name?: string; summary?: string; cover_url?: string; tags?: string; visibility?: string }) {
    return api.put<{ code: number; data: OcGroup }>(`/oc/groups/${id}`, data);
  },
  remove(id: number) {
    return api.delete(`/oc/groups/${id}`);
  },
  listMembers(id: number) {
    return api.get<{ code: number; data: { members: OcGroupMember[] } }>(`/oc/groups/${id}/members`);
  },
  addMember(id: number, characterId: number) {
    return api.post(`/oc/groups/${id}/members`, { character_id: characterId });
  },
  removeMember(id: number, characterId: number) {
    return api.delete(`/oc/groups/${id}/members/${characterId}`);
  },
  reorder(id: number, characterIds: number[]) {
    return api.put(`/oc/groups/${id}/members/order`, { character_ids: characterIds });
  },
};

// ==================== OC 角色卡 AI 生成 / 补全（借鉴作者工作台 AI 写作流） ====================

export const ocAiApi = {
  // 从灵感生成一张完整 OC 角色卡（furry 时传 oc_kind/species/traits）
  generate(data: {
    concept: string;
    style_directive?: string;
    oc_kind?: string;
    species?: string;
    species_traits?: string;
    anthro_type?: string;
    extra_requirements?: string;
  }) {
    return api.post<{ code: number; data: OcCharacter }>('/oc/ai-character', data);
  },
  // 在已有角色卡基础上补全/润色
  fill(
    characterId: number,
    data: {
      focus?: string;
      extra_requirements?: string;
      oc_kind?: string;
      species?: string;
      species_traits?: string;
      anthro_type?: string;
    },
  ) {
    return api.post<{ code: number; data: OcCharacter }>(`/oc/characters/${characterId}/ai-fill`, data);
  },
  // 从概念生成世界书（含条目）
  generateBook(data: { concept: string; style_directive?: string; extra_requirements?: string; group_id?: number }) {
    return api.post<{ code: number; data: OcWorldBook }>('/oc/ai-book', data);
  },
  // 快速创建「世界书 + N 张角色卡」，可选打包 OC 组
  quickCreate(data: {
    world_concept?: string;
    world_style?: string;
    character_concept?: string;
    style_directive?: string;
    extra_requirements?: string;
    oc_kind?: string;
    species?: string;
    species_traits?: string;
    anthro_type?: string;
    character_count?: number;
    group_title?: string;
    group_id?: number;
  }) {
    return api.post<{ code: number; data: { book: OcWorldBook; characters: OcCharacter[]; group?: OcGroup | null } }>(
      '/oc/quick-create',
      data,
    );
  },
};

// ==================== OC 立项制：项目 / 成员 / 密码访问 / 跨世界链接 ====================
export const ocProjectApi = {
  list(params: { scope?: string; category?: string; keyword?: string; page?: number; page_size?: number }) {
    return api.get<{ code: number; data: { list: OcProject[]; total: number } }>('/oc/projects', { params });
  },
  get(id: number, token?: string) {
    return api.get<{ code: number; data: OcProject }>(
      `/oc/projects/${id}`,
      token ? { headers: { 'X-Oc-Project-Token': token } } : undefined,
    );
  },
  create(data: { title: string; summary?: string; cover_url?: string; category?: string; visibility: string; password?: string }) {
    return api.post<{ code: number; data: OcProject }>('/oc/projects', data);
  },
  update(id: number, data: { title: string; summary?: string; cover_url?: string; category?: string; visibility: string; password?: string }) {
    return api.put<{ code: number; data: OcProject }>(`/oc/projects/${id}`, data);
  },
  remove(id: number) {
    return api.delete<{ code: number }>(`/oc/projects/${id}`);
  },
  /** 密码公开项目：校验密码换访问码 */
  access(id: number, password: string) {
    return api.post<{ code: number; data: { token: string } }>(`/oc/projects/${id}/access`, { password });
  },
  members(id: number) {
    return api.get<{ code: number; data: { list: OcProjectMember[] } }>(`/oc/projects/${id}/members`);
  },
  addMember(id: number, username: string, role: string) {
    return api.post<{ code: number; data: OcProjectMember }>(`/oc/projects/${id}/members`, { username, role });
  },
  updateMember(id: number, uid: number, role: string) {
    return api.put<{ code: number }>(`/oc/projects/${id}/members/${uid}`, { role });
  },
  removeMember(id: number, uid: number) {
    return api.delete<{ code: number }>(`/oc/projects/${id}/members/${uid}`);
  },
  worlds(id: number) {
    return api.get<{ code: number; data: { list: OcWorldBook[] } }>(`/oc/projects/${id}/worlds`);
  },
  attachWorld(id: number, bookId: number) {
    return api.post<{ code: number }>(`/oc/projects/${id}/worlds`, { book_id: bookId });
  },
  detachWorld(id: number, bookId: number) {
    return api.delete<{ code: number }>(`/oc/projects/${id}/worlds/${bookId}`);
  },
  setCharacter(id: number, characterId: number) {
    return api.post<{ code: number }>(`/oc/projects/${id}/characters`, { character_id: characterId });
  },
  listLinks(bookId: number) {
    return api.get<{ code: number; data: { list: OcWorldLink[] } }>('/oc/world-links', { params: { book_id: bookId } });
  },
  addLink(fromBookId: number, toBookId: number, label: string) {
    return api.post<{ code: number; data: OcWorldLink }>('/oc/world-links', {
      from_book_id: fromBookId,
      to_book_id: toBookId,
      label,
    });
  },
  removeLink(id: number) {
    return api.delete<{ code: number }>(`/oc/world-links/${id}`);
  },
};
