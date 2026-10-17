<template>
  <div class="page-container blog-detail" :class="{ 'toc-collapsed': !tocPanelOpen }" v-loading="loading">
    <template v-if="blog">
      <div class="bd-layout">
        <!-- 正文区 -->
        <div class="bd-main">
          <div class="blog-header">
            <div class="bh-title-row">
              <!-- 目录面板折叠（全屏阅读）时显示展开按钮 -->
              <el-button v-if="!tocPanelOpen && toc.length > 0" size="small" text type="primary" class="bd-toc-restore" @click="tocPanelOpen = true">
                <el-icon style="margin-right:2px"><Expand /></el-icon>展开目录
              </el-button>
              <h1>{{ blog.title }}</h1>
              <el-tag v-if="blog.is_pinned" size="small" type="danger">置顶</el-tag>
            </div>
            <div class="blog-meta-line">
              <router-link :to="`/author/${blog.author_id}`" class="blog-author">
                {{ blog.author?.nickname || blog.author?.username || '作者' }}
              </router-link>
              <el-tag v-if="blog.category" size="small">{{ blog.category }}</el-tag>
              <span>{{ formatDate(blog.created_at) }}</span>
              <span>{{ blog.view_count }} 次阅读</span>
            </div>
            <!-- 标签（#tag，CSDN 式，点击跳转全站标签筛选） -->
            <div class="blog-tag-bar" v-if="blog.tags && blog.tags.length">
              <span
                v-for="t in blog.tags"
                :key="t.id"
                class="blog-tag"
                @click="router.push({ path: '/blogs', query: { tag: t.name } })"
              ># {{ t.name }}</span>
            </div>
            <!-- 话题（#话题，点击跳转话题页） -->
            <div class="blog-topic-bar" v-if="blog.topics && blog.topics.length">
              <span class="bt-label">参与话题：</span>
              <span
                v-for="t in blog.topics"
                :key="t.id"
                class="blog-topic"
                @click="router.push(`/topics/${t.id}`)"
              ># {{ t.name }}</span>
            </div>
            <!-- 点赞区 -->
            <div class="blog-like-bar">
              <el-button
                :type="liked ? 'danger' : 'default'"
                :plain="!liked"
                round
                size="small"
                :loading="liking"
                @click="toggleLike"
              >
                <el-icon style="margin-right:4px"><CaretTop v-if="!liked" /><Check v-else /></el-icon>
                点赞 {{ likeCount > 0 ? likeCount : '' }}
              </el-button>
            </div>
          </div>

          <!-- 自定义模板Header区域（作者绑定的阅读模版，渲染在正文上方） -->
          <div v-if="headerFrames.length > 0" class="template-zone template-header">
            <div v-for="f in headerFrames" :key="f.id" class="template-frame-wrapper">
              <!-- 非沙盒注入：作者本人页面直接同源嵌入（无 sandbox 限制） -->
              <iframe
                v-if="isInlineFrame(f)"
                :src="previewUrl(f.id) + '?blog=' + blog.id"
                class="template-iframe"
                @load="onFrameLoaded($event)"
              />
              <!-- 沙盒模式：受限 iframe -->
              <iframe
                v-else
                :src="previewUrl(f.id) + '?blog=' + blog.id"
                :sandbox="f.has_controls ? 'allow-scripts allow-forms allow-popups' : 'allow-scripts'"
                class="template-iframe"
                @load="onFrameLoaded($event)"
              />
            </div>
          </div>

          <div class="blog-content markdown-body cherry-markdown cherry-previewer" data-code-block-theme="default" data-inline-code-theme="red" v-html="htmlContent" ref="blogContentRef"></div>

          <!-- 自定义模板Footer区域（position=footer 的模版，渲染在正文下方） -->
          <div v-if="footerFrames.length > 0" class="template-zone template-footer">
            <div v-for="f in footerFrames" :key="f.id" class="template-frame-wrapper">
              <iframe
                v-if="isInlineFrame(f)"
                :src="previewUrl(f.id) + '?blog=' + blog.id"
                class="template-iframe"
                @load="onFrameLoaded($event)"
              />
              <iframe
                v-else
                :src="previewUrl(f.id) + '?blog=' + blog.id"
                :sandbox="f.has_controls ? 'allow-scripts allow-forms allow-popups' : 'allow-scripts'"
                class="template-iframe"
                @load="onFrameLoaded($event)"
              />
            </div>
          </div>

          <div class="blog-footer">
            <el-button @click="$router.back()">返回</el-button>
            <router-link :to="`/author/${blog.author_id}`">
              <el-button type="primary" plain>查看作者主页</el-button>
            </router-link>
            <ReportButton
              v-if="authStore.isLoggedIn && authStore.user?.id !== blog.author_id"
              target-type="blog"
              :target-id="blog.id"
            />
          </div>

          <!-- 评论区 -->
          <h2 class="section-title" style="margin-top: 40px">评论（{{ commentTotal }}）</h2>
          <CommentSection :novel-id="0" :novel-category="''" :blog-id="blog.id" :owner-id="blog.author_id" @loaded="(n: number) => commentTotal = n" />
        </div>

        <!-- 右侧目录（CSDN 式可折叠，仅 md 且有标题时显示） -->
        <aside class="bd-toc" v-if="tocPanelOpen && toc.length > 0">
          <div class="bd-toc-title">
            <span>📑 文章目录</span>
            <el-tooltip content="折叠目录，全屏阅读" placement="left">
              <el-icon class="bd-toc-fold" @click="tocPanelOpen = false"><Fold /></el-icon>
            </el-tooltip>
          </div>
          <div class="bd-toc-tree">
            <div v-for="item in visibleToc" :key="item.id" class="bd-toc-item" :style="{ paddingLeft: (item.level - 1) * 14 + 'px' }">
              <span
                class="bd-toc-text"
                :class="{ active: activeHeading === item.id }"
                @click="scrollToHeading(item.id)"
              >{{ item.text }}</span>
              <span
                v-if="hasChildren(item.id)"
                class="bd-toc-toggle"
                @click="toggleHeading(item.id)"
              >{{ isCollapsed(item.id) ? '▸' : '▾' }}</span>
            </div>
          </div>
        </aside>
      </div>
    </template>
    <el-empty v-else-if="!loading" description="博客不存在" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, onBeforeUnmount } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { blogApi } from '@/api/social';
