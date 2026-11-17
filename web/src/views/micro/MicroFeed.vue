<template>
  <div class="micro-page">
    <div class="micro-head">
      <div>
        <h2>微文 · 贴图号</h2>
        <p class="micro-sub">用一张图、几句话，记录此刻</p>
      </div>
      <div class="micro-head-actions">
        <el-radio-group v-model="scope" size="small" @change="reload">
          <el-radio-button value="all">推荐</el-radio-button>
          <el-radio-button value="following">关注</el-radio-button>
          <el-radio-button value="mine">我的</el-radio-button>
        </el-radio-group>
        <el-button size="small" type="primary" @click="$router.push('/feed/compose?kind=micro')">＋ 写微文</el-button>
      </div>
    </div>

    <!-- 标签筛选（小红书式话题 chips） -->
    <div v-if="allTags.length" class="micro-chips">
      <button :class="{ active: !activeTag }" @click="activeTag = ''">全部</button>
      <button v-for="t in allTags" :key="t.name" :class="{ active: activeTag === t.name }" @click="activeTag = t.name">
        #{{ t.name }} <small>{{ t.count }}</small>
      </button>
    </div>

    <div v-loading="loading" class="micro-grid">
      <article v-for="p in visiblePosts" :key="p.id" class="micro-card" @click="openDetail(p)">
        <div class="mc-cover">
          <img v-if="p.images?.[0]" :src="media(p.images[0])" :alt="p.title || '微文配图'" loading="lazy" />
          <div v-else class="mc-cover-empty">✍️</div>
          <span v-if="p.images && p.images.length > 1" class="mc-count">1/{{ p.images.length }}</span>
        </div>
        <div class="mc-body">
          <h3 v-if="p.title" class="mc-title">{{ p.title }}</h3>
          <p class="mc-excerpt">{{ p.content }}</p>
          <div v-if="p.tags" class="mc-tags">
            <span v-for="t in p.tags.split(',').filter(Boolean)" :key="t">#{{ t.trim() }}</span>
          </div>
          <div class="mc-foot">
            <span class="mc-author">
              <el-avatar :size="22" :src="media(p.avatar_url)">{{ (p.author_name || '?')[0] }}</el-avatar>
              {{ p.author_name || '用户' + p.user_id }}
            </span>
            <span class="mc-ops">
              <button :class="{ active: p.liked }" @click.stop="onLike(p)">♥ {{ p.like_count }}</button>
              <button @click.stop="openDetail(p)">💬 {{ p.comment_count }}</button>
            </span>
          </div>
        </div>
      </article>
    </div>

    <el-empty v-if="!loading && visiblePosts.length === 0" description="还没有微文，点「写微文」发第一条" :image-size="70" />
    <div v-if="hasMore && !loading" class="micro-more"><el-button size="small" @click="loadMore">加载更多</el-button></div>

    <!-- 详情：全文 + 多图 + 评论 -->
    <el-dialog v-model="detailVisible" width="min(760px, 94vw)" top="4vh" class="micro-detail">
      <template v-if="detail">
        <h2 class="md-title">{{ detail.title || '微文详情' }}</h2>
        <div class="md-author">
          <el-avatar :size="34" :src="media(detail.avatar_url)">{{ (detail.author_name || '?')[0] }}</el-avatar>
          <div><b>{{ detail.author_name || '用户' + detail.user_id }}</b><small>{{ formatTime(detail.created_at) }}</small></div>
        </div>
        <p class="md-content">{{ detail.content }}</p>
        <div v-if="detail.images?.length" class="md-images">
          <img v-for="(im, i) in detail.images" :key="i" :src="media(im)" :alt="`图${i + 1}`" @click="preview(im)" />
        </div>
        <div class="md-actions">
          <button :class="{ active: detail.liked }" @click="onLike(detail)">♥ {{ detail.like_count }}</button>
          <button :class="{ active: detail.favorited }" @click="onFavorite(detail)">★ {{ detail.favorite_count }}</button>
          <button @click="onShare(detail)">↗ {{ detail.share_count }}</button>
        </div>
        <div class="md-comments">
          <h3>{{ detail.comment_count }} 条评论</h3>
          <div v-for="c in comments" :key="c.id" class="md-comment">
            <el-avatar :size="28" :src="media(c.avatar_url)">{{ (c.author_name || '?')[0] }}</el-avatar>
            <div><b>{{ c.author_name || '用户' + c.user_id }}</b> {{ c.content }}</div>
          </div>
          <el-empty v-if="comments.length === 0" description="还没有评论" :image-size="40" />
          <div class="md-input">
            <el-input v-model="commentInput" size="small" placeholder="说点什么…" @keyup.enter="submitComment(detail)" />
            <el-button size="small" type="primary" @click="submitComment(detail)">发送</el-button>
          </div>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="previewVisible" title="图片预览" width="min(90vw, 860px)">
      <img :src="previewUrl" style="width: 100%; display: block" alt="预览" />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage } from 'element-plus';
