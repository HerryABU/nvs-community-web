<template>
  <div class="pay-page">
    <el-card shadow="never" class="pay-card" v-loading="loading">
      <template #header>
        <div class="pay-header">
          <span class="pay-title">模拟收银台</span>
          <el-tag size="small" type="warning">演示模式 · 不产生真实扣款</el-tag>
        </div>
      </template>

      <div v-if="order" class="pay-body">
        <div class="pay-amount">¥{{ (order.amount_fen / 100).toFixed(2) }}</div>
        <div class="pay-tokens">到账 {{ order.token_amount }} {{ tokenName || '代币' }}</div>
        <div class="pay-meta">订单号：{{ order.order_no }}</div>
        <div class="pay-meta">渠道：模拟支付（Mock）</div>

        <el-result
          v-if="order.status === 'paid'"
          icon="success"
          title="支付成功"
          :sub-title="`${order.token_amount} ${tokenName || '代币'} 已到账`"
        />
        <template v-else-if="order.status === 'pending'">
          <div class="pay-qr-hint">
            <el-icon :size="64" color="var(--el-color-primary, #409eff)"><Wallet /></el-icon>
            <p>真实环境下此处展示微信/支付宝收款码；<br />演示模式下点击下方按钮即模拟扣款成功并触发到账。</p>
          </div>
          <el-button type="primary" size="large" :loading="confirming" @click="confirmPay">模拟支付成功</el-button>
        </template>
        <el-result
          v-else-if="order.status === 'closed'"
          icon="warning"
          title="订单已过期"
          sub-title="请返回钱包重新下单"
        />
        <el-result v-else icon="error" title="订单异常" :sub-title="order.status" />

        <div class="pay-actions">
          <el-button @click="router.push({ name: 'wallet' })">返回钱包</el-button>
        </div>
      </div>

      <el-empty v-else-if="!loading" description="订单不存在或无权查看" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { Wallet } from '@element-plus/icons-vue';
import { paymentApi } from '@/api/payment';
import { tokenApi } from '@/api/token';

const route = useRoute();
const router = useRouter();
const loading = ref(true);
const confirming = ref(false);
const tokenName = ref('代币');
const order = ref<{
  order_no: string; status: string; amount_fen: number; token_amount: number;
} | null>(null);

async function loadOrder() {
  loading.value = true;
  try {
    const res = await paymentApi.orderStatus(route.params.orderNo as string);
    if (res.data.code === 0) order.value = res.data.data;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '订单加载失败');
  } finally {
    loading.value = false;
  }
}

async function confirmPay() {
  confirming.value = true;
  try {
    const res = await paymentApi.mockConfirm(route.params.orderNo as string);
    if (res.data.code === 0) {
      ElMessage.success(`支付成功，到账 ${res.data.data.token_amount} ${tokenName.value}`);
      await loadOrder();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '确认失败');
  } finally {
    confirming.value = false;
  }
}

onMounted(async () => {
  try {
    const st = await tokenApi.status();
    if (st.data.code === 0 && st.data.data.name) tokenName.value = st.data.data.name;
  } catch { /* ignore */ }
  await loadOrder();
});
</script>

<style scoped>
.pay-page { max-width: 480px; margin: 40px auto; padding: 0 16px; }
.pay-card { border-radius: 14px; }
.pay-header { display: flex; align-items: center; justify-content: space-between; }
.pay-title { font-weight: 700; font-size: 1.05rem; }
.pay-body { text-align: center; padding: 8px 0 4px; }
.pay-amount { font-size: 2.2rem; font-weight: 700; color: var(--el-color-danger, #f56c6c); }
.pay-tokens { font-size: 0.9rem; color: var(--text-light, #666); margin: 4px 0 10px; }
.pay-meta { font-size: 0.78rem; color: var(--text-light, #999); word-break: break-all; }
.pay-qr-hint { margin: 16px 0 20px; color: var(--text-light, #888); font-size: 0.82rem; line-height: 1.7; }
.pay-actions { margin-top: 18px; }
</style>
