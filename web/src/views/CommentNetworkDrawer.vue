<template>
  <el-drawer v-model="visible" :title="title" size="640px" direction="rtl">
    <CommentNetworkView :target-type="targetType" :target-i-d="targetID" />
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import CommentNetworkView from '@/components/CommentNetworkView.vue';

const props = defineProps<{
  targetType: string;
  targetID: number;
  modelValue: boolean;
}>();
const emit = defineEmits<{ 'update:modelValue': [v: boolean] }>();

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
});
const title = computed(() => (props.targetType === 'blog' ? '博客评论网分析' : '作品评论网分析'));
</script>
