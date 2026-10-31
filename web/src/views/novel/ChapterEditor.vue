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
            <el-radio-button v-if="isBubbleMode(effectiveMode)" value="bubble">气泡</el-radio-button>
          </el-radio-group>
        </div>
      <div class="ce-toolbar-right">
        <span v-if="lastSaved" class="ce-saved-time">已保存 {{ lastSaved }}</span>
        <PublishQuotaHint />
        <el-button v-if="isBubbleMode(effectiveMode)" :icon="ChatDotRound" @click="openScriptPreview">气泡预览</el-button>
        <el-button :icon="Clock" @click="versionPanelVisible = true">版本</el-button>
        <el-button :icon="MagicStick" @click="openAiWrite">AI 写作</el-button>
        <el-button :icon="Link" @click="showRefPicker = true">插入站内链接</el-button>
        <el-button type="primary" :loading="saving" @click="saveChapter()">保存章节</el-button>
      </div>
    </div>

    <!-- 站内内容引用选择器（16.2：章节正文引用其他小说/博客/论坛帖，相对路径可跳转） -->
    <ContentRefPicker v-model:visible="showRefPicker" @confirm="insertRefLink" />

    <!-- AI 写作：浮动可拖拽层，流式直接写入编辑器（ESC 不关闭，仅 X/取消可关） -->
    <AiWriteLayer
      v-model:visible="showAiWrite"
      :context-text="aiContextText"
      :selection-text="aiSelectionText"
      :scene="isBubbleMode(effectiveMode) ? 'novel_script' : 'novel_chapter'"
      @stream-begin="onAiWriteBegin"
      @stream-chunk="onAiWriteChunk"
      @stream-end="onAiWriteEnd"
    />

    <!-- 章节版本：分支 / 历史 / diff / 回滚 / 删除 / 合并 -->
    <ChapterVersionPanel
      :visible="versionPanelVisible"
      :novel-id="novelId"
      :chapter-num="currentNum"
      @update:visible="onVersionPanelToggle"
    />

    <!-- 保存冲突：并排对比后由作者决定「以我的为准」或「放弃修改」 -->
    <ConflictResolveDialog
      v-model:visible="conflictVisible"
      :conflict="conflictData"
      @resolve="onConflictResolve"
    />

    <!-- 话本气泡预览（script mode）：服务端解析，转义后渲染 -->
    <el-drawer v-model="scriptPreviewVisible" title="气泡预览" size="60%">
      <div class="sp-wrap" v-loading="scriptPreviewing">
        <div class="sp-hint">
          写法：每行一条 <code>角色：台词</code>；整行 <code>【场景】</code> 为场景提示；
          整行 <code>![说明](图片地址)</code> 插入图片；行内 <code>[笑]</code> 插表情。
          <template v-if="effectiveMode === 'mixed'">
            混排模式下，不含对话的连续段落按<strong>散文（Markdown）</strong>渲染。
          </template>
        </div>
        <div v-if="scriptSpeakers.length" class="sp-speakers">
          <el-tag v-for="s in scriptSpeakers" :key="s" size="small">{{ s }}</el-tag>
        </div>
        <div v-if="scriptEmoteKeys.length" class="sp-speakers">
          <el-tag
            v-for="k in scriptEmoteKeys"
            :key="k"
            size="small"
            type="info"
            class="sp-emote"
            @click="insertEmote(k)"
          >[{{ k }}]</el-tag>
        </div>
        <div class="sp-body" v-html="scriptPreviewHtml"></div>
      </div>
    </el-drawer>

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
          <el-tag v-if="isBubbleMode(effectiveMode)" size="small" type="warning" class="ce-mode-tag">
            {{ effectiveMode === 'mixed' ? '混排' : '话本气泡体' }}
          </el-tag>
          <el-select v-model="chapterMode" size="small" style="width: 150px" @change="onChapterModeChange">
            <el-option v-for="m in CHAPTER_MODE_OPTIONS" :key="m.value" :label="m.label" :value="m.value" />
          </el-select>
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

        <!-- 话本气泡体专用编辑器：角色气泡 + 头像 + 左右分侧 + 角色管理 -->
        <div v-show="editorMode === 'bubble'" class="ce-bubble-wrapper">
          <ScriptBubbleEditor
            v-model="bubbleContent"
            :mode="effectiveMode === 'mixed' ? 'mixed' : 'script'"
            :novel-id="novelId"
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
import { ArrowLeft, Plus, Delete, Link, MagicStick, ChatDotRound, Clock } from '@element-plus/icons-vue';
import RichTextEditor from '@/components/RichTextEditor.vue';
import ScriptBubbleEditor from '@/components/editor/script/ScriptBubbleEditor.vue';
import ContentRefPicker from '@/components/frame/ContentRefPicker.vue';
import AiWriteLayer from '@/components/ai/floating/AiWriteLayer.vue';
import { beginStream, writeStreamChunk, endStream, insertAtCursor, readEditorSelection, type StreamSession, type StreamTarget } from '@/utils/editorStream';
import ChapterWorkflowBar from '@/components/collab/ChapterWorkflowBar.vue';
import ChapterVersionPanel from '@/components/collab/ChapterVersionPanel.vue';
import ConflictResolveDialog from '@/components/collab/ConflictResolveDialog.vue';
import { vcsApi, isConflictResponse, type ConflictPayload } from '@/api/novel/vcs';
import { collabApi, chapterStatusLabel, chapterStatusType, type CollabMe } from '@/api/collab';
import { useAuthStore } from '@/stores/auth';
import {
  scriptApi,
  CHAPTER_MODE_OPTIONS,
  isBubbleMode,
  type ChapterMode,
} from '@/api/novel/script';
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
const bubbleContent = ref('');        // 话本气泡编辑器内容（源文本形态）
const editorMode = ref<'md' | 'richtext' | 'bubble'>('md');
const saving = ref(false);
const lastSaved = ref('');

