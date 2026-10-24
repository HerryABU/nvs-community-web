<template>
  <div class="blog-editor">
    <!-- 顶部工具栏：三列布局，全部垂直居中 -->
    <div class="be-toolbar">
      <div class="be-toolbar-left">
        <el-button text @click="$router.push('/author')">
          <el-icon><ArrowLeft /></el-icon>返回工作台
        </el-button>
        <el-button text :icon="Link" @click="showRefPicker = true">插入站内链接</el-button>
        <el-button text @click="openAiPanel">
          <el-icon><MagicStick /></el-icon>AI 助手
        </el-button>
      </div>
      <div class="be-toolbar-center">
        <span class="be-page-title">{{ editingId ? '编辑博客' : '写博客' }}</span>
        <el-radio-group v-model="editorMode" size="small" @change="onModeChange">
          <el-radio-button value="md">Markdown</el-radio-button>
          <el-radio-button value="richtext">富文本</el-radio-button>
        </el-radio-group>
      </div>
      <div class="be-toolbar-right">
        <span v-if="lastSaved" class="be-saved-time">已保存 {{ lastSaved }}</span>
        <el-button :loading="batchImporting" @click="showBatchDialog = true" title="批量导入多个文件为多篇博客">
          <el-icon><UploadFilled /></el-icon><span class="desktop-only"> 批量导入</span>
        </el-button>
        <el-button type="primary" :loading="saving" @click="saveBlog">保存</el-button>
      </div>
    </div>

    <!-- 站内内容引用选择器（16.2：博客正文引用其他小说/博客/论坛帖，相对路径可跳转） -->
    <ContentRefPicker v-model:visible="showRefPicker" @confirm="insertRefLink" />

    <!-- 编辑器内 AI 辅助（与章节编辑器共用同一面板：续写/润色/扩写/摘要，按量计费） -->
    <AiAssistPanel
      v-model:visible="showAiPanel"
      :context-text="aiContextText"
      @insert="insertIntoEditor"
      @replace="aiReplaceAll"
    />

    <!-- 批量导入对话框（CSDN 式：可指定导入到哪个专栏） -->
    <el-dialog v-model="showBatchDialog" title="批量导入博客" width="560px" destroy-on-close>
      <el-form label-width="80px">
        <el-form-item label="选择文件">
          <el-upload
            ref="batchUploadRef"
            :auto-upload="false"
            multiple
            :on-change="onBatchFileChange"
            accept=".md,.txt,.docx,.doc"
            drag
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">将多个 Word / Markdown / 文本文件拖到此处</div>
            <template #tip>
              <div class="el-upload__tip">每个文件将创建为一篇独立博客，标题取文件名（去扩展名）</div>
            </template>
          </el-upload>
        </el-form-item>
        <el-form-item label="导入到分类">
          <el-select v-model="batchCategoryId" placeholder="不选=导入后未分类" clearable style="width: 100%">
            <el-option v-for="c in myCatOptions" :key="c.id" :label="c.label" :value="c.id" />
          </el-select>
          <div class="el-upload__tip">选择后导入的博客将自动归入该分类（个人专栏）</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showBatchDialog = false">取消</el-button>
        <el-button type="primary" :loading="batchImporting" :disabled="batchFiles.length === 0" @click="doBatchImport">
          批量创建（{{ batchFiles.length }} 篇）
        </el-button>
      </template>
    </el-dialog>

    <!-- 标题区 -->
    <div class="be-header">
      <el-input v-model="blogTitle" placeholder="博客标题" size="large" class="be-title-input" />
      <div class="be-header-row">
        <el-input v-model="blogSummary" placeholder="摘要（选填）" size="small" class="be-summary-input" />
        <el-select
          v-model="blogTags"
          multiple
          filterable
          allow-create
          default-first-option
          placeholder="标签 # 输入回车创建"
          size="small"
          class="be-tags-select"
        />
        <el-select
          v-model="blogTopics"
          multiple
          filterable
          allow-create
          default-first-option
          placeholder="话题 # 选择或输入创建"
          size="small"
          class="be-topics-select"
        />
        <el-select
          v-model="blogCategoryId"
          placeholder="选择分类（我的专栏）"
          size="small"
          clearable
          class="be-cat-select"
        >
          <el-option v-for="c in myCatOptions" :key="c.id" :label="c.label" :value="c.id" />
        </el-select>
        <div class="be-media-btns">
          <el-button size="small" @click="openMedia('video')">🎬 视频</el-button>
          <el-button size="small" @click="openMedia('audio')">🎵 音频</el-button>
        </div>
        <div class="be-pinned">
          <el-switch v-model="blogPinned" size="small" />
          <span class="be-pinned-label">置顶</span>
        </div>
      </div>
    </div>

    <!-- 插入视频/音频弹窗（上传或粘贴 URL，仿 CSDN） -->
    <el-dialog v-model="mediaDialog" :title="mediaType === 'video' ? '🎬 插入视频' : '🎵 插入音频'" width="480px">
      <el-upload
        :auto-upload="false"
        :show-file-list="false"
        accept="video/*,audio/*"
        drag
        :on-change="onMediaFileChange"
      >
        <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
        <div class="el-upload__text">点击或拖拽上传{{ mediaType === 'video' ? '视频' : '音频' }}文件（≤50MB）</div>
      </el-upload>
      <div class="media-or">—— 或粘贴外链 URL ——</div>
      <el-input v-model="mediaUrl" :placeholder="mediaType === 'video' ? 'https://example.com/video.mp4' : 'https://example.com/audio.mp3'" />
      <template #footer>
        <el-button @click="mediaDialog = false">取消</el-button>
        <el-button type="primary" :loading="mediaUploading" @click="insertMedia">插入到正文</el-button>
      </template>
    </el-dialog>

    <!-- 编辑器 -->
    <div class="be-editor">
      <div v-show="editorMode === 'md'" class="be-md-wrapper">
        <div ref="cherryContainer" class="be-cherry-container"></div>
      </div>
      <div v-show="editorMode === 'richtext'" class="be-richtext-editor">
        <RichTextEditor v-model="richTextContent" placeholder="开始写作..." @save="saveBlog" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { blogApi, uploadApi } from '@/api/social';
