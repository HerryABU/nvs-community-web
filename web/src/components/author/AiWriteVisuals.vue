<template>
  <div class="aw-visuals">
    <el-tabs v-model="tab">
      <el-tab-pane label="时间线 · 点线法" name="timeline">
        <div class="vis-hint">时间线约定（由 AI 写作系统提示词定义，可在后台自定义）：每行「时间 事件」；嵌套循环 <code>[LOOP:id]…[/LOOP]</code>；特殊回环用锚点 <code>[A]</code> + 回溯语句 <code>ret</code> / <code>returnA</code>（点状跳回，画回环箭线）。用户输入经 AI 归一化后前端渲染。</div>
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
            <!-- 回环/回溯箭线（ret / returnA，从回溯点连回目标锚点） -->
            <path
              v-for="(rp, i) in tlReturns"
              :key="'rp' + i"
              :d="rp.d"
              class="ret-arc"
              :marker-end="'url(#arrowRet)'"
              fill="none"
            />
            <!-- 事件点 + 标签 -->
            <g v-for="(ev, i) in tlEvents" :key="'ev' + i">
              <circle
                :cx="ev.x" :cy="tlSvg.axisY"
                :r="ev.isReturn ? 6 : (ev.inLoop ? 7 : 5)"
                :class="['ev-dot', { 'ev-inloop': ev.inLoop, 'ev-return': ev.isReturn }]"
              />
              <text :x="ev.x" :y="tlSvg.axisY + 18" class="ev-time">{{ ev.time }}</text>
              <text :x="ev.x" :y="tlSvg.axisY - (ev.isReturn ? 18 : (ev.inLoop ? 16 : 12))" class="ev-desc">{{ ev.desc }}</text>
              <text v-if="ev.isReturn" :x="ev.x" :y="tlSvg.axisY - 30" class="ev-return-label">↩{{ ev.returnTo || '起点' }}</text>
            </g>
            <defs>
              <marker id="arrow" markerWidth="10" markerHeight="10" refX="8" refY="3" orient="auto">
                <path d="M0,0 L8,3 L0,6 Z" class="arrow-head" />
              </marker>
              <marker id="arrowRet" markerWidth="10" markerHeight="10" refX="8" refY="3" orient="auto">
                <path d="M0,0 L8,3 L0,6 Z" class="arrow-head-ret" />
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
// 约定（与后端 ParseLoops + authorai-timeline 系统提示词一致）：
//   [LOOP:id]…[/LOOP] 或 [LOOPEND:id]  嵌套循环块
//   [A] 时间锚点（可附描述）            特殊回环锚点
//   ret / returnA / retA                回溯语句（点状跳回，前端画回环箭线）
// 前端负责渲染流程图；用户输入经 AI 洗稿归一化后落此格式。
interface TlEvent { time: string; desc: string; x: number; inLoop: boolean; isReturn?: boolean; returnTo?: string }
interface TlLoop { id: string; startIdx: number; endIdx: number; level: number }
interface TlReturn { d: string; target: string }

const tlSvg = { w: 900, h: 220, pad: 50, axisY: 110, step: 0 };
const tlRaw = computed(() => (props.timeline || '').split('\n').map((s) => s.trim()).filter(Boolean));

const tlLoops = ref<TlLoop[]>([]);
const tlReturns = ref<TlReturn[]>([]);

