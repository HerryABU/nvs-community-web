<template>
  <!-- AI 视频面板（§5.4）：异步任务，提交即扣款，失败自动退款 -->
  <div class="ai-media">
    <div class="ai-media-form">
      <div class="ai-media-row">
        <span class="ai-media-label">模型</span>
        <el-select v-model="modelId" size="small" style="width:220px">
          <el-option v-for="m in status.video_models" :key="m.id" :label="modelLabel(m)" :value="m.id" />
        </el-select>
      </div>
      <div class="ai-media-row">
        <span class="ai-media-label">提示词</span>
        <el-input
          v-model="prompt"
          type="textarea"
          :rows="3"
          resize="none"
          placeholder="描述你想生成的视频，如：夕阳下的海面，海浪缓慢起伏，镜头缓慢推进"
        />
      </div>
      <div class="ai-media-row">
        <span class="ai-media-label">时长</span>
        <el-radio-group v-model="duration" size="small">
          <el-radio-button :value="5">5 秒</el-radio-button>
          <el-radio-button :value="10">10 秒</el-radio-button>
        </el-radio-group>
        <span class="ai-media-label" style="margin-left:16px">比例</span>
        <el-radio-group v-model="vsize" size="small">
          <el-radio-button value="1280x720">横屏</el-radio-button>
          <el-radio-button value="720x1280">竖屏</el-radio-button>
        </el-radio-group>
      </div>
      <div class="ai-media-row ai-media-actions">
        <span class="muted small">
          单价 {{ currentModel?.price_call || 0 }} {{ status.token_name }} / 条 · 余额 {{ status.balance }}
        </span>
        <el-button type="primary" :loading="submitting" :disabled="!prompt.trim()" @click="submit">
          提交生成（{{ currentModel?.price_call || 0 }} {{ status.token_name }}）
        </el-button>
      </div>
    </div>

    <!-- 当前任务 -->
    <div v-if="current" class="ai-media-result">
      <div class="ai-media-result-label">
        {{ statusText(current.status) }}
        <span v-if="current.status === 'succeeded'" class="muted small">扣除 {{ current.cost }}</span>
        <span v-else class="muted small">生成中会自动刷新，可离开页面（完成后在历史中查看）</span>
      </div>
      <video v-if="currentVideo" :src="currentVideo" controls class="ai-video-player" />
      <el-alert v-else-if="current.status === 'failed'" :title="current.error || '生成失败（已退款）'" type="error" :closable="false" />
      <el-progress v-else :percentage="progressPct" :stroke-width="8" striped striped-flow />
    </div>

    <!-- 历史 -->
    <div class="ai-media-history">
      <div class="ai-media-result-label">历史视频</div>
      <el-empty v-if="!history.length" description="暂无视频记录" :image-size="60" />
      <div v-else class="ai-video-grid">
        <div v-for="t in history" :key="t.id" class="ai-video-wrap">
          <video
            v-if="parseVideo(t.result)"
            :src="parseVideo(t.result)"
            controls
            class="ai-video-item"
          />
          <div v-else class="ai-video-pending">
            {{ t.status === 'failed' ? '失败（已退款）' : statusText(t.status) }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue';
import { ElMessage } from 'element-plus';
import { aiHubApi, type AiHubStatus } from '@/api/aiHub';

const props = defineProps<{ status: AiHubStatus }>();

const modelId = ref(0);
const prompt = ref('');
const duration = ref(5);
const vsize = ref('1280x720');
const submitting = ref(false);
const current = ref<{ id: number; status: string; video?: string; error?: string; cost: number } | null>(null);
const history = ref<any[]>([]);
let pollTimer: number | undefined;

watch(
  () => props.status.video_models,
  (list) => {
    if (!modelId.value && list.length) modelId.value = list[0].id;
  },
  { immediate: true },
);

const currentModel = computed(() =>
  props.status.video_models.find((m) => m.id === modelId.value),
);

function modelLabel(m: { name: string; price_call: number }) {
  return `${m.name}（${m.price_call || '免费'}）`;
}

function statusText(s: string) {
  const map: Record<string, string> = {
    pending: '排队中', running: '生成中', succeeded: '已完成', failed: '已失败',
  };
  return map[s] || s;
}

function progressPct() {
  if (!current.value) return 0;
  if (current.value.status === 'running') return 60;
  if (current.value.status === 'succeeded') return 100;
  return 10;
}

function parseVideo(result: string): string {
  try {
    const parsed = JSON.parse(result);
    return parsed.video || '';
  } catch {
    return '';
  }
}

const currentVideo = computed(() => current.value?.video || '');

async function submit() {
  if (!modelId.value) {
    ElMessage.warning('请先选择模型');
    return;
  }
  submitting.value = true;
  try {
    const res = await aiHubApi.videoSubmit({
      model_id: modelId.value,
      prompt: prompt.value.trim(),
      duration: duration.value,
      size: vsize.value,
    });
    if (res.data.code === 0) {
      current.value = {
        id: res.data.data.task_id,
        status: res.data.data.status || 'running',
        cost: res.data.data.cost,
      };
      prompt.value = '';
      startPolling();
      ElMessage.success('已提交，生成完成后自动展示');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败');
  }
  submitting.value = false;
}

async function pollOnce() {
  if (!current.value) return;
  const id = current.value.id;
  try {
    const res = await aiHubApi.videoStatus(id);
    if (res.data.code === 0 && current.value && current.value.id === id) {
      current.value.status = res.data.data.status;
      if (res.data.data.video) current.value.video = res.data.data.video;
      if (res.data.data.error) current.value.error = res.data.data.error;
      if (res.data.data.status === 'succeeded' || res.data.data.status === 'failed') {
        stopPolling();
        await loadHistory();
        if (res.data.data.status === 'failed') ElMessage.warning('视频生成失败，已自动退款');
      }
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

async function loadHistory() {
  try {
    const res = await aiHubApi.mediaHistory('video', 1);
    if (res.data.code === 0) history.value = res.data.data.list || [];
  } catch { /* ignore */ }
}

onMounted(loadHistory);
onUnmounted(stopPolling);
</script>

<style scoped>
.ai-media { padding: 16px; max-width: 860px; }
.ai-media-form {
  background: var(--el-fill-color-light, #f8f9fa); border-radius: 12px;
  padding: 14px 16px; margin-bottom: 16px;
}
.ai-media-row { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.ai-media-row:last-child { margin-bottom: 0; }
.ai-media-label { font-size: 0.85rem; font-weight: 600; min-width: 48px; color: var(--text-color, #333); }
.ai-media-actions { justify-content: space-between; }
.ai-media-result { margin-bottom: 16px; }
.ai-media-result-label { font-size: 0.85rem; font-weight: 600; margin-bottom: 8px; color: var(--text-color, #333); }
.ai-video-player { width: 100%; max-width: 560px; border-radius: 10px; background: #000; }
.ai-video-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 10px; }
.ai-video-item { width: 100%; border-radius: 10px; background: #000; }
.ai-video-pending {
  display: flex; align-items: center; justify-content: center;
  min-height: 120px; border-radius: 10px; font-size: 0.8rem;
  color: var(--text-light, #999); background: var(--el-fill-color-light, #f8f9fa);
}
.muted { color: var(--text-light, #999); }
.small { font-size: 0.75rem; font-weight: 400; margin-left: 8px; }
</style>
