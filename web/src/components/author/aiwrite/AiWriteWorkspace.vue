<template>
  <main class="aww">
    <!-- ===== 概览 / 引子启动（Anchor → Deconstruct） ===== -->
    <section v-if="view === 'overview'" class="aww-view">
      <div class="aww-hero">
        <div class="aww-hero__tag">ADRO · Anchor 锚定</div>
        <h2>从一个话题 / 引子开始</h2>
        <p>写一句话灵感，AI 自动把它拆成世界观、人物、大纲与时间线的<b>独立细节卡</b>；不满意的每一条都能单独改、单独让 AI 微调。</p>
        <el-input v-model="seedDraft" type="textarea" :rows="3" placeholder="例如：1980 年，顾家三代人卷入 ACG 行业的兴衰，而每次轮回都从一幅画开始…" />
        <div class="aww-hero__actions">
          <el-button type="primary" :icon="MagicStick" :loading="seedLoading" @click="emit('from-seed', seedDraft, seedReplace)">
            AI 自动拆解为细节卡
          </el-button>
          <el-button :icon="Select" :loading="saving" @click="$emit('save-seed', seedDraft)">仅保存引子</el-button>
          <label class="aww-hero__replace">
            <input type="checkbox" v-model="seedReplace" /> 清空旧卡后重建
          </label>
        </div>
        <div class="aww-hero__pipe">
          <span v-for="(s, i) in ['⚓ 引子锚定', '🧩 拆解成细节', '🔁 逐条微调', '📤 输出成稿']" :key="i">{{ s }}</span>
        </div>
      </div>

      <div class="aww-section-title">素材模块 · 点击卡片进入独立编辑</div>
      <div class="aww-modgrid">
        <button v-for="m in DETAIL_MODULES" :key="m.value" class="aww-modcard" @click="$emit('update:view', m.value)">
          <div class="aww-modcard__icon">{{ m.icon }}</div>
          <div class="aww-modcard__body">
            <div class="aww-modcard__title">{{ m.label }}</div>
            <div class="aww-modcard__hint">{{ m.hint }}</div>
          </div>
          <div class="aww-modcard__count">{{ countOf(m.value) }}<span>张卡</span></div>
        </button>
      </div>

      <div class="aww-section-title">最近细节卡</div>
      <div v-if="details.length === 0" class="aww-empty">
        <div class="aww-empty__art">🧩</div>
        <p>还没有细节卡。写一个引子让 AI 拆解，或进入任一模块手动新增。</p>
      </div>
      <div v-else class="aww-recent">
        <div v-for="d in details.slice(0, 6)" :key="d.id" class="aww-recent__item" @click="openDetail(d)">
          <span class="aww-recent__mod">{{ moduleIcon(d.module) }}</span>
          <b>{{ d.title || '未命名细节' }}</b>
          <span class="aww-recent__excerpt">{{ d.content.slice(0, 46) }}…</span>
        </div>
      </div>
    </section>

    <!-- ===== 卡片式细节（NovelForge 式独立实体） ===== -->
    <section v-else-if="isDetailView" class="aww-view">
      <div class="aww-bar">
        <div>
          <div class="aww-bar__title">{{ currentModule?.icon }} {{ currentModule?.label }}</div>
          <div class="aww-bar__sub">{{ currentModule?.hint }} · 每张卡独立保存 / 独立微调</div>
        </div>
        <div class="aww-bar__actions">
          <el-button size="small" :icon="Plus" @click="$emit('add-detail', view)">新增细节卡</el-button>
          <el-button size="small" type="primary" plain :icon="MagicStick" :loading="genLoading" @click="$emit('generate-module', view)">AI 补充本模块</el-button>
        </div>
      </div>

      <div v-if="moduleDetails.length === 0" class="aww-empty">
        <div class="aww-empty__art">{{ currentModule?.icon }}</div>
        <p>本模块还没有细节卡。点「AI 补充本模块」或「新增细节卡」开始。</p>
      </div>
      <div v-else class="aww-cards">
        <article
          v-for="(d, i) in moduleDetails"
          :key="d.id"
          class="aww-card"
          :class="{ 'is-active': activeDetailId === d.id, 'is-refined': d.status === 'refined' }"
        >
          <header class="aww-card__head">
            <span class="aww-card__idx">{{ String(i + 1).padStart(2, '0') }}</span>
            <input v-model="d.title" class="aww-card__title" placeholder="细节标题" @blur="$emit('save-detail', d)" />
            <span class="aww-card__status" :class="d.status">{{ statusLabel(d.status) }}</span>
          </header>
          <textarea v-model="d.content" class="aww-card__body" :rows="8" placeholder="这条细节的完整内容…" @blur="$emit('save-detail', d)" />
          <footer class="aww-card__foot">
            <span class="aww-card__count">{{ d.content.length }} 字</span>
            <div class="aww-card__acts">
              <button class="aww-card__refine" @click="$emit('refine-detail', d)">💬 与 AI 微调</button>
              <button @click="$emit('save-detail', d)">保存</button>
              <button class="is-danger" @click="$emit('remove-detail', d)">删除</button>
            </div>
          </footer>
        </article>
      </div>
    </section>

    <!-- ===== 多轨道时间线看板（对标 Scriverse） ===== -->
    <section v-else-if="view === 'timeline'" class="aww-view">
      <div class="aww-bar">
        <div>
          <div class="aww-bar__title">🕰️ 时间线看板</div>
          <div class="aww-bar__sub">按 [LOOP] 层级分轨道；锚点 [A] 与 ret / returnA 回溯会画成回环箭线</div>
        </div>
        <div class="aww-bar__actions">
          <el-button size="small" :icon="Plus" @click="$emit('add-detail', 'timeline')">新增事件卡</el-button>
          <el-button size="small" type="primary" plain :icon="MagicStick" :loading="genLoading" @click="$emit('generate-module', 'timeline')">AI 生成时间线</el-button>
        </div>
      </div>
      <div class="aww-board">
        <div v-for="(track, ti) in timelineTracks" :key="ti" class="aww-track">
          <div class="aww-track__label">
            <span class="aww-track__dot" :style="{ background: trackColor(ti) }" />
            轨道 {{ ti + 1 }}
            <small>{{ track.length }} 事件</small>
          </div>
          <div class="aww-track__items">
            <div
              v-for="d in track"
              :key="d.id"
              class="aww-track__item"
              :class="{ 'is-active': activeDetailId === d.id }"
              @click="$emit('refine-detail', d)"
            >
              <div class="aww-track__item-title">{{ d.title || '事件' }}</div>
              <div class="aww-track__item-body">{{ d.content.slice(0, 64) }}{{ d.content.length > 64 ? '…' : '' }}</div>
            </div>
            <div v-if="track.length === 0" class="aww-track__placeholder">空轨道</div>
          </div>
        </div>
      </div>
      <div class="aww-timeline-cards">
        <article v-for="d in timelineDetails" :key="d.id" class="aww-card is-compact" :class="{ 'is-active': activeDetailId === d.id }">
          <header class="aww-card__head">
            <span class="aww-card__idx">⏱</span>
            <input v-model="d.title" class="aww-card__title" placeholder="事件 / 时间" @blur="$emit('save-detail', d)" />
          </header>
          <textarea v-model="d.content" class="aww-card__body" :rows="4" placeholder="时间线条目，可含 [LOOP:id] / [A] / ret" @blur="$emit('save-detail', d)" />
          <footer class="aww-card__foot">
            <span class="aww-card__count">{{ d.content.length }} 字</span>
            <div class="aww-card__acts">
              <button class="aww-card__refine" @click="$emit('refine-detail', d)">💬 微调</button>
              <button @click="$emit('save-detail', d)">保存</button>
              <button class="is-danger" @click="$emit('remove-detail', d)">删除</button>
            </div>
          </footer>
        </article>
      </div>
    </section>

    <!-- ===== 章节工作台（列表 + 编辑器） ===== -->
    <section v-else-if="view === 'chapters'" class="aww-view">
      <div class="aww-bar">
        <div>
          <div class="aww-bar__title">✍️ 正文 / 章节</div>
          <div class="aww-bar__sub">逐章生成章纲与正文；正文落在每一章内</div>
        </div>
        <el-button size="small" type="primary" :icon="Plus" @click="$emit('add-chapter')">新增章节</el-button>
      </div>
      <div class="aww-chapters">
        <div class="aww-chapters__list">
          <div v-if="chapters.length === 0" class="aww-empty is-sm"><p>还没有章节，点「新增章节」开始。</p></div>
          <button
            v-for="ch in chapters"
            :key="ch.id"
            class="aww-chapters__item"
            :class="{ 'is-active': activeChapterId === ch.id }"
            @click="$emit('select-chapter', ch.id)"
          >
            <span class="aww-chapters__num">{{ ch.order }}</span>
            <span class="aww-chapters__name">{{ ch.title || `第 ${ch.order} 章` }}</span>
            <span class="aww-chapters__badge" :class="ch.status">{{ ch.status === 'generated' ? '已成文' : ch.status === 'outline' ? '有纲' : '草稿' }}</span>
          </button>
        </div>
        <div v-if="activeChapter" class="aww-chapters__editor">
          <div class="aww-chapters__head">
            <span>第 {{ activeChapter.order }} 章</span>
            <el-input v-model="activeChapter.title" size="small" placeholder="章节标题" />
          </div>
          <label>章纲</label>
          <textarea v-model="activeChapter.outline" class="aww-editor-area" rows="4" placeholder="本章章纲（AI 生成或手填）" />
          <div class="aww-chapters__gen">
            <el-button size="small" :icon="MagicStick" :loading="!!chapterLoading[activeChapter.id]?.outline" @click="$emit('generate-chapter', activeChapter, 'outline')">AI 生成章纲</el-button>
            <el-button size="small" type="primary" :icon="MagicStick" :loading="!!chapterLoading[activeChapter.id]?.body" @click="$emit('generate-chapter', activeChapter, 'body')">AI 生成正文</el-button>
          </div>
          <label>正文 <span class="aww-chapters__words">{{ activeChapter.body.length }} 字</span></label>
          <textarea v-model="activeChapter.body" class="aww-editor-area is-body" rows="14" placeholder="本章正文（AI 生成或手填）" />
          <div class="aww-chapters__foot">
            <el-button size="small" :icon="Select" @click="$emit('save-chapter', activeChapter)">保存本章</el-button>
            <el-button size="small" type="danger" plain @click="$emit('remove-chapter', activeChapter)">删除章节</el-button>
          </div>
        </div>
        <div v-else class="aww-chapters__editor aww-empty is-sm"><p>从左侧选一章开始写作</p></div>
      </div>
    </section>

    <!-- ===== 可视化 ===== -->
    <section v-else-if="view === 'visual'" class="aww-view">
      <div class="aww-bar"><div><div class="aww-bar__title">🕸️ 可视化图谱</div><div class="aww-bar__sub">时间线点线法（含嵌套循环与回环）+ 人物关系图</div></div></div>
      <AiWriteVisuals :timeline="materialText.timeline" :characters="materialText.characters" />
    </section>

    <!-- ===== 一致性审阅 ===== -->
    <section v-else-if="view === 'review'" class="aww-view">
      <div class="aww-bar"><div><div class="aww-bar__title">🔍 一致性审阅</div><div class="aww-bar__sub">纯 Go 校验 [LOOP] 嵌套与回环；不依赖模型判断，避免"AI 自己判自己对错"</div></div>
        <el-button size="small" @click="$emit('recheck')">重新校验</el-button>
      </div>
      <el-alert v-if="timelineWarn.length" type="warning" :closable="false" show-icon title="时间线存在一致性问题">
        <ul><li v-for="(e, i) in timelineWarn" :key="i">{{ e }}</li></ul>
      </el-alert>
      <el-alert v-else type="success" :closable="false" show-icon title="未发现时间线结构问题" />
      <div class="aww-review-grid">
        <div class="aww-review-card"><b>{{ details.length }}</b><span>细节卡</span></div>
        <div class="aww-review-card"><b>{{ chapters.length }}</b><span>章节</span></div>
        <div class="aww-review-card"><b>{{ totalWords }}</b><span>正文字数</span></div>
        <div class="aww-review-card"><b>{{ timelineWarn.length }}</b><span>待修问题</span></div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { Plus, MagicStick, Select } from '@element-plus/icons-vue';
