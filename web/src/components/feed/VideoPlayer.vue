<template>
  <div
    ref="wrap"
    class="nvp"
    :class="[mode, { 'is-full': isFull, 'ctrl-hidden': hideCtrl && playing }]"
    @pointermove="wake"
    @pointerdown="wake"
  >
    <video
      ref="video"
      class="nvp-video"
      :class="{ 'is-wide': isWide }"
      :src="src"
      :poster="poster"
      :loop="props.loop"
      :autoplay="autoplay"
      playsinline
      :preload="preload"
      @click="toggle"
      @dblclick="toggleFull"
      @loadedmetadata="onMeta"
      @timeupdate="onTimeUpdate"
      @progress="onProgress"
      @play="onPlay"
      @pause="onPause"
      @waiting="buffering = true"
      @playing="buffering = false"
      @canplay="buffering = false"
      @ended="onEnded"
      @volumechange="onVolumeChange"
      @webkitendfullscreen="onWkFsEnd"
    />

    <DanmakuLayer
      v-if="dmOn"
      ref="dmLayer"
      :items="danmaku"
      :font-size="dmFont"
      :opacity="dmOpacity"
      :area="mode === 'immersive' ? 0.55 : 0.7"
    />

    <!-- 中间播放键：暂停时显示，点击恢复 -->
    <button v-if="!playing && !buffering" class="nvp-big" type="button" @click="toggle">▶</button>
    <div v-if="buffering" class="nvp-buffering"><span /></div>

    <!-- 弹幕计数 + 开关（右上，B 站式常驻） -->
    <div class="nvp-dmbar">
      <button type="button" class="nvp-dmbtn" :class="{ off: !dmOn }" @click="toggleDanmaku">
        {{ dmOn ? '弹' : '弹' }}
        <small>{{ dmOn ? '开' : '关' }}</small>
      </button>
      <span class="nvp-dmcount">{{ dmTotal }} 条弹幕</span>
    </div>

    <!-- 发送弹幕 -->
    <div v-if="canSend" class="nvp-send" :class="{ open: sendOpen }">
      <input
        v-model="draft"
        :maxlength="80"
        placeholder="发条弹幕，和大家一起看…"
        @focus="wake(true)"
        @keyup.enter="sendDanmaku"
      />
      <button type="button" :disabled="sending || !draft.trim()" @click="sendDanmaku">
        {{ sending ? '发送中' : '发送' }}
      </button>
      <button type="button" class="nvp-send-tg" @click="sendOpen = !sendOpen">弹</button>
    </div>

    <!-- 控制条：不使用浏览器原生 controls -->
    <div class="nvp-ctrl" :class="{ hidden: hideCtrl && playing }">
      <div class="nvp-progress" ref="bar" @pointerdown="onSeekDown">
        <div class="nvp-buf" :style="{ width: bufPct + '%' }" />
        <div class="nvp-played" :style="{ width: pct + '%' }" />
        <div class="nvp-knob" :style="{ left: pct + '%' }" />
      </div>
      <div class="nvp-btns">
        <button type="button" :title="playing ? '暂停(空格)' : '播放(空格)'" @click="toggle">
          {{ playing ? '❚❚' : '▶' }}
        </button>
        <span class="nvp-time">{{ fmt(cur) }} / {{ fmt(dur) }}</span>
        <div class="nvp-vol">
          <button type="button" :title="muted ? '取消静音(m)' : '静音(m)'" @click="toggleMute">
            {{ muted || volume === 0 ? '🔇' : volume < 0.5 ? '🔉' : '🔊' }}
          </button>
          <input
            v-model.number="volume"
            class="nvp-vrange"
            type="range"
            min="0"
            max="1"
            step="0.05"
            @input="applyVolume"
          />
        </div>
        <span class="nvp-grow" />
        <select class="nvp-rate" :value="rate" @change="applyRate">
          <option v-for="r in RATES" :key="r" :value="r">{{ r === 1 ? '倍速' : r + 'x' }}</option>
        </select>
        <button type="button" :title="dmOn ? '关闭弹幕' : '开启弹幕'" @click="toggleDanmaku">弹</button>
        <button type="button" title="全屏(f)" @click="toggleFull">{{ isFull ? '⤢' : '⛶' }}</button>
        <!-- 横屏片专用：竖屏流里横屏片只能上下留黑边，点它把屏幕转过来铺满 -->
        <button
          v-if="canRotate"
          type="button"
          class="nvp-rot"
          :class="{ on: rotating }"
          :title="rotating ? '退出旋转(r)' : '旋转全屏(r)'"
          @click="toggleRotate"
        >
          ⟳
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue';
import { ElMessage } from 'element-plus';
import { feedApi, type FeedDanmaku } from '@/api/feed';
import DanmakuLayer from './DanmakuLayer.vue';

