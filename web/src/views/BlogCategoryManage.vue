<template>
  <div class="cat-manage" v-loading="initLoading">
    <div class="cm-topbar">
      <el-button text @click="$router.push(`/author/blog/${owner.username}`)">
        <el-icon><ArrowLeft /></el-icon>返回博客管理
      </el-button>
      <span class="cm-title">📂 分类《{{ cat?.full_name || cat?.name || '' }}》</span>
      <span class="cm-meta">
        <el-tag v-if="cat" size="small" :type="catStatusTag(cat.status)">{{ catStatusLabel(cat.status) }}</el-tag>
        <span v-if="cat && cat.status === 'rejected' && cat.reject_reason" class="cm-reason">原因：{{ cat.reject_reason }}</span>
      </span>
      <el-button size="small" type="primary" plain @click="$router.push(`/author/blog/${owner.username}/category/${catId}/settings`)">⚙ 分类设置</el-button>
    </div>

    <!-- 批量操作提示条 -->
    <div class="cm-toolbar">
      <el-radio-group v-model="viewMode" size="small" @change="switchView">
        <el-radio-button :value="'in'">本分类文章{{ inTotal > 0 ? `（${inTotal}）` : '' }}</el-radio-button>
        <el-radio-button :value="'out'">新增其他文章（未分类 / 其他分类）</el-radio-button>
      </el-radio-group>
      <el-input v-model="search" placeholder="搜索标题…" clearable size="small" style="width: 200px" @keyup.enter="loadList" />
      <span class="cm-hint">
        {{ viewMode === 'in'
          ? '勾选后可批量：移动到其他分类 / 移出到未分类'
          : '勾选「不在本分类」的文章，批量加入本分类（CSDN 式新增）' }}
      </span>
    </div>

    <div class="cm-batchbar" v-if="selected.length > 0">
      <span class="cm-selected">已选 {{ selected.length }} 篇</span>
      <template v-if="viewMode === 'in'">
        <el-select v-model="moveTarget" placeholder="移动到分类…" clearable size="small" style="width: 180px">
          <el-option :value="0" label="（移出到未分类）" />
          <el-option v-for="c in otherCats" :key="c.id" :label="c.label" :value="c.id" />
        </el-select>
        <el-button type="primary" size="small" :disabled="moveTarget === undefined" @click="doBatch('move')">批量移动</el-button>
      </template>
      <template v-else>
        <el-button type="primary" size="small" @click="doBatch('add')">✅ 加入本分类</el-button>
      </template>
      <el-button size="small" @click="selected = []">取消选择</el-button>
    </div>

    <div class="cm-list" v-loading="listLoading" style="min-height: 200px">
      <el-empty v-if="!listLoading && blogs.length === 0" :description="viewMode === 'in' ? '该分类下暂无博客，切到「新增其他文章」可从未分类/其他分类批量加入' : '没有可加入的文章（全部博客都已在本分类）'" />
      <div v-for="b in blogs" :key="b.id" class="cm-item">
        <el-checkbox v-model="selected" :value="b.id" size="small" @click.stop />
        <div class="cm-item-main">
          <div class="cm-item-title" @click="$router.push(`/author/${owner.username}/blogs/${b.id}`)">{{ b.title }}</div>
          <div class="cm-item-meta">
            <el-tag v-if="b.is_pinned" size="small" type="danger">置顶</el-tag>
            <el-tag v-if="b.category_id === 0 && viewMode === 'out'" size="small" type="info">未分类</el-tag>
            <el-tag v-if="b.category" size="small">{{ b.category }}</el-tag>
            <span v-for="t in b.tags || []" :key="t.id" class="cm-tag">#{{ t.name }}</span>
            <span class="cm-date">{{ formatDate(b.created_at) }}</span>
          </div>
        </div>
        <div class="cm-item-actions">
          <el-dropdown v-if="viewMode === 'in'" @command="(cid: number) => moveOne(b, cid)">
            <el-button size="small" text type="primary">移动<el-icon style="margin-left:2px"><ArrowDown /></el-icon></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item :command="0" :disabled="!b.category_id">移出到未分类</el-dropdown-item>
                <el-dropdown-item v-for="c in otherCats" :key="c.id" :command="c.id" :disabled="c.id === b.category_id">{{ c.label }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button size="small" text type="primary" @click="$router.push(`/author/blog/edit/${b.id}`)">编辑</el-button>
        </div>
      </div>
    </div>

    <div class="pagination" v-if="total > pageSize">
      <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" @current-change="loadList" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { blogApi } from '@/api/social';
import { resolveUserKey } from '@/utils/userKey';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ArrowDown, ArrowLeft } from '@element-plus/icons-vue';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const key = computed(() => route.params.key as string);
const catId = computed(() => Number(route.params.id) || 0);
const owner = ref<any>(null);
const initLoading = ref(true);

// 分类信息 + 我的分类平铺
const cat = ref<any>(null);
const myCats = ref<any[]>([]);
const catStatusTag = (s: string) => ({ pending: 'warning', approved: 'success', rejected: 'danger' } as Record<string, any>)[s] || 'info';
const catStatusLabel = (s: string) => ({ pending: '待审核', approved: '已通过', rejected: '已拒绝' } as Record<string, string>)[s] || s;

