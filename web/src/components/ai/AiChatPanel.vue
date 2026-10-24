<template>
  <!-- AI 对话面板（§5.4）：流式输出 / 停止生成 / Markdown 渲染 / 消耗展示 -->
  <div class="ai-chat">
    <!-- 顶部：只留上下文操作。模型/长度/发送都移到输入区，
         顶部不再被一排控件占满（这是标准 AI 站的做法） -->
    <div class="ai-chat-topbar">
      <el-button size="small" text @click="promptOpen = true">
        <el-icon><Collection /></el-icon> 提示词库
      </el-button>
      <el-tooltip content="使用「我的钱包 → 上游密钥」中配置的密钥，不消耗平台代币" placement="bottom">
        <el-checkbox v-model="useByok" size="small" :disabled="generating">我的密钥</el-checkbox>
      </el-tooltip>
      <el-input
        v-if="useByok"
        v-model="byokModel"
        size="small"
        style="width:160px"
        placeholder="上游模型 ID"
        :disabled="generating"
      />
      <span class="ai-chat-balance">余额 <b>{{ status.balance }}</b> {{ status.token_name }}</span>
    </div>

    <!-- 消息列表 -->
    <div ref="msgListEl" class="ai-chat-messages">
      <!-- 空态：大标题 + 推荐问题（标准 AI 站首屏，而不是一句「输入内容开始对话」） -->
      <div v-if="messages.length === 0 && !streamingText" class="ai-welcome">
        <div class="ai-welcome-emoji">{{ currentAgentEmoji }}</div>
        <h1 class="ai-welcome-title">{{ currentAgentName }}</h1>
        <p class="ai-welcome-hint">{{ currentAgentGreeting || '今天想聊点什么？' }}</p>

        <div class="ai-preset-grid">
          <button
            v-for="(p, pi) in presetPrompts"
            :key="pi"
            type="button"
            class="ai-preset"
            @click="usePrompt(p.text)"
          >
            <span class="ai-preset-emoji">{{ p.emoji }}</span>
            <span class="ai-preset-body">
              <b>{{ p.title }}</b>
              <span>{{ p.text }}</span>
            </span>
          </button>
        </div>
      </div>
      <div
        v-for="m in messages"
        :key="m.id"
        :class="['ai-msg', m.role === 'user' ? 'ai-msg-user' : 'ai-msg-assistant']"
      >
        <div class="ai-msg-bubble">
          <div v-if="m.role === 'assistant'" class="ai-msg-md" v-html="renderMd(m.content)" />
          <div v-else class="ai-msg-plain">{{ m.content }}</div>
          <div v-if="m.role === 'assistant'" class="ai-msg-tools">
            <button class="ai-msg-tool" type="button" title="复制内容" @click="copyMsg(m.content)">
              <el-icon><CopyDocument /></el-icon> 复制
            </button>
            <button class="ai-msg-tool" type="button" title="重新发送" @click="regenerate(m)">
              <el-icon><RefreshRight /></el-icon> 重试
            </button>
            <span v-if="m.cost > 0 || m.free" class="ai-msg-meta">
              <span v-if="m.free" class="ai-msg-free">免费额度</span>
              <template v-else>消耗 {{ m.cost }} {{ status.token_name }}</template>
            </span>
          </div>
        </div>
      </div>
      <!-- 流式中的消息（同样走严格 Markdown 渲染） -->
      <div v-if="streamingText" class="ai-msg ai-msg-assistant">
        <div class="ai-msg-bubble">
          <div class="ai-msg-md" v-html="streamingHtml" />
          <span class="ai-cursor" />
        </div>
      </div>
    </div>

    <!-- 输入区：圆角大输入框 + 底部工具行（模型 / 长度 / 发送） -->
    <div class="ai-composer">
      <div class="ai-composer-box" :class="{ 'is-focus': composerFocus, 'is-generating': generating }">
        <textarea
          ref="composerEl"
          v-model="inputText"
          class="ai-composer-input"
          rows="1"
          :disabled="generating"
          placeholder="给 AI 发消息…"
          @focus="composerFocus = true"
          @blur="composerFocus = false"
          @input="autoGrow"
          @keydown.enter.exact.prevent="send"
        />
        <div class="ai-composer-tools">
          <div class="ai-composer-left">
            <el-select
              v-if="vendors.length > 1"
              v-model="vendor"
              size="small"
              style="width:104px"
              :disabled="generating || useByok"
            >
              <el-option label="全部厂商" value="" />
              <el-option v-for="v in vendors" :key="v" :label="v" :value="v" />
            </el-select>
            <el-select v-model="modelId" size="small" style="width:168px" :disabled="generating || useByok">
              <el-option v-for="m in filteredModels" :key="m.id" :label="modelLabel(m)" :value="m.id" />
            </el-select>
            <el-select v-model="maxTokens" size="small" style="width:100px" :disabled="generating">
              <el-option :value="1024" label="短回复" />
              <el-option :value="2048" label="中回复" />
              <el-option :value="4096" label="长回复" />
            </el-select>
          </div>
          <div class="ai-composer-right">
            <button v-if="generating" type="button" class="ai-send-btn is-stop" title="停止生成" @click="stop">
              <el-icon><VideoPause /></el-icon>
            </button>
            <button
              v-else
              type="button"
              class="ai-send-btn"
              :disabled="!inputText.trim()"
              title="发送（Enter）"
              @click="send"
            >
              <el-icon><Top /></el-icon>
            </button>
          </div>
        </div>
      </div>
      <p class="ai-composer-tip">Enter 发送 · Shift+Enter 换行 · 停止生成前的内容正常计费</p>
    </div>

    <AiPromptLibrary
      v-model="promptOpen"
      kind="chat"
      :draft="inputText"
      @pick="onPickPrompt"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { CopyDocument, RefreshRight, Collection, Top, VideoPause } from '@element-plus/icons-vue';
