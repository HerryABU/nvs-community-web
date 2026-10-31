<template>
  <div class="admin-page">
    <AdminHeader
      title="密码与国密"
      subtitle="国际轨 / 国密轨 / 双轨 · 证书就绪状态 · 握手可观测"
      :icon="Key"
    />

    <div class="admin-page-body">
      <!-- ── 当前状态 ── -->
      <div class="admin-panel">
        <SectionHeader
          title="当前状态"
          subtitle="以下为进程启动时确定的实际生效值，运行期不可热改"
          :icon="'🔐'"
          gradient="primary"
        />

        <el-alert
          v-if="loadError"
          type="error"
          :title="loadError"
          :closable="false"
          show-icon
        />

        <template v-else>
          <div class="crypto-grid">
            <div class="crypto-tile">
              <div class="tile-label">密码轨道</div>
              <div class="tile-value" :class="'track-' + data.track">{{ trackLabel(data.track) }}</div>
              <div class="tile-hint">{{ trackHint(data.track) }}</div>
            </div>
            <div class="crypto-tile">
              <div class="tile-label">国密实现</div>
              <div class="tile-value">
                {{ data.gm_compiled ? (data.gm_provider || '未指定') : '未编入' }}
              </div>
              <div class="tile-hint">
                {{ data.gm_compiled ? '本二进制含：' + (data.compiled_providers || []).join(' / ') : '当前为国际轨构建，不含任何国密模块' }}
              </div>
            </div>
            <div class="crypto-tile">
              <div class="tile-label">SM2 证书就绪</div>
              <div class="tile-value" :class="data.sm2_cert_present ? 'ok' : 'muted'">
                {{ data.sm2_cert_present ? '已就绪' : '未配置' }}
              </div>
              <div class="tile-hint">国密轨与双轨均需签名 + 加密两套证书</div>
            </div>
            <div class="crypto-tile">
              <div class="tile-label">会话复用</div>
              <div class="tile-value">{{ data.session_reuse_on ? '开启' : '已禁用' }}</div>
              <div class="tile-hint">对应 SessionTicketsDisabled</div>
            </div>
            <div class="crypto-tile">
              <div class="tile-label">握手日志</div>
              <div class="tile-value" :class="data.handshake_log_on ? 'ok' : 'muted'">
                {{ data.handshake_log_on ? '已开启' : '未落盘' }}
              </div>
              <div class="tile-hint">
                {{ data.handshake_log_path || '未配置路径 —— 握手明细不会写盘，统计仅内存可见' }}
              </div>
            </div>
            <div class="crypto-tile">
              <div class="tile-label">实现版本</div>
              <div class="tile-value small">
                {{ data.purego_version || data.tongsuo_version || '—' }}
              </div>
              <div class="tile-hint">
                <template v-if="data.tongsuo_version">Tongsuo：{{ data.tongsuo_version }}</template>
                <template v-else>纯 Go 路径（无 CGO 依赖）</template>
              </div>
            </div>
          </div>
        </template>
      </div>

      <!-- ── 三档轨道 ── -->
      <div class="admin-panel">
        <SectionHeader
          title="双轨制：三档可选"
          subtitle="同样支持标准 TLS —— 国密不是「只走国密」"
          :icon="'🔀'"
          gradient="teal"
        />
        <div class="track-cards">
          <div
            v-for="t in data.tracks"
            :key="t.value"
            class="track-card"
            :class="{ 'is-current': t.value === data.track }"
          >
            <div class="track-head">
              <span class="track-name">{{ t.label }}</span>
              <el-tag v-if="t.value === data.track" size="small" type="success" effect="dark">当前</el-tag>
            </div>
            <p class="track-summary">{{ t.summary }}</p>
            <dl class="track-meta">
              <dt>证书</dt>
              <dd>{{ t.certs.join(' ＋ ') }}</dd>
              <dt>构建</dt>
              <dd>{{ t.requires }}</dd>
              <dt>适用</dt>
              <dd>{{ t.best_for }}</dd>
            </dl>
          </div>
        </div>
        <el-alert type="info" :closable="false" show-icon class="mt12">
          <template #title>
            <b>双轨（dual）是绝大多数站点的落地选择</b>：同一端口按 ClientHello 自动分流 ——
            国密客户端走 TLCP（SM2 双证书 + SM4/SM3），普通浏览器走标准 TLS（RSA/ECDSA + AES-GCM）。
            既满足密评要求，又不影响任何现有访客。
          </template>
        </el-alert>
      </div>

      <!-- ── 证书 ── -->
      <div class="admin-panel">
        <SectionHeader title="证书" subtitle="磁盘上的真实状态 · 缺失或过期都在这里暴露" :icon="'📜'" gradient="warning" />
        <el-table :data="data.certs" size="small" border style="width: 100%">
          <el-table-column prop="label" label="用途" width="150" />
          <el-table-column label="文件" min-width="260">
            <template #default="{ row }">
              <span v-if="row.path" class="mono">{{ row.path }}</span>
              <span v-else class="muted">未配置</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag v-if="!row.present" size="small" type="info">未配置</el-tag>
              <el-tag v-else-if="row.expired" size="small" type="danger">已过期</el-tag>
              <el-tag v-else-if="row.error" size="small" type="warning">{{ row.error }}</el-tag>
              <el-tag v-else size="small" type="success">可用</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="到期" width="200">
            <template #default="{ row }">
              <span v-if="row.not_after">{{ fmtDate(row.not_after) }}</span>
              <span v-else class="muted">—</span>
              <el-tag
                v-if="row.not_after && !row.expired && row.days_left <= 30"
                size="small"
                type="warning"
                style="margin-left: 6px"
              >{{ row.days_left }} 天</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="subject" label="主体（CN）" min-width="160">
            <template #default="{ row }">
              <span v-if="row.subject">{{ row.subject }}</span>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
        </el-table>
        <p class="hint-text">
          国际轨证书只在「设置证书路径」时才显示细节；留空则由运行时生成自签证书（浏览器会提示不安全，
          生产环境请换成正式证书）。SM2 两张证书的有效期由 gm 轨专用解析器读取，国际轨构建下不可见。
        </p>
      </div>

      <!-- ── 握手统计 ── -->
      <div class="admin-panel">
        <SectionHeader title="握手统计" subtitle="密评举证时可作为「协商过程可审计」的佐证" :icon="'📊'" gradient="purple" />
        <div class="hs-row">
          <div class="hs-num">
            <span class="hs-val">{{ data.handshake?.total ?? 0 }}</span>
            <span class="hs-key">总握手</span>
          </div>
          <div class="hs-num">
            <span class="hs-val" :class="{ danger: (data.handshake?.failed ?? 0) > 0 }">{{ data.handshake?.failed ?? 0 }}</span>
            <span class="hs-key">失败</span>
          </div>
          <div class="hs-num">
            <span class="hs-val">{{ Object.keys(data.handshake?.by_proto || {}).length }}</span>
            <span class="hs-key">协议种类</span>
          </div>
          <div class="hs-num">
            <span class="hs-val">{{ Object.keys(data.handshake?.by_suite || {}).length }}</span>
            <span class="hs-key">套件种类</span>
          </div>
        </div>
        <div class="hs-tables">
          <div>
            <h4 class="hs-title">按协议</h4>
            <el-table :data="kvRows(data.handshake?.by_proto)" size="small" border>
              <el-table-column prop="k" label="协议版本" min-width="160" />
              <el-table-column prop="v" label="次数" width="100" align="right" />
            </el-table>
          </div>
          <div>
            <h4 class="hs-title">按密码套件</h4>
            <el-table :data="kvRows(data.handshake?.by_suite)" size="small" border>
              <el-table-column prop="k" label="套件" min-width="160" />
              <el-table-column prop="v" label="次数" width="100" align="right" />
            </el-table>
          </div>
        </div>
        <p class="hint-text">
          统计来自握手 JSONL 日志（配置 HandshakeLogPath 后才会累积）与内存快照。
          未配置日志路径时，重启即归零。
        </p>
      </div>

      <!-- ── 怎么开启 ── -->
      <div class="admin-panel">
        <SectionHeader title="如何切到国密 / 双轨" subtitle="改 .env 后必须重启 —— TLS 参数在监听器创建前生效" :icon="'🛠'" gradient="accent" />
        <ol class="steps">
          <li>
            <b>换成带国密能力的二进制</b>：默认发布包是国际轨（<code class="mono">go.mod</code> 零国密依赖）。
            需自行构建：<code class="mono">make build-gm</code>（纯 Go，无 CGO）或
            <code class="mono">make build-gm-cgo</code>（Tongsuo，需 <code class="mono">TONGSUO_HOME</code>）。
          </li>
          <li>
            <b>准备证书</b>：国密需 <b>签名证书 + 加密证书</b> 两套（TLCP 的双证书要求）；
            双轨还要额外准备一套普通国际证书。
          </li>
          <li>
            <b>改 .env 并重启</b>（键名与含义见
            <el-link type="primary" :underline="false" @click="showEnvKeys = !showEnvKeys">
              {{ showEnvKeys ? '收起' : '展开' }}全部键
            </el-link>）。
          </li>
        </ol>

        <el-collapse-transition>
          <div v-show="showEnvKeys">
            <el-table :data="envKeys" size="small" border class="mt8">
              <el-table-column prop="key" label=".env 键" width="290">
                <template #default="{ row }"><code class="mono">{{ row.key }}</code></template>
              </el-table-column>
              <el-table-column prop="desc" label="说明" min-width="320" />
            </el-table>
          </div>
        </el-collapse-transition>

        <pre class="code-block"># 双轨示例（国密客户端走 TLCP，普通浏览器走标准 TLS）
