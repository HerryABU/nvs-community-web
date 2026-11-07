<template>
  <div class="awx" :class="{ 'is-immersive': immersive }">
    <!-- ===== 工程列表（进入工作台前） ===== -->
    <div v-if="!current" class="awx-list">
      <div class="awx-list__head">
        <div>
          <h2>AI 小说写作工作台</h2>
          <p>引子锚定 → 拆成独立细节 → 逐细节与 AI 微调 → 输出成稿。参考 Scriverse / NovelForge / 织叙的专业创作台。</p>
        </div>
        <el-button type="primary" :icon="Plus" @click="openCreate">新建工程</el-button>
      </div>
      <div v-if="loading" class="awx-list__state">加载中…</div>
      <div v-else-if="projects.length === 0" class="awx-list__empty">
        <div class="awx-list__art">✍️</div>
        <h3>还没有写作工程</h3>
        <p>从一个话题开始，让 AI 帮你铺开整个世界。</p>
        <el-button type="primary" :icon="Plus" @click="openCreate">新建第一个工程</el-button>
      </div>
      <div v-else class="awx-list__grid">
        <div v-for="p in projects" :key="p.id" class="awx-proj" @click="openProject(p.id)">
          <div class="awx-proj__cover"><span>✍️</span><i>{{ styleLabel(p.style_preset) }}</i></div>
          <div class="awx-proj__body">
            <div class="awx-proj__title">{{ p.title }}</div>
            <div class="awx-proj__seed">{{ p.seed_topic || '尚未设定引子' }}</div>
            <div class="awx-proj__foot">
              <span>{{ formatDate(p.updated_at) }}</span>
              <el-button size="small" type="danger" plain @click.stop="removeProject(p.id)">删除</el-button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ===== 三栏工作台 ===== -->
    <div v-else class="awx-work">
      <AiWriteSidebar
        :project="current"
        :view="view"
        :details="details"
        :chapter-count="chapters.length"
        :total-words="totalWords"
        :warn-count="timelineWarn.length"
        :immersive="immersive"
        :publishing="publishing"
        :style-label="styleLabel(current.style_preset)"
        @update:view="view = $event"
        @update:immersive="setImmersive"
        @back="backToList"
        @publish="publishNovel"
      />

      <AiWriteWorkspace
        :view="view"
        :project-seed="current.seed_topic || ''"
        :details="details"
        :chapters="chapters"
        :active-detail-id="activeDetail?.id ?? null"
        :active-chapter-id="activeChapterId"
        :material-text="materialText"
        :timeline-warn="timelineWarn"
        :total-words="totalWords"
        :seed-loading="seedLoading"
        :gen-loading="genLoading"
        :saving="saving"
        :chapter-loading="chGen"
        @update:view="view = $event"
        @from-seed="fromSeed"
        @save-seed="saveSeed"
        @add-detail="addDetail"
        @save-detail="saveDetail"
        @remove-detail="removeDetail"
        @refine-detail="selectDetail"
        @generate-module="generateModule"
        @add-chapter="createChapter"
        @select-chapter="activeChapterId = $event"
        @save-chapter="saveChapter"
        @remove-chapter="removeChapter"
        @generate-chapter="generateChapter"
        @recheck="checkTimeline"
      />

      <AiWriteAgentPanel
        :detail="activeDetail"
        :seed="current.seed_topic || ''"
        :style-label="styleLabel(current.style_preset)"
        :refine="callRefine"
        @apply="applyRefine"
      />
    </div>

    <!-- 新建工程 -->
    <el-dialog v-model="createVisible" title="新建 AI 写作工程" width="480px">
      <el-form :model="createForm" label-width="90px">
        <el-form-item label="工程名称" required>
          <el-input ref="createTitleRef" v-model="createForm.title" placeholder="如《画中梦》" maxlength="80" @keyup.enter="confirmCreate" />
        </el-form-item>
        <el-form-item label="引子 / 话题">
          <el-input v-model="createForm.seed_topic" type="textarea" :rows="3" placeholder="一句话灵感（可留空，进入工作台后再写）" />
        </el-form-item>
        <el-form-item label="风格库">
          <el-select v-model="createForm.style_preset" style="width: 100%">
            <el-option v-for="o in STYLE_OPTIONS" :key="o.value" :label="o.label" :value="o.value" />
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
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus } from '@element-plus/icons-vue';
import {
  authorAiApi, STYLE_OPTIONS,
  type AiWriteProject, type AiWriteChapter, type AiWriteDetail,
  type DetailModule, type RefineResult, type RefineMode, type RefineTurn, type StylePreset,
} from '@/api/authorai';
import AiWriteSidebar from '@/components/author/aiwrite/AiWriteSidebar.vue';
import AiWriteWorkspace from '@/components/author/aiwrite/AiWriteWorkspace.vue';
import AiWriteAgentPanel from '@/components/author/aiwrite/AiWriteAgentPanel.vue';

