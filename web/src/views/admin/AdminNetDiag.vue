<template>
  <div class="admin-page">
    <AdminHeader title="网络诊断与攻击流量" subtitle="用户分布 · IP/UA 分类 · 防爬与攻击流量识别" :icon="Connection" />
    <div class="admin-page-body">

      <!-- 统计卡片 -->
      <div class="admin-panel">
        <SectionHeader title="访问概览" subtitle="累计 / 今日请求 · 爬虫与攻击流量" :icon="'📊'" gradient="blue" />
        <div class="admin-stat-grid" v-loading="loadingStats">
          <div class="admin-stat-card">
            <div class="admin-stat-icon" style="background: var(--gradient-blue, var(--gradient-primary))">📊</div>
            <div>
              <div class="admin-stat-value">{{ stats.total }}</div>
              <div class="admin-stat-label">累计请求</div>
            </div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-icon" style="background: var(--gradient-primary, var(--gradient-primary))">📅</div>
            <div>
              <div class="admin-stat-value">{{ stats.today }}</div>
              <div class="admin-stat-label">今日请求</div>
            </div>
          </div>
          <div class="admin-stat-card stat-bot">
            <div class="admin-stat-icon" style="background: var(--gradient-amber, var(--gradient-primary))">🕷️</div>
            <div>
              <div class="admin-stat-value">{{ stats.bot_total }}</div>
              <div class="admin-stat-label">爬虫命中总数</div>
            </div>
          </div>
          <div class="admin-stat-card stat-attack">
            <div class="admin-stat-icon" style="background: var(--gradient-danger, var(--gradient-primary))">🛡️</div>
            <div>
              <div class="admin-stat-value">{{ stats.attack_total }}</div>
              <div class="admin-stat-label">近24h 攻击流量</div>
            </div>
          </div>
        </div>
      </div>

      <!-- 分布网格 -->
      <div class="admin-panel">
        <SectionHeader title="分布分析" subtitle="Top IP · 状态码 · UA 家族 · 爬虫原因 · 路径" :icon="'📈'" gradient="purple" />
        <p class="hint-text">
          名义上是「看用户分布、诊断网络」，同时用于识别防爬与攻击流量。<b>关于 TTL：</b>HTTP 应用层无法读取 IP 层 TTL 字段，本面板以
          <code>X-Forwarded-For</code> 跳数（Proxy Hops）近似代理深度，辅助判断真实客户端位置。
        </p>
        <div class="dist-grid" v-loading="loadingStats">
          <div class="dist-box">
            <div class="dist-title">Top IP（请求数）</div>
            <el-table :data="stats.top_ips" size="small" max-height="220" border>
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
              <el-table-column prop="ip" label="IP" min-width="140" />
              <el-table-column prop="cnt" label="次数" width="80" />
            </el-table>
          </div>
          <div class="dist-box">
            <div class="dist-title">状态码分布</div>
            <el-table :data="stats.status_dist" size="small" max-height="220" border>
              <el-table-column label="状态码" width="100">
                <template #default="{ row }"><span :class="['st', 'st-' + String(row.status_code)[0]]">{{ row.status_code }}</span></template>
              </el-table-column>
              <el-table-column prop="cnt" label="次数" width="80" />
            </el-table>
          </div>
          <div class="dist-box">
            <div class="dist-title">UA 家族分布</div>
            <el-table :data="stats.ua_family_dist" size="small" max-height="220" border>
              <el-table-column label="家族" width="120">
                <template #default="{ row }"><el-tag :type="familyTag(row.family)" size="small">{{ row.family }}</el-tag></template>
              </el-table-column>
              <el-table-column prop="cnt" label="次数" width="80" />
            </el-table>
          </div>
          <div class="dist-box">
            <div class="dist-title">爬虫命中原因</div>
            <el-table :data="stats.bot_reason_dist" size="small" max-height="220" border>
              <el-table-column prop="reason" label="原因" min-width="140" />
              <el-table-column prop="cnt" label="次数" width="80" />
            </el-table>
          </div>
        </div>

        <div class="dist-box dist-paths">
          <div class="dist-title">Top 路径（请求数）</div>
          <el-table :data="stats.top_paths" size="small" max-height="200" border>
            <el-table-column prop="path" label="路径" min-width="300" show-overflow-tooltip />
            <el-table-column prop="cnt" label="次数" width="90" />
          </el-table>
        </div>
      </div>

      <!-- 访问明细 -->
      <div class="admin-panel">
        <SectionHeader title="访问明细" subtitle="按条件检索 Web 访问日志" :icon="'🗂️'" gradient="primary" />
        <!-- 9 个筛选条件一行排不下会折成 3 行，把表格顶到 800px 以下；
             常用的（时间范围 / IP / 身份）留在外面，其余收进「更多筛选」。 -->
        <AdminFilterBar storage-key="net-diag" :more-count="5">
          <template #primary>
            <el-date-picker v-model="filters.from" type="date" placeholder="起始" size="small" value-format="YYYY-MM-DD" style="width:140px" />
            <el-date-picker v-model="filters.to" type="date" placeholder="结束" size="small" value-format="YYYY-MM-DD" style="width:140px" />
            <el-input v-model="filters.ip" placeholder="IP" size="small" style="width:140px" clearable @keyup.enter="reload" />
            <el-select v-model="filters.identity" placeholder="身份" clearable size="small" style="width:120px">
              <el-option label="key" value="key" />
              <el-option label="user" value="user" />
              <el-option label="anonymous" value="anonymous" />
              <el-option label="bot" value="bot" />
            </el-select>
          </template>

          <template #more>
            <el-input v-model="filters.ua" placeholder="User-Agent 关键词" size="small" style="width:180px" clearable @keyup.enter="reload" />
            <el-select v-model="filters.ua_family" placeholder="UA 家族" clearable size="small" style="width:130px">
              <el-option v-for="f in families" :key="f" :label="f" :value="f" />
            </el-select>
            <el-select v-model="filters.is_bot" placeholder="是否爬虫" clearable size="small" style="width:120px">
              <el-option label="是" value="1" />
              <el-option label="否" value="0" />
            </el-select>
            <el-input v-model="filters.status" placeholder="状态码" size="small" style="width:100px" clearable @keyup.enter="reload" />
            <el-checkbox v-model="filters.attack" size="small" border>仅看攻击流量</el-checkbox>
          </template>

          <template #actions>
            <el-button size="small" type="primary" @click="reload" :icon="Search">查询</el-button>
            <el-button size="small" @click="resetFilters" :icon="RefreshLeft">重置</el-button>
            <el-button size="small" type="danger" plain @click="cleanup" :icon="Delete">清空日志</el-button>
          </template>
        </AdminFilterBar>

        <el-table :data="rows" v-loading="loading" stripe border style="width:100%"
                  :row-class-name="rowCls" :default-sort="{ prop: 'created_at', order: 'descending' }">
          <el-table-column label="时间" prop="created_at" width="180" sortable />
          <el-table-column label="IP" width="150">
            <template #default="{ row }">
              <span>{{ row.ip }}</span>
              <el-tooltip v-if="row.proxy_hops > 0" :content="'X-Forwarded-For 跳数: ' + row.proxy_hops" placement="top">
                <span class="hops">↪{{ row.proxy_hops }}</span>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column label="UA 家族" width="110">
            <template #default="{ row }"><el-tag :type="familyTag(row.ua_family)" size="small">{{ row.ua_family || 'unknown' }}</el-tag></template>
          </el-table-column>
          <el-table-column label="身份" width="100">
            <template #default="{ row }"><el-tag :type="identityTag(row.identity_type)" size="small">{{ row.identity_type }}</el-tag></template>
          </el-table-column>
          <el-table-column label="方法" prop="method" width="80" />
          <el-table-column label="路径" min-width="240" show-overflow-tooltip>
            <template #default="{ row }"><span class="path-text">{{ row.path }}</span></template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }"><span :class="['st', 'st-' + String(row.status_code)[0]]">{{ row.status_code }}</span></template>
          </el-table-column>
          <el-table-column label="爬虫" width="80">
            <template #default="{ row }">
              <el-tag v-if="row.is_bot" type="danger" size="small">是</el-tag>
              <el-tag v-else type="info" size="small">否</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="耗时" width="90">
            <template #default="{ row }">{{ row.latency_ms }}ms</template>
          </el-table-column>
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ row }"><el-button size="small" text type="primary" @click="openDetail(row)">详情</el-button></template>
          </el-table-column>
          <template #empty><EmptyState icon="📭" text="暂无访问记录" /></template>
        </el-table>

        <div class="pager">
          <el-pagination background layout="total, prev, pager, next" :total="total"
            :page-size="filters.page_size" :current-page="filters.page" @current-change="onPage" />
        </div>
      </div>
    </div>

    <!-- 详情抽屉 -->
    <el-drawer v-model="drawer" title="访问记录详情" size="460px">
      <template v-if="current">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="时间">{{ current.created_at }}</el-descriptions-item>
          <el-descriptions-item label="用户ID">{{ current.user_id || '匿名(0)' }}</el-descriptions-item>
          <el-descriptions-item label="身份">
            <el-tag :type="identityTag(current.identity_type)" size="small">{{ current.identity_type }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="IP">{{ current.ip }}</el-descriptions-item>
          <el-descriptions-item label="X-Forwarded-For">{{ current.x_forwarded_for || '—' }}</el-descriptions-item>
          <el-descriptions-item label="代理跳数(Proxy Hops)">{{ current.proxy_hops }}</el-descriptions-item>
          <el-descriptions-item label="方法 / 路径">{{ current.method }} {{ current.path }}</el-descriptions-item>
          <el-descriptions-item label="状态码">
            <span :class="['st', 'st-' + String(current.status_code)[0]]">{{ current.status_code }}</span>
            （{{ current.size }} bytes / {{ current.latency_ms }}ms）
          </el-descriptions-item>
          <el-descriptions-item label="是否爬虫">
            <el-tag :type="current.is_bot ? 'danger' : 'info'" size="small">{{ current.is_bot ? '是' : '否' }}</el-tag>
            <span v-if="current.bot_reason" class="reason"> · {{ current.bot_reason }}</span>
          </el-descriptions-item>
        </el-descriptions>
        <div class="ua-block">
          <div class="ua-title">User-Agent</div>
          <pre class="ua-pre">{{ current.user_agent || '—' }}</pre>
        </div>
        <el-alert type="warning" :closable="false" class="ttl-note">
          TTL 为 IP 协议层字段，HTTP 应用层（含本中间件）无法读取；代理跳数已作为近似网络特征呈现。
        </el-alert>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { Connection, Search, RefreshLeft, Delete } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';
import StateBlock from '@/components/common/StateBlock.vue';
import AdminFilterBar from '@/components/admin/AdminFilterBar.vue';

interface WebRow {
  id: number; user_id: number; identity_type: string; ua_family: string;
  method: string; path: string; ip: string; x_forwarded_for: string; proxy_hops: number;
  user_agent: string; status_code: number; size: number; latency_ms: number;
  is_bot: boolean; bot_reason: string; created_at: string;
}

const rows = ref<WebRow[]>([]);
const total = ref(0);
const loading = ref(false);
const loadingStats = ref(false);
const current = ref<WebRow | null>(null);
const drawer = ref(false);

const families = ['browser', 'mobile', 'bot', 'tool', 'unknown'];
const stats = reactive({
  total: 0, today: 0, bot_total: 0, attack_total: 0,
  top_ips: [] as any[], top_paths: [] as any[], status_dist: [] as any[],
  ua_family_dist: [] as any[], bot_reason_dist: [] as any[],
});

const filters = reactive({
  ip: '', ua: '', ua_family: '', identity: '', is_bot: '', status: '',
  from: '', to: '', attack: false, page: 1, page_size: 30,
});

function familyTag(f: string): any {
  return ({ browser: 'primary', mobile: 'success', bot: 'danger', tool: 'warning', unknown: 'info' } as any)[f] || 'info';
}
function identityTag(t: string): any {
  return ({ key: 'warning', user: 'primary', anonymous: 'info', bot: 'danger' } as any)[t] || 'info';
}
function rowCls({ row }: any) {
  return row.is_bot || row.status_code >= 400 ? 'attack-row' : '';
}

async function loadStats() {
  loadingStats.value = true;
  try {
    const res = await adminApi.getWebAccessStats();
    const d = res.data.data || res.data || {};
    Object.assign(stats, {
      total: d.total || 0, today: d.today || 0, bot_total: d.bot_total || 0, attack_total: d.attack_total || 0,
      top_ips: d.top_ips || [], top_paths: d.top_paths || [], status_dist: d.status_dist || [],
      ua_family_dist: d.ua_family_dist || [], bot_reason_dist: d.bot_reason_dist || [],
    });
  } catch { /* ignore */ }
  loadingStats.value = false;
}

async function reload() {
  loading.value = true;
  try {
    const params: Record<string, any> = { page: filters.page, page_size: filters.page_size };
    if (filters.ip) params.ip = filters.ip;
    if (filters.ua) params.ua = filters.ua;
    if (filters.ua_family) params.ua_family = filters.ua_family;
    if (filters.identity) params.identity = filters.identity;
    if (filters.is_bot) params.is_bot = filters.is_bot === '1';
    if (filters.status) params.status = Number(filters.status);
    if (filters.from) params.from = filters.from;
    if (filters.to) params.to = filters.to;
    if (filters.attack) params.attack = true;
    const res = await adminApi.getWebAccessLogs(params);
    const d = res.data.data || res.data || {};
    rows.value = d.items || [];
    total.value = d.total || 0;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  }
  loading.value = false;
}

function onPage(p: number) { filters.page = p; reload(); }
function resetFilters() {
  filters.ip = ''; filters.ua = ''; filters.ua_family = ''; filters.identity = '';
  filters.is_bot = ''; filters.status = ''; filters.from = ''; filters.to = '';
  filters.attack = false; filters.page = 1; reload();
}
function openDetail(row: WebRow) { current.value = row; drawer.value = true; }

async function cleanup() {
  try {
    await ElMessageBox.confirm('确认清空 Web 访问日志？合规归档证据由文件日志保留，本表仅用于前台诊断。', '清空确认', { type: 'warning' });
  } catch { return; }
  try {
    await adminApi.cleanupWebAccessLogs();
    ElMessage.success('已清空');
    loadStats(); reload();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '清空失败');
  }
}

