<template>
  <div class="floor-list">
    <!-- 狂暴模式（F19）：实时连接状态徽标（仅传入 liveChannel 时展示） -->
    <div v-if="liveChannel" class="floor-live-bar">
      <LiveBadge :connected="liveConnected" />
      <span class="floor-live-tip">狂暴模式已开启，新评论将实时推送</span>
    </div>
    <div v-for="floor in floors" :key="floor.id" class="floor" :id="`floor-${floor.id}`">
      <!-- ====== 楼主位（本楼主贴） ====== -->
      <div class="floor-main">
        <div class="floor-head">
          <span class="floor-no">{{ floor.floor_no || '?' }}楼</span>
          <span class="floor-user">{{ displayName(floor) }}</span>
          <el-tag v-if="showDeleted(floor)" size="small" type="danger" effect="dark">已删除</el-tag>
          <el-tag v-if="ownerId && floor.user_id === ownerId" size="small" type="danger" effect="plain">楼主</el-tag>
          <el-tag v-if="floor.has_password" size="small" type="warning" effect="plain" class="floor-lock-tag">
            <el-icon class="lock-ico"><Lock /></el-icon> 已加密
          </el-tag>
          <span class="floor-time">{{ formatTime(floor.created_at) }}</span>
          <span class="floor-actions">
            <slot name="actions" :item="floor" />
            <el-button v-if="replyable" size="small" text @click="toggleReply(floor.id)">
              {{ replyTarget === floor.id ? '取消' : '回复' }}
            </el-button>
            <ReportButton
              v-if="targetType && authStore.isLoggedIn && authStore.user?.id !== floor.user_id"
              :target-type="targetType"
              :target-id="floor.id"
            />
            <el-button
              v-if="authStore.user?.id === floor.user_id"
              size="small" text type="danger"
              @click="$emit('delete', floor.id)"
            >删除</el-button>
            <el-button
              v-if="canMute && authStore.user?.id !== floor.user_id"
              size="small" text type="warning"
              @click="$emit('mute', floor)"
            >禁言</el-button>
            <el-button
              v-if="canModerate && authStore.user?.id !== floor.user_id"
              size="small" text type="danger"
              @click="$emit('delete', floor.id)"
            >删除</el-button>
          </span>
        </div>
        <div v-if="!floor.Locked" class="floor-content cherry-markdown cherry-previewer" data-code-block-theme="default" data-inline-code-theme="red" :class="{ 'deleted-content': showDeleted(floor) }">
          <div v-if="showDeleted(floor)" class="deleted-banner">⚠ 该内容已被删除，以下为警方取证留证（删除人：{{ floor.deleted_by === 'admin' ? '管理员' : floor.deleted_by === 'user' ? '用户自删' : '系统' }}）</div>
          <div v-else-if="floor.is_deleted && !isModerator" class="deleted-placeholder">[ 该内容已删除 ]</div>
          <div v-html="renderMarkdown(floor.content)"></div>
        </div>
        <div v-else class="floor-locked">
          <div class="locked-icon"><el-icon><Lock /></el-icon></div>
          <div class="locked-text">该楼已加密，需要密码才能查看内容</div>
          <div v-if="authStore.isLoggedIn" class="locked-input">
            <el-input
              v-model="pwdInput[floor.id]"
              type="password"
              show-password
              size="small"
              placeholder="输入访问密码"
              @keyup.enter="unlock(floor)"
            />
            <el-button size="small" type="primary" :loading="unlocking[floor.id]" @click="unlock(floor)">解锁</el-button>
          </div>
          <div v-else class="locked-hint">请先 <router-link to="/login">登录</router-link> 后输入密码查看</div>
          <el-button v-if="canSetPassword(floor)" size="small" text type="warning" @click="openManage(floor)">管理密码</el-button>
        </div>

        <!-- 楼内回复（可折叠，公众号评论区交互） -->
        <div v-if="repliesOf(floor.id).length > 0" class="floor-replies">
          <div
            v-for="r in visibleReplies(floor)"
            :key="r.id"
            class="reply"
            :id="`reply-${r.id}`"
          >
            <span class="reply-user">{{ displayName(r) }}</span>
            <span v-if="r.reply_to_name" class="reply-arrow">回复</span>
            <span v-if="r.reply_to_name" class="reply-to">{{ r.reply_to_name }}</span>
            <span class="reply-colon">：</span>
            <span class="reply-body cherry-markdown cherry-previewer" data-code-block-theme="default" data-inline-code-theme="red" :class="{ 'deleted-content': showDeleted(r) }">
              <el-tag v-if="showDeleted(r)" size="small" type="danger" effect="dark" class="reply-deleted-tag">已删除</el-tag>
              <span v-if="showDeleted(r) === false && r.is_deleted && !isModerator">[ 该内容已删除 ]</span>
              <span v-html="renderMarkdown(r.content)"></span>
            </span>
            <span class="reply-meta">
              <span class="reply-time">{{ formatTime(r.created_at) }}</span>
              <el-button v-if="replyable" size="small" text @click="toggleReply(r.id)">回复</el-button>
              <ReportButton
                v-if="targetType && authStore.isLoggedIn && authStore.user?.id !== r.user_id"
                :target-type="targetType"
                :target-id="r.id"
              />
              <el-button
                v-if="authStore.user?.id === r.user_id"
                size="small" text type="danger"
                @click="$emit('delete', r.id)"
              >删除</el-button>
              <el-button
                v-if="canModerate && authStore.user?.id !== r.user_id"
                size="small" text type="danger"
                @click="$emit('delete', r.id)"
              >删除</el-button>
            </span>
          </div>

          <div v-if="repliesOf(floor.id).length > previewCount" class="reply-toggle">
            <el-button text size="small" @click="toggleExpand(floor.id)">
              {{ expanded[floor.id] ? '收起回复' : `展开其余 ${repliesOf(floor.id).length - previewCount} 条回复` }}
            </el-button>
          </div>
        </div>

        <!-- 楼内回复输入框 -->
        <div v-if="replyTarget !== null && replyScope === floor.id" class="reply-form">
          <el-input
            v-model="replyContent"
            type="textarea"
            :rows="2"
            :placeholder="replyPlaceholder + '（Ctrl+B 粗体 / Ctrl+I 斜体 / Ctrl+U 下划线 / Ctrl+S 回复）'"
            maxlength="2000"
            size="small"
            @keydown="onReplyKeydown"
          />
          <div class="reply-form-actions">
            <el-button size="small" @click="cancelReply">取消</el-button>
            <el-button size="small" type="primary" :loading="replying" @click="submitReply">回复</el-button>
          </div>
        </div>
      </div>
    </div>

    <el-empty v-if="floors.length === 0" description="暂无内容" />

    <!-- 楼加密管理弹窗（仅楼作者 / 警方 / 管理员）：设置 / 修改 / 解除访问密码 -->
    <el-dialog v-model="manageVisible" title="楼访问密码管理" width="420px">
      <div v-if="manageTarget" class="manage-body">
        <el-alert type="info" :closable="false" show-icon title="入口限制，非内容加密">
          <template #default>密码仅控制是否返回楼内容，内容仍明文存储；警方 / 站长 / 管理员可随时查看。</template>
        </el-alert>
        <el-input
          v-model="managePassword"
          type="password"
          show-password
          class="manage-input"
          placeholder="新访问密码（至少 4 位，不可含逗号 / 冒号）"
        />
        <div class="manage-actions">
          <el-button type="primary" :disabled="!managePassword.trim()" :loading="managing" @click="saveManage">设置 / 修改密码</el-button>
          <el-button v-if="manageTarget.has_password" type="danger" text :loading="managing" @click="removeManage">解除加密</el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive } from 'vue';
