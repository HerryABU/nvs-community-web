<template>
  <div class="admin-page">
    <AdminHeader title="数据库管理" subtitle="SQLite ↔ MySQL 迁移 · 多库备份" :icon="DataLine" />
    <div class="admin-page-body admin-panel">
      <el-tabs v-model="activeTab">
        <!-- ==================== 数据库迁移 ==================== -->
        <el-tab-pane label="数据库迁移" name="migrate">
          <el-alert type="info" :closable="false" show-icon style="margin-bottom:16px">
            <template #title>
              支持 <b>SQLite → MySQL</b>、<b>MySQL → SQLite</b>、<b>MySQL → MySQL</b>。
              迁移完成后自动切换当前数据库连接（无需重启），并写入加密配置持久化。
            </template>
          </el-alert>
          <el-form label-width="110px" style="max-width:560px">
            <el-form-item label="目标数据库类型">
              <el-radio-group v-model="migrate.driver">
                <el-radio-button value="mysql">MySQL</el-radio-button>
                <el-radio-button value="sqlite">SQLite</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <template v-if="migrate.driver === 'mysql'">
              <el-form-item label="主机"><el-input v-model="migrate.host" placeholder="127.0.0.1" /></el-form-item>
              <el-form-item label="端口"><el-input v-model="migrate.port" placeholder="3306" /></el-form-item>
              <el-form-item label="用户名"><el-input v-model="migrate.user" placeholder="nvs_user" /></el-form-item>
              <el-form-item label="密码"><el-input v-model="migrate.password" type="password" show-password /></el-form-item>
              <el-form-item label="数据库名"><el-input v-model="migrate.name" placeholder="nvs" /></el-form-item>
            </template>
            <template v-else>
              <el-alert type="success" :closable="false" show-icon style="margin-bottom:12px">
                <template #title>目标为本地文件 data/nvs.db（覆盖当前文件前请先备份）</template>
              </el-alert>
            </template>
            <el-form-item>
              <el-button type="primary" :loading="migrating" @click="startMigrate">开始迁移</el-button>
            </el-form-item>
          </el-form>

          <!-- 迁移进度 -->
          <div v-if="job" class="job-box">
            <div class="job-head">
              <el-tag :type="job.status === 'done' ? 'success' : job.status === 'error' ? 'danger' : 'primary'">
                {{ jobLabel(job.status) }}
              </el-tag>
              <span class="job-msg">{{ job.message }}</span>
            </div>
            <el-progress v-if="job.status === 'running' && job.tables_total > 0"
              :percentage="Math.round((job.tables_done / job.tables_total) * 100)" />
            <div v-if="job.status === 'running'" class="job-meta">
              正在复制表 <b>{{ job.table }}</b>（{{ job.tables_done }}/{{ job.tables_total }} 张，{{ job.rows }} 行）
            </div>
          </div>
        </el-tab-pane>

        <!-- ==================== 备份管理 ==================== -->
        <el-tab-pane label="备份管理" name="backup">
          <div class="toolbar">
            <el-button type="primary" :icon="Box" :loading="backingUp" @click="startBackup">立即备份</el-button>
            <div class="sched">
              <span>定时备份：</span>
              <el-switch v-model="sched.enabled" @change="saveSched" />
              <span style="margin-left:12px">间隔</span>
              <el-input-number v-model="sched.interval" :min="1" :max="720" @change="saveSched" style="width:110px" />
              <span style="margin-left:6px">小时</span>
              <span v-if="sched.last_backup" class="muted" style="margin-left:12px">上次备份：{{ sched.last_backup }}</span>
            </div>
          </div>

          <el-table :data="backups" style="margin-top:8px" v-loading="backupsLoading">
            <el-table-column prop="name" label="文件名" min-width="220" show-overflow-tooltip />
            <el-table-column label="类型" width="90">
              <template #default="{ row }">
                <el-tag size="small" :type="row.driver === 'mysql' ? 'warning' : 'success'">{{ row.driver }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="大小" width="110">
              <template #default="{ row }">{{ fmtSize(row.size) }}</template>
            </el-table-column>
            <el-table-column label="时间" width="170">
              <template #default="{ row }">{{ fmtTime(row.mod_time) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="200">
              <template #default="{ row }">
                <el-button size="small" :icon="Download" @click="downloadBackup(row)">下载</el-button>
                <el-button v-if="row.driver === 'sqlite'" size="small" type="warning" @click="restoreBackup(row)">恢复</el-button>
                <el-button size="small" type="danger" :icon="Delete" @click="removeBackup(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount } from 'vue';
import { Box, Download, Delete, DataLine } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';

const activeTab = ref('migrate');

// ---------- 迁移 ----------
const migrating = ref(false);
const migrate = reactive({ driver: 'mysql', host: '127.0.0.1', port: '3306', user: '', password: '', name: '' });

const job = ref<any>(null);
let jobTimer: any = null;

async function startMigrate() {
  if (migrate.driver === 'mysql' && (!migrate.host || !migrate.port || !migrate.user || !migrate.name)) {
    ElMessage.warning('请完整填写 MySQL 连接信息');
    return;
  }
  try {
    await ElMessageBox.confirm(
      '迁移将把当前数据库全部数据复制到目标库，完成后自动切换连接。建议先备份。是否继续？',
      '确认迁移', { type: 'warning', confirmButtonText: '继续', cancelButtonText: '取消' },
    );
  } catch { return; }

  migrating.value = true;
  job.value = null;
  try {
    const res = await adminApi.migrateDb({
      driver: migrate.driver,
      host: migrate.driver === 'mysql' ? migrate.host : '',
      port: migrate.driver === 'mysql' ? migrate.port : '',
      user: migrate.driver === 'mysql' ? migrate.user : '',
      password: migrate.driver === 'mysql' ? migrate.password : '',
      name: migrate.driver === 'mysql' ? migrate.name : '',
      path: '',
    });
    startPolling(res.data?.data?.job_id);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '迁移启动失败');
  } finally {
    migrating.value = false;
  }
}

