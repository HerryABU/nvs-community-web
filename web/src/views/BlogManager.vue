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
          </div>

          <div class="bm-list" v-loading="listLoading">
            <el-empty v-if="!listLoading && blogs.length === 0" description="还没有博客，点击「写博客」开始创作" />
            <div v-for="blog in blogs" :key="blog.id" class="bm-item">
              <div class="bm-item-main">
                <div class="bm-item-tags">
                  <el-tag v-if="blog.is_pinned" size="small" type="danger">置顶</el-tag>
                  <el-tag v-if="blog.category" size="small">{{ blog.category }}</el-tag>
                  <span class="bm-item-meta">{{ formatDate(blog.created_at) }} · {{ blog.view_count }} 阅读<template v-if="blog.like_count"> · 👍 {{ blog.like_count }}</template></span>
                </div>
                <div class="bm-item-title" @click="$router.push(`/author/${owner.username}/blogs/${blog.id}`)">{{ blog.title }}</div>
              </div>
              <div class="bm-item-actions">
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
            <span class="bm-cat-hint">💡 支持三级分类：<b>拖拽</b>分类到另一分类上可挂靠为其子分类；「编辑/挂靠」可设置父分类与排序。</span>
            <el-button type="primary" size="small" @click="openCatCreate(0)">+ 新增一级分类</el-button>
          </div>

          <div class="bm-cat-tree" v-loading="catLoading">
            <el-empty v-if="!catLoading && categoryTree.length === 0" description="暂无分类，点击「新增一级分类」创建" />

            <!-- 一级分类 -->
            <div v-for="cat in categoryTree" :key="cat.id"
                 class="bm-cat-node bm-cat-l1"
                 :class="{ 'drag-over': dragOverId === cat.id }"
                 draggable="true"
                 @dragstart="onDragStart(cat.id)"
                 @dragover.prevent="dragOverId = cat.id"
                 @dragleave="dragOverId = null"
                 @drop.prevent="onDrop(cat.id)">
              <span class="bm-cat-fold" @click="toggleCat('p'+cat.id)">{{ expanded['p'+cat.id] ? '▾' : '▸' }}</span>
              <span class="bm-cat-name">{{ cat.name }}</span>
              <el-tag size="small" type="info">{{ cat.blog_count }} 篇</el-tag>
              <span class="bm-cat-actions">
                <el-button size="small" text type="primary" @click="openCatCreate(cat.id)">+ 子分类</el-button>
                <el-button size="small" text type="primary" @click="openCatEdit(cat)">编辑/挂靠</el-button>
                <el-button size="small" text type="danger" @click="removeCat(cat)">删除</el-button>
              </span>
            </div>

            <!-- 二级/三级分类（展开时显示） -->
            <template v-for="cat in categoryTree" :key="'c'+cat.id">
              <div v-if="expanded['p'+cat.id]" class="bm-cat-children">
                <div v-for="sub in cat.children || []" :key="sub.id"
                     class="bm-cat-node bm-cat-l2"
                     :class="{ 'drag-over': dragOverId === sub.id }"
                     draggable="true"
                     @dragstart="onDragStart(sub.id)"
                     @dragover.prevent="dragOverId = sub.id"
                     @dragleave="dragOverId = null"
                     @drop.prevent="onDrop(sub.id)">
                  <span class="bm-cat-fold" @click="toggleCat('s'+sub.id)">{{ expanded['s'+sub.id] ? '▾' : '▸' }}</span>
                  <span class="bm-cat-name">└ {{ sub.name }}</span>
                  <el-tag size="small" type="info">{{ sub.blog_count }} 篇</el-tag>
                  <span class="bm-cat-actions">
                    <el-button size="small" text type="primary" @click="openCatCreate(sub.id)">+ 子分类</el-button>
                    <el-button size="small" text type="primary" @click="openCatEdit(sub)">编辑/挂靠</el-button>
                    <el-button size="small" text type="danger" @click="removeCat(sub)">删除</el-button>
                  </span>
                </div>
                <template v-for="sub in cat.children || []" :key="'s'+sub.id">
                  <div v-if="expanded['s'+sub.id]" class="bm-cat-children3">
                    <div v-for="g in sub.children || []" :key="g.id"
                         class="bm-cat-node bm-cat-l3"
                         :class="{ 'drag-over': dragOverId === g.id }"
                         draggable="true"
                         @dragstart="onDragStart(g.id)"
                         @dragover.prevent="dragOverId = g.id"
                         @dragleave="dragOverId = null"
                         @drop.prevent="onDrop(g.id)">
                      <span class="bm-cat-name">　└ {{ g.name }}</span>
                      <el-tag size="small" type="info">{{ g.blog_count }} 篇</el-tag>
                      <span class="bm-cat-actions">
                        <el-button size="small" text type="primary" @click="openCatEdit(g)">编辑/挂靠</el-button>
                        <el-button size="small" text type="danger" @click="removeCat(g)">删除</el-button>
                      </span>
                    </div>
                  </div>
                </template>
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
    </template>
    <el-empty v-else-if="!loading" description="无权访问该博客管理页" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { blogApi, type BlogCategoryNode } from '@/api/social';
import { publicApi } from '@/api/admin';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';

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

// ===== 分类管理 =====
const categoryTree = ref<BlogCategoryNode[]>([]);
const catLoading = ref(false);
const expanded = reactive<Record<string, boolean>>({});
const dragOverId = ref<number | null>(null);
const draggingId = ref<number | null>(null);
const catDialog = ref(false);
const catSaving = ref(false);
const catEditId = ref(0);
const catParentId = ref(0);
const catForm = reactive({ name: '', parent_id: 0, sort_order: 0 });

// 分类树平铺（带缩进标签，用于下拉选择）
const flatCats = computed(() => {
  const out: { id: number; label: string }[] = [];
  for (const c of categoryTree.value) {
    out.push({ id: c.id, label: c.name });
    for (const s of c.children || []) {
      out.push({ id: s.id, label: `　└ ${s.name}` });
      for (const g of s.children || []) {
        out.push({ id: g.id, label: `　　└ ${g.name}` });
      }
    }
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

// ===== 分类管理操作 =====
async function loadCategories() {
  catLoading.value = true;
  try {
    const res = await blogApi.getCategories();
    if (res.data.code === 0) categoryTree.value = res.data.data || [];
  } catch { /* ignore */ }
  finally { catLoading.value = false; }
}

function toggleCat(k: string) { expanded[k] = !expanded[k]; }

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
      ElMessage.success('已创建');
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
    // 解析 URL 中的用户标识（数字=id / 用户名）
    const r = await publicApi.resolveUser(key.value);
    if (r.data.code !== 0) {
      // 解析不到用户：纯数字 → 兼容旧「编辑博客」链接 /author/blog/{blogId}
      if (/^\d+$/.test(key.value)) {
        router.replace(`/author/blog/edit/${key.value}`);
        return;
      }
      return; // 404
    }
    const u = r.data.data;
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
.bm-cat-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.bm-cat-actions { display: flex; flex-shrink: 0; }
.bm-cat-form-hint { font-size: 0.75rem; color: #999; margin-top: 4px; width: 100%; }

@media (max-width: 640px) {
  .bm-item-actions { width: 100%; justify-content: flex-end; }
  .bm-cat-actions { flex-wrap: wrap; }
}
</style>
