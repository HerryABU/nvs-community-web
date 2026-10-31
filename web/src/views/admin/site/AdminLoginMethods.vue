<template>
  <div class="admin-page">
    <AdminHeader title="登录方式管理" subtitle="邮箱 / 手机号 / 微信扫码 · 注册验证 · 邮件通道" :icon="Message" />
    <div class="admin-page-body">

      <!-- 登录方式开关 -->
      <div class="admin-panel">
        <SectionHeader title="允许的登录方式" subtitle="控制前台登录页渲染哪些入口（至少保留一种）" :icon="'🔑'" gradient="primary" />
        <div class="login-method-cards">
          <div class="lm-card" :class="{ off: !loginEmail }">
            <div class="lm-icon" style="background: var(--gradient-blue)">✉️</div>
            <div class="lm-body">
              <div class="lm-name">邮箱 + 密码</div>
              <div class="lm-desc">传统账号体系，支持邮箱验证码找回密码</div>
            </div>
            <el-switch v-model="loginEmail" />
          </div>
          <div class="lm-card" :class="{ off: !loginPhone }">
            <div class="lm-icon" style="background: var(--gradient-teal)">📱</div>
            <div class="lm-body">
              <div class="lm-name">手机号 + 验证码</div>
              <div class="lm-desc">
                依赖短信网关，可在下方「短信网关」面板配置
                <el-tag size="small" :type="smsTagType" style="margin-left: 6px">{{ smsGatewayLabel }}</el-tag>
              </div>
            </div>
            <el-switch v-model="loginPhone" />
          </div>
          <div class="lm-card" :class="{ off: !loginWechat }">
            <div class="lm-icon" style="background: var(--gradient-success)">💬</div>
            <div class="lm-body">
              <div class="lm-name">微信扫码</div>
              <div class="lm-desc">
                {{ wechatAvailable ? '微信开放平台网页应用扫码登录' : '需在 .env 配置 WECHAT_APP_ID / WECHAT_APP_SECRET / WECHAT_CALLBACK' }}
              </div>
            </div>
            <el-switch v-model="loginWechat" :disabled="!wechatAvailable" />
          </div>
        </div>
        <p class="hint-text">
          关闭全部登录方式会被后端兜底拦截（自动恢复邮箱+手机号），防止站长误配置导致全员无法登录。
        </p>
      </div>

      <!-- 注册验证安全 -->
      <div class="admin-panel">
        <SectionHeader title="注册验证安全" subtitle="注册/登录时的校验强度" :icon="'🛡️'" gradient="danger" />
        <el-form label-width="140px" class="policy-form" @submit.prevent="saveConfig">
          <el-form-item label="邮箱验证">
            <el-switch v-model="emailVerifyEnabled" active-text="开启" inactive-text="关闭" />
            <span class="hint-text">开启后注册需验证邮箱（需先配置下方 SMTP 通道）</span>
          </el-form-item>
          <el-form-item label="滑块验证码">
            <el-switch v-model="captchaEnabled" active-text="开启" inactive-text="关闭" />
            <span class="hint-text">在注册/登录/发送验证码前显示滑块验证码，防止接口被批量调用</span>
          </el-form-item>
        </el-form>
      </div>

      <!-- 邮件通道（SMTP） -->
      <div class="admin-panel">
        <SectionHeader title="邮件通道（SMTP）" subtitle="邮箱验证码 / 通知邮件的发件配置" :icon="'✉️'" gradient="blue" />
        <el-form label-width="140px" class="policy-form" @submit.prevent="saveConfig">
          <el-form-item label="SMTP 服务器">
            <el-input v-model="configForm.smtp_host" placeholder="smtp.qq.com" style="width: 220px" />
          </el-form-item>
          <el-form-item label="SMTP 端口">
            <el-input v-model="configForm.smtp_port" placeholder="587" style="width: 120px" />
          </el-form-item>
          <el-form-item label="发件邮箱">
            <el-input v-model="configForm.smtp_user" placeholder="your-email@qq.com" style="width: 280px" />
          </el-form-item>
          <el-form-item label="SMTP 密码/授权码">
            <el-input v-model="configForm.smtp_password" type="password" show-password placeholder="授权码" style="width: 260px" />
          </el-form-item>
          <el-form-item label="发件人地址">
            <el-input v-model="configForm.smtp_from" placeholder="同发件邮箱" style="width: 280px" />
          </el-form-item>
          <el-form-item label="证书校验">
            <el-switch v-model="smtpInsecureVerify" active-text="跳过证书校验" inactive-text="校验证书" />
            <span class="hint-text">默认校验证书（安全）。仅当使用自签名/内网 SMTP 服务器报证书错误时再开启</span>
          </el-form-item>
          <el-form-item label="SMTP 测试">
            <div class="smtp-test-row">
              <el-input v-model="smtpTestTo" placeholder="测试收件邮箱，如 you@example.com" style="width: 280px" />
              <el-button type="primary" plain :loading="smtpTesting" @click="runSmtpTest">发送测试邮件</el-button>
            </div>
            <div v-if="smtpTestSteps.length" class="smtp-diag">
              <div v-for="s in smtpTestSteps" :key="s.step" class="diag-step" :class="s.ok ? 'is-ok' : 'is-fail'">
                <span class="diag-icon">{{ s.ok ? '✓' : '✗' }}</span>
                <span class="diag-name">{{ stepName(s.step) }}</span>
                <span class="diag-ms">{{ s.ms }}ms</span>
                <span class="diag-detail">{{ s.detail }}</span>
              </div>
            </div>
            <span class="hint-text">测试将逐阶段诊断：配置 → DNS → 连接 → TLS → 认证 → 发件，快速定位问题</span>
          </el-form-item>
        </el-form>
      </div>

      <!-- 邮箱域名限制 -->
      <div class="admin-panel">
        <SectionHeader title="邮箱域名限制" subtitle="注册邮箱黑白名单" :icon="'🚧'" gradient="amber" />
        <el-form label-width="140px" class="policy-form" @submit.prevent="saveConfig">
          <el-form-item label="域名黑名单">
            <el-select
              v-model="emailDomainBlacklistArr"
              multiple
              filterable
              allow-create
              default-first-option
              clearable
              placeholder="输入域名后回车添加，如 qq.com"
              style="width: 100%"
            />
            <div class="bulk-import">
              <el-input
                v-model="blacklistImport"
                type="textarea"
                :rows="2"
                placeholder="批量导入：粘贴多个域名，逗号 / 空格 / 换行分隔，如 qq.com, gmail.com, 163.com"
              />
              <el-button size="small" type="primary" plain @click="importDomains('black', blacklistImport)">导入</el-button>
            </div>
            <span class="hint-text">黑名单域名禁止注册（留空则不限制）</span>
          </el-form-item>
          <el-form-item label="域名白名单">
            <el-select
              v-model="emailDomainWhitelistArr"
              multiple
              filterable
              allow-create
              default-first-option
              clearable
              placeholder="输入域名后回车添加，如 company.com"
              style="width: 100%"
            />
            <div class="bulk-import">
              <el-input
                v-model="whitelistImport"
                type="textarea"
                :rows="2"
                placeholder="批量导入：粘贴多个域名，逗号 / 空格 / 换行分隔，如 company.com, university.edu"
              />
              <el-button size="small" type="primary" plain @click="importDomains('white', whitelistImport)">导入</el-button>
            </div>
            <span class="hint-text">白名单开启后仅允许这些域名注册（留空则不限制）；黑名单优先级高于白名单</span>
          </el-form-item>
        </el-form>
      </div>

      <!-- 短信网关（手机号验证码登录，管理端可配） -->
      <div class="admin-panel">
        <SectionHeader title="短信网关（手机号登录）" subtitle="手机号验证码登录依赖的短信通道，管理端可配，DB 优先 + .env 同步" :icon="'📨'" gradient="teal" />
        <el-form label-width="150px" class="policy-form" @submit.prevent="saveConfig">
          <el-form-item label="网关形态">
            <el-select v-model="smsForm.sms_gateway" style="width: 320px">
              <el-option label="开发网关（验证码仅入日志）" value="log" />
              <el-option label="通用 HTTP 网关" value="http" />
            </el-select>
          </el-form-item>
          <el-form-item label="网关地址">
            <el-input
              v-model="smsForm.sms_http_url"
              placeholder="https://sms.example.com/api/send"
              style="width: 380px"
              :disabled="smsForm.sms_gateway !== 'http'"
            />
          </el-form-item>
          <el-form-item label="网关 Token">
            <el-input
              v-model="smsForm.sms_http_token"
              type="password"
              show-password
              placeholder="Bearer Token"
              style="width: 380px"
              :disabled="smsForm.sms_gateway !== 'http'"
            />
          </el-form-item>
          <el-form-item label="短信签名">
            <el-input v-model="smsForm.sms_sign_name" placeholder="如 星海文学" style="width: 240px" />
          </el-form-item>
          <el-form-item label="模板编码">
            <el-input v-model="smsForm.sms_template_code" placeholder="如 SMS_123456789" style="width: 240px" />
          </el-form-item>
          <p class="hint-text">HTTP 网关将收到 POST JSON：{ phone, code, sign_name, template_code }；网关地址与 Token 留空视为「未配置」。</p>
        </el-form>
      </div>

      <!-- 微信扫码登录配置（管理端可配） -->
      <div class="admin-panel">
        <SectionHeader title="微信扫码登录" subtitle="微信开放平台「网站应用」三件套，管理端可配，DB 优先 + .env 同步" :icon="'💬'" gradient="success" />
        <el-form label-width="150px" class="policy-form" @submit.prevent="saveConfig">
          <el-form-item label="AppID">
            <el-input v-model="wechatForm.wechat_app_id" placeholder="微信开放平台网站应用 AppID" style="width: 380px" />
          </el-form-item>
          <el-form-item label="AppSecret">
            <el-input v-model="wechatForm.wechat_app_secret" type="password" show-password placeholder="AppSecret" style="width: 380px" />
          </el-form-item>
          <el-form-item label="授权回调地址">
            <el-input v-model="wechatForm.wechat_callback" placeholder="https://your-site.com/api/wechat/callback" style="width: 380px" />
          </el-form-item>
          <p class="hint-text">三件套齐全后，登录页将出现「微信扫码」入口；当前状态：{{ wechatAvailable ? '可用' : '不可用（请补全配置后保存）' }}。</p>
        </el-form>
      </div>

      <!-- 保存 -->
      <div class="admin-panel save-panel">
        <div class="save-row">
          <el-button type="primary" size="large" :loading="savingConfig" @click="saveConfig">保存登录方式配置</el-button>
          <span v-if="envNote" class="env-note">📄 {{ envNote }}</span>
          <span v-if="envError" class="env-error">⚠️ .env 同步失败：{{ envError }}（数据库配置仍已生效）</span>
        </div>
        <p class="hint-text">
          保存后自动同步写入 .env（{{ envFilePath }}），重启后依然生效；数据库中的配置即时生效。
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed, onMounted } from 'vue';
import { Message } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const configForm = reactive<Record<string, string>>({
  smtp_host: '', smtp_port: '587', smtp_user: '', smtp_password: '', smtp_from: '',
});

