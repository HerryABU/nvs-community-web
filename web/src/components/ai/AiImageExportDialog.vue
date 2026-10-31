<template>
  <el-dialog
    :model-value="modelValue"
    title="导出图片"
    width="860px"
    align-center
    append-to-body
    class="aigc-export-dialog"
    @update:model-value="emit('update:modelValue', $event)"
    @open="onOpen"
  >
    <div class="aigc-export">
      <!-- 左：裁剪框（只采集坐标，不输出像素） -->
      <div class="aigc-stage-wrap">
        <div ref="stageEl" class="aigc-stage">
          <img ref="imgEl" :src="imageUrl" alt="待导出原图" draggable="false" @load="onImgLoad" />
          <div
            v-if="ready"
            class="aigc-box"
            :style="{ left: box.x + 'px', top: box.y + 'px', width: box.w + 'px', height: box.h + 'px' }"
            @pointerdown.stop="startDrag('move', $event)"
          >
            <span
              v-for="d in HANDLES"
              :key="d"
              class="aigc-handle"
              :class="'h-' + d"
              @pointerdown.stop="startDrag(d, $event)"
            />
          </div>
        </div>
        <p class="aigc-hint">
          拖动选框移动，拖四角缩放；坐标会换算成原图像素交给服务端裁剪。
        </p>
      </div>

      <!-- 右：参数 -->
      <div class="aigc-form">
        <div class="aigc-field">
          <label class="aigc-label">输出格式</label>
          <el-radio-group v-model="format" size="small">
            <el-radio-button value="jpeg">JPEG</el-radio-button>
            <el-radio-button value="png">PNG</el-radio-button>
          </el-radio-group>
          <p class="aigc-hint">原图与合成都在服务端处理，浏览器不重新编码。</p>
        </div>

        <div class="aigc-field">
          <label class="aigc-label">输出尺寸</label>
          <div class="aigc-size">
            <el-input-number v-model="targetW" :min="16" :max="20000" :step="64" size="small" controls-position="right" />
            <span class="aigc-x">×</span>
            <el-input-number v-model="targetH" :min="16" :max="20000" :step="64" size="small" controls-position="right" />
            <el-button link size="small" @click="resetSize">按选区</el-button>
          </div>
          <p class="aigc-hint">选区 {{ cropNat.w }} × {{ cropNat.h }} 像素（原图口径）</p>
        </div>

        <div class="aigc-field">
          <div class="aigc-row">
            <span class="aigc-label">显式标识（角标）</span>
            <el-tag v-if="!opts.waiver_enabled" size="small" type="success" effect="plain">
              会烧录「{{ opts.label_text }}」
            </el-tag>
            <el-switch v-else v-model="waive" active-text="申请不含" inactive-text="默认含" inline-prompt />
          </div>
          <template v-if="opts.waiver_enabled && waive">
            <p class="aigc-agreement">{{ opts.waiver_agreement }}</p>
            <p class="aigc-hint">
              请逐字输入下面这句话以确认：
              <code class="aigc-code">{{ opts.waiver_confirm_text }}</code>
            </p>
            <el-input
              v-model="waiverText"
              type="textarea"
              :rows="2"
              placeholder="在此手动输入确认语句"
              maxlength="200"
            />
          </template>
          <p v-else class="aigc-hint">
            隐式标识（AIGC 元数据）无论是否豁免都会写入文件。
          </p>
        </div>

        <div v-if="opts.prompt_attach_enabled" class="aigc-field">
          <div class="aigc-row">
            <span class="aigc-label">附带提示词</span>
            <el-switch v-model="includePrompt" />
          </div>
          <p class="aigc-hint">该信息将写入文件，分享图片时可能被他人读取。</p>
          <el-input v-if="includePrompt" v-model="promptText" type="textarea" :rows="2" placeholder="将随文件写入提示词" />
        </div>

        <el-alert
          v-if="lastResult"
          :closable="false"
          type="success"
          show-icon
          class="aigc-result"
          :title="`已导出${lastResult.labeled ? '（含角标）' : '（不含角标）'}`"
          :description="`ProduceID：${lastResult.produceId}${lastResult.note ? '　' + lastResult.note : ''}`"
        />
        <p v-if="error" class="aigc-error">{{ error }}</p>
      </div>
    </div>

    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="busy" :disabled="!ready" @click="doExport">导出并下载</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { aigcApi, type AigcOptions } from '@/api/aigc';

const props = defineProps<{
  modelValue: boolean;
  /** 预览用图片地址（远程或站内），仅用于画面与取坐标，不上传 */
  imageUrl: string;
  /** 二选一：任务 id（AI 出图）或原图 id */
  taskId?: number;
  assetId?: number;
  /** 同一任务多张图时导出第几张（0 起） */
  imageIndex?: number;
  /** 出图时的提示词，勾选「附带」时默认带出 */
  prompt?: string;
}>();

