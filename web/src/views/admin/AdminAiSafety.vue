<template>
  <div class="admin-page">
    <AdminHeader
      title="AI 防越狱"
      subtitle="三层拦截：规则引擎（快、宁枉勿纵）→ 蓝队模型 B（批次判定、双向裁定）→ 用户特征（观察期 / 冷却）"
      :icon="Lock"
    />
    <div class="admin-page-body">
      <p class="hint-text">
        AI 对话此前<b>没有任何审核</b>，真实站点上模型答过 TATP 合成条件、冰毒合成路线。
        本页的三层结构分工不同：<b>规则</b>负责兜住明显的（词表 + 信号组合，毫秒级）；
        <b>蓝队模型 B</b> 每 N 轮把上下文送审一次，既补规则的漏（ADRO 逐轮推进），
        也<b>推翻规则的误判</b>（写小说查资料被误拦时放行）；<b>用户特征</b>让反复试探的账号自动收紧。
        所有阈值都在这里配，代码里没有写死的策略值。
      </p>

      <!-- 工具条 -->
      <div class="safety-toolbar">
        <div class="safety-toggle">
          <span>总开关</span>
          <el-switch v-model="cfg.enabled" />
          <span class="muted small">{{ cfg.enabled ? '已启用' : '已关闭（对话不做任何检查）' }}</span>
        </div>
        <div class="safety-actions">
          <el-button size="small" :loading="loading" @click="loadAll">刷新</el-button>
          <el-button size="small" @click="onReset">恢复默认</el-button>
          <el-button size="small" type="primary" :loading="saving" @click="onSave">保存配置</el-button>
        </div>
      </div>

      <!-- 概览 -->
      <div class="safety-stats">
        <div v-for="s in statCards" :key="s.label" class="safety-stat">
          <div class="safety-stat-num">{{ s.value }}</div>
          <div class="safety-stat-label">{{ s.label }}</div>
        </div>
      </div>

      <el-tabs v-model="tab" class="safety-tabs">
        <!-- ========== 策略配置 ========== -->
        <el-tab-pane label="策略配置" name="config">
          <div class="admin-panel">
            <SectionHeader title="基础拦截" subtitle="输入侧直接拒答（不调上游、不扣费）；输出侧先缓冲再放行" :icon="'🛡️'" gradient="danger" />
            <el-form label-width="170px" class="safety-form">
              <el-form-item label="输入侧拦截">
                <el-switch v-model="cfg.guard_input" />
                <span class="muted small">扫描用户提问（含近几轮上下文）</span>
              </el-form-item>
              <el-form-item label="输出侧拦截">
                <el-switch v-model="cfg.guard_output" />
                <span class="muted small">模型回答边生成边扫描，命中即整段替换</span>
              </el-form-item>
              <el-form-item label="输入硬拦分">
                <el-input-number v-model="cfg.input_block_score" :min="1" :max="200" />
                <span class="muted small">物质 + 工艺 + 参数齐全约为 120</span>
              </el-form-item>
              <el-form-item label="输出硬拦分">
                <el-input-number v-model="cfg.output_block_score" :min="1" :max="200" />
                <span class="muted small">输出侧更严：宁可整段替换</span>
              </el-form-item>
              <el-form-item label="可疑线（只记录）">
                <el-input-number v-model="cfg.review_score" :min="0" :max="200" />
              </el-form-item>
              <el-form-item label="输出缓冲字数">
                <el-input-number v-model="cfg.prebuffer_chars" :min="40" :max="2000" />
                <span class="muted small">攒够这么多字、扫过一遍才开始放行；越大越安全、首字越慢</span>
              </el-form-item>
              <el-form-item label="每次放行字数">
                <el-input-number v-model="cfg.flush_chars" :min="20" :max="500" />
              </el-form-item>
              <el-form-item label="保护系统提示词">
                <el-switch v-model="cfg.protect_system" />
                <span class="muted small">把安全准则前置到人设之前（智能体人设压不过它）</span>
              </el-form-item>
            </el-form>
          </div>

          <div class="admin-panel">
            <SectionHeader title="用户特征" subtitle="命中会累计升级：观察期收紧阈值 → 冷却暂停功能" :icon="'👤'" gradient="amber" />
            <el-form label-width="170px" class="safety-form">
              <el-form-item label="几次命中进观察期">
                <el-input-number v-model="cfg.watch_after_hits" :min="1" :max="20" />
              </el-form-item>
              <el-form-item label="观察期时长（小时）">
                <el-input-number v-model="cfg.watch_hours" :min="1" :max="720" />
              </el-form-item>
              <el-form-item label="观察期内放宽到">
                <el-input-number v-model="cfg.watch_block_score" :min="1" :max="200" />
                <span class="muted small">观察期用更低的硬拦分</span>
              </el-form-item>
              <el-form-item label="观察期可疑线">
                <el-input-number v-model="cfg.watch_review_score" :min="0" :max="200" />
              </el-form-item>
              <el-form-item label="再命中几次冷却">
                <el-input-number v-model="cfg.cooldown_hits" :min="1" :max="50" />
              </el-form-item>
              <el-form-item label="冷却时长（分钟）">
                <el-input-number v-model="cfg.cooldown_minutes" :min="1" :max="10080" />
              </el-form-item>
              <el-form-item label="风险分半衰期（小时）">
                <el-input-number v-model="cfg.score_decay_hours" :min="0" :max="8760" />
                <span class="muted small">0 = 不衰减；衰减太快等于没有记忆</span>
              </el-form-item>
              <el-form-item label="创作语境减免">
                <el-switch v-model="cfg.allow_fiction_relief" />
                <span class="muted small">有「小说/剧情」等词时减分；<b>出现参数即失效</b></span>
              </el-form-item>
              <el-form-item label="危害科普减免">
                <el-switch v-model="cfg.allow_harm_relief" />
                <span class="muted small">讲危害/违法/举报时减分</span>
              </el-form-item>
            </el-form>
          </div>

          <div class="admin-panel">
            <SectionHeader
              title="蓝队模型 B"
              subtitle="每 N 轮批次送审一次干净上下文；既补漏也纠错（避免误伤）"
              :icon="'🧪'"
              gradient="purple"
            />
            <el-form label-width="170px" class="safety-form">
              <el-form-item label="启用批次判定">
                <el-switch v-model="cfg.judge_enabled" />
              </el-form-item>
              <el-form-item label="判定模型">
                <el-select v-model="cfg.judge_model_id" placeholder="自动选一个启用的对话模型" clearable style="width:320px">
                  <el-option :value="0" label="自动（优先非思考型，更便宜更快）" />
                  <el-option v-for="m in chatModels" :key="m.id" :value="m.id" :label="`${m.name}（#${m.id}）`" />
                </el-select>
                <span class="muted small">可以就是主模型本身</span>
              </el-form-item>
              <el-form-item label="每几轮送审">
                <el-input-number v-model="cfg.judge_every_turns" :min="1" :max="50" />
                <span class="muted small">批次制：同一批次内只判一次，不会每轮都花一次调用</span>
              </el-form-item>
              <el-form-item label="可疑时提前触发">
                <el-switch v-model="cfg.judge_on_suspect" />
                <span class="muted small">批次内出现规则可疑，用掉本次名额提前判</span>
              </el-form-item>
              <el-form-item label="规则命中先复核">
                <el-switch v-model="cfg.judge_review_blocks" />
                <span class="muted small"><b>避免误伤的主开关</b>：规则要拦的先问 B，B 判安全就放行</span>
              </el-form-item>
              <el-form-item label="B 判越狱拦截分">
                <el-input-number v-model="cfg.judge_block_score" :min="1" :max="100" />
              </el-form-item>
              <el-form-item label="B 判安全放行分">
                <el-input-number v-model="cfg.judge_release_score" :min="0" :max="100" />
                <span class="muted small">分数 ≤ 此值且未判越狱 → 放行（救回规则的误判）</span>
              </el-form-item>
              <el-form-item label="送审上下文轮数">
                <el-input-number v-model="cfg.judge_max_turns" :min="1" :max="50" />
              </el-form-item>
              <el-form-item label="送审文本上限">
                <el-input-number v-model="cfg.judge_max_chars" :min="500" :max="20000" />
                <span class="muted small">字符，省 token</span>
              </el-form-item>
              <el-form-item label="判定超时（秒）">
                <el-input-number v-model="cfg.judge_timeout_sec" :min="3" :max="120" />
              </el-form-item>
              <el-form-item label="失败时放行">
                <el-switch v-model="cfg.judge_fail_open" />
                <span class="muted small">
                  开=B 挂了不阻断对话（但账号进观察期）；关=直接拒答。<br />
                  注意：规则已命中时无论如何都维持拦截，否则打挂 B 就能绕过
                </span>
              </el-form-item>
              <el-form-item label="判定提示词">
                <el-input v-model="cfg.judge_prompt" type="textarea" :rows="10" spellcheck="false" />
              </el-form-item>
            </el-form>
          </div>

          <div class="admin-panel">
            <SectionHeader title="文案与自定义词表" subtitle="拒绝文案要稳定、不复述问题、不给参数" :icon="'💬'" gradient="blue" />
            <el-form label-width="170px" class="safety-form">
              <el-form-item label="安全系统提示词">
                <el-input v-model="cfg.system_guard_prompt" type="textarea" :rows="10" spellcheck="false" />
              </el-form-item>
              <el-form-item label="危险内容拒答">
                <el-input v-model="cfg.refusal_text" type="textarea" :rows="5" />
                <span class="muted small">命中管制物质/工艺时用（可给出替代方向）</span>
              </el-form-item>
              <el-form-item label="越狱拒答文案">
                <el-input v-model="cfg.refusal_jailbreak_text" type="textarea" :rows="3" />
                <span class="muted small">仅「忽略以上指令」这类话术时用；否则回危险品文案会很荒谬</span>
              </el-form-item>
              <el-form-item label="冷却文案">
                <el-input v-model="cfg.cooldown_text" type="textarea" :rows="3" />
                <span class="muted small">用 %s 占位恢复时间；不写 %s 会自动追加</span>
              </el-form-item>
              <el-form-item label="观察期提示">
                <el-input v-model="cfg.watch_notice_text" type="textarea" :rows="2" />
              </el-form-item>
              <el-form-item label="额外禁止词">
                <el-input
                  v-model="cfg.extra_terms"
                  type="textarea"
                  :rows="4"
                  placeholder="每行一个，或用逗号分隔。命中按管制物质级别计权"
                />
              </el-form-item>
              <el-form-item label="留存触发片段">
                <el-switch v-model="cfg.record_excerpt" />
                <el-input-number v-model="cfg.excerpt_chars" :min="40" :max="2000" style="margin-left:8px" />
                <span class="muted small">字符；留证据供人工复核，也涉及用户隐私，按站规取舍</span>
              </el-form-item>
            </el-form>
          </div>
        </el-tab-pane>

        <!-- ========== 拦截记录 ========== -->
        <el-tab-pane label="拦截记录" name="events">
          <div class="queue-toolbar">
            <el-select v-model="evFilter.direction" placeholder="全部方向" clearable size="small" style="width:130px" @change="loadEvents(1)">
              <el-option label="仅输入" value="in" />
              <el-option label="仅输出" value="out" />
            </el-select>
            <el-select v-model="evFilter.action" placeholder="全部动作" clearable size="small" style="width:140px" @change="loadEvents(1)">
              <el-option label="拦截" value="block" />
              <el-option label="观察期" value="watch" />
              <el-option label="冷却" value="cooldown" />
              <el-option label="仅记录" value="review" />
            </el-select>
            <span class="muted small">共 {{ evTotal }} 条</span>
          </div>
          <StateBlock v-if="evError" state="error" title="加载失败" @retry="loadEvents(evPage)" />
          <el-table v-else v-loading="evLoading" :data="events" size="small">
            <el-table-column label="时间" width="150">
              <template #default="{ row }">{{ fmt(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="用户" width="140">
              <template #default="{ row }">
                {{ row.username || '—' }}<span class="muted small"> #{{ row.user_id }}</span>
              </template>
            </el-table-column>
            <el-table-column label="方向" width="80">
              <template #default="{ row }">
                <el-tag size="small" :type="row.direction === 'out' ? 'warning' : 'info'">
                  {{ row.direction === 'out' ? '输出' : '输入' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="处置" width="90">
              <template #default="{ row }">
                <el-tag size="small" :type="tagType(row.action)">{{ actionText(row.action) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="分" width="60" prop="score" />
            <el-table-column label="命中" min-width="200">
              <template #default="{ row }">
                <span class="small">{{ row.categories || '—' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="片段" min-width="240">
              <template #default="{ row }">
                <span class="excerpt">{{ row.excerpt || '（未留存）' }}</span>
              </template>
            </el-table-column>
            <template #empty>
              <StateBlock state="empty" title="还没有拦截记录" description="AI 对话触发安全规则后会出现在这里" />
            </template>
          </el-table>
          <el-pagination
            v-if="evTotal > evSize"
            class="safety-pager"
            layout="prev, pager, next"
            :total="evTotal"
            :page-size="evSize"
            :current-page="evPage"
            @current-change="loadEvents"
          />
        </el-tab-pane>

        <!-- ========== 蓝队判定 ========== -->
        <el-tab-pane label="蓝队判定" name="judge">
          <div class="queue-toolbar">
            <el-checkbox v-model="judgeOnlyBad" size="small" @change="loadJudge(1)">只看判为越狱</el-checkbox>
            <span class="muted small">
              共 {{ judgeTotal }} 次；判越狱 {{ stats.jailbroken }} 次，其中救回误伤 {{ stats.judge_released }} 次
            </span>
          </div>
          <StateBlock v-if="judgeError" state="error" title="加载失败" @retry="loadJudge(judgePage)" />
          <el-table v-else v-loading="judgeLoading" :data="judgeLogs" size="small">
            <el-table-column label="时间" width="150">
              <template #default="{ row }">{{ fmt(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="用户" width="130">
              <template #default="{ row }">{{ row.username || '—' }}</template>
            </el-table-column>
            <el-table-column label="轮次" width="70" prop="turn" />
            <el-table-column label="触发" width="100">
              <template #default="{ row }">
                <span class="small">{{ triggerText(row.trigger) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="结论" width="120">
              <template #default="{ row }">
                <el-tag v-if="row.failed" size="small" type="info">失败</el-tag>
                <el-tag v-else-if="row.jailbroken" size="small" type="danger">越狱 {{ row.score }}</el-tag>
                <el-tag v-else size="small" type="success">安全 {{ row.score }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="处置" width="110">
              <template #default="{ row }">
                <el-tag v-if="row.released" size="small" type="success">放行（纠错）</el-tag>
                <el-tag v-else-if="row.blocked" size="small" type="danger">拦截</el-tag>
                <span v-else class="muted small">—</span>
              </template>
            </el-table-column>
            <el-table-column label="耗时" width="80">
              <template #default="{ row }">{{ row.latency_ms }}ms</template>
            </el-table-column>
            <el-table-column label="模型" width="150" prop="model_name" />
            <el-table-column label="理由" min-width="220">
              <template #default="{ row }">
                <span class="small">{{ row.reason || row.raw_reply || '—' }}</span>
              </template>
            </el-table-column>
            <template #empty>
              <StateBlock state="empty" title="还没有判定记录" description="对话达到批次轮次或规则命中时，蓝队会介入判定" />
            </template>
          </el-table>
          <el-pagination
            v-if="judgeTotal > judgeSize"
            class="safety-pager"
            layout="prev, pager, next"
            :total="judgeTotal"
            :page-size="judgeSize"
            :current-page="judgePage"
            @current-change="loadJudge"
          />
        </el-tab-pane>

        <!-- ========== 用户画像 ========== -->
        <el-tab-pane label="用户画像" name="profiles">
          <div class="queue-toolbar">
            <el-checkbox v-model="onlyActive" size="small" @change="loadProfiles(1)">只看生效中（观察期 / 冷却）</el-checkbox>
            <span class="muted small">共 {{ profTotal }} 人</span>
          </div>
          <StateBlock v-if="profError" state="error" title="加载失败" @retry="loadProfiles(profPage)" />
          <el-table v-else v-loading="profLoading" :data="profiles" size="small">
            <el-table-column label="用户" width="180">
              <template #default="{ row }">
                {{ row.username || '—' }}<span class="muted small"> #{{ row.user_id }}</span>
              </template>
            </el-table-column>
            <el-table-column label="风险分" width="90" prop="risk_score" />
            <el-table-column label="累计命中" width="100" prop="block_count" />
            <el-table-column label="状态" width="180">
              <template #default="{ row }">
                <el-tag v-if="isCooldown(row)" size="small" type="danger">冷却至 {{ fmt(row.cooldown_until) }}</el-tag>
                <el-tag v-else-if="isWatch(row)" size="small" type="warning">观察期至 {{ fmt(row.watch_until) }}</el-tag>
                <span v-else class="muted small">正常</span>
              </template>
            </el-table-column>
            <el-table-column label="最近命中" width="150">
              <template #default="{ row }">{{ fmt(row.last_hit_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" min-width="200">
              <template #default="{ row }">
                <el-button size="small" text type="primary" @click="onClear(row)">解除限制</el-button>
                <el-button size="small" text @click="onResetProfile(row)">重置画像</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <StateBlock state="empty" title="还没有触发过的用户" description="有拦截记录的用户会出现在这里" />
            </template>
          </el-table>
          <el-pagination
            v-if="profTotal > profSize"
            class="safety-pager"
            layout="prev, pager, next"
            :total="profTotal"
            :page-size="profSize"
            :current-page="profPage"
            @current-change="loadProfiles"
          />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Lock } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import SectionHeader from '@/components/common/SectionHeader.vue';
import StateBlock from '@/components/common/StateBlock.vue';
import { aiApi } from '@/api/ai';
import {
  aiSafetyApi,
  type JudgeLog,
  type SafetyConfig,
  type SafetyEvent,
  type SafetyProfile,
  type SafetyStats,
} from '@/api/aiSafety';

const tab = ref('config');
const loading = ref(false);
const saving = ref(false);
const cfg = ref<SafetyConfig>({} as SafetyConfig);

const chatModels = ref<{ id: number; name: string }[]>([]);
const stats = reactive<SafetyStats>({
  events: 0, blocks: 0, reviews: 0, cooldowns: 0, output_blocks: 0,
  judged: 0, jailbroken: 0, judge_released: 0, judge_failed: 0,
  active_watch: 0, active_cooldown: 0,
});

const statCards = computed(() => [
  { label: '拦截总数', value: stats.blocks },
  { label: '输出整段替换', value: stats.output_blocks },
  { label: '蓝队判定次数', value: stats.judged },
  { label: '判为越狱', value: stats.jailbroken },
  { label: '救回误伤', value: stats.judge_released },
  { label: '观察期中', value: stats.active_watch },
  { label: '冷却中', value: stats.active_cooldown },
]);

// 拦截记录
const events = ref<SafetyEvent[]>([]);
const evLoading = ref(false);
const evError = ref(false);
const evTotal = ref(0);
const evPage = ref(1);
const evSize = 30;
const evFilter = reactive<{ direction: string; action: string }>({ direction: '', action: '' });

// 蓝队判定
const judgeLogs = ref<JudgeLog[]>([]);
const judgeLoading = ref(false);
const judgeError = ref(false);
const judgeTotal = ref(0);
const judgePage = ref(1);
const judgeSize = 30;
const judgeOnlyBad = ref(false);

// 用户画像
const profiles = ref<SafetyProfile[]>([]);
const profLoading = ref(false);
const profError = ref(false);
const profTotal = ref(0);
const profPage = ref(1);
const profSize = 30;
const onlyActive = ref(false);

function fmt(v?: string | null) {
  if (!v) return '—';
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return String(v);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
function tagType(action: string) {
  if (action === 'block' || action === 'cooldown') return 'danger';
  if (action === 'watch') return 'warning';
  return 'info';
}
function actionText(action: string) {
  return ({ block: '拦截', watch: '观察期', cooldown: '冷却', review: '仅记录', released: '放行' } as Record<string, string>)[action] || action;
}
function triggerText(t: string) {
  return ({ schedule: '批次到期', suspect: '可疑提前', review_block: '命中复核' } as Record<string, string>)[t] || t;
}
function isCooldown(r: SafetyProfile) {
  return !!r.cooldown_until && new Date(r.cooldown_until) > new Date();
}
function isWatch(r: SafetyProfile) {
  return !!r.watch_until && new Date(r.watch_until) > new Date();
}

async function loadConfig() {
  try {
    const res = await aiSafetyApi.getConfig();
    if (res.data.code === 0 && res.data.data?.config) {
      cfg.value = res.data.data.config;
    }
  } catch {
    ElMessage.error('读取安全配置失败');
  }
}

async function loadStats() {
  try {
    const res = await aiSafetyApi.stats();
    if (res.data.code === 0 && res.data.data) Object.assign(stats, res.data.data);
  } catch {
    // 统计失败不阻断：计数是辅助信息，列表才是主体
  }
}

async function loadModels() {
  try {
    const res = await aiApi.listModels();
    const list = res.data.data?.list || [];
    chatModels.value = list.filter((m) => m.kind === 'chat').map((m) => ({ id: m.id, name: m.name }));
  } catch {
    // 模型列表拿不到时仍可保存（judge_model_id 保留原值）
  }
}

async function loadEvents(page = 1) {
  evPage.value = page;
  evLoading.value = true;
  evError.value = false;
  try {
    const res = await aiSafetyApi.events({
      page, page_size: evSize,
      direction: evFilter.direction || undefined,
      action: evFilter.action || undefined,
    });
    if (res.data.code === 0 && res.data.data) {
      events.value = res.data.data.items || [];
      evTotal.value = res.data.data.total || 0;
    }
  } catch {
    evError.value = true;
  } finally {
    evLoading.value = false;
  }
}

async function loadJudge(page = 1) {
  judgePage.value = page;
  judgeLoading.value = true;
  judgeError.value = false;
  try {
    const res = await aiSafetyApi.judgeLogs({
      page, page_size: judgeSize,
      only_jailbreak: judgeOnlyBad.value ? '1' : undefined,
    });
    if (res.data.code === 0 && res.data.data) {
      judgeLogs.value = res.data.data.items || [];
      judgeTotal.value = res.data.data.total || 0;
    }
  } catch {
    judgeError.value = true;
  } finally {
    judgeLoading.value = false;
  }
}

async function loadProfiles(page = 1) {
  profPage.value = page;
  profLoading.value = true;
  profError.value = false;
  try {
    const res = await aiSafetyApi.profiles({
      page, page_size: profSize,
      only_active: onlyActive.value ? '1' : undefined,
    });
    if (res.data.code === 0 && res.data.data) {
      profiles.value = res.data.data.items || [];
      profTotal.value = res.data.data.total || 0;
    }
  } catch {
    profError.value = true;
  } finally {
    profLoading.value = false;
  }
}

async function loadAll() {
  loading.value = true;
  await Promise.all([loadConfig(), loadStats(), loadModels()]);
  loading.value = false;
  if (tab.value === 'events') loadEvents(evPage.value);
  if (tab.value === 'judge') loadJudge(judgePage.value);
  if (tab.value === 'profiles') loadProfiles(profPage.value);
}

async function onSave() {
  saving.value = true;
  try {
    const res = await aiSafetyApi.saveConfig(cfg.value);
    if (res.data.code === 0) {
      if (res.data.data?.config) cfg.value = res.data.data.config;
      ElMessage.success('已保存，立即生效');
    } else {
      ElMessage.error('保存失败');
    }
  } catch {
    ElMessage.error('保存失败');
  } finally {
    saving.value = false;
  }
}

async function onReset() {
  try {
    await ElMessageBox.confirm(
      '把所有阈值与文案恢复为内置默认值？已记录的拦截事件与用户画像不受影响。',
      '恢复默认',
      { type: 'warning', confirmButtonText: '恢复', cancelButtonText: '取消' },
    );
  } catch {
    return;
  }
  try {
    const res = await aiSafetyApi.resetConfig();
    if (res.data.code === 0 && res.data.data?.config) {
      cfg.value = res.data.data.config;
      ElMessage.success('已恢复默认');
    }
  } catch {
    ElMessage.error('恢复失败');
  }
}

async function onClear(row: SafetyProfile) {
  try {
    await ElMessageBox.confirm(
      `解除用户 ${row.username || '#' + row.user_id} 的冷却与观察期？风险分保留，仍能看出历史。`,
      '解除限制',
      { type: 'warning', confirmButtonText: '解除', cancelButtonText: '取消' },
    );
  } catch {
    return;
  }
  try {
    await aiSafetyApi.clearCooldown(row.user_id);
    ElMessage.success('已解除');
    loadProfiles(profPage.value);
    loadStats();
  } catch {
    ElMessage.error('操作失败');
  }
}

async function onResetProfile(row: SafetyProfile) {
  try {
    await ElMessageBox.confirm(
      `彻底重置 ${row.username || '#' + row.user_id} 的风险分与命中次数？误判申诉时用。`,
      '重置画像',
      { type: 'warning', confirmButtonText: '重置', cancelButtonText: '取消' },
    );
  } catch {
    return;
  }
  try {
    await aiSafetyApi.resetProfile(row.user_id);
    ElMessage.success('已重置');
    loadProfiles(profPage.value);
    loadStats();
  } catch {
    ElMessage.error('操作失败');
  }
}

onMounted(() => {
  loadAll();
  loadEvents(1);
  loadJudge(1);
  loadProfiles(1);
});
</script>

<style scoped>
.safety-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}
.safety-toggle { display: flex; align-items: center; gap: 10px; }
.safety-actions { display: flex; gap: 8px; }
.safety-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
  gap: 10px;
  margin-bottom: 16px;
}
.safety-stat {
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 10px;
  padding: 12px 10px;
  text-align: center;
}
.safety-stat-num { font-size: 1.35rem; font-weight: 700; line-height: 1.2; }
.safety-stat-label { font-size: 0.75rem; color: var(--text-light, #999); margin-top: 4px; }
.safety-form { max-width: 860px; margin-top: 8px; }
.safety-form :deep(.el-form-item) { margin-bottom: 14px; }
.safety-form :deep(.muted) { margin-left: 10px; }
.queue-toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 12px; }
.excerpt {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  font-size: 0.8rem;
  color: var(--text-light, #888);
  word-break: break-all;
}
.safety-pager { margin-top: 12px; justify-content: flex-end; }
[data-theme='dark'] .safety-stat { background: #161b28; border-color: #262d3d; }
</style>
