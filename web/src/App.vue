<template>
  <div id="app-container">
    <NavBar />
    <main class="main-content">
      <router-view />
    </main>
    <SiteFooter />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import NavBar from '@/components/NavBar.vue';
import SiteFooter from '@/components/SiteFooter.vue';
import { useAuthStore } from '@/stores/auth';
import { useThemeStore } from '@/stores/theme';
import { useAppStore } from '@/stores/app';

const authStore = useAuthStore();
const themeStore = useThemeStore();
const appStore = useAppStore();

// 在 setup 阶段同步启动：首屏落地页时首页内容并入同一批 → 严格 1 次 /api/batch；
// 子组件（NavBar/Home）挂载后通过 ensure* 等待同一 promise，无竞态。
appStore.bootstrap();

onMounted(() => {
  authStore.fetchUser();
  themeStore.applyTheme();
});
</script>

<style scoped>
#app-container {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background-color: var(--bg-color);
}

.main-content {
  flex: 1;
  margin-top: 60px;
  min-height: calc(100vh - 60px);
}
</style>
