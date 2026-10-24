<template>
  <div class="admin-page">
    <AdminHeader title="HTML / WASM 审核工作台" subtitle="可疑内容人工判定 · 封存件处置 · 审核记录" :icon="Lock" />
    <div class="admin-page-body">
      <p class="hint-text">
        对用户上传的自定义 HTML（主题模板 + 扩展应用）进行人工复核。
        解压前预检命中高危的 ZIP 会<b>原样封存、不解压</b>（状态「已封存」），需在此人工判定：
        <b>放行</b>会按封存的 ZIP 补一次解压；<b>驳回</b>可选择连封存件一起删除。
        中/低危告警的扩展已解压但状态为「待复核」，通过后才可公开。
      </p>

      <!-- 概览统计 -->
      <div class="admin-stat-grid" v-loading="loading">
        <div class="admin-stat-card">
          <div class="admin-stat-icon" style="background: var(--gradient-danger)">🛡</div>
          <div>
            <div class="admin-stat-value">{{ counts.suspicious }}</div>
            <div class="admin-stat-label">可疑待审</div>
          </div>
        </div>
        <div class="admin-stat-card">
          <div class="admin-stat-icon" style="background: var(--gradient-amber)">📦</div>
          <div>
            <div class="admin-stat-value">{{ counts.quarantine }}</div>
            <div class="admin-stat-label">已封存（高危未解压）</div>
          </div>
        </div>
        <div class="admin-stat-card">
          <div class="admin-stat-icon" style="background: var(--gradient-blue)">⚠️</div>
          <div>
            <div class="admin-stat-value">{{ counts.pending }}</div>
            <div class="admin-stat-label">待复核（中低危已解压）</div>
          </div>
        </div>
        <div class="admin-stat-card">
          <div class="admin-stat-icon" style="background: var(--gradient-slate)">📋</div>
          <div>
            <div class="admin-stat-value">{{ counts.history }}</div>
            <div class="admin-stat-label">审核记录</div>
          </div>
        </div>
      </div>

      <div class="admin-panel">
        <el-tabs v-model="activeTab" class="review-tabs">
          <!-- ========== 标签页一：可疑审核 ========== -->
          <el-tab-pane name="suspicious">
            <template #label>
              <span class="tab-label">
                <span class="tab-dot" style="background: var(--gradient-danger)" />
                可疑审核
                <span v-if="counts.suspicious" class="tab-badge tab-badge-danger">{{ counts.suspicious }}</span>
              </span>
            </template>

            <div class="admin-toolbar">
              <el-radio-group v-model="subStatus" size="small">
                <el-radio-button value="">全部可疑</el-radio-button>
                <el-radio-button value="quarantine">已封存</el-radio-button>
                <el-radio-button value="pending">待复核</el-radio-button>
              </el-radio-group>
              <el-radio-group v-model="subType" size="small" style="margin-left:12px">
                <el-radio-button value="">全部类型</el-radio-button>
                <el-radio-button value="frame">主题模板</el-radio-button>
                <el-radio-button value="html">扩展应用</el-radio-button>
              </el-radio-group>
              <el-checkbox v-model="wasmOnly" size="small" style="margin-left:12px">仅 WASM</el-checkbox>
              <el-button size="small" @click="load" :loading="loading" :icon="Refresh" style="margin-left:auto">刷新</el-button>
            </div>

            <el-table :data="suspiciousRows" v-loading="loading" style="width:100%;margin-top:12px">
              <el-table-column label="类型" width="130">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.type === 'frame' ? 'primary' : 'success'">{{ row.type === 'frame' ? '主题模板' : '扩展应用' }}</el-tag>
                  <el-tag v-if="row.allow_wasm" size="small" type="warning" effect="dark" style="margin-left:4px">WASM</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip />
              <el-table-column prop="owner_name" label="作者" width="110" show-overflow-tooltip />
              <el-table-column label="运行模式" width="90">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.run_mode === 'inline' ? 'danger' : 'info'">{{ row.run_mode === 'inline' ? '非沙盒' : '沙盒' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="可疑度" width="100">
                <template #default="{ row }">
                  <el-tag size="small" :type="severityTag(row)" effect="dark">{{ severityLabel(row) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="告警" width="80" align="center">
                <template #default="{ row }">
                  <el-badge v-if="row.warnings?.length" :value="row.warnings.length" type="warning" />
                  <span v-else class="muted">—</span>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag size="small" :type="reviewTag(row.review_status)">{{ reviewLabel(row.review_status) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="提交/更新" width="150">
                <template #default="{ row }">
                  <span class="time-text">{{ fmtTime(row.updated_at) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="90" fixed="right">
                <template #default="{ row }">
                  <el-button size="small" type="primary" text @click="openReview(row)">复核</el-button>
                </template>
              </el-table-column>
              <template #empty>
                <EmptyState icon="✅" text="暂无可疑内容" description="当前筛选条件下没有需要人工判定的条目" />
              </template>
            </el-table>
          </el-tab-pane>

          <!-- ========== 标签页二：审核记录 ========== -->
          <el-tab-pane name="history">
            <template #label>
              <span class="tab-label">
                <span class="tab-dot" style="background: var(--gradient-slate)" />
                审核记录
                <span v-if="counts.history" class="tab-badge tab-badge-info">{{ counts.history }}</span>
              </span>
            </template>

            <div class="admin-toolbar">
              <el-radio-group v-model="histStatus" size="small">
                <el-radio-button value="">全部</el-radio-button>
                <el-radio-button value="approved">已通过</el-radio-button>
                <el-radio-button value="rejected">已驳回</el-radio-button>
              </el-radio-group>
              <el-radio-group v-model="subType" size="small" style="margin-left:12px">
                <el-radio-button value="">全部类型</el-radio-button>
                <el-radio-button value="frame">主题模板</el-radio-button>
                <el-radio-button value="html">扩展应用</el-radio-button>
              </el-radio-group>
            </div>

            <el-table :data="historyRows" v-loading="loading" style="width:100%;margin-top:12px">
              <el-table-column label="类型" width="130">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.type === 'frame' ? 'primary' : 'success'">{{ row.type === 'frame' ? '主题模板' : '扩展应用' }}</el-tag>
                  <el-tag v-if="row.allow_wasm" size="small" type="warning" effect="dark" style="margin-left:4px">WASM</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip />
              <el-table-column prop="owner_name" label="作者" width="110" show-overflow-tooltip />
              <el-table-column label="运行模式" width="90">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.run_mode === 'inline' ? 'danger' : 'info'">{{ row.run_mode === 'inline' ? '非沙盒' : '沙盒' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="结果" width="100">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.review_status === 'approved' ? 'success' : 'danger'">
                    {{ row.review_status === 'approved' ? '通过' : '驳回' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="复核意见" min-width="160" show-overflow-tooltip>
                <template #default="{ row }">
                  <span v-if="row.review_note" class="note">{{ row.review_note }}</span>
                  <span v-else class="muted">—</span>
                </template>
              </el-table-column>
              <el-table-column label="处理时间" width="150">
                <template #default="{ row }">
                  <span class="time-text">{{ fmtTime(row.updated_at) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="90" fixed="right">
                <template #default="{ row }">
                  <el-button size="small" type="primary" text @click="openReview(row)">查看</el-button>
                </template>
              </el-table-column>
              <template #empty>
                <EmptyState icon="📭" text="暂无审核记录" description="已通过 / 已驳回的条目会归档到这里" />
              </template>
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </div>
    </div>

    <!-- 复核弹窗 -->
    <el-dialog v-model="showReview" :title="`复核：${current?.meta?.name || ''}`" width="80%" destroy-on-close>
      <!-- 封存件：先讲清楚为什么被拦、放行会发生什么 -->
      <el-alert
        v-if="isQuarantined"
        type="error"
        :closable="false"
        show-icon
        style="margin-bottom:12px"
      >
        <template #title>安全预检未通过，该扩展尚未解压</template>
        <div class="warning-line">阻断原因：{{ current?.meta?.scan_block_reason || '（未记录）' }}</div>
        <div class="warning-line">封存路径：{{ current?.meta?.quarantine_path || '—' }}</div>
        <div class="warning-line">点「通过」将按封存的 ZIP 补一次解压（会再验一次 ZIP 炸弹）后启用。</div>
      </el-alert>

      <!-- WASM 特别提示：编译后二进制无法静态审计 -->
      <el-alert
        v-if="current?.meta?.allow_wasm"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom:12px"
      >
        <template #title>该条目允许加载 WASM</template>
        <div class="warning-line">WASM 为编译后的二进制，静态扫描无法审计其行为逻辑，请重点人工判定。</div>
      </el-alert>

      <el-alert v-if="currentWarnings.length > 0" type="warning" :closable="false" style="margin-bottom:12px">
        <template #title>自动扫描告警（{{ currentWarnings.length }} 条）</template>
        <div v-for="(w, i) in currentWarnings" :key="i" class="warning-line">
          [{{ w.category }}/{{ w.severity }}] {{ w.detail }}
        </div>
      </el-alert>
      <el-alert v-else-if="!isQuarantined" type="success" :closable="false" title="未发现自动扫描告警" style="margin-bottom:12px" />

      <div class="meta-row">
        <el-tag size="small" :type="current?.meta?.run_mode === 'inline' ? 'danger' : 'info'">
          {{ current?.meta?.run_mode === 'inline' ? '非沙盒注入' : '沙盒模式' }}
        </el-tag>
        <el-tag size="small" v-if="current?.meta?.allow_wasm" type="warning" effect="dark">WASM</el-tag>
        <el-tag size="small" v-if="current?.meta?.is_public" type="success">公开</el-tag>
        <span v-if="current?.meta?.review_note" class="note">上次意见：{{ current.meta.review_note }}</span>
      </div>

      <div class="code-preview">
        <pre>{{ reviewContent }}</pre>
      </div>

      <el-form label-position="top" style="margin-top:12px">
        <el-form-item label="复核意见（选填）">
          <el-input v-model="reviewNote" type="textarea" :rows="2" maxlength="200" placeholder="驳回时建议填写原因，作者可在工作台看到" />
        </el-form-item>
        <el-form-item v-if="isQuarantined">
          <el-checkbox v-model="reviewPublish">通过后立即公布到广场</el-checkbox>
          <el-checkbox v-model="reviewPurge" style="margin-left:16px">驳回时连封存的 ZIP 一并删除</el-checkbox>
        </el-form-item>
        <el-form-item v-else-if="current?.meta?.type === 'html'">
          <el-checkbox v-model="reviewPublish">通过后公布到广场（通过审核才可公开）</el-checkbox>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showReview = false">取消</el-button>
        <el-button type="danger" :loading="reviewing" @click="submitReview('reject')">
          {{ isQuarantined ? '驳回' : '驳回并停用' }}
        </el-button>
        <el-button type="success" :loading="reviewing" @click="submitReview('approve')">
          {{ isQuarantined ? '通过并解压启用' : '通过' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { Refresh, Lock } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';
import StateBlock from '@/components/common/StateBlock.vue';

interface ReviewItem {
  type: string; id: number; user_id: number; owner_name: string;
  name: string; description: string; review_status: string; review_note: string;
  run_mode: string; is_active: boolean; is_public: boolean; warnings: any[];
  allow_wasm: boolean; updated_at: string;
}

// ===== 状态 =====
const items = ref<ReviewItem[]>([]);
const loading = ref(false);
const activeTab = ref('suspicious');
// 可疑页子筛选
const subStatus = ref('');   // ''=全部可疑 / quarantine / pending
const subType = ref('');     // ''=全部 / frame / html
const wasmOnly = ref(false);
// 记录页子筛选
const histStatus = ref('');

// ===== 弹窗 =====
const showReview = ref(false);
const current = ref<any>(null);
const reviewContent = ref('');
const reviewNote = ref('');
const reviewPublish = ref(false);
const reviewPurge = ref(false);
const reviewing = ref(false);

const currentWarnings = computed(() => current.value?.meta?.warnings || []);
// 封存件：预检未通过、尚未解压，放行前需补解压
const isQuarantined = computed(() => current.value?.meta?.review_status === 'quarantine' || current.value?.meta?.is_extracted === false);

// ===== 分组与统计 =====
// 可疑 = quarantine（高危未解压）+ pending（中低危已解压待复核）
const suspiciousItems = computed(() => items.value.filter((i) => i.review_status === 'quarantine' || i.review_status === 'pending'));
const historyItems = computed(() => items.value.filter((i) => i.review_status === 'approved' || i.review_status === 'rejected'));

const counts = computed(() => ({
  suspicious: suspiciousItems.value.length,
  quarantine: items.value.filter((i) => i.review_status === 'quarantine').length,
  pending: items.value.filter((i) => i.review_status === 'pending').length,
  history: historyItems.value.length,
}));

function applyFilters(list: ReviewItem[], status: string): ReviewItem[] {
  return list.filter((i) => {
    if (status && i.review_status !== status) return false;
    if (subType.value && i.type !== subType.value) return false;
    return true;
  });
}

const suspiciousRows = computed(() => {
  let rows = applyFilters(suspiciousItems.value, subStatus.value);
  if (wasmOnly.value) rows = rows.filter((i) => i.allow_wasm);
  return rows;
});
const historyRows = computed(() => applyFilters(historyItems.value, histStatus.value));

// ===== 展示辅助 =====
const SEV_RANK: Record<string, number> = { critical: 4, high: 3, medium: 2, low: 1 };

function topSeverity(row: ReviewItem): string | null {
  let best: string | null = null;
  let bestRank = 0;
  for (const w of row.warnings || []) {
    const rank = SEV_RANK[w.severity] || 0;
    if (rank > bestRank) { bestRank = rank; best = w.severity; }
  }
  return best;
}

// 可疑度：quarantine 无告警时归因于「预检拦截」（解压前就被拦下）
function severityLabel(row: ReviewItem): string {
  const sev = topSeverity(row);
  if (sev === 'critical' || sev === 'high') return '高危';
  if (sev === 'medium') return '中危';
  if (sev === 'low') return '低危';
  if (row.review_status === 'quarantine') return '预检拦截';
  return '—';
}
function severityTag(row: ReviewItem): string {
  const sev = topSeverity(row);
  if (sev === 'critical' || sev === 'high') return 'danger';
  if (sev === 'medium') return 'warning';
  if (sev === 'low') return 'info';
  if (row.review_status === 'quarantine') return 'danger';
  return 'info';
}

function reviewTag(s: string) {
  const map: Record<string, string> = {
    pending: 'warning', approved: 'success', rejected: 'danger', quarantine: 'danger',
  };
  return (map[s] as any) || 'info';
}
function reviewLabel(s: string) {
  const map: Record<string, string> = {
    pending: '待复核', approved: '已通过', rejected: '已驳回', quarantine: '已封存',
  };
  return map[s] || s;
}

function fmtTime(t: string): string {
  if (!t) return '—';
  const d = new Date(t);
  if (Number.isNaN(d.getTime())) return '—';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// ===== 数据加载 =====
// 一次拉全量本地分组：审查列表规模可控，tab 切换与子筛选零延迟、badge 实时
async function load() {
  loading.value = true;
  try {
    const res = await adminApi.getHtmlReviews();
    if (res.data.code === 0) items.value = res.data.data || [];
  } catch { /* ignore */ }
  loading.value = false;
}

async function openReview(row: ReviewItem) {
  try {
    const res = await adminApi.getHtmlReview(row.type, row.id);
    if (res.data.code === 0) {
      current.value = res.data.data;
      reviewContent.value = res.data.data.content || '';
      reviewNote.value = res.data.data.meta?.review_note || '';
      reviewPublish.value = false;
      reviewPurge.value = false;
      showReview.value = true;
    }
  } catch { ElMessage.error('加载内容失败'); }
}

async function submitReview(action: 'approve' | 'reject') {
  if (!current.value) return;
  reviewing.value = true;
  try {
    const res = await adminApi.reviewHtml(current.value.meta.type, current.value.meta.id, {
      action,
      note: reviewNote.value.trim(),
      publish: reviewPublish.value,
      purge_file: reviewPurge.value,
    });
    if (res.data.code === 0) {
      if (action === 'approve') {
        ElMessage.success(isQuarantined.value ? '已通过并解压启用' : '已通过');
      } else {
        ElMessage.success(reviewPurge.value ? '已驳回并删除封存件' : '已驳回并停用');
      }
      showReview.value = false;
      await load();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
  reviewing.value = false;
}

onMounted(load);
</script>

<style scoped>
.admin-page { --admin-content-max: 1180px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: 1180px; }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; line-height: 1.6; }
.warning-line { font-size: 0.82rem; line-height: 1.5; margin: 2px 0; }
.meta-row { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
.note { font-size: 0.8rem; color: var(--text-light, #666); }
.muted { color: var(--text-light, #999); }
.time-text { font-size: 12px; color: var(--text-light, #999); font-variant-numeric: tabular-nums; }
.code-preview { max-height: 320px; overflow: auto; background: #0f172a; border-radius: 8px; padding: 12px; }
.code-preview pre { margin: 0; color: #e2e8f0; font-family: 'SF Mono', 'Fira Code', monospace; font-size: 0.8rem; white-space: pre-wrap; word-break: break-all; line-height: 1.5; }
[data-theme="dark"] .admin-page { background: #0f172a; }

/* 标签页 */
.review-tabs { margin-top: 4px; }
.tab-label { display: inline-flex; align-items: center; gap: 6px; }
.tab-dot { width: 8px; height: 8px; border-radius: 3px; }
.tab-badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 18px; height: 18px; padding: 0 5px;
  border-radius: 9px; font-size: 11px; font-weight: 700; color: #fff;
}
.tab-badge-danger { background: var(--el-color-danger); }
.tab-badge-info { background: var(--el-color-info); }
</style>
