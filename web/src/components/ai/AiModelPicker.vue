<template>
  <el-popover
    v-model:visible="visible"
    placement="top-start"
    :width="368"
    trigger="click"
    :disabled="disabled || !models.length"
    popper-class="ai-mp-popper"
  >
    <template #reference>
      <button type="button" class="ai-mp-trigger" :disabled="disabled || !models.length">
        <span class="ai-mp-dot" :class="{ on: !!modelValue }" />
        <span class="ai-mp-cur">{{ currentLabel }}</span>
        <span class="ai-mp-caret" :class="{ open: visible }">▾</span>
      </button>
    </template>

    <div class="ai-mp-head">
      <span class="ai-mp-title">选择模型</span>
      <span class="ai-mp-count">{{ models.length }} 个可用</span>
    </div>

    <div class="ai-mp-grid">
      <button
        v-for="m in models"
        :key="m.id"
        type="button"
        class="ai-mp-card"
        :class="{ active: m.id === modelValue }"
        @click="pick(m)"
      >
        <span class="ai-mp-name">{{ m.name }}</span>
        <span class="ai-mp-meta">
          <span v-if="m.vendor" class="ai-mp-vendor">{{ m.vendor }}</span>
          <span class="ai-mp-price" :class="{ free: isFree(m) }">{{ priceText(m) }}</span>
        </span>
        <span v-if="m.thinking" class="ai-mp-flag">思考型</span>
      </button>
    </div>
    <div v-if="!models.length" class="ai-mp-empty">当前筛选下没有可用模型</div>
  </el-popover>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';

export interface PickableModel {
  id: number;
  name: string;
  vendor?: string;
  price_in?: number;
  price_out?: number;
  thinking?: boolean;
}

const props = withDefaults(
  defineProps<{
    models: PickableModel[];
    modelValue: number;
    disabled?: boolean;
  }>(),
  { disabled: false },
);

const emit = defineEmits<{ (e: 'update:modelValue', v: number): void }>();

const visible = ref(false);

const current = computed(() => props.models.find((m) => m.id === props.modelValue));
const currentLabel = computed(() => {
  if (!props.models.length) return '暂无可用模型';
  return current.value?.name || props.models[0].name;
});

function isFree(m: PickableModel) {
  return !m.price_in && !m.price_out;
}

function priceText(m: PickableModel) {
  return isFree(m) ? '免费' : `${m.price_in ?? 0}/${m.price_out ?? 0}`;
}

function pick(m: PickableModel) {
  emit('update:modelValue', m.id);
  visible.value = false;
}
</script>

<style scoped>
.ai-mp-trigger {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--border-color, #dcdfe6);
  border-radius: 14px;
  background: var(--bg-card, #fff);
  color: var(--text-main, #303133);
  font-size: 0.8rem;
  cursor: pointer;
  max-width: 190px;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.ai-mp-trigger:hover:not(:disabled) {
  border-color: var(--el-color-primary, #409eff);
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.12);
}
.ai-mp-trigger:disabled { opacity: 0.55; cursor: not-allowed; }
.ai-mp-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--text-light, #c0c4cc); flex: none; }
.ai-mp-dot.on { background: var(--el-color-success, #67c23a); }
.ai-mp-cur { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ai-mp-caret { font-size: 0.7rem; color: var(--text-light, #909399); flex: none; }

.ai-mp-head {
  display: flex; align-items: baseline; justify-content: space-between;
  padding: 2px 2px 8px;
}
.ai-mp-title { font-size: 0.82rem; font-weight: 600; color: var(--text-main, #303133); }
.ai-mp-count { font-size: 0.72rem; color: var(--text-light, #909399); }

.ai-mp-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  max-height: 292px;
  overflow-y: auto;
  padding: 2px;
}
.ai-mp-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 9px 10px;
  text-align: left;
  border: 1px solid var(--border-color, #e4e7ed);
  border-radius: 9px;
  background: var(--bg-soft, #fafbfc);
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s, transform 0.1s;
  min-width: 0;
}
.ai-mp-card:hover { border-color: var(--el-color-primary, #409eff); background: var(--bg-card, #fff); }
.ai-mp-card:active { transform: scale(0.985); }
.ai-mp-card.active {
  border-color: var(--el-color-primary, #409eff);
  background: rgba(64, 158, 255, 0.08);
  box-shadow: inset 0 0 0 1px var(--el-color-primary, #409eff);
}
.ai-mp-name {
  font-size: 0.8rem; font-weight: 600; color: var(--text-main, #303133);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.ai-mp-meta { display: flex; align-items: center; gap: 6px; min-width: 0; }
.ai-mp-vendor {
  font-size: 0.68rem; color: var(--text-light, #909399);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 70px;
}
.ai-mp-price { font-size: 0.68rem; color: var(--el-color-warning, #e6a23c); font-variant-numeric: tabular-nums; }
.ai-mp-price.free { color: var(--el-color-success, #67c23a); }
.ai-mp-flag {
  position: absolute; top: 6px; right: 6px;
  font-size: 0.6rem; padding: 0 4px; border-radius: 4px;
  background: rgba(230, 162, 60, 0.16); color: var(--el-color-warning, #e6a23c);
}
.ai-mp-empty { padding: 16px 0; text-align: center; font-size: 0.8rem; color: var(--text-light, #909399); }
</style>
