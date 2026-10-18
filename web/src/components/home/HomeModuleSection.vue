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

    <div v-loading="loading" class="hms-body">
      <!-- 小说：卡片网格 -->
      <div v-if="module === 'novels'" class="hms-grid">
        <NovelCard
          v-for="item in items"
          :key="item.id"
          :novel="item"
          @click="navigate(item.to)"
        />
      </div>

      <!-- 博客 / 论坛：列表 -->
      <ul v-else class="hms-list">
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

      <el-empty v-if="!loading && items.length === 0" :description="`暂无${meta.title}`" />
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

const metaMap: Record<ModuleType, { title: string; icon: string; more: string }> = {
  novels: { title: '类型文学', icon: '📚', more: '/forums' },
  blogs: { title: '创作博客', icon: '✍️', more: '/blogs' },
  forums: { title: '讨论论坛', icon: '💬', more: '/forums' },
};
const meta = computed(() => metaMap[props.module]);

interface Item { id: number; title: string; subtitle: string; to: string }

const items = ref<Item[]>([]);
const loading = ref(false);

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
  background: #fff;
  border-radius: var(--radius-lg);
  padding: 22px 26px;
  margin-bottom: 26px;
  box-shadow: var(--shadow-card);
}
[data-theme="dark"] .home-module-section {
  background: #1a1d27;
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