import { feedApi, type FeedPost, type FeedComment } from '@/api/feed';
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
function formatTime(s: string) { return s ? s.slice(0, 16).replace('T', ' ') : ''; }

const posts = ref<FeedPost[]>([]);
const scope = ref('all');
const loading = ref(false);
const page = ref(1);
const hasMore = ref(true);
const maxImages = ref(20);
const activeTag = ref('');

const allTags = computed(() => {
  const m = new Map<string, number>();
  posts.value.forEach((p) => (p.tags || '').split(',').forEach((t) => {
    const k = t.trim();
    if (k) m.set(k, (m.get(k) || 0) + 1);
  }));
  return [...m.entries()].map(([name, count]) => ({ name, count })).sort((a, b) => b.count - a.count).slice(0, 12);
});
const visiblePosts = computed(() =>
  activeTag.value ? posts.value.filter((p) => (p.tags || '').split(',').some((t) => t.trim() === activeTag.value)) : posts.value);

async function load(reset = false) {
  if (loading.value) return;
  if (reset) { page.value = 1; posts.value = []; hasMore.value = true; }
  loading.value = true;
  try {
    const res = await feedApi.list({ kind: 'micro', scope: scope.value, page: page.value, page_size: 12 });
    const list = res.data?.data?.list || [];
    posts.value = reset ? list : [...posts.value, ...list];
    hasMore.value = list.length >= 12;
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '加载失败'); }
  finally { loading.value = false; }
}
function reload() { activeTag.value = ''; load(true); }
function loadMore() { page.value++; load(); }
async function loadSingle(id: number) {
  loading.value = true;
  try {
    const r = await feedApi.get(id);
    const p = r.data?.data;
    posts.value = p ? [p] : [];
    hasMore.value = false;
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '加载失败'); }
  finally { loading.value = false; }
}

async function onLike(p: FeedPost) {
  if (!myId.value) { ElMessage.warning('请先登录'); return; }
  try {
    const r = await feedApi.like(p.id);
    p.liked = r.data?.data?.liked;
    p.like_count += p.liked ? 1 : -1;
    if (detail.value?.id === p.id) detail.value.liked = p.liked;
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '操作失败'); }
}
async function onFavorite(p: FeedPost) {
  if (!myId.value) { ElMessage.warning('请先登录'); return; }
  try {
    const r = await feedApi.favorite(p.id);
    p.favorited = r.data?.data?.favorited;
    p.favorite_count += p.favorited ? 1 : -1;
    if (detail.value?.id === p.id) detail.value.favorited = p.favorited;
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '操作失败'); }
}
async function onShare(p: FeedPost) {
  try {
    await feedApi.share(p.id);
    p.share_count++;
    await navigator.clipboard?.writeText(location.origin + prefix + '/micro?id=' + p.id);
    ElMessage.success('链接已复制');
  } catch { ElMessage.success('已记录分享'); }
}

// 详情 + 评论
const detailVisible = ref(false);
const detail = ref<FeedPost | null>(null);
const comments = ref<FeedComment[]>([]);
const commentInput = ref('');
async function openDetail(p: FeedPost) {
  detail.value = p;
  detailVisible.value = true;
  commentInput.value = '';
  await loadComments(p.id);
}
async function loadComments(id: number) {
  try {
    const r = await feedApi.comments(id);
    comments.value = r.data?.data?.list || [];
  } catch { comments.value = []; }
}
async function submitComment(p: FeedPost) {
  if (!commentInput.value.trim()) return;
  if (!myId.value) { ElMessage.warning('请先登录'); return; }
  try {
    await feedApi.addComment(p.id, commentInput.value.trim());
    commentInput.value = '';
    p.comment_count++;
    if (detail.value?.id === p.id) detail.value.comment_count = p.comment_count;
    await loadComments(p.id);
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '评论失败'); }
}

const previewVisible = ref(false);
const previewUrl = ref('');
function preview(u: string) { previewUrl.value = media(u); previewVisible.value = true; }

