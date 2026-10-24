<template>
  <div class="admin-page">
    <div class="head">
      <h2>注册门槛与防刷</h2>
      <span class="sub">邀请码 · 设备指纹 · 账号年龄 · 新号限额 · API Key 审计告警</span>
    </div>

    <el-tabs v-model="tab">
      <!-- 门槛配置 -->
      <el-tab-pane label="门槛配置" name="config">
        <el-alert type="info" :closable="false" class="mb"
          title="门槛默认全关，开启后不影响已有账号。设备指纹可被伪造（清 localStorage 即变），属「提高批量注册成本」而非绝对防御；真正硬的是邀请码与蜜罐 IP。" />
        <el-form label-width="180px" class="panel">
          <el-form-item label="启用门槛">
            <el-switch v-model="cfg.enabled" />
          </el-form-item>
          <el-form-item label="必须邀请码">
            <el-switch v-model="cfg.require_invite" :disabled="!cfg.enabled" />
            <span class="hint">开启后注册必须带邀请码，需在「邀请码」页签生成</span>
          </el-form-item>
          <el-form-item label="同设备账号上限">
            <el-input-number v-model="cfg.max_accounts_per_device" :min="0" :max="50" :disabled="!cfg.enabled" />
            <span class="hint">0 = 不限；校园网/公司出口会因兜底指纹（ip+ua）误判，判断时以设备页签为准</span>
          </el-form-item>
          <el-form-item label="拒绝蜜罐命中 IP">
            <el-switch v-model="cfg.block_honeypot_ip" :disabled="!cfg.enabled" />
          </el-form-item>
          <el-form-item label="拒绝临时邮箱">
            <el-switch v-model="cfg.block_disposable_email" :disabled="!cfg.enabled" />
          </el-form-item>
          <el-form-item label="发文最小账号年龄(小时)">
            <el-input-number v-model="cfg.post_min_age_hours" :min="0" :max="720" :disabled="!cfg.enabled" />
          </el-form-item>
          <el-form-item label="评论最小账号年龄(小时)">
            <el-input-number v-model="cfg.comment_min_age_hours" :min="0" :max="720" :disabled="!cfg.enabled" />
          </el-form-item>
          <el-form-item label="新号每日上限(条)">
            <el-input-number v-model="cfg.new_user_daily_posts" :min="0" :max="200" :disabled="!cfg.enabled" />
            <el-input-number v-model="cfg.new_user_days" :min="1" :max="90" :disabled="!cfg.enabled" class="ml" />
            <span class="hint">天内算新号</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="saving" @click="saveCfg">保存</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 邀请码 -->
      <el-tab-pane label="邀请码" name="invite">
        <div class="toolbar">
          <el-input-number v-model="newInvite.count" :min="1" :max="100" />
          <span class="hint">个 · 每个可用</span>
          <el-input-number v-model="newInvite.max_uses" :min="1" :max="100" />
          <span class="hint">次 · 有效</span>
          <el-input-number v-model="newInvite.ttl_hours" :min="0" :max="8760" />
          <span class="hint">小时(0=永久)</span>
          <el-input v-model="newInvite.note" placeholder="备注（可选）" style="width: 180px" />
          <el-button type="primary" :loading="creating" @click="createInvites">生成</el-button>
        </div>
        <el-table :data="invites" size="small" class="panel">
          <el-table-column prop="code" label="邀请码" width="220">
            <template #default="{ row }">
              <code class="code">{{ row.code }}</code>
              <el-button text size="small" @click="copy(row.code)">复制</el-button>
            </template>
          </el-table-column>
          <el-table-column label="用量" width="100">
            <template #default="{ row }">{{ row.used }} / {{ row.max_uses }}</template>
          </el-table-column>
          <el-table-column prop="note" label="备注" />
          <el-table-column label="过期" width="170">
            <template #default="{ row }">{{ row.expires_at ? fmt(row.expires_at) : '永久' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="90">
            <template #default="{ row }">
              <el-button text type="danger" size="small" @click="revoke(row)">作废</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- 设备 -->
      <el-tab-pane label="设备指纹" name="device">
        <div class="toolbar">
          <el-input v-model="deviceKeyword" placeholder="按设备 ID / IP 搜索" style="width: 240px" clearable
            @keyup.enter="loadDevices" />
          <el-button @click="loadDevices">搜索</el-button>
          <span class="hint">{{ deviceNote }}</span>
        </div>
        <el-table :data="devices" size="small" class="panel">
          <el-table-column prop="device_id" label="设备 ID" min-width="240" />
          <el-table-column prop="accounts" label="账号数" width="100">
            <template #default="{ row }">
              <el-tag :type="row.accounts >= 3 ? 'danger' : 'info'" size="small">{{ row.accounts }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="last_ip" label="最近 IP" width="160" />
          <el-table-column prop="last_at" label="最近注册" width="180" />
        </el-table>
      </el-tab-pane>

      <!-- API Key 告警 -->
      <el-tab-pane label="API Key 告警" name="alarm">
        <div class="toolbar">
          <el-select v-model="alarmKind" placeholder="全部类型" clearable style="width: 180px" @change="loadAlarms">
            <el-option label="多 IP 使用" value="multi_ip" />
            <el-option label="解密失败" value="decrypt_fail" />
            <el-option label="封禁后仍使用" value="banned_use" />
            <el-option label="指纹不匹配" value="hmac_mismatch" />
          </el-select>
          <el-select v-model="alarmAcked" placeholder="全部状态" clearable style="width: 140px" @change="loadAlarms">
            <el-option label="待处理" :value="false" />
            <el-option label="已处理" :value="true" />
          </el-select>
          <span class="hint">多 IP 只是提醒（移动网络切换会触发），是否处置由人决定</span>
        </div>
        <el-table :data="alarms" size="small" class="panel">
          <el-table-column prop="kind" label="类型" width="130" />
          <el-table-column prop="detail" label="详情" min-width="300" />
          <el-table-column label="Key / 用户" width="140">
            <template #default="{ row }">#{{ row.key_id }} / {{ row.user_id }}</template>
          </el-table-column>
          <el-table-column label="时间" width="170">
            <template #default="{ row }">{{ fmt(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag :type="row.acked ? 'info' : 'warning'" size="small">{{ row.acked ? '已处理' : '待处理' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="90">
            <template #default="{ row }">
              <el-button v-if="!row.acked" text size="small" @click="ack(row)">标记处理</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { gateApi, type GateConfig, type Invite, type DeviceRow, type KeyAlarm } from '@/api/gate';

const tab = ref('config');
const cfg = reactive<GateConfig>({
  enabled: false, require_invite: false, max_accounts_per_device: 3,
  block_honeypot_ip: true, block_disposable_email: false,
  post_min_age_hours: 0, comment_min_age_hours: 0,
  new_user_daily_posts: 5, new_user_days: 3,
});
const saving = ref(false);

const invites = ref<Invite[]>([]);
const creating = ref(false);
const newInvite = reactive({ count: 1, max_uses: 1, note: '', ttl_hours: 0 });

const devices = ref<DeviceRow[]>([]);
const deviceKeyword = ref('');
const deviceNote = ref('');

const alarms = ref<KeyAlarm[]>([]);
const alarmKind = ref('');
const alarmAcked = ref<boolean | undefined>(undefined);

function fmt(s: string) {
  return s ? new Date(s).toLocaleString('zh-CN') : '';
}
async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text);
    ElMessage.success('已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择');
  }
}

