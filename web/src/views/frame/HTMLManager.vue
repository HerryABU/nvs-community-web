<template>
  <div class="page-container">
    <SectionHeader title="扩展应用" subtitle="上传、管理与运行你的自定义 HTML 扩展应用。" :icon="'🔌'" gradient="primary">
      <template #actions>
        <el-button v-if="authStore.isLoggedIn && tab==='mine'" type="primary" @click="showUpload = true" :icon="Upload">上传ZIP</el-button>
      </template>
    </SectionHeader>

    <el-tabs v-model="tab" @tab-change="onTabChange" style="margin-bottom:16px">
      <el-tab-pane label="📂 我的项目" name="mine" />
      <el-tab-pane label="🏪 广场" name="plaza" />
    </el-tabs>

    <template v-if="tab==='mine'">

    <EmptyState v-if="htmls.length === 0 && !loading" icon="📦" text="还没有扩展应用" description="上传你的第一个 ZIP 包，创建自定义 HTML 扩展应用。">
      <template #actions>
        <el-button type="primary" @click="showUpload = true" :icon="Upload">上传第一个ZIP</el-button>
      </template>
    </EmptyState>

    <div v-else class="html-grid">
      <div v-for="h in htmls" :key="h.id" class="html-card" :class="{ inactive: !h.is_active }">
        <div class="card-header">
          <span class="card-name">{{ h.name }}</span>
          <div class="card-badges">
            <el-tag v-if="h.port > 0" type="danger" size="small">端口:{{ h.port }}</el-tag>
            <el-tag v-if="h.allow_wasm" type="warning" size="small">WASM</el-tag>
            <el-tag v-if="h.is_public" type="success" size="small">🌐 广场</el-tag>
            <el-tag v-if="h.blog_id" type="info" size="small">📝 博客:{{ h.blog_id }}</el-tag>
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

    <!-- 预览 / iframe 嵌入 -->
    <el-dialog v-model="showPreview" title="沙盒预览 / iframe 嵌入" width="90%" destroy-on-close>
      <el-card shadow="never" class="iframe-preview-card">
        <template #header>
          <div class="iframe-ctrl">
            <span class="iframe-ctrl-title">预览窗口</span>
            <el-space wrap>
              <span class="dim-label">宽(px)</span>
              <el-input-number v-model="previewW" :min="200" :max="2000" :step="20" size="small" controls-position="right" />
              <span class="dim-label">高(px)</span>
              <el-input-number v-model="previewH" :min="200" :max="2000" :step="20" size="small" controls-position="right" />
              <el-select v-model="embedSource" placeholder="选择站内扩展嵌入" size="small" clearable style="width:220px" @change="onEmbedChange">
                <el-option v-for="o in htmls" :key="o.id" :label="o.name" :value="o.id" />
              </el-select>
              <el-input-number v-model="previewBlogId" :min="0" :step="1" size="small" placeholder="绑定博客ID" style="width:160px" controls-position="right" />
              <el-button size="small" type="primary" :loading="savingPreview" @click="savePreviewSettings">保存尺寸/绑定</el-button>
            </el-space>
          </div>
        </template>
        <div class="iframe-stage">
          <iframe
            v-if="previewSrc"
            :key="previewSrc"
            :src="previewSrc"
            :style="{ width: previewW + 'px', height: previewH + 'px' }"
            sandbox="allow-scripts allow-forms allow-popups"
            class="embed-iframe"
            title="iframe 预览"
          />
        </div>
      </el-card>
      <el-alert class="compile-hint" type="info" :closable="false" show-icon
        title="编译产物提示"
        description="若 ZIP 解压后的产物路径（入口文件 / 资源目录）发生变更，需重新上传 ZIP，预览与运行才会指向新路径；旧产物路径不会自动更新，请重新上传。" />
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
  <EmptyState v-if="plazaItems.length===0&&!plazaLoading" icon="🏪" text="还没有公开项目" description="成为第一个把扩展应用发布到广场的作者吧。" />
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
          <!-- 🔀 站点路径前缀：运行/下载为直用 URL，需拼 apiPrefix -->
          <a :href="apiPrefix + '/app/'+h.id" target="_blank"><el-button size="small" type="primary">▶ 运行</el-button></a>
          <a v-if="h.is_downloadable" :href="apiPrefix + '/api/userhtmls/'+h.id+'/download'"><el-button size="small">📥</el-button></a>
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
import api, { apiPrefix } from '@/api/index';
import { appPrefix } from '@/utils/prefix';
import { useAuthStore } from '@/stores/auth';

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

function openRunner(h: HTMLItem) { window.open(`${appPrefix}/app/${h.id}`, '_blank'); }

interface HTMLItem {
  id: number; user_id: number; name: string; description: string; entry_file: string;
  file_count: number; total_size: number; port: number; project_name: string;
  is_active: boolean; is_public: boolean; allow_wasm: boolean; updated_at: string;
  frame_width?: number; frame_height?: number; blog_id?: number | null;
}
interface ZipEntryItem { name: string; size: number; }

