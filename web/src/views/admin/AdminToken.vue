<template>
  <div class="admin-page">
    <AdminHeader title="代币与卡密" subtitle="平台内消费积分：开关 / 命名 / 卡密批次 / 授信调整，与作者收益账相互独立" :icon="Wallet" />
    <div class="admin-page-body">
      <p class="hint-text">
        代币是<b>平台内消费积分</b>：不可提现、不可转账、不可兑换现金；作者收益（法币、可提现）为另一套账，两不相混。
        默认关闭，开启后立即生效。卡密仅<b>哈希存储</b>，明文只在生成后展示一次，请及时下发或留存。
      </p>

      <!-- ============ 基础配置 ============ -->
      <div class="admin-panel">
        <SectionHeader title="基础配置" subtitle="总开关与代币展示名" :icon="'💰'" gradient="accent" />
        <div class="enable-row">
          <span>启用代币系统</span>
          <el-switch v-model="config.enabled" />
        </div>
        <el-form :inline="true" @submit.prevent style="margin-top:8px">
          <el-form-item label="代币名称">
            <el-input v-model="config.name" placeholder="如：星币" maxlength="16" show-word-limit style="width:200px" />
            <span class="muted small">最多 16 字，展示于钱包与流水</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingConfig" @click="saveConfig">保存配置（立即生效）</el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- ============ 卡密生成 ============ -->
      <div class="admin-panel" style="margin-top:16px">
        <SectionHeader title="生成卡密" subtitle="批量生成，明文仅展示一次" :icon="'🎫'" gradient="purple" />
        <el-form :inline="true" @submit.prevent>
          <el-form-item label="数量">
            <el-input-number v-model="gen.count" :min="1" :max="1000" :step="1" controls-position="right" />
            <span class="muted small">1 ~ 1000 张</span>
          </el-form-item>
          <el-form-item label="面值（每个）">
            <el-input-number v-model="gen.value" :min="1" :max="100000000" :step="1" controls-position="right" />
          </el-form-item>
          <el-form-item label="有效期">
            <el-input-number v-model="gen.expires_in" :min="0" :max="3650" :step="1" controls-position="right" />
            <span class="muted small">天，0 = 永久</span>
          </el-form-item>
          <el-form-item label="备注">
            <el-input v-model="gen.note" placeholder="选填，如渠道/活动" style="width:180px" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="generating" @click="generateCards">生成卡密</el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- ============ 卡密列表 ============ -->
      <div class="admin-panel" style="margin-top:16px">
        <SectionHeader title="卡密列表" subtitle="可按状态 / 批次筛选，未使用的可作废" :icon="'📦'" gradient="blue" />
        <el-form :inline="true" @submit.prevent>
          <el-form-item label="状态">
            <el-select v-model="cardFilter.status" placeholder="全部" clearable style="width:140px" @change="loadCards">
              <el-option label="未使用" value="unused" />
              <el-option label="已使用" value="used" />
              <el-option label="已作废" value="void" />
            </el-select>
          </el-form-item>
          <el-form-item label="批次号">
            <el-input v-model="cardFilter.batch" placeholder="如 B20260101120000" clearable style="width:220px" @keyup.enter="loadCards" @clear="loadCards" />
          </el-form-item>
          <el-form-item>
            <el-button @click="loadCards">查询</el-button>
          </el-form-item>
        </el-form>

        <el-table :data="cards" v-loading="loadingCards" style="width:100%">
          <el-table-column prop="code_prefix" label="卡密（前缀）" width="130">
            <template #default="{ row }"><span class="mono">{{ row.code_prefix }}-****-****-****</span></template>
          </el-table-column>
          <el-table-column prop="value" label="面值" width="90" />
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="cardStatusTag(row.status)">{{ cardStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="batch_id" label="批次号" min-width="150" show-overflow-tooltip />
          <el-table-column label="有效期" width="170">
            <template #default="{ row }">
              <span v-if="row.expires_at" :class="{ expired: isExpired(row.expires_at) }">{{ row.expires_at }}</span>
              <span v-else class="muted">永久</span>
            </template>
          </el-table-column>
          <el-table-column label="使用者" min-width="120" show-overflow-tooltip>
            <template #default="{ row }">
              {{ row.used_by_username || (row.used_by ? `#${row.used_by}` : '—') }}
            </template>
          </el-table-column>
          <el-table-column prop="note" label="备注" min-width="110" show-overflow-tooltip>
            <template #default="{ row }">{{ row.note || '—' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="row.status === 'unused'"
                size="small" text type="danger"
                :loading="voidingId === row.id"
                @click="voidCard(row)"
              >作废</el-button>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <template #empty><EmptyState icon="🎫" text="暂无卡密记录" /></template>
        </el-table>
      </div>

      <!-- ============ 授信调整 ============ -->
      <div class="admin-panel" style="margin-top:16px">
        <SectionHeader title="授信调整" subtitle="管理员增发 / 扣减（支持按用户名定位）" :icon="'⚖️'" gradient="teal" />
        <el-form :inline="true" @submit.prevent>
          <el-form-item label="用户（ID 或用户名）">
            <el-input v-model="adjust.userKey" placeholder="如 123 或 herryabu" style="width:180px" />
          </el-form-item>
          <el-form-item label="金额">
            <el-input-number v-model="adjust.amount" :step="1" controls-position="right" style="width:160px" />
            <span class="muted small">正数 = 增发，负数 = 扣减</span>
          </el-form-item>
          <el-form-item label="备注">
            <el-input v-model="adjust.remark" placeholder="选填，如活动补偿" style="width:180px" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="adjusting" @click="doAdjust">确认调整</el-button>
          </el-form-item>
        </el-form>
        <p class="muted small" style="margin:0">扣减后余额为负时将被拒绝（余额不足）。调整会记入用户流水，类型为「增发 / 扣减」。</p>
      </div>
    </div>

    <!-- ============ 卡密明文一次性展示 ============ -->
    <el-dialog v-model="codesDialog" title="卡密已生成（仅本次可见）" width="560px" :close-on-click-modal="false">
      <el-alert
        title="以下卡密仅本次展示，关闭后无法再次查看；请立即复制下发。"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom:12px"
      />
      <div class="codes-box mono">{{ generatedCodes.join('\n') }}</div>
      <template #footer>
        <el-button @click="copyCodes">{{ copied ? '已复制' : '复制全部' }}</el-button>
        <el-button type="primary" @click="codesDialog = false">我已保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Wallet } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import { tokenApi } from '@/api/token';

// ---------- 基础配置 ----------
const config = reactive<{ enabled: boolean; name: string }>({ enabled: false, name: '代币' });
const savingConfig = ref(false);

async function loadConfig() {
  try {
    const res = await tokenApi.getConfig();
    if (res.data.code === 0) {
      config.enabled = res.data.data.enabled;
      config.name = res.data.data.name;
    }
  } catch { /* ignore */ }
}

async function saveConfig() {
  savingConfig.value = true;
  try {
    const res = await tokenApi.setConfig({ enabled: config.enabled, name: config.name.trim() });
    if (res.data.code === 0) ElMessage.success('已保存（立即生效）');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingConfig.value = false;
}

// ---------- 生成卡密 ----------
const gen = reactive<{ count: number; value: number; expires_in: number; note: string }>({
  count: 10,
  value: 100,
  expires_in: 0,
  note: '',
});
const generating = ref(false);
const codesDialog = ref(false);
const generatedCodes = ref<string[]>([]);
const copied = ref(false);

async function generateCards() {
  generating.value = true;
  try {
    const res = await tokenApi.generateCards({
      count: gen.count,
      value: gen.value,
      expires_in: gen.expires_in,
      note: gen.note.trim(),
    });
    if (res.data.code === 0) {
      generatedCodes.value = res.data.data.codes || [];
      codesDialog.value = true;
      copied.value = false;
      await loadCards();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成失败');
  }
  generating.value = false;
}

async function copyCodes() {
  const text = generatedCodes.value.join('\n');
  try {
    await navigator.clipboard.writeText(text);
    copied.value = true;
    ElMessage.success('已复制到剪贴板');
  } catch {
    ElMessage.warning('浏览器拒绝访问剪贴板，请手动选择复制');
  }
}

// ---------- 卡密列表 ----------
const cards = ref<any[]>([]);
const loadingCards = ref(false);
const cardFilter = reactive<{ status: string; batch: string }>({ status: '', batch: '' });
const voidingId = ref(0);

async function loadCards() {
  loadingCards.value = true;
  try {
    const res = await tokenApi.listCards({ status: cardFilter.status || undefined, batch: cardFilter.batch.trim() || undefined });
    if (res.data.code === 0) cards.value = res.data.data || [];
  } catch { /* ignore */ }
  loadingCards.value = false;
}

function cardStatusTag(s: string) {
  const map: Record<string, string> = { unused: 'success', used: 'info', void: 'danger' };
  return (map[s] as any) || 'info';
}
function cardStatusLabel(s: string) {
  const map: Record<string, string> = { unused: '未使用', used: '已使用', void: '已作废' };
  return map[s] || s;
}
function isExpired(t: string) {
  return new Date(t).getTime() < Date.now();
}

async function voidCard(row: any) {
  try {
    await ElMessageBox.confirm(`确定作废卡密 ${row.code_prefix}-****？作废后无法恢复。`, '作废确认', { type: 'warning' });
  } catch { return; }
  voidingId.value = row.id;
  try {
    const res = await tokenApi.voidCard(row.id);
    if (res.data.code === 0) {
      ElMessage.success('已作废');
      await loadCards();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '作废失败');
  }
  voidingId.value = 0;
}

// ---------- 授信调整 ----------
const adjust = reactive<{ userKey: string; amount: number; remark: string }>({ userKey: '', amount: 0, remark: '' });
const adjusting = ref(false);

async function doAdjust() {
  const key = adjust.userKey.trim();
  if (!key) { ElMessage.warning('请输入用户 ID 或用户名'); return; }
  if (!adjust.amount) { ElMessage.warning('调整金额不能为 0'); return; }
  let userId = 0;
  if (/^\d+$/.test(key)) {
    userId = Number(key);
  } else {
    try {
      const res = await tokenApi.resolveUser(key);
      if (res.data.code === 0 && res.data.data?.id) userId = res.data.data.id;
    } catch { /* ignore */ }
    if (!userId) { ElMessage.warning('未找到该用户'); return; }
  }
  adjusting.value = true;
  try {
    const res = await tokenApi.adjust({ user_id: userId, amount: adjust.amount, remark: adjust.remark.trim() });
    if (res.data.code === 0) {
      ElMessage.success(`已调整，该用户当前余额 ${res.data.data?.balance ?? ''}`);
      adjust.userKey = '';
      adjust.amount = 0;
      adjust.remark = '';
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '调整失败');
  }
  adjusting.value = false;
}

onMounted(() => { loadConfig(); loadCards(); });
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; line-height: 1.6; }
.enable-row { display: flex; align-items: center; justify-content: space-between; padding: 8px 4px; font-weight: 600; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; margin-left: 6px; }
.mono { font-family: Consolas, 'Courier New', monospace; }
.expired { color: #ef4444; }
.codes-box {
  max-height: 320px;
  overflow: auto;
  white-space: pre;
  border: 1px solid var(--el-border-color, #dcdfe6);
  border-radius: 8px;
  padding: 12px;
  font-size: 0.9rem;
  line-height: 1.9;
  background: var(--el-fill-color-light, #f8f9fa);
  user-select: all;
}
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .codes-box { background: #1e293b; }
</style>
