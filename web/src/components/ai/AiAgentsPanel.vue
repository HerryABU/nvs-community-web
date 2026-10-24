<template>
  <!-- ===== AI 智能体 =====
       参照参考站的「AI 智能体 / Agent 应用」：卡片墙而非左侧列表，
       每个智能体有自己的定位说明，降低"不知道选哪个"的门槛。 -->
  <div class="ag">
    <div class="ag-head">
      <div>
        <h2 class="ag-title">AI 智能体</h2>
        <p class="ag-sub">
          预设角色与提示词的专属助手，点一个直接开始对话。
          <template v-if="!agents.length">站长还没有创建智能体。</template>
        </p>
      </div>
      <el-input v-model="keyword" class="ag-search" placeholder="搜索智能体" clearable>
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
    </div>

    <div v-if="filtered.length" class="ag-grid">
      <article
        v-for="a in filtered"
        :key="a.id"
        class="ag-card"
        role="button"
        tabindex="0"
        @click="$emit('start', a)"
        @keydown.enter="$emit('start', a)"
        @keydown.space.prevent="$emit('start', a)"
      >
        <span class="ag-emoji">{{ a.emoji || '🤖' }}</span>
        <h3 class="ag-name">{{ a.name }}</h3>
        <p v-if="a.description" class="ag-desc">{{ a.description }}</p>
        <div class="ag-foot">
          <span v-if="a.category" class="ag-cat">{{ a.category }}</span>
          <span v-if="a.upstream === 'coze'" class="ag-coze">Coze</span>
          <span v-if="a.used_count" class="ag-used">{{ formatCount(a.used_count) }} 次使用</span>
        </div>
        <span class="ag-go">开始对话 →</span>
      </article>
    </div>

    <StateBlock
      v-else
      state="empty"
      :title="agents.length ? '没有匹配的智能体' : '还没有智能体'"
      :description="
        agents.length ? '换个关键词再试。' : isAdmin ? '到「AI 智能体」后台创建角色化助手。' : '站长还没有配置智能体。'
      "
      :action-text="!agents.length && isAdmin ? '去创建' : agents.length ? '清空搜索' : ''"
      @action="onEmptyAction"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { Search } from '@element-plus/icons-vue';
import { useRouter } from 'vue-router';
import StateBlock from '@/components/common/StateBlock.vue';
import type { AiAgent } from '@/api/aiHub';

const props = defineProps<{ agents: AiAgent[]; isAdmin: boolean }>();
defineEmits<{ (e: 'start', a: AiAgent): void }>();
const router = useRouter();

const keyword = ref('');
const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  if (!kw) return props.agents;
  return props.agents.filter(
    (a) =>
      a.name.toLowerCase().includes(kw) ||
      (a.category || '').toLowerCase().includes(kw) ||
      (a.description || '').toLowerCase().includes(kw),
  );
});

function formatCount(n?: number) {
  const v = n || 0;
  if (v >= 10000) return `${(v / 10000).toFixed(1)}w`;
  if (v >= 1000) return `${(v / 1000).toFixed(1)}k`;
  return String(v);
}

function onEmptyAction() {
  if (!props.agents.length && props.isAdmin) {
    router.push('/admin/ai-agents');
    return;
  }
  keyword.value = '';
}
</script>

<style scoped>
.ag {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-6, 32px);
  max-width: 1180px;
  width: 100%;
  margin: 0 auto;
  box-sizing: border-box;
}
.ag-head {
  display: flex; flex-wrap: wrap; gap: var(--space-4, 16px);
  align-items: flex-start; justify-content: space-between;
  margin-bottom: var(--space-4, 16px);
}
.ag-title { margin: 0 0 var(--space-1, 4px); font-size: var(--text-xl, 22px); font-weight: 700; color: var(--text-color, #1f2328); }
.ag-sub { margin: 0; font-size: var(--text-sm, 13px); color: var(--text-light, #6b7280); }
.ag-search { max-width: 240px; }

.ag-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: var(--space-4, 16px); }
.ag-card {
  display: flex; flex-direction: column; gap: var(--space-2, 8px);
  padding: var(--space-4, 16px);
  cursor: pointer;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-lg, 16px);
  box-shadow: var(--shadow-xs, 0 1px 3px rgba(0, 0, 0, 0.04));
  transition: transform 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease;
}
.ag-card:hover {
  transform: translateY(-2px);
  border-color: var(--el-color-primary, #6366f1);
  box-shadow: var(--shadow-md, 0 8px 24px rgba(0, 0, 0, 0.08));
}
.ag-card:focus-visible { outline: 2px solid var(--el-color-primary, #6366f1); outline-offset: 2px; }
.ag-emoji { font-size: 26px; line-height: 1; }
.ag-name { margin: 0; font-size: var(--text-base, 14px); font-weight: 600; color: var(--text-color, #1f2328); }
.ag-desc {
  margin: 0; font-size: var(--text-sm, 13px); line-height: 1.55; color: var(--text-light, #6b7280);
  display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden;
}
.ag-foot { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; font-size: var(--text-xs, 12px); }
.ag-cat { padding: 1px 6px; border-radius: 4px; background: var(--bg-page, #f5f6f8); color: var(--text-light, #6b7280); }
.ag-coze { padding: 1px 6px; border-radius: 4px; color: #8b5cf6; background: rgba(139, 92, 246, 0.12); }
.ag-used { color: var(--text-light, #9ca3af); }
.ag-go { margin-top: auto; font-size: var(--text-xs, 12px); color: var(--el-color-primary, #6366f1); }

[data-theme="dark"] .ag-card { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .ag-title, [data-theme="dark"] .ag-name { color: #e2e8f0; }
[data-theme="dark"] .ag-cat { background: #0f172a; }

@media (max-width: 640px) {
  .ag { padding: var(--space-4, 16px); }
  .ag-grid { grid-template-columns: 1fr; }
  .ag-search { max-width: none; width: 100%; }
}
</style>
