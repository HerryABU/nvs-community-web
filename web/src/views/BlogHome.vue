<template>
  <div class="blog-home-wrap" v-loading="loading">
    <template v-if="user">
      <!-- 作者信息卡 -->
      <div class="bh-user-card">
        <div class="bh-user-avatar">
          <el-avatar :size="72" :src="profile?.author?.avatar_url">
            {{ (profile?.author?.nickname || user.nickname || user.username || 'A')[0] }}
          </el-avatar>
        </div>
        <div class="bh-user-info">
          <div class="bh-user-name">
            {{ profile?.author?.nickname || user.nickname || user.username }}
            <el-tag v-if="user.role === 'admin'" size="small" type="danger" style="margin-left:6px">管理员</el-tag>
            <el-tag v-else-if="user.role === 'author'" size="small" style="margin-left:6px">作者</el-tag>
          </div>
          <div class="bh-user-sub">
            <span>用户名：{{ user.username }}</span>
            <template v-if="profile">
              <span>作品 {{ profile.total_novels || 0 }} 部 · 章节 {{ profile.total_chapters || 0 }} · 字数 {{ (profile.total_words || 0).toLocaleString() }}</span>
            </template>
          </div>
          <div class="bh-user-links">
            <router-link :to="`/author/${user.username}`" class="bh-link">作者主页 →</router-link>
          </div>
        </div>
      </div>

      <!-- 布局：分类栏 + 博客列表 -->
      <div class="bh-layout">
        <!-- 左侧分类（该用户博客分类过滤） -->
        <aside class="bh-cats" v-if="userCats.length > 0">
          <div class="bh-cats-title">博客分类</div>
          <div class="bh-cat-item" :class="{ active: filterCatId === 0 }" @click="selectCat(0)">全部</div>
          <template v-for="cat in userCats" :key="cat.id">
            <div class="bh-cat-item" :class="{ active: filterCatId === cat.id }" @click="selectCat(cat.id)">
              <span class="bh-cat-arrow" :class="{ open: expanded[cat.id] }" v-if="cat.children?.length" @click.stop="toggle(cat.id)">▸</span>
              <span v-else class="bh-cat-arrow placeholder">▸</span>
              {{ cat.name }}
            </div>
            <div v-if="cat.children?.length && expanded[cat.id]" class="bh-cat-children">
              <div v-for="sub in cat.children" :key="sub.id" class="bh-cat-sub" :class="{ active: filterCatId === sub.id }" @click="selectCat(sub.id)">└ {{ sub.name }}</div>
            </div>
          </template>
        </aside>

        <!-- 博客列表 -->
        <div class="bh-main">
          <div class="bh-header">
            <h1>📝 {{ user.nickname || user.username }} 的博客</h1>
            <p class="bh-subtitle">共 {{ total }} 篇文章</p>
          </div>

          <div class="bh-list">
            <el-empty v-if="!listLoading && blogs.length === 0" description="还没有博客文章" />
            <div v-for="blog in blogs" :key="blog.id" class="bh-card" @click="$router.push(`/author/${user.username}/blogs/${blog.id}`)">
              <div class="bh-card-row">
                <el-tag v-if="blog.is_pinned" size="small" type="danger">置顶</el-tag>
                <el-tag v-if="blog.category" size="small">{{ blog.category }}</el-tag>
                <span class="bh-card-date">{{ formatDate(blog.created_at) }}</span>
                <span class="bh-card-meta">{{ blog.view_count }} 阅读<template v-if="blog.like_count"> · 👍 {{ blog.like_count }}</template></span>
              </div>
              <h2 class="bh-card-title">{{ blog.title }}</h2>
              <p class="bh-card-summary">{{ blog.summary || '暂无摘要' }}</p>
            </div>
          </div>

          <div class="pagination" v-if="total > pageSize">
            <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" @current-change="loadBlogs" />
          </div>
        </div>
      </div>
    </template>
    <el-empty v-else-if="!loading" description="用户不存在" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { blogApi, type BlogCategoryNode } from '@/api/social';
import { publicApi } from '@/api/admin';

const route = useRoute();
const router = useRouter();

const key = ref(String(route.params.key ?? ''));
const user = ref<any>(null);
const profile = ref<any>(null);
const loading = ref(false);
const listLoading = ref(false);
const blogs = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 10;
const filterCatId = ref(0);
const expanded = reactive<Record<number, boolean>>({});
const allCats = ref<BlogCategoryNode[]>([]);

// 该用户用到的分类（从博客列表里收集）
const userCats = computed(() => allCats.value.filter(c => c.blog_count > 0));

function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleDateString('zh-CN'); }
function toggle(id: number) { expanded[id] = !expanded[id]; }
function selectCat(id: number) { filterCatId.value = id; page.value = 1; loadBlogs(); }

async function loadBlogs() {
  listLoading.value = true;
  try {
    const res = await blogApi.listByAuthor(user.value.id, page.value);
    if (res.data.code === 0) {
      let list = res.data.data.list || [];
      total.value = res.data.data.total || 0;
      if (filterCatId.value > 0) {
        list = list.filter((b: any) => b.category_id === filterCatId.value);
      }
      blogs.value = list;
    }
  } catch { /* ignore */ }
  finally { listLoading.value = false; }
}

