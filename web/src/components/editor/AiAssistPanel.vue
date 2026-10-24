<template>
  <!-- 编辑区 AI 助手（§13.3）：动作选择 → 预估费用 → 执行 → 插入/替换 -->
  <el-drawer
    :model-value="visible"
    title="AI 助手"
    size="min(460px, 92vw)"
    @update:model-value="v => emit('update:visible', v)"
  >
    <div v-if="!status.ai_enabled" class="ai-blocked">
      <el-empty description="站长暂未开启 AI 功能">
        <template #description>
          <p>开启后即可使用续写 / 润色 / 扩写 / 摘要等写作辅助。</p>
        </template>
      </el-empty>
    </div>
    <div v-else-if="status.models.length === 0" class="ai-blocked">
      <el-empty description="暂无可用模型">
        <template #description>
          <p>站长尚未配置可用的 AI 模型。</p>
        </template>
      </el-empty>
    </div>
    <div v-else class="ai-body">
      <!-- 余额 -->
      <div class="ai-balance-row">
        <span>
          余额：<b class="ai-balance-num">{{ status.balance }}</b> {{ status.token_name }}
        </span>
        <el-button text type="primary" size="small" @click="goWallet">去充值 / 兑换</el-button>
      </div>

      <!-- 模型与动作 -->
      <el-form label-width="72px" class="ai-form">
        <el-form-item label="模型">
          <el-select v-model="form.model_id" size="small" style="width:100%" :disabled="form.byok">
            <el-option
              v-for="m in status.models"
              :key="m.id"
              :label="modelLabel(m)"
              :value="m.id"
            />
          </el-select>
          <el-checkbox v-model="form.byok" size="small" style="margin-left:8px">我的密钥</el-checkbox>
          <el-input
            v-if="form.byok"
            v-model="form.byok_model"
            size="small"
            style="width:56%; margin-top:6px"
            placeholder="上游模型 ID（如 deepseek-chat）"
          />
        </el-form-item>
        <el-form-item label="动作">
          <el-radio-group v-model="form.action" size="small">
            <el-radio-button v-for="a in AI_ACTIONS" :key="a.value" :value="a.value">
              {{ a.label }}
            </el-radio-button>
          </el-radio-group>
          <div class="ai-action-hint">{{ currentActionHint }}</div>
        </el-form-item>
        <el-form-item label="输出长度">
          <el-radio-group v-model="form.max_tokens" size="small">
            <el-radio-button :value="512">短</el-radio-button>
            <el-radio-button :value="1024">中</el-radio-button>
            <el-radio-button :value="2048">长</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <!-- 长篇上下文（§13.5） -->
        <el-form-item v-if="memoryEnabled" label="长篇上下文">
          <div class="ai-memory-block">
            <el-checkbox v-model="form.use_memory" size="small">注入作品记忆</el-checkbox>
            <span class="muted small">
              大纲 {{ memory.outline ? '已设' : '未设' }} · 摘要 {{ memory.summaries }} 条 ·
              最近 {{ memory.recent }} 章入上下文 · 预算 {{ memory.budget }} tokens
            </span>
            <el-button text type="primary" size="small" @click="memoryPanelOpen = !memoryPanelOpen">
              {{ memoryPanelOpen ? '收起记忆管理' : '管理记忆' }}
            </el-button>
          </div>
          <div v-if="memoryPanelOpen" class="ai-memory-panel">
            <div class="ai-memory-sub">作品大纲（注入最高优先级）</div>
            <el-input
              v-model="outlineDraft"
              type="textarea"
              :rows="3"
              resize="none"
              placeholder="用几句话概括作品主线、人物与设定，AI 续写时将以此为纲"
              maxlength="8000"
            />
            <div class="ai-memory-actions">
              <el-button size="small" type="primary" :loading="memorySaving" @click="saveOutline">保存大纲</el-button>
            </div>
            <div class="ai-memory-sub">章节摘要（按章保存，新→旧取最近 {{ memory.recent }} 条）</div>
            <el-table :data="memoryList" size="small" :show-header="true" max-height="220">
              <el-table-column label="章节" width="90">
                <template #default="{ row }">#{{ row.chapter_id }}</template>
              </el-table-column>
              <el-table-column label="摘要" min-width="160" show-overflow-tooltip>
                <template #default="{ row }">{{ row.content }}</template>
              </el-table-column>
              <el-table-column label="操作" width="60" fixed="right">
                <template #default="{ row }">
                  <el-button text type="danger" size="small" @click="removeSummary(row)">删</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div class="ai-memory-actions">
              <el-button size="small" :loading="memorySaving" @click="saveCurrentSummary">
                将当前正文存为本章摘要
              </el-button>
            </div>
            <div class="muted small" style="margin-top:6px">
              提示：用上方「摘要」动作生成后，也可点结果区「存为章节摘要」。
            </div>
          </div>
        </el-form-item>
      </el-form>

      <!-- 输入 -->
      <div class="ai-input-label">
        输入内容
        <el-button text size="small" type="primary" @click="resetFromContext">恢复为当前章节</el-button>
      </div>
      <el-input
        v-model="form.text"
        type="textarea"
        :rows="8"
        resize="none"
        placeholder="将基于这段内容执行所选动作；可只粘贴需要的片段以节省费用"
      />

      <!-- 预估 -->
      <div class="ai-estimate-row">
        <span v-if="estimating" class="muted">预估中…</span>
        <span v-else-if="estimate" :class="{ 'ai-warn': !estimate.affordable }">
          <template v-if="estimate.free_eligible">
            预估：{{ estimate.tokens_in }} 入 / {{ estimate.tokens_out }} 出 tokens，<b>免费额度</b>
          </template>
          <template v-else>
            预估：{{ estimate.tokens_in }} 入 / {{ estimate.tokens_out }} 出 tokens，约
            <b>{{ estimate.cost }}</b> {{ status.token_name }}
            <span v-if="!estimate.affordable">（余额不足，请先充值）</span>
          </template>
        </span>
        <span v-else class="muted">输入内容后自动预估费用</span>
      </div>

      <el-button
        type="primary"
        class="ai-run"
        :loading="running"
        :disabled="!form.text.trim() || (estimate && !estimate.affordable)"
        @click="run"
      >
        <template v-if="estimate?.free_eligible">执行（免费额度）</template>
        <template v-else>执行（约 {{ estimate?.cost ?? '—' }} {{ status.token_name }}）</template>
      </el-button>

      <!-- 结果 -->
      <div v-if="result" class="ai-result">
        <div class="ai-result-label">
          AI 结果
          <span class="muted small">
            <template v-if="result.free">免费额度 · </template>
            <template v-else>实耗 {{ result.tokens_in }}/{{ result.tokens_out }} tokens，扣除 {{ result.cost }}，余额 {{ result.balance_after }}</template>
          </span>
        </div>
        <div class="ai-result-text">{{ result.content }}</div>
        <div class="ai-result-actions">
          <el-button size="small" @click="copyResult">复制</el-button>
          <el-button v-if="memoryEnabled && props.chapterId" size="small" @click="saveResultAsSummary">存为章节摘要</el-button>
          <el-button size="small" type="primary" plain @click="emit('replace', result!.content, { model: usedModelName })">替换正文</el-button>
          <el-button size="small" type="primary" @click="emit('insert', result!.content, { model: usedModelName })">插入编辑器</el-button>
        </div>
      </div>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { aiApi, AI_ACTIONS, type AiStatus, type AiEstimateResult, type AiNovelMemory } from '@/api/ai';

