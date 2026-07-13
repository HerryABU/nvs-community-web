import api from '@/api/index';
import type { Chapter, ChapterDetail, ChapterVerify } from './types';

export const chapterApi = {
  getChapters(novelId: number) {
    return api.get<{ code: number; data: Chapter[] }>(`/novels/${novelId}/chapters`);
  },

  getChapter(novelId: number, chapterNum: number) {
    return api.get<{ code: number; data: ChapterDetail }>(
      `/novels/${novelId}/chapters/${chapterNum}`
    );
  },

  verifyChapter(novelId: number, chapterNum: number) {
    return api.get<{ code: number; data: ChapterVerify }>(
      `/novels/${novelId}/chapters/${chapterNum}/verify`
    );
  },

  createChapter(novelId: number, data: { title: string; content: string }) {
    return api.post(`/novels/${novelId}/chapters`, data);
  },

  updateChapter(novelId: number, chapterNum: number, data: Partial<Chapter>) {
    return api.put(`/novels/${novelId}/chapters/${chapterNum}`, data);
  },

  deleteChapter(novelId: number, chapterNum: number) {
    return api.delete(`/novels/${novelId}/chapters/${chapterNum}`);
  },
};
