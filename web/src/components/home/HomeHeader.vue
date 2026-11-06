<template>
  <header class="home-header" :class="{ 'is-dark': isDark }">
    <!-- 装饰性渐变光斑（现代感，不抢内容） -->
    <div class="hh-blob hh-blob-1" />
    <div class="hh-blob hh-blob-2" />

    <div class="hh-inner">
      <div class="hh-text">
        <h1 class="hh-title">{{ siteName || '欢迎来到社区' }}</h1>
        <p class="hh-subtitle">{{ subtitle }}</p>
      </div>

      <div class="hh-actions">
        <!-- 醒目搜索（语义同导航栏：回车跳首页搜索结果） -->
        <el-input
          v-model="searchText"
          class="hh-search"
          size="large"
          clearable
          placeholder="搜索作品 / 作者 / 标签…"
          @keyup.enter="doSearch"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
          <template #append>
            <el-button :icon="Search" @click="doSearch">搜索</el-button>
          </template>
        </el-input>

        <div class="hh-btns">
          <el-button type="primary" size="large" round @click="$router.push('/novels')">
            <el-icon><Reading /></el-icon> 浏览作品
          </el-button>
          <el-button v-if="authStore.isLoggedIn" size="large" round @click="$router.push('/author/editor')">
            <el-icon><EditPen /></el-icon> 开始创作
          </el-button>
          <el-button v-else size="large" round @click="$router.push('/register')">
            <el-icon><UserFilled /></el-icon> 加入我们
          </el-button>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { useRouter } from 'vue-router';
import { storeToRefs } from 'pinia';
import { Search, Reading, EditPen, UserFilled } from '@element-plus/icons-vue';
import { useAppStore } from '@/stores/app';
import { useAuthStore } from '@/stores/auth';
import { useThemeStore } from '@/stores/theme';

const router = useRouter();
const appStore = useAppStore();
const authStore = useAuthStore();
const themeStore = useThemeStore();
const { siteName } = storeToRefs(appStore);
const isDark = computed(() => themeStore.isDark);

const searchText = ref('');
// 一句契合「ACG 创作社区」气质的副标题；后续可由平台配置覆盖
const subtitle = computed(() => '在这里，写下你的故事，遇见同好——小说、博客、论坛与创作工具，一站直达。');

function doSearch() {
  const q = searchText.value.trim();
  if (q) router.push({ path: '/', query: { search: q } });
}
</script>

<style scoped>
.home-header {
  position: relative;
  overflow: hidden;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-color);
  background: var(--bg-card);
  box-shadow: var(--shadow-card);
  margin-bottom: 26px;
  /* 玻璃质感顶栏：随结构化主题变量变化 */
  backdrop-filter: blur(var(--glass-blur, 20px));
  -webkit-backdrop-filter: blur(var(--glass-blur, 20px));
}
.is-dark.home-header {
  background: rgba(26, 29, 39, 0.82);
}

/* 渐变光斑：纯装饰，低透明度，避免喧宾夺主 */
.hh-blob {
  position: absolute;
  border-radius: 50%;
  filter: blur(70px);
  opacity: 0.35;
  pointer-events: none;
}
.hh-blob-1 {
  width: 320px; height: 320px;
  background: radial-gradient(circle, #6366f1 0%, transparent 70%);
  top: -120px; right: -60px;
}
.hh-blob-2 {
  width: 260px; height: 260px;
  background: radial-gradient(circle, #8b5cf6 0%, transparent 70%);
  bottom: -120px; left: -40px;
  opacity: 0.28;
}

.hh-inner {
  position: relative;
  z-index: 1;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 40px 44px;
}
.hh-text { min-width: 260px; flex: 1 1 320px; }
.hh-title {
  font-size: 2.4rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  margin: 0 0 10px;
  background: linear-gradient(120deg, var(--primary-color), #8b5cf6 60%, #ec4899);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  color: var(--text-color);
}
.hh-subtitle {
  margin: 0;
  font-size: 1rem;
  line-height: 1.7;
  color: var(--text-light);
  max-width: 520px;
}
.hh-actions {
  flex: 1 1 360px;
  max-width: 460px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.hh-search { width: 100%; }
.hh-btns { display: flex; gap: 12px; flex-wrap: wrap; }

@media (max-width: 768px) {
  .hh-inner { padding: 28px 22px; }
  .hh-title { font-size: 1.8rem; }
  .hh-actions { max-width: 100%; }
}
</style>
