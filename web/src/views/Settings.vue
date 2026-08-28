<template>
  <div class="settings-page">
    <div class="settings-container">
      <h2 class="page-title">账号设置</h2>

      <!-- 修改昵称 -->
      <el-card class="settings-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <el-icon><User /></el-icon>
            <span>修改昵称</span>
          </div>
        </template>
        <el-form label-width="100px" label-position="top" @submit.prevent="submitNickname">
          <el-form-item label="当前昵称">
            <el-input :model-value="currentNickname" disabled size="large" />
          </el-form-item>
          <el-form-item label="新昵称">
            <el-input
              v-model="nicknameForm"
              maxlength="32"
              show-word-limit
              placeholder="1-32 个字符，将展示在评论、论坛等位置"
              size="large"
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="large" :loading="nicknameSaving" @click="submitNickname">
              保存昵称
            </el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <!-- 修改密码 -->
      <el-card class="settings-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <el-icon><Lock /></el-icon>
            <span>修改密码</span>
          </div>
        </template>
        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          label-width="100px"
          label-position="top"
          @submit.prevent="handleSubmit"
        >
          <el-form-item label="原密码" prop="oldPassword">
            <el-input
              v-model="form.oldPassword"
              type="password"
              placeholder="请输入当前密码"
              show-password
              size="large"
            />
          </el-form-item>

          <el-form-item label="新密码" prop="newPassword">
            <el-input
              v-model="form.newPassword"
              type="password"
              placeholder="请输入新密码（至少6位）"
              show-password
              size="large"
            />
          </el-form-item>

          <el-form-item label="确认新密码" prop="confirmPassword">
            <el-input
              v-model="form.confirmPassword"
              type="password"
              placeholder="请再次输入新密码"
              show-password
              size="large"
            />
          </el-form-item>

          <el-form-item>
            <el-button
              type="primary"
              size="large"
              :loading="submitting"
              @click="handleSubmit"
            >
              修改密码
            </el-button>
            <el-button size="large" @click="resetForm">重置</el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <!-- 个性化推荐（176号令 第七条第九项：算法推荐选择权） -->
      <el-card class="settings-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <el-icon><DataAnalysis /></el-icon>
            <span>推荐设置</span>
          </div>
        </template>
        <el-form label-width="100px" label-position="top">
          <el-form-item label="个性化推荐">
            <el-switch v-model="personalizedRecommend" @change="savePreference" />
            <div class="hint" style="margin-top: 6px">
              平台默认不基于个人阅读历史进行个性化推荐；此开关保留您的算法选择权（《公安机关网络空间安全监督检查办法》第七条第九项）。
              关闭后，即使平台未来启用个性化推荐，您也不会收到基于阅读历史的推荐内容。
            </div>
          </el-form-item>
        </el-form>
      </el-card>

      <!-- API Key 管理（第三方客户端/阅读器/爬虫合规访问） -->
      <el-card class="settings-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <el-icon><Key /></el-icon>
            <span>API Key 管理</span>
          </div>
        </template>
        <el-alert
          type="info"
          :closable="false"
          show-icon
          style="margin-bottom: 14px"
          :title="`当前身份：${roleLabel} · 本角色 Key 配额上限 ${roleQuota.req} 次 / ${roleQuota.window} 秒`"
        >
          <template #default>
            <div class="hint" style="margin: 4px 0 0">
              创建 API Key 后，调用本站 API 时携带 <code>X-API-Key</code> 请求头即可获得高于匿名访问的配额
              （无 Key 的匿名/爬虫请求会被限流在较低水平）。可自设不超过角色上限的更小配额，随时停用/吊销。
            </div>
          </template>
        </el-alert>

        <!-- 创建 -->
        <el-form label-width="90px" label-position="top" style="margin-bottom: 16px">
          <el-row :gutter="12">
            <el-col :xs="24" :sm="8">
              <el-form-item label="备注名">
                <el-input v-model="createForm.name" placeholder="如：我的阅读器" size="small" />
              </el-form-item>
            </el-col>
            <el-col :xs="12" :sm="5">
              <el-form-item label="配额（次/分）">
                <el-input-number v-model="createForm.quota" :min="1" :max="roleQuota.req" size="small" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="11">
              <el-form-item label="过期时间（可选）">
                <el-date-picker
                  v-model="createForm.expiresAt"
                  type="date"
                  placeholder="永久有效"
                  size="small"
                  style="width: 100%"
                  value-format="YYYY-MM-DD"
                />
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item label="IP 白名单（可选）">
            <el-input v-model="createForm.allowedIps" placeholder="如 1.2.3.4, 10.0.0.0/8（留空 = 不限 IP）" size="small" />
          </el-form-item>
          <el-button type="primary" size="small" @click="createKey" :loading="creatingKey">创建 API Key</el-button>
        </el-form>

        <el-divider style="margin: 8px 0 14px" />
        <el-table :data="myKeys" size="small" v-loading="loadingKeys" style="width: 100%">
          <el-table-column prop="name" label="备注" min-width="100">
            <template #default="{ row }">{{ row.name || '—' }}</template>
          </el-table-column>
          <el-table-column label="Key 前缀" width="160">
            <template #default="{ row }"><code>{{ row.key_prefix }}</code></template>
          </el-table-column>
          <el-table-column label="配额" width="100">
            <template #default="{ row }">{{ row.quota_per_min }} 次/分</template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{ row }">
              <el-tag :type="row.is_active ? 'success' : 'danger'" size="small">{{ row.is_active ? '启用' : '停用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="过期" width="100">
            <template #default="{ row }">{{ row.expires_at ? row.expires_at.slice(0, 10) : '永久' }}</template>
          </el-table-column>
          <el-table-column label="最近使用" width="150">
            <template #default="{ row }">{{ formatTime(row.last_used_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="200" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text @click="toggleKey(row)">{{ row.is_active ? '停用' : '启用' }}</el-button>
              <el-button size="small" text type="warning" @click="regenerateKey(row)">重生成</el-button>
              <el-button size="small" text type="danger" @click="deleteKey(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <p v-if="!myKeys.length && !loadingKeys" class="hint" style="text-align: center; margin-top: 8px">尚未创建 API Key</p>
      </el-card>
    </div>

    <!-- 完整 Key 一次性展示 -->
    <el-dialog v-model="showNewKey" title="API Key 创建成功" width="540px">
      <el-alert type="warning" :closable="false" show-icon title="完整 Key 仅此一次展示，请立即复制保存" style="margin-bottom: 12px" />
      <div class="newkey-box">
        <code class="newkey-code">{{ newKeyValue }}</code>
        <el-button type="primary" @click="copyKey">复制</el-button>
      </div>
      <p class="hint">
        请求时携带 <code>X-API-Key: {{ newKeyPrefix }}…</code>（完整 Key）。服务端仅存储哈希，遗失后只能重新生成。
      </p>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { authApi } from '@/api/auth';
import { apiKeyApi } from '@/api/apikey';
import api from '@/api';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { Lock, User, DataAnalysis, Key } from '@element-plus/icons-vue';

const router = useRouter();
const authStore = useAuthStore();
const formRef = ref<FormInstance>();
const submitting = ref(false);

// ====== 修改昵称 ======
const currentNickname = ref(authStore.user?.nickname || authStore.user?.username || '');
const nicknameForm = ref('');
const nicknameSaving = ref(false);

async function submitNickname() {
  const nick = nicknameForm.value.trim();
  if (!nick) { ElMessage.warning('请输入新昵称'); return; }
  nicknameSaving.value = true;
  try {
    const res = await authApi.updateProfile({ nickname: nick });
    if (res.data.code === 0) {
      if (authStore.user) authStore.user.nickname = nick;
      currentNickname.value = nick;
      nicknameForm.value = '';
      ElMessage.success('昵称已更新');
    } else {
      ElMessage.error(res.data.message || '更新失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '更新失败');
  } finally {
    nicknameSaving.value = false;
  }
}

const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
});

// ====== 个性化推荐开关（176号令 第七条第九项） ======
const personalizedRecommend = ref(false);
async function savePreference() {
  try {
    const res = await api.put('/users/me/preferences', { personalized_recommend: personalizedRecommend.value });
    if (res.data?.code === 0) ElMessage.success('推荐偏好已保存');
    else ElMessage.error(res.data?.message || '保存失败');
  } catch { ElMessage.error('保存失败'); }
}

const validateConfirmPassword = (_rule: any, value: string, callback: (error?: Error) => void) => {
  if (value !== form.newPassword) {
    callback(new Error('两次输入的密码不一致'));
  } else {
    callback();
  }
};

const rules: FormRules = {
  oldPassword: [
    { required: true, message: '请输入原密码', trigger: 'blur' },
  ],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '新密码长度至少6位', trigger: 'blur' },
    { max: 128, message: '新密码长度不能超过128位', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' },
  ],
};

async function handleSubmit() {
  if (!formRef.value) return;

  const valid = await formRef.value.validate().catch(() => false);
  if (!valid) return;

  submitting.value = true;
  try {
    const res = await authApi.changePassword(form.oldPassword, form.newPassword);
    if (res.data.code === 0) {
      ElMessage.success('密码修改成功，请重新登录');
      resetForm();
      // 修改密码成功后跳转到登录页
      setTimeout(() => {
        router.push('/login');
      }, 1500);
    } else {
      ElMessage.error(res.data.message || '修改失败');
    }
  } catch (err: any) {
    const msg = err?.response?.data?.message || '修改密码失败，请稍后再试';
    ElMessage.error(msg);
  } finally {
    submitting.value = false;
  }
}

function resetForm() {
  formRef.value?.resetFields();
  form.oldPassword = '';
  form.newPassword = '';
  form.confirmPassword = '';
}

// ====== API Key 管理 ======
const myKeys = ref<any[]>([]);
const loadingKeys = ref(false);
const roleLabel = ref('读者');
const roleQuota = reactive({ req: 60, window: 60 });
const createForm = reactive({ name: '', quota: 0, allowedIps: '', expiresAt: '' });
const creatingKey = ref(false);
const showNewKey = ref(false);
const newKeyValue = ref('');
const newKeyPrefix = ref('');

function formatTime(s?: string) {
  if (!s) return '—';
  return new Date(s).toLocaleString('zh-CN');
}

async function loadMyKeys() {
  loadingKeys.value = true;
  try {
    const res = await apiKeyApi.list();
    const d = res.data.data || {};
    myKeys.value = d.list || [];
    const rl: Record<string, string> = { admin: '站长', author: '作者', reader: '读者' };
    roleLabel.value = rl[d.role] || '读者';
    roleQuota.req = d.role_quota?.req || 60;
    roleQuota.window = d.role_quota?.window || 60;
  } catch { /* ignore */ }
  loadingKeys.value = false;
}

async function createKey() {
  if (createForm.quota < 1) createForm.quota = roleQuota.req;
  creatingKey.value = true;
  try {
    const res = await apiKeyApi.create({
      name: createForm.name,
      quota_per_min: createForm.quota,
      allowed_ips: createForm.allowedIps,
      expires_at: createForm.expiresAt || undefined,
    });
    if (res.data.code === 0) {
      newKeyValue.value = res.data.data.key || '';
      newKeyPrefix.value = res.data.data.key_prefix || '';
      showNewKey.value = true;
      ElMessage.success('API Key 已创建');
      createForm.name = '';
      createForm.allowedIps = '';
      createForm.expiresAt = '';
      await loadMyKeys();
    } else {
      ElMessage.error(res.data.message || '创建失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '创建失败');
  }
  creatingKey.value = false;
}

async function copyKey() {
  try {
    await navigator.clipboard.writeText(newKeyValue.value);
    ElMessage.success('已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择复制');
  }
}

async function toggleKey(row: any) {
  try {
    await apiKeyApi.update(row.id, { is_active: !row.is_active });
    ElMessage.success('已更新状态');
    await loadMyKeys();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function regenerateKey(row: any) {
  try {
    await ElMessageBox.confirm('重新生成后旧 Key 立即失效，确定继续？', '重新生成', { type: 'warning' });
    const res = await apiKeyApi.regenerate(row.id);
    if (res.data.code === 0) {
      newKeyValue.value = res.data.data.key || '';
      newKeyPrefix.value = res.data.data.key_prefix || '';
      showNewKey.value = true;
      ElMessage.success('已重新生成');
      await loadMyKeys();
    }
  } catch { /* cancelled */ }
}

async function deleteKey(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除 API Key「${row.key_prefix}」吗？删除后立即失效。`, '删除 API Key', { type: 'warning' });
    await apiKeyApi.remove(row.id);
    ElMessage.success('已删除');
    await loadMyKeys();
  } catch { /* cancelled */ }
}

onMounted(async () => {
  try {
    const res = await api.get('/users/me/preferences');
    if (res.data?.code === 0) personalizedRecommend.value = !!res.data.data?.personalized_recommend;
  } catch { /* ignore */ }
  loadMyKeys();
});
</script>

<style scoped>
.settings-page {
  min-height: 100vh;
  background: var(--bg-color);
  padding: 40px 20px;
}

.settings-container {
  max-width: 520px;
  margin: 0 auto;
}

.page-title {
  font-size: 1.5rem;
  color: var(--text-primary);
  margin-bottom: 24px;
  text-align: center;
}

.settings-card {
  border-radius: 12px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
}

:deep(.el-form-item__label) {
  color: var(--text-secondary);
}

.settings-container code {
  background: #eef2ff;
  color: #4f46e5;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 0.82rem;
}
.newkey-box {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.newkey-code {
  flex: 1;
  font-size: 0.95rem;
  word-break: break-all;
  background: #eef2ff;
  color: #4f46e5;
  padding: 10px 12px;
  border-radius: 6px;
}
.hint {
  color: var(--text-light, #666);
  font-size: 0.85rem;
  line-height: 1.6;
}
:deep(.el-table) {
  font-size: 0.85rem;
}

/* ====== 移动端账号设置适配 ====== */
@media (max-width: 640px) {
  .settings-page {
    padding: 20px 12px;
  }
  .page-title {
    font-size: 1.25rem;
    margin-bottom: 18px;
  }
  .card-header {
    font-size: 1rem;
  }
}
</style>
