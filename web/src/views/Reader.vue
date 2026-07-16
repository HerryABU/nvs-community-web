<template>
  <div class="reader-page" v-loading="loading">
    <!-- 顶部导航 -->
    <div class="reader-toolbar">
      <el-button text @click="$router.push(`/novel/${novelId}`)">
        <el-icon><ArrowLeft /></el-icon> 返回目录
      </el-button>
      <span class="reader-title">{{ novelTitle }} · 第{{ chapterNum }}章 {{ chapter?.title }}</span>
      <div class="reader-nav" v-if="!isTrialExceeded">
        <el-button size="small" :disabled="chapterNum <= 1" @click="goChapter(chapterNum - 1)">
          上一章
        </el-button>
        <el-button size="small" :disabled="chapterNum >= totalChapters" @click="goChapter(chapterNum + 1)">
          下一章
        </el-button>
      </div>
    </div>

    <!-- 自定义模板Header区域 -->
    <div v-if="headerFrames.length > 0" class="template-zone template-header">
      <div v-for="f in headerFrames" :key="f.id" class="template-frame-wrapper">
        <iframe
          :src="previewUrl(f.id) + '?chapter=' + chapterNum + '&novel=' + novelId"
          :sandbox="f.has_controls ? 'allow-scripts allow-same-origin allow-forms allow-popups' : 'allow-scripts allow-same-origin'"
          class="template-iframe"
          @load="onFrameLoaded($event)"
        />
      </div>
    </div>

    <!-- 正文 -->
    <div class="reader-body">
      <div v-if="isTrialExceeded" class="trial-notice">
        <el-result icon="warning" title="试读已结束" sub-title="未登录用户仅可阅读前 {{ trialLimit }} 章（约30%）。请登录后继续阅读全文。">
          <template #extra>
            <el-button type="primary" @click="$router.push('/login')">去登录</el-button>
            <el-button @click="$router.push(`/novel/${novelId}`)">返回作品页</el-button>
          </template>
        </el-result>
      </div>
      <div v-else class="reader-content">
        <div v-if="chapter?.content" class="reader-md-wrap">
          <div class="reader-md markdown-body" v-html="renderedHtml" ref="mdContainer"></div>
        </div>
        <p v-else-if="chapter" style="text-align:center;color:#f59e0b">章节已加载但内容为空（content长度=0）</p>
        <p v-else style="text-align:center;color:#999">暂无内容（chapter 为 null）</p>
      </div>
    </div>

    <!-- 底部导航 -->
    <div v-if="!isTrialExceeded" class="reader-footer">
      <el-button :disabled="chapterNum <= 1" @click="goChapter(chapterNum - 1)">上一章</el-button>
      <el-button :disabled="chapterNum >= totalChapters" @click="goChapter(chapterNum + 1)">下一章</el-button>
    </div>

    <!-- 自定义模板Footer区域 -->
    <div v-if="footerFrames.length > 0" class="template-zone template-footer">
      <div v-for="f in footerFrames" :key="f.id" class="template-frame-wrapper">
        <iframe
          :src="previewUrl(f.id) + '?chapter=' + chapterNum + '&novel=' + novelId"
          :sandbox="f.has_controls ? 'allow-scripts allow-same-origin allow-forms allow-popups' : 'allow-scripts allow-same-origin'"
          class="template-iframe"
          @load="onFrameLoaded($event)"
        />
      </div>
    </div>

    <!-- 评论 -->
    <div class="page-container" v-if="!isTrialExceeded">
      <h2 class="section-title">本章评论</h2>
      <CommentSection :novel-id="novelId" :chapter-number="chapterNum" :novel-category="novelCategory" />
    </div>

    <!-- 敏感分区确认弹窗 -->
    <SensitiveZoneGuard
      :visible="showZoneGuard"
      :zone-name="zoneGuardName"
      :is-cross-domain="zoneGuardCross"
      :custom-warning="novelWallWarning"
      @confirm="onZoneConfirmed"
      @cancel="onZoneCancelled"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { novelApi, chapterApi, type Chapter } from '@/api/novel';
