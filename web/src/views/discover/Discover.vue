<template>
  <div class="discover-page">
    <div class="discover-head">
      <div>
        <h2>发现</h2>
        <p class="discover-sub">图文 · 短视频 · 音乐，一起看</p>
      </div>
      <div class="discover-actions">
        <el-radio-group v-model="scope" size="small" @change="reload">
          <el-radio-button value="all">推荐</el-radio-button>
          <el-radio-button value="following">关注</el-radio-button>
          <el-radio-button value="mine">我的</el-radio-button>
        </el-radio-group>
        <el-button size="small" type="primary" @click="$router.push('/feed/compose')">＋ 发布</el-button>
      </div>
    </div>

    <div class="discover-chips">
      <button v-for="f in filters" :key="f.key" :class="{ active: filter === f.key }" @click="filter = f.key">
        {{ f.label }}
      </button>
    </div>

    <div v-loading="loading" class="discover-grid">
      <article v-for="p in visiblePosts" :key="p.id" class="ds-card" @click="openDetail(p)">
        <div class="ds-cover">
          <img v-if="coverOf(p)" :src="mediaUrl(coverOf(p))" :alt="p.title || '封面'" loading="lazy" />
          <div v-else class="ds-cover-empty">{{ p.kind === 'music' ? '🎵' : p.kind === 'video' ? '🎬' : '✍️' }}</div>
          <span v-if="p.kind === 'video'" class="ds-badge play">▶ 视频</span>
          <span v-else-if="p.kind === 'music'" class="ds-badge music">♫ 音乐</span>
          <span v-else-if="p.images?.length > 1" class="ds-badge">1/{{ p.images.length }}</span>
        </div>
        <div class="ds-body">
          <h3 class="ds-title">{{ p.title || (p.content || '').slice(0, 40) || '未命名' }}</h3>
          <p class="ds-excerpt">{{ (p.content || '').replace(/[#*`>\[\]]/g, '').slice(0, 70) }}</p>
          <div class="ds-foot">
            <span class="ds-author">
              <el-avatar :size="20" :src="mediaUrl(p.avatar_url)">{{ (p.author_name || '?')[0] }}</el-avatar>
              {{ p.author_name || '用户' + p.user_id }}
            </span>
            <span class="ds-ops">
              <button :class="{ active: p.liked }" @click.stop="onLike(p)">♥ {{ p.like_count }}</button>
              <button @click.stop="openDetail(p)">💬 {{ p.comment_count }}</button>
            </span>
          </div>
        </div>
      </article>
    </div>

    <el-empty v-if="!loading && visiblePosts.length === 0" description="还没有内容，去发布第一条吧" :image-size="70" />
    <div v-if="hasMore && !loading" class="discover-more"><el-button size="small" @click="loadMore">加载更多</el-button></div>

    <!-- 详情：三种 kind 分别播放/展示 -->
    <el-dialog v-model="detailVisible" width="min(760px, 94vw)" top="4vh" class="ds-detail">
      <template v-if="detail">
        <h2 class="dd-title">{{ detail.title || '详情' }}</h2>
        <div class="dd-author">
          <el-avatar :size="34" :src="mediaUrl(detail.avatar_url)">{{ (detail.author_name || '?')[0] }}</el-avatar>
          <div><b>{{ detail.author_name || '用户' + detail.user_id }}</b><small>{{ (detail.created_at || '').slice(0, 16).replace('T', ' ') }}</small></div>
        </div>

        <video v-if="detail.kind === 'video'" :src="mediaUrl(detail.video_url)" :poster="mediaUrl(detail.cover_url)" controls style="width: 100%; max-height: 460px" />
        <div v-else-if="detail.kind === 'music'" class="dd-music">
          <img v-if="detail.cover_url" :src="mediaUrl(detail.cover_url)" alt="封面" />
          <audio :src="mediaUrl(detail.audio_url)" controls style="width: 100%" />
        </div>
        <div v-else-if="detail.images?.length" class="dd-images">
          <img v-for="(im, i) in detail.images" :key="i" :src="mediaUrl(im)" :alt="`图${i + 1}`" />
        </div>

        <!-- 正文走全站 Markdown 渲染器，与编辑器预览同源 -->
        <div v-if="detail.content" class="dd-content cherry-markdown cherry-previewer" v-html="detailHtml" />

        <div class="dd-actions">
          <button :class="{ active: detail.liked }" @click="onLike(detail)">♥ {{ detail.like_count }}</button>
          <button :class="{ active: detail.favorited }" @click="onFavorite(detail)">★ {{ detail.favorite_count }}</button>
          <button @click="onShare(detail)">↗ {{ detail.share_count }}</button>
        </div>
        <div class="dd-comments">
          <h3>{{ detail.comment_count }} 条评论</h3>
          <div v-for="c in comments" :key="c.id" class="dd-comment">
            <el-avatar :size="28" :src="mediaUrl(c.avatar_url)">{{ (c.author_name || '?')[0] }}</el-avatar>
            <div><b>{{ c.author_name || '用户' + c.user_id }}</b> {{ c.content }}</div>
          </div>
          <el-empty v-if="comments.length === 0" description="还没有评论" :image-size="40" />
          <div class="dd-input">
            <el-input v-model="commentInput" size="small" placeholder="说点什么…" @keyup.enter="submitComment" />
            <el-button size="small" type="primary" @click="submitComment">发送</el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage } from 'element-plus';
import { feedApi, type FeedPost, type FeedComment } from '@/api/feed';
import { mediaUrl } from '@/utils/prefix';
import { useAuthStore } from '@/stores/auth';

const route = useRoute();
const auth = useAuthStore();
const myId = computed(() => auth.user?.id || 0);

const filters = [
  { key: 'all', label: '全部' },
  { key: 'micro', label: '图文' },
  { key: 'video', label: '短视频' },
  { key: 'music', label: '音乐' },
];
const filter = ref('all');
const scope = ref('all');
const posts = ref<FeedPost[]>([]);
const loading = ref(false);
const page = ref(1);
const hasMore = ref(true);
const visiblePosts = computed(() => filter.value === 'all' ? posts.value : posts.value.filter((p) => p.kind === filter.value));
const detailVisible = ref(false);
const detail = ref<FeedPost | null>(null);
const detailHtml = ref('');
const comments = ref<FeedComment[]>([]);
const commentInput = ref('');

function coverOf(p: FeedPost) {
  if (p.cover_url) return p.cover_url;
  if (p.kind === 'micro') return p.images?.[0] || '';
  return '';
}

async function load(reset = false) {
  if (loading.value) return;
  if (reset) { page.value = 1; posts.value = []; hasMore.value = true; }
  loading.value = true;
  try {
    const r = await feedApi.list({ scope: scope.value, page: page.value, page_size: 12 });
    const list = r.data?.data?.list || [];
    posts.value = reset ? list : [...posts.value, ...list];
    hasMore.value = list.length >= 12;
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '加载失败'); }
  finally { loading.value = false; }
}
function reload() { filter.value = 'all'; load(true); }
function loadMore() { page.value++; load(); }

async function openDetail(p: FeedPost) {
  detail.value = p;
  detailHtml.value = '';
  commentInput.value = '';
  detailVisible.value = true;
  if (p.content) {
    const { renderMarkdown } = await import('@/markdown/renderer');
    detailHtml.value = renderMarkdown(p.content);
  }
  await loadComments(p.id);
}
async function loadComments(id: number) {
  try { const r = await feedApi.comments(id); comments.value = r.data?.data?.list || []; }
  catch { comments.value = []; }
}
async function submitComment() {
  if (!detail.value || !commentInput.value.trim()) return;
  if (!myId.value) { ElMessage.warning('请先登录'); return; }
  try {
    await feedApi.addComment(detail.value.id, commentInput.value.trim());
    commentInput.value = '';
    detail.value.comment_count++;
    await loadComments(detail.value.id);
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '评论失败'); }
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
    await navigator.clipboard?.writeText(location.href.split('?')[0] + `?id=${p.id}`);
    ElMessage.success('链接已复制');
  } catch { ElMessage.success('已记录分享'); }
}

