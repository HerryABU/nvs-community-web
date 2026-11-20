<template>
  <div v-loading="loading" class="vdetail">
    <template v-if="post">
      <div class="vd-main">
        <!-- 从发现页点进来，永远能回到唯一的聚合入口 -->
        <router-link to="/discover" class="vd-back">← 返回发现</router-link>

        <!-- 播放区：视频黑底播放器；音乐/图文走各自的呈现；纯文字微文不占黑块 -->
        <div
          v-if="post.kind === 'video' || post.kind === 'music' || (post.images?.length || 0) > 0"
          class="vd-stage"
          :class="{ 'is-video': post.kind === 'video' }"
        >
          <video
            v-if="post.kind === 'video'"
            ref="player"
            :src="mediaUrl(post.video_url)"
            :poster="mediaUrl(post.cover_url)"
            controls
            playsinline
            preload="metadata"
            @loadedmetadata="onMeta"
          />
          <div v-else-if="post.kind === 'music'" class="vd-music">
            <img v-if="post.cover_url" :src="mediaUrl(post.cover_url)" alt="封面" />
            <div v-else class="vd-music-empty">♫<small>{{ post.title || '无封面音频' }}</small></div>
            <audio :src="mediaUrl(post.audio_url)" controls />
          </div>
          <div v-else-if="post.images?.length" class="vd-images">
            <el-image
              v-for="(im, i) in post.images"
              :key="i"
              :src="mediaUrl(im)"
              :preview-src-list="previewList"
              :initial-index="i"
              preview-teleported
              fit="cover"
            />
          </div>
        </div>

        <h1 class="vd-title">{{ post.title || '未命名内容' }}</h1>
        <div class="vd-meta">
          <span class="vd-kind">{{ kindLabel(displayKind) }}</span>
          <span>·</span>
          <span>{{ formatCount(post.view_count) }} 次浏览</span>
          <span>·</span>
          <span>{{ formatWhen(post.created_at) }}</span>
          <span v-if="post.duration">·</span>
          <span v-if="post.duration">{{ formatDuration(post.duration) }}</span>
        </div>

        <div class="vd-authorbar">
          <el-avatar :size="40" :src="mediaUrl(post.avatar_url)">{{ (post.author_name || '?')[0] }}</el-avatar>
          <div class="vd-author-info">
            <b>{{ post.author_name || '用户' + post.user_id }}</b>
            <small>{{ authorPosts.length }} 个作品</small>
          </div>
          <div class="vd-ops">
            <el-button size="small" :type="post.liked ? 'danger' : 'default'" @click="onLike">
              ♥ {{ formatCount(post.like_count) }}
            </el-button>
            <el-button size="small" :type="post.favorited ? 'warning' : 'default'" @click="onFavorite">
              ★ {{ formatCount(post.favorite_count) }}
            </el-button>
            <el-button size="small" @click="onShare">↗ 分享</el-button>
            <el-button v-if="isMine" size="small" @click="$router.push('/feed/compose?id=' + post.id)">编辑</el-button>
          </div>
        </div>

        <div class="vd-desc" :class="{ collapsed: !descOpen && longDesc }">
          <div class="vd-desc-body cherry-markdown cherry-previewer" v-html="descHtml" />
          <button v-if="longDesc" class="vd-more" @click="descOpen = !descOpen">
            {{ descOpen ? '收起' : '展开' }}
          </button>
        </div>

        <div v-if="tags.length" class="vd-tags">
          <span v-for="t in tags" :key="t" class="vd-tag">#{{ t }}</span>
        </div>

        <section class="vd-comments">
          <h2>{{ post.comment_count }} 条评论</h2>
          <div class="vd-input">
            <el-input v-model="commentInput" placeholder="发一条友善的评论…" @keyup.enter="submitComment" />
            <el-button type="primary" @click="submitComment">发送</el-button>
          </div>
          <div v-for="c in comments" :key="c.id" class="vd-comment">
            <el-avatar :size="30" :src="mediaUrl(c.avatar_url)">{{ (c.author_name || '?')[0] }}</el-avatar>
            <div>
              <b>{{ c.author_name || '用户' + c.user_id }}</b>
              <p>{{ c.content }}</p>
              <small>{{ formatWhen(c.created_at) }}</small>
            </div>
          </div>
          <el-empty v-if="comments.length === 0" description="还没有评论，来抢沙发" :image-size="60" />
        </section>
      </div>

      <!-- 右栏：同作者/同类推荐（B 站式），移动端沉到底部 -->
      <aside class="vd-rail">
        <h3>相关推荐</h3>
        <div v-for="r in rail" :key="r.id" class="vd-rail-item" @click="$router.push(postPath(r))">
          <div class="vd-rail-cover">
            <img v-if="r.cover_url || r.images?.length" :src="mediaUrl(r.cover_url || r.images?.[0])" :alt="r.title" loading="lazy" />
            <span v-else class="vd-rail-ph">{{ r.kind === 'music' ? '🎵' : r.kind === 'video' ? '🎬' : '✍️' }}</span>
            <span v-if="r.duration" class="vd-rail-dur">{{ formatDuration(r.duration) }}</span>
          </div>
          <div class="vd-rail-info">
            <p>{{ r.title || kindLabel(r.kind) }}</p>
            <small>{{ r.author_name || '用户' + r.user_id }} · {{ formatCount(r.view_count) }} 次浏览</small>
          </div>
        </div>
        <el-empty v-if="rail.length === 0" description="暂无推荐" :image-size="50" />
      </aside>
    </template>

    <el-empty v-else-if="!loading" description="内容不存在或已下架" :image-size="80" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { feedApi, type FeedPost, type FeedComment } from '@/api/feed';
