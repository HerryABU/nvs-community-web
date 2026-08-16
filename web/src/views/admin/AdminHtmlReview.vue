<template>
  <div class="admin-page">
    <div class="admin-page-header">
      <el-button text @click="$router.push('/admin')"><el-icon><ArrowLeft /></el-icon> 返回后台</el-button>
      <h2>HTML 内容审查</h2>
    </div>
    <div class="admin-page-body">
      <p class="hint-text">对用户上传的自定义 HTML（主题模板 + 扩展应用）进行人工复核。自动扫描仅阻断高危代码，中等/低危告警在此人工判定。</p>

      <div class="toolbar">
        <el-radio-group v-model="filterStatus" @change="load">
          <el-radio-button value="">全部</el-radio-button>
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
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" text @click="openReview(row)">复核</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 复核弹窗 -->
    <el-dialog v-model="showReview" :title="`复核：${current?.meta?.name || ''}`" width="80%" destroy-on-close>
      <el-alert v-if="currentWarnings.length > 0" type="warning" :closable="false" style="margin-bottom:12px">
        <template #title>自动扫描告警（{{ currentWarnings.length }} 条）</template>
        <div v-for="(w, i) in currentWarnings" :key="i" class="warning-line">
          [{{ w.category }}/{{ w.severity }}] {{ w.detail }}
        </div>
      </el-alert>
      <el-alert v-else type="success" :closable="false" title="未发现自动扫描告警" style="margin-bottom:12px" />

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
      </el-form>

      <template #footer>
        <el-button @click="showReview = false">取消</el-button>
        <el-button type="danger" :loading="reviewing" @click="submitReview('reject')">驳回并停用</el-button>
        <el-button type="success" :loading="reviewing" @click="submitReview('approve')">通过</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { ArrowLeft, Refresh } from '@element-plus/icons-vue';
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
const reviewing = ref(false);

const currentWarnings = computed(() => current.value?.meta?.warnings || []);

function reviewTag(s: string) {
  const map: Record<string, string> = { pending: 'warning', approved: 'success', rejected: 'danger' };
  return (map[s] as any) || 'info';
}
function reviewLabel(s: string) {
  const map: Record<string, string> = { pending: '待复核', approved: '已通过', rejected: '已驳回' };
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
    });
    if (res.data.code === 0) {
      ElMessage.success(action === 'approve' ? '已通过' : '已驳回并停用');
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
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 1100px; }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 12px; }
.toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.warning-line { font-size: 0.82rem; line-height: 1.5; margin: 2px 0; }
.meta-row { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.note { font-size: 0.8rem; color: var(--text-light, #666); }
.code-preview { max-height: 320px; overflow: auto; background: #0f172a; border-radius: 8px; padding: 12px; }
.code-preview pre { margin: 0; color: #e2e8f0; font-family: 'SF Mono', 'Fira Code', monospace; font-size: 0.8rem; white-space: pre-wrap; word-break: break-all; line-height: 1.5; }
.muted { color: var(--text-light, #999); }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
</style>
