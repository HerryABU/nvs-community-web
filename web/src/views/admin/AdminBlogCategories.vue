<template>
  <div class="admin-page">
    <el-tabs v-model="tab">
      <!-- ── 博客分类管理（按用户 + 事后审核） ── -->
      <el-tab-pane label="博客分类管理" name="cats">
        <div class="panel-head">
          <h2>博客分类管理：按用户创建 + 审核（普通作者创建的分类需审核后公开）</h2>
          <el-radio-group v-model="statusFilter" size="small" @change="loadTree">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="pending">待审核</el-radio-button>
            <el-radio-button value="approved">已通过</el-radio-button>
            <el-radio-button value="rejected">已拒绝</el-radio-button>
          </el-radio-group>
        </div>

        <!-- 审核模式开关（管理员可切换：事后审核 / 先公开后封禁） -->
        <div class="mode-bar">
          <span class="mode-label">审核模式：</span>
          <el-radio-group v-model="reviewMode" size="small" @change="changeMode">
            <el-radio-button value="post_review">① 事后审核（通过后公开）</el-radio-button>
            <el-radio-button value="pre_public">② 先公开（事后封禁）</el-radio-button>
          </el-radio-group>
          <span class="mode-hint">
            {{ reviewMode === 'pre_public'
              ? '作者创建的分类直接公开显示，管理员发现违规可随时封禁下架（拒绝）'
              : '作者创建的分类先进入待审核，管理员通过后才会公开显示' }}
          </span>
        </div>

        <div v-loading="loading">
          <el-empty v-if="!loading && filteredGroups.length === 0" description="暂无分类" />

          <!-- 二级目录：用户 → 该用户的分类；待审核用户优先展示 -->
          <div v-for="g in filteredGroups" :key="'u' + g.user_id" class="user-group">
            <div class="user-group-head" @click="toggleUser(g.user_id)">
              <span class="ug-arrow" :class="{ open: expandedUsers[g.user_id] }">▸</span>
              <el-avatar :size="26" :src="g.avatar_url || undefined">{{ ((g.nickname || g.user_name || '?')[0] || '?').toUpperCase() }}</el-avatar>
              <span class="ug-name">{{ g.nickname || g.user_name }}</span>
              <el-tag v-if="g.pending_count > 0" size="small" type="danger" effect="dark">⏳ {{ g.pending_count }} 条待审核</el-tag>
              <el-tag v-else-if="g.user_id > 0" size="small" type="success">已全部通过</el-tag>
              <el-tag v-else size="small" type="info">平台分类</el-tag>
              <span class="ug-meta">共 {{ g.total_count }} 个分类</span>
            </div>

            <div v-show="expandedUsers[g.user_id]" class="user-group-body">
              <div v-for="item in g.categories" :key="item.id" class="review-row">
                <el-tag size="small" :type="statusTag(item.status)">{{ statusLabel(item.status) }}</el-tag>
                <span class="rev-name">{{ item.full_name || item.name }}</span>
                <span class="rev-meta">
                  创建者：{{ item.creator_name || '#' + item.created_by }} · {{ item.blog_count }} 篇 · {{ formatDate(item.created_at) }}
                </span>
                <span v-if="item.status === 'rejected' && item.reject_reason" class="rev-reason">原因：{{ item.reject_reason }}</span>
                <div class="rev-actions">
                  <template v-if="item.status === 'pending'">
                    <el-button size="small" text type="success" @click="review(item, 'approve')">✅ 通过</el-button>
                    <el-button size="small" text type="danger" @click="openReject(item)">❌ 拒绝</el-button>
                  </template>
                  <template v-else-if="item.status === 'rejected'">
                    <el-button size="small" text type="success" @click="review(item, 'approve')">重新通过</el-button>
                  </template>
                  <template v-else-if="item.status === 'approved'">
                    <!-- 先公开模式下：事后封禁（下架） -->
                    <el-button size="small" text type="danger" @click="openReject(item)">⛔ 封禁</el-button>
                  </template>
                  <el-button size="small" text type="primary" @click="openEdit(item)">编辑</el-button>
                  <el-button size="small" text type="danger" @click="removeCat(item)">删除</el-button>
                </div>
              </div>
            </div>
          </div>
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

    <!-- 编辑分类弹窗（管理员） -->
    <el-dialog v-model="showDialog" title="编辑分类" width="420px">
      <el-form label-width="90px">
        <el-form-item label="分类名称" required>
          <el-input v-model="form.name" placeholder="分类名称" maxlength="64" />
        </el-form-item>
        <el-form-item label="挂靠父分类">
          <el-select v-model="form.parent_id" placeholder="不选=一级分类" clearable style="width: 100%">
            <el-option :value="0" label="（无，作为一级分类）" />
            <el-option v-for="c in allFlat" :key="c.id" :label="c.label" :value="c.id" />
          </el-select>
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

    <!-- 拒绝审核弹窗 -->
    <el-dialog v-model="rejectDialog" title="拒绝分类（填写原因）" width="420px">
      <el-input v-model="rejectReason" type="textarea" :rows="3" placeholder="拒绝原因（如：分类名称不规范 / 涉及违规内容）" maxlength="256" />
      <template #footer>
        <el-button @click="rejectDialog = false">取消</el-button>
        <el-button type="danger" :loading="saving" @click="doReject">确认拒绝</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted } from 'vue';
