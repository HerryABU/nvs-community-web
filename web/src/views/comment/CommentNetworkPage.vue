<template>
  <div class="comment-network-page page-container">
    <div class="cn-page-head">
      <el-button @click="$router.back()" text size="small">← 返回</el-button>
      <h2>评论网分析（社交图谱）</h2>
      <p class="cn-page-sub">
        公开分析：谁回复了谁的哪条评论，并基于时间估计评论者实际指向的最新评论。
        高频互怼将被标记为「疑似刷评/抱团」。
      </p>
    </div>
    <CommentNetworkView :target-type="targetType" :target-i-d="targetID" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import CommentNetworkView from '@/components/CommentNetworkView.vue';

const route = useRoute();
const targetType = computed(() => (route.query.type as string) || 'novel');
const targetID = computed(() => Number(route.query.id) || 0);
</script>

<style scoped>
.comment-network-page { max-width: 980px; margin: 0 auto; padding: 24px 16px; }
.cn-page-head h2 { margin: 8px 0 4px; }
.cn-page-sub { color: var(--text-light, #999); font-size: 0.85rem; margin: 0 0 16px; }
</style>
