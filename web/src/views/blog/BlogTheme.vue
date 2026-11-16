<template>
  <div class="bt-page">
    <!-- Hero -->
    <header class="bt-hero">
      <div class="bt-hero-inner">
        <el-avatar :size="72" :src="media(data?.author?.avatar_url)" class="bt-avatar">
          {{ (data?.author?.nickname || '?')[0] }}
        </el-avatar>
        <div class="bt-hero-text">
          <h1>{{ data?.author?.nickname || data?.author?.username || '博客' }}</h1>
          <p>{{ data?.author?.signature || '记录想法，分享所见。' }}</p>
          <div class="bt-hero-stats">
            <span><b>{{ data?.stats?.posts || 0 }}</b> 文章</span>
            <span><b>{{ data?.stats?.categories || 0 }}</b> 分类</span>
            <span><b>{{ data?.stats?.tags || 0 }}</b> 标签</span>
            <span><b>{{ data?.stats?.views || 0 }}</b> 阅读</span>
          </div>
        </div>
        <el-button class="bt-mode" circle :title="themeStore.isDark ? '切换浅色' : '切换深色'" @click="themeStore.toggle()">
          {{ themeStore.isDark ? '☀' : '☾' }}
        </el-button>
      </div>
    </header>

    <!-- 导航 -->
    <nav class="bt-nav">
      <button v-for="s in sections" :key="s.key" :class="{ active: section === s.key }" @click="goSection(s.key)">
        {{ s.label }}
      </button>
      <span class="bt-nav-spacer" />
      <el-input v-model="keyword" size="small" placeholder="搜本站文章" clearable style="width: 200px" @keyup.enter="goSection('search')" />
    </nav>

    <div class="bt-layout">
      <main class="bt-main">
        <!-- 文章卡片流 -->
        <template v-if="section === 'home' || section === 'search' || section === 'category' || section === 'tag' || section === 'archive'">
          <div class="bt-section-title">
            {{ sectionTitle }}
            <span class="bt-muted">共 {{ filteredPosts.length }} 篇</span>
          </div>

          <div v-if="currentTag || currentCategory || currentMonth" class="bt-filterbar">
            <el-tag v-if="currentTag" closable @close="clearFilter('#tag')">标签：{{ currentTag }}</el-tag>
            <el-tag v-if="currentCategory" closable @close="clearFilter('#category')">分类：{{ currentCategory }}</el-tag>
            <el-tag v-if="currentMonth" closable @close="clearFilter('#archive')">归档：{{ currentMonth }}</el-tag>
          </div>

          <div class="bt-posts">
            <article v-for="p in pagedPosts" :key="p.id" class="bt-post">
              <router-link :to="`/author/${data?.author?.id}/blogs/${p.id}`" class="bt-cover">
                <img v-if="p.cover_url" :src="media(p.cover_url)" :alt="p.title" loading="lazy" />
                <span v-else class="bt-cover-ph">✍️</span>
                <span v-if="p.is_pinned" class="bt-pin">置顶</span>
              </router-link>
              <div class="bt-post-body">
                <router-link :to="`/author/${data?.author?.id}/blogs/${p.id}`" class="bt-post-title">{{ p.title }}</router-link>
                <p class="bt-excerpt">{{ p.summary || '（暂无摘要）' }}</p>
                <div class="bt-post-meta">
                  <span>📅 {{ p.created_at }}</span>
                  <span v-if="p.category">📁 {{ p.category }}</span>
                  <span>👁 {{ p.view_count }}</span>
                  <span>♥ {{ p.like_count }}</span>
                  <span>💬 {{ p.comment_count }}</span>
                </div>
                <div v-if="p.tags?.length" class="bt-tags">
                  <button v-for="t in p.tags" :key="t" @click="goSection('tag', t)">#{{ t }}</button>
                </div>
              </div>
            </article>
            <el-empty v-if="!loading && filteredPosts.length === 0" description="这里还没有文章" :image-size="70" />
          </div>
          <div v-if="filteredPosts.length > pageSize" class="bt-more">
            <el-button size="small" @click="page++">加载更多（{{ Math.max(0, filteredPosts.length - pagedPosts.length) }}）</el-button>
          </div>
        </template>

        <!-- 友链 -->
        <section v-else-if="section === 'links'" class="bt-panel">
          <h2>友情链接</h2>
          <div class="bt-links">
            <a v-for="s in sites" :key="s.id" :href="s.url" target="_blank" rel="noopener noreferrer" class="bt-link">
              <b>{{ s.name }}</b><span>{{ s.description || s.url }}</span>
            </a>
          </div>
          <el-empty v-if="sites.length === 0" description="暂无友链" :image-size="50" />
        </section>

        <!-- 关于 -->
        <section v-else-if="section === 'about'" class="bt-panel">
          <h2>关于本站</h2>
          <p class="bt-about">{{ about || '这是一个使用高级博客主题的个人空间。' }}</p>
        </section>
      </main>

      <!-- 侧栏：作者 / 统计 / 分类 / 标签 / 归档 / 近期文章 -->
      <aside class="bt-side">
        <div class="bt-widget bt-author-card">
          <el-avatar :size="52" :src="media(data?.author?.avatar_url)">{{ (data?.author?.nickname || '?')[0] }}</el-avatar>
          <div class="bt-author-name">{{ data?.author?.nickname || data?.author?.username }}</div>
          <div class="bt-author-bio">{{ data?.author?.bio || data?.author?.signature || '这个人很神秘，什么也没写。' }}</div>
          <div class="bt-author-stats">
            <div><b>{{ data?.stats?.posts || 0 }}</b><span>文章</span></div>
            <div><b>{{ data?.stats?.comments || 0 }}</b><span>评论</span></div>
            <div><b>{{ data?.stats?.likes || 0 }}</b><span>获赞</span></div>
          </div>
        </div>

        <div class="bt-widget">
          <h3>分类</h3>
          <button class="bt-list-item" :class="{ active: section === 'category' && !currentCategory }" @click="goSection('category')">
            全部分类 <span>{{ data?.stats?.posts || 0 }}</span>
          </button>
          <button v-for="c in data?.categories || []" :key="c.id" class="bt-list-item"
            :class="{ active: currentCategory === c.name }" @click="goSection('category', c.name)">
            {{ c.name }} <span>{{ c.count }}</span>
          </button>
        </div>

        <div class="bt-widget">
          <h3>标签云</h3>
          <div class="bt-cloud">
            <button v-for="t in data?.tags || []" :key="t.name" :style="{ fontSize: tagSize(t.count) }" @click="goSection('tag', t.name)">
              {{ t.name }} <small>{{ t.count }}</small>
            </button>
          </div>
        </div>

        <div class="bt-widget">
          <h3>归档</h3>
          <button v-for="a in data?.archives || []" :key="a.month" class="bt-list-item"
            :class="{ active: currentMonth === a.month }" @click="goSection('archive', a.month)">
            {{ a.month }} <span>{{ a.count }}</span>
          </button>
        </div>

        <div class="bt-widget">
          <h3>近期文章</h3>
          <router-link v-for="p in (data?.posts || []).slice(0, 5)" :key="p.id"
            :to="`/author/${data?.author?.id}/blogs/${p.id}`" class="bt-recent">
            {{ p.title }}
          </router-link>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { themeBlogApi, type ThemeBlogOverview } from '@/api/themeBlog';
