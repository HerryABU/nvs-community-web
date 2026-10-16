<template>
  <div class="admin-page">
    <AdminHeader title="Web 项目额度" subtitle="公开额度管控与超额审核" :icon="Stamp" />

    <div class="admin-page-body admin-panel">
      <p class="hint-text">
        平台对每位用户公开到广场的 Web 项目（自定义 HTML 扩展 + UI 模板合计）数量设上限。
        达到额度后继续公开的项目将进入「超额待审核」，由管理员批准后才能在广场展示。
      </p>

      <!-- ============ 额度配置 ============ -->
      <el-divider content-position="left">额度配置</el-divider>

      <el-card shadow="never" class="config-card">
        <template #header>
          <span class="card-title">全局默认额度</span>
        </template>
        <el-form :inline="true" @submit.prevent>
          <el-form-item label="每人默认可公开数">
            <el-input-number v-model="globalQuota" :min="0" :max="1000" :step="1" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingGlobal" @click="saveGlobal">保存全局默认</el-button>
            <span class="muted small">0 = 禁止任何人公开发布</span>
          </el-form-item>
        </el-form>
      </el-card>

      <el-card shadow="never" class="config-card" style="margin-top:16px">
        <template #header>
          <span class="card-title">按人覆盖</span>
        </template>
        <el-form :inline="true" @submit.prevent>
          <el-form-item label="用户 ID">
            <el-input v-model.number="overrideUserID" type="number" placeholder="用户 ID" style="width:140px" />
          </el-form-item>
          <el-form-item label="额度">
            <el-input-number v-model="overrideQuota" :min="-1" :max="1000" :step="1" />
            <span class="muted small">-1 跟随全局 / 0 禁止 / &gt;0 专属上限</span>
          </el-form-item>
          <el-form-item label="备注">
            <el-input v-model="overrideNote" placeholder="选填" style="width:180px" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingOverride" @click="saveOverride">设置</el-button>
          </el-form-item>
        </el-form>

        <el-table :data="overrides" style="width:100%;margin-top:8px" v-loading="loadingOverrides">
          <el-table-column prop="user_id" label="用户 ID" width="100" />
          <el-table-column prop="username" label="用户名" min-width="120" show-overflow-tooltip />
          <el-table-column label="额度" width="160">
            <template #default="{ row }">
              <el-tag size="small" :type="row.quota === -1 ? 'info' : (row.quota === 0 ? 'danger' : 'success')">
                {{ row.quota === -1 ? '跟随全局' : (row.quota === 0 ? '禁止公开' : row.quota + ' 个') }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="note" label="备注" min-width="140" show-overflow-tooltip />
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="danger" text @click="removeOverride(row)">撤销</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <!-- ============ 超额待审列表 ============ -->
      <el-divider content-position="left">超额待审列表</el-divider>

      <div class="toolbar">
        <el-radio-group v-model="filterStatus" @change="loadRequests">
          <el-radio-button value="">全部</el-radio-button>
          <el-radio-button value="pending">待审核</el-radio-button>
          <el-radio-button value="approved">已批准</el-radio-button>
          <el-radio-button value="rejected">已驳回</el-radio-button>
        </el-radio-group>
        <el-button size="small" :loading="loadingRequests" @click="loadRequests" :icon="Refresh" style="margin-left:8px">刷新</el-button>
      </div>

      <el-table :data="requests" v-loading="loadingRequests" style="width:100%;margin-top:12px">
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.target_type === 'frame' ? 'primary' : 'success'">
              {{ row.target_type === 'frame' ? 'UI 模板' : 'HTML 扩展' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="target_name" label="项目名称" min-width="160" show-overflow-tooltip />
        <el-table-column prop="username" label="申请人" width="120" show-overflow-tooltip />
        <el-table-column prop="reason" label="申请理由" min-width="180" show-overflow-tooltip />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="reqTag(row.status)">{{ reqLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="提交时间" width="170" show-overflow-tooltip />
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <template v-if="row.status === 'pending'">
              <el-button size="small" type="success" text @click="openReview(row, 'approve')">批准</el-button>
              <el-button size="small" type="danger" text @click="openReview(row, 'reject')">驳回</el-button>
            </template>
            <span v-else class="muted small">{{ row.review_note || '—' }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 审核意见弹窗 -->
    <el-dialog v-model="showReview" :title="reviewAction === 'approve' ? '批准超额公开' : '驳回超额公开'" width="480px" destroy-on-close>
      <div class="review-meta">
        项目：<b>{{ current?.target_name || '—' }}</b>（{{ current?.target_type === 'frame' ? 'UI 模板' : 'HTML 扩展' }}）
        申请人：{{ current?.username || '—' }}
      </div>
      <el-input v-model="reviewNote" type="textarea" :rows="3" maxlength="200"
        :placeholder="reviewAction === 'reject' ? '驳回建议填写原因，申请人可在工作台看到' : '选填'" />
      <template #footer>
        <el-button @click="showReview = false">取消</el-button>
        <el-button :type="reviewAction === 'approve' ? 'success' : 'danger'" :loading="reviewing" @click="submitReview">
          {{ reviewAction === 'approve' ? '批准并公开' : '驳回' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { Refresh, Stamp } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { webQuotaApi } from '@/api/webQuota';
import { ElMessage } from 'element-plus';

interface QuotaRequest {
  id: number; user_id: number; target_type: string; target_id: number; status: string;
  reason: string; review_note: string; username: string; target_name: string; created_at: string;
}
interface OverrideRow {
  user_id: number; username: string; quota: number; note: string;
}

// ---- 配置 ----
const globalQuota = ref(3);
const savingGlobal = ref(false);
const overrides = ref<OverrideRow[]>([]);
const loadingOverrides = ref(false);

const overrideUserID = ref<number | null>(null);
const overrideQuota = ref(-1);
const overrideNote = ref('');
const savingOverride = ref(false);

async function loadConfig() {
  try {
    const res = await webQuotaApi.getConfig();
    if (res.data.code === 0) {
      globalQuota.value = res.data.data.max_per_user ?? 3;
      overrides.value = res.data.data.overrides || [];
    }
  } catch { /* ignore */ }
  loadingOverrides.value = false;
}
async function saveGlobal() {
  savingGlobal.value = true;
  try {
    const res = await webQuotaApi.setConfig({ action: 'global', max_per_user: globalQuota.value });
    if (res.data.code === 0) ElMessage.success('全局默认额度已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingGlobal.value = false;
}
async function saveOverride() {
  if (!overrideUserID.value) {
    ElMessage.warning('请填写用户 ID');
    return;
  }
  savingOverride.value = true;
  try {
    const res = await webQuotaApi.setConfig({
      action: 'set_user',
      user_id: overrideUserID.value,
      quota: overrideQuota.value,
      note: overrideNote.value,
    });
    if (res.data.code === 0) {
      ElMessage.success('已保存按人额度');
      overrideNote.value = '';
      await loadConfig();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingOverride.value = false;
}
async function removeOverride(row: OverrideRow) {
  try {
    const res = await webQuotaApi.setConfig({ action: 'remove_user', user_id: row.user_id });
    if (res.data.code === 0) {
      ElMessage.success('已撤销按人额度（恢复跟随全局）');
      await loadConfig();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

// ---- 待审列表 ----
const requests = ref<QuotaRequest[]>([]);
const loadingRequests = ref(false);
const filterStatus = ref('pending');

async function loadRequests() {
  loadingRequests.value = true;
  try {
    const res = await webQuotaApi.listRequests(filterStatus.value);
    if (res.data.code === 0) requests.value = res.data.data || [];
  } catch { /* ignore */ }
  loadingRequests.value = false;
}

function reqTag(s: string) {
  const map: Record<string, string> = { pending: 'warning', approved: 'success', rejected: 'danger' };
  return (map[s] as any) || 'info';
}
function reqLabel(s: string) {
  const map: Record<string, string> = { pending: '待审核', approved: '已批准', rejected: '已驳回' };
  return map[s] || s;
}

// ---- 审核弹窗 ----
const showReview = ref(false);
const current = ref<QuotaRequest | null>(null);
const reviewAction = ref<'approve' | 'reject'>('approve');
const reviewNote = ref('');
const reviewing = ref(false);

function openReview(row: QuotaRequest, action: 'approve' | 'reject') {
  current.value = row;
  reviewAction.value = action;
  reviewNote.value = '';
  showReview.value = true;
}
async function submitReview() {
  if (!current.value) return;
  reviewing.value = true;
  try {
    const fn = reviewAction.value === 'approve' ? webQuotaApi.approveRequest : webQuotaApi.rejectRequest;
    const res = await fn(current.value.id, reviewNote.value.trim());
    if (res.data.code === 0) {
      ElMessage.success(reviewAction.value === 'approve' ? '已批准并公开' : '已驳回');
      showReview.value = false;
      await loadRequests();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
  reviewing.value = false;
}

onMounted(() => {
  loadConfig();
  loadRequests();
});
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: 1100px; }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; }
.card-title { font-weight: 600; color: var(--text-color, #333); }
.config-card { border-radius: 10px; }
.toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.review-meta { font-size: 0.85rem; color: var(--text-color, #333); margin-bottom: 10px; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; margin-left: 8px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .card-title,
[data-theme="dark"] .review-meta { color: #e2e8f0; }
</style>
