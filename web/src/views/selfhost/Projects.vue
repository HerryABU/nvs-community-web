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
        <el-table-column label="模式" width="110">
          <template #default="{ row }">{{ row.mode === 'compiled' ? '编译版' : '源码' }}</template>
        </el-table-column>
        <el-table-column label="审核" width="130">
          <template #default="{ row }"><el-tag :type="statusType(row.review_status)" size="small">{{ statusText(row.review_status) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="可见" width="100">
          <template #default="{ row }">{{ row.visibility === 'public' ? '公开' : '私有' }}</template>
        </el-table-column>
        <el-table-column label="更新时间" width="170">
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

    <el-dialog v-model="editorVisible" :title="form.id ? '编辑项目' : '新建项目'" width="640px" destroy-on-close>
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
        <el-form-item label="启动命令"><el-input v-model="form.start_command" placeholder="如 java -jar app.jar --port 9000" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editorVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save()">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="detailVisible" title="文件与审核记录" width="720px" destroy-on-close>
      <template v-if="detail">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="项目">{{ detail.name }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ statusText(detail.review_status) }}</el-descriptions-item>
          <el-descriptions-item label="审核意见" :span="2">{{ detail.review_note || '—' }}</el-descriptions-item>
        </el-descriptions>
        <div class="sh-uploads">
          <div class="sh-upload">
            <div class="sh-upload-title">源码包（zip / tar / gz / tgz，≤50MB）</div>
            <el-upload :auto-upload="true" :show-file-list="false" :http-request="(o: any) => doUpload(detail.id, 'source', o.file)">
              <el-button size="small">上传源码</el-button>
            </el-upload>
            <div v-if="detail.source_name" class="sh-hint">
              已上传：{{ detail.source_name }}
              <el-link type="primary" :href="selfhostApi.fileUrl(detail.id, 'source')" target="_blank">下载</el-link>
            </div>
          </div>
          <div class="sh-upload">
            <div class="sh-upload-title">可执行文件（jar / exe / bin…，≤200MB）</div>
            <el-upload :auto-upload="true" :show-file-list="false" :http-request="(o: any) => doUpload(detail.id, 'binary', o.file)">
              <el-button size="small">上传可执行文件</el-button>
            </el-upload>
            <div v-if="detail.binary_name" class="sh-hint">
              已上传：{{ detail.binary_name }}
              <el-link type="primary" :href="selfhostApi.fileUrl(detail.id, 'binary')" target="_blank">下载</el-link>
            </div>
          </div>
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { selfhostApi, type SelfHostProject, type SelfHostRuntime, type SelfHostReviewLog } from '@/api/selfhost';

const keyword = ref('');
const loading = ref(false);
const saving = ref(false);
const projects = ref<SelfHostProject[]>([]);
const runtimes = ref<SelfHostRuntime[]>([]);
const editorVisible = ref(false);
const detailVisible = ref(false);
const detail = ref<SelfHostProject | null>(null);
const logs = ref<SelfHostReviewLog[]>([]);
const form = reactive<Partial<SelfHostProject>>({});

const statusText = (s: string) => ({ draft: '草稿', pending: '待审核', approved: '已通过', rejected: '已驳回', suspended: '已停止' } as Record<string, string>)[s] || s;
const statusType = (s: string) => ({ draft: 'info', pending: 'warning', approved: 'success', rejected: 'danger', suspended: 'danger' } as Record<string, string>)[s] || 'info';
const actionText = (a: string) => ({ submit: '提交审核', approved: '审核通过', rejected: '审核驳回', suspend: '停止项目', restore: '恢复项目' } as Record<string, string>)[a] || a;
const fmt = (s?: string) => (s ? new Date(s).toLocaleString('zh-CN') : '—');
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
</style>
