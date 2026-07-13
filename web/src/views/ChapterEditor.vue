<template>
  <div class="chapter-editor">
    <!-- 顶部工具栏 -->
    <div class="ce-toolbar">
      <div class="ce-toolbar-left">
        <el-button text @click="goBack">
          <el-icon><ArrowLeft /></el-icon>返回作品
        </el-button>
        <span class="ce-novel-title">{{ novelTitle }}</span>
      </div>
        <div class="ce-toolbar-center">
          <el-radio-group v-model="editorMode" size="small" @change="onModeChange">
            <el-radio-button value="md">Markdown</el-radio-button>
            <el-radio-button value="richtext">富文本</el-radio-button>
          </el-radio-group>
        </div>
      <div class="ce-toolbar-right">
        <span v-if="lastSaved" class="ce-saved-time">已保存 {{ lastSaved }}</span>
        <el-button type="primary" :loading="saving" @click="saveChapter">保存章节</el-button>
      </div>
    </div>

    <div class="ce-body">
      <!-- 左侧章节列表 -->
      <aside class="ce-sidebar">
        <div class="ce-sidebar-header">
          <span>章节列表</span>
          <el-button size="small" type="primary" circle @click="addChapter">
            <el-icon><Plus /></el-icon>
          </el-button>
        </div>
        <div class="ce-chapter-list">
          <div
            v-for="ch in chapters"
            :key="ch.chapter_number"
            :class="['ce-chapter-item', { active: currentNum === ch.chapter_number }]"
            @click="switchChapter(ch.chapter_number)"
          >
            <span class="ce-ch-num">{{ ch.chapter_number }}</span>
            <span class="ce-ch-title">{{ ch.title || '无标题' }}</span>
            <span class="ce-ch-words">{{ ch.word_count }}字</span>
            <el-popconfirm
              title="确认删除此章节？"
              @confirm="deleteChapterItem(ch.chapter_number)"
            >
              <template #reference>
                <el-button size="small" text type="danger" class="ce-ch-del">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </aside>

      <!-- 编辑区 -->
      <main class="ce-main">
        <div class="ce-editor-header">
          <el-input
            v-model="chapterTitle"
            placeholder="章节标题（如：第一章 开端）"
            size="large"
            class="ce-title-input"
          />
        </div>

        <!-- Cherry Markdown 编辑器 -->
        <div
          v-show="editorMode === 'md'"
          class="ce-md-wrapper"
        >
          <div ref="cherryContainer" class="ce-cherry-container"></div>
        </div>

        <!-- 富文本编辑器 -->
        <div v-show="editorMode === 'richtext'" class="ce-richtext-editor">
          <RichTextEditor
            v-model="richTextContent"
            placeholder="开始写作..."
          />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { novelApi, chapterApi, type Chapter } from '@/api/novel';
import { ElMessage } from 'element-plus';
import { ArrowLeft, Plus, Delete } from '@element-plus/icons-vue';
import RichTextEditor from '@/components/RichTextEditor.vue';
import TurndownService from 'turndown';
import { renderMarkdown, engineConfig } from '@/markdown/renderer';
import katex from 'katex';

// mhchem 副作用注册（与 renderer.ts 保持一致）
import 'katex/dist/contrib/mhchem.mjs';

const route = useRoute();
const router = useRouter();

const novelId = computed(() => Number(route.params.id));
const currentNum = ref(Number(route.params.num) || 1);

const novelTitle = ref('');
const chapters = ref<Chapter[]>([]);
const chapterTitle = ref('');
const chapterContent = ref('');       // Markdown 源文本
const richTextContent = ref('');      // 富文本 HTML 内容
const editorMode = ref<'md' | 'richtext'>('md');
const saving = ref(false);
const lastSaved = ref('');

// Cherry 编辑器实例
const cherryContainer = ref<HTMLDivElement | null>(null);
let cherryInstance: any = null;
let cherryReady = false;

