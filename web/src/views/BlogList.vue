<template>
  <div class="blog-list-wrap">
    <div class="bl-layout">
      <!-- 左侧分类栏（CSDN 式：一级 + 可展开二级） -->
      <aside class="bl-cats" v-if="categoryTree.length > 0">
        <div class="bl-cats-title">博客分类</div>
        <div class="bl-cat-item" :class="{ active: currentCatId === 0 }" @click="selectCategory(0)">
          <span class="bl-cat-name">全部博客</span>
          <span class="bl-cat-count">{{ totalAll }}</span>
        </div>
        <template v-for="cat in categoryTree" :key="cat.id">
          <div class="bl-cat-item" :class="{ active: currentCatId === cat.id }" @click="selectCategory(cat.id)">
            <span class="bl-cat-arrow" :class="{ open: expandedParents['p'+cat.id] }" v-if="cat.children && cat.children.length > 0"
              @click.stop="toggleParent('p'+cat.id)">▸</span>
            <span v-else class="bl-cat-arrow placeholder">▸</span>
            <span class="bl-cat-name">{{ cat.name }}</span>
            <span class="bl-cat-count">{{ cat.blog_count }}</span>
          </div>
          <div v-if="cat.children && cat.children.length > 0 && expandedParents['p'+cat.id]" class="bl-cat-children">
            <template v-for="sub in cat.children" :key="sub.id">
              <div class="bl-cat-sub" :class="{ active: currentCatId === sub.id }" @click="selectCategory(sub.id)">
                <span class="bl-cat-arrow" :class="{ open: expandedParents['s'+sub.id] }" v-if="sub.children && sub.children.length > 0"
                  @click.stop="toggleParent('s'+sub.id)">▸</span>
                <span v-else class="bl-cat-arrow placeholder">▸</span>
                <span class="bl-cat-name">└ {{ sub.name }}</span>
                <span class="bl-cat-count">{{ sub.blog_count }}</span>
              </div>
              <!-- 三级分类 -->
              <div v-if="sub.children && sub.children.length > 0 && expandedParents['s'+sub.id]" class="bl-cat-children3">
                <div v-for="g in sub.children" :key="g.id" class="bl-cat-sub3" :class="{ active: currentCatId === g.id }"
                  @click="selectCategory(g.id)">
                  <span class="bl-cat-name">　└ {{ g.name }}</span>
                  <span class="bl-cat-count">{{ g.blog_count }}</span>
                </div>
              </div>
            </template>
          </div>
        </template>
      </aside>

      <!-- 右侧列表 -->
      <div class="bl-main">
        <div class="bl-header">
          <h1>📝 作者博客</h1>
          <p class="bl-subtitle">{{ currentCatName ? `分类：${currentCatName}` : '发现作者们的创作心得与技术分享' }}</p>
        </div>

        <div v-loading="loading" class="bl-list" style="min-height: 200px">
          <el-empty v-if="!loading && blogs.length === 0" description="该分类下暂无博客文章" />

          <div v-for="blog in blogs" :key="blog.id" class="bl-card" @click="$router.push(`/author/${blog.author_id}/blogs/${blog.id}`)">
            <!-- 作者信息行 -->
            <div class="bl-author-row">
              <el-avatar :size="32" :src="blog.author?.avatar_url" @click.stop="$router.push(`/author/${blog.author_id}`)">
                {{ blog.author?.nickname?.[0] || 'A' }}
              </el-avatar>
              <span class="bl-author-name" @click.stop="$router.push(`/author/${blog.author_id}`)">
                {{ blog.author?.nickname || blog.author?.username || '作者' }}
              </span>
              <el-tag v-if="blog.category" size="small" class="bl-cat-tag">{{ blog.category }}</el-tag>
              <span class="bl-date">{{ formatDate(blog.created_at) }}</span>
              <span class="bl-views">{{ blog.view_count }} 阅读</span>
              <span class="bl-likes" v-if="blog.like_count > 0">👍 {{ blog.like_count }}</span>
            </div>

            <!-- 标题 -->
            <h2 class="bl-title">
              <el-tag v-if="blog.is_pinned" type="danger" size="small" class="bl-pin-tag">置顶</el-tag>
              {{ blog.title }}
            </h2>

            <!-- 内容预览：前100字 + 图片提取 -->
            <div class="bl-preview">
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
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue';
import { blogApi, type BlogCategoryNode } from '@/api/social';

