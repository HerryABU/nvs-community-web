<template>
  <!-- ====== 版块大厅区块（F15：论坛首页风格的版块卡片列表） ====== -->
  <section class="home-block block-forums">
    <header class="bf-header">
      <h2>{{ title || '版块大厅' }}</h2>
      <el-button text type="primary" @click="$router.push('/forums')">
        进入论坛 <el-icon><ArrowRight /></el-icon>
      </el-button>
    </header>
    <div v-loading="loading" class="bf-grid">
      <template v-if="!loading && items.length">
        <article v-for="f in items" :key="f.id" class="bf-card" @click="$router.push(`/forums`)">
          <div class="bf-icon">{{ f.name?.slice(0, 1) || '版' }}</div>
          <div class="bf-main">
            <h3 class="bf-name">
              {{ f.name }}
              <el-tag v-if="f.has_password" size="small" type="warning" effect="plain">密码</el-tag>
            </h3>
            <p class="bf-desc">{{ f.description || '这个版块还没有简介' }}</p>
            <div class="bf-meta">{{ f.thread_count || 0 }} 帖</div>
          </div>
        </article>
      </template>
      <el-empty v-else-if="!loading" description="暂无版块" :image-size="60" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ArrowRight } from '@element-plus/icons-vue';
import api from '@/api/index';

const props = defineProps<{
  title?: string;
  /** {limit: 8} */
  config: string;
}>();

const items = ref<any[]>([]);
const loading = ref(true);

const parsed = JSON.parse(props.config || '{}');
const limit = Math.min(Math.max(Number(parsed.limit) || 8, 1), 20);

onMounted(async () => {
  try {
    // 论坛列表接口直接返回数组（系统版块 + 大厅自建区公开版块）
    const res = await api.get('/forums');
    const list: any[] = res.data?.data || [];
    // 隐藏密码版块的敏感度提示仅前端展示用；列表本身即公开数据
    items.value = list.slice(0, limit);
  } catch (e) {
    console.error('[BlockForums] load failed', e);
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.home-block {
  background: #fff;
  border-radius: var(--radius-lg);
  padding: 22px 26px;
  margin-bottom: 26px;
  box-shadow: var(--shadow-card);
}
[data-theme="dark"] .home-block { background: #1a1d27; }

.bf-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.bf-header h2 { margin: 0; font-size: 1.15rem; font-weight: 600; }

.bf-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 14px; }
.bf-card {
  display: flex;
  gap: 12px;
  padding: 14px;
  border-radius: var(--radius-md);
  border: 1px solid var(--el-border-color-lighter);
  cursor: pointer;
  transition: border-color .2s, transform .2s;
}
.bf-card:hover { border-color: var(--el-color-primary); transform: translateY(-1px); }
.bf-icon {
  flex-shrink: 0;
  width: 42px; height: 42px;
  border-radius: 10px;
  display: flex; align-items: center; justify-content: center;
  font-weight: 700;
  color: #fff;
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
}
.bf-main { flex: 1; min-width: 0; }
.bf-name { margin: 0 0 4px; font-size: .98rem; display: flex; align-items: center; gap: 6px; }
.bf-desc {
  margin: 0 0 6px;
  font-size: .82rem;
  color: var(--text-light);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.bf-meta { font-size: .76rem; color: var(--text-light); }
</style>
