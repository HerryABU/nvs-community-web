<template>
  <aside class="aap">
    <!-- ADRO 四阶段（两用方法：创作引导骨架） -->
    <div class="aap-adro">
      <div class="aap-adro__title">ADRO · 创作向微调</div>
      <div class="aap-adro__steps">
        <span v-for="(s, i) in adroSteps" :key="s.key" class="aap-adro__step" :class="{ 'is-on': adroActive >= i }">
          <i>{{ i + 1 }}</i>{{ s.label }}
        </span>
      </div>
      <div class="aap-adro__note">
        引子锚定 → 拆成独立细节 → 对每个细节多轮追问 → 输出改写。
        ADRO 也是苏格拉底式追问与费曼式复述的骨架，本工作台用它的创作面。
      </div>
    </div>

    <!-- 当前微调对象 -->
    <div class="aap-target" v-if="detail">
      <div class="aap-target__label">正在微调的细节</div>
      <div class="aap-target__title">{{ detail.title || '（无标题）' }}</div>
      <div class="aap-target__mod">{{ moduleLabel }} · {{ detail.content.length }} 字</div>
      <div class="aap-target__excerpt">{{ detail.content.slice(0, 90) }}{{ detail.content.length > 90 ? '…' : '' }}</div>
    </div>
    <div class="aap-target is-empty" v-else>
      <div class="aap-target__label">尚未选择细节</div>
      <div class="aap-target__hint">点中间任意一张细节卡的「与 AI 微调」，即可对<b>每一个细节</b>单独追问或改写。</div>
    </div>

    <!-- 模式切换 -->
    <div class="aap-modes">
      <button
        v-for="m in REFINE_MODES"
        :key="m.value"
        class="aap-mode"
        :class="{ 'is-active': mode === m.value }"
        :title="m.hint"
        @click="mode = m.value"
      >
        <span>{{ m.icon }}</span>{{ m.label }}
      </button>
    </div>
    <div class="aap-modehint">{{ currentModeHint }}</div>

    <!-- 多轮对话（Recur） -->
    <div class="aap-chat" ref="chatRef">
      <div v-if="history.length === 0" class="aap-chat__empty">
        <div class="aap-chat__art">💬</div>
        <p>和 AI 一起把这个细节想透</p>
        <span>可直接说「把主角的秘密改得更隐蔽」，或先让 AI 用苏格拉底式追问帮你找漏洞。</span>
      </div>
      <div v-for="(t, i) in history" :key="i" class="aap-msg" :class="t.role">
        <div class="aap-msg__role">{{ t.role === 'user' ? '作者' : 'AI' }}</div>
        <div class="aap-msg__body">{{ t.content }}</div>
        <button
          v-if="t.role === 'assistant' && mode === 'direct' && i === history.length - 1"
          class="aap-msg__apply"
          @click="emit('apply', t.content)"
        >应用为这条细节</button>
      </div>
      <div v-if="loading" class="aap-msg assistant"><div class="aap-msg__role">AI</div><div class="aap-msg__body is-typing">思考中…</div></div>
    </div>

    <!-- 输入区 -->
    <div class="aap-input">
      <el-input
        v-model="instruction"
        type="textarea"
        :rows="3"
        :disabled="!detail || loading"
        :placeholder="inputPlaceholder"
        @keydown.ctrl.enter.prevent="send"
        @keydown.meta.enter.prevent="send"
      />
      <div class="aap-input__foot">
        <label class="aap-apply">
          <input type="checkbox" v-model="autoApply" :disabled="mode !== 'direct'" />
          直接改写后自动应用
        </label>
        <el-button type="primary" size="small" :loading="loading" :disabled="!detail" @click="send">发送 ⌘↵</el-button>
      </div>
    </div>

    <!-- 上下文引用（动态 RAG 轻量视图） -->
    <div class="aap-ctx">
      <div class="aap-ctx__title">本轮上下文引用</div>
      <ul>
        <li>⚓ 引子锚点：{{ seed ? seed.slice(0, 40) + (seed.length > 40 ? '…' : '') : '未设定' }}</li>
        <li>🎨 风格库：{{ styleLabel }}</li>
        <li>📎 当前细节 + 同模块其它卡标题（只取相关切片，避免长篇超 Token）</li>
        <li>🔁 本轮对话 {{ history.length }} 条（Recur 多轮累积）</li>
      </ul>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { REFINE_MODES, type AiWriteDetail, type RefineMode, type RefineTurn, type RefineResult } from '@/api/authorai';

