<template>
  <div class="micro-page">
    <div class="micro-head">
      <h2>微文 · 贴图</h2>
      <div class="micro-head-actions">
        <el-radio-group v-model="scope" size="small" @change="reload">
          <el-radio-button value="all">推荐</el-radio-button>
          <el-radio-button value="following">关注</el-radio-button>
          <el-radio-button value="mine">我的</el-radio-button>
        </el-radio-group>
        <el-button size="small" type="primary" @click="$router.push('/feed/compose?kind=micro')">＋ 写微文</el-button>
      </div>
    </div>

    <div v-loading="loading" class="micro-list">
      <article v-for="p in posts" :key="p.id" class="micro-card">
        <header class="mc-head">
          <el-avatar :size="40" :src="media(p.avatar_url)">{{ (p.author_name || '?')[0] }}</el-avatar>
          <div class="mc-author">
            <div class="mc-name">{{ p.author_name || '用户' + p.user_id }}</div>
            <div class="mc-time">{{ formatTime(p.created_at) }}</div>
          </div>
        </header>

        <h3 v-if="p.title" class="mc-title">{{ p.title }}</h3>
        <div class="mc-text" :class="{ 'is-fold': !expanded[p.id] && (p.content || '').length > 160 }">
          {{ p.content }}
        </div>
        <button v-if="(p.content || '').length > 160" class="mc-expand" @click="expanded[p.id] = !expanded[p.id]">
          {{ expanded[p.id] ? '收起' : '展开全文' }}
        </button>

        <div v-if="p.images && p.images.length" class="mc-images" :class="`n${Math.min(p.images.length, 9)}`">
          <img v-for="(im, i) in p.images.slice(0, 9)" :key="i" :src="media(im)" :alt="`图${i + 1}`" loading="lazy" @click="preview(im)" />
        </div>

        <div v-if="p.tags" class="mc-tags"><span v-for="t in p.tags.split(',').filter(Boolean)" :key="t">#{{ t.trim() }}</span></div>

        <footer class="mc-actions">
          <button :class="{ active: p.liked }" @click="onLike(p)">♥ {{ p.like_count }}</button>
          <button @click="openComments(p)">💬 {{ p.comment_count }}</button>
          <button :class="{ active: p.favorited }" @click="onFavorite(p)">★ {{ p.favorite_count }}</button>
          <button @click="onShare(p)">↗ {{ p.share_count }}</button>
        </footer>

        <!-- 内嵌评论（微信式） -->
        <div v-if="commentFor === p.id" class="mc-comments">
          <div v-for="c in comments" :key="c.id" class="mcc-row">
            <el-avatar :size="26" :src="media(c.avatar_url)">{{ (c.author_name || '?')[0] }}</el-avatar>
            <div class="mcc-body">
              <b>{{ c.author_name || '用户' + c.user_id }}</b> {{ c.content }}
              <button @click="feedApi.likeComment(c.id).then(() => c.like_count++)">♥ {{ c.like_count }}</button>
            </div>
          </div>
          <div class="mcc-input">
            <el-input v-model="commentInput" size="small" placeholder="评论…" @keyup.enter="submitComment(p)" />
            <el-button size="small" type="primary" @click="submitComment(p)">发送</el-button>
          </div>
        </div>
      </article>

      <el-empty v-if="!loading && posts.length === 0" description="还没有微文，点「写微文」发第一条" :image-size="70" />
      <div v-if="hasMore && !loading" class="micro-more"><el-button size="small" @click="loadMore">加载更多</el-button></div>
    </div>

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
const expanded = ref<Record<number, boolean>>({});

async function load(reset = false) {
  if (loading.value) return;
  if (reset) { page.value = 1; posts.value = []; hasMore.value = true; }
  loading.value = true;
  try {
    const res = await feedApi.list({ kind: 'micro', scope: scope.value, page: page.value, page_size: 10 });
    const list = res.data?.data?.list || [];
    posts.value = reset ? list : [...posts.value, ...list];
    hasMore.value = list.length >= 10;
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '加载失败'); }
  finally { loading.value = false; }
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
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '加载失败'); }
  finally { loading.value = false; }
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
    await navigator.clipboard?.writeText(location.origin + prefix + '/micro?id=' + p.id);
    ElMessage.success('链接已复制');
  } catch { ElMessage.success('已记录分享'); }
}