const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>();

const HANDLES = ['nw', 'ne', 'sw', 'se'] as const;
type Dir = 'move' | (typeof HANDLES)[number];

const stageEl = ref<HTMLElement>();
const imgEl = ref<HTMLImageElement>();
const natW = ref(0);
const natH = ref(0);
const stageW = ref(0);
const stageH = ref(0);
const ready = ref(false);

const box = reactive({ x: 0, y: 0, w: 0, h: 0 });
const format = ref<'jpeg' | 'png'>('jpeg');
const targetW = ref(1024);
const targetH = ref(1024);
const waive = ref(false);
const waiverText = ref('');
const includePrompt = ref(false);
const promptText = ref('');
const busy = ref(false);
const error = ref('');
const lastResult = ref<{ produceId: string; labeled: boolean; note: string } | null>(null);

const opts = ref<AigcOptions>({
  enabled: true,
  waiver_enabled: false,
  waiver_agreement: '',
  waiver_confirm_text: '',
  prompt_attach_enabled: false,
  label_text: 'AI生成',
  max_output_pixels: 40000000,
});

/** 选区换算成原图像素（服务端要的就是这个） */
const cropNat = computed(() => {
  if (!stageW.value) return { x: 0, y: 0, w: 0, h: 0 };
  const s = natW.value / stageW.value;
  return {
    x: Math.round(box.x * s),
    y: Math.round(box.y * s),
    w: Math.round(box.w * s),
    h: Math.round(box.h * s),
  };
});

async function onOpen() {
  error.value = '';
  lastResult.value = null;
  waive.value = false;
  waiverText.value = '';
  includePrompt.value = !!props.prompt;
  promptText.value = props.prompt || '';
  try {
    const res = await aigcApi.options();
    if (res.data?.data) opts.value = res.data.data;
  } catch {
    /* 取不到就用默认：默认显示会烧角标，安全侧 */
  }
  await nextTick();
  if (imgEl.value?.complete) onImgLoad();
}

function onImgLoad() {
  const el = imgEl.value;
  const stage = stageEl.value;
  if (!el || !stage) return;
  natW.value = el.naturalWidth;
  natH.value = el.naturalHeight;
  // 以图片实际渲染尺寸为准（stage 用 inline-block 包住它），这样选框坐标不用猜缩放
  stageW.value = el.clientWidth;
  stageH.value = el.clientHeight;
  const size = Math.min(stageW.value, stageH.value);
  box.w = Math.round(size * 0.8);
  box.h = Math.round(size * 0.8);
  box.x = Math.round((stageW.value - box.w) / 2);
  box.y = Math.round((stageH.value - box.h) / 2);
  ready.value = stageW.value > 0 && natW.value > 0;
  resetSize();
}

function resetSize() {
  targetW.value = Math.max(16, cropNat.value.w);
  targetH.value = Math.max(16, cropNat.value.h);
}

// ---------- 拖拽 ----------
let dragDir: Dir = 'move';
let startX = 0;
let startY = 0;
let startBox = { x: 0, y: 0, w: 0, h: 0 };

const MIN = 24;

function startDrag(dir: Dir, e: PointerEvent) {
  dragDir = dir;
  startX = e.clientX;
  startY = e.clientY;
  startBox = { ...box };
  (e.target as HTMLElement).setPointerCapture?.(e.pointerId);
  window.addEventListener('pointermove', onMove);
  window.addEventListener('pointerup', onUp, { once: true });
}

function onMove(e: PointerEvent) {
  const dx = e.clientX - startX;
  const dy = e.clientY - startY;
  const s = startBox;
  if (dragDir === 'move') {
    box.x = clamp(s.x + dx, 0, stageW.value - s.w);
    box.y = clamp(s.y + dy, 0, stageH.value - s.h);
    return;
  }
  // 四角缩放：左上角移动时同时改 x/y 与宽高
  let left = s.x;
  let top = s.y;
  let right = s.x + s.w;
  let bottom = s.y + s.h;
  if (dragDir.includes('w')) left = clamp(s.x + dx, 0, right - MIN);
  if (dragDir.includes('e')) right = clamp(s.x + s.w + dx, left + MIN, stageW.value);
  if (dragDir.includes('n')) top = clamp(s.y + dy, 0, bottom - MIN);
  if (dragDir.includes('s')) bottom = clamp(s.y + s.h + dy, top + MIN, stageH.value);
  box.x = Math.round(left);
  box.y = Math.round(top);
  box.w = Math.round(right - left);
  box.h = Math.round(bottom - top);
}

