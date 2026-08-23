<template>
  <div class="blog-manager" v-loading="loading">
    <!-- 非本人访问 → 重定向公开主页 -->
    <template v-if="owner">
      <div class="bm-header">
        <h1>📝 我的博客管理</h1>
        <div class="bm-actions">
          <el-button type="primary" @click="$router.push('/author/blog/new')">✍️ 写博客</el-button>
          <el-button @click="$router.push(`/blog/${owner.username}`)">查看公开主页</el-button>
        </div>
      </div>

      <div class="bm-toolbar">
        <el-input v-model="search" placeholder="搜索标题…" clearable style="width: 220px" size="small" @keyup.enter="loadBlogs" />
        <el-select v-model="filterCat" placeholder="全部分类" clearable size="small" style="width: 160px" @change="loadBlogs">
          <el-option-group v-for="cat in categoryTree" :key="cat.id" :label="cat.name">
            <el-option :value="cat.id" :label="cat.name" />
            <el-option v-for="sub in cat.children || []" :key="sub.id" :value="sub.id" :label="`　└ ${sub.name}`" />
            <template v-if="cat.children?.length">
              <el-option v-for="sub in cat.children" :key="'s' + sub.id" :label="`　└ ${sub.name}`" :value="sub.id" />
            </template>
          </el-option-group>
        </el-select>
        <span class="bm-count">共 {{ total }} 篇</span>
      </div>

      <div class="bm-list" v-loading="listLoading">
        <el-empty v-if="!listLoading && blogs.length === 0" description="还没有博客，点击「写博客」开始创作" />
        <div v-for="blog in blogs" :key="blog.id" class="bm-item">
          <div class="bm-item-main">
            <div class="bm-item-tags">
              <el-tag v-if="blog.is_pinned" size="small" type="danger">置顶</el-tag>
              <el-tag v-if="blog.category" size="small">{{ blog.category }}</el-tag>
              <span class="bm-item-meta">{{ formatDate(blog.created_at) }} · {{ blog.view_count }} 阅读<template v-if="blog.like_count"> · 👍 {{ blog.like_count }}</template> · 评论 {{ blog.comment_count || 0 }}</span>
            </div>
            <div class="bm-item-title" @click="$router.push(`/author/${owner.username}/blogs/${blog.id}`)">{{ blog.title }}</div>
          </div>
          <div class="bm-item-actions">
            <el-button size="small" text type="primary" @click="$router.push(`/author/blog/edit/${blog.id}`)">编辑</el-button>
            <el-button size="small" text type="warning" @click="togglePin(blog)">{{ blog.is_pinned ? '取消置顶' : '置顶' }}</el-button>
            <el-button size="small" text type="danger" @click="removeBlog(blog)">删除</el-button>
          </div>
        </div>
      </div>

      <div class="pagination" v-if="total > pageSize">
        <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" @current-change="loadBlogs" />
      </div>
    </template>
    <el-empty v-else-if="!loading" description="无权访问该博客管理页" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { blogApi } from '@/api/social';
import { publicApi } from '@/api/admin';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const key = ref(String(route.params.key ?? ''));
const owner = ref<any>(null);
const loading = ref(false);
const listLoading = ref(false);
const blogs = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 10;
const search = ref('');
const filterCat = ref<number | undefined>(undefined);
const categoryTree = ref<any[]>([]);

function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleDateString('zh-CN'); }

async function loadCategories() {
  try {
    const res = await blogApi.getCategories();
    if (res.data.code === 0) categoryTree.value = res.data.data || [];
  } catch { /* ignore */ }
}

async function loadBlogs() {
  listLoading.value = true;
  try {
    const res = await blogApi.listByAuthor(owner.value.id, page.value);
    if (res.data.code === 0) {
      let list = res.data.data.list || [];
      total.value = res.data.data.total || 0;
      if (search.value.trim()) {
        const q = search.value.trim();
        list = list.filter((b: any) => b.title.includes(q));
      }
      if (filterCat.value) {
        list = list.filter((b: any) => b.category_id === filterCat.value);
      }
      blogs.value = list;
    }
  } catch { /* ignore */ }
  finally { listLoading.value = false; }
}

async function togglePin(blog: any) {
  try {
    await blogApi.update(blog.id, { is_pinned: !blog.is_pinned });
    ElMessage.success(blog.is_pinned ? '已取消置顶' : '已置顶');
    loadBlogs();
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '操作失败'); }
}

async function removeBlog(blog: any) {
  try {
    await ElMessageBox.confirm(`确定删除博客《${blog.title}》？删除后进入回收站。`, '删除确认', {
      confirmButtonText: '删除', type: 'warning',
    });
    await blogApi.delete(blog.id);
    ElMessage.success('已删除');
    loadBlogs();
  } catch { /* 取消 */ }
}

async function init() {
  loading.value = true;
  try {
    // 解析 URL 中的用户标识（数字=id / 用户名）
    const r = await publicApi.resolveUser(key.value);
    if (r.data.code !== 0) {
      // 解析不到用户：纯数字 → 兼容旧「编辑博客」链接 /author/blog/{blogId}
      if (/^\d+$/.test(key.value)) {
        router.replace(`/author/blog/edit/${key.value}`);
        return;
      }
      return; // 404
    }
    const u = r.data.data;
    // 仅本人可管理：他人访问 → 重定向公开博客主页
    if (!authStore.isLoggedIn || authStore.user?.id !== u.id) {
      router.replace(`/blog/${u.username}`);
      return;
    }
    owner.value = u;
    loadCategories();
    loadBlogs();
  } catch { /* ignore */ }
  finally { loading.value = false; }
}

onMounted(init);
</script>

<style scoped>
.blog-manager { max-width: 920px; margin: 0 auto; padding: 24px 16px; }
.bm-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 18px; flex-wrap: wrap; gap: 10px; }
.bm-header h1 { font-size: 1.4rem; color: var(--primary-color); margin: 0; }
.bm-actions { display: flex; gap: 8px; }
.bm-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; flex-wrap: wrap; }
.bm-count { font-size: 0.85rem; color: #999; }
.bm-list { display: flex; flex-direction: column; gap: 12px; min-height: 200px; }
.bm-item {
  display: flex; align-items: center; justify-content: space-between; gap: 12px;
  padding: 14px 18px; background: #fff; border: 1px solid #eee; border-radius: 12px; flex-wrap: wrap;
}
[data-theme="dark"] .bm-item { background: #1e293b; border-color: #334155; }
.bm-item-main { flex: 1; min-width: 200px; }
.bm-item-tags { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.bm-item-meta { font-size: 0.75rem; color: #999; }
.bm-item-title { font-size: 1rem; font-weight: 600; margin-top: 6px; cursor: pointer; }
.bm-item-title:hover { color: var(--primary-color); }
.bm-item-actions { display: flex; flex-shrink: 0; }
.pagination { margin-top: 20px; display: flex; justify-content: center; }
@media (max-width: 640px) {
  .bm-item-actions { width: 100%; justify-content: flex-end; }
}
</style>
