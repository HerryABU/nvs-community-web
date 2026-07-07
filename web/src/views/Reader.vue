<template>
  <div class="reader-page" v-loading="loading">
    <!-- 顶部导航 -->
    <div class="reader-toolbar">
      <el-button text @click="$router.push(`/novel/${novelId}`)">
        <el-icon><ArrowLeft /></el-icon> 返回目录
      </el-button>
      <span class="reader-title">{{ novelTitle }} · 第{{ chapterNum }}章 {{ chapter?.title }}</span>
      <div class="reader-nav" v-if="!isTrialExceeded">
        <el-button size="small" :disabled="chapterNum <= 1" @click="goChapter(chapterNum - 1)">
          上一章
        </el-button>
        <el-button size="small" :disabled="chapterNum >= totalChapters" @click="goChapter(chapterNum + 1)">
          下一章
        </el-button>
      </div>
    </div>

    <!-- 正文 -->
    <div class="reader-body">
      <div v-if="isTrialExceeded" class="trial-notice">
        <el-result icon="warning" title="试读已结束" sub-title="未登录用户仅可阅读前 {{ trialLimit }} 章（约30%）。请登录后继续阅读全文。">
          <template #extra>
            <el-button type="primary" @click="$router.push('/login')">去登录</el-button>
            <el-button @click="$router.push(`/novel/${novelId}`)">返回作品页</el-button>
          </template>
        </el-result>
      </div>
      <div v-else class="reader-content">
        <div v-if="chapter?.content" class="reader-md-wrap">
          <div v-if="htmlContent" class="reader-html" v-html="htmlContent"></div>
          <div class="reader-md" v-show="!htmlContent">
            <v-md-preview :text="chapter!.content" />
          </div>
        </div>
        <p v-else style="text-align:center;color:#999">暂无内容</p>
      </div>
    </div>

    <!-- 底部导航 -->
    <div v-if="!isTrialExceeded" class="reader-footer">
      <el-button :disabled="chapterNum <= 1" @click="goChapter(chapterNum - 1)">上一章</el-button>
      <el-button :disabled="chapterNum >= totalChapters" @click="goChapter(chapterNum + 1)">下一章</el-button>
    </div>

    <!-- 评论 -->
    <div class="page-container" v-if="!isTrialExceeded">
      <h2 class="section-title">本章评论</h2>
      <CommentSection :novel-id="novelId" :chapter-number="chapterNum" :novel-category="novelCategory" />
    </div>

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
import { ref, computed, onMounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { novelApi, type Chapter } from '@/api/novel';
import { useAuthStore } from '@/stores/auth';
import CommentSection from '@/components/CommentSection.vue';
import SensitiveZoneGuard from '@/components/SensitiveZoneGuard.vue';
import { shouldShowGuard, markZoneConfirmed, setLastZone, recordZoneVisit } from '@/utils/sensitiveZone';

const authStore = useAuthStore();

const route = useRoute();
const router = useRouter();

const novelId = ref(Number(route.params.id));
const chapterNum = ref(Number(route.params.chapter));
const loading = ref(false);
const chapter = ref<Chapter | null>(null);
const htmlContent = ref('');
const novelTitle = ref('');
const novelCategory = ref('');
const novelAuthorId = ref(0);
const totalChapters = ref(0);

// 敏感分区确认
const showZoneGuard = ref(false);
const zoneGuardName = ref('');
const zoneGuardCross = ref(false);

const trialLimit = computed(() => Math.min(5, Math.ceil(totalChapters.value * 0.3)));
const isTrialExceeded = computed(() => !authStore.isLoggedIn && chapterNum.value > trialLimit.value);

function goChapter(num: number) {
  router.push(`/novel/${novelId.value}/read/${num}`);
}

async function loadChapter() {
  loading.value = true;
  try {
    const [chRes, novelRes] = await Promise.all([
      novelApi.getChapter(novelId.value, chapterNum.value),
      novelApi.getNovel(novelId.value),
    ]);
    const detail = chRes.data.data;
    chapter.value = detail?.chapter || null;
    htmlContent.value = detail?.html_content || '';
    if (!chapter.value) {
      console.error('章节内容为空', detail);
    }
    novelTitle.value = novelRes.data.data.title;
    novelCategory.value = novelRes.data.data.category || '';
    novelAuthorId.value = novelRes.data.data.author_id || 0;
    totalChapters.value = novelRes.data.data.total_chapters;
    localStorage.setItem(
      `reading-progress-${novelId.value}`,
      JSON.stringify({ chapter: chapterNum.value, time: Date.now() })
    );
    window.scrollTo(0, 0);

    // 敏感分区检查
    checkSensitiveZone();
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

function checkSensitiveZone() {
  const cat = novelCategory.value;
  if (!cat) return;
  // 记录该区访问（用于读者倾向检测）
  recordZoneVisit(cat);
  const guard = shouldShowGuard(cat, {
    authorId: novelAuthorId.value,
    userId: authStore.user?.id,
  });
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

watch(
  () => route.params.chapter,
  (newVal) => {
    chapterNum.value = Number(newVal);
    loadChapter();
  }
);

onMounted(() => {
  loadChapter();
});
</script>

<style scoped>
.reader-page {
  background: var(--reader-bg);
  min-height: calc(100vh - 60px);
}

.reader-toolbar {
  position: sticky;
  top: 60px;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 24px;
  background: #fff;
  border-bottom: 1px solid var(--border-color);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.05);
}

[data-theme="dark"] .reader-toolbar {
  background: #1e293b;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.3);
}

.reader-title {
  flex: 1;
  font-weight: 500;
  color: var(--text-color);
}

.reader-body {
  padding: 40px 16px;
}

.reader-content {
  max-width: 800px;
  margin: 0 auto;
  padding: 32px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
}

[data-theme="dark"] .reader-content {
  background: #1e293b;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.3);
}

.reader-md-wrap {
  min-height: 200px;
}

.reader-html {
  font-family: Georgia, 'Noto Serif SC', serif;
  font-size: 1.1rem;
  line-height: 2;
  color: #333;
}

[data-theme="dark"] .reader-html {
  color: #cbd5e1;
}

.reader-html :deep(p) {
  margin-bottom: 1em;
  text-indent: 2em;
}

.reader-html :deep(h1),
.reader-html :deep(h2),
.reader-html :deep(h3) {
  margin: 1.5em 0 0.8em;
  text-indent: 0;
  font-weight: 700;
}

[data-theme="dark"] .reader-html :deep(h1),
[data-theme="dark"] .reader-html :deep(h2),
[data-theme="dark"] .reader-html :deep(h3) {
  color: #e2e8f0;
}

.reader-html :deep(pre) {
  background: #f5f5f5;
  padding: 16px;
  border-radius: 6px;
  overflow-x: auto;
  margin: 12px 0;
}

[data-theme="dark"] .reader-html :deep(pre) {
  background: #0f172a;
  color: #cbd5e1;
}

.reader-html :deep(blockquote) {
  border-left: 4px solid #e67e22;
  padding: 8px 16px;
  margin: 12px 0;
  background: #fef9f3;
}

[data-theme="dark"] .reader-html :deep(blockquote) {
  background: #1e293b;
  color: #cbd5e1;
}

.reader-content :deep(.v-md-editor-preview) {
  color: #333 !important;
  font-size: 1.05rem;
  line-height: 2;
  min-height: 100px;
}

[data-theme="dark"] .reader-content :deep(.v-md-editor-preview) {
  color: #cbd5e1 !important;
}

.reader-content :deep(.v-md-editor-preview) * {
  color: #333 !important;
}

[data-theme="dark"] .reader-content :deep(.v-md-editor-preview) * {
  color: #cbd5e1 !important;
}

.reader-footer {
  display: flex;
  justify-content: center;
  gap: 24px;
  padding: 24px;
}

.trial-notice {
  display: flex;
  justify-content: center;
  padding: 60px 16px;
}

@media (max-width: 768px) {
  .reader-toolbar {
    flex-wrap: wrap;
    gap: 8px;
  }
  .reader-title {
    width: 100%;
    order: -1;
    text-align: center;
    font-size: 0.9rem;
  }
  .reader-content {
    padding: 16px;
  }
}
</style>

<style>
[data-theme="dark"] .reader-body {
  background: transparent;
}
[data-theme="dark"] .trial-notice .el-result__title {
  color: #e2e8f0;
}
[data-theme="dark"] .trial-notice .el-result__subtitle {
  color: #94a3b8;
}
</style>
