<template>
  <section class="home-module-section" :class="{ 'is-featured': featured }">
    <!-- 区块头：标题 + 查看全部 -->
    <div class="hms-header">
      <div class="hms-title">
        <span class="hms-icon">{{ meta.icon }}</span>
        <h2>{{ meta.title }}</h2>
        <el-tag v-if="featured" size="small" type="primary" effect="plain">首页主模块</el-tag>
      </div>
      <el-button text type="primary" size="small" @click="goMore">
        查看全部 <el-icon><ArrowRight /></el-icon>
      </el-button>
    </div>

    <div class="hms-body">
      <!-- 首次加载用骨架：避免"先空一块、再突然撑开"的跳动 -->
      <div v-if="loading" class="hms-skeleton">
        <template v-if="module === 'novels'">
          <div v-for="i in 4" :key="i" class="hms-skeleton-card" />
        </template>
        <template v-else>
          <div v-for="i in 4" :key="i" class="hms-skeleton-row" />
        </template>
      </div>

      <!-- 小说：卡片网格 -->
      <div v-else-if="module === 'novels'" class="hms-grid">
        <NovelCard
          v-for="item in items"
          :key="item.id"
          :novel="item"
          @click="navigate(item.to)"
        />
      </div>

      <!-- 博客 / 论坛：列表 -->
      <ul v-else-if="!isEmpty" class="hms-list">
        <li
          v-for="item in items"
          :key="item.id"
          class="hms-list-item"
          @click="navigate(item.to)"
        >
          <span class="hms-item-title">{{ item.title }}</span>
          <span class="hms-item-sub">{{ item.subtitle }}</span>
        </li>
      </ul>

      <!-- 空态：新站首页曾出现三块各 297px 的 el-empty，整页上千像素全是"暂无"。
           改成一行紧凑提示 + 一个真能点的下一步。 -->
      <div v-if="!loading && isEmpty" class="hms-empty">
        <span class="hms-empty-icon">{{ meta.icon }}</span>
        <div class="hms-empty-text">
          <b>{{ meta.emptyTitle }}</b>
          <span>{{ meta.emptyHint }}</span>
        </div>
        <el-button size="small" type="primary" plain @click="goEmptyAction">
          {{ meta.emptyAction }}
        </el-button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useRouter } from 'vue-router';
import { ArrowRight } from '@element-plus/icons-vue';
import { novelApi } from '@/api/novel';
import api from '@/api/index';
import NovelCard from '@/components/NovelCard.vue';

type ModuleType = 'novels' | 'blogs' | 'forums';

const props = defineProps<{ module: ModuleType; featured?: boolean }>();
const router = useRouter();

// more 是「查看全部」的去处：小说的列表页是 /feed（原来错写成 /forums，
// 点"查看全部"会从文学作品跳到论坛，是明显的串台）
const metaMap: Record<
  ModuleType,
  { title: string; icon: string; more: string; emptyTitle: string; emptyHint: string; emptyAction: string }
> = {
  novels: {
    title: '类型文学', icon: '📚', more: '/feed',
    emptyTitle: '还没有作品', emptyHint: '第一篇永远是最难写的，但也是最值得的。',
    emptyAction: '去发布作品',
  },
  blogs: {
    title: '创作博客', icon: '✍️', more: '/blogs',
    emptyTitle: '还没有博客', emptyHint: '写下创作手记、灵感碎片，或者连载之外的碎碎念。',
    emptyAction: '写第一篇',
  },
  forums: {
    title: '讨论论坛', icon: '💬', more: '/forums',
    emptyTitle: '还没有版块', emptyHint: '版块是讨论的起点，站长可以在后台创建。',
    emptyAction: '去论坛看看',
  },
};
const meta = computed(() => metaMap[props.module]);

interface Item { id: number; title: string; subtitle: string; to: string }

const items = ref<Item[]>([]);
const loading = ref(true);
const isEmpty = computed(() => items.value.length === 0);

/** 空态按钮：新站引导用户去"能产生内容"的地方 */
function goEmptyAction() {
  if (props.module === 'novels') return router.push('/author/editor');
  if (props.module === 'blogs') return router.push('/author/editor');
  return router.push('/forums');
}

