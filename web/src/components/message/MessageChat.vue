<template>
  <div class="chat-wrap" v-loading="loading">
    <!-- 顶部：对方信息 + 操作 -->
    <div class="chat-header">
      <el-avatar :size="36" :src="peerAvatar">{{ initial }}</el-avatar>
      <div class="peer-info">
        <div class="peer-name">{{ peerName || '用户' }}</div>
      </div>
      <el-popconfirm title="删除后聊天记录将从你的视图消失，对方不受影响。确定删除会话？" confirm-button-text="删除" cancel-button-text="取消" @confirm="deleteConversation">
        <template #reference>
          <el-button size="small" text type="danger">删除会话</el-button>
        </template>
      </el-popconfirm>
    </div>

    <!-- 消息区 -->
    <div ref="scrollBoxRef" class="chat-body">
      <div v-if="hasMore" class="load-more">
        <el-button size="small" text :loading="loadingMore" @click="loadMore">加载更早的消息</el-button>
      </div>
      <el-empty v-if="!loading && !messages.length" description="还没有消息，发第一条打个招呼吧" :image-size="80" />
      <div v-for="m in messages" :key="m.id" class="msg-row" :class="{ own: m.sender_id === currentUserId }">
        <div class="bubble" :class="{ recalled: m.recalled }">
          <template v-if="!m.recalled">{{ m.content }}</template>
          <span v-else class="recalled-text">消息已撤回</span>
        </div>
        <div class="msg-meta">
          <span class="msg-time">{{ formatTime(m.created_at) }}</span>
          <span v-if="m.sender_id === currentUserId && !m.recalled" class="msg-status">
            {{ m.read_at ? '已读' : '已送达' }}
          </span>
          <el-button
            v-if="m.sender_id === currentUserId && !m.recalled && canRecall(m)"
            size="small" text type="primary" class="recall-btn"
            @click="doRecall(m)"
          >撤回</el-button>
        </div>
      </div>
    </div>

    <!-- 输入区 -->
    <div class="chat-input">
      <el-input
        v-model="draft"
        type="textarea"
        :rows="3"
        resize="none"
        :maxlength="MESSAGE_MAX_LENGTH"
        show-word-limit
        placeholder="说点什么…（Enter 发送，Shift+Enter 换行）"
        @keydown.enter.exact.prevent="send"
      />
      <div class="input-bar">
        <span class="input-hint">双方对话请遵守社区规范</span>
        <el-button type="primary" size="small" :loading="sending" :disabled="!draft.trim()" @click="send">发送</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { messageApi, MESSAGE_MAX_LENGTH, RECALL_WINDOW_MS, type MessageItem } from '@/api/message';
import { useAuthStore } from '@/stores/auth';

const props = defineProps<{
  peerId: number;
  peerName: string;
  peerAvatar: string;
}>();

const emit = defineEmits<{
  (e: 'sent'): void;
  (e: 'read'): void;
  (e: 'deleted'): void;
}>();

const authStore = useAuthStore();
const currentUserId = computed(() => authStore.user?.id || 0);

const messages = ref<MessageItem[]>([]); // 内部按时间正序（旧→新）
const draft = ref('');
const loading = ref(false);
const loadingMore = ref(false);
const sending = ref(false);
const page = ref(1);
const hasMore = ref(false);
const scrollBoxRef = ref<HTMLElement | null>(null);
let pollTimer: number | undefined;

const initial = computed(() => (props.peerName || 'U').trim().charAt(0).toUpperCase());