// turndown 实例（HTML → Markdown）
const turndown = new TurndownService({
  headingStyle: 'atx',
  hr: '---',
  bulletListMarker: '-',
  codeBlockStyle: 'fenced',
});

// 获取当前要保存的内容（富文本模式先转换）
function getContentToSave(): string {
  if (editorMode.value === 'richtext' && richTextContent.value.trim()) {
    return htmlToMd(richTextContent.value);
  }
  // MD 模式：如果 Cherry 已初始化，从 Cherry 取值；否则用 chapterContent
  if (editorMode.value === 'md' && cherryInstance) {
    return cherryInstance.getValue() || chapterContent.value;
  }
  return chapterContent.value;
}

// HTML → Markdown 转换
function htmlToMd(html: string): string {
  if (!html) return '';
  return turndown.turndown(html);
}

// ─── Cherry 编辑器初始化 ───

async function initCherryEditor() {
  if (!cherryContainer.value) return;
  const Cherry = (await import('cherry-markdown')).default;
  cherryInstance = new Cherry({
    el: cherryContainer.value,
    value: chapterContent.value,
    editor: {
      defaultModel: 'edit&preview',
      height: `${editorHeight.value}px`,
    },
    toolbars: {
      toolbar: [
        'undo', 'redo', '|',
        'bold', 'italic', 'strikethrough', 'underline', '|',
        'header', '|',
        'ul', 'ol', 'checklist', 'quote', 'code', '|',
        'table', '|',
        'formula', 'graph', '|',
        'image', 'link', '|',
        'hr', '|',
        'toc', 'switchModel', 'fullScreen',
      ],
      toolbarRight: ['togglePreview'],
    },
    // 使用与阅读器相同的引擎配置
    ...engineConfig,
    // 编辑器独有的配置
    callback: {
      afterChange: (text: string) => {
        chapterContent.value = text;
      },
    },
  });
  cherryReady = true;
}

function destroyCherryEditor() {
  if (cherryInstance) {
    try { cherryInstance.destroy?.(); } catch {}
    cherryInstance = null;
  }
  cherryReady = false;
}

// 编辑器高度
const editorHeight = computed(() => window.innerHeight - 160);

// ─── 模式切换 ───

function onModeChange(mode: string) {
  if (mode === 'richtext') {
    // Markdown → 富文本：从 Cherry 或 chapterContent 取 Markdown，转为 HTML
    const mdContent = cherryInstance ? (cherryInstance.getValue() || '') : chapterContent.value;
    richTextContent.value = renderMarkdown(mdContent);
  } else if (mode === 'md') {
    // 富文本 → Markdown
    if (richTextContent.value.trim()) {
      const md = htmlToMd(richTextContent.value);
      chapterContent.value = md;
      if (cherryInstance) {
        cherryInstance.setValue(md);
      }
    }
    // 确保 Cherry 编辑器已初始化
    if (!cherryReady) {
      nextTick(() => initCherryEditor());
    }
  }
}

// ─── 章节操作 ───

async function loadNovel() {
  try {
    const [novelRes, chaptersRes] = await Promise.all([
      novelApi.getNovel(novelId.value),
      chapterApi.getChapters(novelId.value),
    ]);
    novelTitle.value = novelRes.data.data.title;
    chapters.value = chaptersRes.data.data || [];

    if (chapters.value.length === 0) {
      chapters.value = [{
        id: 0, novel_id: novelId.value, chapter_number: 1,
        title: '', content: '', word_count: 0, status: 'draft',
        created_at: '',
      }];
    }
    loadCurrentChapter();
  } catch {
    ElMessage.error('加载作品失败');
    router.push('/author');
  }
}

async function loadCurrentChapter() {
  try {
    const res = await chapterApi.getChapter(novelId.value, currentNum.value);
    const detail = res.data.data;
    const ch = detail.chapter;
    chapterTitle.value = ch.title || '';
    const content = ch.content || '';
    chapterContent.value = content;

    if (editorMode.value === 'md' && cherryInstance) {
      cherryInstance.setValue(content);
    } else if (editorMode.value === 'md' && !cherryReady) {
      // 等待 Cherry 初始化后再设置值——通过 chapterContent 同步
    }
    richTextContent.value = renderMarkdown(content);
  } catch {
    chapterTitle.value = `第${currentNum.value}章`;
    chapterContent.value = '';
    richTextContent.value = '';
    if (editorMode.value === 'md' && cherryInstance) {
      cherryInstance.setValue('');
    }
  }
}

