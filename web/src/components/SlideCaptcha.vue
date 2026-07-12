<template>
  <div class="slide-captcha" v-if="!verified">
    <div class="sc-track" ref="trackRef">
      <div class="sc-fill" :style="{ width: sliderLeft + 'px' }"></div>
      <div
        class="sc-thumb"
        :style="{ left: sliderLeft + 'px' }"
        @mousedown="startDrag"
        @touchstart="startDrag"
        ref="thumbRef"
      >
        <el-icon><ArrowRightBold /></el-icon>
      </div>
      <span class="sc-text" :class="{ hidden: dragging }">请按住滑块拖动到最右侧</span>
    </div>
    <p class="sc-error" v-if="errorMsg">{{ errorMsg }}</p>
  </div>
  <div class="slide-captcha-success" v-else>
    <el-icon><CircleCheckFilled /></el-icon>
    <span>验证通过</span>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { ArrowRightBold, CircleCheckFilled } from '@element-plus/icons-vue';

const emit = defineEmits<{
  (e: 'verified', value: boolean): void;
}>();

const verified = ref(false);
const dragging = ref(false);
const sliderLeft = ref(0);
const errorMsg = ref('');
const trackRef = ref<HTMLElement | null>(null);
const thumbRef = ref<HTMLElement | null>(null);

let startX = 0;
let trackWidth = 0;
let thumbWidth = 0;

function getClientX(e: MouseEvent | TouchEvent): number {
  if ('touches' in e) return e.touches[0].clientX;
  return (e as MouseEvent).clientX;
}

function startDrag(e: MouseEvent | TouchEvent) {
  if (verified.value) return;
  e.preventDefault();
  dragging.value = true;
  errorMsg.value = '';
  startX = getClientX(e);
  if (trackRef.value) {
    trackWidth = trackRef.value.clientWidth;
    thumbWidth = thumbRef.value?.clientWidth || 40;
  }
  document.addEventListener('mousemove', onDrag);
  document.addEventListener('mouseup', endDrag);
  document.addEventListener('touchmove', onDrag, { passive: false });
  document.addEventListener('touchend', endDrag);
}

function onDrag(e: MouseEvent | TouchEvent) {
  if (!dragging.value) return;
  e.preventDefault();
  const delta = getClientX(e) - startX;
  const maxLeft = trackWidth - thumbWidth - 2;
  sliderLeft.value = Math.max(0, Math.min(delta, maxLeft));

  if (sliderLeft.value >= maxLeft) {
    // 验证通过
    endDrag();
    verified.value = true;
    emit('verified', true);
  }
}

function endDrag() {
  dragging.value = false;
  if (!verified.value) {
    sliderLeft.value = 0;
    errorMsg.value = '验证失败，请重试';
    setTimeout(() => { errorMsg.value = ''; }, 2000);
  }
  document.removeEventListener('mousemove', onDrag);
  document.removeEventListener('mouseup', endDrag);
  document.removeEventListener('touchmove', onDrag);
  document.removeEventListener('touchend', endDrag);
}

onUnmounted(() => {
  document.removeEventListener('mousemove', onDrag);
  document.removeEventListener('mouseup', endDrag);
  document.removeEventListener('touchmove', onDrag);
  document.removeEventListener('touchend', endDrag);
});

defineExpose({ reset: () => { verified.value = false; sliderLeft.value = 0; } });
</script>

<style scoped>
.slide-captcha {
  width: 100%;
  user-select: none;
}

.sc-track {
  position: relative;
  height: 42px;
  background: #e8e8e8;
  border-radius: 6px;
  overflow: hidden;
}

.sc-fill {
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  background: linear-gradient(90deg, #409eff, #67c23a);
  border-radius: 6px 0 0 6px;
  transition: width 0.05s;
}

.sc-thumb {
  position: absolute;
  top: 1px;
  width: 40px;
  height: 40px;
  background: #fff;
  border-radius: 5px;
  box-shadow: 0 1px 4px rgba(0,0,0,.2);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: grab;
  z-index: 2;
  color: #409eff;
}

.sc-thumb:active {
  cursor: grabbing;
}

.sc-text {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  font-size: 0.85rem;
  color: #999;
  pointer-events: none;
  transition: opacity 0.2s;
}

.sc-text.hidden {
  opacity: 0;
}

.sc-error {
  color: #f56c6c;
  font-size: 0.8rem;
  margin: 4px 0 0 0;
  text-align: center;
}

.slide-captcha-success {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #67c23a;
  font-size: 0.9rem;
  padding: 8px 0;
}

.slide-captcha-success .el-icon {
  font-size: 1.2rem;
}
</style>