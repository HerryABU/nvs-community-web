<template>
  <div class="page-container blog-list-page">
    <div class="bl-header">
      <h1>📝 作者博客</h1>
      <p class="bl-subtitle">发现作者们的创作心得与技术分享</p>
    </div>

    <div v-loading="loading" class="bl-list">
      <el-empty v-if="!loading && blogs.length === 0" description="暂无博客文章" />

      <div v-for="blog in blogs" :key="blog.id" class="bl-card" @click="$router.push(`/author/${blog.author_id}/blogs/${blog.id}`)">
        <!-- 置顶标记 -->
        <el-tag v-if="blog.is_pinned" type="danger" size="small" class="bl-pin">置顶</el-tag>

        <!-- 作者信息行 -->
        <div class="bl-author-row">
          <el-avatar :size="32" :src="blog.author?.avatar_url" @click.stop="$router.push(`/author/${blog.author_id}`)">
            {{ blog.author?.nickname?.[0] || 'A' }}
          </el-avatar>
          <span class="bl-author-name" @click.stop="$router.push(`/author/${blog.author_id}`)">
            {{ blog.author?.nickname || blog.author?.username || '作者' }}
          </span>
          <span class="bl-date">{{ formatDate(blog.created_at) }}</span>
          <span class="bl-views">{{ blog.view_count }} 阅读</span>
        </div>

        <!-- 标题 -->
        <h2 class="bl-title">{{ blog.title }}</h2>

        <!-- 内容预览：前100字 + 图片提取 -->
        <div class="bl-preview">
          <!-- 提取首张图片 -->
          <img
            v-if="extractFirstImage(blog.content)"
            :src="extractFirstImage(blog.content)"
            class="bl-cover-img"
          />
          <p class="bl-summary">
            {{ blog.summary || extractText(blog.content, previewLength) }}
          </p>
        </div>

        <!-- 底部标签 -->
        <div class="bl-footer">
          <el-tag v-if="blog.is_pinned" size="small" type="danger">置顶</el-tag>
          <span class="bl-read-more">阅读全文 →</span>
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
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { blogApi } from '@/api/social';

const blogs = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 12;
const loading = ref(false);
const previewLength = 100;

function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleDateString('zh-CN'); }

// 提取纯文本（去掉 HTML/Markdown 标记）
function extractText(html: string, maxLen: number): string {
  if (!html) return '';
  const div = document.createElement('div');
  div.innerHTML = html;
  const text = (div.textContent || '').replace(/\s+/g, ' ').trim();
  return text.length > maxLen ? text.slice(0, maxLen) + '…' : text;
}

// 提取首张图片 URL
function extractFirstImage(html: string): string | null {
  if (!html) return null;
  const m = html.match(/<img[^>]+src="([^">]+)"/i);
  return m ? m[1] : null;
}

async function loadBlogs() {
  loading.value = true;
  try {
    const res = await blogApi.listPublic(page.value, pageSize);
    if (res.data.code === 0) {
      blogs.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
    }
  } catch { /* ignore */ }
  finally { loading.value = false; }
}

onMounted(() => loadBlogs());
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
.bl-author-name { font-size: 0.85rem; color: var(--primary-color); cursor: pointer; font-weight: 500; }
.bl-author-name:hover { text-decoration: underline; }
.bl-date, .bl-views { font-size: 0.75rem; color: #999; }

.bl-title { font-size: 1.2rem; font-weight: 700; margin-bottom: 10px; line-height: 1.4; }

.bl-preview { display: flex; gap: 16px; margin-bottom: 10px; }
.bl-cover-img {
  width: 120px; height: 80px; object-fit: cover; border-radius: 6px; flex-shrink: 0;
  border: 1px solid #eee;
}
.bl-summary { font-size: 0.9rem; color: #666; line-height: 1.7; flex: 1; }
[data-theme="dark"] .bl-summary { color: #94a3b8; }

.bl-footer { display: flex; justify-content: space-between; align-items: center; }
.bl-read-more { font-size: 0.85rem; color: var(--primary-color); }

.pagination { margin-top: 24px; display: flex; justify-content: center; }
</style>