<template>
  <div class="topics-page page-container">
    <!-- ── 话题列表 ── -->
    <template v-if="!topicId">
      <div class="tp-head">
        <h1>🔥 热门话题</h1>
        <p class="tp-sub">讨论进行中的话题，点击进入参与</p>
      </div>
      <div v-loading="loading" class="tp-grid">
        <el-empty v-if="!loading && topics.length === 0" description="还没有话题，写博客时创建第一个话题吧" />
        <div v-for="t in topics" :key="t.id" class="tp-card" @click="$router.push(`/topics/${t.id}`)">
          <div class="tp-card-name"># {{ t.name }}</div>
          <div v-if="t.description" class="tp-card-desc">{{ t.description }}</div>
          <div class="tp-card-count">{{ t.blog_count }} 篇参与</div>
        </div>
      </div>
    </template>

    <!-- ── 话题详情 ── -->
    <template v-else>
      <div class="tp-head">
        <el-button text @click="$router.push('/topics')"><el-icon><ArrowLeft /></el-icon>返回话题列表</el-button>
        <h1># {{ topic?.name }}</h1>
        <p class="tp-sub">{{ topic?.description || '参与讨论：' }} · {{ blogCount }} 篇博客参与</p>
      </div>
      <div v-loading="listLoading" class="tp-blogs" style="min-height: 200px">
        <el-empty v-if="!listLoading && blogs.length === 0" description="该话题下暂无博客" />
        <div v-for="b in blogs" :key="b.id" class="tp-blog" @click="$router.push(`/author/${b.author_id}/blogs/${b.id}`)">
          <div class="tp-blog-main">
            <div class="tp-blog-title">
              <el-tag v-if="b.is_pinned" size="small" type="danger">置顶</el-tag>
              {{ b.title }}
            </div>
            <div class="tp-blog-meta">
              <span class="tp-author">{{ b.author?.nickname || b.author?.username || '作者' }}</span>
              <span v-for="t in b.tags || []" :key="t.id" class="tp-tag">#{{ t.name }}</span>
              <span class="tp-date">{{ formatDate(b.created_at) }}</span>
            </div>
          </div>
          <span class="tp-views">{{ b.view_count }} 阅读</span>
        </div>
      </div>
      <div class="pagination" v-if="total > pageSize">
        <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" @current-change="loadTopicBlogs" />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { blogApi } from '@/api/social';
import { ArrowLeft } from '@element-plus/icons-vue';

const route = useRoute();
const router = useRouter();

const topicId = computed(() => Number(route.params.id) || 0);
const topics = ref<any[]>([]);
const topic = ref<any>(null);
const blogCount = ref(0);
const blogs = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 12;
const loading = ref(false);
const listLoading = ref(false);

function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleDateString('zh-CN'); }

async function loadTopics() {
  loading.value = true;
  try {
    const res = await blogApi.getTopics();
    if (res.data.code === 0) topics.value = res.data.data || [];
  } catch { /* ignore */ }
  finally { loading.value = false; }
}

async function loadTopic() {
  if (!topicId.value) return;
  try {
    const res = await blogApi.getTopic(topicId.value);
    if (res.data.code === 0) {
      topic.value = res.data.data?.topic || null;
      blogCount.value = res.data.data?.blog_count || 0;
    }
  } catch { topic.value = null; }
}

async function loadTopicBlogs() {
  listLoading.value = true;
  try {
    const res = await blogApi.getTopicBlogs(topicId.value, page.value, pageSize);
    if (res.data.code === 0) {
      blogs.value = res.data.data?.list || [];
      total.value = res.data.data?.total || 0;
    }
  } catch { /* ignore */ }
  finally { listLoading.value = false; }
}

watch(topicId, (v) => {
  if (v) { page.value = 1; loadTopic(); loadTopicBlogs(); }
  else loadTopics();
});

onMounted(() => {
  if (topicId.value) { loadTopic(); loadTopicBlogs(); }
  else loadTopics();
});
</script>

<style scoped>
.topics-page { max-width: 960px; margin: 0 auto; padding: 24px; }
.tp-head { margin-bottom: 20px; }
.tp-head h1 { font-size: 1.4rem; margin: 8px 0 4px; }
.tp-sub { color: var(--text-light, #999); font-size: 0.9rem; }
.tp-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 14px; }
.tp-card {
  padding: 16px; border: 1px solid var(--border-color); border-radius: 12px;
  cursor: pointer; transition: all 0.2s; background: var(--bg-card, #fff);
}
.tp-card:hover { transform: translateY(-2px); box-shadow: 0 6px 18px rgba(0,0,0,0.08); border-color: var(--primary-color); }
.tp-card-name { font-weight: 700; color: var(--primary-color); font-size: 1rem; }
.tp-card-desc { font-size: 0.8rem; color: var(--text-light, #999); margin-top: 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tp-card-count { font-size: 0.75rem; color: #b45309; margin-top: 10px; }
.tp-blogs { display: flex; flex-direction: column; gap: 10px; }
.tp-blog {
  display: flex; align-items: center; gap: 12px;
  padding: 14px 16px; border: 1px solid var(--border-color);
  border-radius: 10px; cursor: pointer; transition: all 0.2s;
}
.tp-blog:hover { border-color: var(--primary-color); }
.tp-blog-main { flex: 1; min-width: 0; }
.tp-blog-title { font-weight: 600; }
.tp-blog-meta { display: flex; align-items: center; gap: 8px; margin-top: 4px; flex-wrap: wrap; }
.tp-author { font-size: 0.8rem; color: var(--primary-color); }
.tp-tag { font-size: 0.75rem; color: var(--primary-color); }
.tp-date { font-size: 0.78rem; color: var(--text-light, #999); }
.tp-views { font-size: 0.78rem; color: var(--text-light, #999); flex-shrink: 0; }
</style>
