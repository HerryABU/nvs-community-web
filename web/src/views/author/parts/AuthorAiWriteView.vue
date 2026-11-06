<template>
  <div class="ai-write">
    <!-- ===== 工程列表 / 新建 ===== -->
    <div v-if="!current" class="aw-list">
      <div class="aw-list-head">
        <h3>AI 写作工程</h3>
        <el-button type="primary" @click="openCreate">新建工程</el-button>
      </div>
      <div v-if="loading" class="aw-empty">加载中…</div>
      <div v-else-if="projects.length === 0" class="aw-empty">还没有写作工程，点「新建工程」开始。</div>
      <div v-else class="aw-cards">
        <div v-for="p in projects" :key="p.id" class="aw-card" @click="openProject(p.id)">
          <div class="aw-card-title">{{ p.title }}</div>
          <div class="aw-card-meta">
            <el-tag size="small">{{ styleLabel(p.style_preset) }}</el-tag>
            <span class="aw-card-time">{{ formatDate(p.updated_at) }}</span>
          </div>
          <div class="aw-card-actions">
            <el-button size="small" type="danger" plain @click.stop="removeProject(p.id)">删除</el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- ===== 工程编辑器 ===== -->
    <div v-else class="aw-editor">
      <div class="aw-editor-head">
        <el-button text @click="backToList">← 返回工程列表</el-button>
        <span class="aw-editor-title">{{ current.title }}</span>
        <el-button type="success" :loading="publishing" @click="publishNovel">生成小说</el-button>
      </div>

      <div class="aw-meta">
        <el-form :model="form" label-width="80px" inline>
          <el-form-item label="作品标题">
            <el-input v-model="form.title" placeholder="作品标题" style="width: 240px" />
          </el-form-item>
          <el-form-item label="风格库">
            <el-select v-model="form.style_preset" placeholder="选择风格" style="width: 180px">
              <el-option v-for="o in styleOptions" :key="o.value" :label="o.label" :value="o.value" />
            </el-select>
          </el-form-item>
        </el-form>
        <el-input
          v-model="form.style_detail"
          type="textarea"
          :rows="2"
          placeholder="更详细的风格要求（如：第三人称限知、冷峻白描、每章结尾留钩子…）"
        />
        <el-button size="small" :loading="saving" @click="saveAll">保存进度与设置</el-button>
        <el-button size="small" @click="visVisible = true">可视化（时间线/人物关系）</el-button>
      </div>

      <!-- 结构化物料：设定 / 人物 / 大纲 / 时间线 / 正文 -->
      <div class="aw-sections">
        <section
          v-for="sec in sections"
          :key="sec.key"
          class="aw-section"
        >
          <div class="aw-section-head">
            <h4>{{ sec.label }}</h4>
            <div class="aw-section-actions">
              <el-input
                v-model="sec.prompt"
                size="small"
                :placeholder="sec.promptPh"
                style="width: 200px"
              />
              <el-button
                size="small"
                type="primary"
                :loading="sec.loading"
                @click="generate(sec)"
              >AI 生成</el-button>
            </div>
          </div>
          <el-input
            v-model="form[sec.field]"
            type="textarea"
            :rows="sec.rows"
            :placeholder="sec.ph"
          />
        </section>
      </div>

      <div class="aw-footer">
        <el-button :loading="saving" @click="saveAll">保存进度</el-button>
        <el-button type="success" :loading="publishing" @click="publishNovel">生成小说（落地草稿）</el-button>
      </div>
    </div>

    <!-- 新建工程：先定义名字，再落入项目 -->
    <el-dialog v-model="createVisible" title="新建 AI 写作工程" width="480px" @closed="onCreateClosed">
      <el-form :model="createForm" label-width="80px">
        <el-form-item label="工程名称" required>
          <el-input
            ref="createTitleRef"
            v-model="createForm.title"
            placeholder="给写作工程起个名字，如《画中梦》"
            maxlength="80"
            show-word-limit
            @keyup.enter="confirmCreate"
          />
        </el-form-item>
        <el-form-item label="风格库">
          <el-select v-model="createForm.style_preset" placeholder="可选，创建后也能改" style="width: 100%">
            <el-option v-for="o in styleOptions" :key="o.value" :label="o.label" :value="o.value" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="confirmCreate">创建并进入</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="visVisible" title="AI 写作可视化" width="80%" top="5vh">
      <AiWriteVisuals :timeline="form.timeline" :characters="form.characters" />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { authorAiApi, STYLE_OPTIONS, type AiWriteProject, type StylePreset } from '@/api/authorai';