NVS_CRYPTO_TRACK=dual
NVS_CRYPTO_GM_PROVIDER=purego
NVS_CRYPTO_INTL_CERT_FILE=./certs/fullchain.pem
NVS_CRYPTO_INTL_KEY_FILE=./certs/privkey.pem
NVS_CRYPTO_SM_SIGN_CERT_FILE=./certs/sign.pem
NVS_CRYPTO_SM_SIGN_KEY_FILE=./certs/sign.key
NVS_CRYPTO_SM_ENC_CERT_FILE=./certs/enc.pem
NVS_CRYPTO_SM_ENC_KEY_FILE=./certs/enc.key
NVS_CRYPTO_HANDSHAKE_LOG=./data/tls-handshake.jsonl</pre>

        <p class="hint-text">
          命令行自检（与页面数据同源）：<code class="mono">nvs-server gm-audit --out gm-report.md</code>
          输出 Markdown 报告，含轨道、实现路径、证书有效期、握手统计。
        </p>
      </div>

      <!-- ── 免责 ── -->
      <el-alert type="warning" :closable="false" show-icon>
        <template #title>
          本页只做状态展示，<b>不构成合规结论</b>。等保定级备案、密评（GB/T 39786-2021）、
          密码模块认证（GM/T 0028）均需测评机构出具正式报告。框架提供的是可观测能力与配置开关，
          证书签发、私钥保管、测评举证由站长完成。
        </template>
      </el-alert>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { Key } from '@element-plus/icons-vue';
