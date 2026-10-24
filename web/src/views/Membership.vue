<template>
  <div class="mb-page">
    <!-- 顶部：标题 + 我的会员状态 -->
    <header class="mb-head">
      <div class="mb-head-text">
        <h1 class="mb-title">会员套餐</h1>
        <p class="mb-sub">
          开通后按天获得会员权益，同时按兑换比例到账代币。代币用于 AI 对话、绘画与视频。
        </p>
      </div>

      <div class="mb-status" :class="{ 'is-member': info.status.is_member }">
        <template v-if="info.status.is_member">
          <span class="mb-status-badge">会员</span>
          <div class="mb-status-body">
            <b>剩余 {{ info.status.days_left }} 天</b>
            <span>{{ formatDate(info.status.until) }} 到期</span>
          </div>
        </template>
        <template v-else>
          <span class="mb-status-badge mb-status-badge--off">未开通</span>
          <div class="mb-status-body">
            <b>还不是会员</b>
            <span>选一个套餐即可开通</span>
          </div>
        </template>
      </div>
    </header>

    <!-- 未开启提示：告诉用户找谁，而不是给一排点不动的按钮 -->
    <el-alert
      v-if="!loading && !info.payment_enabled"
      class="mb-alert"
      type="info"
      :closable="false"
      show-icon
      title="在线支付尚未开启"
      description="站长还没有配置支付通道。可以先在「钱包」里用卡密充值，或联系站长开通在线支付。"
    />

    <!-- 套餐网格（布局参照参考站会员页：3 列、卡片自带权益清单） -->
    <div v-if="loading" class="mb-grid">
      <div v-for="i in 4" :key="i" class="mb-card mb-card--skeleton">
        <el-skeleton :rows="6" animated />
      </div>
    </div>

    <div v-else-if="info.tiers.length" class="mb-grid">
      <article
        v-for="(t, idx) in info.tiers"
        :key="`${t.fen}-${idx}`"
        class="mb-card"
        :class="{ 'is-featured': !!t.badge }"
      >
        <div class="mb-card-top" :class="`mb-card-top--${idx % 4}`">
          <span v-if="t.badge" class="mb-card-badge">{{ t.badge }}</span>
          <h2 class="mb-card-name">{{ t.label }}</h2>
          <div class="mb-card-price">
            <span class="mb-card-cur">¥</span>
            <span class="mb-card-num">{{ yuan(t.fen) }}</span>
            <span v-if="t.days" class="mb-card-unit">/ {{ t.days }} 天</span>
          </div>
          <p v-if="t.days" class="mb-card-per">{{ perDay(t) }}</p>
        </div>

        <ul class="mb-card-feats">
          <li v-if="t.days">
            <span class="mb-feat-ico">✓</span>
            <span>会员有效期 <b>{{ t.days }}</b> 天</span>
          </li>
          <li v-for="(f, fi) in t.features || []" :key="fi">
            <span class="mb-feat-ico">✓</span>
            <span>{{ f }}</span>
          </li>
          <li>
            <span class="mb-feat-ico">✓</span>
            <span>到账 <b>{{ tokenAmount(t) }}</b> {{ tokenName }}</span>
          </li>
        </ul>

        <div class="mb-card-foot">
          <el-button
            type="primary"
            size="large"
            class="mb-card-btn"
            :loading="payingFen === t.fen"
            :disabled="!canBuy"
            @click="buy(t)"
          >
            {{ info.payment_enabled ? '立即开通' : '支付未开启' }}
          </el-button>
          <p class="mb-card-note">支付成功后代币与会员时长同时到账</p>
        </div>
      </article>
    </div>

    <StateBlock
      v-else
      state="empty"
      title="还没有可购买的套餐"
      description="站长还没有配置会员套餐。"
    />

    <p class="mb-foot">
      代币为平台内消费积分，不可提现、不可转账。会员到期后权益自动失效，不会自动续费。
    </p>

    <!-- 收银台 -->
    <el-dialog v-model="payVisible" title="完成支付" width="440px" @close="stopPolling">
      <template v-if="order">
        <div class="pay-box">
          <div class="pay-row"><span>套餐</span><b>{{ payLabel }}</b></div>
          <div class="pay-row"><span>应付</span><b class="pay-amount">¥{{ yuan(order.amount_fen) }}</b></div>
          <div class="pay-row"><span>订单号</span><code>{{ order.order_no }}</code></div>
          <div class="pay-row" v-if="countdown > 0">
            <span>剩余</span><b>{{ mmss(countdown) }}</b>
          </div>
        </div>
        <template v-if="order.status === 'paid'">
          <el-result icon="success" title="已到账" sub-title="会员权益已生效，可在上方状态条查看" />
        </template>
        <template v-else>
          <div class="pay-actions">
            <el-button v-if="order.pay_url" type="primary" @click="openCashier">去支付</el-button>
            <el-button v-if="order.order_no" @click="checkOrder">我已支付</el-button>
          </div>
          <p class="pay-hint">支付完成后本页会自动刷新；若长时间未到账，请勿重复下单。</p>
        </template>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue';
