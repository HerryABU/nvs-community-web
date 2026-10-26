<template>
  <div class="admin-page">
    <AdminHeader
      title="舆情守卫"
      subtitle="评论区按话题条件拦截：同一句话在亚文化话题下只标注，在性别议题/政治批判下才拦。标签由 AI 审核敲定，可人工修正。"
      :icon="Lock"
    />
    <div class="admin-page-body">
      <div class="admin-panel">
        <div class="queue-toolbar">
          <el-radio-group v-model="tab" size="small">
            <el-radio-button value="config">策略配置</el-radio-button>
            <el-radio-button value="terms">词表（待审 {{ pendingCount }}）</el-radio-button>
            <el-radio-button value="hits">拦截留证</el-radio-button>
          </el-radio-group>
          <span class="muted small">{{ errorHint }}</span>
        </div>
      </div>

      <!-- ===== 策略配置 ===== -->
      <div v-if="tab === 'config'" class="admin-panel">
        <StateBlock v-if="cfgLoading" state="loading" skeleton="card" :rows="6" />
        <el-form v-else label-width="190px" class="sent-form">
          <el-form-item label="启用评论守卫">
            <el-switch v-model="cfg.enabled" />
            <span class="muted small">关闭后评论区不做任何话题判定</span>
          </el-form-item>
          <el-form-item label="拦截门槛（话题风险）">
            <el-select v-model="cfg.block_risk" style="width: 200px">
              <el-option label="中风险" value="mid" />
              <el-option label="中高风险" value="midhigh" />
              <el-option label="高风险" value="high" />
            </el-select>
            <span class="muted small">达到该风险才允许把命中词升级为「拦截」</span>
          </el-form-item>
          <el-form-item label="标注门槛（话题风险）">
            <el-select v-model="cfg.flag_risk" style="width: 200px">
              <el-option label="低风险" value="low" />
              <el-option label="中风险" value="mid" />
              <el-option label="中高风险" value="midhigh" />
            </el-select>
            <span class="muted small">达到该风险就记录标注（不拦）</span>
          </el-form-item>
          <el-form-item label="自动拦截刷屏出征">
            <el-switch v-model="cfg.flood_block" />
            <span class="muted small">大量 @、短语重复 + 连续标点、叠字喊话</span>
          </el-form-item>
          <el-form-item label="标签置信度门槛">
            <el-input-number v-model="cfg.min_confidence" :min="0" :max="100" />
            <span class="muted small">
              低于该值的 AI 标签**只能标注、不能拦截**（标签错一次会连带误杀整段评论区）
            </span>
          </el-form-item>
          <el-form-item label="长评论折叠字数">
            <el-input-number v-model="cfg.fold_chars" :min="0" :max="5000" />
            <span class="muted small">0 = 不折叠；超出显示「展开全文」</span>
          </el-form-item>

          <el-divider content-position="left">每日词表自动更新</el-divider>
          <el-form-item label="对内：AI 生成候选">
            <el-switch v-model="cfg.learn_from_ai" />
            <span class="muted small">只送「已被标注/拦截」的命中词，不外发普通用户原文</span>
          </el-form-item>
          <el-form-item label="对外：调用 /search 检索">
            <el-switch v-model="cfg.search_enabled" />
            <span class="muted small">检索结果**不入库**，只用于判断；落库的是词 + 标签 + 一句话依据</span>
          </el-form-item>
          <el-form-item label="检索路径">
            <el-input v-model="cfg.search_path" style="width: 260px" placeholder="/search" />
            <span class="muted small">DeepSeek/Kimi 为 /tools/search，Perplexity 为 /search</span>
          </el-form-item>
          <el-form-item label="检索词模板">
            <el-input v-model="cfg.search_query" style="width: 360px" />
            <span class="muted small">{date} 会替换为当月</span>
          </el-form-item>
          <el-form-item label="每日检索预算">
            <el-input-number v-model="cfg.search_budget_per_day" :min="0" :max="500" />
            <span class="muted small">每次检索 + 总结是两次 AI 调用，设上限避免成本失控</span>
          </el-form-item>
          <el-form-item label="检索时间窗（天）">
            <el-input-number v-model="cfg.recency_days" :min="1" :max="365" />
          </el-form-item>
          <el-form-item label="每日入库上限">
            <el-input-number v-model="cfg.daily_quota" :min="0" :max="1000" />
          </el-form-item>
          <el-form-item label="自动学来的词需人工审核">
            <el-switch v-model="cfg.require_review" />
            <span class="muted small">建议开启：词表错了会连带误伤正常表达</span>
          </el-form-item>
          <el-form-item label="每天几点执行">
            <el-input-number v-model="cfg.auto_update_hour" :min="0" :max="23" />
          </el-form-item>

          <el-form-item>
            <el-button type="primary" :loading="saving" @click="saveConfig">保存配置</el-button>
            <el-button :loading="running" @click="runUpdate">立即跑一次</el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- ===== 词表 ===== -->
      <div v-if="tab === 'terms'" class="admin-panel">
        <div class="queue-toolbar">
          <el-radio-group v-model="termStatus" size="small" @change="loadTerms(1)">
            <el-radio-button value="pending">待审</el-radio-button>
            <el-radio-button value="active">生效中</el-radio-button>
            <el-radio-button value="rejected">已否决</el-radio-button>
            <el-radio-button value="">全部</el-radio-button>
          </el-radio-group>
          <el-input
            v-model="termKeyword"
            size="small"
            placeholder="搜词"
            style="width: 160px"
            clearable
            @keyup.enter="loadTerms(1)"
          />
          <el-button size="small" @click="openCreate">新增词条</el-button>
          <span class="muted small">共 {{ termTotal }} 条</span>
        </div>

        <StateBlock v-if="termLoading" state="loading" skeleton="table" :rows="6" />
        <StateBlock
          v-else-if="termError"
          state="error"
          title="加载失败"
          description="请检查网络或稍后重试"
          action-text="重试"
          @action="loadTerms(1)"
        />
        <el-table v-else :data="terms" size="small">
          <el-table-column label="词条" min-width="140">
            <template #default="{ row }">
              <span class="term-word">{{ row.word }}</span>
              <el-tag v-if="row.source === 'ai'" size="small" type="warning" class="ml4">AI</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="类别" width="100">
            <template #default="{ row }">{{ catLabel(row.cat) }}</template>
          </el-table-column>
          <el-table-column label="绑定话题" min-width="160">
            <template #default="{ row }">
              <el-tag v-for="g in groupsOf(row.escalate_under)" :key="g" size="small" class="mr4">
                {{ groupLabel(g) }}
              </el-tag>
              <span v-if="!groupsOf(row.escalate_under).length" class="muted small">不限（按风险升档）</span>
            </template>
          </el-table-column>
          <el-table-column label="词级" width="70" prop="severity" />
          <el-table-column label="置信度" width="90">
            <template #default="{ row }">{{ row.confidence }}</template>
          </el-table-column>
          <el-table-column label="命中" width="70" prop="hit_count" />
          <el-table-column label="操作" width="210">
            <template #default="{ row }">
              <el-button v-if="row.status === 'pending'" size="small" text type="primary" @click="review(row, 'approve')">
                通过
              </el-button>
              <el-button v-if="row.status === 'pending'" size="small" text type="danger" @click="review(row, 'reject')">
                否决
              </el-button>
              <el-button size="small" text @click="openEdit(row)">编辑</el-button>
              <el-button size="small" text type="danger" @click="removeTerm(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty>
            <StateBlock
              state="empty"
              title="没有词条"
              :description="termStatus === 'pending' ? 'AI 学来的候选会出现在这里，待人确认' : '可以在上方新增，或等待每日自动更新'"
            />
          </template>
        </el-table>
        <el-pagination
          v-if="termTotal > termSize"
          class="sent-pager"
          layout="prev, pager, next"
          :total="termTotal"
          :page-size="termSize"
          :current-page="termPage"
          @current-change="loadTerms"
        />
      </div>

      <!-- ===== 拦截留证 ===== -->
      <div v-if="tab === 'hits'" class="admin-panel">
        <div class="queue-toolbar">
          <el-radio-group v-model="hitDecision" size="small" @change="loadHits(1)">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="block">已拦截</el-radio-button>
            <el-radio-button value="flag">仅标注</el-radio-button>
          </el-radio-group>
          <span class="muted small">
            只给判定摘要，不含评论正文（正文按评论自身的权限与脱敏口径查看）
          </span>
        </div>
        <StateBlock v-if="hitLoading" state="loading" skeleton="table" :rows="6" />
        <StateBlock v-else-if="hitError" state="error" title="加载失败" action-text="重试" @action="loadHits(1)" />
        <el-table v-else :data="hits" size="small">
          <el-table-column label="时间" width="160">
            <template #default="{ row }">{{ fmt(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="处置" width="90">
            <template #default="{ row }">
              <el-tag :type="row.decision === 'block' ? 'danger' : 'warning'" size="small">
                {{ row.decision === 'block' ? '拦截' : '标注' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="话题" min-width="120">
            <template #default="{ row }">{{ groupsText(row.groups) }}</template>
          </el-table-column>
          <el-table-column label="风险" width="80" prop="risk" />
          <el-table-column label="命中词" min-width="150">
            <template #default="{ row }">
              {{ row.matched_words || '—' }}
              <el-tag v-if="row.variant_words" size="small" type="info" class="ml4">变形</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="用户" width="120">
            <template #default="{ row }">{{ row.username || '#' + row.user_id }}</template>
          </el-table-column>
          <el-table-column label="理由" min-width="220" prop="reason" />
          <template #empty>
            <StateBlock state="empty" title="暂无记录" description="评论区命中词条后会在这里留证" />
          </template>
        </el-table>
        <el-pagination
          v-if="hitTotal > hitSize"
          class="sent-pager"
          layout="prev, pager, next"
          :total="hitTotal"
          :page-size="hitSize"
          :current-page="hitPage"
          @current-change="loadHits"
        />
      </div>
    </div>

    <!-- 词条编辑 -->
    <el-dialog v-model="dialog" :title="editing.id ? '编辑词条' : '新增词条'" width="520px">
      <el-form label-width="110px">
        <el-form-item label="词条">
          <el-input v-model="editing.word" :disabled="!!editing.id" placeholder="要匹配的词" />
        </el-form-item>
        <el-form-item label="类别">
          <el-select v-model="editing.cat" style="width: 100%">
            <el-option label="辱骂" value="abuse" />
            <el-option label="刷屏出征" value="raid" />
            <el-option label="圈层黑话" value="slang" />
            <el-option label="群体蔑称" value="bigotry" />
          </el-select>
        </el-form-item>
        <el-form-item label="绑定话题">
          <el-select v-model="editing.groups" multiple style="width: 100%" placeholder="不选=只按风险升档">
            <el-option v-for="g in allGroups" :key="g" :label="groupLabel(g)" :value="g" />
          </el-select>
          <span class="muted small">只有命中这些话题时，该词才会从「标注」升级为「拦截」</span>
        </el-form-item>
        <el-form-item label="词级 1-3">
          <el-input-number v-model="editing.severity" :min="1" :max="3" />
          <span class="muted small">1=黑话（永不升级拦截） 2-3=可能升级</span>
        </el-form-item>
        <el-form-item label="强制拦截">
          <el-switch v-model="editing.force" />
          <span class="muted small">无视话题直接拦（慎用，等于全局黑名单）</span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="editing.note" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="savingTerm" @click="submitTerm">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Lock } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import StateBlock from '@/components/common/StateBlock.vue';
import { sentimentApi, type SentimentConfig, type SentimentHit, type SentimentTerm } from '@/api/sentiment';

const tab = ref<'config' | 'terms' | 'hits'>('config');
const errorHint = ref('');

const cfg = reactive<SentimentConfig>({
  enabled: true,
  block_risk: 'midhigh',
  flag_risk: 'mid',
  flood_block: true,
  min_confidence: 60,
  learn_from_ai: false,
  search_enabled: false,
  search_model_id: 0,
  search_path: '/search',
  search_query: '',
  search_budget_per_day: 20,
  recency_days: 30,
  auto_update_hour: 4,
  daily_quota: 50,
  require_review: true,
  fold_chars: 400,
});
const cfgLoading = ref(false);
const saving = ref(false);
const running = ref(false);

// ── 词表 ──
const terms = ref<SentimentTerm[]>([]);
const termLoading = ref(false);
const termError = ref(false);
const termTotal = ref(0);
const termPage = ref(1);
const termSize = 30;
const termStatus = ref('pending');
const termKeyword = ref('');
const pendingCount = ref(0);

// ── 留证 ──
const hits = ref<SentimentHit[]>([]);
const hitLoading = ref(false);
const hitError = ref(false);
const hitTotal = ref(0);
const hitPage = ref(1);
const hitSize = 30;
const hitDecision = ref('');

const allGroups = ['subculture', 'gender', 'politics', 'capital', 'region', 'other'];

const groupNames: Record<string, string> = {
  subculture: '亚文化',
  gender: '性别议题',
  politics: '政治批判',
  capital: '资本批判',
  region: '地域民族',
  other: '其它争议',
};
function groupLabel(g: string) {
  return groupNames[g] || g;
}
function groupsOf(raw: string): string[] {
  if (!raw) return [];
  if (raw.trim().startsWith('[')) {
    try {
      return (JSON.parse(raw) as string[]).filter(Boolean);
    } catch {
      return [];
    }
  }
  return raw.split(',').map((s) => s.trim()).filter(Boolean);
}
function groupsText(raw: string) {
  const gs = groupsOf(raw);
  return gs.length ? gs.map(groupLabel).join('+') : '—';
}
function catLabel(c: string) {
  return { abuse: '辱骂', raid: '刷屏出征', slang: '圈层黑话', bigotry: '群体蔑称' }[c] || c;
}
function fmt(s: string) {
  if (!s) return '—';
  const d = new Date(s);
  return isNaN(d.getTime()) ? s : d.toLocaleString('zh-CN');
}

async function loadConfig() {
  cfgLoading.value = true;
  errorHint.value = '';
  try {
    const res = await sentimentApi.getConfig();
    if (res.data.code === 0 && res.data.data?.config) Object.assign(cfg, res.data.data.config);
  } catch {
    errorHint.value = '配置读取失败，请刷新重试';
  } finally {
    cfgLoading.value = false;
  }
}

async function saveConfig() {
  saving.value = true;
  try {
    await sentimentApi.saveConfig({ ...cfg });
    ElMessage.success('已保存');
    loadConfig();
  } catch {
    ElMessage.error('保存失败');
  } finally {
    saving.value = false;
  }
}

async function runUpdate() {
  running.value = true;
  try {
    const res = await sentimentApi.runUpdate();
    const r = res.data.data?.result;
    ElMessage.success(
      r ? `已执行：候选 ${r.learned} / 检索 ${r.searched} / 入库 ${r.inserted}（待审 ${r.pending}）` : '已触发',
    );
    loadTerms(1);
  } catch {
    ElMessage.error('执行失败（检查自动更新开关与模型配置）');
  } finally {
    running.value = false;
  }
}

async function loadTerms(page = 1) {
  termPage.value = page;
  termLoading.value = true;
  termError.value = false;
  try {
    const res = await sentimentApi.terms({
      status: termStatus.value,
      keyword: termKeyword.value || undefined,
      page,
      page_size: termSize,
    });
    if (res.data.code === 0 && res.data.data) {
      terms.value = res.data.data.items || [];
      termTotal.value = res.data.data.total || 0;
      if (termStatus.value === 'pending') pendingCount.value = termTotal.value;
    }
  } catch {
    termError.value = true;
  } finally {
    termLoading.value = false;
  }
}

async function loadPendingCount() {
  try {
    const res = await sentimentApi.terms({ status: 'pending', page: 1, page_size: 1 });
    if (res.data.code === 0 && res.data.data) pendingCount.value = res.data.data.total || 0;
  } catch {
    pendingCount.value = 0;
  }
}

async function loadHits(page = 1) {
  hitPage.value = page;
  hitLoading.value = true;
  hitError.value = false;
  try {
    const res = await sentimentApi.hits({
      decision: hitDecision.value || undefined,
      page,
      page_size: hitSize,
    });
    if (res.data.code === 0 && res.data.data) {
      hits.value = res.data.data.items || [];
      hitTotal.value = res.data.data.total || 0;
    }
  } catch {
    hitError.value = true;
  } finally {
    hitLoading.value = false;
  }
}

// ── 词条编辑 ──
const dialog = ref(false);
const savingTerm = ref(false);
const editing = reactive<{
  id: number;
  word: string;
  cat: string;
  groups: string[];
  severity: number;
  force: boolean;
  note: string;
}>({ id: 0, word: '', cat: 'slang', groups: [], severity: 1, force: false, note: '' });

function openCreate() {
  Object.assign(editing, { id: 0, word: '', cat: 'slang', groups: [], severity: 1, force: false, note: '' });
  dialog.value = true;
}

function openEdit(row: SentimentTerm) {
  Object.assign(editing, {
    id: row.id,
    word: row.word,
    cat: row.cat,
    groups: groupsOf(row.escalate_under),
    severity: row.severity,
    force: row.action_override === 'block',
    note: row.note || '',
  });
  dialog.value = true;
}

async function submitTerm() {
  if (!editing.word.trim()) {
    ElMessage.warning('请填写词条');
    return;
  }
  savingTerm.value = true;
  const payload = {
    word: editing.word.trim(),
    cat: editing.cat,
    escalate_under: editing.groups,
    severity: editing.severity,
    action_override: editing.force ? 'block' : '',
    note: editing.note,
  };
  try {
    if (editing.id) await sentimentApi.updateTerm(editing.id, payload);
    else await sentimentApi.createTerm(payload);
    ElMessage.success('已保存');
    dialog.value = false;
    loadTerms(termPage.value);
    loadPendingCount();
  } catch {
    ElMessage.error('保存失败（可能该词已存在）');
  } finally {
    savingTerm.value = false;
  }
}

async function review(row: SentimentTerm, action: 'approve' | 'reject') {
  try {
    await ElMessageBox.confirm(
      action === 'approve'
        ? `确认启用「${row.word}」？启用后它会参与评论区判定。`
        : `否决「${row.word}」？记录会保留但不再生效。`,
      action === 'approve' ? '通过词条' : '否决词条',
      { confirmButtonText: action === 'approve' ? '通过' : '否决', cancelButtonText: '取消', type: 'warning' },
    );
  } catch {
    return;
  }
  try {
    await sentimentApi.reviewTerm(row.id, action);
    ElMessage.success('已处理');
    loadTerms(termPage.value);
    loadPendingCount();
  } catch {
    ElMessage.error('操作失败');
  }
}

async function removeTerm(row: SentimentTerm) {
  try {
    await ElMessageBox.confirm(`删除「${row.word}」？建议用「否决」保留审计记录。`, '删除词条', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    });
  } catch {
    return;
  }
  try {
    await sentimentApi.deleteTerm(row.id);
    ElMessage.success('已删除');
    loadTerms(termPage.value);
  } catch {
    ElMessage.error('删除失败');
  }
}

const currentTabLabel = computed(() => ({ config: '策略配置', terms: '词表', hits: '拦截留证' }[tab.value]));

onMounted(() => {
  loadConfig();
  loadPendingCount();
  loadTerms(1);
  loadHits(1);
});
</script>

<style scoped>
.queue-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.sent-form :deep(.el-form-item) {
  margin-bottom: 14px;
}
.sent-pager {
  margin-top: 12px;
  justify-content: flex-end;
}
.term-word {
  font-weight: 600;
}
.ml4 {
  margin-left: 4px;
}
.mr4 {
  margin-right: 4px;
}
.muted {
  color: var(--text-light, #999);
}
.small {
  font-size: 0.76rem;
  margin-left: 8px;
}
</style>