import { useThemeStore } from '@/stores/theme';
import { getAppPrefix } from '@/utils/prefix';
import api from '@/api/index';

const route = useRoute();
const router = useRouter();
const themeStore = useThemeStore();
const prefix = getAppPrefix() || '';
function media(u?: string) {
  if (!u) return '';
  if (/^https?:\/\//i.test(u)) return u;
  return prefix + (u.startsWith('/') ? u : '/' + u);
}

const data = ref<ThemeBlogOverview | null>(null);
const loading = ref(false);
const keyword = ref('');
const page = ref(1);
const pageSize = 8;
const sites = ref<any[]>([]);
const about = ref('');

const sections = [
  { key: 'home', label: '首页' },
  { key: 'category', label: '分类' },
  { key: 'tag', label: '标签' },
  { key: 'archive', label: '归档' },
  { key: 'links', label: '友链' },
  { key: 'about', label: '关于' },
];
const section = computed(() => (route.query.s as string) || 'home');
const currentCategory = computed(() => (section.value === 'category' ? (route.query.v as string) || '' : ''));
const currentTag = computed(() => (section.value === 'tag' ? (route.query.v as string) || '' : ''));
const currentMonth = computed(() => (section.value === 'archive' ? (route.query.v as string) || '' : ''));

const sectionTitle = computed(() => {
  switch (section.value) {
    case 'search': return `搜索：${keyword.value || '本站'}`;
    case 'category': return currentCategory.value ? `分类：${currentCategory.value}` : '全部分类';
    case 'tag': return currentTag.value ? `标签：${currentTag.value}` : '全部标签';
    case 'archive': return currentMonth.value ? `归档：${currentMonth.value}` : '全部归档';
    default: return '近期文章';
  }
});

const filteredPosts = computed(() => {
  let list = [...(data.value?.posts || [])];
  if (section.value === 'search' && keyword.value.trim()) {
    const q = keyword.value.trim().toLowerCase();
    list = list.filter(p => (p.title + p.summary + p.tags.join(' ')).toLowerCase().includes(q));
  } else if (currentCategory.value) {
    list = list.filter(p => p.category === currentCategory.value);
  } else if (currentTag.value) {
    list = list.filter(p => p.tags?.includes(currentTag.value));
  } else if (currentMonth.value) {
    list = list.filter(p => p.month === currentMonth.value);
  }
  return list;
});
const pagedPosts = computed(() => filteredPosts.value.slice(0, page.value * pageSize));

function tagSize(n: number) {
  return `${Math.min(1.35, 0.82 + n * 0.06)}rem`;
}

function goSection(s: string, v?: string) {
  page.value = 1;
  router.push({ path: route.path, query: s === 'home' ? {} : { s, ...(v ? { v } : {}) } });
  if (s === 'links' && sites.value.length === 0) loadLinks();
  if (s === 'about' && !about.value) loadAbout();
}
function clearFilter(target: string) {
  router.push({ path: route.path, query: { s: target.replace('#', '') } });
}

async function load() {
  loading.value = true;
  try {
    const r = await themeBlogApi.overview(String(route.params.key || ''));
    data.value = r.data?.data || null;
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '博客不存在');
  } finally { loading.value = false; }
}
async function loadLinks() {
  try {
    const r = await api.get('/federated/sites');
    sites.value = r.data?.data?.list || r.data?.list || [];
  } catch { sites.value = []; }
}
async function loadAbout() {
  try {
    const r = await api.get('/site-info');
    about.value = (r.data?.data || r.data)?.guidelines || '';
  } catch { about.value = ''; }
}

