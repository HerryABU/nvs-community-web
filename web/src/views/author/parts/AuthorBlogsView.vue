<template>
  <div class="novels-section">
    <h2 class="section-title-bar">
      <span class="title-bar-text">我的博客</span>
      <span class="title-bar-count">{{ myBlogs.length }} 篇</span>
    </h2>
    <div v-loading="loadingBlogs" style="min-height:60px">
      <el-empty v-if="!loadingBlogs && myBlogs.length === 0" description="还没有博客文章，点击上方「写博客」开始吧！" />
      <div v-else class="blog-list-dash">
        <div v-for="blog in myBlogs" :key="blog.id" class="blog-item-dash">
          <div class="blog-item-main">
            <el-tag v-if="blog.is_pinned" size="small" type="danger" style="margin-right:6px">置顶</el-tag>
            <el-tag v-if="blog.category" size="small" style="margin-right:6px">{{ blog.category }}</el-tag>
            <span class="blog-item-title-dash">{{ blog.title }}</span>
            <span class="blog-item-meta">{{ formatDate(blog.created_at) }} · {{ blog.view_count }} 阅读<template v-if="blog.like_count"> · 👍 {{ blog.like_count }}</template></span>
          </div>
          <div class="blog-item-actions">
            <el-button size="small" text type="primary" @click="$router.push(`/author/${authStore.user?.username || authStore.user?.id}/blogs/${blog.id}`)">查看</el-button>
            <el-button size="small" text type="primary" @click="$router.push(`/author/blog/edit/${blog.id}`)">编辑</el-button>
            <el-popconfirm title="确定删除？" @confirm="deleteMyBlog(blog.id)">
              <template #reference><el-button size="small" text type="danger">删除</el-button></template>
            </el-popconfirm>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { authorDataVersion } from './authorRefresh';
import { ref, onMounted, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { blogApi } from '@/api/social';

// 博客视图：只管「我的博客」这一块（列表 / 查看 / 编辑 / 删除）。
const authStore = useAuthStore();
const myBlogs = ref<any[]>([]);
const loadingBlogs = ref(false);

function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleString('zh-CN'); }

async function loadMyBlogs() {
  if (!authStore.user) return;
  loadingBlogs.value = true;
  try {
    const res = await blogApi.listByAuthor(authStore.user.id);
    if (res.data.code === 0) { myBlogs.value = res.data.data.list || []; }
  } catch { /* ignore */ }
  finally { loadingBlogs.value = false; }
}

async function deleteMyBlog(id: number) {
  try { await blogApi.delete(id); ElMessage.success('已删除'); loadMyBlogs(); }
  catch { ElMessage.error('删除失败'); }
}

onMounted(loadMyBlogs);

// 其它视图改动了数据（如回收站恢复作品）→ 重新拉取
watch(authorDataVersion, () => { loadMyBlogs });
</script>

<style scoped>
@import './section.css';

/* 博客管理列表 */
.blog-list-dash { display: flex; flex-direction: column; gap: 6px; }
.blog-item-dash {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 14px; background: rgba(255,255,255,0.04); border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.06);
}
.blog-item-main { display: flex; align-items: center; gap: 8px; flex: 1; min-width: 0; }
.blog-item-title-dash { font-weight: 500; font-size: 0.9rem; }
.blog-item-meta { font-size: 0.75rem; color: #999; white-space: nowrap; }
.blog-item-actions { display: flex; gap: 4px; flex-shrink: 0; }
[data-theme="light"] .blog-item-dash {
  background: #fafafa; border-color: #eee;
}

/* 博客 Cherry 编辑器 */
.blog-cherry-container { min-height: 360px; }

@media (max-width: 768px) {
  /* 博客管理列表：行式改纵向 */
  .blog-item-dash {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  .blog-item-actions {
    width: 100%;
    justify-content: flex-end;
  }
  .blog-item-main {
    flex-wrap: wrap;
  }
  .blog-item-meta {
    white-space: normal;
  }
}
</style>
