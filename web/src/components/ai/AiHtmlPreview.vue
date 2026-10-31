<template>
  <el-dialog
    v-model="visible"
    class="ai-html-preview-dialog"
    :width="dialogWidth"
    top="4vh"
    :show-close="false"
    :close-on-click-modal="false"
    destroy-on-close
    append-to-body
  >
    <template #header>
      <div class="hp-head">
        <span class="hp-title">
          <span class="hp-dot" />
          HTML 预览
        </span>
        <span class="hp-meta">{{ sizeText }} · 沙箱隔离运行</span>
        <div class="hp-tabs">
          <button
            type="button"
            class="hp-tab"
            :class="{ 'is-active': tab === 'preview' }"
            @click="tab = 'preview'"
          >预览</button>
          <button
            type="button"
            class="hp-tab"
            :class="{ 'is-active': tab === 'code' }"
            @click="tab = 'code'"
          >源码</button>
        </div>
        <div class="hp-actions">
          <el-button size="small" text @click="reload">↻ 重载</el-button>
          <el-button size="small" text @click="download">下载 .html</el-button>
          <el-button size="small" text @click="visible = false">关闭</el-button>
        </div>
      </div>
    </template>

    <!-- 预览：iframe 用 sandbox 隔离，绝不 allow-same-origin -->
    <div v-show="tab === 'preview'" class="hp-body">
      <iframe
        ref="frameEl"
        :key="reloadKey"
        class="hp-frame"
        title="HTML 预览"
        sandbox="allow-scripts allow-popups allow-forms allow-modals"
        referrerpolicy="no-referrer"
        :srcdoc="srcDoc"
      />
    </div>
    <div v-show="tab === 'code'" class="hp-body hp-code-body">
      <pre class="hp-code"><code>{{ code }}</code></pre>
    </div>

    <div class="hp-foot">
      <span>
        🔒 页面运行在独立的空源沙箱里：读不到本站 Cookie / localStorage，也不能跳转或改写本页面。
      </span>
      <span class="hp-foot-hint">若预览空白，多半是代码用了外部资源（CDN / 图片）被网络拦截</span>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from 'vue';

const props = defineProps<{
  modelValue: boolean;
  /** 待预览的 HTML 源码 */
  code: string;
}>();

const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>();

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v),
});

const tab = ref<'preview' | 'code'>('preview');
const frameEl = ref<HTMLIFrameElement>();
const reloadKey = ref(0);

const dialogWidth = computed(() => (window.innerWidth <= 768 ? '96%' : 'min(1180px, 92vw)'));

const sizeText = computed(() => {
  const n = (props.code || '').length;
  return n > 1024 ? `${(n / 1024).toFixed(1)} KB` : `${n} B`;
});

watch(
  () => props.modelValue,
  (open) => {
    if (open) tab.value = 'preview';
  },
);

/**
 * 内存版 Storage 兜底。
 *
 * 无 allow-same-origin 的沙箱里，访问 localStorage / sessionStorage 会直接抛
 * SecurityError —— AI 生成的小游戏、待办清单十有八九要写 localStorage，
 * 一抛就整页白屏。这里在文档最前面把它们换成内存实现，
 * 让「刷新即丢」代替「直接崩溃」。只影响预览这个沙箱，不涉及任何真实存储。
 *
 * ⚠️ 标签必须用字符串拼接：SFC 的 <script> 块里一旦出现结束标签字面量，
 * 外层 HTML 解析器会把整块提前截断，整个组件直接编译失败。
 */
const S_OPEN = '<' + 'script>';
const S_CLOSE = '<' + '/script>';
const STORAGE_SHIM = `${S_OPEN}
(function(){
  function mem(){
    var m = Object.create(null);
    return {
      getItem: function(k){ return Object.prototype.hasOwnProperty.call(m, k) ? m[k] : null; },
      setItem: function(k, v){ m[k] = String(v); },
      removeItem: function(k){ delete m[k]; },
      clear: function(){ m = Object.create(null); },
      key: function(i){ return Object.keys(m)[i] || null; },
      get length(){ return Object.keys(m).length; }
    };
  }
  try {
    window.localStorage.setItem('__nvs_probe__', '1');
    window.localStorage.removeItem('__nvs_probe__');
  } catch (e) {
    try { Object.defineProperty(window, 'localStorage', { value: mem(), configurable: true }); } catch (e2) {}
    try { Object.defineProperty(window, 'sessionStorage', { value: mem(), configurable: true }); } catch (e2) {}
  }
})();
${S_CLOSE}`;

