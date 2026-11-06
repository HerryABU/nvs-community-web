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

    <!-- ===== 工程编辑器（标签式） ===== -->
    <div v-else class="aw-editor">
      <div class="aw-editor-head">
        <el-button text @click="backToList">← 返回工程列表</el-button>
        <span class="aw-editor-title">{{ current.title }}</span>
        <el-button type="success" :loading="publishing" @click="publishNovel">生成小说</el-button>
      </div>

      <el-tabs v-model="activeTab" class="aw-tabs">
        <!-- ===== 素材层 ===== -->
        <el-tab-pane label="素材" name="materials">
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
            <div class="aw-meta-actions">
              <el-button size="small" :loading="saving" @click="saveAll">保存进度与设置</el-button>
              <el-button size="small" type="primary" :loading="genAllLoading" @click="generateAll">AI 一键全生成素材</el-button>
            </div>
          </div>

          <el-alert
            v-if="timelineWarn.length"
            class="aw-tl-warn"
            type="warning"
            :closable="false"
            show-icon
            :title="'时间线 [LOOP] 嵌套循环存在一致性问题：'"
          >
            <ul>
              <li v-for="(e, i) in timelineWarn" :key="i">{{ e }}</li>
            </ul>
          </el-alert>

          <div class="aw-sections">
            <section v-for="sec in materialSections" :key="sec.key" class="aw-section">
              <div class="aw-section-head">
                <h4>{{ sec.label }}</h4>
                <div class="aw-section-actions">
                  <el-input v-model="form[sec.inputField]" size="small" :placeholder="sec.promptPh" style="width: 220px" />
                  <el-button size="small" type="primary" :loading="sec.loading" @click="generateMaterial(sec)">AI 生成</el-button>
                </div>
              </div>
              <el-input
                v-model="form[sec.field]"
                type="textarea"
                :rows="sec.rows"
                :placeholder="sec.ph"
              />
              <div class="aw-section-foot">
                <span class="aw-count">{{ charCount(form[sec.field]) }} 字</span>
              </div>
            </section>
          </div>
        </el-tab-pane>

        <!-- ===== 章节（独立页面，逐章生成） ===== -->
        <el-tab-pane label="章节" name="chapters">
          <div class="aw-chapters-head">
            <el-button size="small" type="primary" @click="createChapter">+ 新增章节</el-button>
          </div>
          <div v-if="chaptersLoading" class="aw-empty">加载章节中…</div>
          <div v-else-if="chapters.length === 0" class="aw-empty">还没有章节，点「新增章节」开始逐章创作。</div>
          <div v-else class="aw-chapter-list">
            <div v-for="ch in chapters" :key="ch.id" class="aw-chapter">
              <div class="aw-chapter-head">
                <span class="aw-chapter-order">第 {{ ch.order }} 章</span>
                <el-input v-model="ch.title" size="small" class="aw-chapter-title" placeholder="章节标题" />
                <span class="aw-chapter-status">
                  <el-tag size="small" v-if="ch.status === 'generated'" type="success">已生成正文</el-tag>
                  <el-tag size="small" v-else-if="ch.status === 'outline'" type="warning">已有章纲</el-tag>
                  <el-tag size="small" v-else type="info">未生成</el-tag>
                </span>
              </div>
              <el-input v-model="ch.outline" type="textarea" :rows="3" placeholder="本章章纲（AI 生成或手填）" class="aw-chapter-outline" />
              <div class="aw-chapter-outline-actions">
                <el-button size="small" :loading="chGen[ch.id]?.outline" @click="generateChapter(ch, 'outline')">AI 生成章纲</el-button>
                <el-button size="small" type="primary" :loading="chGen[ch.id]?.body" @click="generateChapter(ch, 'body')">AI 生成正文</el-button>
              </div>
              <el-input v-model="ch.body" type="textarea" :rows="8" placeholder="本章正文（AI 生成或手填）" />
              <div class="aw-chapter-foot">
                <span class="aw-count">{{ charCount(ch.body) }} 字</span>
                <span class="aw-chapter-foot-actions">
                  <el-button size="small" @click="saveChapter(ch)">保存本章</el-button>
                  <el-button size="small" type="danger" plain @click="removeChapter(ch)">删除</el-button>
                </span>
              </div>
            </div>
          </div>
        </el-tab-pane>

        <!-- ===== 可视化（内嵌，非弹窗） ===== -->
        <el-tab-pane label="可视化" name="visual">
          <AiWriteVisuals :timeline="form.timeline" :characters="form.characters" />
        </el-tab-pane>
      </el-tabs>
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { authorAiApi, STYLE_OPTIONS, type AiWriteProject, type AiWriteChapter, type StylePreset } from '@/api/authorai';
import AiWriteVisuals from '@/components/author/AiWriteVisuals.vue';

