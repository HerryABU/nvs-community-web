<template>
  <div class="is-wrap">
    <el-card shadow="never">
      <template #header>
        <div class="is-head">
          <div>
            <div class="is-title">路径隔离代理（三种模式 · GWh 完整引擎）</div>
            <div class="is-sub">pass-through 原样透传 · strip 剥前缀并改写 · isolated 剥前缀只改头。含 Cookie 帽子、熔断、指标、审计。</div>
          </div>
          <el-button type="primary" @click="openEditor()">新增服务</el-button>
        </div>
      </template>
      <el-alert v-if="warnings.length" type="warning" :closable="false" show-icon class="is-alert">
        <template #title>引擎提示：{{ warnings.join('；') }}</template>
      </el-alert>
      <el-table :data="services" v-loading="loading" stripe>
        <el-table-column label="服务名" min-width="130">
          <template #default="{ row }"><code>/{{ row.name }}/</code></template>
        </el-table-column>
        <el-table-column label="模式" width="150">
          <template #default="{ row }"><el-tag size="small" :type="modeType(row.mode)">{{ modeText(row.mode) }}</el-tag></template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="目标" min-width="200" show-overflow-tooltip prop="target" />
        <el-table-column v-if="!isMobile" label="Cookie 帽" width="110" prop="cookie_hat" />
        <el-table-column v-if="!isMobile" label="改写" width="80">
          <template #default="{ row }"><el-tag size="small" :type="row.html_rewrite ? 'success' : 'info'">{{ row.html_rewrite ? '开' : '关' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '生效' : '停用' }}</el-tag>
            <span v-if="row.enabled" class="is-hit">命中 {{ hitCount(row.name) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button link @click="openEditor(row)">编辑</el-button>
            <el-button link :type="row.enabled ? 'warning' : 'success'" @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</el-button>
            <el-button link type="danger" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && !services.length" description="还没有隔离服务" />
    </el-card>

    <el-row :gutter="12" class="is-row">
      <el-col :xs="24" :md="12">
        <el-card shadow="never" header="运行状态（熔断 / 请求指标 · GWh 同款）">
          <el-table :data="snapshot" size="small" max-height="320">
            <el-table-column label="服务" min-width="100" prop="name" />
            <el-table-column label="熔断" width="110">
              <template #default="{ row }"><el-tag size="small" :type="breakerType(row.breaker)">{{ breakerText(row.breaker) }}</el-tag></template>
            </el-table-column>
            <el-table-column label="请求" width="90">
              <template #default="{ row }">{{ num(row.metrics?.requests) }}</template>
            </el-table-column>
            <el-table-column label="失败" width="90">
              <template #default="{ row }">{{ num(row.metrics?.failures) }}</template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!snapshot.length" description="暂无运行数据" :image-size="50" />
        </el-card>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-card shadow="never" header="最近审计（配置变更 / 自指告警）">
          <el-table :data="audit" size="small" max-height="320">
            <el-table-column label="时间" min-width="150" prop="timestamp" />
            <el-table-column label="服务" width="110" prop="service" />
            <el-table-column label="类型" width="120" prop="kind" />
            <el-table-column label="路径" min-width="140" prop="path" show-overflow-tooltip />
          </el-table>
          <el-empty v-if="!audit.length" description="暂无审计记录" :image-size="50" />
        </el-card>
      </el-col>
    </el-row>

    <el-dialog v-model="visible" :title="form.id ? '编辑隔离服务' : '新增隔离服务'" :width="isMobile ? '94%' : '680px'" destroy-on-close>
      <el-form :model="form" label-width="120px">
        <el-form-item label="服务名" required><el-input v-model="form.name" placeholder="如 files（对外路径 /files/）" /></el-form-item>
        <el-form-item label="目标地址" required><el-input v-model="form.target" placeholder="http://127.0.0.1:9000" /></el-form-item>
        <el-form-item label="代理模式" required>
          <el-select v-model="form.mode" style="width: 100%">
            <el-option label="pass-through：原样透传，后端自己认前缀" value="pass-through" />
            <el-option label="strip：剥前缀，允许改写响应体" value="strip" />
            <el-option label="isolated：剥前缀，只做头层修正（不改 body）" value="isolated" />
          </el-select>
        </el-form-item>
        <el-form-item label="超时（秒）"><el-input-number v-model="form.timeout" :min="1" :max="120" /></el-form-item>
        <el-form-item label="Host 策略">
          <el-select v-model="form.host_mode" style="width: 100%">
            <el-option label="keep：保留浏览器侧域名" value="keep" />
            <el-option label="rewrite：改写为后端地址" value="rewrite" />
          </el-select>
        </el-form-item>
        <el-form-item label="Cookie 帽子">
          <el-input v-model="form.cookie_hat" placeholder="如 files__，给 Cookie 名加服务前缀防串号" />
        </el-form-item>
        <el-form-item label="改写 HTML 链接">
          <el-switch v-model="form.html_rewrite" />
          <span class="is-hint">仅 strip 模式生效；把绝对链接补上服务前缀</span>
        </el-form-item>
        <el-form-item label="已知绝对前缀">
          <el-input v-model="form.known_abs_prefixes" placeholder="逗号分隔，如 /assets, /static" />
        </el-form-item>
        <el-form-item label="丢弃外来 Cookie"><el-switch v-model="form.drop_foreign_cookies" /></el-form-item>
        <el-form-item label="Location 补前缀"><el-switch v-model="form.passthrough_location_prefix" /></el-form-item>
        <el-form-item label="最大改写字节">
          <el-input-number v-model="form.max_rewrite_bytes" :min="0" :step="1024" />
          <span class="is-hint">0 = 用默认 2MB</span>
        </el-form-item>
        <el-form-item label="说明"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="立即启用"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" @click="save()">保存并热加载</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { useIsMobile } from '@/utils/mobile';
import { selfhostApi, type IsoService, type IsoSnapshot, type IsoAuditEntry } from '@/api/selfhost';

const isMobile = useIsMobile();
const loading = ref(false);
const visible = ref(false);
const services = ref<IsoService[]>([]);
const snapshot = ref<IsoSnapshot[]>([]);
const audit = ref<IsoAuditEntry[]>([]);
const warnings = ref<string[]>([]);
const form = reactive<Partial<IsoService>>({});
const modeText = (m: string) => ({ 'pass-through': '原样透传', strip: '剥前缀+改写', isolated: '仅头层修正' } as Record<string, string>)[m] || m;
const modeType = (m: string) => ({ 'pass-through': 'info', strip: 'primary', isolated: 'warning' } as Record<string, string>)[m] || 'info';
const num = (v: unknown) => Number(v || 0).toLocaleString('zh-CN');
const hitCount = (name: string) => {
  const s = snapshot.value.find((x) => x.name === name);
  return s ? num(s.metrics?.requests) : '0';
};
const breakerText = (b: any) => (b?.state === 'open' ? '熔断中' : b?.state === 'half-open' ? '半开' : '正常');
const breakerType = (b: any) => (b?.state === 'open' ? 'danger' : b?.state === 'half-open' ? 'warning' : 'success');

async function load() {
  loading.value = true;
  try {
    const r = (await selfhostApi.adminIsoServices()).data.data;
    services.value = r.services || [];
    snapshot.value = r.snapshot || [];
    warnings.value = r.warnings || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
  try {
    audit.value = (await selfhostApi.adminIsoAudit(100)).data.data.entries || [];
  } catch {
    audit.value = [];
  }
}
function openEditor(row?: IsoService) {
  Object.keys(form).forEach((k) => delete (form as any)[k]);
  Object.assign(form, row ? { ...row } : {
    name: '', target: '', mode: 'strip', timeout: 10, description: '', enabled: true,
    host_mode: 'keep', cookie_hat: '', html_rewrite: false, known_abs_prefixes: '',
    drop_foreign_cookies: false, passthrough_location_prefix: false, max_rewrite_bytes: 0,
  });
  visible.value = true;
}
async function save() {
  try {
    await selfhostApi.adminSaveIsoService(form);
    visible.value = false;
    ElMessage.success('已保存并热加载');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
}
async function toggle(row: IsoService) {
  try {
    await selfhostApi.adminToggleIsoService(row.id, !row.enabled);
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}
async function remove(id: number) {
  try {
    await ElMessageBox.confirm('删除后该服务路径立即失效，确定继续？', '提示', { type: 'warning' });
    await selfhostApi.adminDeleteIsoService(id);
    await load();
  } catch (e: any) {
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}
onMounted(load);
</script>

<style scoped>
.is-wrap { padding: 16px; }
.is-head { display: flex; justify-content: space-between; align-items: center; gap: 16px; }
.is-title { font-size: 18px; font-weight: 700; }
.is-sub { font-size: 12px; color: var(--el-text-color-secondary); }
.is-hint { font-size: 12px; color: var(--el-text-color-secondary); margin-left: 8px; }
.is-hit { display: block; font-size: 11px; color: var(--el-text-color-secondary); }
.is-alert { margin-bottom: 12px; }
.is-row { margin-top: 12px; }
@media (max-width: 767px) {
  .is-wrap { padding: 8px; }
  .is-head { flex-direction: column; align-items: stretch; gap: 10px; }
  .is-head .el-button { width: 100%; }
}
</style>
