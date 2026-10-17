<template>
  <div class="me-works">
    <h2 class="section-title">作品列表</h2>
    <div class="novel-list" v-if="novels.length > 0">
      <div v-for="novel in novels" :key="novel.id" class="novel-item" @click="$router.push(`/novel/${novel.id}`)">
        <div class="novel-cover-sm">
          <el-image v-if="novel.cover_url" :src="novel.cover_url" fit="cover" style="width:64px;height:85px;border-radius:4px">
            <template #error><div class="cover-ph">封</div></template>
          </el-image>
          <div v-else class="cover-ph">封</div>
        </div>
        <div class="novel-body">
          <h3>{{ novel.title }}</h3>
          <div class="novel-meta">
            <el-tag v-if="novel.categories && novel.categories.length" v-for="c in novel.categories" :key="c" size="small" style="margin-right:4px">{{ c }}</el-tag>
            <el-tag v-else size="small">{{ novel.category }}</el-tag>
            <span>{{ novel.total_words?.toLocaleString() || 0 }} 字</span>
            <span>{{ novel.total_chapters || 0 }} 章</span>
            <el-tag size="small" :type="novel.status === 'published' ? 'success' : 'info'">{{ novel.status === 'published' ? '已发布' : '草稿' }}</el-tag>
            <el-tag v-if="novel.source_type === 'reprint'" size="small" type="warning">转载</el-tag>
            <el-tag v-else-if="novel.source_type === 'original'" size="small">原创</el-tag>
            <el-tag v-if="novel.creation_method === 'ai'" size="small" type="info">AI创作</el-tag>
            <el-tag v-else-if="novel.creation_method === 'human_ai_assisted'" size="small" type="info">AI辅助</el-tag>
          </div>
          <p class="novel-summary-line">{{ novel.summary?.slice(0, 120) || '暂无简介' }}</p>
        </div>
      </div>
    </div>
    <el-empty v-else description="暂无作品" />
  </div>
</template>

<script setup lang="ts">
import { computed, inject, type Ref } from 'vue';

const profile = inject<Ref<any>>('meProfile');
const novels = computed(() => profile?.value?.novels || []);
</script>

<style scoped>
.section-title { font-size: 1.15rem; margin-bottom: 12px; color: var(--text-color); }
.novel-item {
  display: flex;
  gap: 16px;
  padding: 16px;
  background: #fff;
  border-radius: 8px;
  margin-bottom: 8px;
  cursor: pointer;
  transition: background 0.2s;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);
}
[data-theme="dark"] .novel-item { background: #1e293b; box-shadow: 0 1px 4px rgba(0, 0, 0, 0.25); }
.novel-item:hover { background: #f5f7fa; }
[data-theme="dark"] .novel-item:hover { background: #334155; }
.novel-body { flex: 1; }
.novel-body h3 { font-size: 1.05rem; margin-bottom: 6px; }
.novel-meta {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
  font-size: 0.8rem;
  color: var(--text-light);
  margin-bottom: 4px;
}
.novel-summary-line {
  font-size: 0.85rem;
  color: #888;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
[data-theme="dark"] .novel-summary-line { color: #64748b; }
.cover-ph {
  width: 64px;
  height: 85px;
  background: #e2e8f0;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #94a3b8;
  font-size: 1.2rem;
}
[data-theme="dark"] .cover-ph { background: #334155; color: #64748b; }
</style>
