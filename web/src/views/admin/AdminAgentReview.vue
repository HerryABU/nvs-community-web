<template>
  <div class="admin-page">
    <AdminHeader
      title="智能体投稿"
      subtitle="用户提交的特殊提示词模板 · 站长审核后上架到广场"
      :icon="ChatDotSquare"
    />
    <div class="admin-page-body">
      <p class="hint-text">
        本站的智能体本质是<b>一段特殊提示词</b>。用户可以把自己的提示词投稿上来，
        由你决定是否上架。审核通过后会直接出现在「AI 工作台 → 智能体广场」，
        用户点卡片即带该提示词开始对话 —— 对话页不再有角色下拉，入口统一在广场。
      </p>

      <!-- 投稿开关（站长设置） -->
      <div class="as-cfg">
        <div class="as-cfg-row">
          <el-switch v-model="cfg.enabled" />
          <span class="as-cfg-label">开放用户投稿</span>
          <span class="muted small">关闭后前台不再显示「投稿智能体」入口（已投稿件不受影响）</span>
        </div>
        <div class="as-cfg-row">
          <el-switch v-model="cfg.need_review" :disabled="!cfg.enabled" />
          <span class="as-cfg-label">需要审核</span>
          <span class="muted small">关闭 = 用户提交即上架（风险自负；仍会留一条已通过记录便于追溯作者）</span>
        </div>
        <div class="as-cfg-row">
          <span class="as-cfg-label">每人待审上限</span>
          <el-input-number v-model="cfg.max_pending" :min="0" :max="50" size="small" style="width:110px" />
          <span class="muted small">0 = 不限；用于防止一个人刷满审核队列</span>
        </div>
        <div class="as-cfg-row">
          <el-button type="primary" size="small" :loading="savingCfg" @click="saveCfg">保存开关</el-button>
          <el-tag v-if="pendingCount" type="warning" size="small" effect="dark">待审 {{ pendingCount }}</el-tag>
        </div>
      </div>

      <div class="table-toolbar">
        <el-radio-group v-model="statusFilter" size="small" @change="load(1)">
          <el-radio-button value="pending">待审 {{ statusCounts.pending || 0 }}</el-radio-button>
          <el-radio-button value="approved">已通过</el-radio-button>
          <el-radio-button value="rejected">已拒绝</el-radio-button>
          <el-radio-button value="">全部</el-radio-button>
        </el-radio-group>
        <span class="muted small">共 {{ total }} 条</span>
      </div>

      <el-table v-if="list.length" :data="list" v-loading="loading" style="width:100%">
        <el-table-column label="智能体" min-width="150">
          <template #default="{ row }">
            <span class="as-emoji">{{ row.emoji || '🤖' }}</span>
            <span class="as-name">{{ row.name }}</span>
            <div v-if="row.description" class="as-desc">{{ row.description }}</div>
          </template>
        </el-table-column>
        <el-table-column label="投稿人" width="130">
          <template #default="{ row }">{{ row.user_name || ('#' + row.user_id) }}</template>
        </el-table-column>
        <el-table-column label="分类" width="90">
          <template #default="{ row }">
            <span v-if="row.category" class="as-cat">{{ row.category }}</span>
            <span v-else class="muted small">—</span>
          </template>
        </el-table-column>
        <el-table-column label="提示词" min-width="280">
          <template #default="{ row }">
            <div class="as-prompt" :class="{ expanded: expanded[row.id] }">{{ row.system_prompt }}</div>
            <el-button link size="small" @click="toggleExpand(row.id)">
              {{ expanded[row.id] ? '收起' : '展开全文' }}
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="statusTag(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="备注" min-width="120">
          <template #default="{ row }">
            <span v-if="row.review_note" class="muted small">{{ row.review_note }}</span>
            <span v-else class="muted small">—</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <template v-if="row.status === 'pending'">
              <el-button type="primary" link size="small" @click="approve(row)">通过</el-button>
              <el-button type="danger" link size="small" @click="reject(row)">拒绝</el-button>
            </template>
            <span v-else-if="row.agent_id" class="muted small">已上架 #{{ row.agent_id }}</span>
            <span v-else class="muted small">—</span>
          </template>
        </el-table-column>
      </el-table>

      <StateBlock
        v-else-if="!loading"
        state="empty"
        :title="statusFilter === 'pending' ? '没有待审投稿' : '没有记录'"
        description="用户投稿后会出现在这里，审核通过即上架到智能体广场。"
      />

      <div v-if="total > pageSize" class="as-pager">
        <el-pagination
          layout="prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="load"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ChatDotSquare } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import StateBlock from '@/components/common/StateBlock.vue';