async function init() {
  loading.value = true;
  try {
    // 1) 先按用户标识解析（数字=id，非数字=用户名）
    const r = await publicApi.resolveUser(key.value);
    if (r.data.code === 0) {
      user.value = r.data.data;
      try {
        const p = await publicApi.getAuthorProfile(user.value.id);
        if (p.data.code === 0) profile.value = p.data.data;
      } catch { /* ignore */ }
      // 分类树（全站）
      try {
        const c = await blogApi.getCategories();
        if (c.data.code === 0) allCats.value = c.data.data || [];
      } catch { /* ignore */ }
      loadBlogs();
      return;
    }
    // 2) 解析不到用户：若 key 是数字，尝试当作「博客 id」→ 重定向到规范详情
    if (/^\d+$/.test(key.value)) {
      try {
        const b = await blogApi.getBlog(Number(key.value));
        if (b.data.code === 0 && b.data.data) {
          const blog = b.data.data;
          router.replace(`/author/${blog.author_id}/blogs/${blog.id}`);
          return;
        }
      } catch { /* ignore */ }
    }
    user.value = null; // 404
  } catch { /* ignore */ }
  finally { loading.value = false; }
}

onMounted(init);
</script>

<style scoped>
.blog-home-wrap { min-height: 100vh; background: var(--bg-color); padding: 24px 16px; }

/* 作者卡 */
.bh-user-card {
  max-width: 1080px; margin: 0 auto 20px;
  display: flex; align-items: center; gap: 20px;
  background: #fff; border: 1px solid #eee; border-radius: 14px; padding: 20px 24px;
}
[data-theme="dark"] .bh-user-card { background: #1e293b; border-color: #334155; }
.bh-user-name { font-size: 1.3rem; font-weight: 700; display: flex; align-items: center; }
.bh-user-sub { display: flex; gap: 14px; font-size: 0.82rem; color: #999; margin-top: 6px; flex-wrap: wrap; }
.bh-user-links { margin-top: 8px; }
.bh-link { font-size: 0.85rem; color: var(--primary-color); text-decoration: none; }

.bh-layout { max-width: 1080px; margin: 0 auto; display: flex; gap: 24px; align-items: flex-start; }

/* 左侧分类 */
.bh-cats {
  width: 200px; flex-shrink: 0;
  background: #fff; border: 1px solid #eee; border-radius: 12px; padding: 12px 10px;
  position: sticky; top: 80px; max-height: calc(100vh - 110px); overflow-y: auto;
}
[data-theme="dark"] .bh-cats { background: #1e293b; border-color: #334155; }
.bh-cats-title { font-size: 0.9rem; font-weight: 700; padding: 4px 10px 8px; border-bottom: 1px solid #f0f0f0; margin-bottom: 6px; }
.bh-cat-item, .bh-cat-sub {
  display: flex; align-items: center; gap: 4px;
  padding: 7px 10px; border-radius: 8px; cursor: pointer; font-size: 0.86rem;
  color: var(--text-color); transition: background 0.15s;
}
.bh-cat-item:hover, .bh-cat-sub:hover { background: #f5f7fa; }
[data-theme="dark"] .bh-cat-item:hover, [data-theme="dark"] .bh-cat-sub:hover { background: rgba(255,255,255,0.05); }
.bh-cat-item.active, .bh-cat-sub.active { background: var(--primary-color); color: #fff; }
.bh-cat-arrow { width: 12px; font-size: 0.7rem; color: #aaa; transition: transform 0.15s; }
.bh-cat-arrow.open { transform: rotate(90deg); }
.bh-cat-arrow.placeholder { visibility: hidden; }
.bh-cat-children { padding-left: 14px; }

/* 右侧列表 */
.bh-main { flex: 1; min-width: 0; }
.bh-header { margin-bottom: 16px; text-align: center; }
.bh-header h1 { font-size: 1.4rem; color: var(--primary-color); margin-bottom: 4px; }
.bh-subtitle { font-size: 0.88rem; color: #999; }
.bh-list { display: flex; flex-direction: column; gap: 14px; }
.bh-card {
  padding: 16px 20px; background: #fff; border: 1px solid #eee; border-radius: 12px;
  cursor: pointer; transition: box-shadow 0.2s;
}
.bh-card:hover { box-shadow: 0 4px 16px rgba(0,0,0,0.08); }
[data-theme="dark"] .bh-card { background: #1e293b; border-color: #334155; }
.bh-card-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.bh-card-date, .bh-card-meta { font-size: 0.75rem; color: #999; }
.bh-card-title { font-size: 1.1rem; font-weight: 700; margin: 8px 0 6px; line-height: 1.4; }
.bh-card-summary { font-size: 0.86rem; color: #666; line-height: 1.7; }
[data-theme="dark"] .bh-card-summary { color: #94a3b8; }
.pagination { margin-top: 20px; display: flex; justify-content: center; }

/* 移动端 */
@media (max-width: 768px) {
  .bh-user-card { flex-direction: column; text-align: center; }
  .bh-layout { flex-direction: column; }
  .bh-cats { width: 100%; position: static; max-height: none; display: flex; flex-wrap: wrap; gap: 4px; }
  .bh-cats-title { width: 100%; }
  .bh-cat-children { padding-left: 0; width: 100%; display: flex; flex-wrap: wrap; }
}
</style>
