<template>
  <div class="ap-wrap">
    <el-card shadow="never">
      <template #header>
        <div class="ap-head">
          <div>
            <div class="ap-title">自托管项目管理与审核</div>
            <div class="ap-sub">先审源码与可执行文件，通过后作者才能申请公开与反向代理路由。</div>
          </div>
          <div class="ap-tools">
            <el-select v-model="status" clearable placeholder="全部状态" style="width: 140px" @change="load()">
              <el-option label="草稿" value="draft" />
              <el-option label="待审核" value="pending" />
              <el-option label="已通过" value="approved" />
              <el-option label="已驳回" value="rejected" />
              <el-option label="已停止" value="suspended" />
            </el-select>
            <el-input v-model="keyword" placeholder="搜索项目" clearable style="width: 200px" @keyup.enter="load()" />
            <el-button @click="load()">刷新</el-button>
          </div>
        </div>
      </template>

      <el-table :data="projects" v-loading="loading" stripe>
        <el-table-column v-if="!isMobile" label="ID" width="70" prop="id" />
        <el-table-column label="项目" min-width="180">
          <template #default="{ row }">
            <div class="ap-name">{{ row.name }}</div>
            <div class="ap-slug">{{ row.slug }} · {{ row.language }}</div>
          </template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="作者" width="90" prop="user_id" />
        <el-table-column v-if="!isMobile" label="模式" width="100">
          <template #default="{ row }">{{ row.mode === 'compiled' ? '编译版' : '源码' }}</template>
        </el-table-column>
        <el-table-column label="源码" width="120">
          <template #default="{ row }">
            <el-link v-if="row.source_file" type="primary" @click="download(row, 'source')">查看</el-link>
            <span v-else class="ap-mute">未上传</span>
          </template>
        </el-table-column>
        <el-table-column label="可执行文件" width="120">
          <template #default="{ row }">
            <el-link v-if="row.binary_file" type="primary" @click="download(row, 'binary')">查看</el-link>
            <span v-else class="ap-mute">—</span>
          </template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="服务地址" min-width="200" show-overflow-tooltip prop="upstream" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }"><el-tag :type="statusType(row.review_status)" size="small">{{ statusText(row.review_status) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.review_status === 'pending'" link type="primary" @click="openReview(row, true)">通过</el-button>
            <el-button v-if="row.review_status === 'pending'" link type="danger" @click="openReview(row, false)">驳回</el-button>
            <el-button v-if="row.review_status === 'approved'" link type="warning" @click="suspend(row, true)">停止</el-button>
            <el-button v-if="row.review_status === 'suspended'" link type="success" @click="suspend(row, false)">恢复</el-button>
            <el-button link @click="showNote(row)">意见</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && !projects.length" description="没有符合条件的项目" />
    </el-card>

    <el-dialog v-model="reviewVisible" :title="reviewApprove ? '通过审核' : '驳回项目'" :width="isMobile ? '94%' : '520px'" destroy-on-close>
      <el-form label-width="110px">
        <el-form-item label="审核意见"><el-input v-model="reviewNote" type="textarea" :rows="3" /></el-form-item>
        <el-form-item v-if="reviewApprove" label="源码豁免">
          <el-switch v-model="waiveSource" />
          <div class="ap-hint">仅在确认源码无法随项目提供时开启；开启后该项目可免于强制源码要求。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reviewVisible = false">取消</el-button>
        <el-button :type="reviewApprove ? 'primary' : 'danger'" @click="doReview()">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { selfhostApi, type SelfHostProject } from '@/api/selfhost';
import { useIsMobile } from '@/utils/mobile';

const isMobile = useIsMobile();
const loading = ref(false);
const keyword = ref('');
const status = ref('');
const projects = ref<SelfHostProject[]>([]);
const reviewVisible = ref(false);
const reviewApprove = ref(true);
const reviewNote = ref('');
const waiveSource = ref(false);
const currentId = ref(0);

const statusText = (s: string) => ({ draft: '草稿', pending: '待审核', approved: '已通过', rejected: '已驳回', suspended: '已停止' } as Record<string, string>)[s] || s;
const statusType = (s: string) => ({ draft: 'info', pending: 'warning', approved: 'success', rejected: 'danger', suspended: 'danger' } as Record<string, string>)[s] || 'info';

async function load() {
  loading.value = true;
  try {
    projects.value = (await selfhostApi.adminListProjects(status.value, keyword.value)).data.data.projects || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

async function download(row: SelfHostProject, kind: 'source' | 'binary') {
  try {
    await selfhostApi.downloadFile(row.id, kind, kind === 'source' ? row.source_name : row.binary_name);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '下载失败');
  }
}

function openReview(row: SelfHostProject, approve: boolean) {
  currentId.value = row.id;
  reviewApprove.value = approve;
  reviewNote.value = '';
  waiveSource.value = false;
  reviewVisible.value = true;
}

async function doReview() {
  try {
    await selfhostApi.adminReview(currentId.value, reviewApprove.value, reviewNote.value, waiveSource.value);
    reviewVisible.value = false;
    ElMessage.success('审核已处理');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function suspend(row: SelfHostProject, yes: boolean) {
  try {
    await selfhostApi.adminSuspend(row.id, yes, yes ? '管理员停止' : '管理员恢复');
    ElMessage.success('状态已更新');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function showNote(row: SelfHostProject) {
  await ElMessageBox.alert(row.review_note || '（无审核意见）', `${row.name} · 审核意见`);
}

onMounted(load);
</script>

<style scoped>
.ap-wrap { padding: 16px; }
.ap-head { display: flex; justify-content: space-between; align-items: center; gap: 16px; }
.ap-title { font-size: 18px; font-weight: 700; }
.ap-sub { font-size: 12px; color: var(--el-text-color-secondary); }
.ap-tools { display: flex; gap: 8px; }
.ap-name { font-weight: 600; }
.ap-slug, .ap-mute { font-size: 12px; color: var(--el-text-color-secondary); }
.ap-hint { font-size: 12px; color: var(--el-text-color-secondary); }
</style>

@media (max-width: 767px) {
  .ap-wrap { padding: 8px; }
  .ap-head { flex-direction: column; align-items: stretch; gap: 10px; }
  .ap-tools { flex-wrap: wrap; }
  .ap-tools .el-input, .ap-tools .el-select { width: 100% !important; }
}
