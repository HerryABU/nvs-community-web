<template>
  <!--
    后台筛选条（常用 + 更多）。

    为什么需要：筛选条件多的页面（如网络诊断 9 个）一行排不下，会折成 3 行把表格顶到
    800px 以下；而绝大多数时候用户只用前两三个。这里把低频条件收进「更多筛选」，
    默认收起，展开状态按页面记住（localStorage）——换页回来不会被重置。

    用法：
      <AdminFilterBar storage-key="net-diag" :more-count="4">
        <template #primary> 常用的几个控件 </template>
        <template #more> 低频控件 </template>
        <template #actions> 查询 / 重置 / 危险操作 </template>
      </AdminFilterBar>
  -->
  <div class="afb">
    <div class="afb-row">
      <slot name="primary" />
      <el-button v-if="hasMore" text size="small" class="afb-toggle" @click="toggle">
        {{ expanded ? '收起筛选' : '更多筛选' }}
        <span v-if="!expanded && moreCount" class="afb-count">{{ moreCount }}</span>
        <el-icon class="afb-caret" :class="{ 'is-open': expanded }"><ArrowDown /></el-icon>
      </el-button>
      <div class="afb-actions">
        <slot name="actions" />
      </div>
    </div>

    <div v-if="hasMore" v-show="expanded" class="afb-row afb-more">
      <slot name="more" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, useSlots, watch } from 'vue';
import { ArrowDown } from '@element-plus/icons-vue';

const props = withDefaults(
  defineProps<{
    /** 记忆键（一般填页面 slug）；不传则不记忆 */
    storageKey?: string;
    /** 「更多筛选」里有多少个条件，仅用于角标提示 */
    moreCount?: number;
    /** 默认是否展开 */
    defaultExpanded?: boolean;
  }>(),
  { moreCount: 0, defaultExpanded: false },
);

const slots = useSlots();
const hasMore = computed(() => !!slots.more);

const lsKey = computed(() => (props.storageKey ? `nvs_admin_filters_open_${props.storageKey}` : ''));
const expanded = ref(props.defaultExpanded);

if (lsKey.value) {
  const saved = localStorage.getItem(lsKey.value);
  if (saved === '1') expanded.value = true;
  else if (saved === '0') expanded.value = false;
}

function toggle() {
  expanded.value = !expanded.value;
}

watch(expanded, (v) => {
  if (lsKey.value) localStorage.setItem(lsKey.value, v ? '1' : '0');
});
</script>

<style scoped>
.afb {
  margin-bottom: var(--space-3, 12px);
}
.afb-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-2, 8px);
}
.afb-more {
  margin-top: var(--space-2, 8px);
  padding-top: var(--space-2, 8px);
  border-top: 1px dashed var(--border-color, #e5e7eb);
}
.afb-toggle {
  color: var(--el-color-primary, #6366f1);
}
.afb-count {
  display: inline-block;
  min-width: 16px;
  margin: 0 3px;
  padding: 0 4px;
  border-radius: 8px;
  font-size: 11px;
  line-height: 16px;
  color: #fff;
  background: var(--el-color-primary, #6366f1);
}
.afb-caret { transition: transform 0.18s ease; }
.afb-caret.is-open { transform: rotate(180deg); }
/* 操作按钮靠右：查询/重置/危险操作固定在同一位置，扫视时位置稳定 */
.afb-actions {
  display: flex;
  align-items: center;
  gap: var(--space-2, 8px);
  margin-left: auto;
}
@media (max-width: 768px) {
  .afb-actions {
    margin-left: 0;
    width: 100%;
  }
}
</style>