// ── 工程列表 ──
const loading = ref(false);
const creating = ref(false);
const createVisible = ref(false);
const createTitleRef = ref();
const createForm = reactive({ title: '', seed_topic: '', style_preset: '' as StylePreset });
const projects = ref<AiWriteProject[]>([]);
const current = ref<AiWriteProject | null>(null);

// ── 工作台状态 ──
const view = ref('overview');
const details = ref<AiWriteDetail[]>([]);
const chapters = ref<AiWriteChapter[]>([]);
const activeDetail = ref<AiWriteDetail | null>(null);
const activeChapterId = ref<number | null>(null);
const timelineWarn = ref<string[]>([]);
const immersive = ref(false);
const seedLoading = ref(false);
const genLoading = ref(false);
const saving = ref(false);
const publishing = ref(false);
const chGen = reactive<Record<number, { outline?: boolean; body?: boolean }>>({});

const totalWords = computed(() => chapters.value.reduce((n, c) => n + (c.body || '').length, 0));
const materialText = computed(() => ({
  timeline: current.value?.timeline || '',
  characters: current.value?.characters || '',
}));

function styleLabel(preset: string) {
  return STYLE_OPTIONS.find((o) => o.value === preset)?.label || '默认风格';
}
function formatDate(d?: string) { return d ? new Date(d).toLocaleString('zh-CN') : ''; }

async function loadList() {
  loading.value = true;
  try {
    const res = await authorAiApi.list();
    if (res.data.code === 0) projects.value = res.data.data || [];
  } catch { ElMessage.error('加载工程列表失败'); }
  finally { loading.value = false; }
}

function openCreate() {
  createForm.title = ''; createForm.seed_topic = ''; createForm.style_preset = '';
  createVisible.value = true;
  requestAnimationFrame(() => createTitleRef.value?.focus?.());
}

async function confirmCreate() {
  if (!createForm.title.trim()) { ElMessage.warning('请填写工程名称'); return; }
  creating.value = true;
  try {
    const res = await authorAiApi.create({
      title: createForm.title.trim(),
      seed_topic: createForm.seed_topic.trim() || undefined,
      style_preset: createForm.style_preset || undefined,
    });
    if (res.data.code === 0) {
      createVisible.value = false;
      await loadList();
      await openProject(res.data.data.id);
    }
  } catch { ElMessage.error('新建工程失败'); }
  finally { creating.value = false; }
}

async function openProject(id: number) {
  try {
    const res = await authorAiApi.get(id);
    if (res.data.code !== 0) return;
    current.value = res.data.data;
    view.value = 'overview';
    activeDetail.value = null;
    await Promise.all([loadDetails(), loadChapters(), checkTimeline()]);
  } catch { ElMessage.error('打开工程失败'); }
}

function backToList() {
  current.value = null;
  setImmersive(false);
  loadList();
}

async function removeProject(id: number) {
  try { await ElMessageBox.confirm('确认删除该写作工程？', '提示', { type: 'warning' }); } catch { return; }
  try { await authorAiApi.remove(id); ElMessage.success('已删除'); await loadList(); }
  catch { ElMessage.error('删除失败'); }
}

// ── 细节卡（真源） ──
async function loadDetails() {
  if (!current.value) return;
  try {
    const res = await authorAiApi.details(current.value.id);
    if (res.data.code === 0) details.value = res.data.data || [];
  } catch { /* 静默 */ }
}

async function refreshProject() {
  if (!current.value) return;
  try {
    const res = await authorAiApi.get(current.value.id);
    if (res.data.code === 0) current.value = res.data.data;
  } catch { /* 静默 */ }
}

async function saveSeed(seed: string) {
  if (!current.value) return;
  saving.value = true;
  try {
    const res = await authorAiApi.update(current.value.id, { seed_topic: seed });
    if (res.data.code === 0) { current.value = res.data.data; ElMessage.success('引子已保存'); }
  } catch { ElMessage.error('保存失败'); }
  finally { saving.value = false; }
}

async function fromSeed(seed: string, replace: boolean) {
  if (!current.value) return;
  if (!seed.trim()) { ElMessage.warning('请先写一句引子 / 话题'); return; }
  seedLoading.value = true;
  try {
    const res = await authorAiApi.fromSeed(current.value.id, { seed: seed.trim(), replace });
    if (res.data.code === 0) {
      current.value = res.data.data.project;
      await loadDetails();
      ElMessage.success(`已拆解为 ${res.data.data.cards.length} 张细节卡`);
      view.value = 'setting';
    }
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || 'AI 拆解失败'); }
  finally { seedLoading.value = false; }
}

