<template>
  <div class="admin-page">
    <AdminHeader title="警方取管台" subtitle="取证留证 · 软删除查看 · 凭证下载（176号令合规）" :icon="Stamp">
      <template #actions>
        <el-button type="success" :loading="exporting" @click="doExport">
          <el-icon><Download /></el-icon> 导出全部取证包
        </el-button>
        <el-button @click="$router.push(authStore.user?.role === 'admin' ? '/admin' : '/')">
          返回
        </el-button>
      </template>
    </AdminHeader>

    <el-alert
      type="warning"
      :closable="false"
      show-icon
      title="权限说明"
      description="本台为警方取证专用：可查看全部软删除留证记录（softdel_*）并下载完整取证凭证。普通管理功能（用户封禁、内容治理等）不在此处。取证导出含软删除留证 + 隔离墙访问 + Web 访问 + 用户审计，便于提前准备取证材料。"
      style="margin-bottom: 16px"
    />

    <el-tabs v-model="activeTab" class="pc-tabs" @tab-change="onTabChange">
      <!-- 舆情看板：放在最前，警方进来先看「这个站的舆论有没有失控」 -->
      <el-tab-pane label="舆情看板" name="sentiment">
        <StateBlock v-if="sentLoading" state="loading" skeleton="card" :rows="4" />
        <StateBlock
          v-else-if="sentError"
          state="error"
          title="统计加载失败"
          description="请稍后重试"
          action-text="重试"
          @action="loadSentiment"
        />
        <template v-else>
          <div class="pc-stats">
            <div class="pc-stat">
              <span class="pc-stat-num">{{ sent?.last_24h ?? 0 }}</span>
              <span class="pc-stat-label">24 小时命中</span>
            </div>
            <div class="pc-stat pc-stat--danger">
              <span class="pc-stat-num">{{ sent?.blocked ?? 0 }}</span>
              <span class="pc-stat-label">累计拦截</span>
            </div>
            <div class="pc-stat">
              <span class="pc-stat-num">{{ sent?.flagged ?? 0 }}</span>
              <span class="pc-stat-label">仅标注</span>
            </div>
            <div class="pc-stat pc-stat--warn">
              <span class="pc-stat-num">{{ sent?.high_risk ?? 0 }}</span>
              <span class="pc-stat-label">高风险话题命中</span>
            </div>
            <div class="pc-stat">
              <span class="pc-stat-num">{{ sent?.pending_terms ?? 0 }}</span>
              <span class="pc-stat-label">待审词条</span>
            </div>
          </div>

          <div class="pc-note">
            本页只给聚合计数，不含被拦截评论正文。逐条内容取证请用「软删除留证」或评论区留证视图
            （均带权限校验与审计留痕）。
          </div>

          <div class="pc-panels">
            <div class="pc-panel">
              <h4>话题分布（近 30 天）</h4>
              <StateBlock v-if="!sent?.top_groups?.length" state="empty" title="暂无数据" description="有命中记录后会出现" />
              <ul v-else class="pc-rank">
                <li v-for="g in sent.top_groups" :key="g.key">
                  <span>{{ groupLabel(g.key) }}</span>
                  <b>{{ g.count }}</b>
                </li>
              </ul>
            </div>
            <div class="pc-panel">
              <h4>高频命中词（近 30 天）</h4>
              <StateBlock v-if="!sent?.top_words?.length" state="empty" title="暂无数据" description="有命中记录后会出现" />
              <ul v-else class="pc-rank">
                <li v-for="w in sent.top_words" :key="w.key">
                  <span>{{ w.key }}</span>
                  <b>{{ w.count }}</b>
                </li>
              </ul>
            </div>
            <div class="pc-panel">
              <h4>近 14 天趋势</h4>
              <StateBlock v-if="!trend.length" state="empty" title="暂无数据" description="按天统计拦截与标注" />
              <ul v-else class="pc-rank">
                <li v-for="t in trend" :key="t.day">
                  <span>{{ t.day }}</span>
                  <b>拦截 {{ t.blocked }} · 标注 {{ t.flagged }}</b>
                </li>
              </ul>
            </div>
          </div>
        </template>
      </el-tab-pane>

      <el-tab-pane label="软删除留证" name="softdel">
        <!-- 概览：按类型计数（点击即筛选） -->
        <div class="pc-stats">
          <button
            class="pc-stat"
            :class="{ active: !filterType }"
            @click="pickType('')"
          >
            <span class="pc-stat-num">{{ list.length }}</span>
            <span class="pc-stat-label">全部留证</span>
          </button>
          <button
            v-for="t in typeStats"
            :key="t.type"
            class="pc-stat"
            :class="{ active: filterType === t.type }"
            @click="pickType(t.type)"
          >
            <span class="pc-stat-num">{{ t.count }}</span>
            <span class="pc-stat-label">{{ typeLabel(t.type) }}</span>
          </button>
        </div>

        <!-- 筛选条 -->
        <div class="pc-toolbar">
          <el-input
            v-model="keyword"
            placeholder="搜索原因 / 摘要 / 处置人 / 原ID"
            clearable
            style="width: 260px"
            :prefix-icon="Search"
          />
          <el-select v-model="filterType" placeholder="全部类型" clearable style="width: 160px" @change="page = 1">
            <el-option v-for="t in types" :key="t" :label="typeLabel(t)" :value="t" />
          </el-select>
          <el-button type="primary" :loading="loading" @click="loadList">
            <el-icon><Refresh /></el-icon> 刷新
          </el-button>
          <span class="pc-count" v-if="!loading && !loadError">
            共 {{ filtered.length }} 条<template v-if="keyword">（已过滤）</template>
          </span>
          <div class="pc-color">
            <span class="pc-color-label">已删除高亮色</span>
            <el-color-picker :model-value="delColor" @change="onColorChange" :disabled="!canEditColor" />
            <span class="pc-color-hint" v-if="!canEditColor">（仅管理员可修改）</span>
          </div>
        </div>

        <el-table :data="pagedRows" v-loading="loading" stripe style="width: 100%">
          <!-- 表级空态：加载骨架 / 错误重试 / 真空态（列级 #empty 无效，不要重复写） -->
          <template #empty>
            <StateBlock
              :state="loading ? 'loading' : loadError ? 'error' : 'empty'"
              skeleton="table"
              :rows="4"
              compact
              :title="loadError ? '留证列表加载失败' : '暂无留证记录'"
              :description="loadError || '换个筛选条件试试，或等有数据后再来看。'"
              @retry="loadList"
            />
          </template>
          <el-table-column prop="type" label="类型" width="100">
            <template #default="{ row }"><el-tag size="small">{{ typeLabel(row.type) }}</el-tag></template>
          </el-table-column>
          <el-table-column prop="original_id" label="原ID" width="84" />
          <el-table-column prop="operator" label="处置人" width="120" />
          <el-table-column prop="source" label="来源" width="96" />
          <el-table-column prop="reason" label="原因" min-width="170" show-overflow-tooltip>
            <template #default="{ row }"><span :style="{ color: delColor }">{{ row.reason }}</span></template>
          </el-table-column>
          <el-table-column prop="summary" label="摘要" min-width="220" show-overflow-tooltip>
            <template #default="{ row }"><span :style="{ color: delColor }">{{ row.summary }}</span></template>
          </el-table-column>
          <el-table-column prop="deleted_at" label="删除时间" width="165">
            <template #default="{ row }">{{ formatDate(row.deleted_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="180" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="primary" plain @click="openDetail(row)">详情</el-button>
              <el-button size="small" type="success" plain @click="doDownload(row)">
                <el-icon><Download /></el-icon> 凭证
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="pc-pager" v-if="filtered.length > pageSize">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :page-sizes="[20, 50, 100, 200]"
            :total="filtered.length"
            layout="total, sizes, prev, pager, next"
            background
          />
        </div>
      </el-tab-pane>

      <el-tab-pane label="论坛巡查" name="forums" lazy>
        <PoliceForumPatrol />
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="detailVisible" title="取证凭证详情" width="760px">
      <div v-if="detail">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="类型">{{ typeLabel(detail.type) }}</el-descriptions-item>
          <el-descriptions-item label="原ID">{{ detail.original_id }}</el-descriptions-item>
          <el-descriptions-item label="处置人">{{ detail.operator || '—' }}</el-descriptions-item>
          <el-descriptions-item label="来源">{{ detail.source || '—' }}</el-descriptions-item>
          <el-descriptions-item label="原因" :span="2"><span :style="{ color: delColor }">{{ detail.reason || '—' }}</span></el-descriptions-item>
          <el-descriptions-item label="删除时间" :span="2">{{ formatDate(detail.deleted_at) }}</el-descriptions-item>
        </el-descriptions>
        <div class="pc-snapshot-title">完整快照（JSON）</div>
        <pre class="pc-snapshot">{{ prettySnapshot(detail.snapshot) }}</pre>
      </div>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
        <el-button type="success" :loading="dlLoading" @click="detail && doDownload(detail, true)">
          <el-icon><Download /></el-icon> 下载凭证
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import api from '@/api';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import PoliceForumPatrol from './police/PoliceForumPatrol.vue';
import { Stamp, Search, Download, Refresh } from '@element-plus/icons-vue';
import StateBlock from '@/components/common/StateBlock.vue';

