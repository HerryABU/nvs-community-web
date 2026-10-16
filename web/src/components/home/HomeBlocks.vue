<template>
  <!-- ====== 自定义主页区块编排（F15） ======
       拉取启用区块按序渲染；接口失败/空数据时回退到默认布局（头图+公告+模块区），
       保证任何部署状态下首页都有内容。 -->
  <div class="home-blocks">
    <template v-if="!loading">
      <template v-for="b in blocks" :key="b.id">
        <!-- 头图 -->
        <HomeHero
          v-if="b.type === 'hero'"
          :site-name="siteName"
          :categories="categories"
          :show-categories="showCategories(b.config)"
        />
        <!-- 公告栏 -->
        <HomeNoticeBar v-else-if="b.type === 'notice'" ref="noticeRefs" :max-items="maxItems(b.config)" />
        <!-- 模块区（小说/博客/论坛） -->
        <BlockModules v-else-if="b.type === 'modules'" :config="b.config" />
        <!-- 最新博客 -->
        <BlockBlogs v-else-if="b.type === 'blogs'" :title="b.title" :config="b.config" />
        <!-- 版块大厅 -->
        <BlockForums v-else-if="b.type === 'forums'" :title="b.title" :config="b.config" />
        <!-- 最新帖 -->
        <BlockThreads v-else-if="b.type === 'threads'" :title="b.title" :config="b.config" />
        <!-- 自定义 HTML（服务端已消毒） -->
        <BlockCustom v-else-if="b.type === 'custom'" :title="b.title" :config="b.config" />
        <!-- 链接组 -->
        <BlockLinks v-else-if="b.type === 'links'" :title="b.title" :config="b.config" />
      </template>

      <!-- 回退：无任何区块时渲染默认布局 -->
      <template v-if="!blocks.length">
        <HomeHero :site-name="siteName" :categories="categories" />
        <HomeNoticeBar />
        <BlockModules :config="JSON.stringify({ modules: ['novels', 'blogs', 'forums'], featured_first: true })" />
      </template>
    </template>

    <div v-else class="home-blocks__loading">
      <el-skeleton :rows="4" animated />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { storeToRefs } from 'pinia';
import { useAppStore } from '@/stores/app';
import { homeBlockApi, type HomeBlock } from '@/api/home';
import HomeHero from '@/components/home/HomeHero.vue';
import HomeNoticeBar from '@/components/home/HomeNoticeBar.vue';
import BlockModules from '@/components/home/BlockModules.vue';
import BlockBlogs from '@/components/home/BlockBlogs.vue';
import BlockForums from '@/components/home/BlockForums.vue';
import BlockThreads from '@/components/home/BlockThreads.vue';
import BlockCustom from '@/components/home/BlockCustom.vue';
import BlockLinks from '@/components/home/BlockLinks.vue';

const appStore = useAppStore();
const { siteName, categories } = storeToRefs(appStore);

const blocks = ref<HomeBlock[]>([]);
const loading = ref(true);

function parseCfg(config: string): Record<string, any> {
  try {
    return JSON.parse(config || '{}');
  } catch {
    return {};
  }
}
function showCategories(config: string) { return parseCfg(config).show_categories !== false; }
function maxItems(config: string) {
  const n = Number(parseCfg(config).max_items);
  return n >= 1 && n <= 10 ? n : 3;
}

onMounted(async () => {
  try {
    const res = await homeBlockApi.list();
    if (res.data.code === 0) {
      blocks.value = res.data.data.items || [];
    }
  } catch (e) {
    console.error('[HomeBlocks] load failed, 回退默认布局', e);
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.home-blocks__loading { max-width: 1200px; margin: 24px auto; }
</style>
