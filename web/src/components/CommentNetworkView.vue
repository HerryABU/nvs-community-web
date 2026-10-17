<template>
  <div class="comment-network" v-loading="loading">
    <!-- 概览统计 -->
    <div class="cn-stats" v-if="net">
      <div class="cn-stat">
        <div class="cn-stat-num">{{ net.total_comments }}</div>
        <div class="cn-stat-label">评论总数</div>
      </div>
      <div class="cn-stat">
        <div class="cn-stat-num">{{ net.commenter_count }}</div>
        <div class="cn-stat-label">参与评论者</div>
      </div>
      <div class="cn-stat">
        <div class="cn-stat-num">{{ net.interaction_count }}</div>
        <div class="cn-stat-label">互动边数</div>
      </div>
    </div>

    <!-- 视图切换 -->
    <div class="cn-toolbar">
      <el-radio-group v-model="viewMode" size="small">
        <el-radio-button value="force">力导向</el-radio-button>
        <el-radio-button value="flow">折线流程图</el-radio-button>
        <el-radio-button value="divergent">发散形状</el-radio-button>
        <el-radio-button value="3d">3D 图谱</el-radio-button>
        <el-radio-button value="list">互动清单</el-radio-button>
      </el-radio-group>
      <el-button v-if="filterUserID" size="small" text type="primary" @click="clearFilter">
        清除「{{ filterUserName }}」筛选 ✕
      </el-button>
      <span v-if="suspiciousCount > 0" class="cn-warn">⚠ {{ suspiciousCount }} 组高频互怼（疑似刷评/抱团）</span>
    </div>

    <!-- 2D 关系图（ECharts graph，多形态） -->
    <div v-if="viewMode !== 'list' && viewMode !== '3d'" class="cn-graph-wrap">
      <v-chart :option="currentGraphOption" autoresize class="cn-graph" @click="onGraphClick" />
      <div class="cn-graph-hint">{{ graphHint }}</div>
    </div>

    <!-- 3D 图谱（three.js / 3d-force-graph，按需加载） -->
    <div v-else-if="viewMode === '3d'" class="cn-graph-wrap">
      <div ref="graph3dEl" class="cn-graph-3d"></div>
      <div class="cn-graph-hint">
        拖拽旋转 · 滚轮缩放 · 点击节点下钻到该评论者；箭头 A→B 表示「A 回复了 B」，链路粒子流即回复流向。
      </div>
    </div>

    <!-- 互动清单 -->
    <div v-else class="cn-list">
      <h4 class="cn-subtitle">高频互动 TOP（A 回复 B）</h4>
      <div
        v-for="p in net.top_pairs"
        :key="p.from_user_id + '-' + p.to_user_id"
        class="cn-pair"
        :class="{ suspicious: p.suspicious }"
        @click="setFilter(p.from_user_id)"
      >
        <span class="cn-pair-a">{{ p.from_name }}</span>
        <span class="cn-pair-arrow">→</span>
        <span class="cn-pair-b">{{ p.to_name }}</span>
        <span class="cn-pair-count">{{ p.count }} 次</span>
        <el-tag v-if="p.suspicious" size="small" type="danger" effect="dark">疑似刷评</el-tag>
      </div>

      <h4 class="cn-subtitle">评论网明细（谁回复了谁的哪条评论）</h4>
      <div v-if="visibleEdges.length === 0" class="cn-empty">暂无互动数据</div>
      <div v-for="e in visibleEdges" :key="e.comment_id" class="cn-edge">
        <div class="cn-edge-head">
          <span class="cn-edge-from">{{ e.from_name }}</span>
          <span class="cn-edge-arrow">回复了</span>
          <span class="cn-edge-to">{{ e.to_name }}</span>
          <span class="cn-edge-time">{{ formatRelativeTime(undefined, e.created_micro) }}</span>
        </div>
        <div class="cn-edge-quote">「{{ e.parent_excerpt }}」</div>
        <div v-if="e.is_estimated" class="cn-edge-est">
          ⟳ 时间估计：大概率实际指向其更新的评论「{{ e.estimated_target_excerpt }}」
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue';
import { commentNetworkApi } from '@/api/novel/comments';
import { formatRelativeTime } from '@/utils/relativeTime';
import type { CommentNetwork, NetworkEdge } from '@/api/novel/types';

const props = defineProps<{
  targetType: string;
  targetID: number;
}>();

const net = ref<CommentNetwork | null>(null);
const loading = ref(false);
const viewMode = ref<'force' | 'flow' | 'divergent' | '3d' | 'list'>('force');
const filterUserID = ref<number>(0);

const filterUserName = computed(() => {
  if (!filterUserID.value || !net.value) return '';
  const u = net.value.nodes.find((n) => n.user_id === filterUserID.value);
  return u ? u.nickname || u.username : '';
});
const suspiciousCount = computed(() => (net.value ? net.value.top_pairs.filter((p) => p.suspicious).length : 0));