const usedModelName = computed(() => {
  const m = status.models.find((x) => x.id === form.model_id);
  return m ? m.name : '';
});

const props = defineProps<{
  visible: boolean;
  /** 当前章节文本（打开面板时预填） */
  contextText?: string;
  /** 作品 ID（§13.5 长篇上下文；0=不启用记忆） */
  novelId?: number;
  /** 当前章节 ID（§13.5；0=新章节） */
  chapterId?: number;
}>();
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void;
  (e: 'insert', text: string): void;
  (e: 'replace', text: string): void;
}>();

const router = useRouter();
const status = reactive<AiStatus>({
  ai_enabled: false,
  token_enabled: true,
  token_name: '代币',
  balance: 0,
  models: [],
});

const form = reactive<{ model_id: number; action: string; max_tokens: number; text: string; byok: boolean; byok_model: string; use_memory: boolean }>({
  model_id: 0,
  action: 'continue',
  max_tokens: 1024,
  text: '',
  byok: false,
  byok_model: '',
  use_memory: true,
});

const estimate = ref<AiEstimateResult | null>(null);
const estimating = ref(false);
const running = ref(false);
const result = ref<{ content: string; tokens_in: number; tokens_out: number; cost: number; free?: boolean; balance_after: number } | null>(null);

const currentActionHint = computed(
  () => AI_ACTIONS.find((a) => a.value === form.action)?.hint || '',
);