import { useAuthStore } from '@/stores/auth';
import { renderMarkdown } from '@/markdown/renderer';
import ReportButton from './ReportButton.vue';
import LiveBadge from './comment/LiveBadge.vue';
import { useLiveComments } from '@/composables/useLiveComments';
import { commentApi, setFloorPassword } from '@/api/novel/comments';
import { Lock } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import { editorKeydownHandler, MARKDOWN_TOKENS } from '@/utils/editorShortcuts';

// 楼层模型（公众号评论区 + Discuz 楼中楼）：
//  - 回复主内容（作品/博客/帖子）→ 开新楼（parent_id 为空，floor_no 递增）
//  - 回复楼内任意一条 → 挂靠该楼（root_id 继承），展示为「X 回复 Y」，可折叠
// 后端返回的是平铺列表（本页楼层 + 这些楼层下的全部回复），此处按 root_id 分组还原成楼。
const props = withDefaults(defineProps<{
  items: any[];                 // 平铺列表（含楼层与楼内回复）
  ownerId?: number;             // 主内容作者 id（打「楼主」标）
  targetType?: string;          // 举报目标类型（comment / post），留空不显示举报
  replyable?: boolean;          // 是否允许回复
  canModerate?: boolean;        // 版主/管理员：可删他人内容
  canMute?: boolean;            // 版主：可禁言
  previewCount?: number;        // 楼内回复默认展示条数，超出折叠
  liveChannel?: string;         // 狂暴模式（F19）SSE 频道 key；传入即开启实时评论订阅
}>(), {
  items: () => [],
  ownerId: 0,
  targetType: '',
  replyable: true,
  canModerate: false,
  canMute: false,
  previewCount: 3,
  liveChannel: '',
});