import { bookshelfApi } from '@/api/bookshelf';
import { useAuthStore } from '@/stores/auth';
import { renderMarkdown, renderMermaidBlocks, renderChemfigBlocks } from '@/markdown/renderer';
import CommentSection from '@/components/CommentSection.vue';
import SensitiveZoneGuard from '@/components/SensitiveZoneGuard.vue';
import { shouldShowGuard, markZoneConfirmed, setLastZone } from '@/utils/sensitiveZone';
import { frameApi } from '@/api/frame';

const authStore = useAuthStore();

const route = useRoute();
const router = useRouter();

const novelId = ref(Number(route.params.id));
const chapterNum = ref(Number(route.params.chapter));
const loading = ref(false);
const chapter = ref<Chapter | null>(null);
const htmlContent = ref('');
const novelTitle = ref('');
const novelCategory = ref('');
const novelCategories = ref<string[]>([]);
const novelAuthorId = ref(0);
const novelWallEnabled = ref(true);
const novelWallWarning = ref('');
const totalChapters = ref(0);

// 自定义模板
const headerFrames = ref<any[]>([]);
const footerFrames = ref<any[]>([]);
const frameLoaded = ref<Record<number, boolean>>({});

function previewUrl(frameId: number) { return frameApi.getPreview(frameId); }
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
  try {
    const res = await frameApi.getByNovel(novelId.value);
    if (res.data.code === 0) {
      const frames = res.data.data || [];
      headerFrames.value = frames.filter((f: any) => f.position !== 'bottom');
      footerFrames.value = frames.filter((f: any) => f.position === 'bottom');
    }
  } catch { /* 静默失败，模板非必需 */ }
}

// 敏感分区确认
const showZoneGuard = ref(false);
const zoneGuardName = ref('');
const zoneGuardCross = ref(false);

const trialLimit = computed(() => Math.min(5, Math.ceil(totalChapters.value * 0.3)));
const isTrialExceeded = computed(() => !authStore.isLoggedIn && chapterNum.value > trialLimit.value);

function goChapter(num: number) {
  router.push(`/author/${novelAuthorId.value}/novel/${novelId.value}/read/${num}`);
}