const tlEvents = computed<TlEvent[]>(() => {
  const lines = tlRaw.value;
  const loopStack: TlLoop[] = [];
  const loops: TlLoop[] = [];
  const evs: TlEvent[] = [];
  // 锚点：名称 → 事件索引（[A] 显式声明 + [LOOP:id] 开启隐式锚点）
  const anchorPos: Record<string, number> = {};
  const anchorOrder: { name: string; idx: number }[] = [];
  const returns: { fromIdx: number; target: string }[] = [];

  const pushEvent = (time: string, desc: string, isReturn = false, returnTo = '') => {
    evs.push({ time, desc, x: 0, inLoop: loopStack.length > 0, isReturn, returnTo });
  };

  for (const line of lines) {
    // 循环开：[LOOP:id] 或 [LOOP]（自动编号）
    const openM = line.match(/^\[LOOP(?::([^\]]+))?\]/i);
    if (openM) {
      const id = (openM[1] || '').trim() || `loop_${loopStack.length + 1}`;
      loopStack.push({ id, startIdx: evs.length, endIdx: -1, level: loopStack.length });
      anchorPos[id] = evs.length; anchorOrder.push({ name: id, idx: evs.length });
      continue;
    }
    // 循环闭：[/LOOP]（LIFO 关闭栈顶）或 [LOOPEND:id]（按标识关闭）
    const closeM = line.match(/^\[\/LOOP\]|^\[LOOPEND(?::([^\]]+))?\]/i);
    if (closeM) {
      const id = (closeM[1] || '').trim();
      if (id) {
        // 从栈顶向下找到首个匹配 id，关闭其上的所有内层循环（按嵌套顺序）
        let pop = -1;
        for (let k = loopStack.length - 1; k >= 0; k--) {
          if (loopStack[k].id === id) { pop = k; break; }
        }
        if (pop < 0) { continue; } // 找不到匹配 id，忽略
        for (let k = loopStack.length - 1; k >= pop; k--) {
          const top = loopStack[k];
          if (top) { top.endIdx = evs.length - 1; loops.push(top); }
        }
        loopStack.splice(pop);
      } else {
        const top = loopStack.pop();
        if (top) { top.endIdx = evs.length - 1; loops.push(top); }
      }
      continue;
    }
    // 锚点声明：[A]（可附描述，如 [A] 1990 世界线开启）
    const anchorM = line.match(/^\[([A-Za-z][A-Za-z0-9_]*)\]/);
    if (anchorM) {
      const name = anchorM[1];
      const up = name.toUpperCase();
      if (up === 'LOOP' || up.startsWith('LOOPEND')) {
        // 理论不可达（已被开/闭处理），当普通事件
      } else {
        const rest = line.slice(anchorM[0].length).trim();
        let idx = evs.length;
        if (rest) { const m = rest.match(/^(\S+)\s+(.*)$/); pushEvent(m ? m[1] : rest, m ? m[2] : ''); idx = evs.length - 1; }
        anchorPos[name] = idx;
        anchorOrder.push({ name, idx });
        continue;
      }
    }
    // 回溯语句：ret / returnA / retA
    const retM = line.match(/^ret(?:urn)?([A-Za-z0-9_]*)/i);
    if (retM) {
      const target = (retM[1] || '').trim();
      const rest = line.slice(retM[0].length).trim();
      pushEvent(target ? `return${target}` : 'ret', rest, true, target);
      returns.push({ fromIdx: evs.length - 1, target });
      continue;
    }
    // 普通事件：时间 事件简述
    const m = line.match(/^(\S+)\s+(.*)$/);
    if (m) pushEvent(m[1], m[2]);
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

  // 回环箭线：从回溯点连回目标锚点（前端渲染；目标未定义则跳过）
  const rets: TlReturn[] = [];
  for (const r of returns) {
    let toIdx = -1;
    if (r.target === '') {
      // 回到最近锚点；无锚点则回到时间线起点（索引 0）
      let best = -1;
      for (const a of anchorOrder) { if (a.idx < r.fromIdx && a.idx > best) best = a.idx; }
      toIdx = best >= 0 ? best : (n > 0 ? 0 : -1);
    } else if (anchorPos[r.target] !== undefined) {
      toIdx = anchorPos[r.target];
    }
    if (toIdx < 0 || toIdx >= n) { continue; }
    const from = evs[r.fromIdx];
    const to = evs[toIdx];
    if (!from || !to) { continue; }
    const lift = 70;
    const cx = (from.x + to.x) / 2;
    const cy = tlSvg.axisY + lift;
    rets.push({ d: `M ${from.x} ${tlSvg.axisY + 8} Q ${cx} ${cy} ${to.x} ${tlSvg.axisY + 8}`, target: r.target });
  }
  tlReturns.value = rets;
  return evs;
});

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
.vis-hint code { background: color-mix(in srgb, var(--aw-node, #6366f1) 16%, transparent); padding: 1px 5px; border-radius: 4px; color: var(--text-color); }
.svg-wrap { width: 100%; overflow-x: auto; }
.vis-svg { width: 100%; min-width: 600px; background: var(--bg-card, #fff); border: 1px solid var(--border-color, #eee); border-radius: 10px; }
.axis { stroke: var(--aw-axis, #cbd5e1); stroke-width: 2; }
.ev-dot { fill: var(--aw-node, #6366f1); }
.ev-dot.ev-inloop { fill: var(--aw-loop, #ef4444); }
.ev-time { font-size: 11px; fill: var(--text-color, #333); text-anchor: middle; }
.ev-desc { font-size: 10px; fill: var(--text-light, #888); text-anchor: middle; }
.loop-arc { stroke-width: 2; stroke-dasharray: 5 4; }
.loop-0 { stroke: var(--aw-loop, #ef4444); }
.loop-1 { stroke: var(--aw-loop-2, #f59e0b); }
.loop-2 { stroke: var(--aw-loop-3, #10b981); }
.arrow-head { fill: var(--aw-loop, #ef4444); }
.ret-arc { stroke-width: 2.5; stroke: var(--aw-return, #06b6d4); stroke-dasharray: 2 5; }
.arrow-head-ret { fill: var(--aw-return, #06b6d4); }
.ev-dot.ev-return { fill: var(--aw-return, #06b6d4); }
.ev-return-label { font-size: 11px; fill: var(--aw-return, #06b6d4); text-anchor: middle; font-weight: 600; }
.cg-node { fill: var(--aw-node, #6366f1); opacity: 0.92; }
.cg-nlabel { font-size: 12px; fill: #fff; text-anchor: middle; }
.cg-edge { stroke: var(--text-light, #aaa); stroke-width: 1.5; }
.cg-elabel { font-size: 10px; fill: var(--text-light, #888); text-anchor: middle; }

/* 移动端：缩小最小宽度便于单手滑动查看，保留横向滚动 */
@media (max-width: 768px) {
  .svg-wrap { padding-bottom: 6px; -webkit-overflow-scrolling: touch; }
  .vis-svg { min-width: 480px; }
}
</style>
