<template>
  <div class="wallet-page">
    <h2 class="page-title">我的钱包</h2>

    <!-- 未开启提示 -->
    <el-card v-if="!loading && !status.enabled" shadow="never" class="disabled-card">
      <el-empty description="代币系统尚未开启">
        <template #description>
          <p>站长暂未启用代币系统，开启后即可使用卡密充值与代币消费。</p>
        </template>
      </el-empty>
    </el-card>

    <!-- 上游密钥（BYOK，独立于代币系统：始终可用） -->
    <el-card shadow="never" class="byok-card">
      <div class="card-title">
        我的上游密钥
        <span class="muted small">（BYOK：使用自己的 OpenAI 兼容密钥调用 AI，不消耗平台代币）</span>
      </div>
      <div class="byok-tip">
        <el-icon><InfoFilled /></el-icon>
        密钥经 AES-256-GCM <b>加密存储、只写不读</b>（添加后不可查看，遗失请删除重加）；最多 {{ keyMax }} 个。
        在「AI 工作台」或章节编辑器勾选「使用我的密钥」即可生效。
      </div>
      <el-table :data="myKeys" v-loading="keysLoading" style="width:100%" size="small">
        <el-table-column label="备注名" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.label }}
            <el-tag v-if="row.is_default" size="small" type="success" style="margin-left:4px">默认</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="base_url" label="上游地址" min-width="180" show-overflow-tooltip />
        <el-table-column label="默认模型" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">{{ row.model_id || '—' }}</template>
        </el-table-column>
        <el-table-column label="密钥" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="row.has_key ? 'success' : 'info'">已配置</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button v-if="!row.is_default" text type="primary" size="small" @click="setDefaultKey(row)">设默认</el-button>
            <el-button text type="danger" size="small" @click="deleteKey(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="byok-actions">
        <el-button type="primary" size="small" :icon="Plus" @click="keyDialog = true">添加密钥</el-button>
      </div>
    </el-card>

    <template v-if="status.enabled">
      <!-- 余额卡 -->
      <el-card shadow="never" class="balance-card">
        <div class="balance-label">{{ status.name || '代币' }}余额</div>
        <div class="balance-num">{{ status.balance }}</div>
        <div class="balance-note">
          <el-icon><InfoFilled /></el-icon>
          代币为平台内消费积分，<b>不可提现、不可转账、不可兑换现金</b>；与作者收益（可提现）为两套独立账目。
        </div>
      </el-card>

      <!-- 在线充值（§7.4：微信/支付宝聚合 / 模拟支付） -->
      <el-card v-if="pay.enabled" shadow="never" class="recharge-card">
        <div class="card-title">
          在线充值
          <span class="muted small">
            （{{ pay.provider === 'aggregate' ? '微信 / 支付宝' : '模拟支付（演示）' }} · 1 元 = {{ pay.token_per_fen * 100 }} {{ status.name || '代币' }}）
          </span>
        </div>
        <div class="tier-grid">
          <div
            v-for="t in pay.tiers"
            :key="t.fen"
            class="tier-item"
            :class="{ active: selectedFen === t.fen }"
            @click="selectedFen = t.fen"
          >
            <div class="tier-label">{{ t.label }}</div>
            <div class="tier-tokens">到账 {{ tierTokens(t) }} {{ status.name || '代币' }}</div>
            <div v-if="t.bonus > 0" class="tier-bonus">赠 {{ t.bonus }}</div>
          </div>
        </div>
        <div class="recharge-actions">
          <el-radio-group v-model="payChannel" v-if="pay.channels.length > 1">
            <el-radio-button v-for="ch in pay.channels" :key="ch.value" :value="ch.value">{{ ch.label }}</el-radio-button>
          </el-radio-group>
          <el-button type="primary" size="large" :loading="creating" :disabled="!selectedFen" @click="createOrder">
            立即充值
          </el-button>
        </div>
        <div class="redeem-hint">代币为平台内消费积分，不可提现、不可转账；充值时请认准本站域名，谨防钓鱼。</div>
        <div v-if="payOrders.list.length" class="recent-orders">
          <div class="recent-title">最近充值订单</div>
          <el-table :data="payOrders.list" size="small">
            <el-table-column prop="order_no" label="订单号" min-width="150" show-overflow-tooltip />
            <el-table-column label="渠道" width="90">
              <template #default="{ row }">{{ PAY_CHANNEL_LABELS[row.channel] || row.channel }}</template>
            </el-table-column>
            <el-table-column label="金额" width="90">
              <template #default="{ row }">¥{{ (row.amount_fen / 100).toFixed(2) }}</template>
            </el-table-column>
            <el-table-column label="代币" width="90">
              <template #default="{ row }">+{{ row.token_amount }}</template>
            </el-table-column>
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag size="small" :type="row.status === 'paid' ? 'success' : row.status === 'pending' ? 'warning' : 'info'">
                  {{ PAY_STATUS_LABELS[row.status] || row.status }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="时间" min-width="150" show-overflow-tooltip />
          </el-table>
        </div>
      </el-card>

      <!-- 卡密兑换 -->
      <el-card shadow="never" class="redeem-card">
        <div class="card-title">卡密兑换</div>
        <div class="redeem-row">
          <el-input
            v-model="redeemCode"
            placeholder="请输入卡密（如 XXXX-XXXX-XXXX-XXXX）"
            clearable
            size="large"
            style="max-width: 420px"
            @keyup.enter="doRedeem"
          />
          <el-button type="primary" size="large" :loading="redeeming" @click="doRedeem">兑换</el-button>
        </div>
        <div class="redeem-hint">卡密由站长发放，一次性使用；兑换后代币立即到账。</div>
      </el-card>

      <!-- 流水 -->
      <el-card shadow="never" class="ledger-card">
        <div class="card-title">收支流水</div>
        <el-table :data="ledger.list" v-loading="ledgerLoading" style="width:100%">
          <el-table-column label="类型" width="110">
            <template #default="{ row }">
              <el-tag size="small" :type="typeTag(row.type)">{{ TOKEN_TYPE_LABELS[row.type] || row.type }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="金额" width="120">
            <template #default="{ row }">
              <span :class="row.amount >= 0 ? 'amount-in' : 'amount-out'">
                {{ row.amount >= 0 ? '+' + row.amount : row.amount }}
              </span>
            </template>
          </el-table-column>
          <el-table-column prop="balance_after" label="变动后余额" width="120" />
          <el-table-column prop="remark" label="备注" min-width="140" show-overflow-tooltip>
            <template #default="{ row }">{{ row.remark || '—' }}</template>
          </el-table-column>
          <el-table-column prop="created_at" label="时间" width="180" show-overflow-tooltip />
        </el-table>
        <div class="pager-row">
          <el-pagination
            v-model:current-page="ledgerPage"
            :page-size="20"
            :total="ledgerTotal"
            layout="prev, pager, next"
            small
            background
            @current-change="loadLedger"
          />
        </div>
      </el-card>
    </template>

    <!-- 在线支付浮层：收银台入口 + 订单状态轮询 -->
    <el-dialog
      v-model="payDialog"
      title="在线支付"
      width="440px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      @closed="stopPayPolling"
    >
      <div class="pay-dialog-body">
        <div class="pay-amount">¥{{ ((payDialogOrder?.amount_fen || 0) / 100).toFixed(2) }}</div>
        <div class="pay-tokens">到账 {{ payDialogOrder?.token_amount }} {{ status.name || '代币' }}</div>
        <div class="pay-order-no">订单号：{{ payDialogOrder?.order_no }}</div>
        <div v-if="payCountdown > 0" class="pay-countdown">支付剩余时间：{{ mmss(payCountdown) }}</div>
        <div v-else class="pay-countdown expired">订单已过期，请关闭后重新下单</div>
        <el-result
          v-if="payDialogOrder?.status === 'paid'"
          icon="success"
          title="支付成功"
          :sub-title="`${payDialogOrder?.token_amount} ${status.name || '代币'} 已到账`"
        />
        <el-button
          v-else-if="payDialogOrder?.status === 'pending'"
          type="primary"
          size="large"
          @click="openCashier"
        >
          打开收银台
        </el-button>
        <p class="pay-tip">
          请勿关闭本页，支付完成后将自动刷新到账状态；如已扣款但长时间未到账，请联系站长补单。
        </p>
      </div>
    </el-dialog>

    <!-- 添加密钥弹窗 -->
    <el-dialog v-model="keyDialog" title="添加上游密钥" width="460px">
      <el-form label-width="90px">
        <el-form-item label="备注名">
          <el-input v-model="keyForm.label" placeholder="如：我的 DeepSeek" maxlength="64" />
        </el-form-item>
        <el-form-item label="上游地址" required>
          <el-input v-model="keyForm.base_url" placeholder="如 https://api.deepseek.com/v1（不含 /chat/completions）" />
        </el-form-item>
        <el-form-item label="API Key" required>
          <el-input v-model="keyForm.api_key" type="password" show-password placeholder="上游密钥，加密后存储" />
        </el-form-item>
        <el-form-item label="默认模型">
          <el-input v-model="keyForm.model_id" placeholder="如 deepseek-chat（调用时可改）" maxlength="128" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="keyDialog = false">取消</el-button>
        <el-button type="primary" :loading="keySaving" @click="saveKey">添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { InfoFilled, Plus } from '@element-plus/icons-vue';
import { tokenApi, TOKEN_TYPE_LABELS } from '@/api/token';
import { aiApi, type MyAiKey } from '@/api/ai';
import {
  paymentApi, PAY_STATUS_LABELS, PAY_CHANNEL_LABELS,
  type PaymentStatus, type PaymentTier, type PaymentOrderItem,
} from '@/api/payment';

const status = reactive<{ enabled: boolean; name: string; balance: number }>({
  enabled: false,
  name: '代币',
  balance: 0,
});
const loading = ref(false);

async function loadStatus() {
  loading.value = true;
  try {
    const res = await tokenApi.status();
    if (res.data.code === 0) {
      status.enabled = res.data.data.enabled;
      status.name = res.data.data.name;
      status.balance = res.data.data.balance;
      if (status.enabled) loadLedger();
    }
  } catch { /* ignore */ }
  loading.value = false;
}

// ---------- 兑换 ----------
const redeemCode = ref('');
const redeeming = ref(false);

async function doRedeem() {
  const code = redeemCode.value.trim();
  if (!code) { ElMessage.warning('请输入卡密'); return; }
  redeeming.value = true;
  try {
    const res = await tokenApi.redeem(code);
    if (res.data.code === 0) {
      ElMessage.success(`兑换成功，当前余额 ${res.data.data?.balance ?? status.balance}`);
      redeemCode.value = '';
      await loadStatus();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '兑换失败');
  }
  redeeming.value = false;
}

// ---------- 流水 ----------
const ledger = reactive<{ list: any[] }>({ list: [] });
const ledgerLoading = ref(false);
const ledgerPage = ref(1);
const ledgerTotal = ref(0);

async function loadLedger() {
  ledgerLoading.value = true;
  try {
    const res = await tokenApi.ledger({ page: ledgerPage.value, page_size: 20 });
    if (res.data.code === 0) {
      ledger.list = res.data.data.list || [];
      ledgerTotal.value = res.data.data.total || 0;
    }
  } catch { /* ignore */ }
  ledgerLoading.value = false;
}

function typeTag(t: string) {
  const map: Record<string, string> = {
    recharge_card: 'success',
    recharge_pay: 'success',
    consume: 'warning',
    grant: 'primary',
    deduct: 'danger',
    refund: 'info',
  };
  return (map[t] as any) || 'info';
}

// ---------- 在线充值（§7.4） ----------
const pay = reactive<PaymentStatus>({
  enabled: false, token_enabled: false, token_name: '', provider: 'mock',
  channels: [], tiers: [], token_per_fen: 10, order_ttl_min: 30,
});
const payChannel = ref('mock');
const selectedFen = ref<number | null>(null);
const creating = ref(false);
const payDialog = ref(false);
const payDialogOrder = ref<PaymentOrderItem | null>(null);
const payCountdown = ref(0);
let payTimer: number | undefined;
let payPollTimer: number | undefined;

function tierTokens(t: PaymentTier) {
  return t.fen * pay.token_per_fen + t.bonus;
}
function mmss(sec: number) {
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m}:${String(s).padStart(2, '0')}`;
}

async function loadPayStatus() {
  try {
    const res = await paymentApi.status();
    if (res.data.code === 0) {
      Object.assign(pay, res.data.data);
      if (pay.channels.length) payChannel.value = pay.channels[0].value;
      if (pay.enabled) loadPayOrders();
    }
  } catch { /* 支付未配置时静默 */ }
}

const payOrders = reactive<{ list: PaymentOrderItem[] }>({ list: [] });

async function loadPayOrders() {
  try {
    const res = await paymentApi.myOrders({ page: 1, page_size: 5 });
    if (res.data.code === 0) payOrders.list = res.data.data.list || [];
  } catch { /* ignore */ }
}

async function createOrder() {
  if (!selectedFen.value) { ElMessage.warning('请选择充值档位'); return; }
  creating.value = true;
  try {
    const res = await paymentApi.createOrder({ channel: payChannel.value, fen: selectedFen.value });
    if (res.data.code === 0) {
      payDialogOrder.value = res.data.data as unknown as PaymentOrderItem;
      payDialog.value = true;
      startPayPolling();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '下单失败');
  }
  creating.value = false;
}

function openCashier() {
  const url = payDialogOrder.value?.pay_url;
  if (url) window.open(url, '_blank', 'noopener');
}

function stopPayPolling() {
  if (payTimer !== undefined) { clearInterval(payTimer); payTimer = undefined; }
  if (payPollTimer !== undefined) { clearInterval(payPollTimer); payPollTimer = undefined; }
}

function startPayPolling() {
  stopPayPolling();
  const expires = new Date(payDialogOrder.value!.expires_at).getTime();
  payCountdown.value = Math.max(0, Math.floor((expires - Date.now()) / 1000));
  payTimer = window.setInterval(() => {
    payCountdown.value = Math.max(0, Math.floor((expires - Date.now()) / 1000));
    if (payCountdown.value === 0) {
      stopPayPolling();
      loadPayOrders();
    }
  }, 1000);
  payPollTimer = window.setInterval(async () => {
    if (!payDialogOrder.value) return;
    try {
      const res = await paymentApi.orderStatus(payDialogOrder.value.order_no);
      if (res.data.code === 0) {
        const st = res.data.data;
        payDialogOrder.value = { ...payDialogOrder.value, ...st } as PaymentOrderItem;
        if (st.status !== 'pending') {
          stopPayPolling();
          if (st.status === 'paid') {
            ElMessage.success(`充值成功，+${st.token_amount} ${pay.token_name || '代币'}`);
            status.balance = st.balance;
            loadStatus();
            loadLedger();
            loadPayOrders();
          } else {
            ElMessage.warning(PAY_STATUS_LABELS[st.status] || '订单已关闭');
            loadPayOrders();
          }
        }
      }
    } catch { /* 轮询失败静默重试 */ }
  }, 2500);
}

// ---------- 上游密钥（BYOK §5.5） ----------
const myKeys = ref<MyAiKey[]>([]);
const keysLoading = ref(false);
const keyMax = ref(5);
const keyDialog = ref(false);
const keySaving = ref(false);
const keyForm = reactive({ label: '', base_url: '', api_key: '', model_id: '' });

async function loadKeys() {
  keysLoading.value = true;
  try {
    const res = await aiApi.myKeys();
    if (res.data.code === 0) {
      myKeys.value = res.data.data.list || [];
      keyMax.value = res.data.data.max || 5;
    }
  } catch { /* AI 未开启时静默 */ }
  keysLoading.value = false;
}

async function saveKey() {
  if (!keyForm.base_url.trim() || !keyForm.api_key.trim()) {
    ElMessage.warning('上游地址与 API Key 均为必填');
    return;
  }
  keySaving.value = true;
  try {
    const res = await aiApi.createMyKey({
      label: keyForm.label.trim(),
      base_url: keyForm.base_url.trim(),
      api_key: keyForm.api_key.trim(),
      model_id: keyForm.model_id.trim(),
    });
    if (res.data.code === 0) {
      ElMessage.success('已添加（密钥不可再次查看）');
      keyDialog.value = false;
      keyForm.label = ''; keyForm.base_url = ''; keyForm.api_key = ''; keyForm.model_id = '';
      await loadKeys();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '添加失败');
  }
  keySaving.value = false;
}

async function setDefaultKey(row: MyAiKey) {
  try {
    const res = await aiApi.setDefaultMyKey(row.id);
    if (res.data.code === 0) {
      ElMessage.success('已设为默认');
      await loadKeys();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function deleteKey(row: MyAiKey) {
  await ElMessageBox.confirm(`删除密钥「${row.label}」？删除后使用该密钥的调用将失败，且密钥无法恢复。`, '确认删除', { type: 'warning' });
  try {
    const res = await aiApi.deleteMyKey(row.id);
    if (res.data.code === 0) {
      ElMessage.success('已删除');
      await loadKeys();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

onMounted(() => {
  loadStatus();
  loadKeys();
  loadPayStatus();
});

onUnmounted(stopPayPolling);
</script>

<style scoped>
.wallet-page { max-width: 900px; margin: 0 auto; padding: 24px; }
.page-title { margin: 0 0 16px; font-size: 1.3rem; color: var(--text-color, #333); }
.balance-card { border-radius: 12px; margin-bottom: 16px; text-align: center; padding: 12px 0; }
.balance-label { font-size: 0.9rem; color: var(--text-light, #666); }
.balance-num { font-size: 2.6rem; font-weight: 700; color: var(--el-color-primary, #409eff); margin: 6px 0 10px; }
.balance-note { font-size: 0.8rem; color: var(--text-light, #888); line-height: 1.6; }
.redeem-card, .ledger-card { border-radius: 10px; margin-bottom: 16px; }
.byok-card { border-radius: 10px; margin-bottom: 16px; }
.byok-tip { font-size: 0.78rem; color: var(--text-light, #888); line-height: 1.6; margin-bottom: 10px; }
.byok-actions { margin-top: 10px; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.75rem; font-weight: 400; }
.card-title { font-weight: 600; font-size: 0.95rem; color: var(--text-color, #333); margin-bottom: 12px; }
.redeem-row { display: flex; gap: 12px; align-items: center; }
.redeem-hint { font-size: 0.78rem; color: var(--text-light, #999); margin-top: 8px; }
.pager-row { display: flex; justify-content: flex-end; margin-top: 12px; }
.amount-in { color: #16a34a; font-weight: 600; }
.amount-out { color: #ef4444; font-weight: 600; }
.disabled-card { border-radius: 12px; }
/* ---------- 在线充值 ---------- */
.recharge-card { border-radius: 10px; margin-bottom: 16px; }
.tier-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(130px, 1fr)); gap: 10px; margin-bottom: 14px; }
.tier-item {
  position: relative; border: 1px solid var(--el-border-color, #dcdfe6); border-radius: 10px;
  padding: 12px 10px; text-align: center; cursor: pointer; transition: all 0.15s;
  background: var(--el-fill-color-light, #fafafa);
}
.tier-item:hover { border-color: var(--el-color-primary, #409eff); }
.tier-item.active { border-color: var(--el-color-primary, #409eff); background: var(--el-color-primary-light-9, #ecf5ff); }
.tier-label { font-size: 1.05rem; font-weight: 700; color: var(--text-color, #333); }
.tier-tokens { font-size: 0.75rem; color: var(--text-light, #888); margin-top: 4px; }
.tier-bonus {
  position: absolute; top: -8px; right: -6px; font-size: 0.68rem; line-height: 1;
  background: var(--el-color-danger, #f56c6c); color: #fff; padding: 3px 6px; border-radius: 8px;
}
.recharge-actions { display: flex; align-items: center; gap: 14px; }
.recent-orders { margin-top: 14px; }
.recent-title { font-size: 0.82rem; color: var(--text-light, #888); margin-bottom: 6px; }
.pay-dialog-body { text-align: center; }
.pay-amount { font-size: 2rem; font-weight: 700; color: var(--el-color-danger, #f56c6c); }
.pay-tokens { font-size: 0.9rem; color: var(--text-light, #666); margin: 4px 0 8px; }
.pay-order-no { font-size: 0.75rem; color: var(--text-light, #999); word-break: break-all; }
.pay-countdown { font-size: 0.8rem; color: var(--el-color-warning, #e6a23c); margin: 10px 0; }
.pay-countdown.expired { color: var(--el-color-danger, #f56c6c); }
.pay-tip { font-size: 0.75rem; color: var(--text-light, #999); line-height: 1.6; margin-top: 12px; }
[data-theme="dark"] .page-title,
[data-theme="dark"] .card-title { color: #e2e8f0; }
</style>