import { mediaUrl } from '@/utils/prefix';
import { useAuthStore } from '@/stores/auth';
import { formatCount, formatDuration, formatWhen } from '@/utils/feedFormat';
import { postPath, POST_KIND_LABEL } from '@/utils/feedRoute';

function kindLabel(kind?: string) {
  return POST_KIND_LABEL[kind || 'micro'] || '内容';
}

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();

const post = ref<FeedPost | null>(null);
const loading = ref(false);
const comments = ref<FeedComment[]>([]);
const commentInput = ref('');
const descHtml = ref('');
const descOpen = ref(false);
const rail = ref<FeedPost[]>([]);
const authorPosts = ref<FeedPost[]>([]);
const player = ref<HTMLVideoElement | null>(null);

const myId = computed(() => auth.user?.id || 0);
const isMine = computed(() => !!post.value && post.value.user_id === myId.value);
const longDesc = computed(() => (post.value?.content || '').length > 220);
// 底层 kind 只有 video/micro/music；带图内容走 /image/:id，标签也应显示「图片」
const displayKind = computed(() => {
  if (route.name === 'image-detail') return 'image';
  if (post.value?.kind === 'micro' && (post.value.images?.length || 0) > 0) return 'image';
  return post.value?.kind || 'micro';
});
const previewList = computed(() => (post.value?.images || []).map((u) => mediaUrl(u)));
const tags = computed(() =>
  (post.value?.tags || '')
    .split(/[,，]/)
    .map((s) => s.trim())
    .filter(Boolean)
);

function onMeta() {
  // 视频没存时长时，用播放器解析出来的补一次展示（不写库，避免每看一次都打接口）
  const v = player.value;
  if (v && post.value && !post.value.duration && isFinite(v.duration)) {
    post.value.duration = Math.round(v.duration);
  }
}

