<template>
  <div class="gp">
    <div class="gp-bar">
      <el-input v-model="keyword" size="small" placeholder="搜索帖子标题" clearable style="width:200px" />
      <el-switch v-if="canManage" v-model="pendingOnly" size="small" active-text="只看待审" @change="load" />
      <div class="gp-spacer" />
      <el-button v-if="canRead" type="primary" size="small" @click="composerVisible = true">
        <el-icon><EditPen /></el-icon> 发帖
      </el-button>
    </div>

    <div v-loading="loading" class="gp-list">
      <div v-for="p in filtered" :key="p.id" class="gp-item" @click="openPost(p)">
        <div class="gp-item-head">
          <span class="gp-title">{{ p.title }}</span>
          <el-tag v-if="p.status === 'pending'" size="small" type="warning" effect="plain">待审</el-tag>
          <el-tag v-if="p.ai_assisted" size="small" type="info" effect="plain">AI 辅助</el-tag>
        </div>
        <p class="gp-excerpt">{{ excerpt(p.content) }}</p>
        <div class="gp-meta">
          <span>{{ p.author_name || '匿名' }}</span>
          <span>{{ formatTime(p.created_at) }}</span>
          <span>{{ p.reply_count }} 回复</span>
        </div>
      </div>
      <EmptyState v-if="!loading && !filtered.length" icon="📝" :text="pendingOnly ? '没有待审帖子' : '还没有帖子，来发第一帖'" />
    </div>

    <!-- 发帖 -->
    <el-dialog v-model="composerVisible" title="发布帖子" width="640px">
      <el-input v-model="draft.title" maxlength="80" show-word-limit placeholder="标题（2~80 字）" style="margin-bottom:10px" />
      <el-input v-model="draft.content" type="textarea" :rows="10" maxlength="20000" show-word-limit placeholder="正文（支持 Markdown，发布后按群设置决定是否需要审核）" />
      <div v-if="aiEnabled" class="gp-aihint">💡 正文里 @ai 可以召唤群智能体回复（消耗你的 AI 额度）</div>
      <template #footer>
        <el-button @click="composerVisible = false">取消</el-button>
        <el-button type="primary" :loading="posting" @click="submitPost">发布</el-button>
      </template>
    </el-dialog>

    <!-- 帖子详情 -->
    <el-drawer v-model="postVisible" size="min(720px, 96vw)" :title="current?.title || '帖子'">
      <template v-if="current">
        <div class="gpd-meta">
          <span>{{ current.author_name }}</span>
          <span>{{ formatTime(current.created_at) }}</span>
          <el-tag v-if="current.ai_assisted" size="small" type="info" effect="plain">AI 辅助创作</el-tag>
          <div class="gp-spacer" />
          <el-button v-if="canManage && current.status === 'pending'" size="small" type="success" @click="review(true)">通过</el-button>
          <el-button v-if="canManage && current.status === 'pending'" size="small" type="warning" @click="review(false)">拒绝</el-button>
          <el-button v-if="canManage || current.user_id === auth.user?.id" size="small" type="danger" plain @click="removePost">删除</el-button>
        </div>
        <div class="gpd-body">{{ current.content }}</div>

        <div class="gpd-replies">
          <div class="gpd-reply-title">回复（{{ replies.length }}）</div>
          <div v-for="r in replies" :key="r.id" class="gpd-reply">
            <span class="gpd-reply-user">{{ r.author_name || '匿名' }}</span>
            <span class="gpd-reply-time">{{ formatTime(r.created_at) }}</span>
            <p class="gpd-reply-body">{{ r.content }}</p>
          </div>
          <EmptyState v-if="!replies.length" icon="💬" text="还没有回复" />
        </div>

        <div v-if="canRead" class="gpd-reply-box">
          <el-input v-model="replyText" type="textarea" :rows="3" maxlength="2000" show-word-limit placeholder="写下你的回复，@ai 可召唤智能体" />
          <el-button type="primary" size="small" :loading="replying" style="margin-top:8px" @click="submitReply">回复</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { EditPen } from '@element-plus/icons-vue';
import { groupApi, type GroupPost } from '@/api/group';
import { useAuthStore } from '@/stores/auth';
import EmptyState from '@/components/common/EmptyState.vue';

const props = defineProps<{
  groupId: number;
  canRead: boolean;
  canManage: boolean;
  aiEnabled: boolean;
}>();

const auth = useAuthStore();
const list = ref<GroupPost[]>([]);
const loading = ref(false);
const keyword = ref('');
const pendingOnly = ref(false);

const composerVisible = ref(false);
const posting = ref(false);
const draft = ref({ title: '', content: '' });

const postVisible = ref(false);
const current = ref<GroupPost | null>(null);
const replies = ref<any[]>([]);
const replyText = ref('');
const replying = ref(false);

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  if (!kw) return list.value;
  return list.value.filter((p) => p.title.toLowerCase().includes(kw));
});