async function loadCfg() {
  const res = await gateApi.adminConfig();
  Object.assign(cfg, res.data.data.config);
}
async function saveCfg() {
  saving.value = true;
  try {
    await gateApi.saveConfig({ ...cfg });
    ElMessage.success('已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}
async function loadInvites() {
  const res = await gateApi.invites();
  invites.value = res.data.data.list || [];
}
async function createInvites() {
  creating.value = true;
  try {
    const res = await gateApi.createInvites({ ...newInvite });
    const codes = res.data.data?.codes || [];
    ElMessage.success(`已生成 ${codes.length} 个`);
    await loadInvites();
    if (codes.length) {
      ElMessageBox.alert(codes.join('\n'), '新邀请码', { confirmButtonText: '好' });
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成失败');
  } finally {
    creating.value = false;
  }
}
async function revoke(row: Invite) {
  await ElMessageBox.confirm(`作废 ${row.code}？已使用的次数保留。`, '确认', { type: 'warning' });
  await gateApi.revokeInvite(row.id);
  ElMessage.success('已作废');
  await loadInvites();
}
async function loadDevices() {
  const res = await gateApi.devices(deviceKeyword.value);
  devices.value = res.data.data.list || [];
  deviceNote.value = res.data.data.note || '';
}
async function loadAlarms() {
  const res = await gateApi.alarms(1, 50, alarmKind.value, alarmAcked.value);
  alarms.value = res.data.data.list || [];
}
async function ack(row: KeyAlarm) {
  const { value } = await ElMessageBox.prompt('处理说明（写清是本人多设备还是需要封禁）', '标记处理', {
    inputPlaceholder: '如：本人多设备登录，已确认',
  });
  await gateApi.ackAlarm(row.id, value || '');
  ElMessage.success('已标记');
  await loadAlarms();
}

onMounted(async () => {
  await loadCfg();
  await loadInvites();
  await loadDevices();
  await loadAlarms();
});
</script>

<style scoped>
.admin-page { padding: 16px; }
.head { display: flex; align-items: baseline; gap: 12px; margin-bottom: 8px; }
.head h2 { margin: 0; }

.sub { color: var(--text-muted); font-size: 0.82rem; }
.panel { max-width: 980px; margin-top: 12px; }
.mb { margin-bottom: 12px; }
.toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; max-width: 1100px; }
.hint { color: var(--text-muted); font-size: 0.78rem; }
.ml { margin-left: 8px; }
.code { font-family: ui-monospace, monospace; background: var(--bg-muted); padding: 1px 6px; border-radius: 4px; }
</style>
