import { defineStore } from 'pinia';
import { ref } from 'vue';

export const useThemeStore = defineStore('theme', () => {
  const isDark = ref(localStorage.getItem('nvs-theme') === 'dark');

  const applyTheme = () => {
    document.documentElement.setAttribute('data-theme', isDark.value ? 'dark' : 'light');
  };

  const toggle = () => {
    isDark.value = !isDark.value;
    localStorage.setItem('nvs-theme', isDark.value ? 'dark' : 'light');
    applyTheme();
  };

  // 初始化时立即应用主题
  applyTheme();

  return { isDark, toggle, applyTheme };
});