async function switchChapter(num: number) {
  if (num === currentNum.value) return;
  await saveChapterSilent();
  currentNum.value = num;
  router.replace(`/author/editor/${novelId.value}/chapter/${num}`);
  await loadCurrentChapter();
}

async function addChapter() {
  await saveChapterSilent();
  const newNum = chapters.value.length > 0
    ? Math.max(...chapters.value.map(c => c.chapter_number)) + 1
    : 1;
  chapterTitle.value = `第${newNum}章`;
  chapterContent.value = '';
  richTextContent.value = '';
  currentNum.value = newNum;
  if (cherryInstance) cherryInstance.setValue('');
  chapters.value.push({
    id: 0, novel_id: novelId.value, chapter_number: newNum,
    title: chapterTitle.value, content: '', word_count: 0, status: 'draft',
    created_at: '',
  });
  router.replace(`/author/editor/${novelId.value}/chapter/${newNum}`);
}

async function saveChapter() {
  if (!chapterTitle.value.trim()) {
    ElMessage.warning('请输入章节标题');
    return;
  }
  const contentToSave = getContentToSave();
  saving.value = true;
  try {
    const existing = chapters.value.find(c => c.chapter_number === currentNum.value && c.id > 0);
    if (existing) {
      await chapterApi.updateChapter(novelId.value, currentNum.value, {
        title: chapterTitle.value,
        content: contentToSave,
      });
    } else {
      await chapterApi.createChapter(novelId.value, {
        title: chapterTitle.value,
        content: contentToSave,
      });
    }
    const idx = chapters.value.findIndex(c => c.chapter_number === currentNum.value);
    if (idx >= 0) {
      chapters.value[idx].title = chapterTitle.value;
      chapters.value[idx].word_count = contentToSave.length;
      if (chapters.value[idx].id === 0) chapters.value[idx].id = -1;
    }
    const now = new Date();
    lastSaved.value = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`;
    ElMessage.success('已保存');
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function saveChapterSilent() {
  const contentToSave = getContentToSave();
  if (!chapterTitle.value.trim() && !contentToSave.trim()) return;
  try {
    const existing = chapters.value.find(c => c.chapter_number === currentNum.value && c.id > 0);
    if (existing) {
      await chapterApi.updateChapter(novelId.value, currentNum.value, {
        title: chapterTitle.value,
        content: contentToSave,
      });
    } else if (chapterTitle.value.trim()) {
      await chapterApi.createChapter(novelId.value, {
        title: chapterTitle.value,
        content: contentToSave,
      });
    }
  } catch { /* 静默保存失败不提示 */ }
}

async function deleteChapterItem(num: number) {
  try {
    await chapterApi.deleteChapter(novelId.value, num);
    chapters.value = chapters.value.filter(c => c.chapter_number !== num);
    if (currentNum.value === num && chapters.value.length > 0) {
      switchChapter(chapters.value[0].chapter_number);
    }
    ElMessage.success('已删除');
  } catch {
    ElMessage.error('删除失败');
  }
}

function goBack() {
  saveChapterSilent();
  router.push(`/author/editor/${novelId.value}`);
}

// ─── 快捷键 ───

function onKeyDown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault();
    saveChapter();
  }
}

// ─── 生命周期 ───

onMounted(() => {
  window.addEventListener('keydown', onKeyDown);
  loadNovel();
  // 初始模式为 MD 时初始化 Cherry
  if (editorMode.value === 'md') {
    nextTick(() => initCherryEditor());
  }
});

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeyDown);
  destroyCherryEditor();
});
</script>

<style scoped>
.chapter-editor {
  position: fixed;
  top: 60px;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  background: #fff;
  z-index: 50;
}

/* 顶部工具栏 */
.ce-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  padding: 0 16px;
  border-bottom: 1px solid #e8e8e8;
  background: #fafafa;
  flex-shrink: 0;
}

.ce-toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.ce-novel-title {
  font-weight: 600;
  color: var(--primary-color);
  font-size: 0.95rem;
}

.ce-toolbar-center {
  display: flex;
  align-items: center;
}

.ce-toolbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.ce-saved-time {
  font-size: 0.8rem;
  color: #999;
}

/* 主体 */
.ce-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* 左侧章节列表 */
.ce-sidebar {
  width: 240px;
  flex-shrink: 0;
  border-right: 1px solid #e8e8e8;
  display: flex;
  flex-direction: column;
  background: #fafafa;
}

.ce-sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  font-weight: 600;
  font-size: 0.9rem;
  border-bottom: 1px solid #e8e8e8;
}

.ce-chapter-list {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
}

.ce-chapter-item {
  display: flex;
  align-items: center;
  padding: 8px 16px;
  cursor: pointer;
  font-size: 0.85rem;
  gap: 8px;
  transition: background 0.15s;
}

.ce-chapter-item:hover {
  background: #f0f0f0;
}

.ce-chapter-item.active {
  background: #e6f7ff;
  color: var(--primary-color);
}

.ce-ch-num {
  font-weight: 600;
  min-width: 24px;
  text-align: center;
  color: #999;
}

.ce-ch-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ce-ch-words {
  font-size: 0.75rem;
  color: #bbb;
  flex-shrink: 0;
}

.ce-ch-del {
  visibility: hidden;
}

.ce-chapter-item:hover .ce-ch-del {
  visibility: visible;
}

/* 主编辑区 */
.ce-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.ce-editor-header {
  padding: 8px 16px;
  border-bottom: 1px solid #e8e8e8;
  flex-shrink: 0;
}

.ce-title-input :deep(.el-input__inner) {
  border: none !important;
  font-size: 1.15rem;
  font-weight: 600;
  padding-left: 0;
}

/* Cherry 编辑器容器 */
.ce-md-wrapper {
  flex: 1;
  overflow: hidden;
  display: flex;
}

.ce-cherry-container {
  flex: 1;
  overflow: hidden;
}

/* 富文本编辑器容器 */
.ce-richtext-editor {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}
</style>

<!-- 非 scoped 样式（Cherry 主题适配） -->
<style>
/* Cherry 暗色主题适配 */
[data-theme="dark"] .chapter-editor {
  background: #1a1a2e;
}

[data-theme="dark"] .ce-toolbar {
  background: #16213e;
  border-bottom-color: rgba(255,255,255,.06);
}

[data-theme="dark"] .ce-novel-title {
  color: #e2e8f0;
}

[data-theme="dark"] .ce-sidebar {
  background: #16213e;
  border-right-color: rgba(255,255,255,.06);
}

[data-theme="dark"] .ce-sidebar-header {
  border-bottom-color: rgba(255,255,255,.06);
  color: #e2e8f0;
}

[data-theme="dark"] .ce-chapter-item:hover {
  background: rgba(255,255,255,.04);
}

[data-theme="dark"] .ce-chapter-item.active {
  background: rgba(66,133,244,.15);
}

[data-theme="dark"] .ce-editor-header {
  border-bottom-color: rgba(255,255,255,.06);
}

/* Cherry 编辑器内部暗色主题 */
[data-theme="dark"] .cherry {
  background: #1a1a2e !important;
}

[data-theme="dark"] .cherry-editor {
  background: #1a1a2e !important;
  color: #e2e8f0 !important;
}

[data-theme="dark"] .cherry-previewer {
  background: #1e293b !important;
  color: #e2e8f0 !important;
}

[data-theme="dark"] .cherry-toolbar {
  background: #16213e !important;
  border-bottom-color: rgba(255,255,255,.06) !important;
}
</style>