// ===== 短信网关（手机号验证码登录，管理端可配，DB 优先 + .env 同步） =====
const smsForm = reactive<Record<string, string>>({
  sms_gateway: 'log', sms_http_url: '', sms_http_token: '', sms_sign_name: '', sms_template_code: '',
});

// ===== 微信扫码登录配置（管理端可配，DB 优先 + .env 同步） =====
const wechatForm = reactive<Record<string, string>>({
  wechat_app_id: '', wechat_app_secret: '', wechat_callback: '',
});
const emailVerifyEnabled = ref(false);
const captchaEnabled = ref(false);
const smtpInsecureVerify = ref(false);
const savingConfig = ref(false);

// ===== 登录方式（至少保留一种，后端兜底） =====
const loginEmail = ref(true);
const loginPhone = ref(true);
const loginWechat = ref(false);
const wechatAvailable = ref(false);

// ===== 短信网关状态（来自 .env，只读展示） =====
const smsGateway = ref('log');
const smsHttpConfigured = ref(false);
const smsTagType = computed<'success' | 'warning' | 'info'>(() => {
  if (smsGateway.value !== 'http') return 'info';
  return smsHttpConfigured.value ? 'success' : 'warning';
});
const smsGatewayLabel = computed(() => {
  if (smsGateway.value !== 'http') return '开发网关（验证码仅入日志）';
  return smsHttpConfigured.value ? 'HTTP 网关已配置' : 'HTTP 网关未配置 URL';
});