/**
 * 自绘视频播放器（替代浏览器原生 controls）
 *
 * - 控制条：播放/暂停、进度（可拖、带缓冲）、时间、音量、倍速、弹幕开关、全屏；
 * - 键盘：空格播放、←/→ 快退快进、↑/↓ 音量、m 静音、f 全屏；
 * - 弹幕：rAF 驱动弹幕层，和播放进度严格对齐；发送后立即上屏；
 * - 两种形态：detail（详情页，控制条常驻）与 immersive（竖滑，极简、自动播停）。
 */
const props = withDefaults(
  defineProps<{
    src?: string;
    poster?: string;
    postId?: number;
    mode?: 'detail' | 'immersive';
    loop?: boolean;
    autoplay?: boolean;
    muted?: boolean;
    /** immersive 模式下由父级控制当前是否该播放 */
    active?: boolean;
    canSend?: boolean;
    durationHint?: number;
    /**
     * 预加载策略。竖滑页里当前条与「下一条」给 auto（提前缓冲），
     * 其余给 metadata 甚至 none，避免十几条视频同时抢带宽与解码器。
     */
    preload?: 'none' | 'metadata' | 'auto';
  }>(),
  {
    src: '',
    poster: '',
    postId: 0,
    mode: 'detail',
    loop: false,
    autoplay: false,
    muted: false,
    active: true,
    canSend: true,
    durationHint: 0,
    preload: 'metadata',
  },
);

const emit = defineEmits<{ (e: 'meta', duration: number): void; (e: 'playing', v: boolean): void }>();

const RATES = [0.75, 1, 1.25, 1.5, 2];

const wrap = ref<HTMLElement | null>(null);
const video = ref<HTMLVideoElement | null>(null);
const bar = ref<HTMLElement | null>(null);
const dmLayer = ref<InstanceType<typeof DanmakuLayer> | null>(null);

const playing = ref(false);
const buffering = ref(false);
/** 片源是否为横向（宽>高）。竖屏流里横屏片要完整显示、上下留黑边，不能裁切。 */
const isWide = ref(false);
const cur = ref(0);
const dur = ref(props.durationHint || 0);
const bufEnd = ref(0);
const volume = ref(1);
const muted = ref(props.muted);
const rate = ref(1);
const isFull = ref(false);
/** 是否处于「旋转全屏」（全屏 + 锁横屏）状态 */
const rotating = ref(false);
const hideCtrl = ref(false);
const dragging = ref(false);

const dmOn = ref(true);
const dmFont = ref(20);
const dmOpacity = ref(0.92);
const dmTotal = ref(0);
const danmaku = ref<FeedDanmaku[]>([]);

const draft = ref('');
const sending = ref(false);
const sendOpen = ref(false);

const pct = computed(() => (dur.value > 0 ? Math.min(100, (cur.value / dur.value) * 100) : 0));
const bufPct = computed(() => (dur.value > 0 ? Math.min(100, (bufEnd.value / dur.value) * 100) : 0));
/** 横屏片才给「旋转全屏」：竖屏片本来就铺满，转了反而上下留黑边 */
const canRotate = computed(() => isWide.value);

let rafId = 0;
let hideTimer: number | null = null;

// ---------- 播放控制 ----------
function toggle() {
  const v = video.value;
  if (!v) return;
  if (v.paused) play();
  else pause();
}

