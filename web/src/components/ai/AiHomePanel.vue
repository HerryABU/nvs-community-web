<template>
  <!-- ===== AI 工作台门户 =====
       设计参照 SparkAi 类站点的入口式布局：用户进来先看到"能干什么"，
       而不是一上来就被丢进一个对话框或一句"暂未开启"。
       视觉全部走 NVS 的设计变量（--bg-card / --radius-* / --gradient-*），不引入外部皮肤。 -->
  <div class="ai-home">
    <!-- 状态条：开关、余额、免费额度 -->
    <section class="ai-hero">
      <div class="ai-hero-main">
        <h1 class="ai-hero-title">AI 工作台</h1>
        <p class="ai-hero-sub">
          对话、绘画、视频、智能体与思维导图，统一在一个地方。
          <template v-if="status.ai_enabled">当前可用 {{ modelCount }} 个模型。</template>
        </p>
        <div class="ai-hero-tags">
          <span v-if="status.ai_enabled" class="ai-pill ai-pill--on">● AI 已开启</span>
          <span v-else class="ai-pill ai-pill--off">● AI 未开启</span>
          <span v-if="status.token_enabled" class="ai-pill">
            余额 <b>{{ status.balance }}</b> {{ status.token_name }}
          </span>
          <span v-if="status.free_quota?.enabled" class="ai-pill ai-pill--free">
            今日免费
            {{ status.free_quota.calls_left < 0 ? '不限' : status.free_quota.calls_left }} 次
          </span>
        </div>
      </div>

      <!-- 未开启时：管理员就地拿到「怎么开」，普通用户知道「找谁开」 -->
      <div v-if="!status.ai_enabled" class="ai-hero-cta">
        <template v-if="isAdmin">
          <p class="ai-hero-cta-tip">AI 默认关闭。先开总开关，再添加一个模型即可使用。</p>
          <div class="ai-hero-cta-btns">
            <el-button type="primary" @click="$emit('admin', 'features')">去开启 AI</el-button>
            <el-button @click="$emit('admin', 'pool')">配置模型池</el-button>
          </div>
        </template>
        <template v-else>
          <p class="ai-hero-cta-tip">站长还没有开放 AI 功能，可以先逛逛灵感广场。</p>
          <el-button @click="$emit('navigate', 'gallery')">看看灵感广场</el-button>
        </template>
      </div>
    </section>

    <!-- 功能入口宫格 -->
    <section class="ai-grid">
      <button
        v-for="entry in entries"
        :key="entry.key"
        type="button"
        class="ai-entry"
        :class="{ 'ai-entry--dim': entry.needsAi && !status.ai_enabled }"
        @click="$emit('navigate', entry.key)"
      >
        <span class="ai-entry-icon" :style="{ background: entry.gradient }">
          <span class="ai-entry-emoji">{{ entry.emoji }}</span>
        </span>
        <span class="ai-entry-body">
          <span class="ai-entry-head">
            <span class="ai-entry-title">{{ entry.title }}</span>
            <span v-if="entry.badge" class="ai-entry-badge">{{ entry.badge }}</span>
          </span>
          <span class="ai-entry-desc">{{ entry.desc }}</span>
          <span v-if="entry.needsAi && !status.ai_enabled" class="ai-entry-lock">未开启</span>
          <span v-else-if="entry.meta" class="ai-entry-meta">{{ entry.meta }}</span>
        </span>
        <el-icon class="ai-entry-arrow"><ArrowRight /></el-icon>
      </button>
    </section>

    <!-- 最近会话：让门户不只是"菜单页" -->
    <section v-if="sessions.length" class="ai-recent">
      <div class="ai-recent-head">
        <span class="ai-recent-title">最近对话</span>
        <el-button text type="primary" size="small" @click="$emit('navigate', 'chat')">全部</el-button>
      </div>
      <div class="ai-recent-list">
        <button
          v-for="s in sessions.slice(0, 4)"
          :key="s.id"
          type="button"
          class="ai-recent-item"
          @click="$emit('open-session', s)"
        >
          <span class="ai-recent-name">{{ s.title || '新对话' }}</span>
          <span class="ai-recent-agent">{{ s.agent_name || 'AI 对话' }}</span>
        </button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { ArrowRight } from '@element-plus/icons-vue';
import type { AiHubStatus, AiAgent, AiChatSession } from '@/api/aiHub';

const props = defineProps<{
  status: AiHubStatus;
  agents: AiAgent[];
  sessions: AiChatSession[];
  isAdmin: boolean;
}>();