import { adminApi } from '@/api/admin';
import { ElMessage, ElMessageBox } from 'element-plus';

const tab = ref('cats');
const statusFilter = ref('');
const reviewMode = ref<'post_review' | 'pre_public'>('post_review');
const rawList = ref<any[]>([]); // 全量平铺（编辑弹窗父分类下拉）
const groups = ref<any[]>([]);  // 按创建者分组（二级目录）
const expandedUsers = reactive<Record<number, boolean>>({}); // 用户块展开状态
const loading = ref(false);
const saving = ref(false);
const showDialog = ref(false);
const rejectDialog = ref(false);
const editingId = ref(0);
const rejectingItem = ref<any>(null);
const rejectReason = ref('');
const form = reactive({ name: '', parent_id: 0, sort_order: 0 });

// 审核模式切换
async function changeMode() {
  try {
    const res = await adminApi.setCategoryReviewMode(reviewMode.value);
    if (res.data.code === 0) {
      ElMessage.success('已切换为：' + (res.data.data?.label || reviewMode.value));
      loadTree();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '切换失败');
    reviewMode.value = reviewMode.value === 'pre_public' ? 'post_review' : 'pre_public';
  }
}

// 全量平铺（编辑弹窗父分类下拉）
const allFlat = computed(() =>
  rawList.value.map((i: any) => ({ id: i.id, label: i.full_name || i.name })),
);

// 按状态筛选后的分组（过滤后为空的用户隐藏；pending_count 保持真实值）
const filteredGroups = computed(() => {
  if (!statusFilter.value) return groups.value;
  return groups.value
    .map((g: any) => ({
      ...g,
      categories: (g.categories || []).filter((i: any) => i.status === statusFilter.value),
    }))
    .filter((g: any) => (g.categories || []).length > 0);
});

function toggleUser(uid: number) {
  expandedUsers[uid] = !expandedUsers[uid];
}

function statusTag(s: string) {
  return ({ pending: 'warning', approved: 'success', rejected: 'danger' } as Record<string, any>)[s] || 'info';
}
function statusLabel(s: string) {
  return ({ pending: '待审核', approved: '已通过', rejected: '已拒绝' } as Record<string, string>)[s] || s;
}
function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleString('zh-CN'); }

async function loadTree() {
  loading.value = true;
  try {
    const [res, modeRes] = await Promise.all([
      adminApi.getBlogCategories(),
      adminApi.getCategoryReviewMode(),
    ]);
    if (modeRes.data.code === 0) reviewMode.value = modeRes.data.data?.mode || 'post_review';
    if (res.data.code === 0) {
      const d = res.data.data || {};
      rawList.value = d.list || [];
      groups.value = d.groups || [];
      // 有待审核分类的用户默认展开（管理员一眼看到"谁还需审核"），其余折叠
      for (const g of groups.value) {
        if (g.pending_count > 0) expandedUsers[g.user_id] = true;
      }
    }
  } catch { /* ignore */ }
  finally { loading.value = false; }
}

