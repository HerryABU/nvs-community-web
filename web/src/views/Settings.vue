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
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { authApi } from '@/api/auth';
import { ElMessage, type FormInstance, type FormRules } from 'element-plus';
import { Lock, User } from '@element-plus/icons-vue';

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
