<template>
  <div class="shorts-page">
    <div class="shorts-head">
      <h2>短视频</h2>
      <div class="shorts-head-actions">
        <el-radio-group v-model="scope" size="small" @change="reload">
          <el-radio-button value="all">推荐</el-radio-button>
          <el-radio-button value="following">关注</el-radio-button>
          <el-radio-button value="mine">我的</el-radio-button>
        </el-radio-group>
        <el-button size="small" type="primary" @click="$router.push('/feed/compose?kind=video')">＋ 发视频</el-button>
      </div>
    </div>

    <div ref="containerRef" class="shorts-feed" @scroll.passive="onScroll">
      <div v-for="(p, i) in posts" :key="p.id" class="short-item" :data-index="i">
        <div class="short-stage">
          <video
            v-if="p.kind === 'video'"
            :ref="(el) => setVideoRef(p.id, el)"
            class="short-video"
            :src="media(p.video_url)"
            :poster="media(p.cover_url)"
            playsinline
            loop
            muted
            preload="metadata"
            @click="togglePlay(p.id)"
          />
          <img v-else class="short-video" :src="media(p.cover_url || p.images?.[0])" :alt="p.title" />

          <!-- 底部信息 -->
          <div class="short-meta">
            <div class="short-author">
              <el-avatar :size="34" :src="media(p.avatar_url)">{{ (p.author_name || '?')[0] }}</el-avatar>
              <span class="short-name">@{{ p.author_name || '用户' + p.user_id }}</span>
              <el-button
                v-if="p.user_id !== myId"
                size="small"
                :type="followed[p.user_id] ? 'info' : 'primary'"
                round
                @click="toggleFollow(p)"
              >
                {{ followed[p.user_id] ? '已关注' : '关注' }}
              </el-button>
            </div>
            <div class="short-desc">{{ p.content || p.title }}</div>
            <div v-if="p.tags" class="short-tags">{{ p.tags }}</div>
          </div>

          <!-- 右侧操作栏（类微信视频号） -->
          <div class="short-actions">
            <button class="sa-btn" :class="{ active: p.liked }" title="点赞" @click="onLike(p)">
              <span class="sa-ico">{{ p.liked ? '♥' : '♡' }}</span>
              <span>{{ p.like_count }}</span>
            </button>
            <button class="sa-btn" title="评论" @click="openComments(p)">
              <span class="sa-ico">💬</span>
              <span>{{ p.comment_count }}</span>
            </button>
            <button class="sa-btn" :class="{ active: p.favorited }" title="收藏" @click="onFavorite(p)">
              <span class="sa-ico">{{ p.favorited ? '★' : '☆' }}</span>
              <span>{{ p.favorite_count }}</span>
            </button>
            <button class="sa-btn" title="分享" @click="onShare(p)">
              <span class="sa-ico">↗</span>
              <span>{{ p.share_count }}</span>
            </button>
          </div>

          <div v-if="p.kind === 'video' && activeIndex !== i" class="short-playhint">▶</div>
        </div>
      </div>

      <el-empty v-if="!loading && posts.length === 0" description="还没有短视频，点「发视频」上传第一条" :image-size="70" />
      <div v-if="loading" class="shorts-loading">加载中…</div>
      <div v-if="hasMore && !loading" class="shorts-more"><el-button size="small" @click="loadMore">加载更多</el-button></div>
    </div>

    <!-- 评论抽屉（微信式底部弹出） -->
    <el-drawer v-model="commentVisible" direction="btt" size="72%" :with-header="false" class="short-comments">
      <div class="sc-head">
        <span>{{ current?.comment_count || 0 }} 条评论</span>
        <el-button text @click="commentVisible = false">关闭</el-button>
      </div>
      <div class="sc-list">
        <div v-for="c in comments" :key="c.id" class="sc-row">
          <el-avatar :size="32" :src="media(c.avatar_url)">{{ (c.author_name || '?')[0] }}</el-avatar>
          <div class="sc-body">
            <div class="sc-name">{{ c.author_name || '用户' + c.user_id }}</div>
            <div class="sc-text">{{ c.content }}</div>
            <div class="sc-ops">
              <span class="sc-time">{{ formatTime(c.created_at) }}</span>
              <button @click="replyTo = c.id; commentInput = `回复 @${c.author_name}: `">回复</button>
              <button @click="onLikeComment(c)">♥ {{ c.like_count }}</button>
              <button v-if="c.user_id === myId" @click="onDeleteComment(c)">删除</button>
            </div>
          </div>
        </div>
        <el-empty v-if="comments.length === 0" description="还没有评论" :image-size="50" />
      </div>
      <div class="sc-input">
        <el-input v-model="commentInput" placeholder="说点什么…" clearable @keyup.enter="submitComment" />
        <el-button type="primary" :loading="commenting" @click="submitComment">发送</el-button>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage } from 'element-plus';
