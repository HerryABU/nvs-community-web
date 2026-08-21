<template>
  <el-button size="small" text type="warning" @click.stop="open">
    <el-icon style="margin-right:2px"><Warning /></el-icon>{{ label }}
  </el-button>

  <el-dialog v-model="visible" title="举报内容" width="560px" append-to-body @click.stop>
    <p class="report-dialog-tip">请选择最符合的举报理由，提交后管理员将及时处理。</p>
    <el-form label-position="top">
      <el-form-item label="举报理由" required>
        <div class="report-reason-grid">
          <label
            v-for="r in reasons"
            :key="r.code"
            class="report-reason-card"
            :class="{ active: reasonCode === r.code }"
            @click="reasonCode = r.code"
          >
            <span class="rr-label">{{ r.label }}</span>
            <span class="rr-desc">{{ r.desc }}</span>
          </label>
        </div>
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
import { Warning } from '@element-plus/icons-vue';
import { reportApi, type ReportReason } from '@/api/report';

const props = defineProps<{
  targetType: string; // novel / comment / thread / post / blog / forum / chapter / user / userframe / userhtml
  targetId: number;
  label?: string;
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
.report-dialog-tip {
  margin: 0 0 12px;
  font-size: 0.85rem;
  color: var(--text-light);
}
.report-reason-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
  width: 100%;
}
@media (max-width: 480px) {
  .report-reason-grid { grid-template-columns: 1fr; }
}
.report-reason-card {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 12px;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
  user-select: none;
}
.report-reason-card:hover {
  border-color: var(--primary-color);
}
.report-reason-card.active {
  border-color: var(--primary-color);
  background: rgba(var(--primary-rgb, 64, 158, 255), 0.08);
}
.rr-label {
  font-weight: 600;
  font-size: 0.88rem;
  color: var(--text-color, #333);
}
.rr-desc {
  font-size: 0.74rem;
  color: var(--text-light);
  line-height: 1.35;
}
</style>
