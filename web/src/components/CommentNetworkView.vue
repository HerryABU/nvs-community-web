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

    <!-- 视图切换：2D 形态 + 3D 形态 两组互斥 -->
    <div class="cn-toolbar">
      <span class="cn-toolbar-label">2D</span>
      <el-radio-group v-model="mode2d" size="small">
        <el-radio-button value="force">力导向</el-radio-button>
        <el-radio-button value="flow">折线流程图</el-radio-button>
        <el-radio-button value="flowchart">标准流程图</el-radio-button>
        <el-radio-button value="divergent">发散形状</el-radio-button>
        <el-radio-button value="tree">树状图</el-radio-button>
        <el-radio-button value="tree-flow">树状流程图</el-radio-button>
        <el-radio-button value="list">互动清单</el-radio-button>
      </el-radio-group>
      <span class="cn-toolbar-label">3D</span>
      <el-radio-group v-model="mode3d" size="small">
        <el-radio-button value="3d">3D 图谱</el-radio-button>
        <el-radio-button value="3d-flow">3D 流程图</el-radio-button>
      </el-radio-group>
      <span class="cn-toolbar-sep"></span>
      <label class="cn-toggle">
        <el-switch v-model="showUsername" size="small" />
        <span>用户名</span>
      </label>
      <label class="cn-toggle">
        <el-switch v-model="showComment" size="small" />
        <span>评论内容</span>
      </label>
      <el-button v-if="filterUserID" size="small" text type="primary" @click="clearFilter">
        清除「{{ filterUserName }}」筛选 ✕
      </el-button>
      <span v-if="suspiciousCount > 0" class="cn-warn">⚠ {{ suspiciousCount }} 组高频互怼（疑似刷评/抱团）</span>
    </div>

    <!-- 2D 关系图：graph 系与 tree 系各用独立 <v-chart> 实例，均通过 v-if 按需挂载，
         确保各自在「可见」状态下初始化（echarts.init 在 display:none 容器下不会创建 canvas） -->
    <div v-if="isGraph2D" class="cn-graph-wrap">
      <v-chart ref="graphChartRef" :option="graphOption" autoresize class="cn-graph" @click="onGraphClick" />
      <div class="cn-graph-hint">{{ graphHint }}</div>
    </div>
    <div v-else-if="isTree2D" class="cn-graph-wrap">
      <div ref="treeEl" class="cn-graph"></div>
      <div class="cn-graph-hint">{{ graphHint }}</div>
    </div>

    <!-- 3D 图谱 / 3D 流程图（three.js / 3d-force-graph，按需加载，可旋转） -->
    <div v-else-if="is3D" class="cn-graph-wrap">
      <div ref="graph3dEl" class="cn-graph-3d"></div>
      <div class="cn-3d-controls">
        <el-button size="small" :type="autoRotate ? 'primary' : 'default'" @click="toggleRotate">
          {{ autoRotate ? '停止旋转' : '自动旋转' }}
        </el-button>
        <el-button size="small" @click="resetView">重置视角</el-button>
      </div>
      <div class="cn-graph-hint">
        拖拽旋转 · 滚轮缩放 · 点击节点下钻到该评论者；箭头 A→B 表示「A 回复了 B」，边上文字即该条回复正文，沿回复链即可追踪内容演变。
      </div>
    </div>

    <!-- 互动清单 -->
    <div v-else-if="viewMode === 'list'" class="cn-list">
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
          <template v-if="showUsername">
            <span class="cn-edge-from">{{ e.from_name }}</span>
            <span class="cn-edge-arrow">回复了</span>
            <span class="cn-edge-to">{{ e.to_name }}</span>
          </template>
          <template v-else>
            <span class="cn-edge-from">评论者</span>
            <span class="cn-edge-arrow">回复了</span>
            <span class="cn-edge-to">评论者</span>
          </template>
          <span class="cn-edge-time">{{ formatRelativeTime(undefined, e.created_micro) }}</span>
        </div>
        <div v-if="showComment" class="cn-edge-quote">「{{ e.comment_excerpt || e.parent_excerpt }}」</div>
        <div v-if="showComment && e.parent_excerpt && e.comment_excerpt && e.parent_excerpt !== e.comment_excerpt" class="cn-edge-replyto">
          ↩ 回复：「{{ e.parent_excerpt }}」
        </div>
        <div v-if="showComment && e.is_estimated" class="cn-edge-est">
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
import * as echarts from 'echarts';
import * as THREE from 'three';
import type { CommentNetwork, NetworkEdge } from '@/api/novel/types';