import { ElMessage } from 'element-plus';
import { ArrowLeft, MagicStick, UploadFilled, Link } from '@element-plus/icons-vue';
import RichTextEditor from '@/components/RichTextEditor.vue';
import ContentRefPicker from '@/components/frame/ContentRefPicker.vue';
// engineConfig 已内置 physicsKatex（含 mhchem + physics 物理宏包）
import { renderMarkdown, editorEngineConfig } from '@/markdown/renderer';
import AiAssistPanel from '@/components/editor/AiAssistPanel.vue';
import { guardLeak } from '@/composables/useLeakGuard';
import { htmlToMarkdown } from '@/utils/turndown';

const route = useRoute();
const router = useRouter();

// ====== 批量导入 ======
const showBatchDialog = ref(false);
const batchImporting = ref(false);
const batchUploadRef = ref();
const batchFiles = ref<File[]>([]);
const batchCategoryId = ref<number | undefined>(undefined);

function onBatchFileChange(file: any) {
  if (file.raw) batchFiles.value.push(file.raw);
}

async function doBatchImport() {
  if (batchFiles.value.length === 0) { ElMessage.warning('请选择文件'); return; }
  batchImporting.value = true;
  try {
    const res = await blogApi.importBatch(batchFiles.value, batchCategoryId.value);
    if (res.data.code === 0) {
      const d = res.data.data;
      ElMessage.success(d.message || `成功导入 ${d.count} 篇博客`);
      showBatchDialog.value = false;
      batchFiles.value = [];
      batchCategoryId.value = undefined;
      batchUploadRef.value?.clearFiles();
      // 回到作者工作台，可看到新博客列表
      setTimeout(() => router.push('/author'), 600);
    } else {
      ElMessage.error(res.data.message || '批量导入失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '批量导入失败');
  } finally {
    batchImporting.value = false;
  }
}

const editingId = ref(Number(route.params.id) || 0);

const blogTitle = ref('');
const blogSummary = ref('');
const blogPinned = ref(false);
const blogCategoryId = ref<number | undefined>(undefined); // 提交用的分类（个人专栏，一级/二级/三级均可）
const blogTags = ref<string[]>([]); // 博客标签（#tag）
const blogTopics = ref<string[]>([]); // 博客话题（#话题）
const topicOptions = ref<{ name: string }[]>([]);
const myCats = ref<any[]>([]); // 我的分类（个人专栏，含待审核）
const blogContent = ref('');
const richTextContent = ref('');
const editorMode = ref<'md' | 'richtext'>('md');
const saving = ref(false);
const lastSaved = ref('');

// 我的分类平铺选项（带缩进/待审核标注）
const myCatOptions = computed(() =>
  (myCats.value || []).map((c: any) => ({
    id: c.id,
    label: (c.full_name || c.name) + (c.status === 'pending' ? '（待审核）' : ''),
  })),
);

async function loadCategories() {
  try {
    // 个人专栏：只加载自己创建的分类（含待审核/已拒绝状态）
    const res = await blogApi.getManageCategories();
    if (res.data.code === 0) {
      myCats.value = res.data.data?.mine || [];
    }
    // 已有话题（供选择）
    const t = await blogApi.getTopics();
    if (t.data.code === 0) topicOptions.value = t.data.data || [];
  } catch { /* ignore */ }
}

const cherryContainer = ref<HTMLDivElement | null>(null);
let cherryInstance: any = null;
let cherryReady = false;

async function getContentToSave(): Promise<string> {
  if (editorMode.value === 'richtext' && richTextContent.value.trim()) return await htmlToMarkdown(richTextContent.value);
  if (editorMode.value === 'md' && cherryInstance) return cherryInstance.getValue() || blogContent.value;
  return blogContent.value;
}

const editorHeight = computed(() => window.innerHeight - 260);

async function initCherryEditor() {
  if (!cherryContainer.value) return;
  const Cherry = (await import('cherry-markdown')).default;
  cherryInstance = new Cherry({
    el: cherryContainer.value,
    value: blogContent.value,
    editor: { defaultModel: 'edit&preview', height: `${editorHeight.value}px` },
    toolbars: {
      toolbar: ['undo', 'redo', '|', 'bold', 'italic', 'strikethrough', 'underline', '|',
        'header', '|', 'ul', 'ol', 'checklist', 'quote', 'code', '|',
        'table', '|', 'formula', 'graph', '|', 'image', 'link', '|',
        'hr', '|', 'toc', 'switchModel', 'fullScreen'],
      toolbarRight: ['togglePreview'],
    },
    ...editorEngineConfig,
    // CSDN 式图片上传：走统一 /api/upload
    imageUpload: async (file: File, callback: (url: string) => void) => {
      try {
        const res = await uploadApi.upload(file);
        const url = res.data.data?.url;
        callback(url || '');
        if (!url) ElMessage.error('图片上传失败');
      } catch (e: any) {
        ElMessage.error(e?.response?.data?.message || '图片上传失败');
        callback('');
      }
    },
    callback: { afterChange: (text: string) => { blogContent.value = text; } },
  });
  cherryReady = true;
}

// ===== 插入音视频（CSDN 式：上传文件或粘贴 URL）=====
const mediaDialog = ref(false);
const mediaType = ref<'video' | 'audio'>('video');
const mediaUrl = ref('');
const mediaFile = ref<File | null>(null);
const mediaUploading = ref(false);

function openMedia(type: 'video' | 'audio') {
  mediaType.value = type;
  mediaUrl.value = '';
  mediaFile.value = null;
  mediaDialog.value = true;
}

function onMediaFileChange(file: any) {
  if (file.raw) mediaFile.value = file.raw;
}

// ===== 编辑器内 AI 辅助 =====
const showAiPanel = ref(false);
const aiContextText = ref('');
// 优先用富文本纯文本，其次 Markdown 原文；截取尾部避免超长
async function openAiPanel() {
  try {
    const raw = editorMode.value === 'richtext'
      ? richTextContent.value.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim()
      : await getContentToSave();
    aiContextText.value = (raw || '').slice(-4000);
  } catch {
    aiContextText.value = '';
  }
  showAiPanel.value = true;
}

// AI 结果整体替换正文（Markdown 模式写回 Cherry；富文本模式按 HTML 段落写入）
async function aiReplaceAll(text: string, meta?: { model?: string }) {
  if (!text) return;
  aiUsed.value = true;
  if (meta?.model) aiModelName.value = meta.model;
  if (editorMode.value === 'md' && cherryInstance) {
    cherryInstance.setValue(text);
    blogContent.value = text;
  } else {
    const html = renderMarkdown(text);
    richTextContent.value = html;
  }
  ElMessage.success('已替换正文');
}

// AI 生成标识（176 号令）：用过 AI 且回填正文后置位，保存时提交
const aiUsed = ref(false);
const aiModelName = ref('');

function insertIntoEditor(text: string, meta?: { model?: string }) {
  aiUsed.value = true;
  if (meta?.model) aiModelName.value = meta.model;
  if (editorMode.value === 'md' && cherryInstance) {
    // Cherry 内部 CodeMirror 光标处插入；兼容 insert API
    const cm = cherryInstance.editor?.codemirror || cherryInstance.codemirror;
    if (cm && cm.replaceSelection) { cm.replaceSelection(text); }
    else if (cherryInstance.insert) { cherryInstance.insert(text); }
    else { blogContent.value += '\n\n' + text; }
  } else {
    richTextContent.value += (richTextContent.value ? '<p></p>' : '') + text;
  }
}

// ===== 站内内容引用（16.2）：正文引用其他小说/博客/论坛帖，相对路径可跳转 =====
const showRefPicker = ref(false);

function insertRefLink(link: string, ref: { type: string; id: number; title: string }) {
  insertIntoEditor(`[${ref.title || link}](${link})`);
  ElMessage.success('已插入引用');
}

async function insertMedia() {
  let url = mediaUrl.value.trim();
  if (!url && mediaFile.value) {
    mediaUploading.value = true;
    try {
      const res = await uploadApi.upload(mediaFile.value);
      url = res.data.data?.url || '';
      if (!url) ElMessage.error('上传失败');
    } catch (e: any) {
      ElMessage.error(e?.response?.data?.message || '上传失败');
      return;
    } finally { mediaUploading.value = false; }
  }
  if (!url) { ElMessage.warning('请上传文件或粘贴 URL'); return; }
  const html = mediaType.value === 'video'
    ? `<video src="${url}" controls preload="metadata" style="max-width:100%"></video>`
    : `<audio src="${url}" controls preload="metadata"></audio>`;
  insertIntoEditor(html);
  mediaDialog.value = false;
  mediaUrl.value = '';
  mediaFile.value = null;
  ElMessage.success('已插入' + (mediaType.value === 'video' ? '视频' : '音频'));
}

function destroyCherryEditor() {
  if (cherryInstance) { try { cherryInstance.destroy?.(); } catch {}; cherryInstance = null; }
  cherryReady = false;
}

async function onModeChange(mode: string) {
  if (mode === 'richtext') {
    const md = cherryInstance ? (cherryInstance.getValue() || '') : blogContent.value;
    richTextContent.value = renderMarkdown(md);
  } else if (mode === 'md') {
    if (richTextContent.value.trim()) {
      const md = await htmlToMarkdown(richTextContent.value);
      blogContent.value = md;
      if (cherryInstance) cherryInstance.setValue(md);
    }
    if (!cherryReady) nextTick(() => initCherryEditor());
  }
}

async function loadBlog() {
  if (!editingId.value) return;
  try {
    const res = await blogApi.getBlog(editingId.value);
    if (res.data.code === 0) {
      const b = res.data.data;
      blogTitle.value = b.title || '';
      blogContent.value = b.content || '';
      blogSummary.value = b.summary || '';
      blogPinned.value = b.is_pinned || false;
      blogCategoryId.value = b.category_id || undefined;
      blogTags.value = (b.tags || []).map((t: any) => t.name);
      blogTopics.value = (b.topics || []).map((t: any) => t.name);
      // 分类树可能尚未加载完成：重新拉一次（个人专栏）
      await loadCategories();
      if (editorMode.value === 'md') {
        if (cherryInstance) cherryInstance.setValue(b.content || '');
      } else {
        richTextContent.value = renderMarkdown(b.content || '');
      }
    }
  } catch { ElMessage.error('加载博客失败'); router.push('/author'); }
}

async function saveBlog() {
  if (!blogTitle.value.trim()) { ElMessage.warning('请输入标题'); return; }
  const content = await getContentToSave();
  if (!content.trim()) { ElMessage.warning('请输入内容'); return; }
  // 个人信息泄漏提示（本地检测，高危需确认；后端只收命中类型计数）
  if (!(await guardLeak(content, '博客正文'))) return;
  saving.value = true;
  try {
    const payload = {
      title: blogTitle.value,
      content,
      summary: blogSummary.value,
      is_pinned: blogPinned.value,
      category_id: blogCategoryId.value,
      tags: blogTags.value,
      topics: blogTopics.value,
      ai_assisted: aiUsed.value || undefined,
      ai_model: aiUsed.value ? aiModelName.value : undefined,
    };
    if (editingId.value) {
      await blogApi.update(editingId.value, payload);
    } else {
      const res = await blogApi.create(payload);
      editingId.value = res.data.data.id;
      router.replace(`/author/blog/edit/${editingId.value}`);
    }
    const now = new Date();
    lastSaved.value = `${now.getHours().toString().padStart(2,'0')}:${now.getMinutes().toString().padStart(2,'0')}:${now.getSeconds().toString().padStart(2,'0')}`;
    ElMessage.success('已保存');
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '保存失败'); }
  finally { saving.value = false; }
}

function onKeyDown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') { e.preventDefault(); saveBlog(); }
}

