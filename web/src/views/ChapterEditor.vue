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
        <PublishQuotaHint />
        <el-button :icon="MagicStick" @click="showAiPanel = true">AI 助手</el-button>
        <el-button :icon="Link" @click="showRefPicker = true">插入站内链接</el-button>
        <el-button type="primary" :loading="saving" @click="saveChapter">保存章节</el-button>
      </div>
    </div>

    <!-- 站内内容引用选择器（16.2：章节正文引用其他小说/博客/论坛帖，相对路径可跳转） -->
    <ContentRefPicker v-model:visible="showRefPicker" @confirm="insertRefLink" />

    <!-- AI 助手（§13.3：续写/润色/扩写/摘要，代币按量计费；§13.5 长篇上下文） -->
    <AiAssistPanel
      v-model:visible="showAiPanel"
      :context-text="aiContextText"
      :novel-id="novelId"
      :chapter-id="currentChapterId"
      @insert="aiInsert"
      @replace="aiReplace"
    />

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
            <el-tag
              v-if="ch.status && ch.status !== 'published'"
              size="small"
              :type="chapterStatusType(ch.status)"
              class="ce-ch-status"
            >{{ chapterStatusLabel(ch.status) }}</el-tag>
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
          <!-- 多人协作：章节状态机（提审 / 审核 / 发布 / 撤回） -->
          <ChapterWorkflowBar
            :novel-id="novelId"
            :chapter-num="currentNum"
            :status="chapterStatus"
            :me="collabMe"
            :can-edit="canEditThis"
            :review-note="reviewNote"
            @changed="onWorkflowChanged"
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
            @save="saveChapter"
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
import { ArrowLeft, Plus, Delete, Link, MagicStick } from '@element-plus/icons-vue';
import RichTextEditor from '@/components/RichTextEditor.vue';
import ContentRefPicker from '@/components/frame/ContentRefPicker.vue';
import AiAssistPanel from '@/components/editor/AiAssistPanel.vue';
import ChapterWorkflowBar from '@/components/collab/ChapterWorkflowBar.vue';
import { collabApi, chapterStatusLabel, chapterStatusType, type CollabMe } from '@/api/collab';
import { useAuthStore } from '@/stores/auth';
import { guardLeak } from '@/composables/useLeakGuard';
// engineConfig 已内置 physicsKatex（含 mhchem + physics 物理宏包）
import { renderMarkdown, editorEngineConfig } from '@/markdown/renderer';
import { htmlToMarkdown } from '@/utils/turndown';

const route = useRoute();
const router = useRouter();

const novelId = computed(() => Number(route.params.id));
const currentNum = ref(Number(route.params.num) || 1);
// 当前章节 ID（§13.5 长篇上下文关联；未保存的新章节为 0）
const currentChapterId = computed(() => {
  const c = chapters.value.find((ch) => ch.chapter_number === currentNum.value);
  return c && c.id > 0 ? c.id : 0;
});

const authStore = useAuthStore();

const novelTitle = ref('');
const chapters = ref<Chapter[]>([]);
// 多人协作：我的协作身份与本章状态
const collabMe = ref<CollabMe | null>(null);
const chapterStatus = ref('draft');
const reviewNote = ref('');
const canEditThis = ref(true);
const chapterTitle = ref('');
const chapterContent = ref('');       // Markdown 源文本
const richTextContent = ref('');      // 富文本 HTML 内容
const editorMode = ref<'md' | 'richtext'>('md');
const saving = ref(false);
const lastSaved = ref('');

// Cherry 编辑器实例
const cherryContainer = ref<HTMLDivElement | null>(null);
let cherryInstance: any = null;

// ===== 站内内容引用（16.2）：正文引用其他小说/博客/论坛帖，相对路径可跳转 =====
const showRefPicker = ref(false);

// ===== AI 助手（§13.3）：续写/润色/扩写/摘要，代币按量计费 =====
const showAiPanel = ref(false);

// AI 面板上下文文本：MD 模式直接取源文；富文本模式去标签取纯文本
const aiContextText = computed(() => {
  if (editorMode.value === 'richtext') {
    return richTextContent.value.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
  }
  return chapterContent.value;
});

// AI 生成标识：编辑过程中用过 AI 就置位，保存时随正文提交，供发布页/详情页打标
const aiUsed = ref(false);
const aiModelName = ref('');

