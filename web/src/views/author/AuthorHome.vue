<template>
  <div class="page-container author-home" v-loading="loading">
    <template v-if="author">
      <!-- ===== 封面：作者装扮（消毒 + 站长审核通过）优先，否则渐变底 ===== -->
      <div class="ah-cover" :class="{ 'is-custom': !!publicHtml }">
        <div v-if="publicHtml" class="ah-cover-custom" v-html="publicHtml" />
        <div v-else class="ah-cover-gradient" />
      </div>

      <!-- ===== 资料头：头像 / 昵称 / 签名 / 简介 / 统计 / 动作 ===== -->
      <div class="ah-profile">
        <div class="ah-avatar-wrap">
          <el-avatar :size="96" :src="author.avatar_url || ''" class="ah-avatar">
            {{ author.nickname?.[0] || author.username?.[0] || 'A' }}
          </el-avatar>
        </div>

        <div class="ah-main">
          <div class="ah-name-row">
            <h1 class="ah-name">{{ author.nickname || author.username }}</h1>
            <el-tag v-if="author.role === 'vip_author'" type="warning" size="small">VIP作者</el-tag>
            <el-tag v-else-if="author.role === 'author'" size="small">作者</el-tag>
            <el-tag v-if="isMember" type="warning" size="small" effect="plain">会员</el-tag>
            <el-tag v-if="author.role === 'police'" type="info" size="small" effect="dark">警方</el-tag>
            <span class="ah-username">@{{ author.username }}</span>
          </div>

          <p class="ah-signature" v-if="author.signature">✍️ {{ author.signature }}</p>
          <p class="ah-bio" v-if="author.bio">{{ author.bio }}</p>

          <div class="ah-stats">
            <span class="ah-stat"><b>{{ followStats.followers }}</b> 粉丝</span>
            <span class="ah-stat"><b>{{ followStats.following }}</b> 关注</span>
            <span class="ah-stat"><b>{{ data.total_novels || 0 }}</b> 作品</span>
            <span class="ah-stat"><b>{{ (data.total_words || 0).toLocaleString() }}</b> 字</span>
            <span class="ah-stat"><b>{{ data.total_chapters || 0 }}</b> 章</span>
            <span class="ah-stat"><b>{{ (data.total_comments || 0).toLocaleString() }}</b> 评论</span>
            <span class="ah-rating" v-if="avgRatingDisplay > 0">
              <el-rate :model-value="avgRatingDisplay" disabled size="small" />
              <span class="ah-rating-num">{{ data.avg_rating?.toFixed(1) }}</span>
            </span>
          </div>
        </div>

        <div class="ah-actions">
          <template v-if="isSelf">
            <el-button type="primary" @click="$router.push('/me/decorate')">编辑我的主页</el-button>
          </template>
          <template v-else-if="authStore.isLoggedIn">
            <el-button :type="isFollowing ? 'default' : 'primary'" :loading="followLoading" @click="toggleFollow">
              {{ isFollowing ? '已关注' : '+ 关注' }}
            </el-button>
            <el-button @click="goMessage">私信</el-button>
          </template>
          <el-button text @click="goForum">讨论区</el-button>
          <ReportButton
            v-if="authStore.isLoggedIn && !isSelf && authorId"
            target-type="user"
            :target-id="authorId"
          />
        </div>
      </div>

      <!-- ===== 内容分栏：作品 / 博客 / 数据 / 留言 ===== -->
      <el-tabs v-model="activeTab" class="ah-tabs">
        <!-- ---------- 作品 ---------- -->
        <el-tab-pane :label="`作品 ${data.total_novels || 0}`" name="works">
          <div class="novel-list" v-if="data.novels && data.novels.length > 0">
            <div
              v-for="novel in data.novels"
              :key="novel.id"
              class="novel-item"
              @click="$router.push(`/author/${authorKey}/novel/${novel.id}`)"
            >
              <div class="novel-cover-sm">
                <el-image v-if="novel.cover_url" :src="novel.cover_url" fit="cover" style="width:64px;height:85px;border-radius:4px">
                  <template #error><div class="cover-ph">封</div></template>
                </el-image>
                <div v-else class="cover-ph">封</div>
              </div>
              <div class="novel-body">
                <h3>{{ novel.title }}</h3>
                <div class="novel-meta">
                  <template v-if="novel.categories && novel.categories.length">
                    <el-tag v-for="c in novel.categories" :key="c" size="small" style="margin-right:4px">{{ c }}</el-tag>
                  </template>
                  <el-tag v-else size="small">{{ novel.category }}</el-tag>
                  <span>{{ novel.total_words?.toLocaleString() || 0 }} 字</span>
                  <span>{{ novel.total_chapters || 0 }} 章</span>
                  <el-tag size="small" :type="novel.status === 'published' ? 'success' : 'info'">
                    {{ novel.status === 'published' ? '已发布' : '草稿' }}
                  </el-tag>
                  <el-tag v-if="novel.source_type === 'reprint'" size="small" type="warning">转载</el-tag>
                  <el-tag v-else-if="novel.source_type === 'original'" size="small">原创</el-tag>
                  <el-tag v-if="novel.creation_method === 'ai'" size="small" type="info">AI创作</el-tag>
                  <el-tag v-else-if="novel.creation_method === 'human_ai_assisted'" size="small" type="info">AI辅助</el-tag>
                </div>
                <p class="novel-summary-line">{{ novel.summary?.slice(0, 120) || '暂无简介' }}</p>
              </div>
            </div>
          </div>
          <div v-else class="home-block-empty">
            {{ isSelf ? '你还没有发布作品，去创作第一篇吧' : '这位作者还没有公开的作品' }}
          </div>
        </el-tab-pane>

        <!-- ---------- 博客 ---------- -->
        <el-tab-pane :label="`博客 ${blogTotal || 0}`" name="blogs">
          <div class="blog-list" v-if="blogs.length > 0">
            <div v-for="blog in blogs" :key="blog.id" class="blog-item" @click="$router.push(`/author/${authorKey}/blogs/${blog.id}`)">
              <div class="blog-title-row">
                <el-tag v-if="blog.is_pinned" size="small" type="danger" style="margin-right:6px">置顶</el-tag>
                <span class="blog-item-title">{{ blog.title }}</span>
              </div>
              <div class="blog-meta">
                <span>{{ formatDate(blog.created_at) }}</span>
                <span>{{ blog.view_count }} 次阅读</span>
              </div>
              <p class="blog-summary" v-if="blog.summary">{{ blog.summary }}</p>
            </div>
          </div>
          <div v-else class="home-block-empty">还没有公开的博客文章</div>
          <div class="ah-more" v-if="blogTotal > blogs.length">
            <el-button size="small" text @click="loadMoreBlogs">加载更多（{{ blogs.length }}/{{ blogTotal }}）</el-button>
          </div>
          <div class="ah-more" v-else-if="blogs.length">
            <el-button size="small" text @click="$router.push(`/author/${authorKey}/blogs`)">查看全部博客 →</el-button>
          </div>
        </el-tab-pane>

        <!-- ---------- 数据 ---------- -->
        <el-tab-pane label="数据" name="stats">
          <div class="ah-metric-cards">
            <div class="ah-metric">
              <div class="ah-metric-num">{{ (data.total_words || 0).toLocaleString() }}</div>
              <div class="ah-metric-label">累计字数</div>
            </div>
            <div class="ah-metric">
              <div class="ah-metric-num">{{ data.total_chapters || 0 }}</div>
              <div class="ah-metric-label">累计章节</div>
            </div>
            <div class="ah-metric">
              <div class="ah-metric-num">{{ (data.total_comments || 0).toLocaleString() }}</div>
              <div class="ah-metric-label">收到评论</div>
            </div>
            <div class="ah-metric">
              <div class="ah-metric-num">{{ data.avg_rating ? data.avg_rating.toFixed(2) : '—' }}</div>
              <div class="ah-metric-label">平均评分</div>
            </div>
          </div>
          <!-- 单个图表实例即可（DashboardCharts 自带指标切换）；旧版放了两个同样的实例，重复渲染 -->
          <div class="chart-card" v-if="hasChartData">
            <DashboardCharts :metrics="authorHomeTrendMetrics" />
          </div>
          <div v-else class="home-block-empty">暂无趋势数据（作品发布后开始统计）</div>
        </el-tab-pane>

        <!-- ---------- 留言 ---------- -->
        <el-tab-pane label="留言" name="comments">
          <CommentSection
            :novel-id="data.novels && data.novels.length > 0 ? data.novels[0].id : 0"
            novel-category=""
            :owner-id="authorId"
          />
        </el-tab-pane>
      </el-tabs>
    </template>

    <div v-else-if="!loading" class="home-block-empty">用户不存在或已注销</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { publicApi } from '@/api/admin';
