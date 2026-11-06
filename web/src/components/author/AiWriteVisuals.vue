<template>
  <div class="aw-visuals">
    <el-tabs v-model="tab">
      <el-tab-pane label="时间线 · 点线法" name="timeline">
        <div class="vis-hint">解析约定：每行「时间 事件」；嵌套循环用 <code>[LOOP]</code> 开始、<code>[/LOOP]</code> 结束。</div>
        <div class="svg-wrap">
          <svg :viewBox="`0 0 ${tlSvg.w} ${tlSvg.h}`" class="vis-svg" v-if="tlEvents.length">
            <!-- 主轴 -->
            <line :x1="tlSvg.pad" :y1="tlSvg.axisY" :x2="tlSvg.w - tlSvg.pad" :y2="tlSvg.axisY" class="axis" />
            <!-- 循环弧（嵌套用不同色） -->
            <path
              v-for="(lp, i) in tlLoops"
              :key="'lp' + i"
              :d="loopArc(lp)"
              :class="['loop-arc', 'loop-' + (i % 3)]"
              :marker-end="'url(#arrow)'"
              fill="none"
            />
            <!-- 事件点 + 标签 -->
            <g v-for="(ev, i) in tlEvents" :key="'ev' + i">
              <circle
                :cx="ev.x" :cy="tlSvg.axisY"
                :r="ev.inLoop ? 7 : 5"
                :class="['ev-dot', { 'ev-inloop': ev.inLoop }]"
              />
              <text :x="ev.x" :y="tlSvg.axisY + 18" class="ev-time">{{ ev.time }}</text>
              <text :x="ev.x" :y="tlSvg.axisY - (ev.inLoop ? 16 : 12)" class="ev-desc">{{ ev.desc }}</text>
            </g>
            <defs>
              <marker id="arrow" markerWidth="10" markerHeight="10" refX="8" refY="3" orient="auto">
                <path d="M0,0 L8,3 L0,6 Z" class="arrow-head" />
              </marker>
            </defs>
          </svg>
          <el-empty v-else description="暂无时间线数据，先在「大事年表」填写" />
        </div>
      </el-tab-pane>

      <el-tab-pane label="人物关系图" name="chars">
        <div class="vis-hint">
          解析约定：人物用 <code>@姓名</code> 标注；关系行用 <code>@A → @B：关系</code> 或 <code>@A 与 @B：关系</code>。
        </div>
        <div class="svg-wrap">
          <svg :viewBox="`0 0 ${cgSvg.w} ${cgSvg.h}`" class="vis-svg" v-if="cgNodes.length">
            <!-- 边 -->
            <g v-for="(e, i) in cgEdges" :key="'e' + i">
              <line :x1="e.x1" :y1="e.y1" :x2="e.x2" :y2="e.y2" class="cg-edge" :marker-end="'url(#arrow2)'" />
              <text :x="(e.x1 + e.x2) / 2" :y="(e.y1 + e.y2) / 2 - 4" class="cg-elabel">{{ e.label }}</text>
            </g>
            <!-- 节点 -->
            <g v-for="(n, i) in cgNodes" :key="'n' + i">
              <circle :cx="n.x" :cy="n.y" r="22" class="cg-node" />
              <text :x="n.x" :y="n.y + 4" class="cg-nlabel">{{ n.name }}</text>
            </g>
            <defs>
              <marker id="arrow2" markerWidth="10" markerHeight="10" refX="8" refY="3" orient="auto">
                <path d="M0,0 L8,3 L0,6 Z" class="arrow-head" />
              </marker>
            </defs>
          </svg>
          <el-empty v-else description="未识别到 @姓名，请按约定标注人物" />
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';

const props = defineProps<{ timeline: string; characters: string }>();
const tab = ref<'timeline' | 'chars'>('timeline');

// ── 时间线解析 ──
interface TlEvent { time: string; desc: string; x: number; inLoop: boolean }
interface TlLoop { startIdx: number; endIdx: number; level: number }

const tlSvg = { w: 900, h: 200, pad: 50, axisY: 110, step: 0 };
const tlRaw = computed(() => (props.timeline || '').split('\n').map((s) => s.trim()).filter(Boolean));