async function load(id: number) {
  loading.value = true;
  post.value = null;
  comments.value = [];
  rail.value = [];
  descOpen.value = false;
  try {
    const r = await feedApi.get(id);
    post.value = r.data?.data || null;
    if (!post.value) return;
    if (post.value.content) {
      const { renderMarkdown } = await import('@/markdown/renderer');
      descHtml.value = renderMarkdown(post.value.content);
    } else {
      descHtml.value = '';
    }
    await Promise.all([loadComments(id), loadRail(id), loadAuthorPosts(post.value.user_id)]);
  } catch {
    post.value = null;
  } finally {
    loading.value = false;
  }
}

async function loadComments(id: number) {
  try {
    const r = await feedApi.comments(id);
    comments.value = r.data?.data?.list || [];
  } catch {
    comments.value = [];
  }
}

async function loadRail(id: number) {
  try {
    const r = await feedApi.list({ page: 1, page_size: 10 });
    const list = r.data?.data?.list || [];
    rail.value = list.filter((x) => x.id !== id).slice(0, 8);
  } catch {
    rail.value = [];
  }
}

async function loadAuthorPosts(uid: number) {
  try {
    const r = await feedApi.list({ author_id: uid, page: 1, page_size: 1 });
    authorPosts.value = r.data?.data?.list || [];
  } catch {
    authorPosts.value = [];
  }
}

async function onLike() {
  if (!post.value) return;
  if (!myId.value) return ElMessage.warning('请先登录');
  const r = await feedApi.like(post.value.id);
  post.value.liked = !!r.data?.data?.liked;
  post.value.like_count += post.value.liked ? 1 : -1;
}

async function onFavorite() {
  if (!post.value) return;
  if (!myId.value) return ElMessage.warning('请先登录');
  const r = await feedApi.favorite(post.value.id);
  post.value.favorited = !!r.data?.data?.favorited;
  post.value.favorite_count += post.value.favorited ? 1 : -1;
}

async function onShare() {
  if (!post.value) return;
  try {
    await feedApi.share(post.value.id);
    post.value.share_count++;
    await navigator.clipboard?.writeText(location.href);
    ElMessage.success('链接已复制');
  } catch {
    ElMessage.success('已记录分享');
  }
}

async function submitComment() {
  if (!post.value || !commentInput.value.trim()) return;
  if (!myId.value) return ElMessage.warning('请先登录');
  await feedApi.addComment(post.value.id, commentInput.value.trim());
  commentInput.value = '';
  post.value.comment_count++;
  await loadComments(post.value.id);
}

watch(
  () => route.params.id,
  (v) => {
    const id = Number(v);
    if (id) load(id);
  },
  { immediate: true }
);
</script>