function onUp() {
  window.removeEventListener('pointermove', onMove);
  resetSize();
}

function clamp(v: number, lo: number, hi: number) {
  return Math.max(lo, Math.min(hi, v));
}

// 选区变了就同步输出尺寸（除非用户手动改过）
watch(
  () => [box.w, box.h],
  () => {
    if (!ready.value) return;
    resetSize();
  },
);

// ---------- 导出 ----------
function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 4000);
}

async function doExport() {
  error.value = '';
  lastResult.value = null;
  if (waive.value && !waiverText.value.trim()) {
    error.value = '请先手动输入确认语句';
    return;
  }
  if (targetW.value * targetH.value > opts.value.max_output_pixels) {
    error.value = `输出像素超出本站上限（${opts.value.max_output_pixels.toLocaleString()}），请调小尺寸`;
    return;
  }
  busy.value = true;
  try {
    const c = cropNat.value;
    const res = await aigcApi.exportImage({
      asset_id: props.assetId || undefined,
      task_id: props.taskId || undefined,
      image_index: props.imageIndex ?? 0,
      crop_x: c.x,
      crop_y: c.y,
      crop_w: c.w,
      crop_h: c.h,
      target_w: targetW.value,
      target_h: targetH.value,
      format: format.value,
      waive: waive.value,
      waiver_text: waive.value ? waiverText.value : undefined,
      include_prompt: includePrompt.value,
      prompt: includePrompt.value ? promptText.value : undefined,
    });
    saveBlob(res.blob, res.filename);
    lastResult.value = {
      produceId: res.produceId,
      labeled: res.labeled,
      note: res.noteCode === 'webp_fallback_jpeg' ? '（WebP 已按 JPEG 输出）' : '',
    };
    ElMessage.success('已导出，标识已写入文件');
  } catch (e: any) {
    error.value = e?.message || '导出失败';
  } finally {
    busy.value = false;
  }
}
</script>

<style scoped>
.aigc-export {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 18px;
}
.aigc-stage-wrap { min-width: 0; }
.aigc-stage {
  position: relative;
  display: inline-block;
  max-width: 100%;
  line-height: 0;
  border-radius: 10px;
  overflow: hidden;
  background: var(--bg-page, #f3f4f6);
  user-select: none;
  touch-action: none;
}
.aigc-stage img {
  display: block;
  max-width: 100%;
  max-height: 420px;
  -webkit-user-drag: none;
}
.aigc-box {
  position: absolute;
  border: 1.5px solid var(--el-color-primary, #409eff);
  box-shadow: 0 0 0 9999px rgba(15, 23, 42, 0.42);
  cursor: move;
  touch-action: none;
}
.aigc-handle {
  position: absolute;
  width: 12px;
  height: 12px;
  background: var(--el-color-primary, #409eff);
  border: 2px solid #fff;
  border-radius: 50%;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
}
.h-nw { left: -7px; top: -7px; cursor: nwse-resize; }
.h-ne { right: -7px; top: -7px; cursor: nesw-resize; }
.h-sw { left: -7px; bottom: -7px; cursor: nesw-resize; }
.h-se { right: -7px; bottom: -7px; cursor: nwse-resize; }

.aigc-form { display: flex; flex-direction: column; gap: 16px; }
.aigc-field { display: flex; flex-direction: column; gap: 6px; }
.aigc-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.aigc-label { font-size: 0.82rem; font-weight: 500; color: var(--text-main, #1f2937); }
.aigc-hint { margin: 0; font-size: 0.72rem; line-height: 1.65; color: var(--text-light, #9ca3af); }
.aigc-size { display: flex; align-items: center; gap: 6px; }
.aigc-x { color: var(--text-light, #9ca3af); font-size: 0.8rem; }
.aigc-agreement {
  margin: 0;
  padding: 8px 10px;
  border-radius: 6px;
  font-size: 0.72rem;
  line-height: 1.7;
  color: var(--el-color-warning-dark-2, #b88230);
  background: var(--el-color-warning-light-9, #fdf6ec);
  border: 1px solid var(--el-color-warning-light-5, #f5dab1);
}
.aigc-code {
  padding: 0 4px;
  border-radius: 3px;
  background: var(--bg-page, #eef2f7);
  font-size: 0.72rem;
}
.aigc-error { margin: 0; font-size: 0.76rem; color: var(--el-color-danger, #f56c6c); }
.aigc-result :deep(.el-alert__description) { word-break: break-all; }

@media (max-width: 860px) {
  .aigc-export { grid-template-columns: 1fr; }
}
</style>
