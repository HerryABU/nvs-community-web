<template>
  <div class="admin-page">
    <AdminHeader title="站点设置" subtitle="站点名称 · 首页主模块 · 平台指南 · 备案 · robots.txt" :icon="Setting" />
    <div class="admin-page-body admin-panel">

      <!-- 快捷入口：登录方式与实名/未成年已独立成页 -->
      <div class="quick-nav">
        <div class="quick-nav-item" @click="go('/admin/login-methods')">
          <span class="qn-icon" style="background: var(--gradient-indigo)">🔑</span>
          <div class="qn-body">
            <div class="qn-title">登录方式管理</div>
            <div class="qn-desc">邮箱/手机号/微信扫码入口 · 注册验证 · SMTP 通道 · 域名黑白名单</div>
          </div>
          <el-icon><ArrowRight /></el-icon>
        </div>
        <div class="quick-nav-item" @click="go('/admin/realname')">
          <span class="qn-icon" style="background: var(--gradient-success)">🪪</span>
          <div class="qn-body">
            <div class="qn-title">实名与未成年保护</div>
            <div class="qn-desc">核验提供商（国家/腾讯/阿里/华为/网易） · 发布实名强制 · 未成年模式 · 实名记录</div>
          </div>
          <el-icon><ArrowRight /></el-icon>
        </div>
      </div>

      <el-form :model="configForm" label-width="120px" @submit.prevent="saveConfig">
        <SectionHeader
          title="基础设置"
          subtitle="站点名称 · 首页主模块 · 平台指南"
          :icon="'⚙️'"
          gradient="primary"
        />
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
        <SectionHeader
          title="联网备案"
          subtitle="176号令 第七条第一项 · ICP / 公安 / 许可证"
          :icon="'📋'"
          gradient="teal"
        />
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

        <!-- 其他备案信息（可选，未配置项不在前台展示） -->
        <el-form-item label="网络文化经营许可证">
          <el-input v-model="configForm.site_net_culture_license" placeholder="如：京网文〔2026〕0001-000号" style="width: 320px" />
        </el-form-item>
        <el-form-item label="APP 备案号">
          <el-input v-model="configForm.site_app_filing" placeholder="如：京ICP备2026000000号-1A" style="width: 320px" />
        </el-form-item>
        <el-form-item label="广播电视节目制作经营许可证">
          <el-input v-model="configForm.site_broadcast_license" placeholder="如：（京）字第00001号" style="width: 320px" />
        </el-form-item>
        <el-form-item label="IDC / ISP 许可证">
          <el-input v-model="configForm.site_idc_license" placeholder="如：京IDC证0001号" style="width: 320px" />
        </el-form-item>
        <SectionHeader
          title="SEO 设置"
          subtitle="robots.txt 抓取策略"
          :icon="'🔍'"
          gradient="cyan"
        />
        <el-form-item label="robots.txt">
          <el-input v-model="configForm.robots_txt" type="textarea" :rows="6" placeholder="User-agent: *&#10;Allow: /" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveConfig" :loading="savingConfig">保存设置</el-button>
          <span v-if="envNote" class="env-note">📄 {{ envNote }}</span>
          <span v-if="envError" class="env-error">⚠️ .env 同步失败（数据库配置仍已生效）</span>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { Setting, ArrowRight } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const router = useRouter();
function go(path: string) { router.push(path); }

const configForm = reactive<Record<string, string>>({
  site_name: '', vip_enabled: '',
  robots_txt: '', guidelines: '',
  site_icp: '', site_psb: '', site_psb_link: '',
  // 其他备案信息（可选）
  site_net_culture_license: '', site_app_filing: '', site_broadcast_license: '', site_idc_license: '' });
const vipEnabled = ref(false);
const savingConfig = ref(false);
const envNote = ref('');
const envError = ref('');
// 首页主模块：novels / blogs / forums / combined，控制前台首页默认展示的内容模块
const homeModule = ref<string>('novels');

async function loadConfig() {
  try {
    const res = await adminApi.getConfig();
    const data = res.data.data || res.data || {};
    configForm.site_name = data.site_name || '';
    vipEnabled.value = data.vip_enabled === 'true';
    configForm.robots_txt = data.robots_txt || '';
    configForm.guidelines = data.guidelines || '';
    configForm.site_icp = data.site_icp || '';
    configForm.site_psb = data.site_psb || '';
    configForm.site_psb_link = data.site_psb_link || '';
    configForm.site_net_culture_license = data.site_net_culture_license || '';
    configForm.site_app_filing = data.site_app_filing || '';
    configForm.site_broadcast_license = data.site_broadcast_license || '';
    configForm.site_idc_license = data.site_idc_license || '';
    homeModule.value = data.home_module || 'novels';
  } catch { /* ignore */ }
}

async function saveConfig() {
  savingConfig.value = true;
  envNote.value = '';
  envError.value = '';
  try {
    const res = await adminApi.updateConfig({
      site_name: configForm.site_name,
      vip_enabled: vipEnabled.value ? 'true' : 'false',
      robots_txt: configForm.robots_txt,
      guidelines: configForm.guidelines,
      site_icp: configForm.site_icp,
      site_psb: configForm.site_psb,
      site_psb_link: configForm.site_psb_link,
      site_net_culture_license: configForm.site_net_culture_license,
      site_app_filing: configForm.site_app_filing,
      site_broadcast_license: configForm.site_broadcast_license,
      site_idc_license: configForm.site_idc_license,
      home_module: homeModule.value,
    });
    const d = res.data.data || res.data || {};
    if (d.env_sync_error) {
      envError.value = d.env_sync_error;
      ElMessage.warning('配置已保存，但 .env 同步失败');
    } else {
      if (d.env_note) envNote.value = d.env_note;
      ElMessage.success('配置已保存');
    }
  } catch { ElMessage.error('保存失败'); }
  finally { savingConfig.value = false; }
}

onMounted(loadConfig);
</script>

<style scoped>
.admin-page { --admin-content-max: 760px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: 760px; }
.hint-text { font-size: 0.8rem; color: var(--text-light, #999); margin-left: 8px; }
.quick-nav { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-bottom: 20px; }
.quick-nav-item { display: flex; align-items: center; gap: 12px; padding: 14px 16px; border: 1px solid var(--border-color); border-radius: 10px; background: var(--bg-card); cursor: pointer; transition: transform .15s, border-color .15s; }
.quick-nav-item:hover { transform: translateY(-1px); border-color: var(--primary-color); }
.qn-icon { width: 38px; height: 38px; border-radius: 9px; display: flex; align-items: center; justify-content: center; font-size: 1.1rem; flex-shrink: 0; }
.qn-body { flex: 1; min-width: 0; }
.qn-title { font-weight: 600; font-size: 0.92rem; color: var(--text-color); }
.qn-desc { font-size: 0.74rem; color: var(--text-muted); margin-top: 2px; }
.env-note { font-size: 0.82rem; color: var(--el-color-success); margin-left: 12px; }
.env-error { font-size: 0.82rem; color: var(--el-color-danger); margin-left: 12px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
