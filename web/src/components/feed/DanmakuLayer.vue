<template>
  <div ref="host" class="dm-layer" :style="{ opacity }">
    <!-- 弹幕节点由脚本动态插入（不走 v-for：一条视频可能上千条，虚拟化管理太重） -->
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue';
import type { FeedDanmaku } from '@/api/feed';

/**
 * 弹幕渲染层
 *
 * 设计要点：
 * - 由播放器用 rAF 驱动 `sync(tMs, playing)`，不依赖 timeupdate（4 次/秒太粗，会成串冒出）；
 * - 节点用 textContent 写入，天然免疫 XSS，不做 v-html；
 * - 滚动弹幕按轨道分配，用「该轨道上一条尾巴离开右边缘」的时间判断能否复用，避免重叠；
 * - 暂停时把 CSS 动画 play-state 置 paused，画面与弹幕严格同步。
 */
const props = withDefaults(
  defineProps<{
    items?: FeedDanmaku[];
    fontSize?: number;
    opacity?: number;
    area?: number; // 弹幕占用画面高度比例
    duration?: number; // 滚动一条横穿屏幕的时长（毫秒）
  }>(),
  { items: () => [], fontSize: 20, opacity: 0.92, area: 0.7, duration: 8000 },
);

const host = ref<HTMLElement | null>(null);

let sorted: FeedDanmaku[] = [];
let cursor = 0; // 下一条待发射的弹幕下标
let lastMs = -1; // 上一帧的视频时间
let isPlaying = true; // 当前播放状态（push 直接发射时也要遵循，否则暂停时新弹幕会自己飘）
const live = new Set<HTMLElement>();
let scrollFree: number[] = []; // 每条轨道可再次发射的时间（视频时间轴）
let topFree: number[] = [];
let bottomFree: number[] = [];
const STICKY_MS = 3000; // 顶部/底部固定弹幕停留时长
const GAP_PX = 56; // 同轨两条弹幕的最小水平间距

function sortItems(list: FeedDanmaku[]) {
  return [...list].sort((a, b) => a.time_ms - b.time_ms || a.id - b.id);
}

watch(
  () => props.items,
  (v, old) => {
    const list = v || [];
    // 追加一条（自己刚发的）：只更新索引，绝不重建——重建会把在屏弹幕全清掉，
    // 而且 reset 会把游标归零，暂停状态下没有 sync 驱动，新弹幕就永远不上屏。
    if (old && list.length === old.length + 1) {
      sorted = sortItems(list);
      return;
    }
    reset(list);
  },
  { immediate: true },
);

function lineHeight() {
  return Math.round(props.fontSize * 1.6);
}

function trackCount() {
  const el = host.value;
  if (!el) return 4;
  const usable = el.clientHeight * Math.min(Math.max(props.area, 0.2), 1);
  return Math.max(2, Math.floor(usable / lineHeight()));
}

function allocTrack(free: number[], now: number) {
  const n = free.length || trackCount();
  for (let i = 0; i < n; i++) {
    if ((free[i] ?? 0) <= now) return i;
  }
  return -1;
}

function ensureTracks() {
  const n = trackCount();
  if (scrollFree.length !== n) {
    scrollFree = new Array(n).fill(0);
    topFree = new Array(Math.min(3, n)).fill(0);
    bottomFree = new Array(Math.min(3, n)).fill(0);
  }
}

function clearNodes() {
  live.forEach((el) => el.remove());
  live.clear();
}

/** 从头重建：换视频 / 数据更新时调用 */
function reset(list: FeedDanmaku[]) {
  sorted = sortItems(list);
  cursor = 0;
  lastMs = -1;
  clearNodes();
  ensureTracks();
  scrollFree.fill(0);
  topFree.fill(0);
  bottomFree.fill(0);
}

/** 播放器每帧调用：发射到期弹幕并同步暂停状态 */
function sync(tMs: number, playing: boolean) {
  const el = host.value;
  if (!el) return;
  ensureTracks();

  // 拖回前面（回退超过 0.8 秒）：清屏并重算游标，否则会漏发/错发
  if (lastMs >= 0 && tMs < lastMs - 800) {
    clearNodes();
    scrollFree.fill(0);
    topFree.fill(0);
    bottomFree.fill(0);
    cursor = lowerBound(tMs);
  }

  while (cursor < sorted.length && sorted[cursor].time_ms <= tMs) {
    spawn(sorted[cursor], tMs);
    cursor++;
  }
  lastMs = tMs;
  isPlaying = playing;

  const state = playing ? 'running' : 'paused';
  live.forEach((n) => {
    if (n.style.animationPlayState !== state) n.style.animationPlayState = state;
  });
}

function lowerBound(tMs: number) {
  let lo = 0;
  let hi = sorted.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (sorted[mid].time_ms < tMs) lo = mid + 1;
    else hi = mid;
  }
  return lo;
}