// Cherry 编辑器实例
const cherryContainer = ref<HTMLDivElement | null>(null);
let cherryInstance: any = null;

// ===== 站内内容引用（16.2）：正文引用其他小说/博客/论坛帖，相对路径可跳转 =====
const showRefPicker = ref(false);

// ===== AI 助手（§13.3）：续写/润色/扩写/摘要，代币按量计费 =====
const showAiWrite = ref(false);
const aiSelectionText = ref('');
const aiStreamSess = ref<StreamSession | null>(null);

// ===== 叙事形态（作品级 + 章节级）与话本预览 =====
const novelMode = ref('prose');
// 章节形态：'' = 继承作品（后端存空值），显式设置时为 prose/script/mixed
const chapterMode = ref<ChapterMode>('');
const effectiveMode = computed(() => chapterMode.value || novelMode.value || 'prose');
const scriptPreviewVisible = ref(false);
const scriptPreviewHtml = ref('');
const scriptSpeakers = ref<string[]>([]);
const scriptEmoteKeys = ref<string[]>([]);
const scriptPreviewing = ref(false);

async function openScriptPreview() {
  const content = await getContentToSave();
  scriptPreviewVisible.value = true;
  scriptPreviewing.value = true;
  try {
    const res = await scriptApi.preview(content, novelId.value, effectiveMode.value);
    scriptPreviewHtml.value = res.data.data?.html || '';
    scriptSpeakers.value = res.data.data?.speakers || [];
    scriptEmoteKeys.value = res.data.data?.emote_keys || [];
  } catch (e: any) {
    scriptPreviewHtml.value = '';
    ElMessage.error(e?.response?.data?.message || '预览失败');
  } finally {
    scriptPreviewing.value = false;
  }
}

// 切形态只改标记，保存时随正文一起提交（后端按章存储）
function onChapterModeChange() {
  // 形态切走（不再是话本/混排）→ 气泡编辑器不可用，退回 Markdown
  if (!isBubbleMode(effectiveMode.value) && editorMode.value === 'bubble') {
    editorMode.value = 'md';
    prevEditorMode.value = 'md';
  }
  ElMessage.info(
    chapterMode.value === ''
      ? '本章将继承作品形态，保存后生效'
      : `本章形态已设为「${CHAPTER_MODE_OPTIONS.find((m) => m.value === chapterMode.value)?.label}」，保存后生效`,
  );
}

// 预览抽屉里点表情 → 插入编辑器
function insertEmote(key: string) {
  const mark = `[${key}]`;
  if (editorMode.value === 'md' && cherryInstance) {
    if (insertAtCursor(cherryInstance, mark)) {
      ElMessage.success(`已插入 ${mark}`);
      return;
    }
  }
  chapterContent.value += mark;
  richTextContent.value += mark;
  ElMessage.success(`已插入 ${mark}`);
}

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