import { renderAiMarkdown } from '@/markdown/renderer';
import AiPromptLibrary from '@/components/ai/AiPromptLibrary.vue';
import { streamChat, type AiHubStatus, type AiAgent, type AiChatMessage, type AiChatSession } from '@/api/aiHub';

const props = defineProps<{
  status: AiHubStatus;
  agents: AiAgent[];
  sessionId: number | null;
  agentId: number;
  /** 从「大模型广场」带入的模型 id（0 = 用默认第一个） */
  presetModelId?: number;
}>();
const emit = defineEmits<{
  (e: 'session-created', id: number): void;
  (e: 'balance-updated', balance: number): void;
  (e: 'sessions-changed'): void;
}>();

const messages = ref<AiChatMessage[]>([]);
const inputText = ref('');
const generating = ref(false);
const streamingText = ref('');
const msgListEl = ref<HTMLElement>();
const maxTokens = ref(2048);
let abortCtrl: AbortController | null = null;
let loadingSession = 0;

const modelId = ref(0);
const vendor = ref(''); // 厂商筛选（空=全部）
// 厂商维度：只列出现过的厂商，避免空选项
const vendors = computed(() => {
  const set = new Set<string>();
  for (const m of props.status.chat_models) {
    const v = (m.vendor || '').trim();
    if (v) set.add(v);
  }
  return Array.from(set);
});
const filteredModels = computed(() => {
  if (!vendor.value) return props.status.chat_models;
  return props.status.chat_models.filter((m) => (m.vendor || '').trim() === vendor.value);
});
// 切换厂商后若当前模型不在结果里，自动选第一个
watch([vendor, filteredModels], () => {
  const list = filteredModels.value;
  if (!list.length) return;
  if (!list.some((m) => m.id === modelId.value)) modelId.value = list[0].id;
}, { immediate: true });
const useByok = ref(false);
const byokModel = ref('');
// 提示词库
const promptOpen = ref(false);
function onPickPrompt(content: string) {
  inputText.value = inputText.value.trim() ? `${inputText.value}\n\n${content}` : content;
}
watch(
  () => props.status.chat_models,
  (list) => {
    if (!list.length) return;
    // 广场带入的模型优先（用户是从模型卡片点进来的，不要给他换成默认第一个）；下架则回落默认
    if (props.presetModelId && list.some((m) => m.id === props.presetModelId)) {
      if (modelId.value !== props.presetModelId) modelId.value = props.presetModelId;
      return;
    }
    if (!modelId.value) modelId.value = (filteredModels.value[0] || list[0]).id;
  },
  { immediate: true },
);

const currentAgent = computed(() => props.agents.find((a) => a.id === props.agentId));