import { feedApi, type FeedPost, type FeedComment } from '@/api/feed';
import { followApi } from '@/api/social';
import { getAppPrefix } from '@/utils/prefix';
import { useAuthStore } from '@/stores/auth';

const route = useRoute();
const auth = useAuthStore();
const myId = computed(() => auth.user?.id || 0);
const prefix = getAppPrefix() || '';

function media(u?: string) {
  if (!u) return '';
  if (/^https?:\/\//i.test(u)) return u;
  return prefix + (u.startsWith('/') ? u : '/' + u);
}
function formatTime(s: string) {
  if (!s) return '';
  return s.slice(5, 16).replace('T', ' ');
}

const posts = ref<FeedPost[]>([]);
const scope = ref('all');
const loading = ref(false);
const page = ref(1);
const hasMore = ref(true);
const containerRef = ref<HTMLElement | null>(null);
const activeIndex = ref(0);
const videoEls = new Map<number, HTMLVideoElement>();
const followed = ref<Record<number, boolean>>({});

function setVideoRef(id: number, el: any) {
  if (el) videoEls.set(id, el as HTMLVideoElement);
  else videoEls.delete(id);
}

async function load(reset = false) {
  if (loading.value) return;
  if (reset) {
    page.value = 1;
    posts.value = [];
    hasMore.value = true;
  }
  loading.value = true;
  try {
    const res = await feedApi.list({ kind: 'video', scope: scope.value, page: page.value, page_size: 8 });
    const list = res.data?.data?.list || [];
    posts.value = reset ? list : [...posts.value, ...list];
    hasMore.value = list.length >= 8;
    await nextTick();
    syncPlayback();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}
function reload() { load(true); }
function loadMore() { page.value++; load(); }
async function loadSingle(id: number) {
  loading.value = true;
  try {
    const r = await feedApi.get(id);
    const p = r.data?.data;
    posts.value = p ? [p] : [];
    hasMore.value = false;
    await nextTick();
    syncPlayback();
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '加载失败'); }
  finally { loading.value = false; }
}

function onScroll() {
  const el = containerRef.value;
  if (!el) return;
  const idx = Math.round(el.scrollTop / el.clientHeight);
  if (idx !== activeIndex.value) {
    activeIndex.value = idx;
    syncPlayback();
  }
}
function syncPlayback() {
  posts.value.forEach((p, i) => {
    const v = videoEls.get(p.id);
    if (!v) return;
    if (i === activeIndex.value) {
      v.play().catch(() => { /* 自动播放被浏览器拦截时可点画面播放 */ });
    } else {
      v.pause();
    }
  });
}
function togglePlay(id: number) {
  const v = videoEls.get(id);
  if (!v) return;
  if (v.paused) v.play().catch(() => {}); else v.pause();
}

function onKey(e: KeyboardEvent) {
  if (commentVisible.value) return;
  if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
  e.preventDefault();
  const next = Math.max(0, Math.min(posts.value.length - 1, activeIndex.value + (e.key === 'ArrowDown' ? 1 : -1)));
  containerRef.value?.scrollTo({ top: next * (containerRef.value.clientHeight || 1), behavior: 'smooth' });
}

async function onLike(p: FeedPost) {
  if (!myId.value) { ElMessage.warning('请先登录'); return; }
  try {
    const r = await feedApi.like(p.id);
    p.liked = r.data?.data?.liked;
    p.like_count += p.liked ? 1 : -1;
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '操作失败'); }
}
async function onFavorite(p: FeedPost) {
  if (!myId.value) { ElMessage.warning('请先登录'); return; }
  try {
    const r = await feedApi.favorite(p.id);
    p.favorited = r.data?.data?.favorited;
    p.favorite_count += p.favorited ? 1 : -1;
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '操作失败'); }
}
async function onShare(p: FeedPost) {
  try {
    await feedApi.share(p.id);
    p.share_count++;
    await navigator.clipboard?.writeText(location.origin + prefix + '/shorts?id=' + p.id);
    ElMessage.success('链接已复制');
  } catch { ElMessage.success('已记录分享'); }
}
async function toggleFollow(p: FeedPost) {
  if (!myId.value) { ElMessage.warning('请先登录'); return; }
  try {
    if (followed.value[p.user_id]) {
      await followApi.unfollow(p.user_id);
      followed.value[p.user_id] = false;
    } else {
      await followApi.follow(p.user_id);
      followed.value[p.user_id] = true;
    }
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '操作失败'); }
}

// ── 评论 ──
const commentVisible = ref(false);
const current = ref<FeedPost | null>(null);
const comments = ref<FeedComment[]>([]);
const commentInput = ref('');
const commenting = ref(false);
const replyTo = ref(0);

