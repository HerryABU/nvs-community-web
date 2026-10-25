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
          <!-- 被安全闸拦截的回答：渲染成警告块，不当作普通回答展示 -->
          <div
            v-if="m.role === 'assistant' && m.safety_blocked"
            class="ai-blocked-card"
            :class="{ 'is-strict': sessionLocked }"
          >
            <div class="ai-blocked-title">
              <el-icon><CircleCloseFilled /></el-icon>
              <span>{{ sessionLocked ? '该回答涉嫌违规，已被安全系统拦截' : '该回答未通过安全审核，已拦截' }}</span>
            </div>
            <p class="ai-blocked-text">{{ m.content }}</p>
          </div>
          <template v-else-if="m.role === 'assistant'">
            <!-- 思考型模型的推理过程：默认折叠，不干扰阅读 -->
            <details v-if="m.reasoning" class="ai-think">
              <summary>思考过程（{{ m.reasoning.length }} 字）</summary>
              <div class="ai-think-body">{{ m.reasoning }}</div>
            </details>
            <div class="ai-msg-md" v-html="renderMd(m.content)" @click="onMdClick" />
          </template>
          <template v-else>
            <div class="ai-msg-plain">{{ m.content }}</div>
            <div v-if="(m.sibling_count || 0) > 1" class="ai-branch ai-branch-user">
              <button
                class="ai-msg-tool"
                type="button"
                :disabled="(m.sibling_index || 1) <= 1"
                @click="switchSibling(m, -1)"
              >‹</button>
              <span class="ai-branch-idx">{{ m.sibling_index }}/{{ m.sibling_count }}</span>
              <button
                class="ai-msg-tool"
                type="button"
                :disabled="(m.sibling_index || 1) >= (m.sibling_count || 1)"
                @click="switchSibling(m, 1)"
              >›</button>
            </div>
          </template>
          <div v-if="m.role === 'assistant' && !m.safety_blocked" class="ai-msg-tools">
            <button class="ai-msg-tool" type="button" title="复制内容" @click="copyMsg(m.content)">
              <el-icon><CopyDocument /></el-icon> 复制
            </button>
            <button class="ai-msg-tool" type="button" title="重新生成（会产生分支，可左右切换）" @click="regenerate(m)">
              <el-icon><RefreshRight /></el-icon> 重新生成
            </button>
            <span v-if="(m.sibling_count || 0) > 1" class="ai-branch">
              <button
                class="ai-msg-tool"
                type="button"
                :disabled="(m.sibling_index || 1) <= 1"
                title="上一个分支"
                @click="switchSibling(m, -1)"
              >‹</button>
              <span class="ai-branch-idx">{{ m.sibling_index }}/{{ m.sibling_count }}</span>
              <button
                class="ai-msg-tool"
                type="button"
                :disabled="(m.sibling_index || 1) >= (m.sibling_count || 1)"
                title="下一个分支"
                @click="switchSibling(m, 1)"
              >›</button>
            </span>
            <span v-if="m.cost > 0 || m.free" class="ai-msg-meta">
              <span v-if="m.free" class="ai-msg-free">免费额度</span>
              <template v-else>消耗 {{ m.cost }} {{ status.token_name }}</template>
            </span>
          </div>
        </div>
      </div>
      <!-- 流式中的消息（同样走严格 Markdown 渲染） -->
      <div v-if="streamingText || streamingReasoning" class="ai-msg ai-msg-assistant">
        <div class="ai-msg-bubble">
          <details v-if="streamingReasoning" class="ai-think" open>
            <summary>思考过程{{ streamingText ? '' : '（正在思考…）' }}</summary>
            <div class="ai-think-body">{{ streamingReasoning }}</div>
          </details>
          <div v-if="streamingText" class="ai-msg-md" v-html="streamingHtml" @click="onMdClick" />
          <span v-if="!streamingText" class="ai-think-hint">模型正在推理，正文稍后出现…</span>
          <span class="ai-cursor" />
        </div>
      </div>
    </div>

    <!-- 内容安全：严格模式锁死整个对话；中档模式只标记「上下文不可信」，仍可继续聊别的 -->
    <div
      v-if="sessionLocked || contaminated"
      class="ai-locked-banner"
      :class="{ 'is-moderate': !sessionLocked }"
    >
      <el-icon><Lock /></el-icon>
      <div class="ai-locked-body">
        <strong>{{ sessionLocked ? '本次对话已终止' : '对话已标记为「上下文不可信」' }}</strong>
        <p v-if="sessionLocked">
          {{ sessionLockReason || '检测到危险内容的生成或索取，这段上下文已不可继续使用。' }}
        </p>
        <p v-else>
          前面的回答已被拦截并标记为不可信。模型不会再延续、补全，也不会因为你换个说法就恢复那个话题；
          其他话题可以照常聊。
        </p>
        <p class="ai-locked-hint">
          {{ sessionLocked
            ? '如认为属于误判，可以申诉；管理员复核后可恢复该会话。'
            : '提示：同一个话题反复试探会被升级为终止整个对话。' }}
        </p>
        <el-button v-if="sessionLocked" size="small" class="ai-appeal-btn" @click="onAppeal">
          申诉 / 反馈
        </el-button>
      </div>
    </div>

    <!-- 输入区：圆角大输入框 + 底部工具行（模型 / 长度 / 发送） -->
    <div class="ai-composer">
      <!-- 中档模式：对话还能继续，但要让用户知道审查已收紧，而不是莫名其妙被拦 -->
      <div v-if="contaminated && !sessionLocked" class="ai-safety-hint">
        <el-icon><WarningFilled /></el-icon>
        <span>安全提示：由于上下文存在敏感讨论，本对话已切换至严格审查模式，请文明交流。</span>
      </div>
      <div class="ai-composer-box" :class="{ 'is-focus': composerFocus, 'is-generating': generating }">
        <!-- 附件预览：图片出缩略图，文档出文件名 + 大小 -->
        <div v-if="attachments.length" class="ai-attach-list">
          <div v-for="(a, i) in attachments" :key="a.url" class="ai-attach-item" :title="a.name">
            <img v-if="a.kind === 'image'" :src="a.url" class="ai-attach-thumb" :alt="a.name" />
            <span v-else class="ai-attach-file">
              <span class="ai-attach-ext">{{ (a.name.split('.').pop() || 'file').toUpperCase() }}</span>
              <span class="ai-attach-name">{{ a.name }}</span>
              <span class="ai-attach-size">{{ fmtSize(a.size) }}</span>
            </span>
            <button type="button" class="ai-attach-del" title="移除" @click="removeAttachment(i)">
              <el-icon><Close /></el-icon>
            </button>
          </div>
        </div>
        <textarea
          ref="composerEl"
          v-model="inputText"
          class="ai-composer-input"
          rows="1"
          :disabled="generating || sessionLocked"
          :placeholder="sessionLocked ? '本对话已被终止，请新建对话并遵守社区规范' : '给 AI 发消息…'"
          @focus="composerFocus = true"
          @blur="composerFocus = false"
          @input="autoGrow"
          @keydown.enter.exact.prevent="send()"
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
            <el-tag v-if="selectedThinking" size="small" type="warning" effect="plain" title="先输出推理过程，再给正文；思考过程可折叠">
              思考型
            </el-tag>
            <el-select v-model="maxTokens" size="small" style="width:100px" :disabled="generating">
              <el-option :value="1024" label="短回复" />
              <el-option :value="2048" label="中回复" />
              <el-option :value="4096" label="长回复" />
            </el-select>
          </div>
          <div class="ai-composer-right">
            <input
              ref="fileInputRef"
              type="file"
              multiple
              :accept="ATTACH_ACCEPT"
              style="display: none"
              @change="onFilesPicked"
            />
            <button
              type="button"
              class="ai-attach-btn"
              :disabled="generating || uploading"
              :title="uploading ? '上传中…' : '上传图片 / 文档（PDF、Word、文本等）'"
              @click="pickFiles"
            >
              <el-icon><Paperclip /></el-icon>
            </button>
            <button v-if="generating" type="button" class="ai-send-btn is-stop" title="停止生成" @click="stop">
              <el-icon><VideoPause /></el-icon>
            </button>
            <button
              v-else
              type="button"
              class="ai-send-btn"
              :disabled="(!inputText.trim() && !attachments.length) || uploading"
              title="发送（Enter）"
              @click="send"
            >
              <el-icon><Top /></el-icon>
            </button>
          </div>
        </div>
      </div>
      <p class="ai-composer-tip">
        Enter 发送 · Shift+Enter 换行 · 停止生成前的内容正常计费
        <span v-if="attachments.length" class="ai-composer-tip-attach">
          · 附件按原样随 API 发给模型，需模型本身支持图片 / 文档输入
        </span>
      </p>
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
import { CopyDocument, RefreshRight, Collection, Top, VideoPause, Paperclip, Close } from '@element-plus/icons-vue';
import { renderAiMarkdown } from '@/markdown/renderer';
import AiPromptLibrary from '@/components/ai/AiPromptLibrary.vue';
import { aiHubApi, streamChat, uploadChatAttachment, type AiHubStatus, type AiAgent, type AiChatMessage, type AiChatSession, type ChatAttachment } from '@/api/aiHub';

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
// 会话被内容安全终止：整段上下文不可用，输入区禁用，只能新建对话
const sessionLocked = ref(false);
const sessionLockReason = ref('');
// 中档模式：会话可继续，但上下文被标记为不可信（每轮注入提示词）
const contaminated = ref(false);

