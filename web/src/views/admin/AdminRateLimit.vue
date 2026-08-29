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
          配额按身份从高到低共四档：<b>携带 API Key</b> ≫ <b>匿名（无 Key）</b> &gt; <b>爬虫（识别命中）</b> &gt; <b>已知名头爬虫（直接 403）</b>。
          无 Key 的匿名访问只给低配额，被识别为爬虫后给更低的配额 —— 爬虫脚本想提速，唯一途径是申请并在请求中携带 API Key
          （从而可审计、可吊销）。用户创建自己的 Key 时，可在对应角色上限之内自设更小配额。
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
          <h3 class="panel-title">已知名头爬虫：直接阻断（403）</h3>
          <p class="hint-text">
            UA 命中下方名单的请求<b>直接返回 403</b>，不消耗任何配额、不进入限流计数。
            与「爬虫识别规则」的区别：识别规则只是把请求压到低配额（仍可低速访问），阻断名单是彻底拒绝。
          </p>
          <el-form-item label="自定义阻断关键词">
            <el-input
              type="textarea"
              :rows="3"
              v-model="botBlockKeywordsText"
              placeholder="每行一个关键词（小写子串匹配），如：yandexbot、some-ai-crawler"
            />
            <div class="form-tip">
              与内置阻断名单取并集，保存后立即生效。若需阻断某些搜索引擎爬虫，在此填写其 UA 特征词即可。
            </div>
          </el-form-item>
          <el-form-item label="内置阻断名单">
            <div class="builtin-tags">
              <el-tag v-for="kw in builtinBlockKeywords" :key="kw" size="small" type="danger" effect="plain">{{ kw }}</el-tag>
            </div>
            <div class="form-tip">
              内置覆盖 AI 训练/生成爬虫（GPTBot、MetaWebIndexer、ClaudeBot、Bytespider、CCBot、PerplexityBot、AmazonBot、Google-Extended、PetalBot 等）
              与商业 SEO 采集器（MJ12bot、AhrefsBot、SemrushBot、DotBot 等）。
              <b>主流搜索引擎（Googlebot / Bingbot / Baiduspider）默认不在阻断名单内</b>，以免影响收录 —— 需要阻断请在上方的自定义栏追加。
            </div>
          </el-form-item>

          <el-divider />
          <h3 class="panel-title">爬虫识别规则（低配额，不阻断）</h3>
          <p class="hint-text">命中的请求仍可低速访问，只是配额被压到「匿名」之下。适合处理无头浏览器、脚本库这类不确定身份的客户端。</p>
          <el-form-item label="自定义爬虫 UA 关键词">
            <el-input
              type="textarea"
              :rows="3"
              v-model="botKeywordsText"
              placeholder="每行一个关键词（小写子串匹配），如：my-bot、spider/1.0"
            />
            <div class="form-tip">与内置规则取并集。内置覆盖：HeadlessChrome、Playwright、python-requests、curl、wget、Scrapy、okhttp 等无头浏览器与脚本库。</div>
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
          <el-table-column label="配额" width="130">
            <template #default="{ row }">
              <span>{{ row.effective_req ?? row.quota_per_min }} 次/分</span>
              <el-tooltip v-if="row.user_quota > 0" content="管理员为该用户设置了专属配额（取更严格值）" placement="top">
                <el-tag size="small" type="warning" effect="plain" style="margin-left: 4px">限流</el-tag>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="170">
            <template #default="{ row }">
              <el-tag v-if="row.is_banned || row.user_banned" type="danger" size="small">
                {{ row.user_banned ? '用户已封禁' : 'Key 已封禁' }}
              </el-tag>
              <el-tag v-else :type="row.is_active ? 'success' : 'info'" size="small">{{ row.is_active ? '启用' : '停用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="最近使用" width="160">
            <template #default="{ row }">{{ formatTime(row.last_used_at) }}</template>
          </el-table-column>
          <el-table-column label="调用次数" width="90">
            <template #default="{ row }">{{ row.total_calls ?? 0 }}</template>
          </el-table-column>
          <el-table-column label="操作" width="250" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text @click="toggleKey(row)">{{ row.is_active ? '停用' : '启用' }}</el-button>
              <el-button
                size="small" text
                :type="row.is_banned ? 'success' : 'danger'"
                @click="row.is_banned ? unbanKey(row) : banKey(row)"
              >{{ row.is_banned ? '解封' : '封禁' }}</el-button>
              <el-button size="small" text type="warning" @click="regenerateKey(row)">重生成</el-button>
              <el-button size="small" text type="danger" @click="deleteKey(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 按人管控：封禁某人全部 Key + 专属限流配额 -->
      <div class="admin-panel">
        <h3 class="panel-title">按人管控：封禁 / 专属限流</h3>
        <p class="hint-text">
          针对具体用户下刀，优先级高于角色全局配额：
          <b>封禁</b>后该用户名下所有 Key（含以后新建的）立即 403；
          <b>专属配额</b>按 min(角色上限, 专属值) 生效，可把某人单独压到很低的频率而不影响同角色其他人。
        </p>
        <el-form label-width="120px" label-position="left" style="max-width: 760px">
          <el-form-item label="目标用户">
            <div style="display: flex; gap: 8px; width: 100%">
              <el-input v-model="targetUserKey" placeholder="用户名 或 数字 ID（如 zhangsan / 12）" style="flex: 1" />
              <el-button :loading="resolvingUser" @click="resolveTargetUser">载入</el-button>
            </div>
            <div class="form-tip">载入后可查看该用户当前的专属策略，并设置配额 / 封禁。</div>
          </el-form-item>
          <template v-if="targetUser">
            <el-form-item label="已选用户">
              <el-tag type="primary" size="small">#{{ targetUser.id }} {{ targetUser.username }}</el-tag>
              <span class="quota-unit" style="margin-left: 8px">{{ targetUser.nickname || '' }}</span>
            </el-form-item>
            <el-form-item label="专属配额">
              <div class="quota-row">
                <el-input-number v-model="policyForm.quota_per_min" :min="0" :max="1000000" :step="10" size="small" />
                <span class="quota-unit">次 /</span>
                <el-input-number v-model="policyForm.window_secs" :min="0" :max="86400" :step="10" size="small" />
                <span class="quota-unit">秒</span>
              </div>
              <div class="form-tip">均为 0 表示跟随角色全局（配额取角色上限，窗口取角色窗口）。</div>
            </el-form-item>
            <el-form-item label="Key 数上限">
              <el-input-number v-model="policyForm.max_keys" :min="0" :max="1000" size="small" />
              <div class="form-tip">0 = 不限制。达到上限后该用户无法再新建 Key（需先删除旧的）。</div>
            </el-form-item>
            <el-form-item label="备注">
              <el-input v-model="policyForm.note" placeholder="选填：仅供管理员内部记录，不对外展示" maxlength="120" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="savingPolicy" @click="savePolicy">保存专属策略</el-button>
              <el-button @click="revokePolicy">撤销（恢复全局）</el-button>
              <el-button
                v-if="currentPolicy?.keys_banned"
                type="success"
                :loading="banningUser"
                @click="unbanUserKeys"
              >解除该用户的 API 封禁</el-button>
              <el-button v-else type="danger" :loading="banningUser" @click="banUserKeys">封禁该用户全部 Key</el-button>
            </el-form-item>
          </template>
        </el-form>

        <el-divider />
        <h3 class="panel-title">已单独设置 / 封禁的用户</h3>
        <el-table :data="userPolicies" v-loading="loadingPolicies" size="small" style="width: 100%">
          <el-table-column prop="user_id" label="用户ID" width="80" />
          <el-table-column prop="username" label="用户名" width="130" />
          <el-table-column label="封禁" width="120">
            <template #default="{ row }">
              <el-tag v-if="row.keys_banned" type="danger" size="small">
                {{ row.ban_until ? '至 ' + formatTime(row.ban_until) : '永久' }}
              </el-tag>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column label="专属配额" width="150">
            <template #default="{ row }">
              <span v-if="row.quota_per_min > 0">{{ row.quota_per_min }} 次/{{ row.window_secs || 60 }}秒</span>
              <span v-else class="muted">跟随全局</span>
            </template>
          </el-table-column>
          <el-table-column label="Key 上限" width="90">
            <template #default="{ row }">{{ row.max_keys > 0 ? row.max_keys : '不限' }}</template>
          </el-table-column>
          <el-table-column prop="ban_reason" label="封禁原因" min-width="140">
            <template #default="{ row }">{{ row.ban_reason || row.note || '—' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="180" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text @click="editPolicy(row)">编辑</el-button>
              <el-button v-if="row.keys_banned" size="small" text type="success" @click="unbanUserKeysDirect(row)">解封</el-button>
              <el-button size="small" text type="danger" @click="revokePolicyDirect(row)">撤销</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 重生成后一次性展示完整 Key -->
      <el-dialog v-model="showNewKey" title="API Key 已重生成" width="540px">
        <el-alert type="warning" :closable="false" show-icon title="完整 Key 仅此一次展示，请立即复制保存" style="margin-bottom: 12px" />
        <div class="newkey-box">
          <code class="newkey-code">{{ newKeyValue }}</code>
          <el-button type="primary" @click="copyKey">复制</el-button>
        </div>
        <p class="hint-text">
          请求时携带 <code>X-API-Key: {{ newKeyPrefix }}…</code>（完整 Key）。服务端仅存储哈希，遗失后只能重新生成。
        </p>
      </el-dialog>

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
            <el-option label="已阻断" value="blocked" />
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
  { key: 'anonymous', label: '匿名（无 Key）', tip: '普通访客/搜索引擎，按 IP 限流。无 Key 一律低配额' },
  { key: 'bot', label: '爬虫（识别命中）', tip: '无头/伪装/高频爬虫，配额比匿名更低 —— 想提速就必须改用 API Key' },
];

const form = reactive<Record<string, any>>({});
const botKeywordsText = ref('');
const botBlockKeywordsText = ref('');
const builtinBlockKeywords = ref<string[]>([]);
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
    botBlockKeywordsText.value = (cfg.bot_block_keywords || []).join('\n');
    builtinBlockKeywords.value = res.data.data?.builtin_block_keywords || [];
  } catch { /* ignore */ }
}