const tlEvents = computed<TlEvent[]>(() => {
  const lines = tlRaw.value;
  const loopStack: TlLoop[] = [];
  const loops: TlLoop[] = [];
  const evs: TlEvent[] = [];
  for (const line of lines) {
    if (/^\[LOOP\b/i.test(line)) {
      loopStack.push({ startIdx: evs.length, endIdx: -1, level: loopStack.length });
      continue;
    }
    if (/^\[\/LOOP\]/i.test(line)) {
      const top = loopStack.pop();
      if (top) {
        top.endIdx = evs.length - 1;
        loops.push(top);
      }
      continue;
    }
    const m = line.match(/^(\S+)\s+(.*)$/);
    if (m) evs.push({ time: m[1], desc: m[2], x: 0, inLoop: loopStack.length > 0 });
  }
  // 闭合未关闭的循环
  while (loopStack.length) {
    const top = loopStack.pop()!;
    top.endIdx = evs.length - 1;
    loops.push(top);
  }
  const n = evs.length;
  const step = n > 1 ? (tlSvg.w - 2 * tlSvg.pad) / (n - 1) : 0;
  tlSvg.step = step;
  evs.forEach((e, i) => { e.x = n > 1 ? tlSvg.pad + i * step : tlSvg.w / 2; });
  tlLoops.value = loops;
  return evs;
});

const tlLoops = ref<TlLoop[]>([]);
function loopArc(lp: TlLoop): string {
  const evs = tlEvents.value;
  if (lp.startIdx < 0 || lp.endIdx < lp.startIdx || lp.endIdx >= evs.length) return '';
  const a = evs[lp.startIdx];
  const b = evs[lp.endIdx];
  const lift = 40 + lp.level * 22;
  const cx = (a.x + b.x) / 2;
  const cy = tlSvg.axisY - lift;
  return `M ${b.x} ${tlSvg.axisY - 8} Q ${cx} ${cy} ${a.x} ${tlSvg.axisY - 8}`;
}

// ── 人物关系解析 ──
interface CgNode { name: string; x: number; y: number }
interface CgEdge { x1: number; y1: number; x2: number; y2: number; label: string }

const cgSvg = { w: 700, h: 460, cx: 350, cy: 230, r: 160 };
const cgNodes = computed<CgNode[]>(() => {
  const text = props.characters || '';
  const names = new Set<string>();
  const nameRe = /@([^\s，。、,]+)/g;
  let m: RegExpExecArray | null;
  while ((m = nameRe.exec(text))) names.add(m[1]);
  const arr = Array.from(names);
  const n = arr.length;
  return arr.map((name, i) => {
    const ang = (i / Math.max(1, n)) * Math.PI * 2 - Math.PI / 2;
    return {
      name,
      x: cgSvg.cx + (n === 1 ? 0 : cgSvg.r * Math.cos(ang)),
      y: cgSvg.cy + (n === 1 ? 0 : cgSvg.r * Math.sin(ang)),
    };
  });
});

const cgEdges = computed<CgEdge[]>(() => {
  const text = props.characters || '';
  const nodes = cgNodes.value;
  const pos = new Map(nodes.map((nn) => [nn.name, nn]));
  const edges: CgEdge[] = [];
  const lines = text.split('\n');
  for (const line of lines) {
    const relMatch = line.match(/@(\S+)\s*(→|←|与|—)\s*@(\S+)\s*[:：]?\s*(.*)$/);
    if (relMatch) {
      const a = pos.get(relMatch[1]);
      const b = pos.get(relMatch[3]);
      if (a && b) edges.push({ x1: a.x, y1: a.y, x2: b.x, y2: b.y, label: relMatch[4] || '关联' });
      continue;
    }
    const multi = line.match(/@(\S+)\s*[,，、]?\s*与\s*@(\S+)\s*[:：]?\s*(.*)$/);
    if (multi) {
      const a = pos.get(multi[1]);
      const b = pos.get(multi[2]);
      if (a && b) edges.push({ x1: a.x, y1: a.y, x2: b.x, y2: b.y, label: multi[3] || '关联' });
    }
  }
  return edges;
});
</script>

<style scoped>
.vis-hint { font-size: 0.8rem; color: var(--text-light, #999); margin-bottom: 10px; }
.vis-hint code { background: rgba(139, 92, 246, 0.15); padding: 1px 5px; border-radius: 4px; }
.svg-wrap { width: 100%; overflow-x: auto; }
.vis-svg { width: 100%; min-width: 600px; background: var(--bg-card, #fff); border: 1px solid var(--border-color, #eee); border-radius: 10px; }
.axis { stroke: var(--text-light, #bbb); stroke-width: 2; }
.ev-dot { fill: #8b5cf6; }
.ev-dot.ev-inloop { fill: #ef4444; }
.ev-time { font-size: 11px; fill: var(--text-color, #333); text-anchor: middle; }
.ev-desc { font-size: 10px; fill: var(--text-light, #888); text-anchor: middle; }
.loop-arc { stroke-width: 2; stroke-dasharray: 5 4; }
.loop-0 { stroke: #ef4444; }
.loop-1 { stroke: #f59e0b; }
.loop-2 { stroke: #10b981; }
.arrow-head { fill: #ef4444; }
.cg-node { fill: #6366f1; opacity: 0.92; }
.cg-nlabel { font-size: 12px; fill: #fff; text-anchor: middle; }
.cg-edge { stroke: var(--text-light, #aaa); stroke-width: 1.5; }
.cg-elabel { font-size: 10px; fill: var(--text-light, #888); text-anchor: middle; }
</style>
