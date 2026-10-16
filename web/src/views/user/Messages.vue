<template>
  <div class="page-container msg-page">
    <div class="msg-header">
      <h2>私信</h2>
      <div class="header-right">
        <span v-if="totalUnread" class="unread-total">未读 {{ totalUnread }}</span>
        <el-button size="small" plain @click="openSettings">私信设置</el-button>
      </div>
    </div>

    <div class="msg-layout" v-loading="loading">
      <!-- 左侧：会话列表 -->
      <aside class="conv-panel" :class="{ 'mobile-hidden': activePeer }">
        <el-empty v-if="!convs.length && !loading" description="还没有会话" :image-size="72" />
        <div
          v-for="cv in convs"
          :key="cv.peer_id"
          class="conv-item"
          :class="{ active: activePeer?.peer_id === cv.peer_id }"
          @click="selectConv(cv)"
        >
          <el-badge :value="cv.unread" :hidden="!cv.unread" :max="99">
            <el-avatar :size="42" :src="cv.peer_avatar_url">{{ (cv.peer_nickname || cv.peer_username || 'U').charAt(0) }}</el-avatar>
          </el-badge>
          <div class="conv-body">
            <div class="conv-top">
              <span class="conv-name">{{ cv.peer_nickname || cv.peer_username }}</span>
              <span class="conv-time">{{ formatTime(cv.last_at) }}</span>
            </div>
            <div class="conv-last">
              {{ lastPreview(cv) }}
            </div>
          </div>
        </div>
      </aside>

      <!-- 右侧：聊天窗 -->
      <section class="chat-panel" :class="{ 'mobile-hidden': !activePeer }">
        <el-button v-if="activePeer" class="back-btn" size="small" text @click="activePeer = null">‹ 返回会话列表</el-button>
        <MessageChat
          v-if="activePeer"
          :key="activePeer.peer_id"
          :peer-id="activePeer.peer_id"
          :peer-name="activePeer.peer_nickname || activePeer.peer_username"
          :peer-avatar="activePeer.peer_avatar_url"
          @sent="loadConversations"
          @read="onPeerRead"
          @deleted="onConvDeleted"
        />
        <el-empty v-else class="chat-empty" description="选择左侧会话开始聊天" :image-size="96" />
      </section>
    </div>

    <!-- 私信设置弹窗 -->
    <el-dialog v-model="settingsVisible" title="私信设置" width="380px">
      <div class="settings-row">
        <div class="settings-label">
          <div>互相关注才能发消息</div>
          <div class="settings-desc">开启后，陌生人只能给你发 2 条消息；第 3 条起需双方互相关注</div>
        </div>
        <el-switch v-model="settingsForm" :disabled="settingsLoading" />
      </div>
      <template #footer>
        <el-button @click="settingsVisible = false">取消</el-button>
        <el-button type="primary" :loading="settingsSaving" @click="saveSettings">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import { ElMessage } from 'element-plus';
import MessageChat from '@/components/message/MessageChat.vue';
import { messageApi, type ConversationItem } from '@/api/message';

const convs = ref<ConversationItem[]>([]);
const activePeer = ref<ConversationItem | null>(null);
const loading = ref(false);
let pollTimer: number | undefined;

const totalUnread = computed(() => convs.value.reduce((sum, c) => sum + (c.unread || 0), 0));

// 会话列表最后一条消息预览（撤回/空态处理）
function lastPreview(cv: ConversationItem) {
  const m = cv.last_message;
  if (!m) return '暂无消息';
  if (m.recalled) return '【撤回了一条消息】';
  const prefix = m.sender_id === cv.peer_id ? '' : '我：';
  return prefix + m.content;
}