const blogs = ref<any[]>([]);
const total = ref(0);
const totalAll = ref(0);
const page = ref(1);
const pageSize = 12;
const loading = ref(false);
const previewLength = 100;

const categoryTree = ref<BlogCategoryNode[]>([]);
const currentCatId = ref(0);
const expandedParents = reactive<Record<number, boolean>>({});

const currentCatName = computed(() => {
  if (currentCatId.value === 0) return '';
  for (const c of categoryTree.value) {
    if (c.id === currentCatId.value) return c.name;
    for (const s of c.children || []) {
      if (s.id === currentCatId.value) return `${c.name} / ${s.name}`;
      for (const g of s.children || []) {
        if (g.id === currentCatId.value) return `${c.name} / ${s.name} / ${g.name}`;
      }
    }
  }
  return '';
});

function toggleParent(id: string) {
  expandedParents[id] = !expandedParents[id];
}

function selectCategory(id: number) {
  currentCatId.value = id;
  page.value = 1;
  loadBlogs();
}

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

async function loadCategories() {
  try {
    const res = await blogApi.getCategories();
    if (res.data.code === 0) {
      categoryTree.value = res.data.data || [];
      totalAll.value = categoryTree.value.reduce((sum, c) => sum + c.blog_count, 0);
    }
  } catch { /* ignore */ }
}

async function loadBlogs() {
  loading.value = true;
  try {
    const res = await blogApi.listPublic(page.value, pageSize, currentCatId.value || undefined);
    if (res.data.code === 0) {
      blogs.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
    }
  } catch { /* ignore */ }
  finally { loading.value = false; }
}

onMounted(() => {
  loadCategories();
  loadBlogs();
});
</script>

<style scoped>
.blog-list-wrap { min-height: 100vh; background: var(--bg-color); }
.bl-layout {
  max-width: 1080px;
  margin: 0 auto;
  padding: 24px 16px;
  display: flex;
  gap: 24px;
  align-items: flex-start;
}