// ---------- 空态推荐提示词 ----------
// 不用行业模板话术，全部贴着本站的写作/阅读场景，点一下直接进输入框（还能自己改）
const presetPrompts = [
  { emoji: '✍️', title: '续写一段', text: '帮我接着往下写 300 字，保持第一人称和现有的语气，不要收尾。' },
  { emoji: '🎭', title: '人物小传', text: '给我设计一个配角的人设：姓名、年龄、外貌特征、说话习惯、一个不愿提起的过去。' },
  { emoji: '⚡', title: '制造冲突', text: '当前剧情太平淡，给我三个能立刻提升张力的冲突方案，并说明各自的代价。' },
  { emoji: '🔍', title: '挑毛病', text: '把下面这段文字按「逻辑漏洞 / 冗余表达 / 视角混乱」三类问题逐条挑出来：\n\n' },
];

function usePrompt(text: string) {
  inputText.value = text;
  nextTick(() => {
    composerEl.value?.focus();
    autoGrow();
  });
}

// ---------- 输入框自适应高度 ----------
const composerEl = ref<HTMLTextAreaElement>();
const composerFocus = ref(false);
function autoGrow() {
  const el = composerEl.value;
  if (!el) return;
  el.style.height = 'auto';
  el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
}
const currentAgentName = computed(() => currentAgent.value?.name || 'AI 对话');
const currentAgentEmoji = computed(() => currentAgent.value?.emoji || '💬');
const currentAgentGreeting = computed(() => currentAgent.value?.greeting || '');

function modelLabel(m: { name: string; price_in: number; price_out: number }) {
  const price = m.price_in === 0 && m.price_out === 0 ? '免费' : `${m.price_in}/${m.price_out}`;
  return `${m.name}（${price}）`;
}

function renderMd(md: string) {
  // AI 生成内容按不可信输入处理：严格白名单 + 全量转义兜底
  try {
    return renderAiMarkdown(md);
  } catch {
    return md.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }
}

// 流式渲染节流：每 120ms 重算一次 Markdown，避免每个 chunk 全量渲染造成卡顿
const streamingHtml = ref('');
let streamTimer: number | null = null;
watch(streamingText, () => {
  if (streamTimer) return;
  streamTimer = window.setTimeout(() => {
    streamTimer = null;
    streamingHtml.value = renderMd(streamingText.value);
  }, 120);
});

async function copyMsg(text: string) {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      const ta = document.createElement('textarea');
      ta.value = text;
      ta.style.position = 'fixed';
      ta.style.left = '-9999px';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      ta.remove();
    }
    ElMessage.success('已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择文本');
  }
}

// 重试：找到该助手消息之前最近的一条用户消息，重新发起
function regenerate(m: AiChatMessage) {
  if (generating.value) return;
  const idx = messages.value.findIndex((x) => x.id === m.id);
  for (let i = idx - 1; i >= 0; i--) {
    if (messages.value[i].role === 'user') {
      send(messages.value[i].content);
      return;
    }
  }
}

function scrollBottom() {
  nextTick(() => {
    if (msgListEl.value) msgListEl.value.scrollTop = msgListEl.value.scrollHeight;
  });
}

// 切换会话：加载历史
watch(
  () => props.sessionId,
  async (id) => {
    if (!id) {
      messages.value = [];
      return;
    }
    const token = ++loadingSession;
    try {
      const { aiHubApi } = await import('@/api/aiHub');
      const res = await aiHubApi.sessionMessages(id);
      if (token !== loadingSession) return; // 防竞态
      if (res.data.code === 0) messages.value = res.data.data.messages || [];
    } catch { /* ignore */ }
    scrollBottom();
  },
  { immediate: true },
);