function spawn(d: FeedDanmaku, now: number) {
  const el = host.value;
  if (!el) return;
  const text = (d.content || '').trim();
  if (!text) return;

  const node = document.createElement('div');
  node.className = 'dm-item';
  node.textContent = text;
  node.style.color = d.color || '#ffffff';
  node.style.fontSize = props.fontSize + 'px';
  node.style.animationPlayState = isPlaying ? 'running' : 'paused';

  if (d.mode === 'top' || d.mode === 'bottom') {
    const pool = d.mode === 'top' ? topFree : bottomFree;
    const track = allocTrack(pool, now);
    if (track < 0) return; // 顶部轨道占满就丢弃，不排队（排队会越积越多）
    pool[track] = now + STICKY_MS;
    node.style.top = d.mode === 'top' ? track * lineHeight() + 'px' : '';
    if (d.mode === 'bottom') {
      node.style.bottom = track * lineHeight() + 8 + 'px';
    }
    node.style.animationDuration = STICKY_MS + 'ms';
    // 暂停态发射：负延迟把动画拨到中间状态，否则 opacity:0 看不见
    if (!isPlaying) node.style.animationDelay = `-${Math.round(STICKY_MS * 0.45)}ms`;
    el.appendChild(node);
    // 先落位再挂动画类：动画一旦开始就按最终样式跑，避免出现一帧错位
    node.classList.add('dm-sticky');
    live.add(node);
    node.addEventListener('animationend', () => {
      node.remove();
      live.delete(node);
    });
    return;
  }

  const track = allocTrack(scrollFree, now);
  if (track < 0) return;
  node.style.top = track * lineHeight() + 4 + 'px';
  node.style.animationDuration = props.duration + 'ms';
  // 暂停态发射：负延迟把动画拨到画面中段，否则停在右边缘外看不见
  if (!isPlaying) node.style.animationDelay = `-${Math.round(props.duration * 0.45)}ms`;
  el.appendChild(node);

  // 按实测宽度算出「尾巴让出 GAP_PX」需要的时间，决定同轨下一次可发射的时刻
  const w = node.offsetWidth || props.fontSize * text.length;
  const W = el.clientWidth || 800;
  node.style.setProperty('--dm-from', W + 'px');
  node.classList.add('dm-scroll'); // 测完宽高、写完 --dm-from 才启动动画
  const gapMs = Math.max(200, (props.duration * (w + GAP_PX)) / (W + w));
  scrollFree[track] = now + gapMs;

  live.add(node);
  node.addEventListener('animationend', () => {
    node.remove();
    live.delete(node);
  });
}

/** 手动插一条（自己发的弹幕立刻可见，不等下一轮拉取） */
function push(d: FeedDanmaku) {
  if (!sorted.some((x) => x.id === d.id)) sorted = sortItems([...sorted, d]);
  const now = lastMs < 0 ? d.time_ms : lastMs;
  // 时间点已到：立即发射，并把游标推过这条，避免 sync 之后重复发一次
  if (d.time_ms <= now + 400) {
    const idx = sorted.findIndex((x) => x.id === d.id);
    if (idx >= 0 && idx >= cursor) cursor = idx + 1;
    spawn(d, Math.max(now, d.time_ms));
    return;
  }
  // 还没到点：交给 sync 的游标自然发射
  cursor = Math.min(cursor, lowerBound(d.time_ms));
}

onMounted(() => {
  ensureTracks();
  if (typeof ResizeObserver !== 'undefined' && host.value) {
    const ro = new ResizeObserver(() => {
      scrollFree.fill(0);
      topFree.fill(0);
      bottomFree.fill(0);
      ensureTracks();
    });
    ro.observe(host.value);
  }
});

onBeforeUnmount(clearNodes);

defineExpose({ sync, reset, push });
</script>

<style scoped>
.dm-layer {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 3;
  contain: strict;
}
:deep(.dm-item) {
  position: absolute;
  left: 0;
  white-space: nowrap;
  font-weight: 600;
  line-height: 1.4;
  text-shadow:
    0 0 2px rgba(0, 0, 0, 0.9),
    0 1px 3px rgba(0, 0, 0, 0.75);
  will-change: transform;
  animation-timing-function: linear;
  animation-fill-mode: forwards;
}
/* 滚动：从右边缘外进入，到左边缘外消失 */
:deep(.dm-scroll) {
  animation-name: dm-scroll;
  transform: translateX(100%);
}
@keyframes dm-scroll {
  from {
    transform: translateX(var(--dm-from, 100vw));
  }
  to {
    transform: translateX(-100%);
  }
}
:deep(.dm-sticky) {
  left: 50%;
  animation-name: dm-fade;
  animation-timing-function: ease-out;
}
@keyframes dm-fade {
  0% {
    opacity: 0;
    transform: translateX(-50%) translateY(4px);
  }
  8% {
    opacity: 1;
    transform: translateX(-50%) translateY(0);
  }
  88% {
    opacity: 1;
  }
  100% {
    opacity: 0;
  }
}
</style>