function play() {
  video.value?.play().catch(() => {
    // 自动播放被拦截：退回静音播放（移动端浏览器要求）
    const v = video.value;
    if (v && !v.muted) {
      v.muted = true;
      muted.value = true;
      v.play().catch(() => {});
    }
  });
}

function pause() {
  video.value?.pause();
}

function toggleMute() {
  const v = video.value;
  if (!v) return;
  v.muted = !v.muted;
  muted.value = v.muted;
  if (!v.muted && volume.value === 0) {
    volume.value = 1;
    v.volume = 1;
  }
}

function applyVolume() {
  const v = video.value;
  if (!v) return;
  v.volume = Math.min(1, Math.max(0, volume.value));
  v.muted = v.volume === 0 ? true : muted.value;
}

function applyRate(e: Event) {
  const val = Number((e.target as HTMLSelectElement).value);
  rate.value = val;
  if (video.value) video.value.playbackRate = val;
}

function onVolumeChange() {
  const v = video.value;
  if (!v) return;
  muted.value = v.muted;
  volume.value = v.volume;
}

function onMeta() {
  const v = video.value;
  if (!v) return;
  const d = Number.isFinite(v.duration) ? v.duration : 0;
  dur.value = d;
  // 横屏判定：竖滑流里竖屏片铺满（cover），横屏片完整显示 + 上下黑边（contain）
  isWide.value = v.videoWidth > 0 && v.videoHeight > 0 && v.videoWidth > v.videoHeight;
  emit('meta', d);
  v.volume = volume.value;
  v.muted = muted.value;
  v.playbackRate = rate.value;
  // immersive 模式由父级 active 决定是否播
  if (props.mode === 'immersive' && props.active) play();
  else if (props.autoplay) play();
}

function onTimeUpdate() {
  if (!video.value) return;
  cur.value = video.value.currentTime;
  if (!dragging.value && dmLayer.value) {
    dmLayer.value.sync(Math.round(cur.value * 1000), !video.value.paused);
  }
}

function onProgress() {
  const v = video.value;
  if (!v || !v.buffered.length) return;
  try {
    bufEnd.value = v.buffered.end(v.buffered.length - 1);
  } catch {
    /* buffered 边界可能瞬时非法 */
  }
}

function onPlay() {
  playing.value = true;
  emit('playing', true);
  startLoop();
  scheduleHide();
}

function onPause() {
  playing.value = false;
  emit('playing', false);
  stopLoop();
  hideCtrl.value = false;
}

function onEnded() {
  if (props.loop && video.value) play();
}

// ---------- 进度条拖动 ----------
function ratioOf(clientX: number) {
  const el = bar.value;
  if (!el) return 0;
  const rect = el.getBoundingClientRect();
  return Math.min(1, Math.max(0, (clientX - rect.left) / Math.max(1, rect.width)));
}

function seekTo(ratio: number, commit = false) {
  const v = video.value;
  if (!v || !dur.value) return;
  const t = ratio * dur.value;
  cur.value = t;
  if (commit) v.currentTime = t;
}

function onSeekDown(e: PointerEvent) {
  if (!dur.value) return;
  dragging.value = true;
  seekTo(ratioOf(e.clientX));
  window.addEventListener('pointermove', onSeekMove);
  window.addEventListener('pointerup', onSeekUp);
}

function onSeekMove(e: PointerEvent) {
  if (!dragging.value) return;
  seekTo(ratioOf(e.clientX));
}

function onSeekUp(e: PointerEvent) {
  if (!dragging.value) return;
  dragging.value = false;
  seekTo(ratioOf(e.clientX), true);
  window.removeEventListener('pointermove', onSeekMove);
  window.removeEventListener('pointerup', onSeekUp);
  // 拖动后弹幕游标要按新位置重排
  dmLayer.value?.sync(Math.round(cur.value * 1000), !video.value?.paused);
}

// ---------- 弹幕 ----------
async function loadDanmaku() {
  if (!props.postId) return;
  try {
    const r = await feedApi.danmaku(props.postId, { limit: 3000 });
    danmaku.value = r.data?.data?.list || [];
    dmTotal.value = r.data?.data?.total ?? danmaku.value.length;
  } catch {
    /* 弹幕加载失败不影响播放 */
  }
}

