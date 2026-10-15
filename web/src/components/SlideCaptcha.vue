<template>
  <div class="slide-puzzle" :class="{ 'is-verified': verified, 'is-error': !!errorMsg }">
    <!-- 拼图舞台：背景缺口图 + 可拖动拼图块（1:1 像素，保证上报 x 与服务端 correctX 同坐标系） -->
    <div class="sp-stage" ref="stageRef" :style="{ width: stageW + 'px', height: stageH + 'px' }">
      <img
        class="sp-bg"
        :src="bgImage"
        :width="stageW"
        :height="stageH"
        alt="滑块验证背景"
        draggable="false"
      />
      <img
        v-if="!verified"
        class="sp-piece"
        :src="pieceImage"
        :style="{ left: sliderLeft + 'px', top: pieceTop + 'px', width: pieceSize + 'px', height: pieceSize + 'px' }"
        alt="拼图块"
        draggable="false"
        @pointerdown.prevent="onDown"
      />
      <div v-if="verified" class="sp-ok-mask">
        <el-icon><CircleCheckFilled /></el-icon>
        <span>验证通过</span>
      </div>
      <span v-if="!verified && !dragging && !loading" class="sp-tip">{{ tip }}</span>
      <span v-if="loading" class="sp-tip">拼图生成中…</span>
    </div>

    <!-- 底部滑道：与拼图联动，给出明确交互反馈 -->
    <div class="sp-track" ref="trackRef" v-if="!verified">
      <div class="sp-fill" :style="{ width: sliderLeft + 'px' }"></div>
      <div
        class="sp-thumb"
        :style="{ left: sliderLeft + 'px' }"
        @pointerdown.prevent="onDown"
      >
        <el-icon v-if="!dragging"><ArrowRightBold /></el-icon>
        <el-icon v-else class="sp-spin"><Loading /></el-icon>
      </div>
      <span class="sp-track-text" :class="{ hidden: dragging || loading }">{{ tip }}</span>
    </div>

    <p class="sp-error" v-if="errorMsg">{{ errorMsg }}</p>

    <button class="sp-refresh" type="button" @click="reset" v-if="!verified && !loading">
      ↻ 换一张
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, onUnmounted, onMounted } from 'vue';
import { ArrowRightBold, CircleCheckFilled, Loading } from '@element-plus/icons-vue';
import { authApi } from '@/api/auth';

const emit = defineEmits<{
  (e: 'verified', token: string): void;
}>();

// 拼图块尺寸（须与 server/security/slidecaptcha 中的 pieceSize 保持一致）
const PIECE_SIZE = 42;

const stageW = ref(320);
const stageH = ref(160);
const pieceSize = PIECE_SIZE;
const pieceTop = ref(Math.round((160 - PIECE_SIZE) / 2));

const challengeId = ref('');
const bgImage = ref('');
const pieceImage = ref('');
const tip = ref('拖动拼图块到缺口位置完成验证');

const sliderLeft = ref(0);
const verified = ref(false);
const dragging = ref(false);
const loading = ref(false);
const errorMsg = ref('');

let startX = 0;
let startLeft = 0;
let maxLeft = 0;

async function issue() {
  loading.value = true;
  errorMsg.value = '';
  try {
    const res = await authApi.captchaIssue();
    if (res.data.code === 0 && res.data.data) {
      const d = res.data.data;
      challengeId.value = d.challenge_id;
      bgImage.value = d.bg_image;
      pieceImage.value = d.piece_image;
      if (d.width) stageW.value = d.width;
      if (d.height) stageH.value = d.height;
      pieceTop.value = Math.round((stageH.value - PIECE_SIZE) / 2);
      tip.value = d.tip || tip.value;
      // 最大可拖动距离 = 舞台宽 - 拼图块宽
      maxLeft = stageW.value - PIECE_SIZE;
      sliderLeft.value = 0;
    } else {
      errorMsg.value = res.data.message || '验证码加载失败，请重试';
    }
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.message || '验证码加载失败，请重试';
  } finally {
    loading.value = false;
  }
}

function onDown(e: PointerEvent) {
  if (verified.value || loading.value) return;
  e.preventDefault();
  dragging.value = true;
  errorMsg.value = '';
  startX = e.clientX;
  startLeft = sliderLeft.value;
  window.addEventListener('pointermove', onMove);
  window.addEventListener('pointerup', onUp);
}