const flatCats = computed(() =>
  (myCats.value || []).map((m: any) => ({
    id: m.id,
    label: (m.full_name || m.name) + (m.status === 'pending' ? '（待审核）' : ''),
  })),
);
// 目标分类（排除当前分类）
const otherCats = computed(() => flatCats.value.filter(c => c.id !== catId.value));

// 列表
const viewMode = ref<'in' | 'out'>('in');
const blogs = ref<any[]>([]);
const total = ref(0);
const inTotal = ref(0);
const page = ref(1);
const pageSize = 10;
const search = ref('');
const selected = ref<number[]>([]);
const moveTarget = ref<number | undefined>(undefined);
const listLoading = ref(false);

function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleDateString('zh-CN'); }

async function loadList() {
  listLoading.value = true;
  try {
    const params: any = { page: page.value };
    if (viewMode.value === 'in') params.category_id = catId.value;
    else params.exclude_category_id = catId.value;
    if (search.value.trim()) params.search = search.value.trim();
    // listByAuthor 只支持 categoryId/excludeCategoryId 参数，search 走前端过滤
    const res = await blogApi.listByAuthor(owner.value.id, page.value, params.category_id, undefined, params.exclude_category_id);
    if (res.data.code === 0) {
      let list = res.data.data.list || [];
      total.value = res.data.data.total || 0;
      if (search.value.trim()) {
        const q = search.value.trim();
        list = list.filter((b: any) => (b.title || '').includes(q));
        total.value = list.length;
      }
      blogs.value = list;
      if (viewMode.value === 'in') inTotal.value = res.data.data.total || 0;
    }
  } catch { /* ignore */ }
  finally { listLoading.value = false; }
}

function switchView() {
  page.value = 1;
  selected.value = [];
  moveTarget.value = undefined;
  loadList();
}

async function loadCategoryInfo() {
  try {
    const res = await blogApi.getManageCategories();
    if (res.data.code === 0) {
      myCats.value = res.data.data?.mine || [];
      cat.value = myCats.value.find((m: any) => m.id === catId.value) || null;
    }
  } catch { /* ignore */ }
}

// 批量：移动（本分类） / 新增（其他文章加入本分类）
async function doBatch(action: 'move' | 'add') {
  if (selected.value.length === 0) return;
  if (action === 'move' && moveTarget.value === undefined) { ElMessage.warning('请选择目标分类'); return; }
  const target = action === 'add' ? catId.value : moveTarget.value!;
  const n = selected.value.length;
  try {
    await ElMessageBox.confirm(
      action === 'add'
        ? `确定把这 ${n} 篇博客加入分类「${cat.value?.full_name || cat.value?.name}」？`
        : `确定把这 ${n} 篇博客${target === 0 ? '移出到未分类' : `移动到「${otherCats.value.find(c => c.id === target)?.label || target}」`}？`,
      '批量操作确认', { confirmButtonText: '确定', type: 'warning' },
    );
    await blogApi.batchSetCategory(selected.value, target);
    ElMessage.success(`已处理 ${n} 篇博客`);
    selected.value = [];
    moveTarget.value = undefined;
    loadList();
    loadCategoryInfo();
  } catch (e: any) {
    if (e !== 'cancel' && e?.response) ElMessage.error(e.response.data?.message || '操作失败');
  }
}

async function moveOne(b: any, target: number) {
  try {
    await blogApi.batchSetCategory([b.id], target);
    ElMessage.success(target === 0 ? '已移出到未分类' : '已移动');
    loadList();
    loadCategoryInfo();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

onMounted(async () => {
  const u = await resolveUserKey(key.value, { verifyNumeric: true });
  if (!u) { initLoading.value = false; return; }
  // 仅本人可管理
  if (!authStore.isLoggedIn || authStore.user?.id !== u.id) {
    router.replace(`/blog/${u.username}`);
    return;
  }
  owner.value = u;
  await loadCategoryInfo();
  await loadList();
  initLoading.value = false;
});
</script>

<style scoped>
.cat-manage { max-width: 960px; margin: 0 auto; padding: 24px; }
.cm-topbar { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
.cm-title { font-size: 1.2rem; font-weight: 700; }
.cm-meta { display: flex; align-items: center; gap: 8px; }
.cm-reason { font-size: 0.8rem; color: #e53e3e; }
.cm-toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; flex-wrap: wrap; }
.cm-hint { font-size: 0.8rem; color: #999; }
.cm-batchbar {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 14px; margin-bottom: 12px;
  background: rgba(94,114,228,0.06); border: 1px dashed var(--border-color);
  border-radius: 10px; flex-wrap: wrap;
}
.cm-selected { font-weight: 700; color: var(--primary-color); }
.cm-list { display: flex; flex-direction: column; gap: 8px; }
.cm-item {
  display: flex; align-items: center; gap: 10px;
  padding: 12px 14px; border: 1px solid var(--border-color);
  border-radius: 10px; flex-wrap: wrap;
}
.cm-item-main { flex: 1; min-width: 200px; }
.cm-item-title { font-weight: 600; cursor: pointer; }
.cm-item-title:hover { color: var(--primary-color); }
.cm-item-meta { display: flex; align-items: center; gap: 8px; margin-top: 4px; flex-wrap: wrap; }
.cm-tag { font-size: 0.75rem; color: var(--primary-color); }
.cm-date { font-size: 0.78rem; color: #999; }
.cm-item-actions { display: flex; gap: 2px; flex-shrink: 0; }
</style>
