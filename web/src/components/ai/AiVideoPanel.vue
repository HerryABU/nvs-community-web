<template>
  <!-- AI 视频面板：左侧 320 参数区 + 右侧纵向任务流（与绘画页同一套布局语言） -->
  <div class="ai-paint">
    <aside class="pd-side">
      <div class="pd-side-scroll">
        <div class="pd-block">
          <div class="pd-block-title">
            模型
            <el-checkbox v-model="useByok" size="small" :disabled="submitting">我的密钥</el-checkbox>
          </div>
          <el-select v-model="modelId" size="small" style="width:100%" :disabled="useByok || submitting">
            <el-option v-for="m in status.video_models" :key="m.id" :label="modelLabel(m)" :value="m.id" />
          </el-select>
          <el-input
            v-if="useByok"
            v-model="byokModel"
            size="small"
            style="margin-top:6px"
            placeholder="上游模型 ID"
            :disabled="submitting"
          />
          <p v-if="!status.video_models.length" class="pd-hint">
            暂无视频模型。到「AI 模型池」添加视频类模型（可灵 / Runway / Sora 等）。
          </p>
          <p v-else-if="useByok" class="pd-hint">用你自己的上游密钥，不消耗平台代币</p>
        </div>

        <div class="pd-block">
          <div class="pd-block-title">提示词</div>
          <el-input
            v-model="prompt"
            type="textarea"
            :rows="5"
            resize="none"
            placeholder="描述画面与镜头运动，如：夕阳下的海面，海浪缓慢起伏，镜头缓慢推进"
          />
        </div>

        <div class="pd-block">
          <div class="pd-block-title">时长</div>
          <div class="pd-durations">
            <button
              v-for="d in DURATIONS"
              :key="d.value"
              type="button"
              :class="['pd-duration', { active: duration === d.value }]"
              @click="duration = d.value"
            >
              <b>{{ d.value }} 秒</b>
              <span>{{ d.label }}</span>
            </button>
          </div>
        </div>

        <div class="pd-block">
          <div class="pd-block-title">画面</div>
          <div class="pd-sizes">
            <button
              v-for="s in SIZES"
              :key="s.value"
              type="button"
              :class="['pd-size', { active: vsize === s.value }]"
              @click="vsize = s.value"
            >
              <span class="pd-size-shape" :class="s.shape" />
              <b>{{ s.label }}</b>
            </button>
          </div>
        </div>
      </div>

      <div class="pd-side-foot">
        <div class="pd-cost">
          <span class="pd-cost-model">{{ currentModel?.name || '未选模型' }}</span>
          <span class="pd-cost-balance">余额 {{ status.balance }} {{ status.token_name }}</span>
        </div>
        <el-button
          type="primary"
          size="large"
          class="pd-generate"
          :loading="submitting"
          :disabled="!prompt.trim()"
          @click="submit"
        >
          提交生成 · {{ currentModel?.price_call || 0 }} {{ status.token_name }}
        </el-button>
        <p class="pd-hint" style="text-align:center;margin:6px 0 0">
          异步任务：提交即扣款，失败自动退款；可离开页面，回来仍会继续刷新
        </p>
      </div>
    </aside>

    <section class="pd-stream">
      <header class="pd-stream-head">
        <div>
          <h2 class="pd-stream-title">视频任务</h2>
          <p class="pd-stream-sub">
            <template v-if="history.length">共 {{ history.length }} 个任务，按时间倒序</template>
            <template v-else>提交后任务会出现在这里，可离开页面</template>
          </p>
        </div>
        <el-button size="small" text @click="loadHistory">刷新</el-button>
      </header>

      <StateBlock
        v-if="!history.length"
        state="empty"
        title="还没有视频任务"
        description="在左侧写好提示词并提交，任务会在这里排队、生成、出片。"
      />

      <article
        v-for="t in history"
        :key="t.id"
        class="pd-card"
        :class="{ 'is-latest': t.id === latestTaskId, 'is-failed': t.status === 'failed' }"
      >
        <header class="pd-card-head">
          <span class="pd-card-model">{{ t.model_name || 'AI 视频' }}</span>
          <span class="pd-card-time">{{ formatTime(t.created_at) }}</span>
          <span class="pd-card-flag" :class="statusClass(t.status)">{{ statusText(t.status) }}</span>
          <span v-if="t.is_public" class="pd-card-flag">已发布</span>
          <span v-if="t.cost" class="pd-card-cost">-{{ t.cost }} {{ status.token_name }}</span>
          <span class="pd-card-spacer" />
          <button
            v-if="parseVideo(t.result)"
            class="pd-card-tool"
            type="button"
            @click="publishVideo(t)"
          >{{ t.is_public ? '管理发布' : '发布' }}</button>
          <button class="pd-card-tool" type="button" @click="reusePrompt(t)">再来一条</button>
          <button class="pd-card-tool" type="button" @click="copyPrompt(t)">复制提示词</button>
        </header>

        <p
          class="pd-card-prompt"
          :class="{ 'is-open': expandedIds.has(t.id) }"
          :title="expandedIds.has(t.id) ? '点击收起' : '点击展开'"
          @click="toggleExpand(t.id)"
        >
          {{ t.prompt || '（无提示词）' }}
        </p>

        <!-- 出片 -->
        <video
          v-if="parseVideo(t.result)"
          :src="parseVideo(t.result)"
          controls
          preload="metadata"
          class="pd-video"
        />

        <!-- 失败 -->
        <p v-else-if="t.status === 'failed'" class="pd-card-error">
          {{ t.error || '生成失败（已自动退款）' }}
        </p>

        <!-- 进行中：状态条 + 进度 -->
        <div v-else class="pd-running">
          <div class="pd-running-head">
            <span class="pd-running-text">
              {{ t.status === 'pending' ? '排队中…' : '正在生成…' }}
              <span v-if="isPollingTask(t)" class="pd-running-live">自动刷新中</span>
            </span>
            <span class="pd-running-hint">可离开页面，完成后在历史中查看</span>
          </div>
          <el-progress
            :percentage="t.status === 'pending' ? 12 : 60"
            :stroke-width="6"
            :show-text="false"
            striped
            striped-flow
          />
        </div>

        <footer v-if="attachmentsOf(t).length" class="pd-card-foot">
          参考素材 {{ attachmentsOf(t).length }} 个
        </footer>
      </article>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { aiHubApi, type AiHubStatus } from '@/api/aiHub';
