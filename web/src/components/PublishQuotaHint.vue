<template>
  <span v-if="visible" class="pq-hint" :class="{ warn: exhausted }">
    <el-tooltip :content="tipText" placement="bottom">
      <span class="pq-hint-inner">
        <el-icon class="pq-ico"><Timer /></el-icon>{{ text }}
      </span>
    </el-tooltip>
  </span>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { Timer } from '@element-plus/icons-vue';
import { publishQuotaApi, type MyPublishQuota } from '@/api/publishQuota';

const q = ref<MyPublishQuota | null>(null);

onMounted(async () => {
  try {
    const res = await publishQuotaApi.myStatus();
    if (res.data.code === 0) q.value = res.data.data;
  } catch { /* 额度查询失败不打扰发布流程，静默隐藏 */ }
});

// 仅当启用了限额且存在数量/字数上限时展示
const visible = computed(() => !!q.value?.enabled && ((q.value?.count_limit ?? -1) > 0 || (q.value?.word_limit ?? 0) > 0));
const remainCount = computed(() => (q.value && q.value.count_limit > 0 ? Math.max(0, q.value.count_limit - q.value.used_count) : null));
const remainWords = computed(() => (q.value && q.value.word_limit > 0 ? Math.max(0, q.value.word_limit - q.value.used_words) : null));
const exhausted = computed(() => remainCount.value === 0 || remainWords.value === 0);

const text = computed(() => {
  const parts: string[] = [];
  if (remainCount.value !== null) parts.push(`还可发布 ${remainCount.value} 条`);
  if (remainWords.value !== null) parts.push(`还可 ${remainWords.value} 字`);
  return `今日发文：${parts.join(' · ')}`;
});
const tipText = computed(() => {
  if (!q.value) return '';
  const reset = new Date(q.value.reset_at);
  const hm = `${String(reset.getHours()).padStart(2, '0')}:${String(reset.getMinutes()).padStart(2, '0')}`;
  return `已用 ${q.value.used_count} 条` + (q.value.word_limit > 0 ? ` / ${q.value.used_words} 字` : '') + `，${hm} 重置`;
});
</script>

<style scoped>
.pq-hint { display: inline-flex; align-items: center; }
.pq-hint-inner {
  display: inline-flex; align-items: center; gap: 4px;
  font-size: 0.78rem; color: var(--text-light, #64748b);
  background: var(--bg-subtle, #f1f5f9); border-radius: 6px; padding: 3px 8px;
  cursor: default;
}
.pq-ico { font-size: 0.9rem; }
.pq-hint.warn .pq-hint-inner { color: #ef4444; background: rgba(239, 68, 68, 0.1); }
[data-theme="dark"] .pq-hint-inner { background: rgba(255,255,255,0.06); color: #94a3b8; }
[data-theme="dark"] .pq-hint.warn .pq-hint-inner { color: #f89898; background: rgba(239,68,68,0.15); }
</style>
