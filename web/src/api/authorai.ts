import api from '@/api/index';

// 作者工作台内置 AI 写作流（与 AI 中台数据隔离）。
// 后端：server/handlers/authorai + server/models/authorai。

export interface AiWriteProject {
  id: number;
  user_id: number;
  novel_id: number;
  title: string;
  setting: string;
  characters: string;
  timeline: string;
  outline: string;
  style_preset: string;
  style_detail: string;
  /** 引子 / 话题（ADRO Anchor）：全书拆解的锚点 */
  seed_topic: string;
  body_draft: string;
  // 四个素材模块的「用户补充要求」（持久化输入状态，空=待 AI 生成）
  setting_input: string;
  character_input: string;
  outline_input: string;
  timeline_input: string;
  status: string;
  created_at: string;
  updated_at: string;
}

// 独立章节实体（authorai_chapters 表）
export interface AiWriteChapter {
  id: number;
  project_id: number;
  order: number;
  title: string;
  outline: string;
  body: string;
  status: string;
  created_at: string;
  updated_at: string;
}

// 时间线 [LOOP] 解析结果
export interface TimelineLoop {
  loop_id: string;
  parent_loop_id: string;
  depth: number;
  open_line: number;
  close_line: number;
}
export interface TimelineCheckResult {
  has_loop: boolean;
  loops: TimelineLoop[];
  events: any[];
  errors: string[];
  ok: boolean;
}

// ── 结构化细节卡（NovelForge 式独立实体，真源；四个大字段是投影）──
export type DetailModule = 'setting' | 'character' | 'outline' | 'timeline';
export interface AiWriteDetail {
  id: number;
  project_id: number;
  module: DetailModule;
  title: string;
  content: string;
  sort_order: number;
  status: 'draft' | 'generated' | 'refined';
  created_at: string;
  updated_at: string;
}

export const DETAIL_MODULES: { value: DetailModule; label: string; icon: string; hint: string }[] = [
  { value: 'setting', label: '世界观 / 设定', icon: '🌍', hint: '时代、地理、势力、核心矛盾' },
  { value: 'character', label: '人物', icon: '👤', hint: '角色卡、欲望、秘密、关系张力' },
  { value: 'outline', label: '大纲', icon: '📑', hint: '分卷 / 分章、节奏与伏笔' },
  { value: 'timeline', label: '时间线', icon: '🕰️', hint: '大事年表、[LOOP] 嵌套循环与回环' },
];

export type RefineMode = 'socratic' | 'feynman' | 'direct';
export const REFINE_MODES: { value: RefineMode; label: string; hint: string; icon: string }[] = [
  { value: 'socratic', label: '苏格拉底式追问', hint: 'AI 不改稿，用反问把没想清楚的地方问出来', icon: '❓' },
  { value: 'feynman', label: '费曼式复述', hint: 'AI 用大白话复述，指出讲不通的含糊点', icon: '🗣️' },
  { value: 'direct', label: '直接改写', hint: '按你的指令只改这一条，可一键应用', icon: '✍️' },
];

export interface RefineTurn { role: 'user' | 'assistant'; content: string }
export interface RefineResult {
  mode: RefineMode;
  reply: string;
  applied: boolean;
  detail: AiWriteDetail;
}

export type StylePreset = 'realism' | 'hard_sci_fi' | 'light_novel' | 'custom' | '';

export const STYLE_OPTIONS: { value: StylePreset; label: string }[] = [
  { value: '', label: '默认（自然流畅）' },
  { value: 'realism', label: '现实主义' },
  { value: 'hard_sci_fi', label: '硬科幻' },
  { value: 'light_novel', label: '轻小说' },
  { value: 'custom', label: '自定义（以补充要求为准）' },
];

