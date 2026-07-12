<template>
  <div class="page-container novel-detail" v-loading="loading">
    <template v-if="novel">
      <!-- 作品信息头 -->
      <div class="novel-header">
        <div class="novel-cover">
          <el-image
            v-if="novel.cover_url"
            :src="novel.cover_url"
            fit="cover"
            style="width: 180px; height: 240px; border-radius: 8px"
          >
            <template #error>
              <div class="cover-placeholder">暂无封面</div>
            </template>
          </el-image>
          <div v-else class="cover-placeholder">暂无封面</div>
        </div>
        <div class="novel-info">
          <h1 class="novel-title">{{ novel.title }}</h1>
          <div class="novel-meta">
            <el-tag size="small" type="warning">{{ novel.category }}</el-tag>
            <span class="meta-item">作者：<router-link :to="`/author/${novel.author_id}`" class="author-link">{{ novel.author_name || '未知' }}</router-link></span>
            <span class="meta-item">{{ novel.total_words?.toLocaleString() || 0 }} 字</span>
            <span class="meta-item">{{ novel.total_chapters || 0 }} 章</span>
            <span class="meta-item">更新：{{ formatDate(novel.updated_at) }}</span>
          </div>
          <div class="novel-summary">
            <h3>作品简介</h3>
            <p>{{ novel.summary || '暂无简介' }}</p>
          </div>
          <div class="novel-actions">
            <el-button type="primary" @click="startReading">开始阅读</el-button>
            <el-button @click="toggleShelf" :loading="shelfLoading">
              {{ onShelf ? '已加入书架' : '加入书架' }}
            </el-button>
          </div>
        </div>
      </div>

      <!-- 章节列表 -->
      <h2 class="section-title">目录（共 {{ chapters.length }} 章）</h2>
      <div class="chapter-list" v-if="chapters.length > 0">
        <div
          v-for="ch in chapters"
          :key="ch.chapter_number"
          class="chapter-item"
          @click="$router.push(`/novel/${novel.id}/read/${ch.chapter_number}`)"
        >
          <span class="chapter-num">第{{ ch.chapter_number }}章</span>
          <span class="chapter-title">{{ ch.title }}</span>
          <span class="chapter-words">{{ ch.word_count?.toLocaleString() }}字</span>
          <span class="chapter-date">{{ formatDate(ch.created_at) }}</span>
        </div>
      </div>
      <el-empty v-else description="暂无章节" />

      <!-- 评分 -->
      <h2 class="section-title" style="margin-top: 40px">作品评分</h2>
      <StarRating :novel-id="novel.id" :disabled="!authStore.isLoggedIn" />

      <!-- 子论坛 -->
      <div style="margin-top: 24px">
        <el-button @click="goToForum">进入作品讨论区</el-button>
      </div>

      <!-- 评论区 -->
      <h2 class="section-title" style="margin-top: 40px">评论</h2>
      <CommentSection :novel-id="novel.id" :novel-category="novel.category" />
    </template>
    <el-empty v-else-if="!loading" description="作品不存在" />

    <!-- 敏感分区确认弹窗 -->
    <SensitiveZoneGuard
      :visible="showZoneGuard"
      :zone-name="zoneGuardName"
      :is-cross-domain="zoneGuardCross"
      @confirm="onZoneConfirmed"
      @cancel="onZoneCancelled"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { novelApi, type Novel, type Chapter } from '@/api/novel';
import { bookshelfApi } from '@/api/bookshelf';
import CommentSection from '@/components/CommentSection.vue';
import StarRating from '@/components/StarRating.vue';
import SensitiveZoneGuard from '@/components/SensitiveZoneGuard.vue';
import { useAuthStore } from '@/stores/auth';
import { shouldShowGuard, markZoneConfirmed, setLastZone, recordZoneVisit } from '@/utils/sensitiveZone';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const loading = ref(false);
const novel = ref<Novel | null>(null);
const chapters = ref<Chapter[]>([]);
const onShelf = ref(false);
const shelfLoading = ref(false);

// 敏感分区确认
const showZoneGuard = ref(false);
const zoneGuardName = ref('');
const zoneGuardCross = ref(false);

function formatDate(dateStr?: string) {
  if (!dateStr) return '';
  return new Date(dateStr).toLocaleDateString('zh-CN');
}

function startReading() {
  if (chapters.value.length > 0) {
    router.push(`/novel/${novel.value!.id}/read/${chapters.value[0].chapter_number}`);
  }
}

async function toggleShelf() {
  if (!authStore.isLoggedIn) {
    router.push('/login');
    return;
  }
  if (!novel.value || shelfLoading.value) return;
  shelfLoading.value = true;
  try {
    if (onShelf.value) {
      await bookshelfApi.remove(novel.value.id);
      onShelf.value = false;
    } else {
      await bookshelfApi.add(novel.value.id);
      onShelf.value = true;
    }
  } catch {
    // ignore
  } finally {
    shelfLoading.value = false;
  }
}

