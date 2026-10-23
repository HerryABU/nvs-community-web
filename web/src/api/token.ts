import api from './index';
import { adminApi } from './admin';

// 代币钱包 / 卡密（需求 §7）
// 注意：baseURL 已含前缀（apiPrefix+/api），严禁在此硬编码 /go-nvs。
export interface TokenStatus {
  enabled: boolean;
  name: string;
  balance: number;
}
export interface TokenLedgerItem {
  id: number;
  type: string;
  amount: number;
  balance_after: number;
  ref_type: string;
  ref_id: number;
  remark: string;
  created_at: string;
}

export const TOKEN_TYPE_LABELS: Record<string, string> = {
  recharge_card: '卡密充值',
  recharge_pay: '支付充值',
  consume: '消费',
  grant: '增发',
  deduct: '扣减',
  refund: '退款',
};

export const tokenApi = {
  // 用户侧
  status() {
    return api.get<{ code: number; data: TokenStatus }>('/token/status');
  },
  ledger(params?: { page?: number; page_size?: number }) {
    return api.get<{ code: number; data: { list: TokenLedgerItem[]; total: number } }>('/token/ledger', { params });
  },
  redeem(code: string) {
    return api.post('/token/redeem', { code });
  },

  // 管理端
  getConfig() {
    return api.get<{ code: number; data: { enabled: boolean; name: string } }>('/admin/token/config');
  },
  setConfig(data: { enabled?: boolean; name?: string }) {
    return api.put('/admin/token/config', data);
  },
  generateCards(data: { count: number; value: number; expires_in?: number; note?: string }) {
    return api.post<{ code: number; data: { codes: string[]; count: number } }>('/admin/token/cards', data);
  },
  listCards(params?: { status?: string; batch?: string }) {
    return api.get('/admin/token/cards', { params });
  },
  voidCard(id: number) {
    return api.post(`/admin/token/cards/${id}/void`);
  },
  adjust(data: { user_id: number; amount: number; remark?: string }) {
    return api.post('/admin/token/adjust', data);
  },
  resolveUser(key: string) {
    return adminApi.resolveUser(key);
  },
};