function excerpt(s: string) {
  const t = (s || '').replace(/\s+/g, ' ').trim();
  return t.length > 120 ? t.slice(0, 120) + '…' : t;
}
function formatTime(s: string) {
  if (!s) return '';
  const d = new Date(s);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

async function load() {
  if (!props.canRead) return;
  loading.value = true;
  try {
    const res = await groupApi.posts(props.groupId, { page: 1, size: 30, pending: pendingOnly.value ? 1 : 0 });
    if (res.data.code === 0) list.value = res.data.data.list || [];
  } catch { /* ignore */ }
  loading.value = false;
}

async function submitPost() {
  if (!draft.value.title.trim() || !draft.value.content.trim()) {
    ElMessage.warning('标题和正文都不能为空');
    return;
  }
  posting.value = true;
  try {
    const res = await groupApi.createPost(props.groupId, {
      title: draft.value.title.trim(),
      content: draft.value.content,
    });
    if (res.data.code === 0) {
      ElMessage.success(res.data.message || '已发布');
      composerVisible.value = false;
      draft.value = { title: '', content: '' };
      await load();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发布失败');
  }
  posting.value = false;
}

async function openPost(p: GroupPost) {
  current.value = p;
  postVisible.value = true;
  replies.value = [];
  try {
    const res = await groupApi.replies(props.groupId, p.id, { page: 1, size: 100 });
    if (res.data.code === 0) replies.value = res.data.data.list || [];
  } catch { /* ignore */ }
}

async function review(approve: boolean) {
  if (!current.value) return;
  try {
    await groupApi.reviewPost(props.groupId, current.value.id, approve);
    ElMessage.success(approve ? '已通过' : '已拒绝');
    postVisible.value = false;
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function removePost() {
  if (!current.value) return;
  try {
    await ElMessageBox.confirm('删除后不可恢复，确认删除？', '删除帖子', { type: 'warning' });
  } catch { return; }
  try {
    await groupApi.deletePost(props.groupId, current.value.id);
    ElMessage.success('已删除');
    postVisible.value = false;
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function submitReply() {
  if (!current.value || !replyText.value.trim()) return;
  replying.value = true;
  try {
    await groupApi.createReply(props.groupId, current.value.id, replyText.value.trim());
    replyText.value = '';
    const res = await groupApi.replies(props.groupId, current.value.id, { page: 1, size: 100 });
    if (res.data.code === 0) replies.value = res.data.data.list || [];
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '回复失败');
  }
  replying.value = false;
}

defineExpose({ reload: load });
load();
</script>

<style scoped>
.gp-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; flex-wrap: wrap; }
.gp-spacer { flex: 1; }
.gp-list { display: flex; flex-direction: column; gap: 10px; min-height: 100px; }
.gp-item {
  border: 1px solid var(--border-color, #e5e7eb); border-radius: 10px; padding: 12px 14px;
  cursor: pointer; transition: border-color 0.15s, background 0.15s; background: var(--card-bg, #fff);
}
.gp-item:hover { border-color: var(--primary-color, #409eff); background: rgba(64, 158, 255, 0.03); }
.gp-item-head { display: flex; align-items: center; gap: 8px; }
.gp-title { font-weight: 600; font-size: 0.95rem; }
.gp-excerpt { margin: 6px 0; font-size: 0.8rem; color: var(--text-light, #64748b); line-height: 1.6; }
.gp-meta { display: flex; gap: 12px; font-size: 0.72rem; color: var(--text-light, #94a3b8); }
.gp-aihint { margin-top: 8px; font-size: 0.74rem; color: var(--text-light, #64748b); }
.gpd-meta { display: flex; align-items: center; gap: 10px; font-size: 0.76rem; color: var(--text-light, #64748b); padding-bottom: 10px; border-bottom: 1px solid var(--border-color, #eee); }
.gpd-body { white-space: pre-wrap; line-height: 1.85; font-size: 0.92rem; padding: 14px 0; }
.gpd-replies { margin-top: 8px; border-top: 1px solid var(--border-color, #eee); padding-top: 10px; }
.gpd-reply-title { font-size: 0.82rem; font-weight: 600; margin-bottom: 8px; }
.gpd-reply { padding: 8px 0; border-bottom: 1px dashed var(--border-color, #eee); }
.gpd-reply-user { font-size: 0.8rem; font-weight: 600; margin-right: 8px; }
.gpd-reply-time { font-size: 0.7rem; color: var(--text-light, #94a3b8); }
.gpd-reply-body { margin: 4px 0 0; font-size: 0.84rem; white-space: pre-wrap; line-height: 1.7; }
.gpd-reply-box { margin-top: 14px; }
</style>
