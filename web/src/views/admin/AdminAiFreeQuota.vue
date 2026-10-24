<template>
  <div class="admin-page" v-loading="loading">
    <!-- 接口异常常驻提示：本页没有 AdminHeader，靠这一行拿到全局错误反馈 -->
    <AdminApiAlert />
    <SectionHeader title="免费 AI 额度" subtitle="每日免费调用额度，用尽才扣代币；支持按人覆盖" />

    <el-card class="cfg-card" shadow="never">
      <template #header><span>平台默认额度</span></template>
      <el-form label-width="150px" style="max-width: 560px">
        <el-form-item label="启用免费额度">
          <el-switch v-model="form.enabled" />
          <span class="muted small" style="margin-left: 10px">关闭后所有调用均按代币计费</span>
        </el-form-item>
        <el-form-item label="每日免费次数">
          <el-radio-group v-model="callsMode">
            <el-radio-button value="limited">限制次数</el-radio-button>
            <el-radio-button value="unlimited">不限次数</el-radio-button>
            <el-radio-button value="off">不提供</el-radio-button>
          </el-radio-group>
          <el-input-number
            v-if="callsMode === 'limited'"
            v-model="form.calls_per_day"
            :min="1" :max="1000" :step="1"
            controls-position="right" style="margin-left: 10px"
          />
          <span class="muted small" style="margin-left: 10px">次 / 日</span>
        </el-form-item>
        <el-form-item label="每日免费 tokens 上限">
          <el-radio-group v-model="tokensMode">
            <el-radio-button value="unlimited">不限</el-radio-button>
            <el-radio-button value="limited">限制</el-radio-button>
          </el-radio-group>
          <el-input-number
            v-if="tokensMode === 'limited'"
            v-model="form.max_tokens_per_day"
            :min="1000" :max="10000000" :step="1000"
            controls-position="right" style="margin-left: 10px"
          />
          <span class="muted small" style="margin-left: 10px">tokens / 日</span>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="saveConfig">保存（立即生效）</el-button>
        </el-form-item>
      </el-form>
      <p class="hint-text">
        免费额度<b>优先于代币扣费</b>：命中免费额度的调用不产生代币流水（调用记录标注「免费」）。
        次日 0 点重置。未开启代币系统时，免费额度仍可独立使用。
      </p>
    </el-card>

    <el-card class="ov-card" shadow="never">
      <template #header>
        <div class="ov-head">
          <span>按人覆盖（优先于平台默认）</span>
          <el-button type="primary" size="small" :icon="Plus" @click="openAdd">添加覆盖</el-button>
        </div>
      </template>
      <el-table :data="overrides" v-loading="listLoading" style="width:100%">
        <template #empty>
          <StateBlock
            :state="loading ? 'loading' : 'empty'"
            skeleton="table"
            :rows="4"
            compact
            title="暂无数据"
            description="换个筛选条件试试，或等有数据后再来看。"
          />
        </template>
        <el-table-column prop="user_id" label="用户 ID" width="90" />
        <el-table-column prop="username" label="用户名" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">{{ row.username || '—' }}</template>
        </el-table-column>
        <el-table-column label="每日免费次数" min-width="130">
          <template #default="{ row }">
            <el-tag size="small" :type="row.daily_calls === 0 ? 'danger' : row.daily_calls < 0 ? 'info' : 'success'">
              {{ row.daily_calls === 0 ? '禁止' : row.daily_calls < 0 ? '不限' : row.daily_calls + ' 次' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="每日 tokens" min-width="120">
          <template #default="{ row }">
            {{ row.daily_tokens < 0 ? '不可用' : row.daily_tokens === 0 ? '不限' : row.daily_tokens }}
          </template>
        </el-table-column>
        <el-table-column prop="note" label="备注" min-width="140" show-overflow-tooltip />
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button text type="danger" size="small" @click="removeOverride(row)">撤销</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 添加/编辑覆盖 -->
    <el-dialog v-model="dialogVisible" title="设置用户免费额度" width="440px">
      <el-form label-width="110px">
        <el-form-item label="用户" required>
          <el-input v-model="resolveQuery" placeholder="用户 ID 或用户名" @keyup.enter="resolveUser" />
          <el-button size="small" style="margin-left: 8px" :loading="resolving" @click="resolveUser">解析</el-button>
          <div v-if="resolved" class="resolved-tip">已选择：{{ resolved }}</div>
        </el-form-item>
        <el-form-item label="每日免费次数">
          <el-radio-group v-model="ovCallsMode">
            <el-radio-button value="limited">限制</el-radio-button>
            <el-radio-button value="unlimited">不限</el-radio-button>
            <el-radio-button value="off">禁止</el-radio-button>
          </el-radio-group>
          <el-input-number
            v-if="ovCallsMode === 'limited'"
            v-model="ovForm.daily_calls"
            :min="1" :max="1000" :step="1"
            controls-position="right" style="margin-left: 10px"
          />
        </el-form-item>
        <el-form-item label="每日 tokens">
          <el-radio-group v-model="ovTokensMode">
            <el-radio-button value="unlimited">不限</el-radio-button>
            <el-radio-button value="limited">限制</el-radio-button>
          </el-radio-group>
          <el-input-number
            v-if="ovTokensMode === 'limited'"
            v-model="ovForm.daily_tokens"
            :min="1000" :max="10000000" :step="1000"
            controls-position="right" style="margin-left: 10px"
          />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="ovForm.note" placeholder="选填，如：活动赠送" maxlength="128" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveOverride">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus } from '@element-plus/icons-vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import { aiApi, type AdminAiFreeQuotaOverride } from '@/api/ai';
import { adminApi } from '@/api/admin';
import StateBlock from '@/components/common/StateBlock.vue';
import AdminApiAlert from '@/components/admin/AdminApiAlert.vue';

const loading = ref(false);
const listLoading = ref(false);
const saving = ref(false);

const form = reactive({ enabled: false, calls_per_day: 5, max_tokens_per_day: 0 });
const callsMode = ref<'limited' | 'unlimited' | 'off'>('limited');
const tokensMode = ref<'unlimited' | 'limited'>('unlimited');

const overrides = ref<AdminAiFreeQuotaOverride[]>([]);

// callsMode ↔ calls_per_day（-1=不限 / 0=不提供 / >0=限制）
watch(callsMode, (m) => {
  if (m === 'unlimited') form.calls_per_day = -1;
  else if (m === 'off') form.calls_per_day = 0;
  else if (form.calls_per_day <= 0) form.calls_per_day = 5;
});
watch(tokensMode, (m) => {
  form.max_tokens_per_day = m === 'unlimited' ? 0 : 100000;
});
watch(() => form.calls_per_day, (v) => {
  if (v < 0) callsMode.value = 'unlimited';
  else if (v === 0) callsMode.value = 'off';
  else callsMode.value = 'limited';
});

async function loadConfig() {
  loading.value = true;
  try {
    const res = await aiApi.getFreeQuotaConfig();
    if (res.data.code === 0) {
      form.enabled = res.data.data.enabled;
      form.calls_per_day = res.data.data.config.calls_per_day;
      form.max_tokens_per_day = res.data.data.config.max_tokens_per_day;
    }
  } catch { /* ignore */ }
  loading.value = false;
}

async function saveConfig() {
  saving.value = true;
  try {
    const res = await aiApi.setFreeQuotaConfig({
      enabled: form.enabled,
      calls_per_day: form.calls_per_day,
      max_tokens_per_day: form.max_tokens_per_day,
    });
    if (res.data.code === 0) ElMessage.success('已保存（立即生效）');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  saving.value = false;
}

async function loadOverrides() {
  listLoading.value = true;
  try {
    const res = await aiApi.listFreeQuotaOverrides();
    if (res.data.code === 0) {
      const list = res.data.data || [];
      // 批量补用户名（resolveUser：数字=id）
      await Promise.all(list.map(async (o) => {
        try {
          const r = await adminApi.resolveUser(String(o.user_id));
          if (r.data.code === 0 && r.data.data?.username) o.username = r.data.data.username;
        } catch { /* ignore */ }
      }));
      overrides.value = list;
    }
  } catch { /* ignore */ }
  listLoading.value = false;
}

// ── 覆盖弹窗 ──
const dialogVisible = ref(false);
const resolving = ref(false);
const resolveQuery = ref('');
const resolved = ref('');
const ovForm = reactive({ user_id: 0, daily_calls: 5, daily_tokens: 0, note: '' });
const ovCallsMode = ref<'limited' | 'unlimited' | 'off'>('limited');
const ovTokensMode = ref<'unlimited' | 'limited'>('unlimited');

watch(ovCallsMode, (m) => {
  ovForm.daily_calls = m === 'unlimited' ? -1 : m === 'off' ? 0 : 5;
});
watch(ovTokensMode, (m) => {
  ovForm.daily_tokens = m === 'unlimited' ? 0 : 100000;
});

function openAdd() {
  resolveQuery.value = '';
  resolved.value = '';
  ovForm.user_id = 0;
  ovForm.daily_calls = 5;
  ovForm.daily_tokens = 0;
  ovForm.note = '';
  ovCallsMode.value = 'limited';
  ovTokensMode.value = 'unlimited';
  dialogVisible.value = true;
}

async function resolveUser() {
  const q = resolveQuery.value.trim();
  if (!q) return;
  resolving.value = true;
  try {
    const res = await adminApi.resolveUser(q);
    if (res.data.code === 0 && res.data.data) {
      ovForm.user_id = res.data.data.id;
      resolved.value = `${res.data.data.username}（ID ${res.data.data.id}）`;
    } else {
      ElMessage.warning('未找到该用户');
    }
  } catch {
    ElMessage.error('解析失败');
  }
  resolving.value = false;
}

async function saveOverride() {
  if (!ovForm.user_id) {
    ElMessage.warning('请先解析并选择用户');
    return;
  }
  saving.value = true;
  try {
    const res = await aiApi.manageFreeQuotaOverride({
      action: 'set',
      user_id: ovForm.user_id,
      daily_calls: ovForm.daily_calls,
      daily_tokens: ovForm.daily_tokens,
      note: ovForm.note.trim(),
    });
    if (res.data.code === 0) {
      ElMessage.success('已保存');
      dialogVisible.value = false;
      await loadOverrides();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  saving.value = false;
}

async function removeOverride(row: AdminAiFreeQuotaOverride) {
  await ElMessageBox.confirm(`撤销用户 #${row.user_id} 的免费额度覆盖？恢复后将跟随平台默认。`, '确认', { type: 'warning' });
  try {
    const res = await aiApi.manageFreeQuotaOverride({ action: 'remove', user_id: row.user_id });
    if (res.data.code === 0) {
      ElMessage.success('已撤销');
      await loadOverrides();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

onMounted(async () => {
  await loadConfig();
  await loadOverrides();
});
</script>

<style scoped>
.cfg-card, .ov-card { margin-bottom: 16px; }
.ov-head { display: flex; align-items: center; justify-content: space-between; }
.hint-text { color: var(--text-light, #888); font-size: 0.82rem; line-height: 1.7; }
.resolved-tip { margin-top: 6px; font-size: 0.8rem; color: #16a34a; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; }
</style>
