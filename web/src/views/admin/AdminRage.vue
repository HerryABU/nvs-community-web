<template>
  <div class="admin-page">
    <AdminHeader title="狂暴模式（评论实时推送）" subtitle="开启后评论区以 SSE 实时推送新评论，带宽可控" :icon="Lightning" />
    <div class="admin-page-body">
      <div class="admin-panel" v-loading="loading">
        <SectionHeader title="配置参数" subtitle="连接数、推送频率与批量阈值" :icon="'⚙️'" gradient="amber" />
        <el-alert type="warning" :closable="false" show-icon class="rage-warn">
          <template #title>开启前请知悉</template>
          狂暴模式会为每篇作品/博客的评论区建立 SSE 长连接，实时推送新评论（类直播弹幕体验）。
          连接数、推送频率均由下方参数约束，过量连接会被服务端主动断开并由前端退避重连。
        </el-alert>

        <el-form label-width="160px" class="rage-form">
          <el-form-item label="启用狂暴模式">
            <el-switch v-model="form.enabled" :disabled="saving" />
            <span class="form-tip">默认关闭；关闭后评论区不建立实时连接，照常按原逻辑加载</span>
          </el-form-item>

          <el-form-item label="批量推送周期（秒）">
            <el-input-number v-model="form.batch_interval_sec" :min="1" :max="60" :disabled="saving" />
            <span class="form-tip">多条评论合并成一批下发的间隔（1~60s）</span>
          </el-form-item>

          <el-form-item label="批量条数阈值">
            <el-input-number v-model="form.batch_size" :min="1" :max="100" :disabled="saving" />
            <span class="form-tip">pending 累积达该条数立即下发，不等周期（1~100）</span>
          </el-form-item>

          <el-form-item label="全局连接数上限">
            <el-input-number v-model="form.max_conns" :min="100" :max="10000" :step="100" :disabled="saving" />
            <span class="form-tip">超出后拒绝新连接，前端指数退避重试（100~10000）</span>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" :loading="saving" @click="save">保存配置（立即生效）</el-button>
            <el-button @click="reload" :disabled="saving">重置为已保存</el-button>
          </el-form-item>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { Lightning } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { liveApi, type RageConfig } from '@/api/live';

const loading = ref(false);
const saving = ref(false);
const form = reactive<RageConfig>({
  enabled: false,
  batch_interval_sec: 3,
  batch_size: 20,
  max_conns: 2000,
});

async function reload() {
  loading.value = true;
  try {
    const r = await liveApi.getRageConfig();
    if (r.data?.code === 0 && r.data.data) {
      Object.assign(form, {
        enabled: !!r.data.data.enabled,
        batch_interval_sec: r.data.data.batch_interval_sec ?? 3,
        batch_size: r.data.data.batch_size ?? 20,
        max_conns: r.data.data.max_conns ?? 2000,
      });
    }
  } catch {
    ElMessage.error('加载配置失败');
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  try {
    const r = await liveApi.updateRageConfig({ ...form });
    if (r.data?.code === 0) {
      ElMessage.success('已保存，立即生效');
      await reload();
    } else {
      ElMessage.error(r.data?.message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

onMounted(reload);
</script>

<style scoped>
.admin-page { --admin-content-max: 820px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.rage-warn { margin-bottom: 20px; }
.rage-form { max-width: 720px; margin-top: 8px; }
.form-tip {
  margin-left: 12px;
  color: var(--text-light, #999);
  font-size: 0.8rem;
}
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