watch(() => route.query.id, async (v) => {
  const id = Number(v);
  if (!id) return;
  try {
    const r = await feedApi.get(id);
    const p = r.data?.data;
    if (p) { posts.value = [p]; hasMore.value = false; await openDetail(p); }
  } catch { /* ignore */ }
}, { immediate: true });

onMounted(() => { if (!route.query.id) load(true); });
</script>

<style scoped>
.discover-page { max-width: 1120px; margin: 0 auto; padding: 22px 18px 50px; }
.discover-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; }
.discover-head h2 { margin: 0; font-size: 1.4rem; }
.discover-sub { margin: 4px 0 0; font-size: .8rem; color: var(--text-muted, #6b7280); }
.discover-actions { display: flex; gap: 8px; }
.discover-chips { display: flex; gap: 8px; margin-bottom: 18px; }
.discover-chips button { border: 1px solid var(--border-color, #e5e7eb); background: var(--bg-card, #fff); color: var(--text-color, #222); border-radius: 18px; padding: 5px 14px; font-size: .83rem; cursor: pointer; }
.discover-chips button.active { background: var(--primary-color, #5b6ef5); border-color: var(--primary-color); color: #fff; }
.discover-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(248px, 1fr)); gap: 16px; min-height: 220px; }
.ds-card { background: var(--bg-card, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 14px; overflow: hidden; cursor: pointer; transition: transform .15s, box-shadow .15s; }
.ds-card:hover { transform: translateY(-3px); box-shadow: 0 10px 24px rgba(0,0,0,.09); }
.ds-cover { position: relative; aspect-ratio: 4 / 3; background: var(--fill-color, #f0f2f5); }
.ds-cover img { width: 100%; height: 100%; object-fit: cover; display: block; }
.ds-cover-empty { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; font-size: 2.6rem; }
.ds-badge { position: absolute; left: 10px; top: 10px; background: rgba(0,0,0,.6); color: #fff; font-size: .7rem; padding: 3px 8px; border-radius: 10px; }
.ds-badge.play { background: rgba(91,110,245,.85); }
.ds-badge.music { background: rgba(236,72,153,.85); }
.ds-body { padding: 13px 15px 11px; }
.ds-title { margin: 0 0 6px; font-size: .98rem; line-height: 1.45; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.ds-excerpt { margin: 0; font-size: .82rem; color: var(--text-muted, #6b7280); line-height: 1.6; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.ds-foot { display: flex; justify-content: space-between; align-items: center; margin-top: 11px; padding-top: 9px; border-top: 1px solid var(--fill-color, #f3f4f6); }
.ds-author { display: flex; align-items: center; gap: 6px; font-size: .76rem; color: var(--text-muted, #6b7280); }
.ds-ops { display: flex; gap: 9px; }
.ds-ops button { border: none; background: transparent; color: var(--text-muted, #6b7280); cursor: pointer; font-size: .78rem; padding: 0; }
.ds-ops button.active { color: #ff4d6d; }
.discover-more { text-align: center; padding: 14px; }
[data-theme="dark"] .ds-card { background: #1e293b; border-color: #334155; }

.dd-title { margin: 0 0 12px; font-size: 1.2rem; }
.dd-author { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
.dd-author div { display: flex; flex-direction: column; font-size: .85rem; }
.dd-author small { color: var(--text-muted, #888); }
.dd-music { display: flex; flex-direction: column; gap: 12px; align-items: center; }
.dd-music img { max-width: 280px; max-height: 280px; border-radius: 10px; object-fit: cover; }
.dd-images { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
.dd-images img { width: 100%; aspect-ratio: 1; object-fit: cover; border-radius: 8px; }
.dd-content { margin: 14px 0; line-height: 1.8; font-size: .92rem; }
.dd-actions { display: flex; gap: 20px; padding: 12px 0; border-top: 1px solid var(--border-color); border-bottom: 1px solid var(--border-color); }
.dd-actions button { border: none; background: transparent; color: var(--text-muted, #666); cursor: pointer; font-size: .9rem; }
.dd-actions button.active { color: #ff4d6d; }
.dd-comments h3 { font-size: 1rem; margin: 14px 0 10px; }
.dd-comment { display: flex; gap: 8px; padding: 7px 0; font-size: .86rem; }
.dd-comment b { color: var(--primary-color); margin-right: 6px; }
.dd-input { display: flex; gap: 8px; margin-top: 12px; }
@media (max-width: 768px) {
  .discover-page { padding: 14px 10px 30px; }
  .discover-grid { grid-template-columns: repeat(2, 1fr); gap: 10px; }
  .discover-head { flex-direction: column; align-items: flex-start; gap: 10px; }
  .dd-images { grid-template-columns: repeat(2, 1fr); }
}
</style>
