<template>
  <!-- AI 提示词库：平台预设 + 我的自定义；点击卡片插入到调用方输入框 -->
  <el-drawer
    :model-value="modelValue"
    title="提示词库"
    size="520px"
    :with-header="true"
    @update:model-value="(v: boolean) => emit('update:modelValue', v)"
  >
    <div class="pl-wrap">
      <div class="pl-toolbar">
        <el-radio-group v-model="kind" size="small" @change="load">
          <el-radio-button value="chat">写作/对话</el-radio-button>
          <el-radio-button value="image">绘画</el-radio-button>
          <el-radio-button value="video">视频</el-radio-button>
        </el-radio-group>
        <el-input
          v-model="keyword"
          size="small"
          placeholder="搜索标题/内容/分类"
          clearable
          style="width:170px"
          @keyup.enter="load"
          @clear="load"
        >
          <template #append>
            <el-button size="small" @click="load">搜索</el-button>
          </template>
        </el-input>
      </div>

      <div v-if="categories.length" class="pl-cats">
        <span :class="['pl-cat', { active: !category }]" @click="pickCategory('')">全部</span>
        <span
          v-for="c in categories"
          :key="c"
          :class="['pl-cat', { active: category === c }]"
          @click="pickCategory(c)"
        >{{ c }}</span>
      </div>

      <div v-loading="loading" class="pl-list">
        <div v-if="!filtered.length && !loading" class="pl-empty">暂无提示词，可在下方保存一条</div>
        <div
          v-for="p in filtered"
          :key="p.id"
          class="pl-card"
          @click="usePrompt(p)"
        >
          <div class="pl-card-head">
            <span class="pl-card-title">{{ p.title }}</span>
            <span v-if="p.builtin" class="pl-tag">预设</span>
            <span v-else class="pl-tag pl-tag-own">我的</span>
            <span v-if="p.category" class="pl-cat-name">{{ p.category }}</span>
            <span class="pl-card-tools">
              <el-icon title="编辑" @click.stop="editPrompt(p)"><EditPen /></el-icon>
              <el-icon v-if="!p.builtin" title="删除" class="pl-del" @click.stop="removePrompt(p)"><Delete /></el-icon>
            </span>
          </div>
          <div class="pl-card-body">{{ p.content }}</div>
        </div>
      </div>

      <div class="pl-save">
        <el-button type="primary" plain size="small" @click="openCreate">
          <el-icon><Plus /></el-icon> 保存为我的提示词
        </el-button>
        <span class="pl-save-hint">支持变量占位 <code>&#123;&#123;变量&#125;&#125;</code>，插入后自行替换</span>
      </div>
    </div>

    <!-- 新建/编辑 -->
    <el-dialog v-model="editVisible" :title="editing ? '编辑提示词' : '新建提示词'" width="560px" append-to-body>
      <el-form label-width="72px">
        <el-form-item label="标题">
          <el-input v-model="form.title" maxlength="64" show-word-limit placeholder="如：小说续写" />
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="form.category" maxlength="32" placeholder="如：写作 / 人像 / 空镜（可空）" />
        </el-form-item>
        <el-form-item label="用途">
          <el-radio-group v-model="form.kind">
            <el-radio-button value="chat">写作/对话</el-radio-button>
            <el-radio-button value="image">绘画</el-radio-button>
            <el-radio-button value="video">视频</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="内容">
          <el-input v-model="form.content" type="textarea" :rows="8" maxlength="8000" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus, EditPen, Delete } from '@element-plus/icons-vue';
import { aiPromptApi, type AiPrompt, type AiPromptKind } from '@/api/aiPrompt';

const props = defineProps<{
  modelValue: boolean;
  /** 当前用途（打开时默认选中） */
  kind?: AiPromptKind;
  /** 待保存的草稿内容（父组件当前输入） */
  draft?: string;
}>();
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void;
  (e: 'pick', content: string): void;
}>();

const kind = ref<AiPromptKind>(props.kind || 'chat');
const keyword = ref('');
const category = ref('');
const loading = ref(false);
const saving = ref(false);
const list = ref<AiPrompt[]>([]);
const categories = ref<string[]>([]);

const filtered = computed(() =>
  category.value ? list.value.filter((p) => p.category === category.value) : list.value,
);

async function load() {
  loading.value = true;
  try {
    const res = await aiPromptApi.list({ kind: kind.value });
    if (res.data.code === 0) {
      list.value = res.data.data.list || [];
      categories.value = res.data.data.categories || [];
      if (keyword.value.trim()) {
        const kw = keyword.value.trim().toLowerCase();
        list.value = list.value.filter(
          (p) =>
            p.title.toLowerCase().includes(kw) ||
            p.content.toLowerCase().includes(kw) ||
            (p.category || '').toLowerCase().includes(kw),
        );
      }
    }
  } catch { /* ignore */ }
  loading.value = false;
}

