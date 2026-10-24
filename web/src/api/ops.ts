import api from '@/api/index';

// ==================== 运营位：导航菜单 / 轮播 / 签到 / 访客设置（gap-audit #20 #23）====================

export interface NavMenuItem {
  id: number;
  parent_id: number;
  title: string;
  icon: string;
  url: string;
  target: 'self' | 'blank';
  sort_order: number;
  visible: boolean;
  require_login: boolean;
  admin_only: boolean;
  badge: string;
  children?: NavMenuItem[];
}

export interface Carousel {
  id: number;
  title: string;
  image_url: string;
  link_url: string;
  target: 'self' | 'blank';
  sort_order: number;
  enabled: boolean;
  start_at?: string | null;
  end_at?: string | null;
}

export interface CheckInConfig {
  enabled: boolean;
  base_reward: number;
  streak_bonus: number;
  max_streak: number;
  require_post: boolean;
}

export interface CheckInStatus {
  enabled: boolean;
  date: string;
  checked: boolean;
  streak: number;
  reward?: number;
  total_days?: number;
  config: CheckInConfig;
}

export interface VisitSetting {
  allow_guest_browse: boolean;
  allow_guest_search: boolean;
  allow_guest_read_all: boolean;
  guest_max_chapters: number;
  show_visit_count: boolean;
  block_guest_post: boolean;
  notice_text: string;
}

export const opsApi = {
  // ── 前台 ──
  navMenus() {
    return api.get<{ code: number; data: { items: NavMenuItem[] } }>('/nav-menus');
  },
  carousels() {
    return api.get<{ code: number; data: { items: Carousel[] } }>('/carousels');
  },
  checkInStatus() {
    return api.get<{ code: number; data: CheckInStatus }>('/checkin/status');
  },
  checkIn() {
    return api.post<{ code: number; data: { already: boolean; reward: number; streak: number; balance: number } }>(
      '/checkin',
    );
  },
  visitSetting() {
    return api.get<{ code: number; data: VisitSetting }>('/visit-setting');
  },

  // ── 管理端 ──
  adminNavMenus() {
    return api.get<{ code: number; data: { items: NavMenuItem[] } }>('/admin/nav-menus');
  },
  saveNavMenu(data: Partial<NavMenuItem>) {
    return api.post('/admin/nav-menus', data);
  },
  deleteNavMenu(id: number) {
    return api.delete(`/admin/nav-menus/${id}`);
  },
  adminCarousels() {
    return api.get<{ code: number; data: { items: Carousel[] } }>('/admin/carousels');
  },
  saveCarousel(data: Partial<Carousel>) {
    return api.post('/admin/carousels', data);
  },
  deleteCarousel(id: number) {
    return api.delete(`/admin/carousels/${id}`);
  },
  adminCheckIn() {
    return api.get<{ code: number; data: { config: CheckInConfig; stats: any[] } }>('/admin/checkin');
  },
  saveCheckIn(data: CheckInConfig) {
    return api.put('/admin/checkin', data);
  },
  adminVisit() {
    return api.get<{ code: number; data: { config: VisitSetting; stats: any[] } }>('/admin/visit-setting');
  },
  saveVisit(data: VisitSetting) {
    return api.put('/admin/visit-setting', data);
  },
};