async function openComments(p: FeedPost) {
  current.value = p;
  replyTo.value = 0;
  commentInput.value = '';
  commentVisible.value = true;
  await loadComments(p.id);
}
async function loadComments(id: number) {
  try {
    const r = await feedApi.comments(id);
    comments.value = r.data?.data?.list || [];
  } catch { comments.value = []; }
}
async function submitComment() {
  if (!current.value || !commentInput.value.trim()) return;
  if (!myId.value) { ElMessage.warning('请先登录'); return; }
  commenting.value = true;
  try {
    await feedApi.addComment(current.value.id, commentInput.value.trim(), replyTo.value);
    commentInput.value = '';
    replyTo.value = 0;
    current.value.comment_count++;
    await loadComments(current.value.id);
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '评论失败'); }
  finally { commenting.value = false; }
}
async function onLikeComment(c: FeedComment) {
  try {
    await feedApi.likeComment(c.id);
    c.like_count++;
  } catch { /* ignore */ }
}
async function onDeleteComment(c: FeedComment) {
  try {
    await feedApi.deleteComment(c.id);
    if (current.value) current.value.comment_count--;
    await loadComments(c.post_id);
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '删除失败'); }
}

onMounted(() => {
  const id = Number(route.query.id);
  if (id) loadSingle(id); else load(true);
  window.addEventListener('keydown', onKey);
});
onUnmounted(() => {
  window.removeEventListener('keydown', onKey);
  videoEls.forEach((v) => v.pause());
  videoEls.clear();
});
</script>

<style scoped>
.shorts-page { position: relative; height: calc(100vh - 60px); background: #0b0b0d; color: #fff; overflow: hidden; }
.shorts-head {
  position: absolute; top: 8px; left: 0; right: 0; z-index: 5;
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 20px; pointer-events: none;
}
.shorts-head h2 { margin: 0; font-size: 1.05rem; text-shadow: 0 1px 4px #000; pointer-events: none; }
.shorts-head-actions { display: flex; gap: 8px; pointer-events: auto; }
.shorts-feed { height: 100%; overflow-y: scroll; scroll-snap-type: y mandatory; scrollbar-width: none; }
.shorts-feed::-webkit-scrollbar { display: none; }
.short-item { height: 100%; scroll-snap-align: start; display: flex; align-items: center; justify-content: center; }
.short-stage {
  position: relative; height: 100%; width: min(100%, 480px);
  display: flex; align-items: center; justify-content: center;
  background: #000; overflow: hidden;
}
.short-video { width: 100%; height: 100%; object-fit: contain; background: #000; cursor: pointer; }
.short-meta { position: absolute; left: 14px; right: 78px; bottom: 22px; text-shadow: 0 1px 4px #000; }
.short-author { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.short-name { font-weight: 600; font-size: 0.92rem; }
.short-desc { font-size: 0.88rem; line-height: 1.5; white-space: pre-wrap; }
.short-tags { margin-top: 6px; font-size: 0.8rem; color: #7cc4ff; }
.short-actions {
  position: absolute; right: 12px; bottom: 26px;
  display: flex; flex-direction: column; gap: 16px; align-items: center;
}
.sa-btn {
  background: transparent; border: none; color: #fff; cursor: pointer;
  display: flex; flex-direction: column; align-items: center; gap: 2px;
  font-size: 0.75rem; text-shadow: 0 1px 3px #000; padding: 0;
}
.sa-btn.active { color: #ff4d6d; }
.sa-ico { font-size: 1.6rem; line-height: 1.1; }
.short-playhint {
  position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%);
  font-size: 2.4rem; color: rgba(255,255,255,.7); pointer-events: none;
}
.shorts-loading, .shorts-more { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); z-index: 4; }
.shorts-more { top: auto; bottom: 10px; }

.sc-head { display: flex; justify-content: space-between; align-items: center; padding: 14px 18px; border-bottom: 1px solid var(--border-color); }
.sc-list { height: calc(100% - 118px); overflow-y: auto; padding: 10px 18px; }
.sc-row { display: flex; gap: 10px; padding: 10px 0; border-bottom: 1px solid var(--fill-color, #f3f4f6); }
.sc-body { flex: 1; min-width: 0; }
.sc-name { font-size: 0.82rem; color: var(--text-muted, #6b7280); }
.sc-text { font-size: 0.92rem; margin: 3px 0 6px; word-break: break-word; }
.sc-ops { display: flex; gap: 12px; font-size: 0.75rem; color: var(--text-muted, #6b7280); }
.sc-ops button { border: none; background: transparent; color: inherit; cursor: pointer; padding: 0; }
.sc-ops button:hover { color: var(--primary-color); }
.sc-time { margin-right: auto; }
.sc-input { display: flex; gap: 8px; padding: 12px 18px; border-top: 1px solid var(--border-color); }
</style>
