<template>
  <!-- ====== 自定义 HTML 区块（F15：管理员输入，服务端已消毒，前端 v-html 渲染） ====== -->
  <section v-if="html" class="home-block block-custom">
    <header v-if="title" class="bc-header">
      <h2>{{ title }}</h2>
    </header>
    <div class="bc-body" v-html="html"></div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  title?: string;
  /** {html: "..."} —— 服务端保存时已过 security.SanitizeUserContent */
  config: string;
}>();

const html = computed(() => {
  try {
    const parsed = JSON.parse(props.config || '{}');
    return typeof parsed.html === 'string' ? parsed.html : '';
  } catch {
    return '';
  }
});
</script>

<style scoped>
.home-block {
  background: var(--bg-card);
  border-radius: var(--radius-lg);
  padding: 22px 26px;
  margin-bottom: 26px;
  box-shadow: var(--shadow-card);
}
.bc-header h2 { margin: 0 0 12px; font-size: 1.15rem; font-weight: 600; }
.bc-body { font-size: .92rem; line-height: 1.7; }
</style>
