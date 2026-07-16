<template>
  <div class="settings-page">
    <div class="settings-container">
      <h2 class="page-title">账号设置</h2>

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
import { ref, reactive } from 'vue';
import { useRouter } from 'vue-router';
import { authApi } from '@/api/auth';
import { ElMessage, type FormInstance, type FormRules } from 'element-plus';
import { Lock } from '@element-plus/icons-vue';

const router = useRouter();
const formRef = ref<FormInstance>();
const submitting = ref(false);

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
</style>
