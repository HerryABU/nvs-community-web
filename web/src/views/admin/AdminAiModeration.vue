<template>
  <div class="admin-page">
    <AdminHeader
      title="AI 审核"
      subtitle="AI 内容审核中枢：总开关 · 判定模型 · 送检策略 · 处置阈值。所有策略都在本页调整，代码里不写死。"
      :icon="MagicStick"
    />
    <div class="admin-page-body">
      <!-- 概览 -->
      <div class="stat-grid">
        <div class="stat-card stat-card--warn">
          <span class="stat-num">{{ stats.pending_count }}</span>
          <span class="stat-label">待人工处置</span>
        </div>
        <div class="stat-card stat-card--danger">
          <span class="stat-num">{{ stats.fail_count }}</span>
          <span class="stat-label">判定违规</span>
        </div>
        <div class="stat-card">
          <span class="stat-num">{{ stats.review_count }}</span>
          <span class="stat-label">存疑待复核</span>
        </div>
        <div class="stat-card stat-card--ok">
          <span class="stat-num">{{ stats.pass_count }}</span>
          <span class="stat-label">判定合规</span>
        </div>
        <div class="stat-card">
          <span class="stat-num">{{ stats.malicious_reporters }}</span>
          <span class="stat-label">不实举报者</span>
        </div>
        <div class="stat-card">
          <span class="stat-num">{{ stats.banned_reporters }}</span>
          <span class="stat-label">已没收举报权</span>
        </div>
      </div>

      <!-- 总开关 + 模型 -->
      <div class="admin-panel">
        <SectionHeader title="总开关与判定模型" subtitle="关闭后不再送检；已产生的审核记录仍可在「审核」页处置" :icon="'🔌'" gradient="purple" />
        <div class="cfg-rows">
          <div class="cfg-row">
            <span class="cfg-label">启用 AI 审核</span>
            <el-switch v-model="cfg.enabled" />
            <span class="muted small">关闭时「发布送检 / 定时批审 / 举报送检」全部不执行</span>
          </div>
          <div class="cfg-row">
            <span class="cfg-label">判定模型</span>
            <el-select v-model="cfg.model_id" placeholder="自动（第一个启用的对话模型）" clearable style="width: 260px">
              <el-option :value="0" label="自动（第一个启用的对话模型）" />
              <el-option v-for="m in chatModels" :key="m.id" :label="`${m.name}（${m.model_id}）`" :value="m.id" />
            </el-select>
            <span class="muted small">走模型池里的对话模型；审核调用不计费</span>
          </div>
        </div>
      </div>

      <!-- 送检策略 -->
      <div class="admin-panel" style="margin-top: 16px">
        <SectionHeader title="送检策略" subtitle="什么内容、在什么时机送 AI 审核" :icon="'📤'" gradient="blue" />
        <div class="cfg-rows">
          <div class="cfg-row">
            <span class="cfg-label">小说 / 博客</span>
            <el-switch v-model="cfg.novel_blog_on_write" />
            <span class="muted small">发布 / 提交时立即送检</span>
          </div>
          <div class="cfg-row">
            <span class="cfg-label">论坛 / 评论区</span>
            <el-switch v-model="cfg.forum_comment_batch" />
            <span class="muted small">按下面的间隔定时批量送检（举报触发的即时送检不受此开关影响）</span>
          </div>
          <div class="cfg-row">
            <span class="cfg-label">批次间隔</span>
            <el-select v-model="cfg.batch_interval_hours" style="width: 160px">
              <el-option v-for="h in INTERVAL_OPTIONS" :key="h" :label="`每 ${h} 小时`" :value="h" />
            </el-select>
            <el-button :loading="batching" @click="runBatch">立即批审一次</el-button>
            <span class="muted small">{{ batchHint }}</span>
          </div>
        </div>
      </div>

      <!-- 处置策略 -->
      <div class="admin-panel" style="margin-top: 16px">
        <SectionHeader title="处置与阈值" subtitle="违规怎么处置、举报刷到什么程度停止复检、恶意举报怎么限制" :icon="'⚖️'" gradient="amber" />
        <div class="cfg-rows">
          <div class="cfg-row">
            <span class="cfg-label">AI 判违规自动下架</span>
            <el-switch v-model="cfg.auto_hide_on_fail" />
            <span class="muted small warn-text">
              开启后 AI 判定违规会<strong>立即软删并留证</strong>（可在「内容留证」恢复）；拿不准就关掉，让违规记录进「审核」待办由人工处置
            </span>
          </div>
          <div class="cfg-row">
            <span class="cfg-label">不实举报没收举报权</span>
            <el-select v-model="cfg.malicious_threshold" style="width: 160px">
              <el-option v-for="n in MALICIOUS_OPTIONS" :key="n" :label="`累计 ${n} 次`" :value="n" />
            </el-select>
            <span class="muted small">举报内容被判「合规」即计一次不实举报</span>
          </div>
          <div class="cfg-row">
            <span class="cfg-label">重复举报复检阈值</span>
            <el-select v-model="cfg.recheck_report_count" style="width: 160px">
              <el-option v-for="n in RECHECK_OPTIONS" :key="n" :label="`被举报 ${n} 次`" :value="n" />
            </el-select>
            <span class="muted small">同一内容被举报达此数且此前判定通过 → 换中立提示词复检一次</span>
          </div>
          <div class="cfg-row">
            <span class="cfg-label">停止 AI 复检</span>
            <el-select v-model="cfg.stop_review_after" style="width: 160px">
              <el-option v-for="n in STOP_OPTIONS" :key="n" :label="`被举报 ${n} 次后`" :value="n" />
            </el-select>
            <span class="muted small">防止恶意刷举报烧审核成本；停止后内容仍存续</span>
          </div>
          <div class="cfg-row">
            <span class="cfg-label">对举报者隐藏</span>
            <el-switch v-model="cfg.hide_from_reporter" />
            <span class="muted small">内容判定合规却遭反复举报 → 仅对该举报者不可见（其他人照常看，内容不删）</span>
          </div>
        </div>
        <div class="cfg-actions">
          <el-button type="primary" :loading="saving" @click="saveConfig">保存配置</el-button>
          <span class="muted small">保存后立即生效</span>
        </div>
      </div>

      <!-- 恶意举报者 -->
      <div class="admin-panel" style="margin-top: 16px">
        <SectionHeader title="恶意举报者" subtitle="不实举报累计达阈值即自动没收举报功能；可在此人工恢复" :icon="'🚩'" gradient="danger" />
        <el-table :data="reporters" v-loading="loadingReporters" style="width: 100%">
          <el-table-column label="用户 ID" prop="user_id" width="100" />
          <el-table-column label="累计举报" prop="total_reports" width="110" />
          <el-table-column label="不实举报" width="110">
            <template #default="{ row }">
              <el-tag size="small" :type="row.malicious_count > 0 ? 'warning' : 'info'">{{ row.malicious_count }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="举报功能" width="130">
            <template #default="{ row }">
              <el-tag size="small" :type="row.reporting_banned ? 'danger' : 'success'">
                {{ row.reporting_banned ? '已没收' : '正常' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="原因" prop="ban_reason" min-width="200" show-overflow-tooltip />
          <el-table-column label="更新时间" min-width="170">
            <template #default="{ row }">{{ fmtTime(row.updated_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="130" fixed="right">
            <template #default="{ row }">
              <el-button v-if="row.reporting_banned" size="small" text type="primary" @click="unban(row)">恢复举报</el-button>
              <span v-else class="muted small">—</span>
            </template>
          </el-table-column>
          <template #empty>
            <StateBlock state="empty" title="还没有恶意举报记录" description="内容被判合规且举报人累计不实举报达阈值时会出现在这里" />
          </template>
        </el-table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { MagicStick } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import StateBlock from '@/components/common/StateBlock.vue';
import { aiApi, type AdminAiModel } from '@/api/ai';
import { moderationApi, type MaliciousReporter, type ModerationConfig, type ModerationStats } from '@/api/moderation';

// 选择项由后台给出候选，实际生效值一律来自后端配置（不在前端写死策略）
const INTERVAL_OPTIONS = [1, 2, 6, 12, 24];
const MALICIOUS_OPTIONS = [3, 5, 10, 20];
const RECHECK_OPTIONS = [3, 5, 10, 15];
const STOP_OPTIONS = [20, 50, 100];

const cfg = reactive<ModerationConfig>({
  id: 1,
  enabled: false,
  model_id: 0,
  novel_blog_on_write: true,
  forum_comment_batch: true,
  batch_interval_hours: 6,
  auto_hide_on_fail: false,
  malicious_threshold: 5,
  recheck_report_count: 5,
  stop_review_after: 20,
  hide_from_reporter: true,
});

const stats = ref<ModerationStats>({
  total_records: 0, pass_count: 0, fail_count: 0, review_count: 0,
  skipped_count: 0, pending_count: 0, malicious_reporters: 0, banned_reporters: 0, hidden_count: 0,
});
const chatModels = ref<AdminAiModel[]>([]);
const reporters = ref<MaliciousReporter[]>([]);
const loadingReporters = ref(false);
const saving = ref(false);
const batching = ref(false);
const batchHint = ref('');
const loadError = ref('');

function fmtTime(s?: string | null) {
  if (!s) return '—';
  return new Date(s).toLocaleString();
}

async function loadConfig() {
  try {
    const res = await moderationApi.getConfig();
    if (res.data.code === 0 && res.data.data) Object.assign(cfg, res.data.data);
  } catch (e: any) {
    loadError.value = e?.response?.data?.message || '配置读取失败';
  }
}

async function loadStats() {
  try {
    const res = await moderationApi.stats();
    if (res.data.code === 0 && res.data.data) stats.value = res.data.data;
  } catch { /* 统计失败不阻断配置页 */ }
}

async function loadModels() {
  try {
    const res = await aiApi.listModels();
    if (res.data.code === 0) {
      chatModels.value = (res.data.data.list || []).filter((m) => (m.kind || 'chat') === 'chat' && m.enabled);
    }
  } catch { /* 模型列表失败时仍可保存其它配置 */ }
}

async function loadReporters() {
  loadingReporters.value = true;
  try {
    const res = await moderationApi.malicious();
    if (res.data.code === 0) reporters.value = res.data.data || [];
  } catch { /* ignore */ } finally {
    loadingReporters.value = false;
  }
}

async function saveConfig() {
  saving.value = true;
  try {
    const res = await moderationApi.saveConfig({ ...cfg });
    if (res.data.code === 0) {
      ElMessage.success('已保存（立即生效）');
      await loadConfig();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function runBatch() {
  batching.value = true;
  batchHint.value = '';
  try {
    const res = await moderationApi.runBatch();
    if (res.data.code === 0) {
      const n = res.data.data?.reviewed || 0;
      batchHint.value = `本次送检 ${n} 条`;
      ElMessage.success(`批量审核完成，送检 ${n} 条`);
      await loadStats();
    }
  } catch (e: any) {
    batchHint.value = e?.response?.data?.message || '批量审核失败';
    ElMessage.error(batchHint.value);
  } finally {
    batching.value = false;
  }
}

async function unban(row: MaliciousReporter) {
  try {
    await ElMessageBox.confirm(
      `恢复用户 ${row.user_id} 的举报功能？（不实举报计数仍保留）`,
      '恢复举报',
      { type: 'warning', confirmButtonText: '恢复', cancelButtonText: '取消' },
    );
  } catch { return; }
  try {
    const res = await moderationApi.unbanReporter(row.id);
    if (res.data.code === 0) {
      ElMessage.success('已恢复举报功能');
      await loadReporters();
      await loadStats();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

onMounted(() => {
  loadConfig();
  loadStats();
  loadModels();
  loadReporters();
});
</script>

<style scoped>
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; }
.warn-text { color: var(--el-color-warning, #e6a23c); }

.stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin-bottom: 16px; }
.stat-card {
  display: flex; flex-direction: column; gap: 2px;
  padding: 14px 16px; border-radius: 10px;
  background: var(--bg-card, #fff); border: 1px solid var(--border-color, #ebeef5);
  border-left: 4px solid var(--border-color, #dcdfe6);
}
.stat-card--warn { border-left-color: var(--el-color-warning, #e6a23c); }
.stat-card--danger { border-left-color: var(--el-color-danger, #f56c6c); }
.stat-card--ok { border-left-color: var(--el-color-success, #67c23a); }
.stat-num { font-size: 1.5rem; font-weight: 700; color: var(--text-main, #303133); line-height: 1.2; }
.stat-label { font-size: 0.76rem; color: var(--text-light, #909399); }

.cfg-rows { display: flex; flex-direction: column; gap: 14px; margin-top: 12px; }
.cfg-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.cfg-label { width: 170px; flex-shrink: 0; font-size: 0.86rem; color: var(--text-main, #303133); font-weight: 500; }
.cfg-actions { display: flex; align-items: center; gap: 10px; margin-top: 18px; padding-top: 14px; border-top: 1px dashed var(--border-color, #ebeef5); }

[data-theme='dark'] .admin-page { background: #0f172a; }
</style>
