<template>
  <div class="comment-section">
    <!-- 发表评论 -->
    <div class="comment-form" v-if="authStore.isLoggedIn">
      <div v-if="mentionCanEdit" class="mention-source-row">
        <span class="mention-source-label">@ 提及来源</span>
        <el-select
          v-model="mentionSource"
          size="small"
          :loading="mentionLoading"
          @change="onChangeMentionSource"
          style="width: 170px"
        >
          <el-option v-for="o in MENTION_SOURCE_OPTIONS" :key="o.value" :label="o.label" :value="o.value" />
        </el-select>
      </div>
      <MentionTextarea
        v-model="newComment"
        :rows="3"
        :source="mentionSource"
        :target-type="mentionTargetType"
        :target-i-d="mentionTargetID"
        :maxlength="2000"
        @save="submitComment"
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

    <!-- 工具条：计数 + 排序 + 密度 + 评论网入口（把低频入口收进工具条，减少纵向空白） -->
    <div class="comment-toolbar">
      <span class="ct-count">
        <template v-if="totalCount > 0">共 <b>{{ totalCount }}</b> 条评论 · {{ floorTotal }} 层楼</template>
        <template v-else>还没有评论</template>
      </span>
      <div class="ct-right">
        <el-radio-group v-model="sortKey" size="small" class="ct-sort" @change="onSortChange">
          <el-radio-button value="new">最新</el-radio-button>
          <el-radio-button value="old">最早</el-radio-button>
          <el-radio-button value="hot">最热</el-radio-button>
        </el-radio-group>
        <el-tooltip :content="density === 'compact' ? '切换为舒适视图' : '切换为紧凑视图'" placement="top">
          <el-button size="small" text class="ct-density" :aria-label="density === 'compact' ? '切换为舒适视图' : '切换为紧凑视图'" @click="toggleDensity">
            <el-icon><component :is="density === 'compact' ? Expand : Fold" /></el-icon>
          </el-button>
        </el-tooltip>
        <el-button size="small" text type="primary" @click="networkDrawer = true">🕸 评论网</el-button>
      </div>
    </div>

    <!-- 新评论提示条 -->
    <div v-if="newComments > 0" class="new-comments-bar" @click="fetchComments(true)">
      🔔 有 {{ newComments }} 条新评论，点击查看
    </div>

    <!-- 评论列表（楼层模型：楼 + 楼内可折叠回复，与作品/博客/作者共用同一套） -->
    <div class="comment-list" :class="`density-${density}`">
      <!-- 首次加载骨架 -->
      <div v-if="loading" class="comment-skeleton" aria-busy="true">
        <div v-for="i in 2" :key="i" class="cs-row">
          <div class="cs-line cs-line-head"></div>
          <div class="cs-line"></div>
          <div class="cs-line cs-line-short"></div>
        </div>
      </div>

      <!-- 错误态：可重试 -->
      <div v-else-if="loadError" class="comment-error">
        <p>{{ loadError }}</p>
        <el-button size="small" @click="fetchComments(true)">重试</el-button>
      </div>

      <!-- 空态：引导发言 -->
      <div v-else-if="comments.length === 0" class="comment-empty">
        <p class="ce-title">还没有评论</p>
        <p class="ce-tip">{{ authStore.isLoggedIn ? '说点什么，让作者看到你的想法。' : '登录后即可参与讨论。' }}</p>
      </div>

      <FloorComments
        v-else
        :items="comments"
        :owner-id="ownerUserId"
        target-type="comment"
        :live-channel="liveChannel"
        :mention-source="mentionSource"
        :work-type="mentionTargetType"
        :target-i-d="mentionTargetID"
        @reply="handleReply"
        @delete="handleDelete"
        @live="onLiveComment"
        @unlocked="fetchComments(true)"
      />
    </div>

    <!-- 加载更多 -->
    <div class="load-more" v-if="hasMore">
      <el-button text :loading="loadingMore" @click="loadMore">加载更多（已显示 {{ comments.length }} / {{ totalCount }}）</el-button>
    </div>

    <!-- 评论网分析抽屉 -->
    <CommentNetworkDrawer
      v-model="networkDrawer"
      :target-type="mentionTargetType"
      :target-i-d="mentionTargetID"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { commentApi, type Comment } from '@/api/novel';
