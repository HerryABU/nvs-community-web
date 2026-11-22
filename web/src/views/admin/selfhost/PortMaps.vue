<template>
  <div class="pm-wrap">
    <el-card shadow="never">
      <template #header>
        <div class="pm-head">
          <div>
            <div class="pm-title">TCP 端口映射</div>
            <div class="pm-sub">四层转发：监听一个端口，把流量转到另一台主机的端口。仅管理员可配置。</div>
          </div>
          <el-button type="primary" @click="openEditor()">新建映射</el-button>
        </div>
      </template>
      <el-alert type="warning" :closable="false" show-icon class="pm-notice">
        监听端口不能与 NVS 自身端口冲突；127.0.0.1 上不能做同端口自环映射。
      </el-alert>
      <el-table :data="maps" v-loading="loading" stripe>
        <el-table-column label="监听端口" width="120" prop="listen_port" />
        <el-table-column label="目标" min-width="180">
          <template #default="{ row }">{{ row.target_host }}:{{ row.target_port }}</template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="说明" min-width="160" show-overflow-tooltip prop="description" />
        <el-table-column label="运行" width="100">
          <template #default="{ row }"><el-tag size="small" :type="row.running ? 'success' : 'info'">{{ row.running ? '监听中' : '未运行' }}</el-tag></template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="错误" min-width="160" show-overflow-tooltip prop="last_error" />
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button link @click="openEditor(row)">编辑</el-button>
            <el-button link :type="row.enabled ? 'warning' : 'success'" @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</el-button>
            <el-button link type="danger" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && !maps.length" description="还没有端口映射" />
    </el-card>

    <el-dialog v-model="visible" :title="form.id ? '编辑映射' : '新建映射'" :width="isMobile ? '94%' : '560px'" destroy-on-close>
      <el-form :model="form" label-width="110px">
        <el-form-item label="监听端口" required><el-input-number v-model="form.listen_port" :min="1" :max="65535" style="width: 100%" /></el-form-item>
        <el-form-item label="目标主机" required><el-input v-model="form.target_host" placeholder="127.0.0.1" /></el-form-item>
        <el-form-item label="目标端口" required><el-input-number v-model="form.target_port" :min="1" :max="65535" style="width: 100%" /></el-form-item>
        <el-form-item label="说明"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="立即启用"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" @click="save()">保存并生效</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { useIsMobile } from '@/utils/mobile';
import { selfhostApi, type PortMap } from '@/api/selfhost';

const isMobile = useIsMobile();
const loading = ref(false);
const visible = ref(false);
const maps = ref<PortMap[]>([]);
const form = reactive<Partial<PortMap>>({});

async function load() {
  loading.value = true;
  try {
    maps.value = (await selfhostApi.adminPortMaps()).data.data.maps || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}
function openEditor(row?: PortMap) {
  Object.keys(form).forEach((k) => delete (form as any)[k]);
  Object.assign(form, row ? { ...row } : { listen_port: 19100, target_host: '127.0.0.1', target_port: 19099, description: '', enabled: true });
  visible.value = true;
}
async function save() {
  try {
    await selfhostApi.adminSavePortMap(form);
    visible.value = false;
    ElMessage.success('映射已生效');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
}
async function toggle(row: PortMap) {
  try {
    await selfhostApi.adminTogglePortMap(row.id, !row.enabled);
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}
async function remove(id: number) {
  try {
    await ElMessageBox.confirm('删除后监听立即停止，确定继续？', '提示', { type: 'warning' });
    await selfhostApi.adminDeletePortMap(id);
    await load();
  } catch (e: any) {
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}
onMounted(load);
</script>

<style scoped>
.pm-wrap { padding: 16px; }
.pm-head { display: flex; justify-content: space-between; align-items: center; gap: 16px; }
.pm-title { font-size: 18px; font-weight: 700; }
.pm-sub { font-size: 12px; color: var(--el-text-color-secondary); }
.pm-notice { margin-bottom: 12px; }
@media (max-width: 767px) {
  .pm-wrap { padding: 8px; }
  .pm-head { flex-direction: column; align-items: stretch; gap: 10px; }
  .pm-head .el-button { width: 100%; }
}
</style>
