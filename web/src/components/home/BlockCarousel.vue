<template>
  <!-- 轮播（gap-audit #23）：后端按时段过滤，前端只负责展示与跳转 -->
  <div v-if="items.length" class="carousel-wrap">
    <el-carousel :interval="5000" :height="height" indicator-position="outside" arrow="hover">
      <el-carousel-item v-for="it in items" :key="it.id">
        <component
          :is="it.link_url ? 'a' : 'div'"
          :href="it.link_url || undefined"
          :target="it.link_url && isExternal(it.link_url) ? '_blank' : undefined"
          :rel="it.link_url && isExternal(it.link_url) ? 'noopener noreferrer' : undefined"
          class="slide"
        >
          <img :src="it.image_url" :alt="it.title" class="slide-img" loading="lazy" />
          <div class="slide-title">{{ it.title }}</div>
        </component>
      </el-carousel-item>
    </el-carousel>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { opsApi, type Carousel } from '@/api/ops';

const props = defineProps<{ config?: string }>();
const items = ref<Carousel[]>([]);

// config.height 可调（120~400），默认 220
const height = computed(() => {
  try {
    const n = Number(JSON.parse(props.config || '{}').height);
    if (n >= 120 && n <= 400) return `${n}px`;
  } catch { /* ignore */ }
  return '220px';
});

function isExternal(u: string) {
  return /^https?:\/\//i.test(u || '');
}

onMounted(async () => {
  try {
    const res = await opsApi.carousels();
    items.value = res.data.data?.items || [];
  } catch { /* 轮播不可用不影响首页 */ }
});
</script>

<style scoped>
.carousel-wrap {
  max-width: 1200px;
  margin: 20px auto 8px;
  padding: 0 16px;
}
.slide {
  display: block;
  position: relative;
  width: 100%;
  height: 100%;
  border-radius: 12px;
  overflow: hidden;
}
.slide-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.slide-title {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  padding: 10px 14px;
  color: #fff;
  font-size: 0.92rem;
  background: linear-gradient(transparent, rgb(0 0 0 / 55%));
}
</style>
