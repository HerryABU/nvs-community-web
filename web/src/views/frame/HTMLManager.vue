<template>
  <div class="page-container">
    <div class="page-header">
      <div class="header-left">
        <h2>🔌 扩展应用</h2>
      </div>
      <el-button v-if="authStore.isLoggedIn && tab==='mine'" type="primary" @click="showUpload = true" :icon="Upload">上传ZIP</el-button>
    </div>

    <el-tabs v-model="tab" @tab-change="onTabChange" style="margin-bottom:16px">
      <el-tab-pane label="📂 我的项目" name="mine" />
      <el-tab-pane label="🏪 广场" name="plaza" />
    </el-tabs>

    <template v-if="tab==='mine'">

    <div v-if="htmls.length === 0 && !loading" class="empty-state">
      <el-empty description="还没有扩展应用">
        <el-button type="primary" @click="showUpload = true" :icon="Upload">上传第一个ZIP</el-button>
      </el-empty>
    </div>

    <div v-else class="html-grid">
      <div v-for="h in htmls" :key="h.id" class="html-card" :class="{ inactive: !h.is_active }">
        <div class="card-header">
          <span class="card-name">{{ h.name }}</span>
          <div class="card-badges">
            <el-tag v-if="h.port > 0" type="danger" size="small">端口:{{ h.port }}</el-tag>
            <el-tag v-if="h.allow_wasm" type="warning" size="small">WASM</el-tag>
            <el-tag v-if="h.is_public" type="success" size="small">🌐 广场</el-tag>
            <el-tag type="info" size="small">{{ h.file_count }} 文件</el-tag>
          </div>
        </div>
        <div class="card-body">
          <p class="card-desc">{{ h.description || '暂无描述' }}</p>
          <p class="card-entry">入口: {{ h.entry_file }}</p>
          <p v-if="h.port > 0 && h.project_name" class="card-proxy">
            代理: <code>/sandbox/proxy/{{ h.user_id }}/{{ h.project_name }}/</code>
          </p>
          <div class="card-meta"><span>{{ fmtDate(h.updated_at) }}</span><span>{{ fmtSize(h.total_size) }}</span></div>
        </div>
        <div class="card-actions">
          <el-button size="small" type="primary" @click="openRunner(h)" :icon="VideoPlay">运行</el-button>
          <el-button size="small" @click="preview(h)" :icon="View">预览</el-button>
          <el-button size="small" @click="viewFiles(h)" :icon="FolderOpened">文件</el-button>
          <el-button v-if="authStore.isLoggedIn" size="small" :type="h.is_public?'success':''" @click="togglePublic(h)" :icon="h.is_public?View:View">{{ h.is_public?'已发布':'发布' }}</el-button>
          <el-button v-if="authStore.isLoggedIn" size="small" @click="changeEntry(h)" :icon="Edit">入口</el-button>
          <el-button v-if="authStore.isLoggedIn && !h.port" size="small" type="success" @click="allocPort(h)" :icon="Connection">分配端口</el-button>
          <el-button v-if="authStore.isLoggedIn && h.port" size="small" type="warning" @click="freePort(h)" :icon="Close">释放端口</el-button>
          <el-button v-if="authStore.isLoggedIn" size="small" type="danger" @click="del(h)" :icon="Delete">删除</el-button>
        </div>
      </div>
    </div>

    <!-- 上传对话框 -->
    <el-dialog v-model="showUpload" title="上传扩展ZIP" width="550px" destroy-on-close @closed="resetUpload">
      <el-form :model="uploadForm" label-position="top">
        <el-form-item label="扩展名称" required>
          <el-input v-model="uploadForm.name" maxlength="128" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="uploadForm.description" type="textarea" :rows="2" maxlength="512" />
        </el-form-item>
        <el-form-item label="ZIP文件" required>
          <el-upload ref="uploadRef" drag :auto-upload="false" :limit="1" accept=".zip"
            :on-change="onFileChange" :on-remove="onFileRemove">
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">将 .zip 文件拖到此处，或<em>点击上传</em></div>
            <template #tip>
              <div class="el-upload__tip">最大 20MB · 解压 ≤ 50MB · 压缩比 ≤ 100:1 · 最多500文件</div>
            </template>
          </el-upload>
        </el-form-item>
        <!-- 入口文件选择（上传后展示） -->
        <el-form-item v-if="extractedFiles.length > 0" label="选择入口文件" required>
          <el-select v-model="uploadForm.entry_file" placeholder="选择入口HTML" style="width:100%">
            <el-option v-for="f in htmlOnlyFiles" :key="f" :label="f" :value="f" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-space>
            <el-checkbox v-model="uploadForm.allow_wasm">允许WASM</el-checkbox>
            <el-checkbox v-model="uploadForm.is_public">广场展示</el-checkbox>
            <el-checkbox v-model="uploadForm.is_downloadable">允许下载</el-checkbox>
          </el-space>
        </el-form-item>
      </el-form>
      <el-alert v-if="uploadError" :title="uploadError" type="error" show-icon :closable="false" />
      <el-alert v-if="uploadResult" :title="uploadResult" type="success" show-icon :closable="false" />
      <template #footer>
        <el-button @click="showUpload = false">取消</el-button>
        <el-button type="primary" @click="doUpload" :loading="uploading" :disabled="!uploadFile">
          {{ uploading ? '上传中...' : '上传并解压' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 修改入口对话框 -->
    <el-dialog v-model="showEntryChange" title="修改入口文件" width="500px">
      <el-form label-position="top">
        <el-form-item label="当前入口: {{ changingEntryName }}">
          <el-select v-model="newEntryFile" placeholder="选择新的入口HTML" style="width:100%">
            <el-option v-for="f in changingFiles" :key="f.name" :label="f.name" :value="f.name" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEntryChange = false">取消</el-button>
        <el-button type="primary" @click="saveEntry">保存</el-button>
      </template>
    </el-dialog>

    <!-- 预览 -->
    <el-dialog v-model="showPreview" title="沙盒预览" width="85%" destroy-on-close>
      <SandboxPreview v-if="previewSrc" :src="previewSrc" :title="previewName" height="600px"
        sandbox-policy="allow-scripts allow-same-origin allow-forms" />
      <p class="network-hint">💡 此沙盒环境允许网络请求（fetch/XHR/WebSocket），不阻断网络</p>
    </el-dialog>

    <!-- 文件树 -->
    <el-dialog v-model="showFiles" :title="'📁 ' + viewingName" width="650px">
      <el-tree :data="fileTree" :props="{label:'name',children:'children'}" node-key="name" default-expand-all style="max-height:450px;overflow:auto" v-loading="fileTreeLoading">
        <template #default="{node,data}">
          <span style="display:flex;justify-content:space-between;width:100%">
            <span>{{ data.icon }} {{ node.label }}</span>
            <span v-if="!data.children" style="color:var(--text-secondary);font-size:.8rem">{{ fmtSize(data.size) }}</span>
          </span>
        </template>
      </el-tree>
    </el-dialog>
</template>

<!-- 广场Tab -->
<div v-if="tab==='plaza'">
  <div v-if="plazaItems.length===0&&!plazaLoading" class="empty-state"><el-empty description="还没有公开项目" /></div>
  <div v-else class="plaza-grid">
    <div v-for="h in plazaItems" :key="h.id" class="plaza-card">
      <a :href="'/app/'+h.id" target="_blank" style="text-decoration:none;color:inherit">
        <div class="plaza-thumb">
          <img v-if="h.thumb_url" :src="h.thumb_url" class="plaza-img" />
          <div v-else class="plaza-placeholder">🔌</div>
        </div>
      </a>
      <div class="plaza-info">
        <div class="plaza-title">{{ h.name }}</div>
        <div class="plaza-author">by {{ h.user?.nickname || h.user?.username }}</div>
        <div class="plaza-actions">
          <a :href="'/app/'+h.id" target="_blank"><el-button size="small" type="primary">▶ 运行</el-button></a>
          <a v-if="h.is_downloadable" :href="'/api/userhtmls/'+h.id+'/download'"><el-button size="small">📥</el-button></a>
        </div>
      </div>
    </div>
  </div>
</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Upload, View, Delete, FolderOpened, Edit, Connection, Close, UploadFilled, VideoPlay } from '@element-plus/icons-vue';
import { htmlApi } from '@/api/frame';
import api from '@/api/index';
import { useAuthStore } from '@/stores/auth';
import SandboxPreview from './SandboxPreview.vue';

const authStore = useAuthStore();
const router = useRouter();
const tab = ref(authStore.isLoggedIn ? 'mine' : 'plaza');
const plazaItems = ref<any[]>([]);
const plazaLoading = ref(false);

function onTabChange(t: string) { if (t === 'plaza') loadPlaza(); }
async function loadPlaza() {
  plazaLoading.value = true;
  try {
    const r = await api.get('/htmls/public', { params: { page: 1, size: 30 } });
    if (r.data.code === 0) plazaItems.value = r.data.data.items;
  } catch { /* */ }
  plazaLoading.value = false;
}

function openRunner(h: HTMLItem) { window.open(`/app/${h.id}`, '_blank'); }

interface HTMLItem {
  id: number; user_id: number; name: string; description: string; entry_file: string;
  file_count: number; total_size: number; port: number; project_name: string;
  is_active: boolean; is_public: boolean; allow_wasm: boolean; updated_at: string;
}
interface ZipEntryItem { name: string; size: number; }

const htmls = ref<HTMLItem[]>([]); const loading = ref(false);
const showUpload = ref(false); const uploading = ref(false);
const uploadFile = ref<File | null>(null); const uploadError = ref(''); const uploadResult = ref('');
const uploadRef = ref<any>(); const extractedFiles = ref<ZipEntryItem[]>([]);
const showPreview = ref(false); const previewSrc = ref(''); const previewName = ref('');
const showFiles = ref(false); const fileTree = ref<any[]>([]); const fileTreeLoading = ref(false); const viewingName = ref('');
const showEntryChange = ref(false); const newEntryFile = ref(''); const changingEntryName = ref('');
const changingFiles = ref<ZipEntryItem[]>([]); const changingHtmlId = ref(0);

const uploadForm = ref({ name: '', description: '', entry_file: '', allow_wasm: false, is_public: false, is_downloadable: false });

const htmlOnlyFiles = computed(() =>
  extractedFiles.value.filter(f => f.name.endsWith('.html') || f.name.endsWith('.htm')).map(f => f.name)
);

function resetUpload() {
  uploadForm.value = { name: '', description: '', entry_file: '', allow_wasm: false, is_public: false, is_downloadable: false };
  uploadFile.value = null; uploadError.value = ''; uploadResult.value = '';
  extractedFiles.value = []; uploadRef.value?.clearFiles();
}

onMounted(() => load());

async function load() {
  loading.value = true;
  try { const r = await htmlApi.list(); if (r.data.code === 0) htmls.value = r.data.data; } catch { ElMessage.error('加载失败'); }
  finally { loading.value = false; }
}

function onFileChange(file: any) {
  uploadFile.value = file.raw; uploadError.value = '';
  // 前端预览ZIP文件列表（客户端解压预览）
  previewZipFiles(file.raw);
}

function onFileRemove() { uploadFile.value = null; extractedFiles.value = []; }

async function previewZipFiles(file: File) {
  try {
    // 使用 Web API 读取ZIP文件列表（仅展示，不上传）
    const arrayBuffer = await file.arrayBuffer();
    // 简单解析ZIP中央目录获取文件名
    const view = new DataView(arrayBuffer);
    const files: ZipEntryItem[] = [];
    // 扫描本地文件头签名 PK\x03\x04
    let offset = 0;
    while (offset < arrayBuffer.byteLength - 30) {
      if (view.getUint32(offset, true) === 0x04034b50) {
        const nameLen = view.getUint16(offset + 26, true);
        const extraLen = view.getUint16(offset + 28, true);
        const compSize = view.getUint32(offset + 18, true);
        const nameStart = offset + 30;
        if (nameStart + nameLen <= arrayBuffer.byteLength) {
          const name = new TextDecoder().decode(new Uint8Array(arrayBuffer, nameStart, nameLen));
          if (!name.endsWith('/')) {
            files.push({ name, size: compSize });
          }
        }
        offset += 30 + nameLen + extraLen + compSize;
      } else {
        offset++;
      }
    }
    extractedFiles.value = files;
    if (htmlOnlyFiles.value.length > 0) {
      // 自动选 index.html
      const idx = htmlOnlyFiles.value.find(f => f.toLowerCase().endsWith('index.html') || f.toLowerCase().endsWith('index.htm'));
      uploadForm.value.entry_file = idx || htmlOnlyFiles.value[0];
    }
  } catch { /* 预览失败，上传时后端会处理 */ }
}

async function doUpload() {
  if (!uploadForm.value.name.trim()) { ElMessage.warning('请输入名称'); return; }
  if (!uploadFile.value) { ElMessage.warning('请选择ZIP文件'); return; }
  uploading.value = true; uploadError.value = ''; uploadResult.value = '';
  try {
    const fd = new FormData();
    fd.append('name', uploadForm.value.name);
    fd.append('description', uploadForm.value.description || '');
    fd.append('entry_file', uploadForm.value.entry_file || '');
    fd.append('allow_wasm', uploadForm.value.allow_wasm ? 'true' : 'false');
    fd.append('is_public', uploadForm.value.is_public ? 'true' : 'false');
    fd.append('is_downloadable', uploadForm.value.is_downloadable ? 'true' : 'false');
    fd.append('file', uploadFile.value);
    const r = await htmlApi.upload(fd);
    if (r.data.code === 0) {
      uploadResult.value = `${r.data.data.files?.length || 0} 个文件已解压`;
      ElMessage.success('上传成功');
      setTimeout(() => { showUpload.value = false; resetUpload(); load(); }, 1500);
    } else {
      uploadError.value = r.data.message || '上传失败';
    }
  } catch (e: any) {
    uploadError.value = e.response?.data?.message || '上传失败';
  }
  finally { uploading.value = false; }
}

function preview(h: HTMLItem) {
  previewSrc.value = htmlApi.getPreview(h.id);
  previewName.value = h.name;
  showPreview.value = true;
}

async function togglePublic(h: HTMLItem) {
  try {
    await htmlApi.update(h.id, { is_public: !h.is_public });
    h.is_public = !h.is_public;
    ElMessage.success(h.is_public ? '已发布到广场' : '已从广场撤回');
  } catch { ElMessage.error('操作失败'); }
}
async function viewFiles(h: HTMLItem) {
  viewingName.value = h.name;
  fileTreeLoading.value = true;
  showFiles.value = true;
  try {
    const r = await htmlApi.get(h.id);
    if (r.data.code === 0) fileTree.value = buildFileTree(r.data.data.files || []);
  } catch { fileTree.value = []; }
  fileTreeLoading.value = false;
}
function buildFileTree(files: ZipEntryItem[]): any[] {
  const root: any = {};
  files.forEach(f => {
    if (f.name.includes('node_modules/') || f.name.startsWith('node_modules')) return;
    const parts = f.name.split('/');
    let node = root;
    parts.forEach((part, i) => {
      if (!node[part]) node[part] = i === parts.length - 1 ? { name: part, size: f.size, icon: iconFor(part) } : { name: part, children: {} };
      node = i < parts.length - 1 ? (node[part].children || (node[part].children = {})) : null;
    });
  });
  return flattenTree(root);
}
function iconFor(name: string) { const e = name.split('.').pop()?.toLowerCase(); return {html:'🌐',css:'🎨',js:'📜',wasm:'⚙️',json:'📋',png:'🖼',jpg:'🖼',svg:'🖼',woff2:'🔤',ttf:'🔤',md:'📝'}[e||''] || '📄'; }
function flattenTree(obj: any): any[] {
  return Object.values(obj).map((v: any) => {
    if (v.children) { v.children = flattenTree(v.children); v.icon = '📁'; delete v.size; }
    return v;
  });
}

async function changeEntry(h: HTMLItem) {
  changingHtmlId.value = h.id;
  changingEntryName.value = h.entry_file;
  try {
    const r = await htmlApi.get(h.id);
    if (r.data.code === 0) changingFiles.value = (r.data.data.files || []).filter((f: ZipEntryItem) =>
      f.name.endsWith('.html') || f.name.endsWith('.htm')
    );
  } catch { changingFiles.value = []; }
  newEntryFile.value = h.entry_file;
  showEntryChange.value = true;
}

async function saveEntry() {
  try {
    await htmlApi.update(changingHtmlId.value, { entry_file: newEntryFile.value });
    ElMessage.success('入口文件已更新');
    showEntryChange.value = false;
    await load();
  } catch { ElMessage.error('更新失败'); }
}

async function allocPort(h: HTMLItem) {
  try {
    const { value } = await ElMessageBox.prompt('请为项目命名（字母/数字/下划线/横线，2-32字符，URL中会隐藏端口号）', '命名项目', {
      confirmButtonText: '分配',
      cancelButtonText: '取消',
      inputPattern: /^[a-zA-Z0-9_-]{2,32}$/,
      inputErrorMessage: '仅允许字母/数字/下划线/横线，2-32字符',
    });
    if (!value) return;
    const r = await api.post('/port/allocate', { html_id: h.id, project_name: value });
    if (r.data.code === 0) {
      const d = r.data.data;
      ElMessage.success(`端口 ${d.port} 已分配 · 代理: ${d.proxy_path}`);
      await load();
    } else { ElMessage.error(r.data.message); }
  } catch { /* cancelled */ }
}

async function freePort(h: HTMLItem) {
  if (!h.port) return;
  try {
    await api.post('/port/release', { html_id: h.id, project_name: h.project_name, port: h.port });
    ElMessage.success(`代理已释放`);
    await load();
  } catch { ElMessage.error('释放失败'); }
}

async function del(h: HTMLItem) {
  try {
    await ElMessageBox.confirm(`删除「${h.name}」及其全部文件？`, '确认', { type: 'warning', confirmButtonText: '删除' });
    if (h.port) await api.post('/port/release', { port: h.port });
    await htmlApi.delete(h.id); ElMessage.success('已删除'); await load();
  } catch { /* cancelled */ }
}

function fmtSize(b: number) { if (b < 1024) return b + 'B'; if (b < 1048576) return (b/1024).toFixed(1)+'KB'; return (b/1048576).toFixed(1)+'MB'; }
function fmtDate(d: string) { return new Date(d).toLocaleDateString('zh-CN'); }
</script>

<style scoped>
.page-container { max-width: 1100px; margin: 0 auto; padding: 24px 20px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; flex-wrap: wrap; gap: 12px; }
.header-left h2 { margin: 0; font-size: 1.4rem; }
.subtitle { color: var(--text-secondary); margin: 4px 0 0; font-size: 0.9rem; }
.html-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 16px; }
.html-card { background: var(--card-bg); border: 1px solid var(--border-color); border-radius: 10px; padding: 16px; display: flex; flex-direction: column; gap: 10px; transition: box-shadow 0.2s; }
.html-card:hover { box-shadow: 0 2px 12px rgba(0,0,0,0.08); }
.html-card.inactive { opacity: 0.6; }
.card-header { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.card-name { font-weight: 600; font-size: 1.05rem; flex: 1; }
.card-badges { display: flex; gap: 4px; flex-wrap: wrap; }
.card-body { flex: 1; }
.card-desc { color: var(--text-secondary); font-size: 0.88rem; margin: 0; }
.card-entry { color: var(--primary-color); font-size: 0.82rem; font-family: monospace; margin: 2px 0 0; }
.card-proxy { color: var(--el-color-danger); font-size: 0.78rem; font-family: monospace; margin: 2px 0 0; }
.card-proxy code { background: rgba(245,108,108,0.1); padding: 1px 4px; border-radius: 3px; }
.card-meta { display: flex; gap: 16px; margin-top: 6px; font-size: 0.8rem; color: var(--text-secondary); }
.card-actions { display: flex; gap: 4px; justify-content: flex-end; flex-wrap: wrap; }
.empty-state { padding: 40px 0; display: flex; justify-content: center; }
.network-hint { color: var(--text-secondary); font-size: 0.85rem; margin-top: 8px; text-align: center; }
.plaza-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(240px,1fr));gap:14px}
.plaza-card{background:var(--card-bg);border:1px solid var(--border-color);border-radius:10px;overflow:hidden;cursor:pointer;transition:box-shadow .2s}
.plaza-card:hover{box-shadow:0 4px 16px rgba(0,0,0,.08)}
.plaza-thumb{height:140px;display:flex;align-items:center;justify-content:center;background:var(--bg-color)}
.plaza-img{width:100%;height:100%;object-fit:cover}
.plaza-placeholder{font-size:2.5rem;opacity:.25}
.plaza-info{padding:10px 12px}
.plaza-title{font-weight:600;font-size:.95rem;margin-bottom:2px}
.plaza-author{color:var(--text-secondary);font-size:.8rem;margin-bottom:6px}
.plaza-actions{display:flex;gap:4px}
</style>