import AiWriteVisuals from '@/components/author/AiWriteVisuals.vue';
import { DETAIL_MODULES, type AiWriteDetail, type AiWriteChapter, type DetailModule } from '@/api/authorai';

const props = defineProps<{
  view: string;
  projectSeed: string;
  details: AiWriteDetail[];
  chapters: AiWriteChapter[];
  activeDetailId: number | null;
  activeChapterId: number | null;
  materialText: { timeline: string; characters: string };
  timelineWarn: string[];
  totalWords: number;
  seedLoading: boolean;
  genLoading: boolean;
  saving: boolean;
  chapterLoading: Record<number, { outline?: boolean; body?: boolean }>;
}>();
const emit = defineEmits<{
  (e: 'update:view', v: string): void;
  (e: 'from-seed', seed: string, replace: boolean): void;
  (e: 'save-seed', seed: string): void;
  (e: 'add-detail', m: string): void;
  (e: 'save-detail', d: AiWriteDetail): void;
  (e: 'remove-detail', d: AiWriteDetail): void;
  (e: 'refine-detail', d: AiWriteDetail): void;
  (e: 'generate-module', m: string): void;
  (e: 'add-chapter'): void;
  (e: 'select-chapter', id: number): void;
  (e: 'save-chapter', ch: AiWriteChapter): void;
  (e: 'remove-chapter', ch: AiWriteChapter): void;
  (e: 'generate-chapter', ch: AiWriteChapter, part: 'outline' | 'body'): void;
  (e: 'recheck'): void;
}>();