async function review(item: any, action: 'approve' | 'reject') {
  saving.value = true;
  try {
    await adminApi.reviewBlogCategory(item.id, action);
    ElMessage.success(action === 'approve' ? '已通过审核' : '已拒绝');
    loadTree();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally { saving.value = false; }
}

function openReject(item: any) {
  rejectingItem.value = item;
  rejectReason.value = '';
  rejectDialog.value = true;
}

async function doReject() {
  if (!rejectingItem.value) return;
  saving.value = true;
  try {
    await adminApi.reviewBlogCategory(rejectingItem.value.id, 'reject', rejectReason.value.trim() || undefined);
    ElMessage.success('已拒绝');
    rejectDialog.value = false;
    loadTree();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally { saving.value = false; }
}

function openEdit(item: any) {
  editingId.value = item.id;
  form.name = item.name;
  form.parent_id = item.parent_id || 0;
  form.sort_order = item.sort_order || 0;
  showDialog.value = true;
}

async function saveCat() {
  if (!form.name.trim()) { ElMessage.warning('请填写分类名称'); return; }
  saving.value = true;
  try {
    await adminApi.updateBlogCategory(editingId.value, {
      name: form.name.trim(),
      parent_id: form.parent_id || 0,
      sort_order: form.sort_order,
    });
    ElMessage.success('已更新');
    showDialog.value = false;
    loadTree();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally { saving.value = false; }
}

async function removeCat(item: any) {
  try {
    await ElMessageBox.confirm(`确定删除分类「${item.full_name || item.name}」？其下博客将回到未分类。`, '删除确认', {
      confirmButtonText: '删除', type: 'warning',
    });
    await adminApi.deleteBlogCategory(item.id);
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
.panel-head h2 { font-size: 1.05rem; margin: 0; }

/* 审核模式切换条 */
.mode-bar {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 14px; margin-bottom: 14px;
  border: 1px dashed var(--border-color); border-radius: 10px;
  background: var(--bg-color, #f8fafc);
  flex-wrap: wrap;
}
[data-theme="dark"] .mode-bar { background: rgba(255,255,255,0.03); }
.mode-label { font-weight: 700; font-size: 0.9rem; }
.mode-hint { font-size: 0.8rem; color: #999; }

.review-row {
  display: flex; align-items: center; gap: 10px;
  padding: 12px 16px; border: 1px solid var(--border-color);
  border-radius: 10px; margin-bottom: 8px; flex-wrap: wrap;
}
.rev-name { font-weight: 600; min-width: 140px; }
.rev-meta { font-size: 0.78rem; color: #999; flex: 1; min-width: 200px; }
.rev-reason { font-size: 0.78rem; color: #e53e3e; }
.rev-actions { display: flex; gap: 2px; flex-shrink: 0; }

/* ===== 二级目录：用户组 ===== */
.user-group {
  border: 1px solid var(--border-color);
  border-radius: 12px;
  margin-bottom: 12px;
  overflow: hidden;
}
.user-group-head {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 14px;
  background: var(--bg-color, #f8fafc);
  cursor: pointer;
  user-select: none;
  transition: background 0.2s;
}
.user-group-head:hover { background: rgba(94, 114, 228, 0.06); }
[data-theme="dark"] .user-group-head { background: rgba(255, 255, 255, 0.03); }
.ug-arrow {
  width: 14px; color: #999; font-size: 0.8rem;
  transition: transform 0.2s;
}
.ug-arrow.open { transform: rotate(90deg); }
.ug-name { font-weight: 700; font-size: 0.95rem; }
.ug-meta { font-size: 0.78rem; color: #999; margin-left: auto; }
.user-group-body { padding: 10px 12px 4px; }

.recycle-item {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 14px; border: 1px solid var(--border-color);
  border-radius: 8px; margin-bottom: 8px; flex-wrap: wrap;
}
.recycle-title { flex: 1; min-width: 160px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.recycle-meta { font-size: 0.78rem; color: #999; white-space: nowrap; }
.recycle-actions { flex-shrink: 0; }
</style>
