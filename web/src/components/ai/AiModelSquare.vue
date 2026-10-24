<template>
  <!-- ===== 大模型广场 =====
       参照 SparkAi 的「大模型广场」：先让人看清"有哪些模型、多少钱、能干什么"，
       再决定用哪个。原来 NVS 只把模型塞在一个下拉框里，用户根本不知道有什么可用。 -->
  <div class="ms">
    <div class="ms-head">
      <div>
        <h2 class="ms-title">大模型广场</h2>
        <p class="ms-sub">
          共 <b>{{ all.length }}</b> 个可用模型
          <template v-if="status.token_enabled">
            · 对话按 tokens 计费，绘画与视频按次计费，余额 <b>{{ status.balance }}</b> {{ status.token_name }}
          </template>
        </p>
      </div>
      <el-input v-model="keyword" class="ms-search" placeholder="搜索模型名称或厂商" clearable>
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
    </div>

    <div class="ms-filters">
      <el-radio-group v-model="kindFilter" size="small">
        <el-radio-button value="all">全部 {{ all.length }}</el-radio-button>
        <el-radio-button value="chat">对话 {{ status.chat_models.length }}</el-radio-button>
        <el-radio-button value="image">绘画 {{ status.image_models.length }}</el-radio-button>
        <el-radio-button value="video">视频 {{ status.video_models.length }}</el-radio-button>
      </el-radio-group>

      <el-select v-model="vendorFilter" size="small" class="ms-vendor" placeholder="全部厂商" clearable>
        <el-option v-for="v in vendors" :key="v" :label="v" :value="v" />
      </el-select>
    </div>

    <div v-if="filtered.length" class="ms-grid">
      <article v-for="m in filtered" :key="`${m.kind}-${m.id}`" class="ms-card">
        <header class="ms-card-head">
          <span class="ms-card-icon" :class="`ms-card-icon--${m.kind}`">{{ kindEmoji(m.kind) }}</span>
          <div class="ms-card-titles">
            <h3 class="ms-card-name">{{ m.name }}</h3>
            <span class="ms-card-vendor">{{ m.vendor || m.provider_hint || '未标注厂商' }}</span>
          </div>
          <el-tag size="small" :type="kindTag(m.kind)">{{ kindLabel(m.kind) }}</el-tag>
        </header>

        <p v-if="m.note" class="ms-card-note">{{ m.note }}</p>
        <code class="ms-card-id">{{ m.model_id }}</code>

        <footer class="ms-card-foot">
          <span class="ms-card-price">{{ priceText(m) }}</span>
          <el-button type="primary" size="small" @click="use(m)">
            {{ m.kind === 'chat' ? '开始对话' : m.kind === 'image' ? '去绘画' : '去创作' }}
          </el-button>
        </footer>
      </article>
    </div>

    <StateBlock
      v-else
      state="empty"
      :title="emptyTitle"
      :description="emptyDesc"
      :action-text="emptyAction"
      @action="onEmptyAction"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { Search } from '@element-plus/icons-vue';
import { useRouter } from 'vue-router';
import StateBlock from '@/components/common/StateBlock.vue';
import type { AiHubStatus } from '@/api/aiHub';

type Kind = 'chat' | 'image' | 'video';
interface Row {
  id: number;
  name: string;
  model_id: string;
  kind: Kind;
  price_in?: number;
  price_out?: number;
  price_call?: number;
  max_tokens?: number;
  vendor?: string;
  provider_hint?: string;
  note?: string;
}

const props = defineProps<{ status: AiHubStatus; isAdmin: boolean }>();
const emit = defineEmits<{ (e: 'use', payload: { kind: Kind; modelId: number }): void }>();
const router = useRouter();

const keyword = ref('');
const kindFilter = ref<'all' | Kind>('all');
const vendorFilter = ref('');

const all = computed<Row[]>(() => [
  ...props.status.chat_models.map((m: any) => ({ ...m, kind: 'chat' as Kind })),
  ...props.status.image_models.map((m: any) => ({ ...m, kind: 'image' as Kind })),
  ...props.status.video_models.map((m: any) => ({ ...m, kind: 'video' as Kind })),
]);

const vendors = computed(() => {
  const set = new Set<string>();
  for (const m of all.value) {
    const v = (m.vendor || '').trim();
    if (v) set.add(v);
  }
  return Array.from(set).sort();
});

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  return all.value.filter((m) => {
    if (kindFilter.value !== 'all' && m.kind !== kindFilter.value) return false;
    if (vendorFilter.value && (m.vendor || '').trim() !== vendorFilter.value) return false;
    if (!kw) return true;
    return (
      m.name.toLowerCase().includes(kw) ||
      m.model_id.toLowerCase().includes(kw) ||
      (m.vendor || '').toLowerCase().includes(kw)
    );
  });
});

