<template>
  <div class="runner-page">
    <div class="runner-toolbar">
      <el-button text @click="$router.back()"><el-icon><ArrowLeft /></el-icon> 返回</el-button>
      <span class="runner-title">{{ projectName || '扩展应用' }}</span>
      <div class="runner-actions">
        <el-tag v-if="projectData.allow_wasm" type="warning" size="small">WASM</el-tag>
        <el-tag v-if="projectData.port > 0" type="danger" size="small">端口:{{ projectData.port }}</el-tag>
        <el-button size="small" text @click="reload"><el-icon><Refresh /></el-icon></el-button>
      </div>
    </div>
    <div class="runner-body" v-loading="loading">
      <div v-if="error" class="runner-error">
        <el-result icon="error" title="加载失败" :sub-title="error">
          <template #extra><el-button @click="$router.back()">返回</el-button></template>
        </el-result>
      </div>
      <iframe
        v-else-if="previewSrc"
        :key="iframeKey"
        :src="previewSrc"
        :sandbox="sandboxPolicy"
        class="runner-iframe"
        allow="cross-origin-isolated"
        @load="onLoad"
        @error="onError"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useRoute } from 'vue-router';
import { ArrowLeft, Refresh } from '@element-plus/icons-vue';
import api from '@/api/index';

const route = useRoute();
const htmlId = computed(() => Number(route.params.htmlId));
const previewSrc = ref('');
const projectName = ref('');
const projectData = ref<any>({});
const loading = ref(true);
const error = ref('');
const iframeKey = ref(0);
const sandboxPolicy = ref('allow-scripts allow-same-origin allow-forms allow-popups');

onMounted(async () => {
  try {
    const r = await api.get(`/userhtmls/${htmlId.value}`);
    if (r.data.code === 0) {
      projectData.value = r.data.data.html || {};
      projectName.value = projectData.value.name || '';
    }
  } catch { /* */ }
  // 始终使用虚拟路径加载，完全不暴露真实API路径
  previewSrc.value = `/app/${htmlId.value}`;
  loading.value = false;
});

function reload() {
  iframeKey.value++;
  loading.value = true;
  error.value = '';
}

function onLoad() { loading.value = false; }
function onError() { loading.value = false; error.value = '内容加载失败'; }
</script>

<style scoped>
.runner-page { display:flex;flex-direction:column;height:100vh;background:var(--bg-color) }
.runner-toolbar { display:flex;align-items:center;justify-content:space-between;padding:6px 16px;background:var(--card-bg);border-bottom:1px solid var(--border-color);height:44px;flex-shrink:0 }
.runner-title { font-size:.95rem;font-weight:500;color:var(--text-color);flex:1;text-align:center }
.runner-actions { display:flex;align-items:center;gap:8px }
.runner-body { flex:1;position:relative;overflow:hidden }
.runner-iframe { width:100%;height:100%;border:none;display:block }
.runner-error { display:flex;align-items:center;justify-content:center;height:100% }
</style>
