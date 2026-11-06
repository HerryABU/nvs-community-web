import api from '@/api/index';

// #66 VIP 等级卡密：管理端发放、用户端兑换。
// 卡密等级上限由站长在「阅读权限」配置里设定（vip_max_level）。

export interface VipCard {
  id: number;
  code_prefix: string;
  level: number;
  days: number;
  status: 'unused' | 'used' | 'void' | string;
  batch_id: string;
  created_by: number;
  used_by?: number | null;
  used_at?: string | null;
  expires_at?: string | null;
  note: string;
  created_at: string;
  used_by_username?: string;
}

export interface GenerateVipCardsInput {
  count: number;
  level: number;
  days: number;
  expires_in?: number;
  note?: string;
}

export interface VipMembership {
  level: number;
  member_until: string | null;
  active: boolean;
}

export const vipCardApi = {
  adminList(params?: { status?: string; batch?: string }) {
    return api.get<{ code: number; data: VipCard[] }>('/admin/vip-cards', { params });
  },
  adminGenerate(data: GenerateVipCardsInput) {
    return api.post<{ code: number; data: { codes: string[]; count: number } }>('/admin/vip-cards', data);
  },
  adminVoid(id: number) {
    return api.post(`/admin/vip-cards/${id}/void`);
  },
  redeem(code: string) {
    return api.post<{ code: number; data: { level: number; until: string; days: number } }>('/vip-cards/redeem', { code });
  },
  membership() {
    return api.get<{ code: number; data: VipMembership }>('/vip-cards/membership');
  },
};