// 文本截断：超长评论正文省略为 …（用于流程图框内 / 3D 边标签）
function trunc(s: string, n = 16): string {
  if (!s) return '';
  return s.length > n ? s.slice(0, n - 1) + '…' : s;
}

const props = defineProps<{
  targetType: string;
  targetID: number;
}>();

const net = ref<CommentNetwork | null>(null);
const loading = ref(false);

type ViewMode = 'force' | 'flow' | 'flowchart' | 'divergent' | 'tree' | 'tree-flow' | '3d' | '3d-flow' | 'list';
const mode2d = ref<ViewMode>('force');
const mode3d = ref<'' | '3d' | '3d-flow'>('');
const viewMode = computed<ViewMode>(() => (mode3d.value || mode2d.value) as ViewMode);
// 两组互斥：选中其一即清空另一组
watch(mode2d, (v) => { if (v) mode3d.value = ''; });
watch(mode3d, (v) => { if (v) mode2d.value = ''; });

const isGraph2D = computed(() => ['force', 'flow', 'flowchart', 'divergent'].includes(viewMode.value));
const isTree2D = computed(() => viewMode.value === 'tree' || viewMode.value === 'tree-flow');
const is3D = computed(() => viewMode.value === '3d' || viewMode.value === '3d-flow');

// 两组 2D 图表各自的 <v-chart> 实例引用（graph 系用 vue-echarts；tree 系因 vue-echarts 在
// display:none / 后期挂载场景下 echarts.init 无法创建 canvas，改为手写 ECharts 实例管理）
const graphChartRef = ref<any>(null);
const treeEl = ref<HTMLElement | null>(null);
let treeInst: any = null;
let treeRO: ResizeObserver | null = null;

function renderTree() {
  if (!treeEl.value) return;
  if (!treeInst) treeInst = echarts.init(treeEl.value);
  treeInst.setOption(treeOption.value, { notMerge: true });
  treeInst.resize();
}
function disposeTree() {
  if (treeRO) { treeRO.disconnect(); treeRO = null; }
  if (treeInst) { treeInst.dispose(); treeInst = null; }
}
// 进入/离开树状图：按需初始化与销毁（避免实例长期驻留于隐藏容器）
watch(isTree2D, async (on) => {
  await nextTick();
  if (on) {
    renderTree();
    if (treeEl.value && !treeRO) {
      treeRO = new ResizeObserver(() => treeInst?.resize());
      treeRO.observe(treeEl.value);
    }
  } else {
    disposeTree();
  }
});
// 树状图数据 / 内容开关变化时刷新
watch(treeOption, () => { if (isTree2D.value && treeInst) renderTree(); });
onMounted(() => { if (isTree2D.value) renderTree(); });
onBeforeUnmount(() => disposeTree());

// 显示开关：用户名 / 评论内容（可折叠以减小图面密度）
const showUsername = ref(true);
const showComment = ref(true);

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
  mode2d.value = 'list';
  mode3d.value = '';
}
function clearFilter() {
  filterUserID.value = 0;
  load();
}

// 点击关系图节点 → 下钻该评论者
function onGraphClick(params: any) {
  if (params?.dataType === 'node' && (params.data?.uid || params.data?.id)) {
    setFilter(Number(params.data.uid ?? params.data.id));
  }
}

// ===================== 基础图数据（Top 50 节点 + 聚合边） =====================
interface GNode { user_id: number; name: string; val: number; color?: string; }
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

