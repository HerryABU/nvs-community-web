<template>
  <el-dialog
    :model-value="modelValue"
    title="扫码 / 图片识别"
    width="460px"
    append-to-body
    @update:model-value="(v: boolean) => emit('update:modelValue', v)"
    @closed="stopCamera"
  >
    <el-tabs v-model="tab">
      <el-tab-pane label="摄像头扫码" name="camera">
        <div class="sc-video-wrap">
          <video ref="videoRef" class="sc-video" playsinline muted />
          <div v-if="!scanning" class="sc-video-mask">
            <el-button type="primary" size="small" @click="startCamera">开启摄像头</el-button>
          </div>
        </div>
        <div class="sc-row">
          <el-button v-if="scanning" size="small" @click="stopCamera">停止</el-button>
          <span class="sc-hint">{{ cameraHint }}</span>
        </div>
      </el-tab-pane>

      <el-tab-pane label="图片识别" name="image">
        <div
          class="sc-drop"
          @click="pickFile"
          @dragover.prevent
          @drop.prevent="onDrop"
        >
          <div class="sc-drop-icon">🖼️</div>
          <div class="sc-drop-title">点击选择、拖入图片，或按 Ctrl+V 粘贴二维码</div>
          <div class="sc-drop-sub">支持截图、拍照、网页另存的二维码图片</div>
          <input ref="fileRef" type="file" accept="image/*" hidden @change="onFileChange" />
        </div>
        <div v-if="decoding" class="sc-hint">正在本地解析…</div>
        <div v-else-if="imageError" class="sc-error">{{ imageError }}</div>
      </el-tab-pane>
    </el-tabs>

    <div class="sc-note">
      🔒 图片仅在本机浏览器（Canvas）内解析，<b>不会上传到服务器</b>。
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';
import jsQR from 'jsqr';
import { ElMessage } from 'element-plus';

const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void;
  (e: 'result', text: string): void;
}>();

const tab = ref<'camera' | 'image'>('image');
const fileRef = ref<HTMLInputElement | null>(null);
const videoRef = ref<HTMLVideoElement | null>(null);
const scanning = ref(false);
const decoding = ref(false);
const imageError = ref('');
const cameraHint = ref('把二维码放进取景框内，识别后自动关闭');

let stream: MediaStream | null = null;
let rafID = 0;
let stopped = true;

/* ---------------- 图片识别（本地） ---------------- */
function pickFile() {
  fileRef.value?.click();
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement;
  const f = input.files?.[0];
  if (f) decodeImageFile(f);
  input.value = '';
}

function onDrop(e: DragEvent) {
  const f = e.dataTransfer?.files?.[0];
  if (f) decodeImageFile(f);
}

async function decodeImageFile(file: File) {
  if (!file.type.startsWith('image/')) {
    imageError.value = '请选择图片文件';
    return;
  }
  if (file.size > 12 * 1024 * 1024) {
    imageError.value = '图片过大（上限 12MB）';
    return;
  }
  decoding.value = true;
  imageError.value = '';
  try {
    const bitmap = await createImageBitmap(file);
    const text = decodeBitmap(bitmap);
    bitmap.close?.();
    if (text) {
      emit('result', text);
      emit('update:modelValue', false);
    } else {
      imageError.value = '未识别到二维码，请换一张更清晰、无遮挡的图片';
    }
  } catch {
    imageError.value = '图片解码失败（可能已损坏）';
  }
  decoding.value = false;
}

/** 用 Canvas + jsQR 在本地解码（不发起任何网络请求） */
function decodeBitmap(bitmap: ImageBitmap | HTMLVideoElement): string {
  const max = 1400;
  const w = 'width' in bitmap ? bitmap.width : 0;
  const h = 'height' in bitmap ? bitmap.height : 0;
  const scale = Math.min(1, max / Math.max(w || 1, h || 1));
  const cw = Math.max(1, Math.round(w * scale));
  const ch = Math.max(1, Math.round(h * scale));
  const canvas = document.createElement('canvas');
  canvas.width = cw;
  canvas.height = ch;
  const ctx = canvas.getContext('2d', { willReadFrequently: true });
  if (!ctx) return '';
  ctx.drawImage(bitmap as CanvasImageSource, 0, 0, cw, ch);
  const data = ctx.getImageData(0, 0, cw, ch);
  const hit = jsQR(data.data, cw, ch, { inversionAttempts: 'attemptBoth' });
  return hit?.data || '';
}

