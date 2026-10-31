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
            <el-tag v-for="c in extraCategories" :key="c" size="small" effect="plain">{{ c }}</el-tag>
            <el-tag size="small" :type="novel.status === 'completed' ? 'success' : 'info'" effect="plain">
              {{ novel.status === 'completed' ? '已完结' : novel.status === 'published' ? '连载中' : '未发布' }}
            </el-tag>
            <el-tag v-if="novel.mode === 'script'" size="small" type="info" effect="plain">话本体</el-tag>
            <el-tag v-else-if="novel.mode === 'mixed'" size="small" type="info" effect="plain">散文 / 话本混排</el-tag>
            <el-tag v-if="novel.source_type === 'reprint'" size="small" type="warning" effect="plain">转载</el-tag>
            <el-tag v-if="novel.creation_method === 'ai'" size="small" effect="plain">AI 创作</el-tag>
            <el-tag v-else-if="novel.creation_method === 'human_ai_assisted'" size="small" effect="plain">AI 辅助</el-tag>
            <span class="meta-item">作者：<router-link :to="`/author/${novel.author_id}`" class="author-link">{{ novel.author_name || novel.author?.nickname || novel.author?.username || '未知' }}</router-link></span>
            <span class="meta-item">{{ novel.total_words?.toLocaleString() || 0 }} 字</span>
            <span class="meta-item">{{ novel.total_chapters || 0 }} 章</span>
            <span class="meta-item">更新：{{ formatDate(novel.updated_at) }}</span>
            <span class="meta-item" v-if="novel.price_per_chapter > 0">约 {{ novel.price_per_chapter }} 代币/章</span>
          </div>
          <div class="novel-tags" v-if="novel.tags && novel.tags.length">
            <span class="tags-label">标签</span>
            <router-link
              v-for="t in novel.tags"
              :key="t"
              :to="`/feed?tag=${encodeURIComponent(t)}`"
              class="novel-tag-item"
            >#{{ t }}</router-link>
          </div>
          <div class="novel-summary">
            <h3>作品简介</h3>
            <p>{{ novel.summary || '暂无简介' }}</p>
          </div>
          <div class="novel-actions">
            <el-button type="primary" @click="startReading">
              {{ lastReadChapter ? `继续阅读 第${lastReadChapter}章` : '开始阅读' }}
            </el-button>
            <el-button @click="toggleShelf" :loading="shelfLoading">
              {{ onShelf ? '已加入书架' : '加入书架' }}
            </el-button>
            <ReportButton
              v-if="authStore.isLoggedIn && authStore.user?.id !== novel.author_id"
              target-type="novel"
              :target-id="novel.id"
            />
          </div>
        </div>
      </div>

      <!-- 目录工具条：搜索 / 排序 / 计数（章节多时不用一路滚） -->
      <div class="chapter-toolbar">
        <h2 class="section-title">目录<span class="ct-count">共 {{ chapters.length }} 章 · {{ (novel.total_words || 0).toLocaleString() }} 字</span></h2>
        <div class="ct-controls">
          <el-input
            v-model="chapterKeyword"
            placeholder="搜章节号 / 标题"
            clearable
            size="small"
            style="width: 200px"
            :prefix-icon="Search"
          />
          <el-radio-group v-model="chapterOrder" size="small">
            <el-radio-button value="asc">正序</el-radio-button>
            <el-radio-button value="desc">倒序</el-radio-button>
          </el-radio-group>
          <el-button
            v-if="lastReadChapter"
            size="small"
            text
            type="primary"
            @click="goChapter(lastReadChapter)"
          >跳到上次阅读</el-button>
        </div>
      </div>

      <div class="chapter-list" v-if="filteredChapters.length > 0">
        <template v-for="(ch, idx) in displayChapters" :key="ch.chapter_number">
          <!-- 折叠分隔线：在第5个后（即前5和后5之间）插入展开按钮 -->
          <div
            v-if="idx === 5 && filteredChapters.length > 10 && !showAllChapters"
            class="chapter-collapse-row"
            @click="showAllChapters = true"
          >
            <el-icon><ArrowDown /></el-icon>
            <span>展开全部章节（中间省略 {{ filteredChapters.length - 10 }} 章）</span>
          </div>
          <div
            class="chapter-item"
            :class="{ 'is-read': ch.chapter_number === lastReadChapter }"
            @click="goChapter(ch.chapter_number)"
          >
            <span class="chapter-num">第{{ ch.chapter_number }}章</span>
            <span class="chapter-title">{{ ch.title }}</span>
            <span class="chapter-words">{{ ch.word_count?.toLocaleString() }}字</span>
            <span class="chapter-date">{{ formatDate(ch.created_at) }}</span>
          </div>
        </template>
        <!-- 收起按钮：全部展开后显示在末尾 -->
        <div
          v-if="filteredChapters.length > 10 && showAllChapters"
          class="chapter-collapse-row"
          @click="showAllChapters = false"
        >
          <el-icon><ArrowUp /></el-icon>
          <span>收起目录，仅显示头尾章节</span>
        </div>
      </div>
      <div class="home-block-empty" v-else-if="chapters.length === 0">作者还没有上传章节</div>
      <div class="home-block-empty" v-else>没有匹配「{{ chapterKeyword }}」的章节</div>

      <!-- 评分 -->
      <h2 class="section-title" style="margin-top: 40px">作品评分</h2>
      <StarRating :novel-id="novel.id" :disabled="!authStore.isLoggedIn" />

      <!-- 子论坛 -->
      <div style="margin-top: 24px">
        <el-button @click="goToForum">进入作品讨论区</el-button>
      </div>

      <!-- 评论区 -->
      <h2 class="section-title" style="margin-top: 40px">评论</h2>
      <CommentSection :novel-id="novel.id" :novel-category="novel.category" :owner-id="novel.author_id" />
    </template>
    <el-empty v-else-if="!loading" description="作品不存在" />

    <!-- 敏感分区确认弹窗 -->
    <SensitiveZoneGuard
      :visible="showZoneGuard"
      :zone-name="zoneGuardName"
      :is-cross-domain="zoneGuardCross"
      :custom-warning="novel?.wall_warning"
      @confirm="onZoneConfirmed"
      @cancel="onZoneCancelled"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { novelApi, chapterApi, forumApi, type Novel, type Chapter } from '@/api/novel';
