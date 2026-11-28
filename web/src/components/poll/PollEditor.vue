<template>
  <el-dialog v-model="visible" :title="pollId ? '编辑投票' : '发起投票'" width="560px" destroy-on-close>
    <el-form :model="form" label-width="90px">
      <el-form-item label="议题" required><el-input v-model="form.question" maxlength="200" placeholder="例如：下一本书写什么题材？" /></el-form-item>
      <el-form-item label="说明"><el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选" /></el-form-item>
      <el-form-item label="选项" required>
        <div v-for="(o, i) in form.options" :key="i" class="pe-option">
          <el-input v-model="form.options[i]" :placeholder="'选项 ' + (i + 1)" />
          <el-button link type="danger" :disabled="form.options.length <= 2" @click="form.options.splice(i, 1)">删除</el-button>
        </div>
        <el-button link type="primary" :disabled="form.options.length >= 20" @click="form.options.push('')">＋ 添加选项</el-button>
      </el-form-item>
      <el-form-item label="多选"><el-switch v-model="form.multiple" /></el-form-item>
      <el-form-item label="匿名投票"><el-switch v-model="form.anonymous" /></el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { pollApi } from '@/api/poll';

const props = defineProps<{
  modelValue: boolean;
  targetKind: string;
  targetId: number;
}>();
const emit = defineEmits<{ 'update:modelValue': [v: boolean]; created: [id: number] }>();

const visible = ref(props.modelValue);
const saving = ref(false);
const pollId = ref<number | null>(null);
const form = ref({ question: '', description: '', options: ['', ''], multiple: false, anonymous: false });

watch(() => props.modelValue, (v) => { visible.value = v; });
watch(visible, (v) => { emit('update:modelValue', v); });

async function save() {
  if (!form.value.question.trim()) { ElMessage.warning('请填写议题'); return; }
  const opts = form.value.options.map(x => x.trim()).filter(Boolean);
  if (opts.length < 2) { ElMessage.warning('至少两个选项'); return; }
  saving.value = true;
  try {
    const r = await pollApi.create({
      target_kind: props.targetKind, target_id: props.targetId,
      question: form.value.question.trim(), description: form.value.description,
      multiple: form.value.multiple, anonymous: form.value.anonymous, options: opts,
    });
    const id = (r.data?.data as any)?.id;
    ElMessage.success('投票已发起');
    emit('created', id);
    visible.value = false;
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '发起失败'); }
  finally { saving.value = false; }
}
</script>

<style scoped>
.pe-option { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
</style>
