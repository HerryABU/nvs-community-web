<template>
  <div class="sh-wrap">
    <el-card shadow="never">
      <template #header>
        <div class="sh-head">
          <div>
            <div class="sh-title">我的自托管项目</div>
            <div class="sh-sub">源码与可执行文件由平台托管；作者自带服务，审核通过后才能开放反向代理。</div>
          </div>
          <div class="sh-actions">
            <el-input v-model="keyword" placeholder="搜索项目" clearable style="width: 200px" @keyup.enter="load()" />
            <el-button @click="$router.push('/htmls')">扩展应用管理</el-button>
            <el-button @click="openMyRuntimeRequests()">运行时申请</el-button>
            <el-button @click="openMyGrants()">我的授权</el-button>
            <el-button type="primary" @click="openEditor()">新建项目</el-button>
          </div>
        </div>
      </template>

      <el-alert type="info" :closable="false" show-icon class="sh-notice">
        编译型语言（Java / Go / .NET 等）必须随项目提供源码，站长可在后台调整该规则并逐项目豁免。
        平台不会在本机运行你的程序；请填写你自己服务器上的服务地址。
      </el-alert>

      <el-table :data="projects" v-loading="loading" stripe>
        <el-table-column label="项目" min-width="180">
          <template #default="{ row }">
            <div class="sh-name">{{ row.name }}</div>
            <div class="sh-slug">{{ routePrefix(row) }}</div>
          </template>
        </el-table-column>
        <el-table-column label="运行时" width="150">
          <template #default="{ row }">{{ row.language }} {{ row.runtime?.version || '' }}</template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="模式" width="110">
          <template #default="{ row }">{{ row.mode === 'compiled' ? '编译版' : '源码' }}</template>
        </el-table-column>
        <el-table-column label="审核" width="130">
          <template #default="{ row }"><el-tag :type="statusType(row.review_status)" size="small">{{ statusText(row.review_status) }}</el-tag></template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="可见" width="100">
          <template #default="{ row }">{{ row.visibility === 'public' ? '公开' : '私有' }}</template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="更新时间" width="170">
          <template #default="{ row }">{{ fmt(row.updated_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" min-width="290" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEditor(row)">编辑</el-button>
            <el-button link type="primary" @click="openDetail(row)">文件与记录</el-button>
            <el-button link type="primary" :disabled="row.review_status === 'pending'" @click="submit(row.id)">提交审核</el-button>
            <el-button link :type="row.visibility === 'public' ? 'warning' : 'primary'" :disabled="row.review_status !== 'approved'" @click="toggleVisibility(row)">
              {{ row.visibility === 'public' ? '转私有' : '公开' }}
            </el-button>
            <el-button link type="danger" @click="remove(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && !projects.length" description="还没有项目，点「新建项目」开始" />
    </el-card>

    <el-dialog v-model="editorVisible" :title="form.id ? '编辑项目' : '新建项目'" :width="isMobile ? '94%' : '640px'" destroy-on-close>
      <el-form :model="form" label-width="110px">
        <el-form-item label="项目名称" required><el-input v-model="form.name" maxlength="128" /></el-form-item>
        <el-form-item label="项目标识" required>
          <el-input v-model="form.slug" :disabled="!!form.id" placeholder="小写字母、数字、连字符，如 order-svc" />
        </el-form-item>
        <el-form-item label="简介"><el-input v-model="form.description" type="textarea" :rows="2" maxlength="512" /></el-form-item>
        <el-form-item label="运行时" required>
          <el-select v-model="form.runtime_id" filterable placeholder="选择语言版本" style="width: 100%">
            <el-option v-for="r in runtimes" :key="r.id" :label="`${r.label || r.language} ${r.version}（${r.kind === 'compiled' ? '编译型' : '解释型'}）`" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="模式">
          <el-radio-group v-model="form.mode">
            <el-radio value="source">源码模式</el-radio>
            <el-radio value="compiled">编译版（另附可执行文件）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="服务地址" required>
          <el-input v-model="form.upstream" placeholder="https://your-host:9000 或 your-host:9000/app" />
          <div class="sh-hint">填你自己服务器上的地址；不能指向平台本机 localhost/127.0.0.1。</div>
        </el-form-item>
        <el-form-item label="构建命令"><el-input v-model="form.build_command" placeholder="如 mvn -q package" /></el-form-item>
        <el-form-item label="启动命令" required>
          <el-input v-model="form.start_command" placeholder="如 java -jar app.jar --port 9000" />
          <div class="sh-hint">上传可执行文件的项目必须填写启动参数，否则无法提交审核。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editorVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save()">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="detailVisible" title="文件与审核记录" :width="isMobile ? '94%' : '720px'" destroy-on-close>
      <template v-if="detail">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="项目">{{ detail.name }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ statusText(detail.review_status) }}</el-descriptions-item>
          <el-descriptions-item label="审核意见" :span="2">{{ detail.review_note || '—' }}</el-descriptions-item>
          <el-descriptions-item label="占用体积">{{ mb(detail.storage_bytes) }} MB</el-descriptions-item>
          <el-descriptions-item label="启动参数">{{ detail.start_command || '—' }}</el-descriptions-item>
          <el-descriptions-item label="代理流量" :span="2">
            上行 {{ mb(detail.usage?.bytes_up) }} MB · 下行 {{ mb(detail.usage?.bytes_down) }} MB · {{ detail.usage?.requests || 0 }} 次请求
          </el-descriptions-item>
        </el-descriptions>
        <div class="sh-uploads">
          <div class="sh-upload">
            <div class="sh-upload-title">源码包（zip / tar / gz / tgz，≤50MB）</div>
            <el-upload :auto-upload="true" :show-file-list="false" :http-request="(o: any) => doUpload(detail.id, 'source', o.file)">
              <el-button size="small">上传源码</el-button>
            </el-upload>
            <div v-if="detail.source_name" class="sh-hint">
              已上传：{{ detail.source_name }}
              <el-link type="primary" @click="downloadFile(detail, 'source')">下载</el-link>
            </div>
          </div>
          <div class="sh-upload">
            <div class="sh-upload-title">可执行文件（jar / exe / bin…，≤200MB）</div>
            <el-upload :auto-upload="true" :show-file-list="false" :http-request="(o: any) => doUpload(detail.id, 'binary', o.file)">
              <el-button size="small">上传可执行文件</el-button>
            </el-upload>
            <div v-if="detail.binary_name" class="sh-hint">
              已上传：{{ detail.binary_name }}
              <el-link type="primary" @click="downloadFile(detail, 'binary')">下载</el-link>
            </div>
          </div>
        </div>
        <el-divider content-position="left">文件共享与授权</el-divider>
        <div class="sh-sharing">
          <div class="sh-share-row">
            <span>允许已登录用户查看源码</span>
            <el-switch v-model="sharingSource" @change="saveSharing()" />
          </div>
          <div class="sh-share-row">
            <span>允许已登录用户下载可执行文件</span>
            <el-switch v-model="sharingBinary" @change="saveSharing()" />
          </div>
          <div class="sh-hint">默认全部关闭。公开项目不等于公开源码；也可以只给指定用户单独授权。</div>
        </div>
        <div v-if="grants.length" class="sh-grants">
          <div class="sh-log-title">访问申请（{{ grants.length }}）</div>
          <el-table :data="grants" size="small">
            <el-table-column label="用户" width="90" prop="user_id" />
            <el-table-column label="类型" width="90">
              <template #default="{ row }">{{ row.kind === 'source' ? '源码' : row.kind === 'binary' ? '可执行文件' : '全部' }}</template>
            </el-table-column>
            <el-table-column label="用途" min-width="160" show-overflow-tooltip prop="note" />
            <el-table-column label="状态" width="90">
              <template #default="{ row }"><el-tag size="small" :type="grantType(row.status)">{{ grantText(row.status) }}</el-tag></template>
            </el-table-column>
            <el-table-column label="操作" width="140">
              <template #default="{ row }">
                <el-button v-if="row.status === 'pending'" link type="primary" @click="review(row.id, true)">批准</el-button>
                <el-button v-if="row.status === 'pending'" link @click="review(row.id, false)">拒绝</el-button>
                <el-button v-if="row.status === 'approved'" link type="danger" @click="revoke(row.id)">撤销</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
        <div class="sh-log-title">审核记录</div>
        <el-timeline>
          <el-timeline-item v-for="log in logs" :key="log.id" :timestamp="fmt(log.created_at)" placement="top">
            {{ actionText(log.action) }} {{ log.note }}
          </el-timeline-item>
        </el-timeline>
        <el-empty v-if="!logs.length" description="暂无审核记录" :image-size="50" />
      </template>
    </el-dialog>

    <el-dialog v-model="runtimeRequestVisible" title="申请缺失的运行时" :width="isMobile ? '94%' : '520px'" destroy-on-close>
      <p class="sh-hint">没有你需要的语言/版本？提交给管理员。管理员会查看你的程序或代码说明，按原样添加或给出更合适的版本。</p>
      <el-form label-width="100px">
        <el-form-item label="语言" required><el-input v-model="rqLanguage" placeholder="如 java / node / go" /></el-form-item>
        <el-form-item label="版本号" required><el-input v-model="rqVersion" placeholder="如 21 / 22 / 1.26" /></el-form-item>
        <el-form-item label="说明"><el-input v-model="rqNote" type="textarea" :rows="3" maxlength="1000" placeholder="说明用途，或贴上代码需要的最低版本" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="runtimeRequestVisible = false">取消</el-button>
        <el-button type="primary" @click="doRuntimeRequest()">提交申请</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="runtimeRequestsVisible" title="我的运行时申请" :width="isMobile ? '94%' : '640px'" destroy-on-close>
      <el-table :data="runtimeRequests" size="small">
        <el-table-column label="语言" width="100" prop="language" />
        <el-table-column label="版本" width="100" prop="version" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }"><el-tag size="small" :type="row.status === 'approved' ? 'success' : row.status === 'pending' ? 'warning' : 'info'">{{ row.status === 'approved' ? '已批准' : row.status === 'pending' ? '待处理' : '已驳回' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="最终版本" min-width="120">
          <template #default="{ row }">{{ row.final_language ? `${row.final_language} ${row.final_version}` : '—' }}</template>
        </el-table-column>
        <el-table-column label="管理员意见" min-width="180" show-overflow-tooltip prop="admin_note" />
      </el-table>
      <el-empty v-if="!runtimeRequests.length" description="暂无申请" :image-size="50" />
      <template #footer><el-button type="primary" @click="runtimeRequestVisible = true">新建申请</el-button></template>
    </el-dialog>

    <el-dialog v-model="myGrantsVisible" title="我的文件访问授权" :width="isMobile ? '94%' : '560px'" destroy-on-close>
      <el-table :data="myGrants" size="small">
        <el-table-column label="项目" min-width="120" prop="project_id" />
        <el-table-column label="类型" width="90">
          <template #default="{ row }">{{ row.kind === 'source' ? '源码' : row.kind === 'binary' ? '可执行文件' : '全部' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }"><el-tag size="small" :type="grantType(row.status)">{{ grantText(row.status) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="说明" min-width="160" show-overflow-tooltip prop="note" />
      </el-table>
      <el-empty v-if="!myGrants.length" description="还没有申请记录" :image-size="50" />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { selfhostApi, type SelfHostProject, type SelfHostRuntime, type SelfHostReviewLog, type SelfHostGrant, type RuntimeRequest } from '@/api/selfhost';