onMounted(() => { loadStats(); reload(); });
</script>

<style scoped>
.admin-page { --admin-content-max: 1180px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin: 0 0 16px; line-height: 1.6; }
.hint-text code { background: #eef2f7; padding: 1px 5px; border-radius: 4px; font-size: 0.8rem; }
.dist-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 12px; }
.dist-box { background: var(--bg-card, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: var(--radius-md, 10px); padding: 12px; }
.dist-title { font-size: 0.82rem; font-weight: 600; color: var(--text-color, #333); margin-bottom: 8px; }
.dist-paths { margin-bottom: 16px; }
.hops { margin-left: 4px; font-size: 0.72rem; color: var(--el-text-color-secondary); }
.path-text { font-size: 0.8rem; color: var(--text-light, #555); }
.pager { display: flex; justify-content: flex-end; margin-top: 14px; }
.ua-block { margin-top: 16px; }
.ua-title { font-size: 0.85rem; font-weight: 600; margin-bottom: 6px; }
.ua-pre { background: var(--bg-page, #f5f7fa); border-radius: 8px; padding: 10px; font-size: 0.78rem; white-space: pre-wrap; word-break: break-all; max-height: 200px; overflow: auto; }
.reason { color: var(--el-color-danger); font-size: 0.8rem; }
.ttl-note { margin-top: 14px; }
.st { font-weight: 600; }
.st-2 { color: var(--el-color-success); } .st-3 { color: var(--el-color-primary); } .st-4 { color: var(--el-color-warning); } .st-5 { color: var(--el-color-danger); }
.stat-bot .admin-stat-value { color: var(--el-color-warning); }
.stat-attack .admin-stat-value { color: var(--el-color-danger); }
:deep(.attack-row) { background: #fef0f0 !important; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .dist-box { background: #1e293b; border-color: rgba(255,255,255,0.08); }
[data-theme="dark"] .ua-pre { background: #0f172a; }
[data-theme="dark"] .hint-text code { background: #1e293b; }
[data-theme="dark"] :deep(.attack-row) { background: #3a1d1d !important; }
</style>
