<template>
  <div class="page-container" v-loading="loading">
    <template v-if="thread">
      <div class="thread-header">
        <h1>{{ thread.title }}</h1>
        <div class="thread-meta">
          <span>{{ thread.user?.username || '匿名' }}</span>
          <span>{{ formatTime(thread.created_at) }}</span>
          <span>{{ thread.view_count }} 浏览 · {{ thread.post_count }} 回复</span>
        </div>
        <div class="thread-actions">
          <ReportButton
            v-if="authStore.isLoggedIn && authStore.user?.id !== thread.user_id"
            target-type="thread"
            :target-id="thread.id"
          />
          <el-button
            v-if="canDeleteThread"
            size="small"
            text
            type="danger"
            @click="deleteThread"
          >
            删除帖子
          </el-button>
        </div>
      </div>

      <!-- 主帖内容 -->
      <div class="thread-content cherry-markdown cherry-previewer" data-code-block-theme="default" data-inline-code-theme="red">
        <div v-html="renderMarkdown(thread.content)"></div>
      </div>

      <!-- 回复列表（楼层模型：楼 + 楼内可折叠回复，与作品/博客/作者评论区同构） -->
      <h2 class="section-title" style="margin-top: 32px">回复（{{ total }}）</h2>
      <div class="post-list">
        <FloorComments
          :items="posts"
          :owner-id="thread.user_id"
          target-type="post"
          :can-moderate="isModerator"
          :can-mute="isModerator"
          @reply="handleReply"
          @delete="deletePost"
          @mute="muteUser"
        />
      </div>

      <!-- 加载更多楼层 -->
      <div class="load-more" v-if="hasMore">
        <el-button text :loading="loadingMore" @click="loadMore">加载更多</el-button>
      </div>

      <!-- 新回复提示条 -->
      <div v-if="newReplies > 0" class="new-replies-bar" @click="refreshNewReplies">
        🔔 有 {{ newReplies }} 条新回复，点击查看
      </div>

      <!-- 回复楼主（开新楼）：不带 parent_id，脱离既有楼层 -->
      <div class="reply-form" v-if="authStore.isLoggedIn">
        <el-input v-model="replyContent" type="textarea" :rows="3" placeholder="回复楼主，另起一层... 支持 **粗体** *斜体* `代码` 等 Markdown 语法" maxlength="2000" show-word-limit />
        <div class="reply-actions">
          <el-button type="primary" :loading="replying" @click="submitReply">发表回复</el-button>
        </div>
      </div>
      <div v-else-if="!authStore.loading" class="login-hint">
        <router-link to="/login">登录</router-link> 后即可回复
      </div>
    </template>
    <!-- 敏感区隔离弹窗 -->
    <SensitiveZoneGuard
      :visible="showZoneGuard"
      :zone-name="zoneGuardName"
      :is-cross-domain="zoneGuardCross"
      @confirm="onZoneConfirmed"
      @cancel="showZoneGuard = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { novelApi, forumApi } from '@/api/novel';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';
import { renderMarkdown } from '@/markdown/renderer';
import SensitiveZoneGuard from '@/components/SensitiveZoneGuard.vue';
import FloorComments from '@/components/FloorComments.vue';
import { markZoneConfirmed, setLastZone } from '@/utils/sensitiveZone';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const loading = ref(false);
const thread = ref<any>(null);
const posts = ref<any[]>([]);
const total = ref(0);
const replyContent = ref('');
const replying = ref(false);

// 敏感区隔离
const showZoneGuard = ref(false);
const zoneGuardName = ref('');
const zoneGuardCross = ref(false);

// 论坛创建者 / 管理员权限
const isModerator = computed(() => {
  const forum = thread.value?.forum;
  if (!forum) return false;
  if (authStore.user?.role === 'admin') return true;
  return forum.creator_id && forum.creator_id === authStore.user?.id;
});
const canDeleteThread = computed(() => {
  if (!thread.value) return false;
  if (authStore.user?.role === 'admin') return true;
  return thread.value.user_id === authStore.user?.id || isModerator.value;
});