function pickCategory(c: string) {
  category.value = c;
}

async function usePrompt(p: AiPrompt) {
  emit('pick', p.content);
  emit('update:modelValue', false);
  try { await aiPromptApi.use(p.id); } catch { /* 计数失败不影响使用 */ }
}

// ---------- 新建 / 编辑 ----------
const editVisible = ref(false);
const editing = ref<AiPrompt | null>(null);
const form = ref({ title: '', category: '', kind: 'chat' as AiPromptKind, content: '' });

function openCreate() {
  editing.value = null;
  form.value = {
    title: '',
    category: category.value || '',
    kind: kind.value,
    content: (props.draft || '').slice(0, 8000),
  };
  editVisible.value = true;
}

function editPrompt(p: AiPrompt) {
  editing.value = p;
  form.value = { title: p.title, category: p.category || '', kind: p.kind, content: p.content };
  editVisible.value = true;
}

async function save() {
  if (!form.value.title.trim() || !form.value.content.trim()) {
    ElMessage.warning('标题与内容不能为空');
    return;
  }
  saving.value = true;
  try {
    const payload = {
      kind: form.value.kind,
      category: form.value.category.trim(),
      title: form.value.title.trim(),
      content: form.value.content,
    };
    const res = editing.value
      ? await aiPromptApi.update(editing.value.id, payload)
      : await aiPromptApi.create(payload);
    if (res.data.code === 0) {
      ElMessage.success('已保存');
      editVisible.value = false;
      await load();
    } else {
      ElMessage.error((res.data as any).message || '保存失败');
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

async function removePrompt(p: AiPrompt) {
  try {
    await ElMessageBox.confirm(`确定删除提示词「${p.title}」？`, '删除确认', { type: 'warning' });
  } catch { return; }
  try {
    const res = await aiPromptApi.remove(p.id);
    if (res.data.code === 0) {
      ElMessage.success('已删除');
      await load();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

watch(
  () => [props.modelValue, props.kind],
  ([open, k]) => {
    if (open) {
      if (k && k !== kind.value) kind.value = k;
      load();
    }
  },
  { immediate: true },
);
</script>

<style scoped>
.pl-wrap { display: flex; flex-direction: column; height: 100%; gap: 10px; }
.pl-toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.pl-cats { display: flex; gap: 6px; flex-wrap: wrap; }
.pl-cat {
  font-size: 0.75rem; padding: 2px 10px; border-radius: 999px; cursor: pointer;
  background: var(--el-fill-color-light, #f5f7fa); color: var(--text-light, #666);
}
.pl-cat.active { background: var(--el-color-primary, #409eff); color: #fff; }
.pl-list { flex: 1; overflow-y: auto; min-height: 120px; display: flex; flex-direction: column; gap: 8px; }
.pl-empty { text-align: center; color: var(--text-light, #999); font-size: 0.82rem; padding: 30px 0; }
.pl-card {
  border: 1px solid var(--el-border-color-lighter, #ebeef5); border-radius: 10px;
  padding: 10px 12px; cursor: pointer; transition: border-color 0.15s, box-shadow 0.15s;
}
.pl-card:hover { border-color: var(--el-color-primary, #409eff); box-shadow: 0 2px 8px rgba(64, 158, 255, 0.12); }
.pl-card-head { display: flex; align-items: center; gap: 6px; }
.pl-card-title { font-size: 0.86rem; font-weight: 600; color: var(--text-color, #333); }
.pl-tag {
  font-size: 0.65rem; padding: 0 5px; border-radius: 4px;
  background: rgba(64, 158, 255, 0.12); color: #3b82f6;
}
.pl-tag-own { background: rgba(22, 163, 74, 0.12); color: #16a34a; }
.pl-cat-name { font-size: 0.68rem; color: var(--text-light, #999); }
.pl-card-tools { margin-left: auto; display: none; gap: 8px; color: var(--text-light, #888); }
.pl-card:hover .pl-card-tools { display: inline-flex; }
.pl-del { color: #ef4444; }
.pl-card-body {
  margin-top: 5px; font-size: 0.78rem; color: var(--text-light, #666);
  line-height: 1.6; max-height: 62px; overflow: hidden;
  display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical;
  white-space: pre-wrap;
}
.pl-save { border-top: 1px dashed var(--el-border-color-lighter, #e5e7eb); padding-top: 10px; display: flex; align-items: center; gap: 10px; }
.pl-save-hint { font-size: 0.72rem; color: var(--text-light, #999); }
</style>
