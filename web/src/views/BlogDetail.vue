<template>
  <div class="page-container blog-detail" v-loading="loading">
    <template v-if="blog">
      <div class="blog-header">
        <h1>{{ blog.title }}</h1>
        <div class="blog-meta-line">
          <router-link :to="`/author/${blog.author_id}`" class="blog-author">
            {{ blog.author?.nickname || blog.author?.username || '作者' }}
          </router-link>
          <span>{{ formatDate(blog.created_at) }}</span>
          <span>{{ blog.view_count }} 次阅读</span>
          <el-tag v-if="blog.is_pinned" size="small" type="danger">置顶</el-tag>
        </div>
      </div>

      <div class="blog-content markdown-body" v-html="htmlContent"></div>

      <div class="blog-footer">
        <el-button @click="$router.back()">返回</el-button>
        <router-link :to="`/author/${blog.author_id}`">
          <el-button type="primary" plain>查看作者主页</el-button>
        </router-link>
      </div>

      <!-- 评论区 -->
      <h2 class="section-title" style="margin-top: 40px">评论</h2>
      <CommentSection :novel-id="0" :novel-category="''" :blog-id="blog.id" />
    </template>
    <el-empty v-else-if="!loading" description="博客不存在" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { blogApi } from '@/api/social';
import { renderMarkdown } from '@/markdown/renderer';
import CommentSection from '@/components/CommentSection.vue';

const route = useRoute();
const blog = ref<any>(null);
const htmlContent = ref('');
const loading = ref(false);

function formatDate(d?: string) {
  if (!d) return '';
  return new Date(d).toLocaleString('zh-CN');
}

onMounted(async () => {
  const id = Number(route.params.id);
  loading.value = true;
  try {
    const res = await blogApi.getBlog(id);
    if (res.data.code === 0) {
      blog.value = res.data.data;
      htmlContent.value = renderMarkdown(blog.value.content || '');
    }
  } catch { /* ignore */ }
  finally { loading.value = false; }
});
</script>

<style scoped>
.page-container { max-width: 800px; margin: 0 auto; padding: 24px; }
.blog-header { margin-bottom: 24px; padding-bottom: 16px; border-bottom: 1px solid #eee; }
.blog-header h1 { font-size: 1.5rem; margin-bottom: 8px; }
.blog-meta-line {
  display: flex; align-items: center; gap: 14px;
  font-size: 0.85rem; color: #999; flex-wrap: wrap;
}
.blog-author { font-weight: 600; color: var(--primary-color); text-decoration: none; }
.blog-content { line-height: 1.8; font-size: 1rem; }
.blog-footer { margin-top: 32px; display: flex; gap: 12px; }
.section-title { font-size: 1.15rem; font-weight: 600; margin-bottom: 16px; }
</style>