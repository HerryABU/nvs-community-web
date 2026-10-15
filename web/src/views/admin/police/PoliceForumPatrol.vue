<template>
  <div class="pfp">
    <el-alert
      type="info"
      :closable="false"
      show-icon
      title="论坛巡查（警方取证）"
      description="此处列出全部论坛，含隐藏（不可见）、加密（有密码）、待审 / 驳回及已软删除的论坛。点击「进入查看」可越过房间号 / 密码门禁直接阅览（后端已对警方 / 管理员豁免），无需房间号。"
      style="margin-bottom: 14px"
    />

    <div class="pfp-stats">
      <div class="pfp-stat"><span class="pfp-stat-num">{{ rows.length }}</span><span class="pfp-stat-label">论坛总数</span></div>
      <div class="pfp-stat is-hidden"><span class="pfp-stat-num">{{ countOf(r => !r.is_public) }}</span><span class="pfp-stat-label">隐藏论坛</span></div>
      <div class="pfp-stat is-lock"><span class="pfp-stat-num">{{ countOf(r => r.has_password) }}</span><span class="pfp-stat-label">加密论坛</span></div>
      <div class="pfp-stat is-del"><span class="pfp-stat-num">{{ countOf(r => r.is_deleted) }}</span><span class="pfp-stat-label">已删除</span></div>
    </div>

    <div class="pfp-toolbar">
      <el-input
        v-model="keyword"
        placeholder="按论坛名 / 房间号 / 创建者搜索"
        clearable
        style="width: 280px"
        :prefix-icon="Search"
      />
      <el-select v-model="visFilter" placeholder="可见性" clearable style="width: 150px">
        <el-option label="公开" value="public" />
        <el-option label="隐藏" value="hidden" />
        <el-option label="加密" value="password" />
      </el-select>
      <el-button type="primary" :loading="loading" @click="load">
        <el-icon><Refresh /></el-icon> 刷新
      </el-button>
    </div>

    <el-table :data="filtered" v-loading="loading" stripe style="width: 100%">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="论坛名称" min-width="180">
        <template #default="{ row }">
          <span class="pfp-name">{{ row.name }}</span>
          <el-tag v-if="row.is_deleted" size="small" type="info" effect="plain" class="pfp-flag">已删除</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="type" label="类型" width="110">
        <template #default="{ row }"><el-tag size="small" :type="typeTag(row.type)">{{ row.type }}</el-tag></template>
      </el-table-column>
      <el-table-column label="可见性" width="150">
        <template #default="{ row }">
          <el-tag size="small" :type="row.is_public ? 'success' : 'warning'" effect="plain">
            {{ row.is_public ? '公开' : '隐藏' }}
          </el-tag>
          <el-tag v-if="row.has_password" size="small" type="danger" effect="plain" class="pfp-flag">加密</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="room_code" label="房间号" width="100">
        <template #default="{ row }">
          <span v-if="row.room_code" class="pfp-room">{{ row.room_code }}</span>
          <span v-else class="pfp-muted">—</span>
        </template>
      </el-table-column>
      <el-table-column prop="creator_name" label="创建者" width="120">
        <template #default="{ row }">{{ row.creator_name || (row.creator_id ? '用户#' + row.creator_id : '系统') }}</template>
      </el-table-column>
      <el-table-column prop="review_status" label="审核" width="90">
        <template #default="{ row }"><el-tag size="small" :type="reviewTag(row.review_status)">{{ reviewLabel(row.review_status) }}</el-tag></template>
      </el-table-column>
      <el-table-column label="帖子/回复" width="100">
        <template #default="{ row }">{{ row.thread_total }} / {{ row.post_total }}</template>
      </el-table-column>
      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="primary" plain @click="enter(row)">进入查看</el-button>
        </template>
      </el-table-column>
      <template #empty><el-empty description="暂无论坛" /></template>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';
import { Search, Refresh } from '@element-plus/icons-vue';

interface ForumRow {
  id: number;
  name: string;
  type: string;
  is_public: boolean;
  has_password: boolean;
  room_code: string;
  creator_id: number;
  creator_name?: string;
  review_status: string;
  is_deleted: boolean;
  thread_total: number;
  post_total: number;
}

const router = useRouter();
const rows = ref<ForumRow[]>([]);
const loading = ref(false);
const keyword = ref('');
const visFilter = ref('');

function countOf(pred: (r: ForumRow) => boolean) {
  return rows.value.filter(pred).length;
}

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  return rows.value.filter((r) => {
    if (visFilter.value === 'public' && !r.is_public) return false;
    if (visFilter.value === 'hidden' && r.is_public) return false;
    if (visFilter.value === 'password' && !r.has_password) return false;
    if (kw) {
      const hay = `${r.name} ${r.room_code || ''} ${r.creator_name || ''}`.toLowerCase();
      if (!hay.includes(kw)) return false;
    }
    return true;
  });
});

function typeTag(t: string) {
  const m: Record<string, string> = { sensitive: 'danger', custom: 'warning', general: '', reader: 'success' };
  return m[t] ?? '';
}
function reviewLabel(s: string) {
  const m: Record<string, string> = { approved: '通过', pending: '待审', rejected: '驳回' };
  return m[s] || s || '通过';
}
function reviewTag(s: string) {
  const m: Record<string, string> = { approved: 'success', pending: 'warning', rejected: 'danger' };
  return m[s] ?? 'success';
}

async function load() {
  loading.value = true;
  try {
    const res = await adminApi.policeListForums();
    if (res.data.code === 0) {
      rows.value = res.data.data?.forums || [];
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载论坛失败');
  } finally {
    loading.value = false;
  }
}

// 进入查看：警方/管理员由后端豁免门禁，直接跳论坛详情页即可
function enter(row: ForumRow) {
  if (row.is_deleted) {
    ElMessage.warning('该论坛已被软删除，仅供巡查留证');
  }
  router.push(`/forum/${row.id}`);
}

onMounted(load);
</script>

<style scoped>
.pfp-stats {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}
.pfp-stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 96px;
  padding: 10px 16px;
  border-radius: 10px;
  background: var(--bg-subtle, #f8fafc);
  border: 1px solid var(--border-color, #e2e8f0);
}
.pfp-stat-num { font-size: 1.4rem; font-weight: 700; color: var(--primary-color, #4f46e5); }
.pfp-stat-label { font-size: 0.78rem; color: var(--text-light, #999); margin-top: 2px; }
.pfp-stat.is-hidden .pfp-stat-num { color: #e6a23c; }
.pfp-stat.is-lock .pfp-stat-num { color: #d03050; }
.pfp-stat.is-del .pfp-stat-num { color: #909399; }

.pfp-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; flex-wrap: wrap; }
.pfp-name { font-weight: 600; }
.pfp-room { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 1px; }
.pfp-muted { color: var(--text-light, #bbb); }
.pfp-flag { margin-left: 6px; }

[data-theme="dark"] .pfp-stat { background: rgba(255,255,255,.03); border-color: rgba(255,255,255,.08); }
</style>