// ===== .env 同步回显 =====
const envFilePath = ref('.env');
const envNote = ref('');
const envError = ref('');

// ===== 邮箱域名黑白名单 =====
const emailDomainBlacklistArr = ref<string[]>([]);
const emailDomainWhitelistArr = ref<string[]>([]);
const blacklistImport = ref('');
const whitelistImport = ref('');

function jsonToArray(json: string): string[] {
  if (!json) return [];
  try {
    const arr = JSON.parse(json);
    return Array.isArray(arr)
      ? arr.map((s: unknown) => String(s).trim().toLowerCase()).filter(Boolean)
      : [];
  } catch {
    return [];
  }
}
function arrayToJson(arr: string[]): string {
  return JSON.stringify((arr || []).map((s) => s.trim().toLowerCase()).filter(Boolean));
}
function importDomains(target: 'black' | 'white', text: string) {
  const parts = (text || '')
    .split(/[,，;\s]+/)
    .map((s) => s.trim().toLowerCase())
    .filter(Boolean);
  if (!parts.length) {
    ElMessage.warning('请输入至少一个域名');
    return;
  }
  const arr = target === 'black' ? emailDomainBlacklistArr : emailDomainWhitelistArr;
  const exists = new Set(arr.value);
  let added = 0;
  for (const p of parts) {
    if (!exists.has(p)) {
      exists.add(p);
      arr.value.push(p);
      added++;
    }
  }
  if (target === 'black') blacklistImport.value = '';
  else whitelistImport.value = '';
  ElMessage.success(`已导入 ${added} 个域名（重复自动忽略）`);
}