// 评论内容映射：聚合边 (from->to) → 该对最新一条回复的正文（用于 2D 边标签）
const pairExcerpt = computed<Map<string, string>>(() => {
  const m = new Map<string, string>();
  if (!net.value) return m;
  for (const e of net.value.edges) {
    const key = `${e.from_user_id}->${e.to_user_id}`;
    if (!m.has(key)) m.set(key, e.comment_excerpt || e.parent_excerpt || '');
  }
  return m;
});
// 评论者 → 其最新一条回复正文（用于 3D 节点标签）
const userExcerptMap = computed<Map<number, string>>(() => {
  const m = new Map<number, string>();
  if (!net.value) return m;
  for (const e of net.value.edges) {
    if (!m.has(e.from_user_id)) {
      const ex = e.comment_excerpt || e.parent_excerpt;
      if (ex) m.set(e.from_user_id, ex);
    }
  }
  return m;
});

// 通用 BFS 分层（DAG 层级）
function computeLayers(nodes: GNode[], links: GLink[]): Map<number, number> {
  const outAdj = new Map<number, number[]>();
  const inAdj = new Map<number, number[]>();
  nodes.forEach((n) => { outAdj.set(n.user_id, []); inAdj.set(n.user_id, []); });
  links.forEach((l) => {
    outAdj.get(l.source)?.push(l.target);
    inAdj.get(l.target)?.push(l.source);
  });
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
  return level;
}

