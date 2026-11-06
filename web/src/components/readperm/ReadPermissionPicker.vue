<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { readPermApi, type ReadPermLevel } from '@/api/readperm';

// 作者端共用的阅读权限选择器（小说 Editor / 博客 BlogEditor 复用）。
// 可选项来自 /api/read-permission/levels，随站长配置实时变化。
const props = withDefaults(
  defineProps<{
    modelValue?: string;
    price?: number;
    disabled?: boolean;
  }>(),
  { modelValue: 'public', price: 0, disabled: false },
);

const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void;
  (e: 'update:price', v: number): void;
}>();

const levels = ref<ReadPermLevel[]>([]);
const minPrice = ref(1);
const loaded = ref(false);

onMounted(async () => {
  try {
    const res = await readPermApi.levels();
    const d = res.data?.data;
    levels.value = d?.levels || [];
    minPrice.value = d?.paid_min_price ?? 1;
    if (props.modelValue === 'paid' && (props.price ?? 0) < minPrice.value) {
      emit('update:price', minPrice.value);
    }
  } catch {
    // 网络异常时退化为「公开」，不让编辑器卡在空下拉上
    levels.value = [{ key: 'public', label: '公开' }];
  } finally {
    loaded.value = true;
  }
});

const level = computed({
  get: () => props.modelValue || 'public',
  set: (v: string) => emit('update:modelValue', v),
});
const priceVal = computed({
  get: () => props.price ?? 0,
  set: (v: number) => emit('update:price', v),
});

// 站长后来停用了作者原先选的等级：不静默改写数据，明确提示并要求重选。
const stale = computed(
  () =>
    loaded.value &&
    levels.value.length > 0 &&
    !levels.value.some((l) => l.key === level.value),
);

const hints: Record<string, string> = {
  public: '所有人可见，包括未登录访客',
  login: '登录用户可见，访客需先登录',
  follower: '仅关注了你的用户可见',
  vip: '达到站长设定的会员等级后可见',
  paid: '读者购买本内容后可见；售价不得低于站长设定的最低价',
  private: '仅你自己与平台管理员可见，且不会出现在任何列表页',
};
const hint = computed(() => hints[level.value] || '');
</script>

<template>
  <div class="rpp">
    <div class="rpp-row">
      <el-select v-model="level" :disabled="disabled" style="width: 210px" placeholder="选择阅读权限">
        <el-option v-for="l in levels" :key="l.key" :label="l.label" :value="l.key" />
        <el-option v-if="stale" :label="`${level}（站长已停用）`" :value="level" disabled />
      </el-select>
      <template v-if="level === 'paid'">
        <el-input-number
          v-model="priceVal"
          :min="minPrice"
          :step="1"
          :disabled="disabled"
          controls-position="right"
          style="width: 150px"
        />
        <span class="rpp-unit">代币 · 最低 {{ minPrice }}</span>
      </template>
    </div>
    <div class="rpp-hint">{{ hint }}</div>
    <el-alert
      v-if="stale"
      class="rpp-alert"
      type="warning"
      :closable="false"
      show-icon
      title="该等级已被站长停用"
      description="保存会校验失败，请改选其它等级；历史内容会自动降级为「登录可见」，不会被锁死。"
    />
  </div>
</template>

<style scoped>
.rpp { width: 100%; }
.rpp-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.rpp-unit { font-size: 0.8rem; color: var(--text-light, #999); }
.rpp-hint { margin-top: 6px; font-size: 0.8rem; color: var(--text-light, #999); line-height: 1.5; }
.rpp-alert { margin-top: 8px; }
</style>
