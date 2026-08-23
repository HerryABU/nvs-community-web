<template>
  <div class="admin-page">
    <el-tabs v-model="tab">
      <!-- ── 博客分类管理 ── -->
      <el-tab-pane label="博客分类管理" name="cats">
        <div class="panel-head">
          <h2>博客分类（支持一级 / 二级，类似 CSDN）</h2>
          <el-button type="primary" @click="openCreate()">新增一级分类</el-button>
        </div>

        <div v-loading="loading">
          <div v-for="cat in tree" :key="cat.id" class="cat-row">
            <div class="cat-main">
              <span class="cat-name">{{ cat.name }}</span>
              <el-tag size="small" type="info">{{ cat.blog_count }} 篇</el-tag>
              <span class="cat-actions">
                <el-button size="small" text type="primary" @click="openCreate(cat.id)">+ 子分类</el-button>
                <el-button size="small" text type="primary" @click="openEdit(cat)">编辑</el-button>
                <el-button size="small" text type="danger" @click="removeCat(cat)">删除</el-button>
              </span>
            </div>
            <div v-if="cat.children && cat.children.length" class="cat-children">
              <template v-for="sub in cat.children" :key="sub.id">
                <div class="cat-sub">
                  <span class="cat-name">└ {{ sub.name }}</span>
                  <el-tag size="small" type="info">{{ sub.blog_count }} 篇</el-tag>
                  <span class="cat-actions">
                    <el-button size="small" text type="primary" @click="openCreate(sub.id)">+ 子分类</el-button>
                    <el-button size="small" text type="primary" @click="openEdit(sub)">编辑</el-button>
                    <el-button size="small" text type="danger" @click="removeCat(sub)">删除</el-button>
                  </span>
                </div>
                <!-- 三级分类 -->
                <div v-if="sub.children && sub.children.length" class="cat-children3">
                  <div v-for="g in sub.children" :key="g.id" class="cat-sub3">
                    <span class="cat-name">　└ {{ g.name }}</span>
                    <el-tag size="small" type="info">{{ g.blog_count }} 篇</el-tag>
                    <span class="cat-actions">
                      <el-button size="small" text type="primary" @click="openEdit(g)">编辑</el-button>
                      <el-button size="small" text type="danger" @click="removeCat(g)">删除</el-button>
                    </span>
                  </div>
                </div>
              </template>
            </div>
          </div>
          <el-empty v-if="!loading && tree.length === 0" description="暂无分类，点击「新增一级分类」创建" />
        </div>
      </el-tab-pane>

      <!-- ── 回收站监管（用户自删内容） ── -->
      <el-tab-pane label="回收站监管（用户自删）" name="recycle">
        <div class="panel-head">
          <h2>全站回收站：用户自己删除的小说 / 博客 / 章节 / 评论（管理员可查看并恢复）</h2>
          <el-select v-model="recycleType" size="small" style="width: 140px" @change="loadRecycle">
            <el-option label="全部类型" value="" />
            <el-option label="小说" value="novel" />
            <el-option label="博客" value="blog" />
            <el-option label="章节" value="chapter" />
            <el-option label="评论" value="comment" />
          </el-select>
        </div>
        <div v-loading="recycleLoading" style="min-height: 120px">
          <el-empty v-if="!recycleLoading && recycleItems.length === 0" description="暂无用户自删内容" />
          <div v-for="it in recycleItems" :key="it.type + it.id" class="recycle-item">
            <el-tag size="small" :type="typeTag(it.type)">{{ typeLabel(it.type) }}</el-tag>
            <span class="recycle-title">{{ it.title }}</span>
            <span class="recycle-meta">作者：{{ it.author_name || '#' + it.author_id }} · {{ formatDate(it.deleted_at) }}</span>
            <div class="recycle-actions">
              <el-button size="small" text type="success" @click="restoreRecycle(it)">恢复</el-button>
            </div>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 新增/编辑分类弹窗 -->
    <el-dialog v-model="showDialog" :title="editingId ? '编辑分类' : (parentId ? (parentLevel === 2 ? '新增三级分类' : '新增二级分类') : '新增一级分类')" width="420px">
      <el-form label-width="90px">
        <el-form-item label="分类名称" required>
          <el-input v-model="form.name" placeholder="分类名称（如：技术 / 随笔）" maxlength="64" />
        </el-form-item>
        <el-form-item v-if="parentId" label="上级分类">
          <el-input :model-value="parentName" disabled />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort_order" :min="0" :max="9999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveCat">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';

const tab = ref('cats');

// ===== 分类 =====
const tree = ref<any[]>([]);
const loading = ref(false);
const showDialog = ref(false);
const saving = ref(false);
const editingId = ref(0);
const parentId = ref(0);
const parentName = ref('');
const parentLevel = ref(1); // 父分类层级（1=一级 → 新增二级；2=二级 → 新增三级）
const form = reactive({ name: '', sort_order: 0 });

