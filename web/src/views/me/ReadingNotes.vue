<template>
  <div class="reading-page">
    <div class="reading-header">
      <h2>📚 我的精读</h2>
      <div class="reading-filters">
        <el-radio-group v-model="filter" size="small" @change="load">
          <el-radio-button label="all">全部</el-radio-button>
          <el-radio-button label="bookmark">书签</el-radio-button>
          <el-radio-button label="annotation">批注</el-radio-button>
        </el-radio-group>
      </div>
    </div>

    <el-empty v-if="!loading && !list.length" description="还没有精读记录，去阅读时点「书签」或选中文字「批注」吧" />

    <div v-else class="reading-list">
      <div v-for="n in filteredList" :key="n.id" class="reading-item" @click="openNote(n)">
        <div class="reading-icon">{{ n.type === 'bookmark' ? '🔖' : '📝' }}</div>
        <div class="reading-body">
          <div class="reading-title">
            <span class="reading-type" :class="n.type">{{ n.type === 'bookmark' ? '书签' : '批注' }}</span>
            <span class="reading-vis" :class="n.visibility">{{ n.visibility === 'public' ? '公开' : '私密' }}</span>
            <span class="reading-novel">《{{ n.novel_title || '未知作品' }}》</span>
            <span v-if="n.chapter_number" class="reading-chapter">第{{ n.chapter_number }}章{{ n.chapter_title ? ' ' + n.chapter_title : '' }}</span>
          </div>
          <div v-if="n.selected_text" class="reading-quote">{{ n.selected_text }}</div>
          <div v-if="n.content" class="reading-content">{{ n.content }}</div>
          <div class="reading-meta">{{ formatTime(n.created_at) }}</div>
        </div>
        <el-button v-if="n.visibility === 'public'" size="small" text type="warning" @click.stop="setPrivate(n)">转私密</el-button>
        <el-button size="small" text type="danger" @click.stop="removeNote(n)">删除</el-button>
      </div>
    </div>

    <div v-if="total > list.length" class="reading-more">
      <el-button size="small" text :loading="loadingMore" @click="loadMore">加载更多（{{ list.length }}/{{ total }}）</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { readingApi, type ReadingNote } from '@/api/reading';
import { ElMessage, ElMessageBox } from 'element-plus';

const router = useRouter();
const list = ref<ReadingNote[]>([]);
const total = ref(0);
const page = ref(1);
const loading = ref(false);
const loadingMore = ref(false);
const filter = ref<'all' | 'bookmark' | 'annotation'>('all');

const filteredList = computed(() =>
  filter.value === 'all' ? list.value : list.value.filter((n) => n.type === filter.value)
);

function formatTime(iso: string) {
  if (!iso) return '';
  const d = new Date(iso);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

async function load(append = false) {
  if (append) loadingMore.value = true;
  else loading.value = true;
  try {
    const res = await readingApi.listAllMine(append ? page.value + 1 : 1, 20);
    if (res.data.code === 0) {
      const d = res.data.data;
      if (append) { list.value.push(...(d.list || [])); page.value += 1; }
      else { list.value = d.list || []; page.value = 1; }
      total.value = Number(d.total) || 0;
    }
  } catch { ElMessage.error('加载精读失败'); }
  finally { loading.value = false; loadingMore.value = false; }
}

function loadMore() { load(true); }

function openNote(n: ReadingNote) {
  router.push(`/author/${n.user_id}/novel/${n.novel_id}/read/${n.chapter_number || 1}`);
}

async function setPrivate(n: ReadingNote) {
  try {
    await readingApi.update(n.id, { visibility: 'private' });
    n.visibility = 'private';
    ElMessage.success('已转为私密');
  } catch { ElMessage.error('操作失败'); }
}

async function removeNote(n: ReadingNote) {
  try {
    await ElMessageBox.confirm('确定删除这条精读记录？', '确认删除', { type: 'warning' });
    await readingApi.remove(n.id);
    list.value = list.value.filter((x) => x.id !== n.id);
    total.value = Math.max(0, total.value - 1);
    ElMessage.success('已删除');
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

onMounted(() => load());
</script>

<style scoped>
.reading-page { max-width: 760px; margin: 0 auto; padding: 90px 20px 40px; min-height: 70vh; }
.reading-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 18px; }
.reading-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.reading-list { display: flex; flex-direction: column; gap: 10px; }
.reading-item {
  display: flex; gap: 12px; padding: 14px 16px; border-radius: 10px;
  background: var(--bg-card, #fff); border: 1px solid var(--glass-border, #e5e7eb);
  cursor: pointer; transition: all 0.2s ease; align-items: flex-start;
}
.reading-item:hover { border-color: var(--primary-color, #6366f1); transform: translateY(-1px); }
.reading-icon { font-size: 1.3rem; line-height: 1.6; }
.reading-body { flex: 1; min-width: 0; }
.reading-title { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 0.95rem; }
.reading-type { font-size: 0.7rem; padding: 1px 8px; border-radius: 8px; background: rgba(99,102,241,0.12); color: var(--primary-color, #6366f1); }
.reading-type.annotation { background: rgba(245,158,11,0.15); color: #d97706; }
.reading-vis { font-size: 0.7rem; padding: 1px 8px; border-radius: 8px; }
.reading-vis.public { background: rgba(16,185,129,0.12); color: #059669; }
.reading-vis.private { background: rgba(148,163,184,0.15); color: #64748b; }
.reading-novel { font-weight: 600; color: var(--text-color, #333); }
.reading-chapter { font-size: 0.8rem; color: var(--text-light, #888); }
.reading-quote {
  margin-top: 8px; font-size: 0.82rem; color: #92610a; background: rgba(245,158,11,0.08);
  border-left: 3px solid #f59e0b; padding: 6px 10px; border-radius: 0 4px 4px 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.reading-content { margin-top: 6px; font-size: 0.85rem; color: var(--text-light, #666); white-space: pre-wrap; word-break: break-all; }
.reading-meta { margin-top: 6px; font-size: 0.75rem; color: #aaa; }
.reading-more { text-align: center; margin-top: 16px; }
[data-theme="dark"] .reading-item { background: #1a1f2e; border-color: rgba(255,255,255,0.08); }
[data-theme="dark"] .reading-quote { background: rgba(245,158,11,0.1); }

/* ====== 移动端精读笔记适配 ====== */
@media (max-width: 640px) {
  .reading-page {
    padding: 76px 12px 32px;
  }
  .reading-header h2 {
    font-size: 1.1rem;
  }
  .reading-item {
    padding: 12px;
    gap: 10px;
  }
  .reading-quote {
    white-space: normal;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    word-break: break-all;
  }
}
</style>