import { ElMessage } from 'element-plus';
import StateBlock from '@/components/common/StateBlock.vue';
import { paymentApi, type MembershipInfo, type PaymentTier, type PaymentOrder } from '@/api/payment';

const loading = ref(true);
const info = reactive<MembershipInfo>({
  status: { is_member: false, days_left: 0 },
  tiers: [],
  token_per_fen: 10,
  payment_enabled: false,
  token_enabled: true,
});

const tokenName = computed(() => '代币');
const canBuy = computed(() => info.payment_enabled && info.token_enabled && info.tiers.length > 0);

function yuan(fen: number) {
  return (fen / 100).toFixed(2).replace(/\.00$/, '');
}
function perDay(t: PaymentTier) {
  if (!t.days) return '';
  const per = t.fen / 100 / t.days;
  return `折合 ¥${per.toFixed(2)} / 天`;
}
function tokenAmount(t: PaymentTier) {
  return t.fen * info.token_per_fen + (t.bonus || 0);
}
function formatDate(iso?: string) {
  if (!iso) return '';
  const d = new Date(iso);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
function mmss(sec: number) {
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m}:${String(s).padStart(2, '0')}`;
}

async function load() {
  loading.value = true;
  try {
    const res = await paymentApi.membership();
    if (res.data.code === 0) Object.assign(info, res.data.data);
  } catch { /* 未登录/网络异常时保持默认空态 */ }
  loading.value = false;
}

// ---------- 下单与轮询 ----------
const payVisible = ref(false);
// 下单接口返回的是 PaymentOrder（无 id/created_at），别用 PaymentOrderItem 硬转
const order = ref<PaymentOrder | null>(null);
const payLabel = ref('');
const payingFen = ref(0);
const countdown = ref(0);
let pollTimer = 0;
let tickTimer = 0;

async function buy(t: PaymentTier) {
  if (!canBuy.value) {
    ElMessage.warning(info.payment_enabled ? '代币系统未开启' : '在线支付未开启，请联系站长');
    return;
  }
  payingFen.value = t.fen;
  try {
    const res = await paymentApi.createOrder({ channel: 'mock', fen: t.fen });
    if (res.data.code === 0) {
      order.value = res.data.data;
      payLabel.value = t.label;
      payVisible.value = true;
      startPolling();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '下单失败');
  }
  payingFen.value = 0;
}

function startPolling() {
  stopPolling();
  const expires = order.value ? new Date(order.value.expires_at).getTime() : Date.now();
  countdown.value = Math.max(0, Math.floor((expires - Date.now()) / 1000));
  tickTimer = window.setInterval(() => {
    countdown.value = Math.max(0, Math.floor((expires - Date.now()) / 1000));
    if (countdown.value === 0) stopPolling();
  }, 1000);
  pollTimer = window.setInterval(checkOrder, 3000);
}

function stopPolling() {
  if (pollTimer) window.clearInterval(pollTimer);
  if (tickTimer) window.clearInterval(tickTimer);
  pollTimer = 0;
  tickTimer = 0;
}

async function checkOrder() {
  if (!order.value) return;
  try {
    const res = await paymentApi.orderStatus(order.value.order_no);
    if (res.data.code === 0) {
      const st = res.data.data;
      order.value = { ...order.value, ...st };
      if (st.status === 'paid') {
        stopPolling();
        ElMessage.success('开通成功，会员权益已生效');
        await load();
      } else if (st.status !== 'pending') {
        stopPolling();
        ElMessage.warning('订单已关闭，请重新下单');
      }
    }
  } catch { /* 轮询失败不打扰用户，等下一次 */ }
}

function openCashier() {
  if (order.value?.pay_url) window.open(order.value.pay_url, '_blank', 'noopener');
}

onMounted(load);
onUnmounted(stopPolling);
</script>

<style scoped>
.mb-page {
  max-width: 1080px;
  margin: 0 auto;
  padding: var(--space-6, 32px) var(--space-4, 16px) var(--space-7, 48px);
}

/* ---------- 头部 ---------- */
.mb-head {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4, 16px);
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-5, 24px);
}
.mb-title {
  margin: 0 0 var(--space-1, 4px);
  font-size: var(--text-2xl, 28px);
  font-weight: 700;
  color: var(--text-color, #1f2328);
}
.mb-sub {
  margin: 0;
  font-size: var(--text-base, 14px);
  color: var(--text-light, #6b7280);
  line-height: 1.6;
  max-width: 620px;
}
.mb-status {
  display: flex;
  align-items: center;
  gap: var(--space-3, 12px);
  padding: 10px 16px;
  border-radius: var(--radius-lg, 16px);
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
}
.mb-status.is-member { border-color: rgba(234, 179, 8, 0.45); background: rgba(234, 179, 8, 0.07); }
.mb-status-badge {
  font-size: var(--text-xs, 12px);
  font-weight: 600;
  padding: 3px 9px;
  border-radius: 999px;
  color: #fff;
  background: linear-gradient(135deg, #f59e0b, #d97706);
}
.mb-status-badge--off { background: var(--text-light, #9ca3af); }
.mb-status-body { display: flex; flex-direction: column; font-size: var(--text-xs, 12px); color: var(--text-light, #6b7280); }
.mb-status-body b { font-size: var(--text-base, 14px); color: var(--text-color, #1f2328); }

.mb-alert { margin-bottom: var(--space-4, 16px); border-radius: var(--radius-md, 10px); }

/* ---------- 套餐网格 ---------- */
.mb-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: var(--space-4, 16px);
}
.mb-card {
  display: flex;
  flex-direction: column;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-lg, 16px);
  box-shadow: var(--shadow-xs, 0 1px 3px rgba(0, 0, 0, 0.04));
  overflow: hidden;
  transition: transform 0.16s ease, box-shadow 0.16s ease;
}
.mb-card:hover { transform: translateY(-3px); box-shadow: var(--shadow-md, 0 10px 28px rgba(0, 0, 0, 0.1)); }
.mb-card.is-featured { border-color: rgba(99, 102, 241, 0.5); box-shadow: 0 6px 22px rgba(99, 102, 241, 0.14); }
.mb-card--skeleton { padding: var(--space-4, 16px); }

.mb-card-top {
  position: relative;
  padding: var(--space-5, 24px) var(--space-4, 16px) var(--space-4, 16px);
  color: #fff;
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
}
.mb-card-top--1 { background: var(--gradient-blue, linear-gradient(135deg, #3b82f6, #06b6d4)); }
.mb-card-top--2 { background: var(--gradient-purple, linear-gradient(135deg, #8b5cf6, #d946ef)); }
.mb-card-top--3 { background: var(--gradient-rose, linear-gradient(135deg, #f43f5e, #f97316)); }
.mb-card-badge {
  position: absolute;
  top: 12px;
  right: 12px;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.24);
  backdrop-filter: blur(4px);
}
.mb-card-name { margin: 0 0 var(--space-2, 8px); font-size: var(--text-lg, 18px); font-weight: 600; }
.mb-card-price { display: flex; align-items: baseline; gap: 2px; }
.mb-card-cur { font-size: var(--text-base, 14px); opacity: 0.9; }
.mb-card-num { font-size: 34px; font-weight: 700; line-height: 1; letter-spacing: -0.02em; }
.mb-card-unit { font-size: var(--text-sm, 13px); opacity: 0.85; margin-left: 4px; }
.mb-card-per { margin: 6px 0 0; font-size: var(--text-xs, 12px); opacity: 0.82; }

.mb-card-feats {
  list-style: none;
  margin: 0;
  padding: var(--space-4, 16px);
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex: 1;
}
.mb-card-feats li {
  display: flex;
  gap: 8px;
  font-size: var(--text-sm, 13px);
  line-height: 1.5;
  color: var(--text-color, #374151);
}
.mb-feat-ico { color: #16a34a; font-weight: 700; flex-shrink: 0; }
.mb-card-feats b { color: var(--text-color, #1f2328); }

.mb-card-foot { padding: 0 var(--space-4, 16px) var(--space-4, 16px); }
.mb-card-btn { width: 100%; }
.mb-card-note {
  margin: 8px 0 0;
  text-align: center;
  font-size: 11px;
  color: var(--text-light, #9ca3af);
}

.mb-foot {
  margin: var(--space-6, 32px) 0 0;
  text-align: center;
  font-size: var(--text-xs, 12px);
  color: var(--text-light, #9ca3af);
  line-height: 1.7;
}

/* ---------- 收银台 ---------- */
.pay-box { display: flex; flex-direction: column; gap: 10px; }
.pay-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: var(--text-sm, 13px);
  color: var(--text-light, #6b7280);
}
.pay-row b { color: var(--text-color, #1f2328); }
.pay-amount { font-size: var(--text-xl, 22px); }
.pay-row code {
  font-size: 12px;
  background: var(--bg-page, #f5f6f8);
  padding: 2px 6px;
  border-radius: 4px;
}
.pay-actions { margin-top: var(--space-4, 16px); display: flex; gap: var(--space-2, 8px); justify-content: center; }
.pay-hint { margin: var(--space-3, 12px) 0 0; text-align: center; font-size: 12px; color: var(--text-light, #9ca3af); }

/* ---------- 深色 ---------- */
[data-theme="dark"] .mb-card,
[data-theme="dark"] .mb-status { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .mb-title,
[data-theme="dark"] .mb-status-body b,
[data-theme="dark"] .mb-card-feats li,
[data-theme="dark"] .pay-row b { color: #e2e8f0; }
[data-theme="dark"] .pay-row code { background: #0f172a; }

@media (max-width: 640px) {
  .mb-page { padding: var(--space-4, 16px); }
  .mb-grid { grid-template-columns: 1fr; }
  .mb-head { flex-direction: column; align-items: flex-start; }
}
</style>
