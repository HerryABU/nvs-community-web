<template>
  <div class="admin-page">
    <AdminHeader title="公告栏" subtitle="站点公告管理 · 合规通知发布" :icon="Bell" />
    <div class="admin-page-body admin-panel">
      <div class="admin-toolbar">
        <el-button type="primary" @click="openCreate">+ 发布公告</el-button>
        <el-radio-group v-model="statusFilter" size="small">
          <el-radio-button label="all">全部</el-radio-button>
          <el-radio-button label="published">已发布</el-radio-button>
          <el-radio-button label="draft">草稿</el-radio-button>
          <el-radio-button label="pinned">置顶</el-radio-button>
        </el-radio-group>
      </div>
      <el-table :data="filteredList" border stripe size="small" v-loading="loading">
        <el-table-column label="置顶" width="70">
          <template #default="{ row }">
            <el-tag v-if="row.pinned" type="danger" size="small">置顶</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.published ? 'success' : 'info'" size="small">{{ row.published ? '已发布' : '草稿' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="160">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-popconfirm title="删除后不可恢复，确认删除该公告？" @confirm="remove(row.id)">
              <template #reference><el-button size="small" type="danger" plain>删除</el-button></template>
            </el-popconfirm>
          </template>
          </el-table-column>
        <template #empty>
          <EmptyState icon="📭" text="暂无公告" description="点击「发布公告」创建第一条公告" />
        </template>
      </el-table>

      <el-dialog v-model="dlg.visible" :title="dlg.id ? '编辑公告' : '发布公告'" width="640px" @closed="onDlgClosed">
        <el-form label-width="72px">
          <el-form-item label="标题" required>
            <el-input v-model="dlg.form.title" maxlength="255" show-word-limit placeholder="公告标题（最多 255 字）" />
          </el-form-item>
          <el-form-item label="内容">
            <el-input
              v-model="dlg.form.content"
              type="textarea"
              :rows="8"
              placeholder="支持 Markdown 格式（标题 / 列表 / 代码块 / 表格等）"
            />
          </el-form-item>
          <el-form-item label="预览">
            <el-collapse class="ann-preview-collapse">
              <el-collapse-item title="展开 Markdown 实时预览">
                <div class="ann-preview cherry-markdown cherry-previewer">
                  <div v-html="previewHtml"></div>
                </div>
              </el-collapse-item>
            </el-collapse>
          </el-form-item>
          <el-form-item label="置顶">
            <el-switch v-model="dlg.form.pinned" />
            <span class="form-hint">置顶后前台公告栏优先展示</span>
          </el-form-item>
          <el-form-item label="发布">
            <el-switch v-model="dlg.form.published" active-text="立即发布" inactive-text="存为草稿" />
            <span class="form-hint">草稿不在前台展示</span>
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="dlg.visible = false">取消</el-button>
          <el-button type="primary" :loading="dlg.saving" @click="saveDlg">保存</el-button>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { Bell } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { renderMarkdown } from '@/markdown/renderer';
import { ElMessage } from 'element-plus';
import StateBlock from '@/components/common/StateBlock.vue';

// 列表按「置顶优先 + 创建时间倒序」展示（后端 AdminListAnnouncements 已同序，前端再保险）
const list = ref<any[]>([]);
const loading = ref(false);
// 状态下筛选（前端过滤，不改变后端返回的全量数据）
const statusFilter = ref<'all' | 'published' | 'draft' | 'pinned'>('all');

const filteredList = computed(() => {
  if (statusFilter.value === 'published') return list.value.filter((a) => a.published);
  if (statusFilter.value === 'draft') return list.value.filter((a) => !a.published);
  if (statusFilter.value === 'pinned') return list.value.filter((a) => a.pinned);
  return list.value;
});

function formatTime(d?: string) {
  return (d || '').slice(0, 16).replace('T', ' ');
}

const dlg = reactive({ visible: false, id: 0, saving: false, form: {} as any });

// 编辑对话框内的 Markdown 实时预览（与前台公告详情同款 cherry 渲染）
const previewHtml = computed(() => renderMarkdown(dlg.form.content || ''));

async function load() {
  loading.value = true;
  try {
    const res = await adminApi.getAnnouncements();
    list.value = res.data.data || [];
  } catch { /* ignore */ } finally {
    loading.value = false;
  }
}
function openCreate() {
  dlg.id = 0;
  dlg.form = { title: '', content: '', pinned: false, published: true };
  dlg.visible = true;
}
function openEdit(row: any) {
  dlg.id = row.id;
  dlg.form = { title: row.title, content: row.content, pinned: row.pinned, published: row.published };
  dlg.visible = true;
}
// 对话框关闭后清理表单，避免下次打开时闪现上一条内容
function onDlgClosed() {
  dlg.form = { title: '', content: '', pinned: false, published: true };
}
async function saveDlg() {
  if (!dlg.form.title || !dlg.form.title.trim()) { ElMessage.warning('请填写公告标题'); return; }
  dlg.saving = true;
  try {
    if (dlg.id) await adminApi.updateAnnouncement(dlg.id, dlg.form);
    else await adminApi.createAnnouncement(dlg.form);
    ElMessage.success('已保存');
    dlg.visible = false;
    load();
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '保存失败'); }
  finally { dlg.saving = false; }
}
async function remove(id: number) {
  try {
    await adminApi.deleteAnnouncement(id);
    ElMessage.success('已删除');
    load();
  } catch { ElMessage.error('删除失败'); }
}
onMounted(load);
</script>

<style scoped>
.admin-page { --admin-content-max: 1000px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: 1000px; }
.form-hint { margin-left: 10px; font-size: 0.78rem; color: var(--text-light, #999); }
/* 实时预览区：限高滚动，避免长公告把对话框撑爆 */
.ann-preview { max-height: 260px; overflow-y: auto; padding: 8px 12px; border: 1px dashed var(--el-border-color); border-radius: 6px; }
.ann-preview-collapse { width: 100%; }
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
