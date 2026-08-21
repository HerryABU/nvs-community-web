<template>
  <div class="admin-page">
    <AdminHeader title="远程站点互通" subtitle="远程 NVS 站点互联与作品同步" :icon="Connection" />
    <div class="admin-page-body admin-panel">
      <p class="hint-text">模仿 AList 的站点互联：添加远程 NVS 站点后可同步作品列表。</p>
      <div style="margin-bottom:12px">
        <el-button type="primary" size="small" @click="showAddSiteDialog">添加站点</el-button>
      </div>
      <el-table :data="sites" v-loading="loadingSites" style="width:100%">
        <el-table-column prop="name" label="站点名称" min-width="120" />
        <el-table-column prop="url" label="站点地址" min-width="180" show-overflow-tooltip />
        <el-table-column prop="api_url" label="API 地址" min-width="200" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'" size="small">
              {{ row.status === 'active' ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="novel_count" label="缓存作品" width="90" />
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="syncSite(row)" :loading="syncingId === row.id">同步</el-button>
            <el-button size="small" @click="editSite(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="deleteSite(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 谁连了我（入站连接） -->
      <div class="inbound-block">
        <div class="inbound-head">
          <h3>谁连了我（入站连接）</h3>
          <el-button text size="small" @click="loadInbound">刷新</el-button>
        </div>
        <p class="hint-text">
          当其它 NVS 站点通过联邦接口同步/访问本站时，这里会记录对方的身份（站点名、地址、来源 IP 与访问次数）。
          对方需在「API 地址」填到 <code>/api/federated</code>，同步时会自动向本站自报身份。
        </p>
        <el-table :data="inbound" v-loading="loadingInbound" size="small" style="width:100%">
          <el-table-column prop="name" label="站点名" min-width="120">
            <template #default="{ row }">
              <strong>{{ row.name || '（未自报）' }}</strong>
            </template>
          </el-table-column>
          <el-table-column prop="url" label="站点地址" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">{{ row.url || '—' }}</template>
          </el-table-column>
          <el-table-column prop="ip" label="来源 IP" width="140" />
          <el-table-column label="接口" width="140">
            <template #default="{ row }"><code>{{ row.endpoint }}</code></template>
          </el-table-column>
          <el-table-column prop="count" label="访问次数" width="90" />
          <el-table-column label="最近访问" width="170">
            <template #default="{ row }">{{ formatTime(row.last_seen) }}</template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <!-- 添加/编辑站点弹窗 -->
    <el-dialog v-model="showAddSite" :title="editingSite ? '编辑站点' : '添加远程站点'" width="520px">
      <el-form :model="siteForm" label-width="100px">
        <el-form-item label="站点名称" required>
          <el-input v-model="siteForm.name" placeholder="如：星海文学" />
        </el-form-item>
        <el-form-item label="站点地址" required>
          <el-input v-model="siteForm.url" placeholder="如：https://nvs.example.com" />
        </el-form-item>
        <el-form-item label="API 地址" required>
          <el-input v-model="siteForm.api_url" placeholder="如：https://nvs.example.com/api" />
          <div class="form-tip">对方站点 API 根地址（以 <code>/api</code> 结尾）。同步时会调用其 <code>/api/novels</code> 公开作品列表接口。</div>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="siteForm.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddSite = false">取消</el-button>
        <el-button type="primary" @click="submitSite" :loading="submittingSite">
          {{ editingSite ? '保存' : '添加' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { Connection } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';

const sites = ref<any[]>([]);
const loadingSites = ref(false);
const showAddSite = ref(false);
const editingSite = ref<any>(null);
const submittingSite = ref(false);
const syncingId = ref(0);
const siteForm = reactive({ name: '', url: '', api_url: '', description: '' });

const inbound = ref<any[]>([]);
const loadingInbound = ref(false);

async function loadSites() {
  loadingSites.value = true;
  try { const res = await adminApi.getSites(); sites.value = res.data.data || []; } catch { /* ignore */ }
  loadingSites.value = false;
}

async function loadInbound() {
  loadingInbound.value = true;
  try { const res = await adminApi.getInboundSites(); inbound.value = res.data.data || []; } catch { /* ignore */ }
  loadingInbound.value = false;
}

function formatTime(s?: string) {
  if (!s) return '';
  return new Date(s).toLocaleString('zh-CN');
}

function showAddSiteDialog() { resetSiteForm(); showAddSite.value = true; }

async function syncSite(site: any) {
  syncingId.value = site.id;
  try {
    const res = await adminApi.syncSite(site.id);
    if (res.data.code === 0) {
      ElMessage.success(`同步完成，新增 ${res.data.data.synced} 部作品，共 ${res.data.data.total} 部`);
      await loadSites();
    } else {
      ElMessage.error(res.data.message || '同步失败');
    }
  } catch (e: any) {
    ElMessage.error('同步失败：' + (e?.response?.data?.message || e.message));
  }
  syncingId.value = 0;
}

function editSite(site: any) {
  editingSite.value = site;
  siteForm.name = site.name;
  siteForm.url = site.url;
  siteForm.api_url = site.api_url;
  siteForm.description = site.description || '';
  showAddSite.value = true;
}

async function submitSite() {
  if (!siteForm.name || !siteForm.url || !siteForm.api_url) { ElMessage.warning('请填写站点名称、地址和API地址'); return; }
  submittingSite.value = true;
  try {
    if (editingSite.value) {
      await adminApi.updateSite(editingSite.value.id, { ...siteForm });
      ElMessage.success('站点已更新');
    } else {
      await adminApi.createSite({ ...siteForm });
      ElMessage.success('站点已添加');
    }
    showAddSite.value = false;
    resetSiteForm();
    await loadSites();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
  submittingSite.value = false;
}

function resetSiteForm() {
  editingSite.value = null;
  siteForm.name = '';
  siteForm.url = '';
  siteForm.api_url = '';
  siteForm.description = '';
}

async function deleteSite(site: any) {
  try {
    await ElMessageBox.confirm(`确定要删除站点「${site.name}」吗？`, '确认删除', { type: 'warning' });
    await adminApi.deleteSite(site.id);
    ElMessage.success('已删除');
    await loadSites();
  } catch { /* cancelled */ }
}

onMounted(() => { loadSites(); loadInbound(); });
</script>

<style scoped>
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 1100px; }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; }
.form-tip { width: 100%; font-size: 0.78rem; color: var(--text-light, #999); line-height: 1.5; }
.form-tip code { background: #eef2ff; color: #4f46e5; padding: 1px 6px; border-radius: 4px; font-size: 0.85rem; }
.inbound-block { margin-top: 28px; padding-top: 20px; border-top: 1px solid var(--border-color, #e5e7eb); }
.inbound-head { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.inbound-head h3 { margin: 0; font-size: 1.05rem; color: var(--text-color, #333); }
.inbound-block code { background: #eef2ff; color: #4f46e5; padding: 1px 6px; border-radius: 4px; font-size: 0.85rem; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
[data-theme="dark"] .inbound-head h3 { color: #e2e8f0; }
[data-theme="dark"] .inbound-block { border-color: #334155; }
</style>