<style scoped>
.vdetail { max-width: 1280px; margin: 0 auto; padding: 20px 18px 48px; display: grid; grid-template-columns: minmax(0, 1fr) 330px; gap: 26px; }
.vd-main { min-width: 0; }
.vd-back { display: inline-block; margin-bottom: 14px; font-size: 0.85rem; color: var(--text-muted, #6b7280); text-decoration: none; }
.vd-back:hover { color: var(--primary-color, #5b6ef5); }
.vd-kind { color: var(--primary-color, #5b6ef5); }
.vd-stage { background: #000; border-radius: 10px; overflow: hidden; }
.vd-stage.is-video { aspect-ratio: 16 / 9; display: flex; align-items: center; justify-content: center; }
.vd-stage video { width: 100%; height: 100%; object-fit: contain; background: #000; display: block; }
.vd-music { background: var(--bg-card, #fff); padding: 18px; display: flex; flex-direction: column; gap: 12px; align-items: center; }
.vd-music img { max-width: 300px; max-height: 300px; border-radius: 10px; object-fit: cover; }
.vd-music audio { width: 100%; max-width: 520px; }
.vd-music-empty {
  width: 240px; height: 150px; border-radius: 10px; display: flex; flex-direction: column;
  align-items: center; justify-content: center; gap: 6px; font-size: 2rem; color: #fff;
  background: linear-gradient(135deg, #a78bfa, #f472b6);
}
.vd-music-empty small { font-size: 0.78rem; max-width: 90%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.vd-images { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; padding: 10px; background: var(--bg-card, #fff); }
.vd-images :deep(.el-image) { width: 100%; aspect-ratio: 1; border-radius: 8px; overflow: hidden; cursor: zoom-in; }

.vd-title { margin: 14px 0 8px; font-size: 1.3rem; line-height: 1.4; }
.vd-meta { display: flex; gap: 8px; align-items: center; font-size: 0.8rem; color: var(--text-muted, #6b7280); margin-bottom: 12px; }
.vd-authorbar { display: flex; align-items: center; gap: 10px; padding: 12px 0; border-bottom: 1px solid var(--border-color); }
.vd-author-info { display: flex; flex-direction: column; font-size: 0.88rem; }
.vd-author-info small { color: var(--text-muted, #6b7280); font-size: 0.76rem; }
.vd-ops { margin-left: auto; display: flex; gap: 8px; flex-wrap: wrap; }
.vd-desc { position: relative; margin: 14px 0; font-size: 0.9rem; line-height: 1.8; }
.vd-desc.collapsed { max-height: 120px; overflow: hidden; }
.vd-desc.collapsed::after {
  content: ''; position: absolute; left: 0; right: 0; bottom: 0; height: 48px;
  background: linear-gradient(to bottom, transparent, var(--bg-color, #fff));
}
.vd-more { border: none; background: transparent; color: var(--primary-color, #5b6ef5); cursor: pointer; font-size: 0.84rem; padding: 4px 0; }
.vd-tags { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 8px; }
.vd-tag { font-size: 0.8rem; color: var(--primary-color, #5b6ef5); }
.vd-comments { margin-top: 18px; border-top: 1px solid var(--border-color); padding-top: 14px; }
.vd-comments h2 { font-size: 1.02rem; margin: 0 0 12px; }
.vd-input { display: flex; gap: 8px; margin-bottom: 16px; }
.vd-comment { display: flex; gap: 10px; padding: 9px 0; font-size: 0.88rem; }
.vd-comment b { color: var(--primary-color, #5b6ef5); font-size: 0.84rem; }
.vd-comment p { margin: 3px 0; line-height: 1.65; }
.vd-comment small { color: var(--text-muted, #9ca3af); font-size: 0.74rem; }

.vd-rail h3 { font-size: 0.98rem; margin: 2px 0 12px; }
.vd-rail-item { display: flex; gap: 10px; margin-bottom: 12px; cursor: pointer; }
.vd-rail-cover {
  position: relative; width: 132px; flex: 0 0 132px; aspect-ratio: 16 / 9; border-radius: 8px;
  overflow: hidden; background: #e5e7eb; display: flex; align-items: center; justify-content: center;
}
.vd-rail-cover img { width: 100%; height: 100%; object-fit: cover; }
.vd-rail-ph { font-size: 1.3rem; opacity: 0.6; }
.vd-rail-dur { position: absolute; right: 4px; bottom: 4px; background: rgba(0, 0, 0, 0.7); color: #fff; font-size: 0.68rem; padding: 0 4px; border-radius: 3px; }
.vd-rail-info { min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.vd-rail-info p {
  margin: 0; font-size: 0.84rem; line-height: 1.4; display: -webkit-box;
  -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.vd-rail-info small { color: var(--text-muted, #6b7280); font-size: 0.74rem; }
@media (max-width: 1024px) {
  .vdetail { grid-template-columns: 1fr; }
  .vd-rail { border-top: 1px solid var(--border-color); padding-top: 16px; }
}
@media (max-width: 768px) {
  .vdetail { padding: 12px 10px 32px; gap: 18px; }
  .vd-stage { border-radius: 0; margin: 0 -10px; }
  .vd-images { grid-template-columns: repeat(2, 1fr); }
  .vd-ops { margin-left: 0; width: 100%; }
  .vd-authorbar { flex-wrap: wrap; }
}
</style>