import { followApi, blogApi } from '@/api/social';
import { useAuthStore } from '@/stores/auth';
import { ElMessage } from 'element-plus';
import CommentSection from '@/components/CommentSection.vue';
import DashboardCharts from '@/components/DashboardCharts.vue';
import ReportButton from '@/components/ReportButton.vue';
import { homeCustomizeApi } from '@/api/homeCustomize';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

// :id 兼容数字主键与用户名（后端 getAuthorProfile 统一解析）
const authorKey = ref(String(route.params.id ?? ''));
const authorId = ref(0); // 解析后的数字主键（异步填充）
const loading = ref(false);
const author = ref<any>(null);
const data = ref<any>({});

// 作者公开主页装扮（已消毒 + 审核通过的对外片段）
const publicHtml = ref('');

// 内容分栏
const activeTab = ref<'works' | 'blogs' | 'stats' | 'comments'>('works');

// 关注
const isFollowing = ref(false);
const followLoading = ref(false);
const followStats = ref({ followers: 0, following: 0 });

// 博客
const blogs = ref<any[]>([]);
const blogPage = ref(1);
const blogTotal = ref(0);

const isSelf = computed(() => !!authStore.user?.id && authStore.user.id === authorId.value);
const isMember = computed(() => {
  const until = author.value?.member_until;
  return !!until && new Date(until).getTime() > Date.now();
});

