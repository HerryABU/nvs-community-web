<template>
  <el-dialog
    :model-value="visible"
    :title="mode === 'strict' ? '出生日期登记（必填）' : '出生日期登记'"
    width="440px"
    :close-on-click-modal="false"
    :show-close="mode !== 'strict'"
    @update:model-value="onVisibleChange"
  >
    <div class="minor-gate">
      <el-alert
        :type="mode === 'strict' ? 'warning' : 'info'"
        :closable="false"
        show-icon
      >
        <template #title>
          {{ mode === 'strict' ? '严格保护模式已开启' : '青少年保护' }}
        </template>
        根据《未成年人网络保护条例》，本站需要了解你的年龄段，以提供相应的保护措施。
        填写后如需变更，可在「设置 → 实名与未成年保护」中修改。
      </el-alert>

      <el-form label-position="top" @submit.prevent>
        <el-form-item label="你的出生日期">
          <el-date-picker
            v-model="birthDate"
            type="date"
            value-format="YYYY-MM-DD"
            placeholder="请选择出生日期"
            format="YYYY年MM月DD日"
            :clearable="false"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>

      <p class="minor-gate__privacy">
        我们仅依据出生日期推算你的年龄段（是否未满 18 周岁），不会收集其他个人信息。
      </p>
    </div>

    <template #footer>
      <div class="minor-gate__footer">
        <el-button v-if="mode !== 'strict'" text @click="dismiss">暂不填写</el-button>
        <el-button type="primary" :loading="saving" :disabled="!birthDate" @click="submit">
          确认提交
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { ElMessage } from 'element-plus';
import { authApi } from '@/api/auth';

const props = defineProps<{
  visible: boolean;
  /** soft=可跳过（仅提醒），strict=必填（不可关闭） */
  mode: 'soft' | 'strict';
}>();
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void;
  (e: 'saved', payload: { age: number; minor_protected: boolean }): void;
  (e: 'dismiss'): void;
}>();

const birthDate = ref('');
const saving = ref(false);

function onVisibleChange(v: boolean) {
  if (props.mode === 'strict') return; // 必填模式不允许点遮罩关闭
  emit('update:visible', v);
}

function dismiss() {
  emit('dismiss');
  emit('update:visible', false);
}

async function submit() {
  if (!birthDate.value) {
    ElMessage.warning('请选择出生日期');
    return;
  }
  saving.value = true;
  try {
    const resp = await authApi.setBirthDate(birthDate.value);
    if (resp.data.code === 0) {
      ElMessage.success(resp.data.data.minor_protected ? '已登记，未成年人保护已生效' : '已登记');
      emit('saved', resp.data.data);
      emit('update:visible', false);
    } else {
      ElMessage.error(resp.data.message || '提交失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败，请稍后重试');
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.minor-gate__privacy {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.6;
}
.minor-gate__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
