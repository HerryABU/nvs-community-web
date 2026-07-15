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
  signature: string;
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

  // 发送邮箱验证码
  sendVerificationCode(email: string) {
    return api.post<{ code: number; message: string }>('/auth/send-code', { email });
  },

  // 验证邮箱验证码
  verifyEmailCode(email: string, code: string) {
    return api.post<{ code: number; message: string }>('/auth/verify-code', { email, code });
  },

  // 忘记密码 - 发送重置验证码
  forgotPassword(email: string) {
    return api.post<{ code: number; message: string }>('/auth/forgot-password', { email });
  },

  // 重置密码
  resetPassword(email: string, code: string, newPassword: string) {
    return api.post<{ code: number; message: string }>('/auth/reset-password', {
      email,
      code,
      new_password: newPassword,
    });
  },

  // 上传头像
  uploadAvatar(formData: FormData) {
    return api.post<{ code: number; message: string; data: { avatar_url: string } }>(
      '/author/avatar',
      formData,
      { headers: { 'Content-Type': 'multipart/form-data' } }
    );
  },

  // 上传图片（重新编码防马）
  uploadImage(formData: FormData) {
    return api.post<{ code: number; message: string; data: { url: string } }>(
      '/author/image',
      formData,
      { headers: { 'Content-Type': 'multipart/form-data' } }
    );
  },

  // 更新作者资料
  updateProfile(data: { signature?: string; wall_enabled?: boolean }) {
    return api.put<{ code: number; message: string }>('/author/profile', data);
  },

  // 获取签名状态
  getSignatureStatus() {
    return api.get<{
      code: number; message: string; data: {
        has_signing_key: boolean; total_chapters: number;
        signed_chapters: number; unsigned_chapters: number; coverage_percent: number;
      };
    }>('/author/signature-status');
  },
};