async function addDetail(m: string) {
  if (!current.value) return;
  const mod = (m === 'timeline' || m === 'outline' || m === 'character' || m === 'setting') ? m : 'setting';
  try {
    const res = await authorAiApi.createDetail(current.value.id, { module: mod as DetailModule, title: '', content: '' });
    if (res.data.code === 0) { await loadDetails(); await refreshProject(); }
  } catch { ElMessage.error('新增失败'); }
}

async function saveDetail(d: AiWriteDetail) {
  if (!current.value) return;
  try {
    await authorAiApi.updateDetail(current.value.id, d.id, {
      module: d.module, title: d.title, content: d.content,
    });
    await refreshProject(); // 投影已重编译，刷新四个大字段供可视化/生成使用
  } catch { ElMessage.error('保存细节失败'); }
}

async function removeDetail(d: AiWriteDetail) {
  if (!current.value) return;
  try { await ElMessageBox.confirm(`删除细节「${d.title || '未命名'}」？`, '提示', { type: 'warning' }); } catch { return; }
  try {
    await authorAiApi.removeDetail(current.value.id, d.id);
    if (activeDetail.value?.id === d.id) activeDetail.value = null;
    await loadDetails(); await refreshProject();
  } catch { ElMessage.error('删除失败'); }
}

function selectDetail(d: AiWriteDetail) { activeDetail.value = d; }

async function generateModule(m: string) {
  if (!current.value) return;
  // 注意后端常量是 CompCharacter = "character"（单数）；旧前端误传 "characters" 导致写回分支不匹配
  const map: Record<string, 'setting' | 'character' | 'outline' | 'timeline'> = {
    setting: 'setting', character: 'character', characters: 'character', outline: 'outline', timeline: 'timeline',
  };
  const component = map[m] || 'setting';
  genLoading.value = true;
  try {
    const res = await authorAiApi.generate(current.value.id, { component });
    if (res.data.code === 0) {
      // 生成结果落进大字段（投影）后，把它拆成一张可独立微调的细节卡
      const content = res.data.data.content;
      const mod = (component === 'characters' ? 'character' : component) as DetailModule;
      await authorAiApi.createDetail(current.value.id, { module: mod, title: `AI 生成 · ${new Date().toLocaleTimeString('zh-CN')}`, content });
      await loadDetails(); await refreshProject();
      ElMessage.success('已生成为独立细节卡，可逐条微调');
    }
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || 'AI 生成失败'); }
  finally { genLoading.value = false; }
}

// ── 右侧 Agent 微调（ADRO Recur → Output） ──
async function callRefine(payload: { mode: RefineMode; instruction: string; history: RefineTurn[]; apply: boolean }): Promise<RefineResult | null> {
  if (!current.value || !activeDetail.value) return null;
  try {
    const res = await authorAiApi.refineDetail(current.value.id, activeDetail.value.id, payload);
    if (res.data.code === 0) {
      if (res.data.data.applied) { await loadDetails(); await refreshProject(); }
      return res.data.data;
    }
  } catch { /* 交由面板显示失败 */ }
  return null;
}

async function applyRefine(content: string) {
  if (!current.value || !activeDetail.value) return;
  const d = details.value.find((x) => x.id === activeDetail.value?.id);
  if (!d) return;
  d.content = content;
  d.status = 'refined';
  await saveDetail(d);
  await loadDetails();
  ElMessage.success('已应用为这条细节');
}

// ── 章节 ──
async function loadChapters() {
  if (!current.value) return;
  try {
    const res = await authorAiApi.chapters(current.value.id);
    if (res.data.code === 0) {
      chapters.value = res.data.data || [];
      for (const ch of chapters.value) chGen[ch.id] = {};
      if (!activeChapterId.value && chapters.value.length) activeChapterId.value = chapters.value[0].id;
    }
  } catch { ElMessage.error('加载章节失败'); }
}

async function createChapter() {
  if (!current.value) return;
  try {
    const res = await authorAiApi.createChapter(current.value.id, {});
    if (res.data.code === 0) {
      const ch = res.data.data; chGen[ch.id] = {};
      chapters.value.push(ch); activeChapterId.value = ch.id;
    }
  } catch { ElMessage.error('新增章节失败'); }
}

async function saveChapter(ch: AiWriteChapter) {
  if (!current.value) return;
  try {
    const res = await authorAiApi.updateChapter(current.value.id, ch.id, {
      title: ch.title, outline: ch.outline, body: ch.body, order: ch.order, status: ch.status,
    });
    if (res.data.code === 0) ElMessage.success('已保存本章');
  } catch { ElMessage.error('保存章节失败'); }
}

