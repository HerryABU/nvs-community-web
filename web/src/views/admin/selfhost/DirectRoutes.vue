<template>
  <div class="dr-wrap">
    <el-card shadow="never">
      <template #header>
        <div class="dr-head">
          <div>
            <div class="dr-title">端口直派</div>
            <div class="dr-sub">与 gatewayhub 一样，管理员直接把对外路径指派到本机端口，保存即热加载，无需项目审核。</div>
          </div>
          <el-button type="primary" @click="openEditor()">新建指派</el-button>
        </div>
      </template>
      <el-alert type="warning" :closable="false" show-icon class="dr-notice">
        仅管理员可配置。对外路径为 <code>/前缀/</code>，上游固定为 <code>127.0.0.1:端口</code>。
      </el-alert>
      <el-table :data="routes" v-loading="loading" stripe>
        <el-table-column label="对外路径" min-width="180">
          <template #default="{ row }"><code>{{ row.prefix }}/</code></template>
        </el-table-column>
        <el-table-column label="本机端口" width="110" prop="port" />
        <el-table-column v-if="!isMobile" label="目标" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">{{ row.target_url || `127.0.0.1:${row.port}${row.root_path || ''}` }}</template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="说明" min-width="180" show-overflow-tooltip prop="description" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }"><el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '生效' : '停用' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button link @click="openEditor(row)">编辑</el-button>
            <el-button link :type="row.enabled ? 'warning' : 'success'" @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</el-button>
            <el-button link type="danger" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && !routes.length" description="还没有端口直派" />
    </el-card>

    <el-dialog v-model="visible" :title="form.id ? '编辑指派' : '新建指派'" :width="isMobile ? '94%' : '560px'" destroy-on-close>
      <el-form :model="form" label-width="110px">
        <el-form-item label="对外前缀" required><el-input v-model="form.prefix" placeholder="/demo 或 /team/app" /></el-form-item>
        <el-form-item label="本机端口" required><el-input-number v-model="form.port" :min="1" :max="65535" style="width: 100%" /></el-form-item>
        <el-form-item label="后端根路径"><el-input v-model="form.root_path" placeholder="可选，如 /api/v1" /></el-form-item>
        <el-form-item label="完整目标 URL">
          <el-input v-model="form.target_url" placeholder="可选。自指/嵌套用，如 http://127.0.0.1:8000/other" />
          <div class="dr-hint">填写后优先于「端口 + 根路径」。自指由跳数守卫保护，超过 5 跳返回 508。</div>
        </el-form-item>
        <el-form-item label="Cookie 帽子">
          <el-input v-model="form.cookie_hat" placeholder="可选，如 app__（GWh 同款：请求侧脱帽 + 响应侧加帽）" />
          <div class="dr-hint">留空则只收紧 Path、不改名；填写后必须成对启用，后端才能读到自己下的 cookie。</div>
        </el-form-item>
        <el-form-item label="丢弃外来 Cookie"><el-switch v-model="form.drop_foreign_cookies" /></el-form-item>
        <el-form-item label="自签 HTTPS"><el-switch v-model="form.tls_insecure_skip_verify" /><div class="dr-hint">后端用自签证书时开启（仅网关→后端这一段）。</div></el-form-item>
        <el-form-item label="SDK 登录回原站"><el-switch v-model="form.sdk_origin_auth" /><div class="dr-hint">第三方 OAuth 回调改走原站域名，绕过开放平台域名白名单。</div></el-form-item>
        <el-form-item label="微信官方 AppID"><el-input v-model="form.wx_appid" placeholder="可选，如 wx932d4fdaf46d5611" /><div class="dr-hint">配合上一项：shim 拦截 WxLogin 并把 appid 换成官方值。</div></el-form-item>
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
import { selfhostApi, type DirectRoute } from '@/api/selfhost';

const isMobile = useIsMobile();
const loading = ref(false);
const visible = ref(false);
const routes = ref<DirectRoute[]>([]);
const form = reactive<Partial<DirectRoute>>({});

async function load() {
  loading.value = true;
  try {
    routes.value = (await selfhostApi.adminDirectRoutes()).data.data.routes || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}
function openEditor(row?: DirectRoute) {
  Object.keys(form).forEach((k) => delete (form as any)[k]);
  Object.assign(form, row ? { ...row } : { prefix: '', port: 8080, root_path: '', target_url: '', cookie_hat: '', drop_foreign_cookies: false, tls_insecure_skip_verify: false, sdk_origin_auth: false, wx_appid: '', description: '', enabled: true });
  visible.value = true;
}
async function save() {
  try {
    await selfhostApi.adminSaveDirectRoute(form);
    visible.value = false;
    ElMessage.success('已指派并热加载');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
}
async function toggle(row: DirectRoute) {
  try {
    await selfhostApi.adminToggleDirectRoute(row.id, !row.enabled);
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}
async function remove(id: number) {
  try {
    await ElMessageBox.confirm('删除后该路径立即失效，确定继续？', '提示', { type: 'warning' });
    await selfhostApi.adminDeleteDirectRoute(id);
    await load();
  } catch (e: any) {
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}
onMounted(load);
</script>

<style scoped>
.dr-wrap { padding: 16px; }
.dr-head { display: flex; justify-content: space-between; align-items: center; gap: 16px; }
.dr-title { font-size: 18px; font-weight: 700; }
.dr-sub { font-size: 12px; color: var(--el-text-color-secondary); }
.dr-notice { margin-bottom: 12px; }
.dr-hint { font-size: 12px; color: var(--el-text-color-secondary); }
@media (max-width: 767px) {
  .dr-wrap { padding: 8px; }
  .dr-head { flex-direction: column; align-items: stretch; gap: 10px; }
  .dr-head .el-button { width: 100%; }
}
</style>
