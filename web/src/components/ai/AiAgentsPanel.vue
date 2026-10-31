<template>
  <!-- ===== AI 智能体 =====
       参照参考站的「AI 智能体 / Agent 应用」：卡片墙而非左侧列表，
       每个智能体有自己的定位说明，降低"不知道选哪个"的门槛。 -->
  <div class="ag">
    <div class="ag-head">
      <div>
        <h2 class="ag-title">AI 智能体</h2>
        <p class="ag-sub">
          预设角色与提示词的专属助手，点一个直接开始对话。
          <template v-if="!agents.length">站长还没有创建智能体。</template>
        </p>
      </div>
      <div class="ag-tools">
        <el-input v-model="keyword" class="ag-search" placeholder="搜索智能体" clearable>
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button v-if="submitCfg.enabled" @click="openSubmit">投稿智能体</el-button>
        <el-button v-if="mySubs.length" @click="myDialog = true">我的投稿</el-button>
      </div>
    </div>

    <div v-if="filtered.length" class="ag-grid">
      <article
        v-for="a in filtered"
        :key="a.id"
        class="ag-card"
        role="button"
        tabindex="0"
        @click="$emit('start', a)"
        @keydown.enter="$emit('start', a)"
        @keydown.space.prevent="$emit('start', a)"
      >
        <span class="ag-emoji">{{ a.emoji || '🤖' }}</span>
        <h3 class="ag-name">{{ a.name }}</h3>
        <p v-if="a.description" class="ag-desc">{{ a.description }}</p>
        <div class="ag-foot">
          <span v-if="a.category" class="ag-cat">{{ a.category }}</span>
          <span v-if="a.upstream === 'coze'" class="ag-coze">Coze</span>
          <span v-if="a.used_count" class="ag-used">{{ formatCount(a.used_count) }} 次使用</span>
        </div>
        <span class="ag-go">开始对话 →</span>
      </article>
    </div>

    <StateBlock
      v-else
      state="empty"
      :title="agents.length ? '没有匹配的智能体' : '还没有智能体'"
      :description="
        agents.length ? '换个关键词再试。' : isAdmin ? '到「AI 智能体」后台创建角色化助手。' : '站长还没有配置智能体。'
      "
      :action-text="!agents.length && isAdmin ? '去创建' : agents.length ? '清空搜索' : ''"
      @action="onEmptyAction"
    />

    <!-- ===== 投稿弹窗 =====
         用户可以把自己的特殊提示词提交成智能体；是否要站长审核由站长决定。 -->
    <el-dialog v-model="submitDialog" title="投稿智能体" width="620px" append-to-body class="ag-submit-dlg">
      <div class="ag-submit-tip">
        智能体的本质就是一段<b>特殊提示词</b>。写清「它是什么角色、遵守什么规则、输出什么格式」，就能成为一个可被大家使用的智能体。
        <template v-if="submitCfg.need_review">提交后需要站长审核，通过后才会出现在广场。</template>
        <template v-else>本站已关闭审核，提交后立即上架。</template>
      </div>
      <el-form label-width="86px" class="ag-submit-form">
        <el-form-item label="名称" required>
          <el-input v-model="subForm.name" maxlength="32" show-word-limit placeholder="如：严谨校对员" />
        </el-form-item>
        <el-form-item label="图标">
          <el-input v-model="subForm.emoji" maxlength="4" placeholder="一个 emoji，如 🔍" style="width:120px" />
          <el-input v-model="subForm.category" maxlength="16" placeholder="分类，如 写作" style="width:140px;margin-left:10px" />
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="subForm.description" maxlength="200" show-word-limit placeholder="一句话说清它擅长什么" />
        </el-form-item>
        <el-form-item label="开场白">
          <el-input v-model="subForm.greeting" maxlength="200" placeholder="用户进入对话时它的第一句话（可留空）" />
        </el-form-item>
        <el-form-item label="提示词" required>
          <el-input
            v-model="subForm.system_prompt"
            type="textarea"
            :rows="9"
            maxlength="8000"
            show-word-limit
            placeholder="你是……。规则：1) …… 2) …… 输出格式：……"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="submitDialog = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="doSubmit">提交</el-button>
      </template>
    </el-dialog>

    <!-- ===== 我的投稿 ===== -->
    <el-dialog v-model="myDialog" title="我的投稿" width="640px" append-to-body>
      <el-table :data="mySubs" size="small">
        <el-table-column label="智能体" min-width="150">
          <template #default="{ row }">
            <span class="ag-mini-emoji">{{ row.emoji || '🤖' }}</span>
            <span>{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="statusTag(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="站长备注" min-width="180">
          <template #default="{ row }">
            <span v-if="row.review_note">{{ row.review_note }}</span>
            <span v-else class="muted small">—</span>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button type="primary" @click="myDialog = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted } from 'vue';
import { Search } from '@element-plus/icons-vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import StateBlock from '@/components/common/StateBlock.vue';
import { aiHubApi, type AiAgent, type AgentSubmission } from '@/api/aiHub';

const props = defineProps<{ agents: AiAgent[]; isAdmin: boolean }>();
const emit = defineEmits<{ (e: 'start', a: AiAgent): void; (e: 'refresh'): void }>();
const router = useRouter();

const keyword = ref('');
const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  if (!kw) return props.agents;
  return props.agents.filter(
    (a) =>
      a.name.toLowerCase().includes(kw) ||
      (a.category || '').toLowerCase().includes(kw) ||
      (a.description || '').toLowerCase().includes(kw),
  );
});

