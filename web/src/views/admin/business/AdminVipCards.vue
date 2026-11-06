<template>
  <div class="admin-page">
    <AdminHeader
      title="VIP 等级卡密"
      subtitle="发放会员等级与天数 · 卡号仅哈希存储 · 等级上限由「阅读权限」配置决定"
      :icon="Medal"
    />
    <div class="admin-page-body">
      <p class="hint-text">
        与「代币与卡密」相互独立：这里发放的是<b>会员等级 + 有效天数</b>，用于解锁作品级
        <b>VIP 可见</b>内容。卡密明文只在生成后展示一次，库中只存 sha256；请及时下发或留存。
        兑换时等级取 <b>max(当前等级, 卡等级)</b>，时长从当前到期日起累加。
      </p>

      <div class="admin-panel">
        <SectionHeader title="生成卡密" subtitle="批量生成，等级上限由站长配置" :icon="'🎫'" gradient="purple" />
        <el-form :inline="true" @submit.prevent>
          <el-form-item label="数量">
            <el-input-number v-model="gen.count" :min="1" :max="1000" :step="1" controls-position="right" />
            <span class="muted small">1 ~ 1000 张</span>
          </el-form-item>
          <el-form-item label="会员等级">
            <el-input-number v-model="gen.level" :min="1" :max="vipMaxLevel" :step="1" controls-position="right" />
            <span class="muted small">站长上限 {{ vipMaxLevel }} 级</span>
          </el-form-item>
          <el-form-item label="有效天数">
            <el-input-number v-model="gen.days" :min="1" :max="3650" :step="1" controls-position="right" />
            <span class="muted small">兑换后会员延长天数</span>
          </el-form-item>
          <el-form-item label="卡密有效期">
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
        <div class="vc-config-tip">
          作品侧要求的会员等级（VIP 门槛）与这里的发行上限都在
          <router-link to="/admin/read-permission">阅读权限</router-link> 配置中调整。
        </div>
      </div>

      <div class="admin-panel" style="margin-top:16px">
        <SectionHeader title="卡密列表" subtitle="按状态 / 批次筛选，未使用的可作废" :icon="'📦'" gradient="blue" />
        <el-form :inline="true" @submit.prevent>
          <el-form-item label="状态">
            <el-select v-model="filter.status" placeholder="全部" clearable style="width:140px" @change="loadCards">
              <el-option label="未使用" value="unused" />
              <el-option label="已使用" value="used" />
              <el-option label="已作废" value="void" />
            </el-select>
          </el-form-item>
          <el-form-item label="批次">
            <el-input v-model="filter.batch" placeholder="如 V20260101120000" clearable style="width:220px" @keyup.enter="loadCards" @clear="loadCards" />
          </el-form-item>
          <el-form-item>
            <el-button @click="loadCards">刷新</el-button>
          </el-form-item>
        </el-form>

        <el-table :data="cards" v-loading="loadingCards" style="width:100%" size="small">
          <el-table-column prop="code_prefix" label="卡号前缀" width="110" />
          <el-table-column label="等级" width="80">
            <template #default="{ row }"><el-tag size="small" effect="plain">Lv.{{ row.level }}</el-tag></template>
          </el-table-column>
          <el-table-column prop="days" label="天数" width="80" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag size="small" :type="statusTag(row.status)">{{ statusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="batch_id" label="批次号" min-width="150" show-overflow-tooltip />
          <el-table-column label="使用者" min-width="120" show-overflow-tooltip>
            <template #default="{ row }">{{ row.used_by_username || (row.used_by ? `#${row.used_by}` : '—') }}</template>
          </el-table-column>
          <el-table-column prop="note" label="备注" min-width="120" show-overflow-tooltip />
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="row.status === 'unused'"
                size="small"
                text
                type="danger"
                :loading="voidingId === row.id"
                @click="voidCard(row)"
              >作废</el-button>
              <span v-else class="muted small">—</span>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <!-- 明文仅本次可见 -->
    <el-dialog v-model="codesDialog" title="卡密已生成（仅本次可见）" width="560px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" show-icon title="请立即复制保存"
        description="关闭后无法再次查看明文，系统只保留哈希。" />
      <div class="vc-codes">
        <div v-for="c in generatedCodes" :key="c" class="vc-code">{{ c }}</div>
      </div>
      <template #footer>
        <el-button @click="copyCodes">{{ copied ? '已复制' : '复制全部' }}</el-button>
        <el-button type="primary" @click="codesDialog = false">我已保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Medal } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import { vipCardApi, type VipCard, type GenerateVipCardsInput } from '@/api/vipcard';
import { readPermApi } from '@/api/readperm';

const vipMaxLevel = ref(3);
const generating = ref(false);
const codesDialog = ref(false);
const generatedCodes = ref<string[]>([]);
const copied = ref(false);

const gen = reactive<GenerateVipCardsInput>({
  count: 10, level: 1, days: 30, expires_in: 0, note: '',
});

const cards = ref<VipCard[]>([]);
const loadingCards = ref(false);
const voidingId = ref(0);
const filter = reactive<{ status: string; batch: string }>({ status: '', batch: '' });

function statusTag(s: string) {
  const map: Record<string, string> = { unused: 'success', used: 'info', void: 'danger' };
  return (map[s] as any) || 'info';
}
function statusLabel(s: string) {
  const map: Record<string, string> = { unused: '未使用', used: '已使用', void: '已作废' };
  return map[s] || s;
}

async function loadVipMaxLevel() {
  try {
    const res = await readPermApi.adminGet();
    const max = res.data?.data?.config?.vip_max_level;
    if (max && max > 0) {
      vipMaxLevel.value = max;
      if (gen.level > max) gen.level = max;
    }
  } catch { /* 使用默认上限 3 */ }
}

async function generateCards() {
  generating.value = true;
  try {
    const res = await vipCardApi.adminGenerate({
      count: gen.count,
      level: gen.level,
      days: gen.days,
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
  } finally {
    generating.value = false;
  }
}

async function copyCodes() {
  try {
    await navigator.clipboard.writeText(generatedCodes.value.join('\n'));
    copied.value = true;
    ElMessage.success('已复制到剪贴板');
  } catch {
    ElMessage.warning('浏览器拒绝访问剪贴板，请手动选择复制');
  }
}

async function loadCards() {
  loadingCards.value = true;
  try {
    const res = await vipCardApi.adminList({
      status: filter.status || undefined,
      batch: filter.batch.trim() || undefined,
    });
    if (res.data.code === 0) cards.value = res.data.data || [];
  } catch { /* ignore */ }
  finally { loadingCards.value = false; }
}

async function voidCard(row: VipCard) {
  try {
    await ElMessageBox.confirm(`确定作废卡密 ${row.code_prefix}-****？作废后无法恢复。`, '作废确认', { type: 'warning' });
  } catch { return; }
  voidingId.value = row.id;
  try {
    await vipCardApi.adminVoid(row.id);
    ElMessage.success('已作废');
    await loadCards();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '作废失败');
  } finally {
    voidingId.value = 0;
  }
}

onMounted(() => {
  loadVipMaxLevel();
  loadCards();
});
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: var(--admin-content-max); margin: 0 auto; }
.hint-text { font-size: 0.85rem; line-height: 1.7; color: var(--text-light, #666); margin-bottom: 16px; }
.admin-panel { background: var(--bg-card, #fff); border-radius: var(--radius-md, 10px); padding: 20px; box-shadow: var(--shadow-xs, 0 1px 3px rgba(0,0,0,.06)); }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; margin-left: 8px; }
.vc-config-tip { margin-top: 6px; font-size: 0.8rem; color: var(--text-light, #999); }
.vc-config-tip a { color: var(--el-color-primary); }
.vc-codes { margin-top: 12px; max-height: 320px; overflow: auto; padding: 10px; border-radius: 8px; background: var(--bg-page, #f7f8fa); font-family: ui-monospace, Consolas, monospace; }
.vc-code { padding: 3px 0; font-size: 0.9rem; letter-spacing: 0.06em; }
</style>