const props = defineProps<{
  detail: AiWriteDetail | null;
  seed: string;
  styleLabel: string;
  refine: (payload: { mode: RefineMode; instruction: string; history: RefineTurn[]; apply: boolean }) => Promise<RefineResult | null>;
}>();
const emit = defineEmits<{ (e: 'apply', content: string): void }>();

const mode = ref<RefineMode>('socratic');
const instruction = ref('');
const autoApply = ref(false);
const loading = ref(false);
const history = ref<RefineTurn[]>([]);
const chatRef = ref<HTMLElement>();

const adroSteps = [
  { key: 'anchor', label: '锚定' },
  { key: 'deconstruct', label: '拆解' },
  { key: 'recur', label: '微调' },
  { key: 'output', label: '输出' },
];
const adroActive = computed(() => (props.detail ? (history.value.length ? 3 : 2) : 1));
const currentModeHint = computed(() => REFINE_MODES.find((m) => m.value === mode.value)?.hint || '');
const inputPlaceholder = computed(() => {
  if (!props.detail) return '请先选择一张细节卡';
  if (mode.value === 'socratic') return '（可选）给 AI 一个追问方向，如「重点问主角的动机」';
  if (mode.value === 'feynman') return '（可选）指定要复述的角度，如「把时间线循环讲清楚」';
  return '说清你想怎么改，如「把主角的秘密改得更隐蔽，别提前暴露」';
});
const moduleLabel = computed(() => {
  const map: Record<string, string> = { setting: '世界观 / 设定', character: '人物', outline: '大纲', timeline: '时间线' };
  return props.detail ? map[props.detail.module] || props.detail.module : '';
});

// 切换微调对象时清空对话，避免串台
watch(() => props.detail?.id, () => { history.value = []; instruction.value = ''; });

async function scrollBottom() {
  await nextTick();
  if (chatRef.value) chatRef.value.scrollTop = chatRef.value.scrollHeight;
}

async function send() {
  if (!props.detail || loading.value) return;
  const text = instruction.value.trim();
  const snapshot = history.value.map((t) => ({ ...t }));
  const userTurn: RefineTurn = { role: 'user', content: text || (mode.value === 'socratic' ? '请针对这个细节追问' : mode.value === 'feynman' ? '请复述这个细节' : '请按上一轮意见改写') };
  history.value.push(userTurn);
  instruction.value = '';
  loading.value = true;
  await scrollBottom();
  try {
    const res = await props.refine({
      mode: mode.value,
      instruction: userTurn.content,
      history: snapshot,
      apply: autoApply.value && mode.value === 'direct',
    });
    if (res?.reply) {
      history.value.push({ role: 'assistant', content: res.reply });
      if (res.applied) ElMessage.success('已改写并应用这条细节');
    } else {
      history.value.push({ role: 'assistant', content: '（AI 未返回内容，请重试）' });
    }
  } catch {
    history.value.push({ role: 'assistant', content: '（调用失败，请检查模型配置后重试）' });
  } finally {
    loading.value = false;
    await scrollBottom();
  }
}

</script>