function toggleDanmaku() {
  dmOn.value = !dmOn.value;
  if (dmOn.value) {
    nextTick(() => {
      dmLayer.value?.reset(danmaku.value);
      dmLayer.value?.sync(Math.round(cur.value * 1000), playing.value);
    });
  }
}

async function sendDanmaku() {
  const text = draft.value.trim();
  if (!text) return;
  if (!props.postId) return;
  sending.value = true;
  try {
    const ms = Math.round(cur.value * 1000);
    const r = await feedApi.sendDanmaku(props.postId, { content: text, time_ms: ms, mode: 'scroll' });
    const id = r.data?.data?.id;
    if (id) {
      const item: FeedDanmaku = {
        id,
        post_id: props.postId,
        user_id: 0,
        content: text,
        time_ms: ms,
        mode: 'scroll',
        color: '',
        created_at: new Date().toISOString(),
      };
      danmaku.value = [...danmaku.value, item];
      dmTotal.value += 1;
      dmOn.value = true;
      await nextTick();
      dmLayer.value?.push(item);
    }
    draft.value = '';
    ElMessage.success('弹幕已发送');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发送失败，请先登录');
  } finally {
    sending.value = false;
  }
}

// ---------- 全屏 / 控制条自动隐藏 ----------
function toggleFull() {
  const el = wrap.value;
  if (!el) return;
  if (document.fullscreenElement) {
    document.exitFullscreen?.();
  } else {
    el.requestFullscreen?.().catch(() => {});
  }
}

function onFsChange() {
  isFull.value = document.fullscreenElement === wrap.value;
  // 用户按 Esc / 手势退出全屏：方向锁要一并释放，否则页面会一直卡在横屏
  if (!document.fullscreenElement && rotating.value) releaseOrientation();
}

// ---------- 旋转全屏（横屏片专用） ----------
function releaseOrientation() {
  try {
    (screen as any)?.orientation?.unlock?.();
  } catch {
    /* 部分浏览器没有 unlock 或不在锁定态 */
  }
  rotating.value = false;
}

function onWkFsEnd() {
  // iOS：video.webkitEnterFullscreen 退出时不会触发 fullscreenchange
  releaseOrientation();
}

/**
 * 旋转全屏：把竖着拿的手机变成看横屏片的状态。
 *
 * 三条降级路径，按可靠性排序：
 * 1) 元素全屏 + Screen Orientation Lock 锁 landscape —— Android Chrome / Edge / 桌面主路径；
 * 2) iOS Safari 没有 Screen Orientation API、也不能对 div 全屏 —— 退回
 *    `video.webkitEnterFullscreen()`，系统原生全屏会自动把横屏片转过来；
 * 3) 都失败时至少进全屏（横屏片 contain 铺满宽度），并提示不支持。
 */
async function enterRotate() {
  const el = wrap.value;
  const v = video.value;
  if (!el || !v) return;

  const so = (screen as any)?.orientation;
  const canElementFs = typeof el.requestFullscreen === 'function' && document.fullscreenEnabled;

  // iOS 分支必须**同步**调用：webkitEnterFullscreen 依赖用户手势，
  // 一旦经过 await 就丢 user activation，会被静默拒绝。
  if (!canElementFs && typeof (v as any).webkitEnterFullscreen === 'function') {
    (v as any).webkitEnterFullscreen();
    rotating.value = true;
    return;
  }

  let ok = false;
  try {
    if (!document.fullscreenElement && canElementFs) {
      await el.requestFullscreen();
    }
    ok = !!document.fullscreenElement;
  } catch {
    /* 全屏被拒（iframe 缺 allowfullscreen / 手势缺失），继续尝试锁方向 */
  }

  if (so?.lock) {
    try {
      // 必须先进全屏才能锁方向；桌面浏览器通常直接 reject，忽略即可
      await so.lock('landscape');
      ok = true;
    } catch {
      /* ignore */
    }
  }

  rotating.value = ok;
  if (!ok) ElMessage.warning('当前浏览器不支持旋转全屏，可手动横置手机');
}

