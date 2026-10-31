<template>
  <!-- 灵感广场独立页：作品流可被外部链接直达（公开浏览） -->
  <div class="gal-page">
    <header class="gal-head">
      <div class="gal-head-left">
        <h1>灵感广场</h1>
        <p>看别人怎么用提示词出图 / 出片，点「复刻同款」直接带回 AI 工作台</p>
      </div>
      <div class="gal-head-right">
        <el-button size="small" @click="$router.push('/ai')">打开 AI 工作台</el-button>
      </div>
    </header>

    <div class="gal-panel">
      <AiGalleryPanel @remix="onRemix" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';
import AiGalleryPanel from '@/components/ai/AiGalleryPanel.vue';

const router = useRouter();

function onRemix(payload: { kind: string; prompt: string }) {
  const tab = payload.kind === 'video' ? 'video' : 'image';
  router.push({ path: '/ai', query: { tab, prompt: payload.prompt.slice(0, 1500) } });
}
</script>

<style scoped>
.gal-page {
  max-width: 1240px; margin: 0 auto; padding: 20px 20px 40px;
  display: flex; flex-direction: column; min-height: calc(100vh - 60px);
}
.gal-head {
  display: flex; align-items: flex-end; justify-content: space-between;
  gap: 16px; flex-wrap: wrap; margin-bottom: 8px;
}
.gal-head h1 { font-size: 1.3rem; margin: 0 0 4px; color: var(--text-color, #222); }
.gal-head p { margin: 0; font-size: 0.82rem; color: var(--text-light, #888); }
.gal-panel {
  flex: 1; min-height: 0; background: var(--el-bg-color, #fff);
  border: 1px solid var(--el-border-color-lighter, #ebeef5); border-radius: 14px; overflow: hidden;
}
@media (max-width: 720px) {
  .gal-page { padding: 14px 12px 30px; }
}
</style>
