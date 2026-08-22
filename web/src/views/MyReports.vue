<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2>🚨 我的举报</h2>
        <p class="subtitle">查看您提交的举报及处理进度</p>
      </div>
      <el-button @click="load" :loading="loading" :icon="Refresh">刷新</el-button>
    </div>

    <div v-loading="loading" class="report-list">
      <el-empty v-if="!loading && reports.length === 0" description="您还没有提交过举报" />
      <el-table v-else :data="reports" style="width: 100%">
        <el-table-column label="举报对象" min-width="200">
          <template #default="{ row }">
            <el-tag size="small" :type="targetTag(row.target_type)">{{ targetLabel(row.target_type) }}</el-tag>
            <div class="preview-text">{{ row.target_preview || `#${row.target_id}` }}</div>
          </template>
        </el-table-column>
        <el-table-column label="举报理由" min-width="120">
          <template #default="{ row }">
            <span>{{ row.reason }}</span>
            <div v-if="row.detail" class="preview-text">{{ row.detail }}</div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="处理意见" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.verdict" class="verdict-text">{{ row.verdict }}</span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="提交时间" width="160">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { Refresh } from '@element-plus/icons-vue';
import { reportApi, type ReportItem } from '@/api/report';

const reports = ref<ReportItem[]>([]);
const loading = ref(false);

function targetLabel(t: string) {
  const map: Record<string, string> = { novel: '作品', comment: '评论', thread: '帖子', post: '回帖' };
  return map[t] || t;
}
function targetTag(t: string) {
  const map: Record<string, string> = { novel: 'primary', comment: 'success', thread: 'warning', post: 'info' };
  return (map[t] as any) || 'info';
}
function statusLabel(s: string) {
  const map: Record<string, string> = { pending: '待处理', accepted: '已成立', rejected: '已驳回' };
  return map[s] || s;
}
function statusTag(s: string) {
  const map: Record<string, string> = { pending: 'warning', accepted: 'danger', rejected: 'info' };
  return (map[s] as any) || 'info';
}
function formatTime(d: string) {
  return d ? new Date(d).toLocaleString('zh-CN') : '';
}

async function load() {
  loading.value = true;
  try {
    const res = await reportApi.mine();
    if (res.data.code === 0) reports.value = res.data.data || [];
  } catch { /* ignore */ }
  loading.value = false;
}

onMounted(load);
</script>

<style scoped>
.page-container { max-width: 960px; margin: 0 auto; padding: 24px 20px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; flex-wrap: wrap; gap: 12px; }
.page-header h2 { margin: 0; font-size: 1.4rem; }
.subtitle { color: var(--text-secondary); margin: 4px 0 0; font-size: 0.9rem; }
.report-list { min-height: 200px; }
.preview-text { font-size: 0.78rem; color: var(--text-secondary); margin-top: 2px; max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.verdict-text { font-size: 0.82rem; }
.muted { color: var(--text-secondary); }

/* ====== 移动端我的举报适配 ====== */
@media (max-width: 768px) {
  .page-header {
    flex-wrap: wrap;
    gap: 8px;
  }
  .report-list {
    overflow-x: auto;
  }
  .report-list .el-table {
    min-width: 640px;
  }
}
</style>
