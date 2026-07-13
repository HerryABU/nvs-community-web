<template>
  <div class="page-container">
    <div class="um-header">
      <h2>用户管理</h2>
      <div class="um-actions">
        <el-input
          v-model="search"
          placeholder="搜索用户名/昵称/邮箱"
          clearable
          style="width: 280px"
          @clear="doSearch"
          @keyup.enter="doSearch"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button type="primary" @click="doSearch" :loading="loading">搜索</el-button>
        <el-button @click="$router.push('/admin')">返回后台</el-button>
      </div>
    </div>

    <el-table :data="users" v-loading="loading" stripe style="width: 100%; margin-top: 16px">
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
      <el-table-column label="操作" width="260" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="primary" plain @click="openEdit(row)">编辑</el-button>
          <el-popconfirm
            title="确定要删除该用户吗？此操作不可恢复。"
            confirm-button-text="删除"
            cancel-button-text="取消"
            @confirm="doDelete(row.id)"
          >
            <template #reference>
              <el-button size="small" type="danger" plain>删除</el-button>
            </template>
          </el-popconfirm>
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
import { ref, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

interface UserRow {
  id: number;
  username: string;
  nickname: string;
  email: string;
  role: string;
  created_at: string;
}

const users = ref<UserRow[]>([]);
const total = ref(0);
const page = ref(1);
const search = ref('');
const loading = ref(false);
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
    arbitrator: '仲裁员', admin: '管理员',
  };
  return m[role] || role;
}

function roleTagType(role: string) {
  const m: Record<string, string> = {
    reader: '', author: 'success', vip_author: 'warning',
    arbitrator: 'info', admin: 'danger',
  };
  return m[role] || '';
}

async function loadUsers() {
  loading.value = true;
  try {
    const res = await adminApi.getUsers(page.value, search.value || undefined);
    if (res.data.code === 0) {
      users.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
    }
  } catch {
    ElMessage.error('加载失败');
  } finally {
    loading.value = false;
  }
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
    ElMessage.success('已删除');
    loadUsers();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

onMounted(() => {
  loadUsers();
});
</script>

<style scoped>
.page-container { max-width: 1100px; margin: 0 auto; padding: 24px; }
.um-header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; }
.um-header h2 { margin: 0; }
.um-actions { display: flex; align-items: center; gap: 8px; }
.um-pagination { margin-top: 16px; display: flex; justify-content: center; }
</style>