async function deleteThread() {
  try {
    await ElMessageBox.confirm('确定删除该帖子及其全部回复？', '删除确认', { type: 'warning', confirmButtonText: '删除' });
    await forumApi.deleteThread(thread.value.id);
    ElMessage.success('帖子已删除');
    router.push(`/forum/${thread.value.forum_id}`);
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function deletePost(postId: number) {
  try {
    await ElMessageBox.confirm('确定删除该回帖？', '删除确认', { type: 'warning', confirmButtonText: '删除' });
    await forumApi.deletePost(postId);
    ElMessage.success('回帖已删除');
    load();
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function muteUser(post: any) {
  try {
    const { value } = await ElMessageBox.prompt(
      `对用户「${post.user?.username || post.user_id}」禁言。请输入禁言时长（小时，0=永久）：`,
      '禁言',
      { confirmButtonText: '确认禁言', cancelButtonText: '取消', inputValue: '24', inputPattern: /^\d+$/, inputErrorMessage: '请输入数字（小时）' }
    );
    const hours = parseInt(value || '24', 10);
    await forumApi.muteUser(thread.value.forum_id, { user_id: post.user_id, duration_hours: hours, reason: '违规内容' });
    ElMessage.success('已禁言');
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

function checkSensitiveForum() {
  const forum = thread.value?.forum;
  if (!forum) return;
  if (forum.type === 'sensitive' || forum.zone) {
    zoneGuardName.value = forum.zone || forum.name;
    zoneGuardCross.value = false;
    showZoneGuard.value = true;
  }
}

function onZoneConfirmed() {
  markZoneConfirmed(zoneGuardName.value);
  setLastZone(zoneGuardName.value);
  showZoneGuard.value = false;
}

function formatTime(s: string) {
  return new Date(s).toLocaleDateString('zh-CN');
}

const page = ref(1);
const hasMore = ref(false);
const loadingMore = ref(false);

async function load(reset = true) {
  if (reset) page.value = 1;
  loading.value = true;
  try {
    const res = await forumApi.getThread(Number(route.params.id), page.value);
    const d = res.data.data;
    thread.value = d.thread;
    checkSensitiveForum();
    if (reset) {
      posts.value = d.posts || [];
    } else {
      // 追加下一页楼层（楼层分页可能重复，按 id 去重）
      const seen = new Set(posts.value.map((p: any) => p.id));
      posts.value.push(...(d.posts || []).filter((p: any) => !seen.has(p.id)));
    }
    total.value = d.total || 0;
    hasMore.value = Boolean(d.has_more);
    latestTotal.value = d.total || 0;
    newReplies.value = 0;
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

async function loadMore() {
  loadingMore.value = true;
  page.value++;
  await load(false);
  loadingMore.value = false;
}

// ====== 新回复轮询提示（30s 检测，不自动刷新打断浏览） ======
const latestTotal = ref(0);
const newReplies = ref(0);
let pollTimer: ReturnType<typeof setInterval> | null = null;

async function pollNewReplies() {
  try {
    const res = await forumApi.getThread(Number(route.params.id));
    const cur = res.data.data?.total || 0;
    if (cur > latestTotal.value) {
      newReplies.value = cur - latestTotal.value;
    }
  } catch { /* 忽略 */ }
}

function refreshNewReplies() {
  newReplies.value = 0;
  latestTotal.value = total.value;
  load();
}

onMounted(() => {
  load();
  pollTimer = setInterval(pollNewReplies, 30000);
});

onUnmounted(() => {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
});

// 底部输入框：不带 parent_id → 回复楼主，另起一层（脱离既有楼层）
async function submitReply() {
  if (!replyContent.value.trim()) return;
  replying.value = true;
  try {
    await forumApi.createPost(Number(route.params.id), { content: replyContent.value });
    ElMessage.success('回复成功');
    replyContent.value = '';
    load();
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '回复失败');
  } finally {
    replying.value = false;
  }
}

// 楼层内回复：parentId 为空=回复楼主开新楼；有值=挂靠该条所属的楼，展示为「X 回复 Y」
async function handleReply(parentId: number | null, content: string) {
  try {
    await forumApi.createPost(Number(route.params.id), {
      content,
      parent_id: parentId || undefined,
    });
    ElMessage.success(parentId ? '回复成功' : '回复成功');
    load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '回复失败');
  }
}
</script>

<style scoped>
.thread-header {
  margin-bottom: 24px;
}

.thread-header h1 {
  font-size: 1.5rem;
  color: var(--primary-color);
  margin-bottom: 8px;
}

.thread-meta {
  display: flex;
  gap: 16px;
  font-size: 0.85rem;
  color: var(--text-light);
}

.new-replies-bar {
  margin: 10px 0;
  padding: 10px 16px;
  border-radius: 8px;
  background: rgba(99, 102, 241, 0.1);
  border: 1px dashed var(--primary-color);
  color: var(--primary-color);
  font-size: 0.9rem;
  cursor: pointer;
  text-align: center;
  transition: all 0.2s ease;
}
.new-replies-bar:hover { background: rgba(99, 102, 241, 0.18); }

.thread-content {
  background: #fafafa;
  padding: 20px;
  border-radius: 8px;
  line-height: 1.8;
  white-space: pre-wrap;
  font-size: 0.95rem;
}

.post-item {
  padding: 16px 0;
  border-bottom: 1px solid #f0f0f0;
}

.post-header {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 6px;
}

.post-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
}

.thread-actions {
  margin-top: 8px;
  display: flex;
  align-items: center;
}

.post-time {
  font-size: 0.8rem;
  color: var(--text-light);
}

.post-content {
  line-height: 1.7;
  white-space: pre-wrap;
}

.reply-form {
  margin-top: 24px;
}

.reply-actions {
  margin-top: 8px;
}

.load-more {
  text-align: center;
  margin-top: 16px;
}

.login-hint {
  text-align: center;
  padding: 20px;
  color: var(--text-light);
}
</style>

<style>
[data-theme="dark"] .thread-header h1 {
  color: #e2e8f0;
}
[data-theme="dark"] .thread-meta {
  color: #94a3b8;
}
[data-theme="dark"] .thread-content {
  background: #1e293b;
  color: #cbd5e1;
}
[data-theme="dark"] .post-item {
  border-bottom-color: rgba(255,255,255,.06);
}
[data-theme="dark"] .post-header strong {
  color: #e2e8f0;
}
[data-theme="dark"] .post-time {
  color: #94a3b8;
}
[data-theme="dark"] .post-content {
  color: #cbd5e1;
}
[data-theme="dark"] .login-hint {
  color: #94a3b8;
}

/* ====== 移动端帖子页适配 ====== */
@media (max-width: 768px) {
  .thread-meta {
    flex-wrap: wrap;
    gap: 6px 12px;
  }
  .thread-header h1 {
    font-size: 1.25rem;
    line-height: 1.4;
  }
  .thread-content {
    padding: 14px;
    font-size: 0.9rem;
  }
  .post-header {
    flex-wrap: wrap;
    gap: 6px;
  }
  .post-actions {
    margin-left: 0;
    width: 100%;
    flex-wrap: wrap;
  }
  .reply-actions {
    display: flex;
  }
  .reply-actions .el-button {
    flex: 1;
  }
}
</style>