/* ===== 左侧分类栏 ===== */
.bl-cats {
  width: 220px;
  flex-shrink: 0;
  background: #fff;
  border: 1px solid #eee;
  border-radius: 12px;
  padding: 14px 10px;
  position: sticky;
  top: 80px;
  max-height: calc(100vh - 110px);
  overflow-y: auto;
}
[data-theme="dark"] .bl-cats { background: #1e293b; border-color: #334155; }
.bl-cats-title { font-size: 0.9rem; font-weight: 700; padding: 4px 10px 10px; color: var(--text-color); border-bottom: 1px solid #f0f0f0; margin-bottom: 8px; }
[data-theme="dark"] .bl-cats-title { border-bottom-color: #334155; }
.bl-cat-item, .bl-cat-sub {
  display: flex; align-items: center; gap: 6px;
  padding: 8px 10px; border-radius: 8px;
  cursor: pointer; font-size: 0.88rem;
  color: var(--text-color); transition: background 0.15s;
}
.bl-cat-item:hover, .bl-cat-sub:hover { background: #f5f7fa; }
[data-theme="dark"] .bl-cat-item:hover, [data-theme="dark"] .bl-cat-sub:hover { background: rgba(255,255,255,0.05); }
.bl-cat-item.active, .bl-cat-sub.active { background: var(--primary-color); color: #fff; }
.bl-cat-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.bl-cat-count { font-size: 0.72rem; color: #aaa; }
.bl-cat-item.active .bl-cat-count, .bl-cat-sub.active .bl-cat-count { color: rgba(255,255,255,0.75); }
.bl-cat-arrow {
  width: 14px; font-size: 0.72rem; color: #aaa;
  display: inline-flex; align-items: center; justify-content: center;
  transition: transform 0.15s;
}
.bl-cat-arrow.open { transform: rotate(90deg); }
.bl-cat-arrow.placeholder { visibility: hidden; }
.bl-cat-children { padding-left: 14px; }
.bl-cat-sub { font-size: 0.82rem; }
.bl-cat-children3 { padding-left: 20px; }
.bl-cat-sub3 { font-size: 0.78rem; padding: 5px 10px; border-radius: 8px; cursor: pointer; display: flex; align-items: center; gap: 6px; }
.bl-cat-sub3:hover { background: #f5f7fa; }
[data-theme="dark"] .bl-cat-sub3:hover { background: rgba(255,255,255,0.05); }
.bl-cat-sub3.active { background: var(--primary-color); color: #fff; }
.bl-cat-sub3 .bl-cat-count { color: #aaa; }
.bl-cat-sub3.active .bl-cat-count { color: rgba(255,255,255,0.75); }

/* ===== 右侧 ===== */
.bl-main { flex: 1; min-width: 0; }
.bl-header { margin-bottom: 20px; text-align: center; }
.bl-header h1 { font-size: 1.6rem; color: var(--primary-color); margin-bottom: 4px; }
.bl-subtitle { font-size: 0.9rem; color: #999; }

.bl-list { display: flex; flex-direction: column; gap: 16px; }

.bl-card {
  padding: 20px 24px;
  background: #fff; border-radius: 12px;
  border: 1px solid #eee;
  cursor: pointer; transition: box-shadow 0.2s;
}
.bl-card:hover { box-shadow: 0 4px 16px rgba(0,0,0,0.08); }
[data-theme="dark"] .bl-card { background: #1e293b; border-color: #334155; }

.bl-author-row {
  display: flex; align-items: center; gap: 8px; margin-bottom: 10px; flex-wrap: wrap;
}
.bl-author-name { font-size: 0.85rem; color: var(--primary-color); cursor: pointer; font-weight: 500; }
.bl-author-name:hover { text-decoration: underline; }
.bl-cat-tag { font-size: 0.72rem; }
.bl-date, .bl-views, .bl-likes { font-size: 0.75rem; color: #999; }

.bl-title { font-size: 1.2rem; font-weight: 700; margin-bottom: 10px; line-height: 1.4; display: flex; align-items: center; gap: 8px; }
.bl-pin-tag { flex-shrink: 0; }

.bl-preview { display: flex; gap: 16px; margin-bottom: 10px; }
.bl-cover-img {
  width: 120px; height: 80px; object-fit: cover; border-radius: 6px; flex-shrink: 0;
  border: 1px solid #eee;
}
.bl-summary { font-size: 0.9rem; color: #666; line-height: 1.7; flex: 1; }
[data-theme="dark"] .bl-summary { color: #94a3b8; }

.bl-footer { display: flex; justify-content: flex-end; align-items: center; }
.bl-read-more { font-size: 0.85rem; color: var(--primary-color); }

.pagination { margin-top: 24px; display: flex; justify-content: center; }

/* ====== 移动端适配 ====== */
@media (max-width: 768px) {
  .bl-layout { flex-direction: column; gap: 12px; }
  .bl-cats {
    width: 100%;
    position: static;
    max-height: none;
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    padding: 10px;
  }
  .bl-cats-title { width: 100%; padding-bottom: 6px; margin-bottom: 4px; }
  .bl-cat-item, .bl-cat-sub { padding: 5px 10px; }
  .bl-cat-children, .bl-cat-children3 { padding-left: 0; width: 100%; display: flex; flex-wrap: wrap; }
  .bl-cat-children .bl-cat-sub { width: auto; }
  .bl-card { padding: 14px 16px; }
  .bl-title { font-size: 1.05rem; }
  .bl-preview { flex-direction: column; gap: 8px; }
  .bl-cover-img { width: 100%; height: auto; max-height: 160px; object-fit: cover; }
}
</style>
