<template>
  <div class="auth-page">
    <el-card class="auth-card">
      <template #header>
        <h2 class="auth-title">注册</h2>
      </template>
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        @submit.prevent="handleRegister"
      >
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="2-20个字符" prefix-icon="User" />
        </el-form-item>
        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="form.nickname" placeholder="选填，不填则默认使用用户名" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" placeholder="请输入邮箱" prefix-icon="Message" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="至少6位"
            show-password
            prefix-icon="Lock"
          />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            placeholder="请再次输入密码"
            show-password
          />
        </el-form-item>
        <el-form-item prop="agreeToGuidelines">
          <el-checkbox v-model="form.agreeToGuidelines">
            我已阅读并同意
            <el-link type="primary" @click.stop="showGuidelines = true">《平台指南》</el-link>
          </el-checkbox>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" native-type="submit" :loading="submitting" style="width: 100%">
            注册
          </el-button>
        </el-form-item>
      </el-form>
      <div class="auth-footer">
        已有账号？<router-link to="/login">立即登录</router-link>
      </div>
    </el-card>

    <!-- 平台指南弹窗 -->
    <el-dialog v-model="showGuidelines" title="平台指南" width="600px" top="5vh">
      <div class="guidelines-content">
        <h3>欢迎加入星海文学</h3>
        <p>本平台致力于打造一个聚焦<strong>高质量类型文学</strong>的创作与阅读社区。在注册之前，请仔细阅读以下指南：</p>

        <h4>1. 创作自由与责任</h4>
        <p>作者拥有作品的完整版权，平台不占有任何作品权利。您可以随时导出、迁移您的作品。同时，您应对所发布的内容承担全部法律责任。</p>

        <h4>2. 社区规范</h4>
        <ul>
          <li>禁止发布违法内容（包括但不限于：危害国家安全、宣扬暴力恐怖、侵犯他人隐私等）。</li>
          <li>禁止人身攻击、恶意辱骂、歧视性言论。</li>
          <li>禁止发布商业广告、垃圾信息。</li>
          <li>尊重知识产权，不得抄袭、盗用他人作品。</li>
        </ul>

        <h4>3. 内容分类与调性</h4>
        <p>平台聚焦硬科幻、奇幻、推演文学、架空历史、现实主义、悬疑推理、实验文学等深度类型。不设立「爽文」「水文」等分类。请将作品发布在合适的分类下。</p>

        <h4>4. 跨区提醒</h4>
        <p>部分分区（如同人区、政治文学区）设有内容确认机制，进入前会有弹窗提示。这是为了保护读者和作者，而非限制创作。</p>

        <h4>5. 定价与收益</h4>
        <p>作者可自主定价。平台统一抽取 10% 用于服务器运营，打赏金额全额转交作者（扣除支付通道费）。</p>

        <h4>6. 违规处理</h4>
        <p>违反社区规范的内容将被下架，严重违规者将被封禁账号。对处理结果有异议可通过举报/申诉渠道反馈。</p>

        <p class="guidelines-footer">点击"同意"即表示您已阅读、理解并承诺遵守以上所有条款。</p>
      </div>
      <template #footer>
        <el-button @click="showGuidelines = false">关闭</el-button>
        <el-button type="primary" @click="agreeGuidelines">同意并继续</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue';
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { ElMessage } from 'element-plus';

const router = useRouter();
const authStore = useAuthStore();

const formRef = ref();
const submitting = ref(false);
const showGuidelines = ref(false);

const form = reactive({
  username: '',
  nickname: '',
  email: '',
  password: '',
  confirmPassword: '',
  agreeToGuidelines: false,
});

const validateConfirmPassword = (_rule: any, value: string, callback: any) => {
  if (value !== form.password) {
    callback(new Error('两次输入的密码不一致'));
  } else {
    callback();
  }
};

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 2, max: 20, message: '用户名长度为2-20个字符', trigger: 'blur' },
  ],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' },
  ],
  agreeToGuidelines: [
    {
      validator: (_rule: any, value: boolean, callback: any) => {
        if (!value) callback(new Error('请阅读并同意平台指南'));
        else callback();
      },
      trigger: 'change',
    },
  ],
};

async function handleRegister() {
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;

  if (!form.agreeToGuidelines) {
    ElMessage.warning('请先阅读并同意平台指南');
    return;
  }

  submitting.value = true;
  try {
    const nickname = form.nickname || form.username;
    const res = await authStore.register(form.username, form.email, form.password, nickname, form.agreeToGuidelines);
    if (res.code === 0) {
      ElMessage.success('注册成功');
      router.push('/');
    } else {
      ElMessage.error(res.message || '注册失败');
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '注册失败，请重试');
  } finally {
    submitting.value = false;
  }
}

function agreeGuidelines() {
  form.agreeToGuidelines = true;
  showGuidelines.value = false;
}
</script>

<style scoped>
.auth-page {
  display: flex;
  justify-content: center;
  align-items: flex-start;
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

.guidelines-content {
  max-height: 55vh;
  overflow-y: auto;
  line-height: 1.8;
  font-size: 0.95rem;
  padding-right: 8px;
}

.guidelines-content h3 {
  text-align: center;
  color: var(--primary-color);
  margin-bottom: 16px;
}

.guidelines-content h4 {
  margin-top: 16px;
  margin-bottom: 8px;
  color: #333;
}

.guidelines-content ul {
  padding-left: 20px;
}

.guidelines-content li {
  margin-bottom: 4px;
}

.guidelines-footer {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid #eee;
  text-align: center;
  font-weight: 600;
  color: var(--primary-color);
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
[data-theme="dark"] .guidelines-content h3 {
  color: #e2e8f0;
}
[data-theme="dark"] .guidelines-content h4 {
  color: #cbd5e1;
}
[data-theme="dark"] .guidelines-content {
  color: #94a3b8;
}
[data-theme="dark"] .guidelines-footer {
  border-top-color: rgba(255,255,255,.08);
  color: #e2e8f0;
}
</style>