import AiWriteVisuals from '@/components/author/AiWriteVisuals.vue';

const loading = ref(false);
const creating = ref(false);
const saving = ref(false);
const publishing = ref(false);
const visVisible = ref(false);
const createVisible = ref(false);
const createTitleRef = ref();
const createForm = reactive({ title: '', style_preset: '' as StylePreset });
const projects = ref<AiWriteProject[]>([]);
const current = ref<AiWriteProject | null>(null);

const styleOptions = STYLE_OPTIONS;

const form = reactive({
  title: '',
  style_preset: '' as StylePreset,
  style_detail: '',
  setting: '',
  characters: '',
  timeline: '',
  outline: '',
  body_draft: '',
});

interface Section {
  key: string;
  field: 'setting' | 'characters' | 'timeline' | 'outline' | 'body_draft';
  label: string;
  ph: string;
  promptPh: string;
  rows: number;
  prompt: string;
  loading: boolean;
}

const sections = reactive<Section[]>([
  { key: 'setting', field: 'setting', label: '世界观 / 设定', ph: '在此填写或 AI 生成世界观设定', promptPh: '补充设定要求', rows: 6, prompt: '', loading: false },
  { key: 'characters', field: 'characters', label: '人物设定', ph: '在此填写或 AI 生成人物设定卡', promptPh: '补充人物要求', rows: 8, prompt: '', loading: false },
  { key: 'outline', field: 'outline', label: '大纲', ph: '在此填写或 AI 生成分卷/分章大纲', promptPh: '补充大纲要求', rows: 8, prompt: '', loading: false },
  { key: 'timeline', field: 'timeline', label: '大事年表 / 时间线（含嵌套循环）', ph: '在此填写或 AI 生成大事年表；嵌套循环用 [LOOP] 标注', promptPh: '补充时间线要求', rows: 6, prompt: '', loading: false },
  { key: 'body_draft', field: 'body_draft', label: '正文草稿', ph: '在此续写或 AI 生成章节正文', promptPh: '补充本章要求', rows: 12, prompt: '', loading: false },
]);

function formatDate(d?: string) {
  if (!d) return '';
  return new Date(d).toLocaleString('zh-CN');
}

function styleLabel(preset: string) {
  const f = STYLE_OPTIONS.find((o) => o.value === preset);
  return f ? f.label : '默认风格';
}

async function loadList() {
  loading.value = true;
  try {
    const res = await authorAiApi.list();
    if (res.data.code === 0) projects.value = res.data.data || [];
  } catch (e) {
    ElMessage.error('加载工程列表失败');
  } finally {
    loading.value = false;
  }
}

// 新建工程：先弹窗定义名字，再创建并落入项目
function openCreate() {
  createForm.title = '';
  createForm.style_preset = '';
  createVisible.value = true;
  // 自动聚焦名称输入框
  requestAnimationFrame(() => createTitleRef.value?.focus?.());
}

function onCreateClosed() {
  creating.value = false;
}

async function confirmCreate() {
  const title = createForm.title.trim();
  if (!title) {
    ElMessage.warning('请先填写工程名称');
    createTitleRef.value?.focus?.();
    return;
  }
  creating.value = true;
  try {
    const res = await authorAiApi.create({
      title,
      style_preset: (createForm.style_preset || undefined) as StylePreset | undefined,
    });
    if (res.data.code === 0) {
      ElMessage.success('工程已创建');
      createVisible.value = false;
      await loadList();
      openProject(res.data.data.id);
    }
  } catch (e) {
    ElMessage.error('新建工程失败');
  } finally {
    creating.value = false;
  }
}

