<template>
  <el-dialog
    :model-value="modelValue"
    width="min(820px, 94vw)"
    top="5vh"
    class="pdd"
    destroy-on-close
    @update:model-value="(v: boolean) => emit('update:modelValue', v)"
  >
    <template v-if="post">
      <h2 class="pdd-title">{{ post.title || '详情' }}</h2>
      <div class="pdd-author">
        <el-avatar :size="36" :src="mediaUrl(post.avatar_url)">{{ (post.author_name || '?')[0] }}</el-avatar>
        <div class="pdd-author-info">
          <b>{{ post.author_name || '用户' + post.user_id }}</b>
          <small>{{ formatWhen(post.created_at) }} · {{ formatCount(post.view_count) }} 次浏览</small>
        </div>
        <el-button v-if="isMine" size="small" text @click="$router.push('/feed/compose?id=' + post.id)">编辑</el-button>
      </div>

      <!-- 视频：独立播放器，点开就是完整播放（不再塞在窄弹窗里） -->
      <div v-if="post.kind === 'video'" class="pdd-video">
        <video
          :src="mediaUrl(post.video_url)"
          :poster="mediaUrl(post.cover_url)"
          controls
          playsinline
          preload="metadata"
        />
      </div>

      <div v-else-if="post.kind === 'music'" class="pdd-music">
        <img v-if="post.cover_url" :src="mediaUrl(post.cover_url)" alt="封面" />
        <div v-else class="pdd-music-empty">♫<small>{{ post.title || '无封面音频' }}</small></div>
        <audio :src="mediaUrl(post.audio_url)" controls />
      </div>

      <div v-else-if="post.images?.length" class="pdd-images">
        <el-image
          v-for="(im, i) in post.images"
          :key="i"
          :src="mediaUrl(im)"
          :preview-src-list="imagePreviewList"
          :initial-index="i"
          preview-teleported
          fit="cover"
          :alt="`图 ${i + 1}`"
        />
      </div>

      <div v-if="post.content" class="pdd-content cherry-markdown cherry-previewer" v-html="html" />

      <div v-if="tags.length" class="pdd-tags">
        <span v-for="t in tags" :key="t" class="pdd-tag">#{{ t }}</span>
      </div>

      <div class="pdd-actions">
        <button :class="{ active: post.liked }" @click="onLike">♥ {{ formatCount(post.like_count) }}</button>
        <button :class="{ active: post.favorited }" @click="onFavorite">★ {{ formatCount(post.favorite_count) }}</button>
        <button @click="onShare">↗ {{ formatCount(post.share_count) }}</button>
      </div>

      <div class="pdd-comments">
        <h3>{{ post.comment_count }} 条评论</h3>
        <div v-for="c in comments" :key="c.id" class="pdd-comment">
          <el-avatar :size="28" :src="mediaUrl(c.avatar_url)">{{ (c.author_name || '?')[0] }}</el-avatar>
          <div>
            <b>{{ c.author_name || '用户' + c.user_id }}</b>
            <span class="pdd-comment-body">{{ c.content }}</span>
            <small>{{ formatWhen(c.created_at) }}</small>
          </div>
        </div>
        <el-empty v-if="comments.length === 0" description="还没有评论" :image-size="40" />
        <div class="pdd-input">
          <el-input v-model="commentInput" size="small" placeholder="说点什么…" @keyup.enter="submitComment" />
          <el-button size="small" type="primary" @click="submitComment">发送</el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { feedApi, type FeedPost, type FeedComment } from '@/api/feed';
import { mediaUrl } from '@/utils/prefix';
import { useAuthStore } from '@/stores/auth';
import { formatCount, formatWhen } from '@/utils/feedFormat';

const props = defineProps<{ modelValue: boolean; post: FeedPost | null }>();
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>();

const router = useRouter();
const auth = useAuthStore();
const myId = computed(() => auth.user?.id || 0);
const isMine = computed(() => !!props.post && props.post.user_id === myId.value);
const imagePreviewList = computed(() => (props.post?.images || []).map((u) => mediaUrl(u)));
const tags = computed(() =>
  (props.post?.tags || '')
    .split(/[,，]/)
    .map((s) => s.trim())
    .filter(Boolean)
);

const html = ref('');
const comments = ref<FeedComment[]>([]);
const commentInput = ref('');