// 评论
const commentFor = ref(0);
const comments = ref<FeedComment[]>([]);
const commentInput = ref('');
async function openComments(p: FeedPost) {
  if (commentFor.value === p.id) { commentFor.value = 0; return; }
  commentFor.value = p.id;
  const r = await feedApi.comments(p.id);
  comments.value = r.data?.data?.list || [];
}
async function submitComment(p: FeedPost) {
  if (!commentInput.value.trim()) return;
  if (!myId.value) { ElMessage.warning('请先登录'); return; }
  try {
    await feedApi.addComment(p.id, commentInput.value.trim());
    commentInput.value = '';
    p.comment_count++;
    const r = await feedApi.comments(p.id);
    comments.value = r.data?.data?.list || [];
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '评论失败'); }
}

// 图片预览
const previewVisible = ref(false);
const previewUrl = ref('');
function preview(u: string) { previewUrl.value = media(u); previewVisible.value = true; }

onMounted(() => {
  const id = Number(route.query.id);
  if (id) loadSingle(id); else load(true);
});
</script>

<style scoped>
.micro-page { max-width: 760px; margin: 0 auto; padding: 20px 16px 40px; }
.micro-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 18px; }
.micro-head h2 { margin: 0; font-size: 1.25rem; }
.micro-head-actions { display: flex; gap: 8px; }
.micro-list { display: flex; flex-direction: column; gap: 16px; min-height: 200px; }
.micro-card {
  background: var(--bg-card, #fff); border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 10px; padding: 18px 20px; box-shadow: 0 1px 4px rgba(0,0,0,.04);
}
[data-theme="dark"] .micro-card { background: #1e293b; border-color: #334155; }
.mc-head { display: flex; gap: 10px; align-items: center; margin-bottom: 14px; }
.mc-name { font-weight: 600; font-size: 0.92rem; }
.mc-time { font-size: 0.75rem; color: var(--text-muted, #6b7280); margin-top: 2px; }
.mc-title { font-size: 1.1rem; margin: 0 0 8px; }
.mc-text { font-size: 0.94rem; line-height: 1.8; white-space: pre-wrap; word-break: break-word; }
.mc-text.is-fold { display: -webkit-box; -webkit-line-clamp: 5; -webkit-box-orient: vertical; overflow: hidden; }
.mc-expand { border: none; background: transparent; color: var(--primary-color); cursor: pointer; font-size: 0.82rem; padding: 4px 0; }
.mc-images { display: grid; gap: 6px; margin-top: 12px; border-radius: 8px; overflow: hidden; }
.mc-images img { width: 100%; height: 180px; object-fit: cover; cursor: zoom-in; display: block; }
.mc-images.n1 img { height: 300px; object-fit: contain; background: var(--fill-color, #f5f5f5); }
.mc-images.n2, .mc-images.n4 { grid-template-columns: repeat(2, 1fr); }
.mc-images.n3, .mc-images.n5, .mc-images.n6, .mc-images.n7, .mc-images.n8, .mc-images.n9 { grid-template-columns: repeat(3, 1fr); }
.mc-tags { margin-top: 10px; display: flex; gap: 8px; color: var(--primary-color); font-size: 0.82rem; }
.mc-actions { display: flex; gap: 22px; margin-top: 14px; padding-top: 12px; border-top: 1px solid var(--fill-color, #f3f4f6); }
.mc-actions button { border: none; background: transparent; color: var(--text-muted, #6b7280); cursor: pointer; font-size: 0.85rem; padding: 2px; }
.mc-actions button.active { color: #ff4d6d; }
.mc-comments { margin-top: 12px; padding: 12px; background: var(--fill-color, #f8f9fa); border-radius: 8px; }
.mcc-row { display: flex; gap: 8px; padding: 6px 0; font-size: 0.86rem; }
.mcc-body { flex: 1; word-break: break-word; }
.mcc-body button { border: none; background: transparent; color: var(--text-muted, #6b7280); cursor: pointer; margin-left: 8px; }
.mcc-input { display: flex; gap: 8px; margin-top: 8px; }
.micro-more { text-align: center; padding: 12px; }
@media (max-width: 768px) {
  .micro-page { padding: 14px 10px 30px; }
  .micro-card { padding: 14px; }
  .mc-images img { height: 130px; }
  .mc-images.n1 img { height: 220px; }
}
</style>