// ---------- 折线流程图：按回复层级（DAG 分层）左→右布局 ----------
function computeFlow(nodes: GNode[], links: GLink[], vertical = false): Map<number, { x: number; y: number }> {
  const level = computeLayers(nodes, links);
  const byLevel = new Map<number, GNode[]>();
  nodes.forEach((n) => {
    const lv = level.get(n.user_id)!;
    if (!byLevel.has(lv)) byLevel.set(lv, []);
    byLevel.get(lv)!.push(n);
  });
  const coords = new Map<number, { x: number; y: number }>();
  const colW = vertical ? 230 : 230;
  const rowH = vertical ? 96 : 96;
  [...byLevel.keys()].sort((a, b) => a - b).forEach((lv) => {
    const arr = byLevel.get(lv)!;
    arr.forEach((n, i) => {
      if (vertical) {
        // 自上而下：根在上（y 取负，越大层级越低）
        coords.set(n.user_id, { x: (i - (arr.length - 1) / 2) * colW, y: -lv * rowH });
      } else {
        coords.set(n.user_id, { x: lv * colW, y: (i - (arr.length - 1) / 2) * rowH });
      }
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
  const ringR = 175;
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

// ---------- 3D 流程图：层级沿 X 轴铺开，层内节点在 Y-Z 平面成环，前向边互不交叉 ----------
function computeFlow3D(nodes: GNode[], links: GLink[]): Map<number, { x: number; y: number; z: number }> {
  const level = computeLayers(nodes, links);
  const byLevel = new Map<number, GNode[]>();
  nodes.forEach((n) => {
    const lv = level.get(n.user_id)!;
    if (!byLevel.has(lv)) byLevel.set(lv, []);
    byLevel.get(lv)!.push(n);
  });
  const coords = new Map<number, { x: number; y: number; z: number }>();
  const layerGap = 200;
  const ringR = 95;
  [...byLevel.keys()].sort((a, b) => a - b).forEach((lv) => {
    const arr = byLevel.get(lv)!;
    arr.forEach((n, i) => {
      const x = lv * layerGap;
      if (arr.length === 1) { coords.set(n.user_id, { x, y: 0, z: 0 }); return; }
      const ang = (2 * Math.PI * i) / arr.length + lv * 0.5;
      coords.set(n.user_id, { x, y: Math.cos(ang) * ringR, z: Math.sin(ang) * ringR });
    });
  });
  return coords;
}

function build2DOption(kind: 'force' | 'flow' | 'flowchart' | 'divergent') {
  const g = baseGraph.value;
  const coords =
    kind === 'flow' ? computeFlow(g.nodes, g.links, false) :
    kind === 'flowchart' ? computeFlow(g.nodes, g.links, true) :
    kind === 'divergent' ? computeRadial(g.nodes, g.links) : null;
  const isRect = kind === 'flowchart';
  const data = g.nodes.map((n) => {
    // 标准流程图：框内直接呈现「用户名 + 该评论者最新评论正文」，随两个开关折叠
    const ex = userExcerptMap.value.get(n.user_id) || '';
    let labelText = '';
    if (showUsername.value) labelText += `{a|${n.name}}`;
    if (showComment.value && ex) labelText += (labelText ? '\n' : '') + `{b|${trunc(ex, 14)}}`;
    const base: any = {
      id: String(n.user_id),
      name: n.name,
      symbol: isRect ? 'rect' : 'circle',
      symbolSize: isRect ? [150, 52] : 14 + Math.min(34, n.val * 2),
      value: `回复/被回合计 ${n.val}`,
      itemStyle: isRect
        ? { color: n.color || '#eef2ff', borderColor: '#4f46e5', borderWidth: 1.5, borderRadius: 6 }
        : { color: n.color },
      label: isRect
        ? {
            show: true,
            position: 'inside',
            formatter: labelText,
            rich: {
              a: { fontSize: 12, color: '#1e293b', fontWeight: 'bold', lineHeight: 18, align: 'center' },
              b: { fontSize: 9, color: '#475569', lineHeight: 12, align: 'center' },
            },
          }
        : { show: showUsername.value, fontSize: 10 },
    };
    if (coords) { const c = coords.get(n.user_id); if (c) { base.x = c.x; base.y = c.y; } }
    return base;
  });
  const links = g.links.map((l) => ({
    source: String(l.source),
    target: String(l.target),
    value: l.count,
    excerpt: pairExcerpt.value.get(`${l.source}->${l.target}`) || '',
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
    // 内容已置于流程图框内，避免重复；其余 2D 形态在边上展示评论正文
    edgeLabel: {
      show: !isRect && showComment.value,
      fontSize: 9,
      color: '#64748b',
      formatter: (p: any) => trunc(p.data?.excerpt || '', 18),
    },
    lineStyle: { color: '#94a3b8', curveness: kind === 'divergent' ? 0.12 : 0 },
  };
  if (kind === 'force') {
    series.force = { repulsion: 1200, edgeLength: [120, 280], gravity: 0.06, friction: 0.6 };
  }
  return {
    tooltip: {
      formatter: (p: any) =>
        p.dataType === 'edge'
          ? `${p.data.source} → ${p.data.target}：${p.data.value} 次回复` +
            (showComment.value && p.data.excerpt ? `\n「${p.data.excerpt}」` : '')
          : `${p.data.name}\n${p.data.value}`,
    },
    series: [series],
  };
}

// ===================== 评论树（评论级：哪条评论引发了哪些回复） =====================
interface TreeNode {
  id: number;
  name: string;
  author: string;
  excerpt: string;
  uid: number;
  children: TreeNode[];
}
// 由 net.edges（回复→父评论 关系）构建评论森林；上限裁剪避免过载
const commentTree = computed<{ roots: TreeNode[] }>(() => {
  const n = net.value;
  if (!n) return { roots: [] };
  const info = new Map<number, { author: string; excerpt: string; uid: number }>();
  const childrenOf = new Map<number, number[]>();
  const childSet = new Set<number>();
  for (const e of n.edges) {
    const child = e.comment_id;
    const parent = e.parent_comment_id;
    if (!info.has(child)) info.set(child, { author: e.from_name, excerpt: e.comment_excerpt, uid: e.from_user_id });
    if (!info.has(parent)) info.set(parent, { author: e.to_name, excerpt: e.parent_excerpt, uid: e.to_user_id });
    if (!childrenOf.has(parent)) childrenOf.set(parent, []);
    childrenOf.get(parent)!.push(child);
    childSet.add(child);
  }
  const sizeOf = new Map<number, number>();
  const build = (id: number): TreeNode => {
    const ch = (childrenOf.get(id) || []).map(build);
    sizeOf.set(id, 1 + ch.reduce((s, c) => s + (sizeOf.get(c.id) || 0), 0));
    const inf = info.get(id)!;
    return { id, name: inf.author, author: inf.author, excerpt: inf.excerpt, uid: inf.uid, children: ch };
  };
  let roots = [...childrenOf.keys()].filter((id) => !childSet.has(id));
  if (roots.length === 0 && childrenOf.size > 0) roots = [...childrenOf.keys()];
  const rootNodes = roots.map(build).sort((a, b) => (sizeOf.get(b.id) || 0) - (sizeOf.get(a.id) || 0));
  // 总节点上限 120：优先保留子树最大的根；单根超限则按深度 3 截断
  const MAX_NODES = 120;
  const truncateDepth = (node: TreeNode, depth: number): TreeNode =>
    depth <= 0
      ? { ...node, children: [] }
      : { ...node, children: node.children.map((c) => truncateDepth(c, depth - 1)) };
  const capped: TreeNode[] = [];
  let total = 0;
  for (const r of rootNodes) {
    const sz = sizeOf.get(r.id) || 0;
    if (total + sz > MAX_NODES) {
      if (capped.length === 0) capped.push(truncateDepth(r, 3));
      break;
    }
    capped.push(r);
    total += sz;
    if (total >= MAX_NODES) break;
  }
  return { roots: capped };
});

function buildTreeOption(orient: 'TB' | 'LR') {
  const roots = commentTree.value.roots;
  if (!roots.length) {
    return {
      title: {
        text: '暂无回复关系（还没有评论引发其他评论）',
        left: 'center',
        top: 'center',
        textStyle: { color: '#999', fontSize: 14 },
      },
      series: [],
    };
  }
  const mkLabel = (d: any) => {
    let s = '';
    if (showUsername.value) s += `{a|${d.author}}`;
    if (showComment.value && d.excerpt) s += (s ? '\n' : '') + `{b|${trunc(d.excerpt, 22)}}`;
    return s;
  };
  const series: any = {
    type: 'tree',
    orient,
    roam: true,
    expandAndCollapse: true,
    initialTreeDepth: -1,
    symbol: 'circle',
    symbolSize: 11,
    data: roots,
    top: '3%',
    bottom: '3%',
    left: orient === 'LR' ? '6%' : '8%',
    right: orient === 'LR' ? '14%' : '8%',
    edgeShape: 'curve',
    edgeForkPosition: '60%',
    lineStyle: { color: '#94a3b8', width: 1, curveness: 0.5 },
    label: {
      show: true,
      fontSize: 10,
      color: '#1e293b',
      formatter: (p: any) => mkLabel(p.data),
      rich: {
        a: { fontSize: 11, color: '#4f46e5', fontWeight: 'bold', lineHeight: 15, align: 'center' },
        b: { fontSize: 9, color: '#475569', lineHeight: 12, align: 'center' },
      },
    },
    emphasis: { focus: 'descendant' },
    animationDuration: 300,
    animationDurationUpdate: 300,
  };
  return {
    tooltip: {
      formatter: (p: any) => {
        const d = p.data;
        let s = `${d.author}`;
        if (showComment.value && d.excerpt) s += `\n「${d.excerpt}」`;
        return s;
      },
    },
    series: [series],
  };
}

const forceOption = computed(() => build2DOption('force'));
const flowOption = computed(() => build2DOption('flow'));
const flowchartOption = computed(() => build2DOption('flowchart'));
const divergentOption = computed(() => build2DOption('divergent'));
const treeOptionTB = computed(() => buildTreeOption('TB'));
const treeOptionLR = computed(() => buildTreeOption('LR'));

const graphOption = computed(() => {
  if (!net.value) return { series: [] };
  switch (viewMode.value) {
    case 'flow': return flowOption.value;
    case 'flowchart': return flowchartOption.value;
    case 'divergent': return divergentOption.value;
    default: return forceOption.value;
  }
});
const treeOption = computed(() => {
  if (!net.value) return { series: [] };
  return viewMode.value === 'tree-flow' ? treeOptionLR.value : treeOptionTB.value;
});

const graphHint = computed(() => {
  switch (viewMode.value) {
    case 'flow': return '折线流程图：按回复层级从左到右分层，箭头 A→B 表示「A 回复了 B」。';
    case 'flowchart': return '标准流程图：根评论者居顶，按回复层级自上而下展开；方框内即「用户名 + 其最新评论正文」，箭头即回复方向，沿链读可见内容演变。';
    case 'divergent': return '发散形状：中心为最活跃评论者，其余按互动距离呈辐射环展开。';
    case 'tree': return '树状图：以「评论」为节点，父子关系即「子评论回复了父评论」；沿枝向下即可看清哪条评论引发了哪些回复。可拖拽缩放、点击节点折叠/展开分支。';
    case 'tree-flow': return '树状流程图：横向展开评论回复树，根评论在左、回复向右衍生；箭头起点（父）引发箭头终点（子）。';
    default: return '箭头 A→B 表示「A 回复了 B」；点击节点可下钻到该评论者的互动。';
  }
});

// ===================== 3D 图谱（3d-force-graph，按需加载，可旋转） =====================
const graph3dEl = ref<HTMLElement | null>(null);
let fgInstance: any = null;
const autoRotate = ref(false);

function applyRotationState() {
  if (!fgInstance) return;
  try {
    const c = fgInstance.controls();
    c.autoRotate = autoRotate.value;
    c.autoRotateSpeed = 1.4;
  } catch { /* noop */ }
}
function toggleRotate() {
  autoRotate.value = !autoRotate.value;
  applyRotationState();
}
function resetView() {
  if (!fgInstance) return;
  try { fgInstance.zoomToFit(500, 60); } catch { /* noop */ }
}

function build3DData(mode: ViewMode) {
  const g = baseGraph.value;
  const coords3d = mode === '3d-flow' ? computeFlow3D(g.nodes, g.links) : null;
  const nameOf = (id: number) => g.nodes.find((n) => n.user_id === id)?.name || `评论者${id}`;
  const nodes = g.nodes.map((n) => {
    const o: any = { id: String(n.user_id), name: n.name, val: n.val, color: n.color || '#4f46e5' };
    if (coords3d) {
      const c = coords3d.get(n.user_id);
      if (c) { o.x = c.x; o.y = c.y; o.z = c.z; o.fx = c.x; o.fy = c.y; o.fz = c.z; }
    }
    return o;
  });
  const links = g.links.map((l) => ({
    source: String(l.source),
    target: String(l.target),
    count: l.count,
    suspicious: l.suspicious,
    fromName: nameOf(l.source),
    toName: nameOf(l.target),
    excerpt: pairExcerpt.value.get(`${l.source}->${l.target}`) || '',
  }));
  return { nodes, links };
}

// 生成贴边的评论正文文字精灵（canvas 纹理），用于 3D 边上持久显示内容演变
function makeLinkSprite(text: string): THREE.Sprite | null {
  if (!text) return null;
  const t = trunc(text, 16);
  const pad = 8;
  const fontPx = 28;
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d')!;
  ctx.font = `${fontPx}px sans-serif`;
  const w = Math.ceil(ctx.measureText(t).width) + pad * 2;
  const h = fontPx + pad * 2;
  canvas.width = w;
  canvas.height = h;
  const c2 = canvas.getContext('2d')!;
  c2.font = `${fontPx}px sans-serif`;
  c2.fillStyle = 'rgba(15,23,42,0.82)';
  const r = 8;
  c2.beginPath();
  c2.moveTo(r, 0);
  c2.arcTo(w, 0, w, h, r);
  c2.arcTo(w, h, 0, h, r);
  c2.arcTo(0, h, 0, 0, r);
  c2.arcTo(0, 0, w, 0, r);
  c2.closePath();
  c2.fill();
  c2.fillStyle = '#e2e8f0';
  c2.textBaseline = 'middle';
  c2.fillText(t, pad, h / 2);
  const tex = new THREE.CanvasTexture(canvas);
  tex.minFilter = THREE.LinearFilter;
  const mat = new THREE.SpriteMaterial({ map: tex, transparent: true, depthWrite: false, depthTest: false });
  const sprite = new THREE.Sprite(mat);
  const scale = 14; // 世界单位/像素（让文字在 3D 场景里可读）
  sprite.scale.set((w / h) * scale, scale, 1);
  return sprite;
}

async function mount3D() {
  if (!graph3dEl.value) return;
  if (fgInstance) { try { fgInstance.graphData({ nodes: [], links: [] }); } catch { /* noop */ } fgInstance = null; }
  graph3dEl.value.innerHTML = '';
  const ForceGraph3D = (await import('3d-force-graph')).default;
  const el = graph3dEl.value;
  const mode = viewMode.value;
  fgInstance = ForceGraph3D()(el)
    .graphData(build3DData(mode))
    .nodeId('id')
    .nodeLabel((n: any) => {
      const parts: string[] = [];
      if (showUsername.value) parts.push(n.name);
      if (showComment.value) {
        const ex = userExcerptMap.value.get(Number(n.id));
        if (ex) parts.push(ex);
      }
      return parts.join('\n');
    })
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
    .linkLabel((l: any) =>
      showComment.value && l.excerpt
        ? `${l.fromName} → ${l.toName}：${trunc(l.excerpt, 40)}`
        : `${l.fromName} → ${l.toName}`,
    )
    // 边上持久显示评论正文，便于沿回复链追踪内容演变；随「评论内容」开关折叠
    .linkThreeObjectExtend(true)
    .linkThreeObject((l: any) => (showComment.value ? makeLinkSprite(l.excerpt) : null))
    .onNodeClick((n: any) => setFilter(Number(n.id)))
    .backgroundColor('rgba(15,23,42,0.04)');
  // 3D 流程图：图层固定、停止力导模拟，路线沿 X 轴前向推进互不交叉
  if (mode === '3d-flow') {
    try { fgInstance.cooldownTicks(0); } catch { /* noop */ }
  }
  fgInstance.width(el.clientWidth || 600).height(420);
  applyRotationState();
}

function refresh3D() {
  if (!fgInstance) return;
  fgInstance.graphData(build3DData(viewMode.value));
  if (viewMode.value === '3d-flow') { try { fgInstance.cooldownTicks(0); } catch { /* noop */ } }
  applyRotationState();
}

function onResize() {
  if (fgInstance && graph3dEl.value) fgInstance.width(graph3dEl.value.clientWidth);
}

const is3DMode = (x: string) => x === '3d' || x === '3d-flow';
watch(viewMode, async (m, old) => {
  if (is3DMode(m)) {
    await nextTick();
    await mount3D();
  } else if (fgInstance) {
    fgInstance = null;
    if (graph3dEl.value) graph3dEl.value.innerHTML = '';
  }
});

// 数据/筛选/显示开关变化时，若正处于 3D 模式则刷新
watch([filterUserID, net, showUsername, showComment], () => { if (is3DMode(viewMode.value) && fgInstance) refresh3D(); });

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

.cn-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; flex-wrap: wrap; }
.cn-toolbar-label { font-size: 0.8rem; color: var(--text-light, #999); font-weight: 600; }
.cn-toolbar-sep { width: 1px; height: 20px; background: var(--border-color, #e2e8f0); margin: 0 2px; }
.cn-toggle { display: inline-flex; align-items: center; gap: 4px; font-size: 0.8rem; color: var(--text-light, #999); }
.cn-warn { font-size: 0.8rem; color: #ef4444; }

.cn-graph-wrap { position: relative; }
.cn-graph { height: 420px; width: 100%; }
.cn-graph-3d { height: 420px; width: 100%; border-radius: 10px; overflow: hidden; background: rgba(15, 23, 42, 0.04); }
.cn-3d-controls { display: flex; gap: 8px; justify-content: center; margin-top: 6px; }
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
.cn-edge-replyto {
  font-size: 0.78rem; color: var(--text-light, #999); margin-top: 2px;
  border-left: 2px solid #cbd5e1; padding-left: 8px;
}
.cn-edge-est {
  font-size: 0.8rem; color: #d97706; margin-top: 2px;
  background: rgba(217, 119, 6, 0.08); border-radius: 6px; padding: 2px 8px;
}
</style>
