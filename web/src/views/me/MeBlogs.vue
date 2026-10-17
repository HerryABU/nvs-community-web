<template>
  <div class="me-blogs">
    <h2 class="section-title">作者博客</h2>
    <div class="blog-list" v-if="blogs.length > 0">
      <div v-for="blog in blogs" :key="blog.id" class="blog-item" @click="$router.push(`/author/${authorKey}/blogs/${blog.id}`)">
        <div class="blog-title-row">
          <el-tag v-if="blog.is_pinned" size="small" type="danger" style="margin-right:6px">置顶</el-tag>
          <span class="blog-item-title">{{ blog.title }}</span>
        </div>
        <div class="blog-meta">
          <span>{{ formatDate(blog.created_at) }}</span>
          <span>{{ blog.view_count }} 次阅读</span>
        </div>
        <p class="blog-summary" v-if="blog.summary">{{ blog.summary }}</p>
      </div>
    </div>
    <div v-else style="color:var(--text-muted);font-size:0.9rem;padding:12px 0">暂无博客文章</div>
    <el-button v-if="showMore" text size="small" @click="loadMore">加载更多</el-button>
  </div>
</template>

<script setup lang="ts">
import { ref, inject, onMounted, type Ref } from 'vue';
import { blogApi } from '@/api/social';
import { ElMessage } from 'element-plus';

const authorId = inject<Ref<number>>('meAuthorId')!;
const authorKey = inject<Ref<string>>('meAuthorKey')!;

const blogs = ref<any[]>([]);
const page = ref(1);
const total = ref(0);
const showMore = ref(false);
const loading = ref(false);

function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleString('zh-CN'); }

async function load() {
  if (loading.value) return;
  loading.value = true;
  try {
    const res = await blogApi.listByAuthor(authorId.value, page.value);
    if (res.data.code === 0) {
      const list = res.data.data.list || [];
      blogs.value = page.value === 1 ? list : [...blogs.value, ...list];
      total.value = res.data.data.total || 0;
      showMore.value = total.value > blogs.value.length;
    }
  } catch {
    ElMessage.error('加载博客失败');
  } finally {
    loading.value = false;
  }
}

function loadMore() {
  page.value++;
  load();
}

onMounted(load);
</script>

<style scoped>
.section-title { font-size: 1.15rem; margin-bottom: 12px; color: var(--text-color); }
.blog-list { display: flex; flex-direction: column; gap: 8px; }
.blog-item {
  padding: 14px 16px;
  background: #fff;
  border-radius: 10px;
  border: 1px solid #eee;
  cursor: pointer;
  transition: box-shadow 0.2s;
}
.blog-item:hover { box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08); }
.blog-title-row { display: flex; align-items: center; gap: 6px; margin-bottom: 4px; }
.blog-item-title { font-weight: 600; font-size: 0.95rem; }
.blog-meta { font-size: 0.75rem; color: var(--text-muted); display: flex; gap: 16px; }
.blog-summary { font-size: 0.85rem; color: #666; margin-top: 4px; }
[data-theme="dark"] .blog-item { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .blog-summary { color: #94a3b8; }
</style>
