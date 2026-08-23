<template>
  <div class="blog-manager" v-loading="loading">
    <!-- 非本人访问 → 重定向公开主页 -->
    <template v-if="owner">
      <div class="bm-header">
        <h1>📝 我的博客管理</h1>
        <div class="bm-actions">
          <el-button type="primary" @click="$router.push('/author/blog/new')">✍️ 写博客</el-button>
          <el-button @click="$router.push(`/blog/${owner.username}`)">查看公开主页</el-button>
        </div>
      </div>

      <el-tabs v-model="activeTab" class="bm-tabs">
        <!-- ── Tab1：博客列表 ── -->
        <el-tab-pane label="我的博客" name="blogs">
          <div class="bm-toolbar">
            <el-input v-model="search" placeholder="搜索标题…" clearable style="width: 220px" size="small" @keyup.enter="loadBlogs" />
            <el-select v-model="filterCat" placeholder="全部分类" clearable size="small" style="width: 160px" @change="loadBlogs">
              <el-option v-for="c in flatCats" :key="c.id" :label="c.label" :value="c.id" />
            </el-select>
            <span class="bm-count">共 {{ total }} 篇</span>
            <span class="bm-toolbar-spacer" />
            <!-- 批量导入到分类 -->
            <template v-if="selectedIds.length > 0">
              <el-dropdown @command="batchSetCategory">
                <el-button type="primary" size="small">
                  移动 {{ selectedIds.length }} 篇到分类<el-icon style="margin-left:2px"><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item :command="0">未分类</el-dropdown-item>
                    <el-dropdown-item v-for="c in flatCats" :key="c.id" :command="c.id">{{ c.label }}</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
              <el-button size="small" @click="selectedIds = []">取消选择</el-button>
            </template>
            <el-button v-else size="small" text @click="batchMode = !batchMode">{{ batchMode ? '退出批量' : '批量分类' }}</el-button>
          </div>

          <div class="bm-list" v-loading="listLoading">
            <el-empty v-if="!listLoading && blogs.length === 0" description="还没有博客，点击「写博客」开始创作" />
            <div v-for="blog in blogs" :key="blog.id" class="bm-item">
              <el-checkbox v-if="batchMode || selectedIds.length > 0" v-model="selectedIds" :value="blog.id" size="small" @click.stop />
              <div class="bm-item-main">
                <div class="bm-item-tags">
                  <el-tag v-if="blog.is_pinned" size="small" type="danger">置顶</el-tag>
                  <el-tag v-if="blog.category" size="small">{{ blog.category }}</el-tag>
                  <span class="bm-item-meta">{{ formatDate(blog.created_at) }} · {{ blog.view_count }} 阅读<template v-if="blog.like_count"> · 👍 {{ blog.like_count }}</template></span>
                </div>
                <div class="bm-item-title" @click="$router.push(`/author/${owner.username}/blogs/${blog.id}`)">{{ blog.title }}</div>
              </div>
              <div class="bm-item-actions">
                <!-- 导入/设置分类：把已有博客归入某个分类 -->
                <el-dropdown @command="(catId: number) => setBlogCategory(blog, catId)">
                  <el-button size="small" text type="primary">设分类<el-icon style="margin-left:2px"><ArrowDown /></el-icon></el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item :command="0" :disabled="!blog.category_id">未分类</el-dropdown-item>
                      <el-dropdown-item v-for="c in flatCats" :key="c.id" :command="c.id" :disabled="c.id === blog.category_id">{{ c.label }}</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
                <el-button size="small" text type="primary" @click="$router.push(`/author/blog/edit/${blog.id}`)">编辑</el-button>
                <el-button size="small" text type="warning" @click="togglePin(blog)">{{ blog.is_pinned ? '取消置顶' : '置顶' }}</el-button>
                <el-button size="small" text type="danger" @click="removeBlog(blog)">删除</el-button>
              </div>
            </div>
          </div>

          <div class="pagination" v-if="total > pageSize">
            <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" @current-change="loadBlogs" />
          </div>
        </el-tab-pane>

        <!-- ── Tab2：分类管理（普通作者也可管理：新增/编辑/删除/拖拽挂靠/排序） ── -->
        <el-tab-pane label="分类管理" name="cats">
          <div class="bm-cat-toolbar">
            <span class="bm-cat-hint">💡 最多三级；新增分类需管理员<strong>事后审核</strong>，通过后才会公开显示。</span>
            <el-button type="primary" size="small" @click="openCatCreate(0)">+ 新增一级分类</el-button>
          </div>

          <div class="bm-cat-tree" v-loading="catLoading">
            <el-empty v-if="!catLoading && myCats.length === 0" description="暂无分类，点击「新增一级分类」创建" />

            <!-- 我的分类（个人专栏：新增/编辑/删除/拖拽挂靠/排序/进入批量迁移博客） -->
            <template v-if="myCats.length > 0">
              <div class="bm-cat-section">我的分类（{{ myCats.length }}）· 个人专栏</div>
              <div v-for="c in myCats" :key="c.id" class="bm-cat-node bm-cat-l1 bm-cat-mine"
                   :class="{ 'drag-over': dragOverId === c.id }"
                   draggable="true"
                   @dragstart="onDragStart(c.id)"
                   @dragover.prevent="dragOverId = c.id"
                   @dragleave="dragOverId = null"
                   @drop.prevent="onDrop(c.id)">
                <span class="bm-cat-name">{{ c.full_name || c.name }}</span>
                <el-tag size="small" :type="catStatusTag(c.status)">{{ catStatusLabel(c.status) }}</el-tag>
                <span v-if="c.status === 'rejected' && c.reject_reason" class="bm-cat-reason">原因：{{ c.reject_reason }}</span>
                <span class="bm-cat-actions">
                  <el-button size="small" text type="primary" @click="openCatBlogs(c)">📂 博客</el-button>
                  <el-button size="small" text type="primary" @click="openCatEdit(c)">编辑/挂靠</el-button>
                  <el-button size="small" text type="danger" @click="removeCat(c)">删除</el-button>
                </span>
              </div>
            </template>
          </div>
        </el-tab-pane>
      </el-tabs>

      <!-- 新增/编辑分类弹窗（挂靠父分类 + 排序） -->
      <el-dialog v-model="catDialog" :title="catEditId ? '编辑分类 / 设置挂靠' : (catParentId ? '新增子分类' : '新增一级分类')" width="440px">
        <el-form label-width="90px">
          <el-form-item label="分类名称" required>
            <el-input v-model="catForm.name" placeholder="分类名称" maxlength="64" />
          </el-form-item>
          <el-form-item label="挂靠父分类">
            <el-select v-model="catForm.parent_id" placeholder="不选=一级分类" clearable style="width: 100%">
              <el-option :value="0" label="（无，作为一级分类）" />
              <el-option v-for="c in flatCats" :key="c.id" :label="c.label" :value="c.id" />
            </el-select>
            <div class="bm-cat-form-hint">一级挂无，二级挂一级，三级挂二级（最多三级）</div>
          </el-form-item>
          <el-form-item label="排序">
            <el-input-number v-model="catForm.sort_order" :min="0" :max="9999" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="catDialog = false">取消</el-button>
          <el-button type="primary" :loading="catSaving" @click="saveCat">保存</el-button>
        </template>
      </el-dialog>

      <!-- 分类博客弹窗：查看该分类博客 + 批量迁移到其他分类 -->
      <el-dialog v-model="catBlogDialog" :title="`📂 分类《${catBlogCat?.full_name || catBlogCat?.name || ''}》的博客`" width="640px">
        <div class="cb-toolbar">
          <span class="cb-count">共 {{ catBlogTotal }} 篇</span>
          <el-select v-model="catBlogTarget" placeholder="迁移到分类…" clearable size="small" style="width: 200px">
            <el-option :value="0" label="（未分类）" />
            <el-option v-for="c in flatCats" :key="c.id" :label="c.label" :value="c.id" :disabled="c.id === catBlogCat?.id" />
          </el-select>
          <el-button type="primary" size="small" :disabled="selectedCatBlogs.length === 0 || catBlogTarget === undefined" @click="migrateCatBlogs">
            迁移 {{ selectedCatBlogs.length }} 篇
          </el-button>
          <el-button size="small" @click="selectedCatBlogs = []">清空选择</el-button>
        </div>
        <div class="cb-list" v-loading="catBlogLoading" style="max-height: 380px; overflow-y: auto">
          <el-empty v-if="!catBlogLoading && catBlogs.length === 0" description="该分类下暂无博客" />
          <div v-for="b in catBlogs" :key="b.id" class="cb-item">
            <el-checkbox v-model="selectedCatBlogs" :value="b.id" size="small" @click.stop />
            <div class="cb-item-main">
              <div class="cb-item-title">{{ b.title }}</div>
              <div class="cb-item-meta">
                <el-tag v-if="b.is_pinned" size="small" type="danger">置顶</el-tag>
                <span v-for="t in b.tags || []" :key="t.id" class="cb-tag">#{{ t.name }}</span>
                <span class="cb-date">{{ formatDate(b.created_at) }}</span>
              </div>
            </div>
          </div>
        </div>
        <template #footer>
          <el-button @click="catBlogDialog = false">关闭</el-button>
        </template>
      </el-dialog>
    </template>
    <el-empty v-else-if="!loading" description="无权访问该博客管理页" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { blogApi } from '@/api/social';
