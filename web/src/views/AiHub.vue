<template>
  <div class="ai-hub">
    <!-- ===== 左侧栏：新建 / 智能体 / 历史会话 ===== -->
    <aside class="ai-hub-side" :class="{ 'ai-hub-side-open': mobileSideOpen }">
      <div class="ai-hub-side-head">
        <el-button type="primary" class="ai-hub-new" @click="newChat">
          <el-icon><Plus /></el-icon> 新建对话
        </el-button>
        <el-button class="ai-hub-side-close" text @click="mobileSideOpen = false">
          <el-icon><Close /></el-icon>
        </el-button>
      </div>

      <div class="ai-hub-side-scroll">
        <div class="ai-hub-side-title">智能体</div>
        <div
          v-for="a in agents"
          :key="a.id"
          :class="['ai-agent-item', { active: activeAgentId === a.id && !activeSessionId }]"
          @click="pickAgent(a)"
        >
          <span class="ai-agent-emoji">{{ a.emoji || '🤖' }}</span>
          <span class="ai-agent-name">{{ a.name }}</span>
        </div>

        <div class="ai-hub-side-title">历史会话</div>
        <div
          v-for="s in sessions"
          :key="s.id"
          :class="['ai-session-item', { active: activeSessionId === s.id }]"
          @click="openSession(s)"
        >
          <span class="ai-session-title">{{ s.title || '新对话' }}</span>
          <el-icon class="ai-session-del" @click.stop="deleteSession(s)"><Delete /></el-icon>
        </div>
        <el-empty v-if="!sessions.length" description="暂无会话" :image-size="50" />
      </div>

      <div class="ai-hub-side-foot">
        <span>余额 {{ status.balance }} {{ status.token_name }}</span>
        <el-button text type="primary" size="small" @click="$router.push('/wallet')">充值</el-button>
      </div>
    </aside>
    <div v-if="mobileSideOpen" class="ai-hub-mask" @click="mobileSideOpen = false" />

    <!-- ===== 主区 ===== -->
    <main class="ai-hub-main">
      <div class="ai-hub-topbar">
        <el-button class="ai-hub-menu-btn" text @click="mobileSideOpen = true">
          <el-icon><Menu /></el-icon>
        </el-button>
        <el-radio-group v-model="activeTab" size="small">
          <el-radio-button value="chat"><el-icon><ChatDotRound /></el-icon> 对话</el-radio-button>
          <el-radio-button value="image"><el-icon><Picture /></el-icon> 绘画</el-radio-button>
          <el-radio-button value="video"><el-icon><VideoCamera /></el-icon> 视频</el-radio-button>
        </el-radio-group>
        <span v-if="activeTab === 'chat' && activeSession" class="ai-hub-current">
          {{ activeSession.agent_name || 'AI 对话' }}
        </span>
      </div>

      <!-- 未开启引导 -->
      <div v-if="!loading && !status.ai_enabled" class="ai-hub-blocked">
        <el-empty description="站长暂未开启 AI 功能">
          <template #description>
            <p>AI 工作台包含对话、绘画、视频与智能体能力，开启后即可使用。</p>
          </template>
        </el-empty>
      </div>
      <div v-else-if="!loading && noModels" class="ai-hub-blocked">
        <el-empty description="暂无可用模型">
          <template #description>
            <p>站长尚未在「AI 模型池」中配置启用的模型。</p>
          </template>
        </el-empty>
      </div>

      <template v-else>
        <AiChatPanel
          v-if="activeTab === 'chat'"
          :key="'chat-' + (activeSessionId || 'new-' + activeAgentId)"
          :status="status"
          :agents="agents"
          :session-id="activeSessionId"
          :agent-id="activeAgentId"
          @session-created="onSessionCreated"
          @balance-updated="onBalanceUpdated"
          @sessions-changed="loadSessions"
        />
        <AiImagePanel v-else-if="activeTab === 'image'" :status="status" />
        <AiVideoPanel v-else :status="status" />
      </template>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus, Close, Delete, Menu, ChatDotRound, Picture, VideoCamera } from '@element-plus/icons-vue';
import { aiHubApi, type AiHubStatus, type AiAgent, type AiChatSession } from '@/api/aiHub';
import AiChatPanel from '@/components/ai/AiChatPanel.vue';
import AiImagePanel from '@/components/ai/AiImagePanel.vue';
import AiVideoPanel from '@/components/ai/AiVideoPanel.vue';

const status = reactive<AiHubStatus>({
  ai_enabled: false,
  token_enabled: true,
  token_name: '代币',
  balance: 0,
  chat_models: [],
  image_models: [],
  video_models: [],
});
const agents = ref<AiAgent[]>([]);
const sessions = ref<AiChatSession[]>([]);
const activeSessionId = ref<number | null>(null);
const activeAgentId = ref(0);
const activeTab = ref<'chat' | 'image' | 'video'>('chat');
const mobileSideOpen = ref(false);
const loading = ref(true);

