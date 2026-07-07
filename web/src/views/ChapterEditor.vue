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

        <!-- Markdown 编辑器 -->
        <div v-show="editorMode === 'md'" class="ce-md-editor">
          <v-md-editor
            v-model="chapterContent"
            :height="editorHeight + 'px'"
            :toolbar="mdToolbar"
            left-toolbar="undo redo clear | h bold italic strikethrough quote | ul ol table | link image code | katex mermaid | tip emoji"
            right-toolbar="preview toc sync-scroll fullscreen"
            :include-level="[2, 3, 4]"
          />
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
import { ref, computed, onMounted, watch, nextTick } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { novelApi, type Chapter } from '@/api/novel';
import { ElMessage } from 'element-plus';
import { ArrowLeft, Plus, Delete } from '@element-plus/icons-vue';
import RichTextEditor from '@/components/RichTextEditor.vue';
import TurndownService from 'turndown';

const route = useRoute();
const router = useRouter();

const novelId = computed(() => Number(route.params.id));
const currentNum = ref(Number(route.params.num) || 1);

const novelTitle = ref('');
const chapters = ref<Chapter[]>([]);
const chapterTitle = ref('');
const chapterContent = ref('');
const richTextContent = ref('');
const editorMode = ref<'md' | 'richtext'>('md');
const saving = ref(false);
const lastSaved = ref('');

// turndown 实例（HTML → Markdown）
const turndown = new TurndownService({
  headingStyle: 'atx',
  hr: '---',
  bulletListMarker: '-',
  codeBlockStyle: 'fenced',
});

// 编辑器高度：窗口高度减去工具栏和标题栏
const editorHeight = computed(() => window.innerHeight - 140);

// MD 工具栏配置
const mdToolbar = {
  katex: true,
  mermaid: true,
};

async function loadNovel() {
  try {
    const [novelRes, chaptersRes] = await Promise.all([
      novelApi.getNovel(novelId.value),
      novelApi.getChapters(novelId.value),
    ]);
    novelTitle.value = novelRes.data.data.title;
    chapters.value = chaptersRes.data.data || [];

    // 如果没有章节，自动新建第一章
    if (chapters.value.length === 0) {
      chapters.value = [{
        id: 0, novel_id: novelId.value, chapter_number: 1,
        title: '', content: '', word_count: 0, status: 'draft',
        created_at: '',
      }];
    }

    // 加载当前章节内容
    loadCurrentChapter();
  } catch (e: any) {
    ElMessage.error('加载作品失败');
    router.push('/author');
  }
}

async function loadCurrentChapter() {
  try {
    const num = currentNum.value;
    const res = await novelApi.getChapter(novelId.value, num);
    const detail = res.data.data;
    const ch = detail.chapter;
    chapterTitle.value = ch.title || '';
    chapterContent.value = ch.content || '';
    // 富文本模式加载时转为 HTML（使用简单的 Markdown→HTML 转换）
    richTextContent.value = mdToHtml(ch.content || '');
  } catch {
    // 新章节，使用默认值
    chapterTitle.value = `第${currentNum.value}章`;
    chapterContent.value = '';
    richTextContent.value = '';
  }
}

// 模式切换处理
function onModeChange(mode: string) {
  if (mode === 'richtext') {
    // Markdown → 富文本：将 Markdown 转为 HTML
    richTextContent.value = mdToHtml(chapterContent.value);
  } else if (mode === 'md') {
    // 富文本 → Markdown：将 HTML 转为 Markdown
    if (richTextContent.value.trim()) {
      chapterContent.value = htmlToMd(richTextContent.value);
    }
  }
}

// 简单的 Markdown → HTML 转换（用于富文本编辑器初始加载）
function mdToHtml(md: string): string {
  if (!md) return '';
  // 使用简单的正则转换（不依赖完整的 markdown-it，因为我们只需要基本转换）
  let html = md;

  // 代码块
  html = html.replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>');

  // 标题
  html = html.replace(/^#### (.+)$/gm, '<h4>$1</h4>');
  html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
  html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
  html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');

  // 粗体和斜体
  html = html.replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>');
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/\*(.+?)\*/g, '<em>$1</em>');

  // 删除线
  html = html.replace(/~~(.+?)~~/g, '<del>$1</del>');

  // 高亮
  html = html.replace(/==(.+?)==/g, '<mark>$1</mark>');

  // 引用
  html = html.replace(/^> (.+)$/gm, '<blockquote><p>$1</p></blockquote>');

  // 分割线
  html = html.replace(/^---$/gm, '<hr>');

  // 链接
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>');

  // 无序列表
  html = html.replace(/^- (.+)$/gm, '<li>$1</li>');
  html = html.replace(/(<li>[\s\S]*?<\/li>)/g, '<ul>$1</ul>');

  // 段落：空行分割
  const paragraphs = html.split(/\n\n+/);
  html = paragraphs.map(p => {
    const trimmed = p.trim();
    if (!trimmed) return '';
    if (trimmed.startsWith('<')) return trimmed;
    return `<p>${trimmed.replace(/\n/g, '<br>')}</p>`;
  }).join('\n');

  return html;
}

