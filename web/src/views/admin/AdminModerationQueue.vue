<template>
  <div class="admin-page">
    <AdminHeader
      title="审核"
      subtitle="AI 只给结论，最终处置由人决定：放行 / 删除留证 / 封禁留证。违规与存疑的内容会排在待办里。"
      :icon="WarningFilled"
    />
    <div class="admin-page-body">
      <!-- 筛选 -->
      <div class="admin-panel">
        <div class="queue-toolbar">
          <el-radio-group v-model="view" size="small" @change="reload(1)">
            <el-radio-button value="todo">待办（{{ stats.pending_count }}）</el-radio-button>
            <el-radio-button value="all">全部记录</el-radio-button>
          </el-radio-group>
          <el-select v-model="filters.scope" placeholder="内容类型" clearable size="small" style="width: 130px" @change="reload(1)">
            <el-option v-for="(label, v) in MOD_SCOPE_LABELS" :key="v" :label="label" :value="v" />
          </el-select>
          <el-select v-model="filters.verdict" placeholder="AI 结论" clearable size="small" style="width: 120px" @change="reload(1)">
            <el-option v-for="(label, v) in MOD_VERDICT_LABELS" :key="v" :label="label" :value="v" />
          </el-select>
          <el-select v-model="filters.trigger" placeholder="触发来源" clearable size="small" style="width: 130px" @change="reload(1)">
            <el-option v-for="(label, v) in MOD_TRIGGER_LABELS" :key="v" :label="label" :value="v" />
          </el-select>
          <el-button size="small" :icon="Refresh" :loading="loading" @click="reload(page)">刷新</el-button>
          <span class="muted small">共 {{ total }} 条</span>
          <span v-if="statsHint" class="muted small hint-warn">{{ statsHint }}</span>
        </div>

        <el-table :data="rows" v-loading="loading" style="width: 100%">
          <el-table-column label="时间" width="165">
            <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="内容" width="150">
            <template #default="{ row }">
              <div class="cell-content">
                <el-tag size="small" type="info">{{ scopeLabel(row.scope) }}</el-tag>
                <span class="mono">#{{ row.target_id }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="来源" width="150">
            <template #default="{ row }">
              <div class="cell-content">
                <span>{{ triggerLabel(row.trigger) }}</span>
                <el-tag v-if="row.re_triggered" size="small" type="warning">复检</el-tag>
                <el-tag v-if="row.report_count > 0" size="small" type="info">举报 {{ row.report_count }}</el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="AI 结论" width="120">
            <template #default="{ row }">
              <el-tooltip :content="row.model_name || '未记录模型'" placement="top">
                <el-tag size="small" :type="verdictTag(row.verdict)">{{ verdictLabel(row.verdict) }}</el-tag>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column label="命中类别" width="150">
            <template #default="{ row }">
              <el-tag v-for="c in parseCategories(row.categories)" :key="c" size="small" type="danger" class="cat-tag">{{ c }}</el-tag>
              <span v-if="!parseCategories(row.categories).length" class="muted small">—</span>
            </template>
          </el-table-column>
          <el-table-column label="判定说明" min-width="240">
            <template #default="{ row }">
              <div class="reason-text">{{ row.reason || '—' }}</div>
            </template>
          </el-table-column>
          <el-table-column label="处置" width="150">
            <template #default="{ row }">
              <template v-if="isResolved(row)">
                <el-tag size="small" type="success">{{ actionLabel(row.review_action) }}</el-tag>
                <div class="muted small">{{ row.review_note || '已处置' }}</div>
              </template>
              <el-tag v-else size="small" type="warning">待处理</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="230" fixed="right">
            <template #default="{ row }">
              <template v-if="!isResolved(row)">
                <el-button size="small" text type="success" @click="decide(row, 'pass')">放行</el-button>
                <el-button size="small" text type="danger" @click="decide(row, 'delete')">删除</el-button>
                <el-button
                  size="small"
                  text
                  type="danger"
                  :disabled="!canBan(row.scope)"
                  :title="canBan(row.scope) ? '' : '该类型不支持封禁，请用删除'"
                  @click="decide(row, 'ban')"
                >封禁</el-button>
              </template>
              <span v-else class="muted small">{{ fmtTime(row.reviewed_at) }}</span>
            </template>
          </el-table-column>
          <template #empty>
            <StateBlock
              state="empty"
              title="没有待处理的审核记录"
              :description="view === 'todo' ? 'AI 判违规或存疑的内容会自动进这里' : '还没有任何审核记录'"
            />
          </template>
        </el-table>

        <el-pagination
          v-if="total > PAGE_SIZE"
          class="queue-pager"
          layout="prev, pager, next"
          :page-size="PAGE_SIZE"
          :current-page="page"
          :total="total"
          @current-change="reload"
        />
      </div>

      <p class="queue-tip">
        处置动作会<strong>留证</strong>：删除 / 封禁都会先写 <code>softdel_*</code> 快照再软删，可在「内容留证」恢复与追溯。
        「放行」只记录结论，不动内容。
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { WarningFilled, Refresh } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import StateBlock from '@/components/common/StateBlock.vue';
import {
  moderationApi,
  MOD_SCOPE_LABELS,
  MOD_VERDICT_LABELS,
  MOD_TRIGGER_LABELS,
  MOD_ACTION_LABELS,
  type ModerationRecord,
  type ModerationStats,
} from '@/api/moderation';

