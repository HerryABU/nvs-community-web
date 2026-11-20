<template>
  <div class="swipe-feed" :class="{ mobile: isMobile }">
    <div ref="scroller" class="sw-scroller" @scroll.passive="onScroll">
      <section
        v-for="(p, idx) in posts"
        :key="p.id"
        class="sw-item"
        :data-idx="idx"
      >
        <video
          class="sw-video"
          :src="mediaUrl(p.video_url)"
          :poster="mediaUrl(p.cover_url)"
          loop
          playsinline
          muted
          preload="metadata"
          @click="togglePlay($event, idx)"
        />
        <div class="sw-mask" />

        <div class="sw-info">
          <div class="sw-author">
            <el-avatar :size="30" :src="mediaUrl(p.avatar_url)">{{ (p.author_name || '?')[0] }}</el-avatar>
            <b>@{{ p.author_name || '用户' + p.user_id }}</b>
          </div>
          <h3 class="sw-title">{{ p.title || '未命名视频' }}</h3>
          <p v-if="p.content" class="sw-desc">{{ plain(p.content) }}</p>
          <div class="sw-tags" v-if="p.tags">
            <span v-for="t in splitTags(p.tags)" :key="t">#{{ t }}</span>
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

        <div v-if="idx === 0 && muted" class="sw-hint">点击画面播放 · 右下角开声音</div>
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

const router = useRouter();
const auth = useAuthStore();

const posts = ref<FeedPost[]>([]);
const loading = ref(false);
const hasMore = ref(true);
const muted = ref(true);
const page = ref(1);
const scroller = ref<HTMLElement | null>(null);
const isMobile = ref(false);
let ob: IntersectionObserver | null = null;

function plain(s: string) {
  return (s || '').replace(/[#*`>\[\]]/g, '').slice(0, 90);
}
function splitTags(s: string) {
  return (s || '').split(/[,，]/).map((x) => x.trim()).filter(Boolean).slice(0, 4);
}

async function load(reset = false) {
  if (loading.value) return;
  if (reset) {
    page.value = 1;
    posts.value = [];
    hasMore.value = true;
  }
  if (!hasMore.value && !reset) return;
  loading.value = true;
  try {
    const r = await feedApi.list({ kind: 'video', page: page.value, page_size: 6 });
    const list = r.data?.data?.list || [];
    posts.value = reset ? list : [...posts.value, ...list];
    hasMore.value = list.length >= 6;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
    await nextTick();
    observeItems();
  }
}

function observeItems() {
  if (!ob) return;
  document.querySelectorAll('.sw-item').forEach((el) => ob!.observe(el));
}

function onScroll() {
  const el = scroller.value;
  if (!el) return;
  // 触底前预取下一页（还剩不到两屏时）
  if (el.scrollTop + el.clientHeight * 2 >= el.scrollHeight && hasMore.value && !loading.value) {
    page.value++;
    load();
  }
}

function togglePlay(e: Event, idx: number) {
  const v = e.target as HTMLVideoElement;
  const others = document.querySelectorAll<HTMLVideoElement>('.sw-video');
  others.forEach((o, i) => {
    if (i !== idx) o.pause();
  });
  if (v.paused) {
    v.muted = muted.value;
    v.play().catch(() => {});
  } else {
    v.pause();
  }
}

function toggleMute() {
  muted.value = !muted.value;
  document.querySelectorAll<HTMLVideoElement>('.sw-video').forEach((v) => {
    v.muted = muted.value;
  });
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

onMounted(async () => {
  isMobile.value = window.matchMedia('(max-width: 767px)').matches;
  ob = new IntersectionObserver(
    (entries) => {
      entries.forEach((en) => {
        const v = en.target.querySelector('video') as HTMLVideoElement | null;
        if (!v) return;
        if (en.isIntersecting && en.intersectionRatio > 0.6) {
          v.muted = muted.value;
          v.play().catch(() => {});
        } else {
          v.pause();
        }
      });
    },
    { threshold: [0, 0.6, 0.95], root: scroller.value }
  );
  await load(true);
});

onBeforeUnmount(() => {
  ob?.disconnect();
  ob = null;
  document.querySelectorAll<HTMLVideoElement>('.sw-video').forEach((v) => v.pause());
});
</script>

<style scoped>
.swipe-feed { height: 100%; background: #000; }
.sw-scroller {
  height: 100%;
  overflow-y: auto;
  scroll-snap-type: y mandatory;
  scrollbar-width: none;
}
.sw-scroller::-webkit-scrollbar { display: none; }
.sw-item {
  position: relative;
  height: 100%;
  scroll-snap-align: start;
  scroll-snap-stop: always;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #000;
}
.sw-video {
  max-height: 100%;
  max-width: 100%;
  width: auto;
  height: 100%;
  object-fit: contain;
  cursor: pointer;
  background: #000;
}
.mobile .sw-video { width: 100%; height: 100%; object-fit: cover; }
/* 桌面端竖滑限制成竖屏比例，居中显示，避免大屏上一条视频占满整宽 */
.swipe-feed:not(.mobile) .sw-item { padding: 0; }
.swipe-feed:not(.mobile) .sw-video { max-width: 480px; }

.sw-mask {
  position: absolute; inset: auto 0 0 0; height: 46%;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.75), rgba(0, 0, 0, 0));
  pointer-events: none;
}
.sw-info {
  position: absolute; left: 16px; right: 76px; bottom: 22px; color: #fff;
  display: flex; flex-direction: column; gap: 8px;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.6);
}
.swipe-feed:not(.mobile) .sw-info { left: calc(50% - 240px + 16px); right: auto; width: 300px; }
.sw-author { display: flex; align-items: center; gap: 8px; font-size: 0.88rem; }
.sw-title { margin: 0; font-size: 1rem; font-weight: 600; line-height: 1.4; }
.sw-desc { margin: 0; font-size: 0.82rem; opacity: 0.9; line-height: 1.5; }
.sw-tags { display: flex; gap: 8px; flex-wrap: wrap; font-size: 0.78rem; opacity: 0.85; }
.sw-rail {
  position: absolute; right: 14px; bottom: 26px; display: flex; flex-direction: column; gap: 16px;
}
.swipe-feed:not(.mobile) .sw-rail { right: calc(50% - 240px + 14px); }
.sw-act {
  border: none; background: transparent; color: #fff; display: flex; flex-direction: column;
  align-items: center; gap: 2px; cursor: pointer; font-size: 0.72rem;
}
.sw-act .sw-ic { font-size: 1.5rem; line-height: 1; filter: drop-shadow(0 1px 3px rgba(0, 0, 0, 0.5)); }
.sw-act.on { color: #ff4d6d; }
.sw-act small { opacity: 0.9; }
.sw-hint {
  position: absolute; left: 50%; top: 14px; transform: translateX(-50%);
  color: #fff; font-size: 0.78rem; background: rgba(0, 0, 0, 0.45);
  padding: 4px 12px; border-radius: 12px;
}
.sw-loading, .sw-end {
  height: 56px; display: flex; align-items: center; justify-content: center;
  color: #9ca3af; font-size: 0.82rem; background: #000;
}
</style>
