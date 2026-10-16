<template>
  <el-dialog
    :model-value="visible"
    title="重置密码"
    width="420px"
    :close-on-click-modal="false"
    @update:model-value="onVisibleChange"
    @closed="resetState"
  >
    <el-tabs v-model="channel" stretch>
      <!-- 邮箱通道 -->
      <el-tab-pane label="邮箱验证" name="email">
        <el-form ref="emailFormRef" :model="emailForm" :rules="emailRules" label-position="top">
          <el-form-item label="注册邮箱" prop="email">
            <el-input v-model="emailForm.email" placeholder="请输入注册邮箱" maxlength="128" />
          </el-form-item>
          <el-form-item v-if="captchaRequired && !captchaToken">
            <SlideCaptcha @verified="onCaptchaVerified" />
          </el-form-item>
          <el-form-item v-if="emailStep === 2" label="验证码" prop="code">
            <div class="code-row">
              <el-input v-model="emailForm.code" placeholder="邮件中的验证码" maxlength="6" />
              <el-button class="code-btn" :disabled="emailCooldown > 0" @click="sendEmailCode">
                {{ emailCooldown > 0 ? `${emailCooldown}s 后重试` : '发送验证码' }}
              </el-button>
            </div>
          </el-form-item>
          <el-form-item v-if="emailStep === 2" label="新密码" prop="newPassword">
            <el-input v-model="emailForm.newPassword" type="password" placeholder="请输入新密码（至少6位）" maxlength="128" show-password />
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 手机号通道（F22：管理员须开启 phone 登录方式） -->
      <el-tab-pane v-if="phoneAllowed" label="手机验证" name="phone">
        <el-form ref="phoneFormRef" :model="phoneForm" :rules="phoneRules" label-position="top">
          <el-form-item label="手机号" prop="phone">
            <el-input v-model="phoneForm.phone" placeholder="请输入注册手机号" maxlength="11" />
          </el-form-item>
          <el-form-item label="验证码" prop="code">
            <div class="code-row">
              <el-input v-model="phoneForm.code" placeholder="短信验证码" maxlength="6" />
              <el-button
                class="code-btn"
                :disabled="phoneCooldown > 0 || sendingPhone"
                :loading="sendingPhone"
                @click="sendPhoneCode"
              >
                {{ phoneCooldown > 0 ? `${phoneCooldown}s 后重试` : '发送验证码' }}
              </el-button>
            </div>
          </el-form-item>
          <el-form-item v-if="captchaRequired && !captchaToken">
            <SlideCaptcha @verified="onCaptchaVerified" />
          </el-form-item>
          <el-form-item label="新密码" prop="newPassword">
            <el-input v-model="phoneForm.newPassword" type="password" placeholder="请输入新密码（至少6位）" maxlength="128" show-password />
          </el-form-item>
        </el-form>
      </el-tab-pane>
    </el-tabs>

    <template #footer>
      <el-button @click="close">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="doReset">重置密码</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue';
import { authApi } from '@/api/auth';
import { useAppStore } from '@/stores/app';
import SlideCaptcha from '@/components/SlideCaptcha.vue';
import { ElMessage } from 'element-plus';

const props = defineProps<{ visible: boolean }>();
const emit = defineEmits<{ (e: 'update:visible', v: boolean): void }>();

const appStore = useAppStore();

// 管理员关闭手机登录时隐藏手机通道（F21.3）
const phoneAllowed = computed(() => appStore.loginMethods.phone);
const captchaRequired = computed(() => !!appStore.siteInfo?.captcha_enabled);

const channel = ref<'email' | 'phone'>('email');
const emailStep = ref(1);
const emailCooldown = ref(0);
const phoneCooldown = ref(0);
const sendingPhone = ref(false);
const submitting = ref(false);
const captchaToken = ref('');
const emailFormRef = ref();
const phoneFormRef = ref();

const emailForm = reactive({ email: '', code: '', newPassword: '' });
const phoneForm = reactive({ phone: '', code: '', newPassword: '' });

const emailRules = {
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

const phoneRules = {
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1[3-9]\d{9}$/, message: '请输入有效的 11 位手机号', trigger: 'blur' },
  ],
  code: [
    { required: true, message: '请输入验证码', trigger: 'blur' },
    { len: 6, message: '验证码为 6 位数字', trigger: 'blur' },
  ],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' },
  ],
};

watch(() => props.visible, (v) => {
  if (v) resetState();
});

function onVisibleChange(v: boolean) {
  emit('update:visible', v);
}

function close() {
  emit('update:visible', false);
}

function resetState() {
  channel.value = 'email';
  emailStep.value = 1;
  emailForm.email = '';
  emailForm.code = '';
  emailForm.newPassword = '';
  phoneForm.phone = '';
  phoneForm.code = '';
  phoneForm.newPassword = '';
  captchaToken.value = '';
}

function onCaptchaVerified(token: string) {
  captchaToken.value = token;
}

function startCooldown(which: 'email' | 'phone') {
  if (which === 'email') emailCooldown.value = 60; else phoneCooldown.value = 60;
  const timer = setInterval(() => {
    if (which === 'email') emailCooldown.value--; else phoneCooldown.value--;
    if ((which === 'email' ? emailCooldown.value : phoneCooldown.value) <= 0) clearInterval(timer);
  }, 1000);
}

async function sendEmailCode() {
  const valid = await emailFormRef.value?.validateField('email').catch(() => false);
  if (!valid) return;
  if (captchaRequired.value && !captchaToken.value) {
    ElMessage.warning('请先完成滑块验证');
    return;
  }
  try {
    await authApi.forgotPassword(emailForm.email, captchaToken.value);
    ElMessage.success('验证码已发送，请查收邮件');
    emailStep.value = 2;
    startCooldown('email');
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '发送失败');
  }
}

async function sendPhoneCode() {
  const valid = await phoneFormRef.value?.validateField('phone').catch(() => false);
  if (!valid) return;
  if (captchaRequired.value && !captchaToken.value) {
    ElMessage.warning('请先完成滑块验证');
    return;
  }
  sendingPhone.value = true;
  try {
    await authApi.phoneSendCode(phoneForm.phone, captchaToken.value, 'reset');
    ElMessage.success('验证码已发送，请查收短信');
    startCooldown('phone');
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '验证码发送失败');
  } finally {
    sendingPhone.value = false;
  }
}

async function doReset() {
  submitting.value = true;
  try {
    if (channel.value === 'email') {
      const valid = await emailFormRef.value?.validate().catch(() => false);
      if (!valid) return;
      await authApi.resetPassword(emailForm.email, emailForm.code, emailForm.newPassword);
    } else {
      const valid = await phoneFormRef.value?.validate().catch(() => false);
      if (!valid) return;
      await authApi.phoneResetPassword(phoneForm.phone, phoneForm.code, phoneForm.newPassword);
    }
    ElMessage.success('密码重置成功，请重新登录');
    close();
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '重置失败');
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
