<template>
  <div class="quota-page">
    <h2 class="page-title">我的 Web 项目额度</h2>
    <p class="hint-text">
      平台对每位用户公开到广场的 Web 项目（自定义 HTML 扩展 + UI 模板合计）设「数量」与「总大小」双重额度。
      任一维度达额后，继续公开的项目会进入「超额待审核」，管理员批准后才能在广场展示。
    </p>

    <!-- 数量维度 -->
    <el-card shadow="never" class="stat-card">
      <div class="dim-title">公开数量额度</div>
      <div class="stat-grid">
        <div class="stat-item">
          <div class="stat-num">{{ data.quota < 0 ? '不限' : data.quota }}</div>
          <div class="stat-label">额度上限（个）</div>
        </div>
        <div class="stat-item">
          <div class="stat-num">{{ data.used }}</div>
          <div class="stat-label">已公开</div>
        </div>
        <div class="stat-item">
          <div class="stat-num" :class="{ 'warn': data.remaining === 0 }">
            {{ data.remaining < 0 ? '不限' : data.remaining }}
          </div>
          <div class="stat-label">剩余可公开</div>
        </div>
      </div>
    </el-card>

    <!-- 大小维度 -->
    <el-card shadow="never" class="stat-card">
      <div class="dim-title">公开总大小额度</div>
      <div class="stat-grid">
        <div class="stat-item">
          <div class="stat-num">{{ data.size_quota_mb <= 0 ? '不限' : fmtMB(data.size_quota_mb) }}</div>
          <div class="stat-label">额度上限</div>
        </div>
        <div class="stat-item">
          <div class="stat-num">{{ fmtBytes(data.size_used_bytes) }}</div>
          <div class="stat-label">已占用</div>
        </div>
        <div class="stat-item">
          <div class="stat-num" :class="{ 'warn': data.size_quota_mb > 0 && data.size_remaining_mb === 0 }">
            {{ data.size_remaining_mb < 0 ? '不限' : fmtMB(data.size_remaining_mb) }}
          </div>
          <div class="stat-label">剩余可用</div>
        </div>
      </div>
    </el-card>

    <div v-if="data.overridden" class="override-tip">
      你的额度由管理员单独设置（数量：{{ data.override_quota === -1 ? '跟随全局' : data.override_quota + ' 个' }}；
      大小：{{ data.override_size_mb === -1 ? '跟随全局' : (data.override_size_mb === 0 ? '不限' : data.override_size_mb + ' MB') }}）
      <span v-if="data.override_note" class="muted">— {{ data.override_note }}</span>
    </div>
    <div v-else class="override-tip muted">
      当前跟随全局默认额度（数量 {{ data.global_default }} 个；大小 {{ data.global_size_mb <= 0 ? '不限' : data.global_size_mb + ' MB' }}）
    </div>

    <el-divider content-position="left">我的超额审核单</el-divider>
    <el-table :data="data.requests || []" v-loading="loading" style="width:100%">
      <el-table-column label="类型" width="110">
        <template #default="{ row }">
          <el-tag size="small" :type="row.target_type === 'frame' ? 'primary' : 'success'">
            {{ row.target_type === 'frame' ? 'UI 模板' : 'HTML 扩展' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="target_name" label="项目名称" min-width="160" show-overflow-tooltip />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag size="small" :type="reqTag(row.status)">{{ reqLabel(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="reason" label="申请理由" min-width="180" show-overflow-tooltip />
      <el-table-column prop="review_note" label="处理意见" min-width="160" show-overflow-tooltip />
      <el-table-column prop="created_at" label="提交时间" width="170" show-overflow-tooltip />
    </el-table>
    <el-empty v-if="!loading && (!data.requests || data.requests.length === 0)" description="暂无超额审核单" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { webQuotaApi } from '@/api/webQuota';
import { ElMessage } from 'element-plus';

interface MyQuotaData {
  quota: number; used: number; remaining: number;
  size_quota_mb: number; size_used_bytes: number; size_remaining_mb: number;
  overridden: boolean; override_quota: number | null; override_size_mb: number | null; override_note: string;
  global_default: number; global_size_mb: number;
  requests: {
    id: number; target_type: string; target_id: number; status: string;
    reason: string; review_note: string; target_name: string; created_at: string;
  }[];
}

const data = ref<MyQuotaData>({
  quota: 0, used: 0, remaining: 0,
  size_quota_mb: 0, size_used_bytes: 0, size_remaining_mb: -1,
  overridden: false, override_quota: null, override_size_mb: null,
  override_note: '', global_default: 3, global_size_mb: 0, requests: [],
});
const loading = ref(false);

/** 字节数格式化为易读单位（B/KB/MB/GB） */
function fmtBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const v = bytes / Math.pow(1024, i);
  return (i === 0 ? v.toFixed(0) : v.toFixed(1)) + ' ' + units[i];
}
/** MB 数格式化为易读单位（>=1024 MB 显示 GB） */
function fmtMB(mb: number): string {
  if (mb < 1024) return mb + ' MB';
  return (mb / 1024).toFixed(1) + ' GB';
}

async function load() {
  loading.value = true;
  try {
    const res = await webQuotaApi.myQuota();
    if (res.data.code === 0) data.value = res.data.data;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  }
  loading.value = false;
}

function reqTag(s: string) {
  const map: Record<string, string> = { pending: 'warning', approved: 'success', rejected: 'danger' };
  return (map[s] as any) || 'info';
}
function reqLabel(s: string) {
  const map: Record<string, string> = { pending: '待审核', approved: '已批准', rejected: '已驳回' };
  return map[s] || s;
}

onMounted(load);
</script>

<style scoped>
.quota-page { max-width: 900px; margin: 0 auto; padding: 24px; }
.page-title { margin: 0 0 8px; font-size: 1.3rem; color: var(--text-color, #333); }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 16px; }
.stat-card { border-radius: 10px; margin-bottom: 16px; }
.dim-title { font-weight: 600; font-size: 0.95rem; color: var(--text-color, #333); margin-bottom: 12px; }
.stat-grid { display: flex; gap: 24px; }
.stat-item { text-align: center; flex: 1; }
.stat-num { font-size: 1.8rem; font-weight: 700; color: var(--el-color-primary, #409eff); }
.stat-num.warn { color: var(--el-color-danger, #f56c6c); }
.stat-label { font-size: 0.85rem; color: var(--text-light, #666); margin-top: 4px; }
.override-tip { margin-top: 4px; margin-bottom: 12px; font-size: 0.82rem; color: var(--text-color, #333); }
.muted { color: var(--text-light, #999); }
[data-theme="dark"] .page-title,
[data-theme="dark"] .dim-title { color: #e2e8f0; }
[data-theme="dark"] .override-tip { color: #cbd5e1; }
</style>
