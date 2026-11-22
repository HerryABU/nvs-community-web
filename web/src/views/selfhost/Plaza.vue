<template>
  <div class="plaza-wrap">
    <div class="plaza-head">
      <div>
        <h2>自托管广场</h2>
        <p>以下项目均由作者自行部署，源码与产物已经站长审核；点击访问其服务。</p>
      </div>
      <el-input v-model="keyword" placeholder="搜索项目" clearable style="width: 240px" @keyup.enter="load()" />
    </div>

    <el-row :gutter="16" v-loading="loading">
      <el-col v-for="p in projects" :key="p.id" :xs="24" :sm="12" :md="8" :lg="6">
        <el-card class="plaza-card" shadow="hover">
          <div class="plaza-title">{{ p.name }}</div>
          <div class="plaza-meta">
            <el-tag size="small">{{ p.language }} {{ p.runtime?.version || '' }}</el-tag>
            <el-tag size="small" type="info">{{ p.mode === 'compiled' ? '编译版' : '源码' }}</el-tag>
          </div>
          <div class="plaza-desc">{{ p.description || '暂无简介' }}</div>
          <div class="plaza-actions">
            <el-button v-if="p.route" type="primary" size="small" :href="`${p.route.prefix}/`" tag="a" target="_blank">访问服务</el-button>
            <el-button v-if="p.can_source" size="small" @click="download(p, 'source')">查看源码</el-button>
            <el-button v-else-if="authStore.isLoggedIn" size="small" @click="openRequest(p, 'source')">申请源码</el-button>
            <el-button v-if="p.binary_file && p.can_binary" size="small" @click="download(p, 'binary')">可执行文件</el-button>
            <el-button v-else-if="p.binary_file && authStore.isLoggedIn" size="small" @click="openRequest(p, 'binary')">申请产物</el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>
    <el-empty v-if="!loading && !projects.length" description="还没有通过审核的公开项目" />
    <div class="plaza-pager" v-if="total > pageSize">
      <el-pagination layout="prev, pager, next" :total="total" :page-size="pageSize" v-model:current-page="page" @current-change="load()" />
    </div>

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
import { selfhostApi, type SelfHostProject } from '@/api/selfhost';

const authStore = useAuthStore();
const isMobile = useIsMobile();
const requestVisible = ref(false);
const requestProject = ref<SelfHostProject | null>(null);
const requestKind = ref<'source' | 'binary'>('source');
const requestNote = ref('');
const keyword = ref('');
const loading = ref(false);
const projects = ref<SelfHostProject[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 12;

async function load() {
  loading.value = true;
  try {
    const res = await selfhostApi.plaza(keyword.value);
    projects.value = res.data.data.projects || [];
    total.value = res.data.data.total || 0;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

async function download(p: SelfHostProject, kind: 'source' | 'binary') {
  try {
    await selfhostApi.downloadFile(p.id, kind, kind === 'source' ? p.source_name : p.binary_name);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '下载失败');
  }
}

function openRequest(p: SelfHostProject, kind: 'source' | 'binary') {
  requestProject.value = p;
  requestKind.value = kind;
  requestNote.value = '';
  requestVisible.value = true;
}

async function doRequest() {
  if (!requestProject.value) return;
  try {
    await selfhostApi.requestAccess(requestProject.value.id, requestKind.value, requestNote.value);
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
.plaza-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.plaza-head h2 { margin: 0 0 4px; }
.plaza-head p { margin: 0; color: var(--el-text-color-secondary); font-size: 13px; }
.plaza-card { margin-bottom: 16px; min-height: 170px; }
.plaza-title { font-weight: 700; font-size: 16px; margin-bottom: 8px; }
.plaza-meta { display: flex; gap: 6px; margin-bottom: 10px; }
.plaza-desc { font-size: 13px; color: var(--el-text-color-regular); min-height: 40px; }
.plaza-actions { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 12px; }
.plaza-pager { display: flex; justify-content: center; margin-top: 12px; }
.plaza-tip { font-size: 13px; color: var(--el-text-color-secondary); line-height: 1.6; margin: 0 0 12px; }

@media (max-width: 767px) {
  .plaza-wrap { padding: 10px 8px; }
  .plaza-head { flex-direction: column; align-items: stretch; gap: 10px; }
  .plaza-head .el-input { width: 100% !important; }
  .plaza-card { min-height: 0; }
  .plaza-actions .el-button { flex: 1 1 45%; margin-left: 0 !important; }
}
</style>
