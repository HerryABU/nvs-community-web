<template>
  <div class="camp-wrap">
    <el-alert type="warning" :closable="false" show-icon class="camp-notice">
      <template #title>阵营中立原则 · 只标记，不裁决</template>
      争议内容的对错取决于站在哪个阵营，这是结构性原因。AI 只标记「阵营」与「语言格式」，
      <b>严禁判断谁对谁错</b>，严禁因为某一方人数多、帖子多、文字长就倒戈向它。
      下图的节点大小与连线粗细<b>只代表规模，不代表谁更有理</b>。
    </el-alert>

    <!-- AI 分析入口：真正把争议文本交给强中立提示词，产出阵营/语言格式标记并落库 -->
    <div class="camp-analyze">
      <div class="camp-analyze__head">
        <b>🤖 AI 阵营分析</b>
        <span>把一段争议文本交给强化后的中立性提示词，标记阵营与语言格式，写入图谱。</span>
      </div>
      <el-input v-model="analyzeText" type="textarea" :rows="3" placeholder="粘贴一段争议讨论…AI 只标记阵营与「饭圈方法 / 政治语言方法 / 普通人语言格式」，不判断谁对谁错，也不会因哪一方字多就倒戈。" />
      <div class="camp-analyze__foot">
        <label class="camp-analyze__opt">
          <input type="checkbox" v-model="analyzeSave" /> 保存标记到图谱
        </label>
        <el-button type="primary" size="small" :loading="analyzing" :disabled="!analyzeText.trim()" @click="runAnalyze">
          AI 分析并标记
        </el-button>
        <span v-if="analyzeResult" class="camp-analyze__result">{{ analyzeResult }}</span>
      </div>
    </div>

    <div class="camp-toolbar">
      <el-button size="small" :loading="loading" @click="load">刷新图谱</el-button>
      <el-button size="small" type="primary" plain @click="showPrompt = !showPrompt">
        {{ showPrompt ? '收起' : '查看' }}AI 中立性提示词
      </el-button>
      <span class="camp-stat" v-if="!loading">阵营 {{ graph.nodes.length }} 个 · 共现 {{ graph.edges.length }} 条</span>
    </div>

    <el-collapse-transition>
      <div v-if="showPrompt" class="camp-prompt">
        <div class="camp-prompt__head">舆论分析专用提示词（三明治结构：头尾各压一遍，禁止 AI 判对错 / 禁止倒戈多数派）</div>
        <pre>{{ promptData.prompt }}</pre>
        <div class="camp-prompt__head" v-if="promptData.tail_prompt" style="margin-top: 12px">尾部复核提示词（长文本后再压一遍）</div>
        <pre v-if="promptData.tail_prompt">{{ promptData.tail_prompt }}</pre>
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
        <h4>阵营标记列表</h4>
        <div class="muted small" style="margin-bottom: 8px">
          <template v-if="canApprove">
            当前为超级管理员（站长）：可直接修改 / 删除阵营标记。
          </template>
          <template v-else>
            当前为管理员：只能【新增】，修改 / 删除需提交申请，由超级管理员（站长）批准。
          </template>
        </div>
        <el-table :data="marks" size="small" max-height="300" empty-text="暂无标记">
          <el-table-column label="阵营" min-width="120">
            <template #default="{ row }">{{ row.camp_label || row.camp_id }}</template>
          </el-table-column>
          <el-table-column label="标识" prop="camp_id" width="90" />
          <el-table-column label="语言格式" width="116">
            <template #default="{ row }">{{ langLabel(row.lang_format) }}</template>
          </el-table-column>
          <el-table-column label="来源" width="64" prop="source" />
          <el-table-column label="操作" width="150">
            <template #default="{ row }">
              <template v-if="canApprove">
                <el-button link type="primary" size="small" @click="openEdit(row, 'direct')">修改</el-button>
                <el-button link type="danger" size="small" @click="directDelete(row)">删除</el-button>
              </template>
              <template v-else>
                <el-button link type="primary" size="small" @click="openEdit(row, 'request')">申请修改</el-button>
                <el-button link type="danger" size="small" @click="requestDelete(row)">申请删除</el-button>
              </template>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="camp-panel">
        <h4>修改 / 删除申请（修改项目）</h4>
        <div class="muted small" style="margin-bottom: 8px">
          普通管理员提交修改 / 删除后会自动通知超级管理员（站长）；站长批准才生效，也可驳回。
        </div>
        <el-table :data="requests" size="small" max-height="240" empty-text="暂无申请">
          <el-table-column label="类型" width="64">
            <template #default="{ row }">
              <el-tag size="small" :type="row.action === 'update' ? 'primary' : 'danger'" effect="plain">
                {{ row.action === 'update' ? '修改' : '删除' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="目标" min-width="120" prop="target_label" />
          <el-table-column label="申请人" width="100" prop="requester" />
          <el-table-column label="变更" width="70">
            <template #default="{ row }">
              <el-popover v-if="row.action === 'update' && row.after" placement="left" :width="280" trigger="click">
                <template #reference>
                  <el-button link type="info" size="small">查看</el-button>
                </template>
                <pre class="patch-pre">{{ patchText(row.before, row.after) }}</pre>
              </el-popover>
              <span v-else class="muted small">—</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="80">
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
          当前账号为管理员，无批准权限；批准 / 驳回由唯一超级管理员（站长）执行。
        </div>
      </div>
    </div>

    <!-- 修改 / 申请修改 对话框 -->
    <el-dialog v-model="editDialog" :title="editMode === 'direct' ? '修改阵营标记（站长直改）' : '申请修改阵营标记'" width="480px">
      <el-form :model="editForm" label-width="92px" size="small">
        <el-form-item label="阵营标识"><el-input v-model="editForm.camp_id" placeholder="如 A / 支持方（中性）" /></el-form-item>
        <el-form-item label="显示名"><el-input v-model="editForm.camp_label" placeholder="中性短名" /></el-form-item>
        <el-form-item label="语言格式">
          <el-select v-model="editForm.lang_format" style="width: 100%">
            <el-option v-for="l in langLegend" :key="l.value" :label="l.label" :value="l.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="立场摘要"><el-input v-model="editForm.position" placeholder="中性摘要，不评价" /></el-form-item>
        <el-form-item label="表达特征"><el-input v-model="editForm.lang_evidence" placeholder="只写表达特征，不评价立场" /></el-form-item>
        <el-form-item label="置信度">
          <el-input-number v-model="editForm.confidence" :min="0" :max="100" />
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="editForm.note" /></el-form-item>
        <el-form-item label="申请理由" v-if="editMode === 'request'">
          <el-input v-model="editForm.reason" placeholder="给站长的说明（可选）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button size="small" @click="editDialog = false">取消</el-button>
        <el-button type="primary" size="small" :loading="editing" @click="submitEdit">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { sentimentApi, type CampGraphData, type CampEdge, type DeleteRequest, type CampMark, type NeutralityPromptData } from '@/api/admin/sentiment';

const loading = ref(false);
const marking = ref(false);
const analyzing = ref(false);
const analyzeText = ref('');
const analyzeSave = ref(true);
const analyzeResult = ref('');
const showPrompt = ref(false);
const graph = ref<CampGraphData>({ nodes: [], edges: [], notice: '' });
const promptData = ref<NeutralityPromptData>({ prompt: '', notice: '', lang_formats: [] });
const requests = ref<DeleteRequest[]>([]);
const marks = ref<CampMark[]>([]);
const canApprove = ref(false);

const editDialog = ref(false);
const editMode = ref<'direct' | 'request'>('request');
const editing = ref(false);
const editTargetId = ref(0);
const editForm = reactive({
  camp_id: '', camp_label: '', lang_format: 'everyday',
  position: '', lang_evidence: '', confidence: 0, note: '', reason: '',
});

const langLabel = (v: string) => langLegend.find((l) => l.value === v)?.label || v || '—';

// 把 before/after 快照渲染成可读的「字段: 旧 → 新」文本（只取白名单字段）
function patchText(beforeJSON: string, afterJSON: string): string {
  const keys: [keyof CampMark, string][] = [
    ['camp_id', '阵营标识'], ['camp_label', '显示名'], ['lang_format', '语言格式'],
    ['position', '立场摘要'], ['lang_evidence', '表达特征'], ['confidence', '置信度'], ['note', '备注'],
  ];
  try {
    const before: any = beforeJSON ? JSON.parse(beforeJSON) : {};
    const after: any = afterJSON ? JSON.parse(afterJSON) : {};
    return keys
      .map(([k, label]) => {
        const b = before[k] ?? '';
        const a = after[k] ?? '';
        if (b === a) return null;
        return `${label}: ${b || '（空）'} → ${a || '（空）'}`;
      })
      .filter(Boolean)
      .join('\n') || '（无字段变化）';
  } catch {
    return '（快照解析失败）';
  }
}

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
    const [g, p, r, m] = await Promise.all([
      sentimentApi.campGraph(),
      sentimentApi.neutralityPrompt(),
      sentimentApi.deleteRequests(),
      sentimentApi.campMarks(),
    ]);
    if (g.data?.code === 0) graph.value = g.data.data;
    if (p.data?.code === 0) promptData.value = p.data.data;
    if (r.data?.code === 0) {
      requests.value = r.data.data.list || [];
      canApprove.value = !!r.data.data.can_approve;
    }
    if (m.data?.code === 0) marks.value = m.data.data || [];
  } catch {
    ElMessage.error('加载舆情图谱失败');
  } finally {
    loading.value = false;
  }
}