function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleString('zh-CN'); }

const avgRatingDisplay = computed(() => {
  const v = data.value.avg_rating;
  if (!v || v <= 0) return 0;
  return Math.min(5, Math.round(v * 2) / 2); // 换算成 0.5 步长
});

const hasChartData = computed(() => {
  return (data.value.chapter_trend?.dates?.length > 0) || (data.value.comment_trend?.dates?.length > 0);
});

// ── 趋势数据（供 DashboardCharts 使用）──
const chapterTrendData = computed(() => {
  const t = data.value.chapter_trend;
  return t ? { title: '章节增长趋势', dates: t.dates || [], values: t.counts || [], seriesName: '新增章节' } : null;
});

const commentTrendData = computed(() => {
  const t = data.value.comment_trend;
  return t ? { title: '评论趋势', dates: t.dates || [], values: t.counts || [], seriesName: '评论数' } : null;
});

const wordCountTrendData = computed(() => {
  const t = data.value.word_count_trend;
  return t ? { title: '字数增长趋势', dates: t.dates || [], values: t.counts || [], seriesName: '新增字数' } : null;
});

// 多指标选项：章节增长 / 字数增长 / 评论趋势（图内切换）
const authorHomeTrendMetrics = computed(() => {
  const opts: any[] = [];
  if (chapterTrendData.value) opts.push({ label: '章节增长趋势', data: chapterTrendData.value });
  if (wordCountTrendData.value) opts.push({ label: '字数增长趋势', data: wordCountTrendData.value });
  if (commentTrendData.value) opts.push({ label: '评论趋势', data: commentTrendData.value });
  return opts;
});