import { resolveUserKey } from '@/utils/userKey';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ArrowDown } from '@element-plus/icons-vue';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const key = ref(String(route.params.key ?? ''));
const owner = ref<any>(null);
const loading = ref(false);
const activeTab = ref('blogs');

// ===== 博客列表 =====
const listLoading = ref(false);
const blogs = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 10;
const search = ref('');
const filterCat = ref<number | undefined>(undefined);
const batchMode = ref(false);
const selectedIds = ref<number[]>([]);

// ===== 分类管理 =====
const catLoading = ref(false);
const dragOverId = ref<number | null>(null);
const draggingId = ref<number | null>(null);
const catDialog = ref(false);
const catSaving = ref(false);
const catEditId = ref(0);
const catParentId = ref(0);
const catForm = reactive({ name: '', parent_id: 0, sort_order: 0 });

// 我的分类平铺（个人专栏，含待审核状态标注；用于下拉选择）
const flatCats = computed(() => {
  const out: { id: number; label: string }[] = [];
  for (const m of myCats.value) {
    out.push({ id: m.id, label: (m.full_name || m.name) + (m.status === 'pending' ? '（待审核）' : '') });
  }
  return out;
});

function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleDateString('zh-CN'); }

// ===== 博客列表操作 =====
async function loadBlogs() {
  if (!owner.value) return;
  listLoading.value = true;
  try {
    const res = await blogApi.listByAuthor(owner.value.id, page.value);
    if (res.data.code === 0) {
      let list = res.data.data.list || [];
      total.value = res.data.data.total || 0;
      if (search.value.trim()) {
        const q = search.value.trim();
        list = list.filter((b: any) => b.title.includes(q));
      }
      if (filterCat.value) {
        list = list.filter((b: any) => b.category_id === filterCat.value);
      }
      blogs.value = list;
    }
  } catch { /* ignore */ }
  finally { listLoading.value = false; }
}

