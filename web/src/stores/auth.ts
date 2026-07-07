import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { authApi, type User } from '@/api/auth';

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

  async function login(email: string, password: string) {
    const res = await authApi.login({ email, password });
    if (res.data.code === 0) {
      user.value = res.data.data.user;
    }
    return res.data;
  }

  async function register(username: string, email: string, password: string, nickname?: string, agreeToGuidelines: boolean = true) {
    const res = await authApi.register({ username, email, password, nickname, agree_to_guidelines: agreeToGuidelines });
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
    window.location.href = '/';
  }

  return {
    user,
    loading,
    isLoggedIn,
    isAuthor,
    fetchUser,
    login,
    register,
    logout,
  };
});