const DefaultPoliceDelColor = '#ff4d4f';

interface SoftDelRow {
  id: number;
  type: string;
  original_id: number;
  operator: string;
  source: string;
  reason: string;
  summary: string;
  snapshot: string;
  deleted_at: string;
}

const authStore = useAuthStore();
// 默认落在舆情看板：警方进来先看「舆论有没有失控」，再决定要不要翻留证
const activeTab = ref('sentiment');

// ── 舆情看板（只有聚合计数，不含正文）──
const sent = ref<any>(null);
const trend = ref<Array<{ day: string; blocked: number; flagged: number }>>([]);
const sentLoading = ref(false);
const sentError = ref(false);

const groupNames: Record<string, string> = {
  subculture: '亚文化', gender: '性别议题', politics: '政治批判',
  capital: '资本批判', region: '地域民族', other: '其它争议',
};
function groupLabel(g: string) {
  return groupNames[g] || g;
}

async function loadSentiment() {
  sentLoading.value = true;
  sentError.value = false;
  try {
    const res = await api.get('/police/sentiment/stats', { params: { days: 30 } });
    if (res.data.code === 0 && res.data.data) sent.value = res.data.data.stats;
    const tr = await api.get('/police/sentiment/trend', { params: { days: 14 } });
    if (tr.data.code === 0 && tr.data.data) trend.value = tr.data.data.items || [];
  } catch {
    // 失败必须可见：警方看不到统计会误以为「没有舆情」，
    // 这比报错更危险，所以走错误态而不是静默
    sentError.value = true;
  } finally {
    sentLoading.value = false;
  }
}