async function send(rawText?: string, confirmed = false) {
  const isRetry = typeof rawText === 'string';
  const text = (isRetry ? rawText : inputText.value).trim();
  if (!text || generating.value) return;
  if (!useByok.value && !modelId.value) {
    ElMessage.warning('暂无可用模型');
    return;
  }
  if (useByok.value && !byokModel.value.trim()) {
    ElMessage.warning('使用我的密钥时需填写上游模型 ID');
    return;
  }
  if (!props.status.token_enabled && !useByok.value) {
    ElMessage.warning('代币系统未开启，可使用「我的密钥」或联系站长');
    return;
  }
  if (!isRetry) inputText.value = '';
  // 乐观上屏
  messages.value.push({
    id: -Date.now(), role: 'user', content: text,
    tokens_in: 0, tokens_out: 0, cost: 0, created_at: '',
  });
  generating.value = true;
  streamingText.value = '';
  streamingHtml.value = '';
  scrollBottom();
  abortCtrl = new AbortController();
  try {
    const result = await streamChat(
      {
        session_id: props.sessionId || undefined,
        model_id: modelId.value,
        agent_id: props.agentId || undefined,
        message: text,
        max_tokens: maxTokens.value,
        byok: useByok.value || undefined,
        byok_model: useByok.value ? byokModel.value.trim() : undefined,
        confirm: confirmed || undefined,
      },
      (chunk) => {
        streamingText.value += chunk;
        scrollBottom();
      },
      abortCtrl?.signal,
    );
    if (!result) {
      ElMessage.warning('连接中断，未生成完整内容');
    } else {
      messages.value.push({
        id: -Date.now() - 1, role: 'assistant', content: result.content,
        tokens_in: 0, tokens_out: 0, cost: result.cost, free: result.free, created_at: '',
      });
      emit('balance-updated', result.balance_after);
      if (!props.sessionId && result.session_id) {
        emit('session-created', result.session_id);
      }
      emit('sessions-changed');
    }
  } catch (e: any) {
    // 42801：预估超阈值，需二次确认（后端未扣款）
    if (e?.code === 42801 && !confirmed) {
      try {
        await ElMessageBox.confirm(String(e.message || '本次调用消耗较高，确认继续？'), '确认消耗', {
          type: 'warning',
          confirmButtonText: '确认并继续',
          cancelButtonText: '取消',
        });
        generating.value = false;
        if (streamTimer) { window.clearTimeout(streamTimer); streamTimer = null; }
        streamingText.value = '';
        await send(text, true);
        return;
      } catch {
        ElMessage.info('已取消');
      }
    }
    if (e?.name === 'AbortError') {
      // 用户主动停止：保留已流式内容为一条助手消息
      if (streamingText.value) {
        messages.value.push({
          id: -Date.now() - 2, role: 'assistant', content: streamingText.value,
          tokens_in: 0, tokens_out: 0, cost: 0, created_at: '',
        });
      }
      ElMessage.info('已停止生成');
    } else {
      ElMessage.error(e?.message || '调用失败');
    }
  } finally {
    streamingText.value = '';
    generating.value = false;
    abortCtrl = null;
    scrollBottom();
  }
}

function stop() {
  abortCtrl?.abort();
}
</script>

