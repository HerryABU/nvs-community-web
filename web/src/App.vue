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

onMounted(() => {
  authStore.fetchUser();
  themeStore.applyTheme();
  appStore.bootstrap(); // 一次 /api/batch 拉取导航栏/页脚等外壳数据
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
