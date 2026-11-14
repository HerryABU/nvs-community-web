<template>
  <div class="ah">
    <div class="ah-head">
      <h3>活跃日历</h3>
      <span class="ah-sub">近 {{ days }} 天创作与互动（评论 / 回复 / 发帖）</span>
    </div>
    <div v-if="loading" class="ah-empty">加载中…</div>
    <div v-else class="ah-grid" :style="{ gridTemplateColumns: `repeat(${weeks}, 12px)` }">
      <span
        v-for="(it, i) in cells"
        :key="i"
        class="ah-cell"
        :class="levelClass(it.count)"
        :title="`${it.date}：${it.count} 次`"
      />
    </div>
    <div class="ah-legend">
      <span>少</span>
      <span class="ah-cell level-0" />
      <span class="ah-cell level-1" />
      <span class="ah-cell level-2" />
      <span class="ah-cell level-3" />
      <span class="ah-cell level-4" />
      <span>多</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import api from '@/api/index';

const props = withDefaults(defineProps<{ days?: number }>(), { days: 365 });
const loading = ref(true);
const items = ref<{ date: string; count: number }[]>([]);

const weeks = computed(() => Math.ceil(items.value.length / 7));

const cells = computed(() => items.value);

function levelClass(n: number): string {
  if (n <= 0) return 'level-0';
  if (n <= 2) return 'level-1';
  if (n <= 5) return 'level-2';
  if (n <= 9) return 'level-3';
  return 'level-4';
}

async function load() {
  loading.value = true;
  try {
    const res = await api.get<{ code: number; data: { items: { date: string; count: number }[] } }>(
      `/users/me/activity-heat?days=${props.days}`,
    );
    items.value = res.data?.data?.items || [];
  } catch {
    items.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(load);
defineExpose({ load });
</script>

<style scoped>
.ah { padding: 14px 16px; border: 1px solid var(--border-color, #e5e7eb); border-radius: var(--radius-md, 12px); background: var(--bg-card, #fff); }
.ah-head { display: flex; align-items: baseline; gap: 10px; margin-bottom: 10px; }
.ah-head h3 { margin: 0; font-size: 0.95rem; color: var(--text-color, #1a1a2e); }
.ah-sub { font-size: 0.72rem; color: var(--text-light, #999); }
.ah-empty { padding: 18px; text-align: center; color: var(--text-light, #999); font-size: 0.82rem; }
.ah-grid { display: grid; gap: 2px; width: max-content; }
.ah-cell { width: 12px; height: 12px; border-radius: 2px; background: var(--bg-soft, #f0f1f3); }
.ah-cell.level-1 { background: #c6e48b; }
.ah-cell.level-2 { background: #7bc96f; }
.ah-cell.level-3 { background: #239a3b; }
.ah-cell.level-4 { background: #196127; }
.ah-legend { display: flex; align-items: center; gap: 4px; margin-top: 8px; font-size: 0.68rem; color: var(--text-light, #999); }
.ah-legend .ah-cell { display: inline-block; }
[data-theme="dark"] .ah-cell.level-0 { background: #22303c; }
[data-theme="dark"] .ah-cell.level-1 { background: #2e4b2e; }
[data-theme="dark"] .ah-cell.level-2 { background: #3f7a3f; }
[data-theme="dark"] .ah-cell.level-3 { background: #55a055; }
[data-theme="dark"] .ah-cell.level-4 { background: #7bc96f; }
</style>