const activeSession = computed(() => sessions.value.find((s) => s.id === activeSessionId.value) || null);
const noModels = computed(
  () => status.chat_models.length + status.image_models.length + status.video_models.length === 0,
);

async function loadAll() {
  loading.value = true;
  try {
    const [st, ag] = await Promise.all([aiHubApi.status(), aiHubApi.agents()]);
    if (st.data.code === 0) Object.assign(status, st.data.data);
    if (ag.data.code === 0) agents.value = ag.data.data.list || [];
  } catch { /* ignore */ }
  loading.value = false;
}

async function loadSessions() {
  try {
    const res = await aiHubApi.sessions();
    if (res.data.code === 0) sessions.value = res.data.data.list || [];
  } catch { /* ignore */ }
}

function newChat() {
  activeSessionId.value = null;
  activeAgentId.value = 0;
  activeTab.value = 'chat';
  mobileSideOpen.value = false;
}

function pickAgent(a: AiAgent) {
  activeSessionId.value = null;
  activeAgentId.value = a.id;
  activeTab.value = 'chat';
  mobileSideOpen.value = false;
}

function openSession(s: AiChatSession) {
  activeSessionId.value = s.id;
  activeAgentId.value = s.agent_id || 0;
  activeTab.value = 'chat';
  mobileSideOpen.value = false;
}

function onSessionCreated(id: number) {
  activeSessionId.value = id;
  loadSessions();
}

function onBalanceUpdated(balance: number) {
  status.balance = balance;
}

async function deleteSession(s: AiChatSession) {
  try {
    await ElMessageBox.confirm(`确定删除会话「${s.title || '新对话'}」？`, '删除确认', { type: 'warning' });
  } catch { return; }
  try {
    const res = await aiHubApi.deleteSession(s.id);
    if (res.data.code === 0) {
      ElMessage.success('已删除');
      if (activeSessionId.value === s.id) newChat();
      await loadSessions();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

onMounted(() => {
  loadAll();
  loadSessions();
});
</script>

<style scoped>
.ai-hub { display: flex; height: calc(100vh - 60px); min-height: 0; background: var(--bg-page, #f5f7fa); }
.ai-hub-side {
  width: 250px; flex-shrink: 0; display: flex; flex-direction: column;
  background: var(--el-bg-color, #fff); border-right: 1px solid var(--el-border-color, #ebeef5);
}
.ai-hub-side-head { display: flex; gap: 8px; padding: 12px; border-bottom: 1px solid var(--el-border-color, #ebeef5); }
.ai-hub-new { flex: 1; }
.ai-hub-side-close { display: none; }
.ai-hub-side-scroll { flex: 1; overflow-y: auto; padding: 8px; min-height: 0; }
.ai-hub-side-title {
  font-size: 0.72rem; color: var(--text-light, #999);
  padding: 10px 8px 4px; letter-spacing: 0.05em;
}
.ai-agent-item, .ai-session-item {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 10px; border-radius: 8px; cursor: pointer;
  font-size: 0.85rem; color: var(--text-color, #333);
}
.ai-agent-item:hover, .ai-session-item:hover { background: var(--el-fill-color-light, #f5f7fa); }
.ai-agent-item.active, .ai-session-item.active {
  background: var(--el-color-primary-light-9, #ecf5ff);
  color: var(--el-color-primary, #409eff);
}
.ai-agent-emoji { font-size: 1.1rem; }
.ai-agent-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ai-session-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ai-session-del { display: none; color: #ef4444; flex-shrink: 0; }
.ai-session-item:hover .ai-session-del { display: inline-block; }
.ai-hub-side-foot {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 14px; border-top: 1px solid var(--el-border-color, #ebeef5);
  font-size: 0.8rem; color: var(--text-light, #666);
}
.ai-hub-mask { display: none; }
.ai-hub-main { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.ai-hub-topbar {
  display: flex; align-items: center; gap: 12px;
  padding: 10px 16px; background: var(--el-bg-color, #fff);
  border-bottom: 1px solid var(--el-border-color, #ebeef5);
}
.ai-hub-menu-btn { display: none; }
.ai-hub-current { font-size: 0.82rem; color: var(--text-light, #666); }
.ai-hub-blocked { flex: 1; display: flex; align-items: center; justify-content: center; }

/* ===== 窄屏：侧栏抽屉化 ===== */
@media (max-width: 860px) {
  .ai-hub-side {
    position: fixed; top: 0; bottom: 0; left: 0; z-index: 20;
    transform: translateX(-100%); transition: transform 0.25s;
    box-shadow: 2px 0 12px rgba(0, 0, 0, 0.1);
  }
  .ai-hub-side-open { transform: translateX(0); }
  .ai-hub-side-close { display: inline-flex; }
  .ai-hub-menu-btn { display: inline-flex; }
  .ai-hub-mask {
    display: block; position: fixed; inset: 0; z-index: 10;
    background: rgba(0, 0, 0, 0.4);
  }
}
[data-theme="dark"] .ai-hub-side,
[data-theme="dark"] .ai-hub-topbar { background: #0f172a; }
</style>
