<template>
  <!-- AI 对话面板（§5.4）：流式输出 / 停止生成 / Markdown 渲染 / 消耗展示 -->
  <div class="ai-chat">
    <!-- 顶部：模型 + 长度 + 余额 -->
    <div class="ai-chat-topbar">
      <el-select v-model="modelId" size="small" style="width:200px" :disabled="generating">
        <el-option v-for="m in status.chat_models" :key="m.id" :label="modelLabel(m)" :value="m.id" />
      </el-select>
      <el-radio-group v-model="maxTokens" size="small" :disabled="generating">
        <el-radio-button :value="1024">短</el-radio-button>
        <el-radio-button :value="2048">中</el-radio-button>
        <el-radio-button :value="4096">长</el-radio-button>
      </el-radio-group>
      <span class="ai-chat-balance">余额 <b>{{ status.balance }}</b> {{ status.token_name }}</span>
    </div>

    <!-- 消息列表 -->
    <div ref="msgListEl" class="ai-chat-messages">
      <div v-if="messages.length === 0 && !streamingText" class="ai-chat-welcome">
        <div class="ai-chat-welcome-emoji">{{ currentAgentEmoji }}</div>
        <div class="ai-chat-welcome-title">{{ currentAgentName }}</div>
        <div class="ai-chat-welcome-hint">{{ currentAgentGreeting || '输入内容开始对话' }}</div>
      </div>
      <div
        v-for="m in messages"
        :key="m.id"
        :class="['ai-msg', m.role === 'user' ? 'ai-msg-user' : 'ai-msg-assistant']"
      >
        <div class="ai-msg-bubble">
          <div v-if="m.role === 'assistant'" class="ai-msg-md" v-html="renderMd(m.content)" />
          <div v-else class="ai-msg-plain">{{ m.content }}</div>
          <div v-if="m.role === 'assistant' && (m.cost > 0 || m.free)" class="ai-msg-meta">
            <span v-if="m.free" class="ai-msg-free">免费额度</span>
            <template v-else>消耗 {{ m.cost }} {{ status.token_name }} · </template>
            {{ m.tokens_in }}/{{ m.tokens_out }} tokens
          </div>
        </div>
      </div>
      <!-- 流式中的消息 -->
      <div v-if="streamingText" class="ai-msg ai-msg-assistant">
        <div class="ai-msg-bubble">
          <div class="ai-msg-plain ai-streaming">{{ streamingText }}<span class="ai-cursor" /></div>
        </div>
      </div>
    </div>

    <!-- 输入区 -->
    <div class="ai-chat-input">
      <el-input
        v-model="inputText"
        type="textarea"
        :rows="3"
        resize="none"
        placeholder="输入消息，Enter 发送，Shift+Enter 换行"
        :disabled="generating"
        @keydown.enter.exact.prevent="send"
      />
      <div class="ai-chat-actions">
        <span class="muted small">按量计费，停止生成前的内容正常计费</span>
        <el-button v-if="generating" type="danger" plain @click="stop">停止生成</el-button>
        <el-button v-else type="primary" :disabled="!inputText.trim()" @click="send">发送</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue';
import { ElMessage } from 'element-plus';
import { renderMarkdown } from '@/markdown/renderer';
import { streamChat, type AiHubStatus, type AiAgent, type AiChatMessage, type AiChatSession } from '@/api/aiHub';

const props = defineProps<{
  status: AiHubStatus;
  agents: AiAgent[];
  sessionId: number | null;
  agentId: number;
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
watch(
  () => props.status.chat_models,
  (list) => {
    if (!modelId.value && list.length) modelId.value = list[0].id;
  },
  { immediate: true },
);

const currentAgent = computed(() => props.agents.find((a) => a.id === props.agentId));
const currentAgentName = computed(() => currentAgent.value?.name || 'AI 对话');
const currentAgentEmoji = computed(() => currentAgent.value?.emoji || '💬');
const currentAgentGreeting = computed(() => currentAgent.value?.greeting || '');

function modelLabel(m: { name: string; price_in: number; price_out: number }) {
  const price = m.price_in === 0 && m.price_out === 0 ? '免费' : `${m.price_in}/${m.price_out}`;
  return `${m.name}（${price}）`;
}

function renderMd(md: string) {
  try {
    return renderMarkdown(md);
  } catch {
    return md.replace(/</g, '&lt;');
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

async function send() {
  const text = inputText.value.trim();
  if (!text || generating.value) return;
  if (!modelId.value) {
    ElMessage.warning('暂无可用模型');
    return;
  }
  if (!props.status.token_enabled) {
    ElMessage.warning('代币系统未开启');
    return;
  }
  inputText.value = '';
  // 乐观上屏
  messages.value.push({
    id: -Date.now(), role: 'user', content: text,
    tokens_in: 0, tokens_out: 0, cost: 0, created_at: '',
  });
  generating.value = true;
  streamingText.value = '';
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
.ai-chat-welcome { text-align: center; padding: 60px 0 30px; color: var(--text-light, #888); }
.ai-chat-welcome-emoji { font-size: 3rem; }
.ai-chat-welcome-title { font-size: 1.1rem; font-weight: 600; color: var(--text-color, #333); margin: 10px 0 6px; }
.ai-chat-welcome-hint { font-size: 0.85rem; }
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
.ai-msg-meta { margin-top: 6px; font-size: 0.72rem; color: var(--text-light, #999); }
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
.ai-chat-input { border-top: 1px solid var(--el-border-color, #ebeef5); padding: 12px 16px; }
.ai-chat-actions { display: flex; align-items: center; justify-content: space-between; margin-top: 8px; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.75rem; }
[data-theme="dark"] .ai-msg-assistant .ai-msg-bubble { background: #1e293b; color: #e2e8f0; }
</style>
