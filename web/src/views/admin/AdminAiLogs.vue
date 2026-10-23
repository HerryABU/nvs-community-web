<template>
  <div class="page">
    <AdminHeader title="AI 数据审计" desc="会话、作品与调用流水只读查询（用于排查与对账）">
      <template #actions>
        <el-button size="small" :loading="loading" @click="reload">刷新</el-button>
      </template>
    </AdminHeader>

    <!-- 概览 -->
    <div class="stat-grid">
      <div v-for="s in statCards" :key="s.key" class="stat-card">
        <div class="stat-label">{{ s.label }}</div>
        <div class="stat-value">{{ fmt(stats[s.key] || 0) }}</div>
      </div>
    </div>

    <el-tabs v-model="tab" class="audit-tabs" @tab-change="onTabChange">
      <!-- 对话数据 -->
      <el-tab-pane label="对话数据" name="sessions">
        <div class="filter-row">
          <el-input
            v-model="userFilter"
            size="small"
            placeholder="按用户 ID 过滤（留空=全部）"
            style="width:220px"
            clearable
            @keyup.enter="loadSessions(1)"
            @clear="loadSessions(1)"
          />
          <el-button size="small" @click="loadSessions(1)">查询</el-button>
          <span class="muted small">共 {{ sessions.total }} 条</span>
        </div>
        <el-table :data="sessions.list" size="small" v-loading="loading">
          <el-table-column prop="id" label="会话" width="72" />
          <el-table-column prop="user_id" label="用户" width="80" />
          <el-table-column prop="title" label="标题" min-width="180" show-overflow-tooltip />
          <el-table-column label="智能体" width="120">
            <template #default="{ row }">{{ row.agent_name || '—' }}</template>
          </el-table-column>
          <el-table-column prop="messages" label="消息数" width="80" />
          <el-table-column label="状态" width="130">
            <template #default="{ row }">
              <el-tag v-if="row.shared" size="small" type="success">已分享</el-tag>
              <el-tag v-if="row.coze_conv" size="small" type="warning" style="margin-left:4px">Coze</el-tag>
              <span v-if="!row.shared && !row.coze_conv" class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column prop="updated_at" label="更新时间" width="170">
            <template #default="{ row }">{{ time(row.updated_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ row }">
              <el-button text type="primary" size="small" @click="openSession(row)">查看</el-button>
            </template>
          </el-table-column>
          <template #empty><EmptyState icon="💬" text="暂无会话" /></template>
        </el-table>
        <div class="pager">
          <el-pagination
            layout="prev, pager, next"
            :total="sessions.total"
            :page-size="20"
            :current-page="sessions.page"
            @current-change="loadSessions"
          />
        </div>
      </el-tab-pane>

      <!-- 绘画 / 视频数据 -->
      <el-tab-pane label="绘画 / 视频" name="media">
        <div class="filter-row">
          <el-radio-group v-model="mediaKind" size="small" @change="loadMedia(1)">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="image">图像</el-radio-button>
            <el-radio-button value="video">视频</el-radio-button>
          </el-radio-group>
          <el-select v-model="mediaStatus" size="small" style="width:140px" clearable placeholder="状态" @change="loadMedia(1)">
            <el-option label="成功" value="succeeded" />
            <el-option label="失败" value="failed" />
            <el-option label="进行中" value="running" />
            <el-option label="待处理" value="pending" />
          </el-select>
          <span class="muted small">共 {{ media.total }} 条</span>
        </div>
        <el-table :data="media.list" size="small" v-loading="loading">
          <el-table-column prop="id" label="任务" width="72" />
          <el-table-column prop="user_id" label="用户" width="80" />
          <el-table-column label="类型" width="72">
            <template #default="{ row }">{{ row.kind === 'video' ? '视频' : '图像' }}</template>
          </el-table-column>
          <el-table-column prop="model_name" label="模型" width="130" show-overflow-tooltip />
          <el-table-column prop="prompt" label="提示词" min-width="200" show-overflow-tooltip />
          <el-table-column label="状态" width="96">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === 'succeeded' ? 'success' : row.status === 'failed' ? 'danger' : 'info'">
                {{ row.status }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="cost" label="消耗" width="80" />
          <el-table-column label="公开" width="70">
            <template #default="{ row }">{{ row.is_public ? '是' : '否' }}</template>
          </el-table-column>
          <el-table-column prop="created_at" label="时间" width="170">
            <template #default="{ row }">{{ time(row.created_at) }}</template>
          </el-table-column>
          <template #empty><EmptyState icon="🎨" text="暂无作品" /></template>
        </el-table>
        <div class="pager">
          <el-pagination
            layout="prev, pager, next"
            :total="media.total"
            :page-size="20"
            :current-page="media.page"
            @current-change="loadMedia"
          />
        </div>
      </el-tab-pane>

      <!-- 调用流水 -->
      <el-tab-pane label="调用流水" name="calls">
        <div class="filter-row">
          <el-select v-model="callStatus" size="small" style="width:140px" clearable placeholder="状态" @change="loadCalls(1)">
            <el-option label="已结算" value="settled" />
            <el-option label="待结算" value="pending" />
            <el-option label="失败" value="failed" />
            <el-option label="已退款" value="refunded" />
          </el-select>
          <span class="muted small">共 {{ calls.total }} 条</span>
        </div>
        <el-table :data="calls.list" size="small" v-loading="loading">
          <el-table-column prop="id" label="#" width="72" />
          <el-table-column prop="user_id" label="用户" width="80" />
          <el-table-column prop="model_name" label="模型" width="130" show-overflow-tooltip />
          <el-table-column prop="action" label="动作" width="100" />
          <el-table-column label="tokens" width="120">
            <template #default="{ row }">{{ row.tokens_in }}/{{ row.tokens_out }}</template>
          </el-table-column>
          <el-table-column prop="cost" label="实扣" width="80" />
          <el-table-column label="免费" width="70">
            <template #default="{ row }">{{ row.free_quota ? '是' : '否' }}</template>
          </el-table-column>
          <el-table-column label="状态" width="96">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === 'failed' ? 'danger' : row.status === 'settled' ? 'success' : 'info'">
                {{ row.status }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="时间" width="170">
            <template #default="{ row }">{{ time(row.created_at) }}</template>
          </el-table-column>
          <template #empty><EmptyState icon="📄" text="暂无流水" /></template>
        </el-table>
        <div class="pager">
          <el-pagination
            layout="prev, pager, next"
            :total="calls.total"
            :page-size="20"
            :current-page="calls.page"
            @current-change="loadCalls"
          />
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 会话详情 -->
    <el-drawer v-model="detailVisible" title="会话详情" size="620px">
      <div v-if="detail.session" class="detail-head">
        <div><b>{{ detail.session.title || '新对话' }}</b></div>
        <div class="muted small">
          用户 {{ detail.session.user_id }} · 智能体 {{ detail.session.agent_name || '—' }} ·
          {{ time(detail.session.updated_at) }}
        </div>
      </div>
      <div class="detail-body">
        <div v-for="m in detail.messages" :key="m.id" class="d-msg" :class="{ mine: m.role === 'user' }">
          <div class="d-role">{{ m.role === 'user' ? '用户' : 'AI' }}</div>
          <pre class="d-text">{{ m.content }}</pre>
          <div class="muted small">
            {{ m.tokens_in }}/{{ m.tokens_out }} tokens · 消耗 {{ m.cost }}
          </div>
        </div>
        <EmptyState v-if="!detail.messages.length" icon="📭" text="该会话没有消息" />
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import { aiApi } from '@/api/ai';

const tab = ref<'sessions' | 'media' | 'calls'>('sessions');
const loading = ref(false);
const stats = ref<Record<string, number>>({});

const statCards = [
  { key: 'sessions', label: '会话总数' },
  { key: 'messages', label: '消息总数' },
  { key: 'sessions_24h', label: '24h 新会话' },
  { key: 'images', label: '图像任务' },
  { key: 'videos', label: '视频任务' },
  { key: 'calls', label: '调用次数' },
  { key: 'cost_total', label: '累计消耗' },
  { key: 'gallery_public', label: '公开作品' },
];

const sessions = reactive({ list: [] as any[], total: 0, page: 1 });
const media = reactive({ list: [] as any[], total: 0, page: 1 });
const calls = reactive({ list: [] as any[], total: 0, page: 1 });

const userFilter = ref('');
const mediaKind = ref('');
const mediaStatus = ref('');
const callStatus = ref('');

function fmt(n: number) {
  if (n >= 100000000) return `${(n / 100000000).toFixed(1)}亿`;
  if (n >= 10000) return `${(n / 10000).toFixed(1)}万`;
  return String(n);
}
function time(s: string) {
  if (!s) return '—';
  try {
    return new Date(s).toLocaleString();
  } catch {
    return s;
  }
}

async function loadStats() {
  try {
    const res = await aiApi.aiAuditStats();
    if (res.data.code === 0) stats.value = res.data.data || {};
  } catch { /* ignore */ }
}

async function loadSessions(page = 1) {
  loading.value = true;
  try {
    const uid = userFilter.value.trim() ? Number(userFilter.value.trim()) : undefined;
    const res = await aiApi.aiAuditSessions({ user_id: uid, page });
    if (res.data.code === 0) {
      sessions.list = res.data.data.list || [];
      sessions.total = res.data.data.total;
      sessions.page = page;
    }
  } catch {
    ElMessage.error('查询失败');
  }
  loading.value = false;
}

async function loadMedia(page = 1) {
  loading.value = true;
  try {
    const res = await aiApi.aiAuditMedia({
      kind: mediaKind.value || undefined,
      status: mediaStatus.value || undefined,
      page,
    });
    if (res.data.code === 0) {
      media.list = res.data.data.list || [];
      media.total = res.data.data.total;
      media.page = page;
    }
  } catch {
    ElMessage.error('查询失败');
  }
  loading.value = false;
}

async function loadCalls(page = 1) {
  loading.value = true;
  try {
    const res = await aiApi.aiAuditCalls({ status: callStatus.value || undefined, page });
    if (res.data.code === 0) {
      calls.list = res.data.data.list || [];
      calls.total = res.data.data.total;
      calls.page = page;
    }
  } catch {
    ElMessage.error('查询失败');
  }
  loading.value = false;
}

function onTabChange(name: any) {
  if (name === 'sessions') loadSessions(1);
  else if (name === 'media') loadMedia(1);
  else loadCalls(1);
}

function reload() {
  loadStats();
  onTabChange(tab.value);
}

const detailVisible = ref(false);
const detail = reactive<{ session: any | null; messages: any[] }>({ session: null, messages: [] });

async function openSession(row: any) {
  detail.session = null;
  detail.messages = [];
  detailVisible.value = true;
  try {
    const res = await aiApi.aiAuditSessionMessages(row.id);
    if (res.data.code === 0) {
      detail.session = res.data.data.session;
      detail.messages = res.data.data.messages || [];
    }
  } catch {
    ElMessage.error('读取会话失败');
  }
}

onMounted(() => {
  loadStats();
  loadSessions(1);
});
</script>

<style scoped>
.page { max-width: 1200px; }
.stat-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 12px; margin: 14px 0 6px;
}
.stat-card {
  background: var(--el-bg-color, #fff); border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 12px; padding: 12px 14px;
}
.stat-label { font-size: 0.74rem; color: var(--text-light, #888); }
.stat-value { font-size: 1.25rem; font-weight: 700; color: var(--text-color, #222); margin-top: 2px; }
.audit-tabs { margin-top: 8px; }
.filter-row { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; flex-wrap: wrap; }
.pager { display: flex; justify-content: flex-end; padding: 10px 0; }
.detail-head { padding-bottom: 10px; border-bottom: 1px solid var(--el-border-color-lighter, #eee); margin-bottom: 10px; }
.detail-body { display: flex; flex-direction: column; gap: 12px; }
.d-msg { border-left: 3px solid var(--el-border-color, #e5e7eb); padding-left: 10px; }
.d-msg.mine { border-left-color: var(--el-color-primary, #409eff); }
.d-role { font-size: 0.72rem; color: var(--text-light, #888); margin-bottom: 3px; }
.d-text {
  margin: 0 0 4px; white-space: pre-wrap; word-break: break-word;
  font-size: 0.82rem; line-height: 1.65; color: var(--text-color, #333);
  background: var(--el-fill-color-light, #f8f9fa); padding: 8px 10px; border-radius: 8px;
}
.muted { color: var(--text-light, #999); }
.small { font-size: 0.75rem; }
</style>
