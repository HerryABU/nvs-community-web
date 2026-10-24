import api from '@/api/index';

/** 模板变量声明 */
export interface TemplateVar {
  name: string;
  label: string;
  required: boolean;
  default: string;
  max_len: number;
}

/** 场景化提示词模板 */
export interface PromptTemplate {
  id: number;
  key: string;
  title: string;
  scene: 'novel_chapter' | 'blog' | 'chat' | string;
  category: string;
  kind: 'chat' | 'image' | 'video' | string;
  content: string;
  variables: TemplateVar[];
  version: number;
  enabled: boolean;
  priority: number;
  builtin: boolean;
  note: string;
  created_at: string;
  updated_at: string;
}

export interface RenderResult {
  text: string;
  missing: string[];
  unused?: string[];
  used?: string[];
}

export const templateApi = {
  /** 用户侧：按场景取启用模板 */
  list(params: { scene?: string; kind?: string } = {}) {
    return api.get<{ code: number; data: { list: PromptTemplate[]; categories: string[] } }>('/ai/templates', { params });
  },
  /** 用户侧：变量渲染 */
  render(id: number, vars: Record<string, string>) {
    return api.post<{ code: number; data: RenderResult & { title: string; version: number } }>(
      `/ai/templates/${id}/render`,
      { vars },
    );
  },

  /**
   * 带作品上下文的渲染：服务端自动注入 作品名/类型/大纲/角色设定/前文摘要/前文/作者样本。
   * vars 里显式传的「原文/正文」优先，不被旧章节覆盖。
   */
  renderContext(id: number, data: { novel_id: number; chapter_id?: number; vars?: Record<string, string> }) {
    return api.post<{
      code: number;
      data: RenderResult & {
        title: string;
        version: number;
        auto_filled: string[];
        sources: Record<string, string>;
        novel_title: string;
        chapter_title: string;
      };
    }>(`/ai/templates/${id}/render-context`, data);
  },

  // ---------- 管理端 ----------
  adminList(params: { scene?: string; kind?: string } = {}) {
    return api.get<{ code: number; data: { list: PromptTemplate[] } }>('/admin/ai/templates', { params });
  },
  adminCreate(data: Partial<PromptTemplate>) {
    return api.post<{ code: number; data: PromptTemplate }>('/admin/ai/templates', data);
  },
  adminUpdate(id: number, data: Partial<PromptTemplate>) {
    return api.put<{ code: number; data: PromptTemplate; message?: string }>(`/admin/ai/templates/${id}`, data);
  },
  adminDelete(id: number) {
    return api.delete(`/admin/ai/templates/${id}`);
  },
  adminVersions(id: number) {
    return api.get<{ code: number; data: { list: Array<{ id: number; version: number; content: string; note: string; created_at: string }> } }>(
      `/admin/ai/templates/${id}/versions`,
    );
  },
  adminRestore(id: number, version: number) {
    return api.post<{ code: number; data: PromptTemplate }>(`/admin/ai/templates/${id}/restore`, { version });
  },
  /** 未保存内容也能预览渲染 */
  adminPreview(data: { content: string; variables: TemplateVar[]; vars: Record<string, string> }) {
    return api.post<{ code: number; data: RenderResult }>('/admin/ai/template-preview', data);
  },
};
