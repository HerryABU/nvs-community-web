<template>
  <!-- ====== 链接组区块（F15：快捷入口按钮组） ====== -->
  <section v-if="items.length" class="home-block block-links">
    <header v-if="title" class="bl-header">
      <h2>{{ title }}</h2>
    </header>
    <div class="bl-body">
      <el-button v-for="(l, i) in items" :key="i" :type="i === 0 ? 'primary' : 'default'" round @click="go(l.url)">
        {{ l.title }}
      </el-button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';

const props = defineProps<{
  title?: string;
  /** {items: [{title, url}]} */
  config: string;
}>();

const router = useRouter();

const items = computed(() => {
  try {
    const parsed = JSON.parse(props.config || '{}');
    return Array.isArray(parsed.items)
      ? parsed.items.filter((l: any) => l && typeof l.title === 'string' && typeof l.url === 'string').slice(0, 12)
      : [];
  } catch {
    return [];
  }
});

function go(url: string) {
  if (/^https?:\/\//.test(url)) {
    window.open(url, '_blank', 'noopener');
  } else {
    router.push(url);
  }
}
</script>

<style scoped>
.home-block {
  background: #fff;
  border-radius: var(--radius-lg);
  padding: 18px 26px;
  margin-bottom: 26px;
  box-shadow: var(--shadow-card);
}
[data-theme="dark"] .home-block { background: #1a1d27; }
.bl-header h2 { margin: 0 0 10px; font-size: 1.05rem; font-weight: 600; }
.bl-body { display: flex; flex-wrap: wrap; gap: 10px; }
</style>