onMounted(async () => {
  try {
    const r = await feedApi.config();
    maxImages.value = r.data?.data?.max_micro_images || 20;
  } catch { maxImages.value = 20; }
  const id = Number(route.query.id);
  if (id) loadSingle(id); else load(true);
});
</script>

<style scoped>
.micro-page { max-width: 1120px; margin: 0 auto; padding: 22px 18px 50px; }
.micro-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.micro-head h2 { margin: 0; font-size: 1.35rem; }
.micro-sub { margin: 4px 0 0; font-size: .8rem; color: var(--text-muted, #6b7280); }
.micro-head-actions { display: flex; gap: 8px; }
.micro-chips { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 18px; }
.micro-chips button { border: 1px solid var(--border-color, #e5e7eb); background: var(--bg-card, #fff); color: var(--text-color, #222); border-radius: 18px; padding: 5px 12px; font-size: .82rem; cursor: pointer; }
.micro-chips button.active { background: var(--primary-color, #5b6ef5); border-color: var(--primary-color); color: #fff; }
.micro-chips small { opacity: .65; }
.micro-grid { column-count: 2; column-gap: 16px; min-height: 220px; }
.micro-card {
  display: inline-block; width: 100%; break-inside: avoid; margin-bottom: 16px;
  background: var(--bg-card, #fff); border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 14px; overflow: hidden; cursor: pointer; transition: transform .15s, box-shadow .15s;
}
.micro-card:hover { transform: translateY(-3px); box-shadow: 0 10px 24px rgba(0,0,0,.09); }
.mc-cover { position: relative; aspect-ratio: 4 / 3; background: var(--fill-color, #f0f2f5); overflow: hidden; }
.mc-cover img { width: 100%; height: 100%; object-fit: cover; display: block; }
.mc-cover-empty { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; font-size: 2.4rem; }
.mc-count { position: absolute; right: 10px; bottom: 10px; background: rgba(0,0,0,.6); color: #fff; font-size: .72rem; padding: 2px 8px; border-radius: 10px; }
.mc-body { padding: 14px 16px 12px; }
.mc-title { margin: 0 0 6px; font-size: 1.02rem; line-height: 1.45; }
.mc-excerpt { margin: 0; font-size: .86rem; color: var(--text-muted, #6b7280); line-height: 1.65;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.mc-tags { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 10px; color: var(--primary-color); font-size: .78rem; }
.mc-foot { display: flex; justify-content: space-between; align-items: center; margin-top: 12px; padding-top: 10px; border-top: 1px solid var(--fill-color, #f3f4f6); }
.mc-author { display: flex; align-items: center; gap: 6px; font-size: .78rem; color: var(--text-muted, #6b7280); }
.mc-ops { display: flex; gap: 10px; }
.mc-ops button { border: none; background: transparent; color: var(--text-muted, #6b7280); cursor: pointer; font-size: .8rem; padding: 0; }
.mc-ops button.active { color: #ff4d6d; }
.micro-more { text-align: center; padding: 14px; }
[data-theme="dark"] .micro-card { background: #1e293b; border-color: #334155; }

.md-title { margin: 0 0 12px; font-size: 1.2rem; }
.md-author { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
.md-author div { display: flex; flex-direction: column; font-size: .85rem; }
.md-author small { color: var(--text-muted, #888); }
.md-content { white-space: pre-wrap; line-height: 1.9; font-size: .95rem; }
.md-images { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
.md-images img { width: 100%; aspect-ratio: 1; object-fit: cover; border-radius: 8px; cursor: zoom-in; }
.md-actions { display: flex; gap: 20px; padding: 14px 0; border-top: 1px solid var(--border-color); border-bottom: 1px solid var(--border-color); margin: 14px 0; }
.md-actions button { border: none; background: transparent; color: var(--text-muted, #666); cursor: pointer; font-size: .9rem; }
.md-actions button.active { color: #ff4d6d; }
.md-comments h3 { font-size: 1rem; margin: 0 0 10px; }
.md-comment { display: flex; gap: 8px; align-items: flex-start; padding: 8px 0; font-size: .86rem; }
.md-comment b { color: var(--primary-color); margin-right: 6px; }
.md-input { display: flex; gap: 8px; margin-top: 12px; }
@media (max-width: 768px) {
  .micro-page { padding: 14px 10px 30px; }
  .micro-grid { column-count: 1; }
  .micro-head { align-items: flex-start; gap: 10px; flex-direction: column; }
  .md-images { grid-template-columns: repeat(2, 1fr); }
}
</style>
