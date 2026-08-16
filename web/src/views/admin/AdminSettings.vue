<template>
  <div class="admin-page">
    <div class="admin-page-header">
      <el-button text @click="$router.push('/admin')"><el-icon><ArrowLeft /></el-icon> 返回后台</el-button>
      <h2>站点设置</h2>
    </div>
    <div class="admin-page-body">
      <el-form :model="configForm" label-width="120px" @submit.prevent="saveConfig">
        <el-form-item label="站点名称">
          <el-input v-model="configForm.site_name" placeholder="星海文学" maxlength="64" />
        </el-form-item>
        <el-form-item label="VIP 付费">
          <el-switch v-model="vipEnabled" active-text="开启" inactive-text="关闭" />
          <span class="hint-text">关闭后，作者无法申请 VIP 付费功能</span>
        </el-form-item>
        <el-divider />
        <h3 class="sec-title">邮件验证设置</h3>
        <el-form-item label="邮箱验证">
          <el-switch v-model="emailVerifyEnabled" active-text="开启" inactive-text="关闭" />
          <span class="hint-text">开启后注册需验证邮箱，关闭则跳过验证</span>
        </el-form-item>
        <el-form-item label="SMTP 服务器">
          <el-input v-model="configForm.smtp_host" placeholder="smtp.qq.com" style="width:220px" />
        </el-form-item>
        <el-form-item label="SMTP 端口">
          <el-input v-model="configForm.smtp_port" placeholder="587" style="width:120px" />
        </el-form-item>
        <el-form-item label="发件邮箱">
          <el-input v-model="configForm.smtp_user" placeholder="your-email@qq.com" style="width:280px" />
        </el-form-item>
        <el-form-item label="SMTP 密码/授权码">
          <el-input v-model="configForm.smtp_password" type="password" show-password placeholder="授权码" style="width:260px" />
        </el-form-item>
        <el-form-item label="发件人地址">
          <el-input v-model="configForm.smtp_from" placeholder="同发件邮箱" style="width:280px" />
        </el-form-item>
        <el-divider />
        <h3 class="sec-title">安全设置</h3>
        <el-form-item label="滑块验证码">
          <el-switch v-model="captchaEnabled" active-text="开启" inactive-text="关闭" />
          <span class="hint-text">在注册/登录时显示滑块验证码</span>
        </el-form-item>
        <el-divider />
        <h3 class="sec-title">SEO 设置</h3>
        <el-form-item label="robots.txt">
          <el-input v-model="configForm.robots_txt" type="textarea" :rows="6" placeholder="User-agent: *&#10;Allow: /" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveConfig" :loading="savingConfig">保存设置</el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ArrowLeft } from '@element-plus/icons-vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const configForm = reactive<Record<string, string>>({
  site_name: '', vip_enabled: '', email_verify: '', captcha_enabled: '',
  smtp_host: '', smtp_port: '', smtp_user: '', smtp_password: '', smtp_from: '',
  robots_txt: '' });
const vipEnabled = ref(false);
const emailVerifyEnabled = ref(false);
const captchaEnabled = ref(false);
const savingConfig = ref(false);

async function loadConfig() {
  try {
    const res = await adminApi.getConfig();
    const data = res.data.data || res.data || {};
    configForm.site_name = data.site_name || '';
    vipEnabled.value = data.vip_enabled === 'true';
    emailVerifyEnabled.value = data.email_verify === 'true';
    captchaEnabled.value = data.captcha_enabled === 'true';
    configForm.smtp_host = data.smtp_host || '';
    configForm.smtp_port = data.smtp_port || '587';
    configForm.smtp_user = data.smtp_user || '';
    configForm.smtp_password = data.smtp_password || '';
    configForm.smtp_from = data.smtp_from || '';
    configForm.robots_txt = data.robots_txt || '';
  } catch { /* ignore */ }
}

async function saveConfig() {
  savingConfig.value = true;
  try {
    await adminApi.updateConfig({
      site_name: configForm.site_name,
      vip_enabled: vipEnabled.value ? 'true' : 'false',
      email_verify: emailVerifyEnabled.value ? 'true' : 'false',
      captcha_enabled: captchaEnabled.value ? 'true' : 'false',
      smtp_host: configForm.smtp_host,
      smtp_port: configForm.smtp_port,
      smtp_user: configForm.smtp_user,
      smtp_password: configForm.smtp_password,
      smtp_from: configForm.smtp_from,
      robots_txt: configForm.robots_txt,
    });
    ElMessage.success('配置已保存');
  } catch { ElMessage.error('保存失败'); }
  finally { savingConfig.value = false; }
}

onMounted(loadConfig);
</script>

<style scoped>
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 760px; }
.sec-title { margin: 0 0 12px 0; font-size: 1rem; color: var(--text-color, #333); }
.hint-text { font-size: 0.8rem; color: var(--text-light, #999); margin-left: 8px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2, [data-theme="dark"] .sec-title { color: #e2e8f0; }
</style>
