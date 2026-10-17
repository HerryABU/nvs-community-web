<template>
  <div class="auth-page">
    <el-card class="auth-card">
      <template #header>
        <h2 class="auth-title">登录</h2>
      </template>

      <!-- 登录方式页签（F21.3：按站长配置渲染；仅一种时隐藏页签） -->
      <el-tabs v-if="showTabs" v-model="activeTab" stretch class="login-tabs">
        <el-tab-pane v-if="methods.email" label="邮箱登录" name="email" />
        <el-tab-pane v-if="methods.phone" label="手机号登录" name="phone" />
        <el-tab-pane v-if="methods.wechat" label="微信扫码" name="wechat" />
      </el-tabs>

      <!-- 邮箱密码登录 -->
      <el-form
        v-if="activeTab === 'email'"
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        @submit.prevent="handleLogin"
      >
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" placeholder="请输入邮箱" maxlength="128" prefix-icon="Message" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="请输入密码"
            maxlength="128"
            show-password
            prefix-icon="Lock"
          />
        </el-form-item>

        <!-- 滑块验证码（站长开启 captcha_enabled 时，登录前必过，防撞库/暴力破解） -->
        <el-form-item v-if="captchaRequired && !captchaToken">
          <SlideCaptcha @verified="onCaptchaVerified" />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" native-type="submit" :loading="submitting" style="width: 100%">
            登录
          </el-button>
        </el-form-item>
      </el-form>

      <!-- 手机号验证码登录（F10） -->
      <PhoneLoginForm v-else-if="activeTab === 'phone'" @success="onLoginSuccess" />

      <!-- 微信扫码登录（F21.2） -->
      <WeChatQRLogin v-else-if="activeTab === 'wechat'" />

      <div class="auth-footer">
        还没有账号？<router-link to="/register">立即注册</router-link>
        <span style="margin: 0 8px; color: #ccc;">|</span>
        <a href="#" @click.prevent="showForgot = true">忘记密码？</a>
      </div>
    </el-card>

    <!-- 忘记密码（F22：邮箱/手机双通道） -->
    <ForgotPasswordDialog v-model:visible="showForgot" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { useAppStore } from '@/stores/app';
import { ElMessage } from 'element-plus';
import PhoneLoginForm from '@/components/auth/PhoneLoginForm.vue';
import ForgotPasswordDialog from '@/components/auth/ForgotPasswordDialog.vue';
import WeChatQRLogin from '@/components/auth/WeChatQRLogin.vue';
import SlideCaptcha from '@/components/SlideCaptcha.vue';

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const appStore = useAppStore();

// 站长配置的登录方式（缺省 email+phone）
const methods = computed(() => appStore.loginMethods);
const showTabs = computed(() => {
  const n = (methods.value.email ? 1 : 0) + (methods.value.phone ? 1 : 0) + (methods.value.wechat ? 1 : 0);
  return n > 1;
});
const activeTab = ref<'email' | 'phone' | 'wechat'>(
  methods.value.email ? 'email' : methods.value.phone ? 'phone' : 'wechat'
);

onMounted(async () => {
  // siteInfo 可能尚未就绪（SPA 直达登录页），确保一次
  if (!appStore.siteInfo) await appStore.bootstrap();
  if (!methods.value.email && methods.value.phone) activeTab.value = 'phone';
  else if (!methods.value.email && !methods.value.phone && methods.value.wechat) activeTab.value = 'wechat';

  // 微信扫码回调落地：后端写 cookie 后重定向到 /login?wechat=bound|failed
  const wx = route.query.wechat as string | undefined;
  if (wx === 'bound') {
    ElMessage.success('微信登录成功');
    await authStore.fetchUser();
    const redirect = route.query.redirect as string;
    router.replace(redirect || '/');
  } else if (wx === 'failed') {
    ElMessage.error(route.query.reason ? String(route.query.reason) : '微信登录失败，请重试');
  }
});

const formRef = ref();
const submitting = ref(false);
const showForgot = ref(false);

// 站长开启滑块验证码时，邮箱登录前必须先过人机校验
const captchaRequired = computed(() => !!appStore.siteInfo?.captcha_enabled);
const captchaToken = ref('');
function onCaptchaVerified(token: string) {
  captchaToken.value = token;
}

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
    if (captchaRequired.value && !captchaToken.value) {
      ElMessage.warning('请先完成滑块验证');
      return;
    }
    const res = await authStore.login(form.email, form.password, captchaToken.value);
    if (res.code === 0) {
      ElMessage.success('登录成功');
      const redirect = route.query.redirect as string;
      router.push(redirect || '/');
    } else {
      // 失败的滑块 pass token 已被服务端一次性消费，重置以强制重新验证
      if (captchaRequired.value) captchaToken.value = '';
      ElMessage.error(res.message || '登录失败');
    }
  } catch (e: any) {
    if (captchaRequired.value) captchaToken.value = '';
    ElMessage.error(e.response?.data?.message || '登录失败，请检查邮箱和密码');
  } finally {
    submitting.value = false;
  }
}

function onLoginSuccess() {
  // PhoneLoginForm 内部已完成跳转，这里仅占位便于后续扩展（如登录后弹窗关闭）
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

.login-tabs {
  margin-bottom: 4px;
}

.auth-footer {
  text-align: center;
  color: var(--text-light);
  font-size: 0.9rem;
}

/* ====== 移动端登录页适配 ====== */
@media (max-width: 640px) {
  .auth-page {
    padding: 24px 12px;
  }
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
