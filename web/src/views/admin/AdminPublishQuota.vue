<template>
  <div class="admin-page">
    <AdminHeader title="每日发文限额" subtitle="按用户等级配置每日发布数量 / 总字数，支持按人覆盖；AI 生成内容同样计入" :icon="Timer" />
    <div class="admin-page-body">
      <p class="hint-text">
        对所有发布入口（作品 / 章节 / 博客 / 论坛帖）统一生效。超额返回 429，次日 0 点自动重置。
        数值约定：数量 <b>-1 = 不限</b>、<b>0 = 禁止发布</b>、&gt;0 = 每日上限；字数 <b>0 = 不限</b>、&gt;0 = 每日总字数上限。
        默认关闭，开启后立即生效。
      </p>

      <!-- ============ 全局配置 ============ -->
      <div class="admin-panel">
        <SectionHeader title="等级额度配置" subtitle="用户等级 × 每日上限" :icon="'⏱'" gradient="accent" />
        <div class="enable-row">
          <span>启用每日发文限额</span>
          <el-switch v-model="config.enabled" />
        </div>
        <el-table :data="roleRows" v-loading="loadingConfig" style="width:100%;margin-top:8px" max-height="420">
          <el-table-column label="用户等级" min-width="120">
            <template #default="{ row }">{{ PUBLISH_ROLE_LABELS[row.role] || row.role }}</template>
          </el-table-column>
          <el-table-column label="每日发布数量" min-width="220">
            <template #default="{ row }">
              <el-input-number v-model="row.limit.count" :min="-1" :max="100000" :step="1" size="small" controls-position="right" />
              <span class="muted small">{{ limitText(row.limit.count) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="每日总字数（可选）" min-width="240">
            <template #default="{ row }">
              <el-input-number v-model="row.limit.word_cap" :min="0" :max="100000000" :step="1000" size="small" controls-position="right" />
              <span class="muted small">{{ row.limit.word_cap > 0 ? `${row.limit.word_cap} 字` : '不限' }}</span>
            </template>
          </el-table-column>
        </el-table>
        <div class="save-row">
          <el-button type="primary" :loading="savingConfig" @click="saveConfig">保存配置（立即生效）</el-button>
        </div>
      </div>

      <!-- ============ 按人覆盖 ============ -->
      <div class="admin-panel" style="margin-top:16px">
        <SectionHeader title="按人覆盖" subtitle="对单个用户单独设置额度，留空跟随等级" :icon="'👤'" gradient="purple" />
        <el-form :inline="true" @submit.prevent>
          <el-form-item label="用户（ID 或用户名）">
            <el-input v-model="overrideUserKey" placeholder="如 123 或 herryabu" style="width:180px" @keyup.enter="saveOverride" />
          </el-form-item>
          <el-form-item label="每日数量">
            <el-input-number v-model="overrideCount" :min="-1" :max="100000" :step="1" controls-position="right" />
            <span class="muted small">-1 跟随 / 0 禁止</span>
          </el-form-item>
          <el-form-item label="每日字数">
            <el-input-number v-model="overrideWordCap" :min="-1" :max="100000000" :step="1000" controls-position="right" />
            <span class="muted small">-1 跟随 / 0 不限</span>
          </el-form-item>
          <el-form-item label="备注">
            <el-input v-model="overrideNote" placeholder="选填" style="width:160px" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingOverride" @click="saveOverride">设置 / 更新</el-button>
          </el-form-item>
        </el-form>

        <el-table :data="overrides" v-loading="loadingOverrides" style="width:100%;margin-top:8px">
          <el-table-column prop="user_id" label="用户 ID" width="90" />
          <el-table-column prop="username" label="用户名" min-width="120" show-overflow-tooltip>
            <template #default="{ row }">{{ row.username || `#${row.user_id}` }}</template>
          </el-table-column>
          <el-table-column label="每日数量" min-width="120">
            <template #default="{ row }"><span :class="{ danger: row.count === 0 }">{{ limitText(row.count) }}</span></template>
          </el-table-column>
          <el-table-column label="每日字数" min-width="120">
            <template #default="{ row }">{{ row.word_cap > 0 ? `${row.word_cap} 字` : (row.word_cap === 0 ? '不限' : '跟随') }}</template>
          </el-table-column>
          <el-table-column prop="note" label="备注" min-width="120" show-overflow-tooltip>
            <template #default="{ row }">{{ row.note || '—' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="danger" @click="removeOverride(row.user_id)">撤销</el-button>
            </template>
          </el-table-column>
          <template #empty><EmptyState icon="📭" text="暂无按人覆盖" /></template>
        </el-table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { Timer } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import {
  publishQuotaApi,
  PUBLISH_ROLE_LABELS,
  limitText,
  type DailyPublishConfig,
} from '@/api/publishQuota';

const ROLE_ORDER = ['reader', 'author', 'vip_author', 'arbitrator', 'police', 'admin'];

const config = reactive<DailyPublishConfig>({ enabled: false, roles: {} });
const loadingConfig = ref(false);
const savingConfig = ref(false);

const roleRows = computed(() =>
  ROLE_ORDER.map((role) => ({
    role,
    limit: config.roles[role] || { count: -1, word_cap: 0 },
  })),
);

async function loadConfig() {
  loadingConfig.value = true;
  try {
    const res = await publishQuotaApi.getConfig();
    if (res.data.code === 0 && res.data.data) {
      config.enabled = res.data.data.enabled;
      config.roles = res.data.data.roles || {};
      // 补齐缺失等级，避免渲染空行
      for (const r of ROLE_ORDER) {
        if (!config.roles[r]) config.roles[r] = { count: -1, word_cap: 0 };
      }
    }
  } catch { /* ignore */ }
  loadingConfig.value = false;
}

async function saveConfig() {
  savingConfig.value = true;
  try {
    const res = await publishQuotaApi.setConfig({ enabled: config.enabled, roles: config.roles });
    if (res.data.code === 0) ElMessage.success('已保存（立即生效）');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingConfig.value = false;
}

// ---------- 按人覆盖 ----------
const overrides = ref<any[]>([]);
const loadingOverrides = ref(false);
const overrideUserKey = ref('');
const overrideCount = ref(-1);
const overrideWordCap = ref(-1);
const overrideNote = ref('');
const savingOverride = ref(false);

async function loadOverrides() {
  loadingOverrides.value = true;
  try {
    const res = await publishQuotaApi.listOverrides();
    if (res.data.code === 0) overrides.value = res.data.data || [];
  } catch { /* ignore */ }
  loadingOverrides.value = false;
}

async function resolveOverrideUser(): Promise<number> {
  const key = overrideUserKey.value.trim();
  if (!key) { ElMessage.warning('请输入用户 ID 或用户名'); return 0; }
  if (/^\d+$/.test(key)) return Number(key);
  try {
    const res = await publishQuotaApi.resolveUser(key);
    if (res.data.code === 0 && res.data.data?.id) return res.data.data.id;
    ElMessage.warning('未找到该用户');
  } catch { ElMessage.error('解析用户失败'); }
  return 0;
}

async function saveOverride() {
  const uid = await resolveOverrideUser();
  if (!uid) return;
  savingOverride.value = true;
  try {
    const res = await publishQuotaApi.manageOverride({
      action: 'set',
      user_id: uid,
      count: overrideCount.value,
      word_cap: overrideWordCap.value,
      note: overrideNote.value.trim(),
    });
    if (res.data.code === 0) {
      ElMessage.success('已保存该用户额度');
      overrideUserKey.value = '';
      await loadOverrides();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
  savingOverride.value = false;
}

async function removeOverride(userId: number) {
  try {
    const res = await publishQuotaApi.manageOverride({ action: 'remove', user_id: userId });
    if (res.data.code === 0) {
      ElMessage.success('已撤销');
      await loadOverrides();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

onMounted(() => { loadConfig(); loadOverrides(); });
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; line-height: 1.6; }
.enable-row { display: flex; align-items: center; justify-content: space-between; padding: 8px 4px; font-weight: 600; }
.save-row { margin-top: 12px; text-align: right; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; margin-left: 6px; }
.danger { color: #ef4444; font-weight: 600; }
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
