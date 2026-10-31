<template>
  <!-- 首页形态由后台配置（novels / blogs / forum / feed）。
       加载期间先占位，避免"闪现小说首页再切走"的错乱感。 -->
  <div v-if="loading" class="home-switch-loading">
    <div class="hs-spinner" />
    <p class="hs-text">正在加载首页…</p>
  </div>
  <component :is="currentView" v-else />
</template>

<script setup lang="ts">
import { ref, computed, onMounted, defineAsyncComponent } from 'vue';
import api from '@/api';

// 懒加载各首页视图：只有被选中的那套才会真正下载对应 chunk
const NovelHome = defineAsyncComponent(() => import('@/views/home/Home.vue'));
const BlogHome = defineAsyncComponent(() => import('@/views/blog/BlogList.vue'));
const ForumHome = defineAsyncComponent(() => import('@/views/forum/Forums.vue'));
const FeedHome = defineAsyncComponent(() => import('@/views/home/Feed.vue'));

const VIEW_MAP: Record<string, any> = {
  novels: NovelHome,
  blogs: BlogHome,
  forum: ForumHome,
  feed: FeedHome,
};

const view = ref('novels');
const loading = ref(true);

const currentView = computed(() => VIEW_MAP[view.value] || NovelHome);

onMounted(async () => {
  try {
    const res = await api.get<{ code: number; data: { view: string } }>('/home-view');
    const v = res.data?.data?.view;
    if (v && VIEW_MAP[v]) view.value = v;
  } catch {
    // 配置读取失败不能把首页搞白：退回作品首页（历史默认行为）
    view.value = 'novels';
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.home-switch-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  min-height: 52vh;
}
.hs-spinner {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  border: 3px solid var(--border-color, #e8eaf0);
  border-top-color: var(--el-color-primary, #409eff);
  animation: hs-spin 0.8s linear infinite;
}
@keyframes hs-spin {
  to { transform: rotate(360deg); }
}
.hs-text {
  font-size: 0.84rem;
  color: var(--text-light, #999);
  margin: 0;
}
</style>