import StateBlock from '@/components/common/StateBlock.vue';

const props = defineProps<{ status: AiHubStatus }>();

const DURATIONS = [
  { value: 5, label: '短片' },
  { value: 10, label: '长片' },
];
const SIZES = [
  { value: '1280x720', label: '横屏', shape: 'landscape' },
  { value: '720x1280', label: '竖屏', shape: 'portrait' },
];

const modelId = ref(0);
const useByok = ref(false);
const byokModel = ref('');
const prompt = ref('');
const duration = ref(5);
const vsize = ref('1280x720');
const submitting = ref(false);
const history = ref<any[]>([]);
/** 正在轮询的任务 id（0 = 没有） */
const latestTaskId = ref(0);
let pollTimer: number | undefined;

watch(
  () => props.status.video_models,
  (list) => {
    if (!modelId.value && list.length) modelId.value = list[0].id;
  },
  { immediate: true },
);

const currentModel = computed(() => props.status.video_models.find((m) => m.id === modelId.value));

function modelLabel(m: { name: string; price_call: number }) {
  return `${m.name}（${m.price_call || '免费'}）`;
}

function statusText(s: string) {
  const map: Record<string, string> = {
    pending: '排队中', running: '生成中', succeeded: '已完成', failed: '失败',
  };
  return map[s] || s;
}
function statusClass(s: string) {
  if (s === 'succeeded') return 'is-ok';
  if (s === 'failed') return 'is-err';
  return 'is-run';
}

function parseVideo(result: string): string {
  try {
    return JSON.parse(result).video || '';
  } catch {
    return '';
  }
}

function attachmentsOf(t: any): any[] {
  try {
    const arr = JSON.parse(t.attachments || '[]');
    return Array.isArray(arr) ? arr : [];
  } catch {
    return [];
  }
}

