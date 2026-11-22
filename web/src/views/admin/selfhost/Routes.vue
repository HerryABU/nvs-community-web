<template>
  <div class="aroute-wrap">
    <el-card shadow="never">
      <template #header>
        <div class="aroute-head">
          <div>
            <div class="aroute-title">反向代理管理</div>
            <div class="aroute-sub">移植自 gatewayhub 的最长前缀匹配与响应改写；只有已过审且启用的路由才生效。</div>
          </div>
          <div class="aroute-tools">
            <el-input v-model="keyword" placeholder="搜索前缀/地址" clearable style="width: 200px" @keyup.enter="load()" />
            <el-button type="primary" @click="openEditor()">新建路由</el-button>
          </div>
        </div>
      </template>

      <el-alert type="warning" :closable="false" show-icon class="aroute-notice">
        对外路径形如 <code>{{ base }}/示例前缀/</code>（前缀挂在根路径，与 gatewayhub 一致）。平台不终止 TLS，HTTPS 请由外层网关承担。
      </el-alert>

      <el-form inline class="aroute-config">
        <el-form-item label="编译型语言必须提供源码">
          <el-switch v-model="sourceRequired" @change="saveConfig()" />
        </el-form-item>
        <el-form-item label="本机执行">
          <el-switch :model-value="false" disabled />
          <span class="aroute-hint">预留能力，当前不接受启用</span>
        </el-form-item>
      </el-form>

      <el-table :data="routes" v-loading="loading" stripe>
        <el-table-column v-if="!isMobile" label="ID" width="70" prop="id" />
        <el-table-column label="对外前缀" min-width="160">
          <template #default="{ row }"><code>{{ base }}{{ row.prefix }}/</code></template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="项目" width="100" prop="project_id" />
        <el-table-column label="上游地址" min-width="230" show-overflow-tooltip prop="target" />
        <el-table-column v-if="!isMobile" label="去前缀" width="90">
          <template #default="{ row }">{{ row.strip_prefix ? '是' : '否' }}</template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="超时" width="90">
          <template #default="{ row }">{{ row.timeout }}s</template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '运行中' : '已停止' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="错误" min-width="160" show-overflow-tooltip prop="last_error" />
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link :type="row.enabled ? 'warning' : 'success'" @click="toggle(row)">{{ row.enabled ? '停止' : '启用' }}</el-button>
            <el-button link type="danger" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && !routes.length" description="还没有反向代理路由" />
      <div class="aroute-sum" v-if="total">共 {{ total }} 条路由，{{ active }} 条启用中</div>
    </el-card>

    <el-dialog v-model="visible" title="新建/保存路由" :width="isMobile ? '94%' : '620px'" destroy-on-close>
      <el-form :model="form" label-width="110px">
        <el-form-item label="项目" required>
          <el-select v-model="form.project_id" filterable placeholder="仅显示已通过审核的项目" style="width: 100%">
            <el-option v-for="p in approvedProjects" :key="p.id" :label="`#${p.id} ${p.name}`" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="对外前缀" required><el-input v-model="form.prefix" placeholder="/demo 或 /team/app" /></el-form-item>
        <el-form-item label="上游地址" required><el-input v-model="form.target" placeholder="https://your-host:9000" /></el-form-item>
        <el-form-item label="去掉前缀"><el-switch v-model="form.strip_prefix" /></el-form-item>
        <el-form-item label="超时（秒）"><el-input-number v-model="form.timeout" :min="1" :max="120" /></el-form-item>
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
import { selfhostApi, type SelfHostRoute, type SelfHostProject } from '@/api/selfhost';
import { useIsMobile } from '@/utils/mobile';

const isMobile = useIsMobile();
const loading = ref(false);
const visible = ref(false);
const keyword = ref('');
const routes = ref<SelfHostRoute[]>([]);
const approvedProjects = ref<SelfHostProject[]>([]);
const total = ref(0);
const active = ref(0);
const base = ref('/sh');
const sourceRequired = ref(true);
const form = reactive<Partial<SelfHostRoute>>({});

async function load() {
  loading.value = true;
  try {
    const [r, p, cfg] = await Promise.all([
      selfhostApi.adminListRoutes(keyword.value),
      selfhostApi.adminListProjects('approved'),
      selfhostApi.adminConfig(),
    ]);
    routes.value = r.data.data.routes || [];
    total.value = r.data.data.total || 0;
    active.value = r.data.data.active || 0;
    base.value = r.data.data.base || '';
    approvedProjects.value = p.data.data.projects || [];
    sourceRequired.value = cfg.data.data.source_required;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

function openEditor() {
  Object.keys(form).forEach((k) => delete (form as any)[k]);
  Object.assign(form, { project_id: approvedProjects.value[0]?.id || 0, prefix: '', target: '', strip_prefix: true, timeout: 10 });
  visible.value = true;
}

async function save() {
  try {
    await selfhostApi.adminSaveRoute(form);
    visible.value = false;
    ElMessage.success('路由已保存并热加载');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
}

async function toggle(row: SelfHostRoute) {
  try {
    await selfhostApi.adminToggleRoute(row.id, !row.enabled);
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function remove(id: number) {
  try {
    await ElMessageBox.confirm('删除后该路径立即失效，确定继续？', '提示', { type: 'warning' });
    await selfhostApi.adminDeleteRoute(id);
    await load();
  } catch (e: any) {
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function saveConfig() {
  try {
    await selfhostApi.adminSaveConfig(sourceRequired.value);
    ElMessage.success('配置已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '配置保存失败');
  }
}

onMounted(load);
</script>

<style scoped>
.aroute-wrap { padding: 16px; }
.aroute-head { display: flex; justify-content: space-between; align-items: center; gap: 16px; }
.aroute-title { font-size: 18px; font-weight: 700; }
.aroute-sub { font-size: 12px; color: var(--el-text-color-secondary); }
.aroute-tools { display: flex; gap: 8px; }
.aroute-notice { margin-bottom: 12px; }
.aroute-config { margin-bottom: 8px; }
.aroute-hint { font-size: 12px; color: var(--el-text-color-secondary); margin-left: 8px; }
.aroute-sum { margin-top: 10px; font-size: 12px; color: var(--el-text-color-secondary); }
</style>

@media (max-width: 767px) {
  .aroute-wrap { padding: 8px; }
  .aroute-head { flex-direction: column; align-items: stretch; gap: 10px; }
  .aroute-tools { flex-wrap: wrap; }
  .aroute-tools .el-input { width: 100% !important; }
  .aroute-config { display: flex; flex-direction: column; align-items: stretch; }
  :deep(.el-form-item__label) { padding-bottom: 2px; }
}
