<template>
  <!-- ====== 模块区（F15：小说/博客/论坛模块按配置顺序排列，主模块在前） ====== -->
  <div class="home-block home-block--modules">
    <HomeModuleSection
      v-for="(m, i) in modules"
      :key="m"
      :module="m"
      :featured="featuredFirst && i === 0"
    />
    <el-empty v-if="!modules.length" description="未配置任何模块" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { storeToRefs } from 'pinia';
import { useAppStore } from '@/stores/app';
import HomeModuleSection from '@/components/home/HomeModuleSection.vue';

const props = defineProps<{
  /** {modules: ["novels","blogs","forums"], featured_first: true}；缺省时沿用管理员「首页主模块」设置 */
  config: string;
}>();

const appStore = useAppStore();
const { homeModule } = storeToRefs(appStore);

const MODULE_ALL = ['novels', 'blogs', 'forums'] as const;

const parsed = computed(() => {
  try {
    return JSON.parse(props.config || '{}');
  } catch {
    return {};
  }
});

const modules = computed<string[]>(() => {
  // 1) 区块显式配置优先（管理员在主页设置里排的）
  const list = Array.isArray(parsed.value.modules) ? parsed.value.modules : [];
  const valid = list.filter((m: any) => MODULE_ALL.includes(m));
  if (valid.length) return [...new Set(valid)] as string[];

  // 2) 回退：首页主模块在前，其余作副区块（与 F5 行为一致，combined 视作 novels）
  const primary = homeModule.value === 'combined' ? 'novels' : (homeModule.value || 'novels');
  return [primary, ...MODULE_ALL.filter((m) => m !== primary)];
});

const featuredFirst = computed(() => parsed.value.featured_first !== false);
</script>

<style scoped>
.home-block--modules { display: block; }
</style>
