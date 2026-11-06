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
  create(data: { title: string; style_preset?: StylePreset; style_detail?: string }) {
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
};