async function exitRotate() {
  releaseOrientation();
  if (document.fullscreenElement) {
    try {
      await document.exitFullscreen?.();
    } catch {
      /* ignore */
    }
  }
}

function toggleRotate() {
  if (rotating.value) exitRotate();
  else enterRotate();
}

function wake(force = false) {
  if (force === true) {
    hideCtrl.value = false;
    return;
  }
  hideCtrl.value = false;
  scheduleHide();
}

function scheduleHide() {
  if (hideTimer) window.clearTimeout(hideTimer);
  if (!playing.value) return;
  // 横屏片控制条常驻：竖屏流里横屏片上下本来就是黑边，控件不挡画面，
  // 且「旋转全屏」按钮必须一直看得见，否则用户根本不知道该转屏。
  if (canRotate.value) return;
  hideTimer = window.setTimeout(() => {
    hideCtrl.value = true;
  }, 2800);
}

// ---------- rAF 驱动弹幕 ----------
// ⚠️ 函数名不能叫 loop：它会遮蔽同名 prop，「:loop="loop"」会绑到这个函数上，
// 于是每个视频都被强制循环播放（DOM 上出现 loop 属性）。
function rafTick() {
  const v = video.value;
  if (v && dmLayer.value && !v.paused && !dragging.value) {
    dmLayer.value.sync(Math.round(v.currentTime * 1000), true);
  }
  rafId = requestAnimationFrame(rafTick);
}

function startLoop() {
  if (!rafId) rafId = requestAnimationFrame(rafTick);
}

function stopLoop() {
  if (rafId) cancelAnimationFrame(rafId);
  rafId = 0;
}

// ---------- 键盘 ----------
function onKey(e: KeyboardEvent) {
  const t = e.target as HTMLElement | null;
  if (t && /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName)) return;
  const v = video.value;
  if (!v) return;
  switch (e.key) {
    case ' ':
      e.preventDefault();
      toggle();
      break;
    case 'ArrowLeft':
      v.currentTime = Math.max(0, v.currentTime - 5);
      break;
    case 'ArrowRight':
      v.currentTime = Math.min(dur.value || v.duration, v.currentTime + 5);
      break;
    case 'ArrowUp':
      e.preventDefault();
      volume.value = Math.min(1, volume.value + 0.1);
      applyVolume();
      break;
    case 'ArrowDown':
      e.preventDefault();
      volume.value = Math.max(0, volume.value - 0.1);
      applyVolume();
      break;
    case 'm':
      toggleMute();
      break;
    case 'f':
      toggleFull();
      break;
    case 'r':
      if (canRotate.value) toggleRotate();
      break;
  }
}

// ---------- 生命周期 ----------
let keyBound = false;

// 竖滑页里同时存在多个播放器实例，键盘监听只该由「当前这一条」持有，
// 否则按一次空格会触发十几个 handler，白白卡一帧。
function syncKeyBinding() {
  const want = props.mode === 'detail' || props.active;
  if (want && !keyBound) {
    window.addEventListener('keydown', onKey);
    keyBound = true;
  } else if (!want && keyBound) {
    window.removeEventListener('keydown', onKey);
    keyBound = false;
  }
}

onMounted(() => {
  document.addEventListener('fullscreenchange', onFsChange);
  syncKeyBinding();
  loadDanmaku();
});

onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', onFsChange);
  if (keyBound) window.removeEventListener('keydown', onKey);
  keyBound = false;
  window.removeEventListener('pointermove', onSeekMove);
  window.removeEventListener('pointerup', onSeekUp);
  if (hideTimer) window.clearTimeout(hideTimer);
  stopLoop();
  // 离开页面（竖滑滑走 / 切路由）时别把屏幕方向锁带走
  releaseOrientation();
  // 只 pause 不够：src 还在，移动端解码器不会被回收（竖滑翻几页就把解码器占满）。
  const v = video.value;
  if (v) {
    v.pause();
    v.removeAttribute('src');
    v.load();
  }
});

