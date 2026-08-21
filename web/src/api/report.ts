import api from './index';

export interface ReportReason {
  code: string;
  label: string;
  desc: string;
}

export interface ReportItem {
  id: number;
  reporter_id: number;
  target_type: string; // novel / comment / thread / post / blog / forum / chapter / user / userframe / userhtml
  target_id: number;
  reason_code: string;
  reason: string;
  detail: string;
  status: string; // pending / accepted / rejected
  verdict: string;
  target_preview?: string;
  created_at: string;
  resolved_at?: string;
}

export const reportApi = {
  // 预置举报理由
  reasons() {
    return api.get<{ code: number; data: ReportReason[] }>('/reports/reasons');
  },
  // 提交举报
  create(data: { target_type: string; target_id: number; reason_code?: string; reason: string; detail?: string }) {
    return api.post('/reports', data);
  },
  // 我的举报（状态追踪）
  mine() {
    return api.get<{ code: number; data: ReportItem[] }>('/reports/mine');
  },
};
