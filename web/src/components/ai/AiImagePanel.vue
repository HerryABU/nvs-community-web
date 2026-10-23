<template>
  <!-- AI 绘画面板（§5.4）：同步出图、按次计费、结果网格 -->
  <div class="ai-media">
    <div class="ai-media-form">
      <div class="ai-media-row">
        <span class="ai-media-label">模型</span>
        <el-select v-model="modelId" size="small" style="width:220px" :disabled="useByok">
          <el-option v-for="m in status.image_models" :key="m.id" :label="modelLabel(m)" :value="m.id" />
        </el-select>
        <el-tooltip content="使用「我的钱包 → 上游密钥」中配置的密钥，不消耗平台代币" placement="top">
          <el-checkbox v-model="useByok" size="small">我的密钥</el-checkbox>
        </el-tooltip>
        <el-input
          v-if="useByok"
          v-model="byokModel"
          size="small"
          style="width:180px"
          placeholder="上游模型 ID"
        />
      </div>
      <div class="ai-media-row">
        <span class="ai-media-label">提示词</span>
        <el-input
          v-model="prompt"
          type="textarea"
          :rows="3"
          resize="none"
          placeholder="描述你想生成的画面，如：雨夜霓虹下的旧书店，暖黄灯光，电影感"
        />
      </div>
      <div class="ai-media-row">
        <span class="ai-media-label">尺寸</span>
        <el-radio-group v-model="size" size="small">
          <el-radio-button value="1024x1024">方形 1024</el-radio-button>
          <el-radio-button value="1024x1792">竖版</el-radio-button>
          <el-radio-button value="1792x1024">横版</el-radio-button>
        </el-radio-group>
        <span class="ai-media-label" style="margin-left:16px">张数</span>
        <el-radio-group v-model="n" size="small">
          <el-radio-button :value="1">1</el-radio-button>
          <el-radio-button :value="2">2</el-radio-button>
          <el-radio-button :value="4">4</el-radio-button>
        </el-radio-group>
      </div>
      <div class="ai-media-row ai-media-actions">
        <span class="muted small">
          单价 {{ currentModel?.price_call || 0 }} {{ status.token_name }} / 张 · 余额 {{ status.balance }}
        </span>
        <el-button type="primary" :loading="generating" :disabled="!prompt.trim()" @click="generate">
          生成（{{ (currentModel?.price_call || 0) * n }} {{ status.token_name }}）
        </el-button>
      </div>
    </div>

    <!-- 结果 -->
    <div v-if="lastImages.length" class="ai-media-result">
      <div class="ai-media-result-label">
        <template v-if="lastFree">本次结果（免费额度，余额 {{ lastBalance }}）</template>
        <template v-else>本次结果（扣除 {{ lastCost }}，余额 {{ lastBalance }}）</template>
      </div>
      <div class="ai-img-grid">
        <img v-for="(img, i) in lastImages" :key="i" :src="img" class="ai-img-item" alt="AI 绘画结果" />
      </div>
    </div>

    <!-- 历史 -->
    <div class="ai-media-history">
      <div class="ai-media-result-label">历史作品</div>
      <el-empty v-if="!history.length" description="暂无绘画记录" :image-size="60" />
      <div v-else class="ai-img-grid">
        <div v-for="t in history" :key="t.id" class="ai-img-wrap">
          <img
            v-for="(img, i) in parseImages(t.result)"
            :key="i"
            :src="img"
            class="ai-img-item"
            :title="t.prompt"
            alt="历史绘画"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { aiHubApi, type AiHubStatus } from '@/api/aiHub';

const props = defineProps<{ status: AiHubStatus }>();

const modelId = ref(0);
const prompt = ref('');
const size = ref('1024x1024');
const n = ref(1);
const generating = ref(false);
const useByok = ref(false);
const byokModel = ref('');
const lastImages = ref<string[]>([]);
const lastCost = ref(0);
const lastFree = ref(false);
const lastBalance = ref(0);
const history = ref<any[]>([]);

// 默认选中首个可用绘画模型
watch(
  () => props.status.image_models,
  (list) => {
    if (!modelId.value && list.length) modelId.value = list[0].id;
  },
  { immediate: true },
);

const currentModel = computed(() =>
  props.status.image_models.find((m) => m.id === modelId.value),
);

function modelLabel(m: { name: string; price_call: number }) {
  return `${m.name}（${m.price_call || '免费'}）`;
}

function parseImages(result: string): string[] {
  try {
    const parsed = JSON.parse(result);
    return parsed.images || [];
  } catch {
    return [];
  }
}

async function generate() {
  if (!modelId.value && !useByok.value) {
    ElMessage.warning('请先选择模型');
    return;
  }
  if (useByok.value && !byokModel.value.trim()) {
    ElMessage.warning('使用我的密钥时需填写上游模型 ID');
    return;
  }
  generating.value = true;
  try {
    const res = await aiHubApi.image({
      model_id: modelId.value,
      prompt: prompt.value.trim(),
      size: size.value,
      n: n.value,
      byok: useByok.value || undefined,
      byok_model: useByok.value ? byokModel.value.trim() : undefined,
    });
    if (res.data.code === 0) {
      lastImages.value = res.data.data.images || [];
      lastCost.value = res.data.data.cost;
      lastFree.value = !!res.data.data.free;
      lastBalance.value = res.data.data.balance_after;
      prompt.value = '';
      await loadHistory();
      ElMessage.success('生成成功');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成失败');
  }
  generating.value = false;
}

async function loadHistory() {
  try {
    const res = await aiHubApi.mediaHistory('image', 1);
    if (res.data.code === 0) history.value = res.data.data.list || [];
  } catch { /* ignore */ }
}

onMounted(loadHistory);
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
.ai-img-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 10px; }
.ai-img-item {
  width: 100%; aspect-ratio: 1; object-fit: cover;
  border-radius: 10px; border: 1px solid var(--el-border-color, #ebeef5);
  background: var(--el-fill-color-light, #f8f9fa);
}
.ai-img-wrap { display: contents; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.75rem; }
</style>
