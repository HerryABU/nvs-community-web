<template>
  <div class="admin-page">
    <AdminHeader title="用户管理" subtitle="搜索 · 编辑 · 禁言 · 封禁" :icon="User">
      <template #actions>
        <div class="um-actions">
          <el-input
            v-model="search"
            placeholder="搜索用户名/昵称/邮箱"
            clearable
            style="width: 260px"
            @clear="doSearch"
            @keyup.enter="doSearch"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="roleFilter" placeholder="角色分类" clearable style="width: 140px" @change="doSearch">
            <el-option label="全部角色" value="" />
            <el-option label="读者" value="reader" />
            <el-option label="作者" value="author" />
            <el-option label="VIP作者" value="vip_author" />
            <el-option label="仲裁员" value="arbitrator" />
            <el-option label="警方" value="police" />
            <el-option label="管理员" value="admin" />
          </el-select>
          <el-select v-model="statusFilter" placeholder="状态" clearable style="width: 130px" @change="doSearch">
            <el-option label="全部状态" value="" />
            <el-option label="正常" value="active" />
            <el-option label="已禁言" value="muted" />
            <el-option label="已封禁" value="banned" />
          </el-select>
          <el-button type="primary" @click="doSearch" :loading="loading">搜索</el-button>
          <el-button @click="$router.push('/admin')">返回后台</el-button>
        </div>
      </template>
    </AdminHeader>

    <div class="admin-page-body admin-panel">
      <!-- 首次加载用骨架屏（内容高度不跳变）；翻页时 users 非空，走表格 + v-loading -->
      <StateBlock v-if="loading && users.length === 0" state="loading" skeleton="table" :rows="6" />

      <!-- 错误态：给出可执行的下一步，而不是只弹一条一闪而过的提示 -->
      <StateBlock
        v-else-if="loadError"
        state="error"
        title="用户列表加载失败"
        description="可能是网络波动或登录状态过期。点重试即可；连续失败请到「健康检查」查看后端状态。"
        @retry="loadUsers"
      />

      <!-- 空态：区分「筛选无结果」与「站点真的没有用户」，并给出对应动作 -->
      <StateBlock
        v-else-if="users.length === 0"
        state="empty"
        :title="hasFilter ? '没有匹配的用户' : '站点还没有注册用户'"
        :description="hasFilter ? '当前搜索/筛选条件下没有结果，放宽条件再试。' : '用户注册后会出现在这里。'"
        :action-text="hasFilter ? '清空筛选' : ''"
        @action="resetFilters"
      />

      <template v-else>
      <el-table :data="users" v-loading="loading" stripe style="width: 100%">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="用户名" width="140" />
      <el-table-column prop="nickname" label="昵称" width="140" />
      <el-table-column prop="email" label="邮箱" min-width="200" />
      <el-table-column prop="role" label="角色" width="120">
        <template #default="{ row }">
          <el-tag :type="roleTagType(row.role)" size="small">{{ roleLabel(row.role) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="注册时间" width="170">
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="禁言" width="100">
        <template #default="{ row }">
          <el-tag v-if="isMuted(row)" type="danger" size="small">已禁言</el-tag>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="340" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="primary" plain @click="openEdit(row)">编辑</el-button>
          <el-button v-if="!isMuted(row)" size="small" type="warning" plain @click="openMute(row)">禁言</el-button>
          <el-button v-else size="small" type="success" plain @click="doUnmute(row)">解禁</el-button>
          <el-popconfirm
            title="确定要封禁该用户吗？将软删除并留证到 softdel 表（可追溯）。"
            confirm-button-text="封禁"
            cancel-button-text="取消"
            @confirm="doDelete(row.id)"
          >
            <template #reference>
              <el-button size="small" type="danger" plain>封禁</el-button>
            </template>
          </el-popconfirm>
          <el-button v-if="row.role !== 'police' && row.role !== 'admin'" size="small" type="danger" plain @click="doPromote(row)">提权警方</el-button>
          <el-button v-else-if="row.role === 'police'" size="small" type="info" plain @click="doRevoke(row)">撤销警方</el-button>
        </template>
      </el-table-column>
    </el-table>

      <div class="um-pagination" v-if="total > 20">
        <el-pagination
          v-model:current-page="page"
          :page-size="20"
          :total="total"
          layout="prev, pager, next, total"
          @current-change="loadUsers"
        />
      </div>
      </template>
    </div>

    <!-- 编辑弹窗 -->
    <el-dialog v-model="editVisible" title="编辑用户" width="460px">
      <el-form :model="editForm" label-width="80px" v-if="editForm">
        <el-form-item label="用户名"><el-input :model-value="editForm.username" disabled /></el-form-item>
        <el-form-item label="昵称"><el-input v-model="editForm.nickname" /></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="editForm.email" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="editForm.role">
            <el-option label="读者 (reader)" value="reader" />
            <el-option label="作者 (author)" value="author" />
            <el-option label="VIP作者 (vip_author)" value="vip_author" />
            <el-option label="仲裁员 (arbitrator)" value="arbitrator" />
            <el-option label="管理员 (admin)" value="admin" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="doSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { User } from '@element-plus/icons-vue';

interface UserRow {
  id: number;
  username: string;
  nickname: string;
  email: string;
  role: string;
  created_at: string;
  muted_until?: string;
  mute_reason?: string;
}

const users = ref<UserRow[]>([]);
const total = ref(0);
const page = ref(1);
const search = ref('');
const roleFilter = ref('');
const statusFilter = ref('');
const loading = ref(false);
/** 加载失败：交给 StateBlock 呈现（重试按钮比一闪而过的 toast 有用） */
const loadError = ref(false);

/** 是否有生效的筛选：决定空态该说「没匹配」还是「站点没用户」 */
const hasFilter = computed(() => !!(search.value || roleFilter.value || statusFilter.value));
const saving = ref(false);

const editVisible = ref(false);
const editForm = ref<UserRow | null>(null);

function formatDate(d?: string) {
  if (!d) return '';
  return new Date(d).toLocaleString('zh-CN');
}

function roleLabel(role: string) {
  const m: Record<string, string> = {
    reader: '读者', author: '作者', vip_author: 'VIP作者',
    arbitrator: '仲裁员', admin: '管理员', police: '警方',
  };
  return m[role] || role;
}

function roleTagType(role: string) {
  const m: Record<string, string> = {
    reader: '', author: 'success', vip_author: 'warning',
    arbitrator: 'info', admin: 'danger', police: 'danger',
  };
  return m[role] || '';
}

async function loadUsers() {
  loading.value = true;
  loadError.value = false;
  try {
    const res = await adminApi.getUsers({
      page: page.value,
      search: search.value || undefined,
      role: roleFilter.value || undefined,
      status: statusFilter.value || undefined,
    });
    if (res.data.code === 0) {
      users.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
    }
  } catch {
    // 不弹 toast：页面上已经有错误态和重试按钮，再弹一条只是噪音
    loadError.value = true;
  } finally {
    loading.value = false;
  }
}

/** 空态里的「清空筛选」：把条件复位再查，避免用户自己一个个清 */
function resetFilters() {
  search.value = '';
  roleFilter.value = '';
  statusFilter.value = '';
  page.value = 1;
  loadUsers();
}

function doSearch() {
  page.value = 1;
  loadUsers();
}

function openEdit(row: UserRow) {
  editForm.value = { ...row };
  editVisible.value = true;
}

async function doSave() {
  if (!editForm.value) return;
  saving.value = true;
  try {
    await adminApi.updateUser(editForm.value.id, {
      role: editForm.value.role,
      nickname: editForm.value.nickname,
      email: editForm.value.email,
    });
    ElMessage.success('保存成功');
    editVisible.value = false;
    loadUsers();
  } catch {
    ElMessage.error('保存失败');
  } finally {
    saving.value = false;
  }
}

async function doDelete(id: number) {
  try {
    await adminApi.deleteUser(id);
    ElMessage.success('已封禁（软删除并留证）');
    loadUsers();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

// 禁言
function isMuted(row: UserRow) {
  return !!row.muted_until && new Date(row.muted_until).getTime() > Date.now();
}

async function openMute(row: UserRow) {
  try {
    const { value } = await ElMessageBox.prompt(
      `对用户「${row.nickname || row.username}」禁言。请输入时长（小时，0=永久）：`,
      '全局禁言',
      { confirmButtonText: '确认禁言', cancelButtonText: '取消', inputValue: '24', inputPattern: /^\d+$/, inputErrorMessage: '请输入数字（小时）' }
    );
    const hours = parseInt(value || '24', 10);
    await adminApi.muteUser(row.id, { duration_hours: hours, reason: '违规行为' });
    ElMessage.success('已禁言');
    loadUsers();
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function doUnmute(row: UserRow) {
  try {
    await adminApi.unmuteUser(row.id);
    ElMessage.success('已解除禁言');
    loadUsers();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

// 提权为警方取管台账号（不可自注册，只能由管理员提权）
async function doPromote(row: UserRow) {
  try {
    await ElMessageBox.confirm(
      `确认将用户「${row.nickname || row.username}」提权为警方取管台账号？该账号将可查看软删除留证并下载取证凭证。`,
      '提权为警方',
      { confirmButtonText: '确认提权', cancelButtonText: '取消', type: 'warning' }
    );
    await adminApi.promotePolice(row.id);
    ElMessage.success('已提权为警方取管台账号');
    loadUsers();
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

// 撤销警方取管台权限（回退为读者）
async function doRevoke(row: UserRow) {
  try {
    await ElMessageBox.confirm(
      `确认撤销用户「${row.nickname || row.username}」的警方取管台权限？`,
      '撤销警方权限',
      { confirmButtonText: '确认撤销', cancelButtonText: '取消', type: 'warning' }
    );
    await adminApi.revokePolice(row.id);
    ElMessage.success('已撤销警方权限');
    loadUsers();
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

onMounted(() => {
  loadUsers();
});
</script>

<style scoped>
.admin-page { --admin-content-max: 1280px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: 1280px; }
.um-actions { display: flex; align-items: center; gap: 8px; }
.um-pagination { margin-top: 16px; display: flex; justify-content: flex-end; }
.muted { color: var(--text-light); }
[data-theme="dark"] .admin-page { background: #0f172a; }

/* ====== 移动端用户管理适配 ====== */
@media (max-width: 768px) {
  .um-actions {
    flex-wrap: wrap;
    width: 100%;
  }
  .um-actions .el-input {
    flex: 1;
    min-width: 140px;
  }
  /* 表格横向滚动 */
  .admin-page-body > .el-table {
    display: block;
    overflow-x: auto;
    min-width: 640px;
  }
  .um-pagination .el-pagination {
    flex-wrap: wrap;
    justify-content: center;
  }
}
</style>