// immersive：父级切换当前条目时自动播停
watch(
  () => props.active,
  async (on) => {
    if (props.mode !== 'immersive') return;
    syncKeyBinding();
    if (on) {
      play();
    } else {
      pause();
      if (video.value) video.value.currentTime = 0;
    }
  },
);

watch(
  () => props.muted,
  (m) => {
    if (video.value) video.value.muted = m;
    muted.value = m;
  },
);

// 片源信息晚到（metadata 才拿到宽高）：一变成横屏片就把控制条唤回来
watch(canRotate, (w) => {
  if (w) {
    hideCtrl.value = false;
    if (hideTimer) window.clearTimeout(hideTimer);
  }
});

watch(
  () => props.preload,
  (p) => {
    // 竖滑窗口滑动后，原本的「下一条」会升格为当前条：给它提权到 auto 开始缓冲
    const v = video.value;
    if (v && v.preload !== p) v.preload = p;
  },
);

watch(
  () => props.src,
  () => {
    cur.value = 0;
    dur.value = props.durationHint || 0;
    bufEnd.value = 0;
    danmaku.value = [];
    dmTotal.value = 0;
    loadDanmaku();
  },
);

defineExpose({ play, pause, toggle, setMuted: (m: boolean) => { if (video.value) video.value.muted = m; muted.value = m; } });

function fmt(s: number) {
  const n = Number.isFinite(s) ? Math.max(0, Math.floor(s)) : 0;
  const m = Math.floor(n / 60);
  const sec = n % 60;
  return `${m}:${sec < 10 ? '0' : ''}${sec}`;
}
</script>

<style scoped>
.nvp {
  position: relative;
  width: 100%;
  height: 100%;
  background: #000;
  overflow: hidden;
  user-select: none;
}
.nvp-video {
  width: 100%;
  height: 100%;
  display: block;
  /* 默认完整显示：详情页的 16:9 舞台与竖屏流里的横屏片都用 contain，
     不会把画面左右裁掉；竖屏片在沉浸模式下另有一条 cover 规则覆盖 */
  object-fit: contain;
  background: #000;
}
/* 竖滑沉浸：竖屏片铺满整屏（短视频平台的做法），
   横屏片（is-wide）继续用 contain —— 画面完整居中，上下是黑边 */
.nvp.immersive .nvp-video:not(.is-wide) {
  object-fit: cover;
}
.nvp-big {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: 64px;
  height: 64px;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, 0.35);
  background: rgba(0, 0, 0, 0.5);
  color: #fff;
  font-size: 24px;
  cursor: pointer;
  z-index: 5;
  display: flex;
  align-items: center;
  justify-content: center;
  padding-left: 3px;
}
.nvp-buffering {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  z-index: 5;
}
.nvp-buffering span {
  display: block;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  border: 3px solid rgba(255, 255, 255, 0.25);
  border-top-color: #fff;
  animation: nvp-spin 0.8s linear infinite;
}
@keyframes nvp-spin {
  to {
    transform: rotate(360deg);
  }
}

/* 弹幕开关 */
.nvp-dmbar {
  position: absolute;
  right: 10px;
  top: 10px;
  z-index: 6;
  display: flex;
  align-items: center;
  gap: 8px;
  color: #fff;
  font-size: 0.72rem;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.8);
}
.nvp-dmbtn {
  display: flex;
  align-items: center;
  gap: 2px;
  border: 1px solid rgba(255, 255, 255, 0.35);
  background: rgba(0, 0, 0, 0.35);
  color: #fff;
  border-radius: 12px;
  padding: 2px 8px;
  cursor: pointer;
  font-size: 0.72rem;
}
.nvp-dmbtn.off {
  opacity: 0.55;
}
.nvp-dmcount {
  opacity: 0.85;
}

