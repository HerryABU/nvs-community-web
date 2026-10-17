<template>
  <div class="empty-state">
    <div v-if="icon" class="es-icon">{{ icon }}</div>
    <p v-if="text" class="es-text">{{ text }}</p>
    <p v-if="description" class="es-desc">{{ description }}</p>
    <div v-if="$slots.actions" class="es-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  icon?: string;
  text?: string;
  description?: string;
}>();
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 20px;
  text-align: center;
  color: var(--text-light);
}
.es-icon {
  font-size: 2.5rem;
  margin-bottom: 12px;
  opacity: 0.65;
}
.es-text {
  margin: 0;
  font-size: 0.95rem;
  color: var(--text-light);
}
.es-desc {
  margin: 6px 0 0;
  font-size: 0.82rem;
  color: var(--text-muted);
  max-width: 360px;
  line-height: 1.5;
}
.es-actions {
  margin-top: 16px;
}
</style>
