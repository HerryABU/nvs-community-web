<template>
  <div class="admin-page">
    <AdminHeader
      title="警方取证申请审批"
      subtitle="审核警方发起的取证包导出申请（批准 / 驳回），批准后方可由警方导出"
      :icon="Stamp"
    >
      <template #actions>
        <el-button size="small" :loading="loading" @click="loadList">刷新</el-button>
      </template>
    </AdminHeader>

    <el-alert
      type="info"
      :closable="false"
      show-icon
      title="审批说明"
      description="警方提交取证申请后，由站长在此审批。批准即授权该警方账号导出指定用户的取证包；驳回须填写理由。所有审批动作记审计留痕。"
      style="margin-bottom: 16px"
    />

    <div class="pr-toolbar">
      <el-select v-model="filterStatus" placeholder="全部状态" clearable style="width: 150px" @change="page = 1; loadList()">
        <el-option label="待审" value="pending" />
        <el-option label="已批准" value="approved" />
        <el-option label="已驳回" value="rejected" />
      </el-select>
      <el-button type="primary" :loading="loading" @click="page = 1; loadList()">查询</el-button>
    </div>

    <StateBlock v-if="loading" state="loading" skeleton="table" :rows="6" />
    <StateBlock v-else-if="loadError" state="error" title="加载失败" description="请稍后重试" action-text="重试" @action="loadList" />
    <template v-else>
      <el-table :data="items" stripe border style="width: 100%">
        <el-table-column prop="no" label="申请编号" width="170" />
        <el-table-column prop="requester_id" label="申请警员ID" width="100" />
        <el-table-column prop="target_user_id" label="目标用户ID" width="100" />
        <el-table-column prop="target_name" label="目标用户名" min-width="120" />
        <el-table-column prop="reason" label="取证理由" min-width="220" show-overflow-tooltip />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="review_note" label="审批意见" min-width="160" show-overflow-tooltip />
        <el-table-column prop="created_at" label="提交时间" width="150">
          <template #default="{ row }">{{ fmt(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <template v-if="row.status === 'pending'">
              <el-button size="small" type="success" :loading="actingId === row.id" @click="doReview(row, true)">批准</el-button>
              <el-button size="small" type="danger" plain :loading="actingId === row.id" @click="openReject(row)">驳回</el-button>
            </template>
            <span v-else class="muted small">已处理</span>
          </template>
        </el-table-column>
      </el-table>

      <div class="pr-pager">
        <el-pagination
          v-model:current-page="page"
          :page-size="size"
          :total="total"
          layout="total, prev, pager, next"
          background
          @current-change="loadList"
        />
      </div>
    </template>

    <!-- 驳回对话框 -->
    <el-dialog v-model="rejectVisible" title="驳回申请" width="460px">
      <el-form label-width="80px">
        <el-form-item label="驳回理由">
          <el-input v-model="rejectForm.note" type="textarea" :rows="3" placeholder="必填：驳回须说明理由" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rejectVisible = false">取消</el-button>
        <el-button type="danger" :loading="actingId === rejectForm.id" @click="doReject">确认驳回</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { Stamp } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import StateBlock from '@/components/common/StateBlock.vue';
import { aiSafetyApi, type EvidenceRequest } from '@/api/aiSafety';

const loading = ref(false);
const loadError = ref(false);
const items = ref<EvidenceRequest[]>([]);
const total = ref(0);
const page = ref(1);
const size = 20;
const filterStatus = ref('');

const actingId = ref(0);
const rejectVisible = ref(false);
const rejectForm = ref({ id: 0, note: '' });

function fmt(v?: string | null) {
  if (!v) return '—';
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return String(v);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
function statusType(s: string) {
  return s === 'approved' ? 'success' : s === 'rejected' ? 'danger' : 'warning';
}
function statusLabel(s: string) {
  return s === 'approved' ? '已批准' : s === 'rejected' ? '已驳回' : '待审';
}

async function loadList() {
  loading.value = true;
  loadError.value = false;
  try {
    const res = await aiSafetyApi.evidenceRequestList({
      page: page.value,
      page_size: size,
      status: filterStatus.value || undefined,
    });
    if (res.data.code === 0 && res.data.data) {
      items.value = res.data.data.items;
      total.value = res.data.data.total;
    }
  } catch {
    loadError.value = true;
  } finally {
    loading.value = false;
  }
}

async function doReview(row: EvidenceRequest, approve: boolean) {
  actingId.value = row.id;
  try {
    await aiSafetyApi.evidenceRequestReview(row.id, approve, '');
    ElMessage.success(approve ? '已批准' : '已驳回');
    await loadList();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '处理失败');
  } finally {
    actingId.value = false;
  }
}
function openReject(row: EvidenceRequest) {
  rejectForm.value = { id: row.id, note: '' };
  rejectVisible.value = true;
}
async function doReject() {
  if (!rejectForm.value.note.trim()) {
    ElMessage.warning('驳回须填写理由');
    return;
  }
  actingId.value = rejectForm.value.id;
  try {
    await aiSafetyApi.evidenceRequestReview(rejectForm.value.id, false, rejectForm.value.note.trim());
    ElMessage.success('已驳回');
    rejectVisible.value = false;
    await loadList();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '驳回失败');
  } finally {
    actingId.value = false;
  }
}

onMounted(loadList);
</script>

<style scoped>
.pr-toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-bottom: 14px;
}
.pr-pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}
</style>
