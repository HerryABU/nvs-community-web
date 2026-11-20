<template>
  <div class="admin-page">
    <AdminHeader title="数据库管理" subtitle="SQLite ↔ MySQL ↔ PostgreSQL 迁移 · 多库备份" :icon="DataLine" />
    <div class="admin-page-body admin-panel">
      <!-- 所有任务（引擎迁移 / 备份 / 恢复 / 正文存储迁移）共用同一进度组件，切 Tab 也不消失 -->
      <DbJobProgress :job="job" />
      <el-tabs v-model="activeTab">
        <!-- ==================== 数据库迁移 ==================== -->
        <el-tab-pane label="数据库迁移" name="migrate">
          <el-alert type="info" :closable="false" show-icon style="margin-bottom:16px">
            <template #title>
              支持 <b>SQLite ↔ MySQL ↔ PostgreSQL</b> 之间的相互迁移与同库搬迁。
              迁移完成后自动切换当前数据库连接（无需重启），并写入加密配置持久化。
            </template>
          </el-alert>
          <el-form label-width="110px" style="max-width:560px">
            <el-form-item label="目标数据库类型">
              <el-radio-group v-model="migrate.driver">
                <el-radio-button value="mysql">MySQL</el-radio-button>
                <el-radio-button value="postgres">PostgreSQL</el-radio-button>
                <el-radio-button value="sqlite">SQLite</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <template v-if="migrate.driver === 'mysql' || migrate.driver === 'postgres'">
              <el-form-item label="主机"><el-input v-model="migrate.host" placeholder="127.0.0.1" /></el-form-item>
              <el-form-item :label="migrate.driver === 'postgres' ? '端口(默认5432)' : '端口(默认3306)'">
                <el-input v-model="migrate.port" :placeholder="migrate.driver === 'postgres' ? '5432' : '3306'" />
              </el-form-item>
              <el-form-item label="用户名"><el-input v-model="migrate.user" placeholder="nvs_user" /></el-form-item>
              <el-form-item label="密码"><el-input v-model="migrate.password" type="password" show-password /></el-form-item>
              <el-form-item label="数据库名"><el-input v-model="migrate.name" placeholder="nvs" /></el-form-item>
              <el-form-item v-if="migrate.driver === 'postgres'" label="SSL 模式">
                <el-select v-model="migrate.sslmode" style="width:200px">
                  <el-option label="disable" value="disable" />
                  <el-option label="require" value="require" />
                  <el-option label="verify-ca" value="verify-ca" />
                  <el-option label="verify-full" value="verify-full" />
                </el-select>
              </el-form-item>
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
        </el-tab-pane>

        <!-- ==================== 正文存储模式 ==================== -->
        <el-tab-pane label="正文存储模式" name="storage">
          <el-alert type="info" :closable="false" show-icon style="margin-bottom:16px">
            <template #title>
              混合模式（默认）：正文写文件，库内保留镜像；纯数据库模式：正文只写数据库，
              适合高性能机器或无共享盘部署。切换会按用户分批热迁移，期间全站禁止写入，
              前端会提示「上传功能正在维护中，请稍后」。
            </template>
          </el-alert>
          <el-form label-width="110px" style="max-width:560px">
            <el-form-item label="当前模式">
              <el-tag :type="storageMode === 'database' ? 'success' : 'primary'">
                {{ storageMode === 'database' ? '纯数据库模式' : '数据库 + 文件系统混合模式' }}
              </el-tag>
            </el-form-item>
            <el-form-item label="切换为">
              <el-radio-group v-model="targetStorageMode">
                <el-radio-button value="hybrid">混合（默认）</el-radio-button>
                <el-radio-button value="database">纯数据库</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="storageBusy" :disabled="job?.status === 'running'" @click="switchStorageMode">
                {{ targetStorageMode === storageMode ? '重新扫描并补齐库内副本' : '开始切换并迁移正文' }}
              </el-button>
            </el-form-item>
          </el-form>

          <!-- 入库健康自查：迁移是否成功看这里，不看数据库文件体积 -->
          <div class="health" v-loading="healthLoading">
            <div class="health-head">
              <span class="health-title">入库健康自查</span>
              <el-button link type="primary" size="small" @click="loadStorageHealth">重新统计</el-button>
            </div>
            <template v-if="health">
              <div class="health-grid">
                <div class="health-cell">
                  <div class="hc-label">章节副本</div>
                  <div class="hc-value">{{ health.chapter.blob }} / {{ health.chapter.total }}</div>
                  <div class="hc-sub">已入库 / 章节总数</div>
                </div>
                <div class="health-cell">
                  <div class="hc-label">博客副本</div>
                  <div class="hc-value">{{ health.blog.blob }} / {{ health.blog.total }}</div>
                  <div class="hc-sub">已入库 / 博客总数</div>
                </div>
                <div class="health-cell">
                  <div class="hc-label">正文合计</div>
                  <div class="hc-value">{{ fmtSize(health.content_bytes) }}</div>
                  <div class="hc-sub">库内正文真实字节</div>
                </div>
                <div class="health-cell">
                  <div class="hc-label">库文件占用</div>
                  <div class="hc-value">{{ fmtSize(health.db_file_bytes) }}</div>
                  <div class="hc-sub">含索引与访问日志</div>
                </div>
                <div class="health-cell">
                  <div class="hc-label">源正文为空</div>
                  <div class="hc-value">{{ health.chapter.empty }}</div>
                  <div class="hc-sub">章节文件本身 0 字节</div>
                </div>
                <div class="health-cell">
                  <div class="hc-label">缺副本</div>
                  <div class="hc-value" :class="{ warn: health.chapter.missing > 0 }">{{ health.chapter.missing }}</div>
                  <div class="hc-sub">需人工处理</div>
                </div>
              </div>
              <el-alert v-if="health.chapter.missing === 0" type="success" :closable="false" show-icon
                title="全部正文已有库内副本，可安全使用纯数据库模式" style="margin-top:12px" />
              <template v-else>
                <el-alert type="warning" :closable="false" show-icon style="margin-top:12px"
                  :title="`有 ${health.chapter.missing} 条章节没有库内副本，纯数据库模式下这些章节会读不到正文`" />
                <el-table :data="health.missing_sample" size="small" style="margin-top:8px">
                  <el-table-column prop="id" label="章节 ID" width="100" />
                  <el-table-column prop="novel_id" label="作品" width="80" />
                  <el-table-column prop="chapter_number" label="章号" width="70" />
                  <el-table-column prop="title" label="标题" min-width="140" show-overflow-tooltip />
                  <el-table-column label="磁盘文件" width="110">
                    <template #default="{ row }">
                      <el-tag size="small" :type="row.file_exists ? 'success' : 'danger'">
                        {{ row.file_exists ? '存在' : '已丢失' }}
                      </el-tag>
                    </template>
                  </el-table-column>
                </el-table>
              </template>
              <p class="health-note">
                为什么库文件体积可能几乎不变：SQLite 复写页面不按内容线性增长，且重复执行迁移是幂等的
                （内容已在库内时不会再次写入）。判断是否迁移成功请看上表的副本条数与正文合计字节。
              </p>
            </template>
          </div>
        </el-tab-pane>

        <!-- ==================== 备份管理 ==================== -->
        <el-tab-pane label="备份管理" name="backup">
          <div class="admin-toolbar">
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
            <el-table-column label="类型" width="100">
              <template #default="{ row }">
                <el-tag size="small" :type="row.driver === 'postgres' ? 'primary' : row.driver === 'mysql' ? 'warning' : 'success'">{{ row.driver }}</el-tag>
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
                <el-button size="small" type="warning" @click="restoreBackup(row)">恢复</el-button>
                <el-button size="small" type="danger" :icon="Delete" @click="removeBackup(row)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <EmptyState icon="💾" text="暂无备份" description="点击「立即备份」创建第一个备份" />
            </template>
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
import { apiPrefix } from '@/api/index';
import { ElMessage, ElMessageBox } from 'element-plus';
import StateBlock from '@/components/common/StateBlock.vue';
import DbJobProgress from '@/components/admin/DbJobProgress.vue';

