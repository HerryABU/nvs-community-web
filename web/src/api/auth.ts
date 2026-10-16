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

  // 无感续期：携带 HttpOnly 刷新令牌 Cookie，向后端换取新的访问令牌 Cookie。
  // 成功返回 code=0；失败（刷新令牌缺失/过期）返回 401，由前端引导重新登录。
  refreshToken() {
    return api.post<{ code: number; message: string }>('/auth/refresh');
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

  // 忘记密码 - 发送重置验证码（必须携带滑块 pass token，否则后端拒绝）
  forgotPassword(email: string, captchaToken?: string) {
    return api.post<{ code: number; message: string }>('/auth/forgot-password', {
      email,
      captcha_token: captchaToken || '',
    });
  },

  // 重置密码
  resetPassword(email: string, code: string, newPassword: string) {
    return api.post<{ code: number; message: string }>('/auth/reset-password', {
      email,
      code,
      new_password: newPassword,
    });
  },

  // ======== 手机号登录（F10） ========
  // 发送手机验证码（必须携带滑块 pass token；login=登录/注册，reset=找回密码）
  phoneSendCode(phone: string, captchaToken?: string, purpose: 'login' | 'reset' = 'login') {
    return api.post<{ code: number; message: string; data: { backend: string } }>(
      '/auth/phone/send-code',
      { phone, captcha_token: captchaToken || '', purpose }
    );
  },
  // 手机号验证码登录（不存在则自动注册）
  phoneLogin(phone: string, code: string) {
    return api.post<{ code: number; message: string; data: { user: User } }>(
      '/auth/phone/login',
      { phone, code }
    );
  },

  // 手机号验证码重置密码（F22）
  phoneResetPassword(phone: string, code: string, newPassword: string) {
    return api.post<{ code: number; message: string }>('/auth/phone/reset-password', {
      phone,
      code,
      new_password: newPassword,
    });
  },

  // ======== 实名制（F10 / F21.1） ========
  // 提交实名（姓名加密存储，身份证仅哈希+脱敏）
  applyRealName(realName: string, idCard: string) {
    return api.post<{ code: number; message: string; data: { id_card_masked: string; minor_protected: boolean } }>(
      '/auth/realname/apply',
      { real_name: realName, id_card: idCard }
    );
  },
  // 我的实名状态
  getRealNameStatus() {
    return api.get<{
      code: number; message: string; data: {
        real_name_verified: boolean; id_card_masked: string;
        birth_date: string | null; minor_protected: boolean;
      };
    }>('/auth/realname');
  },
  // 国家实名检测 API 代理（姓名+身份证不落库明文，仅返回是否一致）
  // advanced=true 为高级模式：只验证年龄，服务器不存储姓名/身份证，
  // 仅按证件推算出生日期与未成年标记（F21.1 未成年检测）
  nationalRealNameCheck(realName: string, idCard: string, advanced = false) {
    return api.post<{
      code: number; message: string; data: {
        matched: boolean; status: string; message: string; minor_protected: boolean;
      };
    }>('/auth/realname/national-check', { real_name: realName, id_card: idCard, advanced });
  },

  // ======== 未成年检测（F21.1） ========
  // 自报出生日期（不强制实名；已实名用户以证件为准不可自报）
  setBirthDate(birthDate: string) {
    return api.post<{ code: number; message: string; data: { age: number; minor_protected: boolean } }>(
      '/auth/birthdate',
      { birth_date: birthDate }
    );
  },
  // 未成年模式与我的状态（need_ask=true 时前端应弹窗询问）
  getMinorStatus() {
    return api.get<{
      code: number; message: string; data: {
        minor_mode: 'off' | 'soft' | 'strict';
        birth_date_set: boolean;
        birth_date?: string;
        age?: number;
        minor_protected: boolean;
        need_ask: boolean;
      };
    }>('/auth/minor-status');
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