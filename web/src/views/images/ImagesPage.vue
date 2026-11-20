<template>
  <div class="images-page">
    <header class="ip-head">
      <div>
        <h1>图片</h1>
        <p>配图作品单独成频道，点开看大图与全文</p>
      </div>
      <div class="ip-actions">
        <el-radio-group v-model="cols" size="small">
          <el-radio-button :value="3">三列</el-radio-button>
          <el-radio-button :value="4">四列</el-radio-button>
        </el-radio-group>
        <el-button size="small" type="primary" @click="$router.push('/feed/compose?kind=micro')">
          ＋ 发布图片
        </el-button>
      </div>
    </header>

    <div v-loading="loading" class="ip-grid" :style="{ '--cols': cols }">
      <figure v-for="p in posts" :key="p.id" class="ip-card" @click="open(p)">
        <div class="ip-cover">
          <img :src="mediaUrl(p.images?.[0])" :alt="p.title || '图片'" loading="lazy" />
          <span v-if="(p.images?.length || 0) > 1" class="ip-count">1/{{ p.images?.length }}</span>
        </div>
        <figcaption>
          <h3>{{ p.title || excerpt(p.content) || '未命名' }}</h3>
          <div class="ip-meta">
            <el-avatar :size="18" :src="mediaUrl(p.avatar_url)">{{ (p.author_name || '?')[0] }}</el-avatar>
            <span>{{ p.author_name || '用户' + p.user_id }}</span>
            <span class="dot">·</span>
            <span>♥ {{ formatCount(p.like_count) }}</span>
          </div>
        </figcaption>
      </figure>
    </div>

    <el-empty v-if="!loading && posts.length === 0" description="还没有图片作品" :image-size="80" />
    <div v-if="hasMore && !loading" class="ip-more">
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
import { formatCount } from '@/utils/feedFormat';
import PostDetailDialog from '@/components/feed/PostDetailDialog.vue';

const posts = ref<FeedPost[]>([]);
const loading = ref(false);
const hasMore = ref(true);
const page = ref(1);
const cols = ref(4);
const dialog = ref(false);
const current = ref<FeedPost | null>(null);

function excerpt(s?: string) {
  return (s || '').replace(/[#*`>\[\]]/g, '').trim().slice(0, 30);
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
    const r = await feedApi.list({ has_images: 1, page: page.value, page_size: 18 });
    const list = r.data?.data?.list || [];
    posts.value = reset ? list : [...posts.value, ...list];
    hasMore.value = list.length >= 18;
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
.images-page { max-width: 1240px; margin: 0 auto; padding: 20px 18px 48px; }
.ip-head { display: flex; justify-content: space-between; align-items: flex-end; gap: 12px; margin-bottom: 18px; flex-wrap: wrap; }
.ip-head h1 { margin: 0; font-size: 1.45rem; }
.ip-head p { margin: 4px 0 0; font-size: 0.8rem; color: var(--text-muted, #6b7280); }
.ip-actions { display: flex; align-items: center; gap: 8px; }
.ip-grid { display: grid; grid-template-columns: repeat(var(--cols, 4), 1fr); gap: 18px 14px; min-height: 180px; }
.ip-card { margin: 0; cursor: pointer; }
.ip-cover { position: relative; aspect-ratio: 1; border-radius: 10px; overflow: hidden; background: #e5e7eb; }
.ip-cover img { width: 100%; height: 100%; object-fit: cover; transition: transform 0.25s; }
.ip-card:hover .ip-cover img { transform: scale(1.05); }
.ip-count { position: absolute; right: 8px; bottom: 8px; background: rgba(0, 0, 0, 0.6); color: #fff; font-size: 0.7rem; padding: 1px 6px; border-radius: 4px; }
.ip-card h3 {
  margin: 8px 0 4px; font-size: 0.88rem; font-weight: 500; line-height: 1.45;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.ip-meta { display: flex; align-items: center; gap: 5px; font-size: 0.74rem; color: var(--text-muted, #6b7280); }
.ip-meta .dot { opacity: 0.6; }
.ip-more { text-align: center; padding: 18px; }
@media (max-width: 1024px) { .ip-grid { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 768px) { .ip-grid { grid-template-columns: repeat(2, 1fr); gap: 14px 10px; } .images-page { padding: 12px 10px 32px; } }
</style>
