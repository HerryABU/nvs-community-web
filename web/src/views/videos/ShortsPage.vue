<template>
  <div class="shorts-page">
    <SwipeFeed />
    <div class="shorts-nav">
      <router-link to="/discover">← 返回发现</router-link>
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

/* 手机端整屏覆盖：不依赖祖先高度，也不会被外层页面抢走手势。
   真机上 100vh 常大于可视区（地址栏），所以优先用 100dvh。 */
@media (max-width: 767px) {
  .shorts-page {
    position: fixed;
    top: 0; left: 0; right: 0; bottom: 0;
    height: 100%;
    z-index: 200;
    padding-bottom: env(safe-area-inset-bottom);
  }
}
@supports (height: 100dvh) {
  @media (max-width: 767px) {
    .shorts-page { bottom: auto; height: 100dvh; }
  }
}
</style>