async function openProject(id: number) {
  try {
    const res = await authorAiApi.get(id);
    if (res.data.code === 0) {
      const p = res.data.data;
      current.value = p;
      form.title = p.title;
      form.style_preset = (p.style_preset as StylePreset) || '';
      form.style_detail = p.style_detail;
      form.setting = p.setting;
      form.characters = p.characters;
      form.timeline = p.timeline;
      form.outline = p.outline;
      form.body_draft = p.body_draft;
    }
  } catch (e) {
    ElMessage.error('打开工程失败');
  }
}

function backToList() {
  current.value = null;
  loadList();
}

async function removeProject(id: number) {
  try {
    await ElMessageBox.confirm('确认删除该写作工程？', '提示', { type: 'warning' });
  } catch {
    return;
  }
  try {
    await authorAiApi.remove(id);
    ElMessage.success('已删除');
    await loadList();
  } catch (e) {
    ElMessage.error('删除失败');
  }
}

async function saveAll() {
  if (!current.value) return;
  saving.value = true;
  try {
    const res = await authorAiApi.update(current.value.id, { ...form });
    if (res.data.code === 0) {
      current.value = res.data.data;
      ElMessage.success('已保存');
    }
  } catch (e) {
    ElMessage.error('保存失败');
  } finally {
    saving.value = false;
  }
}

async function generate(sec: Section) {
  if (!current.value) return;
  sec.loading = true;
  try {
    const res = await authorAiApi.generate(current.value.id, {
      component: sec.key,
      prompt: sec.prompt,
    });
    if (res.data.code === 0) {
      const content = res.data.data.content;
      (form as any)[sec.field] = content;
      ElMessage.success(`${sec.label} 已生成并保存`);
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || 'AI 生成失败');
  } finally {
    sec.loading = false;
  }
}

async function publishNovel() {
  if (!current.value) return;
  try {
    await ElMessageBox.confirm('将把当前工程生成一部小说（草稿），之后可在普通编辑器继续创作。', '生成小说', { type: 'info' });
  } catch {
    return;
  }
  publishing.value = true;
  try {
    const res = await authorAiApi.publish(current.value.id);
    if (res.data.code === 0) {
      const nv = res.data.data;
      ElMessage.success('已生成小说（草稿）');
      if (nv && nv.id) {
        window.open(`/author/${nv.author_id ?? ''}/novel/${nv.id}`, '_blank');
      }
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成小说失败');
  } finally {
    publishing.value = false;
  }
}

loadList();
</script>

<style scoped>
.ai-write { padding: 4px 0; }
.aw-list-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.aw-list-head h3 { margin: 0; color: var(--text-color); }
.aw-empty { color: var(--text-light); padding: 24px 0; text-align: center; }
.aw-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 12px; }
.aw-card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 14px;
  cursor: pointer;
  transition: box-shadow 0.2s, transform 0.2s;
}
.aw-card:hover { box-shadow: 0 4px 14px rgba(15, 23, 42, 0.1); transform: translateY(-1px); }
.aw-card-title { font-weight: 600; color: var(--text-color); margin-bottom: 8px; }
.aw-card-meta { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.aw-card-time { font-size: 0.75rem; color: var(--text-light); }

.aw-editor-head { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; }
.aw-editor-title { font-size: 1.1rem; font-weight: 700; color: var(--text-color); flex: 1; }
.aw-meta { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 12px; padding: 14px; margin-bottom: 16px; display: flex; flex-direction: column; gap: 10px; }
.aw-sections { display: flex; flex-direction: column; gap: 14px; }
.aw-section { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 12px; padding: 14px; }
.aw-section-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; flex-wrap: wrap; gap: 8px; }
.aw-section-head h4 { margin: 0; color: var(--text-color); }
.aw-section-actions { display: flex; align-items: center; gap: 8px; }
.aw-footer { display: flex; gap: 10px; justify-content: flex-end; margin-top: 16px; }
</style>
