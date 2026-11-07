import api from '@/api/index';

/** 舆情守卫配置 */
export interface SentimentConfig {
  enabled: boolean;
  block_risk: 'low' | 'mid' | 'midhigh' | 'high';
  flag_risk: 'low' | 'mid' | 'midhigh' | 'high';
  flood_block: boolean;
  min_confidence: number;

  learn_from_ai: boolean;
  search_enabled: boolean;
  search_model_id: number;
  search_path: string;
  search_query: string;
  search_budget_per_day: number;
  recency_days: number;
  auto_update_hour: number;
  daily_quota: number;
  require_review: boolean;

  fold_chars: number;
  updated_at?: string;
}

/** 词条 */
export interface SentimentTerm {
  id: number;
  word: string;
  cat: string;
  escalate_under: string;
  severity: number;
  action_override: string;
  source: string;
  status: string;
  confidence: number;
  review_cycle_days: number;
  hit_count: number;
  note: string;
  reviewed_at?: string;
  created_at?: string;
}

/** 命中留证（不含评论正文） */
export interface SentimentHit {
  id: number;
  content_type: string;
  content_id: number;
  comment_id: number;
  user_id: number;
  username: string;
  decision: 'allow' | 'flag' | 'block';
  risk: string;
  groups: string;
  matched_words: string;
  variant_words: string;
  flood: string;
  reason: string;
  created_at: string;
}

export interface SentimentAudit {
  id: number;
  term_id: number;
  word: string;
  action: string;
  operator: string;
  note: string;
  before: string;
  after: string;
  created_at: string;
}

export const sentimentApi = {
  getConfig() {
    return api.get<{ code: number; data: { config: SentimentConfig } }>('/admin/sentiment/config');
  },
  saveConfig(data: Partial<SentimentConfig>) {
    return api.put('/admin/sentiment/config', data);
  },
  terms(params: { status?: string; keyword?: string; page?: number; page_size?: number } = {}) {
    return api.get<{ code: number; data: { items: SentimentTerm[]; total: number; page: number } }>(
      '/admin/sentiment/terms',
      { params },
    );
  },
  createTerm(data: Partial<SentimentTerm> & { word: string }) {
    return api.post('/admin/sentiment/terms', data);
  },
  updateTerm(id: number, data: Partial<SentimentTerm>) {
    return api.put(`/admin/sentiment/terms/${id}`, data);
  },
  deleteTerm(id: number) {
    return api.delete(`/admin/sentiment/terms/${id}`);
  },
  reviewTerm(id: number, action: 'approve' | 'reject', note = '') {
    return api.post(`/admin/sentiment/terms/${id}/review`, { action, note });
  },
  audits(termId?: number) {
    return api.get<{ code: number; data: { items: SentimentAudit[] } }>('/admin/sentiment/audits', {
      params: termId ? { term_id: termId } : {},
    });
  },
  hits(params: { decision?: string; risk?: string; page?: number; page_size?: number } = {}) {
    return api.get<{ code: number; data: { items: SentimentHit[]; total: number; page: number } }>(
      '/admin/sentiment/hits',
      { params },
    );
  },
  runUpdate() {
    return api.post<{ code: number; data: { result: any } }>('/admin/sentiment/run-update');
  },

  // ===== 阵营中立标记 / 语言格式 / 图谱 / 删除申请 =====
  neutralityPrompt() {
    return api.get<{ code: number; data: NeutralityPromptData }>('/admin/sentiment/neutral-prompt');
  },
  campGraph() {
    return api.get<{ code: number; data: CampGraphData }>('/admin/sentiment/camp-graph');
  },
  campMarks() {
    return api.get<{ code: number; data: CampMark[] }>('/admin/sentiment/camp-marks');
  },
  createCampMark(data: Partial<CampMark>) {
    return api.post<{ code: number; data: CampMark }>('/admin/sentiment/camp-marks', data);
  },
  deleteRequests() {
    return api.get<{ code: number; data: { list: DeleteRequest[]; can_approve: boolean } }>(
      '/admin/sentiment/delete-requests',
    );
  },
  approveDeleteRequest(id: number) {
    return api.post(`/admin/sentiment/delete-requests/${id}/approve`, {});
  },
  rejectDeleteRequest(id: number, reason = '') {
    return api.post(`/admin/sentiment/delete-requests/${id}/reject`, { reason });
  },
};

export interface NeutralityPromptData {
  prompt: string;
  notice: string;
  lang_formats: { value: string; label: string }[];
}

export interface CampNode {
  id: string;
  label: string;
  size: number;
  lang_format: string;
  lang_label: string;
  lang_mix: Record<string, number>;
  confidence: number;
}

export interface CampEdge {
  source: string;
  target: string;
  weight: number;
}

export interface CampGraphData {
  nodes: CampNode[];
  edges: CampEdge[];
  notice: string;
}

export interface CampMark {
  id: number;
  camp_id: string;
  camp_label: string;
  position: string;
  lang_format: string;
  lang_evidence: string;
  confidence: number;
  note: string;
  content_type: string;
  content_id: number;
  comment_id: number;
  created_at: string;
}

export interface DeleteRequest {
  id: number;
  target_type: string;
  target_id: number;
  target_label: string;
  requester: string;
  reason: string;
  status: 'pending' | 'approved' | 'rejected';
  created_at: string;
}
