<template>
  <div class="page-container">
    <div class="editor-header">
      <h1>{{ isEditing ? '编辑作品' : '新建作品' }}</h1>
      <div class="header-actions">
        <el-button @click="$router.back()">返回</el-button>
        <el-button type="primary" :loading="saving" @click="saveNovel">保存</el-button>
      </div>
    </div>

    <!-- 作品基本信息 -->
    <el-card class="editor-section">
      <template #header>作品信息</template>
      <el-form :model="form" label-width="80px">
        <el-row :gutter="20">
          <el-col :span="16">
            <el-form-item label="标题" required>
              <el-input v-model="form.title" placeholder="请输入作品标题" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="分类">
              <el-select v-model="form.categories" multiple placeholder="可选择多个分类" style="width:100%" filterable allow-create>
                <el-option v-for="cat in categories" :key="cat" :label="cat" :value="cat" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="简介">
          <el-input v-model="form.summary" type="textarea" :rows="4" placeholder="作品简介" />
        </el-form-item>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="封面URL">
              <el-input v-model="form.cover_url" placeholder="封面图片链接（可选）" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="定价(元/章)">
              <el-input-number v-model="form.price_per_chapter" :min="0" :precision="2" :step="0.5" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio value="draft">草稿</el-radio>
            <el-radio value="published">发布</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 章节管理（仅编辑模式） -->
    <template v-if="isEditing && novelId">
      <el-card class="editor-section">
        <template #header>
          <div class="section-header">
            <span>章节管理（共 {{ chapters.length }} 章）</span>
            <div class="section-actions">
              <el-button type="success" size="small" @click="showImportDialog = true">
                <el-icon><Upload /></el-icon>导入文件
              </el-button>
              <el-button type="primary" size="small" @click="goWriteFirst">
                <el-icon><Plus /></el-icon>新增章节
              </el-button>
            </div>
          </div>
        </template>
        <el-empty v-if="chapters.length === 0" description="暂无章节，点击「新增章节」或「导入文件」开始创作" />
        <div v-else class="chapter-list">
          <div v-for="ch in chapters" :key="ch.chapter_number" class="chapter-item">
            <span class="ch-num">第{{ ch.chapter_number }}章</span>
            <span class="ch-title">{{ ch.title }}</span>
            <span class="ch-words">{{ ch.word_count }}字</span>
            <div class="ch-actions">
              <el-button size="small" text @click="goWrite(ch.chapter_number)">编辑</el-button>
              <el-button size="small" text type="danger" @click="deleteChapter(ch)">删除</el-button>
            </div>
          </div>
        </div>
      </el-card>
    </template>

    <!-- 导入文件对话框 -->
    <el-dialog v-model="showImportDialog" title="导入文件" width="700px" destroy-on-close @closed="resetImport">
      <el-form label-width="80px" v-if="!importPreviewData">
        <el-form-item label="作品标题">
          <el-input v-model="importForm.title" placeholder="导入后作品标题（可选）" />
        </el-form-item>
        <el-form-item label="选择文件">
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :limit="1"
            :on-change="onFileChange"
            :on-remove="onFileRemove"
            accept=".md,.txt,.html"
            drag
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">
              将 MD / TXT 文件拖到此处，或<em>点击上传</em>
            </div>
            <template #tip>
              <div class="el-upload__tip">
                支持 Markdown (.md)、纯文本 (.txt)。上传后可预览章节划分并调整分割规则。
              </div>
            </template>
          </el-upload>
        </el-form-item>
      </el-form>

      <!-- 预览阶段 -->
      <div v-if="importPreviewData" class="import-preview">
        <div class="import-preview-header">
          <span>共解析出 <strong>{{ importPreviewData.chapters?.length || 0 }}</strong> 章</span>
          <el-button size="small" text @click="resetImport">重新选择文件</el-button>
        </div>
        <el-form label-width="100px" size="small" style="margin-bottom:12px">
          <el-form-item label="自定义分割规则">
            <el-input v-model="importForm.splitRule" placeholder="默认自动识别。也可输入正则，如：(?m)^第[一二三四五六七八九十百千0-9]+章">
              <template #append>
                <el-button @click="doPreview" :loading="importPreviewing">重新解析</el-button>
              </template>
            </el-input>
            <div class="import-rule-hint">
              快捷规则：
              <el-button size="small" text type="primary" @click="importForm.splitRule='(?m)^第[一二三四五六七八九十百千\\d]+章'">第X章</el-button>
              <el-button size="small" text type="primary" @click="importForm.splitRule='(?m)^#{1,3}\\s'"># / ## / ###</el-button>
              <el-button size="small" text type="primary" @click="importForm.splitRule='(?m)^###\\s'">### 仅三级</el-button>
              <el-button size="small" text type="primary" @click="importForm.splitRule='(?m)^##\\s'">## 仅二级</el-button>
              <el-button size="small" text type="primary" @click="importForm.splitRule='(?m)^-{3,}$'">--- 分割线</el-button>
            </div>
          </el-form-item>
        </el-form>
        <div class="import-chapter-list">
          <div v-for="(ch, idx) in importChapters" :key="idx" class="import-chapter-item">
            <span class="import-ch-num">{{ idx + 1 }}</span>
            <el-input v-model="ch.title" size="small" style="flex:1" placeholder="章节标题" />
            <span class="import-ch-words">{{ ch.words }}字</span>
            <el-button size="small" text type="danger" @click="importChapters.splice(idx, 1)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
          <el-empty v-if="importChapters.length === 0" description="已删除所有章节" :image-size="40" />
        </div>
      </div>

      <template #footer>
        <el-button @click="showImportDialog = false">取消</el-button>
        <el-button v-if="!importPreviewData" type="primary" :loading="importPreviewing" @click="doPreview" :disabled="!importFile">
          预览章节
        </el-button>
        <el-button v-else type="primary" :loading="importing" @click="doImport">确认导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { novelApi, type Novel, type Chapter } from '@/api/novel';
import { publicApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Upload, Plus, UploadFilled } from '@element-plus/icons-vue';

const route = useRoute();
const router = useRouter();

const categories = ref<string[]>([]);

const novelId = ref<number | null>(route.params.id ? Number(route.params.id) : null);
const isEditing = !!novelId.value;

const saving = ref(false);
const chapters = ref<Chapter[]>([]);

const form = reactive<Partial<Novel>>({
  title: '',
  category: '其他',
  categories: [] as string[],
  summary: '',
  cover_url: '',
  price_per_chapter: 0,
  status: 'draft',
});

// 导入相关
const showImportDialog = ref(false);
const importing = ref(false);
const importPreviewing = ref(false);
const uploadRef = ref();
const importFile = ref<File | null>(null);
const importPreviewData = ref<any>(null);
const importChapters = ref<{ title: string; words: number }[]>([]);
const importForm = reactive({ title: '', splitRule: '' });

async function loadNovel() {
  if (!novelId.value) return;
  try {
    const [novelRes, chaptersRes] = await Promise.all([
      novelApi.getNovel(novelId.value),
      novelApi.getChapters(novelId.value),
    ]);
    const novel = novelRes.data.data;
    Object.assign(form, {
      title: novel.title,
      category: novel.category,
      categories: novel.categories || (novel.category ? [novel.category] : []),
      summary: novel.summary,
      cover_url: novel.cover_url,
      price_per_chapter: novel.price_per_chapter,
      status: novel.status,
    });
    chapters.value = chaptersRes.data.data || [];
  } catch (e) {
    console.error(e);
  }
}

