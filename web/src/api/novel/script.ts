import api from '@/api/index';

// ==================== 话本气泡体（script mode） ====================
// 源文约定：一行一条「角色：台词」，【】/（）整行为场景提示，其余为旁白。
// 渲染在服务端完成（server/scriptmode），前端只负责展示，避免两套解析逻辑漂移。

export interface ScriptPreview {
  html: string;
  speakers: string[];
  lines: number;
  speaker_count: number;
  emote_keys: string[];
}

/** 章节叙事形态：''=继承作品 / prose=散文 / script=话本气泡 / mixed=散文与话本混排 */
export type ChapterMode = '' | 'prose' | 'script' | 'mixed';

export const CHAPTER_MODE_OPTIONS: { value: ChapterMode; label: string }[] = [
  { value: '', label: '继承作品' },
  { value: 'prose', label: '散文体' },
  { value: 'script', label: '话本气泡体' },
  { value: 'mixed', label: '混排（散文+气泡）' },
];

export function isBubbleMode(mode: string): boolean {
  return mode === 'script' || mode === 'mixed';
}

export const scriptApi = {
  /** 预览话本气泡（传 novel_id 时服务端校验创作成员身份；mixed 走混排解析） */
  preview(content: string, novelId?: number, mode?: string) {
    return api.post<{ code: number; data: ScriptPreview }>('/script-preview', {
      content,
      novel_id: novelId || 0,
      mode: mode || 'script',
    });
  },
};
