<template>
  <div class="admin-page">
    <AdminHeader title="限流与 API Key" subtitle="四层身份配额 · 爬虫识别 · 访问审计" :icon="Key" />
    <div class="admin-page-body">
      <!-- 当前限流状态 -->
      <div class="admin-panel stats-panel">
        <h3 class="panel-title">当前限流状态（内存）</h3>
        <el-row :gutter="12">
          <el-col :xs="12" :sm="8" :md="4" v-for="(v, k) in stats" :key="k">
            <div class="stat-mini">
              <div class="stat-mini-value">{{ v }}</div>
              <div class="stat-mini-label">{{ statLabels[k] || k }}</div>
            </div>
          </el-col>
        </el-row>
        <div style="margin-top: 12px; display: flex; gap: 8px; align-items: center">
          <el-button size="small" @click="loadStats" :loading="loadingStats">刷新</el-button>
          <el-button size="small" type="warning" @click="clearLimits">清空限流计数（误判解封）</el-button>
          <span class="hint-text" style="margin: 0">清空后所有 IP / Key 的滑动窗口计数归零，立即生效。</span>
        </div>
      </div>

      <!-- 配额配置 -->
      <div class="admin-panel">
        <h3 class="panel-title">各身份限流配额（请求数 / 时间窗口）</h3>
        <p class="hint-text">
          带有效 API Key 的请求按 Key 配额限流；无 Key 的请求按 IP 走匿名配额，被识别为爬虫后走更低配额。
          用户创建自己的 Key 时，可在对应角色上限之内自设更小配额。
        </p>
        <el-form label-width="150px" label-position="left" style="max-width: 900px">
          <el-row :gutter="16">
            <el-col :span="12" v-for="role in roleDefs" :key="role.key">
              <el-form-item :label="role.label">
                <div class="quota-row">
                  <el-input-number
                    :model-value="form[role.key]?.req"
                    :min="1"
                    :max="1000000"
                    :step="10"
                    size="small"
                    @change="v => setReq(role.key, v)"
                  />
                  <span class="quota-unit">次 /</span>
                  <el-input-number
                    :model-value="form[role.key]?.window"
                    :min="1"
                    :max="86400"
                    :step="10"
                    size="small"
                    @change="v => setWindow(role.key, v)"
                  />
                  <span class="quota-unit">秒</span>
                </div>
                <div class="form-tip">{{ role.tip }}</div>
              </el-form-item>
            </el-col>
          </el-row>

          <el-divider />
          <h3 class="panel-title">爬虫识别规则</h3>
          <el-form-item label="自定义爬虫 UA 关键词">
            <el-input
              type="textarea"
              :rows="3"
              v-model="botKeywordsText"
              placeholder="每行一个关键词（小写子串匹配），如：my-bot、spider/1.0"
            />
            <div class="form-tip">与内置规则取并集。内置覆盖：HeadlessChrome、python-requests、curl、wget、GPTBot、MetaWebIndexer、anthropic-ai、Bytespider 等。</div>
          </el-form-item>
          <el-form-item label="伪装头检测">
            <el-switch v-model="form.bot_spoof_check" />
            <div class="form-tip">UA 声称是浏览器（Chrome/Firefox/Safari 等）但请求完全缺少 Sec-Fetch-* 头 → 判为伪装爬虫，走爬虫配额。</div>
          </el-form-item>
          <el-form-item label="单 IP 高频自动降级">
            <el-input-number v-model="form.bot_ip_threshold" :min="0" :max="100000" :step="10" size="small" />
            <span class="quota-unit">次 /</span>
            <el-input-number v-model="form.bot_ip_ttl_min" :min="1" :max="1440" size="small" />
            <span class="quota-unit">分钟</span>
            <div class="form-tip">同一 IP 在窗口内请求数超过阈值即自动降级为爬虫配额（0 = 关闭行为降级）。</div>
          </el-form-item>
          <el-form-item label="访问日志保留天数">
            <el-input-number v-model="form.log_retention_days" :min="1" :max="90" size="small" />
            <div class="form-tip">api_access_logs 仅记录携带 Key / 触发限流 / 爬虫命中的请求（每分钟每维度去重），到期自动清理。</div>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="savingConfig" @click="saveConfig">保存并立即生效</el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 全站 API Key 一览 -->
      <div class="admin-panel">
        <h3 class="panel-title">全站 API Key 一览</h3>
        <p class="hint-text">所有用户创建的用户级 Key（站长 / 作者 / 读者）。联邦 Key 请在「远程站点互通」中管理。</p>
        <el-table :data="allKeys" v-loading="loadingKeys" size="small" style="width: 100%">
          <el-table-column prop="username" label="用户名" width="110" />
          <el-table-column prop="name" label="备注" min-width="100">
            <template #default="{ row }">{{ row.name || '—' }}</template>
          </el-table-column>
          <el-table-column label="身份" width="80">
            <template #default="{ row }">
              <el-tag :type="roleTag(row.role)" size="small">{{ roleLabel(row.role) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="key_prefix" label="Key 前缀" width="170">
            <template #default="{ row }"><code>{{ row.key_prefix }}</code></template>
          </el-table-column>
          <el-table-column label="配额" width="110">
            <template #default="{ row }">{{ row.quota_per_min }} 次/分</template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{ row }">
              <el-tag :type="row.is_active ? 'success' : 'danger'" size="small">{{ row.is_active ? '启用' : '停用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="最近使用" width="160">
            <template #default="{ row }">{{ formatTime(row.last_used_at) }}</template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 访问日志 -->
      <div class="admin-panel">
        <div class="panel-head">
          <h3 class="panel-title">访问日志（限流 / 携带 Key / 爬虫）</h3>
          <el-select v-model="logType" size="small" style="width: 150px" @change="loadLogs(1)">
            <el-option label="全部类型" value="" />
            <el-option label="联邦" value="federated" />
            <el-option label="站长" value="admin" />
            <el-option label="作者" value="author" />
            <el-option label="读者" value="reader" />
            <el-option label="匿名" value="anonymous" />
            <el-option label="爬虫" value="bot" />
          </el-select>
        </div>
        <el-table :data="logs" v-loading="loadingLogs" size="small" style="width: 100%">
          <el-table-column prop="created_at" label="时间" width="160">
            <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="身份" width="80">
            <template #default="{ row }">
              <el-tag :type="logTypeTag(row.identity_type)" size="small">{{ roleLabel(row.identity_type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="key_prefix" label="Key 前缀" width="150">
            <template #default="{ row }"><code>{{ row.key_prefix || '—' }}</code></template>
          </el-table-column>
          <el-table-column label="请求" min-width="180">
            <template #default="{ row }">
              <span class="http-method" :class="'m-' + row.method.toLowerCase()">{{ row.method }}</span>
              <span class="http-path">{{ row.path }}</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="70">
            <template #default="{ row }">
              <span :class="row.status_code >= 400 ? 'status-err' : 'status-ok'">{{ row.status_code || 200 }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="request_ip" label="IP" width="120" />
          <el-table-column prop="bot_reason" label="命中原因" min-width="140">
            <template #default="{ row }">{{ row.bot_reason || '—' }}</template>
          </el-table-column>
        </el-table>
        <div class="pager">
          <el-pagination
            layout="prev, pager, next, total"
            :total="logTotal"
            :page-size="logPageSize"
            :current-page="logPage"
            @current-change="loadLogs"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { Key } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';

const statLabels: Record<string, string> = {
  total: '限流条目总数', by_key: '按 Key', by_bot: '爬虫通道',
  by_anon: '匿名通道', by_ip: '按 IP',
};
const stats = ref<Record<string, number>>({});
const loadingStats = ref(false);

const roleDefs = [
  { key: 'admin', label: '站长 Key', tip: '管理后台操作 / 系统配置，极高配额（建议仅限本机或指定 IP）' },
  { key: 'author', label: '作者 Key', tip: '创作、上传、导入导出、批量操作，可审计' },
  { key: 'reader', label: '读者 Key', tip: '书架、评论、阅读进度同步，第三方阅读器/客户端' },
  { key: 'federated', label: '联邦 Key', tip: '跨站作品同步、站点信息互换，双向可见' },
  { key: 'anonymous', label: '匿名（无 Key）', tip: '普通访客/搜索引擎，按 IP 限流' },
  { key: 'bot', label: '爬虫（识别命中）', tip: '无头/伪装/高频爬虫，远低于匿名配额' },
];

const form = reactive<Record<string, any>>({});
const botKeywordsText = ref('');
const savingConfig = ref(false);

function setReq(role: string, v: number | undefined) {
  if (!form[role]) form[role] = {};
  form[role].req = v || 1;
}
function setWindow(role: string, v: number | undefined) {
  if (!form[role]) form[role] = {};
  form[role].window = v || 1;
}

async function loadConfig() {
  try {
    const res = await adminApi.getRateLimitConfig();
    const cfg = res.data.data?.config || {};
    for (const r of roleDefs) {
      form[r.key] = {
        req: cfg[r.key]?.req ?? 60,
        window: cfg[r.key]?.window ?? 60,
      };
    }
    form.bot_spoof_check = cfg.bot_spoof_check !== false;
    form.bot_ip_threshold = cfg.bot_ip_threshold ?? 300;
    form.bot_ip_ttl_min = cfg.bot_ip_ttl_min ?? 1;
    form.log_retention_days = cfg.log_retention_days ?? 7;
    botKeywordsText.value = (cfg.bot_ua_keywords || []).join('\n');
  } catch { /* ignore */ }
}

async function saveConfig() {
  savingConfig.value = true;
  try {
    const payload: Record<string, any> = {
      admin: form.admin, author: form.author, reader: form.reader,
      federated: form.federated, anonymous: form.anonymous, bot: form.bot,
      bot_ua_keywords: botKeywordsText.value.split('\n').map(s => s.trim()).filter(Boolean),
      bot_spoof_check: form.bot_spoof_check,
      bot_ip_threshold: form.bot_ip_threshold,
      bot_ip_ttl_min: form.bot_ip_ttl_min,
      log_retention_days: form.log_retention_days,
    };
    const res = await adminApi.updateRateLimitConfig(payload);
    if (res.data.code === 0) ElMessage.success(res.data.message || '已保存');
    else ElMessage.error(res.data.message || '保存失败');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingConfig.value = false;
}

async function loadStats() {
  loadingStats.value = true;
  try { const res = await adminApi.getRateLimitStats(); stats.value = res.data.data || {}; } catch { /* ignore */ }
  loadingStats.value = false;
}

async function clearLimits() {
  try {
    await ElMessageBox.confirm('确定清空全部内存限流计数吗？已触达配额的 IP/Key 将立即恢复访问。', '清空限流', { type: 'warning' });
    const res = await adminApi.clearRateLimits();
    ElMessage.success(res.data.message || '已清空');
    await loadStats();
  } catch { /* cancelled */ }
}

// 全站 Key
const allKeys = ref<any[]>([]);
const loadingKeys = ref(false);
async function loadKeys() {
  loadingKeys.value = true;
  try { const res = await adminApi.getAllApiKeys(); allKeys.value = res.data.data || []; } catch { /* ignore */ }
  loadingKeys.value = false;
}

// 日志
const logs = ref<any[]>([]);
const logType = ref('');
const logPage = ref(1);
const logPageSize = 20;
const logTotal = ref(0);
const loadingLogs = ref(false);

async function loadLogs(page = 1) {
  logPage.value = page;
  loadingLogs.value = true;
  try {
    const res = await adminApi.getRateLimitLogs({ type: logType.value, page, page_size: logPageSize });
    logs.value = res.data.data?.list || [];
    logTotal.value = res.data.data?.total || 0;
  } catch { /* ignore */ }
  loadingLogs.value = false;
}

function formatTime(s?: string) {
  if (!s) return '—';
  return new Date(s).toLocaleString('zh-CN');
}
function roleLabel(role: string) {
  return ({ admin: '站长', author: '作者', reader: '读者', federated: '联邦', anonymous: '匿名', bot: '爬虫' } as Record<string, string>)[role] || role;
}
function roleTag(role: string) {
  return ({ admin: 'danger', author: 'warning', reader: 'primary', federated: 'success', anonymous: 'info', bot: 'danger' } as Record<string, string>)[role] || 'info';
}
function logTypeTag(t: string) {
  return t === 'bot' ? 'danger' : t === 'anonymous' ? 'info' : 'primary';
}

onMounted(() => { loadConfig(); loadStats(); loadKeys(); loadLogs(1); });
</script>

<style scoped>
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: 1100px; }
.admin-panel { background: #fff; border-radius: 10px; padding: 20px; margin-bottom: 20px; border: 1px solid var(--border-color, #e5e7eb); }
[data-theme="dark"] .admin-panel { background: #1e293b; border-color: #334155; }
.panel-title { margin: 0 0 12px; font-size: 1.05rem; color: var(--text-color, #333); }
[data-theme="dark"] .panel-title { color: #e2e8f0; }
.panel-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.panel-head .panel-title { margin: 0; }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; }
.form-tip { width: 100%; font-size: 0.78rem; color: var(--text-light, #999); line-height: 1.5; }
.quota-row { display: flex; align-items: center; gap: 6px; }
.quota-unit { color: var(--text-light, #666); font-size: 0.85rem; }
.stats-panel .el-col { margin-bottom: 10px; }
.stat-mini { background: var(--bg-page, #f5f7fa); border-radius: 8px; padding: 10px; text-align: center; }
[data-theme="dark"] .stat-mini { background: #0f172a; }
.stat-mini-value { font-size: 1.4rem; font-weight: 700; color: var(--gradient-primary, #4f46e5); }
.stat-mini-label { font-size: 0.75rem; color: var(--text-light, #666); margin-top: 2px; }
.http-method { display: inline-block; font-size: 0.72rem; font-weight: 600; padding: 1px 6px; border-radius: 4px; margin-right: 6px; color: #fff; }
.m-get { background: #10b981; }
.m-post { background: #3b82f6; }
.m-put { background: #f59e0b; }
.m-delete { background: #ef4444; }
.http-path { font-size: 0.82rem; color: var(--text-color, #333); word-break: break-all; }
[data-theme="dark"] .http-path { color: #e2e8f0; }
.status-ok { color: #10b981; font-weight: 600; }
.status-err { color: #ef4444; font-weight: 600; }
code { background: #eef2ff; color: #4f46e5; padding: 1px 6px; border-radius: 4px; font-size: 0.82rem; }
.pager { margin-top: 12px; display: flex; justify-content: flex-end; }
</style>