import { frameApi } from '@/api/frame';
import { renderMarkdown, renderMermaidBlocks, renderChemfigBlocks } from '@/markdown/renderer';
import CommentSection from '@/components/CommentSection.vue';
import ReportButton from '@/components/ReportButton.vue';
import { useAuthStore } from '@/stores/auth';
import { resolveUserKey } from '@/utils/userKey';
import { ElMessage } from 'element-plus';
import { CaretTop, Check, Fold, Expand } from '@element-plus/icons-vue';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const blog = ref<any>(null);
const htmlContent = ref('');
const loading = ref(false);
const blogContentRef = ref<HTMLElement | null>(null);
const commentTotal = ref(0);

// 目录面板展开状态：折叠后正文全屏阅读
const tocPanelOpen = ref(true);

// ===== 点赞 =====
const liked = ref(false);
const likeCount = ref(0);
const liking = ref(false);

// ===== 自定义阅读模版（与 Reader.vue 同一套机制：header 正文上方 / footer 正文下方）=====
const headerFrames = ref<any[]>([]);
const footerFrames = ref<any[]>([]);

function previewUrl(frameId: number) { return frameApi.getPreview(frameId); }

// 非沙盒注入判定：仅当模板声明 inline 且当前访问者是模板作者本人时生效
function isInlineFrame(f: any): boolean {
  return f.run_mode === 'inline' && authStore.user?.id === f.user_id;
}

function onFrameLoaded(e: Event) {
  const iframe = e.target as HTMLIFrameElement;
  try {
    // 自适应高度：根据iframe内部内容调整
    const body = iframe.contentDocument?.body;
    if (body) {
      const h = body.scrollHeight;
      if (h > 60) iframe.style.height = h + 'px';
    }
  } catch {
    // 跨域限制下无法读取contentDocument，使用默认高度
    iframe.style.height = '200px';
  }
}

async function loadFrames() {
  if (!blog.value) return;
  try {
    const res = await frameApi.getByBlog(blog.value.id);
    if (res.data.code === 0) {
      const frames = res.data.data || [];
      // position 为 footer 的模板放正文下方；空值或其它值默认 header（向后兼容）
      headerFrames.value = frames.filter((f: any) => f.position !== 'footer');
      footerFrames.value = frames.filter((f: any) => f.position === 'footer');
    }
  } catch { /* 静默失败，模板非必需 */ }
}

async function loadLikes() {
  if (!blog.value) return;
  try {
    const res = await blogApi.getLikes(blog.value.id);
    if (res.data.code === 0) {
      likeCount.value = res.data.data.count;
      liked.value = res.data.data.liked;
    }
  } catch { /* ignore */ }
}

