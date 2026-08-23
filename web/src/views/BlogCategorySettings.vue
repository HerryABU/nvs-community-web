<template>
  <div class="cat-settings" v-loading="initLoading">
    <div class="cs-topbar">
      <el-button text @click="$router.push(`/author/blog/${owner.username}/category/${catId}`)">
        <el-icon><ArrowLeft /></el-icon>返回分类文章管理
      </el-button>
      <span class="cs-title">⚙ 分类设置：{{ cat?.full_name || cat?.name || '' }}</span>
    </div>

    <div v-if="cat" class="cs-card">
      <div class="cs-status">
        <el-tag :type="catStatusTag(cat.status)">{{ catStatusLabel(cat.status) }}</el-tag>
        <span v-if="cat.status === 'pending'" class="cs-hint">待管理员审核，通过后公开显示</span>
        <span v-if="cat.status === 'rejected' && cat.reject_reason" class="cs-reason">拒绝原因：{{ cat.reject_reason }}</span>
        <span v-if="cat.status === 'approved'" class="cs-hint">已公开显示</span>
      </div>

      <el-form label-width="100px" class="cs-form">
        <el-form-item label="分类名称" required>
          <el-input v-model="form.name" placeholder="分类名称" maxlength="64" />
        </el-form-item>
        <el-form-item label="挂靠父分类">
          <el-select v-model="form.parent_id" placeholder="不选=一级分类" clearable style="width: 100%">
            <el-option :value="0" label="（无，作为一级分类）" />
            <el-option v-for="c in parentOptions" :key="c.id" :label="c.label" :value="c.id" />
          </el-select>
          <div class="cs-hint">一级挂无，二级挂一级，三级挂二级（最多三级）；修改后需要重新审核</div>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort_order" :min="0" :max="9999" />
          <span class="cs-hint" style="margin-left:10px">数值越小越靠前</span>
        </el-form-item>
      </el-form>

      <div class="cs-actions">
        <el-button type="primary" :loading="saving" @click="save">保存设置</el-button>
        <el-button type="danger" plain :loading="deleting" @click="remove">删除分类</el-button>
      </div>
      <div class="cs-del-hint">删除后该分类下的博客将回到「未分类」；有子分类需先删除子分类。</div>
    </div>
    <el-empty v-else-if="!initLoading" description="分类不存在或已被删除" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { blogApi } from '@/api/social';
import { resolveUserKey } from '@/utils/userKey';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ArrowLeft } from '@element-plus/icons-vue';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const key = computed(() => route.params.key as string);
const catId = computed(() => Number(route.params.id) || 0);
const owner = ref<any>(null);
const initLoading = ref(true);
const cat = ref<any>(null);
const myCats = ref<any[]>([]);
const saving = ref(false);
const deleting = ref(false);
const form = reactive({ name: '', parent_id: 0, sort_order: 0 });

const catStatusTag = (s: string) => ({ pending: 'warning', approved: 'success', rejected: 'danger' } as Record<string, any>)[s] || 'info';
const catStatusLabel = (s: string) => ({ pending: '待审核', approved: '已通过', rejected: '已拒绝' } as Record<string, string>)[s] || s;

// 父分类候选：自己的分类中层级低于当前分类的（不能挂自己/子孙下）
const parentOptions = computed(() =>
  flatCats()
    .filter(c => c.id !== catId.value)
    .map(c => ({ id: c.id, label: c.label })),
);

function flatCats(): { id: number; label: string }[] {
  const out: { id: number; label: string }[] = [];
  for (const m of myCats.value) {
    out.push({ id: m.id, label: (m.full_name || m.name) + (m.status === 'pending' ? '（待审核）' : '') });
  }
  return out;
}

async function loadCategory() {
  try {
    const res = await blogApi.getManageCategories();
    if (res.data.code === 0) {
      myCats.value = res.data.data?.mine || [];
      cat.value = myCats.value.find((m: any) => m.id === catId.value) || null;
      if (cat.value) {
        form.name = cat.value.name;
        form.parent_id = cat.value.parent_id || 0;
        form.sort_order = cat.value.sort_order || 0;
      }
    }
  } catch { /* ignore */ }
}

async function save() {
  if (!form.name.trim()) { ElMessage.warning('请填写分类名称'); return; }
  saving.value = true;
  try {
    await blogApi.updateCategory(catId.value, {
      name: form.name.trim(),
      parent_id: form.parent_id || 0,
      sort_order: form.sort_order,
    });
    ElMessage.success('已保存' + (cat.value?.status === 'approved' ? '（若改名/挂靠后需重新审核）' : ''));
    await loadCategory();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally { saving.value = false; }
}

async function remove() {
  try {
    await ElMessageBox.confirm(`确定删除分类「${cat.value?.full_name || cat.value?.name}」？其下博客将回到未分类；有子分类需先删除子分类。`, '删除确认', {
      confirmButtonText: '删除', type: 'warning',
    });
    deleting.value = true;
    await blogApi.deleteCategory(catId.value);
    ElMessage.success('已删除');
    router.replace(`/author/blog/${owner.value.username}`);
  } catch (e: any) {
    if (e !== 'cancel' && e?.response) ElMessage.error(e.response.data?.message || '删除失败');
  } finally { deleting.value = false; }
}

onMounted(async () => {
  const u = await resolveUserKey(key.value, { verifyNumeric: true });
  if (!u) { initLoading.value = false; return; }
  if (!authStore.isLoggedIn || authStore.user?.id !== u.id) {
    router.replace(`/blog/${u.username}`);
    return;
  }
  owner.value = u;
  await loadCategory();
  initLoading.value = false;
});
</script>

<style scoped>
.cat-settings { max-width: 640px; margin: 0 auto; padding: 24px; }
.cs-topbar { display: flex; align-items: center; gap: 12px; margin-bottom: 20px; }
.cs-title { font-size: 1.2rem; font-weight: 700; }
.cs-card { background: #fff; border: 1px solid var(--border-color); border-radius: 14px; padding: 24px; }
[data-theme="dark"] .cs-card { background: #1e293b; border-color: #334155; }
.cs-status { display: flex; align-items: center; gap: 10px; margin-bottom: 20px; flex-wrap: wrap; }
.cs-hint { font-size: 0.8rem; color: #999; }
.cs-reason { font-size: 0.85rem; color: #e53e3e; }
.cs-form { max-width: 480px; }
.cs-actions { display: flex; gap: 12px; margin-top: 8px; }
.cs-del-hint { font-size: 0.78rem; color: #999; margin-top: 12px; }
</style>
