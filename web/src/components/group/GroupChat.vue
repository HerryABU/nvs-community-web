<template>
  <div class="gc">
    <!-- 消息区 -->
    <div ref="listEl" class="gc-list">
      <div v-if="loading" class="gc-empty">加载中…</div>
      <div v-else-if="messages.length === 0" class="gc-empty">
        还没有群聊消息，来说第一句吧 👋
      </div>
      <div
        v-for="m in messages"
        :key="m.id"
        class="gc-msg"
        :class="{ 'is-mine': m.user_id === myId }"
      >
        <div class="gc-bubble">
          <div class="gc-meta">
            <span class="gc-user">{{ m.user_id === myId ? '我' : '用户#' + m.user_id }}</span>
            <span class="gc-time">{{ formatTime(m.created_at) }}</span>
          </div>
          <div class="gc-content">{{ m.content }}</div>
        </div>
      </div>
    </div>

    <!-- 输入区 -->
    <div class="gc-input">
      <el-input
        v-model="draft"
        type="textarea"
        :rows="2"
        resize="none"
        placeholder="发条消息…（Enter 发送，Shift+Enter 换行）"
        maxlength="2000"
        @keydown.enter.exact.prevent="send"
      />
      <el-button type="primary" :loading="sending" :disabled="!draft.trim()" @click="send">发送</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue';
import { ElMessage } from 'element-plus';
import { groupApi, type GroupChatMessage } from '@/api/group';
import { useAuthStore } from '@/stores/auth';

const props = defineProps<{ groupId: number }>();
const auth = useAuthStore();
const myId = computed(() => auth.user?.id || 0);

const messages = ref<GroupChatMessage[]>([]);

const loading = ref(true);
const sending = ref(false);
const draft = ref('');
const listEl = ref<HTMLElement | null>(null);
let timer: ReturnType<typeof setInterval> | null = null;

async function load() {
  try {
    const res = await groupApi.chatList(props.groupId);
    const list = res.data?.data?.list || [];
    // 仅在有新消息时滚动到底，避免轮询打断阅读
    const prev = messages.value.length;
    messages.value = list;
    if (list.length !== prev) await nextTick(() => scrollToBottom());
  } catch { /* 静默：群聊不可用时不打扰 */ }
  finally { loading.value = false; }
}

function scrollToBottom() {
  if (listEl.value) listEl.value.scrollTop = listEl.value.scrollHeight;
}

async function send() {
  const text = draft.value.trim();
  if (!text) return;
  sending.value = true;
  try {
    await groupApi.chatSend(props.groupId, { content: text });
    draft.value = '';
    await load();
    await nextTick(() => scrollToBottom());
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发送失败');
  } finally {
    sending.value = false;
  }
}

function formatTime(s: string) {
  const d = new Date(s);
  return isNaN(d.getTime()) ? '' : d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
}

onMounted(() => {
  load();
  timer = setInterval(load, 5000);
});
onUnmounted(() => { if (timer) clearInterval(timer); });

// 供外部（如实时流）强制刷新
defineExpose({ load });
</script>

<style scoped>
.gc { display: flex; flex-direction: column; height: 100%; min-height: 320px; }
.gc-list {
  flex: 1;
  overflow-y: auto;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: var(--bg-soft, #f7f8fa);
  border-radius: var(--radius-md, 12px);
}
.gc-empty { margin: auto; font-size: 0.85rem; color: var(--text-light, #999); }
.gc-msg { display: flex; }
.gc-msg.is-mine { justify-content: flex-end; }
.gc-bubble {
  max-width: 70%;
  padding: 8px 12px;
  border-radius: 12px;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
}
.gc-msg.is-mine .gc-bubble {
  background: var(--primary-color, #6366f1);
  border-color: transparent;
  color: #fff;
}
.gc-meta { display: flex; gap: 8px; align-items: baseline; margin-bottom: 2px; }
.gc-user { font-size: 0.72rem; opacity: 0.85; }
.gc-time { font-size: 0.68rem; opacity: 0.6; }
.gc-content { font-size: 0.9rem; line-height: 1.5; white-space: pre-wrap; word-break: break-word; }
.gc-input { display: flex; gap: 8px; margin-top: 10px; align-items: flex-end; }
.gc-input .el-textarea { flex: 1; }
</style>