async function toggleLike() {
  if (!authStore.isLoggedIn) { ElMessage.warning('请先登录'); return; }
  if (!blog.value) return;
  liking.value = true;
  try {
    const res = await blogApi.toggleLike(blog.value.id);
    if (res.data.code === 0) {
      liked.value = res.data.data.liked;
      likeCount.value = res.data.data.count;
      blog.value.like_count = res.data.data.count;
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally { liking.value = false; }
}

// ===== 可折叠目录（#/##/###…）=====
interface TocItem { id: string; text: string; level: number; }
const toc = ref<TocItem[]>([]);
const collapsedSet = ref<Set<string>>(new Set());
const activeHeading = ref('');

// 可见目录：父级被折叠时隐藏其下所有子标题
const visibleToc = computed(() =>
  toc.value.filter(item => {
    const idx = toc.value.findIndex(t => t.id === item.id);
    for (let i = idx - 1; i >= 0; i--) {
      if (toc.value[i].level < item.level && collapsedSet.value.has(toc.value[i].id)) return false;
    }
    return true;
  })
);

function hasChildren(id: string) {
  const idx = toc.value.findIndex(t => t.id === id);
  if (idx < 0) return false;
  const level = toc.value[idx].level;
  for (let i = idx + 1; i < toc.value.length; i++) {
    if (toc.value[i].level <= level) return false;
    return true;
  }
  return false;
}

function isCollapsed(id: string) { return collapsedSet.value.has(id); }

function toggleHeading(id: string) {
  const s = new Set(collapsedSet.value);
  if (s.has(id)) s.delete(id); else s.add(id);
  collapsedSet.value = s;
}

function scrollToHeading(id: string) {
  const el = document.getElementById(id);
  if (el) {
    activeHeading.value = id;
    el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    window.history.replaceState(null, '', `#${id}`);
  }
}

function buildToc() {
  if (!blogContentRef.value) return;
  const heads = blogContentRef.value.querySelectorAll('h1, h2, h3, h4, h5, h6');
  const items: TocItem[] = [];
  heads.forEach((h, i) => {
    const level = parseInt(h.tagName.substring(1), 10);
    if (!h.id) h.id = `bd-toc-${i}`;
    items.push({ id: h.id, text: (h.textContent || '').trim(), level });
  });
  toc.value = items;
}

function onScroll() {
  if (!blogContentRef.value || toc.value.length === 0) return;
  const heads = blogContentRef.value.querySelectorAll('h1, h2, h3, h4, h5, h6');
  let current = '';
  for (const h of Array.from(heads) as HTMLElement[]) {
    if (h.getBoundingClientRect().top <= 90) current = h.id;
  }
  if (current) activeHeading.value = current;
}

function formatDate(d?: string) {
  if (!d) return '';
  return new Date(d).toLocaleString('zh-CN');
}

onMounted(async () => {
  // 兼容两种路由：/author/:key/blogs/:blogId（:key 可为数字主键或用户名）与旧 /blog/:id
  const blogId = Number(route.params.blogId || route.params.id);
  loading.value = true;
  try {
    let res;
    if (route.params.blogId) {
      // 统一索引：用户名/数字主键 一律走 resolveUserKey（禁止 Number("qbz95")=NaN）
      const author = await resolveUserKey(route.params.id);
      if (!author) {
        blog.value = null;
        return;
      }
      res = await blogApi.getAuthorBlog(author.id, blogId);
    } else {
      res = await blogApi.getBlog(blogId);
    }
    if (res.data.code === 0) {
      blog.value = res.data.data;
      htmlContent.value = renderMarkdown(blog.value.content || '');
      nextTick(() => {
        if (blogContentRef.value) {
          renderMermaidBlocks(blogContentRef.value);
          renderChemfigBlocks(blogContentRef.value);
          buildToc();
        }
      });
      loadLikes();
      loadFrames();
    }
  } catch { /* ignore */ }
  finally { loading.value = false; }

  window.addEventListener('scroll', onScroll, { passive: true });
});

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll);
});
</script>

<style scoped>
.page-container { max-width: 860px; margin: 0 auto; padding: 24px; transition: max-width 0.25s ease; }
/* 目录折叠 → 正文全屏阅读 */
.page-container.toc-collapsed { max-width: 100%; }
.bd-layout { display: flex; gap: 24px; align-items: flex-start; }
.bd-main { flex: 1; min-width: 0; }

/* 折叠目录后的全屏正文：行宽过宽时适度收窄，保持可读性 */
.toc-collapsed .blog-content { max-width: 920px; margin-left: auto; margin-right: auto; }
.toc-collapsed .blog-footer, .toc-collapsed .section-title, .toc-collapsed .blog-header { max-width: 920px; margin-left: auto; margin-right: auto; }