import { getFloorPasswords, mentionApi, type CommentSort } from '@/api/novel/comments';
import { novelCommentChannel, blogCommentChannel, liveApi } from '@/api/live';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Fold, Expand } from '@element-plus/icons-vue';
import { shouldShowGuard } from '@/utils/sensitiveZone';
import FloorComments from './FloorComments.vue';
import MentionTextarea from './MentionTextarea.vue';
import CommentNetworkDrawer from '@/views/CommentNetworkDrawer.vue';
import type { MentionSourceInfo } from '@/api/novel/types';

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

// ── @ 提及来源（站长/楼主可配置，控制评论框 @ 联想范围） ──
const mentionTargetType = computed(() => (props.blogId ? 'blog' : 'novel'));
const mentionTargetID = computed(() => (props.blogId ? props.blogId : props.novelId));
const mentionSource = ref<string>('participants');
const mentionCanEdit = ref(false);
const mentionLoading = ref(false);
const MENTION_SOURCE_OPTIONS = [
  { value: 'participants', label: '本作品参与者' },
  { value: 'global', label: '全站用户' },
  { value: 'manual', label: '手动输入（不联想）' },
];

// 评论网分析抽屉
const networkDrawer = ref(false);

async function loadMentionSource() {
  if (!authStore.isLoggedIn) return;
  try {
    const res = await mentionApi.getSource({
      target_type: mentionTargetType.value,
      target_id: mentionTargetID.value,
    });
    const info: MentionSourceInfo | undefined = res.data?.data;
    if (info) {
      mentionSource.value = info.effective || 'participants';
      mentionCanEdit.value = !!info.can_edit;
    }
  } catch {
    /* 忽略：使用默认 participants */
  }
}

async function onChangeMentionSource(val: string) {
  if (!mentionCanEdit.value) return;
  mentionLoading.value = true;
  try {
    const res = await mentionApi.setSource({
      target_type: mentionTargetType.value,
      target_id: mentionTargetID.value,
      mode: val,
    });
    if (res.data?.data?.effective) mentionSource.value = res.data.data.effective;
    ElMessage.success('已更新 @ 提及来源');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    mentionLoading.value = false;
  }
}

const comments = ref<Comment[]>([]);
const newComment = ref('');
const submitting = ref(false);
const page = ref(1);
const hasMore = ref(false);
const loadingMore = ref(false);

// ── 列表四态：加载骨架 / 空态引导 / 错误重试 / 内容 ──
const loading = ref(true);
const loadError = ref('');
// 计数：totalCount 含楼内回复，floorTotal 为楼层数（后端 total / floor_total）
const totalCount = ref(0);
const floorTotal = ref(0);

// ── 排序与密度（浏览器记住偏好，跨页面一致）──
const SORT_STORE_KEY = 'nvs:comment-sort';
const DENSITY_STORE_KEY = 'nvs:comment-density';
type Density = 'cozy' | 'compact';

function readSortPref(): CommentSort {
  const v = localStorage.getItem(SORT_STORE_KEY);
  return v === 'old' || v === 'hot' ? v : 'new';
}
function readDensityPref(): Density {
  return localStorage.getItem(DENSITY_STORE_KEY) === 'compact' ? 'compact' : 'cozy';
}

const sortKey = ref<CommentSort>(readSortPref());
const density = ref<Density>(readDensityPref());

function onSortChange() {
  localStorage.setItem(SORT_STORE_KEY, sortKey.value);
  fetchComments(true);
}

