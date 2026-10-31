<template>
  <div class="pricing-page">
    <div class="page-container">
      <h1 class="p-title">AI 模型价格</h1>
      <p class="p-sub">
        价格单位是<b>站内代币</b>：对话按 tokens 计（每 1K），绘画与视频按次计。
        实际扣费以调用返回的用量为准；会员与每日免费额度可抵扣。
      </p>

      <el-alert v-if="!data.models.length && !loading" type="info" :closable="false" class="p-empty">
        站长还没有开启任何 AI 模型。
      </el-alert>

      <div v-for="grp in groups" :key="grp.kind" class="p-group">
        <h2 class="p-group-title">{{ grp.title }}</h2>
        <p class="p-group-note">{{ grp.note }}</p>
        <el-table :data="grp.items" size="small" stripe class="p-table">
          <el-table-column prop="name" label="模型" min-width="180" show-overflow-tooltip />
          <el-table-column v-if="grp.kind === 'chat'" label="输入" width="140">
            <template #default="{ row }">{{ row.price_in }} <span class="p-unit">代币 / 1K</span></template>
          </el-table-column>
          <el-table-column v-if="grp.kind === 'chat'" label="输出" width="140">
            <template #default="{ row }">{{ row.price_out }} <span class="p-unit">代币 / 1K</span></template>
          </el-table-column>
          <el-table-column v-if="grp.kind !== 'chat'" label="每次" width="140">
            <template #default="{ row }">{{ row.price_call }} <span class="p-unit">代币 / 次</span></template>
          </el-table-column>
          <el-table-column label="说明" min-width="160">
            <template #default="{ row }">
              <span v-if="row.price_in === 0 && row.price_out === 0 && row.price_call === 0" class="p-free">免费</span>
              <span v-else class="p-unit">按量结算</span>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import api from '@/api';

interface PItem { id: number; name: string; kind: string; price_in: number; price_out: number; price_call: number }
const data = ref<{ models: PItem[]; categories: any; unit: any }>({ models: [], categories: {}, unit: {} });
const loading = ref(false);

const groups = computed(() => {
  const pick = (k: string, title: string, note: string) => ({
    kind: k, title, note,
    items: data.value.models.filter((m) => m.kind === k),
  });
  return [
    pick('chat', '对话模型', '按输入 / 输出 tokens 分别计价，每 1K tokens 的代币数'),
    pick('image', '绘画模型', '按次计价，每次生成一张图'),
    pick('video', '视频模型', '按次计价，每次生成一段视频'),
  ].filter((g) => g.items.length > 0);
});

onMounted(async () => {
  loading.value = true;
  try {
    const res = await api.get<{ code: number; data: any }>('/ai/pricing');
    if (res.data.code === 0) data.value = res.data.data;
  } catch {
    /* 未开启 AI 或接口不可用时保持空态 */
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.pricing-page { min-height: 60vh; padding: 32px 0 60px; background: var(--bg-page, #f5f7fa); }
.page-container { max-width: 1100px; margin: 0 auto; padding: 0 20px; }
.p-title { margin: 0 0 8px; font-size: 1.6rem; color: var(--text-main, #1e293b); }
.p-sub { margin: 0 0 22px; font-size: 0.86rem; line-height: 1.8; color: var(--text-light, #64748b); }
.p-empty { margin-bottom: 16px; }
.p-group { margin-bottom: 26px; }
.p-group-title { margin: 0 0 4px; font-size: 1.05rem; color: var(--text-main, #1e293b); }
.p-group-note { margin: 0 0 10px; font-size: 0.76rem; color: var(--text-light, #94a3b8); }
.p-table { background: var(--bg-card, #fff); border-radius: 10px; overflow: hidden; }
.p-unit { font-size: 0.74rem; color: var(--text-light, #94a3b8); }
.p-free { font-size: 0.78rem; color: var(--el-color-success, #67c23a); font-weight: 600; }
</style>
