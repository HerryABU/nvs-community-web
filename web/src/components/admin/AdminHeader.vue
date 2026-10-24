<template>
  <div class="admin-header">
    <div class="ah-left">
      <el-button class="ah-back" text @click="$router.push('/admin')">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <div class="ah-icon" :style="{ background: color || 'var(--gradient-primary)' }">
        <el-icon :size="20"><component :is="icon" /></el-icon>
      </div>
      <div class="ah-text">
        <h2 class="ah-title">{{ title }}</h2>
        <p v-if="subtitle" class="ah-subtitle">{{ subtitle }}</p>
      </div>
    </div>
    <div class="ah-actions">
      <slot name="actions" />
    </div>

    <!-- 接口异常横幅：后台 48 个页面的 catch 基本只弹一条一闪而过的 toast，
         用户看到的是"表格空着"，分不清没数据还是接口挂了。这里常驻展示。 -->
    <div v-if="apiError" class="ah-alert" role="alert">
      <span class="ah-alert-icon">⚠️</span>
      <div class="ah-alert-text">
        <b>后台接口异常{{ apiError.status ? `（${apiError.status}）` : '（网络不通）' }}</b>
        <span>{{ apiError.url }} · {{ apiError.message }}</span>
      </div>
      <el-button size="small" type="primary" plain @click="retry">重新加载</el-button>
      <el-button size="small" text @click="dismiss">忽略</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ArrowLeft } from '@element-plus/icons-vue';
import { adminApiError, dismissAdminApiError } from '@/utils/adminHealth';

const apiError = adminApiError;

/** 重试：整页重载。各页 loader 名称不统一（load/reload/loadList…），
 *  逐个接一遍不现实，而"重新加载这个后台页"是用户真正想要的语义。 */
function retry() {
  dismissAdminApiError();
  window.location.reload();
}
function dismiss() {
  dismissAdminApiError();
}

defineProps<{
  title: string;
  subtitle?: string;
  icon?: any;
  color?: string;
}>();
</script>

<style scoped>
.admin-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 20px;
}
.ah-left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.ah-back {
  padding: 6px 8px;
  color: var(--text-light, #666);
}
.ah-back:hover {
  color: var(--primary-color);
}
.ah-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  flex-shrink: 0;
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.12);
}
.ah-text {
  min-width: 0;
}
.ah-title {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--text-color, #1a1a2e);
  line-height: 1.3;
}
.ah-subtitle {
  margin: 2px 0 0;
  font-size: 0.82rem;
  color: var(--text-light, #888);
}
.ah-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
/* ---- 接口异常横幅 ---- */
.ah-alert {
  flex-basis: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid rgba(239, 68, 68, 0.35);
  border-radius: var(--radius-md, 10px);
  background: rgba(239, 68, 68, 0.07);
}
.ah-alert-icon { font-size: 16px; line-height: 1; }
.ah-alert-text { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.ah-alert-text b { font-size: var(--text-sm, 13px); color: #b91c1c; }
.ah-alert-text span {
  font-size: var(--text-xs, 12px);
  color: var(--text-light, #6b7280);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
[data-theme="dark"] .ah-alert {
  background: rgba(239, 68, 68, 0.12);
  border-color: rgba(248, 113, 113, 0.4);
}
[data-theme="dark"] .ah-alert-text b { color: #fca5a5; }

[data-theme="dark"] .ah-title {
  color: #e2e8f0;
}
[data-theme="dark"] .ah-subtitle {
  color: #94a3b8;
}
[data-theme="dark"] .ah-back {
  color: #94a3b8;
}
[data-theme="dark"] .ah-back:hover {
  color: #e2e8f0;
}
</style>
