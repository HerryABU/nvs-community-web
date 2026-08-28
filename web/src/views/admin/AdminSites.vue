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
        <el-table-column label="操作" width="330" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="syncSite(row)" :loading="syncingId === row.id">同步</el-button>
            <el-button size="small" type="primary" plain @click="openKeys(row)">联邦 Key</el-button>
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

    <!-- 联邦 Key 管理弹窗（双向可见） -->
    <el-dialog v-model="showKeys" :title="`联邦 Key · ${keysSite?.name || ''}`" width="720px">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 14px"
        title="双向可见机制"
      >
        <template #default>
          <div class="hint-text" style="margin: 4px 0 0">
            <b>本站签发给对方</b>：对方访问本站同步时携带，本站可识别对方用的是哪个 Key、是谁签发的。
            <b>对方签发给本站</b>：本站同步请求自动携带，对方同样能识别本站身份，并给高配额（避免被匿名限流挡住）。
            两站互换 Key 后，互相都能看到对方在用哪个 Key。
          </div>
        </template>
      </el-alert>

      <!-- 本站签发给对方（inbound） -->
      <div class="key-block">
        <div class="key-block-head">
          <h4>① 本站签发给对方站的 Key</h4>
          <el-button size="small" type="primary" @click="genInboundKey">生成新 Key</el-button>
        </div>
        <p class="hint-text">将完整 Key 转交给对方站，填入其「远程站点互通 → 该站点 → 对方签发给本站的 Key」。</p>
        <el-table :data="inboundKeys" size="small" style="width: 100%">
          <el-table-column label="Key 前缀" width="170">
            <template #default="{ row }"><code>{{ row.key_prefix }}</code></template>
          </el-table-column>
          <el-table-column label="签发方" width="130">
            <template #default="{ row }">{{ row.issuer_name || '—' }}</template>
          </el-table-column>
          <el-table-column label="配额" width="110">
            <template #default="{ row }">{{ row.quota_per_min }} 次/分</template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{ row }">
              <el-tag :type="row.is_active ? 'success' : 'danger'" size="small">{{ row.is_active ? '启用' : '停用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="最近使用" width="150">
            <template #default="{ row }">{{ formatTime(row.last_used_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="120">
            <template #default="{ row }">
              <el-button size="small" text @click="toggleKey(row)">{{ row.is_active ? '停用' : '启用' }}</el-button>
              <el-button size="small" text type="danger" @click="deleteKey(row)">吊销</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 对方签发给本站（outbound） -->
      <div class="key-block">
        <div class="key-block-head">
          <h4>② 对方站签发给本站的 Key</h4>
        </div>
        <p class="hint-text">在对方站后台「远程站点互通」生成 Key 后粘贴到这里；同步时本站会自动携带该 Key（AES 加密存储）。</p>
        <div style="display: flex; gap: 8px; margin-bottom: 10px">
          <el-input v-model="outboundKeyInput" placeholder="粘贴对方站签发给本站的完整 Key" size="small" show-password />
          <el-button size="small" type="primary" @click="saveOutboundKey" :loading="savingOutbound">保存</el-button>
        </div>
        <el-table v-if="outboundKeys.length" :data="outboundKeys" size="small" style="width: 100%">
          <el-table-column label="Key 前缀" width="170">
            <template #default="{ row }"><code>{{ row.key_prefix }}</code></template>
          </el-table-column>
          <el-table-column label="签发方" width="160">
            <template #default="{ row }">{{ row.issuer_name || '—' }}</template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{ row }">
              <el-tag :type="row.is_active ? 'success' : 'danger'" size="small">{{ row.is_active ? '启用' : '停用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120">
            <template #default="{ row }">
              <el-button size="small" text @click="toggleKey(row)">{{ row.is_active ? '停用' : '启用' }}</el-button>
              <el-button size="small" text type="danger" @click="deleteKey(row)">吊销</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-dialog>

    <!-- 新生成的联邦 Key 一次性展示 -->
    <el-dialog v-model="showNewKey" title="联邦 Key 生成成功" width="560px">
      <el-alert type="warning" :closable="false" show-icon title="完整 Key 仅此一次展示，请立即复制并转交对方站点" style="margin-bottom: 12px" />
      <div class="newkey-box">
        <code class="newkey-code">{{ newKeyValue }}</code>
        <el-button size="small" type="primary" @click="copyNewKey">复制</el-button>
      </div>
      <p class="hint-text">对方站收到后，在其「远程站点互通 → 本站点 → 对方签发给本站的 Key」粘贴保存即可。本站可从入站记录中识别对方正在使用这个 Key。</p>
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

// ===== 联邦 Key 管理（双向可见） =====
const showKeys = ref(false);
const keysSite = ref<any>(null);
const inboundKeys = ref<any[]>([]);
const outboundKeys = ref<any[]>([]);
const loadingKeys = ref(false);
const outboundKeyInput = ref('');
const savingOutbound = ref(false);
const showNewKey = ref(false);
const newKeyValue = ref('');

async function openKeys(site: any) {
  keysSite.value = site;
  showKeys.value = true;
  outboundKeyInput.value = '';
  await loadSiteKeys();
}

async function loadSiteKeys() {
  if (!keysSite.value) return;
  loadingKeys.value = true;
  try {
    const res = await adminApi.getSiteKeys(keysSite.value.id);
    const data = res.data.data || {};
    inboundKeys.value = (data.keys || []).filter((k: any) => k.direction === 'inbound');
    outboundKeys.value = (data.keys || []).filter((k: any) => k.direction === 'outbound');
  } catch { /* ignore */ }
  loadingKeys.value = false;
}

async function genInboundKey() {
  if (!keysSite.value) return;
  try {
    const res = await adminApi.createInboundKey(keysSite.value.id);
    if (res.data.code === 0) {
      newKeyValue.value = res.data.data.key || '';
      showNewKey.value = true;
      ElMessage.success('已生成联邦 Key');
      await loadSiteKeys();
    } else {
      ElMessage.error(res.data.message || '生成失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成失败');
  }
}

async function copyNewKey() {
  try {
    await navigator.clipboard.writeText(newKeyValue.value);
    ElMessage.success('已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择复制');
  }
}

async function saveOutboundKey() {
  if (!keysSite.value) return;
  const key = outboundKeyInput.value.trim();
  if (!key) { ElMessage.warning('请粘贴对方站签发给本站的完整 Key'); return; }
  savingOutbound.value = true;
  try {
    const res = await adminApi.saveOutboundKey(keysSite.value.id, key);
    if (res.data.code === 0) {
      ElMessage.success(res.data.message || '已保存');
      outboundKeyInput.value = '';
      await loadSiteKeys();
    } else {
      ElMessage.error(res.data.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingOutbound.value = false;
}

async function toggleKey(row: any) {
  if (!keysSite.value) return;
  try {
    await adminApi.toggleSiteKey(keysSite.value.id, row.id);
    ElMessage.success('已更新状态');
    await loadSiteKeys();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function deleteKey(row: any) {
  if (!keysSite.value) return;
  try {
    await ElMessageBox.confirm('吊销后该 Key 立即失效（对方站点将无法继续使用）。确认吊销？', '吊销 Key', { type: 'warning' });
    await adminApi.deleteSiteKey(keysSite.value.id, row.id);
    ElMessage.success('已吊销');
    await loadSiteKeys();
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
.key-block { margin-bottom: 18px; padding: 14px; border: 1px solid var(--border-color, #e5e7eb); border-radius: 8px; }
.key-block-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.key-block-head h4 { margin: 0; font-size: 0.98rem; color: var(--text-color, #333); }
.key-block code { background: #eef2ff; color: #4f46e5; padding: 1px 6px; border-radius: 4px; font-size: 0.82rem; }
.newkey-box { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.newkey-code { flex: 1; font-size: 0.95rem; word-break: break-all; background: #eef2ff; color: #4f46e5; padding: 10px 12px; border-radius: 6px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
[data-theme="dark"] .inbound-head h3 { color: #e2e8f0; }
[data-theme="dark"] .inbound-block { border-color: #334155; }
[data-theme="dark"] .key-block { border-color: #334155; }
[data-theme="dark"] .key-block-head h4 { color: #e2e8f0; }
</style>