async function togglePin(blog: any) {
  try {
    await blogApi.update(blog.id, { is_pinned: !blog.is_pinned });
    ElMessage.success(blog.is_pinned ? '已取消置顶' : '已置顶');
    loadBlogs();
  } catch (e: any) { ElMessage.error(e?.response?.data?.message || '操作失败'); }
}

async function removeBlog(blog: any) {
  try {
    await ElMessageBox.confirm(`确定删除博客《${blog.title}》？删除后进入回收站。`, '删除确认', {
      confirmButtonText: '删除', type: 'warning',
    });
    await blogApi.delete(blog.id);
    ElMessage.success('已删除');
    loadBlogs();
  } catch { /* 取消 */ }
}

// ===== 博客分类导入（把已有博客归入分类） =====
async function setBlogCategory(blog: any, catId: number) {
  try {
    await blogApi.update(blog.id, { category_id: catId || undefined });
    ElMessage.success(catId ? '已设置分类' : '已设为未分类');
    loadBlogs();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '设置失败');
  }
}

async function batchSetCategory(catId: number) {
  if (selectedIds.value.length === 0) return;
  try {
    for (const id of selectedIds.value) {
      await blogApi.update(id, { category_id: catId || undefined });
    }
    ElMessage.success(`已将 ${selectedIds.value.length} 篇博客${catId ? '移动到分类' : '设为未分类'}`);
    selectedIds.value = [];
    batchMode.value = false;
    loadBlogs();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '批量设置失败');
  }
}

// ===== 分类管理操作 =====
const myCats = ref<any[]>([]); // 自己创建的分类（含待审核状态）

async function loadCategories() {
  catLoading.value = true;
  try {
    // 管理视图：mine=自己创建的分类（个人专栏，含 pending/rejected；platform 不再使用——平台无分类）
    const res = await blogApi.getManageCategories();
    if (res.data.code === 0) {
      myCats.value = res.data.data?.mine || [];
    }
  } catch { /* ignore */ }
  finally { catLoading.value = false; }
}

function catStatusTag(status: string) {
  return ({ pending: 'warning', approved: 'success', rejected: 'danger' } as Record<string, any>)[status] || 'info';
}
function catStatusLabel(status: string) {
  return ({ pending: '待审核', approved: '已通过', rejected: '已拒绝' } as Record<string, string>)[status] || status;
}

