<template>
  <div class="admin-page">
    <div class="admin-page-header">
      <el-button text @click="$router.push('/admin')"><el-icon><ArrowLeft /></el-icon> 返回后台</el-button>
      <h2>远程站点互通</h2>
    </div>
    <div class="admin-page-body">
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
import { ArrowLeft } from '@element-plus/icons-vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';

const sites = ref<any[]>([]);
const loadingSites = ref(false);
const showAddSite = ref(false);
const editingSite = ref<any>(null);
const submittingSite = ref(false);
const syncingId = ref(0);
const siteForm = reactive({ name: '', url: '', api_url: '', description: '' });

async function loadSites() {
  loadingSites.value = true;
  try { const res = await adminApi.getSites(); sites.value = res.data.data || []; } catch { /* ignore */ }
  loadingSites.value = false;
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

onMounted(loadSites);
</script>

<style scoped>
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 1100px; }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
</style>