// 明细：按筛选用户下钻
const visibleEdges = computed<NetworkEdge[]>(() => {
  if (!net.value) return [];
  if (!filterUserID.value) return net.value.edges;
  return net.value.edges.filter(
    (e) => e.from_user_id === filterUserID.value || e.to_user_id === filterUserID.value,
  );
});

async function load() {
  if (!props.targetType || !props.targetID) return;
  loading.value = true;
  try {
    const res = await commentNetworkApi.get({
      target_type: props.targetType,
      target_id: props.targetID,
      user_id: filterUserID.value || undefined,
    });
    net.value = res.data?.data || null;
  } catch {
    net.value = null;
  } finally {
    loading.value = false;
  }
}

function setFilter(uid: number) {
  filterUserID.value = uid;
  viewMode.value = 'list';
}
function clearFilter() {
  filterUserID.value = 0;
  load();
}

// 点击关系图节点 → 下钻该评论者
function onGraphClick(params: any) {
  if (params?.dataType === 'node' && params.data?.id) {
    setFilter(Number(params.data.id));
  }
}

// ===================== 基础图数据（Top 50 节点 + 聚合边） =====================
interface GNode { user_id: number; name: string; val: number; color?: string; x?: number; y?: number; }
interface GLink { source: number; target: number; count: number; suspicious: boolean; }

const baseGraph = computed<{ nodes: GNode[]; links: GLink[] }>(() => {
  const n = net.value;
  if (!n) return { nodes: [], links: [] };
  const nodes = [...n.nodes]
    .sort((a, b) => b.in_degree + b.out_degree - (a.in_degree + a.out_degree))
    .slice(0, 50)
    .map<GNode>((x) => ({
      user_id: x.user_id,
      name: x.nickname || x.username,
      val: x.in_degree + x.out_degree,
      color: x.user_id === filterUserID.value ? '#ef4444' : undefined,
    }));
  const idSet = new Set(nodes.map((x) => x.user_id));
  const links = n.top_pairs
    .filter((p) => idSet.has(p.from_user_id) && idSet.has(p.to_user_id))
    .map<GLink>((p) => ({
      source: p.from_user_id,
      target: p.to_user_id,
      count: p.count,
      suspicious: p.suspicious,
    }));
  return { nodes, links };
});

// ---------- 折线流程图：按回复层级（DAG 分层）左→右布局 ----------
function computeFlow(nodes: GNode[], links: GLink[]): Map<number, { x: number; y: number }> {
  const outAdj = new Map<number, number[]>();
  const inAdj = new Map<number, number[]>();
  nodes.forEach((n) => { outAdj.set(n.user_id, []); inAdj.set(n.user_id, []); });
  links.forEach((l) => {
    outAdj.get(l.source)?.push(l.target);
    inAdj.get(l.target)?.push(l.source);
  });
  // 根节点：无入边；若无则取出度最高者
  let roots = nodes.filter((n) => (inAdj.get(n.user_id)?.length || 0) === 0).map((n) => n.user_id);
  if (roots.length === 0 && nodes.length) {
    const r = [...nodes].sort((a, b) => b.val - a.val)[0];
    roots = [r.user_id];
  }
  const level = new Map<number, number>();
  const queue: number[] = [];
  roots.forEach((r) => { level.set(r, 0); queue.push(r); });
  while (queue.length) {
    const u = queue.shift()!;
    const lv = level.get(u)!;
    for (const v of outAdj.get(u) || []) {
      if (!level.has(v) || level.get(v)! > lv + 1) {
        level.set(v, lv + 1);
        queue.push(v);
      }
    }
  }
  nodes.forEach((n) => { if (!level.has(n.user_id)) level.set(n.user_id, 0); });
  // 按层级分组
  const byLevel = new Map<number, GNode[]>();
  nodes.forEach((n) => {
    const lv = level.get(n.user_id)!;
    if (!byLevel.has(lv)) byLevel.set(lv, []);
    byLevel.get(lv)!.push(n);
  });
  const coords = new Map<number, { x: number; y: number }>();
  const colW = 180;
  const rowH = 72;
  [...byLevel.keys()].sort((a, b) => a - b).forEach((lv) => {
    const arr = byLevel.get(lv)!;
    arr.forEach((n, i) => {
      coords.set(n.user_id, { x: lv * colW, y: (i - (arr.length - 1) / 2) * rowH });
    });
  });
  return coords;
}