<style scoped>
.ai-chat { display: flex; flex-direction: column; height: 100%; min-height: 0; }
.ai-chat-topbar {
  display: flex; align-items: center; gap: 12px;
  padding: 10px 16px; border-bottom: 1px solid var(--el-border-color, #ebeef5);
}
.ai-chat-balance { margin-left: auto; font-size: 0.82rem; color: var(--text-light, #666); }
.ai-chat-balance b { color: var(--el-color-primary, #409eff); }
.ai-chat-messages { flex: 1; overflow-y: auto; padding: 16px; min-height: 0; }
/* ===== 空态：问候 + 推荐词（标准 AI 站首屏）===== */
.ai-welcome {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: var(--space-7, 48px) var(--space-4, 16px) var(--space-5, 24px);
  min-height: 60%;
}
.ai-welcome-emoji { font-size: 40px; line-height: 1; margin-bottom: var(--space-3, 12px); }
.ai-welcome-title {
  margin: 0 0 var(--space-2, 8px);
  font-size: var(--text-2xl, 28px);
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--text-color, #1f2328);
}
.ai-welcome-hint {
  margin: 0 0 var(--space-5, 24px);
  font-size: var(--text-base, 14px);
  color: var(--text-light, #6b7280);
  max-width: 560px;
  line-height: 1.6;
}
.ai-preset-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: var(--space-3, 12px);
  width: 100%;
  max-width: 720px;
}
.ai-preset {
  display: flex;
  gap: 10px;
  padding: var(--space-3, 12px);
  text-align: left;
  cursor: pointer;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-md, 10px);
  font-family: inherit;
  transition: border-color 0.15s ease, transform 0.15s ease, box-shadow 0.15s ease;
}
.ai-preset:hover {
  transform: translateY(-1px);
  border-color: var(--el-color-primary, #6366f1);
  box-shadow: var(--shadow-sm, 0 4px 12px rgba(0, 0, 0, 0.06));
}
.ai-preset-emoji { font-size: 16px; line-height: 1.4; flex-shrink: 0; }
.ai-preset-body { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.ai-preset-body b { font-size: var(--text-sm, 13px); color: var(--text-color, #1f2328); }
.ai-preset-body span {
  font-size: var(--text-xs, 12px);
  line-height: 1.5;
  color: var(--text-light, #6b7280);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.ai-msg { display: flex; margin-bottom: 14px; }
.ai-msg-user { justify-content: flex-end; }
.ai-msg-bubble {
  max-width: 78%; padding: 10px 14px; border-radius: 12px;
  font-size: 0.9rem; line-height: 1.7; word-break: break-word;
}
.ai-msg-user .ai-msg-bubble {
  background: var(--el-color-primary, #409eff); color: #fff;
  border-bottom-right-radius: 4px;
}
.ai-msg-assistant .ai-msg-bubble {
  background: var(--el-fill-color-light, #f5f7fa); color: var(--text-color, #333);
  border-bottom-left-radius: 4px;
}
.ai-msg-plain { white-space: pre-wrap; }
.ai-msg-md :deep(p) { margin: 0 0 8px; }
.ai-msg-md :deep(p:last-child) { margin-bottom: 0; }
.ai-msg-md :deep(pre) {
  background: #1e293b; color: #e2e8f0; padding: 10px 12px;
  border-radius: 8px; overflow-x: auto; font-size: 0.82rem;
}
.ai-msg-md :deep(code) { font-family: Consolas, monospace; }
.ai-msg-md :deep(ul), .ai-msg-md :deep(ol) { margin: 4px 0 8px; padding-left: 20px; }
.ai-msg-meta { font-size: 0.72rem; color: var(--text-light, #999); }
.ai-msg-tools {
  display: flex; align-items: center; gap: 10px; flex-wrap: wrap;
  margin-top: 6px; padding-top: 6px;
  border-top: 1px dashed var(--el-border-color-lighter, #e5e7eb);
}
.ai-msg-tool {
  display: inline-flex; align-items: center; gap: 3px;
  border: 0; background: transparent; padding: 0; cursor: pointer;
  font-size: 0.72rem; color: var(--el-color-primary, #409eff);
}
.ai-msg-tool:hover { text-decoration: underline; }
.ai-msg-free {
  display: inline-block; padding: 0 6px; margin-right: 4px; border-radius: 4px;
  background: rgba(22, 163, 74, 0.12); color: #16a34a;
}
.ai-cursor {
  display: inline-block; width: 8px; height: 1em;
  background: var(--el-color-primary, #409eff);
  vertical-align: text-bottom; animation: ai-blink 0.8s infinite;
}
@keyframes ai-blink { 0%, 100% { opacity: 1; } 50% { opacity: 0; } }
/* ===== 输入区：圆角大输入框 + 内嵌工具行 ===== */
.ai-composer {
  padding: var(--space-3, 12px) var(--space-4, 16px) var(--space-3, 12px);
  border-top: 1px solid var(--border-color, #ebeef5);
  background: var(--bg-card, #fff);
}
.ai-composer-box {
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-lg, 16px);
  background: var(--bg-card, #fff);
  box-shadow: var(--shadow-xs, 0 1px 3px rgba(0, 0, 0, 0.04));
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
  max-width: 900px;
  margin: 0 auto;
}
.ai-composer-box.is-focus {
  border-color: var(--el-color-primary, #6366f1);
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.12);
}
.ai-composer-input {
  display: block;
  width: 100%;
  box-sizing: border-box;
  border: none;
  outline: none;
  resize: none;
  background: transparent;
  padding: var(--space-3, 12px) var(--space-3, 12px) 4px;
  font-family: inherit;
  font-size: var(--text-base, 14px);
  line-height: 1.6;
  color: var(--text-color, #1f2328);
  max-height: 200px;
  overflow-y: auto;
}
.ai-composer-input::placeholder { color: var(--text-light, #9ca3af); }
.ai-composer-tools {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2, 8px);
  padding: 4px var(--space-2, 8px) var(--space-2, 8px);
}
.ai-composer-left { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; min-width: 0; }
.ai-composer-right { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
.ai-send-btn {
  width: 34px;
  height: 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 50%;
  cursor: pointer;
  color: #fff;
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.ai-send-btn:hover:not(:disabled) { transform: scale(1.05); }
.ai-send-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.ai-send-btn.is-stop { background: #ef4444; }
.ai-composer-tip {
  margin: 6px auto 0;
  max-width: 900px;
  text-align: center;
  font-size: 11px;
  color: var(--text-light, #9ca3af);
}
.muted { color: var(--text-light, #999); }
.small { font-size: 0.75rem; }
[data-theme="dark"] .ai-msg-assistant .ai-msg-bubble { background: #1e293b; color: #e2e8f0; }
</style>