function modelLabel(m: { id: number; name: string; price_in: number; price_out: number }) {
  const price = m.price_in === 0 && m.price_out === 0 ? '免费' : `${m.price_in}/${m.price_out}`;
  return `${m.name}（${price}）`;
}

function goWallet() {
  emit('update:visible', false);
  router.push('/wallet');
}

function resetFromContext() {
  form.text = props.contextText || '';
}

async function loadStatus() {
  try {
    const res = await aiApi.status();
    if (res.data.code === 0) {
      Object.assign(status, res.data.data);
      if (!form.model_id && status.models.length) form.model_id = status.models[0].id;
    }
  } catch { /* ignore */ }
}

// ── 预估（输入防抖 600ms） ──
let estimateTimer: number | undefined;
watch([() => form.text, () => form.model_id, () => form.max_tokens, () => form.action], () => {
  result.value = null;
  if (estimateTimer) window.clearTimeout(estimateTimer);
  if (!form.text.trim() || !form.model_id) {
    estimate.value = null;
    return;
  }
  estimateTimer = window.setTimeout(doEstimate, 600);
});

async function doEstimate() {
  if (!form.text.trim() || !form.model_id) return;
  estimating.value = true;
  try {
    const res = await aiApi.estimate({
      model_id: form.model_id,
      text: form.text,
      max_tokens: form.max_tokens,
    });
    if (res.data.code === 0) estimate.value = res.data.data;
  } catch {
    estimate.value = null;
  }
  estimating.value = false;
}

// ── 执行 ──
async function run() {
  if (!form.text.trim()) {
    ElMessage.warning('请输入内容');
    return;
  }
  if (form.byok && !form.byok_model.trim()) {
    ElMessage.warning('使用我的密钥时需填写上游模型 ID');
    return;
  }
  await doEstimate();
  if (estimate.value && !estimate.value.affordable && !form.byok) {
    ElMessage.warning(`${status.token_name}不足，请先充值`);
    return;
  }
  running.value = true;
  try {
    const res = await aiApi.complete({
      model_id: form.model_id,
      action: form.action,
      text: form.text,
      max_tokens: form.max_tokens,
      ref_type: 'chapter',
      byok: form.byok || undefined,
      byok_model: form.byok ? form.byok_model.trim() : undefined,
      novel_id: memoryEnabled.value ? props.novelId : undefined,
      chapter_id: memoryEnabled.value ? props.chapterId : undefined,
      use_memory: memoryEnabled.value && form.use_memory,
    });
    if (res.data.code === 0) {
      result.value = res.data.data;
      status.balance = res.data.data.balance_after;
      ElMessage.success(res.data.data.byok ? '生成成功（使用自备密钥）' : res.data.data.free ? '生成成功（免费额度）' : '生成成功');
      if (memoryEnabled.value && form.use_memory) refreshMemory();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '调用失败');
  }
  running.value = false;
}

// ==================== 长篇上下文（§13.5） ====================
const memoryEnabled = computed(() => !!props.novelId);
const memoryPanelOpen = ref(false);
const memorySaving = ref(false);
const memory = reactive({ outline: false, summaries: 0, budget: 3000, recent: 5 });
const memoryList = ref<AiNovelMemory[]>([]);
const outlineDraft = ref('');

async function refreshMemory() {
  if (!props.novelId) return;
  try {
    const res = await aiApi.memoryList(props.novelId);
    if (res.data.code === 0) {
      const d = res.data.data;
      memory.outline = d.outline;
      memory.summaries = d.summaries;
      memory.budget = d.budget;
      memory.recent = d.recent;
      memoryList.value = d.list.filter((m) => m.kind === 'chapter_summary');
      const outline = d.list.find((m) => m.kind === 'outline');
      if (outline && !outlineDraft.value) outlineDraft.value = outline.content;
    }
  } catch { /* ignore */ }
}