// ---------- 发散形状：中心枢纽 + 辐射环布局 ----------
function computeRadial(nodes: GNode[], links: GLink[]): Map<number, { x: number; y: number }> {
  if (!nodes.length) return new Map();
  const hub = [...nodes].sort((a, b) => b.val - a.val)[0];
  const hubId = hub.user_id;
  const dist = new Map<number, number>([[hubId, 0]]);
  const queue: number[] = [hubId];
  while (queue.length) {
    const u = queue.shift()!;
    const d = dist.get(u)!;
    for (const l of links) {
      if (l.source === u && !dist.has(l.target)) { dist.set(l.target, d + 1); queue.push(l.target); }
      if (l.target === u && !dist.has(l.source)) { dist.set(l.source, d + 1); queue.push(l.source); }
    }
  }
  nodes.forEach((n) => { if (!dist.has(n.user_id)) dist.set(n.user_id, 1); });
  const byRing = new Map<number, GNode[]>();
  nodes.forEach((n) => {
    const r = dist.get(n.user_id)!;
    if (!byRing.has(r)) byRing.set(r, []);
    byRing.get(r)!.push(n);
  });
  const coords = new Map<number, { x: number; y: number }>();
  const ringR = 130;
  [...byRing.keys()].sort((a, b) => a - b).forEach((r) => {
    const arr = byRing.get(r)!;
    arr.forEach((n, i) => {
      if (r === 0) { coords.set(n.user_id, { x: 0, y: 0 }); return; }
      const ang = (2 * Math.PI * i) / arr.length + r * 0.45;
      coords.set(n.user_id, { x: Math.cos(ang) * r * ringR, y: Math.sin(ang) * r * ringR });
    });
  });
  return coords;
}

function build2DOption(kind: 'force' | 'flow' | 'divergent') {
  const g = baseGraph.value;
  const coords = kind === 'flow' ? computeFlow(g.nodes, g.links) : kind === 'divergent' ? computeRadial(g.nodes, g.links) : null;
  const data = g.nodes.map((n) => {
    const base: any = {
      id: String(n.user_id),
      name: n.name,
      symbolSize: 14 + Math.min(34, n.val * 2),
      value: `回复/被回合计 ${n.val}`,
      itemStyle: { color: n.color },
    };
    if (coords) { const c = coords.get(n.user_id); if (c) { base.x = c.x; base.y = c.y; } }
    return base;
  });
  const links = g.links.map((l) => ({
    source: String(l.source),
    target: String(l.target),
    value: l.count,
    lineStyle: { width: Math.min(8, 1 + l.count / 3), color: l.suspicious ? '#ef4444' : '#94a3b8' },
  }));
  const series: any = {
    type: 'graph',
    layout: kind === 'force' ? 'force' : 'none',
    roam: true,
    draggable: true,
    edgeSymbol: ['none', 'arrow'],
    edgeSymbolSize: [0, 6],
    label: { show: true, fontSize: 10 },
    data,
    links,
    lineStyle: { color: '#94a3b8', curveness: kind === 'divergent' ? 0.12 : 0 },
  };
  if (kind === 'force') {
    series.force = { repulsion: 240, edgeLength: [60, 160] };
  }
  return {
    tooltip: {
      formatter: (p: any) =>
        p.dataType === 'edge'
          ? `${p.data.source} → ${p.data.target}：${p.data.value} 次回复`
          : `${p.data.name}\n${p.data.value}`,
    },
    series: [series],
  };
}

const forceOption = computed(() => build2DOption('force'));
const flowOption = computed(() => build2DOption('flow'));
const divergentOption = computed(() => build2DOption('divergent'));

const currentGraphOption = computed(() => {
  if (!net.value) return { series: [] };
  switch (viewMode.value) {
    case 'flow': return flowOption.value;
    case 'divergent': return divergentOption.value;
    default: return forceOption.value;
  }
});

const graphHint = computed(() => {
  switch (viewMode.value) {
    case 'flow': return '折线流程图：按回复层级从左到右分层，箭头 A→B 表示「A 回复了 B」。';
    case 'divergent': return '发散形状：中心为最活跃评论者，其余按互动距离呈辐射环展开。';
    default: return '箭头 A→B 表示「A 回复了 B」；点击节点可下钻到该评论者的互动。';
  }
});

// ===================== 3D 图谱（3d-force-graph，按需加载） =====================
const graph3dEl = ref<HTMLElement | null>(null);
let fgInstance: any = null;

function build3DData() {
  const g = baseGraph.value;
  const nodes = g.nodes.map((n) => ({ id: String(n.user_id), name: n.name, val: n.val, color: n.color || '#4f46e5' }));
  const links = g.links.map((l) => ({
    source: String(l.source),
    target: String(l.target),
    count: l.count,
    suspicious: l.suspicious,
  }));
  return { nodes, links };
}