// AI 结果插入编辑器：MD 模式优先插到光标处，失败回退追加；富文本模式追加
function aiInsert(text: string, meta?: { model?: string }) {
  aiUsed.value = true;
  if (meta?.model) aiModelName.value = meta.model;
  if (editorMode.value === 'md' && cherryInstance) {
    const cm = cherryInstance.editor?.codemirror || cherryInstance.codemirror;
    if (cm && cm.replaceSelection) {
      cm.replaceSelection((cm.getValue() ? '\n\n' : '') + text);
      ElMessage.success('已插入 AI 结果');
      return;
    }
    if (cherryInstance.insert) {
      cherryInstance.insert('\n\n' + text);
      ElMessage.success('已插入 AI 结果');
      return;
    }
    chapterContent.value = (chapterContent.value ? chapterContent.value + '\n\n' : '') + text;
  } else {
    richTextContent.value = (richTextContent.value ? richTextContent.value + '\n\n' : '') + text;
  }
  ElMessage.success('已插入 AI 结果');
}

// AI 结果替换整个正文
function aiReplace(text: string, meta?: { model?: string }) {
  aiUsed.value = true;
  if (meta?.model) aiModelName.value = meta.model;
  if (editorMode.value === 'md') {
    chapterContent.value = text;
    if (cherryInstance) cherryInstance.setValue(text);
  } else {
    richTextContent.value = renderMarkdown(text);
  }
  ElMessage.success('已替换正文');
}

// 在编辑器光标处插入 [标题](相对链接)；Markdown 模式走 Cherry 内 CodeMirror 光标处，
// 富文本模式追加到 v-model 内容（RichTextEditor 内部按 md 渲染链接）
function insertRefLink(link: string, ref: { type: string; id: number; title: string }) {
  const md = `[${ref.title || link}](${link})`;
  if (editorMode.value === 'md' && cherryInstance) {
    const cm = cherryInstance.editor?.codemirror || cherryInstance.codemirror;
    if (cm && cm.replaceSelection) { cm.replaceSelection(md); ElMessage.success('已插入引用'); return; }
    if (cherryInstance.insert) { cherryInstance.insert(md); ElMessage.success('已插入引用'); return; }
    chapterContent.value += '\n\n' + md;
  } else {
    richTextContent.value += (richTextContent.value ? '\n\n' : '') + md;
  }
  ElMessage.success('已插入引用');
}
let cherryReady = false;