import { useIsMobile } from '@/utils/mobile';

const isMobile = useIsMobile();
const keyword = ref('');
const loading = ref(false);
const saving = ref(false);
const projects = ref<SelfHostProject[]>([]);
const runtimes = ref<SelfHostRuntime[]>([]);
const editorVisible = ref(false);
const detailVisible = ref(false);
const detail = ref<SelfHostProject | null>(null);
const logs = ref<SelfHostReviewLog[]>([]);
const grants = ref<SelfHostGrant[]>([]);
const myGrants = ref<SelfHostGrant[]>([]);
const myGrantsVisible = ref(false);
const runtimeRequestVisible = ref(false);
const runtimeRequestsVisible = ref(false);
const runtimeRequests = ref<RuntimeRequest[]>([]);
const rqLanguage = ref('');
const rqVersion = ref('');
const rqNote = ref('');
const sharingSource = ref(false);
const sharingBinary = ref(false);
const form = reactive<Partial<SelfHostProject>>({});

const statusText = (s: string) => ({ draft: '草稿', pending: '待审核', approved: '已通过', rejected: '已驳回', suspended: '已停止' } as Record<string, string>)[s] || s;
const statusType = (s: string) => ({ draft: 'info', pending: 'warning', approved: 'success', rejected: 'danger', suspended: 'danger' } as Record<string, string>)[s] || 'info';
const actionText = (a: string) => ({ submit: '提交审核', approved: '审核通过', rejected: '审核驳回', suspend: '停止项目', restore: '恢复项目' } as Record<string, string>)[a] || a;
const grantText = (s: string) => ({ pending: '待审批', approved: '已批准', rejected: '已拒绝' } as Record<string, string>)[s] || s;
const grantType = (s: string) => ({ pending: 'warning', approved: 'success', rejected: 'info' } as Record<string, string>)[s] || 'info';
const fmt = (s?: string) => (s ? new Date(s).toLocaleString('zh-CN') : '—');
const mb = (n?: number) => ((Number(n) || 0) / 1048576).toFixed(2);
// 路由前缀由后台在审核通过后配置，项目列表里只展示约定路径。
const routePrefix = (row: SelfHostProject) => (row.route?.prefix ? `${row.route.prefix}/` : '未配置代理路径');

