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
        <el-radio-button value="graph">关系图</el-radio-button>
        <el-radio-button value="list">互动清单</el-radio-button>
      </el-radio-group>
      <el-button v-if="filterUserID" size="small" text type="primary" @click="clearFilter">
        清除「{{ filterUserName }}」筛选 ✕
      </el-button>
      <span v-if="suspiciousCount > 0" class="cn-warn">⚠ {{ suspiciousCount }} 组高频互怼（疑似刷评/抱团）</span>
    </div>

    <!-- 关系图（ECharts graph） -->
    <div v-show="viewMode === 'graph'" class="cn-graph-wrap">
      <v-chart :option="graphOption" autoresize class="cn-graph" @click="onGraphClick" />
      <!-- 注意：v-chart 为 main.ts 中全局注册的 vue-echarts 组件 -->
      <div class="cn-graph-hint">箭头 A→B 表示「A 回复了 B」；点击节点可下钻到该评论者的互动</div>
    </div>

    <!-- 互动清单 -->
    <div v-show="viewMode === 'list'" class="cn-list">
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
import { ref, computed, watch, onMounted } from 'vue';
import { commentNetworkApi } from '@/api/novel/comments';
import { formatRelativeTime } from '@/utils/relativeTime';
import type { CommentNetwork, NetworkEdge } from '@/api/novel/types';

const props = defineProps<{
  targetType: string;
  targetID: number;
}>();

const net = ref<CommentNetwork | null>(null);
const loading = ref(false);
const viewMode = ref<'graph' | 'list'>('graph');
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

// ECharts graph option：节点=评论者，边=A→B 回复关系（权重=次数）
const graphOption = computed(() => {
  const n = net.value;
  if (!n) return { series: [] };
  const nodes = [...n.nodes]
    .sort((a, b) => b.in_degree + b.out_degree - (a.in_degree + a.out_degree))
    .slice(0, 50);
  const idSet = new Set(nodes.map((x) => x.user_id));
  const gNodes = nodes.map((x) => ({
    id: String(x.user_id),
    name: x.nickname || x.username,
    symbolSize: 14 + Math.min(30, (x.in_degree + x.out_degree) * 2),
    value: `回复${x.out_degree}/被回${x.in_degree}`,
    itemStyle: { color: x.user_id === filterUserID.value ? '#ef4444' : undefined },
  }));
  const gLinks = n.top_pairs
    .filter((p) => idSet.has(p.from_user_id) && idSet.has(p.to_user_id))
    .map((p) => ({
      source: String(p.from_user_id),
      target: String(p.to_user_id),
      value: p.count,
      lineStyle: { width: Math.min(8, 1 + p.count / 3), color: p.suspicious ? '#ef4444' : '#94a3b8' },
    }));
  return {
    tooltip: {
      formatter: (p: any) =>
        p.dataType === 'edge'
          ? `${p.data.source} → ${p.data.target}：${p.data.value} 次回复`
          : `${p.data.name}\n${p.data.value}`,
    },
    series: [
      {
        type: 'graph',
        layout: 'force',
        roam: true,
        draggable: true,
        edgeSymbol: ['none', 'arrow'],
        edgeSymbolSize: [0, 6],
        force: { repulsion: 240, edgeLength: [60, 160] },
        label: { show: true, fontSize: 10 },
        data: gNodes,
        links: gLinks,
      },
    ],
  };
});

onMounted(load);
watch(() => [props.targetType, props.targetID], load);
watch(filterUserID, () => {
  if (filterUserID.value) load();
});
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
