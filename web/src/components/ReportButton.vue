<template>
  <el-button size="small" text type="warning" @click.stop="open">
    举报
  </el-button>

  <el-dialog v-model="visible" title="举报内容" width="480px" append-to-body @click.stop>
    <el-form label-position="top">
      <el-form-item label="举报理由" required>
        <el-radio-group v-model="reasonCode" class="report-reason-group">
          <el-radio v-for="r in reasons" :key="r.code" :value="r.code" border class="report-reason-item">
            <div class="report-reason-label">{{ r.label }}</div>
            <div class="report-reason-desc">{{ r.desc }}</div>
          </el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="补充说明（选填）">
        <el-input v-model="detail" type="textarea" :rows="3" maxlength="500" show-word-limit placeholder="请简要说明举报理由" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" :disabled="!reasonCode" @click="submit">
        提交举报
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage } from 'element-plus';
import { reportApi, type ReportReason } from '@/api/report';

const props = defineProps<{
  targetType: string; // novel / comment / thread / post
  targetId: number;
}>();

const visible = ref(false);
const submitting = ref(false);
const reasons = ref<ReportReason[]>([]);
const reasonCode = ref('');
const detail = ref('');

async function open() {
  if (reasons.value.length === 0) {
    try {
      const res = await reportApi.reasons();
      if (res.data.code === 0) reasons.value = res.data.data || [];
    } catch { /* ignore */ }
    if (reasons.value.length === 0) {
      reasons.value = [{ code: 'other', label: '其他', desc: '其他需要说明的问题' }];
    }
  }
  reasonCode.value = '';
  detail.value = '';
  visible.value = true;
}

async function submit() {
  const reason = reasons.value.find(r => r.code === reasonCode.value);
  if (!reason) { ElMessage.warning('请选择举报理由'); return; }
  submitting.value = true;
  try {
    const res = await reportApi.create({
      target_type: props.targetType,
      target_id: props.targetId,
      reason_code: reason.code,
      reason: reason.label,
      detail: detail.value.trim(),
    });
    if (res.data.code === 0) {
      ElMessage.success('举报已提交，感谢您的反馈');
      visible.value = false;
    } else {
      ElMessage.warning(res.data.message || '举报失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '举报失败');
  } finally {
    submitting.value = false;
  }
}
</script>

<style scoped>
.report-reason-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}
.report-reason-item {
  width: 100%;
  height: auto;
  margin: 0;
  padding: 8px 12px;
}
.report-reason-label {
  font-weight: 600;
  font-size: 0.9rem;
}
.report-reason-desc {
  font-size: 0.78rem;
  color: var(--text-light);
  white-space: normal;
}
</style>
