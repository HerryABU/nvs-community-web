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
        </el-form>

        <!-- 多协议传输层（按需开启）：核心仍是 SSE；以下协议为可选叠加层，
             保存即由 realtime.Manager 拉起/关停对应端口，无需重启。 -->
        <SectionHeader title="多协议传输层（按需开启）" subtitle="核心 SSE 之外的附加通道，独立端口" :icon="'🔌'" gradient="violet" style="margin-top: 28px" />
        <el-alert type="info" :closable="false" show-icon class="rage-warn">
          <template #title>协议说明</template>
          <b>WebSocket</b>：浏览器友好，复用同一事件 Hub；未开启或代理不支持时前端自动回退 SSE。<br />
          <b>MQTT</b>：内嵌 Broker，供第三方/IoT 客户端订阅 <code>nvs/comment/&lt;type&gt;/&lt;id&gt;</code>（QoS 0）。<br />
          <b>gRPC</b>：内嵌 HTTP/2 服务端，JSON Codec，服务名 <code>nvs.realtime.Live</code>，方法 <code>Stream</code>。
        </el-alert>

        <el-form label-width="160px" class="rage-form">
          <el-form-item label="WebSocket 通道">
            <el-switch v-model="form.ws_enabled" :disabled="saving" />
            <span class="form-tip">开启后浏览器优先走 WS 订阅（回退 SSE）</span>
          </el-form-item>

          <el-form-item label="MQTT Broker">
            <el-switch v-model="form.mqtt_enabled" :disabled="saving" />
            <span class="form-tip">内嵌 Broker，独立监听端口（开启即生效）</span>
          </el-form-item>
          <el-form-item label="MQTT 监听地址">
            <el-input v-model="form.mqtt_bind" :disabled="saving || !form.mqtt_enabled" style="max-width: 220px" />
          </el-form-item>
          <el-form-item label="MQTT 端口">
            <el-input-number v-model="form.mqtt_port" :min="1" :max="65535" :disabled="saving || !form.mqtt_enabled" />
          </el-form-item>

          <el-form-item label="gRPC 服务端">
            <el-switch v-model="form.grpc_enabled" :disabled="saving" />
            <span class="form-tip">内嵌 HTTP/2 流式服务端，独立监听端口</span>
          </el-form-item>
          <el-form-item label="gRPC 监听地址">
            <el-input v-model="form.grpc_bind" :disabled="saving || !form.grpc_enabled" style="max-width: 220px" />
          </el-form-item>
          <el-form-item label="gRPC 端口">
            <el-input-number v-model="form.grpc_port" :min="1" :max="65535" :disabled="saving || !form.grpc_enabled" />
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
import SectionHeader from '@/components/common/SectionHeader.vue';
import { liveApi, type RageConfig } from '@/api/home/live';

const loading = ref(false);
const saving = ref(false);
const form = reactive<RageConfig>({
  enabled: false,
  batch_interval_sec: 3,
  batch_size: 20,
  max_conns: 2000,
  ws_enabled: false,
  mqtt_enabled: false,
  mqtt_bind: '0.0.0.0',
  mqtt_port: 1883,
  grpc_enabled: false,
  grpc_bind: '0.0.0.0',
  grpc_port: 50051,
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
        ws_enabled: !!r.data.data.ws_enabled,
        mqtt_enabled: !!r.data.data.mqtt_enabled,
        mqtt_bind: r.data.data.mqtt_bind || '0.0.0.0',
        mqtt_port: r.data.data.mqtt_port || 1883,
        grpc_enabled: !!r.data.data.grpc_enabled,
        grpc_bind: r.data.data.grpc_bind || '0.0.0.0',
        grpc_port: r.data.data.grpc_port || 50051,
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
