<template>
  <div class="admin-page">
    <AdminHeader title="警方取管台" subtitle="取证留证 · 软删除查看 · 凭证下载（176号令合规）" :icon="Stamp">
      <template #actions>
        <el-button type="success" :loading="exporting" @click="doExport">
          <el-icon><Download /></el-icon> 导出全部取证包
        </el-button>
        <el-button @click="$router.push(authStore.user?.role === 'admin' ? '/admin' : '/')">
          返回
        </el-button>
      </template>
    </AdminHeader>

    <el-alert
      type="warning"
      :closable="false"
      show-icon
      title="权限说明"
      description="本台为警方取证专用：可查看全部软删除留证记录（softdel_*）并下载完整取证凭证。普通管理功能（用户封禁、内容治理等）不在此处。取证导出含软删除留证 + 隔离墙访问 + Web 访问 + 用户审计，便于提前准备取证材料。"
      style="margin-bottom: 16px"
    />

    <el-tabs v-model="activeTab" type="border-card" class="pc-tabs">
      <el-tab-pane label="软删除留证" name="softdel">
      <div class="pc-toolbar">
      <el-select v-model="filterType" placeholder="全部类型" clearable style="width: 200px" @change="loadList">
        <el-option v-for="t in types" :key="t" :label="typeLabel(t)" :value="t" />
      </el-select>
      <el-button type="primary" :loading="loading" @click="loadList">
        <el-icon><Search /></el-icon> 查询
      </el-button>
      <span class="pc-count">共 {{ list.length }} 条留证</span>
      <div class="pc-color">
        <span class="pc-color-label">已删除高亮色</span>
        <el-color-picker :model-value="delColor" @change="onColorChange" :disabled="!canEditColor" />
        <span class="pc-color-hint" v-if="!canEditColor">（仅管理员可修改）</span>
      </div>
    </div>

    <el-table :data="list" v-loading="loading" stripe style="width: 100%">
      <el-table-column prop="type" label="类型" width="110">
        <template #default="{ row }"><el-tag size="small">{{ typeLabel(row.type) }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="original_id" label="原ID" width="90" />
      <el-table-column prop="operator" label="处置人" width="130" />
      <el-table-column prop="source" label="来源" width="100" />
      <el-table-column prop="reason" label="原因" min-width="180" show-overflow-tooltip>
        <template #default="{ row }"><span :style="{ color: delColor }">{{ row.reason }}</span></template>
      </el-table-column>
      <el-table-column prop="summary" label="摘要" min-width="200" show-overflow-tooltip>
        <template #default="{ row }"><span :style="{ color: delColor }">{{ row.summary }}</span></template>
      </el-table-column>
      <el-table-column prop="deleted_at" label="删除时间" width="170">
        <template #default="{ row }">{{ formatDate(row.deleted_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="primary" plain @click="openDetail(row)">详情</el-button>
          <el-button size="small" type="success" plain @click="doDownload(row)">
            <el-icon><Download /></el-icon> 凭证
          </el-button>
        </template>
      </el-table-column>
      </el-table>
      </el-tab-pane>
      <el-tab-pane label="论坛巡查" name="forums" lazy>
        <PoliceForumPatrol />
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="detailVisible" title="取证凭证详情" width="720px">
      <div v-if="detail">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="类型">{{ typeLabel(detail.type) }}</el-descriptions-item>
          <el-descriptions-item label="原ID">{{ detail.original_id }}</el-descriptions-item>
          <el-descriptions-item label="处置人">{{ detail.operator }}</el-descriptions-item>
          <el-descriptions-item label="来源">{{ detail.source }}</el-descriptions-item>
          <el-descriptions-item label="原因" :span="2"><span :style="{ color: delColor }">{{ detail.reason }}</span></el-descriptions-item>
          <el-descriptions-item label="删除时间" :span="2">{{ formatDate(detail.deleted_at) }}</el-descriptions-item>
        </el-descriptions>
        <div class="pc-snapshot-title">完整快照（JSON）</div>
        <pre class="pc-snapshot">{{ prettySnapshot(detail.snapshot) }}</pre>
      </div>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
        <el-button type="success" :loading="dlLoading" @click="detail && doDownload(detail, true)">
          <el-icon><Download /></el-icon> 下载凭证
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import api from '@/api';
import { useAuthStore } from '@/stores/auth';
import { ElMessage } from 'element-plus';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import PoliceForumPatrol from './police/PoliceForumPatrol.vue';
import { Stamp, Search, Download } from '@element-plus/icons-vue';