const loading = ref(false);
const creating = ref(false);
const saving = ref(false);
const publishing = ref(false);
const genAllLoading = ref(false);
const chaptersLoading = ref(false);
const createVisible = ref(false);
const createTitleRef = ref();
const createForm = reactive({ title: '', style_preset: '' as StylePreset });
const projects = ref<AiWriteProject[]>([]);
const current = ref<AiWriteProject | null>(null);
const activeTab = ref('materials');
const chapters = ref<AiWriteChapter[]>([]);
const timelineWarn = ref<string[]>([]);
const chGen = reactive<Record<number, { outline: boolean; body: boolean }>>({});

const styleOptions = STYLE_OPTIONS;

const form = reactive({
  title: '',
  style_preset: '' as StylePreset,
  style_detail: '',
  setting: '',
  characters: '',
  timeline: '',
  outline: '',
  setting_input: '',
  character_input: '',
  outline_input: '',
  timeline_input: '',
});

interface MaterialSection {
  key: 'setting' | 'characters' | 'outline' | 'timeline';
  field: 'setting' | 'characters' | 'outline' | 'timeline';
  inputField: 'setting_input' | 'character_input' | 'outline_input' | 'timeline_input';
  label: string;
  ph: string;
  promptPh: string;
  rows: number;
  loading: boolean;
}

const materialSections = reactive<MaterialSection[]>([
  { key: 'setting', field: 'setting', inputField: 'setting_input', label: '世界观 / 设定', ph: '在此填写或 AI 生成世界观设定', promptPh: '补充设定要求', rows: 6, loading: false },
  { key: 'characters', field: 'characters', inputField: 'character_input', label: '人物设定', ph: '在此填写或 AI 生成人物设定卡', promptPh: '补充人物要求', rows: 8, loading: false },
  { key: 'outline', field: 'outline', inputField: 'outline_input', label: '大纲', ph: '在此填写或 AI 生成分卷/分章大纲', promptPh: '补充大纲要求', rows: 8, loading: false },
  { key: 'timeline', field: 'timeline', inputField: 'timeline_input', label: '大事年表 / 时间线（含嵌套循环）', ph: '在此填写或 AI 生成大事年表；嵌套循环用 [LOOP:loop_key] 开始、[/LOOP] 结束', promptPh: '补充时间线要求', rows: 6, loading: false },
]);

function charCount(s?: string) {
  return (s || '').length;
}

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

function openCreate() {
  createForm.title = '';
  createForm.style_preset = '';
  createVisible.value = true;
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
      form.style_detail = p.style_detail || '';
      form.setting = p.setting || '';
      form.characters = p.characters || '';
      form.timeline = p.timeline || '';
      form.outline = p.outline || '';
      form.setting_input = p.setting_input || '';
      form.character_input = p.character_input || '';
      form.outline_input = p.outline_input || '';
      form.timeline_input = p.timeline_input || '';
      activeTab.value = 'materials';
      await checkTimeline();
      await loadChapters();
    }
  } catch (e) {
    ElMessage.error('打开工程失败');
  }
}

async function checkTimeline() {
  if (!current.value) return;
  try {
    const res = await authorAiApi.timelineCheck(current.value.id);
    if (res.data.code === 0) {
      timelineWarn.value = res.data.data.errors || [];
    }
  } catch (e) {
    timelineWarn.value = [];
  }
}

