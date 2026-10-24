<template>
  <div class="admin-page" v-loading="loading">
    <SectionHeader
      title="在线支付通道"
      subtitle="微信/支付宝聚合充值 · 档位面额 · 幂等到账 · 手动补单"
    />

    <!-- 统计 -->
    <div class="stats-row">
      <el-card shadow="never" class="stat-card">
        <div class="stat-num">{{ cfg.stats.paid_count }}</div>
        <div class="stat-label">已到账笔数</div>
      </el-card>
      <el-card shadow="never" class="stat-card">
        <div class="stat-num">¥{{ (cfg.stats.paid_fen / 100).toFixed(2) }}</div>
        <div class="stat-label">累计实付金额</div>
      </el-card>
      <el-card shadow="never" class="stat-card">
        <div class="stat-num">{{ cfg.stats.paid_token }}</div>
        <div class="stat-label">累计发放代币</div>
      </el-card>
      <el-card shadow="never" class="stat-card">
        <div class="stat-num warning">{{ cfg.stats.pending_count }}</div>
        <div class="stat-label">待支付订单</div>
      </el-card>
      <el-card shadow="never" class="stat-card">
        <div class="stat-num muted">{{ cfg.stats.closed_count }}</div>
        <div class="stat-label">已关单（超时）</div>
      </el-card>
    </div>

    <!-- 通道配置 -->
    <el-card shadow="never" class="cfg-card">
      <el-form label-width="150px" style="max-width: 680px">
        <el-form-item label="启用在线充值">
          <el-switch v-model="form.enabled" />
          <span class="muted small" style="margin-left: 10px">
            关闭后钱包页不展示充值区；需代币系统已开启
          </span>
        </el-form-item>
        <el-form-item label="支付提供者">
          <el-radio-group v-model="form.provider">
            <el-radio-button value="mock">模拟支付</el-radio-button>
            <el-radio-button value="aggregate">聚合支付</el-radio-button>
          </el-radio-group>
          <span class="muted small" style="margin-left: 10px">
            模拟支付走站内收银台，不接真实网关，用于演示与测试
          </span>
        </el-form-item>
      </el-form>

      <el-divider content-position="left">聚合网关配置（provider=aggregate 时生效）</el-divider>
      <el-form label-width="150px" style="max-width: 680px">
        <el-form-item label="网关下单地址">
          <el-input v-model="form.gateway_url" placeholder="如 https://pay.example.com/submit.php" />
        </el-form-item>
        <el-form-item label="商户 ID（pid）">
          <el-input v-model="form.app_id" placeholder="聚合支付平台分配的商户号" style="width: 260px" />
        </el-form-item>
        <el-form-item label="商户密钥">
          <el-input
            v-model="form.secret"
            type="password"
            show-password
            :placeholder="cfg.secret_set ? '已设置（不修改请留空）' : '未设置'"
            style="width: 320px"
          />
          <el-button v-if="cfg.secret_set" text type="danger" size="small" @click="form.secret = ''; secretCleared = true">
            清空密钥
          </el-button>
          <div class="muted small">密钥只写不读：保存后不可查看，仅用于服务端验签</div>
        </el-form-item>
        <el-form-item label="异步通知地址">
          <el-input v-model="form.notify_url" placeholder="如 https://你的域名{前缀}/api/payment/notify/alipay（需公网可达）" />
        </el-form-item>
        <el-form-item label="支付完成跳转">
          <el-input v-model="form.return_url" placeholder="如 https://你的域名{前缀}/wallet（可留空，默认站内结果页）" />
        </el-form-item>
      </el-form>

      <el-divider content-position="left">兑换与经济参数</el-divider>
      <el-form label-width="150px" style="max-width: 680px">
        <el-form-item label="兑换比例">
          <el-input-number v-model="form.token_per_fen" :min="1" :max="10000" :step="1" controls-position="right" />
          <span class="muted small" style="margin-left: 10px">代币 / 分（默认 10 → 1 元 = 1000 代币）</span>
        </el-form-item>
        <el-form-item label="单笔限额">
          <el-input-number v-model="form.min_fen" :min="1" :max="100000000" :step="100" controls-position="right" />
          <span style="margin: 0 8px">~</span>
          <el-input-number v-model="form.max_fen" :min="1" :max="100000000" :step="1000" controls-position="right" />
          <span class="muted small" style="margin-left: 10px">分（100 分 = 1 元）</span>
        </el-form-item>
        <el-form-item label="订单有效期">
          <el-input-number v-model="form.order_ttl_min" :min="1" :max="1440" :step="5" controls-position="right" />
          <span class="muted small" style="margin-left: 10px">分钟（超时自动关单）</span>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="saveConfig">保存配置（立即生效）</el-button>
        </el-form-item>
      </el-form>
      <p class="hint-text">
        回调安全模型：不依赖登录态，以 <b>MD5 验签 + 金额核对 + 幂等到账</b> 三道闸门准入；
        订单到账与状态更新、代币入账在同一事务完成，渠道重推不会重复入账。
        渠道金额与档位面额不一致的回调一律拒绝。
      </p>
    </el-card>

    <!-- 会员套餐 / 充值档位 -->
    <el-card shadow="never" class="cfg-card">
      <div class="card-title">
        会员套餐
        <span class="muted small">
          （天数为 0 = 纯充值档位，不延长会员；天数 &gt; 0 会在支付到账时同事务延长会员有效期）
        </span>
      </div>
      <el-table :data="tiers" size="small" border style="max-width: 980px">
        <el-table-column label="面额（分）" width="140">
          <template #default="{ row }">
            <el-input-number v-model="row.fen" :min="100" :max="100000000" :step="100" size="small" controls-position="right" style="width: 120px" />
          </template>
        </el-table-column>
        <el-table-column label="赠送代币" width="130">
          <template #default="{ row }">
            <el-input-number v-model="row.bonus" :min="0" :step="10" size="small" controls-position="right" style="width: 110px" />
          </template>
        </el-table-column>
        <el-table-column label="展示名" width="150">
          <template #default="{ row }">
            <el-input v-model="row.label" maxlength="16" size="small" style="width: 130px" />
          </template>
        </el-table-column>
        <el-table-column label="会员天数" width="120">
          <template #default="{ row }">
            <el-input-number v-model="row.days" :min="0" :max="3650" size="small" controls-position="right" style="width: 100px" />
          </template>
        </el-table-column>
        <el-table-column label="角标" width="110">
          <template #default="{ row }">
            <el-input v-model="row.badge" maxlength="8" size="small" placeholder="推荐" style="width: 90px" />
          </template>
        </el-table-column>
        <!-- 权益用弹窗编辑：内联 textarea 在表格里会挤成一条线，站长根本改不了 -->
        <el-table-column label="权益" width="110">
          <template #default="{ row, $index }">
            <el-button text type="primary" size="small" @click="openFeatures($index)">
              {{ (row.features || []).length }} 条
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80">
          <template #default="{ $index }">
            <el-button text type="danger" size="small" @click="tiers.splice($index, 1)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="tier-actions">
        <el-button
          size="small"
          :icon="Plus"
          @click="tiers.push({ fen: 1000, bonus: 0, label: '新套餐', days: 0, badge: '', features: [] })"
        >添加套餐</el-button>
        <el-button type="primary" size="small" :loading="tiersSaving" @click="saveTiers">
          保存（立即生效，仅影响新订单）
        </el-button>
      </div>
    </el-card>

    <!-- 权益编辑 -->
    <el-dialog v-model="featuresVisible" title="编辑权益说明" width="520px">
      <p class="muted small" style="margin-top:0">
        一行一条，会员页会按顺序逐条展示。不要在这里写"赠送多少代币"——代币由兑换比例实时换算，
        写死会与实际到账不一致。
      </p>
      <el-input
        v-model="featuresText"
        type="textarea"
        :rows="6"
        maxlength="400"
        show-word-limit
        placeholder="会员 30 天&#10;每日免费 AI 次数 ×2"
      />
      <template #footer>
        <el-button @click="featuresVisible = false">取消</el-button>
        <el-button type="primary" @click="saveFeatures">保存</el-button>
      </template>
    </el-dialog>

    <!-- 订单列表 -->
    <el-card shadow="never" class="cfg-card">
      <div class="card-title">充值订单</div>
      <div class="order-filters">
        <el-select v-model="orderFilter.status" placeholder="状态" clearable style="width: 130px" @change="loadOrders">
          <el-option v-for="(v, k) in PAY_STATUS_LABELS" :key="k" :label="v" :value="k" />
        </el-select>
        <el-input
          v-model="orderFilter.userKey"
          placeholder="用户 ID 或用户名"
          style="width: 200px"
          clearable
          @keyup.enter="applyUserFilter"
        />
        <el-button @click="applyUserFilter">查询</el-button>
        <el-button v-if="orderFilter.userID" text type="primary" @click="clearUserFilter">清除用户筛选</el-button>
      </div>
      <el-table :data="orders.list" v-loading="ordersLoading" size="small">
        <el-table-column prop="order_no" label="订单号" min-width="160" show-overflow-tooltip />
        <el-table-column label="用户" min-width="110" show-overflow-tooltip>
          <template #default="{ row }">{{ row.username || row.user_id }}</template>
        </el-table-column>
        <el-table-column label="渠道" width="90">
          <template #default="{ row }">{{ PAY_CHANNEL_LABELS[row.channel] || row.channel }}</template>
        </el-table-column>
        <el-table-column label="金额" width="90">
          <template #default="{ row }">¥{{ (row.amount_fen / 100).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column label="到账代币" width="100">
          <template #default="{ row }">+{{ row.token_amount }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'paid' ? 'success' : row.status === 'pending' ? 'warning' : 'info'">
              {{ PAY_STATUS_LABELS[row.status] || row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="transaction_id" label="渠道流水号" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">{{ row.transaction_id || '—' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="下单时间" min-width="150" show-overflow-tooltip />
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.status === 'pending'"
              text type="primary" size="small"
              @click="markPaid(row)"
            >补单</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager-row">
        <el-pagination
          v-model:current-page="orderFilter.page"
          :page-size="20"
          :total="orders.total"
          layout="prev, pager, next"
          small
          background
          @current-change="loadOrders"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus } from '@element-plus/icons-vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import { paymentApi, PAY_STATUS_LABELS, PAY_CHANNEL_LABELS, type PaymentAdminConfig, type PaymentTier, type PaymentOrderItem } from '@/api/payment';

const loading = ref(false);
const saving = ref(false);
const tiersSaving = ref(false);
const ordersLoading = ref(false);
const secretCleared = ref(false);

const emptyConfig: PaymentAdminConfig = {
  enabled: false, provider: 'mock', gateway_url: '', app_id: '', secret_set: false,
  notify_url: '', return_url: '', token_per_fen: 10, min_fen: 100, max_fen: 1000000,
  order_ttl_min: 30, tiers: [],
  stats: { paid_count: 0, paid_fen: 0, paid_token: 0, pending_count: 0, closed_count: 0 },
};
const cfg = reactive<PaymentAdminConfig>({ ...emptyConfig });
const form = reactive({
  enabled: false, provider: 'mock', gateway_url: '', app_id: '', secret: '',
  notify_url: '', return_url: '', token_per_fen: 10, min_fen: 100, max_fen: 1000000, order_ttl_min: 30,
});
const tiers = ref<PaymentTier[]>([]);

const orderFilter = reactive({ status: '', userKey: '', userID: 0, page: 1 });
const orders = reactive<{ list: PaymentOrderItem[]; total: number }>({ list: [], total: 0 });

async function load() {
  loading.value = true;
  try {
    const res = await paymentApi.getConfig();
    if (res.data.code === 0) {
      Object.assign(cfg, res.data.data);
      form.enabled = cfg.enabled;
      form.provider = cfg.provider;
      form.gateway_url = cfg.gateway_url;
      form.app_id = cfg.app_id;
      form.secret = '';
      form.notify_url = cfg.notify_url;
      form.return_url = cfg.return_url;
      form.token_per_fen = cfg.token_per_fen;
      form.min_fen = cfg.min_fen;
      form.max_fen = cfg.max_fen;
      form.order_ttl_min = cfg.order_ttl_min;
      tiers.value = (cfg.tiers || []).map((t) => ({ ...t }));
      secretCleared.value = false;
    }
  } catch { /* ignore */ }
  loading.value = false;
}

async function saveConfig() {
  saving.value = true;
  try {
    const payload: Parameters<typeof paymentApi.setConfig>[0] = {
      enabled: form.enabled,
      provider: form.provider,
      gateway_url: form.gateway_url.trim(),
      app_id: form.app_id.trim(),
      notify_url: form.notify_url.trim(),
      return_url: form.return_url.trim(),
      token_per_fen: form.token_per_fen,
      min_fen: form.min_fen,
      max_fen: form.max_fen,
      order_ttl_min: form.order_ttl_min,
    };
    // secret 指针语义：未输入且未点清空 → undefined（保持原值）；清空 → ''（清除）
    if (form.secret.trim() !== '') payload.secret = form.secret.trim();
    else if (secretCleared.value) payload.secret = '';
    const res = await paymentApi.setConfig(payload);
    if (res.data.code === 0) {
      ElMessage.success('已保存（立即生效）');
      await load();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  saving.value = false;
}

// ---------- 权益弹窗 ----------
const featuresVisible = ref(false);
const featuresText = ref('');
const featuresIndex = ref(-1);

function openFeatures(i: number) {
  featuresIndex.value = i;
  featuresText.value = (tiers.value[i].features || []).join('\n');
  featuresVisible.value = true;
}

function saveFeatures() {
  const i = featuresIndex.value;
  if (i < 0 || !tiers.value[i]) return;
  tiers.value[i].features = featuresText.value
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
    .slice(0, 8);
  featuresVisible.value = false;
}

async function saveTiers() {
  tiersSaving.value = true;
  try {
    // ⚠️ 必须整条透传：早期这里只挑 fen/bonus/label，结果站长一保存，
    // 套餐的 days / badge / features 全被抹掉，会员页瞬间退回"纯充值"。
    const res = await paymentApi.setTiers(
      tiers.value.map((t) => ({
        fen: Number(t.fen),
        bonus: Number(t.bonus),
        label: String(t.label || '').trim(),
        days: Number(t.days || 0),
        badge: String(t.badge || '').trim(),
        features: (t.features || []).map((f) => String(f).trim()).filter(Boolean),
      })),
    );
    if (res.data.code === 0) {
      ElMessage.success('档位已保存（立即生效）');
      await load();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  tiersSaving.value = false;
}

async function applyUserFilter() {
  orderFilter.page = 1;
  orderFilter.userID = 0;
  const key = orderFilter.userKey.trim();
  if (!key) { loadOrders(); return; }
  // 数字按 ID，否则按用户名解析
  if (/^\d+$/.test(key)) {
    orderFilter.userID = Number(key);
    loadOrders();
    return;
  }
  try {
    const res = await paymentApi.resolveUser(key);
    if (res.data.code === 0 && res.data.data?.id) {
      orderFilter.userID = res.data.data.id;
      loadOrders();
    } else {
      ElMessage.warning('未找到该用户');
    }
  } catch {
    ElMessage.error('用户解析失败');
  }
}

function clearUserFilter() {
  orderFilter.userKey = '';
  orderFilter.userID = 0;
  orderFilter.page = 1;
  loadOrders();
}

async function loadOrders() {
  ordersLoading.value = true;
  try {
    const res = await paymentApi.listOrders({
      status: orderFilter.status || undefined,
      user_id: orderFilter.userID || undefined,
      page: orderFilter.page,
      page_size: 20,
    });
    if (res.data.code === 0) {
      orders.list = res.data.data.list || [];
      orders.total = res.data.data.total || 0;
    }
  } catch { /* ignore */ }
  ordersLoading.value = false;
}

async function markPaid(row: PaymentOrderItem) {
  const { value: txn } = await ElMessageBox.prompt(
    `确认给订单「${row.order_no}」手动补单？\n到账 ${row.token_amount} 代币给用户 ${row.username || row.user_id}。\n可填写渠道流水号（留空则自动生成）：`,
    '手动补单',
    { confirmButtonText: '补单到账', cancelButtonText: '取消', inputPlaceholder: '渠道流水号（选填）' },
  ).catch(() => ({ value: null as string | null }));
  if (txn === null || txn === undefined) return;
  try {
    const res = await paymentApi.markPaid(row.order_no, txn || '');
    if (res.data.code === 0) {
      ElMessage.success(res.data.data.credited ? '补单完成，代币已到账' : '该订单此前已到账（未重复入账）');
      await Promise.all([load(), loadOrders()]);
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '补单失败');
  }
}

onMounted(() => {
  load();
  loadOrders();
});
</script>

<style scoped>
.admin-page { padding: 20px; }
.stats-row { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 12px; margin-bottom: 16px; }
.stat-card { border-radius: 10px; text-align: center; padding: 10px 0; }
.stat-num { font-size: 1.5rem; font-weight: 700; color: var(--el-color-primary, #409eff); }
.stat-num.warning { color: var(--el-color-warning, #e6a23c); }
.stat-num.muted { color: var(--text-light, #999); }
.stat-label { font-size: 0.78rem; color: var(--text-light, #888); margin-top: 2px; }
.cfg-card { border-radius: 10px; margin-bottom: 16px; }
.card-title { font-weight: 600; font-size: 0.95rem; color: var(--text-color, #333); margin-bottom: 12px; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; font-weight: 400; }
.hint-text { color: var(--text-light, #888); font-size: 0.82rem; line-height: 1.7; margin-top: 8px; }
.tier-actions { display: flex; gap: 10px; margin-top: 10px; }
.order-filters { display: flex; gap: 10px; align-items: center; margin-bottom: 12px; }
.pager-row { display: flex; justify-content: flex-end; margin-top: 12px; }
</style>
