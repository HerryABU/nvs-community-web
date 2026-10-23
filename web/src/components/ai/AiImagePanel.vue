<template>
  <!-- AI 绘画面板（§5.4）：文生图 / 混图 / 换脸 + 模型卡片墙 + 五档比例 -->
  <div class="ai-media">
    <!-- 模式 -->
    <div class="ai-mode-tabs">
      <button
        v-for="m in MODES"
        :key="m.key"
        type="button"
        :class="['ai-mode', { active: mode === m.key }]"
        @click="mode = m.key"
      >
        <span class="ai-mode-name">{{ m.name }}</span>
        <span class="ai-mode-desc">{{ m.desc }}</span>
      </button>
    </div>

    <!-- 模型卡片墙 -->
    <div class="ai-model-wall">
      <div
        v-for="m in status.image_models"
        :key="m.id"
        :class="['ai-model-card', { active: modelId === m.id }]"
        @click="modelId = m.id"
      >
        <div class="ai-model-head">
          <span class="ai-model-name">{{ m.name }}</span>
          <span v-if="apiKindTag(m)" class="ai-model-kind">{{ apiKindTag(m) }}</span>
        </div>
        <div class="ai-model-note">{{ m.note || m.model_id }}</div>
        <div class="ai-model-price">{{ m.price_call || '免费' }} {{ status.token_name }}/张</div>
      </div>
      <el-empty v-if="!status.image_models.length" description="暂无可用绘画模型" :image-size="50" />
    </div>

    <div class="ai-media-form">
      <div class="ai-media-row">
        <span class="ai-media-label">提示词</span>
        <el-input
          v-model="prompt"
          type="textarea"
          :rows="3"
          resize="none"
          :placeholder="promptPlaceholder"
        />
      </div>

      <div class="ai-media-row">
        <span class="ai-media-label">比例</span>
        <div class="ai-ratio-row">
          <span
            v-for="r in RATIOS"
            :key="r.value"
            :class="['ai-ratio', { active: ratio === r.value }]"
            @click="ratio = r.value"
          >
            <b>{{ r.value }}</b>
            <i>{{ r.label }}</i>
          </span>
        </div>
      </div>

      <!-- 参考图（混图 / 换脸） -->
      <div v-if="mode !== 't2i'" class="ai-media-row">
        <span class="ai-media-label">参考图</span>
        <div class="ai-refs">
          <div v-for="(img, i) in refImages" :key="i" class="ai-ref-item">
            <img :src="img" alt="参考图" />
            <span class="ai-ref-del" @click="refImages.splice(i, 1)">×</span>
            <span class="ai-ref-index">{{ i + 1 }}</span>
          </div>
          <label v-if="refImages.length < maxRefs" class="ai-ref-add">
            <input type="file" accept="image/png,image/jpeg,image/webp" hidden @change="onPickRef" />
            <span>＋</span>
            <i>{{ refImages.length }}/{{ maxRefs }}</i>
          </label>
        </div>
        <span class="muted small ai-ref-hint">{{ modeHint }}</span>
      </div>

      <div class="ai-media-row">
        <span class="ai-media-label">风格</span>
        <el-input v-model="style" size="small" style="width:180px" placeholder="可选，如 anime / 国风" />
        <span class="ai-media-label" style="margin-left:16px">张数</span>
        <el-radio-group v-model="n" size="small" :disabled="isMJ">
          <el-radio-button :value="1">1</el-radio-button>
          <el-radio-button :value="2">2</el-radio-button>
          <el-radio-button :value="4">4</el-radio-button>
        </el-radio-group>
        <el-button size="small" plain style="margin-left:auto" @click="promptOpen = true">
          <el-icon><Collection /></el-icon> 提示词库
        </el-button>
      </div>

      <div class="ai-media-row ai-media-actions">
        <span class="muted small">
          {{ currentModel?.name || '未选模型' }} · 单价 {{ currentModel?.price_call || 0 }} {{ status.token_name }} / 张 ·
          余额 {{ status.balance }}
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
        <div v-for="t in history" :key="t.id" class="ai-img-cell">
          <img
            v-for="(img, i) in parseImages(t.result)"
            :key="i"
            :src="img"
            class="ai-img-item"
            :title="t.prompt"
            alt="历史绘画"
          />
          <div class="ai-img-actions">
            <button type="button" @click="openPublish(t)">
              {{ t.is_public ? '已发布' : '发布到灵感广场' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 发布弹窗 -->
    <el-dialog v-model="publishVisible" title="发布到灵感广场" width="460px">
      <el-form label-width="64px">
        <el-form-item label="标题">
          <el-input v-model="publishForm.title" maxlength="128" show-word-limit placeholder="留空则取提示词前 40 字" />
        </el-form-item>
        <el-form-item label="风格">
          <el-input v-model="publishForm.style" maxlength="32" placeholder="如 写实摄影 / 国风 / 二次元（用于筛选）" />
        </el-form-item>
      </el-form>
      <p class="muted small">发布后作品与提示词对所有访客可见，可随时取消发布。</p>
      <template #footer>
        <el-button @click="publishVisible = false">取消</el-button>
        <el-button type="primary" :loading="publishing" @click="doPublish(true)">发布</el-button>
        <el-button v-if="publishForm.isPublic" type="danger" plain @click="doPublish(false)">取消发布</el-button>
      </template>
    </el-dialog>

    <AiPromptLibrary v-model="promptOpen" kind="image" :draft="prompt" @pick="onPickPrompt" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { Collection } from '@element-plus/icons-vue';
import { aiHubApi, type AiHubModel, type AiHubStatus } from '@/api/aiHub';
import AiPromptLibrary from '@/components/ai/AiPromptLibrary.vue';

const props = defineProps<{ status: AiHubStatus; initialPrompt?: string }>();

const MODES = [
  { key: 't2i' as const, name: '文生图', desc: '文字直接出图' },
  { key: 'i2i' as const, name: '混图', desc: '参考图 + 描述再创作' },
  { key: 'faceswap' as const, name: '换脸', desc: '2 张图：目标人物 + 面部来源' },
];
const RATIOS = [
  { value: '1:1', label: '头像' },
  { value: '9:16', label: '竖版海报' },
  { value: '2:3', label: '手机壁纸' },
  { value: '3:4', label: '社交媒体' },
  { value: '4:3', label: '横版' },
];

const mode = ref<'t2i' | 'i2i' | 'faceswap'>('t2i');
const modelId = ref(0);
const prompt = ref('');
const ratio = ref('1:1');
const style = ref('');
const n = ref(1);
const generating = ref(false);
const refImages = ref<string[]>([]);
const lastImages = ref<string[]>([]);
const lastCost = ref(0);
const lastFree = ref(false);
const lastBalance = ref(0);
const history = ref<any[]>([]);
const promptOpen = ref(false);

const maxRefs = computed(() => (mode.value === 'faceswap' ? 2 : 3));
const promptPlaceholder = computed(() => {
  if (mode.value === 'faceswap') return '描述换脸后想要的风格/光影；面部替换动作由前两张图决定';
  if (mode.value === 'i2i') return '描述要保留什么、改变什么，如：保留人物姿态，改为赛博朋克夜景霓虹光';
  return '描述你想生成的画面，如：雨夜霓虹下的旧书店，暖黄灯光，电影感';
});
const modeHint = computed(() =>
  mode.value === 'faceswap'
    ? '顺序很重要：第 1 张是目标人物底图，第 2 张提供面部'
    : '支持 1–3 张，仅接受本地上传（不走外链，防 SSRF）',
);

watch(
  () => props.status.image_models,
  (list) => {
    if (!modelId.value && list.length) modelId.value = list[0].id;
  },
  { immediate: true },
);
watch(mode, () => {
  if (refImages.value.length > maxRefs.value) refImages.value = refImages.value.slice(0, maxRefs.value);
});
// 灵感广场「复刻同款」注入提示词
watch(
  () => props.initialPrompt,
  (v) => { if (v) prompt.value = v; },
  { immediate: true },
);

const currentModel = computed<AiHubModel | undefined>(() =>
  props.status.image_models.find((m) => m.id === modelId.value),
);
const apiKind = computed(() => currentModel.value?.api_kind || 'openai');
const isMJ = computed(() => apiKind.value === 'mj');
watch(isMJ, (v) => { if (v) n.value = 1; });

function apiKindTag(m: AiHubModel) {
  const map: Record<string, string> = { mj: 'MJ 代理', gemini: 'Gemini', openai: 'OpenAI 兼容' };
  return map[m.api_kind || 'openai'] || '';
}

function onPickPrompt(content: string) {
  prompt.value = prompt.value.trim() ? `${prompt.value}\n\n${content}` : content;
}

function parseImages(result: string): string[] {
  try {
    return JSON.parse(result).images || [];
  } catch {
    return [];
  }
}

/** 本地图片 → data URL（仅前端读取，不上传服务器） */
function onPickRef(e: Event) {
  const input = e.target as HTMLInputElement;
  const f = input.files?.[0];
  input.value = '';
  if (!f) return;
  if (f.size > 8 * 1024 * 1024) {
    ElMessage.warning('单张参考图上限 8MB');
    return;
  }
  const reader = new FileReader();
  reader.onload = () => {
    const s = String(reader.result || '');
    if (!s.startsWith('data:image/')) {
      ElMessage.warning('仅支持图片文件');
      return;
    }
    refImages.value.push(s);
  };
  reader.readAsDataURL(f);
}

async function generate() {
  if (!modelId.value) {
    ElMessage.warning('请先选择模型');
    return;
  }
  if (mode.value === 'i2i' && !refImages.value.length) {
    ElMessage.warning('混图需先上传参考图');
    return;
  }
  if (mode.value === 'faceswap' && refImages.value.length < 2) {
    ElMessage.warning('换脸需上传 2 张图');
    return;
  }
  generating.value = true;
  try {
    const res = await aiHubApi.image({
      model_id: modelId.value,
      prompt: prompt.value.trim(),
      ratio: ratio.value,
      style: style.value.trim() || undefined,
      n: n.value,
      mode: mode.value,
      ref_images: refImages.value.length ? refImages.value : undefined,
    });
    if (res.data.code === 0) {
      lastImages.value = res.data.data.images || [];
      lastCost.value = res.data.data.cost;
      lastFree.value = !!res.data.data.free;
      lastBalance.value = res.data.data.balance_after;
      await loadHistory();
      ElMessage.success('生成成功');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成失败');
  }
  generating.value = false;
}

// ---------- 发布到灵感广场 ----------
const publishVisible = ref(false);
const publishing = ref(false);
const publishForm = ref({ id: 0, title: '', style: '', isPublic: false });

function openPublish(t: any) {
  publishForm.value = { id: t.id, title: t.title || '', style: t.style || '', isPublic: !!t.is_public };
  publishVisible.value = true;
}

async function doPublish(publicFlag: boolean) {
  publishing.value = true;
  try {
    const res = await aiHubApi.publishMedia(publishForm.value.id, {
      public: publicFlag,
      title: publishForm.value.title,
      style: publishForm.value.style,
    });
    if (res.data.code === 0) {
      ElMessage.success((res.data as any).data?.message || '已更新');
      publishVisible.value = false;
      await loadHistory();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally {
    publishing.value = false;
  }
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
.ai-media { padding: 16px; max-width: 900px; }
/* 模式 */
.ai-mode-tabs { display: flex; gap: 8px; margin-bottom: 12px; }
.ai-mode {
  flex: 1; display: flex; flex-direction: column; gap: 2px; align-items: flex-start;
  padding: 8px 12px; border-radius: 10px; cursor: pointer; text-align: left;
  border: 1px solid var(--el-border-color, #e5e7eb); background: var(--el-bg-color, #fff);
}
.ai-mode.active { border-color: var(--el-color-primary, #409eff); background: var(--el-color-primary-light-9, #ecf5ff); }
.ai-mode-name { font-size: 0.86rem; font-weight: 600; color: var(--text-color, #333); }
.ai-mode-desc { font-size: 0.7rem; color: var(--text-light, #888); }
/* 模型卡片墙 */
.ai-model-wall {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 10px; margin-bottom: 14px;
}
.ai-model-card {
  border: 1px solid var(--el-border-color, #e5e7eb); border-radius: 12px;
  padding: 10px 12px; cursor: pointer; background: var(--el-bg-color, #fff);
  transition: border-color 0.15s, box-shadow 0.15s;
}
.ai-model-card:hover { border-color: var(--el-color-primary-light-5, #a0cfff); }
.ai-model-card.active {
  border-color: var(--el-color-primary, #409eff);
  box-shadow: 0 2px 10px rgba(64, 158, 255, 0.15);
}
.ai-model-head { display: flex; align-items: center; gap: 6px; }
.ai-model-name { font-size: 0.86rem; font-weight: 600; color: var(--text-color, #333); }
.ai-model-kind {
  font-size: 0.62rem; padding: 0 6px; border-radius: 999px;
  background: rgba(64, 158, 255, 0.12); color: #3b82f6; white-space: nowrap;
}
.ai-model-note {
  margin-top: 4px; font-size: 0.7rem; color: var(--text-light, #888); line-height: 1.5;
  overflow: hidden; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical;
}
.ai-model-price { margin-top: 6px; font-size: 0.72rem; color: var(--el-color-primary, #409eff); }
/* 表单 */
.ai-media-form { background: var(--el-fill-color-light, #f8f9fa); border-radius: 12px; padding: 14px 16px; margin-bottom: 16px; }
.ai-media-row { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; flex-wrap: wrap; }
.ai-media-row:last-child { margin-bottom: 0; }
.ai-media-label { font-size: 0.85rem; font-weight: 600; min-width: 48px; color: var(--text-color, #333); }
.ai-media-actions { justify-content: space-between; }
/* 比例 */
.ai-ratio-row { display: flex; gap: 8px; flex-wrap: wrap; }
.ai-ratio {
  display: flex; flex-direction: column; align-items: center; gap: 1px;
  padding: 4px 10px; border-radius: 8px; cursor: pointer;
  border: 1px solid var(--el-border-color, #e5e7eb); background: var(--el-bg-color, #fff);
}
.ai-ratio b { font-size: 0.78rem; color: var(--text-color, #333); }
.ai-ratio i { font-style: normal; font-size: 0.64rem; color: var(--text-light, #999); }
.ai-ratio.active { border-color: var(--el-color-primary, #409eff); background: var(--el-color-primary-light-9, #ecf5ff); }
/* 参考图 */
.ai-refs { display: flex; gap: 8px; align-items: center; }
.ai-ref-item { position: relative; width: 62px; height: 62px; border-radius: 8px; overflow: hidden; }
.ai-ref-item img { width: 100%; height: 100%; object-fit: cover; }
.ai-ref-del {
  position: absolute; top: 0; right: 0; width: 18px; height: 18px; line-height: 16px; text-align: center;
  background: rgba(0, 0, 0, 0.55); color: #fff; font-size: 0.75rem; cursor: pointer; border-bottom-left-radius: 6px;
}
.ai-ref-index {
  position: absolute; bottom: 0; left: 0; padding: 0 5px; font-size: 0.62rem;
  background: rgba(0, 0, 0, 0.55); color: #fff; border-top-right-radius: 6px;
}
.ai-ref-add {
  width: 62px; height: 62px; border-radius: 8px; cursor: pointer;
  border: 1px dashed var(--el-border-color, #c0c4cc); display: flex; flex-direction: column;
  align-items: center; justify-content: center; gap: 2px; background: var(--el-bg-color, #fff);
}
.ai-ref-add span { font-size: 1.1rem; color: var(--text-light, #999); }
.ai-ref-add i { font-style: normal; font-size: 0.62rem; color: var(--text-light, #999); }
.ai-ref-hint { margin-left: 4px; }
/* 结果 */
.ai-media-result { margin-bottom: 16px; }
.ai-media-result-label { font-size: 0.85rem; font-weight: 600; margin-bottom: 8px; color: var(--text-color, #333); }
.ai-img-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 10px; }
.ai-img-cell { position: relative; }
.ai-img-item {
  width: 100%; aspect-ratio: 1; object-fit: cover;
  border-radius: 10px; border: 1px solid var(--el-border-color, #ebeef5);
  background: var(--el-fill-color-light, #f8f9fa);
}
.ai-img-actions {
  position: absolute; left: 6px; right: 6px; bottom: 6px; display: flex; justify-content: center; opacity: 0;
  transition: opacity 0.15s;
}
.ai-img-cell:hover .ai-img-actions { opacity: 1; }
.ai-img-actions button {
  border: 0; cursor: pointer; padding: 3px 10px; border-radius: 999px;
  font-size: 0.68rem; color: #fff; background: rgba(0, 0, 0, 0.6);
}
.muted { color: var(--text-light, #999); }
.small { font-size: 0.75rem; }
</style>
