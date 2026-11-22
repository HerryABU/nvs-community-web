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
            <el-button v-if="p.route" type="primary" size="small" :href="`/sh${p.route.prefix}/`" tag="a" target="_blank">访问服务</el-button>
            <el-button size="small" :href="selfhostApi.fileUrl(p.id, 'source')" target="_blank">查看源码</el-button>
            <el-button v-if="p.binary_file" size="small" :href="selfhostApi.fileUrl(p.id, 'binary')" target="_blank">可执行文件</el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>
    <el-empty v-if="!loading && !projects.length" description="还没有通过审核的公开项目" />
    <div class="plaza-pager" v-if="total > pageSize">
      <el-pagination layout="prev, pager, next" :total="total" :page-size="pageSize" v-model:current-page="page" @current-change="load()" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { selfhostApi, type SelfHostProject } from '@/api/selfhost';

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
</style>
