<template>
  <div class="sandbox-preview">
    <div class="preview-toolbar">
      <span class="preview-title">{{ title || '沙盒预览' }}</span>
      <div class="preview-actions">
        <el-button size="small" @click="reload" :icon="RefreshRight">刷新</el-button>
        <el-button size="small" @click="openNewWindow" :icon="Link">新窗口</el-button>
      </div>
    </div>
    <div class="preview-frame-wrapper" :style="{ height: height }">
      <iframe
        ref="iframeRef"
        :src="src"
        :sandbox="sandboxPolicy"
        :style="{ width: '100%', height: '100%' }"
        loading="lazy"
        allow="cross-origin-isolated"
        @load="onLoad"
        @error="onError"
      />
      <div v-if="loading" class="preview-loading">
        <el-icon class="is-loading"><Loading /></el-icon>
        <span>加载中...</span>
      </div>
      <div v-if="error" class="preview-error">
        <el-icon><WarningFilled /></el-icon>
        <span>{{ error }}</span>
      </div>
    </div>
    <div class="sandbox-badge" v-if="sandboxPolicy">
      🔒 {{ sandboxPolicy }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { RefreshRight, Link, Loading, WarningFilled } from '@element-plus/icons-vue';

const props = withDefaults(defineProps<{
  src: string;
  title?: string;
  height?: string;
  sandboxPolicy?: string;
}>(), {
  height: '500px',
  sandboxPolicy: 'allow-scripts allow-same-origin',
});

const iframeRef = ref<HTMLIFrameElement>();
const loading = ref(true);
const error = ref('');

watch(() => props.src, () => {
  loading.value = true;
  error.value = '';
});

function reload() {
  if (iframeRef.value) {
    loading.value = true;
    error.value = '';
    iframeRef.value.src = props.src;
  }
}

function openNewWindow() {
  window.open(props.src, '_blank', 'noopener,noreferrer');
}

function onLoad() {
  loading.value = false;
}

function onError() {
  loading.value = false;
  error.value = '内容加载失败，请检查链接是否有效';
}
</script>

<style scoped>
.sandbox-preview {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--bg-color);
  position: relative;
}

.preview-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 14px;
  background: var(--card-bg);
  border-bottom: 1px solid var(--border-color);
}

.preview-title {
  font-size: 0.9rem;
  font-weight: 500;
  color: var(--text-color);
}

.preview-actions {
  display: flex;
  gap: 6px;
}

.preview-frame-wrapper {
  position: relative;
  background: #fff;
}

[data-theme="dark"] .preview-frame-wrapper {
  background: #1a1a2e;
}

.preview-frame-wrapper iframe {
  border: none;
  display: block;
}

.preview-loading,
.preview-error {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: var(--text-secondary);
  font-size: 0.9rem;
}

.preview-error {
  color: var(--el-color-danger);
}

.sandbox-badge {
  position: absolute;
  bottom: 6px;
  right: 10px;
  background: rgba(0,0,0,0.65);
  color: #aaa;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-family: monospace;
  pointer-events: none;
}
</style>