// ===== 分类博客：点进分类查看博客 + 批量迁移 =====
const catBlogDialog = ref(false);
const catBlogCat = ref<any>(null);
const catBlogs = ref<any[]>([]);
const catBlogTotal = ref(0);
const catBlogLoading = ref(false);
const selectedCatBlogs = ref<number[]>([]);
const catBlogTarget = ref<number | undefined>(undefined);

async function openCatBlogs(cat: any) {
  catBlogCat.value = cat;
  catBlogs.value = [];
  catBlogTotal.value = 0;
  selectedCatBlogs.value = [];
  catBlogTarget.value = undefined;
  catBlogDialog.value = true;
  await loadCatBlogs();
}

async function loadCatBlogs() {
  if (!catBlogCat.value) return;
  catBlogLoading.value = true;
  try {
    const res = await blogApi.listByAuthor(owner.value.id, 1, catBlogCat.value.id);
    if (res.data.code === 0) {
      catBlogs.value = res.data.data?.list || [];
      catBlogTotal.value = res.data.data?.total || 0;
    }
  } catch { /* ignore */ }
  finally { catBlogLoading.value = false; }
}

async function migrateCatBlogs() {
  const target = catBlogTarget.value;
  if (selectedCatBlogs.value.length === 0 || target === undefined) return;
  const n = selectedCatBlogs.value.length;
  try {
    await ElMessageBox.confirm(`确定把这 ${n} 篇博客迁移到「${target === 0 ? '未分类' : (flatCats.value.find(c => c.id === target)?.label || target)}」？`, '批量迁移确认', {
      confirmButtonText: '迁移', type: 'warning',
    });
    for (const id of selectedCatBlogs.value) {
      await blogApi.update(id, { category_id: target });
    }
    ElMessage.success(`已迁移 ${n} 篇博客`);
    selectedCatBlogs.value = [];
    catBlogTarget.value = undefined;
    await loadCatBlogs();
    loadBlogs();
    loadCategories();
  } catch (e: any) {
    if (e !== 'cancel' && e?.response) ElMessage.error(e.response.data?.message || '迁移失败');
  }
}


function openCatCreate(parentID: number) {
  catEditId.value = 0;
  catParentId.value = parentID;
  catForm.name = '';
  catForm.parent_id = parentID;
  catForm.sort_order = 0;
  catDialog.value = true;
}

function openCatEdit(cat: any) {
  catEditId.value = cat.id;
  catParentId.value = cat.parent_id || 0;
  catForm.name = cat.name;
  catForm.parent_id = cat.parent_id || 0;
  catForm.sort_order = cat.sort_order || 0;
  catDialog.value = true;
}

