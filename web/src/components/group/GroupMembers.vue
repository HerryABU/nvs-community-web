<template>
  <div class="gm">
    <div class="gm-bar">
      <el-radio-group v-model="status" size="small" @change="load">
        <el-radio-button value="active">成员</el-radio-button>
        <el-radio-button v-if="canManage" value="pending">申请（{{ pendingCount }}）</el-radio-button>
      </el-radio-group>
      <div class="gm-spacer" />
      <el-button v-if="canInvite" size="small" type="primary" @click="inviteVisible = true">拉人入群</el-button>
    </div>

    <div v-loading="loading" class="gm-list">
      <div v-for="m in list" :key="m.id" class="gm-item">
        <el-avatar :size="34" :src="m.avatar_url || undefined">{{ (m.nickname || m.username || '?').slice(0, 1) }}</el-avatar>
        <div class="gm-info">
          <div class="gm-name">
            {{ m.nickname || m.username }}
            <el-tag v-if="m.role === 'owner'" size="small" type="danger" effect="plain">群主</el-tag>
            <el-tag v-else-if="m.role === 'admin'" size="small" type="warning" effect="plain">管理员</el-tag>
            <el-tag v-if="m.muted_until && isFuture(m.muted_until)" size="small" type="info" effect="plain">禁言中</el-tag>
          </div>
          <div class="gm-sub">
            <span>@{{ m.username }}</span>
            <span v-if="m.status === 'pending'">申请：{{ m.note || '未填写理由' }}</span>
            <span v-else>加入于 {{ formatDate(m.joined_at) }}</span>
          </div>
        </div>

        <div class="gm-actions">
          <template v-if="m.status === 'pending'">
            <el-button size="small" type="success" text @click="approve(m, true)">通过</el-button>
            <el-button size="small" type="info" text @click="approve(m, false)">拒绝</el-button>
          </template>
          <template v-else-if="canManage && m.role !== 'owner'">
            <el-dropdown trigger="click" @command="(cmd: string) => onCommand(cmd, m)">
              <el-button size="small" text>管理 ▾</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-if="isOwner && m.role === 'member'" command="promote">设为管理员</el-dropdown-item>
                  <el-dropdown-item v-if="isOwner && m.role === 'admin'" command="demote">取消管理员</el-dropdown-item>
                  <el-dropdown-item command="mute60">禁言 1 小时</el-dropdown-item>
                  <el-dropdown-item command="mute1440">禁言 1 天</el-dropdown-item>
                  <el-dropdown-item command="unmute">解除禁言</el-dropdown-item>
                  <el-dropdown-item command="kick" divided>移出群组</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
          <el-button
            v-else-if="m.user_id === auth.user?.id && m.role !== 'owner'"
            size="small" text type="danger" @click="leave"
          >退出</el-button>
        </div>
      </div>
      <EmptyState v-if="!loading && !list.length" icon="👤" :text="status === 'pending' ? '没有待审申请' : '还没有成员'" />
    </div>

    <!-- 拉人 -->
    <el-dialog v-model="inviteVisible" title="拉人入群" width="440px">
      <el-input v-model="inviteName" placeholder="输入用户名或昵称（支持 @用户名）" @keyup.enter="submitInvite" />
      <div class="gm-hint">对方会直接成为群成员，并收到站内通知。</div>
      <template #footer>
        <el-button @click="inviteVisible = false">取消</el-button>
        <el-button type="primary" :loading="inviting" @click="submitInvite">邀请</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { groupApi, type GroupMember } from '@/api/group';
import { useAuthStore } from '@/stores/auth';
import EmptyState from '@/components/common/EmptyState.vue';

const props = defineProps<{
  groupId: number;
  canManage: boolean;
  canInvite: boolean;
  isOwner: boolean;
  pendingCount: number;
}>();

const emit = defineEmits<{ (e: 'changed'): void }>();
const auth = useAuthStore();

const status = ref<'active' | 'pending'>('active');
const list = ref<GroupMember[]>([]);
const loading = ref(false);
const inviteVisible = ref(false);
const inviteName = ref('');
const inviting = ref(false);

function isFuture(s?: string | null) {
  return !!s && new Date(s).getTime() > Date.now();
}
function formatDate(s: string) {
  if (!s) return '';
  const d = new Date(s);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

async function load() {
  loading.value = true;
  try {
    const res = await groupApi.members(props.groupId, { status: status.value, size: 100 });
    if (res.data.code === 0) list.value = res.data.data.list || [];
  } catch { /* ignore */ }
  loading.value = false;
}

async function approve(m: GroupMember, ok: boolean) {
  try {
    await groupApi.approve(props.groupId, m.user_id, ok);
    ElMessage.success(ok ? '已通过' : '已拒绝');
    await load();
    emit('changed');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function onCommand(cmd: string, m: GroupMember) {
  try {
    if (cmd === 'promote') await groupApi.setRole(props.groupId, m.user_id, 'admin');
    else if (cmd === 'demote') await groupApi.setRole(props.groupId, m.user_id, 'member');
    else if (cmd === 'mute60') await groupApi.mute(props.groupId, m.user_id, 60);
    else if (cmd === 'mute1440') await groupApi.mute(props.groupId, m.user_id, 1440);
    else if (cmd === 'unmute') await groupApi.mute(props.groupId, m.user_id, 0);
    else if (cmd === 'kick') {
      await ElMessageBox.confirm(`确认将 ${m.nickname || m.username} 移出群组？`, '踢人确认', { type: 'warning' });
      await groupApi.removeMember(props.groupId, m.user_id);
    }
    ElMessage.success('操作成功');
    await load();
  } catch (e: any) {
    if (e === 'cancel') return;
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function leave() {
  try {
    await ElMessageBox.confirm('确认退出该群组？', '退出群组', { type: 'warning' });
  } catch { return; }
  try {
    await groupApi.leave(props.groupId, auth.user!.id);
    ElMessage.success('已退出');
    emit('changed');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '退出失败');
  }
}

async function submitInvite() {
  const name = inviteName.value.trim().replace(/^@/, '');
  if (!name) return;
  inviting.value = true;
  try {
    await groupApi.invite(props.groupId, name);
    ElMessage.success('已邀请入群');
    inviteVisible.value = false;
    inviteName.value = '';
    await load();
    emit('changed');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '邀请失败');
  }
  inviting.value = false;
}

load();
defineExpose({ reload: load, switchToPending: () => { status.value = 'pending'; load(); } });
</script>

<style scoped>
.gm-bar { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.gm-spacer { flex: 1; }
.gm-list { display: flex; flex-direction: column; gap: 6px; min-height: 100px; }
.gm-item {
  display: flex; align-items: center; gap: 10px; padding: 9px 12px;
  border: 1px solid var(--border-color, #eee); border-radius: 10px; background: var(--card-bg, #fff);
}
.gm-info { flex: 1; min-width: 0; }
.gm-name { font-size: 0.88rem; display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.gm-sub { font-size: 0.72rem; color: var(--text-light, #94a3b8); display: flex; gap: 10px; margin-top: 2px; flex-wrap: wrap; }
.gm-actions { display: flex; gap: 4px; }
.gm-hint { margin-top: 8px; font-size: 0.74rem; color: var(--text-light, #94a3b8); }
</style>
