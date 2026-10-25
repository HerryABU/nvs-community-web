import api from '@/api/index';

// ==================== 作品角色（话本气泡体角色管理） ====================
// 角色跨章节复用：头像 / 主题色 / 默认左右 / 简介。
// 权限：读=主笔/平台/协作者；写=仅主笔（作品级设置）。

export interface NovelCharacter {
  id: number;
  novel_id: number;
  name: string;
  /** 头像图片地址（可空，为空时前端用姓名首字 + color 生成文字头像） */
  avatar: string;
  /** 主题色（#rrggbb 或 CSS 颜色名） */
  color: string;
  /** 默认左右：left / right */
  side: string;
  intro: string;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export const characterApi = {
  list(novelId: number) {
    return api.get<{ code: number; data: { list: NovelCharacter[] } }>(`/novels/${novelId}/characters`);
  },
  create(novelId: number, data: {
    name: string;
    avatar?: string;
    color?: string;
    side?: string;
    intro?: string;
    sort_order?: number;
  }) {
    return api.post<{ code: number; data: NovelCharacter }>(`/novels/${novelId}/characters`, data);
  },
  update(novelId: number, id: number, data: {
    name?: string;
    avatar?: string;
    color?: string;
    side?: string;
    intro?: string;
    sort_order?: number;
  }) {
    return api.put(`/novels/${novelId}/characters/${id}`, data);
  },
  remove(novelId: number, id: number) {
    return api.delete(`/novels/${novelId}/characters/${id}`);
  },
};
