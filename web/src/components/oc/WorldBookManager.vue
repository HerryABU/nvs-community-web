<template>
  <div class="wbm-wrap" v-loading="loading">
    <div class="wbm-bar">
      <el-select v-model="bookId" placeholder="选择世界书" style="width: 220px" @change="loadEntries">
        <el-option v-for="b in books" :key="b.id" :label="`${b.title}（v${b.version}）`" :value="b.id" />
      </el-select>
      <el-button size="small" type="primary" @click="createBook">新建世界书</el-button>
      <el-button size="small" :disabled="!bookId" @click="openEntry()">新建条目</el-button>
      <span class="wbm-tip">设定按键触发注入（酒馆 Lorebook 玩法）：对话里出现触发词才进提示词</span>
    </div>

    <el-descriptions v-if="book" :column="4" size="small" border class="wbm-desc">
      <el-descriptions-item label="扫描深度">{{ book.scan_depth }} 条</el-descriptions-item>
      <el-descriptions-item label="Token 预算">{{ book.token_budget }}</el-descriptions-item>
      <el-descriptions-item label="递归触发">{{ book.recursive_scan ? '开' : '关' }}</el-descriptions-item>
      <el-descriptions-item label="可见性">{{ book.visibility === 'public' ? '公开' : '私有' }}</el-descriptions-item>
    </el-descriptions>

    <el-table :data="entries" size="small">
      <el-table-column label="触发词" min-width="140">
        <template #default="{ row }">
          <el-tag v-for="k in row.key.split(',')" :key="k" size="small" style="margin-right:4px">{{ k.trim() }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="内容" min-width="260">
        <template #default="{ row }">
          <div class="wbm-content">{{ row.content }}</div>
        </template>
      </el-table-column>
      <el-table-column label="触发" width="120">
        <template #default="{ row }">
          <el-tag v-if="row.constant" size="small" type="warning">常驻</el-tag>
          <el-tag v-else size="small" type="info">{{ row.probability }}%</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="insertion_order" label="顺序" width="70" />
      <el-table-column label="操作" width="130">
        <template #default="{ row }">
          <el-button size="small" text type="primary" @click="openEntry(row)">编辑</el-button>
          <el-popconfirm title="删除该条目？" @confirm="removeEntry(row)">
            <template #reference><el-button size="small" text type="danger">删除</el-button></template>
          </el-popconfirm>
        </template>
      </el-table-column>
      <template #empty><el-empty description="还没有条目" :image-size="60" /></template>
    </el-table>

    <el-dialog v-model="dlg" :title="form.id ? '编辑条目' : '新建条目'" width="640px">
      <el-form label-width="96px" size="default">
        <el-form-item label="条目名"><el-input v-model="form.key" placeholder="同时作为主触发词，多个用逗号分隔" /></el-form-item>
        <el-form-item label="分类">
          <el-select v-model="form.kind">
            <el-option label="世界观" value="world" /><el-option label="势力" value="faction" />
            <el-option label="地点" value="place" /><el-option label="规则" value="rule" /><el-option label="事件" value="event" />
          </el-select>
        </el-form-item>
        <el-form-item label="次触发词"><el-input v-model="form.secondary_keys" placeholder="可选：配合 selective 逻辑" /></el-form-item>
        <el-form-item label="内容"><el-input v-model="form.content" type="textarea" :rows="5" /></el-form-item>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="常驻"><el-switch v-model="form.constant" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="概率"><el-input-number v-model="form.probability" :min="1" :max="100" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="插入顺序"><el-input-number v-model="form.insertion_order" :min="1" :max="999" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="位置">
              <el-select v-model="form.position">
                <el-option label="角色定义前" :value="0" /><el-option label="角色定义后" :value="1" />
                <el-option label="对话内" :value="4" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8"><el-form-item label="包含组"><el-input v-model="form.group" placeholder="同组只取其一" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="启用"><el-switch v-model="form.enabled" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dlg = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveEntry">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ocApi, type OcWorldBook, type OcWorldEntry } from '@/api/oc';

const props = defineProps<{ groupId: number }>();

const loading = ref(false);
const saving = ref(false);
const books = ref<OcWorldBook[]>([]);
const bookId = ref<number>(0);
const book = ref<OcWorldBook | null>(null);
const entries = ref<OcWorldEntry[]>([]);

const dlg = ref(false);
const form = reactive<Partial<OcWorldEntry>>({
  key: '', kind: 'world', content: '', secondary_keys: '', constant: false,
  probability: 100, insertion_order: 100, position: 1, group: '', enabled: true,
});

async function loadBooks() {
  loading.value = true;
  try {
    const res = await ocApi.listBooks({ scope: 'mine' });
    books.value = res.data.data?.list || [];
    if (!bookId.value && books.value.length > 0) {
      bookId.value = books.value[0].id;
      await loadEntries();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载世界书失败');
  } finally {
    loading.value = false;
  }
}

async function loadEntries() {
  if (!bookId.value) return;
  loading.value = true;
  try {
    const res = await ocApi.getBook(bookId.value);
    book.value = res.data.data || null;
    entries.value = book.value?.entries || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载条目失败');
  } finally {
    loading.value = false;
  }
}

async function createBook() {
  try {
    const { value } = await ElMessageBox.prompt('世界书标题', '新建世界书', { inputPattern: /\S+/, inputErrorMessage: '请输入标题' });
    await ocApi.createBook({ title: value, group_id: props.groupId || 0 });
    ElMessage.success('已创建');
    await loadBooks();
  } catch { /* 取消 */ }
}

function openEntry(e?: OcWorldEntry) {
  Object.assign(form, e ? { ...e } : {
    id: undefined, key: '', kind: 'world', content: '', secondary_keys: '', constant: false,
    probability: 100, insertion_order: 100, position: 1, group: '', enabled: true,
  });
  dlg.value = true;
}

async function saveEntry() {
  if (!bookId.value) return;
  if (!form.key?.trim() || !form.content?.trim()) {
    ElMessage.warning('触发词与内容不能为空');
    return;
  }
  saving.value = true;
  try {
    await ocApi.saveEntry(bookId.value, form);
    ElMessage.success('已保存');
    dlg.value = false;
    await loadEntries();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function removeEntry(e: OcWorldEntry) {
  try {
    await ocApi.deleteEntry(bookId.value, e.id);
    ElMessage.success('已删除');
    await loadEntries();
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.message || '删除失败');
  }
}

onMounted(loadBooks);
</script>

<style scoped>
.wbm-bar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 10px; }
.wbm-tip { font-size: 0.75rem; color: var(--text-muted); }
.wbm-desc { margin-bottom: 10px; }
.wbm-content {
  font-size: 0.8rem; line-height: 1.6; max-height: 60px; overflow: hidden;
  display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical;
}
</style>
