import api from './index';

export interface ShelfItem {
  id: number;
  novel_id: number;
  last_read_chapter: number;
  added_at: string;
  novel: {
    id: number;
    author_id: number;
    title: string;
    category: string;
    categories?: string[];
    tags: string[];
    summary: string;
    cover_url: string;
    status: string;
    total_words: number;
    total_chapters: number;
    created_at: string;
    updated_at: string;
    author?: {
      id: number;
      username: string;
      nickname: string;
      avatar_url: string;
    };
  };
}

export const bookshelfApi = {
  getList(page = 1, pageSize = 50) {
    return api.get<{ code: number; data: { list: ShelfItem[]; total: number } }>('/bookshelf', {
      params: { page, page_size: pageSize },
    });
  },

  add(novelId: number) {
    return api.post('/bookshelf', { novel_id: novelId });
  },

  remove(novelId: number) {
    return api.delete(`/bookshelf/${novelId}`);
  },

  check(novelId: number) {
    return api.get<{ code: number; data: { on_shelf: boolean } }>(`/bookshelf/check/${novelId}`);
  },

  updateProgress(novelId: number, lastReadChapter: number) {
    return api.post('/bookshelf/progress', {
      novel_id: novelId,
      last_read_chapter: lastReadChapter,
    });
  },
};
