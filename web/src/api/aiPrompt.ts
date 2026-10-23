import api from './index';

// AI 提示词库（§5.4）：平台预设（只读）+ 我的自定义（可增删改）

export type AiPromptKind = 'chat' | 'image' | 'video';

export interface AiPrompt {
  id: number;
  user_id: number;
  kind: AiPromptKind;
  category: string;
  title: string;
  content: string;
  builtin: boolean;
  sort_order: number;
  used_count: number;
  created_at: string;
  updated_at: string;
}

export const aiPromptApi = {
  list(params: { kind?: string; q?: string } = {}) {
    return api.get<{ code: number; data: { list: AiPrompt[]; categories: string[] } }>('/ai/prompts', { params });
  },
  create(data: { kind: string; category?: string; title: string; content: string }) {
    return api.post<{ code: number; data: AiPrompt }>('/ai/prompts', data);
  },
  update(id: number, data: { kind: string; category?: string; title: string; content: string }) {
    return api.put(`/ai/prompts/${id}`, data);
  },
  remove(id: number) {
    return api.delete(`/ai/prompts/${id}`);
  },
  use(id: number) {
    return api.post(`/ai/prompts/${id}/use`);
  },
};