const emit = defineEmits<{
  reply: [parentId: number | null, content: string];
  delete: [id: number];
  mute: [item: any];
  live: [item: any];            // 狂暴模式（F19）：收到服务端推送的新评论
  unlocked: [];                 // 楼加密解锁 / 改密后，通知父组件重拉列表（带解锁密码头）
}>();

// 狂暴模式（F19）：评论区实时订阅。频道为空（未开启/未传）时 composable 内部直接短路不建连接；
// 传 getter 以便频道异步就绪（确认开启后）自动重建连接。组件卸载时由 composable 自行断开。
const live = useLiveComments(() => props.liveChannel || '', (c: any) => emit('live', c));
const liveConnected = live.connected;

const authStore = useAuthStore();
const expanded = reactive<Record<number, boolean>>({});

// 警方/管理员取证视图：后端仅向这两类角色返回已删除评论（is_deleted），
// 前端据此以红字标注「已删除」内容。普通用户拿不到已删除评论，即使拿到也只显示占位符。
const isModerator = computed(() => {
  const r = authStore.user?.role;
  return r === 'admin' || r === 'police';
});
// 已删除内容是否应以红字展示（仅警方/管理员）
function showDeleted(item: any) {
  return isModerator.value && !!item?.is_deleted;
}

const floors = computed(() =>
  (props.items || []).filter(i => !i.parent_id).sort((a, b) => (a.floor_no || 0) - (b.floor_no || 0)),
);

const repliesByRoot = computed(() => {
  const map = new Map<number, any[]>();
  for (const i of props.items || []) {
    if (!i.parent_id) continue;
    const root = i.root_id ?? i.parent_id; // 兼容后端未回填 root_id 的旧数据
    if (!map.has(root)) map.set(root, []);
    map.get(root)!.push(i);
  }
  return map;
});

function repliesOf(floorId: number) {
  return repliesByRoot.value.get(floorId) || [];
}

function visibleReplies(floor: any) {
  const all = repliesOf(floor.id);
  if (expanded[floor.id] || all.length <= props.previewCount) return all;
  return all.slice(0, props.previewCount);
}

function toggleExpand(floorId: number) {
  expanded[floorId] = !expanded[floorId];
}

// ====== 回复（挂靠到被回复者所属的楼） ======
const replyTarget = ref<number | null>(null); // 被回复的那一条 id；null = 回复主内容（开新楼）
const replyScope = ref<number | null>(null);  // 输入框挂在哪一楼下展示
const replyContent = ref('');
const replying = ref(false);

const replyPlaceholder = computed(() => {
  if (!replyTarget.value) return '回复... 支持 Markdown 语法';
  const t = (props.items || []).find(i => i.id === replyTarget.value);
  return t ? `回复 ${displayName(t)}...` : '回复...';
});

function toggleReply(targetId: number) {
  if (replyTarget.value === targetId) {
    cancelReply();
    return;
  }
  replyTarget.value = targetId;
  // 输入框展示在该条所属的楼下面
  const item = (props.items || []).find(i => i.id === targetId);
  if (!item) { replyScope.value = targetId; return; }
  replyScope.value = item.parent_id ? (item.root_id ?? item.parent_id) : item.id;
  replyContent.value = '';
}

function cancelReply() {
  replyTarget.value = null;
  replyScope.value = null;
  replyContent.value = '';
}

// 楼内回复框快捷键：Ctrl/Cmd + B/I/U 包裹选区，Ctrl/Cmd + S 拦截浏览器保存并直接回复
function onReplyKeydown(e: KeyboardEvent) {
  editorKeydownHandler(e, { tokens: MARKDOWN_TOKENS, onSave: () => submitReply() });
}