async function saveOutline() {
  if (!props.novelId) return;
  memorySaving.value = true;
  try {
    const res = await aiApi.saveOutline({ novel_id: props.novelId, content: outlineDraft.value });
    if (res.data.code === 0) {
      ElMessage.success('大纲已保存');
      await refreshMemory();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  memorySaving.value = false;
}

async function saveCurrentSummary() {
  if (!props.novelId || !props.chapterId) {
    ElMessage.warning('当前章节尚未保存，无法关联摘要');
    return;
  }
  memorySaving.value = true;
  try {
    const res = await aiApi.saveSummary({
      novel_id: props.novelId,
      chapter_id: props.chapterId,
      content: form.text,
    });
    if (res.data.code === 0) {
      ElMessage.success('已存为本章摘要');
      await refreshMemory();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  memorySaving.value = false;
}

async function saveResultAsSummary() {
  if (!result.value || !props.novelId || !props.chapterId) return;
  memorySaving.value = true;
  try {
    const res = await aiApi.saveSummary({
      novel_id: props.novelId,
      chapter_id: props.chapterId,
      content: result.value.content,
    });
    if (res.data.code === 0) {
      ElMessage.success('已存为本章摘要');
      await refreshMemory();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  memorySaving.value = false;
}

async function removeSummary(row: AiNovelMemory) {
  await ElMessageBox.confirm(`删除章节 #${row.chapter_id} 的摘要？`, '确认', { type: 'warning' });
  try {
    const res = await aiApi.deleteSummary({ novel_id: row.novel_id, chapter_id: row.chapter_id });
    if (res.data.code === 0) {
      ElMessage.success('已删除');
      await refreshMemory();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

watch(() => props.visible, (v) => {
  if (v && memoryEnabled.value) refreshMemory();
});

async function copyResult() {
  if (!result.value) return;
  try {
    await navigator.clipboard.writeText(result.value.content);
    ElMessage.success('已复制');
  } catch {
    ElMessage.warning('浏览器拒绝访问剪贴板，请手动复制');
  }
}

// 打开时加载状态并预填
watch(
  () => props.visible,
  (v) => {
    if (v) {
      loadStatus();
      if (!form.text.trim() && props.contextText) form.text = props.contextText;
    }
  },
);
</script>

<style scoped>
.ai-body { padding: 0 4px 24px; }
.ai-blocked { padding-top: 40px; }
.ai-balance-row {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 10px; background: var(--el-fill-color-light, #f5f7fa);
  border-radius: 8px; margin-bottom: 14px; font-size: 0.9rem;
}
.ai-balance-num { font-size: 1.2rem; color: var(--el-color-primary, #409eff); }
.ai-form :deep(.el-form-item) { margin-bottom: 10px; }
.ai-action-hint { font-size: 0.75rem; color: var(--text-light, #999); margin-top: 2px; }
.ai-input-label {
  display: flex; align-items: center; justify-content: space-between;
  font-size: 0.85rem; font-weight: 600; margin: 10px 0 6px;
}
.ai-estimate-row {
  font-size: 0.82rem; margin: 10px 0; min-height: 22px; color: var(--text-color, #333);
}
.ai-warn { color: #ef4444; }
.ai-run { width: 100%; }
.ai-result { margin-top: 16px; border-top: 1px solid var(--el-border-color, #ebeef5); padding-top: 12px; }
.ai-result-label { display: flex; align-items: baseline; justify-content: space-between; font-weight: 600; margin-bottom: 8px; }
.ai-result-text {
  max-height: 300px; overflow: auto; white-space: pre-wrap;
  background: var(--el-fill-color-light, #f8f9fa); border-radius: 8px;
  padding: 12px; font-size: 0.88rem; line-height: 1.7;
}
.ai-result-actions { display: flex; gap: 8px; margin-top: 10px; }
.ai-memory-block { display: flex; flex-direction: column; gap: 4px; width: 100%; }
.ai-memory-panel {
  margin-top: 8px; padding: 10px; border-radius: 8px; width: 100%;
  background: var(--el-fill-color-light, #f8f9fa);
}
.ai-memory-sub { font-size: 0.78rem; font-weight: 600; margin: 8px 0 6px; color: var(--text-color, #333); }
.ai-memory-actions { margin-top: 8px; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.75rem; font-weight: 400; }
[data-theme="dark"] .ai-balance-row,
[data-theme="dark"] .ai-result-text,
[data-theme="dark"] .ai-memory-panel { background: #1e293b; }
</style>