function onMove(e: PointerEvent) {
  if (!dragging.value) return;
  const delta = e.clientX - startX;
  sliderLeft.value = Math.max(0, Math.min(startLeft + delta, maxLeft));
}

async function onUp() {
  if (!dragging.value) return;
  dragging.value = false;
  window.removeEventListener('pointermove', onMove);
  window.removeEventListener('pointerup', onUp);

  if (sliderLeft.value <= 0) return; // 没动，不算尝试

  const x = Math.round(sliderLeft.value);
  try {
    const res = await authApi.captchaVerify(challengeId.value, x);
    if (res.data.code === 0 && res.data.data?.captcha_token) {
      verified.value = true;
      errorMsg.value = '';
      emit('verified', res.data.data.captcha_token);
    } else {
      fail(res.data?.message || '验证失败，请重试');
    }
  } catch (e: any) {
    fail(e?.response?.data?.message || '验证失败，请重试');
  }
}

function fail(msg: string) {
  errorMsg.value = msg;
  sliderLeft.value = 0;
  // 失败刷新一次挑战，防止同一缺口被反复试探
  issue();
}

function reset() {
  verified.value = false;
  sliderLeft.value = 0;
  errorMsg.value = '';
  emit('verified', '');
  issue();
}

onMounted(() => {
  issue();
});

onUnmounted(() => {
  window.removeEventListener('pointermove', onMove);
  window.removeEventListener('pointerup', onUp);
});

defineExpose({ reset });
</script>

<style scoped>
.slide-puzzle {
  width: 100%;
  user-select: none;
  -webkit-user-select: none;
}

.sp-stage {
  position: relative;
  margin: 0 auto;
  border-radius: 8px;
  overflow: hidden;
  background: #f0f2f5;
  touch-action: none; /* 防止移动端拖动时页面滚动 */
}

.sp-bg {
  display: block;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.sp-piece {
  position: absolute;
  cursor: grab;
  border-radius: 6px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.35);
  filter: drop-shadow(0 0 2px rgba(0, 0, 0, 0.4));
}

.sp-piece:active {
  cursor: grabbing;
}

.sp-tip {
  position: absolute;
  left: 50%;
  bottom: 8px;
  transform: translateX(-50%);
  font-size: 0.78rem;
  color: #fff;
  background: rgba(0, 0, 0, 0.45);
  padding: 2px 10px;
  border-radius: 12px;
  pointer-events: none;
  white-space: nowrap;
}

.sp-ok-mask {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  background: rgba(103, 194, 58, 0.18);
  color: #67c23a;
  font-weight: 600;
  font-size: 0.95rem;
}

.sp-ok-mask .el-icon {
  font-size: 1.6rem;
}

/* 底部滑道 */
.sp-track {
  position: relative;
  height: 40px;
  margin-top: 8px;
  background: #e8e8e8;
  border-radius: 6px;
  overflow: hidden;
}

.sp-fill {
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  background: linear-gradient(90deg, #409eff, #67c23a);
  border-radius: 6px 0 0 6px;
  transition: width 0.05s linear;
}

.sp-thumb {
  position: absolute;
  top: 1px;
  width: 38px;
  height: 38px;
  background: #fff;
  border-radius: 5px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: grab;
  z-index: 2;
  color: #409eff;
}

.sp-thumb:active {
  cursor: grabbing;
}

.sp-track-text {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  font-size: 0.82rem;
  color: #999;
  pointer-events: none;
  transition: opacity 0.2s;
}

.sp-track-text.hidden {
  opacity: 0;
}

.sp-error {
  color: #f56c6c;
  font-size: 0.8rem;
  margin: 6px 0 0 0;
  text-align: center;
}

.sp-refresh {
  display: block;
  margin: 8px auto 0;
  padding: 4px 14px;
  font-size: 0.8rem;
  color: #409eff;
  background: transparent;
  border: 1px solid #c6e2ff;
  border-radius: 14px;
  cursor: pointer;
}

.sp-refresh:hover {
  background: #ecf5ff;
}

.sp-spin {
  animation: sp-rotate 1s linear infinite;
}

@keyframes sp-rotate {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
