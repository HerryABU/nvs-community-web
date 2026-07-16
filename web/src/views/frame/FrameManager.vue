<template>
  <div class="page-container">
    <div class="page-header">
      <div class="header-left">
        <h2>🎨 自定义UI模板</h2>
        <p class="subtitle">为小说阅读页面创建个性化渲染模板。支持调用平台小说API、沙盒内按钮控件。</p>
      </div>
      <el-button type="primary" @click="showCreate = true" :icon="Plus">新建模板</el-button>
    </div>

    <div v-if="frames.length === 0 && !loading" class="empty-state">
      <el-empty description="还没有自定义模板">
        <el-button type="primary" @click="showCreate = true" :icon="Plus">创建第一个模板</el-button>
      </el-empty>
    </div>

    <div v-else class="frame-grid">
      <div v-for="f in frames" :key="f.id" class="frame-card" :class="{ inactive: !f.is_active }">
        <div class="card-header">
          <span class="card-name">{{ f.name }}</span>
          <div class="card-badges">
            <el-tag v-if="f.uses_novel_api" type="success" size="small">API</el-tag>
            <el-tag v-if="f.has_controls" type="warning" size="small">控件</el-tag>
            <el-tag :type="f.sandbox_level === 'interactive' ? 'primary' : 'info'" size="small">{{ f.sandbox_level === 'interactive' ? '交互' : '只读' }}</el-tag>
            <el-tag v-if="f.is_public" type="success" size="small">公开</el-tag>
            <el-tag v-if="!f.is_active" type="info" size="small">禁用</el-tag>
          </div>
        </div>
        <div class="card-body">
          <p class="card-desc">{{ f.description || '暂无描述' }}</p>
          <div class="card-meta"><span>版本 {{ f.version }}</span><span>{{ fmtDate(f.updated_at) }}</span></div>
        </div>
        <div class="card-actions">
          <el-button size="small" @click="preview(f)" :icon="View">预览</el-button>
          <el-button size="small" @click="edit(f)" :icon="Edit">编辑</el-button>
          <el-button size="small" type="danger" @click="del(f)" :icon="Delete">删除</el-button>
        </div>
      </div>
    </div>

    <!-- 预览 -->
    <el-dialog v-model="showPreview" title="模板预览" width="85%" destroy-on-close>
      <SandboxPreview v-if="previewSrc" :src="previewSrc" :title="previewName" height="550px"
        :sandbox-policy="previewPolicy" />
    </el-dialog>

    <!-- 编辑 -->
    <el-dialog v-model="showCreate" :title="editing ? '编辑模板' : '新建模板'" width="80%" destroy-on-close @closed="resetForm">
      <el-form :model="form" label-position="top">
        <el-row :gutter="16">
          <el-col :span="16"><el-form-item label="名称" required><el-input v-model="form.name" maxlength="128" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="关联作品"><el-input-number v-model="form.novel_id" :min="0" style="width:100%" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="2" maxlength="512" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="应用场景">
              <el-radio-group v-model="form.frame_type">
                <el-radio value="reader">📖 阅读模版</el-radio>
                <el-radio value="author">👤 作者展现</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="交互级别">
              <el-radio-group v-model="form.sandbox_level">
                <el-radio value="strict">只读</el-radio>
                <el-radio value="interactive">交互</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="16" style="display:flex;align-items:flex-end;padding-bottom:18px;gap:16px">
            <el-checkbox v-model="form.uses_novel_api">调用平台小说API</el-checkbox>
            <el-checkbox v-model="form.has_controls">包含按钮/控件</el-checkbox>
            <el-checkbox v-model="form.is_public">公开分享</el-checkbox>
          </el-col>
        </el-row>
        <el-alert v-if="form.uses_novel_api" type="info" :closable="false" show-icon style="margin-bottom:12px">
          <template #title>平台API可用变量</template>
          <code>window.NVS.getNovelData(novelId)</code> 获取小说数据 ·
          <code>window.NVS.getCurrentChapter()</code> 当前章节 ·
          <code>window.NVS.sendMessage(type, data)</code> 向父页面通信
        </el-alert>
        <el-form-item label="HTML 内容" required>
          <el-input v-model="form.content" type="textarea" :rows="16" placeholder="<div class='novel-page'>...</div>" class="code-input" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" @click="save" :loading="saving">{{ editing ? '更新' : '创建' }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus, View, Edit, Delete } from '@element-plus/icons-vue';