import api from '@/api/index';

interface CertInfo {
  role: string;
  label: string;
  path: string;
  present: boolean;
  readable: boolean;
  not_after?: string;
  days_left?: number;
  expired: boolean;
  self_signed?: boolean;
  subject?: string;
  error?: string;
}

interface TrackDoc {
  value: string;
  label: string;
  summary: string;
  certs: string[];
  best_for: string;
  requires: string;
}

interface CryptoStatus {
  track: string;
  gm_provider: string;
  compiled_providers: string[] | null;
  purego_version: string;
  tongsuo_version: string;
  gm_compiled: boolean;
  session_reuse_on: boolean;
  handshake_log_path: string;
  handshake_log_on: boolean;
  sm2_cert_present: boolean;
  certs: CertInfo[];
  handshake: { total: number; failed: number; by_proto: Record<string, number>; by_suite: Record<string, number> };
  tracks: TrackDoc[];
  generated_at: string;
}

const data = ref<Partial<CryptoStatus>>({ certs: [], tracks: [], handshake: { total: 0, failed: 0, by_proto: {}, by_suite: {} } });
const loadError = ref('');
const showEnvKeys = ref(false);

const envKeys = [
  { key: 'NVS_CRYPTO_TRACK', desc: 'intl（默认）/ gm（纯国密）/ dual（双轨并行）' },
  { key: 'NVS_CRYPTO_GM_PROVIDER', desc: 'purego | tongsuo；track 为 gm/dual 时必填，否则启动即报错' },
  { key: 'NVS_CRYPTO_INTL_CERT_FILE', desc: '国际轨证书（PEM）；留空则运行时生成自签' },
  { key: 'NVS_CRYPTO_INTL_KEY_FILE', desc: '国际轨私钥' },
  { key: 'NVS_CRYPTO_SM_SIGN_CERT_FILE', desc: 'SM2 签名证书（TLCP 双证书之一）' },
  { key: 'NVS_CRYPTO_SM_SIGN_KEY_FILE', desc: 'SM2 签名私钥' },
  { key: 'NVS_CRYPTO_SM_ENC_CERT_FILE', desc: 'SM2 加密证书（TLCP 双证书之二）' },
  { key: 'NVS_CRYPTO_SM_ENC_KEY_FILE', desc: 'SM2 加密私钥' },
  { key: 'NVS_CRYPTO_SESSION_TICKETS_DISABLED', desc: '是否禁用 TLS 会话复用（部分密评项要求关闭）' },
  { key: 'NVS_CRYPTO_HANDSHAKE_LOG', desc: '握手 JSONL 输出路径；留空则不落盘，统计仅内存可见' },
];

function trackLabel(t?: string) {
  return ({ intl: '国际轨', gm: '国密轨', dual: '双轨' } as Record<string, string>)[t || ''] || t || '—';
}

