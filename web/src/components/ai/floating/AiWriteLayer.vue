<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="ai-write-layer"
      :class="{ 'is-streaming': streaming }"
      :style="{ left: x + 'px', top: y + 'px' }"
      role="dialog"
      aria-label="AI 写作"
      @keydown.esc.prevent.stop
    >
      <!-- 拖拽手柄 / 标题栏 -->
      <div class="awl-head" @pointerdown="onHandleDown">
        <span class="awl-grip">⠿</span>
        <span class="awl-title">AI 写作</span>
        <span v-if="streaming" class="awl-live-dot" title="正在流式生成">● 生成中</span>
        <span class="awl-spacer" />
        <button class="awl-x" type="button" title="关闭（ESC 不会关闭此窗口）" @click="close">✕</button>
      </div>

      <div class="awl-body">
        <!-- 余额 -->
        <div class="awl-balance">
          <span>余额：<b>{{ status.balance }}</b> {{ status.token_name }}</span>
          <el-button text type="primary" size="small" @click="goWallet">去充值</el-button>
        </div>

        <!-- 模型 -->
        <div class="awl-row">
          <label>模型</label>
          <el-select v-model="form.model_id" size="small" :disabled="streaming" style="flex:1">
            <el-option v-for="m in status.models" :key="m.id" :label="modelLabel(m)" :value="m.id" />
          </el-select>
        </div>

        <!-- 动作 -->
        <div class="awl-row awl-actions">
          <label>动作</label>
          <div class="awl-action-grid">
            <button
              v-for="a in ACTIONS"
              :key="a.value"
              type="button"
              class="awl-action"
              :class="{ active: form.action === a.value, disabled: streaming }"
              :title="a.hint"
              @click="form.action = a.value"
            >{{ a.label }}</button>
          </div>
        </div>

        <!-- 长度 -->
        <div class="awl-row">
          <label>长度</label>
          <el-radio-group v-model="form.max_tokens" size="small" :disabled="streaming">
            <el-radio-button :value="512">短</el-radio-button>
            <el-radio-button :value="1024">中</el-radio-button>
            <el-radio-button :value="2048">长</el-radio-button>
          </el-radio-group>
        </div>

        <!-- 输入：基于的原文 -->
        <div class="awl-input-label">
          写作依据
          <el-button text size="small" type="primary" @click="resetFromContext">恢复为当前内容</el-button>
        </div>
        <el-input
          v-model="form.text"
          type="textarea"
          :rows="6"
          resize="none"
          :disabled="streaming"
          :placeholder="inputPlaceholder"
        />

        <!-- 执行 -->
        <el-button
          type="primary"
          class="awl-run"
          :loading="streaming"
          :disabled="!canRun"
          @click="run"
        >{{ streaming ? '生成中…' : '生成（流式写入编辑器）' }}</el-button>

        <!-- 实时流式预览 -->
        <div v-if="liveText || result" class="awl-live">
          <div class="awl-live-label">
            实时输出
            <span v-if="result" class="awl-cost">
              <template v-if="result.free">免费额度</template>
              <template v-else>耗时 {{ result.tokens_in }}/{{ result.tokens_out }}，扣 {{ result.cost }}，余 {{ result.balance_after }}</template>
            </span>
          </div>
          <div class="awl-live-text">{{ liveText || result?.content }}</div>
          <div class="awl-live-actions" v-if="result">
            <el-button size="small" @click="copyResult">复制</el-button>
          </div>
        </div>

        <div v-if="error" class="awl-error">{{ error }}</div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onBeforeUnmount } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { aiApi } from '@/api/ai';
import { streamChat } from '@/api/aiHub';
import { useDraggable } from './useDraggable';
import type { StreamTarget } from '@/utils/editorStream';

const props = defineProps<{
  visible: boolean;
  /** 当前编辑器全文（续写 / 全部重写用） */
  contextText?: string;
  /** 当前选区文本（改写选中 / 润色用，为空则退化为全文） */
  selectionText?: string;
  /** 场景：仅影响提示词措辞，不影响能力 */
  scene?: 'novel_chapter' | 'novel_script' | 'blog';
}>();

const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void;
  (e: 'stream-begin', target: StreamTarget): void;
  (e: 'stream-chunk', text: string): void;
  (e: 'stream-end', payload: { content: string; model: string }): void;
  (e: 'ai-used', model: string): void;
}>();

const router = useRouter();

// ── 拖拽 ──
const { x, y, onHandleDown } = useDraggable(160, 140);

// ── 状态 ──
const status = reactive<{
  ai_enabled: boolean;
  token_enabled: boolean;
  token_name: string;
  balance: number;
  models: Array<{ id: number; name: string; price_in: number; price_out: number }>;
}>({
  ai_enabled: false,
  token_enabled: true,
  token_name: '代币',
  balance: 0,
  models: [],
});

