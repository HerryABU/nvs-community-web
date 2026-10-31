<template>
  <el-popover
    ref="popRef"
    placement="top-start"
    :width="336"
    trigger="click"
    :teleported="true"
    popper-class="ai-mp-popper"
  >
    <template #reference>
      <button type="button" class="ai-mp-trigger" :disabled="disabled || !models.length">
        <span class="ai-mp-dot" :class="{ on: !!modelValue }" />
        <span class="ai-mp-cur">{{ currentLabel }}</span>
        <span class="ai-mp-caret">▾</span>
      </button>
    </template>

    <!-- 锚定下拉（不是弹窗）：模型名/厂商一眼扫完，搜索框就在上面，点完即关 -->
    <div class="ai-mp-panel">
      <div class="ai-mp-head">
        <el-input
          v-model="kw"
          size="small"
          placeholder="搜索模型 / 厂商"
          clearable
          @keydown.esc.stop="popRef?.hide()"
        />
        <span class="ai-mp-count">{{ shown.length }} / {{ models.length }}</span>
      </div>

      <div class="ai-mp-list">
        <template v-for="g in groups" :key="g.vendor">
          <div class="ai-mp-group">{{ g.vendor }}</div>
          <button
            v-for="m in g.items"
            :key="m.id"
            type="button"
            class="ai-mp-item"
            :class="{ active: m.id === modelValue }"
            @click="pick(m)"
          >
            <span class="ai-mp-name">{{ m.name }}</span>
            <span class="ai-mp-tags">
              <em v-if="m.thinking" class="ai-mp-chip think">思考型</em>
              <em class="ai-mp-chip" :class="{ free: isFree(m) }">{{ priceText(m) }}</em>
            </span>
          </button>
        </template>
        <div v-if="!shown.length" class="ai-mp-empty">没有匹配的模型</div>
      </div>
    </div>
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

const popRef = ref<any>();
const kw = ref('');

// 搜索：模型名 / 厂商 命中即可
const shown = computed(() => {
  const q = kw.value.trim().toLowerCase();
  if (!q) return props.models;
  return props.models.filter(
    (m) => m.name.toLowerCase().includes(q) || (m.vendor || '').toLowerCase().includes(q),
  );
});

/** 按厂商分组（组内保持后端给的顺序：优先级高的在前） */
const groups = computed(() => {
  const out: { vendor: string; items: PickableModel[] }[] = [];
  const idx = new Map<string, number>();
  for (const m of shown.value) {
    const v = (m.vendor || '').trim() || '其它';
    let i = idx.get(v);
    if (i === undefined) {
      i = out.length;
      idx.set(v, i);
      out.push({ vendor: v, items: [] });
    }
    out[i].items.push(m);
  }
  return out;
});

const current = computed(() => props.models.find((m) => m.id === props.modelValue));
const currentLabel = computed(() => {
  if (!props.models.length) return '暂无可用模型';
  return current.value?.name || props.models[0].name;
});

function isFree(m: PickableModel) {
  return !m.price_in && !m.price_out;
}

function priceText(m: PickableModel) {
  if (isFree(m)) return '免费';
  const inP = m.price_in ?? 0;
  const outP = m.price_out ?? 0;
  return outP ? `${inP}/${outP}` : `${inP}`;
}

function pick(m: PickableModel) {
  emit('update:modelValue', m.id);
  kw.value = '';
  popRef.value?.hide();
}
</script>

<style scoped>
.ai-mp-trigger {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 26px;
  padding: 0 9px;
  border: 1px solid var(--border-color, #dcdfe6);
  border-radius: 7px;
  background: var(--bg-card, #fff);
  color: var(--text-main, #303133);
  font-size: 0.78rem;
  cursor: pointer;
  max-width: 200px;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.ai-mp-trigger:hover:not(:disabled) {
  border-color: var(--el-color-primary, #409eff);
}
.ai-mp-trigger:disabled { opacity: 0.55; cursor: not-allowed; }
.ai-mp-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--text-light, #c0c4cc); flex: none; }
.ai-mp-dot.on { background: var(--el-color-success, #67c23a); }
.ai-mp-cur { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ai-mp-caret { font-size: 0.66rem; color: var(--text-light, #909399); flex: none; }

/* ---- 面板 ---- */
.ai-mp-panel { display: flex; flex-direction: column; gap: 8px; }
.ai-mp-head { display: flex; align-items: center; gap: 8px; }
.ai-mp-count { flex: none; font-size: 0.7rem; color: var(--text-light, #909399); font-variant-numeric: tabular-nums; }

.ai-mp-list {
  max-height: 320px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 1px;
  padding-right: 2px;
}
.ai-mp-group {
  position: sticky;
  top: 0;
  z-index: 1;
  padding: 5px 6px 3px;
  margin-top: 2px;
  font-size: 0.68rem;
  font-weight: 600;
  color: var(--text-light, #909399);
  background: var(--bg-card, #fff);
  letter-spacing: 0.02em;
}
.ai-mp-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 6px 8px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
}
.ai-mp-item:hover { background: var(--bg-soft, #f5f7fa); }
.ai-mp-item.active {
  border-color: var(--el-color-primary, #409eff);
  background: rgba(64, 158, 255, 0.08);
}
.ai-mp-name {
  flex: 1;
  min-width: 0;
  font-size: 0.78rem;
  color: var(--text-main, #303133);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ai-mp-tags { display: flex; align-items: center; gap: 4px; flex: none; }
.ai-mp-chip {
  font-style: normal;
  font-size: 0.66rem;
  line-height: 1.5;
  padding: 0 5px;
  border-radius: 4px;
  background: var(--bg-page, #eef2f7);
  color: var(--text-light, #6b7280);
  font-variant-numeric: tabular-nums;
}
.ai-mp-chip.free { background: rgba(103, 194, 58, 0.14); color: var(--el-color-success, #67c23a); }
.ai-mp-chip.think { background: rgba(230, 162, 60, 0.16); color: var(--el-color-warning, #e6a23c); }
.ai-mp-empty { padding: 16px 0; text-align: center; font-size: 0.78rem; color: var(--text-light, #909399); }
</style>
