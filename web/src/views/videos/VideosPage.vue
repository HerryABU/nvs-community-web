<template>
  <div class="videos-page" :class="{ 'is-swipe': mode === 'swipe' }">
    <!-- 竖滑模式：整屏沉浸，导航栏以下全占 -->
    <div v-if="mode === 'swipe'" class="vp-swipe">
      <SwipeFeed />
      <button class="vp-switch" @click="setMode('grid')">▦ 切到网格</button>
    </div>

    <template v-else>
      <header class="vp-head">
        <div class="vp-title">
          <h1>视频</h1>
          <p>横屏作品按频道浏览，移动端默认竖滑</p>
        </div>
        <div class="vp-actions">
          <el-radio-group v-model="sort" size="small" @change="load(true)">
            <el-radio-button value="">最新</el-radio-button>
            <el-radio-button value="hot">最热</el-radio-button>
          </el-radio-group>
          <el-button size="small" @click="setMode('swipe')">▤ 竖滑模式</el-button>
          <el-button size="small" type="primary" @click="$router.push('/feed/compose?kind=video')">
            ＋ 发布视频
          </el-button>
        </div>
      </header>

      <div v-loading="loading" class="vp-grid">
        <VideoCard v-for="p in posts" :key="p.id" :post="p" />
      </div>

      <el-empty v-if="!loading && posts.length === 0" description="还没有视频，去发布第一条吧" :image-size="80" />
      <div v-if="hasMore && !loading" class="vp-more">
        <el-button size="small" @click="loadMore">加载更多</el-button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { feedApi, type FeedPost } from '@/api/feed';
import VideoCard from '@/components/feed/VideoCard.vue';
import SwipeFeed from '@/components/feed/SwipeFeed.vue';

const mode = ref<'grid' | 'swipe'>('grid');
const sort = ref('');
const posts = ref<FeedPost[]>([]);
const loading = ref(false);
const hasMore = ref(true);
const page = ref(1);

function setMode(m: 'grid' | 'swipe') {
  mode.value = m;
  if (m === 'grid' && posts.value.length === 0) load(true);
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
    const r = await feedApi.list({
      kind: 'video',
      sort: sort.value || undefined,
      page: page.value,
      page_size: 12,
    });
    const list = r.data?.data?.list || [];
    posts.value = reset ? list : [...posts.value, ...list];
    hasMore.value = list.length >= 12;
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

onMounted(() => {
  // 移动端默认竖滑（参考 B 站/视频号移动端：进来就是一条一条刷）
  if (window.matchMedia('(max-width: 767px)').matches) {
    mode.value = 'swipe';
    return;
  }
  load(true);
});
</script>

<style scoped>
.videos-page { max-width: 1240px; margin: 0 auto; padding: 20px 18px 48px; }
.videos-page.is-swipe { max-width: none; padding: 0; height: calc(100vh - 60px); background: #000; }
.vp-swipe { position: relative; height: 100%; }
.vp-switch {
  position: absolute; right: 14px; top: 12px; z-index: 3;
  background: rgba(255, 255, 255, 0.16); color: #fff; border: 1px solid rgba(255, 255, 255, 0.35);
  border-radius: 14px; padding: 5px 12px; font-size: 0.78rem; cursor: pointer;
  backdrop-filter: blur(4px);
}
.vp-head { display: flex; justify-content: space-between; align-items: flex-end; gap: 12px; margin-bottom: 18px; flex-wrap: wrap; }
.vp-title h1 { margin: 0; font-size: 1.45rem; }
.vp-title p { margin: 4px 0 0; font-size: 0.8rem; color: var(--text-muted, #6b7280); }
.vp-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.vp-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(248px, 1fr));
  gap: 22px 16px; min-height: 200px;
}
.vp-more { text-align: center; padding: 18px; }
@media (max-width: 768px) {
  .videos-page { padding: 12px 10px 32px; }
  .vp-grid { grid-template-columns: repeat(2, 1fr); gap: 16px 10px; }
  .videos-page.is-swipe { padding: 0; }
}
</style>