async function loadChapter() {
  loading.value = true;
  try {
    const [chRes, novelRes] = await Promise.all([
      chapterApi.getChapter(novelId.value, chapterNum.value),
      novelApi.getNovel(novelId.value),
    ]);
    const detail = chRes.data.data;
    chapter.value = detail?.chapter || null;
    htmlContent.value = detail?.html_content || '';
    if (!chapter.value) {
      console.error('章节内容为空', detail);
    }
    novelTitle.value = novelRes.data.data.title;
    novelCategory.value = novelRes.data.data.category || '';
    novelCategories.value = novelRes.data.data.categories || (novelRes.data.data.category ? [novelRes.data.data.category] : []);
    novelAuthorId.value = novelRes.data.data.author_id || 0;
    novelWallEnabled.value = novelRes.data.data.wall_enabled !== false;
    novelWallWarning.value = novelRes.data.data.wall_warning || '';
    totalChapters.value = novelRes.data.data.total_chapters;
    localStorage.setItem(
      `reading-progress-${novelId.value}`,
      JSON.stringify({ chapter: chapterNum.value, time: Date.now() })
    );
    window.scrollTo(0, 0);

    // 加载自定义模板
    loadFrames();

    // 同步阅读进度到后端书架（静默失败）
    if (authStore.isLoggedIn) {
      try {
        await bookshelfApi.updateProgress(novelId.value, chapterNum.value);
      } catch {
        // 书架同步失败不阻断阅读
      }
    }

    // 敏感分区检查
    await checkSensitiveZone();
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

async function checkSensitiveZone() {
  // author custom wall: if wall_warning present and wall_enabled, trigger directly
  if (novelWallEnabled.value !== false && novelWallWarning.value) {
    zoneGuardName.value = novelCategory.value || novelTitle.value || 'wall';
    zoneGuardCross.value = false;
    showZoneGuard.value = true;
    return;
  }
  const cats = novelCategories.value.length > 0 ? novelCategories.value : (novelCategory.value ? [novelCategory.value] : []);
  if (cats.length === 0) return;
  const guard = await shouldShowGuard(cats, {
    authorId: novelAuthorId.value,
    userId: authStore.user?.id, wallEnabled: novelWallEnabled.value,
  });
  if (guard?.needed) {
    zoneGuardName.value = guard.zoneName;
    zoneGuardCross.value = guard.isCrossDomain;
    showZoneGuard.value = true;
  }
}

function onZoneConfirmed() {
  markZoneConfirmed(zoneGuardName.value);
  setLastZone(zoneGuardName.value);
  showZoneGuard.value = false;
}

function onZoneCancelled() {
  showZoneGuard.value = false;
  router.push('/');
}

watch(
  () => route.params.chapter,
  (newVal) => {
    chapterNum.value = Number(newVal);
    loadChapter();
  }
);

// ── Markdown 渲染 + Mermaid 异步处理 ──
const mdContainer = ref<HTMLElement | null>(null);
const renderedHtml = ref('');

watch(() => chapter.value?.content, (content) => {
  console.log('[NVS] Reader: content changed, length=', content?.length || 0);
  if (!content) { renderedHtml.value = ''; return; }
  try {
    const html = renderMarkdown(content);
    console.log('[NVS] Reader: rendered, html length=', html.length);
    renderedHtml.value = html;
  } catch (e: any) {
    console.error('[NVS] Reader: renderMarkdown error:', e);
    renderedHtml.value = '<p style=color:red>渲染失败: ' + (e.message || String(e)) + '</p>';
  }
  nextTick(() => {
    if (mdContainer.value) {
      renderMermaidBlocks(mdContainer.value);
      renderChemfigBlocks(mdContainer.value);
    }
  });
});

onMounted(() => {
  // 🔀 旧URL格式重定向：/novel/:id → 先获取作者再跳转
  if (!route.params.authorId) {
    novelApi.getNovel(novelId.value).then(res => {
      const aid = res.data?.data?.author_id;
      if (aid) router.replace(`/author/${aid}/novel/${novelId.value}/read/${chapterNum.value}`);
    }).catch(() => {});
  }
  loadChapter();
});
</script>

<style scoped>
.reader-page {
  background: var(--reader-bg);
  min-height: calc(100vh - 60px);
}

.reader-toolbar {
  position: sticky;
  top: 60px;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 24px;
  background: #fff;
  border-bottom: 1px solid var(--border-color);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.05);
}

[data-theme="dark"] .reader-toolbar {
  background: #1e293b;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.3);
}

.reader-title {
  flex: 1;
  font-weight: 500;
  color: var(--text-color);
}

.reader-body {
  padding: 40px 16px;
}

.template-zone {
  max-width: 800px;
  margin: 0 auto 12px;
}
.template-header { padding: 0 16px; margin-top: 8px; }
.template-footer { padding: 0 16px; margin-bottom: 16px; }
.template-frame-wrapper {
  border-radius: 8px; overflow: hidden;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  margin-bottom: 8px;
}
.template-iframe {
  width: 100%; min-height: 150px; border: none;
  display: block;
}

.reader-content {
  max-width: 800px;
  margin: 0 auto;
  padding: 32px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
}

[data-theme="dark"] .reader-content {
  background: #1e293b;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.3);
}

.reader-md-wrap {
  min-height: 200px;
}

.reader-md {
  font-family: Georgia, 'Noto Serif SC', serif;
  font-size: 1.1rem;
  line-height: 2;
  color: #333;
}

[data-theme="dark"] .reader-md {
  color: #cbd5e1;
}

.reader-md :deep(p) {
  margin-bottom: 1em;
  text-indent: 2em;
  color: #1a1a2e;
}

[data-theme="dark"] .reader-md :deep(p) {
  color: #e2e8f0;
}

.reader-md :deep(h1) {
  font-size: 2.2rem;
  text-align: center;
  margin: 2em 0 1em;
  text-indent: 0;
  font-weight: 700;
  color: #1a1a2e;
}
.reader-md :deep(h2) {
  font-size: 1.7rem;
  text-align: center;
  margin: 1.8em 0 0.8em;
  text-indent: 0;
  font-weight: 700;
  color: #1a1a2e;
}
.reader-md :deep(h3) {
  font-size: 1.4rem;
  text-align: center;
  margin: 1.5em 0 0.6em;
  text-indent: 0;
  font-weight: 700;
  color: #1a1a2e;
}
.reader-md :deep(h4),
.reader-md :deep(h5),
.reader-md :deep(h6) {
  margin: 1.5em 0 0.8em;
  text-indent: 0;
  font-weight: 700;
  color: #1a1a2e;
}

[data-theme="dark"] .reader-md :deep(h1),
[data-theme="dark"] .reader-md :deep(h2),
[data-theme="dark"] .reader-md :deep(h3) {
  color: #f1f5f9;
}
[data-theme="dark"] .reader-md :deep(h4),
[data-theme="dark"] .reader-md :deep(h5),
[data-theme="dark"] .reader-md :deep(h6) {
  color: #f1f5f9;
}

.reader-md :deep(ul),
.reader-md :deep(ol) {
  padding-left: 1.5em;
  margin-bottom: 1em;
}

.reader-md :deep(li) {
  margin-bottom: 0.3em;
}

.reader-md :deep(a) {
  color: #2563eb;
  text-decoration: none;
}

[data-theme="dark"] .reader-md :deep(a) {
  color: #60a5fa;
}

.reader-md :deep(strong) {
  font-weight: 700;
  color: #1a1a2e;
}

[data-theme="dark"] .reader-md :deep(strong) {
  color: #f1f5f9;
}

.reader-md :deep(blockquote) {
  border-left: 4px solid #e67e22;
  padding: 10px 18px;
  margin: 14px 0;
  background: #fef9f3;
  color: #4a4a5a;
  border-radius: 0 6px 6px 0;
}

[data-theme="dark"] .reader-md :deep(blockquote) {
  background: #1e293b;
  color: #cbd5e1;
  border-left-color: #f59e0b;
}

.reader-md :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin: 14px 0;
}

