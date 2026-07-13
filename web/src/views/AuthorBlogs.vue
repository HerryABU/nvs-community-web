<template>
  <div class="page-container">
    <div class="editor-header">
      <div>
        <h1>
          <router-link :to="`/author/${authorId}`" style="color:var(--primary-color);text-decoration:none">
            {{ authorName }}
          </router-link>
          的博客
        </h1>
      </div>
      <el-button @click="$router.back()">返回</el-button>
    </div>

    <div v-loading="loading" style="min-height:200px">
      <el-empty v-if="!loading && blogs.length === 0" description="该作者还没有博客文章" />
      <div v-else class="blog-list-dash">
        <div v-for="blog in blogs" :key="blog.id" class="blog-item-dash">
          <div class="blog-item-main">
            <el-tag v-if="blog.is_pinned" size="small" type="danger" style="margin-right:6px">置顶</el-tag>
            <span class="blog-item-title-dash">{{ blog.title }}</span>
            <span class="blog-item-meta">{{ formatDate(blog.created_at) }} · {{ blog.view_count }} 阅读</span>
          </div>
          <div class="blog-item-actions">
            <el-button size="small" text type="primary" @click="$router.push(`/blog/${blog.id}`)">查看</el-button>
          </div>
        </div>
      </div>

      <!-- 分页 -->
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
const pageSize = ref(12);
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