const htmls = ref<HTMLItem[]>([]); const loading = ref(false);
const showUpload = ref(false); const uploading = ref(false);
const uploadFile = ref<File | null>(null); const uploadError = ref(''); const uploadResult = ref('');
const uploadRef = ref<any>(); const extractedFiles = ref<ZipEntryItem[]>([]);
const showPreview = ref(false); const previewSrc = ref(''); const previewName = ref('');
const previewW = ref(960); const previewH = ref(600); const previewBlogId = ref(0);
const embedSource = ref<number | undefined>(undefined); const savingPreview = ref(false);
const previewTarget = ref<HTMLItem | null>(null);
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
  previewTarget.value = h;
  previewName.value = h.name;
  // 🔀 站点路径前缀：iframe 直用 URL，用 apiPrefix 动态拼接（绝不硬编码 /go-nvs）
  previewSrc.value = `${apiPrefix}/app/${h.id}`;
  previewW.value = h.frame_width || 960;
  previewH.value = h.frame_height || 600;
  previewBlogId.value = h.blog_id ?? 0;
  embedSource.value = h.id;
  showPreview.value = true;
}

// 站内资源选择：从当前用户的 UserHTML 列表选一个已有扩展作为内嵌源（相对路径，用 apiPrefix 拼接）
function onEmbedChange(val: number | undefined) {
  if (val) {
    previewSrc.value = `${apiPrefix}/app/${val}`;
  } else if (previewTarget.value) {
    previewSrc.value = `${apiPrefix}/app/${previewTarget.value.id}`;
  }
}

// 保存当前扩展的预览尺寸与博客绑定（blog_id=0 视为解绑 → null）
async function savePreviewSettings() {
  if (!previewTarget.value) return;
  savingPreview.value = true;
  try {
    const data: Record<string, unknown> = {
      frame_width: previewW.value,
      frame_height: previewH.value,
      blog_id: previewBlogId.value > 0 ? previewBlogId.value : null,
    };
    const r = await htmlApi.update(previewTarget.value.id, data);
    if (r.data.code === 0) {
      const d = r.data.data;
      previewTarget.value.frame_width = d.frame_width ?? previewW.value;
      previewTarget.value.frame_height = d.frame_height ?? previewH.value;
      previewTarget.value.blog_id = d.blog_id ?? (previewBlogId.value > 0 ? previewBlogId.value : null);
      ElMessage.success('尺寸/绑定已保存');
    } else {
      ElMessage.error(r.data.message || '保存失败');
    }
  } catch {
    ElMessage.error('保存失败');
  } finally {
    savingPreview.value = false;
  }
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
.html-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 16px; }
.html-card { background: #fff; border: 1px solid var(--border-color); border-radius: var(--radius-md); padding: 16px; display: flex; flex-direction: column; gap: 10px; transition: box-shadow var(--transition-base); }
.html-card:hover { box-shadow: var(--shadow-md); }
.html-card.inactive { opacity: 0.6; }
.card-header { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.card-name { font-weight: 600; font-size: 1.05rem; flex: 1; }
.card-badges { display: flex; gap: 4px; flex-wrap: wrap; }
.card-body { flex: 1; }
.card-desc { color: var(--text-light); font-size: 0.88rem; margin: 0; }
.card-entry { color: var(--primary-color); font-size: 0.82rem; font-family: monospace; margin: 2px 0 0; }
.card-proxy { color: var(--el-color-danger); font-size: 0.78rem; font-family: monospace; margin: 2px 0 0; }
.card-proxy code { background: rgba(239, 68, 68, 0.1); padding: 1px 4px; border-radius: var(--radius-xs); }
.card-meta { display: flex; gap: 16px; margin-top: 6px; font-size: 0.8rem; color: var(--text-light); }
.card-actions { display: flex; gap: 4px; justify-content: flex-end; flex-wrap: wrap; }
.network-hint { color: var(--text-light); font-size: 0.85rem; margin-top: 8px; text-align: center; }
.iframe-preview-card { margin-bottom: 12px; }
.iframe-ctrl { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; margin-bottom: 4px; }
.iframe-ctrl-title { font-weight: 600; font-size: 0.92rem; color: var(--text-color); }
.dim-label { color: var(--text-light); font-size: 0.82rem; }
.iframe-stage { max-height: 70vh; overflow: auto; display: flex; justify-content: center; background: var(--bg-color); padding: 8px; border-radius: var(--radius-sm); }
.embed-iframe { border: 1px solid var(--border-color); display: block; background: #fff; flex-shrink: 0; }
[data-theme="dark"] .embed-iframe { background: #1a1d27; }
.compile-hint { margin-top: 10px; }
.plaza-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 16px; }
.plaza-card { background: #fff; border: 1px solid var(--border-color); border-radius: var(--radius-md); overflow: hidden; cursor: pointer; transition: box-shadow var(--transition-base); }
.plaza-card:hover { box-shadow: var(--shadow-md); }
.plaza-thumb { height: 140px; display: flex; align-items: center; justify-content: center; background: var(--bg-color); }
.plaza-img { width: 100%; height: 100%; object-fit: cover; }
.plaza-placeholder { font-size: 2.5rem; opacity: 0.25; }
.plaza-info { padding: 10px 12px; }
.plaza-title { font-weight: 600; font-size: 0.95rem; margin-bottom: 2px; }
.plaza-author { color: var(--text-light); font-size: 0.8rem; margin-bottom: 6px; }
.plaza-actions { display: flex; gap: 4px; }

[data-theme="dark"] .html-card,
[data-theme="dark"] .plaza-card { background: #1a1d27; }

/* ====== 移动端扩展应用管理适配 ====== */
@media (max-width: 640px) {
  .page-container {
    padding: 16px 12px;
  }
  .html-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .plaza-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .card-meta {
    flex-wrap: wrap;
    gap: 6px 12px;
  }
}
</style>
