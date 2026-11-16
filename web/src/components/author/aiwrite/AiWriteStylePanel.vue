<template>
  <div class="awp-style">
    <div class="awp-style-head">
      <h3>风格基调</h3>
      <span>决定 AI 生成设定/人物/正文时的语言基调；修改后对后续生成生效。</span>
    </div>
    <el-form label-width="90px" v-loading="saving">
      <el-form-item label="风格预设">
        <el-radio-group v-model="preset">
          <el-radio-button v-for="o in STYLE_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="补充要求">
        <el-input v-model="detail" type="textarea" :rows="8" maxlength="4000" show-word-limit
          placeholder="例如：叙述克制、短句为主、少用形容词；对话保留京味口语；避免网络流行语…" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="saving" @click="save">保存风格</el-button>
        <span class="awp-style-tip">当前：{{ currentLabel }}</span>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { authorAiApi, STYLE_OPTIONS, type AiWriteProject, type StylePreset } from '@/api/authorai';

const props = defineProps<{ project: AiWriteProject }>();
const emit = defineEmits<{ (e: 'saved', project: AiWriteProject): void }>();

const preset = ref<string>(props.project.style_preset || 'realism');
const detail = ref(props.project.style_detail || '');
const saving = ref(false);

const currentLabel = computed(() =>
  STYLE_OPTIONS.find((o) => o.value === props.project.style_preset)?.label || '默认风格');

watch(() => props.project.id, () => {
  preset.value = props.project.style_preset || 'realism';
  detail.value = props.project.style_detail || '';
});

async function save() {
  saving.value = true;
  try {
    const r = await authorAiApi.update(props.project.id, {
      style_preset: preset.value as StylePreset,
      style_detail: detail.value,
    });
    if (r.data.code === 0) {
      ElMessage.success('风格已保存');
      emit('saved', r.data.data);
    } else {
      ElMessage.error(r.data.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally { saving.value = false; }
}
</script>

<style scoped>
.awp-style { max-width: 760px; margin: 0 auto; padding: 20px; }
.awp-style-head { margin-bottom: 18px; }
.awp-style-head h3 { margin: 0 0 6px; font-size: 1.15rem; }
.awp-style-head span { font-size: .82rem; color: var(--text-light); }
.awp-style-tip { margin-left: 12px; font-size: .8rem; color: var(--text-light); }
</style>