watch(() => route.params.key, load);
watch(() => route.query.s, () => { page.value = 1; });
onMounted(() => {
  load();
  if (section.value === 'links') loadLinks();
  if (section.value === 'about') loadAbout();
});
</script>

<style scoped>
.bt-page { min-height: 100vh; background: var(--bg-page, #f7f8fa); color: var(--text-color, #222); padding-bottom: 50px; }
.bt-hero { background: linear-gradient(135deg, #5b6ef5, #8f6ff5); color: #fff; padding: 36px 20px 30px; }
.bt-hero-inner { max-width: 1080px; margin: 0 auto; display: flex; gap: 18px; align-items: center; }
.bt-avatar { border: 3px solid rgba(255,255,255,.65); }
.bt-hero-text { flex: 1; }
.bt-hero-text h1 { margin: 0 0 6px; font-size: 1.7rem; }
.bt-hero-text p { margin: 0 0 12px; opacity: .9; }
.bt-hero-stats { display: flex; gap: 18px; font-size: .85rem; opacity: .92; }
.bt-mode { background: rgba(255,255,255,.2); border: none; color: #fff; }
.bt-nav { max-width: 1080px; margin: 0 auto; display: flex; gap: 6px; align-items: center; padding: 12px 20px; }
.bt-nav button { border: none; background: transparent; padding: 7px 14px; border-radius: 18px; cursor: pointer; color: var(--text-muted, #666); }
.bt-nav button.active { background: var(--primary-color, #5b6ef5); color: #fff; }
.bt-nav-spacer { flex: 1; }
.bt-layout { max-width: 1080px; margin: 0 auto; padding: 0 20px; display: grid; grid-template-columns: 1fr 280px; gap: 20px; align-items: start; }
.bt-section-title { font-size: 1.15rem; font-weight: 600; margin: 6px 0 14px; }
.bt-muted { font-weight: 400; font-size: .8rem; color: var(--text-muted, #888); margin-left: 8px; }
.bt-filterbar { display: flex; gap: 8px; margin-bottom: 12px; }
.bt-posts { display: flex; flex-direction: column; gap: 14px; }
.bt-post { display: flex; gap: 14px; background: var(--bg-card, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 10px; padding: 14px; }
.bt-cover { position: relative; width: 170px; height: 112px; flex-shrink: 0; border-radius: 8px; overflow: hidden; background: var(--fill-color, #f0f2f5); }
.bt-cover img { width: 100%; height: 100%; object-fit: cover; }
.bt-cover-ph { display: flex; align-items: center; justify-content: center; height: 100%; font-size: 2rem; }
.bt-pin { position: absolute; top: 6px; left: 6px; background: #f56c6c; color: #fff; font-size: .7rem; padding: 2px 6px; border-radius: 4px; }
.bt-post-body { flex: 1; min-width: 0; }
.bt-post-title { font-size: 1.05rem; font-weight: 600; color: inherit; text-decoration: none; }
.bt-post-title:hover { color: var(--primary-color); }
.bt-excerpt { font-size: .85rem; color: var(--text-muted, #6b7280); line-height: 1.6; margin: 6px 0;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.bt-post-meta { display: flex; gap: 12px; flex-wrap: wrap; font-size: .75rem; color: var(--text-muted, #909399); }
.bt-tags { display: flex; gap: 8px; margin-top: 8px; }
.bt-tags button { border: none; background: transparent; color: var(--primary-color); cursor: pointer; font-size: .78rem; padding: 0; }
.bt-more { text-align: center; margin: 18px 0; }
.bt-side { display: flex; flex-direction: column; gap: 14px; }
.bt-widget { background: var(--bg-card, #fff); border: 1px solid var(--border-color, #e5e7eb); border-radius: 10px; padding: 16px; }
.bt-widget h3 { margin: 0 0 12px; font-size: .95rem; }
.bt-author-card { text-align: center; }
.bt-author-name { font-weight: 600; margin: 8px 0 4px; }
.bt-author-bio { font-size: .8rem; color: var(--text-muted, #6b7280); line-height: 1.5; }
.bt-author-stats { display: flex; justify-content: space-around; margin-top: 14px; padding-top: 12px; border-top: 1px solid var(--fill-color, #f0f2f5); }
.bt-author-stats div { display: flex; flex-direction: column; }
.bt-author-stats b { font-size: 1.05rem; }
.bt-author-stats span { font-size: .72rem; color: var(--text-muted, #909399); }
.bt-list-item { display: flex; justify-content: space-between; width: 100%; border: none; background: transparent; padding: 7px 4px; cursor: pointer; color: inherit; font-size: .85rem; border-radius: 5px; }
.bt-list-item:hover, .bt-list-item.active { background: rgba(91,110,245,.08); color: var(--primary-color); }
.bt-list-item span { color: var(--text-muted, #909399); }
.bt-cloud { display: flex; flex-wrap: wrap; gap: 8px 10px; }
.bt-cloud button { border: none; background: transparent; color: var(--primary-color); cursor: pointer; }
.bt-cloud small { color: var(--text-muted, #aaa); }
.bt-recent { display: block; padding: 6px 2px; font-size: .85rem; color: inherit; text-decoration: none; border-bottom: 1px dashed var(--fill-color, #eee); }
.bt-recent:hover { color: var(--primary-color); }
.bt-panel { background: var(--bg-card, #fff); border: 1px solid var(--border-color); border-radius: 10px; padding: 20px; min-height: 240px; }
.bt-links { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 10px; }
.bt-link { display: flex; flex-direction: column; padding: 10px; border: 1px solid var(--border-color); border-radius: 8px; text-decoration: none; color: inherit; }
.bt-link span { font-size: .75rem; color: var(--text-muted, #888); }
.bt-about { line-height: 1.9; }
[data-theme="dark"] .bt-page { background: #0f172a; }
@media (max-width: 900px) {
  .bt-layout { grid-template-columns: 1fr; }
  .bt-side { order: -1; }
  .bt-cover { width: 110px; height: 80px; }
}
</style>
