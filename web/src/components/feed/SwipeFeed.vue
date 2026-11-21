<template>
  <div class="swipe-feed" :class="{ mobile: isMobile }">
    <div
      ref="scroller"
      class="sw-scroller"
      @scroll.passive="onScroll"
      @touchstart.passive="onTouchStart"
      @touchend.passive="onTouchEnd"
    >
      <section v-for="(p, idx) in posts" :key="p.id" class="sw-item" :data-idx="idx">
        <!-- 只有「当前 ±窗口」的条目才挂真实播放器：
             移动端同时存在十几个 <video> 会占满硬件解码器，滚动直接掉到个位数帧率 -->
        <VideoPlayer
          v-if="inWindow(idx)"
          class="sw-player"
          :src="mediaUrl(p.video_url)"
          :poster="mediaUrl(p.cover_url)"
          :post-id="p.id"
          :muted="muted"
          :active="idx === activeIdx"
          :preload="preloadOf(idx)"
          mode="immersive"
        />
        <!-- 窗口外：封面占位，等高占位保证 scroll-snap 与滚动位置不跳 -->
        <div v-else class="sw-ph">
          <img v-if="p.cover_url" :src="mediaUrl(p.cover_url)" alt="" loading="lazy" decoding="async" />
          <span class="sw-ph-ic">▶</span>
        </div>

        <div class="sw-mask" />

        <template v-if="inWindow(idx)">
          <div class="sw-info">
            <div class="sw-author">
              <el-avatar :size="30" :src="mediaUrl(p.avatar_url)">{{ (p.author_name || '?')[0] }}</el-avatar>
              <b>@{{ p.author_name || '用户' + p.user_id }}</b>
            </div>
            <h3 class="sw-title">{{ p.title || '未命名视频' }}</h3>
            <p v-if="p.content" class="sw-desc">{{ plain(p.content) }}</p>
            <div class="sw-tags" v-if="p.tags">
              <router-link v-for="t in splitTags(p.tags)" :key="t" :to="`/tag/${encodeURIComponent(t)}`">#{{ t }}</router-link>
            </div>
          </div>

          <div class="sw-rail">
            <button class="sw-act" :class="{ on: p.liked }" @click.stop="like(p)">
              <span class="sw-ic">♥</span>
              <small>{{ formatCount(p.like_count) }}</small>
            </button>
            <button class="sw-act" @click.stop="openDetail(p)">
              <span class="sw-ic">💬</span>
              <small>{{ formatCount(p.comment_count) }}</small>
            </button>
            <button class="sw-act" @click.stop="share(p)">
              <span class="sw-ic">↗</span>
              <small>分享</small>
            </button>
            <button class="sw-act" @click.stop="toggleMute">
              <span class="sw-ic">{{ muted ? '🔇' : '🔊' }}</span>
              <small>{{ muted ? '静音' : '有声' }}</small>
            </button>
          </div>
        </template>

        <div v-if="idx === 0 && muted && inWindow(idx)" class="sw-hint">点击画面播放 · 右下角开声音</div>
      </section>

      <div v-if="loading" class="sw-loading">加载中…</div>
      <div v-else-if="!hasMore" class="sw-end">— 已经到底了 —</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { feedApi, type FeedPost } from '@/api/feed';
import { mediaUrl } from '@/utils/prefix';
import { formatCount } from '@/utils/feedFormat';
import { useAuthStore } from '@/stores/auth';
import VideoPlayer from './VideoPlayer.vue';

/**
 * 竖滑短视频（沉浸式）
 *
 * 三条规则决定流畅度：
 * 1) **播放器窗口化**：只有 `active-1 … active+2` 这几条挂真实 <video>，其余渲染封面占位。
 *    DOM 与硬件解码器数量恒定（≤4），翻多少页都不会累积。
 * 2) **预缓存下一条**：当前条与下一条 `preload=auto`，浏览器用 Range 提前缓冲首段，
 *    滑过去基本秒开；其余只取 metadata。配合服务端 /uploads 的长缓存，回看不再走网络。
 * 3) **滚动只做常量时间的事**：scroll 用 rAF 合并，按 `scrollTop/单条高度` 取整判断当前条，
 *    不读 scrollHeight、不用 IntersectionObserver（后台标签会漏回调）。
 */
const router = useRouter();
const auth = useAuthStore();

const PAGE_SIZE = 6;
const WIN_BEFORE = 1; // 往回保留 1 条：滑回去能立刻续播，不用重新缓冲
const WIN_AFTER = 2; // 往前挂 2 条：下一条已就绪，下下条开始预取