async function load() {
  loading.value = true;
  try {
    const res = await publicApi.getAuthorProfile(authorKey.value);
    if (res.data.code === 0) {
      const d = res.data.data;
      author.value = d.author;
      data.value = d;
      authorId.value = d.author_id || d.author?.id || 0;
    }
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
  if (authorId.value > 0) {
    loadFollowStats();
    loadFollowStatus();
    loadBlogs();
    loadPublicHtml();
  }
}

async function loadPublicHtml() {
  try {
    const res = await homeCustomizeApi.getAuthorHomeCustomize(authorId.value);
    if (res.data.code === 0) publicHtml.value = res.data.data.public_html || '';
  } catch { /* 无装扮或审核未通过：用渐变底 */ }
}

/** 粉丝/关注数：必须取「被访问者」的口径（/author/:id/follow-stats），不能用浏览器主人自己的 stats */
async function loadFollowStats() {
  try {
    const res = await followApi.authorStats(authorKey.value);
    if (res.data.code === 0) {
      followStats.value = {
        followers: res.data.data.followers || 0,
        following: res.data.data.following || 0,
      };
    }
  } catch { /* 计数失败不阻断主页 */ }
}

async function loadFollowStatus() {
  if (!authStore.isLoggedIn || isSelf.value) return;
  try {
    const res = await followApi.check(authorId.value);
    isFollowing.value = (res.data as any).data?.is_following || false;
  } catch { /* 未登录/接口不可用：按未关注展示 */ }
}

async function toggleFollow() {
  followLoading.value = true;
  try {
    if (isFollowing.value) {
      await followApi.unfollow(authorId.value);
      ElMessage.success('已取消关注');
    } else {
      await followApi.follow(authorId.value);
      ElMessage.success('关注成功');
    }
    isFollowing.value = !isFollowing.value;
    loadFollowStats();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  } finally {
    followLoading.value = false;
  }
}

/** 私信：直达私信页并打开与该用户的会话（?peer=） */
function goMessage() {
  if (!authStore.isLoggedIn) { router.push('/login'); return; }
  const name = author.value?.nickname || author.value?.username || '';
  router.push({
    path: '/user/messages',
    query: { peer: String(authorId.value), name, avatar: author.value?.avatar_url || '' },
  });
}

async function loadBlogs() {
  try {
    const res = await blogApi.listByAuthor(authorId.value, blogPage.value);
    if (res.data.code === 0) {
      const list = res.data.data.list || [];
      // 追加分页时不要把已加载的覆盖掉
      blogs.value = blogPage.value === 1 ? list : blogs.value.concat(list);
      blogTotal.value = res.data.data.total || 0;
    }
  } catch { /* 博客接口不可用不影响主页其它区块 */ }
}

function loadMoreBlogs() {
  blogPage.value++;
  loadBlogs();
}

async function goForum() {
  try {
    const res = await publicApi.getAuthorForum(authorId.value);
    const forumId = res.data.data?.forum?.id;
    if (forumId) router.push(`/forum/${forumId}`);
  } catch {
    ElMessage.error('讨论区暂不可用');
  }
}

onMounted(load);
</script>

<style scoped>
/* ===== 封面 ===== */
.ah-cover {
  position: relative;
  height: 160px;
  border-radius: 14px;
  overflow: hidden;
  margin-bottom: -48px;
}
.ah-cover-gradient {
  width: 100%;
  height: 100%;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.24), rgba(56, 189, 248, 0.18) 60%, rgba(236, 72, 153, 0.16));
}
/* 自定义装扮：作为封面层，超高时裁切，避免把资料区挤出屏幕 */
.ah-cover-custom {
  width: 100%;
  height: 100%;
  overflow: hidden;
}
.ah-cover-custom :deep(img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

/* ===== 资料头 ===== */
.ah-profile {
  position: relative;
  display: flex;
  gap: 18px;
  align-items: flex-start;
  padding: 16px 20px 20px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 14px;
}
.ah-avatar-wrap {
  margin-top: -56px;
  flex-shrink: 0;
}
.ah-avatar {
  border: 4px solid var(--bg-card);
  background: var(--primary-color);
  font-size: 1.8rem;
}
.ah-main { flex: 1; min-width: 0; }
.ah-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.ah-name {
  font-size: 1.45rem;
  margin: 0;
  color: var(--text-color);
  line-height: 1.3;
}
.ah-username {
  font-size: 0.82rem;
  color: var(--text-light);
}
.ah-signature {
  margin: 6px 0 0;
  font-size: 0.88rem;
  color: var(--primary-color);
}
.ah-bio {
  margin: 6px 0 0;
  font-size: 0.88rem;
  line-height: 1.7;
  color: var(--text-light);
  white-space: pre-wrap;
  max-width: 720px;
}
.ah-stats {
  display: flex;
  align-items: center;
  gap: 18px;
  flex-wrap: wrap;
  margin-top: 10px;
  font-size: 0.82rem;
  color: var(--text-light);
}
.ah-stat b {
  color: var(--text-color);
  font-size: 0.95rem;
  margin-right: 2px;
}
.ah-rating {
  display: flex;
  align-items: center;
  gap: 6px;
}
.ah-rating-num {
  color: #f59e0b;
  font-weight: 600;
}
.ah-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  flex-shrink: 0;
}