// 在树中查找节点（含层级信息）
function findCatInfo(id: number): { name: string; level: number } | null {
  for (const c of tree.value) {
    if (c.id === id) return { name: c.name, level: 1 };
    for (const s of c.children || []) {
      if (s.id === id) return { name: s.name, level: 2 };
      for (const g of s.children || []) {
        if (g.id === id) return { name: g.name, level: 3 };
      }
    }
  }
  return null;
}

async function loadTree() {
  loading.value = true;
  try {
    const res = await adminApi.getBlogCategories();
    if (res.data.code === 0) tree.value = res.data.data || [];
  } catch { /* ignore */ }
  finally { loading.value = false; }
}

function openCreate(parentID = 0) {
  editingId.value = 0;
  parentId.value = parentID;
  parentName.value = '';
  parentLevel.value = 1;
  if (parentID > 0) {
    const info = findCatInfo(parentID);
    parentName.value = info?.name || '';
    parentLevel.value = info?.level || 1;
  }
  form.name = '';
  form.sort_order = 0;
  showDialog.value = true;
}

function openEdit(cat: any) {
  editingId.value = cat.id;
  parentId.value = cat.parent_id || 0;
  parentName.value = '';
  parentLevel.value = 1;
  if (cat.parent_id > 0) {
    const info = findCatInfo(cat.parent_id);
    parentName.value = info?.name || '';
    parentLevel.value = info?.level || 1;
  }
  form.name = cat.name;
  form.sort_order = cat.sort_order || 0;
  showDialog.value = true;
}

async function saveCat() {
  if (!form.name.trim()) { ElMessage.warning('请填写分类名称'); return; }
  saving.value = true;
  try {
    if (editingId.value) {
      await adminApi.updateBlogCategory(editingId.value, { name: form.name.trim(), sort_order: form.sort_order });
      ElMessage.success('已更新');
    } else {
      await adminApi.createBlogCategory({ name: form.name.trim(), parent_id: parentId.value || undefined, sort_order: form.sort_order });
      ElMessage.success('已创建');
    }
    showDialog.value = false;
    loadTree();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally { saving.value = false; }
}

async function removeCat(cat: any) {
  try {
    await ElMessageBox.confirm(
      `确定删除分类「${cat.name}」？其下博客将回到「未分类」。`,
      '删除确认', { confirmButtonText: '删除', type: 'warning' },
    );
    await adminApi.deleteBlogCategory(cat.id);
    ElMessage.success('已删除');
    loadTree();
  } catch { /* 取消 */ }
}

// ===== 回收站监管 =====
const recycleItems = ref<any[]>([]);
const recycleLoading = ref(false);
const recycleType = ref('');

async function loadRecycle() {
  recycleLoading.value = true;
  try {
    const res = await adminApi.getAdminRecycle(recycleType.value || undefined);
    if (res.data.code === 0) recycleItems.value = res.data.data || [];
  } catch { /* ignore */ }
  finally { recycleLoading.value = false; }
}

function typeLabel(t: string) {
  return ({ novel: '小说', blog: '博客', chapter: '章节', comment: '评论' } as Record<string, string>)[t] || t;
}
function typeTag(t: string) {
  return ({ novel: 'danger', blog: 'warning', chapter: 'primary', comment: 'info' } as Record<string, any>)[t] || 'info';
}
function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleString('zh-CN'); }

async function restoreRecycle(it: any) {
  try {
    await ElMessageBox.confirm(`确定恢复这篇${typeLabel(it.type)}《${it.title}》？`, '恢复确认', {
      confirmButtonText: '恢复', type: 'warning',
    });
    await adminApi.restoreContent({ type: it.type, id: it.id });
    ElMessage.success('已恢复');
    loadRecycle();
  } catch { /* 取消 */ }
}

onMounted(() => {
  loadTree();
  loadRecycle();
});
</script>

<style scoped>
.admin-page { max-width: 1100px; margin: 0 auto; padding: 24px; }
.panel-head {
  display: flex; align-items: center; justify-content: space-between;
  gap: 12px; margin-bottom: 18px; flex-wrap: wrap;
}
.panel-head h2 { font-size: 1.1rem; margin: 0; }
.cat-row {
  border: 1px solid var(--border-color); border-radius: 10px;
  margin-bottom: 10px; overflow: hidden;
}
.cat-main, .cat-sub, .cat-sub3 {
  display: flex; align-items: center; gap: 10px;
  padding: 12px 16px;
}
.cat-main { background: var(--bg-color, #f8fafc); font-weight: 600; }
.cat-children { border-top: 1px dashed var(--border-color); }
.cat-sub { padding-left: 40px; }
.cat-children3 { border-top: 1px dashed var(--border-color); }
.cat-sub3 { padding-left: 72px; font-size: 0.92rem; }
.cat-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cat-actions { display: flex; gap: 2px; flex-shrink: 0; }

.recycle-item {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 14px; border: 1px solid var(--border-color);
  border-radius: 8px; margin-bottom: 8px; flex-wrap: wrap;
}
.recycle-title { flex: 1; min-width: 160px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.recycle-meta { font-size: 0.78rem; color: #999; white-space: nowrap; }
.recycle-actions { flex-shrink: 0; }
</style>