const ACTIONS: Array<{ value: string; label: string; hint: string; target: StreamTarget }> = [
  { value: 'continue', label: '续写', hint: '从当前内容往后接着写', target: 'cursor' },
  { value: 'expand', label: '扩写', hint: '就选中段落补充细节与氛围', target: 'selection' },
  { value: 'polish', label: '润色', hint: '优化文笔，不改原意', target: 'selection' },
  { value: 'rewrite-selected', label: '改写选中', hint: '换表达保留原意（替换选区）', target: 'selection' },
  { value: 'rewrite-all', label: '全部重写', hint: '基于全文重写整篇', target: 'all' },
];

const form = reactive<{ model_id: number; action: string; max_tokens: number; text: string }>({
  model_id: 0,
  action: 'continue',
  max_tokens: 1024,
  text: '',
});

const streaming = ref(false);
const liveText = ref('');
const error = ref('');
const result = ref<{ content: string; tokens_in: number; tokens_out: number; cost: number; free?: boolean; balance_after: number } | null>(null);
const usedModelName = ref('');
let abortCtl: AbortController | null = null;

const canRun = computed(() => !!form.text.trim() && !!form.model_id && !streaming.value && status.ai_enabled);

const inputPlaceholder = computed(() => {
  const a = ACTIONS.find((x) => x.value === form.action);
  if (form.action === 'rewrite-all') return '将基于当前全文重写整篇（可在此先微调引导语）';
  if (a?.target === 'selection') return '将基于选中文本改写；为空则取当前全文';
  return '将基于当前内容续写 / 扩写';
});

function modelLabel(m: { id: number; name: string; price_in: number; price_out: number }) {
  const price = m.price_in === 0 && m.price_out === 0 ? '免费' : `${m.price_in}/${m.price_out}`;
  return `${m.name}（${price}）`;
}

function goWallet() {
  emit('update:visible', false);
  router.push('/wallet');
}

function close() {
  // 仅 X / 取消 可关闭；ESC 被头部的 @keydown.esc.prevent.stop 拦截
  if (streaming.value) abortCtl?.abort();
  emit('update:visible', false);
}

// ESC 防护：即便焦点在层内输入框，也不允许 ESC 关闭（只认 X / 取消）
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault();
    e.stopPropagation();
  }
}
watch(
  () => props.visible,
  (v) => {
    if (v) {
      window.addEventListener('keydown', onKeydown, true);
      loadStatus();
    } else {
      window.removeEventListener('keydown', onKeydown, true);
    }
  },
);

async function loadStatus() {
  try {
    const res = await aiApi.status();
    if (res.data.code === 0) {
      Object.assign(status, res.data.data);
      if (!form.model_id && status.models.length) form.model_id = status.models[0].id;
    }
  } catch {
    /* ignore */
  }
}

function resetFromContext() {
  form.text = currentSource();
}

// 按动作决定「写作依据」文本
function currentSource(): string {
  const sel = (props.selectionText || '').trim();
  const ctx = (props.contextText || '').trim();
  if (form.action === 'rewrite-all') return ctx;
  if (ACTIONS.find((a) => a.value === form.action)?.target === 'selection') {
    return sel || ctx;
  }
  return sel || ctx;
}

// 按动作拼提示词
function buildPrompt(): string {
  const t = currentSource();
  switch (form.action) {
    case 'continue':
      return `请基于以下正文继续往下写，保持人物、风格与上下文连贯。只输出续写内容本身，不要任何解释、前缀或引号包裹。\n\n${t}`;
    case 'expand':
      return `请对下面这段内容做合理扩写，补充细节、动作与氛围，保持原意与风格。只输出扩写后的文本，不要解释。\n\n${t}`;
    case 'polish':
      return `请润色下面这段内容，使文笔更流畅自然、用词更准确，但不改变原意与情节。只输出润色后的文本，不要解释。\n\n${t}`;
    case 'rewrite-selected':
      return `请改写下面这段内容，换一种表达但保留原意与风格。只输出改写后的文本，不要解释。\n\n${t}`;
    case 'rewrite-all':
      return `请基于以下全文重写，保持核心设定、人物与主线，优化结构与表达。只输出重写后的完整文本，不要任何解释或前缀。\n\n${t}`;
    default:
      return t;
  }
}

function resolveTarget(): StreamTarget {
  const base = ACTIONS.find((a) => a.value === form.action)?.target || 'cursor';
  // 选区类动作若无选区，退化为光标追加，避免误清空整篇
  if (base === 'selection' && !(props.selectionText || '').trim()) return 'cursor';
  return base;
}

