<template>
  <div class="ai-hub" :class="{ 'ai-hub-immersive': immersive }">
    <!-- ===== 左侧栏：新建 / 智能体 / 历史会话 ===== -->
    <aside class="ai-hub-side" :class="{ 'ai-hub-side-open': mobileSideOpen }">
      <div class="ai-hub-side-head">
        <el-button type="primary" class="ai-hub-new" @click="newChat">
          <el-icon><Plus /></el-icon> 新建对话
          <span class="ai-hub-kbd">Ctrl K</span>
        </el-button>
        <el-button class="ai-hub-side-close" text @click="mobileSideOpen = false">
          <el-icon><Close /></el-icon>
        </el-button>
      </div>

      <div class="ai-hub-side-scroll">
        <template v-for="g in agentGroups" :key="g.name">
          <div class="ai-hub-side-title">{{ g.name }}</div>
          <div
            v-for="a in g.items"
            :key="a.id"
            :class="['ai-agent-item', { active: activeAgentId === a.id && !activeSessionId }]"
            @click="pickAgent(a)"
          >
            <span class="ai-agent-emoji">{{ a.emoji || '🤖' }}</span>
            <span class="ai-agent-name">{{ a.name }}</span>
            <span v-if="a.upstream === 'coze'" class="ai-agent-coze">Coze</span>
            <span v-if="a.used_count" class="ai-agent-used">{{ formatCount(a.used_count) }}</span>
          </div>
        </template>

        <div class="ai-hub-side-title">历史会话</div>
        <div
          v-for="s in sessions"
          :key="s.id"
          :class="['ai-session-item', { active: activeSessionId === s.id }]"
          @click="openSession(s)"
        >
          <span class="ai-session-title">{{ s.title || '新对话' }}</span>
          <span v-if="s.share_token" class="ai-session-shared" title="已分享">🔗</span>
          <span class="ai-session-tools">
            <el-icon title="重命名" @click.stop="renameSession(s)"><EditPen /></el-icon>
            <el-icon title="分享" @click.stop="openShare(s)"><Share /></el-icon>
            <el-icon class="ai-session-del" title="删除" @click.stop="deleteSession(s)"><Delete /></el-icon>
          </span>
        </div>
        <el-empty v-if="!sessions.length" description="暂无会话" :image-size="50" />
      </div>

      <div class="ai-hub-side-foot">
        <span>余额 {{ status.balance }} {{ status.token_name }}</span>
        <el-button text type="primary" size="small" @click="$router.push('/wallet')">充值</el-button>
        <template v-if="status.free_quota?.enabled">
          <span class="ai-hub-free" :class="{ exhausted: status.free_quota.calls_left === 0 }">
            今日免费剩余 {{ status.free_quota.calls_left < 0 ? '不限' : status.free_quota.calls_left }} 次
          </span>
        </template>
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
          <el-radio-button value="gallery"><el-icon><Star /></el-icon> 灵感广场</el-radio-button>
          <el-radio-button value="mind"><el-icon><Share /></el-icon> 思维导图</el-radio-button>
        </el-radio-group>
        <span v-if="activeTab === 'chat' && activeSession" class="ai-hub-current">
          {{ activeSession.agent_name || 'AI 对话' }}
        </span>
        <el-button
          v-if="activeTab === 'gallery'"
          size="small"
          text
          type="primary"
          @click="$router.push('/ai/gallery')"
        >独立页 ↗</el-button>
        <el-tooltip :content="immersive ? '退出沉浸模式（Esc）' : '沉浸模式：隐藏侧栏与顶栏'" placement="bottom">
          <el-button size="small" text class="ai-hub-immersive-btn" @click="toggleImmersive">
            <el-icon><FullScreen /></el-icon>
          </el-button>
        </el-tooltip>
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
        <AiImagePanel
          v-else-if="activeTab === 'image'"
          :status="status"
          :initial-prompt="remixPrompt"
        />
        <AiVideoPanel v-else-if="activeTab === 'video'" :status="status" />
        <AiGalleryPanel v-else-if="activeTab === 'gallery'" @remix="onRemix" />
        <MindMapPanel v-else :status="status" />
      </template>
    </main>

    <!-- 分享对话框 -->
    <el-dialog v-model="shareVisible" title="分享会话" width="520px">
      <template v-if="shareLink">
        <p class="share-tip">任何人通过此链接可查看该会话的只读快照（不含余额与费用信息）。取消分享后链接立即失效。</p>
        <el-input :model-value="shareLink" readonly>
          <template #append>
            <el-button @click="copyShareLink">复制</el-button>
          </template>
        </el-input>
        <div class="share-actions">
          <el-button text type="danger" @click="unshare">取消分享</el-button>
        </div>
      </template>
      <template v-else>
        <p class="share-tip">生成只读分享链接，方便把对话发给他人查看。</p>
        <el-button type="primary" :loading="sharing" @click="doShare">生成分享链接</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus, Close, Delete, Menu, ChatDotRound, Picture, VideoCamera, EditPen, Share, Star, FullScreen } from '@element-plus/icons-vue';
