<template>
  <div class="dashboard-bg">
    <div class="dashboard-container">
      <!-- 顶部标题 -->
      <div class="dashboard-hero">
        <h1 class="dashboard-main-title">
          <span class="title-glow">创作数据大屏</span>
        </h1>
        <p class="dashboard-subtitle">跟踪创作数据 · 管理作品</p>
        <div class="hero-buttons">
          <el-button class="create-btn" size="large" @click="$router.push('/author/editor')">
            <el-icon><EditPen /></el-icon>新建作品
          </el-button>
          <el-button size="large" @click="$router.push('/author/blog/new')">
            <el-icon><Edit /></el-icon>写博客
          </el-button>
          <el-button size="large" @click="goMyBlogs">
            <el-icon><Notebook /></el-icon>我的博客
          </el-button>
          <el-button size="large" @click="$router.push('/collab')">
            <el-icon><UserFilled /></el-icon>协作
          </el-button>
          <el-button size="large" @click="$router.push('/ai-write')">
            <el-icon><MagicStick /></el-icon>AI 写作
          </el-button>
        </div>
      </div>

      <!-- 作者名片：头像 + 签名状态 -->
      <AuthorCard />

      <!--
        独立视图切换栏：当前视图由 URL 的 ?tab= 决定（可分享、可刷新保持、前进后退可用）。
        每个面板 lazy：只有切到该视图时才创建组件、拉取数据 —— 一屏只装一个视图，不再拥挤。
      -->
      <el-tabs v-model="activeTab" class="author-tabs" @tab-change="onTabChange">
        <el-tab-pane label="数据与收视率" name="stats" lazy>
          <AuthorStatsView />
        </el-tab-pane>
        <el-tab-pane label="我的作品" name="novels" lazy>
          <AuthorNovelsView />
        </el-tab-pane>
        <el-tab-pane label="我的博客" name="blogs" lazy>
          <AuthorBlogsView />
        </el-tab-pane>
        <el-tab-pane label="自定义 HTML" name="html" lazy>
          <AuthorHtmlView />
        </el-tab-pane>
        <el-tab-pane label="API Key" name="keys" lazy>
          <AuthorKeysView />
        </el-tab-pane>
        <el-tab-pane label="论坛与回收站" name="content" lazy>
          <AuthorContentView />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import { EditPen, Edit, Notebook, UserFilled, MagicStick } from '@element-plus/icons-vue';
import AuthorStatsView from './parts/AuthorStatsView.vue';
import AuthorNovelsView from './parts/AuthorNovelsView.vue';
import AuthorBlogsView from './parts/AuthorBlogsView.vue';
import AuthorHtmlView from './parts/AuthorHtmlView.vue';
import AuthorKeysView from './parts/AuthorKeysView.vue';
import AuthorContentView from './parts/AuthorContentView.vue';
import { bumpAuthorData } from './parts/authorRefresh';

// 作者工作台：拆成若干独立视图，用顶部切换栏切换，当前视图写进 URL 的 ?tab=。
// （原来这些内容全堆在一页上：卡片 + 4 张图 + 博客 + HTML + Key + 作品 + 论坛 + 回收站 + 封禁，
//   一屏塞不下、加载时还要并发打 8 个接口。现在一屏只装一个视图。）
const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const TABS = ['stats', 'novels', 'blogs', 'html', 'keys', 'content'] as const;
type TabKey = (typeof TABS)[number];
const DEFAULT_TAB: TabKey = 'stats';

function normalizeTab(v: unknown): TabKey {
  const s = String(v || '');
  return (TABS as readonly string[]).includes(s) ? (s as TabKey) : DEFAULT_TAB;
}

const activeTab = ref<TabKey>(normalizeTab(route.query.tab));

// URL → 视图（前进/后退、手动改地址栏都能正确切）
watch(
  () => route.query.tab,
  (v) => {
    const t = normalizeTab(v);
    if (t !== activeTab.value) activeTab.value = t;
  },
);

// 视图 → URL（切视图即写回 ?tab=，保证地址可分享）
function onTabChange(name: string | number) {
  const t = normalizeTab(name);
  if (route.query.tab === t) return;
  router.replace({ path: route.path, query: { ...route.query, tab: t } });
}