async function runAnalyze() {
  if (!analyzeText.value.trim()) return;
  analyzing.value = true;
  analyzeResult.value = '';
  try {
    const res = await sentimentApi.analyzeCamp({ text: analyzeText.value.trim(), save: analyzeSave.value });
    if (res.data?.code === 0) {
      const d = res.data.data;
      analyzeResult.value = `已标记 ${d.marks?.length || 0} 个阵营（${d.source === 'ai' ? 'AI 强中立提示词' : '兜底'}）`;
      ElMessage.success(res.data?.message || 'AI 已完成阵营标记');
      await load();
    } else {
      ElMessage.error(res.data?.message || 'AI 分析失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || 'AI 分析失败，请检查模型配置');
  } finally {
    analyzing.value = false;
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

// 普通管理员「申请修改」/ 超级管理员「直改」共用同一个对话框
function openEdit(row: CampMark, mode: 'direct' | 'request') {
  editMode.value = mode;
  editTargetId.value = row.id;
  Object.assign(editForm, {
    camp_id: row.camp_id, camp_label: row.camp_label, lang_format: row.lang_format || 'everyday',
    position: row.position, lang_evidence: row.lang_evidence, confidence: row.confidence || 0,
    note: row.note, reason: '',
  });
  editDialog.value = true;
}

async function submitEdit() {
  if (!editForm.camp_id.trim()) {
    ElMessage.warning('请填写阵营标识');
    return;
  }
  editing.value = true;
  try {
    const payload = {
      camp_id: editForm.camp_id, camp_label: editForm.camp_label,
      lang_format: editForm.lang_format, position: editForm.position,
      lang_evidence: editForm.lang_evidence, confidence: editForm.confidence, note: editForm.note,
    };
    let res: any;
    if (editMode.value === 'direct') {
      res = await sentimentApi.updateCampMark(editTargetId.value, payload);
    } else {
      res = await sentimentApi.requestUpdateCampMark(editTargetId.value, { ...payload, reason: editForm.reason });
    }
    if (res.data?.code === 0) {
      ElMessage.success(editMode.value === 'direct' ? '已修改（站长）' : '已提交修改申请，等待站长批准');
      editDialog.value = false;
      await load();
    } else {
      ElMessage.error(res.data?.message || '操作失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally {
    editing.value = false;
  }
}

// 普通管理员「申请删除」：不直删，提交申请
async function requestDelete(row: CampMark) {
  try {
    await ElMessageBox.confirm(`提交删除「${row.camp_label || row.camp_id}」的申请？将由超级管理员（站长）批准。`, '申请删除', { type: 'warning' });
  } catch {
    return;
  }
  const reason = await ElMessageBox.prompt('申请理由（给站长，可选）', '申请删除', { inputType: 'textarea', inputValue: '' })
    .then((r) => r.value || '').catch(() => '');
  try {
    const res = await sentimentApi.requestDeleteCampMark(row.id, reason);
    if (res.data?.code === 0) {
      ElMessage.success('已提交删除申请，等待站长批准');
      await load();
    } else {
      ElMessage.error(res.data?.message || '提交失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败');
  }
}

// 超级管理员（站长）直接删除
async function directDelete(row: CampMark) {
  try {
    await ElMessageBox.confirm(`直接删除阵营标记「${row.camp_label || row.camp_id}」？`, '删除标记', { type: 'warning' });
  } catch {
    return;
  }
  try {
    const res = await sentimentApi.deleteCampMark(row.id);
    if (res.data?.code === 0) {
      ElMessage.success('已删除（站长）');
      await load();
    } else {
      ElMessage.error(res.data?.message || '删除失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function approve(row: DeleteRequest) {
  try {
    await ElMessageBox.confirm(`批准${row.action === 'update' ? '修改' : '删除'}「${row.target_label}」？`, '批准申请', { type: 'warning' });
  } catch {
    return;
  }
  const res = await sentimentApi.approveDeleteRequest(row.id);
  if (res.data?.code === 0) {
    ElMessage.success(res.data?.message || '已批准');
    await load();
  } else {
    ElMessage.error(res.data?.message || '操作失败');
  }
}

async function reject(row: DeleteRequest) {
  const reason = await ElMessageBox.prompt('驳回理由（给申请人）', '驳回申请', { inputType: 'textarea', inputValue: '站长认为无需处理' })
    .then((r) => r.value || '站长认为无需处理').catch(() => '站长认为无需处理');
  try {
    const res = await sentimentApi.rejectDeleteRequest(row.id, reason);
    if (res.data?.code === 0) {
      ElMessage.success('已驳回');
      await load();
    } else {
      ElMessage.error(res.data?.message || '操作失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

onMounted(load);
</script>

<style scoped>
.camp-wrap { display: flex; flex-direction: column; gap: 12px; }
.camp-notice { line-height: 1.7; }
.camp-analyze { padding: 12px 14px; border: 1px solid color-mix(in srgb, var(--primary-color) 30%, transparent); border-radius: 10px; background: color-mix(in srgb, var(--primary-color) 5%, transparent); }
.camp-analyze__head { display: flex; align-items: baseline; gap: 8px; flex-wrap: wrap; margin-bottom: 8px; }
.camp-analyze__head b { font-size: 0.88rem; color: var(--primary-color); }
.camp-analyze__head span { font-size: 0.74rem; color: var(--text-light); }
.camp-analyze__foot { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-top: 8px; }
.camp-analyze__opt { display: flex; align-items: center; gap: 4px; font-size: 0.74rem; color: var(--text-light); }
.camp-analyze__result { font-size: 0.74rem; color: var(--success-color); }
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
.patch-pre { margin: 0; white-space: pre-wrap; word-break: break-word; font-size: 0.74rem; line-height: 1.6; color: var(--text-color); }
</style>
