<template>
  <div class="db-job-progress" :class="{ compact }">
    <template v-if="job">
      <div class="dbj-head">
        <span class="dbj-type">{{ typeLabel(job.type) }}</span>
        <el-tag size="small" :type="statusTag(job.status)">{{ statusLabel(job.status) }}</el-tag>
        <span class="dbj-stage">{{ job.stage || '处理中' }}</span>
        <span class="dbj-pct">{{ pct }}%</span>
      </div>
      <el-progress :percentage="pct" :status="progressStatus(job.status)" :stroke-width="compact ? 5 : 8" />
      <div class="dbj-content">{{ job.status_text || job.message || '正在处理…' }}</div>
      <div class="dbj-meta">
        <span v-if="job.tables_total > 0">{{ job.tables_done }}/{{ job.tables_total }} {{ unitLabel }}</span>
        <span v-if="job.rows">{{ formatRows(job.rows) }} 行</span>
        <span>{{ fmtTime(job.started_at) }}</span>
      </div>
    </template>
    <el-empty v-else description="暂无数据库任务" :image-size="compact ? 30 : 50" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = withDefaults(
  defineProps<{
    job?: Record<string, any> | null;
    compact?: boolean;
  }>(),
  { job: null, compact: false }
);

const TYPE_LABEL: Record<string, string> = {
  migrate: '数据库迁移',
  backup: '数据库备份',
  restore: '数据库恢复',
  'storage-mode': '正文存储迁移',
};
const STATUS_LABEL: Record<string, string> = {
  running: '进行中',
  done: '已完成',
  error: '失败',
};

function typeLabel(t?: string) {
  return TYPE_LABEL[t || ''] || t || '后台任务';
}
function statusLabel(s?: string) {
  return STATUS_LABEL[s || ''] || s || '未知';
}
function statusTag(s?: string) {
  return s === 'done' ? 'success' : s === 'error' ? 'danger' : 'primary';
}
function progressStatus(s?: string) {
  return s === 'done' ? 'success' : s === 'error' ? 'exception' : undefined;
}
const pct = computed(() => {
  const n = Number(props.job?.percent || 0);
  return Math.max(0, Math.min(100, n));
});
const unitLabel = computed(() => (props.job?.type === 'storage-mode' ? '作者批次' : '张表'));
function formatRows(n: number) {
  if (!n) return '0';
  if (n < 10000) return String(n);
  return (n / 10000).toFixed(1) + '万';
}
function fmtTime(t?: string) {
  if (!t) return '-';
  const d = new Date(t);
  return Number.isNaN(d.getTime()) ? String(t) : d.toLocaleString('zh-CN', { hour12: false });
}
</script>

<style scoped>
.db-job-progress {
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid var(--border-color, #e5e7eb);
  background: var(--bg-card, #fff);
  margin-bottom: 16px;
}
.db-job-progress.compact { padding: 0; border: none; background: transparent; margin: 0; }
.dbj-head { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
.dbj-type { font-weight: 600; font-size: 0.92rem; }
.dbj-stage { font-size: 0.82rem; color: var(--primary-color, #5b6ef5); }
.dbj-pct { margin-left: auto; font-size: 0.85rem; color: var(--text-muted, #6b7280); }
.dbj-content { margin-top: 9px; font-size: 0.84rem; color: var(--text-color, #222); line-height: 1.5; }
.dbj-meta { display: flex; gap: 14px; margin-top: 6px; font-size: 0.75rem; color: var(--text-muted, #888); flex-wrap: wrap; }
[data-theme='dark'] .db-job-progress:not(.compact) { background: #1e293b; border-color: #334155; }
</style>
