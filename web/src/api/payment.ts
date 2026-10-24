import api from './index';
import { adminApi } from './admin';

// 在线支付充值（需求 §7.4：微信/支付宝聚合通道）
// 注意：baseURL 已含前缀（apiPrefix+/api），严禁在此硬编码 /go-nvs。
export interface PaymentChannel {
  value: string; // wechat / alipay / mock
  label: string;
}
export interface PaymentTier {
  fen: number; // 面额（分）
  bonus: number; // 赠送代币
  label: string; // 展示名（如「月卡」）
  days?: number; // 会员天数（0/缺省 = 纯充值，不加时长）
  badge?: string; // 角标：推荐 / 最超值
  features?: string[]; // 权益说明（会员页逐条展示）
}
export interface MembershipStatus {
  is_member: boolean;
  until?: string;
  days_left: number;
}
export interface MembershipInfo {
  status: MembershipStatus;
  tiers: PaymentTier[];
  token_per_fen: number;
  payment_enabled: boolean;
  token_enabled: boolean;
}
export interface PaymentStatus {
  enabled: boolean;
  token_enabled: boolean;
  token_name: string;
  provider: string; // mock / aggregate
  channels: PaymentChannel[];
  tiers: PaymentTier[];
  token_per_fen: number;
  order_ttl_min: number;
}
export interface PaymentOrder {
  order_no: string;
  status: string; // pending / paid / failed / closed
  channel: string;
  provider: string;
  amount_fen: number;
  token_amount: number;
  expires_at: string;
  paid_at: string | null;
  pay_url?: string;
  member_days?: number;
}
export interface PaymentOrderItem extends PaymentOrder {
  id: number;
  transaction_id: string;
  created_at: string;
  username?: string;
}
export interface PaymentStats {
  paid_count: number;
  paid_fen: number;
  paid_token: number;
  pending_count: number;
  closed_count: number;
}
export interface PaymentAdminConfig {
  enabled: boolean;
  provider: string;
  gateway_url: string;
  app_id: string;
  secret_set: boolean;
  notify_url: string;
  return_url: string;
  token_per_fen: number;
  min_fen: number;
  max_fen: number;
  order_ttl_min: number;
  tiers: PaymentTier[];
  stats: PaymentStats;
}

export const PAY_STATUS_LABELS: Record<string, string> = {
  pending: '待支付',
  paid: '已到账',
  failed: '支付失败',
  closed: '已关单',
};

export const PAY_CHANNEL_LABELS: Record<string, string> = {
  wechat: '微信支付',
  alipay: '支付宝',
  mock: '模拟支付',
};

export const paymentApi = {
  // 用户侧
  status() {
    return api.get<{ code: number; data: PaymentStatus }>('/payment/status');
  },
  /** 会员页初始化：我的会员状态 + 在售套餐 + 支付可用性 */
  membership() {
    return api.get<{ code: number; data: MembershipInfo }>('/membership');
  },
  createOrder(data: { channel: string; fen: number }) {
    return api.post<{ code: number; data: PaymentOrder }>('/payment/order', data);
  },
  orderStatus(orderNo: string) {
    return api.get<{ code: number; data: PaymentOrder & { balance: number } }>(`/payment/order/${encodeURIComponent(orderNo)}`);
  },
  myOrders(params?: { page?: number; page_size?: number }) {
    return api.get<{ code: number; data: { list: PaymentOrderItem[]; total: number } }>('/payment/orders', { params });
  },
  mockConfirm(orderNo: string) {
    return api.post<{ code: number; data: { credited: boolean; status: string; token_amount: number; balance: number } }>('/payment/mock/confirm', { order_no: orderNo });
  },

  // 管理端
  getConfig() {
    return api.get<{ code: number; data: PaymentAdminConfig }>('/admin/payment/config');
  },
  setConfig(data: {
    enabled?: boolean;
    provider?: string;
    gateway_url?: string;
    app_id?: string;
    secret?: string | null; // null=保持，''=清空
    notify_url?: string;
    return_url?: string;
    token_per_fen?: number;
    min_fen?: number;
    max_fen?: number;
    order_ttl_min?: number;
  }) {
    return api.put('/admin/payment/config', data);
  },
  setTiers(tiers: PaymentTier[]) {
    return api.put('/admin/payment/tiers', { tiers });
  },
  listOrders(params?: { status?: string; user_id?: number; page?: number; page_size?: number }) {
    return api.get<{ code: number; data: { list: PaymentOrderItem[]; total: number } }>('/admin/payment/orders', { params });
  },
  markPaid(orderNo: string, transactionId?: string) {
    return api.post<{ code: number; data: { credited: boolean; token_amount: number; balance: number } }>('/admin/payment/orders/mark-paid', {
      order_no: orderNo,
      transaction_id: transactionId || '',
    });
  },
  resolveUser(key: string) {
    return adminApi.resolveUser(key);
  },
};
