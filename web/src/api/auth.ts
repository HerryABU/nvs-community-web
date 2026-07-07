import api from './index';

export interface LoginParams {
  email: string;
  password: string;
}

export interface RegisterParams {
  username: string;
  email: string;
  password: string;
  nickname?: string;
  agree_to_guidelines: boolean;
}

export interface User {
  id: number;
  username: string;
  email: string;
  nickname: string;
  avatar_url: string;
  bio: string;
  role: string;
  created_at: string;
}

export const authApi = {
  register(data: RegisterParams) {
    return api.post<{ code: number; message: string; data: { token: string; user: User } }>(
      '/auth/register',
      data
    );
  },

  login(data: LoginParams) {
    return api.post<{ code: number; message: string; data: { token: string; user: User } }>(
      '/auth/login',
      data
    );
  },

  getMe() {
    return api.get<{ code: number; message: string; data: User }>('/auth/me');
  },

  logout() {
    return api.post('/auth/logout');
  },
};
