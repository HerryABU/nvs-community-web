<template>
  <div class="admin-page">
    <AdminHeader title="公告栏" subtitle="站点公告管理 · 合规通知发布" :icon="Bell" />
    <div class="admin-page-body admin-panel">
      <div class="toolbar">
        <el-button type="primary" @click="openCreate">+ 发布公告</el-button>
      </div>
      <el-table :data="list" border stripe size="small">
        <el-table-column label="置顶" width="70">
          <template #default="{ row }">
            <el-tag v-if="row.pinned" type="danger" size="small">置顶</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip />
        <el-table-column prop="content" label="内容" min-width="260" show-overflow-tooltip />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.published ? 'success' : 'info'" size="small">{{ row.published ? '已发布' : '草稿' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="发布时间" width="150">
          <template #default="{ row }">{{ (row.created_at || '').slice(0, 16).replace('T', ' ') }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-popconfirm title="确认删除该公告？" @confirm="remove(row.id)">
              <template #reference><el-button size="small" type="danger" plain>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <el-dialog v-model="dlg.visible" :title="dlg.id ? '编辑公告' : '发布公告'" width="600px">
        <el-form label-width="80px">
          <el-form-item label="标题" required><el-input v-model="dlg.form.title" maxlength="256" /></el-form-item>
          <el-form-item label="内容"><el-input v-model="dlg.form.content" type="textarea" :rows="6" placeholder="支持 Markdown 格式" /></el-form-item>
          <el-form-item label="置顶"><el-switch v-model="dlg.form.pinned" /></el-form-item>
          <el-form-item label="发布">
            <el-switch v-model="dlg.form.published" active-text="立即发布" inactive-text="存为草稿" />
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
import { ref, reactive, onMounted } from 'vue';
import { Bell } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const list = ref<any[]>([]);
const dlg = reactive({ visible: false, id: 0, saving: false, form: {} as any });

async function load() {
  try {
    const res = await adminApi.getAnnouncements();
    list.value = res.data.data || [];
  } catch { /* ignore */ }
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
async function saveDlg() {
  if (!dlg.form.title) { ElMessage.warning('请填写公告标题'); return; }
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
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-body { max-width: 1000px; }
.toolbar { margin-bottom: 12px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
</style>
