<template>
  <div class="ar-wrap">
    <el-card shadow="never">
      <template #header>
        <div class="ar-head">
          <div>
            <div class="ar-title">自托管运行时管理</div>
            <div class="ar-sub">维护语言与版本目录（如 Java 21 / Go 1.26 / Node 22）。平台只登记版本，不负责安装。</div>
          </div>
          <el-button type="primary" @click="openEditor()">新增运行时</el-button>
        </div>
      </template>
      <el-table :data="runtimes" v-loading="loading" stripe>
        <el-table-column label="语言" width="120" prop="language" />
        <el-table-column label="版本" width="120" prop="version" />
        <el-table-column label="名称" min-width="160" prop="label" />
        <el-table-column label="类型" width="120">
          <template #default="{ row }">{{ row.kind === 'compiled' ? '编译型（须提供源码）' : '解释型' }}</template>
        </el-table-column>
        <el-table-column label="启用" width="90">
          <template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="安装提示" min-width="220" show-overflow-tooltip prop="install_hint" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEditor(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && !runtimes.length" description="还没有运行时版本" />
    </el-card>

    <el-dialog v-model="visible" :title="form.id ? '编辑运行时' : '新增运行时'" width="600px" destroy-on-close>
      <el-form :model="form" label-width="110px">
        <el-form-item label="语言标识" required><el-input v-model="form.language" placeholder="java / go / node / python" /></el-form-item>
        <el-form-item label="版本号" required><el-input v-model="form.version" placeholder="21 / 1.26 / 22" /></el-form-item>
        <el-form-item label="显示名称"><el-input v-model="form.label" /></el-form-item>
        <el-form-item label="类型">
          <el-radio-group v-model="form.kind">
            <el-radio value="compiled">编译型（必须附源码）</el-radio>
            <el-radio value="interpreted">解释型</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="安装提示"><el-input v-model="form.install_hint" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="下载地址"><el-input v-model="form.download_url" /></el-form-item>
        <el-form-item label="校验和"><el-input v-model="form.checksum" placeholder="sha256（可选）" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="form.sort" :min="0" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save()">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { selfhostApi, type SelfHostRuntime } from '@/api/selfhost';

const loading = ref(false);
const saving = ref(false);
const visible = ref(false);
const runtimes = ref<SelfHostRuntime[]>([]);
const form = reactive<Partial<SelfHostRuntime>>({});

async function load() {
  loading.value = true;
  try {
    runtimes.value = (await selfhostApi.adminListRuntimes()).data.data.runtimes || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

function openEditor(row?: SelfHostRuntime) {
  Object.keys(form).forEach((k) => delete (form as any)[k]);
  Object.assign(form, row ? { ...row } : { language: '', version: '', label: '', kind: 'interpreted', install_hint: '', download_url: '', checksum: '', sort: 0, enabled: true });
  visible.value = true;
}

async function save() {
  saving.value = true;
  try {
    await selfhostApi.adminSaveRuntime(form, form.id);
    visible.value = false;
    ElMessage.success('已保存');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function remove(id: number) {
  try {
    await ElMessageBox.confirm('确定删除这个运行时版本？仍有项目绑定时会被拒绝。', '提示', { type: 'warning' });
    await selfhostApi.adminDeleteRuntime(id);
    ElMessage.success('已删除');
    await load();
  } catch (e: any) {
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

onMounted(load);
</script>

<style scoped>
.ar-wrap { padding: 16px; }
.ar-head { display: flex; justify-content: space-between; align-items: center; }
.ar-title { font-size: 18px; font-weight: 700; }
.ar-sub { font-size: 12px; color: var(--el-text-color-secondary); }
</style>