async function submitReply() {
  const text = replyContent.value.trim();
  if (!text) return;
  replying.value = true;
  try {
    emit('reply', replyTarget.value, text);
    cancelReply();
  } finally {
    replying.value = false;
  }
}

function displayName(item: any) {
  return item?.user?.username || item?.username || '匿名用户';
}

function formatTime(s?: string) {
  if (!s) return '';
  const d = new Date(s);
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  if (diff < 60000) return '刚刚';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
  return d.toLocaleDateString('zh-CN');
}

// ====== 楼加密（入口限制）：🔒 占位解锁 + 作者/警方/管理员密码管理 ======
const pwdInput = reactive<Record<number, string>>({});
const unlocking = reactive<Record<number, boolean>>({});
const manageVisible = ref(false);
const manageTarget = ref<any>(null);
const managePassword = ref('');
const managing = ref(false);

// 是否可管理该楼密码：楼作者本人，或警方/管理员
function canSetPassword(floor: any) {
  return authStore.user?.id === floor.user_id || isModerator.value;
}

// 解锁：校验通过后把密码存 sessionStorage，并通知父组件重拉列表（带 X-Floor-Password 头）
async function unlock(floor: any) {
  const pwd = (pwdInput[floor.id] || '').trim();
  if (!pwd) { ElMessage.warning('请输入访问密码'); return; }
  unlocking[floor.id] = true;
  try {
    const res = await commentApi.verifyFloorPassword(floor.id, pwd);
    if (res.data?.data?.ok) {
      setFloorPassword(floor.id, pwd);
      ElMessage.success('解锁成功');
      emit('unlocked');
    } else {
      ElMessage.error(res.data?.message || '密码错误');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '解锁失败');
  } finally {
    unlocking[floor.id] = false;
  }
}

function openManage(floor: any) {
  manageTarget.value = floor;
  managePassword.value = '';
  manageVisible.value = true;
}

