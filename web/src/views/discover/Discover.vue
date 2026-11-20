<template>
  <div class="discover-page">
    <div class="discover-head">
      <div>
        <h2>发现</h2>
        <p class="discover-sub">图文 · 短视频 · 音乐，一起看</p>
      </div>
      <div class="discover-actions">
        <el-radio-group v-model="scope" size="small" @change="load(true)">
          <el-radio-button value="all">推荐</el-radio-button>
          <el-radio-button value="following">关注</el-radio-button>
          <el-radio-button value="mine">我的</el-radio-button>
        </el-radio-group>
        <el-button size="small" type="primary" @click="$router.push('/feed/compose')">＋ 发布</el-button>
      </div>
    </div>

    <div class="discover-chips">
      <button v-for="f in filters" :key="f.key" :class="{ active: filter === f.key }" @click="setFilter(f.key)">
        {{ f.label }}
      </button>
      <button v-if="tag" class="tag-chip active" @click="clearTag">#{{ tag }} <span class="tag-x">×</span></button>
    </div>

    <div v-loading="loading" class="discover-grid">
      <article v-for="p in posts" :key="p.id" class="ds-card" @click="openPost(p)">
        <div class="ds-cover">
          <img v-if="coverOf(p)" :src="mediaUrl(coverOf(p))" :alt="p.title || '封面'" loading="lazy" />
          <div v-else class="ds-cover-empty" :class="p.kind">{{ p.kind === 'music' ? '🎵' : p.kind === 'video' ? '🎬' : '✍️' }}</div>
          <span v-if="p.kind === 'video'" class="ds-badge play">▶ 视频</span>
          <span v-else-if="p.kind === 'music'" class="ds-badge music">♫ 音乐</span>
          <span v-else-if="p.images?.length > 1" class="ds-badge">1/{{ p.images.length }}</span>
        </div>
        <div class="ds-body">
          <h3 class="ds-title">{{ p.title || (p.content || '').slice(0, 40) || '未命名' }}</h3>
          <p class="ds-excerpt">{{ (p.content || '').replace(/[#*`>\[\]]/g, '').slice(0, 70) }}</p>
          <div v-if="tagList(p).length" class="ds-tags">
            <button v-for="t in tagList(p)" :key="t" @click.stop="openTag(t)">#{{ t }}</button>
          </div>
          <div class="ds-foot">
            <span class="ds-author">
              <el-avatar :size="20" :src="mediaUrl(p.avatar_url)">{{ (p.author_name || '?')[0] }}</el-avatar>
              {{ p.author_name || '用户' + p.user_id }}
            </span>
            <span class="ds-ops">
              <button :class="{ active: p.liked }" @click.stop="onLike(p)">♥ {{ p.like_count }}</button>
              <button @click.stop="openPost(p)">💬 {{ p.comment_count }}</button>
            </span>
          </div>
        </div>
      </article>
    </div>

    <el-empty v-if="!loading && posts.length === 0" :description="emptyText" :image-size="70" />
    <div v-if="hasMore && !loading" class="discover-more"><el-button size="small" @click="loadMore">加载更多</el-button></div>

  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { feedApi, type FeedPost } from '@/api/feed';
import { mediaUrl } from '@/utils/prefix';
import { useAuthStore } from '@/stores/auth';
import { postPath } from '@/utils/feedRoute';

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const myId = computed(() => auth.user?.id || 0);

const filters = [
  { key: 'all', label: '全部' },
  { key: 'micro', label: '图文' },
  { key: 'video', label: '短视频' },
  { key: 'music', label: '音乐' },
];
const KINDS = ['all', 'micro', 'video', 'music'];
const filter = ref('all');
const scope = ref('all');
const posts = ref<FeedPost[]>([]);
const loading = ref(false);
const page = ref(1);
const hasMore = ref(true);
const emptyText = computed(() => {
  const hit = filters.find((f) => f.key === filter.value);
  return filter.value === 'all' ? '还没有内容，去发布第一条吧' : `还没有${hit?.label || ''}内容`;
});

const tag = ref('');

function discPath(t = tag.value) {
  return t ? `/tag/${encodeURIComponent(t)}` : '/discover';
}
/** 筛选落进 URL：刷新、分享、从旧 /shorts、/micro 跳过来都停在同一个视图 */
function setFilter(k: string) {
  const next = k === 'all' ? undefined : k;
  const q: Record<string, any> = { ...route.query };
  if (next) q.kind = next; else delete q.kind;
  delete q.id;
  delete q.tag;
  router.replace({ path: discPath(), query: q });
}
function applyQuery() {
  const k = String(route.query.kind || 'all');
  filter.value = KINDS.includes(k) ? k : 'all';
  tag.value = String(route.params.tag || route.query.tag || '').trim();
}
/** 标签点击进入独立的 /tag/:tag 路由，服务端按倒排索引精确筛选 */
function openTag(t: string) {
  const q: Record<string, any> = { ...route.query };
  delete q.kind;
  delete q.id;
  delete q.tag;
  router.push({ path: discPath(t), query: q });
}
function clearTag() {
  const q: Record<string, any> = { ...route.query };
  delete q.tag;
  delete q.id;
  router.replace({ path: '/discover', query: q });
}
function tagList(p: FeedPost) {
  return (p.tags || '').split(/[,，]/).map((s) => s.trim()).filter(Boolean).slice(0, 3);
}
/** 点卡片走真实路由，不叠弹窗层；同一条内容只有一个规范 URL */
function openPost(p: FeedPost) {
  router.push(postPath(p));
}

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
    const r = await feedApi.list({
      scope: scope.value,
      kind: filter.value === 'all' ? undefined : filter.value,
      tag: tag.value || undefined,
      page: page.value,
      page_size: 12,
    });
    const list = r.data?.data?.list || [];
    posts.value = reset ? list : [...posts.value, ...list];
    hasMore.value = list.length >= 12;
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '加载失败'); }
  finally { loading.value = false; }
}
function loadMore() { page.value++; load(); }

async function onLike(p: FeedPost) {
  if (!myId.value) { ElMessage.warning('请先登录'); return; }
  try {
    const r = await feedApi.like(p.id);
    p.liked = r.data?.data?.liked;
    p.like_count += p.liked ? 1 : -1;
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '操作失败'); }
}

