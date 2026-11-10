import api from '@/api/index';

// ==================== 小说文档（设定集 / 大纲 / 人物 / 时间线 / 故事线 / 资料） ====================
// 与小说域绑定：作者维护，可控制对读者是否公开（visibility）。

export interface NovelDocument {
  id: number;
  novel_id: number;
  kind: string; // setting / outline / character / timeline / storyline / extra / material
  title: string;
  content: string;
  sort_order: number;
  visibility: string; // public / private
  status: string; // draft / published
  created_at: string;
  updated_at: string;
}

export const DOC_KIND_OPTIONS = [
  { value: 'setting', label: '设定集' },
  { value: 'outline', label: '大纲' },
  { value: 'character', label: '人物小传' },
  { value: 'timeline', label: '时间线 / 大事年表' },
  { value: 'storyline', label: '故事线' },
  { value: 'extra', label: '番外 / 其他' },
  { value: 'material', label: '资料' },
];

export function docKindLabel(kind: string): string {
  return DOC_KIND_OPTIONS.find((k) => k.value === kind)?.label || kind;
}

export const documentApi = {
  list(novelId: number) {
    return api.get<{ code: number; data: { list: NovelDocument[] } }>(`/novels/${novelId}/documents`);
  },
  create(novelId: number, data: {
    kind?: string;
    title: string;
    content?: string;
    visibility?: string;
    status?: string;
    sort_order?: number;
  }) {
    return api.post<{ code: number; data: NovelDocument }>(`/novels/${novelId}/documents`, data);
  },
  update(novelId: number, id: number, data: Partial<{
    kind: string;
    title: string;
    content: string;
    visibility: string;
    status: string;
    sort_order: number;
  }>) {
    return api.put<{ code: number; data: NovelDocument }>(`/novels/${novelId}/documents/${id}`, data);
  },
  remove(novelId: number, id: number) {
    return api.delete(`/novels/${novelId}/documents/${id}`);
  },
};