async function saveManage() {
  if (!manageTarget.value) return;
  const pwd = managePassword.value.trim();
  if (!pwd) { ElMessage.warning('请输入新密码'); return; }
  managing.value = true;
  try {
    const res = await commentApi.setFloorPassword(manageTarget.value.id, pwd);
    if (res.data?.data?.ok) {
      ElMessage.success(res.data.data.message || '已设置访问密码');
      manageTarget.value.has_password = true;
      manageVisible.value = false;
      emit('unlocked');
    } else {
      ElMessage.error(res.data?.message || '操作失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally {
    managing.value = false;
  }
}

async function removeManage() {
  if (!manageTarget.value) return;
  managing.value = true;
  try {
    const res = await commentApi.setFloorPassword(manageTarget.value.id, '');
    if (res.data?.data?.ok) {
      ElMessage.success('已解除加密');
      manageTarget.value.has_password = false;
      manageVisible.value = false;
      emit('unlocked');
    } else {
      ElMessage.error(res.data?.message || '操作失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally {
    managing.value = false;
  }
}
</script>

<style scoped>
.floor-list {
  /* 🔴 已删除内容标注色（警方/管理员取证视图）——颜色可调，改这一个变量即可 */
  --deleted-color: #d03050;
  --deleted-bg: rgba(208, 48, 80, 0.08);
}
.floor {
  padding: 14px 0;
  border-bottom: 1px solid #f0f0f0;
}
.floor:last-child { border-bottom: none; }

.floor-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 6px;
}

.floor-no {
  font-size: 0.72rem;
  color: #fff;
  background: var(--primary-color, #4f46e5);
  border-radius: 4px;
  padding: 1px 6px;
  flex-shrink: 0;
}

.floor-user {
  font-weight: 600;
  font-size: 0.9rem;
  color: var(--primary-color, #4f46e5);
}

.floor-time {
  font-size: 0.78rem;
  color: var(--text-light, #999);
}

.floor-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
}

.floor-content {
  font-size: 0.95rem;
  line-height: 1.7;
  white-space: normal;
}

/* ====== 警方/管理员取证视图：已删除内容红字标注 ====== */
.deleted-content {
  color: var(--deleted-color);
  background: var(--deleted-bg);
  border-left: 3px solid var(--deleted-color);
  border-radius: 4px;
  padding: 6px 10px;
}
.deleted-content :deep(a),
.deleted-content :deep(.cherry-previewer) { color: var(--deleted-color); }
.deleted-banner {
  font-size: 0.76rem;
  font-weight: 600;
  color: var(--deleted-color);
  margin-bottom: 4px;
}
.deleted-placeholder { color: var(--text-light, #999); font-style: italic; }
.reply-deleted-tag { margin-right: 4px; vertical-align: middle; }

/* ====== 楼内回复：贴左缩进 + 浅底，视觉上「属于这一楼」 ====== */
.floor-replies {
  margin-top: 8px;
  margin-left: 0;
  padding: 8px 10px;
  background: var(--bg-subtle, #f8fafc);
  border-radius: 8px;
  border-left: 2px solid #e2e8f0;
}

.reply {
  font-size: 0.88rem;
  line-height: 1.7;
  padding: 4px 0;
  word-break: break-word;
}

.reply-user {
  font-weight: 600;
  color: var(--primary-color, #4f46e5);
}

.reply-arrow {
  color: var(--text-light, #999);
  margin: 0 4px;
  font-size: 0.8rem;
}

.reply-to {
  font-weight: 600;
  color: var(--primary-color, #4f46e5);
}

.reply-colon { color: var(--text-light, #999); margin-right: 2px; }

.reply-body { white-space: normal; }

.reply-meta {
  margin-left: 6px;
  display: inline-flex;
  align-items: center;
  gap: 2px;
  vertical-align: middle;
}

.reply-time {
  font-size: 0.75rem;
  color: var(--text-light, #bbb);
  margin-right: 4px;
}

.reply-toggle {
  margin-top: 2px;
}

.reply-form {
  margin-top: 10px;
}

.reply-form-actions {
  display: flex;
  gap: 6px;
  justify-content: flex-end;
  margin-top: 6px;
}

/* ====== 楼加密（入口限制）：🔒 占位 + 解锁输入 ====== */
.floor-lock-tag {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  font-weight: 600;
}
.floor-lock-tag .lock-ico { font-size: 0.85rem; }

.floor-locked {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding: 14px 16px;
  border: 1px dashed var(--border-color, #e2e8f0);
  border-radius: 8px;
  background: var(--bg-subtle, #f8fafc);
}
.locked-icon {
  font-size: 1.4rem;
  color: var(--text-light, #94a3b8);
  line-height: 1;
}
.locked-text {
  font-size: 0.9rem;
  color: var(--text-light, #64748b);
}
.locked-input {
  display: flex;
  gap: 6px;
  width: 100%;
  max-width: 320px;
}
.locked-hint {
  font-size: 0.85rem;
  color: var(--text-light, #94a3b8);
}
.locked-hint a { color: var(--primary-color, #4f46e5); }

/* ====== 楼加密管理弹窗 ====== */
.manage-body { display: flex; flex-direction: column; gap: 12px; }
.manage-input { width: 100%; }
.manage-actions { display: flex; gap: 8px; }
</style>

<style>
[data-theme="dark"] .floor { border-bottom-color: rgba(255,255,255,.06); }
[data-theme="dark"] .floor-user { color: #e2e8f0; }
[data-theme="dark"] .floor-time { color: #94a3b8; }
[data-theme="dark"] .floor-content { color: #cbd5e1; }
[data-theme="dark"] .deleted-content { color: #f89898; border-left-color: #f89898; background: rgba(208,48,80,.15); }
[data-theme="dark"] .deleted-banner { color: #f89898; }
[data-theme="dark"] .floor-replies {
  background: rgba(255,255,255,.03);
  border-left-color: rgba(255,255,255,.1);
}
[data-theme="dark"] .reply-user,
[data-theme="dark"] .reply-to { color: #93c5fd; }
[data-theme="dark"] .reply-arrow,
[data-theme="dark"] .reply-colon { color: #94a3b8; }
[data-theme="dark"] .reply-body { color: #cbd5e1; }
[data-theme="dark"] .reply-time { color: #64748b; }

/* ====== 移动端楼层适配 ====== */
@media (max-width: 640px) {
  .floor-head { gap: 6px; }
  .floor-actions { margin-left: 0; width: 100%; }
  .floor-replies { padding: 6px 8px; }
  .floor-lock-tag { background: rgba(245, 158, 11, 0.15); border-color: rgba(245, 158, 11, 0.4); color: #fbbf24; }
  .floor-locked { background: rgba(255,255,255,.03); border-color: rgba(255,255,255,.12); }
  .locked-icon { color: #94a3b8; }
  .locked-text { color: #94a3b8; }
  .locked-hint { color: #94a3b8; }
  .locked-hint a { color: #93c5fd; }
}
</style>
