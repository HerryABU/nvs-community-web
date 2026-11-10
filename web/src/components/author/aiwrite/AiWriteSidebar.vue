<template>
  <aside class="aws">
    <!-- 作品头 -->
    <div class="aws-head">
      <button class="aws-back" title="返回工程列表" @click="$emit('back')">←</button>
      <div class="aws-head__text">
        <div class="aws-title" :title="project.title">{{ project.title }}</div>
        <div class="aws-sub">{{ styleLabel }}</div>
      </div>
    </div>

    <!-- 引子锚点 -->
    <div class="aws-seed" :class="{ 'is-empty': !project.seed_topic }">
      <div class="aws-seed__label">⚓ 引子 / 话题（Anchor）</div>
      <div class="aws-seed__body">{{ project.seed_topic || '尚未设定，去「引子启动」写一句话灵感' }}</div>
    </div>

    <!-- 全流程导航（对标 Pagekin / 织叙的左树） -->
    <nav class="aws-nav">
      <div class="aws-nav__group">创作流程</div>
      <button
        v-for="v in navViews"
        :key="v.key"
        class="aws-nav__item"
        :class="{ 'is-active': view === v.key }"
        @click="$emit('update:view', v.key)"
      >
        <span class="aws-nav__icon">{{ v.icon }}</span>
        <span class="aws-nav__label">{{ v.label }}</span>
        <span v-if="v.key === 'chapters' && chapterCount" class="aws-nav__badge">{{ chapterCount }}</span>
        <span v-else-if="detailCount(v.key)" class="aws-nav__badge">{{ detailCount(v.key) }}</span>
      </button>

      <div class="aws-nav__group">审阅与输出</div>
      <button
        v-for="v in toolViews"
        :key="v.key"
        class="aws-nav__item"
        :class="{ 'is-active': view === v.key }"
        @click="$emit('update:view', v.key)"
      >
        <span class="aws-nav__icon">{{ v.icon }}</span>
        <span class="aws-nav__label">{{ v.label }}</span>
        <span v-if="v.key === 'review' && warnCount" class="aws-nav__badge is-warn">{{ warnCount }}</span>
      </button>
    </nav>

    <!-- 状态快照（对标 天命 / PlotPilot 的章后状态） -->
    <div class="aws-snap">
      <div class="aws-snap__title">项目状态快照</div>
      <div class="aws-snap__grid">
        <div><b>{{ filledModules }}/4</b><span>素材模块</span></div>
        <div><b>{{ totalDetails }}</b><span>细节卡</span></div>
        <div><b>{{ chapterCount }}</b><span>章节</span></div>
        <div><b>{{ totalWords }}</b><span>正文字数</span></div>
      </div>
      <div class="aws-snap__bar"><i :style="{ width: progress + '%' }" /></div>
      <div class="aws-snap__hint">完成度 {{ progress }}% · 真源为细节卡，素材字段自动编译</div>
    </div>

    <!-- 沉浸模式 -->
    <div class="aws-foot">
      <label class="aws-imm">
        <input type="checkbox" :checked="immersive" @change="$emit('update:immersive', !immersive)" />
        <span>沉浸模式（隐藏站点导航）</span>
      </label>
      <button class="aws-publish" :disabled="publishing" @click="$emit('publish')">
        {{ publishing ? '生成中…' : '🚀 生成小说' }}
      </button>
      <div class="aws-kbd">Ctrl / ⌘ + K　快速跳转</div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { AiWriteProject, AiWriteDetail } from '@/api/authorai';

const props = defineProps<{
  project: AiWriteProject;
  view: string;
  details: AiWriteDetail[];
  chapterCount: number;
  totalWords: number;
  warnCount: number;
  immersive: boolean;
  publishing: boolean;
  styleLabel: string;
}>();
defineEmits<{
  (e: 'update:view', v: string): void;
  (e: 'update:immersive', v: boolean): void;
  (e: 'back'): void;
  (e: 'publish'): void;
}>();

const navViews = [
  { key: 'overview', label: '概览 / 引子启动', icon: '◈' },
  { key: 'setting', label: '世界观 / 设定', icon: '🌍' },
  { key: 'character', label: '人物', icon: '👤' },
  { key: 'outline', label: '大纲', icon: '📑' },
  { key: 'timeline', label: '时间线看板', icon: '🕰️' },
  { key: 'storyline', label: '故事线', icon: '🧵' },
  { key: 'chapters', label: '正文 / 章节', icon: '✍️' },
];
const toolViews = [
  { key: 'visual', label: '可视化图谱', icon: '🕸️' },
  { key: 'review', label: '一致性审阅', icon: '🔍' },
];