// ---------- 备份 ----------
const backingUp = ref(false);
const backupsLoading = ref(false);
const backups = ref<any[]>([]);
const sched = reactive({ enabled: false, interval: 24, last_backup: '' });

async function startBackup() {
  backingUp.value = true;
  job.value = null;
  try {
    const res = await adminApi.backupDb();
    ElMessage.success('备份任务已启动');
    startPolling(res.data?.data?.job_id);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '备份失败');
  } finally {
    backingUp.value = false;
  }
}

async function loadBackups() {
  backupsLoading.value = true;
  try {
    const res = await adminApi.getBackups();
    backups.value = res.data?.data || [];
  } catch (e) {
    ElMessage.error('读取备份列表失败');
  } finally {
    backupsLoading.value = false;
  }
}

async function loadSched() {
  try {
    const res = await adminApi.getBackupConfig();
    const d = res.data?.data;
    if (d) {
      sched.enabled = !!d.enabled;
      sched.interval = d.interval || 24;
      sched.last_backup = d.last_backup || '';
    }
  } catch { /* ignore */ }
}

async function saveSched() {
  try {
    await adminApi.updateBackupConfig({ enabled: sched.enabled, interval: sched.interval });
    ElMessage.success('定时备份设置已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
}

function downloadBackup(row: any) {
  window.open(`/api/admin/db/backups/${encodeURIComponent(row.name)}/download`, '_blank');
}

async function removeBackup(row: any) {
  try {
    await ElMessageBox.confirm(`确认删除备份 ${row.name}？`, '删除备份', { type: 'warning' });
  } catch { return; }
  try {
    await adminApi.deleteBackup(row.name);
    ElMessage.success('已删除');
    loadBackups();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function restoreBackup(row: any) {
  try {
    await ElMessageBox.confirm(
      `恢复将用备份 ${row.name} 覆盖当前 SQLite 数据库，且不可撤销。请先备份当前数据。是否继续？`,
      '恢复数据库', { type: 'warning', confirmButtonText: '恢复', cancelButtonText: '取消' },
    );
  } catch { return; }
  try {
    const res = await adminApi.restoreDb(row.name);
    ElMessage.success('恢复任务已启动');
    startPolling(res.data?.data?.job_id);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '恢复失败');
  }
}

// ---------- 后台任务轮询 ----------
function startPolling(jobId: string) {
  if (!jobId) { ElMessage.error('未获取到任务 ID'); return; }
  stopPolling();
  const poll = async () => {
    try {
      const res = await adminApi.getDbJob(jobId);
      job.value = res.data?.data;
      if (job.value?.status === 'done') {
        stopPolling();
        ElMessage.success(job.value.message || '任务完成');
        loadBackups();
        loadHealth();
        return;
      }
      if (job.value?.status === 'error') {
        stopPolling();
        ElMessage.error(job.value.message || '任务失败');
        return;
      }
    } catch { /* 忽略轮询错误 */ }
  };
  poll();
  jobTimer = setInterval(poll, 2000);
}

function stopPolling() {
  if (jobTimer) { clearInterval(jobTimer); jobTimer = null; }
}

// ---------- 工具函数 ----------
function jobLabel(s: string) {
  return { running: '运行中', done: '已完成', error: '失败' }[s] || s;
}
function fmtSize(n: number) {
  if (!n) return '0 B';
  const u = ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
  return v.toFixed(1) + ' ' + u[i];
}
function fmtTime(t: string) {
  if (!t) return '-';
  return new Date(t).toLocaleString();
}

onMounted(() => {
  loadBackups();
  loadSched();
});
onBeforeUnmount(stopPolling);
</script>

<style scoped>
.admin-page { max-width: 1080px; margin: 0 auto; padding: 24px 20px; }
.admin-page-header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.admin-page-header h2 { margin: 0; color: var(--text-color); }
.admin-page-body { background: var(--bg-color); border-radius: 12px; padding: 20px; }
.toolbar { display: flex; align-items: center; gap: 20px; flex-wrap: wrap; margin-bottom: 8px; }
.sched { display: flex; align-items: center; color: var(--text-light); font-size: 0.9rem; }
.muted { color: var(--text-muted); }
.job-box { margin-top: 20px; padding: 16px; background: #fff; border: 1px solid #eef2f7; border-radius: 12px; }
.job-head { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.job-msg { color: var(--text-light); }
.job-meta { margin-top: 8px; color: var(--text-light); font-size: 0.88rem; }
</style>