defineEmits<{
  (e: 'navigate', tab: string): void;
  (e: 'open-session', s: AiChatSession): void;
  (e: 'admin', to: 'features' | 'pool'): void;
}>();

const modelCount = computed(
  () => props.status.chat_models.length + props.status.image_models.length + props.status.video_models.length,
);

interface Entry {
  key: string;
  title: string;
  desc: string;
  emoji: string;
  gradient: string;
  badge?: string;
  meta?: string;
  /** 依赖 AI 总开关：未开启时卡片置灰并标「未开启」，但**仍然可点**（点进去看引导） */
  needsAi?: boolean;
}

const entries = computed<Entry[]>(() => [
  {
    key: 'chat',
    title: 'AI 对话',
    desc: '与模型实时对话，支持流式输出、上下文与智能体',
    emoji: '💬',
    gradient: 'var(--gradient-primary)',
    badge: '常用',
    meta: props.status.chat_models.length ? `${props.status.chat_models.length} 个对话模型` : '',
    needsAi: true,
  },
  {
    key: 'models',
    title: '大模型广场',
    desc: '按类型与厂商浏览可用模型，选一个直接开聊',
    emoji: '🧠',
    gradient: 'var(--gradient-blue)',
    meta: modelCount.value ? `共 ${modelCount.value} 个` : '暂无模型',
    needsAi: true,
  },
  {
    key: 'image',
    title: '专业绘画',
    desc: '文生图与图生图，支持尺寸、风格与批量出图',
    emoji: '🎨',
    gradient: 'var(--gradient-purple)',
    badge: 'HOT',
    meta: props.status.image_models.length ? `${props.status.image_models.length} 个绘画模型` : '',
    needsAi: true,
  },
  {
    key: 'video',
    title: '视频创作',
    desc: '文生视频与图生视频，异步任务与进度回显',
    emoji: '🎬',
    gradient: 'var(--gradient-rose)',
    meta: props.status.video_models.length ? `${props.status.video_models.length} 个视频模型` : '',
    needsAi: true,
  },
  {
    key: 'gallery',
    title: '灵感广场',
    desc: '浏览全站公开作品，一键复刻同款提示词',
    emoji: '✨',
    gradient: 'var(--gradient-amber)',
    badge: 'New',
  },
  {
    key: 'agents',
    title: 'AI 智能体',
    desc: '写作教练、世界观设定、资料考据等角色化助手',
    emoji: '🤖',
    gradient: 'var(--gradient-teal)',
    meta: props.agents.length ? `${props.agents.length} 个智能体` : '',
  },
  {
    key: 'mind',
    title: '思维导图',
    desc: '把大纲或设定整理成结构化脑图',
    emoji: '🕸️',
    gradient: 'var(--gradient-indigo)',
    needsAi: true,
  },
  {
    key: 'membership',
    title: '会员与额度',
    desc: '查看余额与消耗明细，充值或配置自己的上游密钥',
    emoji: '💎',
    gradient: 'var(--gradient-success)',
    meta: props.status.token_enabled ? `余额 ${props.status.balance} ${props.status.token_name}` : '',
  },
]);
</script>

<style scoped>
.ai-home {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-6, 32px);
  max-width: 1180px;
  width: 100%;
  margin: 0 auto;
  box-sizing: border-box;
}