const seedDraft = ref(props.projectSeed);
const seedReplace = ref(false);
watch(() => props.projectSeed, (v) => { if (!seedDraft.value) seedDraft.value = v; });

const isDetailView = computed(() => ['setting', 'character', 'outline'].includes(props.view));
const currentModule = computed(() => DETAIL_MODULES.find((m) => m.value === props.view));
const moduleDetails = computed(() => props.details.filter((d) => d.module === props.view));
const timelineDetails = computed(() => props.details.filter((d) => d.module === 'timeline'));
const activeChapter = computed(() => props.chapters.find((c) => c.id === props.activeChapterId) || null);

// 看板轨道：按内容里的 [LOOP] 层数粗分（0=主轨，1+=嵌套轨），只是展示分组，不做文法裁判
const timelineTracks = computed(() => {
  const tracks: AiWriteDetail[][] = [[], [], []];
  for (const d of timelineDetails.value) {
    const opens = (d.content.match(/\[LOOP(?::[^\]]+)?\]/gi) || []).length;
    const level = Math.min(2, opens);
    tracks[level].push(d);
  }
  return tracks;
});
const trackColor = (i: number) => ['var(--primary-color)', 'var(--accent-color)', 'var(--aw-return, #06b6d4)'][i] || 'var(--text-muted)';

