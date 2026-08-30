<template>
  <div class="admin-page">
    <AdminHeader title="隔离墙访问记录" subtitle="谁、何时、进入哪个敏感分区（176号令合规）" :icon="Histogram" />
    <div class="admin-page-body admin-panel">

      <!-- 统计卡片 -->
      <div class="stat-cards" v-loading="loadingStats">
        <div class="stat-card">
          <div class="stat-value">{{ stats.total }}</div>
          <div class="stat-label">累计进入次数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ stats.today }}</div>
          <div class="stat-label">今日进入次数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ stats.unique_users }}</div>
          <div class="stat-label">去重用户数</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ stats.zones.length }}</div>
          <div class="stat-label">启用分区数</div>
        </div>
      </div>

      <p class="hint-text">把原本只能在数据库看到的隔离墙进入明细，在前台以仪表盘形式呈现。请求头/IP 的细粒度分类与攻击流量识别见后续「网络诊断」面板。</p>

      <!-- 筛选区 -->
      <div class="filter-bar">
        <el-select v-model="filters.zone" placeholder="全部分区" clearable size="small" style="width:160px">
          <el-option v-for="z in zoneOptions" :key="z" :label="z" :value="z" />
        </el-select>
        <el-input v-model="filters.user_id" placeholder="用户ID" size="small" style="width:120px" clearable @keyup.enter="reload" />
        <el-date-picker v-model="filters.from" type="date" placeholder="起始日期" size="small" value-format="YYYY-MM-DD" style="width:150px" />
        <el-date-picker v-model="filters.to" type="date" placeholder="结束日期" size="small" value-format="YYYY-MM-DD" style="width:150px" />
        <el-button size="small" type="primary" @click="reload" :icon="Search">查询</el-button>
        <el-button size="small" @click="resetFilters" :icon="RefreshLeft">重置</el-button>
      </div>

      <!-- 记录表格 -->
      <el-table :data="rows" v-loading="loading" stripe border style="width:100%" :default-sort="{ prop: 'entered_at', order: 'descending' }">
        <el-table-column label="用户" min-width="160">
          <template #default="{ row }">
            <span class="cell-user">{{ row.nickname || row.username || '游客' }}</span>
            <span class="cell-uid" v-if="row.username">@{{ row.username }} #{{ row.user_id }}</span>
          </template>
        </el-table-column>
        <el-table-column label="分区" width="150">
          <template #default="{ row }">
            <el-tag type="danger" size="small">{{ row.zone }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="进入时间" prop="entered_at" width="180" sortable />
        <el-table-column label="IP" prop="ip" width="150" />
        <el-table-column label="User-Agent" min-width="260" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="ua-text">{{ row.user_agent || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openDetail(row)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pager">
        <el-pagination
          background
          layout="total, prev, pager, next"
          :total="total"
          :page-size="filters.page_size"
          :current-page="filters.page"
          @current-change="onPage"
        />
      </div>
    </div>

    <!-- 详情抽屉 -->
    <el-drawer v-model="drawer" title="进入记录详情" size="420px">
      <template v-if="current">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="用户">{{ current.nickname || current.username || '游客' }} (#{{ current.user_id }})</el-descriptions-item>
          <el-descriptions-item label="分区">
            <el-tag type="danger" size="small">{{ current.zone }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="进入时间">{{ current.entered_at }}</el-descriptions-item>
          <el-descriptions-item label="IP">{{ current.ip || '—' }}</el-descriptions-item>
          <el-descriptions-item label="记录ID">{{ current.id }}</el-descriptions-item>
        </el-descriptions>
        <div class="ua-block">
          <div class="ua-title">User-Agent</div>
          <pre class="ua-pre">{{ current.user_agent || '—' }}</pre>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { Histogram, Search, RefreshLeft } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

interface ZoneRow {
  id: number;
  user_id: number;
  username: string;
  nickname: string;
  zone: string;
  entered_at: string;
  ip: string;
  user_agent: string;
}

const rows = ref<ZoneRow[]>([]);
const total = ref(0);
const loading = ref(false);
const loadingStats = ref(false);
const current = ref<ZoneRow | null>(null);
const drawer = ref(false);

const stats = reactive({ total: 0, today: 0, unique_users: 0, zones: [] as { zone: string; count: number }[] });
const zoneOptions = ref<string[]>([]);

const filters = reactive({
  zone: '',
  user_id: '',
  from: '',
  to: '',
  page: 1,
  page_size: 20,
});

async function loadStats() {
  loadingStats.value = true;
  try {
    const res = await adminApi.getZoneAccessStats();
    const d = res.data.data || res.data || {};
    stats.total = d.total || 0;
    stats.today = d.today || 0;
    stats.unique_users = d.unique_users || 0;
    stats.zones = d.zones || [];
    // 分区下拉：统计里出现的分区 + 当前筛选分区
    const set = new Set<string>(stats.zones.map((z: any) => z.zone));
    if (filters.zone) set.add(filters.zone);
    zoneOptions.value = Array.from(set);
  } catch { /* ignore */ }
  loadingStats.value = false;
}

async function reload() {
  loading.value = true;
  try {
    const params: Record<string, any> = { page: filters.page, page_size: filters.page_size };
    if (filters.zone) params.zone = filters.zone;
    if (filters.user_id) params.user_id = Number(filters.user_id);
    if (filters.from) params.from = filters.from;
    if (filters.to) params.to = filters.to;
    const res = await adminApi.getZoneAccessLogs(params);
    const d = res.data.data || res.data || {};
    rows.value = d.items || [];
    total.value = d.total || 0;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  }
  loading.value = false;
}

function onPage(p: number) {
  filters.page = p;
  reload();
}

function resetFilters() {
  filters.zone = '';
  filters.user_id = '';
  filters.from = '';
  filters.to = '';
  filters.page = 1;
  reload();
}

function openDetail(row: ZoneRow) {
  current.value = row;
  drawer.value = true;
}

onMounted(() => {
  loadStats();
  reload();
});
</script>

<style scoped>
.admin-page { --admin-content-max: 1100px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: 1100px; }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin: 8px 0 16px; }
.stat-cards { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 16px; }
.stat-card { background: #fff; border: 1px solid #ebeef5; border-radius: 10px; padding: 16px; text-align: center; }
.stat-value { font-size: 1.6rem; font-weight: 700; color: var(--text-color, #333); }
.stat-label { font-size: 0.8rem; color: var(--text-light, #888); margin-top: 4px; }
.filter-bar { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 14px; }
.cell-user { font-weight: 600; }
.cell-uid { display: block; font-size: 0.72rem; color: var(--text-light, #999); }
.ua-text { font-size: 0.8rem; color: var(--text-light, #555); }
.pager { display: flex; justify-content: flex-end; margin-top: 14px; }
.ua-block { margin-top: 16px; }
.ua-title { font-size: 0.85rem; font-weight: 600; margin-bottom: 6px; }
.ua-pre { background: #f5f7fa; border-radius: 8px; padding: 10px; font-size: 0.78rem; white-space: pre-wrap; word-break: break-all; max-height: 240px; overflow: auto; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .stat-card { background: #1e293b; border-color: rgba(255,255,255,0.08); }
[data-theme="dark"] .ua-pre { background: #0f172a; }
</style>