const posts = ref<FeedPost[]>([]);
const loading = ref(false);
const hasMore = ref(true);
const muted = ref(true);
const page = ref(1);
const scroller = ref<HTMLElement | null>(null);
const isMobile = ref(false);
const activeIdx = ref(0);

function inWindow(idx: number) {
  return idx >= activeIdx.value - WIN_BEFORE && idx <= activeIdx.value + WIN_AFTER;
}

function preloadOf(idx: number) {
  if (idx === activeIdx.value) return 'auto' as const;
  if (idx === activeIdx.value + 1) return 'auto' as const; // 预缓存下一条
  return 'metadata' as const;
}

function plain(s: string) {
  return (s || '').replace(/[#*`>[\]]/g, '').slice(0, 90);
}
function splitTags(s: string) {
  return (s || '')
    .split(/[,，]/)
    .map((x) => x.trim())
    .filter(Boolean)
    .slice(0, 4);
}

async function load(reset = false) {
  if (loading.value) return;
  if (reset) {
    page.value = 1;
    posts.value = [];
    hasMore.value = true;
    activeIdx.value = 0;
  } else if (!hasMore.value) {
    return;
  }
  loading.value = true;
  try {
    const r = await feedApi.list({ kind: 'video', page: page.value, page_size: PAGE_SIZE });
    const list = r.data?.data?.list || [];
    posts.value = reset ? list : [...posts.value, ...list];
    hasMore.value = list.length >= PAGE_SIZE;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
    await nextTick();
    syncActive();
  }
}

/** 只在「真的换了一条」时才翻页，避免滚动过程里反复自增页码 */
function maybeLoadMore() {
  if (loading.value || !hasMore.value) return;
  if (activeIdx.value < posts.value.length - 2) return;
  page.value += 1;
  load();
}

function syncActive() {
  const el = scroller.value;
  if (!el) return;
  const h = el.clientHeight || 1;
  const idx = Math.max(0, Math.min(posts.value.length - 1, Math.round(el.scrollTop / h)));
  if (idx === activeIdx.value) return;
  activeIdx.value = idx;
  maybeLoadMore();
}

// ---------- 滚动：rAF 合并 ----------
let rafPending = false;
function onScroll() {
  if (rafPending) return;
  rafPending = true;
  requestAnimationFrame(() => {
    rafPending = false;
    syncActive();
  });
}

// ---------- 手势兜底 ----------
// 原生 scroll-snap 在部分移动端 WebView（旧版 Android / 某些 App 内置浏览器）不生效，
// 表现为「手指划了但画面不换条」。这里只做保底：原生没滚动时手动翻一条，
// 原生滚了但没对齐时补一次对齐；正常浏览器下不介入。
let touchStartY = 0;
let touchStartTop = 0;

function goTo(idx: number, smooth = true) {
  const el = scroller.value;
  if (!el) return;
  const h = el.clientHeight || 1;
  const max = Math.max(0, el.scrollHeight - el.clientHeight);
  const top = Math.max(0, Math.min(idx * h, max));
  el.scrollTo({ top, behavior: smooth ? 'smooth' : 'auto' });
}

function onTouchStart(e: TouchEvent) {
  touchStartY = e.touches[0]?.clientY ?? 0;
  touchStartTop = scroller.value?.scrollTop ?? 0;
}

function onTouchEnd(e: TouchEvent) {
  const el = scroller.value;
  if (!el) return;
  const endY = e.changedTouches[0]?.clientY ?? touchStartY;
  const dy = touchStartY - endY; // > 0 表示向上滑（下一条）
  const h = el.clientHeight || 1;
  const moved = Math.abs(el.scrollTop - touchStartTop);

  if (moved < 8) {
    if (Math.abs(dy) < 36) return;
    goTo(Math.round(el.scrollTop / h) + (dy > 0 ? 1 : -1));
    return;
  }
  // 原生滚动了但停在不整屏的位置：对齐到最近的一条
  const off = el.scrollTop % h;
  if (off > 8 && h - off > 8) goTo(Math.round(el.scrollTop / h));
}

function toggleMute() {
  muted.value = !muted.value;
}

async function like(p: FeedPost) {
  if (!auth.isLoggedIn) return ElMessage.warning('请先登录');
  try {
    const r = await feedApi.like(p.id);
    p.liked = !!r.data?.data?.liked;
    p.like_count += p.liked ? 1 : -1;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function share(p: FeedPost) {
  try {
    await feedApi.share(p.id);
    p.share_count++;
    await navigator.clipboard?.writeText(location.origin + `/video/${p.id}`);
    ElMessage.success('链接已复制');
  } catch {
    ElMessage.success('已记录分享');
  }
}

function openDetail(p: FeedPost) {
  router.push(`/video/${p.id}`);
}

// 进页面锁住外层文档滚动：避免外层页面把竖滑手势吃掉，也避免 iOS 橡皮筋
function lockPageScroll() {
  document.documentElement.classList.add('sw-scroll-lock');
}
function unlockPageScroll() {
  document.documentElement.classList.remove('sw-scroll-lock');
}

onMounted(async () => {
  isMobile.value = window.matchMedia('(max-width: 767px)').matches;
  lockPageScroll();
  await load(true);
});

onBeforeUnmount(unlockPageScroll);
</script>

<style scoped>
.swipe-feed {
  height: 100%;
  background: #000;
}
.sw-scroller {
  height: 100%;
  overflow-y: auto;
  /* 移动端必需：iOS 惯性滚动、滚动链不外泄、只接管纵向手势 */
  -webkit-overflow-scrolling: touch;
  overscroll-behavior-y: contain;
  touch-action: pan-y;
  scroll-snap-type: y mandatory;
  scrollbar-width: none;
}
.sw-scroller::-webkit-scrollbar {
  display: none;
}
.sw-item {
  position: relative;
  height: 100%;
  scroll-snap-align: start;
  scroll-snap-stop: always;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #000;
  /* 单条自成渲染边界：内部重绘不外溢到相邻条目 */
  contain: layout paint style;
}
.sw-player {
  width: 100%;
  height: 100%;
}
/* 注意：不要在这里强制 object-fit。竖屏/横屏由 VideoPlayer 按 videoWidth/videoHeight
   自行决定（竖屏片 cover 铺满、横屏片 contain 留上下黑边），全局强制 cover 会把横屏片裁掉。 */
/* 桌面端竖滑限制成竖屏比例，居中显示，避免大屏上一条视频占满整宽 */
.swipe-feed:not(.mobile) .sw-player {
  max-width: 480px;
}

/* 窗口外占位：只画封面，不建解码器 */
.sw-ph {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #000;
}
.sw-ph img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  opacity: 0.72;
}
.swipe-feed:not(.mobile) .sw-ph img {
  max-width: 480px;
}
.sw-ph-ic {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: 54px;
  height: 54px;
  border-radius: 50%;
  border: 1px solid rgba(255, 255, 255, 0.3);
  background: rgba(0, 0, 0, 0.4);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  padding-left: 3px;
}

.sw-mask {
  position: absolute;
  inset: auto 0 0 0;
  height: 46%;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.75), rgba(0, 0, 0, 0));
  pointer-events: none;
  z-index: 2;
}
.sw-info {
  position: absolute;
  left: 16px;
  right: 76px;
  bottom: 74px;
  color: #fff;
  display: flex;
  flex-direction: column;
  gap: 8px;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.6);
  z-index: 4;
  pointer-events: none;
}
.sw-info a,
.sw-info :deep(a) {
  pointer-events: auto;
}
.swipe-feed:not(.mobile) .sw-info {
  left: calc(50% - 240px + 16px);
  right: auto;
  width: 300px;
}
.sw-author {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.88rem;
}
.sw-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  line-height: 1.4;
}
.sw-desc {
  margin: 0;
  font-size: 0.82rem;
  opacity: 0.9;
  line-height: 1.5;
}
.sw-tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 0.78rem;
  opacity: 0.85;
}
.sw-tags a {
  color: #fff;
}
.sw-rail {
  position: absolute;
  right: 14px;
  bottom: 78px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  z-index: 8;
}
.swipe-feed:not(.mobile) .sw-rail {
  right: calc(50% - 240px + 14px);
}
.sw-act {
  border: none;
  background: transparent;
  color: #fff;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  cursor: pointer;
  font-size: 0.72rem;
}
.sw-act .sw-ic {
  font-size: 1.5rem;
  line-height: 1;
  filter: drop-shadow(0 1px 3px rgba(0, 0, 0, 0.5));
}
.sw-act.on {
  color: #ff4d6d;
}
.sw-act small {
  opacity: 0.9;
}
.sw-hint {
  position: absolute;
  left: 50%;
  top: 14px;
  transform: translateX(-50%);
  color: #fff;
  font-size: 0.78rem;
  background: rgba(0, 0, 0, 0.45);
  padding: 4px 12px;
  border-radius: 12px;
  pointer-events: none;
  z-index: 4;
}
.sw-loading,
.sw-end {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #9ca3af;
  font-size: 0.82rem;
  background: #000;
}
</style>

<style>
/* 竖滑页期间锁住外层文档滚动（scoped 样式无法命中 html/body） */
html.sw-scroll-lock,
html.sw-scroll-lock body {
  overflow: hidden !important;
  height: 100%;
  overscroll-behavior: none;
}
</style>
