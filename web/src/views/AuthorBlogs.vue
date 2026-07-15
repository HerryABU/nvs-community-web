<template>
  <div class="page-container blog-list-page">
    <div class="bl-header">
      <h1>
        <router-link :to="`/author/${authorId}`" style="color:var(--primary-color);text-decoration:none">
          {{ authorName }}
        </router-link>
        的博客
      </h1>
      <p class="bl-subtitle">共 {{ total }} 篇文章</p>
    </div>

    <div v-loading="loading" class="bl-list">
      <el-empty v-if="!loading && blogs.length === 0" description="该作者还没有博客文章" />

      <div v-for="blog in blogs" :key="blog.id" class="bl-card" @click="$router.push(`/author/${authorId}/blogs/${blog.id}`)">
        <el-tag v-if="blog.is_pinned" type="danger" size="small" class="bl-pin">置顶</el-tag>

        <!-- 作者信息行 -->
        <div class="bl-author-row">
          <el-avatar :size="32" :src="blog.author?.avatar_url">
            {{ blog.author?.nickname?.[0] || authorName?.[0] || 'A' }}
          </el-avatar>
          <span class="bl-author-name">{{ blog.author?.nickname || blog.author?.username || authorName }}</span>
          <span class="bl-date">{{ formatDate(blog.created_at) }}</span>
          <span class="bl-views">{{ blog.view_count }} 阅读</span>
        </div>

        <h2 class="bl-title">{{ blog.title }}</h2>

        <p class="bl-summary">{{ blog.summary || '暂无摘要' }}</p>

        <div class="bl-footer">
          <span class="bl-read-more">阅读全文 →</span>
        </div>
      </div>
    </div>

    <div class="pagination" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="loadBlogs"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { blogApi } from '@/api/social';
import { publicApi } from '@/api/admin';

const route = useRoute();
const authorId = ref(Number(route.params.id));
const authorName = ref('');
const blogs = ref<any[]>([]);
const loading = ref(false);
const page = ref(1);
const pageSize = 12;
const total = ref(0);

function formatDate(d?: string) { return d ? new Date(d).toLocaleDateString('zh-CN') : ''; }

async function loadAuthor() {
  try {
    const res = await publicApi.getAuthorProfile(authorId.value);
    const author = res.data?.data?.author || res.data?.data;
    if (author) {
      authorName.value = author.nickname || author.username || '作者';
    }
  } catch { /* ignore */ }
}

async function loadBlogs() {
  loading.value = true;
  try {
    const res = await blogApi.listByAuthor(authorId.value, page.value);
    if (res.data?.code === 0) {
      blogs.value = res.data.data?.list || res.data.data || [];
      total.value = res.data.data?.total || 0;
    }
  } catch { /* ignore */ }
  loading.value = false;
}

onMounted(() => {
  loadAuthor();
  loadBlogs();
});
</script>

<style scoped>
.page-container { max-width: 780px; margin: 0 auto; padding: 24px 16px; }
.bl-header { margin-bottom: 24px; text-align: center; }
.bl-header h1 { font-size: 1.6rem; color: var(--primary-color); margin-bottom: 4px; }
.bl-subtitle { font-size: 0.9rem; color: #999; }

.bl-list { display: flex; flex-direction: column; gap: 16px; }

.bl-card {
  position: relative;
  padding: 20px 24px;
  background: #fff; border-radius: 12px;
  border: 1px solid #eee;
  cursor: pointer; transition: box-shadow 0.2s;
}
.bl-card:hover { box-shadow: 0 4px 16px rgba(0,0,0,0.08); }
[data-theme="dark"] .bl-card { background: #1e293b; border-color: #334155; }

.bl-pin { position: absolute; top: 12px; right: 16px; }

.bl-author-row {
  display: flex; align-items: center; gap: 8px; margin-bottom: 10px;
}
.bl-author-name { font-size: 0.85rem; color: var(--primary-color); font-weight: 500; }
.bl-date, .bl-views { font-size: 0.75rem; color: #999; }

.bl-title { font-size: 1.2rem; font-weight: 700; margin-bottom: 10px; line-height: 1.4; }
.bl-summary { font-size: 0.9rem; color: #666; line-height: 1.7; }
[data-theme="dark"] .bl-summary { color: #94a3b8; }

.bl-footer { display: flex; justify-content: space-between; align-items: center; margin-top: 8px; }
.bl-read-more { font-size: 0.85rem; color: var(--primary-color); }

.pagination { margin-top: 24px; display: flex; justify-content: center; }
</style>
