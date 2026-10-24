<template>
  <div class="cw-bar">
    <el-tag :type="chapterStatusType(status)" size="small" effect="dark">{{ chapterStatusLabel(status) }}</el-tag>

    <span v-if="status === 'rejected' && reviewNote" class="cw-note" :title="reviewNote">
      驳回意见：{{ reviewNote }}
    </span>

    <el-button
      v-if="canEdit && (status === 'draft' || status === 'rejected')"
      size="small"
      :loading="busy"
      @click="run('submit')"
    >
      提交审核
    </el-button>

    <el-button
      v-if="me?.can_publish && status !== 'published'"
      size="small"
      type="success"
      :loading="busy"
      @click="run('publish')"
    >
      直接发布
    </el-button>

    <el-button
      v-if="me?.can_review && status === 'pending'"
      size="small"
      type="warning"
      :loading="busy"
      @click="reviewVisible = true"
    >
      审核
    </el-button>

    <el-button
      v-if="status === 'published'"
      size="small"
      type="info"
      :loading="busy"
      @click="run('withdraw')"
    >
      撤回为草稿
    </el-button>
  </div>

  <!-- 审核抽屉：通过即发布，驳回需填意见 -->
  <el-dialog v-model="reviewVisible" title="章节审核" width="420px">
    <el-form label-width="72px">
      <el-form-item label="审核意见">
        <el-input
          v-model="note"
          type="textarea"
          :rows="4"
          maxlength="200"
          show-word-limit
          placeholder="驳回时建议写明原因（200 字内）"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="reviewVisible = false">取消</el-button>
      <el-button type="danger" :loading="busy" @click="doReview(false)">驳回</el-button>
      <el-button type="success" :loading="busy" @click="doReview(true)">通过并发布</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage } from 'element-plus';
import { chapterWorkflowApi, chapterStatusLabel, chapterStatusType, type CollabMe } from '@/api/collab';

const props = defineProps<{
  novelId: number;
  chapterNum: number;
  status: string;
  me?: CollabMe | null;
  canEdit?: boolean;
  reviewNote?: string;
}>();

const emit = defineEmits<{ changed: [string] }>();

const busy = ref(false);
const reviewVisible = ref(false);
const note = ref('');

async function run(action: 'submit' | 'publish' | 'withdraw') {
  busy.value = true;
  try {
    if (action === 'submit') await chapterWorkflowApi.submit(props.novelId, props.chapterNum);
    else if (action === 'publish') await chapterWorkflowApi.publish(props.novelId, props.chapterNum);
    else await chapterWorkflowApi.withdraw(props.novelId, props.chapterNum);
    const next = action === 'submit' ? 'pending' : action === 'publish' ? 'published' : 'draft';
    ElMessage.success(
      action === 'submit' ? '已提交审核' : action === 'publish' ? '已发布' : '已撤回为草稿',
    );
    emit('changed', next);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally {
    busy.value = false;
  }
}

async function doReview(approve: boolean) {
  if (!approve && !note.value.trim()) {
    ElMessage.warning('驳回请填写意见');
    return;
  }
  busy.value = true;
  try {
    await chapterWorkflowApi.review(props.novelId, props.chapterNum, { approve, note: note.value });
    ElMessage.success(approve ? '已通过并发布' : '已驳回');
    reviewVisible.value = false;
    note.value = '';
    emit('changed', approve ? 'published' : 'rejected');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '审核失败');
  } finally {
    busy.value = false;
  }
}
</script>

<style scoped>
.cw-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.cw-note {
  font-size: 0.75rem;
  color: var(--text-muted);
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
