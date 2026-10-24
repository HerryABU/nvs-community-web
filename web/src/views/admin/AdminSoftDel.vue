<template>
  <div class="admin-shell">
    <div class="admin-container">
      <AdminHeader
        title="内容留证（回收站）"
        subtitle="软删除留证记录 · softdel_ 表 · 所有删除/封禁操作可追溯、可查证"
        :icon="Box"
        color="var(--gradient-accent)"
      >
        <template #actions>
          <el-button :icon="Refresh" :loading="loading" @click="load">刷新</el-button>
        </template>
      </AdminHeader>

      <div class="admin-panel">
        <el-alert
          type="info"
          :closable="false"
          show-icon
          style="margin-bottom: 16px"
          title="留证说明"
          description="管理员删除评论、封禁评论/用户/小说/章节，以及内容抢救台、举报处置的删除操作，都会先将完整内容快照归档到 softdel_ 系列表，再软删除原始记录。此处可查看全部留证记录，点击「详情」查看原始快照。"
        />

        <div class="admin-toolbar">
          <el-radio-group v-model="filterType" @change="load">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="comment">评论</el-radio-button>
            <el-radio-button value="novel">小说</el-radio-button>
            <el-radio-button value="chapter">章节</el-radio-button>
            <el-radio-button value="user">用户</el-radio-button>
            <el-radio-button value="thread">帖子</el-radio-button>
            <el-radio-button value="post">回复</el-radio-button>
            <el-radio-button value="blog">博客</el-radio-button>
            <el-radio-button value="forum">论坛</el-radio-button>
          </el-radio-group>
        </div>

        <el-table :data="records" v-loading="loading" style="width: 100%">
          <el-table-column label="类型" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="typeTag(row.type)">{{ typeLabel(row.type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="原记录 ID" width="100">
            <template #default="{ row }"><code>#{{ row.original_id }}</code></template>
          </el-table-column>
          <el-table-column label="摘要" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ row.summary || '—' }}</template>
          </el-table-column>
          <el-table-column label="操作者" width="120">
            <template #default="{ row }">{{ row.operator || `#${row.deleted_by}` }}</template>
          </el-table-column>
          <el-table-column label="来源" width="90">
            <template #default="{ row }">
              <el-tag size="small" type="info" effect="plain">{{ sourceLabel(row.source) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="原因" min-width="140" show-overflow-tooltip>
            <template #default="{ row }">{{ row.reason || '—' }}</template>
          </el-table-column>
          <el-table-column label="删除时间" width="170">
            <template #default="{ row }">{{ formatTime(row.deleted_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="primary" @click="openSnapshot(row)">详情</el-button>
            </template>
          </el-table-column>
          <template #empty><el-empty description="暂无留证记录" /></template>
        </el-table>
      </div>
    </div>

    <!-- 快照详情弹窗 -->
    <el-dialog v-model="showSnapshot" title="留证快照详情" width="640px">
      <el-descriptions :column="2" border size="small" v-if="current">
        <el-descriptions-item label="类型">{{ typeLabel(current.type) }}</el-descriptions-item>
        <el-descriptions-item label="原记录 ID">#{{ current.original_id }}</el-descriptions-item>
        <el-descriptions-item label="操作者">{{ current.operator || `#${current.deleted_by}` }}</el-descriptions-item>
        <el-descriptions-item label="来源">{{ sourceLabel(current.source) }}</el-descriptions-item>
        <el-descriptions-item label="删除时间">{{ formatTime(current.deleted_at) }}</el-descriptions-item>
        <el-descriptions-item label="处置原因">{{ current.reason || '—' }}</el-descriptions-item>
      </el-descriptions>
      <div class="sd-snap-label">完整快照（JSON）</div>
      <pre class="sd-snap">{{ prettySnapshot }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { Box, Refresh } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import StateBlock from '@/components/common/StateBlock.vue';

const records = ref<any[]>([]);
const loading = ref(false);
const filterType = ref('');
const showSnapshot = ref(false);
const current = ref<any>(null);

const prettySnapshot = computed(() => {
  if (!current.value?.snapshot) return '（无快照）';
  try {
    return JSON.stringify(JSON.parse(current.value.snapshot), null, 2);
  } catch {
    return current.value.snapshot;
  }
});

function typeLabel(t: string) {
  const map: Record<string, string> = {
    comment: '评论', novel: '小说', chapter: '章节', user: '用户',
    thread: '帖子', post: '回复', blog: '博客', forum: '论坛',
  };
  return map[t] || t;
}
function typeTag(t: string) {
  const map: Record<string, string> = {
    comment: 'success', novel: 'primary', chapter: 'info', user: 'danger',
    thread: 'warning', post: 'info', blog: 'primary', forum: 'warning',
  };
  return (map[t] as any) || 'info';
}
function sourceLabel(s: string) {
  const map: Record<string, string> = {
    admin: '管理员', emergency: '抢救台', report: '举报处置', moderator: '版主',
  };
  return map[s] || s;
}
function formatTime(s?: string) {
  return s ? new Date(s).toLocaleString('zh-CN') : '';
}

async function load() {
  loading.value = true;
  try {
    const res = await adminApi.getSoftDel(filterType.value || undefined);
    records.value = res.data.data || [];
  } catch { /* ignore */ }
  loading.value = false;
}

function openSnapshot(row: any) {
  current.value = row;
  showSnapshot.value = true;
}

onMounted(load);
</script>

<style scoped>
code {
  background: #eef2ff;
  color: #4f46e5;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 0.85rem;
}
.sd-snap-label {
  margin: 16px 0 8px;
  font-weight: 600;
  color: var(--text-color, #333);
}
.sd-snap {
  margin: 0;
  padding: 12px 14px;
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 8px;
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 0.78rem;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 360px;
  overflow: auto;
}
[data-theme="dark"] code {
  background: #1e293b;
  color: #93c5fd;
}
[data-theme="dark"] .sd-snap-label {
  color: #e2e8f0;
}
</style>
