<template>
  <div class="page-container">
    <SectionHeader title="自定义 UI 模板" subtitle="为小说阅读页面创建个性化渲染模板。支持调用平台小说 API、沙盒内按钮控件。" :icon="'🎨'" gradient="primary">
      <template #actions>
        <el-button type="primary" @click="showCreate = true" :icon="Plus">新建模板</el-button>
      </template>
    </SectionHeader>

    <EmptyState v-if="frames.length === 0 && !loading" icon="📭" text="还没有自定义模板" description="为小说阅读页创建个性化渲染模板，支持调用平台 API 与沙盒按钮控件。">
      <template #actions>
        <el-button type="primary" @click="showCreate = true" :icon="Plus">创建第一个模板</el-button>
      </template>
    </EmptyState>

    <div v-else class="frame-grid">
      <div v-for="f in frames" :key="f.id" class="frame-card" :class="{ inactive: !f.is_active }">
        <div class="card-header">
          <span class="card-name">{{ f.name }}</span>
          <div class="card-badges">
            <el-tag v-if="f.uses_novel_api" type="success" size="small">API</el-tag>
            <el-tag v-if="f.has_controls" type="warning" size="small">控件</el-tag>
            <el-tag :type="f.sandbox_level === 'interactive' ? 'primary' : 'info'" size="small">{{ f.sandbox_level === 'interactive' ? '交互' : '只读' }}</el-tag>
            <el-tag v-if="f.run_mode === 'inline'" type="danger" size="small">非沙盒</el-tag>
            <el-tag v-if="f.review_status === 'pending'" type="warning" size="small">待复核</el-tag>
            <el-tag v-else-if="f.review_status === 'rejected'" type="danger" size="small">已驳回</el-tag>
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
          <el-col :span="8">
            <el-form-item label="运行模式">
              <el-radio-group v-model="form.run_mode">
                <el-radio value="sandbox">沙盒</el-radio>
                <el-radio value="inline">非沙盒</el-radio>
              </el-radio-group>
              <div class="run-mode-hint">非沙盒模式仅在您本人的页面直接注入 HTML</div>
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
        <el-alert type="warning" :closable="false" show-icon style="margin-bottom:12px">
          <template #title>推荐编译后的静态产物</template>
          请使用 vite/webpack 等构建产物（dist 目录内的纯 HTML/CSS/JS）；<strong>不推荐</strong> .ts/.vue/.jsx 等需要 Node.js 运行时或构建步骤的源文件
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
  run_mode: string; review_status: string;
}

const frames = ref<Frame[]>([]);
const loading = ref(false); const saving = ref(false);
const showCreate = ref(false); const showPreview = ref(false);
const previewSrc = ref(''); const previewName = ref(''); const previewPolicy = ref('allow-scripts');
const editing = ref<Frame | null>(null);

const form = ref({ name: '', description: '', novel_id: undefined as number | undefined, content: '', is_public: false, has_controls: false, uses_novel_api: false, sandbox_level: 'strict', frame_type: 'reader', run_mode: 'sandbox' });

function resetForm() {
  form.value = { name: '', description: '', novel_id: undefined, content: '', is_public: false, has_controls: false, uses_novel_api: false, sandbox_level: 'strict', frame_type: 'reader', run_mode: 'sandbox' };
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
  form.value = { name: f.name, description: f.description, novel_id: f.novel_id ?? undefined, content: '', is_public: f.is_public, has_controls: f.has_controls, uses_novel_api: f.uses_novel_api, sandbox_level: f.sandbox_level, frame_type: f.frame_type || 'reader', run_mode: f.run_mode || 'sandbox' };
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
  previewPolicy.value = f.has_controls ? 'allow-scripts allow-forms allow-popups' : 'allow-scripts';
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
.frame-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 16px; }
.frame-card { background: #fff; border: 1px solid var(--border-color); border-radius: var(--radius-md); padding: 16px; display: flex; flex-direction: column; gap: 10px; transition: box-shadow var(--transition-base); }
.frame-card:hover { box-shadow: var(--shadow-md); }
.frame-card.inactive { opacity: 0.6; }
.card-header { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.card-name { font-weight: 600; font-size: 1.05rem; flex: 1; }
.card-badges { display: flex; gap: 4px; }
.card-body { flex: 1; }
.card-desc { color: var(--text-light); font-size: 0.88rem; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.card-meta { display: flex; gap: 16px; margin-top: 6px; font-size: 0.8rem; color: var(--text-light); }
.card-actions { display: flex; gap: 6px; justify-content: flex-end; flex-wrap: wrap; }
.code-input :deep(textarea) { font-family: 'SF Mono', 'Fira Code', monospace; font-size: 0.85rem; line-height: 1.5; }
.run-mode-hint { font-size: 0.75rem; color: var(--text-light); margin-top: 4px; line-height: 1.4; }

[data-theme="dark"] .frame-card { background: #1a1d27; }

/* ====== 移动端模板管理适配 ====== */
@media (max-width: 640px) {
  .page-container {
    padding: 16px 12px;
  }
  .frame-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .card-meta {
    flex-wrap: wrap;
    gap: 6px 12px;
  }
  .card-actions {
    flex-wrap: wrap;
  }
}
</style>