async function goToForum() {
  if (!novel.value) return;
  try {
    const res = await novelApi.getNovelForum(novel.value.id);
    const forumId = res.data.data?.forum?.id;
    if (forumId) router.push(`/forum/${forumId}`);
  } catch {
    // 忽略错误
  }
}

function checkSensitiveZone(category: string) {
  const guard = shouldShowGuard(category);
  if (guard?.needed) {
    zoneGuardName.value = guard.zoneName;
    zoneGuardCross.value = guard.isCrossDomain;
    showZoneGuard.value = true;
  }
}

function onZoneConfirmed() {
  markZoneConfirmed(zoneGuardName.value);
  setLastZone(zoneGuardName.value);
  showZoneGuard.value = false;
}

function onZoneCancelled() {
  showZoneGuard.value = false;
  router.push('/');
}

onMounted(async () => {
  const id = Number(route.params.id);
  loading.value = true;
  try {
    const [novelRes, chaptersRes] = await Promise.all([
      novelApi.getNovel(id),
      novelApi.getChapters(id),
    ]);
    novel.value = novelRes.data.data;
    chapters.value = chaptersRes.data.data || [];

    // 检查书架状态
    if (authStore.isLoggedIn && novel.value) {
      try {
        const shelfRes = await bookshelfApi.check(novel.value.id);
        onShelf.value = shelfRes.data.data?.on_shelf || false;
      } catch { /* ignore */ }
    }

    // 内容确认机制：检查所有分类
    if (novel.value) {
      const cats = novel.value.categories?.length
        ? novel.value.categories
        : (novel.value.category ? [novel.value.category] : []);
      for (const cat of cats) {
        recordZoneVisit(cat);
        const guard = shouldShowGuard(cat, {
          authorId: novel.value.author_id,
          userId: authStore.user?.id,
        });
        if (guard?.needed) {
          zoneGuardName.value = guard.zoneName;
          zoneGuardCross.value = guard.isCrossDomain;
          showZoneGuard.value = true;
          break; // 一次只弹一个
        }
      }
    }
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.novel-header {
  display: flex;
  gap: 32px;
  margin-bottom: 40px;
}

.cover-placeholder {
  width: 180px;
  height: 240px;
  border-radius: 8px;
  background: #e0e0e0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #999;
  font-size: 0.9rem;
}

.novel-info {
  flex: 1;
}

.novel-title {
  font-size: 1.8rem;
  color: var(--primary-color);
  margin-bottom: 12px;
}

.novel-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 16px;
  color: var(--text-light);
  font-size: 0.9rem;
}

.novel-summary {
  margin-bottom: 20px;
  line-height: 1.8;
}

.novel-summary h3 {
  font-size: 1rem;
  margin-bottom: 8px;
  color: var(--primary-color);
}

.novel-summary p {
  color: #555;
  white-space: pre-wrap;
}

.novel-actions {
  display: flex;
  gap: 12px;
}

.chapter-list {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
}

.chapter-item {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background 0.2s;
}

.chapter-item:hover {
  background: #f5f7fa;
}

.chapter-item:last-child {
  border-bottom: none;
}

.chapter-num {
  color: var(--text-light);
  min-width: 80px;
}

.chapter-title {
  flex: 1;
  font-weight: 500;
}

.chapter-words,
.chapter-date {
  color: var(--text-light);
  font-size: 0.85rem;
  min-width: 80px;
  text-align: right;
}

.author-link {
  color: var(--primary-color);
  text-decoration: none;
  font-weight: 500;
}
.author-link:hover {
  text-decoration: underline;
}

@media (max-width: 768px) {
  .novel-header {
    flex-direction: column;
  }
}
</style>

<style>
[data-theme="dark"] .novel-detail {
  background: transparent;
}
[data-theme="dark"] .cover-placeholder {
  background: #334155;
  color: #94a3b8;
}
[data-theme="dark"] .novel-title {
  color: #e2e8f0;
}
[data-theme="dark"] .novel-meta {
  color: #94a3b8;
}
[data-theme="dark"] .novel-summary h3 {
  color: #e2e8f0;
}
[data-theme="dark"] .novel-summary p {
  color: #94a3b8;
}
[data-theme="dark"] .chapter-list {
  border-color: #1e293b;
}
[data-theme="dark"] .chapter-item {
  border-bottom-color: rgba(255, 255, 255, 0.06);
  color: #e2e8f0;
}
[data-theme="dark"] .chapter-item:hover {
  background: #1e293b;
}
[data-theme="dark"] .chapter-num {
  color: #94a3b8;
}
[data-theme="dark"] .chapter-title {
  color: #e2e8f0;
}
[data-theme="dark"] .chapter-words,
[data-theme="dark"] .chapter-date {
  color: #94a3b8;
}
</style>