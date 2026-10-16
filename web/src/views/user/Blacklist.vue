<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2>🚫 黑名单管理</h2>
        <p class="subtitle">
          拉黑后：选择「禁止评论」可阻止对方在你的评论区发言；选择「屏蔽内容」则不在评论区展示其内容。
          站长 / 警方 / 管理员不可被拉黑。
        </p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openAdd">添加黑名单</el-button>
    </div>

    <div v-loading="loading" class="blacklist-body">
      <el-empty v-if="!loading && list.length === 0" description="黑名单为空，还没有拉黑任何用户" />
      <el-table v-else :data="list" style="width: 100%">
        <el-table-column label="对方用户" min-width="200">
          <template #default="{ row }">
            <div class="user-cell">
              <el-avatar :size="32" :src="row.blocked_avatar_url">
                {{ (row.blocked_nickname || row.blocked_username || '?')[0] }}
              </el-avatar>
              <div class="user-meta">
                <span class="user-name">{{ row.blocked_nickname || row.blocked_username }}</span>
                <span class="user-sub">@{{ row.blocked_username || `#${row.blocked_user_id}` }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="模式" width="120">
          <template #default="{ row }">
            <el-tag size="small" :type="row.mode === 'hide' ? 'warning' : 'danger'">
              {{ modeLabel(row.mode) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="理由" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.reason">{{ row.reason }}</span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="拉黑时间" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="danger" plain :loading="removingId === row.id" @click="remove(row)">
              解除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination" v-if="total > pageSize">
        <el-pagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          @current-change="load"
        />
      </div>
    </div>

    <!-- 添加黑名单对话框 -->
    <el-dialog v-model="addVisible" title="添加黑名单" width="440px" @closed="resetForm">
      <el-form label-position="top">
        <el-form-item label="用户名或用户 ID" required>
          <el-input
            v-model="form.userKey"
            placeholder="输入对方用户名或数字 ID"
            maxlength="64"
            clearable
            @keyup.enter="submitAdd"
          />
        </el-form-item>
        <el-form-item label="模式" required>
          <el-radio-group v-model="form.mode">
            <el-radio value="block">禁止评论</el-radio>
            <el-radio value="hide">屏蔽内容</el-radio>
          </el-radio-group>
          <div class="mode-hint">
            {{ form.mode === 'hide' ? '不在评论区展示对方的内容与互动' : '阻止对方在你的评论区发表评论' }}
          </div>
        </el-form-item>
        <el-form-item label="理由（选填）">
          <el-input
            v-model="form.reason"
            type="textarea"
            :rows="2"
            maxlength="256"
            show-word-limit
            placeholder="记录拉黑原因，仅自己可见"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitAdd">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { Plus } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { blacklistApi, type BlacklistItem, type BlacklistMode } from '@/api/blacklist';

const MAX_REASON = 256; // 与后端 gorm size:256 / utils.MaxReason 一致

const list = ref<BlacklistItem[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;
const loading = ref(false);

const addVisible = ref(false);
const submitting = ref(false);
const removingId = ref(0);
const form = reactive<{ userKey: string; mode: BlacklistMode; reason: string }>({
  userKey: '',
  mode: 'block',
  reason: '',
});

function modeLabel(mode: BlacklistMode) {
  return mode === 'hide' ? '屏蔽内容' : '禁止评论';
}

function formatTime(d: string) {
  return d ? new Date(d).toLocaleString('zh-CN') : '';
}

async function load() {
  loading.value = true;
  try {
    const res = await blacklistApi.list({ page: page.value, page_size: pageSize });
    if (res.data.code === 0) {
      list.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
    }
  } catch {
    /* ignore */
  } finally {
    loading.value = false;
  }
}

function openAdd() {
  resetForm();
  addVisible.value = true;
}

function resetForm() {
  form.userKey = '';
  form.mode = 'block';
  form.reason = '';
}

async function submitAdd() {
  const key = form.userKey.trim();
  if (!key) {
    ElMessage.warning('请输入要拉黑的用户名或用户 ID');
    return;
  }
  if (form.reason.length > MAX_REASON) {
    form.reason = form.reason.slice(0, MAX_REASON);
  }
  submitting.value = true;
  try {
    await blacklistApi.add({
      user_key: key,
      mode: form.mode,
      reason: form.reason.trim(),
    });
    ElMessage.success('已加入黑名单');
    addVisible.value = false;
    page.value = 1;
    load();
  } catch {
    /* 拦截器已输出错误信息 */
  } finally {
    submitting.value = false;
  }
}

async function remove(row: BlacklistItem) {
  try {
    await ElMessageBox.confirm(
      `确定解除对「${row.blocked_nickname || row.blocked_username}」的拉黑吗？`,
      '解除拉黑',
      { type: 'warning', confirmButtonText: '解除', cancelButtonText: '取消' }
    );
  } catch {
    return; // 用户取消
  }
  removingId.value = row.id;
  try {
    await blacklistApi.remove(row.id);
    ElMessage.success('已解除拉黑');
    // 当前页删空且非首页时回退一页，避免看到空白列表
    if (list.value.length === 1 && page.value > 1) {
      page.value -= 1;
    }
    load();
  } catch {
    /* ignore */
  } finally {
    removingId.value = 0;
  }
}

onMounted(load);
</script>

<style scoped>
.page-container { max-width: 960px; margin: 0 auto; padding: 24px 20px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px; flex-wrap: wrap; gap: 12px; }
.page-header h2 { margin: 0; font-size: 1.4rem; }
.subtitle { color: var(--text-secondary); margin: 4px 0 0; font-size: 0.85rem; max-width: 560px; }
.blacklist-body { min-height: 200px; }
.user-cell { display: flex; align-items: center; gap: 10px; }
.user-meta { display: flex; flex-direction: column; line-height: 1.3; min-width: 0; }
.user-name { font-weight: 600; font-size: 0.9rem; }
.user-sub { color: var(--text-secondary); font-size: 0.75rem; }
.muted { color: var(--text-secondary); }
.mode-hint { color: var(--text-secondary); font-size: 0.78rem; margin-top: 2px; }
.pagination { margin-top: 20px; display: flex; justify-content: center; }

/* ====== 移动端黑名单页适配 ====== */
@media (max-width: 768px) {
  .page-header { flex-wrap: wrap; gap: 8px; }
  .blacklist-body { overflow-x: auto; }
  .blacklist-body .el-table { min-width: 640px; }
}
</style>