import { adminApi } from '@/api/admin';
import type { AgentSubmission } from '@/api/aiHub';

const list = ref<AgentSubmission[]>([]);
const loading = ref(false);
const page = ref(1);
const pageSize = 20;
const total = ref(0);
const statusFilter = ref('pending');
const statusCounts = ref<Record<string, number>>({});
const pendingCount = ref(0);
const expanded = reactive<Record<number, boolean>>({});

const cfg = reactive({ enabled: true, need_review: true, max_pending: 3 });
const savingCfg = ref(false);

function statusLabel(s: string) {
  return s === 'approved' ? '已通过' : s === 'rejected' ? '已拒绝' : '待审核';
}
function statusTag(s: string) {
  return s === 'approved' ? 'success' : s === 'rejected' ? 'danger' : 'warning';
}
function toggleExpand(id: number) {
  expanded[id] = !expanded[id];
}

async function loadCfg() {
  try {
    const res = await adminApi.getAgentSubmitConfig();
    if (res.data.code === 0) {
      Object.assign(cfg, res.data.data.config);
      pendingCount.value = res.data.data.pending || 0;
      statusCounts.value.pending = pendingCount.value;
    }
  } catch { /* 读不到就保持默认，不打断审核 */ }
}

async function load(p = 1) {
  page.value = p;
  loading.value = true;
  try {
    const res = await adminApi.listAgentSubmissions({
      status: statusFilter.value,
      page: p,
      size: pageSize,
    });
    if (res.data.code === 0) {
      list.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
      pendingCount.value = res.data.data.pending || 0;
      statusCounts.value.pending = pendingCount.value;
    }
  } catch {
    ElMessage.error('加载失败');
  } finally {
    loading.value = false;
  }
}

async function approve(row: AgentSubmission) {
  try {
    const { value } = await ElMessageBox.prompt('通过后会立刻出现在智能体广场。可留一句备注给作者。', '通过投稿', {
      inputPlaceholder: '备注（可留空）',
      confirmButtonText: '通过并上架',
      cancelButtonText: '取消',
    });
    const res = await adminApi.reviewAgentSubmission(row.id, { action: 'approve', note: value || '' });
    if (res.data.code === 0) {
      ElMessage.success('已通过，已上架');
      await load(page.value);
    } else {
      ElMessage.error(res.data.message || '操作失败');
    }
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function reject(row: AgentSubmission) {
  try {
    // 拒绝必须写原因：用户要据此改稿，后端也强制校验
    const { value } = await ElMessageBox.prompt('请写明拒绝原因，用户会看到（例如：提示词含越狱指令）', '拒绝投稿', {
      inputPlaceholder: '拒绝原因（必填）',
      inputValidator: (v: string) => (v && v.trim() ? true : '请填写拒绝原因'),
      confirmButtonText: '拒绝',
      cancelButtonText: '取消',
    });
    const res = await adminApi.reviewAgentSubmission(row.id, { action: 'reject', note: value });
    if (res.data.code === 0) {
      ElMessage.success('已拒绝');
      await load(page.value);
    } else {
      ElMessage.error(res.data.message || '操作失败');
    }
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function saveCfg() {
  savingCfg.value = true;
  try {
    const res = await adminApi.saveAgentSubmitConfig({ ...cfg });
    if (res.data.code === 0) ElMessage.success('已保存，前台立即生效');
    else ElMessage.error(res.data.message || '保存失败');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    savingCfg.value = false;
  }
}

onMounted(() => {
  loadCfg();
  load(1);
});
</script>

<style scoped>
.as-cfg {
  margin-bottom: 16px; padding: 14px 16px; border-radius: 10px;
  background: var(--bg-page, #f8fafc);
  border: 1px solid var(--border-color, #e5e7eb);
  display: flex; flex-direction: column; gap: 10px;
}
.as-cfg-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.as-cfg-label { font-size: 0.85rem; font-weight: 600; color: var(--text-main, #334155); }

.as-emoji { margin-right: 6px; }
.as-name { font-weight: 600; color: var(--text-main, #1f2937); }
.as-desc { margin-top: 2px; font-size: 0.75rem; color: var(--text-light, #94a3b8); }
.as-cat {
  padding: 1px 6px; border-radius: 4px; font-size: 0.72rem;
  background: var(--bg-page, #f1f5f9); color: var(--text-light, #64748b);
}
.as-prompt {
  font-size: 0.78rem; line-height: 1.6; color: var(--text-main, #475569);
  white-space: pre-wrap; word-break: break-word;
  max-height: 48px; overflow: hidden;
}
.as-prompt.expanded { max-height: none; }
.as-pager { margin-top: 14px; display: flex; justify-content: center; }
</style>
