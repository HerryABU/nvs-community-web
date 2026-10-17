import api from './index';

// 自定义主页（alist 式单框代码 + 双作用域 + 站长审核）。
//
// 安全模型（与后端 handlers/home_customize.go 对齐）：
//   - 我的主页装扮（/me）：仅本人登录态可见，允许原始 HTML+<script>（自 XSS，等同 alist 个人自定义）。
//   - 作者公开主页（/author/:id）：他人可见，提交即消毒（去脚本/危险 CSS），进入站长/管理员审核队列；
//     自动化预检发现被移除项（had_issues）时原文送站长复核。审核通过则 public_html = safe_html 对外可见。

// 我的主页装扮 + 作者公开主页提交/审核状态
export interface MyHomeCustomize {
  my_raw_html: string;
  author: {
    raw_html: string;
    safe_html: string;
    public_html: string;
    review_status: string; // pending / approved / rejected
    had_issues: boolean;
    review_note: string;
  };
}

// 作者公开主页提交结果（消毒后片段 + 被移除项 + 预检是否发现问题）
export interface AuthorSubmitResult {
  safe_html: string;
  removed: string[];
  had_issues: boolean;
  review_status: string;
}

// 审核队列条目（后端补了 username 便于站长识别）
export interface HomeCustomizeReviewItem {
  author_id: number;
  raw_html: string;
  safe_html: string;
  public_html: string;
  review_status: string;
  had_issues: boolean;
  review_note: string;
  reviewed_by: number;
  reviewed_at: string | null;
  updated_at: string;
  username: string;
}

export const homeCustomizeApi = {
  // 我的主页装扮（需登录）：返回本人原始 HTML + 作者公开主页提交/审核状态
  getMyHomeCustomize() {
    return api.get<{ code: number; data: MyHomeCustomize }>('/me/home-customize');
  },
  // 保存我的主页装扮原始 HTML（不消毒，仅本人可见）
  updateMyHomeCustomize(rawHtml: string) {
    return api.put<{ code: number; data: { raw_html: string } }>('/me/home-customize', { raw_html: rawHtml });
  },
  // 作者公开主页（公开）：返回已通过审核的对外可见片段；无/未通过返回空串
  getAuthorHomeCustomize(authorId: number | string) {
    return api.get<{ code: number; data: { public_html: string } }>(`/author/${authorId}/home-customize`);
  },
  // 提交作者公开主页（作者本人或管理员）：服务端消毒 + 置 pending；预检出问题通知站长
  updateAuthorHomeCustomize(authorId: number | string, rawHtml: string) {
    return api.put<{ code: number; data: AuthorSubmitResult }>(`/author/${authorId}/home-customize`, { raw_html: rawHtml });
  },
  // 审核队列（仅管理员）：列出全部作者公开主页提交（可按 review_status 过滤）；含原文 raw_html
  listReviews(status?: string) {
    return api.get<{ code: number; data: HomeCustomizeReviewItem[] }>('/admin/home-customize/reviews', {
      params: status ? { status } : undefined,
    });
  },
  // 审核（仅管理员）：{ action: 'approve' | 'reject', note? }
  review(authorId: number | string, action: 'approve' | 'reject', note?: string) {
    return api.post<{ code: number; data: { message: string; review_status: string } }>(
      `/admin/home-customize/reviews/${authorId}/review`,
      { action, note: note || '' },
    );
  },
};
