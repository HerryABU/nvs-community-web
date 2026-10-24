<template>
  <div class="at">
    <div class="at-head">
      <div>
        <h1 class="page-title">提示词模板</h1>
        <p class="at-sub">类型文学默认框架 + 站长自定义：变量注入 · 版本回滚 · 按场景挂载 · 渲染预览</p>
      </div>
      <div class="at-actions">
        <el-select v-model="scene" size="small" clearable placeholder="全部场景" style="width:140px" @change="load">
          <el-option label="章节编辑器" value="novel_chapter" />
          <el-option label="博客编辑器" value="blog" />
          <el-option label="AI 工作台" value="chat" />
        </el-select>
        <el-button size="small" type="primary" @click="openCreate">新建模板</el-button>
      </div>
    </div>

    <el-card shadow="never" class="at-card">
      <el-table :data="list" size="small" v-loading="loading">
        <el-table-column label="模板" min-width="220">
          <template #default="{ row }">
            <div class="at-title">
              {{ row.title }}
              <el-tag v-if="row.builtin" size="small" type="info" effect="plain">内置</el-tag>
              <el-tag v-if="!row.enabled" size="small" type="warning" effect="plain">已停用</el-tag>
            </div>
            <div class="at-meta">{{ sceneLabel(row.scene) }} · {{ row.category || '未分类' }} · v{{ row.version }}</div>
          </template>
        </el-table-column>
        <el-table-column prop="key" label="标识" width="150" />
        <el-table-column label="变量" width="90">
          <template #default="{ row }">{{ (row.variables || []).length }} 个</template>
        </el-table-column>
        <el-table-column prop="priority" label="优先级" width="90" />
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" text @click="openVersions(row)">版本</el-button>
            <el-button size="small" text type="danger" :disabled="row.builtin" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty><EmptyState icon="🧩" text="暂无模板" /></template>
      </el-table>
    </el-card>

    <el-dialog v-model="editorVisible" :title="editing ? `编辑模板（v${editing.version}）` : '新建模板'" width="820px" top="4vh">
      <el-form label-width="88px">
        <div class="at-row2">
          <el-form-item label="标题" required>
            <el-input v-model="form.title" maxlength="64" />
          </el-form-item>
          <el-form-item label="标识 key">
            <el-input v-model="form.key" placeholder="留空自动生成，如 tl-continue" :disabled="!!editing?.builtin" />
          </el-form-item>
        </div>
        <div class="at-row3">
          <el-form-item label="场景">
            <el-select v-model="form.scene" style="width:100%">
              <el-option label="章节编辑器" value="novel_chapter" />
              <el-option label="博客编辑器" value="blog" />
              <el-option label="AI 工作台" value="chat" />
            </el-select>
          </el-form-item>
          <el-form-item label="分类">
            <el-input v-model="form.category" maxlength="32" />
          </el-form-item>
          <el-form-item label="类型">
            <el-select v-model="form.kind" style="width:100%">
              <el-option label="文本" value="chat" />
              <el-option label="绘画" value="image" />
              <el-option label="视频" value="video" />
            </el-select>
          </el-form-item>
        </div>
        <div class="at-row3">
          <el-form-item label="优先级">
            <el-input-number v-model="form.priority" :min="0" :max="1000" controls-position="right" />
          </el-form-item>
          <el-form-item label="启用">
            <el-switch v-model="form.enabled" />
          </el-form-item>
          <el-form-item label="备注">
            <el-input v-model="form.note" maxlength="256" />
          </el-form-item>
        </div>
        <el-form-item label="模板内容" required>
          <el-input v-model="form.content" type="textarea" :rows="9" maxlength="8000" show-word-limit
            placeholder="用 {{变量名}} 占位，例如：【前文】{{前文}}" />
        </el-form-item>

        <el-form-item label="变量声明">
          <div class="at-vars">
            <div v-for="(v, i) in form.variables" :key="i" class="at-var-row">
              <el-input v-model="v.name" size="small" placeholder="变量名" style="width:140px" />
              <el-input v-model="v.label" size="small" placeholder="显示名" style="width:120px" />
              <el-checkbox v-model="v.required" size="small">必填</el-checkbox>
              <el-input v-model="v.default" size="small" placeholder="默认值" style="width:110px" />
              <el-input-number v-model="v.max_len" :min="0" :max="20000" size="small" controls-position="right" style="width:110px" />
              <el-button size="small" text type="danger" @click="form.variables.splice(i, 1)">删除</el-button>
            </div>
            <el-button size="small" @click="addVar">+ 添加变量</el-button>
          </div>
        </el-form-item>

        <el-form-item label="渲染预览">
          <div class="at-preview">
            <div class="at-preview-vars">
              <div v-for="v in form.variables" :key="v.name" class="at-preview-var">
                <span class="at-preview-label">{{ v.label || v.name }}<em v-if="v.required">*</em></span>
                <el-input v-model="previewVars[v.name]" size="small" :placeholder="v.default || `填入 ${v.name}`" />
              </div>
              <div v-if="!form.variables.length" class="at-hint">先添加变量后即可填写预览值</div>
            </div>
            <div class="at-preview-out">
              <el-button size="small" :loading="previewing" @click="doPreview">渲染</el-button>
              <div v-if="missing.length" class="at-missing">缺少必填：{{ missing.join('、') }}</div>
              <pre class="at-preview-text">{{ previewText || '（渲染结果会显示在这里）' }}</pre>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editorVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="versionsVisible" title="版本历史" size="min(620px, 94vw)">
      <div v-for="v in versions" :key="v.id" class="at-ver">
        <div class="at-ver-head">
          <span>v{{ v.version }}</span>
          <span class="at-ver-time">{{ formatTime(v.created_at) }}</span>
          <el-button size="small" text type="primary" @click="restore(v.version)">回滚到此版本</el-button>
        </div>
        <pre class="at-ver-text">{{ v.content }}</pre>
      </div>
      <EmptyState v-if="!versions.length" icon="🕘" text="暂无历史版本（保存修改后生成）" />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { templateApi, type PromptTemplate, type TemplateVar } from '@/api/ai/template';
