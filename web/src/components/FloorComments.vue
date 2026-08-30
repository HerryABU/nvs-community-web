<template>
  <div class="floor-list">
    <div v-for="floor in floors" :key="floor.id" class="floor" :id="`floor-${floor.id}`">
      <!-- ====== 楼主位（本楼主贴） ====== -->
      <div class="floor-main">
        <div class="floor-head">
          <span class="floor-no">{{ floor.floor_no || '?' }}楼</span>
          <span class="floor-user">{{ displayName(floor) }}</span>
          <el-tag v-if="ownerId && floor.user_id === ownerId" size="small" type="danger" effect="plain">楼主</el-tag>
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
        <div class="floor-content cherry-markdown cherry-previewer" data-code-block-theme="default" data-inline-code-theme="red">
          <div v-html="renderMarkdown(floor.content)"></div>
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
            <span class="reply-body cherry-markdown cherry-previewer" data-code-block-theme="default" data-inline-code-theme="red" v-html="renderMarkdown(r.content)"></span>
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
            :placeholder="replyPlaceholder"
            maxlength="2000"
            size="small"
          />
          <div class="reply-form-actions">
            <el-button size="small" @click="cancelReply">取消</el-button>
            <el-button size="small" type="primary" :loading="replying" @click="submitReply">回复</el-button>
          </div>
        </div>
      </div>
    </div>

    <el-empty v-if="floors.length === 0" description="暂无内容" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive } from 'vue';
import { useAuthStore } from '@/stores/auth';
import { renderMarkdown } from '@/markdown/renderer';
import ReportButton from './ReportButton.vue';

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
}>(), {
  items: () => [],
  ownerId: 0,
  targetType: '',
  replyable: true,
  canModerate: false,
  canMute: false,
  previewCount: 3,
});

const emit = defineEmits<{
  reply: [parentId: number | null, content: string];
  delete: [id: number];
  mute: [item: any];
}>();

const authStore = useAuthStore();
const expanded = reactive<Record<number, boolean>>({});

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
</script>

<style scoped>
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
</style>

<style>
[data-theme="dark"] .floor { border-bottom-color: rgba(255,255,255,.06); }
[data-theme="dark"] .floor-user { color: #e2e8f0; }
[data-theme="dark"] .floor-time { color: #94a3b8; }
[data-theme="dark"] .floor-content { color: #cbd5e1; }
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
}
</style>
