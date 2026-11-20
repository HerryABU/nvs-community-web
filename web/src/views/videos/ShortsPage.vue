<template>
  <div class="shorts-page">
    <SwipeFeed />
    <div class="shorts-nav">
      <router-link to="/videos">← 视频频道</router-link>
      <span class="tip">上下滑动切换 · 点击画面暂停</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import SwipeFeed from '@/components/feed/SwipeFeed.vue';
</script>

<style scoped>
.shorts-page { height: calc(100vh - 60px); background: #000; position: relative; }
.shorts-nav {
  position: absolute; left: 12px; top: 12px; z-index: 4; display: flex; align-items: center; gap: 10px;
}
.shorts-nav a {
  color: #fff; text-decoration: none; font-size: 0.8rem;
  background: rgba(255, 255, 255, 0.16); border: 1px solid rgba(255, 255, 255, 0.35);
  padding: 5px 12px; border-radius: 14px; backdrop-filter: blur(4px);
}
.shorts-nav .tip { color: rgba(255, 255, 255, 0.7); font-size: 0.74rem; }
@media (max-width: 768px) { .shorts-nav .tip { display: none; } }
</style>