.reader-md :deep(th),
.reader-md :deep(td) {
  border: 1px solid #d1d5db;
  padding: 8px 14px;
  text-align: left;
}

[data-theme="dark"] .reader-md :deep(th),
[data-theme="dark"] .reader-md :deep(td) {
  border-color: #374151;
}

.reader-md :deep(th) {
  background: #f3f4f6;
  font-weight: 600;
}

[data-theme="dark"] .reader-md :deep(th) {
  background: #1f2937;
}

.reader-md :deep(hr) {
  border: none;
  border-top: 1px solid #e5e7eb;
  margin: 24px 0;
}

[data-theme="dark"] .reader-md :deep(hr) {
  border-top-color: #374151;
}

/* Markdown 正文排版（基于 .markdown-body） */
.markdown-body {
  color: #1a1a2e;
  font-size: 1.05rem;
  line-height: 2;
}
[data-theme="dark"] .markdown-body {
  color: #e2e8f0;
}

/* KaTeX 公式 */
.markdown-body .katex { font-size: 1.1em; text-indent: 0; }
.markdown-body .katex-display { margin: 1.2em 0; text-align: center; }
.markdown-body .katex-display > .katex { display: inline-block; }

/* Mermaid 图表容器 */
.markdown-body .mermaid-container {
  text-align: center;
  margin: 1em 0;
  overflow-x: auto;
  background: #fff;
}
.markdown-body .mermaid-container svg { max-width: 100%; height: auto; }
[data-theme="dark"] .markdown-body .mermaid-container { background: #1e293b; }

/* 代码块（Prism） */
.markdown-body pre {
  background: #1e1e1e;
  border-radius: 8px;
  padding: 16px 20px;
  overflow-x: auto;
  margin: 14px 0;
}
[data-theme="dark"] .markdown-body pre { background: #111827; }
.markdown-body pre code {
  color: #e0e0e0;
  font-family: 'Fira Code', 'Cascadia Code', 'JetBrains Mono', Consolas, monospace;
  font-size: 0.9rem;
  line-height: 1.6;
}

.reader-footer {
  display: flex;
  justify-content: center;
  gap: 24px;
  padding: 24px;
}

.trial-notice {
  display: flex;
  justify-content: center;
  padding: 60px 16px;
}

@media (max-width: 768px) {
  .reader-toolbar {
    flex-wrap: wrap;
    gap: 8px;
  }
  .reader-title {
    width: 100%;
    order: -1;
    text-align: center;
    font-size: 0.9rem;
  }
  .reader-content {
    padding: 16px;
  }
}
</style>

<style>
[data-theme="dark"] .reader-body {
  background: transparent;
}
[data-theme="dark"] .trial-notice .el-result__title {
  color: #e2e8f0;
}
[data-theme="dark"] .trial-notice .el-result__subtitle {
  color: #94a3b8;
}
/* Mermaid / KaTeX 背景/颜色修复已统一在 markdown/global-fix.css */
</style>