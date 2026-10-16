<template>
  <div id="app-container">
    <NavBar />
    <!-- 未成年保护横幅（soft 模式 + 已识别为未成年人，可关闭） -->
    <div v-if="showMinorBanner" class="minor-banner">
      <span>🛡️ 青少年模式：合理安排上网时间，适度消费内容</span>
      <el-button text size="small" @click="showMinorBanner = false">知道了</el-button>
    </div>
    <main class="main-content">
      <router-view />
    </main>
    <SiteFooter />

    <!-- 未成年检测引导（F21.1）：平台开启保护且未填报出生日期时询问 -->
    <MinorGateDialog v-model:visible="minorGateVisible" :mode="minorMode" @saved="onMinorSaved" @dismiss="onMinorDismiss" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import NavBar from '@/components/NavBar.vue';
import SiteFooter from '@/components/SiteFooter.vue';
import MinorGateDialog from '@/components/auth/MinorGateDialog.vue';
import { useAuthStore } from '@/stores/auth';
import { useThemeStore } from '@/stores/theme';
import { useAppStore } from '@/stores/app';
import { authApi } from '@/api/auth';

const authStore = useAuthStore();
const themeStore = useThemeStore();
const appStore = useAppStore();

// 在 setup 阶段同步启动：首屏落地页时首页内容并入同一批 → 严格 1 次 /api/batch；
// 子组件（NavBar/Home）挂载后通过 ensure* 等待同一 promise，无竞态。
appStore.bootstrap();

onMounted(() => {
  authStore.fetchUser();
  themeStore.applyTheme();
  // F12 自定义网站主题（alist 式）：拉取站点默认主题 +（已登录时）个人主题并注入 <style>
  themeStore.loadTheme();
});

// ======== 未成年检测（F21.1） ========
// minor_mode：off=关闭 / soft=软模式（横幅提醒）/ strict=严格（必填 + 限制发布）
const minorMode = computed<'off' | 'soft' | 'strict'>(() => appStore.siteInfo?.minor_mode || 'off');
const minorGateVisible = ref(false);
const showMinorBanner = ref(false);
let minorChecked = false; // 每次页面加载只检查一次，避免反复弹窗

async function checkMinorGate() {
  if (minorChecked) return;
  if (minorMode.value === 'off' || !authStore.isLoggedIn) return;
  minorChecked = true;
  try {
    const resp = await authApi.getMinorStatus();
    if (resp.data.code !== 0) return;
    const s = resp.data.data;
    if (s.need_ask) {
      // soft 模式：本会话已选择"暂不填写"则不再打扰
      if (minorMode.value === 'soft' && sessionStorage.getItem('nvs-minor-dismissed') === '1') return;
      minorGateVisible.value = true;
    } else if (s.minor_protected && minorMode.value === 'soft') {
      showMinorBanner.value = true;
    }
  } catch {
    // 静默失败：不影响正常使用，下次加载再问
  }
}

function onMinorSaved(payload: { minor_protected: boolean }) {
  if (payload.minor_protected && minorMode.value === 'soft') {
    showMinorBanner.value = true;
  }
}

function onMinorDismiss() {
  sessionStorage.setItem('nvs-minor-dismissed', '1');
}

// 登录态就绪后检查（fetchUser 在 onMounted 异步返回，watch 兜住时序）
watch(() => authStore.isLoggedIn, (logged) => {
  if (logged) checkMinorGate();
}, { immediate: true });
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

.minor-banner {
  margin-top: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 6px 16px;
  font-size: 13px;
  color: #b7791f;
  background: linear-gradient(90deg, rgba(255, 213, 128, 0.25), rgba(255, 235, 180, 0.35));
  border-bottom: 1px solid rgba(255, 193, 7, 0.35);
}
</style>
