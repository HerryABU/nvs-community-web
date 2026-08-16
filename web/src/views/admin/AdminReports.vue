<template>
  <div class="admin-page">
    <div class="admin-page-header">
      <el-button text @click="$router.push('/admin')"><el-icon><ArrowLeft /></el-icon> 返回后台</el-button>
      <h2>举报处理</h2>
    </div>
    <div class="admin-page-body">
      <p class="hint-text">处理用户举报：判定成立/不成立，成立时可选择删除被举报内容。</p>

      <div class="toolbar">
        <el-radio-group v-model="filterStatus" @change="load">
          <el-radio-button value="">全部</el-radio-button>
          <el-radio-button value="pending">待处理</el-radio-button>
          <el-radio-button value="accepted">已成立</el-radio-button>
          <el-radio-button value="rejected">已驳回</el-radio-button>
        </el-radio-group>
        <el-button size="small" @click="load" :loading="loading" :icon="Refresh" style="margin-left:8px">刷新</el-button>
      </div>

      <el-table :data="reports" v-loading="loading" style="width:100%;margin-top:12px">
        <el-table-column label="对象" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="targetTag(row.target_type)">{{ targetLabel(row.target_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="理由" min-width="100">
          <template #default="{ row }">{{ row.reason }}</template>
        </el-table-column>
        <el-table-column label="说明" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.detail || '—' }}</template>
        </el-table-column>
        <el-table-column label="举报人" width="120">
          <template #default="{ row }">{{ row.reporter?.nickname || row.reporter?.username || `#${row.reporter_id}` }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="statusTag(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="110" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 'pending'" size="small" type="primary" text @click="openHandle(row)">处理</el-button>
            <span v-else class="muted">{{ row.verdict ? '已处理' : '—' }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 处理弹窗 -->
    <el-dialog v-model="showHandle" title="处理举报" width="520px" destroy-on-close>
      <div v-if="current" class="handle-body">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="对象类型">{{ targetLabel(current.target_type) }}</el-descriptions-item>
          <el-descriptions-item label="对象ID">#{{ current.target_id }}</el-descriptions-item>
          <el-descriptions-item label="举报理由">{{ current.reason }}</el-descriptions-item>
          <el-descriptions-item label="举报人">{{ current.reporter?.nickname || current.reporter?.username }}</el-descriptions-item>
        </el-descriptions>
        <div v-if="current.detail" class="detail-box">「{{ current.detail }}」</div>

        <el-form label-position="top" style="margin-top:12px">
          <el-form-item label="处理结论" required>
            <el-radio-group v-model="verdict">
              <el-radio value="accepted">判定成立</el-radio>
              <el-radio value="rejected">判定不成立</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item v-if="verdict === 'accepted'" label="处理措施">
            <el-checkbox v-model="deleteTarget">删除被举报内容</el-checkbox>
          </el-form-item>
          <el-form-item label="处理意见（选填）">
            <el-input v-model="note" type="textarea" :rows="2" maxlength="300" placeholder="填写处理说明，举报人可在「我的举报」中查看" />
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
        <el-button @click="showHandle = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitHandle">提交处理</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ArrowLeft, Refresh } from '@element-plus/icons-vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const reports = ref<any[]>([]);
const loading = ref(false);
const filterStatus = ref('pending');

const showHandle = ref(false);
const current = ref<any>(null);
const verdict = ref<'accepted' | 'rejected'>('accepted');
const deleteTarget = ref(false);
const note = ref('');
const submitting = ref(false);

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

async function load() {
  loading.value = true;
  try {
    const res = await adminApi.getReports(filterStatus.value);
    if (res.data.code === 0) reports.value = res.data.data || [];
  } catch { /* ignore */ }
  loading.value = false;
}

function openHandle(row: any) {
  current.value = row;
  verdict.value = 'accepted';
  deleteTarget.value = false;
  note.value = '';
  showHandle.value = true;
}

async function submitHandle() {
  if (!current.value) return;
  submitting.value = true;
  try {
    const res = await adminApi.handleReport(current.value.id, {
      status: verdict.value,
      verdict: note.value.trim(),
      delete_target: deleteTarget.value,
    });
    if (res.data.code === 0) {
      ElMessage.success('已处理');
      showHandle.value = false;
      await load();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
  submitting.value = false;
}

onMounted(load);
</script>

<style scoped>
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 1100px; }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; }
.toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.handle-body { color: var(--text-color, #333); }
.detail-box { margin-top: 8px; padding: 8px 12px; background: rgba(0,0,0,0.04); border-radius: 6px; font-size: 0.85rem; color: var(--text-secondary, #666); }
.muted { color: var(--text-light, #999); }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
</style>
