import api from '@/api/index';

// ==================== 话本气泡体（script mode） ====================
// 源文约定：一行一条「角色：台词」，【】/（）整行为场景提示，其余为旁白。
// 渲染在服务端完成（server/scriptmode），前端只负责展示，避免两套解析逻辑漂移。

export interface ScriptPreview {
  html: string;
  speakers: string[];
  lines: number;
  speaker_count: number;
}

export const scriptApi = {
  /** 预览话本气泡（传 novel_id 时服务端校验创作成员身份） */
  preview(content: string, novelId?: number) {
    return api.post<{ code: number; data: ScriptPreview }>('/script-preview', {
      content,
      novel_id: novelId || 0,
    });
  },
};