const PAGE_SIZE = 50;

const view = ref<'todo' | 'all'>('todo');
const rows = ref<ModerationRecord[]>([]);
const total = ref(0);
const page = ref(1);
const loading = ref(false);
const filters = reactive({ scope: '', verdict: '', trigger: '' });
const stats = ref<ModerationStats>({
  total_records: 0, pass_count: 0, fail_count: 0, review_count: 0,
  skipped_count: 0, pending_count: 0, malicious_reporters: 0, banned_reporters: 0, hidden_count: 0,
});
const statsHint = ref('');

const scopeLabel = (v: string) => MOD_SCOPE_LABELS[v] || v;
const verdictLabel = (v: string) => MOD_VERDICT_LABELS[v] || v;
const triggerLabel = (v: string) => MOD_TRIGGER_LABELS[v] || v;
const actionLabel = (v: string) => MOD_ACTION_LABELS[v] || v;

function verdictTag(v: string) {
  if (v === 'fail') return 'danger';
  if (v === 'review') return 'warning';
  if (v === 'pass') return 'success';
  return 'info';
}

/** 只有作品/章节/博客/版块/帖子/回帖/评论支持封禁口径（评论封禁等同删除留证） */
function canBan(scope: string) {
  return ['novel', 'chapter', 'blog', 'thread', 'forum', 'post', 'comment'].includes(scope);
}

function isResolved(r: ModerationRecord) {
  return r.review_status === 'resolved';
}

function parseCategories(raw: string): string[] {
  if (!raw) return [];
  try {
    const v = JSON.parse(raw);
    return Array.isArray(v) ? v.filter((x) => typeof x === 'string' && x) : [];
  } catch {
    return [];
  }
}

function fmtTime(s?: string | null) {
  if (!s) return '—';
  return new Date(s).toLocaleString();
}

async function loadStats() {
  try {
    const res = await moderationApi.stats();
    if (res.data.code === 0 && res.data.data) stats.value = res.data.data;
  } catch {
    // 概览失败不影响队列本身，给出可见提示而不是静默吞掉
    statsHint.value = '概览统计暂不可用（队列仍可正常处置）';
  }
}

async function reload(p = 1) {
  page.value = p;
  loading.value = true;
  try {
    const res = await moderationApi.records({
      scope: filters.scope || undefined,
      verdict: filters.verdict || undefined,
      trigger: filters.trigger || undefined,
      todo: view.value === 'todo' ? '1' : undefined,
      page: p,
    });
    if (res.data.code === 0) {
      rows.value = res.data.data.items || [];
      total.value = res.data.data.total || 0;
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

/** 处置：放行无需原因；删除/封禁需要填写原因（留证用） */
async function decide(row: ModerationRecord, action: 'pass' | 'delete' | 'ban') {
  let reason = '';
  if (action !== 'pass') {
    const verb = action === 'ban' ? '封禁' : '删除';
    try {
      const r = await ElMessageBox.prompt(
        `${verb}${scopeLabel(row.scope)} #${row.target_id}？\n原因会写入留证与封禁记录，作者端可查。`,
        `${verb}确认`,
        {
          type: 'warning',
          inputPlaceholder: '处置原因（留空则记「AI 内容审核判定违规（人工确认处置）」）',
          confirmButtonText: verb,
          cancelButtonText: '取消',
        },
      );
      reason = (r as any).value || '';
    } catch { return; }
  } else {
    try {
      await ElMessageBox.confirm(
        `放行${scopeLabel(row.scope)} #${row.target_id}？内容不会改动，只记录「已人工确认合规」。`,
        '放行确认',
        { type: 'info', confirmButtonText: '放行', cancelButtonText: '取消' },
      );
    } catch { return; }
  }

  try {
    const res = await moderationApi.decide(row.id, action, reason);
    if (res.data.code === 0) {
      ElMessage.success(`已${actionLabel(action)}`);
      await reload(page.value);
      await loadStats();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '处置失败');
  }
}

onMounted(() => {
  loadStats();
  reload(1);
});
</script>

<style scoped>
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; }
.mono { font-family: Consolas, 'Courier New', monospace; }

.queue-toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 12px; }
.hint-warn { color: var(--el-color-warning, #e6a23c); }
.queue-pager { margin-top: 12px; justify-content: flex-end; }
.queue-tip { margin-top: 10px; font-size: 0.78rem; color: var(--text-light, #999); line-height: 1.7; }
.queue-tip code { background: var(--el-fill-color-light, #f4f4f5); padding: 1px 5px; border-radius: 3px; }

.cell-content { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.cat-tag { margin: 0 4px 4px 0; }
.reason-text {
  font-size: 0.78rem; line-height: 1.55; color: var(--text-regular, #606266);
  display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden;
}

[data-theme='dark'] .admin-page { background: #0f172a; }
</style>
