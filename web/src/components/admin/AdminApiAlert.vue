<template>
  <!--
    后台接口异常横幅（可复用）。

    为什么抽出来：错误来源已经被 axios 拦截器统一登记在 utils/adminHealth.ts，
    页面不需要各自维护 loadError 状态。多数页面加一行 <AdminApiAlert /> 即可，
    不必再写 10 套 catch → ref → v-if 的重复逻辑。

    variant：
    - inline（默认）：放在 .admin-page-body 顶部，用于没有 AdminHeader 的设置型页面
    - header：铺满一行放在标题下方（AdminHeader 内部使用）
  -->
  <div v-if="err" class="aa-alert" :class="`aa-alert--${variant}`" role="alert">
    <span class="aa-icon">⚠️</span>
    <div class="aa-text">
      <b>后台接口异常{{ err.status ? `（${err.status}）` : '（网络不通）' }}</b>
      <span>{{ err.url }} · {{ err.message }}</span>
    </div>
    <el-button size="small" type="primary" plain @click="retry">重新加载</el-button>
    <el-button size="small" text @click="dismiss">忽略</el-button>
  </div>
</template>

<script setup lang="ts">
import { adminApiError, dismissAdminApiError } from '@/utils/adminHealth';

withDefaults(defineProps<{ variant?: 'inline' | 'header' }>(), { variant: 'inline' });

const err = adminApiError;

/** 各页 loader 命名不统一（load / loadCfg / loadTree…），整页重载是唯一对所有页面都成立的语义 */
function retry() {
  dismissAdminApiError();
  window.location.reload();
}
function dismiss() {
  dismissAdminApiError();
}
</script>

<style scoped>
.aa-alert {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid rgba(239, 68, 68, 0.35);
  border-radius: var(--radius-md, 10px);
  background: rgba(239, 68, 68, 0.07);
}
.aa-alert--inline {
  margin-bottom: var(--space-3, 12px);
}
.aa-alert--header {
  flex-basis: 100%;
}
.aa-icon { font-size: 16px; line-height: 1; }
.aa-text { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.aa-text b { font-size: var(--text-sm, 13px); color: #b91c1c; }
.aa-text span {
  font-size: var(--text-xs, 12px);
  color: var(--text-light, #6b7280);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
[data-theme='dark'] .aa-alert {
  background: rgba(239, 68, 68, 0.12);
  border-color: rgba(248, 113, 113, 0.4);
}
[data-theme='dark'] .aa-text b { color: #fca5a5; }
</style>
