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
        <el-button :loading="batchImporting" @click="showBatchDialog = true" title="批量导入多个文件为多篇博客">
          <el-icon><UploadFilled /></el-icon><span class="desktop-only"> 批量导入</span>
        </el-button>
        <el-button type="primary" :loading="saving" @click="saveBlog">保存</el-button>
      </div>
    </div>

    <!-- 批量导入对话框 -->
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
        <el-cascader
          v-model="blogCategoryPath"
          :options="categoryCascader"
          :props="{ expandTrigger: 'hover', emitPath: true }"
          placeholder="选择分类（可选，支持三级）"
          size="small"
          clearable
          class="be-cat-select"
        />
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
import { ArrowLeft, UploadFilled } from '@element-plus/icons-vue';
import RichTextEditor from '@/components/RichTextEditor.vue';
// engineConfig 已内置 physicsKatex（含 mhchem + physics 物理宏包）
import { renderMarkdown, editorEngineConfig } from '@/markdown/renderer';
import { htmlToMarkdown } from '@/utils/turndown';

const route = useRoute();
const router = useRouter();

// ====== 批量导入 ======
const showBatchDialog = ref(false);
const batchImporting = ref(false);
const batchUploadRef = ref();
const batchFiles = ref<File[]>([]);

function onBatchFileChange(file: any) {
  if (file.raw) batchFiles.value.push(file.raw);
}

async function doBatchImport() {
  if (batchFiles.value.length === 0) { ElMessage.warning('请选择文件'); return; }
  batchImporting.value = true;
  try {
    const res = await blogApi.importBatch(batchFiles.value);
    if (res.data.code === 0) {
      const d = res.data.data;
      ElMessage.success(d.message || `成功导入 ${d.count} 篇博客`);
      showBatchDialog.value = false;
      batchFiles.value = [];
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
const blogCategoryId = ref<number | undefined>(undefined); // 提交用的分类（叶子 id）
const blogCategoryPath = ref<number[]>([]); // 级联选择路径
const categoryTree = ref<any[]>([]);
const blogContent = ref('');
const richTextContent = ref('');
const editorMode = ref<'md' | 'richtext'>('md');
const saving = ref(false);
const lastSaved = ref('');

// 分类树 → el-cascader options（三级）
const categoryCascader = computed(() =>
  (categoryTree.value || []).map((c: any) => ({
    value: c.id,
    label: c.name,
    children: (c.children || []).map((s: any) => ({
      value: s.id,
      label: s.name,
      children: (s.children || []).map((g: any) => ({ value: g.id, label: g.name })),
    })),
  })),
);

// 反查分类路径：给定分类 id，找到 [一级, 二级, 三级] 路径
function findCatPath(target: number): number[] {
  for (const c of categoryTree.value) {
    if (c.id === target) return [c.id];
    for (const s of c.children || []) {
      if (s.id === target) return [c.id, s.id];
      for (const g of s.children || []) {
        if (g.id === target) return [c.id, s.id, g.id];
      }
    }
  }
  return [];
}

async function loadCategories() {
  try {
    const res = await blogApi.getCategories();
    if (res.data.code === 0) {
      categoryTree.value = res.data.data || [];
      // 编辑模式：已有分类 → 恢复级联路径
      if (blogCategoryId.value && categoryTree.value.length) {
        blogCategoryPath.value = findCatPath(blogCategoryId.value);
      }
    }
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
    callback: { afterChange: (text: string) => { blogContent.value = text; } },
  });
  cherryReady = true;
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
      // 分类树可能尚未加载完成：重新拉一次以恢复级联路径（避免竞态导致编辑时分类显示为空）
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
  // 级联选择：取路径最后一项为叶子分类 id
  blogCategoryId.value = blogCategoryPath.value.length
    ? blogCategoryPath.value[blogCategoryPath.value.length - 1]
    : undefined;
  saving.value = true;
  try {
    if (editingId.value) {
      await blogApi.update(editingId.value, { title: blogTitle.value, content, summary: blogSummary.value, is_pinned: blogPinned.value, category_id: blogCategoryId.value });
    } else {
      const res = await blogApi.create({ title: blogTitle.value, content, summary: blogSummary.value, category_id: blogCategoryId.value });
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
.be-header-row { display: flex; align-items: center; gap: 12px; margin-top: 8px; }
.be-summary-input { flex: 1; }
.be-cat-select { width: 200px; flex-shrink: 0; }
.be-pinned { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
.be-pinned-label { font-size: 0.8rem; color: #999; white-space: nowrap; }

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
  .be-cat-select {
    width: 100%;
    flex: 1 0 100%;
  }
  .be-editor {
    padding: 0 10px 10px;
  }
}
</style>