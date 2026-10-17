<template>
  <div class="blog-list-wrap">
    <div class="bl-layout">
      <!-- 左侧分类栏（CSDN 式：一级 + 可展开二级） -->
            <!-- 右侧列表 -->
      <div class="bl-main">
        <SectionHeader
          title="作者博客"
          :subtitle="currentTag ? `标签：#${currentTag}` : (currentCatName ? `分类：${currentCatName}` : '发现作者们的创作心得与技术分享')"
          :icon="'📝'"
          gradient="primary"
        >
          <template #actions>
            <el-tag v-if="currentTag" size="small" closable type="primary" @close="selectTag('')">清除筛选</el-tag>
          </template>
        </SectionHeader>

        <div v-loading="loading" class="bl-list" style="min-height: 200px">
          <EmptyState v-if="!loading && blogs.length === 0" icon="📭" text="该分类下暂无博客文章" />

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

            <!-- 底部：标签 + 阅读 -->
            <div class="bl-footer">
              <div class="bl-card-tags">
                <span v-for="t in blog.tags || []" :key="t.id" class="bl-card-tag" @click.stop="selectTag(t.name)"># {{ t.name }}</span>
              </div>
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
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { blogApi } from '@/api/social';

const route = useRoute();

const blogs = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 12;
const loading = ref(false);
const previewLength = 100;

const tagCloud = ref<{ name: string; count: number }[]>([]);
const currentTag = ref(typeof route.query.tag === 'string' ? route.query.tag : '');


function selectTag(tag: string) {
  currentTag.value = tag;
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
    const res = await blogApi.getTags();
    if (res.data.code === 0) tagCloud.value = res.data.data || [];
  } catch { /* ignore */ }
}

async function loadBlogs() {
  loading.value = true;
  try {
    const res = await blogApi.listPublic(page.value, pageSize, undefined, currentTag.value || undefined);
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
/* 页头改用全局 SectionHeader 组件 */

.bl-list { display: flex; flex-direction: column; gap: 16px; }

.bl-card {
  padding: 20px 24px;
  background: #fff; border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  cursor: pointer; transition: box-shadow 0.2s;
}
.bl-card:hover { box-shadow: var(--shadow-card-hover); }
[data-theme="dark"] .bl-card { background: #1a1d27; border-color: var(--border-color); }

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

/* 热门标签云（CSDN 式） */
.bl-tags { margin-top: 16px; padding: 12px; background: #fff; border: 1px solid var(--border-color); border-radius: 12px; }
[data-theme="dark"] .bl-tags { background: #1e293b; border-color: #334155; }
.bl-tags-title { font-size: 0.85rem; font-weight: 700; margin-bottom: 10px; color: var(--text-color); }
.bl-tags-cloud { display: flex; flex-wrap: wrap; gap: 6px; }
.bl-tag {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 3px 9px; border-radius: 999px;
  background: rgba(94,114,228,0.07); color: var(--primary-color);
  font-size: 0.78rem; cursor: pointer; transition: all 0.2s;
}
.bl-tag em { font-style: normal; font-size: 0.7rem; color: #999; }
.bl-tag:hover, .bl-tag.active { background: var(--primary-color); color: #fff; }
.bl-tag:hover em, .bl-tag.active em { color: rgba(255,255,255,0.8); }
.bl-tag-clear { margin-left: 8px; }
.bl-footer { display: flex; justify-content: space-between; align-items: center; gap: 10px; flex-wrap: wrap; }
.bl-card-tags { display: flex; flex-wrap: wrap; gap: 6px; flex: 1; min-width: 0; }
.bl-card-tag {
  font-size: 0.75rem; color: var(--primary-color);
  background: rgba(94,114,228,0.07);
  padding: 2px 8px; border-radius: 999px; cursor: pointer;
}
.bl-card-tag:hover { background: var(--primary-color); color: #fff; }
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
