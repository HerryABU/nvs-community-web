<template>
  <!-- AI 会话分享页（公开只读）：内容一律按不可信输入渲染 -->
  <div class="ai-share">
    <div class="ai-share-card">
      <header class="ai-share-head">
        <div class="ai-share-title">{{ title || 'AI 对话分享' }}</div>
        <div class="ai-share-sub">
          <span v-if="agentName">{{ agentName }}</span>
          <span v-if="sharedAt">· 分享于 {{ formatTime(sharedAt) }}</span>
        </div>
      </header>

      <div v-if="loading" class="ai-share-loading">加载中…</div>
      <div v-else-if="error" class="ai-share-error">{{ error }}</div>
      <div v-else class="ai-share-body">
        <div
          v-for="(m, i) in messages"
          :key="i"
          :class="['sh-msg', m.role === 'user' ? 'sh-msg-user' : 'sh-msg-assistant']"
        >
          <div class="sh-role">{{ m.role === 'user' ? '提问' : 'AI 回答' }}</div>
          <div v-if="m.role === 'assistant'" class="sh-md" v-html="render(m.content)" />
          <div v-else class="sh-plain">{{ m.content }}</div>
        </div>
      </div>

      <footer class="ai-share-foot">
        <span>只读快照 · 由 NVS AI 工作台生成</span>
        <el-button text type="primary" size="small" @click="$router.push('/ai')">打开 AI 工作台</el-button>
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { aiHubApi } from '@/api/aiHub';
import { renderAiMarkdown } from '@/markdown/renderer';

const route = useRoute();
const loading = ref(true);
const error = ref('');
const title = ref('');
const agentName = ref('');
const sharedAt = ref<string | null>(null);
const messages = ref<Array<{ role: 'user' | 'assistant'; content: string; created_at: string }>>([]);

function render(md: string) {
  try {
    return renderAiMarkdown(md);
  } catch {
    return md.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }
}

function formatTime(s: string | null) {
  if (!s) return '';
  try {
    return new Date(s).toLocaleString();
  } catch {
    return s;
  }
}

onMounted(async () => {
  const token = String(route.params.token || '');
  if (!token) {
    error.value = '分享链接无效';
    loading.value = false;
    return;
  }
  try {
    const res = await aiHubApi.sharedSession(token);
    if (res.data.code === 0) {
      const d = res.data.data;
      title.value = d.title || 'AI 对话分享';
      agentName.value = d.agent_name || '';
      sharedAt.value = d.shared_at;
      messages.value = d.messages || [];
    } else {
      error.value = '分享不存在或已取消';
    }
  } catch (e: any) {
    error.value = e?.response?.data?.message || '分享不存在或已取消';
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.ai-share { min-height: 100vh; padding: 28px 16px; background: var(--bg-page, #f5f7fa); }
.ai-share-card {
  max-width: 860px; margin: 0 auto; background: var(--el-bg-color, #fff);
  border-radius: 14px; box-shadow: 0 2px 16px rgba(0, 0, 0, 0.06); overflow: hidden;
}
.ai-share-head { padding: 18px 22px; border-bottom: 1px solid var(--el-border-color-lighter, #eee); }
.ai-share-title { font-size: 1.05rem; font-weight: 600; color: var(--text-color, #222); }
.ai-share-sub { margin-top: 4px; font-size: 0.78rem; color: var(--text-light, #888); display: flex; gap: 6px; }
.ai-share-loading, .ai-share-error { padding: 48px; text-align: center; color: var(--text-light, #888); }
.ai-share-error { color: #ef4444; }
.ai-share-body { padding: 18px 22px; }
.sh-msg { margin-bottom: 18px; }
.sh-role { font-size: 0.72rem; color: var(--text-light, #999); margin-bottom: 5px; }
.sh-plain {
  white-space: pre-wrap; background: var(--el-fill-color-light, #f5f7fa);
  padding: 10px 14px; border-radius: 10px; font-size: 0.9rem; line-height: 1.7;
}
.sh-md {
  font-size: 0.9rem; line-height: 1.75; color: var(--text-color, #333);
  border-left: 3px solid var(--el-color-primary, #409eff); padding-left: 12px;
}
.sh-md :deep(p) { margin: 0 0 8px; }
.sh-md :deep(pre) {
  background: #1e293b; color: #e2e8f0; padding: 10px 12px;
  border-radius: 8px; overflow-x: auto; font-size: 0.82rem;
}
.sh-md :deep(code) { font-family: Consolas, monospace; }
.sh-md :deep(table) { border-collapse: collapse; font-size: 0.85rem; }
.sh-md :deep(th), .sh-md :deep(td) { border: 1px solid var(--el-border-color, #e5e7eb); padding: 4px 8px; }
.ai-share-foot {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 22px; border-top: 1px solid var(--el-border-color-lighter, #eee);
  font-size: 0.76rem; color: var(--text-light, #999);
}
</style>