watch(
  () => [props.modelValue, props.post?.id] as const,
  async ([visible, id]) => {
    if (!visible || !id) {
      comments.value = [];
      html.value = '';
      return;
    }
    commentInput.value = '';
    if (props.post?.content) {
      const { renderMarkdown } = await import('@/markdown/renderer');
      html.value = renderMarkdown(props.post.content);
    }
    await loadComments(Number(id));
  },
  { immediate: true }
);

async function loadComments(id: number) {
  try {
    const r = await feedApi.comments(id);
    comments.value = r.data?.data?.list || [];
  } catch {
    comments.value = [];
  }
}

async function onLike() {
  if (!props.post) return;
  if (!myId.value) return ElMessage.warning('请先登录');
  try {
    const r = await feedApi.like(props.post.id);
    props.post.liked = !!r.data?.data?.liked;
    props.post.like_count += props.post.liked ? 1 : -1;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function onFavorite() {
  if (!props.post) return;
  if (!myId.value) return ElMessage.warning('请先登录');
  try {
    const r = await feedApi.favorite(props.post.id);
    props.post.favorited = !!r.data?.data?.favorited;
    props.post.favorite_count += props.post.favorited ? 1 : -1;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function onShare() {
  if (!props.post) return;
  try {
    await feedApi.share(props.post.id);
    props.post.share_count++;
    await navigator.clipboard?.writeText(location.origin + `/video/${props.post.id}`);
    ElMessage.success('链接已复制');
  } catch {
    ElMessage.success('已记录分享');
  }
}

async function submitComment() {
  if (!props.post || !commentInput.value.trim()) return;
  if (!myId.value) return ElMessage.warning('请先登录');
  try {
    await feedApi.addComment(props.post.id, commentInput.value.trim());
    commentInput.value = '';
    props.post.comment_count++;
    await loadComments(props.post.id);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '评论失败');
  }
}

// 供外部（如详情页）复用「跳到完整页」能力
function openFull() {
  if (props.post) router.push(`/video/${props.post.id}`);
}
defineExpose({ openFull });
</script>

<style scoped>
.pdd-title { margin: 0 0 12px; font-size: 1.2rem; line-height: 1.4; }
.pdd-author { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
.pdd-author-info { display: flex; flex-direction: column; font-size: 0.85rem; }
.pdd-author-info small { color: var(--text-muted, #888); }
.pdd-video { background: #000; border-radius: 10px; overflow: hidden; }
.pdd-video video { width: 100%; max-height: 62vh; display: block; background: #000; }
.pdd-music { display: flex; flex-direction: column; gap: 12px; align-items: center; }
.pdd-music img { max-width: 280px; max-height: 280px; border-radius: 10px; object-fit: cover; }
.pdd-music audio { width: 100%; max-width: 460px; }
.pdd-music-empty {
  width: 240px; height: 150px; border-radius: 10px; display: flex; flex-direction: column;
  align-items: center; justify-content: center; gap: 6px; font-size: 2rem; color: #fff;
  background: linear-gradient(135deg, #a78bfa, #f472b6);
}
.pdd-music-empty small { font-size: 0.78rem; max-width: 90%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pdd-images { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin-bottom: 12px; }
.pdd-images :deep(.el-image) { width: 100%; aspect-ratio: 1; border-radius: 8px; overflow: hidden; cursor: zoom-in; }
.pdd-content { line-height: 1.8; font-size: 0.93rem; margin: 12px 0; }
.pdd-tags { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 10px; }
.pdd-tag { font-size: 0.8rem; color: var(--primary-color, #5b6ef5); }
.pdd-actions {
  display: flex; gap: 22px; padding: 12px 0; border-top: 1px solid var(--border-color);
  border-bottom: 1px solid var(--border-color);
}
.pdd-actions button { border: none; background: transparent; color: var(--text-muted, #666); cursor: pointer; font-size: 0.9rem; }
.pdd-actions button.active { color: #ff4d6d; }
.pdd-comments h3 { font-size: 1rem; margin: 14px 0 10px; }
.pdd-comment { display: flex; gap: 8px; padding: 7px 0; font-size: 0.86rem; }
.pdd-comment b { color: var(--primary-color); margin-right: 6px; }
.pdd-comment-body { color: var(--text-color); }
.pdd-comment small { display: block; color: var(--text-muted, #999); font-size: 0.74rem; margin-top: 2px; }
.pdd-input { display: flex; gap: 8px; margin-top: 12px; }
@media (max-width: 768px) {
  .pdd-images { grid-template-columns: repeat(2, 1fr); }
}
</style>