const totalDetails = computed(() => props.details.length);
function detailCount(view: string) {
  if (!['setting', 'character', 'outline', 'timeline', 'storyline'].includes(view)) return 0;
  const mod = view === 'character' ? 'character' : view;
  return props.details.filter((d) => d.module === mod).length;
}
const filledModules = computed(() => {
  const p = props.project;
  return [p.setting, p.characters, p.outline, p.timeline, p.storyline].filter((s) => (s || '').trim()).length;
});
const progress = computed(() => {
  const parts = [filledModules.value / 5, props.chapterCount > 0 ? 1 : 0, props.project.seed_topic ? 1 : 0];
  return Math.round((parts.reduce((a, b) => a + b, 0) / parts.length) * 100);
});
</script>

<style scoped>
.aws { width: 240px; flex-shrink: 0; height: calc(100vh - 120px); min-height: 620px; display: flex; flex-direction: column; gap: 12px; padding: 14px; background: var(--surface-color, var(--bg-card)); border: 1px solid var(--border-color); border-radius: var(--radius-lg); box-shadow: var(--shadow-card); position: sticky; top: 16px; }
.aws-head { display: flex; align-items: center; gap: 8px; }
.aws-back { width: 28px; height: 28px; border-radius: 8px; border: 1px solid var(--border-color); background: var(--bg-soft); color: var(--text-color); cursor: pointer; flex-shrink: 0; }
.aws-back:hover { border-color: var(--primary-color); color: var(--primary-color); }
.aws-head__text { min-width: 0; }
.aws-title { font-weight: 700; font-size: 0.95rem; color: var(--text-color); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.aws-sub { font-size: 0.72rem; color: var(--text-light); }
.aws-seed { padding: 10px 11px; border-radius: 10px; background: color-mix(in srgb, var(--accent-color) 10%, transparent); border: 1px solid color-mix(in srgb, var(--accent-color) 30%, transparent); }
.aws-seed.is-empty { background: var(--bg-soft); border-color: var(--border-color); }
.aws-seed__label { font-size: 0.7rem; font-weight: 700; color: var(--accent-color); margin-bottom: 4px; }
.aws-seed.is-empty .aws-seed__label { color: var(--text-light); }
.aws-seed__body { font-size: 0.78rem; color: var(--text-color); line-height: 1.55; display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; }
.aws-nav { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
.aws-nav__group { font-size: 0.68rem; font-weight: 700; letter-spacing: 0.08em; color: var(--text-muted); text-transform: uppercase; padding: 10px 8px 4px; }
.aws-nav__item { display: flex; align-items: center; gap: 8px; width: 100%; padding: 8px 10px; border: none; border-radius: 8px; background: transparent; color: var(--text-color); font-size: 0.85rem; cursor: pointer; text-align: left; transition: background 0.18s, color 0.18s; }
.aws-nav__item:hover { background: var(--bg-hover); }
.aws-nav__item.is-active { background: color-mix(in srgb, var(--primary-color) 12%, transparent); color: var(--primary-color); font-weight: 600; }
.aws-nav__icon { width: 18px; text-align: center; font-size: 0.95rem; }
.aws-nav__label { flex: 1; }
.aws-nav__badge { min-width: 20px; padding: 1px 6px; border-radius: 999px; background: var(--bg-soft); color: var(--text-light); font-size: 0.68rem; text-align: center; }
.aws-nav__badge.is-warn { background: color-mix(in srgb, var(--warning-color) 18%, transparent); color: var(--warning-color); }
.aws-snap { padding: 11px; border-radius: 10px; background: var(--bg-soft); border: 1px solid var(--border-color); }
.aws-snap__title { font-size: 0.72rem; font-weight: 700; color: var(--text-light); margin-bottom: 8px; }
.aws-snap__grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.aws-snap__grid > div { display: flex; flex-direction: column; }
.aws-snap__grid b { font-size: 1.05rem; color: var(--text-color); line-height: 1.2; }
.aws-snap__grid span { font-size: 0.68rem; color: var(--text-light); }
.aws-snap__bar { height: 5px; border-radius: 999px; background: var(--border-color); margin-top: 10px; overflow: hidden; }
.aws-snap__bar i { display: block; height: 100%; background: var(--gradient-primary); border-radius: 999px; transition: width 0.3s; }
.aws-snap__hint { font-size: 0.66rem; color: var(--text-muted); margin-top: 6px; line-height: 1.5; }
.aws-foot { display: flex; flex-direction: column; gap: 8px; }
.aws-imm { display: flex; align-items: center; gap: 6px; font-size: 0.75rem; color: var(--text-light); cursor: pointer; }
.aws-publish { padding: 9px; border: none; border-radius: 9px; background: var(--gradient-success); color: #fff; font-weight: 600; font-size: 0.85rem; cursor: pointer; }
.aws-publish:disabled { opacity: 0.6; cursor: wait; }
.aws-kbd { font-size: 0.66rem; color: var(--text-muted); text-align: center; }
@media (max-width: 1100px) { .aws { width: 200px; } }
/* 移动端：全宽、去 sticky、高度自适应（由父容器堆叠） */
@media (max-width: 768px) {
  .aws { width: 100%; height: auto; min-height: 0; position: static; }
  .aws-nav { max-height: 220px; }
}
</style>
