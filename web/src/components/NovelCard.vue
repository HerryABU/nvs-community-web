<template>
  <el-card class="novel-card" shadow="hover" @click="$emit('click')">
    <div class="card-cover">
      <el-image
        v-if="novel.cover_url"
        :src="novel.cover_url"
        fit="cover"
        style="width: 100%; height: 180px; border-radius: 6px 6px 0 0"
      >
        <template #error>
          <div class="cover-placeholder">暂无封面</div>
        </template>
      </el-image>
      <div v-else class="cover-placeholder">暂无封面</div>
    </div>
    <div class="card-body">
      <h3 class="card-title">{{ novel.title }}</h3>
      <div class="card-meta">
        <el-tag v-for="cat in displayCategories" :key="cat" size="small" type="warning" style="margin-right:4px">
          {{ cat }}
        </el-tag>
        <span class="word-count">{{ (novel.total_words || 0).toLocaleString() }} 字</span>
      </div>
      <p class="card-summary">{{ truncate(novel.summary, 80) }}</p>
      <div class="card-footer">
        <span class="author-name">{{ (novel as any).author?.nickname || (novel as any).author?.username || '未知作者' }}</span>
        <span class="chapter-count">{{ novel.total_chapters || 0 }} 章</span>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { Novel } from '@/api/novel';

const props = defineProps<{ novel: Novel }>();
defineEmits<{ click: [] }>();

const displayCategories = computed(() => {
  return props.novel.categories?.length ? props.novel.categories : (props.novel.category ? [props.novel.category] : ['其他']);
});

function truncate(text: string | undefined, maxLen: number): string {
  if (!text) return '暂无简介';
  return text.length > maxLen ? text.slice(0, maxLen) + '...' : text;
}
</script>

<style scoped>
/* ====== NovelCard — 2024 Modern Card ====== */
.novel-card {
  cursor: pointer;
  overflow: hidden;
  border-radius: 16px !important;
  border: 1px solid var(--border-color) !important;
  box-shadow: var(--shadow-sm) !important;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1) !important;
  background: #fff;
}

.novel-card:hover {
  transform: translateY(-6px);
  box-shadow: var(--shadow-lg) !important;
  border-color: rgba(99, 102, 241, 0.15) !important;
}

.novel-card:hover::after {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  border-radius: 16px;
  background: linear-gradient(
    135deg,
    rgba(99, 102, 241, 0.06) 0%,
    rgba(139, 92, 246, 0.04) 50%,
    transparent 100%
  );
  pointer-events: none;
  z-index: 1;
}

.novel-card :deep(.el-card__body) {
  padding: 0;
  position: relative;
}

/* Cover */
.cover-placeholder {
  width: 100%;
  height: 190px;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 40%, #a78bfa 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.75);
  font-size: 1rem;
  font-weight: 300;
  letter-spacing: 0.05em;
  border-radius: 16px 16px 0 0;
}

.novel-card :deep(.el-image) {
  border-radius: 16px 16px 0 0 !important;
}

.novel-card :deep(.el-image img) {
  border-radius: 16px 16px 0 0 !important;
}

/* Card body */
.card-body {
  padding: 16px 18px;
  position: relative;
  z-index: 2;
}

.card-title {
  font-size: 1.02rem;
  font-weight: 500;
  color: var(--text-color);
  margin-bottom: 10px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  letter-spacing: -0.01em;
}

.card-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.word-count {
  font-size: 0.78rem;
  color: var(--text-light);
  font-weight: 400;
}

.card-summary {
  font-size: 0.82rem;
  color: var(--text-light);
  line-height: 1.6;
  margin-bottom: 12px;
  min-height: 2.6em;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-footer {
  display: flex;
  justify-content: space-between;
  font-size: 0.78rem;
  color: var(--text-muted);
  padding-top: 10px;
  border-top: 1px solid var(--border-color);
}

.author-name {
  font-weight: 400;
}

.chapter-count {
  font-weight: 400;
}
</style>

<style>
/* ====== NovelCard Dark Mode global (unscoped) ====== */
[data-theme="dark"] .novel-card {
  background: #1a1d27 !important;
  border-color: rgba(255, 255, 255, 0.06) !important;
  box-shadow: var(--shadow-sm) !important;
}

[data-theme="dark"] .novel-card:hover {
  box-shadow: var(--shadow-lg) !important;
  border-color: rgba(99, 102, 241, 0.2) !important;
}

[data-theme="dark"] .novel-card:hover::after {
  background: linear-gradient(
    135deg,
    rgba(99, 102, 241, 0.08) 0%,
    rgba(139, 92, 246, 0.05) 50%,
    transparent 100%
  );
}

[data-theme="dark"] .novel-card .cover-placeholder {
  background: linear-gradient(135deg, #1e293b 0%, #334155 50%, #475569 100%);
  color: rgba(255, 255, 255, 0.45);
}

[data-theme="dark"] .novel-card .card-body {
  background: transparent;
}

[data-theme="dark"] .novel-card .card-title {
  color: #e2e8f0;
}

[data-theme="dark"] .novel-card .card-summary {
  color: #94a3b8;
}

[data-theme="dark"] .novel-card .card-footer {
  border-top-color: rgba(255, 255, 255, 0.06);
  color: #64748b;
}

[data-theme="dark"] .novel-card .word-count {
  color: #64748b;
}
</style>