.blog-header { margin-bottom: 24px; padding-bottom: 16px; border-bottom: 1px solid #eee; }

/* ===== 自定义阅读模版区（与 Reader.vue 同构）===== */
.template-zone { margin: 0 0 12px; }
.template-header { margin-top: 8px; }
.template-footer { margin-bottom: 16px; }
.template-frame-wrapper {
  border-radius: 8px; overflow: hidden;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  margin-bottom: 8px;
}
.template-iframe { width: 100%; min-height: 150px; border: none; display: block; }
.bh-title-row { display: flex; align-items: center; gap: 10px; }
.bd-toc-restore { flex-shrink: 0; }
.blog-header h1 { font-size: 1.5rem; margin-bottom: 8px; }
.blog-meta-line {
  display: flex; align-items: center; gap: 14px;
  font-size: 0.85rem; color: #999; flex-wrap: wrap;
}
.blog-author { font-weight: 600; color: var(--primary-color); text-decoration: none; }
.blog-tag-bar { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 14px; }
.blog-tag {
  display: inline-flex; align-items: center;
  padding: 3px 10px; border-radius: 999px;
  background: rgba(94,114,228,0.08); color: var(--primary-color);
  font-size: 0.8rem; cursor: pointer; transition: all 0.2s;
}
.blog-tag:hover { background: var(--primary-color); color: #fff; }
.blog-topic-bar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 8px; }
.bt-label { font-size: 0.8rem; color: #999; }
.blog-topic {
  display: inline-flex; align-items: center;
  padding: 3px 12px; border-radius: 6px;
  background: rgba(245,158,11,0.12); color: #b45309;
  font-size: 0.8rem; cursor: pointer; transition: all 0.2s;
}
.blog-topic:hover { background: #f59e0b; color: #fff; }
[data-theme="dark"] .blog-topic { color: #fbbf24; }
.blog-like-bar { margin-top: 14px; }
.blog-content { line-height: 1.8; font-size: 1rem; }
.blog-footer { margin-top: 32px; display: flex; gap: 12px; }
.section-title { font-size: 1.15rem; font-weight: 600; margin-bottom: 16px; }

/* ===== 右侧目录 ===== */
.bd-toc {
  width: 220px;
  flex-shrink: 0;
  position: sticky;
  top: 80px;
  max-height: calc(100vh - 120px);
  overflow-y: auto;
  background: #fff;
  border: 1px solid #eee;
  border-radius: 12px;
  padding: 14px 12px;
}
[data-theme="dark"] .bd-toc { background: #1e293b; border-color: #334155; }
.bd-toc-title {
  font-size: 0.9rem; font-weight: 700; margin-bottom: 10px; color: var(--text-color);
  display: flex; align-items: center; justify-content: space-between;
}
.bd-toc-fold {
  cursor: pointer; color: #aaa; font-size: 0.95rem;
  padding: 2px; border-radius: 4px; transition: color 0.15s, background 0.15s;
}
.bd-toc-fold:hover { color: var(--primary-color); background: rgba(94, 114, 228, 0.1); }
.bd-toc-tree { display: flex; flex-direction: column; gap: 2px; }
.bd-toc-item { display: flex; align-items: center; gap: 4px; font-size: 0.82rem; line-height: 1.5; }
.bd-toc-text {
  flex: 1;
  color: #666;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 3px 6px;
  border-radius: 6px;
  transition: background 0.15s, color 0.15s;
}
.bd-toc-text:hover { background: #f5f7fa; color: var(--primary-color); }
.bd-toc-text.active { background: rgba(94, 114, 228, 0.12); color: var(--primary-color); font-weight: 600; }
[data-theme="dark"] .bd-toc-text { color: #94a3b8; }
[data-theme="dark"] .bd-toc-text:hover { background: rgba(255,255,255,0.05); }
.bd-toc-toggle {
  font-size: 0.7rem; color: #aaa; cursor: pointer;
  padding: 2px 4px; user-select: none; flex-shrink: 0;
}
.bd-toc-toggle:hover { color: var(--primary-color); }

/* ====== 移动端适配 ====== */
@media (max-width: 768px) {
  .page-container { max-width: 100%; padding: 14px; }
  .bd-layout { flex-direction: column; }
  .bd-toc {
    width: 100%;
    position: static;
    max-height: 260px;
    order: -1;
    margin-bottom: 12px;
  }
  .blog-header h1 {
    font-size: 1.3rem;
    line-height: 1.45;
  }
  .blog-meta-line { flex-wrap: wrap; gap: 6px 12px; }
  .blog-content { font-size: 0.95rem; }
  .blog-footer { flex-wrap: wrap; }
  .blog-footer .el-button { flex: 1; }
}
</style>
