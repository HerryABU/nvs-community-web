import api from './index';

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
};
