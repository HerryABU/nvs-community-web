<template>
  <div class="micro-page">
    <header class="mp-head">
      <div>
        <h1>微文</h1>
        <p>文字 + 贴图的短文频道，独立于视频与图片流</p>
      </div>
      <el-button size="small" type="primary" @click="$router.push('/feed/compose?kind=micro')">＋ 写微文</el-button>
    </header>

    <div v-loading="loading" class="mp-list">
      <article v-for="p in posts" :key="p.id" class="mp-card" @click="open(p)">
        <div class="mp-body" :class="{ 'no-cover': !p.images?.length }">
          <div class="mp-text">
            <h2>{{ p.title || '无标题' }}</h2>
            <p class="mp-excerpt">{{ excerpt(p.content) }}</p>
            <div class="mp-foot">
              <el-avatar :size="20" :src="mediaUrl(p.avatar_url)">{{ (p.author_name || '?')[0] }}</el-avatar>
              <span>{{ p.author_name || '用户' + p.user_id }}</span>
              <span class="dot">·</span>
              <span>{{ formatWhen(p.created_at) }}</span>
              <span class="dot">·</span>
              <span>♥ {{ formatCount(p.like_count) }}</span>
              <span class="dot">·</span>
              <span>💬 {{ formatCount(p.comment_count) }}</span>
            </div>
          </div>
          <div v-if="p.images?.length" class="mp-thumbs">
            <img
              v-for="(im, i) in p.images.slice(0, 3)"
              :key="i"
              :src="mediaUrl(im)"
              :alt="`配图 ${i + 1}`"
              loading="lazy"
            />
            <span v-if="p.images.length > 3" class="mp-moreimgs">+{{ p.images.length - 3 }}</span>
          </div>
        </div>
      </article>
    </div>

    <el-empty v-if="!loading && posts.length === 0" description="还没有微文，去写一篇吧" :image-size="80" />
    <div v-if="hasMore && !loading" class="mp-more">
      <el-button size="small" @click="loadMore">加载更多</el-button>
    </div>

    <PostDetailDialog v-model="dialog" :post="current" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { feedApi, type FeedPost } from '@/api/feed';
import { mediaUrl } from '@/utils/prefix';
import { formatCount, formatWhen } from '@/utils/feedFormat';
import PostDetailDialog from '@/components/feed/PostDetailDialog.vue';

const posts = ref<FeedPost[]>([]);
const loading = ref(false);
const hasMore = ref(true);
const page = ref(1);
const dialog = ref(false);
const current = ref<FeedPost | null>(null);

// 列表只显示纯文本摘要：真去渲染 Markdown 会把 5MB 的 Cherry 引擎拉进这个页面，
// 而且截断后再渲染容易出现半截标签（详情弹窗里才是完整渲染）。
function excerpt(s?: string) {
  return (s || '')
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/[#>*`~_]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 180);
}

function open(p: FeedPost) {
  current.value = p;
  dialog.value = true;
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
    const r = await feedApi.list({ kind: 'micro', page: page.value, page_size: 10 });
    const list = r.data?.data?.list || [];
    posts.value = reset ? list : [...posts.value, ...list];
    hasMore.value = list.length >= 10;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}
function loadMore() {
  page.value++;
  load();
}
onMounted(() => load(true));
</script>

<style scoped>
.micro-page { max-width: 900px; margin: 0 auto; padding: 20px 18px 48px; }
.mp-head { display: flex; justify-content: space-between; align-items: flex-end; gap: 12px; margin-bottom: 18px; flex-wrap: wrap; }
.mp-head h1 { margin: 0; font-size: 1.45rem; }
.mp-head p { margin: 4px 0 0; font-size: 0.8rem; color: var(--text-muted, #6b7280); }
.mp-list { display: flex; flex-direction: column; gap: 14px; min-height: 160px; }
.mp-card {
  background: var(--bg-card, #fff); border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 12px; padding: 16px 18px; cursor: pointer; transition: box-shadow 0.16s;
}
.mp-card:hover { box-shadow: 0 8px 22px rgba(0, 0, 0, 0.07); }
.mp-body { display: grid; grid-template-columns: minmax(0, 1fr) 240px; gap: 16px; align-items: start; }
.mp-body.no-cover { grid-template-columns: 1fr; }
.mp-text h2 { margin: 0 0 8px; font-size: 1.04rem; line-height: 1.45; }
.mp-excerpt { margin: 0; font-size: 0.88rem; line-height: 1.75; color: var(--text-color); max-height: 96px; overflow: hidden; white-space: pre-wrap; }
.mp-foot { display: flex; align-items: center; gap: 6px; margin-top: 12px; font-size: 0.76rem; color: var(--text-muted, #6b7280); flex-wrap: wrap; }
.mp-foot .dot { opacity: 0.6; }
.mp-thumbs { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; position: relative; }
.mp-thumbs img { width: 100%; aspect-ratio: 1; object-fit: cover; border-radius: 8px; }
.mp-moreimgs {
  position: absolute; right: 6px; bottom: 6px; background: rgba(0, 0, 0, 0.6); color: #fff;
  font-size: 0.72rem; padding: 1px 6px; border-radius: 4px;
}
.mp-more { text-align: center; padding: 18px; }
@media (max-width: 768px) {
  .micro-page { padding: 12px 10px 32px; }
  .mp-body { grid-template-columns: 1fr; }
  .mp-thumbs { grid-template-columns: repeat(3, 1fr); }
}
</style>