/* ---------------- 摄像头扫码 ---------------- */
async function startCamera() {
  imageError.value = '';
  if (!navigator.mediaDevices?.getUserMedia) {
    cameraHint.value = '当前浏览器不支持摄像头，请改用「图片识别」或输入群号';
    return;
  }
  try {
    stream = await navigator.mediaDevices.getUserMedia({
      video: { facingMode: { ideal: 'environment' }, width: { ideal: 1280 }, height: { ideal: 720 } },
      audio: false,
    });
  } catch (e: any) {
    cameraHint.value =
      e?.name === 'NotAllowedError'
        ? '摄像头权限被拒绝，可在浏览器地址栏重新授权'
        : '无法打开摄像头（手机端需 HTTPS 或 localhost）';
    return;
  }
  await nextTick();
  const v = videoRef.value;
  if (!v) return;
  v.srcObject = stream;
  await v.play().catch(() => {});
  scanning.value = true;
  stopped = false;
  cameraHint.value = '对准二维码，识别后自动关闭';
  loop();
}

function loop() {
  if (stopped) return;
  const v = videoRef.value;
  if (v && v.readyState === v.HAVE_ENOUGH_DATA) {
    const text = decodeBitmap(v);
    if (text) {
      emit('result', text);
      ElMessage.success('识别成功');
      emit('update:modelValue', false);
      stopCamera();
      return;
    }
  }
  rafID = requestAnimationFrame(loop);
}

function stopCamera() {
  stopped = true;
  scanning.value = false;
  if (rafID) {
    cancelAnimationFrame(rafID);
    rafID = 0;
  }
  if (stream) {
    stream.getTracks().forEach((t) => t.stop());
    stream = null;
  }
  if (videoRef.value) videoRef.value.srcObject = null;
}

/* ---------------- 粘贴（Ctrl+V） ---------------- */
function onPaste(e: ClipboardEvent) {
  if (!props.modelValue) return;
  const items = e.clipboardData?.items;
  if (!items) return;
  for (const it of Array.from(items)) {
    if (it.type.startsWith('image/')) {
      const f = it.getAsFile();
      if (f) {
        tab.value = 'image';
        decodeImageFile(f);
        e.preventDefault();
      }
      break;
    }
  }
}

watch(
  () => props.modelValue,
  (v) => {
    if (v) {
      window.addEventListener('paste', onPaste);
      cameraHint.value = '把二维码放进取景框内，识别后自动关闭';
      imageError.value = '';
    } else {
      window.removeEventListener('paste', onPaste);
      stopCamera();
    }
  },
);

onBeforeUnmount(() => {
  window.removeEventListener('paste', onPaste);
  stopCamera();
});
</script>

<style scoped>
.sc-video-wrap {
  position: relative;
  width: 100%;
  aspect-ratio: 4 / 3;
  background: #0f172a;
  border-radius: 10px;
  overflow: hidden;
}
.sc-video { width: 100%; height: 100%; object-fit: cover; }
.sc-video-mask {
  position: absolute; inset: 0; display: flex; align-items: center; justify-content: center;
  background: rgba(15, 23, 42, 0.6);
}
.sc-row { display: flex; align-items: center; gap: 10px; margin-top: 10px; }
.sc-hint { font-size: 0.76rem; color: var(--text-light, #94a3b8); }
.sc-error { margin-top: 8px; font-size: 0.78rem; color: #ef4444; }
.sc-drop {
  border: 1.5px dashed var(--border-color, #cbd5e1);
  border-radius: 10px;
  padding: 26px 16px;
  text-align: center;
  cursor: pointer;
  transition: border-color 0.2s, background 0.2s;
}
.sc-drop:hover { border-color: var(--primary-color, #409eff); background: rgba(64, 158, 255, 0.04); }
.sc-drop-icon { font-size: 1.6rem; }
.sc-drop-title { margin-top: 6px; font-size: 0.86rem; }
.sc-drop-sub { margin-top: 4px; font-size: 0.74rem; color: var(--text-light, #94a3b8); }
.sc-note {
  margin-top: 12px; font-size: 0.74rem; color: var(--text-light, #64748b);
  background: var(--fill-light, #f8fafc); border-radius: 8px; padding: 8px 10px;
}
</style>