/** 把 AI 吐出的代码补成一份完整文档；已是完整文档的只补 viewport */
function buildDoc(raw: string): string {
  const src = (raw || '').trim();
  if (!src) return '<!doctype html><meta charset="utf-8"><p style="font:14px system-ui;padding:24px;color:#888">没有可预览的内容</p>';

  const isFull = /<html[\s>]/i.test(src) || /^\s*<!doctype/i.test(src);
  const viewport = '<meta name="viewport" content="width=device-width,initial-scale=1">';
  const shim = STORAGE_SHIM;

  if (!isFull) {
    // 片段：套一个中性骨架（不继承站点任何样式，避免「预览效果」失真）
    return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">${viewport}${shim}
<style>html,body{margin:0;padding:16px;font-family:system-ui,-apple-system,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif;color:#1f2328;background:#fff;line-height:1.6}</style>
</head><body>${src}</body></html>`;
  }

  // 完整文档：在 <head> 后插入（没有 head 就插到 <html> 后）
  if (/<head[^>]*>/i.test(src)) {
    return src.replace(/<head([^>]*)>/i, `<head$1>${viewport}${shim}`);
  }
  if (/<html[^>]*>/i.test(src)) {
    return src.replace(/<html([^>]*)>/i, `<html$1><head>${viewport}${shim}</head>`);
  }
  return `${viewport}${shim}${src}`;
}

const srcDoc = computed(() => buildDoc(props.code));

function reload() {
  // 换 key 让 iframe 整体重建，比手动改 srcdoc 可靠
  // （srcdoc 值不变时浏览器不会重跑文档）
  reloadKey.value += 1;
}

function download() {
  try {
    const blob = new Blob([buildDoc(props.code)], { type: 'text/html;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `nvs-ai-${Date.now()}.html`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 4000);
  } catch {
    /* 忽略 */
  }
}

onBeforeUnmount(() => {
  // 提前释放 iframe 里可能还在跑的定时器 / 音视频
  const el = frameEl.value;
  if (el) el.srcdoc = '';
});
</script>

<style scoped>
.hp-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.hp-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  font-size: 15px;
}
.hp-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #22c55e;
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.18);
}
.hp-meta {
  font-size: 12px;
  color: var(--text-light, #909399);
}
.hp-tabs {
  display: inline-flex;
  gap: 2px;
  padding: 2px;
  border-radius: 8px;
  background: var(--bg-page, #f2f3f5);
}
.hp-tab {
  border: 0;
  background: transparent;
  font-size: 12px;
  padding: 4px 12px;
  border-radius: 6px;
  cursor: pointer;
  color: var(--text-secondary, #606266);
  font-family: inherit;
}
.hp-tab.is-active {
  background: var(--card-bg, #fff);
  color: var(--el-color-primary, #409eff);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
}
.hp-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 2px;
}
.hp-body {
  height: 74vh;
  min-height: 320px;
  border: 1px solid var(--el-border-color, #ebeef5);
  border-radius: 10px;
  overflow: hidden;
  background: #fff;
}
.hp-frame {
  width: 100%;
  height: 100%;
  border: 0;
  display: block;
  background: #fff;
}
.hp-code-body {
  overflow: auto;
  background: #1e293b;
}
.hp-code {
  margin: 0;
  padding: 16px;
  color: #e2e8f0;
  font-family: var(--font-mono, Consolas, Monaco, monospace);
  font-size: 0.82rem;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}
.hp-foot {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  margin-top: 8px;
  font-size: 11.5px;
  color: var(--text-light, #909399);
}
.hp-foot-hint {
  opacity: 0.8;
}

@media (max-width: 768px) {
  .hp-body {
    height: 68vh;
  }
  .hp-meta,
  .hp-foot-hint {
    display: none;
  }
  .hp-actions {
    width: 100%;
    margin-left: 0;
  }
}
</style>

<style>
/* 弹窗本体不吃 scoped：它的根节点在 body 下 */
.ai-html-preview-dialog .el-dialog__body {
  padding-top: 4px;
}
.ai-html-preview-dialog .el-dialog__header {
  margin-right: 0;
  padding-bottom: 8px;
}
</style>
