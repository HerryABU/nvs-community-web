<template>
  <div class="hp-page" v-loading="loading">
    <!-- 接口异常常驻提示：本页没有 AdminHeader，靠这一行拿到全局错误反馈 -->
    <AdminApiAlert />
    <div class="hp-head">
      <h2>蜜罐与日志记录</h2>
      <span class="hp-sub">等级由站长控制；日志记录字段可勾选——关掉的字段不落库。</span>
    </div>

    <!-- 概览 -->
    <div class="hp-cards">
      <div class="hp-card"><div class="hp-num">{{ overview?.session_count ?? 0 }}</div><div class="hp-label">攻击会话</div></div>
      <div class="hp-card"><div class="hp-num">{{ overview?.event_count ?? 0 }}</div><div class="hp-label">命中事件</div></div>
      <div class="hp-card"><div class="hp-num">{{ overview?.high_risk ?? 0 }}</div><div class="hp-label">高风险会话</div></div>
      <div class="hp-card"><div class="hp-num">{{ overview?.last_24h ?? 0 }}</div><div class="hp-label">24h 事件</div></div>
      <div class="hp-card"><div class="hp-num">{{ overview?.banned_count ?? 0 }}</div><div class="hp-label">当前封禁 IP</div></div>
    </div>

    <el-card class="hp-section">
      <template #header><span>蜜罐等级</span></template>
      <el-form label-width="120px">
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
          <span class="hp-tip">关闭后所有诱饵路径与普通 404 无差别</span>
        </el-form-item>
        <el-form-item label="等级">
          <el-radio-group v-model="form.level" :disabled="!form.enabled">
            <el-radio-button value="low">低（仅 .env）</el-radio-button>
            <el-radio-button value="medium">中（+.git/后台/清单）</el-radio-button>
            <el-radio-button value="high">高（全量 + 延迟响应）</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="自动封禁">
          <el-switch v-model="form.auto_ban" />
          <el-input-number v-model="form.ban_minutes" :min="1" :max="10080" style="margin-left:10px" />
          <span class="hp-tip">分钟（进程内封禁，重启清空）</span>
        </el-form-item>
        <el-form-item label="命中延迟">
          <el-input-number v-model="form.delay_ms" :min="0" :max="2000" :step="100" />
          <span class="hp-tip">毫秒（高等级生效，上限 2s，拖住自动化）</span>
        </el-form-item>
        <el-form-item label="留存天数">
          <el-input-number v-model="form.retention_days" :min="1" :max="365" />
          <span class="hp-tip">超期事件可手动清理</span>
        </el-form-item>
        <el-form-item label="注释">
          <el-input v-model="form.annotation" type="textarea" :rows="2" placeholder="站长可灵活备注：蜜罐用途、投放策略、对接事项等" />
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="hp-section">
      <template #header><span>蜜罐密钥</span></template>
      <el-form label-width="120px">
        <el-form-item label="当前密钥">
          <code class="hp-key">{{ maskKey(overview?.key_b64) }}</code>
          <el-button size="small" style="margin-left:10px" :loading="regening" type="primary" @click="regenerate">重新生成</el-button>
          <span class="hp-tip">与真实站点密钥物理隔离；重新生成后旧诱饵 .env 暴露值即失效</span>
        </el-form-item>
        <el-form-item label="手动修改">
          <el-input v-model="manualKey" size="small" style="max-width:420px" placeholder="粘贴 base64 密钥（解码后须 ≥ 32 字节）；留空则点「应用」等价于重新生成" />
          <el-button size="small" style="margin-left:10px" :loading="regening" @click="applyManual">应用</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="hp-section">
      <template #header>
        <div class="hp-section-head">
          <span>自定义诱饵内容</span>
          <span class="hp-tip">JSON 数组；可覆盖内置诱饵（env/git/admin…）内容，或为额外路径（/.ssh/id_rsa、/config.yml 等）填内容</span>
        </div>
      </template>
      <el-input v-model="form.custom_decoys" type="textarea" :rows="8" placeholder='[{"id":"env","path":"/.env","content":"DB_PASSWORD=xxx","risk":70,"headers":{"Content-Type":"text/plain"}}]' />
      <div class="hp-tip">留空则使用内置默认内容；保存后立即生效（路径需为已注册诱饵路径之一）。</div>
    </el-card>

    <el-card class="hp-section">
      <template #header>
        <div class="hp-section-head">
          <span>日志记录字段</span>
          <span class="hp-tip">勾选后才写入日志（蜜罐事件与访问日志共用这套策略）</span>
        </div>
      </template>
      <el-form label-width="120px">
        <el-form-item label="记录项">
          <div class="hp-check-grid">
            <el-checkbox v-model="form.record_ua">User-Agent</el-checkbox>
            <el-checkbox v-model="form.record_referer">Referer</el-checkbox>
            <el-checkbox v-model="form.record_xff">X-Forwarded-For（代理链）</el-checkbox>
            <el-checkbox v-model="form.record_query">Query 参数</el-checkbox>
            <el-checkbox v-model="form.record_headers">请求头（白名单）</el-checkbox>
            <el-checkbox v-model="form.record_body_snippet">请求体摘要</el-checkbox>
          </div>
        </el-form-item>
        <el-form-item v-if="form.record_headers" label="请求头白名单">
          <el-input v-model="form.header_allowlist" :placeholder="defaultAllow" />
          <div class="hp-tip">
            逗号分隔；Cookie / Authorization / X-API-Key 永远不记录（强制过滤）
          </div>
        </el-form-item>
        <el-form-item v-if="form.record_body_snippet" label="摘要长度">
          <el-input-number v-model="form.body_snippet_limit" :min="64" :max="4096" :step="64" />
          <span class="hp-tip">字节；含 password/token/secret 等字样时整段省略</span>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">保存策略</el-button>
          <el-button :loading="loading" @click="load">刷新</el-button>
          <el-button type="danger" plain @click="cleanup">清理过期事件</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="hp-section">
      <template #header>
        <div class="hp-section-head">
          <span>攻击会话</span>
          <div class="hp-filters">
            <el-input v-model="keyword" size="small" placeholder="IP / HID / UA" clearable style="width: 200px" @change="loadSessions" />
            <el-select v-model="minRisk" size="small" style="width: 130px" @change="loadSessions">
              <el-option label="全部风险" :value="0" />
              <el-option label="风险 ≥ 40" :value="40" />
              <el-option label="风险 ≥ 60" :value="60" />
            </el-select>
            <el-button size="small" @click="doExportZip(0)">取证包（全部）</el-button>
          </div>
        </div>
      </template>
      <el-table :data="sessions" size="small" @row-click="openDetail">
        <el-table-column prop="src_ip" label="来源 IP" width="140" />
        <el-table-column prop="hid" label="会话" width="110" />
        <el-table-column prop="ua_norm" label="UA" min-width="200" show-overflow-tooltip />
        <el-table-column prop="event_count" label="事件" width="70" />
        <el-table-column label="风险" width="90">
          <template #default="{ row }">
            <el-tag :type="row.max_risk >= 60 ? 'danger' : row.max_risk >= 40 ? 'warning' : 'info'" size="small">
              {{ row.max_risk }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="封禁" width="80">
          <template #default="{ row }">
            <el-tag v-if="row.banned" size="small" type="danger">已封</el-tag>
            <span v-else class="hp-tip">—</span>
          </template>
        </el-table-column>
        <el-table-column label="最近" width="170">
          <template #default="{ row }">{{ fmt(row.last_seen) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click.stop="openDetail(row)">详情</el-button>
            <el-button size="small" text @click.stop="doExport(row.id)">导出 JSON</el-button>
            <el-button size="small" text @click.stop="doExportZip(row.id)">取证包</el-button>
            <el-button v-if="!row.banned" size="small" text type="danger" @click.stop="banIp(row.src_ip)">封禁</el-button>
          </template>
        </el-table-column>
        <template #empty><el-empty description="还没有命中记录" :image-size="60" /></template>
      </el-table>
    </el-card>

    <!-- SQL 注入蜜罐（gap-audit #3） -->
    <el-card class="hp-section">
      <template #header>
        <div class="hp-section-head">
          <span>SQL 注入蜜罐</span>
          <span class="hp-tip">真实链路全参数化，「注入打不穿」；这里是钓鱼式假入口，只记录 payload 与工具特征</span>
        </div>
      </template>
      <div class="hp-cards">
        <div class="hp-card"><div class="hp-num">{{ sqli?.hits ?? 0 }}</div><div class="hp-label">带注入特征</div></div>
        <div class="hp-card"><div class="hp-num">{{ sqli?.probes ?? 0 }}</div><div class="hp-label">仅探路</div></div>
      </div>
      <el-table :data="sqli?.breakdown || []" size="small">
        <el-table-column prop="decoy" label="假入口" width="180" />
        <el-table-column prop="kind" label="类型" width="140" />
        <el-table-column prop="count" label="次数" width="100" />
      </el-table>
    </el-card>

    <!-- TLS 指纹采集率（gap-audit #4） -->
    <el-card class="hp-section">
      <template #header>
        <div class="hp-section-head">
          <span>TLS 指纹（JA3/JA4）</span>
          <span class="hp-tip">JA3 只有终止 TLS 的那一层算得出来；本进程在反代后面时依赖反代透传</span>
        </div>
      </template>
      <div class="hp-cards">
        <div class="hp-card"><div class="hp-num">{{ tls?.fingerprints ?? 0 }}</div><div class="hp-label">指纹总数</div></div>
        <div class="hp-card"><div class="hp-num">{{ tls?.with_ja3 ?? 0 }}</div><div class="hp-label">含 JA3</div></div>
        <div class="hp-card"><div class="hp-num">{{ tls?.with_ja4 ?? 0 }}</div><div class="hp-label">含 JA4</div></div>
        <div class="hp-card"><div class="hp-num">{{ tls?.with_webrtc ?? 0 }}</div><div class="hp-label">含 WebRTC 内网 IP</div></div>
      </div>
      <el-alert :type="(tls?.ja3_ratio ?? 0) > 50 ? 'success' : 'warning'" :closable="false"
        :title="tls?.diagnosis || ''" />
      <div v-if="tls?.hint" class="hp-tls-hint">
        <div class="hp-fp-head">反代透传配置</div>
        <pre class="hp-pre">{{ tls.hint.nginx }}</pre>
        <pre class="hp-pre">{{ tls.hint.caddy }}</pre>
        <div class="hp-tip">{{ tls.hint.note }}</div>
      </div>
    </el-card>

    <!-- 规则反哺 -->
    <el-card class="hp-section">
      <template #header>
        <div class="hp-section-head">
          <span>规则反哺</span>
          <span class="hp-tip">把蜜罐命中里的工具特征写进爬虫阻断名单（即时生效，自动去重）</span>
          <div class="hp-filters">
            <el-button size="small" :loading="rulesLoading" @click="loadRules">重新提炼</el-button>
            <el-button size="small" type="primary" :disabled="selectedRules.length === 0" :loading="applying" @click="applyRules">
              反哺选中（{{ selectedRules.length }}）
            </el-button>
          </div>
        </div>
      </template>
      <el-table :data="rules" size="small" @selection-change="(rows: any[]) => (selectedRules = rows.map((r) => r.keyword))">
        <el-table-column type="selection" width="42" />
        <el-table-column prop="keyword" label="特征" width="180" />
        <el-table-column prop="kind" label="类型" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="row.kind === 'ua' ? 'warning' : 'info'">{{ row.kind }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="来源" min-width="180" />
        <el-table-column prop="hits" label="命中" width="80" />
        <template #empty><el-empty description="暂无候选（需要风险 ≥ 60 的命中事件）" :image-size="50" /></template>
      </el-table>
    </el-card>

    <!-- 会话详情 -->
    <el-drawer v-model="detailVisible" :title="`会话 ${detail?.session?.src_ip || ''}`" size="62%">
      <div v-if="detail" class="hp-detail">
        <el-descriptions :column="3" size="small" border>
          <el-descriptions-item label="IP">{{ detail.session.src_ip }}</el-descriptions-item>
          <el-descriptions-item label="HID">{{ detail.session.hid }}</el-descriptions-item>
          <el-descriptions-item label="风险">{{ detail.session.max_risk }}</el-descriptions-item>
          <el-descriptions-item label="首次">{{ fmt(detail.session.first_seen) }}</el-descriptions-item>
          <el-descriptions-item label="最近">{{ fmt(detail.session.last_seen) }}</el-descriptions-item>
          <el-descriptions-item label="事件数">{{ detail.session.event_count }}</el-descriptions-item>
        </el-descriptions>
        <div v-if="fingerprints.length" class="hp-fp">
          <div class="hp-fp-head">浏览器指纹（验证页加载即采集）</div>
          <div v-for="fp in fingerprints" :key="fp.id" class="hp-fp-item">
            <div class="hp-fp-line">
              <span>canvas {{ fp.canvas }}</span>
              <span>webgl {{ fp.webgl }}</span>
              <span>fonts {{ fp.fonts }}</span>
              <span>UA-DPR {{ fp.dpr }}</span>
              <span>核 {{ fp.cores }} / 内存 {{ fp.memory }}</span>
            </div>
            <div class="hp-fp-line">
              <span>时区 {{ fp.tz }}</span>
              <span>语言 {{ fp.lang }}</span>
              <span>屏幕 {{ fp.screen }}</span>
              <span>平台 {{ fp.client_hints || '—' }}</span>
            </div>
            <div v-if="fp.headless_flags" class="hp-fp-line hp-fp-warn">自动化痕迹：{{ fp.headless_flags }}</div>
          </div>
        </div>

        <el-timeline class="hp-timeline">
          <el-timeline-item
            v-for="e in detail.events"
            :key="e.id"
            :timestamp="fmt(e.created_at)"
            :type="e.risk >= 60 ? 'danger' : e.risk >= 40 ? 'warning' : 'primary'"
          >
            <div class="hp-ev-head">
              <el-tag size="small">{{ e.kind }}</el-tag>
              <b>{{ e.method }} {{ e.path }}</b>
              <el-tag v-if="e.decoy" size="small" type="info">{{ e.decoy }}</el-tag>
              <span class="hp-tip">风险 {{ e.risk }} · {{ e.action }}</span>
            </div>
            <div v-if="e.query" class="hp-ev-line">Query: {{ e.query }}</div>
            <div v-if="e.ua" class="hp-ev-line">UA: {{ e.ua }}</div>
            <div v-if="e.referer" class="hp-ev-line">Referer: {{ e.referer }}</div>
            <div v-if="e.xff" class="hp-ev-line">XFF: {{ e.xff }}</div>
            <div v-if="e.headers" class="hp-ev-line">Headers: {{ e.headers }}</div>
            <div v-if="e.body_snippet" class="hp-ev-line">Body: {{ e.body_snippet }}</div>
          </el-timeline-item>
        </el-timeline>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import StateBlock from '@/components/common/StateBlock.vue';
import AdminApiAlert from '@/components/admin/AdminApiAlert.vue';
import {
  honeypotApi,
  type HoneypotSetting,
  type HoneypotOverview,
  type HoneypotSession,
  type HoneypotEvent,
  type HoneypotFingerprint,
  type HoneypotRuleSuggestion,
} from '@/api/admin/honeypot';

const defaultAllow = 'X-Forwarded-For,X-Real-IP,X-Requested-With,Sec-Fetch-Site,CF-Connecting-IP';
const loading = ref(false);
const saving = ref(false);
const overview = ref<HoneypotOverview | null>(null);
const sessions = ref<HoneypotSession[]>([]);
const keyword = ref('');
const minRisk = ref(0);
const detailVisible = ref(false);
const detail = ref<{ session: HoneypotSession; events: HoneypotEvent[] } | null>(null);
const fingerprints = ref<HoneypotFingerprint[]>([]);
const rules = ref<HoneypotRuleSuggestion[]>([]);
const selectedRules = ref<string[]>([]);
const rulesLoading = ref(false);
const applying = ref(false);
const sqli = ref<any>(null);
const tls = ref<any>(null);
const regening = ref(false);
const manualKey = ref('');

const form = reactive<HoneypotSetting>({
  enabled: false, level: 'off',
  record_ua: true, record_referer: true, record_xff: false, record_query: true,
  record_headers: false, record_body_snippet: false, body_snippet_limit: 512,
  header_allowlist: defaultAllow,
  auto_ban: false, ban_minutes: 60, delay_ms: 0, retention_days: 90,
  annotation: '', custom_decoys: '',
});

function maskKey(k?: string) {
  if (!k) return '（无）';
  if (k.length <= 12) return k;
  return k.slice(0, 8) + '…' + k.slice(-4) + `（${k.length} 字符）`;
}

async function regenerate() {
  try {
    await ElMessageBox.confirm(
      '重新生成蜜罐密钥？旧的诱饵 .env 暴露值将失效，已签发的影子 token 全部作废。',
      '重新生成确认',
      { type: 'warning' },
    );
  } catch {
    return;
  }
  regening.value = true;
  try {
    const res: any = await honeypotApi.regenerateKey({});
    overview.value = overview.value || ({} as any);
    overview.value!.key_b64 = res?.data?.data?.key_b64;
    ElMessage.success('蜜罐密钥已重新生成');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成失败');
  } finally {
    regening.value = false;
  }
}

async function applyManual() {
  regening.value = true;
  try {
    const res: any = await honeypotApi.regenerateKey({ key: manualKey.value });
    overview.value = overview.value || ({} as any);
    overview.value!.key_b64 = res?.data?.data?.key_b64;
    manualKey.value = '';
    ElMessage.success('蜜罐密钥已更新');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '更新失败');
  } finally {
    regening.value = false;
  }
}

function fmt(t: string) {
  return t ? String(t).replace('T', ' ').slice(0, 19) : '';
}

async function load() {
  loading.value = true;
  try {
    const res = await honeypotApi.getConfig();
    overview.value = res.data.data;
    if (res.data.data?.setting) {
      Object.assign(form, res.data.data.setting);
      if (!form.header_allowlist) form.header_allowlist = defaultAllow;
    }
    await loadSessions();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

async function loadSessions() {
  try {
    const res = await honeypotApi.listSessions({ keyword: keyword.value, min_risk: minRisk.value, page_size: 50 });
    sessions.value = res.data.data?.list || [];
  } catch { /* 静默 */ }
}

async function save() {
  saving.value = true;
  try {
    await honeypotApi.saveConfig({ ...form });
    ElMessage.success('已保存');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function openDetail(row: HoneypotSession) {
  try {
    const res = await honeypotApi.sessionDetail(row.id);
    detail.value = res.data.data;
    detailVisible.value = true;
    fingerprints.value = [];
    try {
      const fp = await honeypotApi.fingerprints(row.id);
      fingerprints.value = fp.data.data?.list || [];
    } catch { /* 无指纹不阻断 */ }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载详情失败');
  }
}

async function banIp(ip: string) {
  try {
    await honeypotApi.ban(ip, form.ban_minutes || 60);
    ElMessage.success(`已封禁 ${ip}`);
    await loadSessions();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '封禁失败');
  }
}

async function doExport(sessionId: number) {
  try {
    const res: any = await honeypotApi.export(sessionId);
    const blob = new Blob([JSON.stringify(res.data.data, null, 2)], { type: 'application/json' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `honeypot-export-${sessionId || 'all'}-${Date.now()}.json`;
    a.click();
    URL.revokeObjectURL(a.href);
    ElMessage.success('已导出（导出动作会留痕）');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '导出失败');
  }
}

async function doExportZip(sessionId: number) {
  try {
    const res: any = await honeypotApi.exportZip(sessionId);
    const blob = res.data instanceof Blob ? res.data : new Blob([res.data], { type: 'application/zip' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `honeypot-forensics-${sessionId || 'all'}-${Date.now()}.zip`;
    a.click();
    URL.revokeObjectURL(a.href);
    ElMessage.success('取证包已下载（含 SHA256 清单，导出已留痕）');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '导出失败');
  }
}

async function loadRules() {
  rulesLoading.value = true;
  try {
    const res = await honeypotApi.rules();
    rules.value = res.data.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提炼失败');
  } finally {
    rulesLoading.value = false;
  }
}

async function applyRules() {
  applying.value = true;
  try {
    const res: any = await honeypotApi.applyRules(selectedRules.value);
    const added = res?.data?.data?.added || [];
    ElMessage.success(added.length ? `已写入 ${added.length} 条规则` : '这些规则已存在');
    selectedRules.value = [];
    await loadRules();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '写入失败');
  } finally {
    applying.value = false;
  }
}

async function cleanup() {
  try {
    await ElMessageBox.confirm('按留存天数清理过期事件？此操作不可撤销。', '清理确认', { type: 'warning' });
    const res: any = await honeypotApi.cleanup();
    ElMessage.success(`已清理 ${res?.data?.data?.deleted ?? 0} 条`);
    await load();
  } catch { /* 取消 */ }
}

async function loadAntiAbuse() {
  try {
    const [a, b] = await Promise.all([honeypotApi.sqliStats(), honeypotApi.tlsStats()]);
    sqli.value = a.data.data;
    tls.value = b.data.data;
  } catch { /* 未启用或表不存在时静默 */ }
}

onMounted(async () => {
  await load();
  await loadRules();
  await loadAntiAbuse();
});
</script>

<style scoped>
.hp-page { padding: 16px 20px 48px; max-width: 1180px; margin: 0 auto; }
.hp-head { display: flex; align-items: baseline; gap: 10px; flex-wrap: wrap; margin-bottom: 12px; }
.hp-head h2 { margin: 0; font-size: 1.3rem; }
.hp-sub { color: var(--text-muted); font-size: 0.82rem; }
.hp-cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 10px; margin-bottom: 14px; }
.hp-card { border: 1px solid var(--border-color); border-radius: 10px; padding: 12px; background: var(--bg-card); text-align: center; }
.hp-num { font-size: 1.6rem; font-weight: 700; }
.hp-label { font-size: 0.78rem; color: var(--text-muted); }
.hp-section { margin-bottom: 14px; }
.hp-section-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.hp-filters { margin-left: auto; display: flex; gap: 8px; align-items: center; }
.hp-tip { font-size: 0.75rem; color: var(--text-muted); margin-left: 8px; }
.hp-check-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 6px 12px; }
.hp-detail { display: flex; flex-direction: column; gap: 12px; }
.hp-timeline { padding-left: 4px; }
.hp-ev-head { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.hp-ev-line { font-size: 0.78rem; color: var(--text-muted); word-break: break-all; }
.hp-fp { border: 1px solid var(--border-color); border-radius: 8px; padding: 10px 12px; }
.hp-fp-head { font-size: 0.85rem; font-weight: 600; margin-bottom: 6px; }
.hp-fp-item { border-top: 1px dashed var(--border-color); padding: 6px 0; }
.hp-fp-item:first-of-type { border-top: 0; }
.hp-fp-line { display: flex; gap: 12px; flex-wrap: wrap; font-size: 0.75rem; color: var(--text-muted); }
.hp-fp-warn { color: var(--el-color-danger); }
.hp-tls-hint { margin-top: 10px; }
.hp-pre {
  background: var(--bg-muted); border-radius: 6px; padding: 8px 10px;
  font-size: 0.72rem; white-space: pre-wrap; margin: 6px 0;
}
.hp-key {
  background: var(--bg-muted); border-radius: 6px; padding: 4px 8px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.78rem;
  word-break: break-all; max-width: 360px; display: inline-block;
}
</style>