// HTML → Markdown 转换（用于保存时）
function htmlToMd(html: string): string {
  if (!html) return '';
  return turndown.turndown(html);
}

async function switchChapter(num: number) {
  if (num === currentNum.value) return;
  // 先保存当前章节再切换
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
  currentNum.value = newNum;
  chapters.value.push({
    id: 0, novel_id: novelId.value, chapter_number: newNum,
    title: chapterTitle.value, content: '', word_count: 0, status: 'draft',
    created_at: '',
  });
  router.replace(`/author/editor/${novelId.value}/chapter/${newNum}`);
}

// 获取当前要保存的内容（富文本模式先转换）
function getContentToSave(): string {
  if (editorMode.value === 'richtext' && richTextContent.value.trim()) {
    return htmlToMd(richTextContent.value);
  }
  return chapterContent.value;
}

async function saveChapter() {
  if (!chapterTitle.value.trim()) {
    ElMessage.warning('请输入章节标题');
    return;
  }
  // 富文本模式先转换
  const contentToSave = getContentToSave();
  saving.value = true;
  try {
    const existing = chapters.value.find(c => c.chapter_number === currentNum.value && c.id > 0);
    if (existing) {
      await novelApi.updateChapter(novelId.value, currentNum.value, {
        title: chapterTitle.value,
        content: contentToSave,
      });
    } else {
      await novelApi.createChapter(novelId.value, {
        title: chapterTitle.value,
        content: contentToSave,
      });
    }
    // 更新本地列表中的标题和字数
    const idx = chapters.value.findIndex(c => c.chapter_number === currentNum.value);
    if (idx >= 0) {
      chapters.value[idx].title = chapterTitle.value;
      chapters.value[idx].word_count = contentToSave.length;
      if (chapters.value[idx].id === 0) chapters.value[idx].id = -1; // mark as saved
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
      await novelApi.updateChapter(novelId.value, currentNum.value, {
        title: chapterTitle.value,
        content: contentToSave,
      });
    } else if (chapterTitle.value.trim()) {
      await novelApi.createChapter(novelId.value, {
        title: chapterTitle.value,
        content: contentToSave,
      });
    }
  } catch { /* 静默保存失败不提示 */ }
}

async function deleteChapterItem(num: number) {
  try {
    await novelApi.deleteChapter(novelId.value, num);
    chapters.value = chapters.value.filter(c => c.chapter_number !== num);
    if (currentNum.value === num && chapters.value.length > 0) {
      switchChapter(chapters.value[0].chapter_number);
    }
    ElMessage.success('已删除');
  } catch (e: any) {
    ElMessage.error('删除失败');
  }
}

function goBack() {
  saveChapterSilent();
  router.push(`/author/editor/${novelId.value}`);
}

onMounted(() => {
  loadNovel();
});

// 键盘快捷键 Ctrl+S
function onKeyDown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault();
    saveChapter();
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeyDown);
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
}

.ce-chapter-item {
  display: flex;
  align-items: center;
  padding: 10px 12px;
  cursor: pointer;
  border-bottom: 1px solid #f0f0f0;
  transition: background 0.15s;
  gap: 8px;
}

.ce-chapter-item:hover {
  background: #f0f4ff;
}

.ce-chapter-item.active {
  background: #e8f0ff;
  border-left: 3px solid var(--primary-color);
}

.ce-ch-num {
  font-weight: 600;
  min-width: 24px;
  color: var(--primary-color);
}

.ce-ch-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.85rem;
}

.ce-ch-words {
  font-size: 0.75rem;
  color: #999;
}

.ce-ch-del {
  opacity: 0;
  transition: opacity 0.15s;
}

.ce-chapter-item:hover .ce-ch-del {
  opacity: 1;
}

/* 编辑区 */
.ce-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.ce-editor-header {
  padding: 12px 16px 8px;
  flex-shrink: 0;
}

.ce-title-input {
  font-size: 1.1rem;
}

.ce-md-editor {
  flex: 1;
  overflow: hidden;
}

.ce-richtext-editor {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
</style>
