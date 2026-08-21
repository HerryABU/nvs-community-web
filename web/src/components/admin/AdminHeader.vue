<template>
  <div class="admin-header">
    <div class="ah-left">
      <el-button class="ah-back" text @click="$router.push('/admin')">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <div class="ah-icon" :style="{ background: color || 'var(--gradient-primary)' }">
        <el-icon :size="20"><component :is="icon" /></el-icon>
      </div>
      <div class="ah-text">
        <h2 class="ah-title">{{ title }}</h2>
        <p v-if="subtitle" class="ah-subtitle">{{ subtitle }}</p>
      </div>
    </div>
    <div class="ah-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ArrowLeft } from '@element-plus/icons-vue';

defineProps<{
  title: string;
  subtitle?: string;
  icon?: any;
  color?: string;
}>();
</script>

<style scoped>
.admin-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 20px;
}
.ah-left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.ah-back {
  padding: 6px 8px;
  color: var(--text-light, #666);
}
.ah-back:hover {
  color: var(--primary-color);
}
.ah-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  flex-shrink: 0;
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.12);
}
.ah-text {
  min-width: 0;
}
.ah-title {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--text-color, #1a1a2e);
  line-height: 1.3;
}
.ah-subtitle {
  margin: 2px 0 0;
  font-size: 0.82rem;
  color: var(--text-light, #888);
}
.ah-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
[data-theme="dark"] .ah-title {
  color: #e2e8f0;
}
[data-theme="dark"] .ah-subtitle {
  color: #94a3b8;
}
[data-theme="dark"] .ah-back {
  color: #94a3b8;
}
[data-theme="dark"] .ah-back:hover {
  color: #e2e8f0;
}
</style>
