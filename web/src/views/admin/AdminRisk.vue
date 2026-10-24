<template>
  <div class="page">
    <!-- 接口异常常驻提示：本页没有 AdminHeader，靠这一行拿到全局错误反馈 -->
    <AdminApiAlert />
    <div class="head">
      <h2>充值风控</h2>
      <span class="sub">入账前打分：allow 正常入账 · hold 入账打标待复核 · block 不入账待复核</span>
    </div>

    <div class="cards">
      <div class="card"><div class="num">{{ stats?.pending ?? 0 }}</div><div class="label">待复核</div></div>
      <div class="card"><div class="num">{{ stats?.last7d?.allow ?? 0 }}</div><div class="label">近 7 天放行</div></div>
      <div class="card"><div class="num warn">{{ stats?.last7d?.hold ?? 0 }}</div><div class="label">近 7 天打标</div></div>
      <div class="card"><div class="num danger">{{ stats?.last7d?.block ?? 0 }}</div><div class="label">近 7 天拦截</div></div>
    </div>

    <el-card class="section">
      <template #header><span>规则配置</span></template>
      <el-form label-width="170px" style="max-width: 760px">
        <el-form-item label="启用风控">
          <el-switch v-model="cfg.enabled" />
          <span class="hint">关闭后所有订单直接放行（事件也不再记录）</span>
        </el-form-item>
        <el-form-item label="打标阈值 / 拦截阈值">
          <el-input-number v-model="cfg.hold_score" :min="1" :max="200" />
          <el-input-number v-model="cfg.block_score" :min="1" :max="200" style="margin-left: 10px" />
          <span class="hint">分数越高越异常；拦截必须 ≥ 打标</span>
        </el-form-item>
        <el-form-item label="短时高频">
          <el-input-number v-model="cfg.rapid_window_min" :min="1" :max="1440" />
          <span class="hint">分钟内超过</span>
          <el-input-number v-model="cfg.rapid_count" :min="1" :max="100" style="margin: 0 6px" />
          <span class="hint">笔加分</span>
        </el-form-item>
        <el-form-item label="退款率">
          <el-input-number v-model="cfg.refund_window_days" :min="1" :max="365" />
          <span class="hint">天内退款率超过</span>
          <el-input-number v-model="cfg.max_refund_rate" :min="0" :max="1" :step="0.05" style="margin: 0 6px" />
        </el-form-item>
        <el-form-item label="新号大额">
          <el-input-number v-model="cfg.new_account_hours" :min="0" :max="720" />
          <span class="hint">小时内算新号；大额阈值（分）</span>
          <el-input-number v-model="cfg.big_amount_fen" :min="0" :step="1000" style="margin-left: 6px" />
        </el-form-item>
        <el-form-item label="联动">
          <el-checkbox v-model="cfg.link_honeypot">蜜罐命中 IP 加分</el-checkbox>
          <el-checkbox v-model="cfg.link_device_abuse">同设备多账号加分</el-checkbox>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">保存</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="section">
      <template #header>
        <div class="card-head">
          <span>风控事件</span>
          <div class="filters">
            <el-select v-model="fDecision" clearable placeholder="全部处置" style="width: 130px" @change="loadEvents">
              <el-option label="放行" value="allow" />
              <el-option label="打标" value="hold" />
              <el-option label="拦截" value="block" />
            </el-select>
            <el-select v-model="fStatus" clearable placeholder="全部状态" style="width: 130px" @change="loadEvents">
              <el-option label="待复核" value="open" />
              <el-option label="已放行" value="cleared" />
              <el-option label="已确认" value="confirmed" />
            </el-select>
          </div>
        </div>
      </template>
      <el-table :data="events" size="small">
        <template #empty>
          <StateBlock
            state="empty"
            skeleton="table"
            :rows="4"
            compact
            title="暂无数据"
            description="换个筛选条件试试，或等有数据后再来看。"
          />
        </template>
        <el-table-column prop="order_no" label="订单号" width="180" />
        <el-table-column prop="user_id" label="用户" width="80" />
        <el-table-column prop="score" label="分" width="70" />
        <el-table-column label="处置" width="90">
          <template #empty>
            <StateBlock
              state="empty"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <template #default="{ row }">
            <el-tag size="small" :type="decisionType(row.decision)">{{ decisionLabel(row.decision) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="原因" min-width="280">
          <template #empty>
            <StateBlock
              state="empty"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <template #default="{ row }">
            <span class="reason">{{ reasonText(row.reasons) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #empty>
            <StateBlock
              state="empty"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'open' ? 'warning' : 'info'">
              {{ statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160">
          <template #empty>
            <StateBlock
              state="empty"
              skeleton="table"
              :rows="4"
              compact
              title="暂无数据"
              description="换个筛选条件试试，或等有数据后再来看。"
            />
          </template>
          <template #default="{ row }">
            <template v-if="row.status === 'open'">
              <el-button text size="small" @click="handle(row, 'cleared')">放行</el-button>
              <el-button text size="small" type="danger" @click="handle(row, 'confirmed')">确认异常</el-button>
            </template>
            <span v-else class="hint">{{ row.note || '—' }}</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { riskApi, type RiskConfig, type RiskEvent } from '@/api/costrisk';
import StateBlock from '@/components/common/StateBlock.vue';
import AdminApiAlert from '@/components/admin/AdminApiAlert.vue';

const cfg = reactive<RiskConfig>({
  enabled: true, hold_score: 60, block_score: 85, rapid_window_min: 10, rapid_count: 5,
  refund_window_days: 30, max_refund_rate: 0.3, new_account_hours: 2, big_amount_fen: 50000,
  link_honeypot: true, link_device_abuse: true,
});
const stats = ref<any>(null);
const saving = ref(false);
const events = ref<RiskEvent[]>([]);
const fDecision = ref('');
const fStatus = ref('');

function decisionLabel(d: string) {
  return d === 'block' ? '拦截' : d === 'hold' ? '打标' : '放行';
}
function decisionType(d: string) {
  return d === 'block' ? 'danger' : d === 'hold' ? 'warning' : 'success';
}
function statusLabel(s: string) {
  return s === 'open' ? '待复核' : s === 'cleared' ? '已放行' : '已确认';
}
function reasonText(raw: string) {
  try {
    const arr = JSON.parse(raw || '[]');
    return Array.isArray(arr) && arr.length ? arr.join('；') : '—';
  } catch {
    return raw || '—';
  }
}

async function load() {
  const res = await riskApi.overview();
  Object.assign(cfg, res.data.data.config);
  stats.value = res.data.data.stats;
}
async function save() {
  saving.value = true;
  try {
    await riskApi.saveConfig({ ...cfg });
    ElMessage.success('已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}
async function loadEvents() {
  const res = await riskApi.events(1, 50, fDecision.value, fStatus.value);
  events.value = res.data.data.list || [];
}
async function handle(row: RiskEvent, status: 'cleared' | 'confirmed') {
  const { value } = await ElMessageBox.prompt(
    status === 'cleared' ? '放行说明（如：已电话确认为本人）' : '确认异常说明（如：黑卡，已联系渠道退款）',
    status === 'cleared' ? '放行' : '确认异常',
  );
  await riskApi.handle(row.id, status, value || '');
  ElMessage.success('已处理');
  await loadEvents();
}

onMounted(async () => {
  await load();
  await loadEvents();
});
</script>

<style scoped>
.page { padding: 16px; }
.head { display: flex; align-items: baseline; gap: 12px; margin-bottom: 10px; }
.head h2 { margin: 0; }
.sub, .hint { color: var(--text-muted); font-size: 0.78rem; }
.cards { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-bottom: 14px; }
.card { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 10px; padding: 12px 14px; }
.num { font-size: 1.5rem; font-weight: 700; }
.num.warn { color: var(--el-color-warning); }
.num.danger { color: var(--el-color-danger); }
.label { color: var(--text-muted); font-size: 0.78rem; }
.section { margin-bottom: 14px; }
.card-head { display: flex; align-items: center; justify-content: space-between; }
.filters { display: flex; gap: 8px; }
.reason { font-size: 0.82rem; }
</style>