// 从编辑器 / 博客编辑页返回工作台：让各视图重新拉数据（原来是一个大页面统一刷新）
watch(
  () => route.path,
  (np, op) => {
    if (np === '/author' && op !== '/author') bumpAuthorData();
  },
);

// 进入「我的博客」管理页：/author/blog/{用户名}（数字主键兼容由页面处理）
function goMyBlogs() {
  const u = authStore.user;
  if (u?.username) {
    router.push(`/author/blog/${u.username}`);
  } else {
    router.push('/author/blog/new');
  }
}

// 首屏若没带 ?tab=，补上默认视图参数（让地址栏始终反映当前视图）
if (!route.query.tab) {
  router.replace({ path: route.path, query: { ...route.query, tab: DEFAULT_TAB } });
}
</script>

<style scoped>
@import './parts/statcard.css';

.dashboard-bg {
  min-height: 100vh;
  background: linear-gradient(135deg, #0f172a 0%, #1e293b 40%, #0f172a 100%);
  position: relative;
}

[data-theme="light"] .dashboard-bg {
  background: linear-gradient(135deg, #e2e8f0 0%, #f1f5f9 40%, #e2e8f0 100%);
}

.dashboard-bg::before {
  content: '';
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background:
    radial-gradient(ellipse at 20% 20%, rgba(139, 92, 246, 0.08) 0%, transparent 50%),
    radial-gradient(ellipse at 80% 60%, rgba(99, 102, 241, 0.06) 0%, transparent 50%),
    radial-gradient(ellipse at 50% 80%, rgba(245, 158, 11, 0.04) 0%, transparent 50%);
  pointer-events: none;
  z-index: 0;
}

[data-theme="light"] .dashboard-bg::before {
  background:
    radial-gradient(ellipse at 20% 20%, rgba(139, 92, 246, 0.04) 0%, transparent 50%),
    radial-gradient(ellipse at 80% 60%, rgba(99, 102, 241, 0.03) 0%, transparent 50%),
    radial-gradient(ellipse at 50% 80%, rgba(245, 158, 11, 0.02) 0%, transparent 50%);
}

.dashboard-container {
  max-width: 1280px;
  margin: 0 auto;
  padding: 40px 24px 60px;
  position: relative;
  z-index: 1;
}

/* ===== Hero ===== */
.dashboard-hero {
  text-align: center;
  margin-bottom: 40px;
}

.dashboard-main-title {
  font-size: 2.4rem;
  font-weight: 800;
  margin: 0 0 8px;
  line-height: 1.3;
}

.title-glow {
  background: linear-gradient(135deg, #a78bfa, #f59e0b, #34d399);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  filter: drop-shadow(0 0 18px rgba(167, 139, 250, 0.35));
}

.dashboard-subtitle {
  color: var(--text-light);
  font-size: 0.95rem;
  margin: 0 0 16px;
}

.create-btn {
  background: linear-gradient(135deg, #818cf8, #6366f1) !important;
  border: none !important;
  color: #fff !important;
  font-weight: 600 !important;
  padding: 12px 28px !important;
  border-radius: 12px !important;
  box-shadow: 0 4px 20px rgba(99, 102, 241, 0.35);
}

.create-btn:hover {
  box-shadow: 0 6px 28px rgba(99, 102, 241, 0.5) !important;
  transform: translateY(-2px);
}

/* ===== 独立视图切换栏（?tab= 驱动） ===== */
.author-tabs {
  margin-top: 8px;
}

.author-tabs :deep(.el-tabs__header) {
  margin-bottom: 20px;
}

.author-tabs :deep(.el-tabs__item) {
  font-size: 0.95rem;
  font-weight: 600;
}

[data-theme="light"] .author-tabs :deep(.el-tabs__item) {
  color: var(--text-color);
}

@media (max-width: 768px) {
  .dashboard-container {
    padding: 20px 12px 40px;
  }
  .dashboard-hero {
    margin-bottom: 24px;
  }
  .dashboard-main-title {
    font-size: 1.5rem;
  }
  .dashboard-subtitle {
    font-size: 0.85rem;
  }
  .hero-buttons {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
  }
  .hero-buttons .el-button {
    flex: 1;
    min-width: 120px;
    margin-left: 0 !important;
  }
  .author-tabs :deep(.el-tabs__nav-wrap) {
    overflow-x: auto;
  }
}
</style>
