<template>
  <div class="admin-page">
    <AdminHeader title="日志脱敏" subtitle="环境感知敏感信息脱敏 · 公安部176号令合规" :icon="Lock" />
    <div class="admin-page-body">

      <div class="admin-panel">
        <SectionHeader title="合规说明" subtitle="《公安机关网络空间安全监督检查办法》第七条" :icon="'📋'" gradient="warning" />
        <el-alert
          type="warning"
          :closable="false"
          show-icon
          title="合规说明（《公安机关网络空间安全监督检查办法》第七条）"
          description="生产环境请开启脱敏：访问日志中的 Token / 密码 / 敏感字段将自动替换为掩码后再落盘，满足数据安全与个人信息保护要求；开发环境可关闭以保留原始数据便于调试。脱敏在日志写入前完成，不影响日志归档完整性校验。"
        />
      </div>

      <div class="admin-panel">
        <SectionHeader title="脱敏配置" subtitle="开关 · 敏感字段 · 掩码字符" :icon="'⚙️'" gradient="primary" />
        <el-form label-width="130px" @submit.prevent>
          <el-form-item label="脱敏开关">
            <el-switch v-model="form.enabled" active-text="开启（生产模式）" inactive-text="关闭（开发模式）" />
            <span class="hint-text">对应 .env 的 LOG_REDACT_SENSITIVE；保存后即时生效，无需重启</span>
          </el-form-item>
          <el-form-item label="敏感字段列表">
            <el-select
              v-model="form.sensitive_keys"
              multiple
              filterable
              allow-create
              default-first-option
              clearable
              placeholder="输入字段名回车添加，如 phone / id_card / session"
              style="width: 100%"
            />
            <span class="hint-text">命中的字段将被脱敏（大小写不敏感）；password 及含 password 的字段为内置固定脱敏（******），始终生效</span>
          </el-form-item>
          <el-form-item label="掩码字符">
            <el-input v-model="form.mask" placeholder="***REDACTED***" maxlength="64" style="width: 280px" />
            <span class="hint-text">默认 ***REDACTED***；密码类字段固定使用 ******</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="saving" @click="save">保存配置</el-button>
            <el-button :loading="saving" @click="load">重置</el-button>
          </el-form-item>
        </el-form>
      </div>

      <div class="admin-panel">
        <SectionHeader title="脱敏效果预览" subtitle="F4 · 使用当前表单配置（未保存也生效）" :icon="'🔍'" gradient="blue" />
        <p class="hint-text hint-block">预览使用当前表单配置（未保存也生效），不会修改实际日志配置。</p>
        <el-form label-width="130px">
          <el-form-item label="内容类型">
            <el-radio-group v-model="preview.kind">
              <el-radio-button value="body">请求体 (JSON)</el-radio-button>
              <el-radio-button value="header">请求头</el-radio-button>
              <el-radio-button value="query">Query 参数</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="原始内容">
            <el-input
              v-model="preview.sample"
              type="textarea"
              :rows="4"
              placeholder='请求体：{"email":"test@nvs.local","password":"MyP@ssw0rd123","token":"eyJhbGciOiJIUzI1NiJ9.xxx"}'
            />
          </el-form-item>
          <el-form-item label="脱敏结果">
            <el-input :model-value="preview.result" type="textarea" :rows="4" readonly placeholder="输入原始内容后点击预览" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" plain :loading="previewing" @click="runPreview">预览</el-button>
          </el-form-item>
        </el-form>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { Lock } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const form = reactive<{ enabled: boolean; mask: string; sensitive_keys: string[] }>({
  enabled: false,
  mask: '***REDACTED***',
  sensitive_keys: ['password', 'token', 'authorization', 'cookie', 'secret', 'apikey'],
});
const saving = ref(false);

const preview = reactive({ kind: 'body', sample: '', result: '' });
const previewing = ref(false);

async function load() {
  try {
    const res = await adminApi.getLogRedactConfig();
    const d = res.data.data || {};
    form.enabled = !!d.enabled;
    form.mask = d.mask || '***REDACTED***';
    form.sensitive_keys = Array.isArray(d.sensitive_keys) ? d.sensitive_keys : [];
    preview.result = '';
  } catch {
    /* 保持默认 */
  }
}

async function save() {
  saving.value = true;
  try {
    await adminApi.updateLogRedactConfig({
      enabled: form.enabled,
      mask: form.mask,
      sensitive_keys: form.sensitive_keys,
    });
    ElMessage.success('脱敏配置已保存，即时生效');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function runPreview() {
  if (!preview.sample.trim()) {
    ElMessage.warning('请先输入原始内容');
    return;
  }
  previewing.value = true;
  preview.result = '';
  try {
    const res = await adminApi.previewLogRedact({
      kind: preview.kind,
      sample: preview.sample,
      enabled: form.enabled,
      mask: form.mask,
      sensitive_keys: form.sensitive_keys,
    });
    preview.result = res.data.data?.result || '';
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '预览失败');
  } finally {
    previewing.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
.admin-page { --admin-content-max: 860px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.hint-text { font-size: 0.8rem; color: var(--text-light, #999); margin-left: 8px; line-height: 1.6; }
.hint-block { margin: 0 0 16px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