async function loadChapters() {
  if (!current.value) return;
  chaptersLoading.value = true;
  try {
    const res = await authorAiApi.chapters(current.value.id);
    if (res.data.code === 0) {
      chapters.value = res.data.data || [];
      for (const ch of chapters.value) {
        chGen[ch.id] = { outline: false, body: false };
      }
    }
  } catch (e) {
    ElMessage.error('加载章节失败');
  } finally {
    chaptersLoading.value = false;
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

async function generateMaterial(sec: MaterialSection) {
  if (!current.value) return;
  sec.loading = true;
  try {
    const res = await authorAiApi.generate(current.value.id, {
      component: sec.key,
      prompt: (form[sec.inputField] || '').trim() || undefined,
    });
    if (res.data.code === 0) {
      const content = res.data.data.content;
      (form as any)[sec.field] = content;
      ElMessage.success(`${sec.label} 已生成并保存`);
      await checkTimeline();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || 'AI 生成失败');
  } finally {
    sec.loading = false;
  }
}

async function generateAll() {
  if (!current.value) return;
  // 先持久化补充要求，使「一键全生成」各模块采用各自已填的附加要求
  await saveAll();
  genAllLoading.value = true;
  try {
    const res = await authorAiApi.generateAll(current.value.id, {});
    if (res.data.code === 0) {
      const p = res.data.data.project;
      form.setting = p.setting;
      form.characters = p.characters;
      form.outline = p.outline;
      form.timeline = p.timeline;
      ElMessage.success('素材已全部生成并保存');
      await checkTimeline();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '一键全生成失败');
  } finally {
    genAllLoading.value = false;
  }
}

// ---------- 章节 ----------
async function createChapter() {
  if (!current.value) return;
  try {
    const res = await authorAiApi.createChapter(current.value.id, {});
    if (res.data.code === 0) {
      const ch = res.data.data;
      chGen[ch.id] = { outline: false, body: false };
      chapters.value.push(ch);
      ElMessage.success('已新增章节');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '新增章节失败');
  }
}

async function saveChapter(ch: AiWriteChapter) {
  if (!current.value) return;
  try {
    const res = await authorAiApi.updateChapter(current.value.id, ch.id, {
      title: ch.title,
      outline: ch.outline,
      body: ch.body,
      order: ch.order,
      status: ch.status,
    });
    if (res.data.code === 0) {
      ElMessage.success('已保存本章');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存章节失败');
  }
}

async function generateChapter(ch: AiWriteChapter, part: 'outline' | 'body') {
  if (!current.value) return;
  chGen[ch.id] = chGen[ch.id] || { outline: false, body: false };
  chGen[ch.id][part] = true;
  try {
    const res = await authorAiApi.generateChapter(current.value.id, ch.id, { part });
    if (res.data.code === 0) {
      const updated = res.data.data.chapter;
      ch.outline = updated.outline;
      ch.body = updated.body;
      ch.status = updated.status;
      ElMessage.success(part === 'outline' ? '章纲已生成' : '正文已生成');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || 'AI 生成失败');
  } finally {
    chGen[ch.id][part] = false;
  }
}

async function removeChapter(ch: AiWriteChapter) {
  if (!current.value) return;
  try {
    await ElMessageBox.confirm(`确认删除「第${ch.order}章 ${ch.title}」？`, '提示', { type: 'warning' });
  } catch {
    return;
  }
  try {
    await authorAiApi.removeChapter(current.value.id, ch.id);
    chapters.value = chapters.value.filter((c) => c.id !== ch.id);
    delete chGen[ch.id];
    ElMessage.success('已删除章节');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
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
.aw-tabs { margin-top: 4px; }

.aw-meta { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 12px; padding: 14px; margin-bottom: 16px; display: flex; flex-direction: column; gap: 10px; }
.aw-meta-actions { display: flex; gap: 8px; }

.aw-tl-warn { margin-bottom: 14px; }
.aw-tl-warn ul { margin: 4px 0 0; padding-left: 18px; }

.aw-sections { display: flex; flex-direction: column; gap: 14px; }
.aw-section { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 12px; padding: 14px; }
.aw-section-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; flex-wrap: wrap; gap: 8px; }
.aw-section-head h4 { margin: 0; color: var(--text-color); }
.aw-section-actions { display: flex; align-items: center; gap: 8px; }
.aw-section-foot { margin-top: 6px; }
.aw-count { font-size: 0.75rem; color: var(--text-light); }

.aw-chapters-head { display: flex; gap: 8px; margin-bottom: 12px; }
.aw-chapter-list { display: flex; flex-direction: column; gap: 14px; }
.aw-chapter { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 12px; padding: 14px; }
.aw-chapter-head { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.aw-chapter-order { font-weight: 600; color: var(--text-color); white-space: nowrap; }
.aw-chapter-title { flex: 1; }
.aw-chapter-outline { margin-bottom: 8px; }
.aw-chapter-outline-actions { display: flex; gap: 8px; margin-bottom: 8px; }
.aw-chapter-foot { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-top: 6px; flex-wrap: wrap; }
.aw-chapter-foot-actions { display: flex; gap: 8px; }
</style>