// 获取当前要保存的内容（富文本模式先转换）
async function getContentToSave(): Promise<string> {
  if (editorMode.value === 'richtext' && richTextContent.value.trim()) {
    return await htmlToMarkdown(richTextContent.value);
  }
  // MD 模式：如果 Cherry 已初始化，从 Cherry 取值；否则用 chapterContent
  if (editorMode.value === 'md' && cherryInstance) {
    return cherryInstance.getValue() || chapterContent.value;
  }
  return chapterContent.value;
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
    ...editorEngineConfig,
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

async function onModeChange(mode: string) {
  if (mode === 'richtext') {
    // Markdown → 富文本：从 Cherry 或 chapterContent 取 Markdown，转为 HTML
    const mdContent = cherryInstance ? (cherryInstance.getValue() || '') : chapterContent.value;
    richTextContent.value = renderMarkdown(mdContent);
  } else if (mode === 'md') {
    // 富文本 → Markdown
    if (richTextContent.value.trim()) {
      const md = await htmlToMarkdown(richTextContent.value);
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

// ─── 多人协作 ───

// 我在该作品上的协作身份（主笔/协作作者/编辑/审稿）；非协作作品返回 null，保持原有直发行为
async function loadCollab() {
  try {
    const res = await collabApi.list(novelId.value);
    collabMe.value = res.data.data?.me || null;
  } catch {
    collabMe.value = null;
  }
}

// 我能否编辑本章：主笔 / 有 edit_others / 本章是我写的
function computeCanEdit(ch: { author_id?: number }): boolean {
  const me = collabMe.value;
  if (!me) return true;
  if (me.is_owner || me.can_edit_others) return true;
  const authorId = Number(ch?.author_id || 0);
  return authorId > 0 && authorId === Number(authStore.user?.id || 0);
}

// 工作流回调：本地同步状态（后端已落库）
function onWorkflowChanged(next: string) {
  chapterStatus.value = next;
  if (next !== 'rejected') reviewNote.value = '';
  const idx = chapters.value.findIndex((c) => c.chapter_number === currentNum.value);
  if (idx >= 0) (chapters.value[idx] as Chapter).status = next;
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
    const ch = detail.chapter as Chapter & { review_note?: string; author_id?: number };
    chapterTitle.value = ch.title || '';
    const content = ch.content || '';
    chapterContent.value = content;
    // 协作状态机：草稿/待审/已发布/已驳回 + 驳回意见 + 我能否改本章
    chapterStatus.value = ch.status || 'draft';
    reviewNote.value = ch.review_note || '';
    canEditThis.value = computeCanEdit(ch);

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
  chapterStatus.value = 'draft';
  reviewNote.value = '';
  canEditThis.value = true;
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
  const contentToSave = await getContentToSave();
  // 个人信息泄漏提示（本地检测，高危需确认；后端只收命中类型计数）
  if (!(await guardLeak(contentToSave, '章节正文'))) return;
  saving.value = true;
  try {
    let chapterId = 0;
    const existing = chapters.value.find(c => c.chapter_number === currentNum.value && c.id > 0);
    if (existing) {
      await chapterApi.updateChapter(novelId.value, currentNum.value, {
        title: chapterTitle.value,
        content: contentToSave,
        ai_assisted: aiUsed.value || undefined,
        ai_model: aiUsed.value ? aiModelName.value : undefined,
      } as any);
      chapterId = existing.id;
    } else {
      const res = await chapterApi.createChapter(novelId.value, {
        title: chapterTitle.value,
        content: contentToSave,
        ai_assisted: aiUsed.value || undefined,
        ai_model: aiUsed.value ? aiModelName.value : undefined,
      } as any);
      chapterId = Number(res.data?.data?.id) || 0;
    }
    const idx = chapters.value.findIndex(c => c.chapter_number === currentNum.value);
    if (idx >= 0) {
      chapters.value[idx].title = chapterTitle.value;
      chapters.value[idx].word_count = contentToSave.length;
      // 用真实章节 ID 标记（>0），后续保存走更新而非重复创建
      if (chapters.value[idx].id <= 0 && chapterId > 0) chapters.value[idx].id = chapterId;
    }
    // 新建章节的初始状态：主笔/有发布权者直接发布，协作者落到草稿待提审
    if (!existing) {
      const me = collabMe.value;
      chapterStatus.value = me && !me.is_owner && !me.can_publish ? 'draft' : 'published';
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
  const contentToSave = await getContentToSave();
  if (!chapterTitle.value.trim() && !contentToSave.trim()) return;
  try {
    const existing = chapters.value.find(c => c.chapter_number === currentNum.value && c.id > 0);
    if (existing) {
      await chapterApi.updateChapter(novelId.value, currentNum.value, {
        title: chapterTitle.value,
        content: contentToSave,
        ai_assisted: aiUsed.value || undefined,
        ai_model: aiUsed.value ? aiModelName.value : undefined,
      } as any);
    } else if (chapterTitle.value.trim()) {
      await chapterApi.createChapter(novelId.value, {
        title: chapterTitle.value,
        content: contentToSave,
        ai_assisted: aiUsed.value || undefined,
        ai_model: aiUsed.value ? aiModelName.value : undefined,
      } as any);
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

onMounted(async () => {
  window.addEventListener('keydown', onKeyDown);
  // 先解析协作身份，再加载章节（章节可见性与编辑权依赖身份）
  await loadCollab();
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
  background: var(--bg-card);
  z-index: 50;
}

/* 顶部工具栏 */
.ce-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-soft);
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
  color: var(--text-muted);
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
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  background: var(--bg-soft);
}

.ce-sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  font-weight: 600;
  font-size: 0.9rem;
  border-bottom: 1px solid var(--border-color);
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
  background: var(--bg-hover);
}

.ce-chapter-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--primary-color);
}

.ce-ch-num {
  font-weight: 600;
  min-width: 24px;
  text-align: center;
  color: var(--text-muted);
}

.ce-ch-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ce-ch-words {
  font-size: 0.75rem;
  color: var(--text-muted);
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
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  flex-wrap: wrap;
}

.ce-ch-status {
  flex-shrink: 0;
  transform: scale(0.85);
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

/* ====== 移动端章节编辑器适配 ====== */
@media (max-width: 768px) {
  .chapter-editor {
    top: 62px;
  }
  .ce-toolbar {
    height: auto;
    min-height: 48px;
    padding: 6px 8px;
    flex-wrap: wrap;
    gap: 6px;
  }
  .ce-novel-title {
    display: none;
  }
  .ce-saved-time {
    display: none;
  }
  .ce-toolbar-center {
    order: 3;
    width: 100%;
    justify-content: center;
  }
  /* 侧栏收窄，只保留编号 */
  .ce-sidebar {
    width: 140px;
  }
  .ce-ch-words,
  .ce-ch-del {
    display: none;
  }
  .ce-chapter-item {
    padding: 10px 10px;
  }
  .ce-editor-header {
    padding: 8px;
  }
}
</style>
