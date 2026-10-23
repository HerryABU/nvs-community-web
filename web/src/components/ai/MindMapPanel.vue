<template>
  <!-- 思维导图：需求 → LLM 层级大纲 → mermaid 导图（不引入任何第三方图库） -->
  <div class="mind-wrap">
    <div class="mind-form">
      <el-input
        v-model="topic"
        type="textarea"
        :rows="2"
        resize="none"
        placeholder="请输入您想要生成内容的简单描述，如：为「异步编程」做一份学习路径导图"
        @keydown.enter.exact.prevent="gen"
      />
      <div class="mind-row">
        <span class="mind-label">层级</span>
        <el-radio-group v-model="depth" size="small">
          <el-radio-button :value="2">2 级</el-radio-button>
          <el-radio-button :value="3">3 级</el-radio-button>
          <el-radio-button :value="4">4 级</el-radio-button>
        </el-radio-group>
        <span class="muted small">按 tokens 计费；命中免费额度则不扣代币</span>
        <el-button type="primary" :loading="loading" :disabled="!topic.trim()" @click="gen">
          智能生成思维导图
        </el-button>
      </div>
    </div>

    <div v-if="outline" class="mind-body">
      <div class="mind-view">
        <div class="mind-toolbar">
          <el-radio-group v-model="viewMode" size="small">
            <el-radio-button value="chart">导图</el-radio-button>
            <el-radio-button value="outline">大纲</el-radio-button>
          </el-radio-group>
          <el-button size="small" text @click="copyOutline">复制大纲</el-button>
        </div>
        <div v-show="viewMode === 'chart'" ref="chartEl" class="mind-chart" />
        <pre v-show="viewMode === 'outline'" class="mind-outline">{{ outline }}</pre>
      </div>
    </div>
    <el-empty v-else description="输入需求，一键生成层级导图" :image-size="80" />
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick } from 'vue';
import { ElMessage } from 'element-plus';
import { aiHubApi } from '@/api/aiHub';

const topic = ref('');
const depth = ref(3);
const loading = ref(false);
const outline = ref('');
const viewMode = ref<'chart' | 'outline'>('chart');
const chartEl = ref<HTMLElement>();
let renderSeq = 0;

/** Markdown 大纲 → mermaid mindmap 语法（缩进即层级） */
function toMermaid(md: string): string {
  const clean = (s: string) =>
    s.replace(/[*_`>#]/g, '').replace(/[()\[\]{}]/g, '').replace(/["']/g, '').replace(/\s+/g, ' ').trim().slice(0, 40);
  const lines: string[] = ['mindmap'];
  let rootAdded = false;
  for (const raw of md.split('\n')) {
    const line = raw.replace(/\t/g, '  ');
    const t = line.trim();
    if (!t) continue;
    const h = /^(#{1,6})\s+(.*)$/.exec(t);
    if (h) {
      const level = h[1].length;
      const text = clean(h[2]);
      if (!text) continue;
      if (!rootAdded) {
        lines.push(`  root((${text}))`);
        rootAdded = true;
        continue;
      }
      lines.push('  ' + '  '.repeat(Math.max(0, level - 1)) + text);
      continue;
    }
    const b = /^[-*+]\s+(.*)$/.exec(t);
    if (b) {
      const text = clean(b[1]);
      if (!text) continue;
      if (!rootAdded) {
        lines.push(`  root((${text}))`);
        rootAdded = true;
        continue;
      }
      lines.push('      ' + text);
      continue;
    }
    // 普通段落：并入同级
    const text = clean(t);
    if (text && rootAdded) lines.push('    ' + text);
  }
  if (!rootAdded) lines.push('  root((思维导图))');
  return lines.join('\n');
}

async function renderChart() {
  const el = chartEl.value;
  if (!el || !outline.value) return;
  const seq = ++renderSeq;
  try {
    const m = await import('mermaid');
    const mm = (m as any).default || m;
    mm.initialize({ startOnLoad: false, securityLevel: 'strict', theme: 'default' });
    const { svg } = await mm.render(`mind-${seq}`, toMermaid(outline.value));
    if (seq !== renderSeq) return; // 竞态：只渲染最后一次
    el.innerHTML = svg;
  } catch (e: any) {
    el.innerHTML = `<div class="mind-err">导图渲染失败：${String(e?.message || e)}</div>`;
  }
}

async function gen() {
  const t = topic.value.trim();
  if (!t || loading.value) return;
  loading.value = true;
  try {
    const res = await aiHubApi.mindmap({ topic: t, depth: depth.value });
    if (res.data.code === 0) {
      outline.value = res.data.data.outline || '';
      viewMode.value = 'chart';
      await nextTick();
      await renderChart();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成失败');
  } finally {
    loading.value = false;
  }
}

async function copyOutline() {
  try {
    await navigator.clipboard.writeText(outline.value);
    ElMessage.success('已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择');
  }
}
</script>

<style scoped>
.mind-wrap { padding: 16px; height: 100%; display: flex; flex-direction: column; min-height: 0; }
.mind-form { background: var(--el-fill-color-light, #f8f9fa); border-radius: 12px; padding: 14px 16px; margin-bottom: 14px; }
.mind-row { display: flex; align-items: center; gap: 10px; margin-top: 10px; flex-wrap: wrap; }
.mind-label { font-size: 0.85rem; font-weight: 600; color: var(--text-color, #333); }
.mind-row .el-button { margin-left: auto; }
.mind-body { flex: 1; min-height: 0; display: flex; }
.mind-view { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.mind-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.mind-chart {
  flex: 1; min-height: 240px; overflow: auto; border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 10px; background: var(--el-bg-color, #fff); padding: 10px;
}
.mind-chart :deep(svg) { max-width: 100%; height: auto; }
.mind-outline {
  flex: 1; overflow: auto; margin: 0; padding: 12px 14px; border-radius: 10px;
  background: var(--el-fill-color-light, #f8f9fa); font-size: 0.82rem; line-height: 1.7;
  white-space: pre-wrap; word-break: break-word;
}
.mind-err { color: #ef4444; font-size: 0.82rem; padding: 12px; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.75rem; }
</style>
