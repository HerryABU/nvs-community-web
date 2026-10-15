<template>
  <div class="admin-page">
    <AdminHeader title="站点设置" subtitle="站点名称 · SMTP · robots.txt · 安全设置" :icon="Setting" />
    <div class="admin-page-body admin-panel">
      <el-form :model="configForm" label-width="120px" @submit.prevent="saveConfig">
        <el-form-item label="站点名称">
          <el-input v-model="configForm.site_name" placeholder="星海文学" maxlength="64" />
        </el-form-item>
        <el-form-item label="首页主模块">
          <el-select v-model="homeModule" style="width:240px">
            <el-option label="类型文学（小说）" value="novels" />
            <el-option label="创作博客" value="blogs" />
            <el-option label="讨论论坛" value="forums" />
            <el-option label="综合（小说为主）" value="combined" />
          </el-select>
          <span class="hint-text">设置站点首页默认展示的内容模块，访客进入首页即见该模块</span>
        </el-form-item>
        <el-form-item label="平台指南">
          <el-input
            v-model="configForm.guidelines"
            type="textarea"
            :rows="8"
            placeholder="注册页《平台指南》内容，支持基础 HTML（h3/h4/p/ul/li/strong/a 等）。留空则使用内置默认指南。"
          />
          <span class="hint-text">展示于注册页「我已阅读并同意《平台指南》」弹窗；保存时自动过滤危险标签</span>
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
        <el-form-item label="证书校验">
          <el-switch v-model="smtpInsecureVerify" active-text="跳过证书校验" inactive-text="校验证书" />
          <span class="hint-text">默认校验证书（安全）。仅当使用自签名/内网 SMTP 服务器报证书错误时再开启</span>
        </el-form-item>
        <el-form-item label="SMTP 测试">
          <div class="smtp-test-row">
            <el-input v-model="smtpTestTo" placeholder="测试收件邮箱，如 you@example.com" style="width:280px" />
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
        <el-divider />
        <h3 class="sec-title">安全设置</h3>
        <el-form-item label="滑块验证码">
          <el-switch v-model="captchaEnabled" active-text="开启" inactive-text="关闭" />
          <span class="hint-text">在注册/登录时显示滑块验证码</span>
        </el-form-item>
        <el-form-item label="邮箱域名黑名单">
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
          <span class="hint-text">黑名单域名禁止注册（可逐条添加或批量导入，留空则不限制）</span>
        </el-form-item>
        <el-form-item label="邮箱域名白名单">
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
        <el-divider />
        <h3 class="sec-title">联网备案（176号令 第七条第一项）</h3>
        <el-form-item label="ICP备案号">
          <el-input v-model="configForm.site_icp" placeholder="如：京ICP备2026000000号" style="width: 320px" />
        </el-form-item>
        <el-form-item label="公安备案号">
          <el-input v-model="configForm.site_psb" placeholder="如：京公网安备11000000000000号" style="width: 320px" />
        </el-form-item>
        <el-form-item label="公安备案链接">
          <el-input v-model="configForm.site_psb_link" placeholder="https://beian.cyberpolice.cn" style="width: 320px" />
        </el-form-item>
        <span class="hint-text">配置后站点页脚将自动展示备案号（可点击跳转）；未完成备案的网站可能面临行政处罚，请尽快办理工信部ICP备案与公安联网备案</span>
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
import { Setting } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const configForm = reactive<Record<string, string>>({
  site_name: '', vip_enabled: '', email_verify: '', captcha_enabled: '',
  smtp_host: '', smtp_port: '', smtp_user: '', smtp_password: '', smtp_from: '',
  robots_txt: '', guidelines: '',
  site_icp: '', site_psb: '', site_psb_link: '' });
const vipEnabled = ref(false);
const emailVerifyEnabled = ref(false);
const captchaEnabled = ref(false);
const smtpInsecureVerify = ref(false);
const savingConfig = ref(false);
// 首页主模块：novels / blogs / forums / combined，控制前台首页默认展示的内容模块
const homeModule = ref<string>('novels');

// 邮箱域名黑白名单（以标签形式逐条管理，底层仍以 JSON 数组存储）
const emailDomainBlacklistArr = ref<string[]>([]);
const emailDomainWhitelistArr = ref<string[]>([]);
// 批量导入临时输入（逗号 / 空格 / 换行分隔）
const blacklistImport = ref('');
const whitelistImport = ref('');

// 批量导入域名：支持逗号、空格、换行、分号分隔，自动去重、小写化
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
    smtpInsecureVerify.value = data.smtp_insecure_skip_verify === 'true';
    configForm.robots_txt = data.robots_txt || '';
    configForm.guidelines = data.guidelines || '';
    configForm.site_icp = data.site_icp || '';
    configForm.site_psb = data.site_psb || '';
    configForm.site_psb_link = data.site_psb_link || '';
    homeModule.value = data.home_module || 'novels';
    emailDomainBlacklistArr.value = jsonToArray(data.email_domain_blacklist || '');
    emailDomainWhitelistArr.value = jsonToArray(data.email_domain_whitelist || '');
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
      smtp_insecure_skip_verify: smtpInsecureVerify.value ? 'true' : 'false',
      robots_txt: configForm.robots_txt,
      guidelines: configForm.guidelines,
      site_icp: configForm.site_icp,
      site_psb: configForm.site_psb,
      site_psb_link: configForm.site_psb_link,
      home_module: homeModule.value,
      email_domain_blacklist: arrayToJson(emailDomainBlacklistArr.value),
      email_domain_whitelist: arrayToJson(emailDomainWhitelistArr.value),
    });
    ElMessage.success('配置已保存');
  } catch { ElMessage.error('保存失败'); }
  finally { savingConfig.value = false; }
}

// ====== SMTP 测试与诊断 ======
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

// 使用当前表单里的 SMTP 配置做测试；密码留空时后端沿用已保存的授权码
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
.admin-page { --admin-content-max: 760px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 760px; }
.sec-title { margin: 0 0 12px 0; font-size: 1rem; color: var(--text-color, #333); }
.hint-text { font-size: 0.8rem; color: var(--text-light, #999); margin-left: 8px; }
.smtp-test-row { display: flex; gap: 8px; align-items: center; margin-bottom: 8px; width: 100%; }
.smtp-diag { width: 100%; margin: 6px 0; display: flex; flex-direction: column; gap: 4px; }
.diag-step { display: flex; align-items: center; gap: 8px; font-size: 0.82rem; padding: 4px 10px; border-radius: 6px; }
.diag-step.is-ok { background: #f0f9eb; color: #529b2e; }
.diag-step.is-fail { background: #fef0f0; color: #d03050; }
.diag-icon { font-weight: 700; width: 14px; flex-shrink: 0; }
.diag-name { font-weight: 600; min-width: 64px; flex-shrink: 0; }
.diag-ms { color: #999; min-width: 52px; flex-shrink: 0; }
.diag-detail { flex: 1; word-break: break-all; }
[data-theme="dark"] .diag-step.is-ok { background: rgba(82, 155, 46, 0.15); color: #95d475; }
[data-theme="dark"] .diag-step.is-fail { background: rgba(208, 48, 80, 0.15); color: #f89898; }
.bulk-import { display: flex; gap: 8px; align-items: flex-start; margin: 8px 0; }
.bulk-import .el-textarea { flex: 1; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2, [data-theme="dark"] .sec-title { color: #e2e8f0; }
</style>