// ---------- 作品流交互 ----------
const expandedIds = ref<Set<number>>(new Set());
function toggleExpand(id: number) {
  if (expandedIds.value.has(id)) expandedIds.value.delete(id);
  else expandedIds.value.add(id);
  expandedIds.value = new Set(expandedIds.value);
}

function formatTime(iso: string) {
  if (!iso) return '';
  const d = new Date(iso);
  const pad = (v: number) => String(v).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** 该任务是否正在被轮询（决定是否显示"自动刷新中"） */
function isPollingTask(t: any) {
  return t.id === latestTaskId.value && (t.status === 'pending' || t.status === 'running');
}

async function copyPrompt(t: any) {
  try {
    await navigator.clipboard.writeText(t.prompt || '');
    ElMessage.success('提示词已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择');
  }
}

function reusePrompt(t: any) {
  prompt.value = t.prompt || '';
  ElMessage.success('提示词已回填，确认参数后点「提交生成」');
}

// ---------- 提交与轮询 ----------
async function submit() {
  if (!modelId.value && !useByok.value) {
    ElMessage.warning('请先选择模型');
    return;
  }
  if (useByok.value && !byokModel.value.trim()) {
    ElMessage.warning('使用我的密钥时需填写上游模型 ID');
    return;
  }
  submitting.value = true;
  try {
    const res = await aiHubApi.videoSubmit({
      model_id: modelId.value,
      prompt: prompt.value.trim(),
      duration: duration.value,
      size: vsize.value,
      byok: useByok.value || undefined,
      byok_model: useByok.value ? byokModel.value.trim() : undefined,
    });
    if (res.data.code === 0) {
      // 任务立即进流（不在下方另开"当前任务"区，避免同一任务出现两次）
      latestTaskId.value = res.data.data.task_id;
      prompt.value = '';
      await loadHistory();
      startPolling();
      ElMessage.success('已提交，生成完成后会自动出现');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败');
  }
  submitting.value = false;
}

/**
 * 轮询直接更新流里对应任务的状态。
 * 这样「提交 → 排队 → 生成 → 出片」全程只有一张卡在变，不会出现两处状态不一致。
 */
async function pollOnce() {
  const id = latestTaskId.value;
  if (!id) return;
  try {
    const res = await aiHubApi.videoStatus(id);
    if (res.data.code !== 0) return;
    const st = res.data.data;
    const item = history.value.find((t: any) => t.id === id);
    if (item) {
      item.status = st.status;
      if (st.video) item.result = JSON.stringify({ video: st.video });
      if (st.error) item.error = st.error;
    }
    if (st.status === 'succeeded' || st.status === 'failed') {
      stopPolling();
      latestTaskId.value = 0;
      await loadHistory();
      if (st.status === 'failed') ElMessage.warning('视频生成失败，已自动退款');
      else ElMessage.success('视频已生成');
    }
  } catch { /* 轮询失败保持原状，下次继续 */ }
}

function startPolling() {
  stopPolling();
  pollOnce();
  pollTimer = window.setInterval(pollOnce, 5000);
}
function stopPolling() {
  if (pollTimer) {
    window.clearInterval(pollTimer);
    pollTimer = undefined;
  }
}

// 发布到视频广场（与图像共用灵感广场作品流，kind=video）
async function publishVideo(t: any) {
  try {
    const { value: title } = await ElMessageBox.prompt('作品标题（留空取提示词前 40 字）', '发布到视频广场', {
      inputValue: t.title || '',
      confirmButtonText: '发布',
      cancelButtonText: '取消',
    });
    const res = await aiHubApi.publishMedia(t.id, { public: true, title, style: 'AI视频' });
    if (res.data.code === 0) {
      ElMessage.success('已发布到视频广场');
      await loadHistory();
    }
  } catch (e: any) {
    if (e === 'cancel' || e?.message === 'cancel') return;
    ElMessage.error(e?.response?.data?.message || '发布失败');
  }
}

async function loadHistory() {
  try {
    const res = await aiHubApi.mediaHistory('video', 1);
    if (res.data.code === 0) history.value = res.data.data.list || [];
  } catch { /* ignore */ }
}

onMounted(async () => {
  await loadHistory();
  // 刷新页面后若还有排队/生成中的任务，恢复轮询（否则用户会以为任务丢了）
  const running = history.value.find((t: any) => t.status === 'pending' || t.status === 'running');
  if (running) {
    latestTaskId.value = running.id;
    startPolling();
  }
});
onUnmounted(stopPolling);
</script>

<style scoped>
/* 复用绘画页的布局语言（.ai-paint / .pd-side / .pd-stream 同名同形） */
.ai-paint { flex: 1; display: flex; min-height: 0; min-width: 0; }

.pd-side {
  width: 320px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--bg-card, #fff);
  border-right: 1px solid var(--border-color, #ebeef5);
  min-height: 0;
}
.pd-side-scroll { flex: 1; min-height: 0; overflow-y: auto; padding: var(--space-3, 12px); scrollbar-width: thin; }
.pd-block { margin-bottom: var(--space-4, 16px); }
.pd-block-title {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: var(--space-2, 8px);
  font-size: var(--text-xs, 12px); font-weight: 600;
  color: var(--text-light, #6b7280);
}
.pd-hint { margin: 6px 0 0; font-size: 11px; line-height: 1.5; color: var(--text-light, #9ca3af); }

/* 时长 */
.pd-durations { display: grid; grid-template-columns: repeat(2, 1fr); gap: 6px; }
.pd-duration {
  display: flex; flex-direction: column; align-items: center; gap: 1px;
  padding: 8px 6px; cursor: pointer;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-md, 10px);
  background: var(--bg-card, #fff); font-family: inherit;
}
.pd-duration b { font-size: var(--text-sm, 13px); color: var(--text-color, #1f2328); }
.pd-duration span { font-size: 10px; color: var(--text-light, #9ca3af); }
.pd-duration.active { border-color: var(--el-color-primary, #6366f1); background: var(--el-color-primary-light-9, #eef2ff); }
.pd-duration.active b { color: var(--el-color-primary, #6366f1); }

/* 画面比例 */
.pd-sizes { display: grid; grid-template-columns: repeat(2, 1fr); gap: 6px; }
.pd-size {
  display: flex; align-items: center; justify-content: center; gap: 6px;
  padding: 8px; cursor: pointer;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-md, 10px);
  background: var(--bg-card, #fff); font-family: inherit;
}
.pd-size b { font-size: var(--text-sm, 13px); color: var(--text-color, #1f2328); }
.pd-size-shape { display: inline-block; border: 1.5px solid var(--text-light, #9ca3af); border-radius: 2px; }
.pd-size-shape.landscape { width: 16px; height: 10px; }
.pd-size-shape.portrait { width: 10px; height: 16px; }
.pd-size.active { border-color: var(--el-color-primary, #6366f1); background: var(--el-color-primary-light-9, #eef2ff); }
.pd-size.active b { color: var(--el-color-primary, #6366f1); }
.pd-size.active .pd-size-shape { border-color: var(--el-color-primary, #6366f1); }

/* 底部 */
.pd-side-foot { flex-shrink: 0; padding: var(--space-3, 12px); border-top: 1px solid var(--border-color, #ebeef5); background: var(--bg-card, #fff); }
.pd-cost { display: flex; justify-content: space-between; gap: 8px; margin-bottom: 8px; font-size: 11px; color: var(--text-light, #6b7280); }
.pd-cost-model { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pd-cost-balance { flex-shrink: 0; }
.pd-generate { width: 100%; }

/* 右侧 */
.pd-stream { flex: 1; min-width: 0; overflow-y: auto; padding: var(--space-5, 24px); scrollbar-width: thin; }
.pd-stream-head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-3, 12px); margin-bottom: var(--space-3, 12px); }
.pd-stream-title { margin: 0 0 2px; font-size: var(--text-lg, 18px); font-weight: 600; color: var(--text-color, #1f2328); }
.pd-stream-sub { margin: 0; font-size: var(--text-xs, 12px); color: var(--text-light, #9ca3af); }

.pd-card {
  margin-bottom: var(--space-4, 16px);
  padding: var(--space-3, 12px);
  border-radius: var(--radius-lg, 16px);
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  box-shadow: var(--shadow-xs, 0 1px 3px rgba(0, 0, 0, 0.04));
}
.pd-card.is-latest { border-color: var(--el-color-primary, #6366f1); box-shadow: 0 4px 16px rgba(99, 102, 241, 0.12); }
.pd-card.is-failed { border-color: rgba(239, 68, 68, 0.4); }

.pd-card-head {
  display: flex; align-items: center; flex-wrap: wrap; gap: 8px;
  padding-bottom: 8px; border-bottom: 1px dashed var(--border-color, #e5e7eb);
  font-size: 11px; color: var(--text-light, #9ca3af);
}
.pd-card-model { font-size: var(--text-sm, 13px); font-weight: 600; color: var(--text-color, #1f2328); }
.pd-card-flag { padding: 0 6px; border-radius: 3px; }
.pd-card-flag.is-ok { color: #16a34a; background: rgba(22, 163, 74, 0.1); }
.pd-card-flag.is-err { color: #ef4444; background: rgba(239, 68, 68, 0.1); }
.pd-card-flag.is-run { color: #2563eb; background: rgba(37, 99, 235, 0.1); }
.pd-card-cost { color: var(--text-light, #9ca3af); }
.pd-card-spacer { flex: 1; }
.pd-card-tool {
  border: none; background: none; padding: 2px 6px; border-radius: 5px;
  font-size: 11px; color: var(--text-light, #6b7280); cursor: pointer; font-family: inherit;
}
.pd-card-tool:hover { background: var(--bg-page, #f5f6f8); color: var(--el-color-primary, #6366f1); }

.pd-card-prompt {
  margin: 8px 0 10px; font-size: var(--text-sm, 13px); line-height: 1.6;
  color: var(--text-color, #374151); cursor: pointer;
  display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2;
  -webkit-box-orient: vertical; overflow: hidden;
}
.pd-card-prompt.is-open { -webkit-line-clamp: unset; line-clamp: unset; display: block; }

.pd-video { width: 100%; max-width: 820px; border-radius: var(--radius-md, 10px); background: #000; display: block; }
.pd-card-error {
  margin: 0; padding: 8px 10px; border-radius: 8px;
  font-size: 12px; color: #b91c1c; background: rgba(239, 68, 68, 0.08);
}

/* 进行中状态条 */
.pd-running {
  padding: 10px 12px;
  border-radius: var(--radius-md, 10px);
  background: var(--bg-page, #f7f8fa);
  border: 1px solid var(--border-color, #eef0f3);
}
.pd-running-head {
  display: flex; align-items: center; justify-content: space-between;
  gap: 8px; margin-bottom: 8px; font-size: 11px; color: var(--text-light, #6b7280);
}
.pd-running-text { font-size: var(--text-sm, 13px); color: var(--text-color, #1f2328); }
.pd-running-live {
  margin-left: 6px; padding: 0 6px; border-radius: 999px;
  font-size: 10px; color: #2563eb; background: rgba(37, 99, 235, 0.1);
}
.pd-running-hint { flex-shrink: 0; }
.pd-card-foot { margin-top: 8px; font-size: 11px; color: var(--text-light, #9ca3af); }

/* 深色 */
[data-theme="dark"] .pd-side,
[data-theme="dark"] .pd-side-foot,
[data-theme="dark"] .pd-card { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .pd-duration,
[data-theme="dark"] .pd-size { background: #0f172a; border-color: #334155; }
[data-theme="dark"] .pd-duration.active,
[data-theme="dark"] .pd-size.active { background: rgba(99, 102, 241, 0.18); }
[data-theme="dark"] .pd-running { background: #0f172a; border-color: #334155; }
[data-theme="dark"] .pd-stream-title,
[data-theme="dark"] .pd-card-model,
[data-theme="dark"] .pd-card-prompt,
[data-theme="dark"] .pd-duration b,
[data-theme="dark"] .pd-size b,
[data-theme="dark"] .pd-running-text { color: #e2e8f0; }

@media (max-width: 900px) {
  .ai-paint { flex-direction: column; }
  .pd-side { width: 100%; max-height: 52vh; border-right: none; border-bottom: 1px solid var(--border-color, #ebeef5); }
  .pd-stream { padding: var(--space-4, 16px); }
}
</style>
