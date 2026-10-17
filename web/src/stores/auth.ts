import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { authApi, type User } from '@/api/auth';
import { appPrefix } from '@/utils/prefix';

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null);
  const loading = ref(false);

  const isLoggedIn = computed(() => !!user.value);
  const isAuthor = computed(() => user.value?.role === 'author' || user.value?.role === 'vip_author');

  // 页面初始化时恢复登录状态
  async function fetchUser() {
    try {
      loading.value = true;
      const res = await authApi.getMe();
      if (res.data.code === 0) {
        user.value = res.data.data;
      }
    } catch {
      user.value = null;
    } finally {
      loading.value = false;
    }
  }

  async function login(email: string, password: string, captchaToken?: string) {
    const res = await authApi.login({ email, password, captcha_token: captchaToken });
    if (res.data.code === 0) {
      user.value = res.data.data.user;
    }
    return res.data;
  }

  // 手机号验证码登录（F10；不存在则后端自动注册）
  async function phoneLogin(phone: string, code: string, captchaToken?: string) {
    const res = await authApi.phoneLogin(phone, code, captchaToken);
    if (res.data.code === 0) {
      user.value = res.data.data.user;
    }
    return res.data;
  }

  async function register(username: string, email: string, password: string, nickname?: string, agreeToGuidelines: boolean = true, verifyCode?: string, captchaToken?: string) {
    const res = await authApi.register({ username, email, password, nickname, agree_to_guidelines: agreeToGuidelines, verify_code: verifyCode, captcha_token: captchaToken });
    if (res.data.code === 0) {
      user.value = res.data.data.user;
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
    // 🔀 前缀挂载时跳回 /go-nvs/（根路径 /{prefix}/），无前缀时跳回 /
    window.location.href = appPrefix + '/';
  }

  return {
    user,
    loading,
    isLoggedIn,
    isAuthor,
    fetchUser,
    login,
    phoneLogin,
    register,
    logout,
  };
});
