<template>
  <!-- 警务小图标（内联 SVG，避免外链图片）：盾牌 + 五角星，用于公安联网备案号前 -->
  <svg
    class="police-badge"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    aria-hidden="true"
    focusable="false"
  >
    <path
      d="M12 2 4 5v6.09c0 4.64 3.23 8.98 8 10.91 4.77-1.93 8-6.27 8-10.91V5l-8-3Z"
      fill="currentColor"
      fill-opacity="0.12"
      stroke="currentColor"
      stroke-width="1.4"
      stroke-linejoin="round"
    />
    <path
      d="m12 7.6 1.5 3.05 3.37.49-2.44 2.38.58 3.36L12 15.2l-3.01 1.58.58-3.36-2.44-2.38 3.37-.49L12 7.6Z"
      fill="currentColor"
    />
  </svg>
</template>

<script setup lang="ts">
// 公安联网备案号旁的警务标识（纯装饰，不可点击）
withDefaults(defineProps<{ size?: number | string }>(), { size: 14 });
</script>
