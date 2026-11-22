<template>
  <div class="plaza-wrap">
    <div class="plaza-head">
      <div>
        <h2>自托管广场</h2>
        <p>自托管项目与自定义 HTML 扩展应用统一展示；自托管源码与产物默认需授权才能查看。</p>
      </div>
      <el-input v-model="keyword" placeholder="搜索项目 / 应用" clearable style="width: 240px" @keyup.enter="load()" />
    </div>

    <el-tabs v-model="type" class="plaza-tabs" @tab-change="load()">
      <el-tab-pane label="全部" name="all" />
      <el-tab-pane label="自托管项目" name="selfhost" />
      <el-tab-pane label="扩展应用" name="html" />
    </el-tabs>

    <el-row :gutter="16" v-loading="loading">
      <el-col v-for="e in entries" :key="`${e.type}-${e.id}`" :xs="24" :sm="12" :md="8" :lg="6">
        <el-card class="plaza-card" shadow="hover">
          <div class="plaza-title">
            {{ e.title }}
            <el-tag size="small" :type="e.type === 'html' ? 'warning' : 'primary'" class="plaza-type">
              {{ e.type === 'html' ? '扩展应用' : '自托管' }}
            </el-tag>
          </div>
          <div class="plaza-meta">
            <el-tag size="small" effect="plain">{{ e.language }} {{ e.version }}</el-tag>
            <span v-if="e.author" class="plaza-author">@{{ e.author }}</span>
          </div>
          <div class="plaza-desc">{{ e.description || '暂无简介' }}</div>
          <div class="plaza-actions">
            <el-button v-if="e.url" type="primary" size="small" tag="a" :href="e.url" target="_blank">
              {{ e.type === 'html' ? '运行应用' : '访问服务' }}
            </el-button>
            <template v-if="e.type === 'selfhost'">
              <el-button v-if="e.can_source" size="small" @click="download(e, 'source')">查看源码</el-button>
              <el-button v-else-if="authStore.isLoggedIn" size="small" @click="openRequest(e, 'source')">申请源码</el-button>
              <el-button v-if="e.binary_name && e.can_binary" size="small" @click="download(e, 'binary')">可执行文件</el-button>
              <el-button v-else-if="e.binary_name && authStore.isLoggedIn" size="small" @click="openRequest(e, 'binary')">申请产物</el-button>
            </template>
          </div>
        </el-card>
      </el-col>
    </el-row>
    <el-empty v-if="!loading && !entries.length" description="还没有公开的项目或应用" />

    <el-dialog v-model="requestVisible" title="申请文件访问权限" :width="isMobile ? '94%' : '480px'" destroy-on-close>
      <p class="plaza-tip">源码与可执行文件默认不公开。提交申请后由项目所有者审批，未获批准无法下载。</p>
      <el-form label-width="90px">
        <el-form-item label="申请类型">
          <el-tag>{{ requestKind === 'source' ? '源码包' : '可执行文件' }}</el-tag>
        </el-form-item>
        <el-form-item label="用途说明">
          <el-input v-model="requestNote" type="textarea" :rows="3" maxlength="500" placeholder="简单说明用途，便于所有者判断" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="requestVisible = false">取消</el-button>
        <el-button type="primary" @click="doRequest()">提交申请</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { useAuthStore } from '@/stores/auth';
import { useIsMobile } from '@/utils/mobile';
import { selfhostApi, type PlazaEntry } from '@/api/selfhost';

const authStore = useAuthStore();
const isMobile = useIsMobile();
const requestVisible = ref(false);
const requestEntry = ref<PlazaEntry | null>(null);
const requestKind = ref<'source' | 'binary'>('source');
const requestNote = ref('');
const keyword = ref('');
const type = ref('all');
const loading = ref(false);
const entries = ref<PlazaEntry[]>([]);

async function load() {
  loading.value = true;
  try {
    const res = await selfhostApi.plazaAll(keyword.value, type.value);
    entries.value = res.data.data.entries || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

async function download(e: PlazaEntry, kind: 'source' | 'binary') {
  try {
    await selfhostApi.downloadFile(e.id, kind, kind === 'source' ? e.source_name || '' : e.binary_name || '');
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.message || '下载失败');
  }
}

function openRequest(e: PlazaEntry, kind: 'source' | 'binary') {
  requestEntry.value = e;
  requestKind.value = kind;
  requestNote.value = '';
  requestVisible.value = true;
}

async function doRequest() {
  if (!requestEntry.value) return;
  try {
    await selfhostApi.requestAccess(requestEntry.value.id, requestKind.value, requestNote.value);
    requestVisible.value = false;
    ElMessage.success('申请已提交，等待项目所有者审批');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '申请失败');
  }
}

onMounted(load);
</script>

<style scoped>
.plaza-wrap { padding: 20px; }
.plaza-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.plaza-head h2 { margin: 0 0 4px; }
.plaza-head p { margin: 0; color: var(--el-text-color-secondary); font-size: 13px; }
.plaza-tabs { margin-bottom: 8px; }
.plaza-card { margin-bottom: 16px; min-height: 170px; }
.plaza-title { font-weight: 700; font-size: 16px; margin-bottom: 8px; }
.plaza-type { margin-left: 6px; }
.plaza-meta { display: flex; gap: 6px; align-items: center; margin-bottom: 10px; }
.plaza-author { font-size: 12px; color: var(--el-text-color-secondary); }
.plaza-desc { font-size: 13px; color: var(--el-text-color-regular); min-height: 40px; }
.plaza-actions { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 12px; }
.plaza-tip { font-size: 13px; color: var(--el-text-color-secondary); line-height: 1.6; margin: 0 0 12px; }

@media (max-width: 767px) {
  .plaza-wrap { padding: 10px 8px; }
  .plaza-head { flex-direction: column; align-items: stretch; gap: 10px; }
  .plaza-head .el-input { width: 100% !important; }
  .plaza-card { min-height: 0; }
  .plaza-actions .el-button { flex: 1 1 45%; margin-left: 0 !important; }
}
</style>
