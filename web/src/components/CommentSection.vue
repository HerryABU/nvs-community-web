<template>
  <div class="comment-section">
    <!-- 发表评论 -->
    <div class="comment-form" v-if="authStore.isLoggedIn">
      <el-input
        v-model="newComment"
        type="textarea"
        :rows="3"
        placeholder="写下你的评论... 支持 **粗体** *斜体* `代码` 等 Markdown 语法（Ctrl+B 粗体 / Ctrl+I 斜体 / Ctrl+U 下划线 / Ctrl+S 发表）"
        maxlength="2000"
        show-word-limit
        @keydown="onCommentKeydown"
      />
      <div class="form-actions">
        <span class="tip-text">文明评论，理性表达</span>
        <el-button type="primary" size="small" :loading="submitting" @click="submitComment">
          发表评论
        </el-button>
      </div>
    </div>
    <div v-else class="comment-login-hint">
      <router-link to="/login">登录</router-link> 后即可发表评论
    </div>

    <!-- 新评论提示条 -->
    <div v-if="newComments > 0" class="new-comments-bar" @click="fetchComments(true)">
      🔔 有 {{ newComments }} 条新评论，点击查看
    </div>

    <!-- 评论列表（楼层模型：楼 + 楼内可折叠回复，与作品/博客/作者共用同一套） -->
    <div class="comment-list">
      <FloorComments
        :items="comments"
        :owner-id="ownerUserId"
        target-type="comment"
        :live-channel="liveChannel"
        @reply="handleReply"
        @delete="handleDelete"
        @live="onLiveComment"
        @unlocked="fetchComments(true)"
      />
    </div>

    <!-- 加载更多 -->
    <div class="load-more" v-if="hasMore">
      <el-button text :loading="loadingMore" @click="loadMore">加载更多</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { novelApi, commentApi, type Comment } from '@/api/novel';
import { getFloorPasswords } from '@/api/novel/comments';
import { novelCommentChannel, blogCommentChannel, liveApi } from '@/api/live';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';
import { shouldShowGuard } from '@/utils/sensitiveZone';
import { editorKeydownHandler, MARKDOWN_TOKENS } from '@/utils/editorShortcuts';
import FloorComments from './FloorComments.vue';

const props = defineProps<{
  novelId: number;
  chapterNumber?: number;
  novelCategory?: string;
  blogId?: number;
  ownerId?: number; // 主内容作者 id（楼层里给其评论打「楼主」标）；留空则不显示徽标
}>();

// loaded(total)：评论总数变化时通知父组件（如博客详情页统计条数）
const emit = defineEmits<{ (e: 'loaded', total: number): void }>();

const authStore = useAuthStore();

const comments = ref<Comment[]>([]);
const newComment = ref('');
const submitting = ref(false);
const page = ref(1);
const hasMore = ref(false);
const loadingMore = ref(false);

// 楼主：优先取父组件传入，其次用评论项里的主内容作者信息
const ownerUserId = computed(() => props.ownerId || 0);

// ====== 狂暴模式（F19）：实时评论 ======
// 仅当服务端开启狂暴模式后才订阅对应频道（避免对未开启站点空转重连）。
// 频道按 novel/blog 区分；章节评论只接受同章节的实时推送，跨章节仅提示刷新。
const rageEnabled = ref(false);
const liveChannel = computed(() =>
  rageEnabled.value
    ? (props.blogId ? blogCommentChannel(props.blogId) : (props.novelId ? novelCommentChannel(props.novelId) : ''))
    : '',
);

function onLiveComment(c: any) {
  if (!c || !c.id) return;
  if (props.chapterNumber !== undefined && c.chapter_number !== undefined && c.chapter_number !== props.chapterNumber) {
    newComments.value += 1; // 跨章节新评论：仅提示刷新，不混入当前列表
    return;
  }
  const seen = new Set(comments.value.map((x) => x.id));
  if (seen.has(c.id)) return; // 去重：自己刚发的也会收到推送
  comments.value = [c, ...comments.value];
  emit('loaded', comments.value.length);
}

// 楼层模型：后端返回平铺列表（本页楼层 + 这些楼层下的全部回复），
// 由 FloorComments 按 root_id 分组成「楼」，楼内回复可折叠。
async function fetchComments(reset = false) {
  if (reset) page.value = 1;
  try {
    const res = await commentApi.getComments({
      novel_id: props.blogId ? 0 : props.novelId,
      blog_id: props.blogId || 0,
      chapter_number: props.chapterNumber,
      page: page.value,
      items: comments.value, // 带上已解锁楼密码（X-Floor-Password 头）
    });
    const data = res.data.data;
    if (reset) {
      comments.value = data.list || [];
      latestTotal.value = Number(data.total) || comments.value.length;
      newComments.value = 0;
    } else {
      // 追加下一页楼层（去重：楼层分页可能出现重复 id）
      const seen = new Set(comments.value.map(c => c.id));
      comments.value.push(...(data.list || []).filter((c: Comment) => !seen.has(c.id)));
    }
    hasMore.value = Boolean(data.has_more);
    emit('loaded', Number(data.total) || comments.value.length);
  } catch (e) {
    console.error(e);
  }
}

// ====== 新评论轮询提示（30s 检测，不自动刷新打断浏览） ======
const latestTotal = ref(0);
const newComments = ref(0);
let pollTimer: ReturnType<typeof setInterval> | null = null;

async function pollNewComments() {
  try {
    const res = await commentApi.getComments({
      novel_id: props.blogId ? 0 : props.novelId,
      blog_id: props.blogId || 0,
      chapter_number: props.chapterNumber,
      page: 1,
    });
    const cur = Number(res.data.data?.total) || 0;
    if (cur > latestTotal.value) {
      newComments.value = cur - latestTotal.value;
    }
  } catch { /* 忽略 */ }
}