import { bookshelfApi } from '@/api/bookshelf';
import CommentSection from '@/components/CommentSection.vue';
import StarRating from '@/components/StarRating.vue';
import SensitiveZoneGuard from '@/components/SensitiveZoneGuard.vue';
import ReportButton from '@/components/ReportButton.vue';
import { ArrowDown, ArrowUp, Search } from '@element-plus/icons-vue';
import { useAuthStore } from '@/stores/auth';
import { shouldShowGuard, markZoneConfirmed, setLastZone } from '@/utils/sensitiveZone';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const loading = ref(false);
const novel = ref<Novel | null>(null);
const chapters = ref<Chapter[]>([]);
const onShelf = ref(false);
const shelfLoading = ref(false);

// 目录折叠：章节 > 10 时只展示头尾各5章，中间可展开
const COLLAPSE_THRESHOLD = 10;
const showAllChapters = ref(false);
// 目录搜索 / 排序（章节几百章时靠滚动找非常痛苦）
const chapterKeyword = ref('');
const chapterOrder = ref<'asc' | 'desc'>('asc');
// 上次阅读章节（Reader 写入 reading-progress-<novelId>，这里读出来做「继续阅读」）
const lastReadChapter = ref(0);

const extraCategories = computed(() => {
  const all = novel.value?.categories || [];
  const main = novel.value?.category || '';
  return all.filter((c) => c && c !== main).slice(0, 3);
});

const filteredChapters = computed(() => {
  const kw = chapterKeyword.value.trim().toLowerCase();
  let arr = chapters.value;
  if (kw) {
    arr = arr.filter(
      (c) => String(c.chapter_number).includes(kw) || (c.title || '').toLowerCase().includes(kw),
    );
  }
  if (chapterOrder.value === 'desc') arr = [...arr].reverse();
  return arr;
});

const displayChapters = computed(() => {
  const arr = filteredChapters.value;
  // 搜索时不做折叠（用户就是在找某一章）
  if (arr.length <= COLLAPSE_THRESHOLD || showAllChapters.value || chapterKeyword.value.trim()) {
    return arr;
  }
  return arr.slice(0, 5).concat(arr.slice(-5));
});

// 敏感分区确认
const showZoneGuard = ref(false);
const zoneGuardName = ref('');
const zoneGuardCross = ref(false);

function formatDate(dateStr?: string) {
  if (!dateStr) return '';
  return new Date(dateStr).toLocaleDateString('zh-CN');
}

function startReading() {
  if (novel.value && lastReadChapter.value > 0) {
    goChapter(lastReadChapter.value);
    return;
  }
  if (chapters.value.length > 0) {
    router.push(`/author/${novel.value!.author_id}/novel/${novel.value!.id}/read/${chapters.value[0].chapter_number}`);
  }
}

function goChapter(n: number) {
  if (!novel.value || !n) return;
  router.push(`/author/${novel.value.author_id}/novel/${novel.value.id}/read/${n}`);
}

