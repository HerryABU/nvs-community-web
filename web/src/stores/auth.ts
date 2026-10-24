import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { authApi, type User } from '@/api/auth';
import { appPrefix } from '@/utils/prefix';

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null);
  const loading = ref(false);
  /** 是否已尝试过恢复登录态（成功或失败都算，避免每次导航都请求 /api/auth/me） */
  const ready = ref(false);

  /**
   * 同一个请求只发一次：路由守卫与页面初始化会**同时**调用 fetchUser，
   * 早期实现各自发请求、各自改 loading，守卫还会因为 loading=true 而跳过等待，
   * 结果「已登录但直接打开 /admin/users 被踢到登录页」。
   */
  let inflight: Promise<void> | null = null;

  const isLoggedIn = computed(() => !!user.value);
  const isAuthor = computed(() => user.value?.role === 'author' || user.value?.role === 'vip_author');

  /** 恢复登录态（幂等）：已恢复过直接返回；正在请求则复用同一个 promise，调用方可以安全 await */
  async function fetchUser(force = false) {
    if (!force && ready.value) return;
    if (inflight) return inflight;

    inflight = (async () => {
      loading.value = true;
      try {
        const res = await authApi.getMe();
        user.value = res.data.code === 0 ? res.data.data : null;
      } catch {
        user.value = null;
      } finally {
        loading.value = false;
        ready.value = true;
        inflight = null;
      }
    })();

    return inflight;
  }

  async function login(email: string, password: string, captchaToken?: string) {
    const res = await authApi.login({ email, password, captcha_token: captchaToken });
    if (res.data.code === 0) {
      user.value = res.data.data.user;
      ready.value = true;
    }
    return res.data;
  }

  // 手机号验证码登录（F10；不存在则后端自动注册）
  async function phoneLogin(phone: string, code: string, captchaToken?: string) {
    const res = await authApi.phoneLogin(phone, code, captchaToken);
    if (res.data.code === 0) {
      user.value = res.data.data.user;
      ready.value = true;
    }
    return res.data;
  }

  async function register(username: string, email: string, password: string, nickname?: string, agreeToGuidelines: boolean = true, verifyCode?: string, captchaToken?: string) {
    const res = await authApi.register({ username, email, password, nickname, agree_to_guidelines: agreeToGuidelines, verify_code: verifyCode, captcha_token: captchaToken });
    if (res.data.code === 0) {
      user.value = res.data.data.user;
      ready.value = true;
    }
    return res.data;
  }

  async function logout() {
    try {
      // 调用后端清除 HttpOnly Cookie
      await authApi.logout();
    } catch {
      // 即使后端调用失败也清除本地状态
    }
    user.value = null;
    ready.value = true;
    // 🔀 前缀挂载时跳回 /go-nvs/（根路径 /{prefix}/），无前缀时跳回 /
    window.location.href = appPrefix + '/';
  }

  return {
    user,
    loading,
    ready,
    isLoggedIn,
    isAuthor,
    fetchUser,
    login,
    phoneLogin,
    register,
    logout,
  };
});