// ===== 附件（图片 / 文档）=====
// 平台不解析文件：上传后拿到引用，发送时后端原样内联给上游 API。
const attachments = ref<ChatAttachment[]>([]);
const uploading = ref(false);
const fileInputRef = ref<HTMLInputElement>();
const ATTACH_ACCEPT = '.png,.jpg,.jpeg,.webp,.gif,.pdf,.docx,.doc,.txt,.md,.csv,.json,.xlsx,.pptx';
const IMAGE_MAX = 10 * 1024 * 1024;
const FILE_MAX = 20 * 1024 * 1024;

function pickFiles() {
  fileInputRef.value?.click();
}

async function onFilesPicked(e: Event) {
  const input = e.target as HTMLInputElement;
  const files = Array.from(input.files || []);
  input.value = ''; // 允许重复选择同一文件
  if (!files.length) return;
  for (const f of files) {
    if (attachments.value.length >= 10) {
      ElMessage.warning('单次最多带 10 个附件');
      break;
    }
    const isImg = /^image\//.test(f.type) || /\.(png|jpe?g|webp|gif)$/i.test(f.name);
    const limit = isImg ? IMAGE_MAX : FILE_MAX;
    if (f.size > limit) {
      ElMessage.warning(`${f.name} 超过 ${limit >> 20}MB，已跳过`);
      continue;
    }
    uploading.value = true;
    try {
      const res = await uploadChatAttachment(f);
      if (res.data.code === 0 && res.data.data) {
        attachments.value.push(res.data.data);
      }
    } catch (err: any) {
      ElMessage.error(err?.response?.data?.message || `${f.name} 上传失败`);
    } finally {
      uploading.value = false;
    }
  }
}