const activeTab = ref('migrate');

// ---------- 迁移 ----------
const migrating = ref(false);
const migrate = reactive({ driver: 'mysql', host: '127.0.0.1', port: '3306', user: '', password: '', name: '', sslmode: 'disable' });

const job = ref<any>(null);
let jobTimer: any = null;

async function startMigrate() {
  if ((migrate.driver === 'mysql' || migrate.driver === 'postgres') &&
      (!migrate.host || !migrate.port || !migrate.user || !migrate.name)) {
    ElMessage.warning('请完整填写 ' + (migrate.driver === 'postgres' ? 'PostgreSQL' : 'MySQL') + ' 连接信息');
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
    const isServer = migrate.driver === 'mysql' || migrate.driver === 'postgres';
    const res = await adminApi.migrateDb({
      driver: migrate.driver,
      host: isServer ? migrate.host : '',
      port: isServer ? migrate.port : '',
      user: isServer ? migrate.user : '',
      password: isServer ? migrate.password : '',
      name: isServer ? migrate.name : '',
      sslmode: migrate.driver === 'postgres' ? migrate.sslmode : '',
      path: '',
    });
    startPolling(res.data?.data?.job_id);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '迁移启动失败');
  } finally {
    migrating.value = false;
  }
}

// ---------- 正文存储模式 ----------
const storageMode = ref('hybrid');
const targetStorageMode = ref<'hybrid' | 'database'>('hybrid');
const storageBusy = ref(false);
const health = ref<any>(null);
const healthLoading = ref(false);