// ---------- 投稿 ----------
const submitCfg = reactive({ enabled: false, need_review: true, max_pending: 0 });
const submitDialog = ref(false);
const submitting = ref(false);
const myDialog = ref(false);
const mySubs = ref<AgentSubmission[]>([]);
const subForm = reactive({
  name: '', emoji: '', description: '', greeting: '', category: '', system_prompt: '',
});

function resetSubForm() {
  Object.assign(subForm, { name: '', emoji: '', description: '', greeting: '', category: '', system_prompt: '' });
}

async function loadSubmitConfig() {
  try {
    const res = await aiHubApi.agentSubmitConfig();
    if (res.data.code === 0 && res.data.data) Object.assign(submitCfg, res.data.data);
  } catch {
    // 未登录 / 接口不可用：不显示投稿入口，不影响浏览广场
    submitCfg.enabled = false;
  }
}

async function loadMySubs() {
  try {
    const res = await aiHubApi.myAgentSubmissions();
    if (res.data.code === 0) mySubs.value = res.data.data.list || [];
  } catch {
    mySubs.value = [];
  }
}

function openSubmit() {
  resetSubForm();
  submitDialog.value = true;
}

async function doSubmit() {
  if (!subForm.name.trim() || !subForm.system_prompt.trim()) {
    ElMessage.warning('名称和提示词是必填的');
    return;
  }
  submitting.value = true;
  try {
    const res = await aiHubApi.submitAgent({ ...subForm });
    if (res.data.code !== 0) {
      ElMessage.error(res.data.message || '提交失败');
      return;
    }
    ElMessage.success(res.data.data?.message || '已提交');
    submitDialog.value = false;
    await loadMySubs();
    // 免审核直上时要让广场立刻出现新条目
    if (res.data.data?.status === 'approved') emit('refresh');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败');
  } finally {
    submitting.value = false;
  }
}

function statusLabel(s: string) {
  return s === 'approved' ? '已上架' : s === 'rejected' ? '未通过' : '待审核';
}
function statusTag(s: string) {
  return s === 'approved' ? 'success' : s === 'rejected' ? 'danger' : 'warning';
}

onMounted(() => {
  loadSubmitConfig();
  loadMySubs();
});

function formatCount(n?: number) {
  const v = n || 0;
  if (v >= 10000) return `${(v / 10000).toFixed(1)}w`;
  if (v >= 1000) return `${(v / 1000).toFixed(1)}k`;
  return String(v);
}

function onEmptyAction() {
  if (!props.agents.length && props.isAdmin) {
    router.push('/admin/ai-agents');
    return;
  }
  keyword.value = '';
}
</script>

<style scoped>
.ag {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-6, 32px);
  max-width: 1180px;
  width: 100%;
  margin: 0 auto;
  box-sizing: border-box;
}
.ag-head {
  display: flex; flex-wrap: wrap; gap: var(--space-4, 16px);
  align-items: flex-start; justify-content: space-between;
  margin-bottom: var(--space-4, 16px);
}
.ag-title { margin: 0 0 var(--space-1, 4px); font-size: var(--text-xl, 22px); font-weight: 700; color: var(--text-color, #1f2328); }
.ag-sub { margin: 0; font-size: var(--text-sm, 13px); color: var(--text-light, #6b7280); }
.ag-search { max-width: 240px; }
.ag-tools { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }

/* 投稿弹窗 */
.ag-submit-tip {
  margin-bottom: 14px; padding: 10px 12px; border-radius: 8px;
  background: var(--bg-page, #f5f6f8);
  font-size: var(--text-sm, 13px); line-height: 1.7; color: var(--text-light, #6b7280);
}
.ag-submit-form :deep(.el-form-item) { margin-bottom: 14px; }
.ag-mini-emoji { margin-right: 6px; }

.ag-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: var(--space-4, 16px); }
.ag-card {
  display: flex; flex-direction: column; gap: var(--space-2, 8px);
  padding: var(--space-4, 16px);
  cursor: pointer;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-lg, 16px);
  box-shadow: var(--shadow-xs, 0 1px 3px rgba(0, 0, 0, 0.04));
  transition: transform 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease;
}
.ag-card:hover {
  transform: translateY(-2px);
  border-color: var(--el-color-primary, #6366f1);
  box-shadow: var(--shadow-md, 0 8px 24px rgba(0, 0, 0, 0.08));
}
.ag-card:focus-visible { outline: 2px solid var(--el-color-primary, #6366f1); outline-offset: 2px; }
.ag-emoji { font-size: 26px; line-height: 1; }
.ag-name { margin: 0; font-size: var(--text-base, 14px); font-weight: 600; color: var(--text-color, #1f2328); }
.ag-desc {
  margin: 0; font-size: var(--text-sm, 13px); line-height: 1.55; color: var(--text-light, #6b7280);
  display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden;
}
.ag-foot { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; font-size: var(--text-xs, 12px); }
.ag-cat { padding: 1px 6px; border-radius: 4px; background: var(--bg-page, #f5f6f8); color: var(--text-light, #6b7280); }
.ag-coze { padding: 1px 6px; border-radius: 4px; color: #8b5cf6; background: rgba(139, 92, 246, 0.12); }
.ag-used { color: var(--text-light, #9ca3af); }
.ag-go { margin-top: auto; font-size: var(--text-xs, 12px); color: var(--el-color-primary, #6366f1); }

[data-theme="dark"] .ag-card { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .ag-title, [data-theme="dark"] .ag-name { color: #e2e8f0; }
[data-theme="dark"] .ag-cat { background: #0f172a; }

@media (max-width: 640px) {
  .ag { padding: var(--space-4, 16px); }
  .ag-grid { grid-template-columns: 1fr; }
  .ag-search { max-width: none; width: 100%; }
}
</style>
