<template>
  <div class="admin-page">
    <AdminHeader title="作者自定义主页审核" subtitle="原文送审 · 消毒预览 · 站长/管理员复核" :icon="Brush" />
    <div class="admin-page-body">
      <p class="hint-text">
        作者提交的公开主页片段会先经服务端<b>自动消毒</b>（剥离脚本 / 危险 CSS），再进入此审核队列。
        若自动化预检发现被清理的内容，<b>原文会一并送站长复核</b>。站长核对原文与消毒版后通过，
        则对外展示<b>消毒后版本</b>（绝不发布原始脚本）。
      </p>

      <!-- 概览统计 -->
      <div class="admin-stat-grid" v-loading="loading">
        <div class="admin-stat-card">
          <div class="admin-stat-icon" style="background: var(--gradient-amber)">⏳</div>
          <div>
            <div class="admin-stat-value">{{ counts.pending }}</div>
            <div class="admin-stat-label">待审核</div>
          </div>
        </div>
        <div class="admin-stat-card">
          <div class="admin-stat-icon" style="background: var(--gradient-danger)">⚠️</div>
          <div>
            <div class="admin-stat-value">{{ counts.issues }}</div>
            <div class="admin-stat-label">预检发现清理项</div>
          </div>
        </div>
        <div class="admin-stat-card">
          <div class="admin-stat-icon" style="background: var(--gradient-success)">✅</div>
          <div>
            <div class="admin-stat-value">{{ counts.approved }}</div>
            <div class="admin-stat-label">已通过</div>
          </div>
        </div>
        <div class="admin-stat-card">
          <div class="admin-stat-icon" style="background: var(--gradient-slate)">🚫</div>
          <div>
            <div class="admin-stat-value">{{ counts.rejected }}</div>
            <div class="admin-stat-label">已驳回</div>
          </div>
        </div>
      </div>

      <div class="admin-panel">
        <div class="admin-toolbar">
          <el-radio-group v-model="statusFilter" size="small" @change="load">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="pending">待审核</el-radio-button>
            <el-radio-button value="approved">已通过</el-radio-button>
            <el-radio-button value="rejected">已驳回</el-radio-button>
          </el-radio-group>
          <el-button size="small" @click="load" :loading="loading" :icon="Refresh" style="margin-left:auto">刷新</el-button>
        </div>

        <el-table :data="rows" v-loading="loading" style="width:100%;margin-top:12px">
          <el-table-column prop="username" label="作者" width="140" show-overflow-tooltip />
          <el-table-column label="预检" width="110">
            <template #default="{ row }">
              <el-tag v-if="row.had_issues" size="small" type="danger" effect="dark">有清理项</el-tag>
              <el-tag v-else size="small" type="success">干净</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag size="small" :type="statusTag(row.review_status)">{{ statusLabel(row.review_status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="更新时间" width="160">
            <template #default="{ row }"><span class="time-text">{{ fmtTime(row.updated_at) }}</span></template>
          </el-table-column>
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="primary" text @click="openReview(row)">复核</el-button>
            </template>
          </el-table-column>
          <template #empty>
            <EmptyState icon="✅" text="暂无待审内容" description="当前筛选条件下没有需要复核的条目" />
          </template>
        </el-table>
      </div>
    </div>

    <!-- 复核弹窗 -->
    <el-dialog v-model="showReview" :title="`复核：${current?.username || ''} 的公开主页`" width="82%" destroy-on-close>
      <el-alert v-if="current?.had_issues" type="warning" :closable="false" show-icon style="margin-bottom:12px">
        <template #title>自动化预检发现需清理项</template>
        下方「原文」中高亮的部分已被服务端消毒移除/改写，原文已单独送你复核。通过后将展示「消毒后预览」版本。
      </el-alert>
      <el-alert v-else type="success" :closable="false" title="自动化预检未发现风险，可酌情通过" style="margin-bottom:12px" />

      <el-row :gutter="16">
        <!-- 原文（送站长复核） -->
        <el-col :xs="24" :md="12">
          <div class="review-subtitle">📄 作者原文（送审内容，供站长复核）</div>
          <div class="code-preview">
            <pre>{{ current?.raw_html || '（空）' }}</pre>
          </div>
        </el-col>
        <!-- 消毒后预览 -->
        <el-col :xs="24" :md="12">
          <div class="review-subtitle">🛡 消毒后预览（通过即对外展示此内容）</div>
          <div class="safe-preview" v-html="current?.safe_html || '<span style=\'color:#999\'>（空）</span>'" />
        </el-col>
      </el-row>

      <el-form label-position="top" style="margin-top:14px">
        <el-form-item label="复核意见（选填）">
          <el-input v-model="reviewNote" type="textarea" :rows="2" maxlength="200" placeholder="驳回时建议填写原因" />
        </el-form-item>
        <el-form-item v-if="current?.review_note" label="上次意见">
          <span class="note">{{ current.review_note }}</span>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showReview = false">取消</el-button>
        <el-button type="danger" :loading="reviewing" @click="submit('reject')">驳回</el-button>
        <el-button type="success" :loading="reviewing" @click="submit('approve')">通过</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { Refresh, Brush } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import EmptyState from '@/components/common/EmptyState.vue';
import { homeCustomizeApi, type HomeCustomizeReviewItem } from '@/api/homeCustomize';
import { ElMessage } from 'element-plus';

const items = ref<HomeCustomizeReviewItem[]>([]);
const loading = ref(false);
const statusFilter = ref('');

// 弹窗
const showReview = ref(false);
const current = ref<HomeCustomizeReviewItem | null>(null);
const reviewNote = ref('');
const reviewing = ref(false);

const counts = computed(() => ({
  pending: items.value.filter((i) => i.review_status === 'pending').length,
  issues: items.value.filter((i) => i.had_issues).length,
  approved: items.value.filter((i) => i.review_status === 'approved').length,
  rejected: items.value.filter((i) => i.review_status === 'rejected').length,
}));

const rows = computed(() => {
  if (!statusFilter.value) return items.value;
  return items.value.filter((i) => i.review_status === statusFilter.value);
});

function statusTag(s: string) {
  return (s === 'approved' ? 'success' : s === 'rejected' ? 'danger' : s === 'pending' ? 'warning' : 'info') as any;
}
function statusLabel(s: string) {
  return (s === 'approved' ? '已通过' : s === 'rejected' ? '已驳回' : s === 'pending' ? '待审核' : s || '未提交') as string;
}
function fmtTime(t: string): string {
  if (!t) return '—';
  const d = new Date(t);
  if (Number.isNaN(d.getTime())) return '—';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

async function load() {
  loading.value = true;
  try {
    const res = await homeCustomizeApi.listReviews(statusFilter.value || undefined);
    if (res.data.code === 0) items.value = res.data.data || [];
  } catch { /* ignore */ }
  loading.value = false;
}

function openReview(row: HomeCustomizeReviewItem) {
  current.value = row;
  reviewNote.value = '';
  showReview.value = true;
}

async function submit(action: 'approve' | 'reject') {
  if (!current.value) return;
  reviewing.value = true;
  try {
    const res = await homeCustomizeApi.review(current.value.author_id, action, reviewNote.value.trim());
    if (res.data.code === 0) {
      ElMessage.success(action === 'approve' ? '已通过，消毒版对外可见' : '已驳回');
      showReview.value = false;
      await load();
    } else {
      ElMessage.error(res.data.message || '操作失败');
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
.time-text { font-size: 12px; color: var(--text-light, #999); font-variant-numeric: tabular-nums; }
.note { font-size: 0.8rem; color: var(--text-light, #666); }
[data-theme="dark"] .admin-page { background: #0f172a; }

.admin-stat-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 16px; }
.admin-stat-card { display: flex; align-items: center; gap: 12px; background: #fff; border-radius: 12px; padding: 16px; box-shadow: 0 1px 4px rgba(0,0,0,0.05); }
[data-theme="dark"] .admin-stat-card { background: #1e293b; }
.admin-stat-icon { width: 44px; height: 44px; border-radius: 10px; display: flex; align-items: center; justify-content: center; font-size: 1.3rem; color: #fff; }
.admin-stat-value { font-size: 1.5rem; font-weight: 700; line-height: 1; }
.admin-stat-label { font-size: 0.78rem; color: var(--text-light); margin-top: 2px; }

.admin-panel { background: #fff; border-radius: 12px; padding: 16px; }
[data-theme="dark"] .admin-panel { background: #1e293b; }
.admin-toolbar { display: flex; align-items: center; gap: 8px; }

.code-preview { max-height: 360px; overflow: auto; background: #0f172a; border-radius: 8px; padding: 12px; }
.code-preview pre { margin: 0; color: #e2e8f0; font-family: 'SF Mono','Fira Code',monospace; font-size: 0.8rem; white-space: pre-wrap; word-break: break-all; line-height: 1.5; }
.safe-preview { max-height: 360px; overflow: auto; border: 1px solid #cbd5e1; border-radius: 8px; padding: 12px; background: #fff; font-size: 0.85rem; }
[data-theme="dark"] .safe-preview { background: #0f172a; border-color: #334155; }
.review-subtitle { font-size: 0.85rem; color: var(--text-light); margin-bottom: 6px; font-weight: 600; }
</style>