function toggleDensity() {
  density.value = density.value === 'compact' ? 'cozy' : 'compact';
  localStorage.setItem(DENSITY_STORE_KEY, density.value);
}

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
  if (reset) {
    page.value = 1;
    loading.value = true;
    loadError.value = '';
  }
  try {
    const res = await commentApi.getComments({
      novel_id: props.blogId ? 0 : props.novelId,
      blog_id: props.blogId || 0,
      chapter_number: props.chapterNumber,
      page: page.value,
      sort: sortKey.value,
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
    totalCount.value = Number(data.total) || 0;
    floorTotal.value = Number(data.floor_total) || 0;
    hasMore.value = Boolean(data.has_more);
    emit('loaded', totalCount.value || comments.value.length);
  } catch (e: any) {
    // 首屏失败要给用户出路（重试），追加失败只提示不清空已看内容
    const msg = e?.response?.data?.message || '评论加载失败，请稍后重试';
    if (reset) loadError.value = msg;
    else ElMessage.error(msg);
    console.error(e);
  } finally {
    if (reset) loading.value = false;
  }
}

// ====== 新评论轮询提示（30s 检测，不自动刷新打断浏览） ======
const latestTotal = ref(0);
const newComments = ref(0);
let pollTimer: ReturnType<typeof setInterval> | null = null;

async function pollNewComments() {
  // 「最早」排序下用户是按时间顺序补读，弹「有新评论」会打断阅读 → 不轮询
  if (sortKey.value === 'old') return;
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
  loadMentionSource();
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

  // shouldShowGuard 是 async（内部要先拉分区配置）——漏 await 时拿到的是 Promise，
  // guard?.needed 恒为 undefined，两级确认会被静默跳过（合规门禁形同虚设）。
  const guard = await shouldShowGuard(cat);
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
// 供父组件调用：把话本气泡原文预填为引用（长按互动，gap-audit #22 的一部分）
function setQuote(text: string, speaker?: string) {
  const body = (text || '').trim();
  if (!body) return;
  // 引用格式：每行 > 前缀，首行带说话人（Markdown 引用块）
  const lines = body.split('\n');
  if (speaker) lines[0] = `${speaker}：${lines[0]}`;
  const quoted = lines.map((l) => `> ${l}`).join('\n');
  newComment.value = `${quoted}\n\n`;
  // 滚到输入框并聚焦（移动端键盘不强行拉起，交给用户点）
  const el = document.querySelector('.comment-section .comment-form');
  if (el && 'scrollIntoView' in el) el.scrollIntoView({ behavior: 'smooth', block: 'center' });
  const ta = document.querySelector('.comment-section textarea') as HTMLTextAreaElement | null;
  if (ta) setTimeout(() => ta.focus(), 200);
}

defineExpose({ setQuote });
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

.comment-network-entry {
  margin: 4px 0 10px;
}

/* ====== 工具条：计数 + 排序 + 密度 ====== */
.comment-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin: 4px 0 8px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border-color, #eef0f3);
}
.ct-count {
  font-size: 0.82rem;
  color: var(--text-light, #8a92a6);
}
.ct-count b {
  color: var(--text-color, #1f2430);
  font-weight: 600;
}
.ct-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.ct-density {
  padding: 4px 6px;
}

/* ====== 骨架 / 空态 / 错误态 ====== */
.comment-skeleton .cs-row {
  padding: 14px 12px;
  border-bottom: 1px solid var(--border-color, #f0f0f0);
}
.cs-line {
  height: 12px;
  border-radius: 6px;
  margin-bottom: 8px;
  background: linear-gradient(90deg, rgba(148, 163, 184, 0.12), rgba(148, 163, 184, 0.24), rgba(148, 163, 184, 0.12));
  background-size: 200% 100%;
  animation: cs-shimmer 1.3s ease-in-out infinite;
}
.cs-line-head { width: 32%; height: 10px; }
.cs-line-short { width: 58%; margin-bottom: 0; }
@keyframes cs-shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

.comment-empty,
.comment-error {
  text-align: center;
  padding: 28px 16px;
  border: 1px dashed var(--border-color, #e3e6ec);
  border-radius: 10px;
  background: var(--bg-card, #fbfcfe);
}
.comment-empty .ce-title {
  margin: 0 0 6px;
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-color, #1f2430);
}
.comment-empty .ce-tip {
  margin: 0;
  font-size: 0.84rem;
  color: var(--text-light, #8a92a6);
}
.comment-error p {
  margin: 0 0 12px;
  font-size: 0.86rem;
  color: var(--danger-color, #e5484d);
}

/* ====== 密度：紧凑视图（压楼间距与字号，长评论区一屏看更多） ====== */
.density-compact :deep(.floor) {
  padding: 8px 10px;
}
.density-compact :deep(.floor-head) {
  margin-bottom: 3px;
}
.density-compact :deep(.floor-content) {
  font-size: 0.9rem;
  line-height: 1.55;
}
.density-compact :deep(.floor-replies) {
  margin-top: 6px;
  padding: 5px 9px;
}
.density-compact :deep(.reply) {
  padding: 2px 0;
  font-size: 0.84rem;
  line-height: 1.55;
}
.mention-source-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.mention-source-label {
  font-size: 0.82rem;
  color: var(--text-light, #999);
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