async function mount3D() {
  if (fgInstance || !graph3dEl.value) return;
  const ForceGraph3D = (await import('3d-force-graph')).default;
  const el = graph3dEl.value;
  fgInstance = ForceGraph3D()(el)
    .graphData(build3DData())
    .nodeId('id')
    .nodeLabel('name')
    .nodeVal('val')
    .nodeColor((n: any) => n.color)
    .nodeOpacity(0.95)
    .nodeResolution(12)
    .linkColor((l: any) => (l.suspicious ? '#ef4444' : '#94a3b8'))
    .linkWidth((l: any) => Math.min(3, 0.5 + (l.count || 1) / 5))
    .linkDirectionalArrowLength(3.5)
    .linkDirectionalArrowRelPos(1)
    .linkDirectionalParticles(2)
    .linkDirectionalParticleWidth(1.6)
    .linkDirectionalParticleColor((l: any) => (l.suspicious ? '#ef4444' : '#818cf8'))
    .onNodeClick((n: any) => setFilter(Number(n.id)))
    .backgroundColor('rgba(15,23,42,0.04)');
  fgInstance.width(el.clientWidth || 600).height(420);
}

function refresh3D() {
  if (fgInstance) fgInstance.graphData(build3DData());
}

function onResize() {
  if (fgInstance && graph3dEl.value) fgInstance.width(graph3dEl.value.clientWidth);
}

watch(viewMode, async (m) => {
  if (m === '3d') {
    await nextTick();
    await mount3D();
  } else if (fgInstance) {
    try { fgInstance.pauseAnimation(); } catch { /* noop */ }
    fgInstance = null; // 容器由 v-if 移除，下次进入重建
  }
});

// 数据/筛选变化时，若正处于 3D 模式则刷新
watch([filterUserID, net], () => { if (viewMode.value === '3d' && fgInstance) refresh3D(); });

onMounted(() => {
  load();
  window.addEventListener('resize', onResize);
});
onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize);
  if (fgInstance) { try { fgInstance.graphData({ nodes: [], links: [] }); } catch { /* noop */ } fgInstance = null; }
});

watch(() => [props.targetType, props.targetID], load);
watch(filterUserID, () => { if (filterUserID.value) load(); });
</script>

<style scoped>
.comment-network { padding: 4px 0; }
.cn-stats { display: flex; gap: 12px; margin-bottom: 12px; }
.cn-stat {
  flex: 1;
  background: var(--bg-subtle, #f8fafc);
  border-radius: 10px;
  padding: 10px;
  text-align: center;
}
.cn-stat-num { font-size: 1.4rem; font-weight: 700; color: var(--primary-color, #4f46e5); }
.cn-stat-label { font-size: 0.78rem; color: var(--text-light, #999); }

.cn-toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 10px; flex-wrap: wrap; }
.cn-warn { font-size: 0.8rem; color: #ef4444; }

.cn-graph-wrap { position: relative; }
.cn-graph { height: 420px; width: 100%; }
.cn-graph-3d { height: 420px; width: 100%; border-radius: 10px; overflow: hidden; background: rgba(15, 23, 42, 0.04); }
.cn-graph-hint { font-size: 0.76rem; color: var(--text-light, #999); text-align: center; margin-top: 4px; }

.cn-subtitle { font-size: 0.9rem; margin: 14px 0 8px; color: var(--text, #333); }
.cn-pair {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 10px; border-radius: 8px; cursor: pointer;
  font-size: 0.88rem; border: 1px solid transparent;
}
.cn-pair:hover { background: var(--bg-subtle, #f8fafc); }
.cn-pair.suspicious { background: rgba(239, 68, 68, 0.08); border-color: rgba(239, 68, 68, 0.3); }
.cn-pair-a, .cn-pair-b { font-weight: 600; color: var(--primary-color, #4f46e5); }
.cn-pair-arrow { color: var(--text-light, #999); }
.cn-pair-count { margin-left: auto; color: var(--text-light, #999); font-size: 0.82rem; }

.cn-empty { color: var(--text-light, #999); font-size: 0.85rem; padding: 12px 0; }
.cn-edge { padding: 8px 10px; border-bottom: 1px solid var(--border-color, #eee); }
.cn-edge-head { display: flex; align-items: center; gap: 6px; font-size: 0.88rem; }
.cn-edge-from, .cn-edge-to { font-weight: 600; color: var(--primary-color, #4f46e5); }
.cn-edge-arrow, .cn-edge-time { color: var(--text-light, #999); font-size: 0.8rem; }
.cn-edge-time { margin-left: auto; }
.cn-edge-quote {
  font-size: 0.82rem; color: var(--text, #444); margin-top: 2px;
  border-left: 2px solid var(--border-color, #e2e8f0); padding-left: 8px;
}
.cn-edge-est {
  font-size: 0.8rem; color: #d97706; margin-top: 2px;
  background: rgba(217, 119, 6, 0.08); border-radius: 6px; padding: 2px 8px;
}
</style>
