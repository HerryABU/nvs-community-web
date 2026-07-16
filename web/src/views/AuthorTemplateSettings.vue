<template>
  <div class="page-container">
    <h2>🎨 作者模板设置</h2>
    <p class="subtitle">管理阅读页模板和作者主页模板 — 模板仅在您自己的作品/主页上生效</p>

    <el-tabs v-model="tab" type="border-card">
      <!-- 📖 阅读模板 -->
      <el-tab-pane label="📖 阅读模板" name="reader">
        <p class="tab-desc">这些模板会出现在您的<strong>小说阅读页面</strong>上（章节正文上下方）</p>
        <div v-if="readerFrames.length===0" class="empty"><el-empty description="还没有阅读模板"><el-button type="primary" @click="openCreate('reader')">创建阅读模板</el-button></el-empty></div>
        <div v-else class="frame-grid">
          <div v-for="f in readerFrames" :key="f.id" class="frame-card">
            <div class="card-hd"><span>{{ f.name }}</span><el-tag size="small" :type="f.is_active?'success':'info'">{{ f.is_active?'启用':'停用' }}</el-tag></div>
            <div class="card-bd"><p>{{ f.description||'无描述' }}</p><span class="meta">v{{f.version}} · {{fmt(f.updated_at)}}</span></div>
            <div class="card-ft">
              <el-button size="small" @click="preview(f)">预览</el-button>
              <el-button size="small" @click="edit(f)">编辑</el-button>
              <el-button size="small" type="danger" @click="del(f)">删除</el-button>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <!-- 👤 作者展现模板 -->
      <el-tab-pane label="👤 作者展现" name="author">
        <p class="tab-desc">这些模板会出现在您的<strong>作者主页</strong>上（/author/:id）</p>
        <div v-if="authorFrames.length===0" class="empty"><el-empty description="还没有作者展现模板"><el-button type="primary" @click="openCreate('author')">创建作者展现模板</el-button></el-empty></div>
        <div v-else class="frame-grid">
          <div v-for="f in authorFrames" :key="f.id" class="frame-card">
            <div class="card-hd"><span>{{ f.name }}</span><el-tag size="small" :type="f.is_active?'success':'info'">{{ f.is_active?'启用':'停用' }}</el-tag></div>
            <div class="card-bd"><p>{{ f.description||'无描述' }}</p><span class="meta">v{{f.version}} · {{fmt(f.updated_at)}}</span></div>
            <div class="card-ft">
              <el-button size="small" @click="preview(f)">预览</el-button>
              <el-button size="small" @click="edit(f)">编辑</el-button>
              <el-button size="small" type="danger" @click="del(f)">删除</el-button>
            </div>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 编辑弹窗 -->
    <el-dialog v-model="showEdit" :title="editing?'编辑模板':'新建模板'" width="80%" destroy-on-close @closed="resetForm">
      <el-form :model="form" label-position="top">
        <el-row :gutter="16">
          <el-col :span="16"><el-form-item label="名称"><el-input v-model="form.name" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="关联作品"><el-input-number v-model="form.novel_id" :min="0" style="width:100%" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="HTML内容"><el-input v-model="form.content" type="textarea" :rows="16" class="code-input" /></el-form-item>
        <el-space>
          <el-checkbox v-model="form.has_controls">交互控件</el-checkbox>
          <el-checkbox v-model="form.uses_novel_api">平台API</el-checkbox>
          <el-checkbox v-model="form.is_public">公开分享</el-checkbox>
        </el-space>
      </el-form>
      <template #footer>
        <el-button @click="showEdit=false">取消</el-button>
        <el-button type="primary" @click="save" :loading="saving">{{editing?'更新':'创建'}}</el-button>
      </template>
    </el-dialog>

    <!-- 预览 -->
    <el-dialog v-model="showPreview" title="预览" width="85%"><SandboxPreview v-if="previewSrc" :src="previewSrc" height="500px" sandbox-policy="allow-scripts allow-same-origin"/></el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { frameApi } from '@/api/frame';
import SandboxPreview from '@/views/frame/SandboxPreview.vue';

const tab = ref('reader');
const readerFrames = ref<any[]>([]);
const authorFrames = ref<any[]>([]);
const showEdit = ref(false); const showPreview = ref(false); const saving = ref(false);
const editing = ref<any>(null); const previewSrc = ref('');
const form = ref({ name:'', novel_id:undefined as number|undefined, content:'', has_controls:false, uses_novel_api:false, is_public:false, sandbox_level:'strict', frame_type:'reader' });

function resetForm(){ form.value={ name:'', novel_id:undefined, content:'', has_controls:false, uses_novel_api:false, is_public:false, sandbox_level:'strict', frame_type:'reader' }; editing.value=null; }
function fmt(d:string){return new Date(d).toLocaleDateString('zh-CN')}

onMounted(load);

async function load(){
  try{
    const r=await frameApi.list();
    if(r.data.code===0){
      const all=r.data.data||[];
      readerFrames.value=all.filter((f:any)=>f.frame_type!=='author');
      authorFrames.value=all.filter((f:any)=>f.frame_type==='author');
    }
  }catch{ElMessage.error('加载失败')}
}

function openCreate(type:string){ form.value.frame_type=type; showEdit.value=true; }
function edit(f:any){ editing.value=f; form.value={name:f.name,novel_id:f.novel_id??undefined,content:'',has_controls:f.has_controls,uses_novel_api:f.uses_novel_api,is_public:f.is_public,sandbox_level:f.sandbox_level,frame_type:f.frame_type}; showEdit.value=true; frameApi.get(f.id).then(r=>{if(r.data.code===0)form.value.content=r.data.data.content}) }

async function save(){
  if(!form.value.name||!form.value.content){ElMessage.warning('请填写完整');return}
  saving.value=true;
  try{
    const data:any={...form.value,novel_id:form.value.novel_id as number};
    if(editing.value){await frameApi.update(editing.value.id,data);ElMessage.success('已更新')}
    else {await frameApi.create(data);ElMessage.success('已创建')}
    showEdit.value=false;resetForm();load();
  }catch{ElMessage.error('保存失败')}
  saving.value=false;
}
function preview(f:any){previewSrc.value=frameApi.getPreview(f.id);showPreview.value=true}
async function del(f:any){
  try{await ElMessageBox.confirm('删除？','确认',{type:'warning'});await frameApi.delete(f.id);ElMessage.success('已删除');load()}catch{}
}
</script>

<style scoped>
.page-container{max-width:1000px;margin:0 auto;padding:24px 20px}
.page-container h2{margin:0 0 4px;font-size:1.5rem}
.subtitle{color:var(--text-secondary);margin-bottom:20px}
.tab-desc{color:var(--text-secondary);margin-bottom:16px}
.frame-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:14px}
.frame-card{background:var(--card-bg);border:1px solid var(--border-color);border-radius:10px;padding:14px;display:flex;flex-direction:column;gap:8px}
.card-hd{display:flex;justify-content:space-between;align-items:center;font-weight:600}
.card-bd{flex:1}
.card-bd p{color:var(--text-secondary);font-size:.88rem;margin:0}
.meta{font-size:.78rem;color:var(--text-secondary)}
.card-ft{display:flex;gap:6px;justify-content:flex-end}
.empty{padding:40px 0;text-align:center}
.code-input :deep(textarea){font-family:monospace;font-size:.85rem}
</style>