async function generateChapter(ch: AiWriteChapter, part: 'outline' | 'body') {
  if (!current.value) return;
  chGen[ch.id] = chGen[ch.id] || {}; chGen[ch.id][part] = true;
  try {
    const res = await authorAiApi.generateChapter(current.value.id, ch.id, { part });
    if (res.data.code === 0) {
      const u = res.data.data.chapter;
      ch.outline = u.outline; ch.body = u.body; ch.status = u.status;
      ElMessage.success(part === 'outline' ? '章纲已生成' : '正文已生成');
    }
  } catch { ElMessage.error('AI 生成失败'); }
  finally { chGen[ch.id][part] = false; }
}

async function removeChapter(ch: AiWriteChapter) {
  if (!current.value) return;
  try { await ElMessageBox.confirm(`删除「第 ${ch.order} 章 ${ch.title}」？`, '提示', { type: 'warning' }); } catch { return; }
  try {
    await authorAiApi.removeChapter(current.value.id, ch.id);
    chapters.value = chapters.value.filter((c) => c.id !== ch.id);
    if (activeChapterId.value === ch.id) activeChapterId.value = chapters.value[0]?.id ?? null;
  } catch { ElMessage.error('删除失败'); }
}

async function checkTimeline() {
  if (!current.value) return;
  try {
    const res = await authorAiApi.timelineCheck(current.value.id);
    if (res.data.code === 0) timelineWarn.value = res.data.data.errors || [];
  } catch { timelineWarn.value = []; }
}

async function publishNovel() {
  if (!current.value) return;
  try { await ElMessageBox.confirm('把当前工程生成一部小说（草稿），之后可在普通编辑器继续创作。', '生成小说', { type: 'info' }); } catch { return; }
  publishing.value = true;
  try {
    const res = await authorAiApi.publish(current.value.id);
    if (res.data.code === 0) {
      ElMessage.success('已生成小说（草稿）');
      const nv = res.data.data;
      if (nv?.id) window.open(`/author/${nv.author_id ?? ''}/novel/${nv.id}`, '_blank');
    }
  } catch { ElMessage.error('生成小说失败'); }
  finally { publishing.value = false; }
}

// ── 沉浸模式 / Ctrl+K ──
function setImmersive(v: boolean) {
  immersive.value = v;
  document.body.classList.toggle('ai-immersive', v);
}
function onKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault();
    view.value = 'overview';
  }
}
onMounted(() => { loadList(); window.addEventListener('keydown', onKeydown); });
onUnmounted(() => { window.removeEventListener('keydown', onKeydown); document.body.classList.remove('ai-immersive'); });
</script>

<style scoped>
.awx { max-width: 1680px; margin: 0 auto; padding: 20px; }
.awx-list__head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 20px; }
.awx-list__head h2 { margin: 0; font-size: 1.5rem; font-weight: 700; color: var(--text-color); }
.awx-list__head p { margin: 6px 0 0; color: var(--text-light); font-size: 0.88rem; max-width: 720px; line-height: 1.6; }
.awx-list__state, .awx-list__empty { padding: 48px 16px; text-align: center; color: var(--text-light); border: 1px dashed var(--border-color); border-radius: var(--radius-lg); background: var(--bg-soft); }
.awx-list__art { font-size: 2.6rem; margin-bottom: 10px; }
.awx-list__empty h3 { margin: 0 0 6px; color: var(--text-color); }
.awx-list__empty p { margin: 0 0 16px; font-size: 0.88rem; }
.awx-list__grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 14px; }
.awx-proj { border: 1px solid var(--border-color); border-radius: var(--radius-lg); overflow: hidden; background: var(--surface-color, var(--bg-card)); cursor: pointer; box-shadow: var(--shadow-card); transition: all 0.2s; }
.awx-proj:hover { transform: translateY(-3px); box-shadow: var(--shadow-card-hover); }
.awx-proj__cover { height: 78px; background: var(--gradient-primary); display: flex; align-items: center; justify-content: space-between; padding: 0 16px; color: #fff; }
.awx-proj__cover span { font-size: 1.8rem; }
.awx-proj__cover i { font-style: normal; font-size: 0.7rem; padding: 2px 8px; border-radius: 999px; background: rgba(255,255,255,0.22); }
.awx-proj__body { padding: 12px 14px 14px; }
.awx-proj__title { font-weight: 700; color: var(--text-color); }
.awx-proj__seed { font-size: 0.74rem; color: var(--text-light); margin-top: 4px; line-height: 1.5; height: 34px; overflow: hidden; }
.awx-proj__foot { display: flex; align-items: center; justify-content: space-between; margin-top: 10px; font-size: 0.7rem; color: var(--text-muted); }
.awx-work { display: flex; gap: 12px; align-items: flex-start; }
.awx.is-immersive { padding-top: 8px; }
@media (max-width: 1280px) {
  .awx-work { flex-wrap: wrap; }
  .awx-work > :last-child { width: 100%; height: auto; min-height: 420px; position: static; }
}
</style>
