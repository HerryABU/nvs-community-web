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

/** 章节/作品叙事形态：''=继承作品 / prose=散文 / script=话本气泡 / mixed=散文与话本混排 / comic=漫画体 / image=图片模式 */
export type ChapterMode = '' | 'prose' | 'script' | 'mixed' | 'comic' | 'image';

export const CHAPTER_MODE_OPTIONS: { value: ChapterMode; label: string }[] = [
  { value: '', label: '继承作品' },
  { value: 'prose', label: '散文体' },
  { value: 'script', label: '话本气泡体' },
  { value: 'mixed', label: '混排（散文+气泡）' },
  { value: 'comic', label: '漫画体' },
  { value: 'image', label: '图片模式' },
];

/** 作品级叙事形态选项（不含「继承作品」） */
export const NOVEL_MODE_OPTIONS: { value: ChapterMode; label: string }[] = [
  { value: 'prose', label: '散文体' },
  { value: 'script', label: '话本气泡体' },
  { value: 'comic', label: '漫画体' },
  { value: 'image', label: '图片模式' },
];

export function isBubbleMode(mode: string): boolean {
  return mode === 'script' || mode === 'mixed';
}

/** 是否为图片类形态（漫画体 / 图片模式）：正文不再以文字为主，改为有序图片集 */
export function isImageMode(mode: string): boolean {
  return mode === 'comic' || mode === 'image';
}

/** 章节图片项 */
export interface ChapterImage {
  url: string;
  caption?: string;
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