async function loadStorageMode() {
  try {
    const res = await adminApi.getStorageMode();
    const d = res.data?.data;
    if (d?.mode) {
      storageMode.value = d.mode;
      targetStorageMode.value = d.mode === 'database' ? 'database' : 'hybrid';
    }
  } catch { /* 页面仍可用 */ }
}

// 入库健康自查：判断迁移成败看条数与字节，不看数据库文件体积
// （SQLite 复写页面不线性增长，重跑迁移幂等，体积不变是正常的）
async function loadStorageHealth() {
  healthLoading.value = true;
  try {
    const res = await adminApi.getStorageHealth();
    health.value = res.data?.data || null;
  } catch { /* 忽略 */ } finally {
    healthLoading.value = false;
  }
}

async function switchStorageMode() {
  const same = targetStorageMode.value === storageMode.value;
  try {
    await ElMessageBox.confirm(
      same
        ? '当前已是该模式。重新执行会全站禁止写入，并按作者重新扫描一遍补齐库内副本（修复历史漏迁）。是否继续？'
        : '切换期间全站禁止写入（前端会提示「上传功能正在维护中，请稍后」），系统会按用户分批迁移小说/博客正文。是否继续？',
      same ? '确认重新扫描补齐' : '确认切换存储模式',
      { type: 'warning', confirmButtonText: '继续', cancelButtonText: '取消' },
    );
  } catch { return; }
  storageBusy.value = true;
  try {
    const res = await adminApi.setStorageMode(targetStorageMode.value);
    startPolling(res.data?.data?.job_id);
    loadStorageHealth();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '切换失败');
  } finally {
    storageBusy.value = false;
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
  // 🔀 站点路径前缀：window.open 直用 URL 需手动拼前缀
  window.open(`${apiPrefix}/api/admin/db/backups/${encodeURIComponent(row.name)}/download`, '_blank');
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
  const driverLabel: Record<string, string> = { sqlite: 'SQLite 数据文件', mysql: 'MySQL 数据库', postgres: 'PostgreSQL 数据库' };
  const label = driverLabel[row.driver] || row.driver;
  try {
    await ElMessageBox.confirm(
      `恢复将用备份 ${row.name} 覆盖当前 ${label}，且不可撤销。请先备份当前数据。是否继续？`,
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
        loadSched();
        loadStorageMode();
        // 迁移/补齐结束后立刻刷新入库统计，条数与字节是判断成败的唯一依据
        loadStorageHealth();
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

// 任务可能从别的页面（如管理员首页）发起，进本页要先取一次最近任务，
// 否则会显示「暂无数据库任务」，而实际迁移正在进行。
async function loadLatestJob() {
  try {
    const res = await adminApi.listDbJobs(1);
    job.value = res.data?.data?.list?.[0] || null;
    if (job.value?.status === 'running') startPolling(job.value.id);
  } catch { /* ignore */ }
}

onMounted(() => {
  loadBackups();
  loadSched();
  loadStorageMode();
  loadStorageHealth();
  loadLatestJob();
});
onBeforeUnmount(stopPolling);
</script>

<style scoped>
.admin-page { max-width: 1080px; margin: 0 auto; padding: 24px 20px; }
.admin-page-header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.admin-page-header h2 { margin: 0; color: var(--text-color); }
.admin-page-body { background: var(--bg-color); border-radius: 12px; padding: 20px; }
.sched { display: flex; align-items: center; color: var(--text-light); font-size: 0.9rem; }
.muted { color: var(--text-muted); }
/* ===== 入库健康自查 ===== */
.health { margin-top: 20px; padding-top: 16px; border-top: 1px dashed var(--border-color); }
.health-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.health-title { font-weight: 600; color: var(--text-color); }
.health-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px; }
.health-cell { background: var(--bg-soft, rgba(127, 127, 127, 0.06)); border-radius: 10px; padding: 10px 12px; }
.hc-label { font-size: 0.8rem; color: var(--text-muted); }
.hc-value { font-size: 1.15rem; font-weight: 600; color: var(--text-color); }
.hc-value.warn { color: var(--el-color-danger, #f56c6c); }
.hc-sub { font-size: 0.72rem; color: var(--text-muted); }
.health-note { margin: 12px 0 0; font-size: 0.78rem; line-height: 1.6; color: var(--text-muted); }
</style>