import { aiHubApi, type AiHubStatus, type AiAgent, type AiChatSession } from '@/api/aiHub';
import { appPrefix } from '@/utils/prefix';
import AiChatPanel from '@/components/ai/AiChatPanel.vue';
import AiImagePanel from '@/components/ai/AiImagePanel.vue';
import AiVideoPanel from '@/components/ai/AiVideoPanel.vue';
import AiGalleryPanel from '@/components/ai/AiGalleryPanel.vue';
import MindMapPanel from '@/components/ai/MindMapPanel.vue';

const route = useRoute();
const status = reactive<AiHubStatus>({
  ai_enabled: false,
  token_enabled: true,
  token_name: '代币',
  balance: 0,
  free_quota: undefined,
  chat_models: [],
  image_models: [],
  video_models: [],
});
const agents = ref<AiAgent[]>([]);
const sessions = ref<AiChatSession[]>([]);
const activeSessionId = ref<number | null>(null);
const activeAgentId = ref(0);
const activeTab = ref<'chat' | 'image' | 'video' | 'gallery' | 'mind'>('chat');
const remixPrompt = ref('');
const mobileSideOpen = ref(false);
const loading = ref(true);

const activeSession = computed(() => sessions.value.find((s) => s.id === activeSessionId.value) || null);

// 智能体按分类分组（空分类归「通用」）
const agentGroups = computed(() => {
  const map = new Map<string, AiAgent[]>();
  for (const a of agents.value) {
    const k = (a.category || '').trim() || '通用';
    if (!map.has(k)) map.set(k, []);
    map.get(k)!.push(a);
  }
  return Array.from(map.entries()).map(([name, items]) => ({ name, items }));
});

function formatCount(n?: number) {
  const v = n || 0;
  if (v >= 10000) return `${(v / 10000).toFixed(1)}w+`;
  if (v >= 1000) return `${(v / 1000).toFixed(1)}k+`;
  return String(v);
}
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

// ---------- 沉浸模式（本地偏好持久化） ----------
const IMMERSIVE_KEY = 'nvs_ai_immersive';
const immersive = ref(localStorage.getItem(IMMERSIVE_KEY) === '1');

function applyImmersive(on: boolean) {
  document.body.classList.toggle('ai-immersive', on);
}
function toggleImmersive() {
  immersive.value = !immersive.value;
  localStorage.setItem(IMMERSIVE_KEY, immersive.value ? '1' : '0');
  applyImmersive(immersive.value);
}

