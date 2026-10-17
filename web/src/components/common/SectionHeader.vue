<template>
  <div class="section-header" :class="{ 'section-header--card': card }">
    <div class="sh-left">
      <div v-if="icon" class="sh-icon" :style="iconStyle">
        <component :is="icon" v-if="iconIsComponent" />
        <span v-else>{{ icon }}</span>
      </div>
      <div class="sh-text">
        <component :is="titleTag" class="sh-title">{{ title }}</component>
        <p v-if="subtitle" class="sh-subtitle">{{ subtitle }}</p>
      </div>
    </div>
    <div v-if="$slots.actions" class="sh-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, type Component } from 'vue';

const props = withDefaults(defineProps<{
  title: string;
  subtitle?: string;
  icon?: string | Component;
  gradient?: string;
  card?: boolean;
  titleTag?: string;
}>(), {
  gradient: 'primary',
  titleTag: 'h2',
});

const iconIsComponent = computed(() => typeof props.icon === 'object' && props.icon !== null);
const iconStyle = computed(() => ({
  background: `var(--gradient-${props.gradient}, var(--gradient-primary))`,
}));
</script>

<style scoped>
.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 0 0 16px;
}
.section-header--card {
  padding: 16px 20px;
  background: #fff;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-card);
  margin-bottom: 16px;
}
.sh-left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.sh-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 1.25rem;
  flex-shrink: 0;
}
.sh-text {
  min-width: 0;
}
.sh-title {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 600;
  color: var(--text-color);
  line-height: 1.3;
}
.sh-subtitle {
  margin: 2px 0 0;
  font-size: 0.82rem;
  color: var(--text-light);
  line-height: 1.4;
}
.sh-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
[data-theme="dark"] .section-header--card {
  background: #1a1d27;
}
</style>