/* ---------- 状态条 ---------- */
.ai-hero {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-5, 24px);
  align-items: center;
  justify-content: space-between;
  padding: var(--space-5, 24px);
  border-radius: var(--radius-lg, 16px);
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  box-shadow: var(--shadow-sm, 0 2px 8px rgba(0, 0, 0, 0.05));
  margin-bottom: var(--space-5, 24px);
}
.ai-hero-main { min-width: 0; }
.ai-hero-title {
  margin: 0 0 var(--space-1, 4px);
  font-size: var(--text-2xl, 28px);
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--text-color, #1f2328);
}
.ai-hero-sub {
  margin: 0 0 var(--space-3, 12px);
  font-size: var(--text-base, 14px);
  color: var(--text-light, #6b7280);
  line-height: 1.6;
}
.ai-hero-tags { display: flex; flex-wrap: wrap; gap: var(--space-2, 8px); }
.ai-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 10px;
  border-radius: 999px;
  font-size: var(--text-xs, 12px);
  color: var(--text-light, #6b7280);
  background: var(--bg-page, #f5f6f8);
  border: 1px solid var(--border-color, #e5e7eb);
}
.ai-pill b { color: var(--text-color, #1f2328); }
.ai-pill--on { color: #16a34a; background: rgba(22, 163, 74, 0.1); border-color: rgba(22, 163, 74, 0.25); }
.ai-pill--off { color: #b45309; background: rgba(180, 83, 9, 0.1); border-color: rgba(180, 83, 9, 0.25); }
.ai-pill--free { color: #2563eb; background: rgba(37, 99, 235, 0.08); border-color: rgba(37, 99, 235, 0.2); }

.ai-hero-cta { max-width: 420px; }
.ai-hero-cta-tip {
  margin: 0 0 var(--space-3, 12px);
  font-size: var(--text-sm, 13px);
  color: var(--text-light, #6b7280);
  line-height: 1.6;
}
.ai-hero-cta-btns { display: flex; gap: var(--space-2, 8px); flex-wrap: wrap; }

/* ---------- 入口宫格 ---------- */
.ai-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(268px, 1fr));
  gap: var(--space-4, 16px);
}
.ai-entry {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3, 12px);
  padding: var(--space-4, 16px);
  text-align: left;
  cursor: pointer;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-lg, 16px);
  box-shadow: var(--shadow-xs, 0 1px 3px rgba(0, 0, 0, 0.04));
  transition: transform 0.16s ease, box-shadow 0.16s ease, border-color 0.16s ease;
  font-family: inherit;
}
.ai-entry:hover {
  transform: translateY(-2px);
  border-color: var(--el-color-primary, #6366f1);
  box-shadow: var(--shadow-md, 0 8px 24px rgba(0, 0, 0, 0.1));
}
.ai-entry:focus-visible { outline: 2px solid var(--el-color-primary, #6366f1); outline-offset: 2px; }
.ai-entry--dim { opacity: 0.82; }

.ai-entry-icon {
  flex-shrink: 0;
  width: 44px;
  height: 44px;
  border-radius: var(--radius-md, 10px);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.ai-entry-emoji { font-size: 22px; line-height: 1; }

.ai-entry-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 3px; }
.ai-entry-head { display: flex; align-items: center; gap: 6px; }
.ai-entry-title {
  font-size: var(--text-md, 16px);
  font-weight: 600;
  color: var(--text-color, #1f2328);
}
.ai-entry-badge {
  font-size: 10px;
  line-height: 1.5;
  padding: 0 5px;
  border-radius: 4px;
  color: #fff;
  background: var(--gradient-danger, linear-gradient(135deg, #f43f5e, #e11d48));
}
.ai-entry-desc {
  font-size: var(--text-sm, 13px);
  line-height: 1.55;
  color: var(--text-light, #6b7280);
}
.ai-entry-meta { font-size: var(--text-xs, 12px); color: var(--el-color-primary, #6366f1); }
.ai-entry-lock { font-size: var(--text-xs, 12px); color: #b45309; }
.ai-entry-arrow {
  flex-shrink: 0;
  align-self: center;
  color: var(--text-light, #9ca3af);
  transition: transform 0.16s ease;
}
.ai-entry:hover .ai-entry-arrow { transform: translateX(3px); color: var(--el-color-primary, #6366f1); }

/* ---------- 最近对话 ---------- */
.ai-recent { margin-top: var(--space-6, 32px); }
.ai-recent-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-3, 12px); }
.ai-recent-title { font-size: var(--text-md, 16px); font-weight: 600; color: var(--text-color, #1f2328); }
.ai-recent-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: var(--space-3, 12px); }
.ai-recent-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: var(--space-3, 12px);
  text-align: left;
  cursor: pointer;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-md, 10px);
  font-family: inherit;
  transition: border-color 0.16s ease;
}
.ai-recent-item:hover { border-color: var(--el-color-primary, #6366f1); }
.ai-recent-name {
  font-size: var(--text-base, 14px);
  color: var(--text-color, #1f2328);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ai-recent-agent { font-size: var(--text-xs, 12px); color: var(--text-light, #9ca3af); }

/* ---------- 深色 ---------- */
[data-theme="dark"] .ai-hero,
[data-theme="dark"] .ai-entry,
[data-theme="dark"] .ai-recent-item { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .ai-hero-title,
[data-theme="dark"] .ai-entry-title,
[data-theme="dark"] .ai-recent-title,
[data-theme="dark"] .ai-recent-name { color: #e2e8f0; }
[data-theme="dark"] .ai-pill { background: #0f172a; border-color: #334155; }

/* ---------- 窄屏 ---------- */
@media (max-width: 640px) {
  .ai-home { padding: var(--space-4, 16px); }
  .ai-grid { grid-template-columns: 1fr; }
  .ai-hero { flex-direction: column; align-items: flex-start; }
}
</style>
