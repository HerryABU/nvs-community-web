<template>
  <div class="rn-provider">
    <div class="provider-cards">
      <div
        v-for="p in providerOptions"
        :key="p.value"
        class="provider-card"
        :class="{ active: form.provider === p.value }"
        @click="form.provider = p.value"
      >
        <div class="provider-card-icon" :style="{ background: p.gradient }">{{ p.icon }}</div>
        <div class="provider-card-body">
          <div class="provider-card-name">{{ p.label }}</div>
          <div class="provider-card-desc">{{ p.desc }}</div>
        </div>
        <el-icon v-if="form.provider === p.value" class="provider-card-check"><Select /></el-icon>
      </div>
    </div>

    <!-- 各提供商凭据表单 -->
    <div v-if="form.provider" class="provider-form">
      <el-alert v-if="providerHint" :title="providerHint" type="info" :closable="false" show-icon class="provider-hint" />

      <template v-if="form.provider === 'national'">
        <el-form-item label="核验端点" required>
          <el-input v-model="form.national.url" placeholder="https:// 授权数据服务商 API 地址（POST JSON）" />
        </el-form-item>
        <el-form-item label="接入令牌">
          <el-input v-model="form.national.token" placeholder="已配置，留空则不修改" show-password />
        </el-form-item>
        <el-form-item label="字段名（高级）">
          <el-input v-model="form.national.name_field" placeholder="real_name" style="width: 160px" />
          <el-input v-model="form.national.card_field" placeholder="id_card" style="width: 160px; margin-left: 8px" />
        </el-form-item>
      </template>

      <template v-else-if="form.provider === 'tencent'">
        <el-form-item label="SecretId" required>
          <el-input v-model="form.tencent.secret_id" placeholder="腾讯云 API 密钥 SecretId" />
        </el-form-item>
        <el-form-item label="SecretKey" required>
          <el-input v-model="form.tencent.secret_key" placeholder="已配置，留空则不修改" show-password />
        </el-form-item>
        <el-form-item label="Region">
          <el-input v-model="form.tencent.region" placeholder="ap-guangzhou（该接口不校验 region）" style="width: 280px" />
        </el-form-item>
      </template>

      <template v-else-if="form.provider === 'alibaba'">
        <el-form-item label="AppCode" required>
          <el-input v-model="form.alibaba.app_code" placeholder="已配置，留空则不修改" show-password />
        </el-form-item>
        <el-form-item label="网关地址" required>
          <el-input v-model="form.alibaba.gateway" placeholder="https://xxx.market.alicloudapi.com/idcardcheck" />
        </el-form-item>
        <el-form-item label="请求方法">
          <el-radio-group v-model="form.alibaba.method">
            <el-radio label="GET">GET（query 传参）</el-radio>
            <el-radio label="POST">POST（表单传参）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="字段名（高级）">
          <el-input v-model="form.alibaba.name_param" placeholder="name" style="width: 160px" />
          <el-input v-model="form.alibaba.card_param" placeholder="idcard" style="width: 160px; margin-left: 8px" />
        </el-form-item>
      </template>

      <template v-else-if="form.provider === 'huawei'">
        <el-form-item label="AK" required>
          <el-input v-model="form.huawei.ak" placeholder="华为云访问密钥 AK" />
        </el-form-item>
        <el-form-item label="SK" required>
          <el-input v-model="form.huawei.sk" placeholder="已配置，留空则不修改" show-password />
        </el-form-item>
        <el-form-item label="Endpoint">
          <el-input v-model="form.huawei.endpoint" placeholder="ivs.cn-north-4.myhuaweicloud.com" style="width: 320px" />
        </el-form-item>
        <el-form-item label="URI">
          <el-input v-model="form.huawei.uri" placeholder="/v2.0/ivs-idcard" style="width: 320px" />
        </el-form-item>
      </template>

      <template v-else-if="form.provider === 'netease'">
        <el-form-item label="businessId" required>
          <el-input v-model="form.netease.business_id" placeholder="网易易盾业务 ID" />
        </el-form-item>
        <el-form-item label="secretId" required>
          <el-input v-model="form.netease.secret_id" placeholder="网易易盾 SecretId" />
        </el-form-item>
        <el-form-item label="secretKey" required>
          <el-input v-model="form.netease.secret_key" placeholder="已配置，留空则不修改" show-password />
        </el-form-item>
        <el-form-item label="接口地址">
          <el-input v-model="form.netease.url" placeholder="https://verify.dun.163.com/v1/rp/check（实证认证地址以控制台为准）" />
        </el-form-item>
        <el-form-item label="签名算法">
          <el-radio-group v-model="form.netease.algorithm">
            <el-radio label="MD5">MD5</el-radio>
            <el-radio label="SHA1">SHA1</el-radio>
            <el-radio label="SHA256">SHA256</el-radio>
          </el-radio-group>
        </el-form-item>
      </template>

      <div class="provider-actions">
        <el-button type="primary" :loading="saving" @click="save">保存提供商配置</el-button>
        <el-button :loading="testing" @click="test">测试连接</el-button>
        <span v-if="testResult" class="test-result" :class="testResult.ok ? 'is-ok' : 'is-fail'">
          {{ testResult.ok ? '✓' : '✗' }} {{ testResult.message }}
        </span>
      </div>
    </div>
    <div v-else class="provider-empty">请选择上方一个核验提供商</div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue';
import { Select } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import { adminApi } from '@/api/admin';

