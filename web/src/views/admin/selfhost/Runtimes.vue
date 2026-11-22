<template>
  <div class="ar-wrap">
    <el-card shadow="never">
      <template #header>
        <div class="ar-head">
          <div>
            <div class="ar-title">自托管运行时管理</div>
            <div class="ar-sub">维护语言与版本目录。可扫描本机 PATH 自动发现，也可手动填写可执行文件位置。</div>
          </div>
          <div class="ar-tools">
            <el-button :loading="scanning" @click="scan()">扫描 PATH</el-button>
            <el-button type="primary" @click="openEditor()">新增运行时</el-button>
          </div>
        </div>
      </template>
      <el-table :data="runtimes" v-loading="loading" stripe>
        <el-table-column label="语言" width="110" prop="language" />
        <el-table-column label="版本" width="110" prop="version" />
        <el-table-column label="名称" min-width="140" prop="label" />
        <el-table-column v-if="!isMobile" label="可执行文件" min-width="200" show-overflow-tooltip prop="exec_path" />
        <el-table-column v-if="!isMobile" label="类型" width="120">
          <template #default="{ row }">{{ row.kind === 'compiled' ? '编译型（须提供源码）' : '解释型' }}</template>
        </el-table-column>
        <el-table-column label="启用" width="90">
          <template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEditor(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && !runtimes.length" description="还没有运行时版本，可点「扫描 PATH」" />
    </el-card>

    <el-card shadow="never" class="ar-req">
      <template #header>
        <div class="ar-head">
          <div>
            <div class="ar-title">用户申请</div>
            <div class="ar-sub">管理员查看程序/代码说明后，可按原样添加，也能直接改成合适的语言与版本。</div>
          </div>
          <el-button @click="loadRequests()">刷新</el-button>
        </div>
      </template>
      <el-table :data="requests" stripe size="small">
        <el-table-column label="ID" width="70" prop="id" />
        <el-table-column label="申请人" width="100" prop="user_id" />
        <el-table-column label="申请" min-width="160">
          <template #default="{ row }">{{ row.language }} {{ row.version }}</template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="说明" min-width="220" show-overflow-tooltip prop="note" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }"><el-tag size="small" :type="reqType(row.status)">{{ reqText(row.status) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 'pending'" link type="primary" @click="openReview(row)">处理</el-button>
            <span v-else class="ar-final">{{ row.final_language }} {{ row.final_version }}</span>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!requests.length" description="暂无运行时申请" :image-size="50" />
    </el-card>

    <el-dialog v-model="visible" :title="form.id ? '编辑运行时' : '新增运行时'" :width="isMobile ? '94%' : '600px'" destroy-on-close>
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
        <el-form-item label="可执行文件位置"><el-input v-model="form.exec_path" placeholder="扫描所得或手动填写，如 C:/tools/jdk21/bin/java.exe" /></el-form-item>
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

    <el-dialog v-model="scanVisible" title="PATH 扫描结果" :width="isMobile ? '94%' : '760px'" destroy-on-close>
      <p class="ar-tip">扫描到的是本机 PATH 中可直接执行的命令。点「添加」即登记到运行时目录；版本探测失败的可手动补版本。</p>
      <el-table :data="candidates" size="small">
        <el-table-column label="语言" width="100" prop="language" />
        <el-table-column label="版本" width="110" prop="version" />
        <el-table-column label="可执行路径" min-width="240" show-overflow-tooltip prop="exec_path" />
        <el-table-column v-if="!isMobile" label="备注" min-width="160" show-overflow-tooltip prop="note" />
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="addCandidate(row)">添加</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!candidates.length" description="PATH 中未发现可识别的运行时" :image-size="50" />
    </el-dialog>

    <el-dialog v-model="reviewVisible" title="处理运行时申请" :width="isMobile ? '94%' : '560px'" destroy-on-close>
      <el-descriptions :column="1" border size="small" class="ar-desc">
        <el-descriptions-item label="原申请">{{ reviewRow?.language }} {{ reviewRow?.version }}</el-descriptions-item>
        <el-descriptions-item label="说明">{{ reviewRow?.note || '—' }}</el-descriptions-item>
      </el-descriptions>
      <el-form label-width="110px" class="ar-review-form">
        <el-form-item label="实际语言"><el-input v-model="reviewLanguage" /></el-form-item>
        <el-form-item label="实际版本"><el-input v-model="reviewVersion" /></el-form-item>
        <el-form-item label="可执行位置"><el-input v-model="reviewExecPath" /></el-form-item>
        <el-form-item label="处理意见"><el-input v-model="reviewNote" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reviewVisible = false">取消</el-button>
        <el-button @click="doReview(false)">驳回</el-button>
        <el-button type="primary" @click="doReview(true)">批准并添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { selfhostApi, type SelfHostRuntime, type RuntimeCandidate, type RuntimeRequest } from '@/api/selfhost';
import { useIsMobile } from '@/utils/mobile';

const isMobile = useIsMobile();
const loading = ref(false);
const saving = ref(false);
const scanning = ref(false);
const visible = ref(false);
const scanVisible = ref(false);
const reviewVisible = ref(false);
const runtimes = ref<SelfHostRuntime[]>([]);
const candidates = ref<RuntimeCandidate[]>([]);
const requests = ref<RuntimeRequest[]>([]);
const form = reactive<Partial<SelfHostRuntime>>({});
const reviewRow = ref<RuntimeRequest | null>(null);
const reviewLanguage = ref('');
const reviewVersion = ref('');
const reviewExecPath = ref('');
const reviewNote = ref('');

const reqText = (s: string) => ({ pending: '待处理', approved: '已批准', rejected: '已驳回' } as Record<string, string>)[s] || s;
const reqType = (s: string) => ({ pending: 'warning', approved: 'success', rejected: 'info' } as Record<string, string>)[s] || 'info';

async function load() {
  loading.value = true;
  try {
    runtimes.value = (await selfhostApi.adminListRuntimes()).data.data.runtimes || [];
    await loadRequests();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}
async function loadRequests() {
  try {
    requests.value = (await selfhostApi.adminRuntimeRequests()).data.data.requests || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '申请加载失败');
  }
}
function openEditor(row?: SelfHostRuntime) {
  Object.keys(form).forEach((k) => delete (form as any)[k]);
  Object.assign(form, row ? { ...row } : { language: '', version: '', label: '', kind: 'interpreted', exec_path: '', install_hint: '', download_url: '', checksum: '', sort: 0, enabled: true });
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
async function scan() {
  scanning.value = true;
  try {
    candidates.value = (await selfhostApi.scanRuntimes()).data.data.candidates || [];
    scanVisible.value = true;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '扫描失败');
  } finally {
    scanning.value = false;
  }
}
async function addCandidate(c: RuntimeCandidate) {
  try {
    await selfhostApi.adminSaveRuntime({
      language: c.language, version: c.version, label: c.label, kind: c.kind,
      exec_path: c.exec_path, install_hint: c.note || 'PATH 扫描发现', enabled: true,
    });
    ElMessage.success('已添加');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '添加失败');
  }
}
function openReview(row: RuntimeRequest) {
  reviewRow.value = row;
  reviewLanguage.value = row.language;
  reviewVersion.value = row.version;
  reviewExecPath.value = '';
  reviewNote.value = '';
  reviewVisible.value = true;
}
async function doReview(approve: boolean) {
  if (!reviewRow.value) return;
  try {
    await selfhostApi.adminReviewRuntimeRequest(reviewRow.value.id, {
      approve, language: reviewLanguage.value, version: reviewVersion.value,
      exec_path: reviewExecPath.value, note: reviewNote.value,
    });
    reviewVisible.value = false;
    ElMessage.success('已处理');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '处理失败');
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
.ar-tools { display: flex; gap: 8px; }
.ar-req { margin-top: 16px; }
.ar-tip { font-size: 13px; color: var(--el-text-color-secondary); line-height: 1.6; }
.ar-review-form { margin-top: 12px; }
.ar-final { font-size: 12px; color: var(--el-text-color-secondary); }
@media (max-width: 767px) {
  .ar-wrap { padding: 8px; }
  .ar-head { flex-direction: column; align-items: stretch; gap: 10px; }
  .ar-tools { flex-wrap: wrap; }
  .ar-tools .el-button { flex: 1; }
  :deep(.el-form-item__label) { padding-bottom: 2px; }
}
</style>