function formatTime(iso: string) {
  if (!iso) return '';
  const d = new Date(iso);
  const pad = (x: number) => String(x).padStart(2, '0');
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// 2 分钟撤回窗口
function canRecall(m: MessageItem) {
  return Date.now() - new Date(m.created_at).getTime() <= RECALL_WINDOW_MS;
}

function scrollToBottom() {
  nextTick(() => {
    const el = scrollBoxRef.value;
    if (el) el.scrollTop = el.scrollHeight;
  });
}

// 拉取一页（后端按 id 倒序返回，此处反转为正序合并）
async function fetchPage(p: number): Promise<MessageItem[]> {
  const res = await messageApi.messagesWith(props.peerId, p, 30);
  if (res.data.code === 0) {
    hasMore.value = !!res.data.data.has_more;
    page.value = p;
    return [...(res.data.data.list || [])].reverse();
  }
  return [];
}

async function load(initialLoad = false) {
  loading.value = true;
  try {
    const list = await fetchPage(1);
    messages.value = list;
    if (initialLoad) scrollToBottom();
    // 打开会话即把对方消息标记已读
    await messageApi.markRead(props.peerId);
    emit('read');
  } catch { ElMessage.error('加载聊天记录失败'); }
  finally { loading.value = false; }
}

async function loadMore() {
  loadingMore.value = true;
  try {
    const older = await fetchPage(page.value + 1);
    const existing = new Set(messages.value.map((m) => m.id));
    messages.value = [...older.filter((m) => !existing.has(m.id)), ...messages.value];
  } catch { ElMessage.error('加载失败'); }
  finally { loadingMore.value = false; }
}

async function send() {
  const content = draft.value.trim();
  if (!content || sending.value) return;
  sending.value = true;
  try {
    const res = await messageApi.send(props.peerId, content);
    if (res.data.code === 0) {
      messages.value.push(res.data.data);
      draft.value = '';
      scrollToBottom();
      emit('sent');
    }
  } catch (err: any) {
    // 403=互相关注限制等，展示后端返回的具体原因
    ElMessage.error(err?.response?.data?.message || '发送失败');
  } finally { sending.value = false; }
}

async function doRecall(m: MessageItem) {
  try {
    await ElMessageBox.confirm('撤回后双方都将看到「消息已撤回」，确定撤回？', '撤回消息', {
      confirmButtonText: '撤回', cancelButtonText: '取消', type: 'warning',
    });
  } catch { return; } // 用户取消
  try {
    const res = await messageApi.recall(m.id);
    if (res.data.code === 0) {
      m.recalled = true;
      ElMessage.success('已撤回');
      emit('sent'); // 会话列表的最后一条预览需要刷新
    }
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.message || '撤回失败');
  }
}

async function deleteConversation() {
  try {
    const res = await messageApi.deleteConversation(props.peerId);
    if (res.data.code === 0) {
      ElMessage.success('会话已删除');
      emit('deleted');
    }
  } catch { ElMessage.error('删除失败'); }
}

// 轻量轮询：拉取最新消息合并（仅追加对方新消息）
async function poll() {
  try {
    const res = await messageApi.messagesWith(props.peerId, 1, 30);
    if (res.data.code !== 0) return;
    const latest = [...(res.data.data.list || [])].reverse();
    const existing = new Set(messages.value.map((m) => m.id));
    const fresh = latest.filter((m) => !existing.has(m.id));
    if (!fresh.length) return;
    const fromPeer = fresh.some((m) => m.sender_id !== currentUserId.value);
    messages.value.push(...fresh);
    scrollToBottom();
    if (fromPeer) {
      await messageApi.markRead(props.peerId);
      emit('read');
    }
  } catch { /* 轮询失败静默 */ }
}

onMounted(() => {
  load(true);
  pollTimer = window.setInterval(poll, 8000);
});
onBeforeUnmount(() => {
  if (pollTimer) window.clearInterval(pollTimer);
});
</script>

<style scoped>
.chat-wrap { display: flex; flex-direction: column; height: 100%; min-height: 480px; background: #fff; border-radius: 10px; border: 1px solid #eee; overflow: hidden; }
.chat-header { display: flex; align-items: center; gap: 10px; padding: 10px 14px; border-bottom: 1px solid #f0f0f0; }
.peer-info { flex: 1; }
.peer-name { font-weight: 600; font-size: 0.95rem; }
.chat-body { flex: 1; overflow-y: auto; padding: 16px 14px; display: flex; flex-direction: column; gap: 10px; }
.load-more { text-align: center; }
.msg-row { display: flex; flex-direction: column; align-items: flex-start; max-width: 78%; }
.msg-row.own { align-self: flex-end; align-items: flex-end; }
.bubble { padding: 8px 12px; border-radius: 10px; background: #f4f4f5; font-size: 0.9rem; line-height: 1.5; white-space: pre-wrap; word-break: break-word; }
.msg-row.own .bubble { background: #409eff; color: #fff; }
.bubble.recalled { background: #fafafa !important; color: #999; font-size: 0.78rem; font-style: italic; padding: 4px 10px; }
.recalled-text { font-style: italic; }
.msg-meta { display: flex; align-items: center; gap: 6px; margin-top: 2px; font-size: 0.7rem; color: #aaa; }
.recall-btn { font-size: 0.7rem; padding: 0; height: auto; }
.chat-input { border-top: 1px solid #f0f0f0; padding: 10px 12px; }
.input-bar { display: flex; justify-content: space-between; align-items: center; margin-top: 8px; }
.input-hint { font-size: 0.72rem; color: #bbb; }

@media (max-width: 640px) {
  .chat-wrap { min-height: 60vh; }
  .msg-row { max-width: 88%; }
}
</style>