/** 读上次阅读章节：Reader 里存的是 { chapter, time }；章节已被删就忽略 */
function loadLastRead(id: number) {
  try {
    const raw = localStorage.getItem(`reading-progress-${id}`);
    if (!raw) return;
    const p = JSON.parse(raw);
    const n = Number(p?.chapter) || 0;
    if (n > 0 && chapters.value.some((c) => c.chapter_number === n)) lastReadChapter.value = n;
  } catch {
    /* 解析失败当作没读过 */
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
    const res = await forumApi.getNovelForum(novel.value.id);
    const forumId = res.data.data?.forum?.id;
    if (forumId) router.push(`/forum/${forumId}`);
  } catch {
    // 忽略错误
  }
}

async function checkSensitiveZone(category: string) {
  const guard = await shouldShowGuard(category);
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
      chapterApi.getChapters(id),
    ]);
    novel.value = novelRes.data.data;
    chapters.value = chaptersRes.data.data || [];
    loadLastRead(id);

    // URL 规范化：/novel/:id（旧路径）→ /author/{authorId}/novel/:id（推荐路径）。
    // 数字是主键，规范路径用作者数字 id；不改变作者标识（无用户名可保留）。
    if (novel.value && !route.params.authorId) {
      router.replace(`/author/${novel.value.author_id}/novel/${id}`);
    }

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
        const guard = await shouldShowGuard(cat, {
          authorId: novel.value.author_id,
          userId: authStore.user?.id,
          wallEnabled: novel.value.wall_enabled,
        });
        if (guard?.needed && guard) {
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
  background: var(--border-color);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
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
  color: var(--text-light);
  white-space: pre-wrap;
}

.novel-actions {
  display: flex;
  gap: 12px;
}

.novel-tags {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}
.tags-label {
  font-size: 0.78rem;
  color: var(--text-light);
}
.novel-tag-item {
  font-size: 0.8rem;
  padding: 2px 8px;
  border-radius: var(--radius-full, 999px);
  background: var(--bg-soft, #f1f5f9);
  color: var(--text-light);
  text-decoration: none;
  transition: color 0.15s, background 0.15s;
}
.novel-tag-item:hover {
  color: var(--primary-color);
  background: rgba(79, 70, 229, 0.1);
}

/* 目录工具条：标题 + 搜索/排序 */
.chapter-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 10px;
}
.chapter-toolbar .section-title {
  margin: 0;
}
.ct-count {
  margin-left: 10px;
  font-size: 0.8rem;
  font-weight: 400;
  color: var(--text-light);
}
.ct-controls {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.chapter-list {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
}

/* 上次阅读到的章节：整行高亮，回来不用再找 */
.chapter-item.is-read {
  background: rgba(79, 70, 229, 0.06);
  box-shadow: inset 3px 0 0 var(--primary-color);
}
.chapter-item.is-read .chapter-num {
  color: var(--primary-color);
  font-weight: 600;
}

.chapter-item {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
  cursor: pointer;
  transition: background 0.2s;
}

.chapter-item:hover {
  background: var(--bg-soft);
}

.chapter-item:last-child {
  border-bottom: none;
}

/* 目录折叠行 */
.chapter-collapse-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 14px 0;
  margin: 4px 0;
  cursor: pointer;
  color: var(--primary-color);
  font-size: 0.9rem;
  font-weight: 500;
  border-radius: 6px;
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.04), rgba(64, 158, 255, 0.02));
  transition: background 0.2s, color 0.2s;
  user-select: none;
}
.chapter-collapse-row:hover {
  background: rgba(64, 158, 255, 0.08);
  color: var(--el-color-primary);
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
    align-items: center;
    text-align: center;
    gap: 20px;
    margin-bottom: 28px;
  }
  .novel-cover {
    flex-shrink: 0;
  }
  .novel-title {
    font-size: 1.45rem;
    text-align: center;
  }
  .novel-meta {
    justify-content: center;
    gap: 10px 14px;
  }
  .novel-actions {
    justify-content: center;
    flex-wrap: wrap;
  }
  .novel-actions .el-button {
    flex: 1;
    min-width: 110px;
  }
  /* 章节行：隐藏更新日期，压缩间距 */
  .chapter-item {
    padding: 13px 12px;
  }
  .chapter-num {
    min-width: 0;
    font-size: 0.82rem;
  }
  .chapter-words {
    min-width: 0;
    font-size: 0.78rem;
    flex-shrink: 0;
  }
  .chapter-date {
    display: none;
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
[data-theme="dark"] .chapter-collapse-row {
  background: rgba(64, 158, 255, 0.06);
  color: #60a5fa;
}
[data-theme="dark"] .chapter-collapse-row:hover {
  background: rgba(96, 165, 250, 0.12);
  color: #93c5fd;
}
[data-theme="dark"] .chapter-item:last-child {
  border-bottom: none;
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