function formatTime(iso: string) {
  if (!iso) return '';
  const d = new Date(iso);
  const now = Date.now();
  const diff = now - d.getTime();
  if (diff < 60_000) return '刚刚';
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)} 分钟前`;
  if (diff < 86400_000) return `${Math.floor(diff / 3600_000)} 小时前`;
  const pad = (x: number) => String(x).padStart(2, '0');
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

async function loadConversations() {
  try {
    const res = await messageApi.conversations();
    if (res.data.code === 0) {
      convs.value = res.data.data.list || [];
      // 保持当前会话引用与最新数据同步（未读数等）
      if (activePeer.value) {
        const fresh = convs.value.find((c) => c.peer_id === activePeer.value?.peer_id);
        // 当前打开的会话由聊天窗实时标记已读，轮询回来先清零，避免未读红点闪烁
        if (fresh) fresh.unread = 0;
        activePeer.value = fresh || null;
      }
    }
  } catch { ElMessage.error('加载会话列表失败'); }
}

async function selectConv(cv: ConversationItem) {
  activePeer.value = cv;
  // 打开即本地清零未读（MessageChat 挂载后会调 markRead 同步服务端）
  cv.unread = 0;
}

// 聊天窗标记已读后：同步列表未读数
function onPeerRead() {
  if (activePeer.value) activePeer.value.unread = 0;
}

// 会话被删除：清空当前选择并刷新列表
function onConvDeleted() {
  activePeer.value = null;
  loadConversations();
}

// ===== 私信设置 =====
const settingsVisible = ref(false);
const settingsForm = ref(true);
const settingsLoading = ref(false);
const settingsSaving = ref(false);

async function openSettings() {
  settingsVisible.value = true;
  settingsLoading.value = true;
  try {
    const res = await messageApi.getSettings();
    if (res.data.code === 0) settingsForm.value = res.data.data.require_follow_to_message;
  } catch { ElMessage.error('加载设置失败'); }
  finally { settingsLoading.value = false; }
}

async function saveSettings() {
  settingsSaving.value = true;
  try {
    const res = await messageApi.updateSettings(settingsForm.value);
    if (res.data.code === 0) {
      ElMessage.success('已保存');
      settingsVisible.value = false;
    }
  } catch { ElMessage.error('保存失败'); }
  finally { settingsSaving.value = false; }
}

onMounted(() => {
  loading.value = true;
  loadConversations().finally(() => (loading.value = false));
  // 轻量轮询刷新会话列表（未读红点/最后消息）
  pollTimer = window.setInterval(loadConversations, 15000);
});
onBeforeUnmount(() => {
  if (pollTimer) window.clearInterval(pollTimer);
});
</script>

<style scoped>
.msg-page { max-width: 960px; margin: 0 auto; padding: 24px; }
.msg-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.msg-header h2 { margin: 0; }
.header-right { display: flex; align-items: center; gap: 12px; }
.unread-total { font-size: 0.8rem; color: #f56c6c; }

.msg-layout { display: flex; gap: 14px; align-items: stretch; min-height: 560px; }
.conv-panel { width: 300px; flex-shrink: 0; display: flex; flex-direction: column; gap: 6px; max-height: 640px; overflow-y: auto; }
.conv-item {
  display: flex; align-items: center; gap: 10px; padding: 10px 12px;
  background: #fff; border: 1px solid #eee; border-radius: 10px; cursor: pointer; transition: box-shadow 0.2s;
}
.conv-item:hover { box-shadow: 0 2px 10px rgba(0, 0, 0, 0.08); }
.conv-item.active { border-color: #409eff; background: #ecf5ff; }
.conv-body { flex: 1; min-width: 0; }
.conv-top { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; }
.conv-name { font-weight: 600; font-size: 0.9rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.conv-time { font-size: 0.68rem; color: #bbb; flex-shrink: 0; }
.conv-last { font-size: 0.76rem; color: #999; margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.chat-panel { flex: 1; min-width: 0; }
.back-btn { display: none; padding-left: 0; }
.chat-empty { height: 100%; background: #fff; border: 1px solid #eee; border-radius: 10px; }

.settings-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.settings-label { font-size: 0.9rem; font-weight: 600; }
.settings-desc { font-size: 0.76rem; color: #999; font-weight: 400; margin-top: 4px; line-height: 1.5; }

/* ====== 移动端：列表与聊天窗分屏切换 ====== */
@media (max-width: 768px) {
  .msg-page { padding: 14px; }
  .msg-layout { min-height: 70vh; }
  .conv-panel { width: 100%; max-height: none; }
  .chat-panel { width: 100%; }
  .back-btn { display: inline-flex; margin-bottom: 8px; }
  .mobile-hidden { display: none !important; }
}
</style>
