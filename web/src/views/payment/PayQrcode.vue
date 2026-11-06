<template>
  <div class="qr-page">
    <div class="qr-card">
      <h1 class="qr-title">扫码支付</h1>
      <p class="qr-sub">
        {{ channelLabel }} · 应付 <b>¥{{ yuan(order?.amount_fen || 0) }}</b>
      </p>

      <div v-if="loading" class="qr-box"><el-skeleton :rows="4" animated /></div>
      <template v-else-if="payUrl">
        <div class="qr-box">
          <canvas ref="canvasRef" class="qr-canvas" />
        </div>
        <p class="qr-hint">请使用{{ channelLabel }}扫描二维码完成付款</p>
        <el-alert type="info" :closable="false" show-icon class="qr-alert"
          title="付款后请勿关闭本页" description="系统会自动检测到账并跳转结果页；若长时间未跳转可点下方按钮刷新。" />
        <div class="qr-actions">
          <el-button @click="copyLink">复制支付链接</el-button>
          <el-button type="primary" :loading="checking" @click="checkOnce">我已付款，刷新状态</el-button>
        </div>
        <p class="qr-count" v-if="countdown > 0">二维码有效期剩余 {{ countdownText }}</p>
        <p class="qr-count qr-expired" v-else>二维码已过期，请返回重新下单</p>
      </template>
      <el-empty v-else description="未找到支付二维码，请返回重新下单" />

      <div class="qr-foot">
        <el-button text @click="$router.push('/wallet')">返回钱包</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import QRCode from 'qrcode';
import { paymentApi, PAY_CHANNEL_LABELS } from '@/api/payment';
import type { PaymentOrder } from '@/api/payment';

const route = useRoute();
const router = useRouter();
const orderNo = String(route.params.orderNo || '');

const order = ref<PaymentOrder | null>(null);
const payUrl = ref('');
const canvasRef = ref<HTMLCanvasElement | null>(null);
const loading = ref(true);
const checking = ref(false);
const countdown = ref(0);
let timer: number | undefined;

const channelLabel = computed(() =>
  PAY_CHANNEL_LABELS[order.value?.channel || ''] || '支付渠道');
const countdownText = computed(() => {
  const m = Math.floor(countdown.value / 60);
  const s = countdown.value % 60;
  return `${m}:${String(s).padStart(2, '0')}`;
});

function yuan(fen: number) {
  return (fen / 100).toFixed(2);
}

async function loadOrder() {
  const res = await paymentApi.orderStatus(orderNo);
  if (res.data.code !== 0) return;
  order.value = res.data.data;
  payUrl.value = res.data.data.pay_url || '';
  countdown.value = Math.max(0,
    Math.floor((new Date(order.value.expires_at).getTime() - Date.now()) / 1000));
  if (payUrl.value && canvasRef.value) {
    await QRCode.toCanvas(canvasRef.value, payUrl.value, {
      width: 260, margin: 2,
      color: { dark: '#111827', light: '#ffffff' },
    });
  }
}

async function checkOnce() {
  checking.value = true;
  try {
    const res = await paymentApi.orderStatus(orderNo);
    if (res.data.code === 0 && res.data.data.status === 'paid') {
      ElMessage.success('支付成功，已到账');
      stopPolling();
      router.replace(`/pay/result/${encodeURIComponent(orderNo)}`);
      return;
    }
    ElMessage.info('尚未检测到到账，请稍后重试');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '查询失败');
  } finally {
    checking.value = false;
  }
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(payUrl.value);
    ElMessage.success('已复制支付链接');
  } catch {
    ElMessage.warning('浏览器拒绝访问剪贴板，请手动选择复制');
  }
}

function stopPolling() {
  if (timer !== undefined) {
    clearInterval(timer);
    timer = undefined;
  }
}

onMounted(async () => {
  try {
    await loadOrder();
  } catch {
    ElMessage.error('订单加载失败');
  } finally {
    loading.value = false;
  }
  timer = window.setInterval(async () => {
    countdown.value = Math.max(0, countdown.value - 3);
    try {
      const res = await paymentApi.orderStatus(orderNo);
      if (res.data.code === 0 && res.data.data.status === 'paid') {
        stopPolling();
        router.replace(`/pay/result/${encodeURIComponent(orderNo)}`);
      }
    } catch { /* 静默，等下一次轮询 */ }
  }, 3000);
});
onUnmounted(stopPolling);
</script>

<style scoped>
.qr-page { min-height: 100vh; display: flex; align-items: flex-start; justify-content: center; padding: 48px 16px; background: var(--bg-page, #f5f7fa); }
.qr-card { width: 100%; max-width: 460px; padding: 28px; border-radius: 14px; background: var(--bg-card, #fff); box-shadow: var(--shadow-md, 0 8px 24px rgba(0,0,0,.08)); text-align: center; }
.qr-title { margin: 0 0 6px; font-size: 1.3rem; }
.qr-sub { margin: 0 0 20px; font-size: 0.9rem; color: var(--text-light, #666); }
.qr-box { display: flex; justify-content: center; align-items: center; min-height: 276px; }
.qr-canvas { border: 1px solid var(--border-color, #ebeef5); border-radius: 8px; }
.qr-hint { margin: 14px 0 16px; font-size: 0.9rem; }
.qr-alert { margin-bottom: 16px; text-align: left; }
.qr-actions { display: flex; gap: 10px; justify-content: center; flex-wrap: wrap; }
.qr-count { margin: 16px 0 0; font-size: 0.8rem; color: var(--text-light, #999); }
.qr-expired { color: var(--el-color-danger); }
.qr-foot { margin-top: 20px; }
</style>
