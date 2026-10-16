<template>
  <!-- 微信扫码登录（F21.2）：二维码 + 倒计时 + 刷新 + 链接兜底。
       扫码确认后由微信 302 到后端 callback，后端写登录态 Cookie 后重定回
       /login?wechat=bound，本组件无需轮询登录状态。 -->
  <div class="wechat-qr-login">
    <div v-loading="loading" class="qr-wrap" :class="{ expired: expired }">
      <canvas ref="canvasEl" class="qr-canvas" />
      <div v-if="expired" class="qr-mask">
        <div class="qr-mask-inner">
          <p>二维码已过期</p>
          <el-button type="primary" size="small" @click="refresh">刷新二维码</el-button>
        </div>
      </div>
    </div>

    <p class="qr-tip">
      微信扫码 → 手机确认 → 自动登录
      <span v-if="!expired && countdown > 0" class="qr-countdown">（{{ countdown }}s 后过期）</span>
    </p>

    <div class="qr-actions">
      <el-button size="small" :disabled="!authUrl" @click="copyLink">复制链接</el-button>
      <el-button size="small" :disabled="!authUrl" @click="openInNew">新窗口打开</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue';
import QRCode from 'qrcode';
import { wechatApi } from '@/api/wechat';
import { ElMessage } from 'element-plus';

// 二维码倒计时（秒）。服务端 state TTL 为 300s，这里取 60s 提前置灰提示刷新，
// 避免用户扫到临过期的 state 导致回调失败。
const QR_COUNTDOWN = 60;

const canvasEl = ref<HTMLCanvasElement | null>(null);
const loading = ref(false);
const expired = ref(false);
const countdown = ref(QR_COUNTDOWN);
const authUrl = ref('');

let timer: number | undefined;

function stopTimer() {
  if (timer !== undefined) {
    clearInterval(timer);
    timer = undefined;
  }
}

function startCountdown() {
  stopTimer();
  countdown.value = QR_COUNTDOWN;
  timer = window.setInterval(() => {
    countdown.value--;
    if (countdown.value <= 0) {
      stopTimer();
      expired.value = true;
    }
  }, 1000);
}

async function loadQR() {
  loading.value = true;
  expired.value = false;
  try {
    const res = await wechatApi.getAuthUrl('login');
    if (res.data.code === 0 && res.data.data?.url) {
      authUrl.value = res.data.data.url;
      if (canvasEl.value) {
        await QRCode.toCanvas(canvasEl.value, authUrl.value, { width: 200, margin: 1 });
      }
      startCountdown();
    } else {
      ElMessage.error(res.data.message || '获取微信二维码失败');
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '获取微信二维码失败，请稍后重试');
  } finally {
    loading.value = false;
  }
}

function refresh() {
  loadQR();
}

async function copyLink() {
  if (!authUrl.value) return;
  try {
    await navigator.clipboard.writeText(authUrl.value);
    ElMessage.success('链接已复制，可发送到手机微信打开');
  } catch {
    // 剪贴板不可用（非 HTTPS/旧浏览器）时提示手动复制
    ElMessage.warning('复制失败，请使用「新窗口打开」');
  }
}

function openInNew() {
  if (!authUrl.value) return;
  window.open(authUrl.value, '_blank', 'noopener');
}

onMounted(loadQR);
onBeforeUnmount(stopTimer);
</script>

<style scoped>
.wechat-qr-login {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 8px 0 4px;
}

.qr-wrap {
  position: relative;
  width: 200px;
  height: 200px;
  border: 1px solid var(--el-border-color, #dcdfe6);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fff;
}

.qr-canvas {
  display: block;
}

/* 过期置灰：二维码 + 遮罩提示刷新 */
.qr-wrap.expired {
  filter: grayscale(1);
  opacity: 0.6;
}

.qr-mask {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.85);
  border-radius: 8px;
}

.qr-mask-inner {
  text-align: center;
}

.qr-mask-inner p {
  margin: 0 0 8px;
  font-size: 0.85rem;
  color: var(--el-text-color-regular, #606266);
}

.qr-tip {
  margin: 12px 0 8px;
  font-size: 0.85rem;
  color: var(--text-secondary, #909399);
}

.qr-countdown {
  color: var(--text-light, #c0c4cc);
}

.qr-actions {
  display: flex;
  gap: 8px;
}
</style>