// 老分享链接 /discover?id=123 兼容：查一次类型后 301 到规范的详情路由
watch(() => route.query.id, async (v) => {
  const id = Number(v);
  if (!id) return;
  try {
    const r = await feedApi.get(id);
    const p = r.data?.data;
    if (p) await router.replace(postPath(p));
  } catch { /* ignore */ }
}, { immediate: true });

// 标签路由 /tag/:tag 与 ?kind= 都在这里收敛，改 URL 即重新拉服务端数据
watch(
  () => [route.params.tag, route.query.tag, route.query.kind] as const,
  () => {
    applyQuery();
    if (!route.query.id) load(true);
  },
  { immediate: true }
);
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
/* 封面是可选字段：音频/无封面视频用类型渐变兜底，别让卡片看着像加载失败 */
.ds-cover-empty.music { background: linear-gradient(135deg, #fce7f3, #ede9fe); }
.ds-cover-empty.video { background: linear-gradient(135deg, #e0e7ff, #dbeafe); }
.ds-cover-empty.micro { background: linear-gradient(135deg, #f1f5f9, #e2e8f0); }
[data-theme="dark"] .ds-cover-empty.music { background: linear-gradient(135deg, #4c1d3d, #312e5f); }
[data-theme="dark"] .ds-cover-empty.video { background: linear-gradient(135deg, #1e2a5a, #1e3a5f); }
[data-theme="dark"] .ds-cover-empty.micro { background: linear-gradient(135deg, #1e293b, #273244); }
.ds-badge { position: absolute; left: 10px; top: 10px; background: rgba(0,0,0,.6); color: #fff; font-size: .7rem; padding: 3px 8px; border-radius: 10px; }
.ds-badge.play { background: rgba(91,110,245,.85); }
.ds-badge.music { background: rgba(236,72,153,.85); }
.ds-body { padding: 13px 15px 11px; }
.ds-title { margin: 0 0 6px; font-size: .98rem; line-height: 1.45; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.ds-excerpt { margin: 0; font-size: .82rem; color: var(--text-muted, #6b7280); line-height: 1.6; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.ds-tags { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
.ds-tags button, .tag-chip {
  border: 1px solid var(--border-color, #e5e7eb); background: var(--fill-color, #f3f4f6);
  color: var(--primary-color, #5b6ef5); border-radius: 12px; padding: 2px 8px;
  font-size: .74rem; cursor: pointer;
}
.ds-tags button:hover, .tag-chip:hover { border-color: var(--primary-color, #5b6ef5); }
.tag-chip.active { background: var(--primary-color, #5b6ef5); border-color: var(--primary-color); color: #fff; }
.tag-x { margin-left: 4px; font-size: .9rem; }
.ds-foot { display: flex; justify-content: space-between; align-items: center; margin-top: 11px; padding-top: 9px; border-top: 1px solid var(--fill-color, #f3f4f6); }
.ds-author { display: flex; align-items: center; gap: 6px; font-size: .76rem; color: var(--text-muted, #6b7280); }
.ds-ops { display: flex; gap: 9px; }
.ds-ops button { border: none; background: transparent; color: var(--text-muted, #6b7280); cursor: pointer; font-size: .78rem; padding: 0; }
.ds-ops button.active { color: #ff4d6d; }
.discover-more { text-align: center; padding: 14px; }
[data-theme="dark"] .ds-card { background: #1e293b; border-color: #334155; }

@media (max-width: 768px) {
  .discover-page { padding: 14px 10px 30px; }
  .discover-grid { grid-template-columns: repeat(2, 1fr); gap: 10px; }
  .discover-head { flex-direction: column; align-items: flex-start; gap: 10px; }
}
</style>
