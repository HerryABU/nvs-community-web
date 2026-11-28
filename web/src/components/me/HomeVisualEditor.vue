<template>
  <div class="hve">
    <div class="hve-toolbar">
      <span class="hve-label">预设模板</span>
      <el-select :model-value="templateId" placeholder="选择模板一键套用" style="width: 220px" @update:model-value="applyTemplate">
        <el-option v-for="t in templates" :key="t.id" :label="t.name" :value="t.id" />
      </el-select>
      <el-button link type="primary" @click="applyTemplate(templateId)">重新套用</el-button>
    </div>

    <div class="hve-grid">
      <div class="hve-config">
        <el-form label-width="88px" size="small">
          <el-form-item label="标题"><el-input v-model="cfg.title" maxlength="60" placeholder="主页标题" /></el-form-item>
          <el-form-item label="副标题"><el-input v-model="cfg.subtitle" maxlength="120" placeholder="一句话介绍" /></el-form-item>
          <el-form-item label="背景色">
            <div class="hve-color"><el-color-picker v-model="cfg.bgColor" /><span>{{ cfg.bgColor }}</span></div>
          </el-form-item>
          <el-form-item label="文字色">
            <div class="hve-color"><el-color-picker v-model="cfg.textColor" /><span>{{ cfg.textColor }}</span></div>
          </el-form-item>
          <el-form-item label="背景图"><el-input v-model="cfg.bgImage" placeholder="可选，图片 URL" /></el-form-item>
          <el-form-item label="对齐">
            <el-radio-group v-model="cfg.align">
              <el-radio-button value="left">左</el-radio-button>
              <el-radio-button value="center">居中</el-radio-button>
              <el-radio-button value="right">右</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="圆角"><el-slider v-model="cfg.radius" :min="0" :max="32" /></el-form-item>
          <el-form-item label="卡片阴影"><el-switch v-model="cfg.shadow" /></el-form-item>
          <el-form-item label="显示模块">
            <el-checkbox-group v-model="cfg.modules">
              <el-checkbox value="stats">数据概览</el-checkbox>
              <el-checkbox value="works">最近作品</el-checkbox>
              <el-checkbox value="blogs">最近博客</el-checkbox>
              <el-checkbox value="links">常用链接</el-checkbox>
            </el-checkbox-group>
          </el-form-item>
          <el-form-item label="模块顺序">
            <div class="hve-sort">
              <div v-for="(m, i) in orderedModules" :key="m" class="hve-sort-item">
                <span>{{ moduleLabel(m) }}</span>
                <el-button link size="small" :disabled="i === 0" @click="move(i, -1)">↑</el-button>
                <el-button link size="small" :disabled="i === orderedModules.length - 1" @click="move(i, 1)">↓</el-button>
              </div>
              <div v-if="!orderedModules.length" class="hve-dim">未选择模块</div>
            </div>
          </el-form-item>
        </el-form>
        <el-button type="primary" size="small" @click="onGenerateClick">生成到代码框</el-button>
        <el-button size="small" @click="parseFromHtml">从代码框读取</el-button>
      </div>

      <div class="hve-preview-wrap">
        <div class="hve-preview-title">预览</div>
        <div class="hve-preview" v-html="generatedHtml" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue';
import { ElMessage } from 'element-plus';

const props = defineProps<{ modelValue: string }>();
const emit = defineEmits<{ 'update:modelValue': [v: string]; generate: [html: string] }>();

interface Config {
  title: string; subtitle: string; bgColor: string; textColor: string;
  bgImage: string; align: 'left' | 'center' | 'right'; radius: number;
  shadow: boolean; modules: string[]; order: string[];
}
const cfg = reactive<Config>({
  title: '欢迎来到我的主页', subtitle: '用可视化装修打造属于你的空间',
  bgColor: '#6366f1', textColor: '#ffffff', bgImage: '', align: 'center',
  radius: 16, shadow: true, modules: ['stats', 'works'],
  order: ['stats', 'works', 'blogs', 'links'],
});
const templateId = ref('aurora');
const templates = [
  { id: 'aurora', name: '极光紫', patch: { bgColor: '#6366f1', textColor: '#ffffff', title: '欢迎来到我的主页', radius: 16, shadow: true } },
  { id: 'ocean', name: '深海蓝', patch: { bgColor: '#0ea5e9', textColor: '#ffffff', title: '海风与远方', radius: 20, shadow: true } },
  { id: 'forest', name: '森林绿', patch: { bgColor: '#10b981', textColor: '#ffffff', title: '自然生长', radius: 12, shadow: false } },
  { id: 'sunset', name: '暖阳橙', patch: { bgColor: '#f97316', textColor: '#ffffff', title: '落日余晖', radius: 24, shadow: true } },
  { id: 'paper', name: '素白纸', patch: { bgColor: '#f8fafc', textColor: '#1e293b', title: '简约留白', radius: 8, shadow: false } },
];
const moduleLabel = (m: string) => ({ stats: '数据概览', works: '最近作品', blogs: '最近博客', links: '常用链接' }[m] || m);
const orderedModules = computed(() => cfg.order.filter((m) => cfg.modules.includes(m)));
function move(i: number, d: number) {
  const arr = [...cfg.order];
  const active = orderedModules.value;
  const from = active[i];
  const to = active[i + d];
  const fi = arr.indexOf(from), ti = arr.indexOf(to);
  [arr[fi], arr[ti]] = [arr[ti], arr[fi]];
  cfg.order = arr;
}

