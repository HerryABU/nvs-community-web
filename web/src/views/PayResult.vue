<template>
  <div class="pay-page">
    <el-card shadow="never" class="pay-card" v-loading="loading">
      <template #header>
        <div class="pay-header">
          <span class="pay-title">支付结果</span>
        </div>
      </template>

      <div v-if="order" class="pay-body">
        <el-result
          v-if="order.status === 'paid'"
          icon="success"
          title="支付成功"
          :sub-title="`${order.token_amount} ${tokenName || '代币'} 已到账`"
        />
        <el-result
          v-else-if="order.status === 'pending'"
          icon="info"
          title="等待到账确认"
          sub-title="银行/支付渠道处理中，通常几秒内自动到账，本页将自动刷新"
        />
        <el-result
          v-else-if="order.status === 'closed'"
          icon="warning"
          title="订单已关闭"
          sub-title="订单超时未支付已自动关闭；如已扣款请联系站长补单"
        />
        <el-result v-else icon="error" title="支付未完成" sub-title="如已扣款请联系站长补单" />

        <div class="pay-meta">订单号：{{ order.order_no }}</div>
        <div class="pay-meta">金额：¥{{ (order.amount_fen / 100).toFixed(2) }} · 到账 {{ order.token_amount }} {{ tokenName || '代币' }}</div>

        <div class="pay-actions">
          <el-button type="primary" @click="router.push({ name: 'wallet' })">返回钱包</el-button>
          <el-button v-if="order.status === 'pending'" :loading="loading" @click="loadOrder">刷新状态</el-button>
        </div>
      </div>

      <el-empty v-else-if="!loading" description="订单不存在或无权查看" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { paymentApi } from '@/api/payment';
import { tokenApi } from '@/api/token';

const route = useRoute();
const router = useRouter();
const loading = ref(true);
const tokenName = ref('代币');
const order = ref<{
  order_no: string; status: string; amount_fen: number; token_amount: number;
} | null>(null);
let pollTimer: number | undefined;

async function loadOrder() {
  loading.value = true;
  try {
    const res = await paymentApi.orderStatus(route.params.orderNo as string);
    if (res.data.code === 0) {
      order.value = res.data.data;
      // 到账后停止轮询
      if (order.value.status !== 'pending' && pollTimer !== undefined) {
        clearInterval(pollTimer);
        pollTimer = undefined;
      }
    }
  } catch { /* 静默：可能是未登录或订单不存在 */ } finally {
    loading.value = false;
  }
}

onMounted(async () => {
  try {
    const st = await tokenApi.status();
    if (st.data.code === 0 && st.data.data.name) tokenName.value = st.data.data.name;
  } catch { /* ignore */ }
  await loadOrder();
  // 网关异步通知可能略慢于同步跳转：pending 时每 3 秒轮询一次（最多 30 次）
  let tries = 0;
  pollTimer = window.setInterval(() => {
    if (order.value?.status !== 'pending' || ++tries > 30) {
      if (pollTimer !== undefined) { clearInterval(pollTimer); pollTimer = undefined; }
      return;
    }
    loadOrder();
  }, 3000);
});

onUnmounted(() => {
  if (pollTimer !== undefined) clearInterval(pollTimer);
});
</script>

<style scoped>
.pay-page { max-width: 480px; margin: 40px auto; padding: 0 16px; }
.pay-card { border-radius: 14px; }
.pay-header { display: flex; align-items: center; justify-content: space-between; }
.pay-title { font-weight: 700; font-size: 1.05rem; }
.pay-body { text-align: center; padding: 8px 0 4px; }
.pay-meta { font-size: 0.78rem; color: var(--text-light, #999); word-break: break-all; margin-top: 6px; }
.pay-actions { margin-top: 18px; display: flex; gap: 10px; justify-content: center; }
</style>
