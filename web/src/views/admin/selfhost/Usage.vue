<template>
  <div class="us-wrap">
    <el-card shadow="never">
      <template #header><div class="us-title">自托管用量与空间隔离</div></template>
      <el-alert type="info" :closable="false" show-icon class="us-notice">
        流量按代理前缀归集到项目，按天累计上行/下行字节；体积为源码包与可执行文件的实际落盘大小。
      </el-alert>
      <el-table :data="usage" v-loading="loading" stripe>
        <el-table-column label="项目" min-width="160">
          <template #default="{ row }">{{ row.name }}<div class="us-slug">{{ row.slug }}</div></template>
        </el-table-column>
        <el-table-column label="作者 ID" width="100" prop="owner_id" />
        <el-table-column label="占用体积" width="130">
          <template #default="{ row }">{{ mb(row.storage_bytes) }} MB</template>
        </el-table-column>
        <el-table-column label="上行" width="120">
          <template #default="{ row }">{{ mb(row.bytes_up) }} MB</template>
        </el-table-column>
        <el-table-column label="下行" width="120">
          <template #default="{ row }">{{ mb(row.bytes_down) }} MB</template>
        </el-table-column>
        <el-table-column label="请求数" width="110" prop="requests" />
        <el-table-column v-if="!isMobile" label="统计天数" width="100" prop="days" />
      </el-table>
      <el-empty v-if="!loading && !usage.length" description="暂无项目" />
    </el-card>

    <el-card shadow="never" class="us-iso">
      <template #header><div class="us-title">空间隔离自检</div></template>
      <el-alert v-if="!issues.length" type="success" :closable="false" show-icon title="未发现跨空间访问风险" />
      <el-table v-else :data="issues" stripe size="small">
        <el-table-column label="项目" min-width="140" prop="name" />
        <el-table-column label="上游" min-width="220" prop="upstream" />
        <el-table-column label="问题" min-width="260" prop="reason" />
        <el-table-column label="级别" width="90">
          <template #default="{ row }"><el-tag size="small" :type="row.level === 'block' ? 'danger' : 'warning'">{{ row.level }}</el-tag></template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { useIsMobile } from '@/utils/mobile';
import { selfhostApi, type UsageRow, type IsolationIssue } from '@/api/selfhost';

const isMobile = useIsMobile();
const loading = ref(false);
const usage = ref<UsageRow[]>([]);
const issues = ref<IsolationIssue[]>([]);
const mb = (n: number) => ((Number(n) || 0) / 1048576).toFixed(2);

async function load() {
  loading.value = true;
  try {
    const [u, i] = await Promise.all([selfhostApi.adminUsage(), selfhostApi.adminIsolation()]);
    usage.value = u.data.data.usage || [];
    issues.value = i.data.data.issues || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}
onMounted(load);
</script>

<style scoped>
.us-wrap { padding: 16px; }
.us-title { font-size: 18px; font-weight: 700; }
.us-notice { margin-bottom: 12px; }
.us-slug { font-size: 12px; color: var(--el-text-color-secondary); }
.us-iso { margin-top: 16px; }
@media (max-width: 767px) {
  .us-wrap { padding: 8px; }
}
</style>