function countOf(m: DetailModule) { return props.details.filter((d) => d.module === m).length; }
function moduleIcon(m: string) { return DETAIL_MODULES.find((x) => x.value === m)?.icon || '·'; }
function statusLabel(s: string) { return s === 'refined' ? '已微调' : s === 'generated' ? 'AI 生成' : '手填'; }
function openDetail(d: AiWriteDetail) {
  emit('update:view', d.module === 'timeline' ? 'timeline' : d.module);
  emit('refine-detail', d);
}

</script>

<style scoped>
.aww { flex: 1; min-width: 0; }
.aww-view { display: flex; flex-direction: column; gap: 14px; }
.aww-hero { padding: 20px; border-radius: var(--radius-lg); background: linear-gradient(135deg, color-mix(in srgb, var(--primary-color) 12%, var(--bg-card)), var(--bg-card)); border: 1px solid color-mix(in srgb, var(--primary-color) 25%, var(--border-color)); }
.aww-hero__tag { display: inline-block; padding: 3px 10px; border-radius: 999px; background: var(--gradient-primary); color: #fff; font-size: 0.68rem; font-weight: 700; margin-bottom: 10px; }
.aww-hero h2 { margin: 0 0 6px; font-size: 1.35rem; color: var(--text-color); }
.aww-hero p { margin: 0 0 14px; font-size: 0.86rem; color: var(--text-light); line-height: 1.7; }
.aww-hero__actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: 10px; }
.aww-hero__replace { display: flex; align-items: center; gap: 5px; font-size: 0.76rem; color: var(--text-light); }
.aww-hero__pipe { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 14px; }
.aww-hero__pipe span { padding: 4px 10px; border-radius: 6px; background: var(--bg-soft); border: 1px solid var(--border-color); font-size: 0.72rem; color: var(--text-light); }
.aww-section-title { font-size: 0.82rem; font-weight: 700; color: var(--text-muted); letter-spacing: 0.04em; margin-top: 4px; }
.aww-modgrid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.aww-modcard { display: flex; align-items: center; gap: 10px; padding: 13px; border: 1px solid var(--border-color); border-radius: 12px; background: var(--surface-color, var(--bg-card)); color: var(--text-color); cursor: pointer; text-align: left; transition: all 0.18s; }
.aww-modcard:hover { border-color: var(--primary-color); box-shadow: var(--shadow-md); transform: translateY(-1px); }
.aww-modcard__icon { width: 36px; height: 36px; border-radius: 10px; display: flex; align-items: center; justify-content: center; font-size: 1.15rem; background: var(--bg-soft); flex-shrink: 0; }
.aww-modcard__body { flex: 1; min-width: 0; }
.aww-modcard__title { font-weight: 700; font-size: 0.88rem; }
.aww-modcard__hint { font-size: 0.7rem; color: var(--text-light); }
.aww-modcard__count { font-weight: 700; color: var(--primary-color); font-size: 1.05rem; text-align: center; }
.aww-modcard__count span { display: block; font-size: 0.62rem; font-weight: 400; color: var(--text-muted); }
.aww-empty { padding: 34px 16px; border: 1px dashed var(--border-color); border-radius: 12px; text-align: center; color: var(--text-light); background: var(--bg-soft); }
.aww-empty.is-sm { padding: 20px 12px; font-size: 0.8rem; }
.aww-empty__art { font-size: 2rem; margin-bottom: 8px; opacity: 0.7; }
.aww-empty p { margin: 0; font-size: 0.84rem; }
.aww-recent { display: grid; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); gap: 8px; }
.aww-recent__item { display: flex; align-items: center; gap: 7px; padding: 9px 11px; border: 1px solid var(--border-color); border-radius: 9px; background: var(--surface-color, var(--bg-card)); cursor: pointer; font-size: 0.78rem; min-width: 0; }
.aww-recent__item:hover { border-color: var(--primary-color); }
.aww-recent__item b { color: var(--text-color); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 90px; }
.aww-recent__excerpt { color: var(--text-muted); font-size: 0.7rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; flex: 1; }
.aww-bar { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.aww-bar__title { font-size: 1.1rem; font-weight: 700; color: var(--text-color); }
.aww-bar__sub { font-size: 0.75rem; color: var(--text-light); margin-top: 2px; }
.aww-bar__actions { display: flex; gap: 6px; flex-wrap: wrap; }
.aww-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 12px; }
.aww-card { display: flex; flex-direction: column; border: 1px solid var(--border-color); border-radius: 12px; background: var(--surface-color, var(--bg-card)); overflow: hidden; transition: border-color 0.18s, box-shadow 0.18s; }
.aww-card:hover { box-shadow: var(--shadow-md); }
.aww-card.is-active { border-color: var(--primary-color); box-shadow: 0 0 0 2px color-mix(in srgb, var(--primary-color) 18%, transparent); }
.aww-card.is-refined { border-left: 3px solid var(--success-color); }
.aww-card__head { display: flex; align-items: center; gap: 7px; padding: 9px 11px; background: var(--bg-soft); border-bottom: 1px solid var(--border-color); }
.aww-card__idx { font-size: 0.68rem; font-weight: 700; color: var(--text-muted); font-variant-numeric: tabular-nums; }
.aww-card__title { flex: 1; min-width: 0; border: none; background: transparent; color: var(--text-color); font-size: 0.85rem; font-weight: 700; outline: none; padding: 2px; }
.aww-card__title:focus { box-shadow: inset 0 -1px var(--primary-color); }
.aww-card__status { font-size: 0.64rem; padding: 2px 7px; border-radius: 999px; background: var(--bg-card); color: var(--text-light); white-space: nowrap; }
.aww-card__status.generated { color: var(--primary-color); }
.aww-card__status.refined { color: var(--success-color); }
.aww-card__body { width: 100%; border: none; resize: vertical; padding: 11px; color: var(--text-color); background: transparent; font-size: 0.8rem; line-height: 1.7; outline: none; font-family: var(--font-sans); }
.aww-card__foot { display: flex; align-items: center; justify-content: space-between; gap: 6px; padding: 7px 11px; border-top: 1px solid var(--border-color); }
.aww-card__count { font-size: 0.68rem; color: var(--text-muted); }
.aww-card__acts { display: flex; gap: 4px; }
.aww-card__acts button { padding: 3px 8px; border: 1px solid var(--border-color); border-radius: 6px; background: var(--bg-card); color: var(--text-light); font-size: 0.68rem; cursor: pointer; }
.aww-card__acts button:hover { border-color: var(--primary-color); color: var(--primary-color); }
.aww-card__acts button.aww-card__refine { border-color: color-mix(in srgb, var(--primary-color) 40%, transparent); color: var(--primary-color); font-weight: 600; }
.aww-card__acts button.is-danger:hover { border-color: var(--danger-color); color: var(--danger-color); }
.aww-card.is-compact .aww-card__body { min-height: 80px; }
.aww-board { display: flex; flex-direction: column; gap: 8px; }
.aww-track { display: flex; gap: 10px; align-items: flex-start; padding: 10px; border: 1px solid var(--border-color); border-radius: 10px; background: var(--bg-soft); }
.aww-track__label { width: 90px; flex-shrink: 0; display: flex; align-items: center; gap: 5px; font-size: 0.75rem; font-weight: 700; color: var(--text-color); }
.aww-track__label small { display: block; font-weight: 400; color: var(--text-muted); font-size: 0.64rem; }
.aww-track__dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.aww-track__items { display: flex; gap: 8px; overflow-x: auto; flex: 1; padding-bottom: 3px; }
.aww-track__item { min-width: 150px; max-width: 190px; padding: 8px 10px; border: 1px solid var(--border-color); border-radius: 8px; background: var(--bg-card); cursor: pointer; }
.aww-track__item:hover, .aww-track__item.is-active { border-color: var(--primary-color); }
.aww-track__item-title { font-size: 0.76rem; font-weight: 700; color: var(--text-color); }
.aww-track__item-body { font-size: 0.68rem; color: var(--text-light); line-height: 1.5; margin-top: 3px; }
.aww-track__placeholder { font-size: 0.72rem; color: var(--text-muted); padding: 8px; }
.aww-timeline-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 12px; }
.aww-chapters { display: grid; grid-template-columns: 210px 1fr; gap: 12px; align-items: start; }
.aww-chapters__list { display: flex; flex-direction: column; gap: 4px; max-height: 640px; overflow-y: auto; padding: 8px; border: 1px solid var(--border-color); border-radius: 10px; background: var(--bg-soft); }
.aww-chapters__item { display: flex; align-items: center; gap: 7px; padding: 8px; border: none; border-radius: 7px; background: transparent; color: var(--text-color); cursor: pointer; text-align: left; font-size: 0.78rem; }
.aww-chapters__item:hover { background: var(--bg-card); }
.aww-chapters__item.is-active { background: color-mix(in srgb, var(--primary-color) 12%, transparent); color: var(--primary-color); font-weight: 700; }
.aww-chapters__num { width: 22px; height: 22px; border-radius: 50%; display: flex; align-items: center; justify-content: center; background: var(--gradient-primary); color: #fff; font-size: 0.68rem; flex-shrink: 0; }
.aww-chapters__name { flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.aww-chapters__badge { font-size: 0.62rem; color: var(--text-muted); }
.aww-chapters__badge.generated { color: var(--success-color); }
.aww-chapters__editor { display: flex; flex-direction: column; gap: 8px; padding: 14px; border: 1px solid var(--border-color); border-radius: 12px; background: var(--surface-color, var(--bg-card)); }
.aww-chapters__head { display: flex; align-items: center; gap: 10px; font-weight: 700; color: var(--text-color); }
.aww-chapters__editor label { font-size: 0.76rem; font-weight: 700; color: var(--text-light); margin-top: 4px; }
.aww-chapters__words { font-weight: 400; color: var(--text-muted); }
.aww-editor-area { width: 100%; border: 1px solid var(--border-color); border-radius: 8px; padding: 10px; background: var(--bg-card); color: var(--text-color); font-size: 0.82rem; line-height: 1.8; resize: vertical; outline: none; font-family: var(--font-sans); }
.aww-editor-area:focus { border-color: var(--primary-color); }
.aww-editor-area.is-body { min-height: 260px; }
.aww-chapters__gen, .aww-chapters__foot { display: flex; gap: 8px; }
.aww-review-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; }
.aww-review-card { padding: 16px; border: 1px solid var(--border-color); border-radius: 10px; background: var(--surface-color, var(--bg-card)); text-align: center; }
.aww-review-card b { display: block; font-size: 1.5rem; color: var(--primary-color); }
.aww-review-card span { font-size: 0.72rem; color: var(--text-light); }
@media (max-width: 1100px) { .aww-chapters { grid-template-columns: 1fr; } .aww-modgrid { grid-template-columns: 1fr; } }
</style>
