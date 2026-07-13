<template>
  <div class="auth-page">
    <el-card class="auth-card">
      <template #header>
        <h2 class="auth-title">登录</h2>
      </template>
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        @submit.prevent="handleLogin"
      >
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" placeholder="请输入邮箱" prefix-icon="Message" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="请输入密码"
            show-password
            prefix-icon="Lock"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" native-type="submit" :loading="submitting" style="width: 100%">
            登录
          </el-button>
        </el-form-item>
      </el-form>
      <div class="auth-footer">
        还没有账号？<router-link to="/register">立即注册</router-link>
        <span style="margin: 0 8px; color: #ccc;">|</span>
        <a href="#" @click.prevent="showForgot = true">忘记密码？</a>
      </div>
    </el-card>

    <!-- 忘记密码弹窗 -->
    <el-dialog v-model="showForgot" title="重置密码" width="400px" :close-on-click-modal="false">
      <el-form ref="forgotFormRef" :model="forgotForm" :rules="forgotRules" label-position="top">
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="forgotForm.email" placeholder="请输入注册邮箱" />
        </el-form-item>

        <el-form-item v-if="forgotStep === 2" label="验证码" prop="code">
          <el-input v-model="forgotForm.code" placeholder="请输入邮件中的验证码" />
        </el-form-item>

        <el-form-item v-if="forgotStep === 2" label="新密码" prop="newPassword">
          <el-input v-model="forgotForm.newPassword" type="password" placeholder="请输入新密码（至少6位）" show-password />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showForgot = false">取消</el-button>
        <el-button
          v-if="forgotStep === 1"
          type="primary"
          :loading="forgotLoading"
          :disabled="forgotCooldown > 0"
          @click="sendResetCode"
        >
          {{ forgotCooldown > 0 ? `${forgotCooldown}s 后重试` : '发送验证码' }}
        </el-button>
        <el-button
          v-if="forgotStep === 2"
          type="primary"
          :loading="forgotLoading"
          @click="doResetPassword"
        >
          重置密码
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { authApi } from '@/api/auth';
import { ElMessage } from 'element-plus';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();

const formRef = ref();
const submitting = ref(false);

const form = reactive({
  email: '',
  password: '',
});

const rules = {
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' },
  ],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
};

async function handleLogin() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;

  submitting.value = true;
  try {
    const res = await authStore.login(form.email, form.password);
    if (res.code === 0) {
      ElMessage.success('登录成功');
      const redirect = route.query.redirect as string;
      router.push(redirect || '/');
    } else {
      ElMessage.error(res.message || '登录失败');
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '登录失败，请检查邮箱和密码');
  } finally {
    submitting.value = false;
  }
}

// ============ 忘记密码 ============
const showForgot = ref(false);
const forgotStep = ref(1); // 1: 输入邮箱, 2: 输入验证码和新密码
const forgotLoading = ref(false);
const forgotCooldown = ref(0);
const forgotFormRef = ref();

const forgotForm = reactive({
  email: '',
  code: '',
  newPassword: '',
});

const forgotRules = {
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' },
  ],
  code: [{ required: true, message: '请输入验证码', trigger: 'blur' }],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' },
  ],
};

async function sendResetCode() {
  const valid = await forgotFormRef.value?.validateField('email').catch(() => false);
  if (!valid) return;

  forgotLoading.value = true;
  try {
    await authApi.forgotPassword(forgotForm.email);
    ElMessage.success('验证码已发送，请查收邮件');
    forgotStep.value = 2;
    // 60秒冷却
    forgotCooldown.value = 60;
    const timer = setInterval(() => {
      forgotCooldown.value--;
      if (forgotCooldown.value <= 0) clearInterval(timer);
    }, 1000);
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '发送失败');
  } finally {
    forgotLoading.value = false;
  }
}

async function doResetPassword() {
  const valid = await forgotFormRef.value?.validate().catch(() => false);
  if (!valid) return;

  forgotLoading.value = true;
  try {
    const res = await authApi.resetPassword(forgotForm.email, forgotForm.code, forgotForm.newPassword);
    if (res.data.code === 0) {
      ElMessage.success('密码重置成功，请重新登录');
      showForgot.value = false;
      forgotStep.value = 1;
      forgotForm.email = '';
      forgotForm.code = '';
      forgotForm.newPassword = '';
    } else {
      ElMessage.error(res.data.message || '重置失败');
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '重置密码失败');
  } finally {
    forgotLoading.value = false;
  }
}
</script>

<style scoped>
.auth-page {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: calc(100vh - 60px);
  padding: 40px 16px;
}

.auth-card {
  width: 100%;
  max-width: 420px;
}

.auth-title {
  text-align: center;
  margin: 0;
  color: var(--primary-color);
}

.auth-footer {
  text-align: center;
  color: var(--text-light);
  font-size: 0.9rem;
}
</style>

<style>
[data-theme="dark"] .auth-page {
  background: transparent;
}
[data-theme="dark"] .auth-card {
  background: #1e293b;
  border-color: rgba(255,255,255,.08);
}
[data-theme="dark"] .auth-title {
  color: #e2e8f0;
}
[data-theme="dark"] .auth-footer {
  color: #94a3b8;
}
</style>