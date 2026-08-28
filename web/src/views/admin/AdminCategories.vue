<template>
  <div class="admin-page">
    <AdminHeader title="书目分类管理" subtitle="平台书目分类的增删改" :icon="Collection" />
    <div class="admin-page-body admin-panel">
      <p class="hint-text">定义平台的书目分类类型，读者可按分类浏览作品。双击标签重命名，点击 × 删除，修改后点击保存。</p>
      <div class="tag-list">
        <el-tag
          v-for="(cat, idx) in editableCategories"
          :key="idx"
          closable
          size="large"
          class="category-tag-item"
          @close="removeCategory(idx)"
        >
          <template v-if="editingCatIdx === idx">
            <el-input
              v-model="editCatName"
              size="small"
              class="cat-edit-input"
              @blur="saveCatName(idx)"
              @keyup.enter="saveCatName(idx)"
              @keyup.escape="cancelEditCat"
              ref="catEditRef"
            />
          </template>
          <template v-else>
            <span @dblclick="startEditCat(idx, cat)">{{ cat }}</span>
          </template>
        </el-tag>
      </div>
      <div style="margin-top: 12px">
        <el-button type="primary" @click="addCategory" :icon="Plus">添加分类</el-button>
        <el-button type="primary" @click="saveCategories" :loading="savingCategories">保存分类</el-button>
        <el-button @click="loadCategories" :disabled="savingCategories">重置</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { Plus, Collection } from '@element-plus/icons-vue';
import AdminHeader from '@/components/admin/AdminHeader.vue';
import { adminApi } from '@/api/admin';
import { ElMessage } from 'element-plus';

const editableCategories = ref<string[]>([]);
const editingCatIdx = ref(-1);
const editCatName = ref('');
const savingCategories = ref(false);
const catEditRef = ref<any>(null);

async function loadCategories() {
  try {
    const res = await adminApi.getConfig();
    const data = res.data.data || res.data || {};
    editableCategories.value = parseCategories(data.categories);
  } catch { /* ignore */ }
}

// 兼容 JSON 数组 ["a","b"] 与逗号分隔 "a,b" 两种存储格式
function parseCategories(val: any): string[] {
  if (!val) return [];
  if (Array.isArray(val)) return val.map(String);
  const s = String(val).trim();
  if (s.startsWith('[')) {
    try { const arr = JSON.parse(s); return Array.isArray(arr) ? arr.map(String) : []; } catch { /* ignore */ }
  }
  return s.split(',').map((x: string) => x.trim()).filter(Boolean);
}

async function saveCategories() {
  savingCategories.value = true;
  try {
    await adminApi.updateConfig({ categories: JSON.stringify(editableCategories.value) });
    ElMessage.success('分类已保存');
  } catch { ElMessage.error('保存失败'); }
  finally { savingCategories.value = false; }
}

function addCategory() {
  editableCategories.value.push('新分类');
}

function removeCategory(idx: number) { editableCategories.value.splice(idx, 1); }

function startEditCat(idx: number, name: string) {
  editingCatIdx.value = idx;
  editCatName.value = name;
}

function saveCatName(idx: number) {
  if (editCatName.value.trim()) editableCategories.value[idx] = editCatName.value.trim();
  editingCatIdx.value = -1;
}

function cancelEditCat() { editingCatIdx.value = -1; }

onMounted(loadCategories);
</script>

<style scoped>
.admin-page { --admin-content-max: 760px; min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 760px; }
.hint-text { color: var(--text-light, #666); font-size: 0.85rem; margin-bottom: 16px; }
.tag-list { display: flex; flex-wrap: wrap; gap: 10px; }
.category-tag-item { height: auto; padding: 6px 12px; }
.cat-edit-input { width: 90px; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .admin-page-header h2 { color: #e2e8f0; }
</style>