async function loadConfig() {
  try {
    const res = await adminApi.getConfig();
    const data = res.data.data || res.data || {};
    emailVerifyEnabled.value = data.email_verify === 'true';
    captchaEnabled.value = data.captcha_enabled === 'true';
    configForm.smtp_host = data.smtp_host || '';
    configForm.smtp_port = data.smtp_port || '587';
    configForm.smtp_user = data.smtp_user || '';
    configForm.smtp_password = data.smtp_password || '';
    configForm.smtp_from = data.smtp_from || '';
    smtpInsecureVerify.value = data.smtp_insecure_skip_verify === 'true';
    // login_methods 为 JSON 字符串（map[string]string 契约）
    if (typeof data.login_methods === 'string' && data.login_methods) {
      try {
        const m = JSON.parse(data.login_methods);
        loginEmail.value = !!m.email;
        loginPhone.value = !!m.phone;
        loginWechat.value = !!m.wechat;
      } catch { /* 保持默认 */ }
    }
    wechatAvailable.value = data.wechat_login_available === 'true';
    smsGateway.value = data.sms_gateway || 'log';
    smsHttpConfigured.value = data.sms_http_configured === 'true';
    envFilePath.value = data.env_file_path || '.env';
    emailDomainBlacklistArr.value = jsonToArray(data.email_domain_blacklist || '');
    emailDomainWhitelistArr.value = jsonToArray(data.email_domain_whitelist || '');
    // 短信网关配置（DB 优先；未落库时回退 .env 状态，前端以 'log' 兜底）
    smsForm.sms_gateway = data.sms_gateway || smsGateway.value || 'log';
    smsForm.sms_http_url = data.sms_http_url || '';
    smsForm.sms_http_token = data.sms_http_token || '';
    smsForm.sms_sign_name = data.sms_sign_name || '';
    smsForm.sms_template_code = data.sms_template_code || '';
    // 微信扫码配置
    wechatForm.wechat_app_id = data.wechat_app_id || '';
    wechatForm.wechat_app_secret = data.wechat_app_secret || '';
    wechatForm.wechat_callback = data.wechat_callback || '';
  } catch { /* ignore */ }
}

async function saveConfig() {
  // 至少保留一种登录方式（与后端兜底一致，前端先拦）
  if (!loginEmail.value && !loginPhone.value && !loginWechat.value) {
    ElMessage.warning('至少需要保留一种登录方式');
    return;
  }
  savingConfig.value = true;
  envNote.value = '';
  envError.value = '';
  try {
    const res = await adminApi.updateConfig({
      email_verify: emailVerifyEnabled.value ? 'true' : 'false',
      captcha_enabled: captchaEnabled.value ? 'true' : 'false',
      smtp_host: configForm.smtp_host,
      smtp_port: configForm.smtp_port,
      smtp_user: configForm.smtp_user,
      smtp_password: configForm.smtp_password,
      smtp_from: configForm.smtp_from,
      smtp_insecure_skip_verify: smtpInsecureVerify.value ? 'true' : 'false',
      email_domain_blacklist: arrayToJson(emailDomainBlacklistArr.value),
      email_domain_whitelist: arrayToJson(emailDomainWhitelistArr.value),
      login_methods: JSON.stringify({
        email: loginEmail.value,
        phone: loginPhone.value,
        wechat: loginWechat.value && wechatAvailable.value,
      }),
      // 短信网关（手机号验证码登录）
      sms_gateway: smsForm.sms_gateway,
      sms_http_url: smsForm.sms_http_url,
      sms_http_token: smsForm.sms_http_token,
      sms_sign_name: smsForm.sms_sign_name,
      sms_template_code: smsForm.sms_template_code,
      // 微信扫码登录
      wechat_app_id: wechatForm.wechat_app_id,
      wechat_app_secret: wechatForm.wechat_app_secret,
      wechat_callback: wechatForm.wechat_callback,
    });
    const d = res.data.data || res.data || {};
    if (d.env_sync_error) {
      envError.value = d.env_sync_error;
      ElMessage.warning('配置已保存，但 .env 同步失败');
    } else {
      if (d.env_note) envNote.value = d.env_note;
      ElMessage.success('登录方式配置已保存');
    }
    // 保存后重拉配置，刷新「微信可用 / 短信网关」等派生状态
    await loadConfig();
  } catch {
    ElMessage.error('保存失败');
  } finally {
    savingConfig.value = false;
  }
}

