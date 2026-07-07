<template>
  <div class="page-container author-home" v-loading="loading">
    <template v-if="author">
      <!-- 作者信息头 -->
      <div class="author-header">
        <div class="author-avatar-wrap">
          <el-avatar :size="80" :src="author.avatar_url || ''">
            {{ author.nickname?.[0] || author.username?.[0] || 'A' }}
          </el-avatar>
        </div>
        <div class="author-info">
          <h1>{{ author.nickname || author.username }}</h1>
          <p class="author-bio" v-if="author.bio">{{ author.bio }}</p>
          <div class="author-stats">
            <span>作品：{{ data.total_novels || 0 }}</span>
            <span>总字数：{{ (data.total_words || 0).toLocaleString() }}</span>
            <span>总章节：{{ data.total_chapters || 0 }}</span>
            <el-tag v-if="author.role === 'vip_author'" type="warning" size="small">VIP作者</el-tag>
            <el-tag v-else-if="author.role === 'author'" size="small">作者</el-tag>
          </div>
          <!-- 进入作者大论坛 -->
          <el-button type="primary" text size="small" @click="goForum" style="margin-top:8px">
            进入 {{ author.nickname || author.username }} 的讨论区
          </el-button>
        </div>
      </div>

      <!-- 作品列表 -->
      <h2 class="section-title">作品列表</h2>
      <div class="novel-list" v-if="data.novels && data.novels.length > 0">
        <div v-for="novel in data.novels" :key="novel.id" class="novel-item" @click="$router.push(`/novel/${novel.id}`)">
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
            </div>
            <p class="novel-summary-line">{{ novel.summary?.slice(0, 120) || '暂无简介' }}</p>
          </div>
        </div>
      </div>
      <el-empty v-else description="暂无作品" />

      <!-- 作者评论区 -->
      <h2 class="section-title" style="margin-top:32px">读者留言</h2>
      <!-- 评论区：使用第一个公开作品的 ID -->
      <CommentSection :novel-id="data.novels && data.novels.length > 0 ? data.novels[0].id : 0" :novel-category="''" />
    </template>
    <el-empty v-else-if="!loading" description="作者不存在" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { publicApi } from '@/api/admin';
import { novelApi } from '@/api/novel';
import CommentSection from '@/components/CommentSection.vue';

const route = useRoute();
const router = useRouter();
const authorId = ref(Number(route.params.id));
const loading = ref(false);
const author = ref<any>(null);
const data = ref<any>({});

async function load() {
  loading.value = true;
  try {
    const res = await publicApi.getAuthorProfile(authorId.value);
    if (res.data.code === 0) {
      const d = res.data.data;
      author.value = d.author;
      data.value = d;
    }
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

async function goForum() {
  try {
    const res = await publicApi.getAuthorForum(authorId.value);
    const forumId = res.data.data?.forum?.id;
    if (forumId) router.push(`/forum/${forumId}`);
  } catch {
    // ignore
  }
}

onMounted(load);
</script>

<style scoped>
.author-header {
  display: flex;
  gap: 24px;
  align-items: flex-start;
  margin-bottom: 32px;
  padding: 24px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 1px 6px rgba(0,0,0,0.06);
}

[data-theme="dark"] .author-header {
  background: #1e293b;
  box-shadow: 0 1px 6px rgba(0,0,0,0.25);
}

.author-info h1 {
  font-size: 1.5rem;
  color: var(--primary-color);
  margin-bottom: 6px;
}

.author-bio {
  color: #666;
  margin-bottom: 10px;
  max-width: 500px;
}

[data-theme="dark"] .author-bio {
  color: #94a3b8;
}

.author-stats {
  display: flex;
  gap: 16px;
  font-size: 0.85rem;
  color: var(--text-light);
  flex-wrap: wrap;
  align-items: center;
}

.novel-item {
  display: flex;
  gap: 16px;
  padding: 16px;
  background: #fff;
  border-radius: 8px;
  margin-bottom: 8px;
  cursor: pointer;
  transition: background 0.2s;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}

[data-theme="dark"] .novel-item {
  background: #1e293b;
  box-shadow: 0 1px 4px rgba(0,0,0,0.25);
}

.novel-item:hover {
  background: #f5f7fa;
}

[data-theme="dark"] .novel-item:hover {
  background: #334155;
}

.novel-body {
  flex: 1;
}

.novel-body h3 {
  font-size: 1.05rem;
  margin-bottom: 6px;
}

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

[data-theme="dark"] .novel-summary-line {
  color: #64748b;
}

.cover-ph {
  width: 64px;
  height: 85px;
  border-radius: 4px;
  background: #e0e0e0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #999;
  font-size: 0.75rem;
}

[data-theme="dark"] .cover-ph {
  background: #334155;
  color: #64748b;
}
</style>
