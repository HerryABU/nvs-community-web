<template>
  <div class="msg-page">
    <div class="msg-header">
      <h2>🔔 消息中心</h2>
      <el-button size="small" type="primary" plain :disabled="!unreadTotal" @click="markAllRead">全部已读</el-button>
    </div>

    <el-empty v-if="!loading && !list.length" description="暂无消息" />

    <div v-else class="msg-list">
      <div
        v-for="n in list"
        :key="n.id"
        class="msg-item"
        :class="{ 'is-unread': !n.read }"
        @click="openNotification(n)"
      >
        <span class="msg-icon">{{ typeIcon(n.type) }}</span>
        <div class="msg-body">
          <div class="msg-title">
            <span class="msg-dot" v-if="!n.read"></span>
            {{ n.title }}
            <span class="msg-type">{{ typeLabel(n.type) }}</span>
          </div>
          <div class="msg-content">{{ n.content }}</div>
          <div class="msg-meta">
            <span v-if="n.actor_name">{{ n.actor_name }}</span>
            <span>{{ formatTime(n.created_at) }}</span>
          </div>
        </div>
      </div>
    </div>

    <div v-if="total > list.length" class="msg-more">
      <el-button size="small" text :loading="loadingMore" @click="loadMore">加载更多（{{ list.length }}/{{ total }}）</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { notificationApi, type NotificationItem } from '@/api/notification';
import { ElMessage } from 'element-plus';

const router = useRouter();
const list = ref<NotificationItem[]>([]);
const total = ref(0);
const page = ref(1);
const unreadTotal = ref(0);
const loading = ref(false);
const loadingMore = ref(false);

const typeLabels: Record<string, string> = {
  thread_reply: '帖子回复', comment_reply: '评论回复',
  novel_comment: '作品评论', blog_comment: '博客评论',
  // 未成年人心理危机保护：抚慰私信 / 保护提示 / 危机后的柔和通知
  crisis_comfort: '关怀', shield_notice: '保护提示',
  // 仅站长会收到：AI 判危后的上报
  youth_crisis: '危机告警',
};
const typeIcons: Record<string, string> = {
  thread_reply: '💬', comment_reply: '↩️',
  novel_comment: '📖', blog_comment: '✍️',
  crisis_comfort: '🌿', shield_notice: '🛡️',
  youth_crisis: '⚠️',
};
function typeLabel(t: string) { return typeLabels[t] || '通知'; }
function typeIcon(t: string) { return typeIcons[t] || '🔔'; }

function formatTime(iso: string) {
  if (!iso) return '';
  const d = new Date(iso);
  const now = Date.now();
  const diff = now - d.getTime();
  if (diff < 60_000) return '刚刚';
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)} 分钟前`;
  if (diff < 86400_000) return `${Math.floor(diff / 3600_000)} 小时前`;
  const pad = (x: number) => String(x).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

async function load(append = false) {
  if (append) loadingMore.value = true;
  else loading.value = true;
  try {
    const res = await notificationApi.list(append ? page.value + 1 : 1, 20);
    if (res.data.code === 0) {
      const d = res.data.data;
      if (append) {
        list.value.push(...(d.list || []));
        page.value += 1;
      } else {
        list.value = d.list || [];
        page.value = 1;
      }
      total.value = Number(d.total) || 0;
      unreadTotal.value = list.value.filter((x) => !x.read).length;
      refreshUnreadBadge();
    }
  } catch { ElMessage.error('加载消息失败'); }
  finally { loading.value = false; loadingMore.value = false; }
}

function loadMore() { load(true); }

// 点击：跳转目标并标记该条已读
async function openNotification(n: NotificationItem) {
  if (!n.read) {
    n.read = true;
    unreadTotal.value = Math.max(0, unreadTotal.value - 1);
    refreshUnreadBadge();
    notificationApi.markRead(n.id).catch(() => {});
  }
  if (n.link) router.push(n.link);
  else load();
}

async function markAllRead() {
  try {
    await notificationApi.markRead(0);
    list.value.forEach((x) => (x.read = true));
    unreadTotal.value = 0;
    refreshUnreadBadge();
    ElMessage.success('已全部标记为已读');
  } catch { ElMessage.error('操作失败'); }
}

// 同步导航栏徽标（通过事件广播，避免与 NavBar 状态不同步）
function refreshUnreadBadge() {
  const count = list.value.filter((x) => !x.read).length;
  window.dispatchEvent(new CustomEvent('nvs:unread', { detail: count }));
}

onMounted(() => load());
</script>

<style scoped>
.msg-page { max-width: 720px; margin: 0 auto; padding: 90px 20px 40px; min-height: 70vh; }
.msg-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 18px; }
.msg-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.msg-list { display: flex; flex-direction: column; gap: 10px; }
.msg-item {
  display: flex; gap: 12px; padding: 14px 16px; border-radius: 10px;
  background: var(--bg-card, #fff); border: 1px solid var(--glass-border, #e5e7eb);
  cursor: pointer; transition: all 0.2s ease;
}
.msg-item:hover { border-color: var(--primary-color, #6366f1); transform: translateY(-1px); }
.msg-item.is-unread { background: rgba(99, 102, 241, 0.06); border-color: rgba(99, 102, 241, 0.35); }
.msg-icon { font-size: 1.4rem; line-height: 1.4; }
.msg-body { flex: 1; min-width: 0; }
.msg-title { font-size: 0.95rem; font-weight: 600; color: var(--text-color, #333); display: flex; align-items: center; gap: 6px; }
.msg-dot { width: 8px; height: 8px; border-radius: 50%; background: #ef4444; flex-shrink: 0; }
.msg-type { font-size: 0.7rem; font-weight: 400; color: var(--primary-color, #6366f1); background: rgba(99,102,241,0.1); padding: 1px 6px; border-radius: 8px; }
.msg-content { margin-top: 4px; font-size: 0.85rem; color: var(--text-light, #666); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.msg-meta { margin-top: 6px; font-size: 0.75rem; color: var(--text-light, #999); display: flex; gap: 12px; }
.msg-more { text-align: center; margin-top: 16px; }
[data-theme="dark"] .msg-item { background: #1a1f2e; border-color: rgba(255,255,255,0.08); }
[data-theme="dark"] .msg-item.is-unread { background: rgba(129, 140, 248, 0.12); border-color: rgba(129, 140, 248, 0.4); }

/* ====== 移动端消息中心适配 ====== */
@media (max-width: 640px) {
  .msg-page {
    padding: 76px 12px 32px;
  }
  .msg-header h2 {
    font-size: 1.1rem;
  }
  .msg-item {
    padding: 12px;
    gap: 10px;
  }
  .msg-title {
    flex-wrap: wrap;
  }
  .msg-content {
    white-space: normal;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
  }
}
</style>