function onTabChange(name: string | number) {
  if (name === 'sentiment' && !sent.value) loadSentiment();
}
const list = ref<SoftDelRow[]>([]);
const loading = ref(false);
const loadError = ref('');
const exporting = ref(false);
const dlLoading = ref(false);
const filterType = ref('');
const keyword = ref('');
const page = ref(1);
const pageSize = ref(50);
const detailVisible = ref(false);
const detail = ref<SoftDelRow | null>(null);

// 已删除内容高亮色（警方取证台统一使用）：管理员可改，警方可读不可写
const delColor = ref(DefaultPoliceDelColor);
const canEditColor = computed(() => authStore.user?.role === 'admin');

const types = ['comment', 'novel', 'chapter', 'user', 'thread', 'post', 'blog', 'forum'];

function typeLabel(t: string) {
  const m: Record<string, string> = {
    comment: '评论', novel: '小说', chapter: '章节', user: '用户',
    thread: '帖子', post: '回复', blog: '博客', forum: '论坛',
  };
  return m[t] || t;
}

// 类型计数（概览 chips 用；不受类型筛选影响，始终按全量统计）
const typeStats = computed(() => {
  const counts = new Map<string, number>();
  for (const r of list.value) counts.set(r.type, (counts.get(r.type) || 0) + 1);
  return [...counts.entries()]
    .map(([type, count]) => ({ type, count }))
    .sort((a, b) => b.count - a.count);
});

// 关键词过滤（原因 / 摘要 / 处置人 / 原ID）
const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  if (!kw) return list.value;
  return list.value.filter((r) =>
    [r.reason, r.summary, r.operator, String(r.original_id), typeLabel(r.type)]
      .filter(Boolean)
      .some((v) => String(v).toLowerCase().includes(kw)),
  );
});

const pagedRows = computed(() => {
  const start = (page.value - 1) * pageSize.value;
  return filtered.value.slice(start, start + pageSize.value);
});

function pickType(t: string) {
  filterType.value = t;
  page.value = 1;
  loadList();
}

function formatDate(d?: string) {
  if (!d) return '';
  return new Date(d).toLocaleString('zh-CN');
}