async function load() {
  loading.value = true;
  try {
    const [a, b] = await Promise.all([selfhostApi.listMyProjects(keyword.value), selfhostApi.listRuntimes()]);
    projects.value = a.data.data.projects || [];
    runtimes.value = b.data.data.runtimes || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

function openEditor(row?: SelfHostProject) {
  Object.keys(form).forEach((k) => delete (form as any)[k]);
  Object.assign(form, row ? { ...row } : { name: '', slug: '', description: '', language: 'node', runtime_id: runtimes.value[0]?.id || 0, mode: 'source', upstream: '', build_command: '', start_command: '' });
  editorVisible.value = true;
}

async function save() {
  if (!form.name || !form.slug) {
    ElMessage.warning('请填写项目名称与标识');
    return;
  }
  if (form.mode === 'compiled' && !(form.start_command || '').trim()) {
    ElMessage.warning('编译版项目必须填写启动参数');
    return;
  }
  saving.value = true;
  try {
    const rt = runtimes.value.find((r) => r.id === form.runtime_id);
    const payload = { ...form, language: rt?.language || form.language };
    if (form.id) await selfhostApi.updateProject(form.id, payload);
    else await selfhostApi.createProject(payload);
    editorVisible.value = false;
    ElMessage.success('已保存，请到「文件与记录」上传源码');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function openDetail(row: SelfHostProject) {
  try {
    const res = await selfhostApi.getProject(row.id);
    detail.value = res.data.data.project;
    logs.value = res.data.data.logs || [];
    grants.value = res.data.data.grants || [];
    sharingSource.value = !!detail.value.source_shared;
    sharingBinary.value = !!detail.value.binary_shared;
    detailVisible.value = true;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '读取失败');
  }
}

async function doUpload(id: number, kind: 'source' | 'binary', file: File) {
  try {
    await selfhostApi.uploadFile(id, kind, file.name, file);
    ElMessage.success('上传成功，重新提交审核后生效');
    if (detail.value) await openDetail(detail.value);
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '上传失败');
  }
}

async function submit(id: number) {
  try {
    await selfhostApi.submit(id);
    ElMessage.success('已提交审核');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败');
  }
}

async function toggleVisibility(row: SelfHostProject) {
  try {
    await selfhostApi.setVisibility(row.id, row.visibility === 'public' ? 'private' : 'public');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function remove(id: number) {
  try {
    await ElMessageBox.confirm('删除后源码与可执行文件会一并清理，确定继续？', '提示', { type: 'warning' });
    await selfhostApi.deleteProject(id);
    ElMessage.success('已删除');
    await load();
  } catch (e: any) {
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

async function downloadFile(p: SelfHostProject | null, kind: 'source' | 'binary') {
  if (!p) return;
  try {
    await selfhostApi.downloadFile(p.id, kind, kind === 'source' ? p.source_name : p.binary_name);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '下载失败');
  }
}

async function saveSharing() {
  if (!detail.value) return;
  try {
    await selfhostApi.setSharing(detail.value.id, sharingSource.value, sharingBinary.value);
    detail.value.source_shared = sharingSource.value;
    detail.value.binary_shared = sharingBinary.value;
    ElMessage.success('共享设置已保存');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
}

async function review(id: number, approve: boolean) {
  try {
    await selfhostApi.reviewGrant(id, approve);
    if (detail.value) await openDetail(detail.value);
    ElMessage.success('已处理');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function revoke(id: number) {
  try {
    await selfhostApi.revokeGrant(id);
    if (detail.value) await openDetail(detail.value);
    ElMessage.success('已撤销');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

function openRuntimeRequest() {
  rqLanguage.value = '';
  rqVersion.value = '';
  rqNote.value = '';
  runtimeRequestVisible.value = true;
}

async function doRuntimeRequest() {
  try {
    await selfhostApi.createRuntimeRequest({ language: rqLanguage.value, version: rqVersion.value, note: rqNote.value });
    runtimeRequestVisible.value = false;
    ElMessage.success('已提交给管理员');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败');
  }
}

async function openMyRuntimeRequests() {
  try {
    runtimeRequests.value = (await selfhostApi.myRuntimeRequests()).data.data.requests || [];
    runtimeRequestsVisible.value = true;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '读取失败');
  }
}

async function openMyGrants() {
  try {
    myGrants.value = (await selfhostApi.myGrants()).data.data.grants || [];
    myGrantsVisible.value = true;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '读取失败');
  }
}

onMounted(load);
</script>

<style scoped>
.sh-wrap { padding: 16px; }
.sh-head { display: flex; justify-content: space-between; align-items: center; gap: 16px; }
.sh-title { font-size: 18px; font-weight: 700; }
.sh-sub, .sh-hint { font-size: 12px; color: var(--el-text-color-secondary); }
.sh-actions { display: flex; gap: 8px; }
.sh-notice { margin-bottom: 12px; }
.sh-name { font-weight: 600; }
.sh-slug { font-size: 12px; color: var(--el-text-color-secondary); }
.sh-uploads { display: flex; gap: 24px; margin: 16px 0; }
.sh-upload { flex: 1; }
.sh-upload-title { font-size: 13px; font-weight: 600; margin-bottom: 8px; }
.sh-log-title { font-weight: 600; margin: 12px 0; }
.sh-sharing { background: var(--el-fill-color-lighter); border-radius: 8px; padding: 12px; }
.sh-share-row { display: flex; justify-content: space-between; align-items: center; min-height: 36px; font-size: 13px; }
.sh-grants { margin-top: 12px; }

@media (max-width: 767px) {
  .sh-wrap { padding: 8px; }
  .sh-head { flex-direction: column; align-items: stretch; gap: 10px; }
  .sh-actions { flex-wrap: wrap; }
  .sh-actions .el-input { width: 100% !important; }
  .sh-uploads { flex-direction: column; gap: 12px; }
  .sh-sharing { padding: 8px; }
  :deep(.el-form-item__label) { padding-bottom: 2px; }
  :deep(.el-descriptions__label) { width: 72px; }
}
</style>