// ---------- Ctrl+K / ⌘K：新建对话；Esc：退出沉浸 ----------
function onKeydown(e: KeyboardEvent) {
  const meta = e.ctrlKey || e.metaKey;
  if (meta && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault();
    newChat();
    return;
  }
  if (e.key === 'Escape' && immersive.value) {
    toggleImmersive();
  }
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

// ===== 会话重命名 / 分享 =====
const shareVisible = ref(false);
const shareLink = ref('');
const sharing = ref(false);
const shareSessionId = ref<number | null>(null);

async function renameSession(s: AiChatSession) {
  try {
    const { value } = await ElMessageBox.prompt('新的会话标题', '重命名会话', {
      inputValue: s.title || '',
      inputValidator: (v: string) => (!!v && v.trim().length > 0) || '标题不能为空',
      confirmButtonText: '保存',
      cancelButtonText: '取消',
    });
    const res = await aiHubApi.renameSession(s.id, value.trim());
    if (res.data.code === 0) {
      ElMessage.success('已重命名');
      await loadSessions();
    }
  } catch (e: any) {
    if (e === 'cancel' || e?.message === 'cancel') return;
    ElMessage.error(e?.response?.data?.message || '重命名失败');
  }
}

function openShare(s: AiChatSession) {
  shareSessionId.value = s.id;
  shareLink.value = s.share_token ? buildShareUrl(s.share_token) : '';
  shareVisible.value = true;
}

function buildShareUrl(token: string) {
  return `${location.origin}${appPrefix || ''}/ai/share/${token}`;
}

async function doShare() {
  if (!shareSessionId.value) return;
  sharing.value = true;
  try {
    const res = await aiHubApi.shareSession(shareSessionId.value);
    if (res.data.code === 0 && res.data.data?.share_token) {
      shareLink.value = buildShareUrl(res.data.data.share_token);
      await loadSessions();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '生成失败');
  } finally {
    sharing.value = false;
  }
}

async function unshare() {
  if (!shareSessionId.value) return;
  try {
    const res = await aiHubApi.unshareSession(shareSessionId.value);
    if (res.data.code === 0) {
      ElMessage.success('已取消分享');
      shareVisible.value = false;
      await loadSessions();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function copyShareLink() {
  try {
    await navigator.clipboard.writeText(shareLink.value);
    ElMessage.success('链接已复制');
  } catch {
    ElMessage.warning('复制失败，请手动选择');
  }
}

// 灵感广场「复刻同款」：切到绘画页并注入提示词
function onRemix(payload: { kind: string; prompt: string }) {
  remixPrompt.value = payload.prompt;
  activeTab.value = payload.kind === 'video' ? 'video' : 'image';
  ElMessage.success('已填入提示词，可直接生成');
}

onMounted(() => {
  applyImmersive(immersive.value);
  window.addEventListener('keydown', onKeydown);
  loadAll();
  loadSessions();
  // 深链：/ai?tab=image&prompt=xxx
  const tab = String(route.query.tab || '');
  if (tab === 'image' || tab === 'video' || tab === 'gallery' || tab === 'mind') activeTab.value = tab;
  const p = String(route.query.prompt || '');
  if (p) remixPrompt.value = p;
});

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown);
  applyImmersive(false);
});</script>

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
.ai-agent-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ai-agent-coze {
  font-size: 0.6rem; padding: 0 5px; border-radius: 4px; flex-shrink: 0;
  background: rgba(139, 92, 246, 0.12); color: #8b5cf6;
}
.ai-agent-used { font-size: 0.66rem; color: var(--text-light, #999); flex-shrink: 0; }
.ai-session-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ai-session-shared { font-size: 0.7rem; opacity: 0.7; flex-shrink: 0; }
.ai-session-tools { display: none; align-items: center; gap: 6px; flex-shrink: 0; }
.ai-session-tools .el-icon { color: var(--text-light, #888); }
.ai-session-tools .el-icon:hover { color: var(--el-color-primary, #409eff); }
.ai-session-item:hover .ai-session-tools { display: inline-flex; }
.ai-session-del { color: #ef4444; }
.share-tip { font-size: 0.82rem; color: var(--text-light, #666); margin: 0 0 10px; line-height: 1.6; }
.share-actions { margin-top: 10px; text-align: right; }
.ai-hub-side-foot {
  display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 4px;
  padding: 10px 14px; border-top: 1px solid var(--el-border-color, #ebeef5);
  font-size: 0.8rem; color: var(--text-light, #666);
}
.ai-hub-free { color: #16a34a; }
.ai-hub-free.exhausted { color: #ef4444; }
.ai-hub-mask { display: none; }
.ai-hub-main { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.ai-hub-topbar {
  display: flex; align-items: center; gap: 12px;
  padding: 10px 16px; background: var(--el-bg-color, #fff);
  border-bottom: 1px solid var(--el-border-color, #ebeef5);
}
.ai-hub-menu-btn { display: none; }
.ai-hub-current { font-size: 0.82rem; color: var(--text-light, #666); }
.ai-hub-kbd {
  margin-left: 6px; padding: 0 5px; border-radius: 4px; font-size: 0.62rem;
  background: rgba(255, 255, 255, 0.22); color: #fff; letter-spacing: 0.02em;
}
.ai-hub-immersive-btn { margin-left: auto; }
/* 沉浸模式：收起侧栏、撑满视口 */
.ai-hub.ai-hub-immersive { height: 100vh; }
.ai-hub.ai-hub-immersive .ai-hub-side { display: none; }
.ai-hub.ai-hub-immersive .ai-hub-menu-btn { display: none; }
.ai-hub.ai-hub-immersive .ai-hub-topbar { padding: 8px 12px; }
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
