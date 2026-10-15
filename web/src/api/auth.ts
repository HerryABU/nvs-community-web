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
  verify_code?: string;
  captcha_token?: string;
}

export interface CaptchaIssueResp {
  challenge_id: string;
  bg_image: string;
  piece_image: string;
  width: number;
  height: number;
  tip: string;
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

  // ======== 滑块验证码（货真价实服务端校验） ========
  // issue：向服务端请求一次拼图挑战（返回缺口背景图 + 拼图块 PNG 的 base64）
  captchaIssue() {
    return api.post<{ code: number; message: string; data: CaptchaIssueResp }>(
      '/auth/captcha/issue'
    );
  },
  // verify：上报滑块归位的 x 坐标，服务端校验通过则签发一次性 pass token
  captchaVerify(challengeId: string, x: number) {
    return api.post<{ code: number; message: string; data: { captcha_token: string } }>(
      '/auth/captcha/verify',
      { challenge_id: challengeId, x }
    );
  },

  // 发送邮箱验证码（必须携带滑块 pass token，否则后端拒绝）
  sendVerificationCode(email: string, captchaToken?: string) {
    return api.post<{ code: number; message: string }>('/auth/send-code', {
      email,
      captcha_token: captchaToken || '',
    });
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

  // 上传头像（FormData 不手动设置 Content-Type，避免丢失 boundary）
  uploadAvatar(formData: FormData) {
    return api.post<{ code: number; message: string; data: { avatar_url: string } }>(
      '/author/avatar',
      formData
    );
  },

  // 上传图片（重新编码防马）
  uploadImage(formData: FormData) {
    return api.post<{ code: number; message: string; data: { url: string } }>(
      '/author/image',
      formData
    );
  },

  // 更新作者资料（昵称/签名/隔离开关）
  updateProfile(data: { nickname?: string; signature?: string; wall_enabled?: boolean }) {
    return api.put<{ code: number; message: string }>('/author/profile', data);
  },

  // 修改密码（需登录）
  changePassword(oldPassword: string, newPassword: string) {
    return api.post<{ code: number; message: string }>('/auth/change-password', {
      old_password: oldPassword,
      new_password: newPassword,
    });
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