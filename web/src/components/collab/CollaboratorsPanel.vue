<template>
  <el-dialog
    :model-value="visible"
    title="协作成员与分成"
    width="820px"
    :close-on-click-modal="false"
    @update:model-value="(v: boolean) => emit('update:visible', v)"
  >
    <div v-loading="loading">
      <!-- 主笔 + 署名预览 -->
      <div class="cp-owner">
        <el-avatar :size="36">{{ ownerInitial }}</el-avatar>
        <div class="cp-owner-info">
          <div class="cp-owner-name">{{ ownerName || '主笔' }}</div>
          <div class="cp-owner-sub">主笔 · 拥有全部权限，默认署名第一位</div>
        </div>
        <div class="cp-byline">
          <span class="cp-byline-label">署名顺序</span>
          <span class="cp-byline-text">{{ bylineText }}</span>
        </div>
      </div>

      <!-- 收益分成概览 -->
      <el-alert type="info" :closable="false" class="cp-share-alert">
        <template #title>
          协作者分成合计 {{ (totalShareBP / 100).toFixed(2) }}%，主笔剩余
          {{ ((10000 - totalShareBP) / 100).toFixed(2) }}%（上限 100%）
        </template>
      </el-alert>

      <!-- 邀请 -->
      <div v-if="isOwner" class="cp-invite">
        <el-input v-model="inviteName" placeholder="用户名（站内昵称对应的登录名）" style="width: 240px" />
        <el-select v-model="inviteRole" style="width: 140px">
          <el-option v-for="r in COLLAB_ROLE_OPTIONS" :key="r.value" :label="r.label" :value="r.value" />
        </el-select>
        <el-button type="primary" :loading="inviting" @click="doInvite">发送邀请</el-button>
        <span class="cp-tip">受邀人接受后生效，未接受不授予任何权限</span>
      </div>

      <!-- 成员表 -->
      <el-table :data="members" size="small" class="cp-table">
        <el-table-column label="成员" min-width="150">
          <template #default="{ row }">
            <div class="cp-member">
              <el-avatar :size="26" :src="row.avatar_url || ''">{{ (row.nickname || row.username || '?')[0] }}</el-avatar>
              <span class="cp-member-name">{{ row.nickname || row.username || `用户${row.user_id}` }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="角色" width="110">
          <template #default="{ row }">
            <el-tag size="small">{{ roleLabel(row.role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'active' ? 'success' : 'warning'">
              {{ row.status === 'active' ? '已生效' : '待接受' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="能力" min-width="180">
          <template #default="{ row }">
            <el-tag v-if="row.can_publish" size="small" type="warning">可直接发布</el-tag>
            <el-tag v-if="row.can_review" size="small" type="success">可审稿</el-tag>
            <el-tag v-if="row.can_edit_others" size="small" type="info">可改他人章节</el-tag>
            <span v-if="!row.can_publish && !row.can_review && !row.can_edit_others" class="cp-tip">按角色默认</span>
          </template>
        </el-table-column>
        <el-table-column label="分成" width="90">
          <template #default="{ row }">{{ (row.revenue_share_bp / 100).toFixed(2) }}%</template>
        </el-table-column>
        <el-table-column label="署名次" width="80">
          <template #default="{ row }">{{ row.byline_order }}</template>
        </el-table-column>
        <el-table-column v-if="isOwner" label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openEdit(row)">设置</el-button>
            <el-popconfirm title="确认移除该协作者？历史署名保留" @confirm="doRemove(row)">
              <template #reference>
                <el-button size="small" text type="danger">移除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="还没有协作者，邀请一位一起写吧" :image-size="60" />
        </template>
      </el-table>
    </div>

    <!-- 成员设置抽屉 -->
    <el-dialog v-model="editVisible" title="协作设置" width="460px" append-to-body>
      <el-form v-if="editing" label-width="96px" size="default">
        <el-form-item label="成员">
          <span>{{ editing.nickname || editing.username || `用户${editing.user_id}` }}</span>
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="editForm.role">
            <el-option v-for="r in COLLAB_ROLE_OPTIONS" :key="r.value" :label="r.label" :value="r.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="额外能力">
          <el-checkbox v-model="editForm.can_publish" label="可直接发布章节" />
          <el-checkbox v-model="editForm.can_review" label="可审核章节" />
          <el-checkbox v-model="editForm.can_edit_others" label="可编辑他人章节" />
        </el-form-item>
        <el-form-item label="收益分成">
          <el-input-number v-model="editForm.revenue_share_bp" :min="0" :max="10000" :step="50" />
          <span class="cp-tip" style="margin-left:8px">万分比，{{ (editForm.revenue_share_bp / 100).toFixed(2) }}%</span>
        </el-form-item>
        <el-form-item label="分成备注">
          <el-input v-model="editForm.revenue_note" placeholder="如：按章节结算 / 一次性买断" maxlength="60" />
        </el-form-item>
        <el-form-item label="署名次序">
          <el-input-number v-model="editForm.byline_order" :min="2" :max="99" />
          <span class="cp-tip" style="margin-left:8px">主笔固定为 1，越小越靠前</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="doSave">保存</el-button>
      </template>
    </el-dialog>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { ElMessage } from 'element-plus';
import {
  collabApi,
  COLLAB_ROLE_OPTIONS,
  COLLAB_ROLE_LABEL,
  type Collaborator,
  type CollabRole,
} from '@/api/collab';

const props = defineProps<{ visible: boolean; novelId: number }>();
const emit = defineEmits<{ 'update:visible': [boolean] }>();

const loading = ref(false);
const members = ref<Collaborator[]>([]);
const ownerId = ref(0);
const ownerName = ref('');
const byline = ref<string[]>([]);
const isOwner = ref(false);

const ownerInitial = computed(() => (ownerName.value || '主')[0]);
const bylineText = computed(() => (byline.value.length > 0 ? byline.value.join(' · ') : ownerName.value || '主笔'));
const totalShareBP = computed(() =>
  members.value.filter((m) => m.status === 'active').reduce((s, m) => s + (m.revenue_share_bp || 0), 0),
);

function roleLabel(role: string) {
  return COLLAB_ROLE_LABEL[role] || role;
}

// ─── 邀请 ───
const inviteName = ref('');
const inviteRole = ref<CollabRole>('co_author');
const inviting = ref(false);

async function doInvite() {
  const name = inviteName.value.trim();
  if (!name) {
    ElMessage.warning('请输入用户名');
    return;
  }
  inviting.value = true;
  try {
    await collabApi.invite(props.novelId, { username: name, role: inviteRole.value });
    ElMessage.success('邀请已发送');
    inviteName.value = '';
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '邀请失败');
  } finally {
    inviting.value = false;
  }
}

// ─── 编辑 ───
const editVisible = ref(false);
const editing = ref<Collaborator | null>(null);
const saving = ref(false);
const editForm = ref({
  role: 'co_author' as CollabRole,
  can_publish: false,
  can_review: false,
  can_edit_others: false,
  revenue_share_bp: 0,
  revenue_note: '',
  byline_order: 2,
});

function openEdit(row: Collaborator) {
  editing.value = row;
  editForm.value = {
    role: (row.role as CollabRole) || 'co_author',
    can_publish: !!row.can_publish,
    can_review: !!row.can_review,
    can_edit_others: !!row.can_edit_others,
    revenue_share_bp: row.revenue_share_bp || 0,
    revenue_note: row.revenue_note || '',
    byline_order: row.byline_order || 2,
  };
  editVisible.value = true;
}

async function doSave() {
  if (!editing.value) return;
  saving.value = true;
  try {
    await collabApi.update(props.novelId, editing.value.user_id, { ...editForm.value });
    ElMessage.success('已保存');
    editVisible.value = false;
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function doRemove(row: Collaborator) {
  try {
    await collabApi.remove(props.novelId, row.user_id);
    ElMessage.success('已移除');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '移除失败');
  }
}

async function load() {
  if (!props.novelId) return;
  loading.value = true;
  try {
    const res = await collabApi.list(props.novelId);
    const d = res.data.data || ({} as any);
    members.value = d.list || [];
    ownerId.value = d.owner_id || 0;
    ownerName.value = d.owner_name || '';
    byline.value = d.byline || [];
    isOwner.value = !!d.me?.is_owner;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载协作成员失败');
  } finally {
    loading.value = false;
  }
}

watch(
  () => props.visible,
  (v) => {
    if (v) load();
  },
);
watch(() => props.novelId, () => { if (props.visible) load(); });
</script>

<style scoped>
.cp-owner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  margin-bottom: 12px;
}
.cp-owner-info {
  flex: 1;
  min-width: 0;
}
.cp-owner-name {
  font-weight: 600;
  font-size: 0.95rem;
}
.cp-owner-sub {
  font-size: 0.78rem;
  color: var(--text-muted);
}
.cp-byline {
  max-width: 46%;
  text-align: right;
}
.cp-byline-label {
  display: block;
  font-size: 0.72rem;
  color: var(--text-muted);
}
.cp-byline-text {
  font-size: 0.85rem;
  word-break: break-all;
}
.cp-share-alert {
  margin-bottom: 12px;
}
.cp-invite {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.cp-tip {
  font-size: 0.75rem;
  color: var(--text-muted);
}
.cp-member {
  display: flex;
  align-items: center;
  gap: 8px;
}
.cp-member-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cp-table :deep(.el-tag) {
  margin-right: 4px;
}
</style>
