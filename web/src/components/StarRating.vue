<template>
  <div class="star-rating">
    <div class="rating-dimensions">
      <div v-for="dim in dimensions" :key="dim.key" class="dim-row">
        <span class="dim-label">{{ dim.label }}</span>
        <div class="dim-stars">
          <el-rate
            v-model="scores[dim.key]"
            :max="5"
            :disabled="disabled"
            @change="emitChange"
            size="small"
          />
        </div>
        <span v-if="stats" class="dim-avg">{{ stats[dim.key]?.toFixed(1) || '-' }}</span>
      </div>
    </div>
    <div v-if="stats" class="rating-overall">
      综合评分：<strong>{{ stats.overall?.toFixed(1) || '-' }}</strong>
      <span class="rating-count">（{{ stats.count }} 人评价）</span>
    </div>
    <el-button
      v-if="!disabled"
      type="primary"
      size="small"
      :loading="submitting"
      @click="submitRating"
    >
      提交评分
    </el-button>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onMounted } from 'vue';
import { novelApi } from '@/api/novel';
import { ElMessage } from 'element-plus';

const props = defineProps<{
  novelId: number;
  disabled?: boolean;
}>();

const emit = defineEmits<{ rated: [] }>();

const dimensions = [
  { key: 'type_completion', label: '类型完成度' },
  { key: 'narrative_quality', label: '叙事质量' },
  { key: 'thought_depth', label: '思想深度' },
  { key: 'community_reputation', label: '社区口碑' },
  { key: 'update_stability', label: '更新稳定性' },
];

interface RatingStats {
  dimensions: Record<string, number>;
  overall: number;
  count: number;
}

const scores = reactive<Record<string, number>>({
  type_completion: 3,
  narrative_quality: 3,
  thought_depth: 3,
  community_reputation: 3,
  update_stability: 3,
});

const stats = ref<RatingStats | null>(null);
const submitting = ref(false);

async function loadStats() {
  try {
    const res = await novelApi.getNovelRating(props.novelId);
    stats.value = res.data.data;
  } catch {}
}

async function loadUserRating() {
  try {
    const res = await novelApi.getUserRating(props.novelId);
    if (res.data.data) {
      const r = res.data.data;
      scores.type_completion = r.type_completion || 3;
      scores.narrative_quality = r.narrative_quality || 3;
      scores.thought_depth = r.thought_depth || 3;
      scores.community_reputation = r.community_reputation || 3;
      scores.update_stability = r.update_stability || 3;
    }
  } catch {}
}

function emitChange() {}

async function submitRating() {
  submitting.value = true;
  try {
    await novelApi.submitRating({
      novel_id: props.novelId,
      ...scores,
    } as any);
    ElMessage.success('评分已提交');
    emit('rated');
    loadStats();
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '评分失败');
  } finally {
    submitting.value = false;
  }
}

watch(() => props.novelId, () => {
  loadStats();
  if (!props.disabled) loadUserRating();
});

onMounted(() => {
  loadStats();
  if (!props.disabled) loadUserRating();
});
</script>

<style scoped>
.star-rating {
  padding: 16px;
  background: #fafafa;
  border-radius: 8px;
}

.dim-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
}

.dim-label {
  width: 80px;
  font-size: 0.85rem;
  color: var(--text-color);
  text-align: right;
}

.dim-avg {
  width: 30px;
  font-size: 0.85rem;
  color: var(--accent-color);
  font-weight: 600;
}

.rating-overall {
  text-align: center;
  margin: 12px 0 8px;
  font-size: 1rem;
}

.rating-count {
  font-size: 0.8rem;
  color: var(--text-light);
}
</style>