function removeAttachment(i: number) {
  attachments.value.splice(i, 1);
}

function fmtSize(n?: number) {
  if (!n) return '';
  return n > 1024 * 1024 ? `${(n / 1024 / 1024).toFixed(1)}MB` : `${Math.max(1, Math.round(n / 1024))}KB`;
}
const generating = ref(false);
const streamingText = ref('');
// 思考型模型的推理增量（与正文分开：正文还没来时先给用户看到"它在想"）
const streamingReasoning = ref('');
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

/** 当前选中的模型是否为「思考型」（先出推理再出正文） */
const selectedThinking = computed(
  () => !useByok.value && !!filteredModels.value.find((m) => m.id === modelId.value)?.thinking,
);

function modelLabel(m: { name: string; price_in: number; price_out: number }) {
  const price = m.price_in === 0 && m.price_out === 0 ? '免费' : `${m.price_in}/${m.price_out}`;
  return `${m.name}（${price}）`;
}

function renderMd(md: string) {
  // AI 生成内容按不可信输入处理：严格白名单 + 全量转义兜底
  let html: string;
  try {
    html = renderAiMarkdown(md);
  } catch {
    html = md.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }
  return enhanceCodeBlocks(html);
}

/**
 * 给每个代码块挂一个「复制」按钮。
 * 用 DOMParser 后处理而不是改渲染引擎：cherry 的 HTML 结构随版本会变，
 * 而 v-html 出来的是标准 pre/code，按 DOM 处理最稳。
 * 先把原文存到 data-code，再插按钮——否则按钮文字会被当成代码内容复制走。
 */