const DefaultPoliceDelColor = '#ff4d4f';

interface SoftDelRow {
  id: number;
  type: string;
  original_id: number;
  operator: string;
  source: string;
  reason: string;
  summary: string;
  snapshot: string;
  deleted_at: string;
}

const authStore = useAuthStore();
const activeTab = ref('softdel');
const list = ref<SoftDelRow[]>([]);
const loading = ref(false);
const exporting = ref(false);
const dlLoading = ref(false);
const filterType = ref('');
const detailVisible = ref(false);
const detail = ref<SoftDelRow | null>(null);

// 已删除内容高亮色（警方取证台统一使用）：管理员可改，警方可读不可写
const delColor = ref(DefaultPoliceDelColor);
const canEditColor = computed(() => authStore.user?.role === 'admin');

const types = ['comment', 'novel', 'chapter', 'user', 'thread', 'post', 'blog', 'forum'];

function typeLabel(t: string) {
  const m: Record<string, string> = {
    comment: '评论', novel: '小说', chapter: '章节', user: '用户',
    thread: '帖子', post: '回复', blog: '博客', forum: '论坛',
  };
  return m[t] || t;
}

function formatDate(d?: string) {
  if (!d) return '';
  return new Date(d).toLocaleString('zh-CN');
}

function prettySnapshot(s?: string) {
  if (!s) return '';
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

async function loadList() {
  loading.value = true;
  try {
    const res = await adminApi.policeListSoftDel({
      type: filterType.value || undefined,
      limit: 1000,
    });
    if (res.data.code === 0) {
      list.value = res.data.data || [];
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

async function openDetail(row: SoftDelRow) {
  detail.value = row;
  detailVisible.value = true;
}

// 加载已删除内容高亮色（警方可读），失败则用默认色
async function loadDelColor() {
  try {
    const res = await api.get('/police/del-color');
    if (res.data.code === 0 && res.data.data?.color) {
      delColor.value = res.data.data.color;
    }
  } catch {
    /* 保持默认色 */
  }
}

// 管理员修改高亮色：PUT 落库；失败还原
async function onColorChange(color?: string) {
  if (!color) {
    loadDelColor();
    return;
  }
  try {
    await api.put('/police/del-color', { color });
    ElMessage.success('删除内容高亮色已更新');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
    loadDelColor();
  }
}

function triggerDownload(blob: Blob, filename: string) {
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  window.URL.revokeObjectURL(url);
}

async function doDownload(row: SoftDelRow, fromDetail = false) {
  if (fromDetail) dlLoading.value = true;
  try {
    // G2：长路由 + 查询串，避免路径参数
    const res = await api.get('/police/download', {
      params: { type: row.type, id: row.id },
      responseType: 'blob',
    });
    const blob = new Blob([res.data], { type: 'application/json' });
    triggerDownload(blob, `evidence_${row.type}_${row.id}.json`);
    ElMessage.success('凭证已下载');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '下载失败');
  } finally {
    if (fromDetail) dlLoading.value = false;
  }
}

async function doExport() {
  exporting.value = true;
  try {
    const res = await adminApi.policeExportPackage();
    const blob = new Blob([res.data], { type: 'application/json' });
    const ts = new Date().toISOString().replace(/[-:T]/g, '').slice(0, 15);
    triggerDownload(blob, `police_evidence_package_${ts}.json`);
    ElMessage.success('取证包已导出');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '导出失败');
  } finally {
    exporting.value = false;
  }
}

onMounted(() => {
  loadList();
  loadDelColor();
});
</script>

<style scoped>
.admin-page { --admin-content-max: 1280px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.pc-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; flex-wrap: wrap; }
.pc-color { display: flex; align-items: center; gap: 8px; margin-left: auto; }
.pc-color-label { color: var(--text-light); font-size: 13px; }
.pc-color-hint { color: var(--text-light); font-size: 12px; }
.pc-tabs { border: none; background: transparent; }
.pc-tabs :deep(.el-tabs__content) { padding: 16px 0 0; }
.pc-count { color: var(--text-light); margin-left: 8px; }
.pc-snapshot-title { margin: 14px 0 6px; font-weight: 600; }
.pc-snapshot { max-height: 360px; overflow: auto; background: #0f172a; color: #e2e8f0; padding: 12px; border-radius: 8px; font-size: 12px; line-height: 1.5; }
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