/** AI 未开启 / 已开启但无模型 / 筛选无果 —— 三种空态的正确下一步各不相同 */
const emptyTitle = computed(() => {
  if (filtered.value.length) return '';
  if (all.value.length) return '没有匹配的模型';
  if (!props.status.ai_enabled) return 'AI 功能还没开启';
  return '还没有可用模型';
});
const emptyDesc = computed(() => {
  if (all.value.length) return '换个关键词或厂商再试。';
  if (!props.status.ai_enabled) {
    return props.isAdmin
      ? 'AI 是默认关闭的增值能力：先在「AI 功能配置」打开总开关，再到「AI 模型池」添加一个模型。'
      : '站长还没有开放 AI 功能，稍后再来看看。';
  }
  return props.isAdmin
    ? '到「AI 模型池」用预设一键添加模型，填入上游密钥后把「启用」打开。'
    : '站长还没有配置可用模型，稍后再来。';
});
const emptyAction = computed(() => {
  if (all.value.length) return '清空筛选';
  if (!props.isAdmin) return '';
  return props.status.ai_enabled ? '去配置模型' : '去开启 AI';
});

function kindLabel(k: Kind) {
  return { chat: '对话', image: '绘画', video: '视频' }[k];
}
function kindEmoji(k: Kind) {
  return { chat: '💬', image: '🎨', video: '🎬' }[k];
}
function kindTag(k: Kind): any {
  return { chat: 'primary', image: 'success', video: 'warning' }[k];
}
function priceText(m: Row) {
  if (m.kind === 'chat') {
    const pin = m.price_in || 0;
    const pout = m.price_out || 0;
    return pin === 0 && pout === 0 ? '免费' : `${pin} / ${pout} 代币 · 每 1K tokens`;
  }
  const pc = m.price_call || 0;
  return pc === 0 ? '免费' : `${pc} 代币 / 次`;
}

function use(m: Row) {
  emit('use', { kind: m.kind, modelId: m.id });
}

function onEmptyAction() {
  if (!all.value.length && props.isAdmin) {
    router.push(props.status.ai_enabled ? '/admin/ai-pool' : '/admin/ai-features');
    return;
  }
  keyword.value = '';
  kindFilter.value = 'all';
  vendorFilter.value = '';
}
</script>

<style scoped>
.ms {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-6, 32px);
  max-width: 1180px;
  width: 100%;
  margin: 0 auto;
  box-sizing: border-box;
}
.ms-head {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4, 16px);
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: var(--space-4, 16px);
}
.ms-title { margin: 0 0 var(--space-1, 4px); font-size: var(--text-xl, 22px); font-weight: 700; color: var(--text-color, #1f2328); }
.ms-sub { margin: 0; font-size: var(--text-sm, 13px); color: var(--text-light, #6b7280); }
.ms-sub b { color: var(--text-color, #1f2328); }
.ms-search { max-width: 280px; }

.ms-filters {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3, 12px);
  align-items: center;
  margin-bottom: var(--space-4, 16px);
}
.ms-vendor { width: 160px; }

.ms-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: var(--space-4, 16px);
}
.ms-card {
  display: flex;
  flex-direction: column;
  gap: var(--space-2, 8px);
  padding: var(--space-4, 16px);
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-lg, 16px);
  box-shadow: var(--shadow-xs, 0 1px 3px rgba(0, 0, 0, 0.04));
  transition: border-color 0.16s ease, box-shadow 0.16s ease;
}
.ms-card:hover { border-color: var(--el-color-primary, #6366f1); box-shadow: var(--shadow-md, 0 8px 24px rgba(0, 0, 0, 0.08)); }
.ms-card-head { display: flex; align-items: center; gap: var(--space-2, 8px); }
.ms-card-icon {
  width: 34px; height: 34px; flex-shrink: 0;
  display: inline-flex; align-items: center; justify-content: center;
  border-radius: var(--radius-md, 10px);
  font-size: 17px;
  background: var(--bg-page, #f5f6f8);
}
.ms-card-titles { flex: 1; min-width: 0; }
.ms-card-name {
  margin: 0;
  font-size: var(--text-base, 14px);
  font-weight: 600;
  color: var(--text-color, #1f2328);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.ms-card-vendor { font-size: var(--text-xs, 12px); color: var(--text-light, #9ca3af); }
.ms-card-note {
  margin: 0;
  font-size: var(--text-sm, 13px);
  line-height: 1.55;
  color: var(--text-light, #6b7280);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.ms-card-id {
  font-size: 11px;
  color: var(--text-light, #9ca3af);
  background: var(--bg-page, #f5f6f8);
  padding: 2px 6px;
  border-radius: 4px;
  align-self: flex-start;
  max-width: 100%;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.ms-card-foot {
  margin-top: auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2, 8px);
  padding-top: var(--space-2, 8px);
}
.ms-card-price { font-size: var(--text-xs, 12px); color: var(--text-light, #6b7280); }

[data-theme="dark"] .ms-card { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .ms-title,
[data-theme="dark"] .ms-card-name { color: #e2e8f0; }
[data-theme="dark"] .ms-card-icon,
[data-theme="dark"] .ms-card-id { background: #0f172a; }

@media (max-width: 640px) {
  .ms { padding: var(--space-4, 16px); }
  .ms-grid { grid-template-columns: 1fr; }
  .ms-search { max-width: none; width: 100%; }
}
</style>