function trackHint(t?: string) {
  return (
    ({
      intl: '标准 TLS · 零国密依赖',
      gm: '只接受 TLCP 国密握手',
      dual: '按 ClientHello 自动分流',
    } as Record<string, string>)[t || ''] || ''
  );
}

function kvRows(map?: Record<string, number>) {
  if (!map) return [];
  return Object.entries(map).map(([k, v]) => ({ k, v }));
}

function fmtDate(s?: string) {
  if (!s) return '—';
  try {
    return new Date(s).toLocaleString();
  } catch {
    return s;
  }
}

async function load() {
  loadError.value = '';
  try {
    const r = await api.get('/admin/crypto/status');
    if (r.data?.code === 0) {
      data.value = r.data.data as CryptoStatus;
    } else {
      loadError.value = r.data?.message || '读取失败';
    }
  } catch (e: any) {
    loadError.value = e?.response?.data?.message || '接口不可用（需要管理员权限）';
  }
}

onMounted(load);
</script>

<style scoped>
.mt8 { margin-top: 8px; }
.mt12 { margin-top: 12px; }
.mono { font-family: var(--font-mono, Consolas, Monaco, monospace); font-size: 0.82rem; }
.muted { color: var(--text-light, #999); }
.hint-text {
  margin: 8px 0 0;
  font-size: 0.78rem;
  color: var(--text-light, #909399);
  line-height: 1.6;
}

/* ── 状态磁贴 ── */
.crypto-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  gap: 12px;
}
.crypto-tile {
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 10px;
  padding: 12px 14px;
  background: var(--bg-page, #fafbfc);
}
.tile-label {
  font-size: 0.74rem;
  color: var(--text-light, #909399);
  margin-bottom: 6px;
}
.tile-value {
  font-size: 1.05rem;
  font-weight: 700;
  color: var(--text-color, #1f2328);
  word-break: break-all;
}
.tile-value.small { font-size: 0.82rem; font-weight: 600; }
.tile-value.ok { color: var(--el-color-success, #22c55e); }
.tile-value.muted { color: var(--text-light, #9ca3af); }
.tile-value.track-intl { color: #409eff; }
.tile-value.track-gm { color: #e6a23c; }
.tile-value.track-dual { color: #22c55e; }
.tile-hint {
  margin-top: 6px;
  font-size: 0.72rem;
  color: var(--text-light, #a0a4ab);
  line-height: 1.5;
  word-break: break-all;
}

/* ── 轨道卡片 ── */
.track-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(270px, 1fr));
  gap: 12px;
}
.track-card {
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 10px;
  padding: 14px;
  background: var(--card-bg, #fff);
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}
.track-card.is-current {
  border-color: var(--el-color-success, #22c55e);
  box-shadow: 0 0 0 2px rgba(34, 197, 94, 0.12);
}
.track-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}
.track-name { font-weight: 700; font-size: 0.95rem; }
.track-summary {
  margin: 0 0 10px;
  font-size: 0.8rem;
  line-height: 1.65;
  color: var(--text-secondary, #606266);
}
.track-meta {
  display: grid;
  grid-template-columns: 40px 1fr;
  gap: 4px 8px;
  margin: 0;
  font-size: 0.76rem;
  line-height: 1.6;
}
.track-meta dt { color: var(--text-light, #909399); }
.track-meta dd { margin: 0; color: var(--text-secondary, #606266); }

/* ── 握手统计 ── */
.hs-row {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}
.hs-num { display: flex; flex-direction: column; }
.hs-val { font-size: 1.5rem; font-weight: 700; line-height: 1.2; }
.hs-val.danger { color: var(--el-color-danger, #f56c6c); }
.hs-key { font-size: 0.74rem; color: var(--text-light, #909399); }
.hs-tables {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
}
.hs-title {
  margin: 0 0 6px;
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text-secondary, #606266);
}

/* ── 步骤与代码 ── */
.steps {
  margin: 0;
  padding-left: 20px;
  font-size: 0.84rem;
  line-height: 1.9;
  color: var(--text-secondary, #606266);
}
.code-block {
  margin: 12px 0 0;
  padding: 12px 14px;
  border-radius: 8px;
  background: #1e293b;
  color: #e2e8f0;
  font-family: var(--font-mono, Consolas, Monaco, monospace);
  font-size: 0.78rem;
  line-height: 1.7;
  overflow-x: auto;
  white-space: pre;
}

@media (max-width: 640px) {
  .crypto-grid { grid-template-columns: 1fr; }
  .track-cards { grid-template-columns: 1fr; }
  .hs-row { gap: 16px; }
  .hs-val { font-size: 1.25rem; }
}
</style>
