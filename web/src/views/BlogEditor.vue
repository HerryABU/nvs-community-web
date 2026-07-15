<template>
  <div class="blog-editor">
    <!-- 顶部工具栏：三列布局，全部垂直居中 -->
    <div class="be-toolbar">
      <div class="be-toolbar-left">
        <el-button text @click="$router.push('/author')">
          <el-icon><ArrowLeft /></el-icon>返回工作台
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
        <el-button type="primary" :loading="saving" @click="saveBlog">保存</el-button>
      </div>
    </div>

    <!-- 标题区 -->
    <div class="be-header">
      <el-input v-model="blogTitle" placeholder="博客标题" size="large" class="be-title-input" />
      <div class="be-header-row">
        <el-input v-model="blogSummary" placeholder="摘要（选填）" size="small" class="be-summary-input" />
        <div class="be-pinned">
          <el-switch v-model="blogPinned" size="small" />
          <span class="be-pinned-label">置顶</span>
        </div>
      </div>
    </div>

    <!-- 编辑器 -->
    <div class="be-editor">
      <div v-show="editorMode === 'md'" class="be-md-wrapper">
        <div ref="cherryContainer" class="be-cherry-container"></div>
      </div>
      <div v-show="editorMode === 'richtext'" class="be-richtext-editor">
        <RichTextEditor v-model="richTextContent" placeholder="开始写作..." />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { blogApi } from '@/api/social';
import { ElMessage } from 'element-plus';
import { ArrowLeft } from '@element-plus/icons-vue';
import RichTextEditor from '@/components/RichTextEditor.vue';
import TurndownService from 'turndown';
// engineConfig 已内置 physicsKatex（含 mhchem + physics 物理宏包）
import { renderMarkdown, engineConfig } from '@/markdown/renderer';

const route = useRoute();
const router = useRouter();

const editingId = ref(Number(route.params.id) || 0);

const blogTitle = ref('');
const blogSummary = ref('');
const blogPinned = ref(false);
const blogContent = ref('');
const richTextContent = ref('');
const editorMode = ref<'md' | 'richtext'>('md');
const saving = ref(false);
const lastSaved = ref('');

const cherryContainer = ref<HTMLDivElement | null>(null);
let cherryInstance: any = null;
let cherryReady = false;

const turndown = new TurndownService({ headingStyle: 'atx', hr: '---', bulletListMarker: '-', codeBlockStyle: 'fenced' });

function getContentToSave(): string {
  if (editorMode.value === 'richtext' && richTextContent.value.trim()) return turndown.turndown(richTextContent.value);
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
    ...engineConfig,
    callback: { afterChange: (text: string) => { blogContent.value = text; } },
  });
  cherryReady = true;
}

function destroyCherryEditor() {
  if (cherryInstance) { try { cherryInstance.destroy?.(); } catch {}; cherryInstance = null; }
  cherryReady = false;
}

function onModeChange(mode: string) {
  if (mode === 'richtext') {
    const md = cherryInstance ? (cherryInstance.getValue() || '') : blogContent.value;
    richTextContent.value = renderMarkdown(md);
  } else if (mode === 'md') {
    if (richTextContent.value.trim()) {
      const md = turndown.turndown(richTextContent.value);
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
  const content = getContentToSave();
  if (!content.trim()) { ElMessage.warning('请输入内容'); return; }
  saving.value = true;
  try {
    if (editingId.value) {
      await blogApi.update(editingId.value, { title: blogTitle.value, content, summary: blogSummary.value, is_pinned: blogPinned.value });
    } else {
      const res = await blogApi.create({ title: blogTitle.value, content, summary: blogSummary.value });
      editingId.value = res.data.data.id;
      router.replace(`/author/blog/${editingId.value}`);
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
.be-header-row { display: flex; align-items: center; gap: 12px; margin-top: 8px; }
.be-summary-input { flex: 1; }
.be-pinned { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
.be-pinned-label { font-size: 0.8rem; color: #999; white-space: nowrap; }

/* 编辑器区域 */
.be-editor { flex: 1; overflow: hidden; padding: 0 16px 16px; display: flex; }
.be-md-wrapper { flex: 1; overflow: hidden; display: flex; }
.be-cherry-container { flex: 1; overflow: hidden; }
.be-richtext-editor { flex: 1; overflow: auto; }
</style>