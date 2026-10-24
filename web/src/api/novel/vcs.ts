import api from '@/api/index';

// ==================== 章节版本控制（git 语义） ====================
// 规则见 docs/novel-version-control.md：保存带 base_version，落后即 409 冲突（回传三方内容）。

export interface ChapterVersion {
  id: number;
  chapter_id: number;
  novel_id: number;
  chapter_num: number;
  branch: string;
  version: number;
  parent_ver: number;
  merge_from?: string;
  merge_from_ver?: number;
  title: string;
  content_hash: string;
  word_count: number;
  message: string;
  author_id: number;
  author_name?: string;
  force: boolean;
  deleted: boolean;
  is_head?: boolean;
  created_at: string;
}

export interface ChapterBranchVO {
  id: number;
  chapter_id: number;
  name: string;
  head_ver: number;
  from_ver: number;
  from_branch: string;
  author_id: number;
  created_at: string;
}

export type DiffType = 'equal' | 'add' | 'del';

export interface DiffLine {
  type: DiffType;
  left_no: number;
  right_no: number;
  text: string;
}

export interface DiffResult {
  lines: DiffLine[];
  added: number;
  removed: number;
  truncated: boolean;
}

/** 保存冲突现场（409）：base = 我基于的版本，server = 服务端最新版，mine = 我这次的内容 */
export interface ConflictPayload {
  base_version: number;
  server_version: number;
  server_title: string;
  server_content: string;
  base_content: string;
  mine_content: string;
}

export interface MergeConflict {
  base: string[];
  ours: string[];
  theirs: string[];
}

export interface MergeResult {
  conflict: boolean;
  merged?: string;
  conflicts?: MergeConflict[];
  version?: number;
  from_ver: number;
  into_ver: number;
  from_branch: string;
  into_branch: string;
}

export const vcsApi = {
  listVersions(novelId: number, chapterNum: number, branch?: string) {
    return api.get<{
      code: number;
      data: { list: ChapterVersion[]; branch: string; branches: ChapterBranchVO[]; head: number; novel_id: number };
    }>(`/novels/${novelId}/chapters/${chapterNum}/versions`, { params: { branch: branch || '' } });
  },

  getVersion(novelId: number, chapterNum: number, ver: number, branch?: string) {
    return api.get<{ code: number; data: { version: ChapterVersion; content: string } }>(
      `/novels/${novelId}/chapters/${chapterNum}/versions/${ver}`,
      { params: { branch: branch || '' } },
    );
  },

  /** 版本对比：from → to（to 缺省为分支 head） */
  diff(novelId: number, chapterNum: number, from: number, to: number, branch?: string) {
    return api.get<{
      code: number;
      data: { from: { version: number; author_name: string; message: string }; to: { version: number; author_name: string; message: string }; diff: DiffResult };
    }>(`/novels/${novelId}/chapters/${chapterNum}/versions/${from}/diff`, {
      params: { to, branch: branch || '' },
    });
  },

  revert(novelId: number, chapterNum: number, ver: number, branch?: string) {
    return api.post(`/novels/${novelId}/chapters/${chapterNum}/versions/${ver}/revert`, null, {
      params: { branch: branch || '' },
    });
  },

  deleteVersion(novelId: number, chapterNum: number, ver: number, branch?: string) {
    return api.delete(`/novels/${novelId}/chapters/${chapterNum}/versions/${ver}`, {
      params: { branch: branch || '' },
    });
  },

  listBranches(novelId: number, chapterNum: number) {
    return api.get<{ code: number; data: { list: ChapterBranchVO[] } }>(
      `/novels/${novelId}/chapters/${chapterNum}/branches`,
    );
  },

  createBranch(novelId: number, chapterNum: number, data: { name: string; from_branch?: string; from_ver?: number }) {
    return api.post(`/novels/${novelId}/chapters/${chapterNum}/branches`, data);
  },

  merge(novelId: number, chapterNum: number, data: { from_branch: string; into_branch?: string; message?: string }) {
    return api.post(`/novels/${novelId}/chapters/${chapterNum}/merge`, data);
  },
};

/** 冲突响应判定：后端 utils.Conflict 返回 code=6 + HTTP 409 */
export function isConflictResponse(e: any): ConflictPayload | null {
  const status = e?.response?.status;
  const code = e?.response?.data?.code;
  if (status === 409 || code === 6) {
    return (e?.response?.data?.data as ConflictPayload) || null;
  }
  return null;
}
