<template>
  <div class="camp-wrap">
    <el-alert type="warning" :closable="false" show-icon class="camp-notice">
      <template #title>阵营中立原则 · 只标记，不裁决</template>
      争议内容的对错取决于站在哪个阵营，这是结构性原因。AI 只标记「阵营」与「语言格式」，
      <b>严禁判断谁对谁错</b>，严禁因为某一方人数多、帖子多、文字长就倒戈向它。
      下图的节点大小与连线粗细<b>只代表规模，不代表谁更有理</b>。
    </el-alert>

    <div class="camp-toolbar">
      <el-button size="small" :loading="loading" @click="load">刷新图谱</el-button>
      <el-button size="small" type="primary" plain @click="showPrompt = !showPrompt">
        {{ showPrompt ? '收起' : '查看' }}AI 中立性提示词
      </el-button>
      <span class="camp-stat" v-if="!loading">阵营 {{ graph.nodes.length }} 个 · 共现 {{ graph.edges.length }} 条</span>
    </div>

    <el-collapse-transition>
      <div v-if="showPrompt" class="camp-prompt">
        <div class="camp-prompt__head">舆论分析专用提示词（后台可见，禁止 AI 判对错 / 禁止倒戈多数派）</div>
        <pre>{{ promptData.prompt }}</pre>
      </div>
    </el-collapse-transition>

    <div class="camp-legend">
      <span v-for="l in langLegend" :key="l.value" class="camp-legend__item">
        <i :style="{ background: l.color }" />{{ l.label }}
      </span>
    </div>

    <div v-if="loading" class="camp-empty">加载图谱中…</div>
    <div v-else-if="graph.nodes.length === 0" class="camp-empty">
      暂无阵营标记。可在下方「手动标记」里新增一条，或等待 AI 分析写入。
    </div>
    <div v-else class="camp-svg-wrap">
      <svg :viewBox="`0 0 ${svg.w} ${svg.h}`" class="camp-svg">
        <line
          v-for="(e, i) in positionedEdges"
          :key="'e' + i"
          :x1="e.x1" :y1="e.y1" :x2="e.x2" :y2="e.y2"
          class="camp-edge"
          :stroke-width="Math.min(6, 1 + e.weight)"
        />
        <g v-for="n in positionedNodes" :key="n.id">
          <circle :cx="n.x" :cy="n.y" :r="n.r" :fill="langColor(n.lang_format)" class="camp-node" />
          <text :x="n.x" :y="n.y + 4" class="camp-node-label">{{ shortLabel(n.label) }}</text>
          <text :x="n.x" :y="n.y + n.r + 15" class="camp-node-sub">{{ n.lang_label }} · {{ n.size }}</text>
        </g>
      </svg>
      <div class="camp-disclaimer">{{ graph.notice }}</div>
    </div>

    <!-- 手动标记 + 删除申请 -->
    <div class="camp-panels">
      <div class="camp-panel">
        <h4>手动标记阵营 / 语言格式</h4>
        <el-form :model="markForm" label-width="90px" size="small">
          <el-form-item label="阵营标识"><el-input v-model="markForm.camp_id" placeholder="如 A / 支持方（中性）" /></el-form-item>
          <el-form-item label="显示名"><el-input v-model="markForm.camp_label" placeholder="中性短名" /></el-form-item>
          <el-form-item label="语言格式">
            <el-select v-model="markForm.lang_format" style="width: 100%">
              <el-option v-for="l in langLegend" :key="l.value" :label="l.label" :value="l.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="表达特征"><el-input v-model="markForm.lang_evidence" placeholder="只写表达特征，不评价立场" /></el-form-item>
          <el-form-item label="内容 ID"><el-input-number v-model="markForm.content_id" :min="0" /></el-form-item>
          <el-form-item>
            <el-button type="primary" size="small" :loading="marking" @click="submitMark">新增标记</el-button>
          </el-form-item>
        </el-form>
      </div>
      <div class="camp-panel">
        <h4>删除申请（仅超级管理员可批准）</h4>
        <div class="muted small" style="margin-bottom: 8px">
          普通管理员只能新增规则，不能删除；删除会自动提交申请并通知超级管理员（站长）。
        </div>
        <el-table :data="requests" size="small" max-height="240">
          <el-table-column label="目标" min-width="120" prop="target_label" />
          <el-table-column label="申请人" width="100" prop="requester" />
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === 'pending' ? 'warning' : row.status === 'approved' ? 'success' : 'info'">
                {{ row.status === 'pending' ? '待批' : row.status === 'approved' ? '已批准' : '已驳回' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="140" v-if="canApprove">
            <template #default="{ row }">
              <template v-if="row.status === 'pending'">
                <el-button link type="success" size="small" @click="approve(row)">批准</el-button>
                <el-button link type="danger" size="small" @click="reject(row)">驳回</el-button>
              </template>
              <span v-else class="muted small">—</span>
            </template>
          </el-table-column>
        </el-table>
        <div v-if="!canApprove" class="muted small" style="margin-top: 8px">
          当前账号为管理员，无批准权限；批准由唯一超级管理员（站长）执行。
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { sentimentApi, type CampGraphData, type CampEdge, type DeleteRequest, type NeutralityPromptData } from '@/api/admin/sentiment';

const loading = ref(false);
const marking = ref(false);
const showPrompt = ref(false);
const graph = ref<CampGraphData>({ nodes: [], edges: [], notice: '' });
const promptData = ref<NeutralityPromptData>({ prompt: '', notice: '', lang_formats: [] });
const requests = ref<DeleteRequest[]>([]);
const canApprove = ref(false);

const langLegend = [
  { value: 'fandom', label: '饭圈方法', color: '#ec4899' },
  { value: 'political', label: '政治语言方法', color: '#f59e0b' },
  { value: 'everyday', label: '普通人语言格式', color: '#10b981' },
  { value: 'mixed', label: '混用', color: '#8b5cf6' },
  { value: 'unknown', label: '未识别', color: '#94a3b8' },
];
const langColor = (v: string) => langLegend.find((l) => l.value === v)?.color || '#94a3b8';
const shortLabel = (s: string) => (s && s.length > 6 ? s.slice(0, 6) + '…' : s || '—');

const markForm = reactive({
  camp_id: '', camp_label: '', lang_format: 'everyday',
  lang_evidence: '', content_type: 'forum', content_id: 0,
});

const svg = { w: 720, h: 340, cx: 360, cy: 160, r: 125 };
const positionedNodes = computed(() => {
  const ns = graph.value.nodes;
  const n = ns.length;
  return ns.map((node, i) => {
    const ang = (i / Math.max(1, n)) * Math.PI * 2 - Math.PI / 2;
    return {
      ...node,
      x: n === 1 ? svg.cx : svg.cx + svg.r * Math.cos(ang),
      y: n === 1 ? svg.cy : svg.cy + svg.r * Math.sin(ang),
      r: Math.min(42, 18 + Math.sqrt(node.size) * 7),
    };
  });
});
const positionedEdges = computed(() => {
  const pos = new Map(positionedNodes.value.map((n) => [n.id, n]));
  return (graph.value.edges as CampEdge[]).flatMap((e) => {
    const a = pos.get(e.source);
    const b = pos.get(e.target);
    return a && b ? [{ x1: a.x, y1: a.y, x2: b.x, y2: b.y, weight: e.weight }] : [];
  });
});

async function load() {
  loading.value = true;
  try {
    const [g, p, r] = await Promise.all([
      sentimentApi.campGraph(),
      sentimentApi.neutralityPrompt(),
      sentimentApi.deleteRequests(),
    ]);
    if (g.data?.code === 0) graph.value = g.data.data;
    if (p.data?.code === 0) promptData.value = p.data.data;
    if (r.data?.code === 0) {
      requests.value = r.data.data.list || [];
      canApprove.value = !!r.data.data.can_approve;
    }
  } catch {
    ElMessage.error('加载舆情图谱失败');
  } finally {
    loading.value = false;
  }
}

async function submitMark() {
  if (!markForm.camp_id.trim()) {
    ElMessage.warning('请填写阵营标识');
    return;
  }
  marking.value = true;
  try {
    const res = await sentimentApi.createCampMark({ ...markForm });
    if (res.data?.code === 0) {
      ElMessage.success('已新增标记');
      markForm.camp_id = '';
      markForm.camp_label = '';
      await load();
    }
  } catch {
    ElMessage.error('新增标记失败');
  } finally {
    marking.value = false;
  }
}

async function approve(row: DeleteRequest) {
  try {
    await ElMessageBox.confirm(`批准删除「${row.target_label}」？`, '批准申请', { type: 'warning' });
  } catch {
    return;
  }
  const res = await sentimentApi.approveDeleteRequest(row.id);
  if (res.data?.code === 0) {
    ElMessage.success('已批准并删除');
    await load();
  }
}

async function reject(row: DeleteRequest) {
  const res = await sentimentApi.rejectDeleteRequest(row.id, '站长认为无需删除');
  if (res.data?.code === 0) {
    ElMessage.success('已驳回');
    await load();
  }
}

onMounted(load);
</script>

<style scoped>
.camp-wrap { display: flex; flex-direction: column; gap: 12px; }
.camp-notice { line-height: 1.7; }
.camp-toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.camp-stat { font-size: 0.82rem; color: var(--text-light); }
.camp-prompt { background: var(--bg-soft); border: 1px solid var(--border-color); border-radius: 10px; padding: 12px 14px; }
.camp-prompt__head { font-weight: 600; color: var(--text-color); margin-bottom: 8px; font-size: 0.88rem; }
.camp-prompt pre { margin: 0; white-space: pre-wrap; word-break: break-word; font-size: 0.78rem; line-height: 1.6; color: var(--text-light); max-height: 260px; overflow-y: auto; }
.camp-legend { display: flex; gap: 14px; flex-wrap: wrap; font-size: 0.8rem; color: var(--text-light); }
.camp-legend__item { display: inline-flex; align-items: center; gap: 5px; }
.camp-legend__item i { width: 10px; height: 10px; border-radius: 50%; display: inline-block; }
.camp-empty { padding: 32px; text-align: center; color: var(--text-light); border: 1px dashed var(--border-color); border-radius: 10px; }
.camp-svg-wrap { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 12px; padding: 8px; }
.camp-svg { width: 100%; height: auto; }
.camp-edge { stroke: var(--border-color); opacity: 0.9; }
.camp-node { opacity: 0.88; }
.camp-node-label { font-size: 11px; fill: #fff; text-anchor: middle; font-weight: 600; }
.camp-node-sub { font-size: 10px; fill: var(--text-light); text-anchor: middle; }
.camp-disclaimer { text-align: center; font-size: 0.76rem; color: var(--text-muted); padding: 4px 8px; }
.camp-panels { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 14px; }
.camp-panel { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 12px; padding: 14px; }
.camp-panel h4 { margin: 0 0 12px; font-size: 0.98rem; color: var(--text-color); }
.muted { color: var(--text-light); }
.small { font-size: 0.8rem; }
</style>
