<template>
  <div class="page">
    <div class="head">
      <h2>AI 成本与转化</h2>
      <span class="sub">全站总预算 · 高峰排队 · 三维限流 · 免费→付费转化漏斗 · 语音模型</span>
    </div>

    <el-tabs v-model="tab">
      <!-- 预算与排队 -->
      <el-tab-pane label="总预算与排队" name="budget">
        <div class="cards">
          <div class="card"><div class="num">{{ usage?.today ?? 0 }}</div><div class="label">今日消耗（代币）</div></div>
          <div class="card"><div class="num">{{ usage?.month ?? 0 }}</div><div class="label">本月消耗</div></div>
          <div class="card">
            <div class="num" :class="{ warn: usage?.warn }">{{ pct(usage?.daily_ratio) }}</div>
            <div class="label">日预算使用率</div>
          </div>
          <div class="card"><div class="num">{{ inflight }} / {{ effectiveMax || '不限' }}</div><div class="label">当前并发 / 上限</div></div>
          <div class="card"><div class="num">{{ isPeak ? '高峰' : '平峰' }}</div><div class="label">当前时段</div></div>
        </div>

        <el-alert type="warning" :closable="false" class="mb"
          title="HardStop 关闭时「到线只告警不拒绝」——适合先观察再收紧。开启后到线直接拒绝，用户会看到明确提示，不会静默失败。" />

        <el-form label-width="170px" class="panel" style="max-width: 760px">
          <el-form-item label="启用预算闸门"><el-switch v-model="budget.enabled" /></el-form-item>
          <el-form-item label="每日上限（代币）">
            <el-input-number v-model="budget.daily_cap" :min="0" :step="1000" />
            <span class="hint">0 = 不限</span>
          </el-form-item>
          <el-form-item label="每月上限（代币）">
            <el-input-number v-model="budget.monthly_cap" :min="0" :step="10000" />
          </el-form-item>
          <el-form-item label="到线硬拒绝">
            <el-switch v-model="budget.hard_stop" />
          </el-form-item>
          <el-form-item label="启用并发排队">
            <el-switch v-model="budget.queue_enabled" />
          </el-form-item>
          <el-form-item label="并发上限 / 排队超时">
            <el-input-number v-model="budget.max_concurrent" :min="0" :max="500" />
            <el-input-number v-model="budget.queue_timeout_sec" :min="1" :max="300" style="margin-left: 8px" />
            <span class="hint">秒</span>
          </el-form-item>
          <el-form-item label="高峰时段（小时）">
            <el-select v-model="budget.peak_hours" multiple collapse-tags style="width: 100%">
              <el-option v-for="h in 24" :key="h - 1" :label="`${h - 1} 点`" :value="h - 1" />
            </el-select>
          </el-form-item>
          <el-form-item label="高峰并发上限">
            <el-input-number v-model="budget.peak_concurrent" :min="0" :max="500" />
            <span class="hint">不能超过全站上限（超过会被钳制）</span>
          </el-form-item>
          <el-form-item label="告警比例">
            <el-input-number v-model="budget.alert_at_ratio" :min="0.1" :max="1" :step="0.05" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingBudget" @click="saveBudget">保存</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 三维限流 -->
      <el-tab-pane label="三维限流" name="throttle">
        <el-form label-width="200px" class="panel" style="max-width: 700px">
          <el-form-item label="启用"><el-switch v-model="throttle.enabled" /></el-form-item>
          <el-form-item label="用户 × 模型 / 分钟">
            <el-input-number v-model="throttle.per_user_per_model_per_min" :min="0" :max="10000" />
            <span class="hint">0 = 不限；挡住「一个人死磕一个贵模型」</span>
          </el-form-item>
          <el-form-item label="用户 / 分钟">
            <el-input-number v-model="throttle.per_user_per_min" :min="0" :max="10000" />
            <span class="hint">挡住「换模型刷」</span>
          </el-form-item>
          <el-form-item label="模型（全站）/ 分钟">
            <el-input-number v-model="throttle.per_model_per_min" :min="0" :max="100000" />
            <span class="hint">挡住「一群人打一个模型」被上游连坐</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingThrottle" @click="saveThrottle">保存</el-button>
            <span class="hint" style="margin-left: 10px">单实例内存计数；多实例部署时每实例各限一份（近似限流）</span>
          </el-form-item>
        </el-form>

        <el-table :data="throttleTop" size="small" class="panel">
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
          <el-table-column prop="key" label="维度键" min-width="200" />
          <el-table-column prop="dimension" label="维度" width="140" />
          <el-table-column prop="count_1min" label="近 1 分钟次数" width="140" />
        </el-table>
      </el-tab-pane>

      <!-- 漏斗 -->
      <el-tab-pane label="转化漏斗" name="funnel">
        <div class="toolbar">
          <el-input-number v-model="funnelDays" :min="1" :max="180" @change="loadFunnel" />
          <span class="hint">天</span>
          <span class="hint" style="margin-left: 12px">{{ funnel?.insight }}</span>
        </div>
        <div class="funnel">
          <div v-for="(s, i) in funnel?.stages || []" :key="s.key" class="funnel-row">
            <div class="funnel-label">{{ s.label }}</div>
            <div class="funnel-bar-wrap">
              <div class="funnel-bar" :style="{ width: Math.max(4, s.total_rate) + '%', opacity: 1 - i * 0.08 }" />
            </div>
            <div class="funnel-num">{{ s.users }}</div>
            <div class="funnel-rate">
              <span v-if="i > 0">单步 {{ s.step_rate.toFixed(1) }}%</span>
              <span class="small">累计 {{ s.total_rate.toFixed(1) }}%</span>
            </div>
          </div>
        </div>
        <el-alert type="info" :closable="false" class="mt" :title="funnel?.note || ''" />
      </el-tab-pane>

      <!-- 语音 -->
      <el-tab-pane label="语音 / 音乐" name="audio">
        <el-alert :type="audioCount ? 'success' : 'warning'" :closable="false" class="mb"
          :title="audioCount ? `已配置 ${audioCount} 个 audio 模型` : '未配置 audio 模型：/api/ai/speech 会返回「未配置」而不是假装生成'" />
        <el-table :data="audioModels" size="small" class="panel">
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
          <el-table-column prop="name" label="展示名" width="180" />
          <el-table-column prop="model_id" label="上游模型" min-width="180" />
          <el-table-column prop="price_call" label="每次代币" width="110" />
          <el-table-column label="启用" width="90">
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
              <el-tag size="small" :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '是' : '否' }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
        <p class="hint mt">在「AI 模型池」新增模型时把 kind 设为 audio 即可；单次文本上限 {{ 2000 }} 字。</p>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { aiCostApi, type BudgetConfig } from '@/api/costrisk';
