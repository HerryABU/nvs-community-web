<template>
  <div class="admin-page">
    <AdminHeader title="论坛板块管理" subtitle="板块增删、审查、排序" :icon="ChatDotRound" />
    <div class="admin-page-body admin-panel">
      <div style="margin-bottom:12px">
        <el-button type="primary" size="small" @click="showAddForum = true">新增板块</el-button>
      </div>
      <el-table :data="adminForums" v-loading="loadingForums" style="width:100%">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" label="板块名称" min-width="140" />
        <el-table-column label="类型" width="110">
          <template #default="{ row }">
            <el-tag size="small" :type="forumTypeTag(row.type)">{{ forumTypeLabel(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="审查" width="100" v-if="hasCustomForums">
          <template #default="{ row }">
            <template v-if="row.type === 'custom'">
              <el-tag size="small" :type="forumReviewTag(row.review_status)">{{ forumReviewLabel(row.review_status) }}</el-tag>
            </template>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="180" show-overflow-tooltip />
        <el-table-column prop="thread_count" label="帖子数" width="80" />
        <el-table-column prop="sort_order" label="排序" width="70" />
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <template v-if="row.type === 'custom' && row.review_status === 'pending'">
              <el-button size="small" type="success" @click="reviewForum(row, 'approve')">通过</el-button>
              <el-button size="small" type="danger" @click="reviewForum(row, 'reject')">驳回</el-button>
            </template>
            <el-button size="small" @click="editForum(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="deleteForum(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 添加/编辑论坛弹窗 -->
    <el-dialog v-model="showAddForum" :title="editingForum ? '编辑板块' : '新增板块'" width="480px">
      <el-form :model="forumForm" label-width="90px">
        <el-form-item label="板块名称" required>
          <el-input v-model="forumForm.name" placeholder="如：创作交流区" maxlength="64" />
        </el-form-item>
        <el-form-item label="板块类型" required>
          <el-select v-model="forumForm.type" style="width:100%">
            <el-option label="综合讨论 (general)" value="general" />
            <el-option label="读者区 (reader)" value="reader" />
            <el-option label="读者-作者 (reader_author)" value="reader_author" />
            <el-option label="作者区 (author)" value="author" />
            <el-option label="敏感区 (sensitive) ⚠" value="sensitive" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="forumForm.type === 'sensitive'" label="绑定分区">
          <el-input v-model="forumForm.zone" placeholder="如：同人区、政治文学区" maxlength="64" />
          <span class="hint-text">填写隔离墙分区名，进入论坛时将触发确认弹窗</span>
        </el-form-item>
        <el-form-item label="排序权重">
          <el-input-number v-model="forumForm.sort_order" :min="0" :max="999" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="forumForm.description" type="textarea" :rows="2" placeholder="可选：板块简介" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddForum = false">取消</el-button>
        <el-button type="primary" @click="submitForum" :loading="submittingForum">
          {{ editingForum ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue';
import { ChatDotRound } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';

const adminForums = ref<any[]>([]);
const loadingForums = ref(false);
const showAddForum = ref(false);
const editingForum = ref<any>(null);
const submittingForum = ref(false);
const forumForm = reactive({ name: '', type: 'general', zone: '', description: '', sort_order: 0 });

async function loadForums() {
  loadingForums.value = true;
  try {
    const res = await adminApi.getForums();
    adminForums.value = res.data.data || [];
  } catch { /* ignore */ }
  loadingForums.value = false;
}

function editForum(forum: any) {
  editingForum.value = forum;
  forumForm.name = forum.name || '';
  forumForm.type = forum.type || 'general';
  forumForm.zone = forum.zone || '';
  forumForm.description = forum.description || '';
  forumForm.sort_order = forum.sort_order || 0;
  showAddForum.value = true;
}

async function submitForum() {
  if (!forumForm.name) { ElMessage.warning('请填写板块名称'); return; }
  submittingForum.value = true;
  try {
    if (editingForum.value) {
      await adminApi.updateForum(editingForum.value.id, {
        name: forumForm.name,
        type: forumForm.type,
        zone: forumForm.zone,
        description: forumForm.description,
        sort_order: forumForm.sort_order,
      });
      ElMessage.success('板块已更新');
    } else {
      await adminApi.createForum({
        name: forumForm.name,
        type: forumForm.type,
        zone: forumForm.zone,
        description: forumForm.description,
        sort_order: forumForm.sort_order,
      });
      ElMessage.success('板块已创建');
    }
    showAddForum.value = false;
    resetForumForm();
    await loadForums();
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '操作失败');
  }
  submittingForum.value = false;
}

function resetForumForm() {
  editingForum.value = null;
  forumForm.name = '';
  forumForm.type = 'general';
  forumForm.zone = '';
  forumForm.description = '';
  forumForm.sort_order = 0;
}

async function deleteForum(forum: any) {
  try {
    await ElMessageBox.confirm(`确定要删除板块「${forum.name}」吗？关联帖子将保留但不可见。`, '确认删除', { type: 'warning' });
    await adminApi.deleteForum(forum.id);
    ElMessage.success('已删除');
    await loadForums();
  } catch { /* cancelled */ }
}

function forumTypeTag(t: string) {
  const map: Record<string, string> = { general: 'info', reader: 'success', reader_author: 'warning', author: 'primary', sensitive: 'danger', custom: 'info' };
  return (map[t] as any) || 'info';
}

function forumTypeLabel(t: string) {
  const map: Record<string, string> = { general: '综合讨论', reader: '读者区', reader_author: '读者-作者', author: '作者区', sensitive: '敏感区', custom: '自建论坛' };
  return map[t] || t;
}

const hasCustomForums = computed(() => adminForums.value.some(f => f.type === 'custom'));
function forumReviewTag(s: string) {
  const map: Record<string, string> = { pending: 'warning', approved: 'success', rejected: 'danger' };
  return (map[s] as any) || 'info';
}
function forumReviewLabel(s: string) {
  const map: Record<string, string> = { pending: '待审查', approved: '已通过', rejected: '已驳回' };
  return map[s] || s;
}
async function reviewForum(row: any, action: 'approve' | 'reject') {
  try {
    await adminApi.reviewForum(row.id, { action });
    ElMessage.success(action === 'approve' ? '已通过' : '已驳回');
    await loadForums();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

onMounted(loadForums);
</script>

<style scoped>
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 1100px; }
.hint-text { font-size: 0.8rem; color: var(--text-light, #999); }
.muted { color: var(--text-light, #999); }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
</style>