async function run() {
  if (!canRun.value) {
    if (!status.ai_enabled) ElMessage.warning('站长暂未开启 AI 功能');
    else if (!form.text.trim()) ElMessage.warning('请先填入写作依据');
    else if (!form.model_id) ElMessage.warning('请选择模型');
    return;
  }
  // 同步最新依据
  form.text = currentSource();
  if (!form.text.trim()) {
    ElMessage.warning('写作依据为空，无法生成');
    return;
  }

  const target = resolveTarget();
  streaming.value = true;
  error.value = '';
  liveText.value = '';
  result.value = null;
  abortCtl = new AbortController();

  // 先通知父组件在编辑器里腾好插入锚点（光标/选区/整篇）
  emit('stream-begin', target);

  const m = status.models.find((x) => x.id === form.model_id);
  usedModelName.value = m ? m.name : '';

  try {
    const res = await streamChat(
      {
        model_id: form.model_id,
        message: buildPrompt(),
        max_tokens: form.max_tokens,
      },
      (chunk: string) => {
        liveText.value += chunk;
        emit('stream-chunk', chunk); // 父组件实时写入编辑器
      },
      abortCtl.signal,
    );
    if (res) {
      result.value = {
        content: res.content,
        tokens_in: (res as any).tokens_in || 0,
        tokens_out: (res as any).tokens_out || 0,
        cost: (res as any).cost || 0,
        free: (res as any).free,
        balance_after: (res as any).balance_after || 0,
      };
      status.balance = (res as any).balance_after || status.balance;
      emit('stream-end', { content: res.content, model: usedModelName.value });
      emit('ai-used', usedModelName.value);
    }
  } catch (e: any) {
    if (e?.name === 'AbortError') {
      // 用户取消，不报错
    } else {
      error.value = e?.message || '生成失败';
      ElMessage.error(error.value);
    }
  } finally {
    streaming.value = false;
    abortCtl = null;
  }
}

async function copyResult() {
  const txt = liveText.value || result.value?.content || '';
  if (!txt) return;
  try {
    await navigator.clipboard.writeText(txt);
    ElMessage.success('已复制');
  } catch {
    ElMessage.warning('浏览器拒绝访问剪贴板，请手动复制');
  }
}

// 打开时预填依据
watch(
  () => props.visible,
  (v) => {
    if (v && !form.text.trim()) form.text = currentSource();
  },
);

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown, true);
});
</script>

<style scoped>
.ai-write-layer {
  position: fixed;
  z-index: 3000;
  width: 420px;
  max-width: 94vw;
  max-height: 86vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 12px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.22);
  overflow: hidden;
  color: var(--text-color, #1f2937);
}
[data-theme="dark"] .ai-write-layer {
  background: #1e293b;
  border-color: rgba(255, 255, 255, 0.08);
}

.awl-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--bg-soft, #f5f7fa);
  border-bottom: 1px solid var(--border-color, #e5e7eb);
  cursor: grab;
  user-select: none;
}
.awl-head:active { cursor: grabbing; }
.awl-grip { opacity: 0.5; font-size: 0.9rem; }
.awl-title { font-weight: 600; font-size: 0.95rem; }
.awl-live-dot { font-size: 0.72rem; color: var(--el-color-success, #16a34a); }
.awl-spacer { flex: 1; }
.awl-x {
  border: none; background: transparent; cursor: pointer;
  font-size: 1rem; line-height: 1; color: var(--text-muted, #6b7280);
  padding: 2px 6px; border-radius: 6px;
}
.awl-x:hover { background: rgba(0,0,0,0.06); }

.awl-body {
  padding: 12px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.awl-balance {
  display: flex; align-items: center; justify-content: space-between;
  font-size: 0.85rem;
  background: var(--bg-soft, #f5f7fa);
  border-radius: 8px; padding: 6px 10px;
}
.awl-row { display: flex; align-items: center; gap: 10px; }
.awl-row > label { width: 40px; font-size: 0.85rem; color: var(--text-muted, #6b7280); flex-shrink: 0; }
.awl-actions { align-items: flex-start; }
.awl-action-grid { display: flex; flex-wrap: wrap; gap: 6px; flex: 1; }
.awl-action {
  border: 1px solid var(--border-color, #e5e7eb);
  background: var(--bg-card, #fff);
  border-radius: 8px; padding: 5px 10px; font-size: 0.82rem; cursor: pointer;
  color: var(--text-color, #1f2937);
}
.awl-action:hover { border-color: var(--el-color-primary, #409eff); }
.awl-action.active { background: var(--el-color-primary, #409eff); color: #fff; border-color: var(--el-color-primary, #409eff); }
.awl-action.disabled { opacity: 0.5; pointer-events: none; }

.awl-input-label {
  display: flex; align-items: center; justify-content: space-between;
  font-size: 0.85rem; font-weight: 600; margin-top: 2px;
}
.awl-run { width: 100%; }

.awl-live {
  border-top: 1px solid var(--border-color, #e5e7eb);
  padding-top: 10px;
}
.awl-live-label {
  display: flex; align-items: baseline; justify-content: space-between;
  font-weight: 600; font-size: 0.85rem; margin-bottom: 6px;
}
.awl-cost { font-size: 0.72rem; font-weight: 400; color: var(--text-muted, #6b7280); }
.awl-live-text {
  max-height: 240px; overflow: auto; white-space: pre-wrap;
  background: var(--bg-soft, #f8f9fa); border-radius: 8px;
  padding: 10px; font-size: 0.88rem; line-height: 1.7;
}
.awl-live-actions { margin-top: 8px; }
.awl-error { color: #ef4444; font-size: 0.82rem; }
</style>
