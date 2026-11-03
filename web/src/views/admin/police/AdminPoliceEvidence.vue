<template>
  <div class="admin-page">
    <AdminHeader
      title="取证申请 · 以疏代堵"
      subtitle="向站长申请调取指定用户的取证包；批准后由本台一键导出（176号令合规）"
      :icon="Stamp"
    >
      <template #actions>
        <el-button size="small" :loading="loading" @click="loadAll">刷新</el-button>
      </template>
    </AdminHeader>

    <el-alert
      type="warning"
      :closable="false"
      show-icon
      title="权限说明"
      description="警方取证须经站长审批：先在此提交「目标用户 + 取证理由」，站长批准后，本台才能导出该用户的取证包。越权或未经批准的请求会被拒绝并记审计。"
      style="margin-bottom: 16px"
    />

    <!-- 发起申请 -->
    <div class="admin-panel">
      <SectionHeader title="发起取证申请" subtitle="填写目标用户与取证理由，提交后等待站长审批" :icon="'📝'" gradient="danger" />
      <el-form :inline="true" class="pe-form">
        <el-form-item label="目标用户 ID">
          <el-input v-model="createForm.target_user_id" type="number" :min="1" placeholder="数字用户 ID" style="width: 180px" />
        </el-form-item>
        <el-form-item label="取证理由">
          <el-input v-model="createForm.reason" type="textarea" :rows="1" placeholder="依法调取的事由 / 案件关联" style="width: 360px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="creating" @click="doCreate">提交申请</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 我的申请 -->
    <div class="admin-panel">
      <SectionHeader title="我的申请" subtitle="申请状态：待审 / 已批准 / 已驳回" :icon="'📂'" gradient="blue" />
      <StateBlock v-if="loading" state="loading" skeleton="table" :rows="4" />
      <StateBlock v-else-if="loadError" state="error" title="加载失败" description="请稍后重试" action-text="重试" @action="loadList" />
      <template v-else>
        <el-table :data="items" stripe border style="width: 100%">
          <el-table-column prop="no" label="申请编号" width="170" />
          <el-table-column prop="target_user_id" label="目标用户ID" width="100" />
          <el-table-column prop="target_name" label="目标用户名" min-width="120" />
          <el-table-column prop="reason" label="理由" min-width="200" show-overflow-tooltip />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="statusType(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="review_note" label="审批意见" min-width="160" show-overflow-tooltip />
          <el-table-column prop="created_at" label="提交时间" width="150">
            <template #default="{ row }">{{ fmt(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="140" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="row.status === 'approved'"
                size="small"
                type="success"
                :loading="exportingId === row.id"
                @click="doExport(row)"
              >
                导出取证包
              </el-button>
              <span v-else-if="row.status === 'pending'" class="muted small">审批中</span>
              <span v-else class="muted small">已驳回</span>
            </template>
          </el-table-column>
        </el-table>

        <div class="pe-pager">
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
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { Stamp } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import StateBlock from '@/components/common/StateBlock.vue';
import { aiSafetyApi, type EvidenceRequest } from '@/api/aiSafety';

const loading = ref(false);
const loadError = ref(false);
const items = ref<EvidenceRequest[]>([]);
const total = ref(0);
const page = ref(1);
const size = 20;

const creating = ref(false);
const createForm = ref({ target_user_id: '', reason: '' });

const exportingId = ref(0);

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

function triggerDownload(blob: Blob, filename: string) {
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  window.URL.revokeObjectURL(url);
}

async function loadList() {
  loading.value = true;
  loadError.value = false;
  try {
    const res = await aiSafetyApi.evidenceRequestMy({ page: page.value, page_size: size });
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
async function loadAll() {
  await loadList();
}

async function doCreate() {
  const uid = Number(createForm.value.target_user_id);
  if (!uid || uid <= 0) {
    ElMessage.warning('请填写有效的目标用户 ID');
    return;
  }
  if (!createForm.value.reason.trim()) {
    ElMessage.warning('请填写取证理由');
    return;
  }
  creating.value = true;
  try {
    await aiSafetyApi.evidenceRequestCreate({ target_user_id: uid, reason: createForm.value.reason.trim() });
    ElMessage.success('已提交，等待站长审批');
    createForm.value = { target_user_id: '', reason: '' };
    await loadList();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败');
  } finally {
    creating.value = false;
  }
}

async function doExport(row: EvidenceRequest) {
  exportingId.value = row.id;
  try {
    const res = await aiSafetyApi.exportUserForensics(row.target_user_id);
    const blob = new Blob([res.data as Blob], { type: 'application/zip' });
    const ts = new Date().toISOString().replace(/[-:T]/g, '').slice(0, 15);
    triggerDownload(blob, `supervise_forensics_${row.target_user_id}_${ts}.zip`);
    ElMessage.success('取证包已下载');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '导出失败');
  } finally {
    exportingId.value = false;
  }
}

onMounted(loadAll);
</script>

<style scoped>
.pe-form {
  align-items: flex-start;
}
.pe-pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}
</style>