<style scoped>
.aap { width: 320px; flex-shrink: 0; height: calc(100vh - 120px); min-height: 620px; display: flex; flex-direction: column; gap: 10px; padding: 14px; background: var(--surface-color, var(--bg-card)); border: 1px solid var(--border-color); border-radius: var(--radius-lg); box-shadow: var(--shadow-card); position: sticky; top: 16px; }
.aap-adro { padding: 11px; border-radius: 10px; background: color-mix(in srgb, var(--primary-color) 7%, transparent); border: 1px solid color-mix(in srgb, var(--primary-color) 22%, transparent); }
.aap-adro__title { font-size: 0.78rem; font-weight: 700; color: var(--primary-color); margin-bottom: 8px; }
.aap-adro__steps { display: flex; gap: 4px; }
.aap-adro__step { flex: 1; display: flex; flex-direction: column; align-items: center; gap: 3px; font-size: 0.66rem; color: var(--text-muted); }
.aap-adro__step i { width: 20px; height: 20px; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-style: normal; font-size: 0.66rem; background: var(--bg-soft); color: var(--text-light); }
.aap-adro__step.is-on { color: var(--primary-color); font-weight: 700; }
.aap-adro__step.is-on i { background: var(--gradient-primary); color: #fff; }
.aap-adro__note { font-size: 0.66rem; color: var(--text-light); line-height: 1.55; margin-top: 8px; }
.aap-target { padding: 10px 11px; border-radius: 10px; background: var(--bg-soft); border: 1px solid var(--border-color); }
.aap-target.is-empty { border-style: dashed; }
.aap-target__label { font-size: 0.68rem; font-weight: 700; color: var(--text-muted); text-transform: uppercase; margin-bottom: 5px; }
.aap-target__title { font-weight: 700; font-size: 0.9rem; color: var(--text-color); }
.aap-target__mod { font-size: 0.7rem; color: var(--accent-color); margin-top: 2px; }
.aap-target__excerpt { font-size: 0.74rem; color: var(--text-light); line-height: 1.5; margin-top: 6px; }
.aap-target__hint { font-size: 0.76rem; color: var(--text-light); line-height: 1.6; }
.aap-modes { display: flex; gap: 4px; }
.aap-mode { flex: 1; display: flex; flex-direction: column; align-items: center; gap: 3px; padding: 7px 2px; border: 1px solid var(--border-color); border-radius: 8px; background: var(--bg-card); color: var(--text-light); font-size: 0.68rem; cursor: pointer; transition: all 0.18s; }
.aap-mode span { font-size: 1rem; }
.aap-mode:hover { border-color: var(--primary-color); }
.aap-mode.is-active { border-color: var(--primary-color); background: color-mix(in srgb, var(--primary-color) 10%, transparent); color: var(--primary-color); font-weight: 700; }
.aap-modehint { font-size: 0.68rem; color: var(--text-muted); text-align: center; }
.aap-chat { flex: 1; min-height: 160px; overflow-y: auto; display: flex; flex-direction: column; gap: 8px; padding: 4px 2px; }
.aap-chat__empty { text-align: center; color: var(--text-light); padding: 20px 8px; }
.aap-chat__art { font-size: 1.8rem; margin-bottom: 6px; opacity: 0.7; }
.aap-chat__empty p { margin: 0 0 6px; font-size: 0.82rem; font-weight: 600; color: var(--text-color); }
.aap-chat__empty span { font-size: 0.72rem; line-height: 1.6; }
.aap-msg { max-width: 92%; padding: 8px 10px; border-radius: 10px; font-size: 0.78rem; line-height: 1.6; white-space: pre-wrap; word-break: break-word; }
.aap-msg.user { align-self: flex-end; background: var(--gradient-primary); color: #fff; }
.aap-msg.assistant { align-self: flex-start; background: var(--bg-soft); color: var(--text-color); border: 1px solid var(--border-color); }
.aap-msg__role { font-size: 0.64rem; opacity: 0.7; margin-bottom: 3px; font-weight: 700; }
.aap-msg__body.is-typing { opacity: 0.6; }
.aap-msg__apply { margin-top: 6px; padding: 3px 8px; border: 1px solid var(--success-color); border-radius: 6px; background: transparent; color: var(--success-color); font-size: 0.68rem; cursor: pointer; }
.aap-input { display: flex; flex-direction: column; gap: 6px; }
.aap-input__foot { display: flex; align-items: center; justify-content: space-between; gap: 6px; }
.aap-apply { display: flex; align-items: center; gap: 4px; font-size: 0.68rem; color: var(--text-light); cursor: pointer; }
.aap-ctx { padding: 9px 10px; border-radius: 9px; background: var(--bg-soft); border: 1px solid var(--border-color); }
.aap-ctx__title { font-size: 0.68rem; font-weight: 700; color: var(--text-muted); margin-bottom: 5px; }
.aap-ctx ul { margin: 0; padding-left: 14px; }
.aap-ctx li { font-size: 0.68rem; color: var(--text-light); line-height: 1.7; }
@media (max-width: 1280px) { .aap { width: 280px; } }
</style>