/* 弹幕发送 */
.nvp-send {
  position: absolute;
  left: 12px;
  right: 12px;
  bottom: 62px;
  z-index: 6;
  display: flex;
  gap: 6px;
  align-items: center;
}
.nvp-send input {
  flex: 1;
  min-width: 0;
  height: 32px;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.28);
  background: rgba(0, 0, 0, 0.45);
  color: #fff;
  padding: 0 12px;
  font-size: 0.82rem;
  outline: none;
}
.nvp-send input::placeholder {
  color: rgba(255, 255, 255, 0.6);
}
.nvp-send button {
  height: 32px;
  border-radius: 16px;
  border: none;
  padding: 0 12px;
  background: var(--el-color-primary, #409eff);
  color: #fff;
  font-size: 0.78rem;
  cursor: pointer;
}
.nvp-send button:disabled {
  opacity: 0.6;
  cursor: default;
}
.nvp-send-tg {
  background: rgba(255, 255, 255, 0.18) !important;
}

/* 控制条 */
.nvp-ctrl {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 7;
  padding: 18px 12px 8px;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.72), rgba(0, 0, 0, 0));
  transition: opacity 0.22s ease;
}
.nvp-ctrl.hidden {
  opacity: 0;
  pointer-events: none;
}
.nvp-progress {
  position: relative;
  height: 4px;
  border-radius: 2px;
  background: rgba(255, 255, 255, 0.24);
  cursor: pointer;
  margin-bottom: 6px;
}
.nvp-progress::before {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  top: -6px;
  bottom: -6px;
}
.nvp-buf {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  background: rgba(255, 255, 255, 0.36);
  border-radius: 2px;
}
.nvp-played {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  background: var(--el-color-primary, #409eff);
  border-radius: 2px;
}
.nvp-knob {
  position: absolute;
  top: 50%;
  width: 11px;
  height: 11px;
  border-radius: 50%;
  background: #fff;
  transform: translate(-50%, -50%);
  box-shadow: 0 0 4px rgba(0, 0, 0, 0.5);
}
.nvp-btns {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #fff;
  font-size: 0.78rem;
}
.nvp-btns > button {
  border: none;
  background: transparent;
  color: #fff;
  cursor: pointer;
  font-size: 0.9rem;
  line-height: 1;
  padding: 2px 2px;
}
.nvp-time {
  font-variant-numeric: tabular-nums;
  opacity: 0.9;
}
.nvp-vol {
  display: flex;
  align-items: center;
  gap: 4px;
}
.nvp-vol > button {
  border: none;
  background: transparent;
  color: #fff;
  cursor: pointer;
  font-size: 0.9rem;
}
.nvp-vrange {
  width: 62px;
  accent-color: var(--el-color-primary, #409eff);
}
.nvp-grow {
  flex: 1;
}
.nvp-rate {
  background: rgba(0, 0, 0, 0.4);
  color: #fff;
  border: 1px solid rgba(255, 255, 255, 0.28);
  border-radius: 10px;
  font-size: 0.72rem;
  padding: 1px 4px;
  cursor: pointer;
}
/* 旋转全屏：做成一枚小圆钮，压在控制条最右端（= 画面右下角） */
.nvp-btns > button.nvp-rot {
  width: 24px;
  height: 24px;
  padding: 0;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, 0.38);
  background: rgba(255, 255, 255, 0.18);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 0.88rem;
  line-height: 1;
}
.nvp-btns > button.nvp-rot:active {
  transform: scale(0.92);
}
.nvp-btns > button.nvp-rot.on {
  background: var(--el-color-primary, #409eff);
  border-color: transparent;
}

/* 沉浸模式：只留极简进度与自动隐藏的控制条 */
.nvp.immersive .nvp-ctrl {
  padding: 10px 10px 6px;
}
.nvp.immersive .nvp-vrange,
.nvp.immersive .nvp-rate {
  display: none;
}
.nvp.immersive .nvp-dmbar {
  right: 10px;
  top: auto;
  bottom: 96px;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
}
.nvp.immersive .nvp-send {
  bottom: 46px;
}
.nvp.immersive .nvp-send input {
  display: none;
}
.nvp.immersive .nvp-send.open input {
  display: block;
}
@media (max-width: 767px) {
  .nvp-big {
    width: 54px;
    height: 54px;
    font-size: 20px;
  }
  /* 手指点击目标要够大 */
  .nvp-btns > button.nvp-rot {
    width: 28px;
    height: 28px;
    font-size: 0.95rem;
  }
}
</style>
