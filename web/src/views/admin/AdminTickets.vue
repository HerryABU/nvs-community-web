<template>
  <div class="page">
    <div class="head">
      <h2>警方联动工单</h2>
      <span class="sub">建单 → 派单 → 进展 → 结案（结单必须写结论）；工单只存证据引用，不存证据本体</span>
    </div>

    <div class="cards">
      <div class="card"><div class="num">{{ stats?.by_status?.open ?? 0 }}</div><div class="label">待受理</div></div>
      <div class="card"><div class="num">{{ stats?.by_status?.processing ?? 0 }}</div><div class="label">处理中</div></div>
      <div class="card"><div class="num">{{ stats?.by_status?.waiting ?? 0 }}</div><div class="label">待补材料</div></div>
      <div class="card"><div class="num warn">{{ stats?.overdue ?? 0 }}</div><div class="label">已超期</div></div>
      <div class="card"><div class="num">{{ stats?.unassigned ?? 0 }}</div><div class="label">未派单</div></div>
    </div>

    <div class="toolbar">
      <el-select v-model="f.status" clearable placeholder="状态" style="width: 130px" @change="load">
        <el-option label="待受理" value="open" />
        <el-option label="处理中" value="processing" />
        <el-option label="待补材料" value="waiting" />
        <el-option label="已结单" value="closed" />
      </el-select>
      <el-select v-model="f.priority" clearable placeholder="优先级" style="width: 120px" @change="load">
        <el-option label="紧急" value="urgent" />
        <el-option label="高" value="high" />
        <el-option label="普通" value="normal" />
        <el-option label="低" value="low" />
      </el-select>
      <el-select v-model="f.kind" clearable placeholder="类型" style="width: 130px" @change="load">
        <el-option label="举报升级" value="report" />
        <el-option label="取证协助" value="evidence" />
        <el-option label="协查" value="assist" />
        <el-option label="滥用风控" value="abuse" />
      </el-select>
      <el-checkbox v-model="f.mine" @change="load">只看我处理的</el-checkbox>
      <el-button type="primary" @click="createVisible = true">新建工单</el-button>
    </div>

    <el-table :data="list" size="small" class="panel" @row-click="openDetail">
      <el-table-column prop="no" label="工单号" width="150" />
      <el-table-column prop="title" label="标题" min-width="220" />
      <el-table-column label="类型/优先级" width="150">
        <template #default="{ row }">
          {{ kindLabel(row.kind) }} · <el-tag size="small" :type="priType(row.priority)">{{ priLabel(row.priority) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag size="small" :type="row.status === 'closed' ? 'info' : 'warning'">{{ stLabel(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="handler_id" label="处理人" width="90">
        <template #default="{ row }">{{ row.handler_id || '—' }}</template>
      </el-table-column>
      <el-table-column label="建单/截止" width="170">
        <template #default="{ row }">
          <div class="small">{{ fmt(row.created_at) }}</div>
          <div class="small" :class="{ overdue: row.due_at && !row.closed_at && isOverdue(row.due_at) }">
            {{ row.due_at ? fmt(row.due_at) : '无截止' }}
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- 详情抽屉 -->
    <el-drawer v-model="detailVisible" :title="detail?.ticket?.no || '工单'" size="56%">
      <template v-if="detail">
        <h3 class="d-title">{{ detail.ticket.title }}</h3>
        <el-tag size="small">{{ kindLabel(detail.ticket.kind) }}</el-tag>
        <el-tag size="small" :type="priType(detail.ticket.priority)" style="margin-left: 6px">
          {{ priLabel(detail.ticket.priority) }}
        </el-tag>
        <el-tag size="small" style="margin-left: 6px">{{ stLabel(detail.ticket.status) }}</el-tag>

        <div class="d-block">
          <div class="d-label">摘要</div>
          <div class="d-text">{{ detail.ticket.summary || '—' }}</div>
        </div>
        <div class="d-block">
          <div class="d-label">关联对象</div>
          <div class="d-text small">
            用户 #{{ detail.ticket.subject_user_id || 0 }} · 作品 #{{ detail.ticket.subject_novel_id || 0 }} ·
            帖子 #{{ detail.ticket.subject_post_id || 0 }} · 蜜罐会话 #{{ detail.ticket.session_id || 0 }}
          </div>
        </div>
        <div v-if="detail.ticket.evidence" class="d-block">
          <div class="d-label">证据引用</div>
          <pre class="d-pre">{{ detail.ticket.evidence }}</pre>
        </div>
        <div v-if="detail.ticket.result" class="d-block">
          <div class="d-label">结单结论</div>
          <div class="d-text">{{ detail.ticket.result }}</div>
        </div>

        <div class="d-block">
          <div class="d-label">操作</div>
          <div class="d-actions">
            <el-input-number v-model="assignTo" :min="1" placeholder="处理人 ID" style="width: 130px" />
            <el-button size="small" @click="doAssign">派单</el-button>
            <el-select v-model="newStatus" style="width: 130px" size="small">
              <el-option label="处理中" value="processing" />
              <el-option label="待补材料" value="waiting" />
              <el-option label="结单" value="closed" />
            </el-select>
            <el-button size="small" type="primary" @click="doStatus">更新状态</el-button>
          </div>
        </div>

        <div class="d-block">
          <div class="d-label">追加进展</div>
          <el-input v-model="note" type="textarea" :rows="2" placeholder="写清做了什么，便于复盘" />
          <el-button size="small" style="margin-top: 6px" @click="doNote">追加</el-button>
        </div>

        <div class="d-block">
          <div class="d-label">流转日志</div>
          <el-timeline>
            <el-timeline-item v-for="l in detail.logs" :key="l.id" :timestamp="fmt(l.created_at)">
              <b>{{ l.action }}</b> {{ l.from_to ? `(${l.from_to})` : '' }} — {{ l.note }}
            </el-timeline-item>
          </el-timeline>
        </div>
      </template>
    </el-drawer>

    <!-- 新建 -->
    <el-dialog v-model="createVisible" title="新建工单" width="560px">
      <el-form label-width="100px">
        <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.kind" style="width: 100%">
            <el-option label="举报升级" value="report" />
            <el-option label="取证协助" value="evidence" />
            <el-option label="协查" value="assist" />
            <el-option label="滥用风控" value="abuse" />
          </el-select>
        </el-form-item>
        <el-form-item label="优先级">
          <el-radio-group v-model="form.priority">
            <el-radio-button value="low">低</el-radio-button>
            <el-radio-button value="normal">普通</el-radio-button>
            <el-radio-button value="high">高</el-radio-button>
            <el-radio-button value="urgent">紧急</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="摘要">
          <el-input v-model="form.summary" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="关联">
          <el-input-number v-model="form.subject_user_id" :min="0" placeholder="用户" />
          <el-input-number v-model="form.session_id" :min="0" placeholder="蜜罐会话" style="margin-left: 8px" />
        </el-form-item>
        <el-form-item label="证据引用">
          <el-input v-model="form.evidence" type="textarea" :rows="2"
            placeholder='JSON，如 [{"type":"honeypot","ref":"session:12"}]' />
        </el-form-item>
        <el-form-item label="截止(小时)">
          <el-input-number v-model="form.due_hours" :min="0" :max="720" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" @click="doCreate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { ticketApi, type Ticket, type TicketLog } from '@/api/costrisk';

const list = ref<Ticket[]>([]);
const stats = ref<any>(null);
const f = reactive({ status: '', priority: '', kind: '', mine: false });

const detailVisible = ref(false);
const detail = ref<{ ticket: Ticket; logs: TicketLog[] } | null>(null);
const assignTo = ref(1);
const newStatus = ref('processing');
const note = ref('');

const createVisible = ref(false);
const form = reactive({
  title: '', kind: 'evidence', priority: 'normal', summary: '',
  subject_user_id: 0, session_id: 0, evidence: '', due_hours: 24,
});

function kindLabel(k: string) {
  return { report: '举报升级', evidence: '取证协助', assist: '协查', abuse: '滥用风控' }[k] || k;
}
function priLabel(p: string) {
  return { low: '低', normal: '普通', high: '高', urgent: '紧急' }[p] || p;
}
function priType(p: string) {
  return p === 'urgent' ? 'danger' : p === 'high' ? 'warning' : 'info';
}
function stLabel(s: string) {
  return { open: '待受理', processing: '处理中', waiting: '待补材料', closed: '已结单' }[s] || s;
}
function fmt(s?: string | null) {
  return s ? new Date(s).toLocaleString('zh-CN') : '';
}
function isOverdue(s: string) {
  return new Date(s).getTime() < Date.now();
}

async function load() {
  const res = await ticketApi.list(1, 50, {
    status: f.status, priority: f.priority, kind: f.kind, mine: f.mine ? 1 : undefined,
  });
  list.value = res.data.data.list || [];
  stats.value = res.data.data.stats;
}
async function openDetail(row: Ticket) {
  const res = await ticketApi.detail(row.id);
  detail.value = res.data.data;
  assignTo.value = row.handler_id || 1;
  newStatus.value = row.status === 'closed' ? 'closed' : 'processing';
  note.value = '';
  detailVisible.value = true;
}
async function doAssign() {
  if (!detail.value) return;
  await ticketApi.assign(detail.value.ticket.id, assignTo.value);
  ElMessage.success('已派单');
  await openDetail(detail.value.ticket);
  await load();
}
async function doStatus() {
  if (!detail.value) return;
  let conclusion = note.value;
  if (newStatus.value === 'closed' && !conclusion.trim()) {
    ElMessage.warning('结单必须填写结论');
    return;
  }
  await ticketApi.setStatus(detail.value.ticket.id, newStatus.value, conclusion);
  ElMessage.success('已更新');
  await openDetail(detail.value.ticket);
  await load();
}
async function doNote() {
  if (!detail.value || !note.value.trim()) return;
  await ticketApi.addNote(detail.value.ticket.id, note.value);
  note.value = '';
  ElMessage.success('已追加');
  await openDetail(detail.value.ticket);
}
async function doCreate() {
  if (!form.title.trim()) {
    ElMessage.warning('请填写标题');
    return;
  }
  await ticketApi.create({ ...form });
  ElMessage.success('已创建');
  createVisible.value = false;
  form.title = '';
  form.summary = '';
  await load();
}

onMounted(load);
</script>

<style scoped>
.page { padding: 16px; }
.head { display: flex; align-items: baseline; gap: 12px; margin-bottom: 10px; }
.head h2 { margin: 0; }
.sub { color: var(--text-muted); font-size: 0.8rem; }
.cards { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 12px; margin-bottom: 14px; }
.card { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 10px; padding: 12px 14px; }
.num { font-size: 1.4rem; font-weight: 700; }
.num.warn { color: var(--el-color-warning); }
.label { color: var(--text-muted); font-size: 0.78rem; }
.toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 10px; }
.panel { cursor: pointer; }
.small { font-size: 0.74rem; color: var(--text-muted); }
.overdue { color: var(--el-color-danger); }
.d-title { margin: 0 0 8px; }
.d-block { margin-top: 14px; }
.d-label { font-size: 0.78rem; color: var(--text-muted); margin-bottom: 4px; }
.d-text { font-size: 0.9rem; }
.d-pre { background: var(--bg-muted); padding: 8px; border-radius: 6px; font-size: 0.75rem; white-space: pre-wrap; }
.d-actions { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
</style>
