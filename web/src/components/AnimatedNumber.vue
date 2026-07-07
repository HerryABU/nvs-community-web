<template>
  <span class="animated-number">
    <span class="animated-number-prefix" v-if="prefix">{{ prefix }}</span>
    <span class="animated-number-value" ref="numRef">{{ displayValue }}</span>
    <span class="animated-number-suffix" v-if="suffix">{{ suffix }}</span>
  </span>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';

const props = withDefaults(defineProps<{
  target: number;
  duration?: number;
  prefix?: string;
  suffix?: string;
  decimals?: number;
}>(), {
  duration: 1200,
  decimals: 0,
});

const displayValue = ref('0');
const numRef = ref<HTMLElement | null>(null);
let rafId: number | null = null;

function easeOutExpo(t: number): number {
  return t === 1 ? 1 : 1 - Math.pow(2, -10 * t);
}

function animate(from: number, to: number) {
  if (rafId !== null) {
    cancelAnimationFrame(rafId);
  }

  const startTime = performance.now();
  const duration = props.duration;

  function step(now: number) {
    const elapsed = now - startTime;
    const progress = Math.min(elapsed / duration, 1);
    const eased = easeOutExpo(progress);
    const current = from + (to - from) * eased;

    displayValue.value = current.toFixed(props.decimals);

    if (progress < 1) {
      rafId = requestAnimationFrame(step);
    }
  }

  rafId = requestAnimationFrame(step);
}

onMounted(() => {
  animate(0, props.target);
});

watch(() => props.target, (newVal, oldVal) => {
  animate(oldVal ?? 0, newVal);
});
</script>

<style scoped>
.animated-number {
  display: inline-flex;
  align-items: baseline;
  gap: 2px;
}

.animated-number-value {
  font-size: 2.2rem;
  font-weight: 800;
  line-height: 1.2;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-variant-numeric: tabular-nums;
}

.animated-number-prefix,
.animated-number-suffix {
  font-size: 1rem;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.7);
}
</style>