// ── AI 写作（浮动层，流式直接写入编辑器） ──
function openAiWrite() {
  aiSelectionText.value = currentSelectionText();
  showAiWrite.value = true;
}

// 取当前编辑器选区（无选区返回空串）
function currentSelectionText(): string {
  return readEditorSelection(cherryInstance);
}

// 流式开始：在编辑器里按目标点（cursor/selection/all）腾好锚点并标记「已用 AI」
function onAiWriteBegin(target: StreamTarget) {
  aiStreamSess.value = beginStream(cherryInstance, editorMode.value, target);
  aiUsed.value = true;
}

// 流式增量：实时写入编辑器（MD 走 CM bookmark；富文本/回退追加到变量）
function onAiWriteChunk(chunk: string) {
  const sess = aiStreamSess.value;
  if (!sess || sess.rich) {
    if (editorMode.value === 'richtext') richTextContent.value += chunk;
    else chapterContent.value += chunk;
    return;
  }
  writeStreamChunk(sess, chunk);
}

// 流式结束：记录模型、清理锚点
function onAiWriteEnd(payload: { content: string; model: string }) {
  aiModelName.value = payload.model;
  if (aiStreamSess.value) endStream(aiStreamSess.value);
  aiStreamSess.value = null;
}

// 在编辑器光标处插入 [标题](相对链接)；Markdown 模式走 Cherry 编辑器光标处，
// 富文本模式追加到 v-model 内容（RichTextEditor 内部按 md 渲染链接）
function insertRefLink(link: string, ref: { type: string; id: number; title: string }) {
  const md = `[${ref.title || link}](${link})`;
  if (editorMode.value === 'md' && cherryInstance) {
    if (insertAtCursor(cherryInstance, md)) { ElMessage.success('已插入引用'); return; }
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
  // 气泡编辑模式：内容由气泡编辑器直接维护
  if (editorMode.value === 'bubble') {
    return bubbleContent.value;
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

// 编辑模式切换：md / richtext / bubble
// 注意：v-model 会在 @change 之前改写 editorMode，所以这里用 prevEditorMode 记录「切换前」的模式。
const prevEditorMode = ref<'md' | 'richtext' | 'bubble'>('md');

/** 取某个编辑模式下的源文本 */
async function contentOf(mode: 'md' | 'richtext' | 'bubble'): Promise<string> {
  if (mode === 'bubble') return bubbleContent.value;
  if (mode === 'richtext' && richTextContent.value.trim()) {
    return await htmlToMarkdown(richTextContent.value);
  }
  if (cherryInstance) return cherryInstance.getValue() || chapterContent.value;
  return chapterContent.value;
}

async function onModeChange(mode: string) {
  const from = prevEditorMode.value;
  const to = mode as 'md' | 'richtext' | 'bubble';
  prevEditorMode.value = to;

  if (to === 'bubble') {
    // 其他模式 → 气泡：取当前源文本作为气泡编辑器的初始内容
    bubbleContent.value = await contentOf(from);
    return;
  }
  if (from === 'bubble') {
    // 气泡 → 其他模式：气泡内容是准的，先落到 Cherry / 富文本，避免丢内容
    chapterContent.value = bubbleContent.value;
    if (cherryInstance) cherryInstance.setValue(bubbleContent.value);
    if (to === 'richtext') {
      richTextContent.value = renderMarkdown(bubbleContent.value);
    }
    if (to === 'md' && !cherryReady) {
      nextTick(() => initCherryEditor());
    }
    return;
  }
  if (to === 'richtext') {
    // Markdown → 富文本：从 Cherry 或 chapterContent 取 Markdown，转为 HTML
    const mdContent = cherryInstance ? (cherryInstance.getValue() || '') : chapterContent.value;
    richTextContent.value = renderMarkdown(mdContent);
  } else if (to === 'md') {
    // 富文本 → Markdown
    if (from === 'richtext' && richTextContent.value.trim()) {
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

// ===== 版本控制（docs/novel-version-control.md） =====
// baseVersion：打开/切换章节时记下的服务端版本号，保存时带上做冲突检测
const baseVersion = ref(0);
const currentBranch = ref('main');
const versionPanelVisible = ref(false);
const conflictVisible = ref(false);
const conflictData = ref<ConflictPayload | null>(null);

async function loadVersionHead() {
  try {
    const res = await vcsApi.listVersions(novelId.value, currentNum.value, currentBranch.value);
    const d = res.data.data || ({} as any);
    baseVersion.value = d.head || 0;
  } catch {
    baseVersion.value = 0; // 取不到就不做冲突检测（不阻断写作）
  }
}

// 版本面板关闭（回滚/合并后）→ 重新载入章节正文与版本号
function onVersionPanelToggle(v: boolean) {
  versionPanelVisible.value = v;
  if (!v) {
    loadCurrentChapter();
    loadVersionHead();
  }
}

// 冲突处理：以我的为准（force 覆盖）/ 放弃修改（重载服务端）
async function onConflictResolve(action: 'mine' | 'reload') {
  conflictVisible.value = false;
  if (action === 'mine') {
    await saveChapter(true);
    return;
  }
  await loadCurrentChapter();
  await loadVersionHead();
  ElMessage.info('已放弃本次修改，重载服务端最新版本');
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
    novelMode.value = (novelRes.data.data as any).mode || 'prose';
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
    // 章节叙事形态（空=继承作品；这里取 DB 原值，未被列表回填的生效值污染）
    chapterMode.value = ((ch as any).mode || '') as ChapterMode;

    if (editorMode.value === 'md' && cherryInstance) {
      cherryInstance.setValue(content);
    } else if (editorMode.value === 'md' && !cherryReady) {
      // 等待 Cherry 初始化后再设置值——通过 chapterContent 同步
    }
    richTextContent.value = renderMarkdown(content);
    bubbleContent.value = content; // 气泡编辑器的初始内容（切到气泡模式时直接用）
    // 记下服务端版本号，供本次编辑保存时做冲突检测
    await loadVersionHead();
  } catch {
    chapterTitle.value = `第${currentNum.value}章`;
    chapterContent.value = '';
    richTextContent.value = '';
    baseVersion.value = 0;
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
  chapterMode.value = ''; // 新章节默认继承作品形态
  baseVersion.value = 0; // 新章节还没有历史版本
  currentNum.value = newNum;
  if (cherryInstance) cherryInstance.setValue('');
  chapters.value.push({
    id: 0, novel_id: novelId.value, chapter_number: newNum,
    title: chapterTitle.value, content: '', word_count: 0, status: 'draft',
    created_at: '',
  });
  router.replace(`/author/editor/${novelId.value}/chapter/${newNum}`);
}

async function saveChapter(force = false) {
  if (!chapterTitle.value.trim()) {
    ElMessage.warning('请输入章节标题');
    return;
  }
  // ⚠️ 必须严格判 true：模板里 @click="saveChapter" 会把 MouseEvent 当第一个参数传进来，
  // 直接把事件对象塞进请求体的 force 字段 → 后端 bool 绑定失败 → 400「请求参数错误」，
  // 表现为「点保存没反应」。这里统一收口成真正的布尔值。
  const forceSave = force === true;
  const contentToSave = await getContentToSave();
  // 个人信息泄漏提示（本地检测，高危需确认；后端只收命中类型计数）
  if (!(await guardLeak(contentToSave, '章节正文'))) return;
  saving.value = true;
  try {
    let chapterId = 0;
    const existing = chapters.value.find(c => c.chapter_number === currentNum.value && c.id > 0);
    // 版本控制：base_version 落后即被服务端拒绝（409），前端弹并排对比
    const vcsFields = {
      base_version: baseVersion.value || 0,
      branch: currentBranch.value,
      force: forceSave,
    } as any;
    if (existing) {
      const res = await chapterApi.updateChapter(novelId.value, currentNum.value, {
        title: chapterTitle.value,
        content: contentToSave,
        ai_assisted: aiUsed.value || undefined,
        ai_model: aiUsed.value ? aiModelName.value : undefined,
        mode: chapterMode.value, // '' = 继承作品形态
        ...vcsFields,
      } as any);
      chapterId = existing.id;
      const ver = Number(res?.data?.data?.version || 0);
      if (ver > 0) baseVersion.value = ver;
    } else {
      const res = await chapterApi.createChapter(novelId.value, {
        title: chapterTitle.value,
        content: contentToSave,
        ai_assisted: aiUsed.value || undefined,
        ai_model: aiUsed.value ? aiModelName.value : undefined,
        mode: chapterMode.value,
      } as any);
      chapterId = Number(res.data?.data?.id) || 0;
      const ver = Number(res?.data?.data?.version || 0);
      if (ver > 0) baseVersion.value = ver;
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
    ElMessage.success(force ? '已保存（强制覆盖，冲突现场已留版本）' : '已保存');
  } catch (e: any) {
    const cf = isConflictResponse(e);
    if (cf && !force) {
      conflictData.value = cf;
      conflictVisible.value = true;
      return;
    }
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
    const vcsFields = { base_version: baseVersion.value || 0, branch: currentBranch.value } as any;
    if (existing) {
      const res = await chapterApi.updateChapter(novelId.value, currentNum.value, {
        title: chapterTitle.value,
        content: contentToSave,
        ai_assisted: aiUsed.value || undefined,
        ai_model: aiUsed.value ? aiModelName.value : undefined,
        ...vcsFields,
      } as any);
      const ver = Number(res?.data?.data?.version || 0);
      if (ver > 0) baseVersion.value = ver;
    } else if (chapterTitle.value.trim()) {
      const res = await chapterApi.createChapter(novelId.value, {
        title: chapterTitle.value,
        content: contentToSave,
        ai_assisted: aiUsed.value || undefined,
        ai_model: aiUsed.value ? aiModelName.value : undefined,
      } as any);
      const ver = Number(res?.data?.data?.version || 0);
      if (ver > 0) baseVersion.value = ver;
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

.ce-mode-tag {
  flex-shrink: 0;
}

/* 话本气泡预览（结构与阅读器一致，样式独立一份避免跨组件耦合） */
.sp-wrap {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.sp-hint {
  font-size: 0.8rem;
  color: var(--text-muted);
  line-height: 1.7;
}
.sp-hint code {
  background: var(--bg-soft);
  padding: 1px 4px;
  border-radius: 3px;
}
.sp-speakers {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.sp-body :deep(.script-chapter) {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.sp-body :deep(.script-stage) {
  text-align: center;
  font-size: 0.85rem;
  color: var(--text-muted);
  font-style: italic;
  background: var(--bg-soft);
  border-radius: 6px;
  padding: 6px 10px;
}
.sp-body :deep(.script-narration) {
  text-align: center;
  font-size: 0.95rem;
  line-height: 1.9;
  padding: 4px 12px;
}
.sp-body :deep(.script-bubble) {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  max-width: 82%;
}
.sp-body :deep(.sb-right) {
  flex-direction: row-reverse;
  margin-left: auto;
}
.sp-body :deep(.sb-avatar) {
  flex-shrink: 0;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 600;
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
}
.sp-body :deep(.sb-right .sb-avatar) {
  background: var(--gradient-accent, linear-gradient(135deg, #f59e0b, #ef4444));
}
.sp-body :deep(.sb-body) {
  padding: 8px 12px;
  border-radius: 12px;
  background: var(--bg-soft);
  border: 1px solid var(--border-color);
}
.sp-body :deep(.sb-name) {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin-bottom: 2px;
}
.sp-body :deep(.sb-text) {
  font-size: 1rem;
  line-height: 1.8;
  word-break: break-word;
}
.sp-body :deep(.sb-emote) {
  font-size: 1.25em;
  margin: 0 2px;
}
.sp-body :deep(.script-image-block) {
  text-align: center;
  margin: 4px auto;
}
.sp-body :deep(.script-image-block.sb-invalid) {
  font-size: 0.8rem;
  color: var(--text-muted);
  border: 1px dashed var(--border-color);
  border-radius: 8px;
  padding: 8px 12px;
}
.sp-body :deep(.sb-image) {
  max-width: 100%;
  border-radius: 10px;
}
.sp-body :deep(.script-prose) {
  padding: 0 4px;
}
.sp-emote {
  cursor: pointer;
}
.sp-emote:hover {
  opacity: 0.75;
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

/* 话本气泡体专用编辑器 */
.ce-bubble-wrapper {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  padding: 12px 16px;
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
