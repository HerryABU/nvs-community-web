<template>
  <div class="gs">
    <!-- 群公告 -->
    <el-card shadow="never" class="gs-card">
      <div class="gs-card-head">
        <span>群公告</span>
        <el-button v-if="canManage" size="small" text type="primary" @click="announceVisible = true">编辑</el-button>
      </div>
      <div v-if="group.announcement" class="gs-announce">{{ group.announcement }}</div>
      <div v-else class="gs-empty">暂无公告{{ canManage ? '，点「编辑」发布第一条' : '' }}</div>
    </el-card>

    <!-- 分享二维码 -->
    <el-card shadow="never" class="gs-card">
      <div class="gs-card-head"><span>群号与二维码</span></div>
      <GroupQrCard :group="group" :invite-code="inviteCode" />
      <div v-if="inviteCode" class="gs-invite">
        邀请码：<code>{{ inviteCode }}</code>
        <el-button size="small" text type="primary" @click="reloadInvite">刷新</el-button>
      </div>
    </el-card>

    <!-- 群设置 -->
    <el-card v-if="canManage" shadow="never" class="gs-card">
      <div class="gs-card-head"><span>群设置</span></div>
      <el-form label-width="96px">
        <el-form-item label="群组名">
          <el-input v-model="form.name" maxlength="32" />
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="form.description" type="textarea" :rows="2" maxlength="500" />
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="form.category" maxlength="32" />
        </el-form-item>
        <el-form-item label="可见性">
          <el-radio-group v-model="form.visibility">
            <el-radio-button value="public">公开</el-radio-button>
            <el-radio-button value="semi">半公开</el-radio-button>
            <el-radio-button value="private">私密</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="加入方式">
          <el-radio-group v-model="form.join_mode">
            <el-radio-button value="open">直接加入</el-radio-button>
            <el-radio-button value="apply">申请审核</el-radio-button>
            <el-radio-button value="invite">邀请码</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="人数上限">
          <el-input-number v-model="form.member_limit" :min="2" :max="10000" controls-position="right" />
        </el-form-item>
        <el-form-item label="每日发帖">
          <el-input-number v-model="form.post_limit_per_day" :min="0" :max="500" controls-position="right" />
          <span class="gs-hint" style="margin-left:8px">0 = 不限（仍受全站额度约束）</span>
        </el-form-item>
        <el-form-item label="先审后发">
          <el-switch v-model="form.review_first" />
        </el-form-item>
        <el-form-item label="成员可拉人">
          <el-switch v-model="form.allow_member_invite" />
        </el-form-item>
        <el-form-item label="群内 @ai">
          <el-switch v-model="form.ai_enabled" />
        </el-form-item>
        <el-form-item v-if="form.ai_enabled" label="绑定智能体">
          <el-select v-model="form.ai_agent_id" clearable placeholder="平台默认智能体" style="width:260px">
            <el-option v-for="a in agents" :key="a.id" :label="`${a.emoji || '🤖'} ${a.name}`" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">保存设置</el-button>
        </el-form-item>
      </el-form>

      <div v-if="isOwner" class="gs-danger">
        <span>解散群组（软删除，可审计追溯）</span>
        <el-button size="small" type="danger" plain @click="dissolve">解散群组</el-button>
      </div>
    </el-card>

    <el-dialog v-model="announceVisible" title="编辑群公告" width="520px">
      <el-input v-model="announceText" type="textarea" :rows="6" maxlength="2000" show-word-limit placeholder="公告会展示在群首页，并在更新时通知全体成员" />
      <template #footer>
        <el-button @click="announceVisible = false">取消</el-button>
        <el-button type="primary" :loading="announcing" @click="saveAnnouncement">发布</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { groupApi, type Group } from '@/api/group';
import { aiHubApi, type AiAgent } from '@/api/aiHub';
import GroupQrCard from '@/components/group/GroupQrCard.vue';

const props = defineProps<{ group: Group; canManage: boolean; isOwner: boolean }>();
const emit = defineEmits<{ (e: 'changed'): void }>();
const router = useRouter();

const form = reactive({
  name: props.group.name,
  description: props.group.description || '',
  category: props.group.category || '',
  visibility: props.group.visibility,
  join_mode: props.group.join_mode,
  member_limit: props.group.member_limit,
  post_limit_per_day: props.group.post_limit_per_day,
  review_first: props.group.review_first,
  allow_member_invite: props.group.allow_member_invite !== false,
  ai_enabled: !!props.group.ai_enabled,
  ai_agent_id: props.group.ai_agent_id || undefined,
});
const saving = ref(false);
const agents = ref<AiAgent[]>([]);
const inviteCode = ref(props.group.invite_code || '');

const announceVisible = ref(false);
const announceText = ref(props.group.announcement || '');
const announcing = ref(false);

async function loadAgents() {
  if (!props.canManage) return;
  try {
    const res = await aiHubApi.agents();
    if (res.data.code === 0) agents.value = res.data.data.list || [];
  } catch { /* ignore */ }
}

async function save() {
  saving.value = true;
  try {
    await groupApi.update(props.group.id, {
      name: form.name.trim(),
      description: form.description,
      category: form.category,
      visibility: form.visibility,
      join_mode: form.join_mode,
      member_limit: form.member_limit,
      post_limit_per_day: form.post_limit_per_day,
      review_first: form.review_first,
      allow_member_invite: form.allow_member_invite,
      ai_enabled: form.ai_enabled,
      ai_agent_id: form.ai_agent_id || 0,
    });
    ElMessage.success('已保存');
    emit('changed');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  saving.value = false;
}

async function saveAnnouncement() {
  announcing.value = true;
  try {
    await groupApi.setAnnouncement(props.group.id, announceText.value);
    ElMessage.success('公告已更新');
    announceVisible.value = false;
    emit('changed');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发布失败');
  }
  announcing.value = false;
}

async function reloadInvite() {
  try {
    const res = await groupApi.inviteCode(props.group.id);
    if (res.data.code === 0) {
      inviteCode.value = res.data.data.invite_code;
      ElMessage.success('已刷新邀请码');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '获取失败');
  }
}

async function dissolve() {
  try {
    await ElMessageBox.confirm('解散后群内容将不可访问（软删除留痕），确认继续？', '解散群组', {
      type: 'warning', confirmButtonText: '确认解散', cancelButtonText: '取消',
    });
  } catch { return; }
  try {
    await groupApi.dissolve(props.group.id);
    ElMessage.success('群组已解散');
    router.push('/groups');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '解散失败');
  }
}

loadAgents();
</script>

<style scoped>
.gs { display: flex; flex-direction: column; gap: 12px; }
.gs-card { border-radius: 12px; }
.gs-card-head { display: flex; align-items: center; justify-content: space-between; font-weight: 600; font-size: 0.9rem; margin-bottom: 10px; }
.gs-announce { white-space: pre-wrap; font-size: 0.86rem; line-height: 1.75; color: var(--text-main, #334155); }
.gs-empty { font-size: 0.8rem; color: var(--text-light, #94a3b8); }
.gs-invite { margin-top: 10px; font-size: 0.78rem; color: var(--text-light, #64748b); }
.gs-invite code { background: var(--fill-light, #f1f5f9); padding: 2px 6px; border-radius: 4px; margin: 0 4px; }
.gs-hint { font-size: 0.72rem; color: var(--text-light, #94a3b8); }
.gs-danger {
  display: flex; align-items: center; justify-content: space-between; gap: 10px;
  margin-top: 8px; padding-top: 12px; border-top: 1px dashed var(--border-color, #eee);
  font-size: 0.78rem; color: #ef4444;
}
</style>
