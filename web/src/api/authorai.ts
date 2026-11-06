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
  status: string;
  created_at: string;
  updated_at: string;
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
  publish(id: number) {
    return api.post<{ code: number; data: any }>(`/author-ai/projects/${id}/publish`);
  },
};
