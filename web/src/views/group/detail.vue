<template>
  <div v-loading="loading" class="gd">
    <template v-if="group">
      <!-- 头部 -->
      <div class="gd-head">
        <div class="gd-title-row">
          <h1 class="gd-name">{{ group.name }}</h1>
          <el-tag size="small" effect="plain" :type="group.visibility === 'private' ? 'info' : group.visibility === 'semi' ? 'warning' : 'success'">
            {{ visLabel }}
          </el-tag>
          <el-tag v-if="group.status === 'banned'" size="small" type="danger">已封禁</el-tag>
        </div>
        <p class="gd-desc">{{ group.description || '暂无简介' }}</p>
        <div class="gd-meta">
          <span>群号 <b>{{ group.group_no }}</b></span>
          <span>{{ group.member_count }} 成员</span>
          <span>{{ group.post_count }} 帖子</span>
          <span v-if="group.owner_name">群主 {{ group.owner_name }}</span>
          <span>加入方式：{{ joinLabel }}</span>
        </div>
        <div class="gd-actions">
          <template v-if="!myRole">
            <el-button v-if="myStatus === 'pending'" size="small" disabled>申请审核中</el-button>
            <el-button v-else type="primary" size="small" :loading="joining" @click="join">加入群组</el-button>
          </template>
          <el-tag v-else size="small" type="success" effect="plain">我是{{ roleLabel(myRole) }}</el-tag>
          <span class="gd-live" :class="{ on: streamConnected }" :title="streamConnected ? '实时消息已连接' : '实时消息连接中…'">
            <i class="gd-live-dot" />{{ streamConnected ? '实时' : '连接中' }}
          </span>
          <el-button size="small" @click="qrVisible = true">群二维码</el-button>
          <el-button v-if="isOwner" size="small" text type="danger" @click="leave">退出群组</el-button>
        </div>
        <div v-if="group.announcement" class="gd-announce">
          <span class="gd-announce-tag">公告</span>
          <span class="gd-announce-text">{{ group.announcement }}</span>
        </div>
      </div>

      <!-- 私密/半公开且非成员：仅展示简介 -->
      <div v-if="!canRead && myRole !== 'owner'" class="gd-locked">
        <EmptyState icon="🔒" text="该群内容仅成员可见" :description="myStatus === 'pending' ? '你的申请正在审核中' : '加入后即可查看帖子和群文件'" />
      </div>

      <el-tabs v-else v-model="tab" class="gd-tabs">
        <el-tab-pane label="帖子" name="posts">
          <GroupPosts
            ref="postsRef"
            :group-id="group.id"
            :can-read="true"
            :can-manage="canManage"
            :ai-enabled="!!group.ai_enabled"
          />
        </el-tab-pane>
        <el-tab-pane label="成员" name="members">
          <GroupMembers
            ref="membersRef"
            :group-id="group.id"
            :can-manage="canManage"
            :can-invite="canInvite"
            :is-owner="isOwner"
            :pending-count="pendingCount"
            @changed="reload"
          />
        </el-tab-pane>
        <el-tab-pane label="文件" name="files">
          <GroupFiles ref="filesRef" :group-id="group.id" :can-manage="canManage" />
        </el-tab-pane>
        <el-tab-pane label="关于 / 设置" name="settings">
          <GroupSettings v-if="group" :group="group" :can-manage="canManage" :is-owner="isOwner" @changed="reload" />
        </el-tab-pane>
      </el-tabs>

      <el-dialog v-model="qrVisible" title="邀请加入" width="460px">
        <GroupQrCard :group="group" :invite-code="group.invite_code" />
      </el-dialog>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import { groupApi, type Group } from '@/api/group';
import { useAuthStore } from '@/stores/auth';
import { useGroupStream } from '@/composables/useGroupStream';
import EmptyState from '@/components/common/EmptyState.vue';
import GroupPosts from '@/components/group/GroupPosts.vue';
import GroupMembers from '@/components/group/GroupMembers.vue';
import GroupFiles from '@/components/group/GroupFiles.vue';
import GroupSettings from '@/components/group/GroupSettings.vue';
import GroupQrCard from '@/components/group/GroupQrCard.vue';

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();

const group = ref<Group | null>(null);
const canRead = ref(false);
const myRole = ref('');
const myStatus = ref('');
const isAdmin = ref(false);
const pendingCount = ref(0);
const loading = ref(false);
const joining = ref(false);
const qrVisible = ref(false);
const tab = ref('posts');

const postsRef = ref<any>(null);
const membersRef = ref<any>(null);
const filesRef = ref<any>(null);

const groupId = computed(() => Number(route.params.id));

const visLabel = computed(() => {
  const v = group.value?.visibility;
  return v === 'semi' ? '半公开' : v === 'private' ? '私密' : '公开';
});
const joinLabel = computed(() => {
  const m = group.value?.join_mode;
  return m === 'apply' ? '申请审核' : m === 'invite' ? '邀请码' : '直接加入';
});
const canManage = computed(() => myRole.value === 'owner' || myRole.value === 'admin' || isAdmin.value);
const isOwner = computed(() => myRole.value === 'owner');
const canInvite = computed(() => {
  if (!group.value) return false;
  return myRole.value === 'owner' || myRole.value === 'admin' || (!!myRole.value && group.value.allow_member_invite !== false);
});

function roleLabel(r: string) {
  return r === 'owner' ? '群主' : r === 'admin' ? '管理员' : '成员';
}

