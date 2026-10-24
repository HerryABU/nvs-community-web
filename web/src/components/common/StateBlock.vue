<template>
  <!-- 加载态：骨架屏（比整页转圈更少"白屏感"，也不会让内容高度突然跳变） -->
  <div
    v-if="state === 'loading'"
    class="sb-skel"
    :class="`sb-skel--${skeleton}`"
    aria-busy="true"
    aria-live="polite"
  >
    <template v-if="skeleton === 'table'">
      <div class="sb-skel-head">
        <span v-for="i in 4" :key="i" class="sb-bar sb-bar--head" :style="{ width: colWidth(i) }" />
      </div>
      <div v-for="r in rows" :key="r" class="sb-skel-tr">
        <span v-for="i in 4" :key="i" class="sb-bar" :style="{ width: colWidth(i) }" />
      </div>
    </template>

    <template v-else>
      <div v-for="r in rows" :key="r" class="sb-skel-item">
        <span v-if="skeleton === 'list'" class="sb-skel-avatar" />
        <div class="sb-skel-lines">
          <span class="sb-bar" :style="{ width: lineWidth(r, 0) }" />
          <span class="sb-bar sb-bar--dim" :style="{ width: lineWidth(r, 1) }" />
        </div>
      </div>
    </template>
  </div>

  <!-- 空态 / 错误态 -->
  <div v-else class="sb-state" :class="[`sb-state--${state}`, { 'sb-state--compact': compact }]" role="status">
    <div class="sb-state-icon">
      <el-icon :size="26"><component :is="iconComp" /></el-icon>
    </div>
    <div class="sb-state-title">{{ title || fallbackTitle }}</div>
    <p v-if="description" class="sb-state-desc">{{ description }}</p>
    <div v-if="state === 'error' || actionText || $slots.actions" class="sb-state-actions">
      <el-button v-if="state === 'error'" type="primary" @click="emit('retry')">{{ retryText }}</el-button>
      <el-button v-if="actionText" :type="state === 'error' ? 'default' : 'primary'" @click="emit('action')">
        {{ actionText }}
      </el-button>
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { Box, WarningFilled, Document } from '@element-plus/icons-vue';

/**
 * StateBlock —— 统一的「加载 / 空 / 错误」三态
 *
 * 为什么要有这个组件：全站 117 个页面里，骨架屏只有 1 个、空态有 39 处 el-empty
 * 加 65 处手写"暂无/还没有"、错误态基本靠 ElMessage 一闪而过。
 * 结果是"同样一件事，每个页面长得都不一样"——这正是"粗糙"的主要来源。
 *
 * 用法（成功态由父组件自己渲染，本组件只管三种非成功态）：
 *   <StateBlock :state="state" skeleton="table" :rows="6"
 *     empty-title="还没有用户" action-text="去注册" @action="go"
 *     error-desc="请检查网络后重试" @retry="load" />
 */
const props = withDefaults(
  defineProps<{
    /** 非成功态：loading 显示骨架；empty / error 显示状态块 */
    state: 'loading' | 'empty' | 'error';
    /** 骨架形态：list 带头像行、card 纯文本行、table 表头+行 */
    skeleton?: 'list' | 'card' | 'table';
    /** 骨架行数 */
    rows?: number;
    title?: string;
    description?: string;
    actionText?: string;
    retryText?: string;
    /** 紧凑模式：内边距减半，用于卡片内嵌 */
    compact?: boolean;
  }>(),
  {
    skeleton: 'list',
    rows: 4,
    title: '',
    description: '',
    actionText: '',
    retryText: '重试',
    compact: false,
  },
);

const emit = defineEmits<{
  (e: 'retry'): void;
  (e: 'action'): void;
}>();

const fallbackTitle = computed(() =>
  props.state === 'error' ? '加载失败' : '暂无内容',
);

const iconComp = computed(() => {
  if (props.state === 'error') return WarningFilled;
  if (props.skeleton === 'table') return Document;
  return Box;
});

/** 表格骨架：列宽按 1/4 递减，避免四列等宽显得死板 */
function colWidth(i: number): string {
  return ['28%', '22%', '18%', '14%'][i - 1] || '16%';
}

/** 列表骨架：长短交替，接近真实文本的参差感 */
function lineWidth(row: number, line: number): string {
  const base = [92, 88, 95, 85][(row - 1) % 4];
  return line === 0 ? `${base}%` : `${base - 30}%`;
}
</script>

<style scoped>
/* ---------- 骨架屏 ---------- */
.sb-skel {
  padding: var(--space-4) 0;
}
.sb-skel--table {
  padding: 0;
}
.sb-skel-head {
  display: flex;
  gap: var(--space-5);
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-color);
  margin-bottom: var(--space-2);
}
.sb-skel-tr {
  display: flex;
  gap: var(--space-5);
  padding: var(--space-3) var(--space-4);
}
.sb-skel-item {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
  padding: var(--space-3) 0;
}
.sb-skel-avatar {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-full);
  flex-shrink: 0;
  background: var(--bg-soft);
  animation: sb-shimmer 1.4s ease infinite;
  background-image: linear-gradient(90deg, var(--bg-soft) 25%, var(--bg-hover) 37%, var(--bg-soft) 63%);
  background-size: 400% 100%;
}
.sb-skel-lines {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.sb-bar {
  display: block;
  height: 12px;
  border-radius: var(--radius-xs);
  background-color: var(--bg-soft);
  background-image: linear-gradient(90deg, var(--bg-soft) 25%, var(--bg-hover) 37%, var(--bg-soft) 63%);
  background-size: 400% 100%;
  animation: sb-shimmer 1.4s ease infinite;
}
.sb-bar--head {
  height: 10px;
  opacity: 0.9;
}
.sb-bar--dim {
  opacity: 0.6;
}

@keyframes sb-shimmer {
  0% { background-position: 100% 50%; }
  100% { background-position: 0 50%; }
}

/* 尊重系统"减少动效"设置：骨架停止闪烁，避免不适 */
@media (prefers-reduced-motion: reduce) {
  .sb-bar,
  .sb-skel-avatar {
    animation: none;
  }
}

/* ---------- 空态 / 错误态 ---------- */
.sb-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: var(--space-6) var(--space-4);
  gap: var(--space-2);
}
.sb-state--error .sb-state-icon {
  color: var(--el-color-danger);
  background: color-mix(in srgb, var(--el-color-danger) 10%, transparent);
}
.sb-state-icon {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-full);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  background: var(--bg-soft);
  margin-bottom: var(--space-1);
}
.sb-state-title {
  font-size: var(--text-md);
  font-weight: 600;
  color: var(--text-color);
  line-height: var(--leading-tight);
}
.sb-state-desc {
  margin: 0;
  max-width: 42ch;
  font-size: var(--text-sm);
  color: var(--text-light);
  line-height: var(--leading-normal);
}
.sb-state-actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-top: var(--space-2);
  flex-wrap: wrap;
  justify-content: center;
}

/* 紧凑：卡片内嵌时不要撑太高 */
.sb-state--compact,
.sb-state.compact {
  padding: var(--space-4) var(--space-3);
}

/* 深色模式：--bg-soft / --text-* 已在 global.css 覆写，这里只补图标底色对比 */
[data-theme='dark'] .sb-skel-avatar,
[data-theme='dark'] .sb-bar {
  background-color: var(--bg-hover);
  background-image: linear-gradient(90deg, var(--bg-hover) 25%, var(--bg-subtle) 37%, var(--bg-hover) 63%);
}
</style>