onMounted(() => {
  window.addEventListener('keydown', onKeyDown);
  loadCategories();
  loadBlog();
  if (editorMode.value === 'md') nextTick(() => initCherryEditor());
});

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeyDown);
  destroyCherryEditor();
});
</script>

<style scoped>
.blog-editor { display: flex; flex-direction: column; height: 100vh; background: var(--bg-color); }

/* 顶部工具栏 */
.be-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  height: 52px; padding: 0 16px;
  background: #fff; border-bottom: 1px solid var(--border-color);
  flex-shrink: 0; z-index: 10;
}
[data-theme="dark"] .be-toolbar { background: #1e293b; border-bottom-color: #334155; }

.be-toolbar-left, .be-toolbar-right { flex: 1; display: flex; align-items: center; }
.be-toolbar-right { justify-content: flex-end; gap: 12px; }
.be-toolbar-center {
  display: flex; align-items: center; gap: 16px;
}
.be-page-title { font-weight: 600; font-size: 0.95rem; color: var(--text-primary); white-space: nowrap; }
.be-saved-time { font-size: 0.8rem; color: #999; }

/* 标题区 */
.be-header {
  padding: 16px 16px 0;
  flex-shrink: 0;
}
.be-title-input { --el-input-focus-border-color: var(--primary-color); }
.be-header-row { display: flex; align-items: center; gap: 12px; margin-top: 8px; flex-wrap: wrap; }
.be-summary-input { flex: 1; min-width: 200px; }
.be-tags-select { width: 200px; flex-shrink: 0; }
.be-topics-select { width: 200px; flex-shrink: 0; }
.be-cat-select { width: 200px; flex-shrink: 0; }
.be-media-btns { display: flex; gap: 6px; flex-shrink: 0; }
.be-pinned { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
.be-pinned-label { font-size: 0.8rem; color: #999; white-space: nowrap; }
.media-or { text-align: center; color: #999; font-size: 0.85rem; margin: 14px 0 8px; }

/* 编辑器区域 */
.be-editor { flex: 1; overflow: hidden; padding: 0 16px 16px; display: flex; }
.be-md-wrapper { flex: 1; overflow: hidden; display: flex; }
.be-cherry-container { flex: 1; overflow: hidden; }
.be-richtext-editor { flex: 1; overflow: auto; }

/* ====== 移动端博客编辑器适配 ====== */
@media (max-width: 768px) {
  .be-toolbar {
    height: auto;
    min-height: 52px;
    padding: 6px 10px;
    flex-wrap: wrap;
    gap: 6px;
  }
  .be-page-title {
    display: none;
  }
  .be-saved-time {
    display: none;
  }
  .be-header {
    padding: 10px 10px 0;
  }
  .be-header-row {
    flex-wrap: wrap;
  }
  .be-cat-select, .be-tags-select, .be-topics-select {
    width: 100%;
    flex: 1 0 100%;
  }
  .be-editor {
    padding: 0 10px 10px;
  }
}
</style>