async function saveCat() {
  if (!catForm.name.trim()) { ElMessage.warning('请填写分类名称'); return; }
  catSaving.value = true;
  try {
    if (catEditId.value) {
      await blogApi.updateCategory(catEditId.value, {
        name: catForm.name.trim(),
        parent_id: catForm.parent_id || 0,
        sort_order: catForm.sort_order,
      });
      ElMessage.success('已更新');
    } else {
      await blogApi.createCategory({
        name: catForm.name.trim(),
        parent_id: catForm.parent_id || 0,
        sort_order: catForm.sort_order,
      });
      ElMessage.success(catForm.parent_id ? '已创建（待管理员审核后公开）' : '已创建（待管理员审核后公开）');
    }
    catDialog.value = false;
    loadCategories();
    loadBlogs();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally { catSaving.value = false; }
}

async function removeCat(cat: any) {
  try {
    await ElMessageBox.confirm(
      `确定删除分类「${cat.name}」？其下博客将回到未分类；有子分类需先删除子分类。`,
      '删除确认', { confirmButtonText: '删除', type: 'warning' },
    );
    await blogApi.deleteCategory(cat.id);
    ElMessage.success('已删除');
    loadCategories();
    loadBlogs();
  } catch { /* 取消 */ }
}

// ===== 拖拽挂靠/排序 =====
function onDragStart(id: number) {
  draggingId.value = id;
  dragOverId.value = null;
}
function onDrop(targetId: number) {
  const src = draggingId.value;
  dragOverId.value = null;
  draggingId.value = null;
  if (!src || src === targetId) return;
  // 挂靠：拖到另一分类上 → 成为其子分类（层级/成环由后端校验）
  ElMessageBox.confirm('将该分类挂靠到目标分类下（成为其子分类）？', '挂靠确认', {
    confirmButtonText: '挂靠', cancelButtonText: '取消', type: 'warning',
  }).then(async () => {
    try {
      await blogApi.updateCategory(src, { parent_id: targetId });
      ElMessage.success('已挂靠');
      loadCategories();
      loadBlogs();
    } catch (e: any) {
      ElMessage.error(e?.response?.data?.message || '挂靠失败');
    }
  }).catch(() => {});
}

async function init() {
  loading.value = true;
  try {
    // 统一索引：数字主键（验证存在）/ 用户名
    const u = await resolveUserKey(key.value, { verifyNumeric: true });
    if (!u) {
      // 解析不到用户：纯数字 → 兼容旧「编辑博客」链接 /author/blog/{blogId}
      if (/^\d+$/.test(key.value)) {
        router.replace(`/author/blog/edit/${key.value}`);
        return;
      }
      return; // 404
    }
    // 仅本人可管理：他人访问 → 重定向公开博客主页
    if (!authStore.isLoggedIn || authStore.user?.id !== u.id) {
      router.replace(`/blog/${u.username}`);
      return;
    }
    owner.value = u;
    loadCategories();
    loadBlogs();
  } catch { /* ignore */ }
  finally { loading.value = false; }
}

onMounted(init);
</script>

<style scoped>
.blog-manager { max-width: 920px; margin: 0 auto; padding: 24px 16px; }
.bm-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; flex-wrap: wrap; gap: 10px; }
.bm-header h1 { font-size: 1.4rem; color: var(--primary-color); margin: 0; }
.bm-actions { display: flex; gap: 8px; }
.bm-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; flex-wrap: wrap; }
.bm-toolbar-spacer { flex: 1; }
.bm-count { font-size: 0.85rem; color: #999; }
.bm-list { display: flex; flex-direction: column; gap: 12px; min-height: 200px; }
.bm-item {
  display: flex; align-items: center; justify-content: space-between; gap: 12px;
  padding: 14px 18px; background: #fff; border: 1px solid #eee; border-radius: 12px; flex-wrap: wrap;
}
[data-theme="dark"] .bm-item { background: #1e293b; border-color: #334155; }
.bm-item-main { flex: 1; min-width: 200px; }
.bm-item-tags { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.bm-item-meta { font-size: 0.75rem; color: #999; }
.bm-item-title { font-size: 1rem; font-weight: 600; margin-top: 6px; cursor: pointer; }
.bm-item-title:hover { color: var(--primary-color); }
.bm-item-actions { display: flex; flex-shrink: 0; }
.pagination { margin-top: 20px; display: flex; justify-content: center; }

/* ===== 分类管理 ===== */
.bm-cat-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 14px; flex-wrap: wrap; }
.bm-cat-hint { font-size: 0.82rem; color: #888; }
.bm-cat-tree { display: flex; flex-direction: column; gap: 8px; min-height: 120px; }
.bm-cat-node {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 14px; border: 1px solid #eee; border-radius: 10px;
  background: #fff; cursor: grab; transition: background 0.15s, border-color 0.15s;
}
[data-theme="dark"] .bm-cat-node { background: #1e293b; border-color: #334155; }
.bm-cat-node:hover { border-color: var(--primary-color); }
.bm-cat-node.drag-over { border-color: var(--primary-color); background: rgba(94,114,228,0.1); }
.bm-cat-l1 { font-weight: 600; }
.bm-cat-l2 { margin-left: 28px; font-weight: 500; }
.bm-cat-l3 { margin-left: 64px; font-size: 0.92rem; }
.bm-cat-children { display: flex; flex-direction: column; gap: 8px; }
.bm-cat-children3 { display: flex; flex-direction: column; gap: 8px; }
.bm-cat-fold { width: 16px; cursor: pointer; color: #999; user-select: none; }
.bm-cat-section { font-size: 0.85rem; font-weight: 700; color: var(--text-color); margin: 6px 0 2px; padding: 8px 4px 4px; border-bottom: 1px solid #f0f0f0; }
[data-theme="dark"] .bm-cat-section { border-bottom-color: #334155; }
.bm-cat-mine { border-color: rgba(94,114,228,0.4); background: rgba(94,114,228,0.04); }
.bm-cat-reason { font-size: 0.75rem; color: #e53e3e; }
.bm-cat-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.bm-cat-actions { display: flex; flex-shrink: 0; }
.bm-cat-form-hint { font-size: 0.75rem; color: #999; margin-top: 4px; width: 100%; }

@media (max-width: 640px) {
  .bm-item-actions { width: 100%; justify-content: flex-end; }
  .bm-cat-actions { flex-wrap: wrap; }
}
</style>
