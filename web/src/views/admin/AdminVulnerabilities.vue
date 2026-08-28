<template>
  <div class="admin-page">
    <AdminHeader title="漏洞管理" subtitle="漏洞隐患整改闭环留痕 · 176号令第七条第六项" :icon="Warning" />
    <div class="admin-page-body admin-panel">
      <el-row :gutter="12" class="stats-row">
        <el-col :span="5" v-for="(v, k) in statDefs" :key="k">
          <div class="stat-card" :style="{ borderColor: v.color }">
            <div class="stat-num" :style="{ color: v.color }">{{ stats[k] ?? 0 }}</div>
            <div class="stat-label">{{ v.label }}</div>
          </div>
        </el-col>
      </el-row>

      <div class="toolbar">
        <el-select v-model="filter.status" placeholder="状态筛选" clearable style="width: 140px" @change="load">
          <el-option label="待整改" value="pending" />
          <el-option label="整改中" value="fixing" />
          <el-option label="已整改" value="fixed" />
          <el-option label="已忽略" value="ignored" />
        </el-select>
        <el-button type="primary" @click="openCreate">+ 新建漏洞记录</el-button>
        <el-button type="success" plain @click="exportCsv">导出报告 CSV</el-button>
      </div>

      <el-table :data="list" border stripe size="small">
        <el-table-column prop="name" label="漏洞名称" min-width="160" show-overflow-tooltip />
        <el-table-column label="严重程度" width="90">
          <template #default="{ row }">
            <el-tag :type="sevTag(row.severity)" size="small">{{ sevCN(row.severity) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ statusCN(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="affected" label="影响范围" min-width="120" show-overflow-tooltip />
        <el-table-column prop="reported_by" label="报告者" width="110" show-overflow-tooltip />
        <el-table-column prop="created_at" label="发现时间" width="150">
          <template #default="{ row }">{{ fmt(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-popconfirm title="确认删除该漏洞记录？" @confirm="remove(row.id)">
              <template #reference><el-button size="small" type="danger" plain>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <el-dialog v-model="dlg.visible" :title="dlg.id ? '编辑漏洞记录' : '新建漏洞记录'" width="560px">
        <el-form label-width="90px">
          <el-form-item label="漏洞名称" required><el-input v-model="dlg.form.name" maxlength="256" /></el-form-item>
          <el-form-item label="严重程度" required>
            <el-select v-model="dlg.form.severity" style="width: 100%">
              <el-option label="严重" value="critical" />
              <el-option label="高危" value="high" />
              <el-option label="中危" value="medium" />
              <el-option label="低危" value="low" />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="dlg.form.status" style="width: 100%">
              <el-option label="待整改" value="pending" />
              <el-option label="整改中" value="fixing" />
              <el-option label="已整改" value="fixed" />
              <el-option label="已忽略（不构成风险）" value="ignored" />
            </el-select>
          </el-form-item>
          <el-form-item label="影响范围"><el-input v-model="dlg.form.affected" placeholder="如：登录接口 / 所有用户" /></el-form-item>
          <el-form-item label="报告者"><el-input v-model="dlg.form.reported_by" placeholder="如：安全研究员 / 内测发现" /></el-form-item>
          <el-form-item label="漏洞描述"><el-input v-model="dlg.form.description" type="textarea" :rows="4" /></el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="dlg.visible = false">取消</el-button>
          <el-button type="primary" :loading="dlg.saving" @click="saveDlg">保存</el-button>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { Warning } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const list = ref<any[]>([]);
const stats = ref<Record<string, number>>({});
const filter = reactive({ status: '' });

const statDefs = [
  { key: 'total', label: '全部', color: '#409eff' },
  { key: 'pending', label: '待整改', color: '#e6a23c' },
  { key: 'fixing', label: '整改中', color: '#f56c6c' },
  { key: 'fixed', label: '已整改', color: '#67c23a' },
  { key: 'ignored', label: '已忽略', color: '#909399' },
];

const dlg = reactive({ visible: false, id: 0, saving: false, form: {} as any });

function sevCN(s: string) { return { critical: '严重', high: '高危', medium: '中危', low: '低危' }[s] || s; }
function statusCN(s: string) { return { pending: '待整改', fixing: '整改中', fixed: '已整改', ignored: '已忽略' }[s] || s; }
function sevTag(s: string) { return { critical: 'danger', high: 'warning', medium: 'primary', low: 'info' }[s] || 'info'; }
function statusTag(s: string) { return { pending: 'warning', fixing: 'danger', fixed: 'success', ignored: 'info' }[s] || 'info'; }
function fmt(t: string) { return t ? t.slice(0, 16).replace('T', ' ') : ''; }

async function load() {
  try {
    const res = await adminApi.getVulnerabilities(filter.status ? { status: filter.status } : undefined);
    list.value = res.data.data?.list || [];
    stats.value = res.data.data?.stats || {};
  } catch { /* ignore */ }
}

function openCreate() {
  dlg.id = 0;
  dlg.form = { name: '', severity: 'high', status: 'pending', affected: '', reported_by: '', description: '' };
  dlg.visible = true;
}
function openEdit(row: any) {
  dlg.id = row.id;
  dlg.form = { name: row.name, severity: row.severity, status: row.status, affected: row.affected, reported_by: row.reported_by, description: row.description };
  dlg.visible = true;
}
async function saveDlg() {
  if (!dlg.form.name) { ElMessage.warning('请填写漏洞名称'); return; }
  dlg.saving = true;
  try {
    if (dlg.id) {
      await adminApi.updateVulnerability(dlg.id, dlg.form);
    } else {
      await adminApi.createVulnerability(dlg.form);
    }
    ElMessage.success('已保存');
    dlg.visible = false;
    load();
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '保存失败'); }
  finally { dlg.saving = false; }
}
async function remove(id: number) {
  try {
    await adminApi.deleteVulnerability(id);
    ElMessage.success('已删除');
    load();
  } catch { ElMessage.error('删除失败'); }
}
async function exportCsv() {
  try {
    const res = await adminApi.exportVulnerabilities();
    const blob = new Blob([res.data], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `vulnerabilities_${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  } catch { ElMessage.error('导出失败'); }
}

onMounted(load);
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: 1100px; }
.stats-row { margin-bottom: 14px; }
.stat-card { border: 1px solid var(--border-color); border-top: 3px solid; border-radius: 8px; padding: 10px 14px; background: var(--bg-card, #fff); }
.stat-num { font-size: 1.4rem; font-weight: 700; }
.stat-label { font-size: 0.8rem; color: var(--text-light, #999); }
.toolbar { display: flex; gap: 10px; margin-bottom: 12px; align-items: center; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .stat-card { background: #1e293b; }
</style>