import StateBlock from '@/components/common/StateBlock.vue';

const tab = ref('budget');

const budget = reactive<BudgetConfig>({
  enabled: true, daily_cap: 0, monthly_cap: 0, hard_stop: false,
  queue_enabled: true, max_concurrent: 24, queue_timeout_sec: 20,
  peak_hours: [], peak_concurrent: 0, alert_at_ratio: 0.8,
});
const usage = ref<any>(null);
const inflight = ref(0);
const effectiveMax = ref(0);
const isPeak = ref(false);
const savingBudget = ref(false);

const throttle = reactive({
  enabled: true, per_user_per_model_per_min: 20, per_user_per_min: 40,
  per_model_per_min: 200, on_exceed: 'reject',
});
const throttleTop = ref<any[]>([]);
const savingThrottle = ref(false);

const funnelDays = ref(30);
const funnel = ref<any>(null);

const audioModels = ref<any[]>([]);
const audioCount = ref(0);

function pct(v?: number) {
  if (typeof v !== 'number' || !isFinite(v)) return '—';
  return (v * 100).toFixed(0) + '%';
}

async function loadBudget() {
  const res = await aiCostApi.budget();
  Object.assign(budget, res.data.data.config);
  usage.value = res.data.data.usage;
  inflight.value = res.data.data.inflight;
  effectiveMax.value = res.data.data.effective_max;
  isPeak.value = res.data.data.is_peak;
}
async function saveBudget() {
  savingBudget.value = true;
  try {
    await aiCostApi.saveBudget({ ...budget });
    ElMessage.success('已保存并即时生效');
    await loadBudget();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    savingBudget.value = false;
  }
}
async function loadThrottle() {
  const res = await aiCostApi.throttle();
  Object.assign(throttle, res.data.data.config);
  throttleTop.value = res.data.data.top || [];
}
async function saveThrottle() {
  savingThrottle.value = true;
  try {
    await aiCostApi.saveThrottle({ ...throttle });
    ElMessage.success('已保存并即时生效');
    await loadThrottle();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    savingThrottle.value = false;
  }
}
async function loadFunnel() {
  const res = await aiCostApi.funnel(funnelDays.value);
  funnel.value = res.data.data;
}
async function loadAudio() {
  const res = await aiCostApi.audioModels();
  audioModels.value = res.data.data.list || [];
  audioCount.value = res.data.data.count || 0;
}

onMounted(async () => {
  await loadBudget();
  await loadThrottle();
  await loadFunnel();
  await loadAudio();
});
</script>

<style scoped>
.page { padding: 16px; }
.head { display: flex; align-items: baseline; gap: 12px; margin-bottom: 10px; }
.head h2 { margin: 0; }
.sub, .hint, .small { color: var(--text-muted); font-size: 0.78rem; }
.cards { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 12px; margin-bottom: 14px; }
.card { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 10px; padding: 12px 14px; }
.num { font-size: 1.4rem; font-weight: 700; }
.num.warn { color: var(--el-color-warning); }
.label { color: var(--text-muted); font-size: 0.78rem; }
.panel { margin-top: 12px; max-width: 1000px; }
.mb { margin-bottom: 12px; }
.mt { margin-top: 12px; }
.toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.funnel { max-width: 900px; margin-top: 12px; display: flex; flex-direction: column; gap: 10px; }
.funnel-row { display: grid; grid-template-columns: 200px 1fr 80px 190px; align-items: center; gap: 12px; }
.funnel-label { font-size: 0.86rem; }
.funnel-bar-wrap { background: var(--bg-muted); border-radius: 6px; height: 26px; overflow: hidden; }
.funnel-bar { height: 100%; background: linear-gradient(90deg, var(--el-color-primary), var(--el-color-primary-light-3)); }
.funnel-num { font-weight: 700; text-align: right; }
.funnel-rate { display: flex; flex-direction: column; font-size: 0.76rem; }
</style>