async function saveNovel() {
  if (!form.title) {
    ElMessage.warning('请输入作品标题');
    return;
  }
  saving.value = true;
  try {
    if (isEditing && novelId.value) {
      await novelApi.updateNovel(novelId.value, form);
      ElMessage.success('作品已更新');
    } else {
      const res = await novelApi.createNovel(form);
      ElMessage.success('作品已创建');
      router.replace(`/author/editor/${res.data.data.id}`);
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

function goWrite(chapterNum: number) {
  router.push(`/author/editor/${novelId.value}/chapter/${chapterNum}`);
}

function goWriteFirst() {
  const nextNum = chapters.value.length > 0
    ? Math.max(...chapters.value.map(c => c.chapter_number)) + 1
    : 1;
  router.push(`/author/editor/${novelId.value}/chapter/${nextNum}`);
}

async function deleteChapter(ch: Chapter) {
  try {
    await ElMessageBox.confirm(`确认删除「第${ch.chapter_number}章 ${ch.title}」？`, '删除确认', {
      confirmButtonText: '确认删除',
      type: 'warning',
    });
    await novelApi.deleteChapter(novelId.value!, ch.chapter_number);
    chapters.value = chapters.value.filter(c => c.chapter_number !== ch.chapter_number);
    ElMessage.success('已删除');
  } catch { /* 取消 */ }
}

// 导入文件
function onFileChange(file: any) {
  importFile.value = file.raw;
  importPreviewData.value = null;
  importChapters.value = [];
}

function onFileRemove() {
  importFile.value = null;
  importPreviewData.value = null;
}

function resetImport() {
  importFile.value = null;
  importPreviewData.value = null;
  importChapters.value = [];
  importForm.splitRule = '';
  uploadRef.value?.clearFiles();
}

async function doPreview() {
  if (!importFile.value) {
    ElMessage.warning('请选择文件');
    return;
  }
  importPreviewing.value = true;
  try {
    const res = await novelApi.importPreview(importFile.value, importForm.splitRule || undefined);
    if (res.data.code === 0) {
      importPreviewData.value = res.data.data;
      importChapters.value = (res.data.data.chapters || []).map((ch: any) => ({
        title: ch.title,
        words: ch.words,
      }));
      ElMessage.success(`解析完成，共 ${res.data.data.total} 章`);
    } else {
      ElMessage.error(res.data.message || '解析失败');
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '预览失败');
  } finally {
    importPreviewing.value = false;
  }
}

async function doImport() {
  if (!importFile.value) return;
  importing.value = true;
  try {
    // 编辑模式：追加到已有小说；新建模式：创建新小说
    const targetNovelId = novelId.value || undefined;
    const res = await novelApi.importNovel(
      importFile.value,
      importForm.title || importFile.value.name.replace(/\.[^.]+$/, ''),
      (form.categories && form.categories.length > 0) ? form.categories[0] : '其他',
      targetNovelId,
    );
    const data = res.data.data;
    ElMessage.success(`导入成功！共 ${data.chapters_count} 章`);

    if (!novelId.value && data.novel_id) {
      router.replace(`/author/editor/${data.novel_id}`);
      return;
    }

    if (novelId.value) {
      const chaptersRes = await novelApi.getChapters(novelId.value);
      chapters.value = chaptersRes.data.data || [];
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '导入失败');
  } finally {
    importing.value = false;
    showImportDialog.value = false;
    resetImport();
  }
}

async function loadCategories() {
  try {
    const res = await publicApi.getCategories();
    if (res.data.code === 0 && Array.isArray(res.data.data)) {
      categories.value = res.data.data;
    }
  } catch (e) { /* ignore */ }
}

onMounted(() => {
  loadCategories();
  if (isEditing) loadNovel();
});
</script>

<style scoped>
.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.editor-header h1 {
  font-size: 1.5rem;
  color: var(--primary-color);
}

.header-actions {
  display: flex;
  gap: 12px;
}

.editor-section {
  margin-bottom: 24px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.section-actions {
  display: flex;
  gap: 8px;
}

.chapter-item {
  display: flex;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid #f0f0f0;
}

.chapter-item:last-child {
  border-bottom: none;
}

.ch-num {
  color: var(--text-light);
  min-width: 80px;
}

.ch-title {
  flex: 1;
  font-weight: 500;
}

.ch-words {
  color: var(--text-light);
  font-size: 0.85rem;
  margin-right: 16px;
}

/* 导入预览 */
.import-preview {
  max-height: 60vh;
  overflow-y: auto;
}

.import-preview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid #f0f0f0;
}

.import-rule-hint {
  margin-top: 4px;
  font-size: 0.8rem;
  color: #999;
}

.import-chapter-list {
  max-height: 300px;
  overflow-y: auto;
}

.import-chapter-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid #f5f5f5;
}

.import-ch-num {
  font-weight: 600;
  min-width: 28px;
  color: var(--primary-color);
  text-align: center;
}

.import-ch-words {
  font-size: 0.8rem;
  color: #999;
  white-space: nowrap;
}
</style>