interface ProviderForm {
  provider: string;
  national: { url: string; token: string; name_field: string; card_field: string };
  tencent: { secret_id: string; secret_key: string; region: string };
  alibaba: { app_code: string; gateway: string; method: string; name_param: string; card_param: string };
  huawei: { ak: string; sk: string; endpoint: string; uri: string };
  netease: { business_id: string; secret_id: string; secret_key: string; url: string; algorithm: string };
}

const providerOptions = [
  { value: 'national', label: '国家 / 授权服务商', icon: '🏛️', gradient: 'var(--gradient-primary)', desc: '天远/数脉/聚合数据等对接部委权威数据源的网关，通用 JSON 代理' },
  { value: 'tencent', label: '腾讯云', icon: '🐧', gradient: 'var(--gradient-blue)', desc: '人脸核身 IdCardVerification 二要素核验，TC3 签名，按量计费' },
  { value: 'alibaba', label: '阿里云', icon: '☁️', gradient: 'var(--gradient-amber)', desc: '云市场实名认证商品，AppCode 简单认证，网关/参数可配' },
  { value: 'huawei', label: '华为云', icon: '🔴', gradient: 'var(--gradient-danger)', desc: '人证核身 IVS 证件版二要素，AK/SK 签名' },
  { value: 'netease', label: '网易易盾', icon: '📮', gradient: 'var(--gradient-teal)', desc: '实名核验（实证认证），businessId + MD5 签名' },
];

const providerHints: Record<string, string> = {
  national: '「国家实名接口」说明：公安部公民身份核验能力不对公众开放直连，落地形态为授权数据服务商（对接部委权威数据源）。在此填入服务商提供的端点与令牌即可。',
  tencent: '在腾讯云开通「人脸核身 → 身份证信息核验」后，于访问管理获取 SecretId/SecretKey。结果码 0=一致 / -1=不一致（仅这两种收费）。',
  alibaba: '在阿里云云市场购买身份证二要素核验商品，获取 AppCode 与商品网关地址。不同商品的请求方法/参数名可能不同，均可在下方调整。',
  huawei: '在华为云开通「人证核身服务 IVS」，于「我的凭证」获取 AK/SK。endpoint 中的区域需与开通区域一致（如 cn-north-4）。',
  netease: '在网易易盾申请实名核验服务，获取 businessId/secretId/secretKey。实证认证接口地址以易盾控制台显示为准，默认地址为占位。',
};

const form = reactive<ProviderForm>({
  provider: '',
  national: { url: '', token: '', name_field: '', card_field: '' },
  tencent: { secret_id: '', secret_key: '', region: '' },
  alibaba: { app_code: '', gateway: '', method: 'GET', name_param: '', card_param: '' },
  huawei: { ak: '', sk: '', endpoint: '', uri: '' },
  netease: { business_id: '', secret_id: '', secret_key: '', url: '', algorithm: 'MD5' },
});

const hasSecret = ref<Record<string, boolean>>({});
const saving = ref(false);
const testing = ref(false);
const testResult = ref<{ ok: boolean; message: string } | null>(null);

function providerHint() {
  return providerHints[form.provider] || '';
}

async function load() {
  try {
    const res = await adminApi.getRealNameProvider();
    const d = res.data.data || res.data;
    if (d?.config) {
      form.provider = d.config.provider || '';
      form.national = { name_field: '', card_field: '', ...d.config.national };
      form.tencent = { region: '', ...d.config.tencent };
      form.alibaba = { method: 'GET', name_param: '', card_param: '', ...d.config.alibaba };
      form.huawei = { endpoint: '', uri: '', ...d.config.huawei };
      form.netease = { url: '', algorithm: 'MD5', ...d.config.netease };
    }
    hasSecret.value = d?.has_secret || {};
  } catch { /* ignore */ }
}

async function save() {
  saving.value = true;
  try {
    await adminApi.setRealNameProvider(JSON.parse(JSON.stringify(form)));
    ElMessage.success('提供商配置已保存');
    testResult.value = null;
  } catch { ElMessage.error('保存失败'); }
  finally { saving.value = false; }
}

async function test() {
  testing.value = true;
  testResult.value = null;
  try {
    const res = await adminApi.testRealNameProvider();
    const d = res.data.data || {};
    testResult.value = { ok: !!d.ok, message: d.message || '' };
  } catch (e: any) {
    testResult.value = { ok: false, message: e?.response?.data?.message || '测试请求失败' };
  } finally { testing.value = false; }
}

onMounted(load);
</script>

<style scoped>
.provider-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 12px; margin-bottom: 16px; }
.provider-card { display: flex; align-items: center; gap: 10px; padding: 12px 14px; border: 2px solid var(--border-color); border-radius: 10px; cursor: pointer; transition: border-color .15s, transform .15s; background: var(--bg-card); }
.provider-card:hover { transform: translateY(-1px); }
.provider-card.active { border-color: var(--primary-color); }
.provider-card-icon { width: 36px; height: 36px; border-radius: 8px; display: flex; align-items: center; justify-content: center; font-size: 1.1rem; flex-shrink: 0; }
.provider-card-name { font-weight: 600; font-size: 0.92rem; color: var(--text-color); }
.provider-card-desc { font-size: 0.75rem; color: var(--text-muted); margin-top: 2px; }
.provider-card-check { margin-left: auto; color: var(--primary-color); }
.provider-hint { margin-bottom: 14px; }
.provider-actions { display: flex; align-items: center; gap: 12px; margin-top: 8px; }
.test-result { font-size: 0.82rem; }
.test-result.is-ok { color: var(--el-color-success); }
.test-result.is-fail { color: var(--el-color-danger); }
.provider-empty { padding: 24px; text-align: center; color: var(--text-muted); font-size: 0.85rem; }
</style>