import EmptyState from '@/components/common/EmptyState.vue';

const list = ref<PromptTemplate[]>([]);
const loading = ref(false);
const scene = ref('');

const editorVisible = ref(false);
const editing = ref<PromptTemplate | null>(null);
const saving = ref(false);
const form = reactive<{
  key: string; title: string; scene: string; category: string; kind: string;
  content: string; variables: TemplateVar[]; enabled: boolean; priority: number; note: string;
}>({
  key: '', title: '', scene: 'novel_chapter', category: '', kind: 'chat',
  content: '', variables: [], enabled: true, priority: 0, note: '',
});

const previewVars = reactive<Record<string, string>>({});
const previewText = ref('');
const missing = ref<string[]>([]);
const previewing = ref(false);

const versionsVisible = ref(false);
const versions = ref<any[]>([]);
const versionTarget = ref<PromptTemplate | null>(null);

function sceneLabel(s: string) {
  return s === 'blog' ? '博客编辑器' : s === 'chat' ? 'AI 工作台' : '章节编辑器';
}
function formatTime(s: string) {
  if (!s) return '';
  const d = new Date(s);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

async function load() {
  loading.value = true;
  try {
    const res = await templateApi.adminList({ scene: scene.value || undefined });
    if (res.data.code === 0) list.value = res.data.data.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  }
  loading.value = false;
}

function resetForm() {
  Object.assign(form, {
    key: '', title: '', scene: 'novel_chapter', category: '', kind: 'chat',
    content: '', variables: [], enabled: true, priority: 0, note: '',
  });
  previewText.value = '';
  missing.value = [];
  Object.keys(previewVars).forEach((k) => delete previewVars[k]);
}

function openCreate() {
  editing.value = null;
  resetForm();
  editorVisible.value = true;
}

function openEdit(row: PromptTemplate) {
  editing.value = row;
  resetForm();
  Object.assign(form, {
    key: row.key, title: row.title, scene: row.scene, category: row.category,
    kind: row.kind, content: row.content,
    variables: (row.variables || []).map((v) => ({ ...v })),
    enabled: row.enabled, priority: row.priority, note: row.note,
  });
  editorVisible.value = true;
}

function addVar() {
  form.variables.push({ name: '', label: '', required: false, default: '', max_len: 4000 });
}

async function doPreview() {
  if (!form.content.trim()) {
    ElMessage.warning('请先填写模板内容');
    return;
  }
  previewing.value = true;
  try {
    const res = await templateApi.adminPreview({
      content: form.content,
      variables: form.variables.filter((v) => v.name.trim()),
      vars: { ...previewVars },
    });
    if (res.data.code === 0) {
      previewText.value = res.data.data.text;
      missing.value = res.data.data.missing || [];
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '预览失败');
  }
  previewing.value = false;
}

async function save() {
  if (!form.title.trim() || !form.content.trim()) {
    ElMessage.warning('标题和内容不能为空');
    return;
  }
  saving.value = true;
  try {
    const payload = {
      key: form.key.trim(),
      title: form.title.trim(),
      scene: form.scene,
      category: form.category.trim(),
      kind: form.kind,
      content: form.content,
      variables: form.variables.filter((v) => v.name.trim()),
      enabled: form.enabled,
      priority: form.priority,
      note: form.note.trim(),
    };
    const res = editing.value
      ? await templateApi.adminUpdate(editing.value.id, payload)
      : await templateApi.adminCreate(payload);
    if (res.data.code === 0) {
      ElMessage.success('已保存');
      editorVisible.value = false;
      await load();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  saving.value = false;
}

async function remove(row: PromptTemplate) {
  try {
    await ElMessageBox.confirm(`删除模板「${row.title}」？内置模板只能停用。`, '删除确认', { type: 'warning' });
  } catch { return; }
  try {
    await templateApi.adminDelete(row.id);
    ElMessage.success('已删除');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function openVersions(row: PromptTemplate) {
  versionTarget.value = row;
  versions.value = [];
  versionsVisible.value = true;
  try {
    const res = await templateApi.adminVersions(row.id);
    if (res.data.code === 0) versions.value = res.data.data.list || [];
  } catch { /* ignore */ }
}

async function restore(version: number) {
  if (!versionTarget.value) return;
  try {
    await ElMessageBox.confirm(`回滚到 v${version}？当前内容会自动存档。`, '版本回滚', { type: 'warning' });
  } catch { return; }
  try {
    await templateApi.adminRestore(versionTarget.value.id, version);
    ElMessage.success('已回滚');
    versionsVisible.value = false;
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '回滚失败');
  }
}

onMounted(load);
</script>

<style scoped>
.at { max-width: 1080px; margin: 0 auto; }
.at-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.at-sub { margin: 6px 0 14px; font-size: 0.82rem; color: var(--text-light, #64748b); }
.at-actions { display: flex; gap: 8px; }
.at-card { border-radius: 12px; }
.at-title { display: flex; align-items: center; gap: 6px; font-weight: 600; font-size: 0.88rem; }
.at-meta { font-size: 0.72rem; color: var(--text-light, #94a3b8); margin-top: 2px; }
.at-row2 { display: grid; grid-template-columns: 1.4fr 1fr; gap: 8px; }
.at-row3 { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
.at-vars { width: 100%; display: flex; flex-direction: column; gap: 6px; }
.at-var-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.at-preview { display: grid; grid-template-columns: 1fr 1.1fr; gap: 12px; width: 100%; }
.at-preview-vars { display: flex; flex-direction: column; gap: 6px; }
.at-preview-var { display: flex; flex-direction: column; gap: 2px; }
.at-preview-label { font-size: 0.72rem; color: var(--text-light, #64748b); }
.at-preview-label em { color: #ef4444; font-style: normal; margin-left: 2px; }
.at-preview-out { display: flex; flex-direction: column; gap: 6px; }
.at-preview-text {
  margin: 0; background: var(--fill-light, #f8fafc); border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 8px; padding: 10px; font-size: 0.78rem; line-height: 1.7;
  white-space: pre-wrap; max-height: 260px; overflow: auto;
}
.at-missing { font-size: 0.72rem; color: #ef4444; }
.at-hint { font-size: 0.74rem; color: var(--text-light, #94a3b8); }
.at-ver { border-bottom: 1px dashed var(--border-color, #eee); padding: 10px 0; }
.at-ver-head { display: flex; align-items: center; gap: 10px; font-size: 0.82rem; font-weight: 600; }
.at-ver-time { font-weight: 400; font-size: 0.72rem; color: var(--text-light, #94a3b8); }
.at-ver-text {
  margin: 6px 0 0; font-size: 0.76rem; line-height: 1.7; white-space: pre-wrap;
  background: var(--fill-light, #f8fafc); border-radius: 8px; padding: 10px; max-height: 220px; overflow: auto;
}
</style>
