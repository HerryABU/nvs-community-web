<template>
  <el-dialog
    v-model="visible"
    title="发布到 HTML 广场"
    :width="dialogWidth"
    append-to-body
    destroy-on-close
    class="ai-html-publish-dialog"
  >
    <div class="ap-body">
      <div class="ap-preview-hint">
        <span class="ap-chip">index.html</span>
        <span>{{ sizeText }}</span>
        <span class="ap-sep">·</span>
        <span>单文件发布，CSS / JS 需内联在这份 HTML 里</span>
      </div>

      <el-form label-position="top" class="ap-form">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" maxlength="128" show-word-limit placeholder="给这个页面起个名字" />
        </el-form-item>
        <el-form-item label="简介">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="2"
            maxlength="512"
            show-word-limit
            placeholder="它是做什么的？（会显示在广场卡片上）"
          />
        </el-form-item>
        <div class="ap-switches">
          <el-checkbox v-model="form.isPublic">广场展示</el-checkbox>
          <el-checkbox v-model="form.isDownloadable">允许他人下载</el-checkbox>
        </div>
      </el-form>

      <el-alert type="info" :closable="false" show-icon class="ap-note">
        <template #title>
          上传后会走平台的内容安全预检与人工复核：命中断层级规则会被封存待管理员处置；
          中低危告警或公开额度超限时，需要管理员审核通过才会在广场出现。
        </template>
      </el-alert>

      <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
      <el-alert v-if="warning" :title="warning" type="warning" :closable="false" show-icon />
      <el-alert v-if="done" :title="done" type="success" :closable="false" show-icon />
    </div>

    <template #footer>
      <div class="ap-foot">
        <el-button v-if="done && managerPath" text type="primary" @click="goManager">查看我的 HTML</el-button>
        <span class="ap-grow" />
        <el-button @click="visible = false">{{ done ? '关闭' : '取消' }}</el-button>
        <el-button v-if="!done" type="primary" :loading="busy" :disabled="!canSubmit" @click="submit">
          {{ busy ? '发布中…' : '发布' }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { htmlApi } from '@/api/home/frame';
import { createZip } from '@/utils/miniZip';
import { useAuthStore } from '@/stores/auth';

const props = defineProps<{
  modelValue: boolean;
  code: string;
  /** 建议的默认标题（通常取 AI 回复的片段） */
  suggestedName?: string;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void;
  (e: 'published', id: number): void;
}>();

const router = useRouter();
const auth = useAuthStore();

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v),
});

const dialogWidth = computed(() => (window.innerWidth <= 640 ? '94%' : '520px'));

const form = reactive({
  name: '',
  description: '',
  isPublic: true,
  isDownloadable: true,
});

const busy = ref(false);
const error = ref('');
const warning = ref('');
const done = ref('');
const managerPath = ref('');

/** 体积上限：后端 ZIP 限 20MB，单文件 HTML 留足余量按 8MB 卡住 */
const MAX_BYTES = 8 * 1024 * 1024;

const sizeText = computed(() => {
  const n = new Blob([props.code || '']).size;
  return n > 1024 * 1024 ? `${(n / 1048576).toFixed(2)} MB` : `${(n / 1024).toFixed(1)} KB`;
});

const canSubmit = computed(() => !!form.name.trim() && new Blob([props.code || '']).size <= MAX_BYTES);

function defaultName(): string {
  const s = (props.suggestedName || '').replace(/\s+/g, ' ').trim();
  if (s) return s.slice(0, 60);
  const d = new Date();
  const p = (n: number) => String(n).padStart(2, '0');
  return `AI 生成页面 ${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

// 每次打开重置状态，避免上一轮的错误/成功提示残留
watch(
  () => props.modelValue,
  (open) => {
    if (!open) return;
    busy.value = false;
    error.value = '';
    warning.value = '';
    done.value = '';
    managerPath.value = '';
    form.name = defaultName();
    form.description = '';
    form.isPublic = true;
    form.isDownloadable = true;
  },
);

async function submit() {
  error.value = '';
  warning.value = '';
  done.value = '';

  if (!auth.isLoggedIn) {
    error.value = '请先登录后再发布';
    return;
  }
  const html = (props.code || '').trim();
  if (!html) {
    error.value = '这段代码是空的，无法发布';
    return;
  }
  if (new Blob([html]).size > MAX_BYTES) {
    error.value = `单文件超过 ${MAX_BYTES / 1048576}MB，请拆分成更小的页面再发布`;
    return;
  }

  busy.value = true;
  try {
    // 打成一个 STORE 模式的单文件 ZIP，复用广场既有的上传/预检/封存管线
    const zip = createZip([{ name: 'index.html', data: html }]);
    const fd = new FormData();
    fd.append('name', form.name.trim());
    fd.append('description', form.description.trim());
    fd.append('entry_file', 'index.html');
    fd.append('allow_wasm', 'false');
    fd.append('is_public', form.isPublic ? 'true' : 'false');
    fd.append('is_downloadable', form.isDownloadable ? 'true' : 'false');
    fd.append('run_mode', 'sandbox');
    fd.append('file', zip, 'ai-page.zip');

    const r = await htmlApi.upload(fd);
    const body = r.data || {};
    if (body.code !== 0) {
      error.value = body.message || '发布失败';
      return;
    }
    const d = body.data || {};
    const id = Number(d.html?.id || 0);
    if (d.quarantined) {
      warning.value = d.message || '安全预检未通过，文件已封存，等待管理员处置';
      ElMessage.warning('已提交，但被安全预检拦截，需管理员复核');
    } else {
      done.value = d.public_note || '发布成功！';
      ElMessage.success(d.public_note ? '已提交，等待审核' : '发布成功');
    }
    managerPath.value = '/htmls';
    if (id) emit('published', id);
  } catch (e: any) {
    const msg = e?.response?.data?.message;
    error.value = msg || e?.message || '网络异常，发布失败';
  } finally {
    busy.value = false;
  }
}

function goManager() {
  visible.value = false;
  router.push(managerPath.value || '/htmls');
}
</script>

<style scoped>
.ap-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.ap-preview-hint {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--text-light, #909399);
  padding: 8px 10px;
  border: 1px dashed var(--el-border-color, #ebeef5);
  border-radius: 8px;
  background: var(--bg-page, #fafbfc);
}
.ap-chip {
  font-family: var(--font-mono, Consolas, monospace);
  background: rgba(64, 158, 255, 0.12);
  color: var(--el-color-primary, #409eff);
  padding: 2px 6px;
  border-radius: 4px;
}
.ap-sep {
  opacity: 0.5;
}
.ap-form :deep(.el-form-item) {
  margin-bottom: 12px;
}
.ap-switches {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
}
.ap-note {
  font-size: 12px;
  line-height: 1.6;
}
.ap-foot {
  display: flex;
  align-items: center;
  gap: 8px;
}
.ap-grow {
  flex: 1;
}
</style>