// ===== SMTP 测试与诊断 =====
const smtpTestTo = ref('');
const smtpTesting = ref(false);
const smtpTestSteps = ref<{ step: string; ok: boolean; ms: number; detail: string }[]>([]);

const stepLabels: Record<string, string> = {
  config: '配置检查', dns: 'DNS 解析', connect: 'TCP 连接', hello: 'SMTP 握手',
  starttls: 'TLS 升级', auth: '账号认证', mail: '发件/收件', data: '邮件发送',
};
function stepName(step: string) {
  return stepLabels[step] || step;
}

async function runSmtpTest() {
  const to = smtpTestTo.value.trim();
  if (!to) { ElMessage.warning('请先填写测试收件邮箱'); return; }
  smtpTesting.value = true;
  smtpTestSteps.value = [];
  try {
    const res = await adminApi.testSmtp({
      to,
      host: configForm.smtp_host,
      port: configForm.smtp_port,
      user: configForm.smtp_user,
      password: configForm.smtp_password,
      from: configForm.smtp_from,
    });
    const d = res.data.data || {};
    smtpTestSteps.value = d.steps || [];
    if (d.ok) ElMessage.success(d.message || 'SMTP 测试通过，邮件已发送');
    else ElMessage.error(d.message || 'SMTP 测试未通过，请查看诊断');
  } catch (e: any) {
    smtpTestSteps.value = [];
    ElMessage.error(e?.response?.data?.message || '测试请求失败，请检查网络');
  } finally {
    smtpTesting.value = false;
  }
}

onMounted(loadConfig);
</script>

<style scoped>
.admin-page { --admin-content-max: 860px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.hint-text { font-size: 0.8rem; color: var(--text-light, #999); margin-left: 8px; }
.policy-form { max-width: 680px; }
.login-method-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
.lm-card { display: flex; align-items: center; gap: 12px; padding: 14px 16px; border: 1px solid var(--border-color); border-radius: 10px; background: var(--bg-card); transition: opacity .15s; }
.lm-card.off { opacity: 0.55; }
.lm-icon { width: 40px; height: 40px; border-radius: 10px; display: flex; align-items: center; justify-content: center; font-size: 1.2rem; flex-shrink: 0; }
.lm-body { flex: 1; min-width: 0; }
.lm-name { font-weight: 600; font-size: 0.92rem; color: var(--text-color); }
.lm-desc { font-size: 0.76rem; color: var(--text-muted); margin-top: 2px; }
.smtp-test-row { display: flex; gap: 8px; align-items: center; margin-bottom: 8px; width: 100%; }
.smtp-diag { width: 100%; margin: 6px 0; display: flex; flex-direction: column; gap: 4px; }
.diag-step { display: flex; align-items: center; gap: 8px; font-size: 0.82rem; padding: 4px 10px; border-radius: 6px; }
.diag-step.is-ok { background: var(--el-color-success-light-9); color: var(--el-color-success); }
.diag-step.is-fail { background: var(--el-color-danger-light-9); color: var(--el-color-danger); }
.diag-icon { font-weight: 700; width: 14px; flex-shrink: 0; }
.diag-name { font-weight: 600; min-width: 64px; flex-shrink: 0; }
.diag-ms { color: var(--text-muted); min-width: 52px; }
.diag-detail { flex: 1; word-break: break-all; }
.bulk-import { display: flex; gap: 8px; align-items: flex-start; margin: 8px 0; }
.bulk-import .el-textarea { flex: 1; }
.save-panel .save-row { display: flex; align-items: center; gap: 16px; flex-wrap: wrap; }
.env-note { font-size: 0.82rem; color: var(--el-color-success); }
.env-error { font-size: 0.82rem; color: var(--el-color-danger); }
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