async function load() {
  loading.value = true;
  try {
    const res = await groupApi.detail(groupId.value);
    if (res.data.code === 0) {
      const d = res.data.data;
      group.value = d.group;
      canRead.value = d.can_read;
      myRole.value = d.my_role || '';
      myStatus.value = d.my_status || '';
      isAdmin.value = d.is_admin;
      pendingCount.value = d.pending_requests || 0;
      // 半公开群：非成员也应能读到公告与简介（已包含在 group 中）
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '群组不存在');
    router.replace('/groups');
  }
  loading.value = false;
}

async function reload() {
  await load();
  postsRef.value?.reload?.();
  membersRef.value?.reload?.();
  filesRef.value?.reload?.();
}

async function join() {
  if (!auth.isLoggedIn) {
    router.push('/login');
    return;
  }
  // 邀请码群/申请群：弹输入框
  let code = '';
  if (group.value?.join_mode !== 'open') {
    try {
      const r = await ElMessageBox.prompt(
        group.value?.join_mode === 'invite' ? '请输入邀请码' : '可以填写申请理由',
        group.value?.join_mode === 'invite' ? '邀请码加入' : '申请加入',
        { confirmButtonText: '提交', cancelButtonText: '取消' },
      );
      code = r.value || '';
    } catch { return; }
  }
  joining.value = true;
  try {
    const res = await groupApi.join(groupId.value, code);
    if (res.data.code === 0) {
      ElMessage.success(res.data.message || '已加入');
      await load();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加入失败');
  }
  joining.value = false;
}

async function leave() {
  try {
    await ElMessageBox.confirm('确认退出该群组？', '退出群组', { type: 'warning' });
  } catch { return; }
  try {
    await groupApi.leave(groupId.value, auth.user!.id);
    ElMessage.success('已退出');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '退出失败');
  }
}

// ==================== G4 实时消息（SSE） ====================
// 新帖/回复/公告/成员变动即时刷新；自己的操作不弹通知（避免自己收到自己的提醒）
const pendingReload = ref<number | null>(null);
function schedulePostsReload() {
  if (pendingReload.value) window.clearTimeout(pendingReload.value);
  pendingReload.value = window.setTimeout(() => {
    pendingReload.value = null;
    postsRef.value?.reload?.();
  }, 400);
}

const { connected: streamConnected } = useGroupStream(() => groupId.value, (name, data) => {
  const me = auth.user?.id;
  switch (name) {
    case 'post.created':
      schedulePostsReload();
      void markRead();
      if (data?.author_id !== me) {
        ElNotification({
          title: '群里有新帖',
          message: data?.title ? `${data.author || '成员'}：${data.title}` : '有新帖子',
          type: 'info',
          duration: 3500,
        });
      }
      break;
    case 'reply.created':
      schedulePostsReload();
      void markRead();
      postsRef.value?.refreshCurrent?.(data?.post_id);
      if (data?.author_id !== me && !data?.ai) {
        ElNotification({ title: '新回复', message: `${data?.author || '成员'} 回复了帖子`, type: 'info', duration: 2500 });
      }
      break;
    case 'post.reviewed':
      load();
      schedulePostsReload();
      break;
    case 'announcement':
      load();
      if (data?.preview) {
        ElNotification({ title: '群公告更新', message: data.preview, type: 'warning', duration: 4000 });
      }
      break;
    case 'member.joined':
    case 'member.left':
      load();
      break;
    default:
      break;
  }
});

// 未读：进入群详情即标记已读；页面可见期间有新消息也顺手标记
async function markRead() {
  if (!myRole.value || document.hidden) return;
  try {
    await groupApi.markRead(groupId.value);
  } catch { /* 非成员/网络异常忽略 */ }
}

onMounted(async () => {
  await load();
  await markRead();
});
</script>

<style scoped>
.gd { max-width: 980px; margin: 0 auto; padding-bottom: 40px; }
.gd-head {
  border: 1px solid var(--border-color, #e5e7eb); border-radius: 14px; padding: 18px 20px;
  background: var(--card-bg, #fff); margin-bottom: 14px;
}
.gd-title-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.gd-name { margin: 0; font-size: 1.4rem; }
.gd-desc { margin: 8px 0 10px; font-size: 0.86rem; color: var(--text-light, #64748b); line-height: 1.65; }
.gd-meta { display: flex; gap: 16px; flex-wrap: wrap; font-size: 0.76rem; color: var(--text-light, #94a3b8); }
.gd-actions { display: flex; align-items: center; gap: 8px; margin-top: 12px; flex-wrap: wrap; }
.gd-announce {
  margin-top: 12px; padding: 10px 12px; border-radius: 10px;
  background: rgba(245, 158, 11, 0.08); border: 1px solid rgba(245, 158, 11, 0.25);
  font-size: 0.82rem; line-height: 1.7;
}
.gd-announce-tag {
  display: inline-block; font-size: 0.68rem; padding: 0 6px; border-radius: 4px;
  background: #f59e0b; color: #fff; margin-right: 8px;
}
.gd-announce-text { white-space: pre-wrap; }
.gd-locked {
  border: 1px solid var(--border-color, #e5e7eb); border-radius: 14px; padding: 40px 20px;
  background: var(--card-bg, #fff);
}
.gd-tabs { margin-top: 4px; }
.gd-live {
  display: inline-flex; align-items: center; gap: 5px; font-size: 0.72rem;
  color: var(--text-light, #94a3b8); border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 999px; padding: 2px 9px;
}
.gd-live.on { color: #10b981; border-color: rgba(16, 185, 129, 0.35); background: rgba(16, 185, 129, 0.06); }
.gd-live-dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
.gd-live.on .gd-live-dot { box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.15); }
</style>
