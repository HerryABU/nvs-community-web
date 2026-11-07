<template>
  <div class="aw-page">
    <!-- ===== 工程列表 / 新建 ===== -->
    <div v-if="!current" class="aw-list">
      <div class="aw-list-head">
        <div>
          <h2 class="aw-h2">
            <el-icon class="aw-h2__icon"><EditPen /></el-icon>
            AI 写作工程
          </h2>
          <p class="aw-sub">用 AI 辅助搭建世界观、人物、大纲与时间线，逐章生成正文，一键落地成小说。</p>
        </div>
        <el-button type="primary" :icon="Plus" @click="openCreate">新建工程</el-button>
      </div>

      <div v-if="loading" class="aw-state">
        <el-icon class="is-loading"><Loading /></el-icon>
        <span>加载中…</span>
      </div>

      <div v-else-if="projects.length === 0" class="aw-empty">
        <div class="aw-empty__art"><el-icon><EditPen /></el-icon></div>
        <h3>还没有写作工程</h3>
        <p>从一部作品的设定开始，让 AI 帮你铺开整个世界。</p>
        <el-button type="primary" :icon="Plus" @click="openCreate">新建第一个工程</el-button>
      </div>

      <div v-else class="aw-cards">
        <div v-for="p in projects" :key="p.id" class="aw-proj" @click="openProject(p.id)">
          <div class="aw-proj__cover">
            <el-icon><EditPen /></el-icon>
          </div>
          <div class="aw-proj__body">
            <div class="aw-proj__title">{{ p.title }}</div>
            <div class="aw-proj__meta">
              <el-tag size="small" effect="light" class="aw-proj__tag">{{ styleLabel(p.style_preset) }}</el-tag>
              <span class="aw-proj__time">{{ formatDate(p.updated_at) }}</span>
            </div>
            <div class="aw-proj__foot">
              <span class="aw-proj__open">打开 <el-icon><ArrowRight /></el-icon></span>
              <el-button
                size="small"
                type="danger"
                plain
                :icon="Delete"
                @click.stop="removeProject(p.id)"
              >删除</el-button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ===== 工程编辑器（标签式） ===== -->
    <div v-else class="aw-editor">
      <div class="aw-editor-head">
        <el-button text :icon="ArrowLeft" @click="backToList">返回</el-button>
        <div class="aw-editor-head__title">
          <span class="aw-editor-title">{{ current.title }}</span>
          <el-tag size="small" effect="light">{{ styleLabel(form.style_preset) }}</el-tag>
        </div>
        <div class="aw-pills">
          <span class="aw-pill" :class="{ 'is-done': materialsFilled >= 4 }">
            <el-icon><Collection /></el-icon>素材 {{ materialsFilled }}/4
          </span>
          <span class="aw-pill">
            <el-icon><Document /></el-icon>章节 {{ chapters.length }}
          </span>
        </div>
        <el-button type="success" :loading="publishing" :icon="Promotion" @click="publishNovel">生成小说</el-button>
      </div>

      <el-tabs v-model="activeTab" class="aw-tabs">
        <!-- ===== 素材层 ===== -->
        <el-tab-pane label="素材" name="materials">
          <div class="aw-meta glass-panel">
            <div class="aw-meta-row">
              <div class="aw-field">
                <label>作品标题</label>
                <el-input v-model="form.title" placeholder="作品标题" />
              </div>
              <div class="aw-field aw-field--style">
                <label>风格库</label>
                <el-select v-model="form.style_preset" placeholder="选择风格">
                  <el-option v-for="o in styleOptions" :key="o.value" :label="o.label" :value="o.value" />
                </el-select>
              </div>
            </div>
            <el-input
              v-model="form.style_detail"
              type="textarea"
              :rows="2"
              placeholder="更详细的风格要求（如：第三人称限知、冷峻白描、每章结尾留钩子…）"
            />
            <div class="aw-meta-actions">
              <el-button :icon="Select" size="small" :loading="saving" @click="saveAll">保存进度与设置</el-button>
              <el-button type="primary" :icon="MagicStick" size="small" :loading="genAllLoading" @click="generateAll">AI 一键全生成素材</el-button>
            </div>
          </div>

          <el-alert
            v-if="timelineWarn.length"
            class="aw-tl-warn"
            type="warning"
            :closable="false"
            show-icon
            title="时间线 [LOOP] 嵌套循环存在一致性问题："
          >
            <ul>
              <li v-for="(e, i) in timelineWarn" :key="i">{{ e }}</li>
            </ul>
          </el-alert>

          <div class="aw-mat-grid">
            <section
              v-for="(sec, i) in materialSections"
              :key="sec.key"
              class="aw-mat"
              :class="{ 'is-filled': (form[sec.field] || '').length > 0 }"
            >
              <div class="aw-mat__head">
                <span class="aw-ic" :class="'aw-ic--' + (i % 4)"><el-icon><component :is="sec.icon" /></el-icon></span>
                <div class="aw-mat__titles">
                  <h4>{{ sec.label }}</h4>
                  <span class="aw-mat__count">{{ charCount(form[sec.field]) }} 字</span>
                </div>
                <span class="aw-mat__status" :class="(form[sec.field] || '').length > 0 ? 'ok' : 'todo'">
                  <el-icon><component :is="(form[sec.field] || '').length > 0 ? Check : EditPen" /></el-icon>
                  {{ (form[sec.field] || '').length > 0 ? '已生成' : '待填写' }}
                </span>
              </div>
              <div class="aw-mat__actions">
                <el-input v-model="form[sec.inputField]" size="small" :placeholder="sec.promptPh" />
                <el-button size="small" type="primary" :icon="MagicStick" :loading="sec.loading" @click="generateMaterial(sec)">AI 生成</el-button>
              </div>
              <el-input
                v-model="form[sec.field]"
                type="textarea"
                :rows="sec.rows"
                :placeholder="sec.ph"
              />
            </section>
          </div>
        </el-tab-pane>

        <!-- ===== 章节（独立页面，逐章生成） ===== -->
        <el-tab-pane label="章节" name="chapters">
          <div class="aw-chapters-head">
            <el-button size="small" type="primary" :icon="Plus" @click="createChapter">新增章节</el-button>
            <span class="aw-chapters-hint">逐章生成章纲与正文，已生成的内容会自动落库。</span>
          </div>
          <div v-if="chaptersLoading" class="aw-state">
            <el-icon class="is-loading"><Loading /></el-icon>
            <span>加载章节中…</span>
          </div>
          <div v-else-if="chapters.length === 0" class="aw-empty aw-empty--sm">
            <div class="aw-empty__art"><el-icon><Document /></el-icon></div>
            <p>还没有章节，点「新增章节」开始逐章创作。</p>
          </div>
          <div v-else class="aw-chapter-list">
            <div
              v-for="ch in chapters"
              :key="ch.id"
              class="aw-chapter"
              :class="'aw-chapter--' + (ch.status === 'generated' ? 'done' : ch.status === 'outline' ? 'outline' : 'todo')"
            >
              <div class="aw-chapter__bar" />
              <div class="aw-chapter__main">
                <div class="aw-chapter-head">
                  <span class="aw-chapter-order">{{ ch.order }}</span>
                  <el-input v-model="ch.title" size="small" class="aw-chapter-title" placeholder="章节标题" />
                  <span class="aw-chapter-status">
                    <el-tag size="small" v-if="ch.status === 'generated'" type="success" effect="light">已生成正文</el-tag>
                    <el-tag size="small" v-else-if="ch.status === 'outline'" type="warning" effect="light">已有章纲</el-tag>
                    <el-tag size="small" v-else type="info" effect="light">未生成</el-tag>
                  </span>
                </div>
                <div class="aw-chapter-outline">
                  <el-input v-model="ch.outline" type="textarea" :rows="3" placeholder="本章章纲（AI 生成或手填）" />
                  <div class="aw-chapter-outline-actions">
                    <el-button size="small" :icon="MagicStick" :loading="chGen[ch.id]?.outline" @click="generateChapter(ch, 'outline')">AI 生成章纲</el-button>
                    <el-button size="small" type="primary" :icon="MagicStick" :loading="chGen[ch.id]?.body" @click="generateChapter(ch, 'body')">AI 生成正文</el-button>
                  </div>
                </div>
                <el-input v-model="ch.body" type="textarea" :rows="8" placeholder="本章正文（AI 生成或手填）" />
                <div class="aw-chapter-foot">
                  <span class="aw-count">{{ charCount(ch.body) }} 字</span>
                  <span class="aw-chapter-foot-actions">
                    <el-button size="small" :icon="Select" @click="saveChapter(ch)">保存本章</el-button>
                    <el-button size="small" type="danger" plain :icon="Delete" @click="removeChapter(ch)">删除</el-button>
                  </span>
                </div>
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
        <el-button type="primary" :loading="creating" :icon="Plus" @click="confirmCreate">创建并进入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  Plus, Delete, ArrowLeft, Promotion, MagicStick, Select, EditPen,
  Setting, User, List, Calendar, Collection, Document, ArrowRight, Check, Loading,
} from '@element-plus/icons-vue';
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