import { frameApi } from '@/api/frame';
import SandboxPreview from './SandboxPreview.vue';

interface Frame {
  id: number; name: string; description: string; novel_id: number | null;
  is_active: boolean; is_public: boolean; version: number; updated_at: string;
  has_controls: boolean; uses_novel_api: boolean; sandbox_level: string; frame_type: string;
}

const frames = ref<Frame[]>([]);
const loading = ref(false); const saving = ref(false);
const showCreate = ref(false); const showPreview = ref(false);
const previewSrc = ref(''); const previewName = ref(''); const previewPolicy = ref('allow-scripts allow-same-origin');
const editing = ref<Frame | null>(null);

const form = ref({ name: '', description: '', novel_id: undefined as number | undefined, content: '', is_public: false, has_controls: false, uses_novel_api: false, sandbox_level: 'strict', frame_type: 'reader' });

function resetForm() {
  form.value = { name: '', description: '', novel_id: undefined, content: '', is_public: false, has_controls: false, uses_novel_api: false, sandbox_level: 'strict', frame_type: 'reader' };
  editing.value = null;
}

onMounted(() => load());

async function load() {
  loading.value = true;
  try { const r = await frameApi.list(); if (r.data.code === 0) frames.value = r.data.data; } catch { ElMessage.error('加载失败'); }
  finally { loading.value = false; }
}

function edit(f: Frame) {
  editing.value = f;
  form.value = { name: f.name, description: f.description, novel_id: f.novel_id ?? undefined, content: '', is_public: f.is_public, has_controls: f.has_controls, uses_novel_api: f.uses_novel_api, sandbox_level: f.sandbox_level, frame_type: f.frame_type || 'reader' };
  showCreate.value = true;
  frameApi.get(f.id).then(r => { if (r.data.code === 0) form.value.content = r.data.data.content; });
}

async function save() {
  if (!form.value.name.trim() || !form.value.content.trim()) { ElMessage.warning('请填写完整'); return; }
  saving.value = true;
  try {
    if (editing.value) {
      await frameApi.update(editing.value.id, { ...form.value });
      ElMessage.success('已更新');
    } else {
      await frameApi.create({ ...form.value, novel_id: form.value.novel_id as number });
      ElMessage.success('已创建');
    }
    showCreate.value = false; resetForm(); await load();
  } catch { ElMessage.error('保存失败'); }
  finally { saving.value = false; }
}

function preview(f: Frame) {
  previewSrc.value = frameApi.getPreview(f.id);
  previewName.value = f.name;
  previewPolicy.value = f.has_controls ? 'allow-scripts allow-same-origin allow-forms allow-popups' : 'allow-scripts allow-same-origin';
  showPreview.value = true;
}

async function del(f: Frame) {
  try {
    await ElMessageBox.confirm(`删除「${f.name}」？`, '确认', { type: 'warning', confirmButtonText: '删除' });
    await frameApi.delete(f.id); ElMessage.success('已删除'); await load();
  } catch { /* cancelled */ }
}

function fmtDate(d: string) { return new Date(d).toLocaleDateString('zh-CN'); }
</script>

<style scoped>
.page-container { max-width: 1100px; margin: 0 auto; padding: 24px 20px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; flex-wrap: wrap; gap: 12px; }
.header-left h2 { margin: 0; font-size: 1.4rem; }
.subtitle { color: var(--text-secondary); margin: 4px 0 0; font-size: 0.9rem; }
.frame-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 16px; }
.frame-card { background: var(--card-bg); border: 1px solid var(--border-color); border-radius: 10px; padding: 16px; display: flex; flex-direction: column; gap: 10px; transition: box-shadow 0.2s; }
.frame-card:hover { box-shadow: 0 2px 12px rgba(0,0,0,0.08); }
.frame-card.inactive { opacity: 0.6; }
.card-header { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.card-name { font-weight: 600; font-size: 1.05rem; flex: 1; }
.card-badges { display: flex; gap: 4px; }
.card-body { flex: 1; }
.card-desc { color: var(--text-secondary); font-size: 0.88rem; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.card-meta { display: flex; gap: 16px; margin-top: 6px; font-size: 0.8rem; color: var(--text-secondary); }
.card-actions { display: flex; gap: 6px; justify-content: flex-end; }
.empty-state { padding: 40px 0; display: flex; justify-content: center; }
.code-input :deep(textarea) { font-family: 'SF Mono', 'Fira Code', monospace; font-size: 0.85rem; line-height: 1.5; }
</style>