async function saveConfig() {
  savingConfig.value = true;
  try {
    const payload: Record<string, any> = {
      admin: form.admin, author: form.author, reader: form.reader,
      federated: form.federated, anonymous: form.anonymous, bot: form.bot,
      bot_ua_keywords: botKeywordsText.value.split('\n').map(s => s.trim()).filter(Boolean),
      bot_block_keywords: botBlockKeywordsText.value.split('\n').map(s => s.trim()).filter(Boolean),
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

// 管理动作：启停 / 重生成 / 删除
const showNewKey = ref(false);
const newKeyValue = ref('');
const newKeyPrefix = ref('');

async function toggleKey(row: any) {
  try {
    const res = await adminApi.updateApiKey(row.id, { is_active: !row.is_active });
    if (res.data.code === 0) ElMessage.success('已更新状态');
    else ElMessage.error(res.data.message || '操作失败');
    await loadKeys();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function regenerateKey(row: any) {
  try {
    await ElMessageBox.confirm(`重新生成「${row.key_prefix}」后旧 Key 立即失效，确定继续？`, '重新生成', { type: 'warning' });
    const res = await adminApi.regenerateApiKey(row.id);
    if (res.data.code === 0) {
      newKeyValue.value = res.data.data.key || '';
      newKeyPrefix.value = res.data.data.key_prefix || '';
      showNewKey.value = true;
      ElMessage.success('已重新生成');
      await loadKeys();
    } else {
      ElMessage.error(res.data.message || '重生成失败');
    }
  } catch { /* cancelled */ }
}

async function deleteKey(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除 API Key「${row.key_prefix}」（用户 ${row.username || '?'}）吗？删除后立即失效。`, '删除 API Key', { type: 'warning' });
    const res = await adminApi.deleteApiKey(row.id);
    if (res.data.code === 0) ElMessage.success('已删除');
    else ElMessage.error(res.data.message || '删除失败');
    await loadKeys();
  } catch { /* cancelled */ }
}

async function copyKey() {
  try {
    await navigator.clipboard.writeText(newKeyValue.value);
    ElMessage.success('已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择复制');
  }
}

// ====== 按把封禁 / 解封 ======
async function banKey(row: any) {
  try {
    const { value } = await ElMessageBox.prompt(
      `封禁 Key「${row.key_prefix}」（用户 ${row.username || row.user_id}）。封禁后该 Key 的请求直接返回 403。`,
      '封禁 API Key',
      { confirmButtonText: '确认封禁', cancelButtonText: '取消', inputPlaceholder: '封禁原因（将展示给调用方）', inputValue: '违规调用' }
    );
    const res = await adminApi.banApiKey(row.id, value || '管理员封禁');
    if (res.data.code === 0) ElMessage.success('已封禁');
    else ElMessage.error(res.data.message || '操作失败');
    await loadKeys();
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function unbanKey(row: any) {
  try {
    await ElMessageBox.confirm(`确定解封 Key「${row.key_prefix}」？`, '解封 API Key', { type: 'warning' });
    const res = await adminApi.unbanApiKey(row.id);
    if (res.data.code === 0) ElMessage.success('已解封');
    else ElMessage.error(res.data.message || '操作失败');
    await loadKeys();
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

// ====== 按人管控：封禁全部 Key + 专属限流 ======
const targetUserKey = ref('');
const targetUser = ref<any>(null);
const currentPolicy = ref<any>(null);
const resolvingUser = ref(false);
const savingPolicy = ref(false);
const banningUser = ref(false);
const loadingPolicies = ref(false);
const userPolicies = ref<any[]>([]);
const policyForm = reactive({ quota_per_min: 0, window_secs: 0, max_keys: 0, note: '' });

async function resolveTargetUser() {
  const key = targetUserKey.value.trim();
  if (!key) { ElMessage.warning('请输入用户名或用户 ID'); return; }
  resolvingUser.value = true;
  try {
    const res = await adminApi.resolveUser(key);
    const u = res.data.data;
    if (!u?.id) { ElMessage.error('未找到该用户'); return; }
    targetUser.value = u;
    const pres = await adminApi.getUserApiQuota(u.id);
    const p = pres.data.data || {};
    currentPolicy.value = p;
    policyForm.quota_per_min = p.quota_per_min || 0;
    policyForm.window_secs = p.window_secs || 0;
    policyForm.max_keys = p.max_keys || 0;
    policyForm.note = p.note || '';
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '未找到该用户');
    targetUser.value = null;
  }
  resolvingUser.value = false;
}

async function savePolicy() {
  if (!targetUser.value) return;
  savingPolicy.value = true;
  try {
    const res = await adminApi.setUserApiQuota(targetUser.value.id, {
      quota_per_min: policyForm.quota_per_min,
      window_secs: policyForm.window_secs,
      max_keys: policyForm.max_keys,
      note: policyForm.note,
    });
    if (res.data.code === 0) ElMessage.success('已保存（立即生效）');
    else ElMessage.error(res.data.message || '保存失败');
    await resolveTargetUser();
    await loadPolicies();
    await loadKeys();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  savingPolicy.value = false;
}

async function revokePolicy() {
  if (!targetUser.value) return;
  try {
    await ElMessageBox.confirm(`撤销「${targetUser.value.username}」的专属设置？将恢复为跟随角色全局配额。`, '撤销', { type: 'warning' });
    await adminApi.deleteUserApiQuota(targetUser.value.id);
    ElMessage.success('已撤销');
    await resolveTargetUser();
    await loadPolicies();
    await loadKeys();
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function banUserKeys() {
  if (!targetUser.value) return;
  try {
    const { value } = await ElMessageBox.prompt(
      `封禁「${targetUser.value.username}」名下的全部 API Key（含以后新建的）。请输入封禁时长（小时，0 = 永久）：`,
      '封禁用户 API 访问',
      { confirmButtonText: '确认封禁', cancelButtonText: '取消', inputValue: '0', inputPattern: /^\d+$/, inputErrorMessage: '请输入数字（小时）' }
    );
    banningUser.value = true;
    const res = await adminApi.banUserApiKeys(targetUser.value.id, {
      duration_hours: parseInt(value || '0', 10),
      reason: '管理员封禁',
    });
    if (res.data.code === 0) ElMessage.success('已封禁该用户的全部 API Key');
    else ElMessage.error(res.data.message || '操作失败');
    await resolveTargetUser();
    await loadPolicies();
    await loadKeys();
  } catch (e: any) {
    if (e !== 'cancel' && e !== null) ElMessage.error(e?.response?.data?.message || '操作失败');
  }
  banningUser.value = false;
}

async function unbanUserKeys() {
  if (!targetUser.value) return;
  try {
    await ElMessageBox.confirm(`确定解除「${targetUser.value.username}」的 API 封禁？`, '解封', { type: 'warning' });
    await adminApi.unbanUserApiKeys(targetUser.value.id);
    ElMessage.success('已解封');
    await resolveTargetUser();
    await loadPolicies();
    await loadKeys();
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function loadPolicies() {
  loadingPolicies.value = true;
  try { const res = await adminApi.listUserApiPolicies(); userPolicies.value = res.data.data || []; } catch { /* ignore */ }
  loadingPolicies.value = false;
}

async function editPolicy(row: any) {
  targetUserKey.value = String(row.username || row.user_id);
  await resolveTargetUser();
}

async function unbanUserKeysDirect(row: any) {
  try {
    await adminApi.unbanUserApiKeys(row.user_id);
    ElMessage.success('已解封');
    await loadPolicies();
    await loadKeys();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function revokePolicyDirect(row: any) {
  try {
    await ElMessageBox.confirm(`撤销「${row.username || row.user_id}」的专属设置？`, '撤销', { type: 'warning' });
    await adminApi.deleteUserApiQuota(row.user_id);
    ElMessage.success('已撤销');
    await loadPolicies();
    await loadKeys();
  } catch { /* cancelled */ }
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
  return ({ admin: '站长', author: '作者', reader: '读者', federated: '联邦', anonymous: '匿名', bot: '爬虫', blocked: '已阻断' } as Record<string, string>)[role] || role;
}
function roleTag(role: string) {
  return ({ admin: 'danger', author: 'warning', reader: 'primary', federated: 'success', anonymous: 'info', bot: 'warning', blocked: 'danger' } as Record<string, string>)[role] || 'info';
}
function logTypeTag(t: string) {
  return t === 'blocked' ? 'danger' : t === 'bot' ? 'warning' : t === 'anonymous' ? 'info' : 'primary';
}

onMounted(() => { loadConfig(); loadStats(); loadKeys(); loadLogs(1); loadPolicies(); });
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
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
.builtin-tags { display: flex; flex-wrap: wrap; gap: 6px; max-height: 140px; overflow-y: auto; width: 100%; }
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
.muted { color: var(--text-light, #999); }
.newkey-box { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.newkey-code { flex: 1; background: #0f172a; color: #4ade80; padding: 10px 12px; border-radius: 8px; font-size: 0.82rem; word-break: break-all; }
[data-theme="light"] .newkey-code { background: #f1f5f9; color: #15803d; }
</style>