// 素材完成度（4 个主素材非空计数）
const materialsFilled = computed(() => {
  if (!current.value) return 0;
  return ['setting', 'characters', 'outline', 'timeline'].filter((k) => (form as any)[k]?.length > 0).length;
});

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
  icon: any;
}

const materialSections = reactive<MaterialSection[]>([
  { key: 'setting', field: 'setting', inputField: 'setting_input', label: '世界观 / 设定', ph: '在此填写或 AI 生成世界观设定', promptPh: '补充设定要求', rows: 6, loading: false, icon: Setting },
  { key: 'characters', field: 'characters', inputField: 'character_input', label: '人物设定', ph: '在此填写或 AI 生成人物设定卡', promptPh: '补充人物要求', rows: 8, loading: false, icon: User },
  { key: 'outline', field: 'outline', inputField: 'outline_input', label: '大纲', ph: '在此填写或 AI 生成分卷/分章大纲', promptPh: '补充大纲要求', rows: 8, loading: false, icon: List },
  { key: 'timeline', field: 'timeline', inputField: 'timeline_input', label: '大事年表 / 时间线（含嵌套循环）', ph: '在此填写或 AI 生成大事年表；嵌套循环用 [LOOP:loop_key] 开始、[/LOOP] 结束', promptPh: '补充时间线要求', rows: 6, loading: false, icon: Calendar },
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
.aw-page { max-width: 1180px; margin: 0 auto; padding: 28px 20px 48px; }

/* ===== 列表 ===== */
.aw-list-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 22px; }
.aw-h2 { display: flex; align-items: center; gap: 10px; margin: 0; font-size: 1.5rem; font-weight: 700; color: var(--text-color); }
.aw-h2__icon { color: var(--primary-color); font-size: 1.4rem; }
.aw-sub { margin: 6px 0 0; color: var(--text-light); font-size: 0.9rem; }

.aw-state { display: flex; align-items: center; justify-content: center; gap: 8px; padding: 40px 0; color: var(--text-light); }
.aw-state .is-loading { animation: aw-spin 1s linear infinite; }
@keyframes aw-spin { to { transform: rotate(360deg); } }

.aw-empty { text-align: center; padding: 56px 20px; border: 1px dashed var(--border-color); border-radius: var(--radius-lg); background: var(--bg-soft); }
.aw-empty--sm { padding: 32px 16px; }
.aw-empty__art { width: 64px; height: 64px; margin: 0 auto 14px; border-radius: 18px; display: flex; align-items: center; justify-content: center; font-size: 1.8rem; color: #fff; background: var(--gradient-primary); box-shadow: var(--shadow-md); }
.aw-empty h3 { margin: 0 0 6px; font-size: 1.1rem; color: var(--text-color); }
.aw-empty p { margin: 0 0 16px; color: var(--text-light); font-size: 0.9rem; }

.aw-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 18px; }
.aw-proj {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  overflow: hidden;
  cursor: pointer;
  box-shadow: var(--shadow-card);
  transition: transform var(--transition-base), box-shadow var(--transition-base);
  display: flex; flex-direction: column;
}
.aw-proj:hover { transform: translateY(-4px); box-shadow: var(--shadow-card-hover); }
.aw-proj__cover { height: 88px; background: var(--gradient-primary); display: flex; align-items: center; justify-content: center; font-size: 2rem; color: #fff; }
.aw-proj__body { padding: 14px 16px 16px; display: flex; flex-direction: column; gap: 10px; flex: 1; }
.aw-proj__title { font-weight: 700; font-size: 1.05rem; color: var(--text-color); }
.aw-proj__meta { display: flex; align-items: center; justify-content: space-between; }
.aw-proj__time { font-size: 0.75rem; color: var(--text-light); }
.aw-proj__foot { display: flex; align-items: center; justify-content: space-between; margin-top: auto; }
.aw-proj__open { display: inline-flex; align-items: center; gap: 4px; color: var(--primary-color); font-size: 0.85rem; font-weight: 600; }

/* ===== 编辑器头部 ===== */
.aw-editor-head { display: flex; align-items: center; gap: 14px; margin-bottom: 16px; flex-wrap: wrap; }
.aw-editor-head__title { display: flex; align-items: center; gap: 8px; flex: 1; min-width: 180px; }
.aw-editor-title { font-size: 1.15rem; font-weight: 700; color: var(--text-color); }
.aw-pills { display: flex; gap: 8px; }
.aw-pill { display: inline-flex; align-items: center; gap: 5px; padding: 5px 12px; border-radius: var(--radius-full); background: var(--bg-soft); color: var(--text-light); font-size: 0.8rem; border: 1px solid var(--border-color); }
.aw-pill.is-done { color: #fff; background: var(--gradient-success); border-color: transparent; }

/* ===== Tabs ===== */
.aw-tabs { margin-top: 4px; }
.aw-tabs :deep(.el-tabs__header) { margin-bottom: 18px; }

/* ===== 素材 meta 玻璃卡 ===== */
.aw-meta { padding: 18px; margin-bottom: 16px; display: flex; flex-direction: column; gap: 12px; }
.aw-meta-row { display: flex; gap: 16px; flex-wrap: wrap; }
.aw-field { display: flex; flex-direction: column; gap: 6px; flex: 1; min-width: 200px; }
.aw-field--style { max-width: 280px; }
.aw-field label { font-size: 0.8rem; color: var(--text-light); }
.aw-meta-actions { display: flex; gap: 8px; flex-wrap: wrap; }

.aw-tl-warn { margin-bottom: 14px; }
.aw-tl-warn ul { margin: 4px 0 0; padding-left: 18px; }

/* ===== 素材卡片栅格 ===== */
.aw-mat-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.aw-mat {
  background: var(--surface-color, var(--bg-card));
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: 16px;
  box-shadow: var(--shadow-card);
  display: flex; flex-direction: column; gap: 12px;
  transition: transform var(--transition-base), box-shadow var(--transition-base), border-color var(--transition-base);
}
.aw-mat:hover { box-shadow: var(--shadow-lg); transform: translateY(-2px); }
.aw-mat.is-filled { border-color: color-mix(in srgb, var(--primary-color) 35%, var(--border-color)); }
.aw-mat__head { display: flex; align-items: center; gap: 12px; }
.aw-ic { width: 40px; height: 40px; border-radius: 12px; display: flex; align-items: center; justify-content: center; color: #fff; font-size: 1.2rem; flex-shrink: 0; }
/* 图标渐变跟随主题主色/强调色；自定义 CSS 也可直接覆盖 --aw-ic-* 四个变量 */
.aw-ic--0 { background: linear-gradient(135deg, var(--primary-color), color-mix(in srgb, var(--primary-color) 55%, #fff)); }
.aw-ic--1 { background: linear-gradient(135deg, var(--accent-color), color-mix(in srgb, var(--accent-color) 55%, #fff)); }
.aw-ic--2 { background: linear-gradient(135deg, color-mix(in srgb, var(--primary-color) 60%, var(--accent-color)), var(--primary-color)); }
.aw-ic--3 { background: linear-gradient(135deg, var(--aw-return, #06b6d4), color-mix(in srgb, var(--aw-return, #06b6d4) 55%, #fff)); }
.aw-mat__titles { flex: 1; min-width: 0; }
.aw-mat__titles h4 { margin: 0 0 2px; font-size: 1rem; color: var(--text-color); }
.aw-mat__count { font-size: 0.75rem; color: var(--text-light); }
.aw-mat__status { display: inline-flex; align-items: center; gap: 4px; font-size: 0.72rem; padding: 3px 9px; border-radius: var(--radius-full); white-space: nowrap; }
.aw-mat__status.ok { color: var(--success-color); background: rgba(16, 185, 129, 0.12); }
.aw-mat__status.todo { color: var(--text-light); background: var(--bg-soft); }
.aw-mat__actions { display: flex; align-items: center; gap: 8px; }
.aw-mat__actions .el-input { flex: 1; }

/* ===== 章节 ===== */
.aw-chapters-head { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; }
.aw-chapters-hint { font-size: 0.82rem; color: var(--text-light); }
.aw-chapter-list { display: flex; flex-direction: column; gap: 14px; }
.aw-chapter { display: flex; background: var(--surface-color, var(--bg-card)); border: 1px solid var(--border-color); border-radius: var(--radius-lg); overflow: hidden; box-shadow: var(--shadow-card); }
.aw-chapter__bar { width: 5px; flex-shrink: 0; background: var(--text-muted); }
.aw-chapter--done .aw-chapter__bar { background: var(--gradient-success); }
.aw-chapter--outline .aw-chapter__bar { background: var(--gradient-amber); }
.aw-chapter--todo .aw-chapter__bar { background: var(--gradient-slate); }
.aw-chapter__main { flex: 1; padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; min-width: 0; }
.aw-chapter-head { display: flex; align-items: center; gap: 10px; }
.aw-chapter-order { width: 30px; height: 30px; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-weight: 700; color: #fff; background: var(--gradient-primary); flex-shrink: 0; font-size: 0.9rem; }
.aw-chapter-title { flex: 1; }
.aw-chapter-outline { display: flex; flex-direction: column; gap: 8px; }
.aw-chapter-outline-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.aw-chapter-foot { display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; }
.aw-chapter-foot-actions { display: flex; gap: 8px; }
.aw-count { font-size: 0.75rem; color: var(--text-light); }

@media (max-width: 768px) {
  .aw-page { padding: 18px 12px 32px; }
  .aw-mat-grid { grid-template-columns: 1fr; }
  .aw-list-head { flex-direction: column; }
}
</style>