function applyTemplate(id: string) {
  templateId.value = id;
  const t = templates.find((x) => x.id === id);
  if (!t) return;
  Object.assign(cfg, t.patch);
}

function buildHtml(): string {
  const bg = cfg.bgImage
    ? `background-image:linear-gradient(rgba(0,0,0,.35),rgba(0,0,0,.35)),url('${escapeAttr(cfg.bgImage)}');background-size:cover;background-position:center;`
    : `background:${escapeAttr(cfg.bgColor)};`;
  const shadow = cfg.shadow ? 'box-shadow:0 8px 28px rgba(0,0,0,.18);' : '';
  const blockStyle = `display:flex;justify-content:space-between;padding:10px 12px;margin-bottom:6px;border-radius:8px;background:#fff;border:1px solid #e5e7eb;font-size:14px;color:#1e293b`;
  const blocks = orderedModules.value.map((m) => {
    const text = m === 'stats' ? ['数据概览', '作品 · 字数 · 粉丝']
      : m === 'works' ? ['最近作品', '这里展示你的最新作品']
      : m === 'blogs' ? ['最近博客', '这里展示你的最新博客']
      : ['常用链接', '你的友情链接 / 社交链接'];
    return `<div style="${blockStyle}"><b>${text[0]}</b><span style="color:#94a3b8">${text[1]}</span></div>`;
  }).join('');
  return `<div class="hve-hero" style="${bg}color:${escapeAttr(cfg.textColor)};text-align:${cfg.align};border-radius:${cfg.radius}px;padding:32px;${shadow}">
  <h1 style="margin:0 0 8px;font-size:28px">${escapeHtml(cfg.title)}</h1>
  <p style="margin:0;opacity:.85">${escapeHtml(cfg.subtitle)}</p>
</div>
<div class="hve-modules" style="margin-top:16px">${blocks}</div>`;
}
const generatedHtml = computed(() => buildHtml());

function escapeHtml(s: string) {
  return (s || '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c] || c));
}
function escapeAttr(s: string) {
  return escapeHtml(s).replace(/[()]/g, (c) => (c === '(' ? '%28' : '%29'));
}

function onGenerateClick() {
  const html = buildHtml();
  emit('update:modelValue', html);
  emit('generate', html);
}
defineExpose({ buildHtml, onGenerateClick });

// 从代码框解析（尽力而为）：识别 hve-hero 中的标题/副标题/背景色/文字色/对齐
function parseFromHtml() {
  const html = props.modelValue || '';
  if (!html.includes('hve-hero')) { ElMessage.warning('代码框内容不是本编辑器生成的，无法解析'); return; }
  const title = html.match(/<h1[^>]*>([\s\S]*?)<\/h1>/)?.[1] || '';
  const subtitle = html.match(/<p[^>]*>([\s\S]*?)<\/p>/)?.[1] || '';
  const bg = html.match(/background:(#[0-9a-fA-F]{3,8}|[a-z]+);/)?.[1];
  const color = html.match(/color:(#[0-9a-fA-F]{3,8}|[a-z]+);/)?.[1];
  const align = html.match(/text-align:(left|center|right);/)?.[1] as Config['align'] | undefined;
  cfg.title = decode(title); cfg.subtitle = decode(subtitle);
  if (bg) cfg.bgColor = bg; if (color) cfg.textColor = color; if (align) cfg.align = align;
  ElMessage.success('已从代码框读取可识别设置');
}
function decode(s: string) {
  return s.replace(/&amp;/g, '&').replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"').replace(/&#39;/g, "'");
}

watch(() => props.modelValue, () => { /* 代码模式修改后不自动覆盖可视化配置，避免丢失 */ });
</script>

<style scoped>
.hve-toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }
.hve-label { font-size: 0.85rem; color: var(--text-color); }
.hve-grid { display: grid; grid-template-columns: minmax(280px, 1fr) minmax(280px, 1fr); gap: 16px; }
.hve-color { display: flex; align-items: center; gap: 8px; font-size: 0.78rem; color: var(--text-light, #888); }
.hve-sort { display: flex; flex-direction: column; gap: 4px; }
.hve-sort-item { display: flex; align-items: center; gap: 4px; font-size: 0.8rem; }
.hve-dim { font-size: 0.78rem; color: var(--text-light, #999); }
.hve-preview-wrap { border: 1px solid var(--border-color, #e5e7eb); border-radius: 12px; padding: 12px; background: var(--bg-soft, #f8fafc); }
.hve-preview-title { font-size: 0.8rem; color: var(--text-light, #888); margin-bottom: 8px; }
.hve-preview :deep(.hve-block) { display: flex; justify-content: space-between; padding: 10px 12px; margin-bottom: 6px; border-radius: 8px; background: var(--bg-card, #fff); border: 1px solid var(--border-color, #eee); font-size: 0.82rem; }
.hve-preview :deep(.hve-block span) { color: var(--text-light, #888); }
@media (max-width: 900px) { .hve-grid { grid-template-columns: 1fr; } }
</style>