function prettySnapshot(s?: string) {
  if (!s) return '';
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

async function loadList() {
  loading.value = true;
  loadError.value = '';
  try {
    const res = await adminApi.policeListSoftDel({
      type: filterType.value || undefined,
      limit: 1000,
    });
    if (res.data.code === 0) {
      list.value = res.data.data || [];
      page.value = 1;
    } else {
      loadError.value = (res.data as any).message || '加载失败';
    }
  } catch (e: any) {
    loadError.value = e?.response?.data?.message || '加载失败，请检查网络或稍后重试';
    ElMessage.error(loadError.value);
  } finally {
    loading.value = false;
  }
}

function openDetail(row: SoftDelRow) {
  detail.value = row;
  detailVisible.value = true;
}

// 加载已删除内容高亮色（警方可读），失败则用默认色
async function loadDelColor() {
  try {
    const res = await api.get('/police/del-color');
    if (res.data.code === 0 && res.data.data?.color) {
      delColor.value = res.data.data.color;
    }
  } catch {
    /* 保持默认色 */
  }
}

// 管理员修改高亮色：PUT 落库；失败还原
async function onColorChange(color?: string) {
  if (!color) {
    loadDelColor();
    return;
  }
  try {
    await api.put('/police/del-color', { color });
    ElMessage.success('删除内容高亮色已更新');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
    loadDelColor();
  }
}

function triggerDownload(blob: Blob, filename: string) {
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  window.URL.revokeObjectURL(url);
}

async function doDownload(row: SoftDelRow, fromDetail = false) {
  if (fromDetail) dlLoading.value = true;
  try {
    // G2：长路由 + 查询串，避免路径参数
    const res = await api.get('/police/download', {
      params: { type: row.type, id: row.id },
      responseType: 'blob',
    });
    const blob = new Blob([res.data], { type: 'application/json' });
    triggerDownload(blob, `evidence_${row.type}_${row.id}.json`);
    ElMessage.success('凭证已下载');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '下载失败');
  } finally {
    if (fromDetail) dlLoading.value = false;
  }
}

// 全量取证包含大量个人信息与访问日志 → 必须先确认用途，避免误点批量外带
async function doExport() {
  try {
    await ElMessageBox.confirm(
      '即将导出全量取证包（软删除留证 + 隔离墙访问 + Web 访问 + 用户审计）。' +
        '文件包含个人信息与访问日志，仅限依法调取、留档用途，请勿转发或公开传播。',
      '导出取证包确认',
      { confirmButtonText: '确认导出', cancelButtonText: '取消', type: 'warning' },
    );
  } catch {
    return; // 用户取消
  }
  exporting.value = true;
  try {
    const res = await adminApi.policeExportPackage();
    const blob = new Blob([res.data], { type: 'application/json' });
    const ts = new Date().toISOString().replace(/[-:T]/g, '').slice(0, 15);
    triggerDownload(blob, `police_evidence_package_${ts}.json`);
    ElMessage.success('取证包已导出');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '导出失败');
  } finally {
    exporting.value = false;
  }
}

onMounted(() => {
  loadSentiment();
  loadList();
  loadDelColor();
});
</script>

<style scoped>
.admin-page { --admin-content-max: 1280px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }

/* 概览 chips：一眼看出留证都集中在哪类内容 */
.pc-stats {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}
.pc-stat {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-width: 92px;
  padding: 10px 14px;
  border-radius: 10px;
  border: 1px solid var(--border-color, #e4e7ed);
  background: var(--bg-card, #fff);
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
  font: inherit;
  text-align: left;
}
.pc-stat:hover { border-color: var(--primary-color, #4f46e5); }
.pc-stat.active {
  border-color: var(--primary-color, #4f46e5);
  box-shadow: 0 0 0 2px rgba(79, 70, 229, 0.12);
}
.pc-stat-num {
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--text-color, #1f2430);
  line-height: 1.1;
}
.pc-stat-label {
  font-size: 0.74rem;
  color: var(--text-light, #8a92a6);
}

.pc-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; flex-wrap: wrap; }
.pc-color { display: flex; align-items: center; gap: 8px; margin-left: auto; }
.pc-color-label { color: var(--text-light); font-size: 13px; }
.pc-color-hint { color: var(--text-light); font-size: 12px; }
.pc-note {
  margin: 10px 0 4px;
  font-size: 0.78rem;
  color: var(--text-light, #888);
  line-height: 1.6;
}
.pc-panels {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 14px;
  margin-top: 12px;
}
.pc-panel {
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 10px;
  padding: 12px 14px;
  background: var(--bg-card, #fff);
}
.pc-panel h4 {
  margin: 0 0 8px;
  font-size: 0.86rem;
  font-weight: 600;
}
.pc-rank {
  list-style: none;
  margin: 0;
  padding: 0;
  font-size: 0.8rem;
}
.pc-rank li {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  padding: 4px 0;
  border-bottom: 1px dashed var(--border-color, #eee);
}
.pc-rank li:last-child { border-bottom: none; }
.pc-stat--warn .pc-stat-num { color: var(--el-color-warning, #e6a23c); }

.pc-tabs { border: none; background: transparent; }
.pc-tabs :deep(.el-tabs__content) { padding: 16px 0 0; }
.pc-count { color: var(--text-light); font-size: 0.82rem; }
.pc-pager { display: flex; justify-content: flex-end; margin-top: 14px; }
.pc-snapshot-title { margin: 14px 0 6px; font-weight: 600; }
.pc-snapshot {
  max-height: 360px;
  overflow: auto;
  background: #0f172a;
  color: #e2e8f0;
  padding: 12px;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.5;
}
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .pc-stat { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .pc-stat-num { color: #e2e8f0; }
</style>
