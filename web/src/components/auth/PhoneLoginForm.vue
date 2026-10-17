<template>
  <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="handleLogin">
    <el-form-item label="手机号" prop="phone">
      <el-input v-model="form.phone" placeholder="请输入 11 位手机号" maxlength="11" prefix-icon="Iphone" />
    </el-form-item>
    <el-form-item label="验证码" prop="code">
      <div class="code-row">
        <el-input v-model="form.code" placeholder="请输入短信验证码" maxlength="6" prefix-icon="Message" />
        <el-button
          class="code-btn"
          :disabled="cooldown > 0 || sending"
          :loading="sending"
          @click="sendCode"
        >
          {{ cooldown > 0 ? `${cooldown}s 后重试` : '获取验证码' }}
        </el-button>
      </div>
    </el-form-item>

    <!-- 滑块验证码（站长开启 captcha_enabled 时必过，用于发码与登录两步人机校验，单一滑块顺序复用） -->
    <el-form-item v-if="captchaRequired && !captchaToken">
      <SlideCaptcha @verified="onCaptchaVerified" />
    </el-form-item>

    <el-form-item>
      <el-button type="primary" native-type="submit" :loading="submitting" style="width: 100%">
        登录 / 自动注册
      </el-button>
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { useAppStore } from '@/stores/app';
import { authApi } from '@/api/auth';
import SlideCaptcha from '@/components/SlideCaptcha.vue';
import { ElMessage } from 'element-plus';

const emit = defineEmits<{ (e: 'success'): void }>();

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const appStore = useAppStore();

const formRef = ref();
const submitting = ref(false);
const sending = ref(false);
const cooldown = ref(0);
const captchaToken = ref('');

// 站长开启滑块验证码时，发码前必须先过人机校验
const captchaRequired = computed(() => !!appStore.siteInfo?.captcha_enabled);

const form = reactive({ phone: '', code: '' });

const rules = {
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1[3-9]\d{9}$/, message: '请输入有效的 11 位手机号', trigger: 'blur' },
  ],
  code: [
    { required: true, message: '请输入验证码', trigger: 'blur' },
    { len: 6, message: '验证码为 6 位数字', trigger: 'blur' },
  ],
};

function onCaptchaVerified(token: string) {
  captchaToken.value = token;
}

function startCooldown() {
  cooldown.value = 60;
  const timer = setInterval(() => {
    cooldown.value--;
    if (cooldown.value <= 0) clearInterval(timer);
  }, 1000);
}

async function sendCode() {
  const valid = await formRef.value?.validateField('phone').catch(() => false);
  if (!valid) return;
  if (captchaRequired.value && !captchaToken.value) {
    ElMessage.warning('请先完成滑块验证');
    return;
  }
  sending.value = true;
  try {
    await authApi.phoneSendCode(form.phone, captchaToken.value, 'login');
    ElMessage.success('验证码已发送，请查收短信');
    startCooldown();
    // 发码已消费滑块 pass token，重置以便登录提交前再次完成验证（单一滑块顺序复用）
    captchaToken.value = '';
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '验证码发送失败');
  } finally {
    sending.value = false;
  }
}

async function handleLogin() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;

  submitting.value = true;
  try {
    // 🔒 站长开启滑块时，登录提交前必须先过人机校验（与发码滑块顺序复用同一 token 槽位）
    if (captchaRequired.value && !captchaToken.value) {
      ElMessage.warning('请先完成滑块验证');
      return;
    }
    const res = await authStore.phoneLogin(form.phone, form.code, captchaToken.value);
    if (res.code === 0) {
      ElMessage.success('登录成功');
      const redirect = route.query.redirect as string;
      router.push(redirect || '/');
      emit('success');
    } else {
      // 失败的滑块 pass token 已被服务端一次性消费，重置以强制重新验证
      if (captchaRequired.value) captchaToken.value = '';
      ElMessage.error(res.message || '登录失败');
    }
  } catch (e: any) {
    if (captchaRequired.value) captchaToken.value = '';
    ElMessage.error(e.response?.data?.message || '登录失败，请检查验证码');
  } finally {
    submitting.value = false;
  }
}
</script>

<style scoped>
.code-row {
  display: flex;
  gap: 10px;
  width: 100%;
}
.code-btn {
  flex-shrink: 0;
  width: 120px;
}
</style>
