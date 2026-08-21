<template>
  <div class="admin-page">
    <AdminHeader title="健康检查" subtitle="服务器 / 数据库 / 网络 / 前端 API" :icon="Monitor" />
    <div class="admin-page-body admin-panel">
      <div class="toolbar">
        <el-button type="primary" :icon="Refresh" :loading="loading" @click="loadHealth">立即检测</el-button>
        <el-switch v-model="autoRefresh" active-text="自动刷新（30s）" />
        <el-tag v-if="health" :type="health.status === 'ok' ? 'success' : 'warning'" size="large" effect="dark">
          整体状态：{{ health.status === 'ok' ? '运行正常' : '降级' }}
        </el-tag>
        <span v-if="health" class="muted">检测时间 {{ health.now }}</span>
      </div>

      <!-- 四大核心检查 -->
      <div class="core-grid" v-if="health">
        <div class="core-card" :class="serverOk ? 'ok' : 'bad'">
          <div class="core-icon"><el-icon :size="26"><Monitor /></el-icon></div>
          <div class="core-body">
            <div class="core-label">服务器</div>
            <div class="core-value">{{ serverOk ? '正常' : '异常' }}</div>
            <div class="core-sub">运行 {{ fmtUptime(health.server?.uptime_sec) }} · Goroutine {{ health.server?.goroutines }}</div>
            <div class="core-sub">内存 {{ health.server?.mem_alloc_mb }} MB · 版本 {{ health.version }}</div>
          </div>
        </div>

        <div class="core-card" :class="dbOk ? 'ok' : 'bad'">
          <div class="core-icon"><el-icon :size="26"><Coin /></el-icon></div>
          <div class="core-body">
            <div class="core-label">后端 API（数据库）</div>
            <div class="core-value">{{ dbOk ? '正常' : '异常' }}</div>
            <div class="core-sub">{{ health.database?.driver }}{{ health.database?.latency_ms != null ? ' · ' + health.database.latency_ms + 'ms' : '' }}</div>
            <div class="core-sub">{{ health.database?.error || '业务数据可查询' }}</div>
          </div>
        </div>

        <div class="core-card" :class="netOk ? 'ok' : 'bad'">
          <div class="core-icon"><el-icon :size="26"><Connection /></el-icon></div>
          <div class="core-body">
            <div class="core-label">网络（出站）</div>
            <div class="core-value">{{ netOk ? '正常' : '异常' }}</div>
            <div class="core-sub">{{ netOk ? '外网可达' : (health.network?.error || '不可达') }}</div>
          </div>
        </div>

        <div class="core-card" :class="feOk ? 'ok' : 'bad'">
          <div class="core-icon"><el-icon :size="26"><Monitor /></el-icon></div>
          <div class="core-body">
            <div class="core-label">前端 API（SPA 资源）</div>
            <div class="core-value">{{ feOk ? '正常' : '异常' }}</div>
            <div class="core-sub">{{ health.frontend?.bundle || health.frontend?.error }}</div>
          </div>
        </div>
      </div>

      <!-- 详细报告 -->
      <el-descriptions v-if="health" :column="2" border style="margin-top:20px" title="详细报告">
        <el-descriptions-item label="服务">{{ health.service }} v{{ health.version }}</el-descriptions-item>
        <el-descriptions-item label="安装状态">{{ health.installed ? '已安装' : '未安装' }}</el-descriptions-item>
        <el-descriptions-item label="Redis 缓存">
          <el-tag size="small" :type="health.redis?.status === 'ok' ? 'success' : (health.redis?.status === 'disabled' ? 'info' : 'danger')">
            {{ redisLabel(health.redis?.status) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="业务数据">
          <span v-if="health.business_apis?.ok">
            用户 {{ health.business_apis.users }} · 作品 {{ health.business_apis.novels }} · 论坛 {{ health.business_apis.forums }}
          </span>
          <span v-else class="bad-text">{{ health.business_apis?.error || '异常' }}</span>
        </el-descriptions-item>
      </el-descriptions>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import { Refresh, Monitor, Coin, Connection } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { publicApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const loading = ref(false);
const health = ref<any>(null);
const autoRefresh = ref(false);
let timer: any = null;

const serverOk = computed(() => !!health.value?.server?.ok);
const dbOk = computed(() => !!health.value?.database?.ok);
const netOk = computed(() => !!health.value?.network?.ok);
const feOk = computed(() => !!health.value?.frontend?.ok);

async function loadHealth() {
  loading.value = true;
  try {
    const res = await publicApi.getHealth();
    health.value = res.data;
  } catch {
    ElMessage.error('健康检查失败');
  } finally {
    loading.value = false;
  }
}

function fmtUptime(sec: number) {
  if (!sec) return '0 分钟';
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d} 天 ${h} 小时`;
  if (h > 0) return `${h} 小时 ${m} 分`;
  return `${m} 分钟`;
}

function redisLabel(s: string) {
  return { ok: '正常', disabled: '未启用', error: '异常' }[s] || s;
}

function toggleAuto(v: boolean) {
  if (timer) { clearInterval(timer); timer = null; }
  if (v) {
    timer = setInterval(loadHealth, 30000);
  }
}

onMounted(() => {
  loadHealth();
});
onBeforeUnmount(() => { if (timer) clearInterval(timer); });
</script>

<style scoped>
.admin-page { max-width: 1080px; margin: 0 auto; padding: 24px 20px; }
.admin-page-header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.admin-page-header h2 { margin: 0; color: var(--text-color); }
.admin-page-body { background: var(--bg-color); border-radius: 12px; padding: 20px; }
.toolbar { display: flex; align-items: center; gap: 16px; flex-wrap: wrap; margin-bottom: 16px; }
.muted { color: var(--text-muted); font-size: 0.85rem; }
.core-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 14px; }
.core-card {
  display: flex; gap: 14px; align-items: flex-start;
  background: #fff; border: 1px solid #eef2f7; border-radius: 12px; padding: 18px;
}
.core-card.ok { border-left: 4px solid #10b981; }
.core-card.bad { border-left: 4px solid #ef4444; }
.core-icon { width: 48px; height: 48px; border-radius: 12px; display: flex; align-items: center; justify-content: center; color: #fff; flex-shrink: 0; background: var(--gradient-primary); }
.core-card.ok .core-icon { background: var(--gradient-success); }
.core-card.bad .core-icon { background: var(--gradient-danger); }
.core-body { flex: 1; min-width: 0; }
.core-label { color: var(--text-light); font-size: 0.82rem; }
.core-value { font-size: 1.4rem; font-weight: 700; color: var(--text-color); margin: 2px 0; }
.core-sub { color: var(--text-muted); font-size: 0.78rem; line-height: 1.5; }
.bad-text { color: #ef4444; }
</style>