export const authorAiApi = {
  list() {
    return api.get<{ code: number; data: AiWriteProject[] }>('/author-ai/projects');
  },
  create(data: { title: string; style_preset?: StylePreset; style_detail?: string; seed_topic?: string }) {
    return api.post<{ code: number; data: AiWriteProject }>('/author-ai/projects', data);
  },
  get(id: number) {
    return api.get<{ code: number; data: AiWriteProject }>(`/author-ai/projects/${id}`);
  },
  update(id: number, data: Partial<AiWriteProject>) {
    return api.put<{ code: number; data: AiWriteProject }>(`/author-ai/projects/${id}`, data);
  },
  remove(id: number) {
    return api.delete(`/author-ai/projects/${id}`);
  },
  generate(id: number, data: { component: string; model_id?: number; prompt?: string }) {
    return api.post<{ code: number; data: { component: string; content: string } }>(
      `/author-ai/projects/${id}/generate`,
      data,
    );
  },
  // 素材一键全生成（设定→人物→大纲→时间线）
  generateAll(id: number, data: { model_id?: number; prompt?: string }) {
    return api.post<{ code: number; data: { done: string[]; project: AiWriteProject } }>(
      `/author-ai/projects/${id}/generate-all`,
      data,
    );
  },
  // 时间线 [LOOP] 一致性校验（纯 Go）
  timelineCheck(id: number) {
    return api.get<{ code: number; data: TimelineCheckResult }>(
      `/author-ai/projects/${id}/timeline-check`,
    );
  },
  // 章节 CRUD
  chapters(id: number) {
    return api.get<{ code: number; data: AiWriteChapter[] }>(`/author-ai/projects/${id}/chapters`);
  },
  createChapter(id: number, data: { title?: string; outline?: string; body?: string; order?: number }) {
    return api.post<{ code: number; data: AiWriteChapter }>(`/author-ai/projects/${id}/chapters`, data);
  },
  updateChapter(id: number, cid: number, data: Partial<AiWriteChapter>) {
    return api.put<{ code: number; data: AiWriteChapter }>(
      `/author-ai/projects/${id}/chapters/${cid}`,
      data,
    );
  },
  removeChapter(id: number, cid: number) {
    return api.delete(`/author-ai/projects/${id}/chapters/${cid}`);
  },
  // 逐章生成：part = outline | body
  generateChapter(id: number, cid: number, data: { part: string; model_id?: number; prompt?: string }) {
    return api.post<{ code: number; data: { part: string; content: string; chapter: AiWriteChapter } }>(
      `/author-ai/projects/${id}/chapters/${cid}/generate`,
      data,
    );
  },
  publish(id: number) {
    return api.post<{ code: number; data: any }>(`/author-ai/projects/${id}/publish`);
  },

  // ===== ADRO 创作向：引子 → 拆解成独立细节卡 → 逐细节多轮微调 =====
  details(id: number) {
    return api.get<{ code: number; data: AiWriteDetail[] }>(`/author-ai/projects/${id}/details`);
  },
  createDetail(id: number, data: { module: DetailModule; title: string; content: string }) {
    return api.post<{ code: number; data: AiWriteDetail }>(`/author-ai/projects/${id}/details`, data);
  },
  updateDetail(id: number, did: number, data: Partial<AiWriteDetail>) {
    return api.put<{ code: number; data: AiWriteDetail }>(`/author-ai/projects/${id}/details/${did}`, data);
  },
  removeDetail(id: number, did: number) {
    return api.delete(`/author-ai/projects/${id}/details/${did}`);
  },
  /** 一个话题/引子 → AI 自动拆解出四模块细节卡（Anchor → Deconstruct） */
  fromSeed(id: number, data: { seed: string; replace?: boolean }) {
    return api.post<{ code: number; data: { cards: AiWriteDetail[]; project: AiWriteProject } }>(
      `/author-ai/projects/${id}/from-seed`, data,
    );
  },
  /** 对单个细节做苏格拉底 / 费曼 / 直接改写，带多轮 history（Recur → Output） */
  refineDetail(id: number, did: number, data: {
    mode: RefineMode; instruction: string; history: RefineTurn[]; apply?: boolean;
  }) {
    return api.post<{ code: number; data: RefineResult }>(
      `/author-ai/projects/${id}/details/${did}/refine`, data,
    );
  },
  refineModes(id: number) {
    return api.get(`/author-ai/projects/${id}/refine-modes`);
  },
};
