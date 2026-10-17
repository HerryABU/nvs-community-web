<template>
  <div class="me-comments">
    <h2 class="section-title">读者留言</h2>
    <el-alert
      v-if="!firstNovelId"
      type="info"
      :closable="false"
      title="暂无作品，无法展示留言"
      style="margin-bottom:12px"
    />
    <CommentSection
      v-else
      :novel-id="firstNovelId"
      :novel-category="''"
      :owner-id="authorId"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, inject, type Ref } from 'vue';
import CommentSection from '@/components/CommentSection.vue';

const profile = inject<Ref<any>>('meProfile');
const authorId = inject<Ref<number>>('meAuthorId')!;

// 复用头部已拉取的作品列表，取第一部作为留言区载体（与旧 AuthorHome 行为一致）
const firstNovelId = computed(() => {
  const list = profile?.value?.novels || [];
  return list.length > 0 ? list[0].id : 0;
});
</script>

<style scoped>
.section-title { font-size: 1.15rem; margin-bottom: 12px; color: var(--text-color); }
</style>
