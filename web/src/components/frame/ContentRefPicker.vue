<template>
  <el-dialog
    :model-value="visible"
    title="插入站内链接"
    width="520px"
    destroy-on-close
    @update:model-value="emit('update:visible', $event)"
  >
    <el-input
      v-model="query"
      placeholder="搜索小说 / 博客 / 论坛帖标题"
      clearable
      :prefix-icon="Search"
      @input="onSearch"
    />
    <div v-loading="loading" class="ref-list">
      <el-empty v-if="!loading && results.length === 0" description="没有匹配的内容" :image-size="60" />
      <div
        v-for="r in results"
        :key="r.type + '-' + r.id"
        class="ref-item"
        @click="pick(r)"
      >
        <el-tag size="small" :type="typeTag(r.type)">{{ typeLabel(r.type) }}</el-tag>
        <span class="ref-title" :title="r.title">{{ r.title }}</span>
        <span class="ref-link">/{{ r.type }}/{{ r.id }}</span>
      </div>
    </div>
    <template #footer>
      <el-button @click="emit('update:visible', false)">取消</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { Search } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import api from '@/api/index';

const props = defineProps<{ visible: boolean }>();
const emit = defineEmits<{
  (e: 'update:visible', v: boolean): void;
  (e: 'confirm', link: string): void;
}>();

interface ContentRef {
  type: string;
  id: number;
  title: string;
}

const query = ref('');
const loading = ref(false);
const results = ref<ContentRef[]>([]);
let timer: any = null;

function onSearch() {
  if (timer) clearTimeout(timer);
  timer = setTimeout(search, 250);
}

async function search() {
  loading.value = true;
  try {
    const r = await api.get('/frame/refs', { params: { q: query.value } });
    results.value = r.data.code === 0 ? (r.data.data || []) : [];
  } catch {
    results.value = [];
    ElMessage.error('检索失败');
  } finally {
    loading.value = false;
  }
}

function typeLabel(t: string): string {
  return ({ novel: '小说', blog: '博客', thread: '论坛帖' } as Record<string, string>)[t] || t;
}
function typeTag(t: string): any {
  return ({ novel: 'primary', blog: 'success', thread: 'warning' } as Record<string, any>)[t] || 'info';
}

// 仅生成相对链接（与站点前缀无关，挂载在任意 base href 下都能正确跳转），
// 绝不插入任何脚本/JS。论坛帖实际路由为 /thread/:id。
function relLink(r: ContentRef): string {
  if (r.type === 'blog') return `/blog/${r.id}`;
  if (r.type === 'thread') return `/thread/${r.id}`;
  return `/novel/${r.id}`;
}

function pick(r: ContentRef) {
  emit('confirm', relLink(r));
  emit('update:visible', false);
}
</script>

<style scoped>
.ref-list {
  max-height: 360px;
  overflow: auto;
  margin-top: 12px;
}
.ref-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
}
.ref-item:hover {
  background: rgba(0, 0, 0, 0.04);
}
.ref-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ref-link {
  font-family: monospace;
  font-size: 0.78rem;
  color: var(--text-secondary);
}
[data-theme="dark"] .ref-item:hover {
  background: rgba(255, 255, 255, 0.06);
}
</style>