/* ===== 分栏 ===== */
.ah-tabs {
  margin-top: 18px;
}
.ah-more {
  margin-top: 10px;
  text-align: center;
}

/* 数据卡片 */
.ah-metric-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}
.ah-metric {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 14px 16px;
}
.ah-metric-num {
  font-size: 1.3rem;
  font-weight: 700;
  color: var(--text-color);
  line-height: 1.2;
}
.ah-metric-label {
  margin-top: 4px;
  font-size: 0.78rem;
  color: var(--text-light);
}
.chart-card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 16px;
}

/* ===== 作品卡 ===== */
.novel-list { display: flex; flex-direction: column; gap: 8px; }
.novel-item {
  display: flex;
  gap: 16px;
  padding: 14px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  cursor: pointer;
  transition: box-shadow 0.2s, transform 0.2s;
}
.novel-item:hover {
  box-shadow: 0 4px 14px rgba(15, 23, 42, 0.08);
  transform: translateY(-1px);
}
.novel-body { flex: 1; min-width: 0; }
.novel-body h3 { font-size: 1.02rem; margin: 0 0 6px; color: var(--text-color); }
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
  color: var(--text-light);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin: 0;
}
.cover-ph {
  width: 64px;
  height: 85px;
  background: var(--bg-soft, #e2e8f0);
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-light);
  font-size: 1.2rem;
}

/* ===== 博客卡 ===== */
.blog-list { display: flex; flex-direction: column; gap: 8px; }
.blog-item {
  padding: 14px 16px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  cursor: pointer;
  transition: box-shadow 0.2s;
}
.blog-item:hover { box-shadow: 0 2px 10px rgba(15, 23, 42, 0.08); }
.blog-title-row { display: flex; align-items: center; gap: 6px; margin-bottom: 4px; }
.blog-item-title { font-weight: 600; font-size: 0.95rem; color: var(--text-color); }
.blog-meta { font-size: 0.75rem; color: var(--text-light); display: flex; gap: 16px; }
.blog-summary { font-size: 0.85rem; color: var(--text-light); margin: 4px 0 0; }

/* ===== 移动端 ===== */
@media (max-width: 640px) {
  .ah-cover { height: 120px; margin-bottom: -40px; }
  .ah-profile {
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 10px;
    padding: 14px 14px 18px;
  }
  .ah-stats { justify-content: center; gap: 12px; }
  .ah-actions { justify-content: center; width: 100%; }
  .ah-name { font-size: 1.25rem; }
  .ah-bio { text-align: left; }
}
</style>
