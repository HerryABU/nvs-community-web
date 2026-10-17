<template>
  <div class="admin-page">
    <AdminHeader title="HTML 内容审查" subtitle="自定义 HTML 模板/扩展人工复核" :icon="Cpu" />
    <div class="admin-page-body admin-panel">
      <p class="hint-text">
        对用户上传的自定义 HTML（主题模板 + 扩展应用）进行人工复核。
        解压前预检命中高危的 ZIP 会<b>原样封存、不解压</b>（状态「已封存」），需在此人工判定：
        <b>放行</b>会按封存的 ZIP 补一次解压；<b>驳回</b>可选择连封存件一起删除。
        中/低危告警的扩展已解压但状态为「待复核」，通过后才可公开。
      </p>

      <div class="admin-toolbar">
        <el-radio-group v-model="filterStatus" @change="load">
          <el-radio-button value="">全部</el-radio-button>
          <el-radio-button value="quarantine">已封存</el-radio-button>
          <el-radio-button value="pending">待复核</el-radio-button>
          <el-radio-button value="approved">已通过</el-radio-button>
          <el-radio-button value="rejected">已驳回</el-radio-button>
        </el-radio-group>
        <el-button size="small" @click="load" :loading="loading" :icon="Refresh" style="margin-left:8px">刷新</el-button>
      </div>

      <el-table :data="items" v-loading="loading" style="width:100%;margin-top:12px">
        <el-table-column label="类型" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.type === 'frame' ? 'primary' : 'success'">{{ row.type === 'frame' ? '主题模板' : '扩展应用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip />
        <el-table-column prop="owner_name" label="作者" width="120" show-overflow-tooltip />
        <el-table-column label="运行模式" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.run_mode === 'inline' ? 'danger' : 'info'">{{ row.run_mode === 'inline' ? '非沙盒' : '沙盒' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="告警" width="80" align="center">
          <template #default="{ row }">
            <el-badge v-if="row.warnings?.length" :value="row.warnings.length" type="warning" />
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="审查状态" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="reviewTag(row.review_status)">{{ reviewLabel(row.review_status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="封存" width="80" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.review_status === 'quarantine'" size="small" type="danger" effect="dark">未解压</el-tag>
            <el-tag v-else-if="row.type === 'html' && row.scan_blocked" size="small" type="warning" effect="plain">曾拦截</el-tag>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" text @click="openReview(row)">复核</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <EmptyState icon="📝" text="暂无待审查内容" description="当前筛选条件下没有需要扩展人工复核" />
        </template>
      </el-table>
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
import { ref, onMounted, computed } from 'vue';
import { Refresh, Cpu } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

interface ReviewItem {
  type: string; id: number; user_id: number; owner_name: string;
  name: string; description: string; review_status: string; review_note: string;
  run_mode: string; is_active: boolean; is_public: boolean; warnings: any[];
}

const items = ref<ReviewItem[]>([]);
const loading = ref(false);
const filterStatus = ref('pending');

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

async function load() {
  loading.value = true;
  try {
    const res = await adminApi.getHtmlReviews({ status: filterStatus.value });
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
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 1100px; }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; }
.warning-line { font-size: 0.82rem; line-height: 1.5; margin: 2px 0; }
.meta-row { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.note { font-size: 0.8rem; color: var(--text-light, #666); }
.code-preview { max-height: 320px; overflow: auto; background: #0f172a; border-radius: 8px; padding: 12px; }
.code-preview pre { margin: 0; color: #e2e8f0; font-family: 'SF Mono', 'Fira Code', monospace; font-size: 0.8rem; white-space: pre-wrap; word-break: break-all; line-height: 1.5; }
.muted { color: var(--text-light, #999); }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
</style>