function enhanceCodeBlocks(html: string): string {
  if (!html.includes('<pre')) return html;
  try {
    const doc = new DOMParser().parseFromString(html, 'text/html');
    doc.querySelectorAll('pre').forEach((pre) => {
      if (pre.querySelector('.code-copy')) return;
      const code = pre.textContent || '';
      if (!code.trim()) return;
      const bar = doc.createElement('div');
      bar.className = 'code-bar';
      const btn = doc.createElement('button');
      btn.type = 'button';
      btn.className = 'code-copy';
      btn.textContent = '复制';
      btn.setAttribute('data-code', code);
      bar.appendChild(btn);
      pre.appendChild(bar);
    });
    return doc.body.innerHTML;
  } catch {
    return html;
  }
}

/** 代码块复制（事件委托：v-html 内部无法直接绑事件） */
async function onMdClick(e: MouseEvent) {
  const btn = (e.target as HTMLElement)?.closest?.('.code-copy') as HTMLElement | null;
  if (!btn) return;
  const code = btn.getAttribute('data-code') || '';
  try {
    await navigator.clipboard.writeText(code);
    btn.textContent = '已复制';
    btn.classList.add('is-done');
    window.setTimeout(() => {
      btn.textContent = '复制';
      btn.classList.remove('is-done');
    }, 1500);
  } catch {
    ElMessage.warning('复制失败，请手动选择');
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
/**
 * 重新生成（分支版）。
 *
 * 以前是「截断到该提问之后重发」——旧回复直接被删掉，想对比两个回答就没了。
 * 现在改成新建兄弟分支：原回复保留，新回复挂在同一个提问下，用「‹ 1/2 ›」切换。
 * 后端按 regenerate_msg_id 把新回复的 parent 指向原提问，不覆盖任何东西。
 */
function regenerate(m: AiChatMessage) {
  if (generating.value || sessionLocked.value) return;
  const idx = messages.value.findIndex((x) => x.id === m.id);
  if (idx < 0) return;
  let userIdx = -1;
  for (let i = idx - 1; i >= 0; i--) {
    if (messages.value[i].role === 'user') {
      userIdx = i;
      break;
    }
  }
  if (userIdx < 0) return;
  // 内容不重要（后端会用库里原提问），但必须非空才过 send 的前置校验
  send(messages.value[userIdx].content, false, true, { regenerateMsgId: m.id });
}

/**
 * 申诉指引。
 *
 * 不做「点一下自动解封」——那等于给闸门开后门。
 * 这里只把管理员复核需要的信息整理好让用户带走，
 * 后台「已终止会话」列表里能按会话号直接定位并恢复。
 */
function onAppeal() {
  const sid = props.sessionId || 0;
  ElMessageBox.alert(
    `会话编号：#${sid}\n终止原因：${sessionLockReason.value || '内容安全策略'}\n\n` +
      '请把上面这行信息发给站长。管理员可在后台「内容治理 → AI 防越狱 → 已终止会话」中复核并恢复。',
    '申诉 / 反馈',
    { confirmButtonText: '我知道了', cancelButtonText: '', type: 'info' },
  ).catch(() => {});
}

/** 切换兄弟分支：只改显示位置，不删任何消息 */
async function switchSibling(m: AiChatMessage, dir: number) {
  const ids = m.sibling_ids || [];
  const cur = ids.indexOf(m.id);
  const nextID = ids[cur + dir];
  if (!nextID || !props.sessionId) return;
  try {
    const res = await aiHubApi.switchBranch(props.sessionId, nextID);
    if (res.data.code === 0) {
      messages.value = res.data.data.messages || [];
      scrollBottom();
    }
  } catch {
    ElMessage.error('切换分支失败');
  }
}

function scrollBottom() {
  nextTick(() => {
    if (msgListEl.value) msgListEl.value.scrollTop = msgListEl.value.scrollHeight;
  });
}

// 加载当前分支的消息（含终止状态与兄弟分支信息）
async function reloadMessages() {
  const id = props.sessionId;
  if (!id) return;
  const token = ++loadingSession;
  try {
    const res = await aiHubApi.sessionMessages(id);
    if (token !== loadingSession) return; // 防竞态
    if (res.data.code === 0) {
      messages.value = res.data.data.messages || [];
      const s = res.data.data.session as any;
      sessionLocked.value = !!s?.locked;
      sessionLockReason.value = s?.lock_reason || '';
      contaminated.value = !!s?.contaminated;
    }
  } catch {
    // 加载失败不弹窗：切会话时偶发失败会自己再触发一次
  }
  scrollBottom();
}

// 切换会话：加载历史
watch(
  () => props.sessionId,
  (id) => {
    if (!id) {
      messages.value = [];
      sessionLocked.value = false;
      sessionLockReason.value = '';
      contaminated.value = false;
      return;
    }
    reloadMessages();
  },
  { immediate: true },
);

async function send(rawText?: string, confirmed = false, skipPush = false, opts: { regenerateMsgId?: number } = {}) {
  const isRetry = typeof rawText === 'string';
  const text = (isRetry ? rawText : inputText.value).trim();
  // 重新生成不重发附件（附件只在首次发送时随请求带上）
  const pendingAtts = isRetry ? [] : attachments.value.slice();
  if ((!text && !pendingAtts.length) || generating.value) return;
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
  if (!isRetry) {
    inputText.value = '';
    attachments.value = [];
  }
  // 乐观上屏（重新生成时提问已在列表里，skipPush 避免重复）
  if (!skipPush) {
    const names = pendingAtts.map((a) => a.name).join('、');
    const shown = text || '（已发送附件）';
    messages.value.push({
      id: -Date.now(), role: 'user',
      content: names ? `${shown}\n\n📎 ${names}` : shown,
      tokens_in: 0, tokens_out: 0, cost: 0, created_at: '',
    });
  }
  generating.value = true;
  streamingText.value = '';
  streamingReasoning.value = '';
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
        // 图片/文档：后端读回文件原样内联给上游（平台不解析）
        attachments: pendingAtts.length ? pendingAtts : undefined,
        // 重新生成：把新回复挂到原提问下成为兄弟分支（不覆盖旧回复）
        regenerate_msg_id: opts.regenerateMsgId,
      },
      (chunk) => {
        streamingText.value += chunk;
        scrollBottom();
      },
      abortCtrl?.signal,
      (chunk) => {
        streamingReasoning.value += chunk;
        scrollBottom();
      },
      // 输出侧安全闸整段替换：已流出去的内容作废，直接换成拒答文案
      (text) => {
        streamingText.value = text;
        streamingReasoning.value = '';
        scrollBottom();
      },
    );
    if (!result) {
      ElMessage.warning('连接中断，未生成完整内容');
    } else {
      messages.value.push({
        id: -Date.now() - 1, role: 'assistant', content: result.content,
        tokens_in: 0, tokens_out: 0, cost: result.cost, free: result.free,
        // 推理过程只留在本地（后端未持久化），刷新后不再显示。
        // 若整段回答本就来自 reasoning（思考型模型 content 为空），
        // 它已经是正文了，再折叠一次就等于同一段文字出现两遍。
        reasoning: result.answerInReasoning ? undefined : result.reasoning || undefined,
        created_at: '',
      });
      if (result.blocked) {
        ElMessage.warning(result.terminated ? '该对话已被终止' : '该回答触发内容安全策略，已拦截');
      }
      if (result.terminated) {
        sessionLocked.value = true;
      }
      if (result.terminate_mode === 'moderate') {
        contaminated.value = true;
      }
      // 重新生成产生了兄弟分支：重载才能拿到「‹ 1/2 ›」切换信息
      if (opts.regenerateMsgId) await reloadMessages();
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
/* ===== 思考过程（思考型模型）===== */
.ai-think {
  margin: 0 0 8px;
  border: 1px dashed var(--border-color, #e5e7eb);
  border-radius: 8px;
  background: var(--bg-page, #fafbfc);
  font-size: 12px;
}
.ai-think summary {
  cursor: pointer;
  padding: 6px 10px;
  color: var(--text-light, #6b7280);
  user-select: none;
  list-style: none;
}
.ai-think summary::-webkit-details-marker { display: none; }
.ai-think summary::before { content: '▸ '; }
.ai-think[open] summary::before { content: '▾ '; }
.ai-think-body {
  padding: 0 10px 8px;
  max-height: 240px;
  overflow-y: auto;
  white-space: pre-wrap;
  line-height: 1.65;
  color: var(--text-light, #6b7280);
}
.ai-think-hint {
  display: block;
  font-size: 12px;
  color: var(--text-light, #9ca3af);
  font-style: italic;
}

.ai-msg-md :deep(pre) {
  position: relative;
  background: #1e293b; color: #e2e8f0; padding: 10px 12px;
  border-radius: 8px; overflow-x: auto; font-size: 0.82rem;
}
/* 代码块右上角复制按钮：默认半透明，hover 代码块才完全显形（避免干扰阅读） */
.ai-msg-md :deep(.code-bar) {
  position: absolute; top: 6px; right: 6px;
  display: flex; gap: 4px; opacity: 0.55; transition: opacity 0.15s ease;
}
.ai-msg-md :deep(pre:hover .code-bar) { opacity: 1; }
.ai-msg-md :deep(.code-copy) {
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: rgba(255, 255, 255, 0.08);
  color: #cbd5e1; font-size: 11px; line-height: 1;
  padding: 4px 8px; border-radius: 6px; cursor: pointer;
  font-family: inherit;
}
.ai-msg-md :deep(.code-copy:hover) { background: rgba(255, 255, 255, 0.18); color: #fff; }
.ai-msg-md :deep(.code-copy.is-done) {
  border-color: rgba(34, 197, 94, 0.5);
  background: rgba(34, 197, 94, 0.18);
  color: #86efac;
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

/* ===== 附件（图片 / 文档）===== */
.ai-attach-list {
  display: flex; flex-wrap: wrap; gap: 8px;
  padding: 10px 12px 0;
}
.ai-attach-item {
  position: relative;
  display: flex; align-items: center;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 10px;
  background: var(--el-fill-color-light, #f7f8fa);
  padding: 4px 22px 4px 6px;
  max-width: 260px;
}
.ai-attach-thumb {
  width: 44px; height: 44px; object-fit: cover; border-radius: 6px; display: block;
}
.ai-attach-file { display: flex; align-items: center; gap: 6px; min-width: 0; }
.ai-attach-ext {
  flex-shrink: 0;
  font-size: 0.62rem; font-weight: 700; letter-spacing: 0.02em;
  color: #fff; background: var(--gradient-blue, #6366f1);
  border-radius: 4px; padding: 2px 5px;
}
.ai-attach-name {
  font-size: 0.78rem; color: var(--text-color, #1f2328);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 130px;
}
.ai-attach-size { font-size: 0.7rem; color: var(--text-light, #999); flex-shrink: 0; }
.ai-attach-del {
  position: absolute; top: 2px; right: 2px;
  width: 18px; height: 18px; border: none; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  background: rgba(0, 0, 0, 0.45); color: #fff; cursor: pointer; padding: 0;
}
.ai-attach-del:hover { background: rgba(0, 0, 0, 0.7); }
.ai-attach-del .el-icon { font-size: 11px; }
.ai-attach-btn {
  width: 30px; height: 30px; border-radius: 50%; border: none;
  display: flex; align-items: center; justify-content: center;
  background: var(--el-fill-color-light, #f2f3f5);
  color: var(--text-regular, #606266); cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}
.ai-attach-btn:hover:not(:disabled) { background: var(--el-fill-color, #e9eaf0); color: var(--el-color-primary, #6366f1); }
.ai-attach-btn:disabled { opacity: 0.4; cursor: not-allowed; }

/* ── 对话分支切换 ── */
.ai-branch {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  margin-left: 6px;
  padding: 0 4px;
  border-radius: 6px;
  background: var(--bg-hover, rgba(127, 127, 127, 0.08));
}
.ai-branch-user { margin: 6px 0 0; align-self: flex-end; }
.ai-branch .ai-msg-tool { padding: 0 4px; font-size: 0.95rem; line-height: 1; }
.ai-branch .ai-msg-tool:disabled { opacity: 0.3; cursor: not-allowed; }
.ai-branch-idx { font-size: 0.72rem; color: var(--text-light, #888); min-width: 26px; text-align: center; }

/* ── 会话已终止 ── */
.ai-locked-banner {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  margin: 0 16px 10px;
  padding: 12px 14px;
  border-radius: 10px;
  border: 1px solid var(--el-color-danger, #f56c6c);
  background: rgba(245, 108, 108, 0.08);
  color: var(--text-main, #303133);
}
.ai-locked-banner .el-icon { color: var(--el-color-danger, #f56c6c); margin-top: 2px; }
.ai-locked-body strong { display: block; font-size: 0.9rem; margin-bottom: 4px; }
.ai-locked-body p { margin: 0; font-size: 0.8rem; line-height: 1.5; color: var(--text-light, #666); }
.ai-locked-hint { margin-top: 4px !important; font-size: 0.75rem !important; opacity: 0.8; }

/* 被拦截的回答：警告块而不是普通气泡 */
.ai-blocked-card {
  border: 1px dashed var(--el-color-warning, #e6a23c);
  background: rgba(230, 162, 60, 0.07);
  border-radius: 10px;
  padding: 10px 12px;
  max-width: 100%;
}
.ai-blocked-card.is-strict {
  border-color: var(--el-color-danger, #f56c6c);
  background: rgba(245, 108, 108, 0.07);
}
.ai-blocked-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--el-color-warning, #e6a23c);
  margin-bottom: 6px;
}
.ai-blocked-card.is-strict .ai-blocked-title { color: var(--el-color-danger, #f56c6c); }
.ai-blocked-text {
  margin: 0;
  font-size: 0.8rem;
  line-height: 1.6;
  color: var(--text-light, #666);
  white-space: pre-wrap;
}

/* 中档模式：输入框上方的安全提示（不打断，但要让人看得见） */
.ai-safety-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 8px;
  padding: 7px 10px;
  border-radius: 8px;
  background: rgba(230, 162, 60, 0.1);
  border: 1px solid rgba(230, 162, 60, 0.35);
  font-size: 0.78rem;
  color: var(--el-color-warning, #b88230);
}
.ai-appeal-btn { margin-top: 6px; }

/* 中档模式：只是标记，不是终止 —— 用橙色而不是红色，避免用户以为对话废了 */
.ai-locked-banner.is-moderate {
  border-color: var(--el-color-warning, #e6a23c);
  background: rgba(230, 162, 60, 0.08);
}
.ai-locked-banner.is-moderate .el-icon { color: var(--el-color-warning, #e6a23c); }

[data-theme='dark'] .ai-attach-item { background: #1c2030; border-color: #2b3145; }
[data-theme='dark'] .ai-attach-name { color: #e5e7eb; }
[data-theme='dark'] .ai-attach-btn { background: #232839; color: #c7ccd8; }
.ai-composer-tip-attach { color: var(--el-color-warning, #e6a23c); }
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