onMounted(() => {
  fetchComments(true);
  pollTimer = setInterval(pollNewComments, 30000);
  // 探测狂暴模式是否开启（开启才订阅实时评论，未开启则不建 SSE 连接）
  liveApi.publicConfig()
    .then((r) => { if (r.data?.enabled) rageEnabled.value = true; })
    .catch(() => { /* 探测失败按未开启处理，不影响评论区正常使用 */ });
});

onUnmounted(() => {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
});

// 跨域评论警告：敏感分区评论需两级确认（文化差异 + 法律风险）
async function crossDomainCheck(): Promise<boolean> {
  const cat = props.novelCategory;
  if (!cat) return true;

  const guard = shouldShowGuard(cat);
  if (!guard?.needed) return true;

  const key = `nvs-cross-comment-warned-${cat}`;
  if (localStorage.getItem(key) === '1') return true;

  // 第1级：文化差异提醒
  try {
    await ElMessageBox.confirm(
      `您正在「${cat}」发表评论。该区域具有特定的文化语境和社群规范，请确保您的评论尊重该区域的文化背景和读者感受。`,
      '跨区评论提醒 (1/2)',
      { confirmButtonText: '我了解，继续评论', cancelButtonText: '取消', type: 'warning' }
    );
  } catch {
    return false;
  }

  // 第2级：法律风险告知（评论场景不禁止粘贴）
  try {
    await ElMessageBox.prompt(
      `您确认在「${cat}」发表评论。请手动输入"我承诺承担法律责任"以继续：`,
      '法律风险告知 (2/2)',
      {
        confirmButtonText: '确认',
        cancelButtonText: '取消',
        inputPattern: /^我承诺承担法律责任$/,
        inputErrorMessage: '请准确输入"我承诺承担法律责任"',
        type: 'warning',
      }
    );
  } catch {
    return false;
  }

  localStorage.setItem(key, '1');
  return true;
}

async function submitComment() {
  if (!newComment.value.trim()) return;

  // 跨域评论检查
  const allowed = await crossDomainCheck();
  if (!allowed) return;

  submitting.value = true;
  try {
    await commentApi.createComment({
      novel_id: props.blogId ? 0 : props.novelId,
      blog_id: props.blogId || 0,
      chapter_number: props.chapterNumber,
      content: newComment.value.trim(),
    });
    newComment.value = '';
    ElMessage.success('评论发表成功');
    fetchComments(true);
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '发表失败');
  } finally {
    submitting.value = false;
  }
}

// 评论框快捷键：Ctrl/Cmd + B/I/U 包裹选区（Markdown），Ctrl/Cmd + S 拦截浏览器保存并直接发表
function onCommentKeydown(e: KeyboardEvent) {
  editorKeydownHandler(e, { tokens: MARKDOWN_TOKENS, onSave: () => submitComment() });
}

// parentId 为 null 表示回复主内容（开新楼）；有值表示回复楼内某条（挂靠该楼）
async function handleReply(parentId: number | null, content: string) {
  const allowed = await crossDomainCheck();
  if (!allowed) return;
  try {
    await commentApi.createComment({
      novel_id: props.blogId ? 0 : props.novelId,
      blog_id: props.blogId || 0,
      chapter_number: props.chapterNumber,
      content,
      parent_id: parentId || undefined,
    }, replyFloorHeaders(parentId));
    ElMessage.success(parentId ? '回复成功' : '评论发表成功');
    fetchComments(true);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '回复失败');
  }
}

// 加密楼内回复需带该楼解锁密码（X-Floor-Password: id:password）
function replyFloorHeaders(parentId: number | null): Record<string, string> {
  if (!parentId) return {};
  const parent = comments.value.find((c) => c.id === parentId) as any;
  if (!parent) return {};
  const floorId = parent.parent_id ? (parent.root_id ?? parent.parent_id) : parent.id;
  const pwd = getFloorPasswords()[String(floorId)];
  return pwd ? { 'X-Floor-Password': `${floorId}:${pwd}` } : {};
}

async function handleDelete(id: number) {
  try {
    await ElMessageBox.confirm('确定删除这条评论？', '删除确认', {
      confirmButtonText: '确认删除',
      type: 'warning',
    });
    await commentApi.deleteComment(id);
    ElMessage.success('已删除');
    // 删楼层主评时后端会级联删掉楼内回复，本地过滤不完整 → 直接重拉
    fetchComments(true);
  } catch {
    // 取消
  }
}

async function loadMore() {
  loadingMore.value = true;
  page.value++;
  await fetchComments(false);
  loadingMore.value = false;
}
</script>

<style scoped>
.comment-form {
  margin-bottom: 24px;
}

.form-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 8px;
}

.tip-text {
  font-size: 0.8rem;
  color: var(--text-light);
}

.comment-login-hint {
  text-align: center;
  padding: 24px;
  color: var(--text-light);
  font-size: 0.9rem;
}

.comment-list {
  margin-top: 16px;
}

.new-comments-bar {
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
.new-comments-bar:hover { background: rgba(99, 102, 241, 0.18); }

.load-more {
  text-align: center;
  margin-top: 16px;
}
</style>

<style>
[data-theme="dark"] .comment-login-hint {
  color: #94a3b8;
}
[data-theme="dark"] .tip-text {
  color: #94a3b8;
}

/* ====== 移动端评论区适配 ====== */
@media (max-width: 640px) {
  .form-actions {
    flex-wrap: wrap;
    gap: 8px;
  }
  .form-actions .el-button {
    flex: 1;
  }
  .comment-section {
    padding: 0 2px;
  }
}
</style>