function normalize(list: any[], module: ModuleType): any[] {
  return (list || []).map((r: any) => {
    if (module === 'novels') {
      // 透传整条小说记录，保留 NovelCard 需要的 total_words / total_chapters /
      // cover_url / summary / author / categories 等字段，避免卡片全部回退默认值。
      return {
        ...r,
        title: r.title || '未命名',
        to: `/novel/${r.id}`,
      };
    }
    if (module === 'blogs') {
      return {
        id: r.id,
        title: r.title || '未命名',
        subtitle: r.author_nickname || r.author?.nickname || r.author?.username || '—',
        to: `/blogs/${r.id}`,
      };
    }
    // forums
    return {
      id: r.id,
      title: r.name || r.title || '未命名',
      subtitle: r.description ? String(r.description).slice(0, 40) : (r.type || ''),
      to: `/forums/${r.id}`,
    };
  });
}

async function load() {
  loading.value = true;
  try {
    let list: any[] = [];
    if (props.module === 'novels') {
      const res = await novelApi.getNovels({ page: 1, page_size: props.featured ? 8 : 6, sort_by: 'featured' });
      list = res.data?.data?.list || [];
    } else if (props.module === 'blogs') {
      const res = await api.get('/blogs', { params: { page: 1, page_size: 6 } });
      list = res.data?.data?.list || [];
    } else {
      const res = await api.get('/forums');
      // 论坛接口直接返回数组
      list = res.data?.data || [];
    }
    items.value = normalize(list, props.module);
  } catch (e) {
    console.error('[HomeModuleSection] load failed', e);
  } finally {
    loading.value = false;
  }
}

function navigate(to: string) {
  router.push(to);
}
function goMore() {
  router.push(meta.value.more);
}

onMounted(load);
</script>

<style scoped>
.home-module-section {
  /* 用令牌而不是写死 #fff + 单独覆盖深色：后者每加一个区块就要再写一遍，漏一处就是白底 */
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: 22px 26px;
  margin-bottom: 26px;
  box-shadow: var(--shadow-card);
}
.hms-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.hms-title {
  display: flex;
  align-items: center;
  gap: 10px;
}
.hms-title h2 {
  font-size: 1.25rem;
  font-weight: 300;
  color: var(--text-color);
  letter-spacing: -0.02em;
  margin: 0;
}
.hms-icon { font-size: 1.3rem; }
.hms-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 18px;
}
.hms-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
/* ---- 加载骨架 ---- */
.hms-skeleton { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 18px; }
.hms-skeleton-card {
  height: 210px;
  border-radius: var(--radius-md);
  background: linear-gradient(90deg, var(--bg-page) 25%, var(--bg-soft) 37%, var(--bg-page) 63%);
  background-size: 400% 100%;
  animation: hms-shimmer 1.4s ease infinite;
}
.hms-skeleton-row {
  grid-column: 1 / -1;
  height: 44px;
  border-radius: var(--radius-sm);
  background: linear-gradient(90deg, var(--bg-page) 25%, var(--bg-soft) 37%, var(--bg-page) 63%);
  background-size: 400% 100%;
  animation: hms-shimmer 1.4s ease infinite;
}
@keyframes hms-shimmer {
  0% { background-position: 100% 50%; }
  100% { background-position: 0 50%; }
}
@media (prefers-reduced-motion: reduce) {
  .hms-skeleton-card, .hms-skeleton-row { animation: none; }
}

/* ---- 紧凑空态 ---- */
.hms-empty {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border: 1px dashed var(--border-color);
  border-radius: var(--radius-md);
  background: var(--bg-page);
}
.hms-empty-icon { font-size: 20px; line-height: 1; opacity: .7; }
.hms-empty-text { display: flex; flex-direction: column; gap: 2px; min-width: 0; flex: 1; }
.hms-empty-text b { font-size: var(--text-sm); color: var(--text-color); }
.hms-empty-text span { font-size: var(--text-xs); color: var(--text-light); }

.hms-list-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 12px 6px;
  border-bottom: 1px dashed var(--el-border-color);
  cursor: pointer;
  transition: background 0.15s;
}
.hms-list-item:hover {
  background: var(--bg-soft);
}
.hms-item-title {
  font-weight: 500;
  color: var(--text-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hms-item-sub {
  flex-shrink: 0;
  font-size: 0.8rem;
  color: var(--text-light);
}
</style>
