<template>
  <div class="ai-hub" :class="{ 'ai-hub-immersive': immersive }">
    <!-- ===== 左侧功能导航 184px =====
         布局参照 testlo（SparkAi）：左侧固定竖向导航 + 56px 顶栏 + 主区。
         竖向导航比宫格更像"工作台"：入口常驻，切功能不用回首页。 -->
    <aside class="ai-nav" :class="{ 'ai-nav-open': mobileSideOpen }">
      <div class="ai-nav-head">
        <span class="ai-nav-logo">✦</span>
        <span class="ai-nav-name">AI 工作台</span>
      </div>

      <div class="ai-nav-body">
        <button class="ai-nav-new" type="button" @click="newChat">
          <el-icon><Plus /></el-icon>
          <span class="ai-nav-new-text">新建对话</span>
          <kbd class="ai-nav-kbd">Ctrl K</kbd>
        </button>

        <div class="ai-nav-group">功能</div>
        <button
          v-for="e in navEntries"
          :key="e.key"
          type="button"
          class="ai-nav-item"
          :class="{ active: activeTab === e.key, 'is-dim': e.needsAi && !status.ai_enabled }"
          @click="onNavigate(e.key)"
        >
          <span class="ai-nav-emoji">{{ e.emoji }}</span>
          <span class="ai-nav-label">{{ e.title }}</span>
          <span v-if="e.badge" class="ai-nav-badge">{{ e.badge }}</span>
          <span v-if="e.needsAi && !status.ai_enabled" class="ai-nav-dot" title="AI 未开启" />
        </button>

        <!-- 历史会话：只在对话面板出现，避免其它面板被会话列表干扰 -->
        <template v-if="activeTab === 'chat'">
          <div class="ai-nav-group">
            历史会话
            <button class="ai-nav-refresh" type="button" title="刷新" @click="loadSessions">↻</button>
          </div>
          <div v-if="!sessions.length" class="ai-nav-empty">暂无会话</div>
          <div
            v-for="s in sessions"
            :key="s.id"
            class="ai-nav-session"
            :class="{ active: activeSessionId === s.id }"
            @click="openSession(s)"
          >
            <span class="ai-nav-session-title">{{ s.title || '新对话' }}</span>
            <span v-if="s.share_token" class="ai-nav-share" title="已分享">🔗</span>
            <el-dropdown trigger="click" @command="(cmd: string) => onSessionCmd(cmd, s)">
              <button class="ai-nav-session-more" type="button" @click.stop>⋯</button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="rename">重命名</el-dropdown-item>
                  <el-dropdown-item command="share">分享</el-dropdown-item>
                  <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </template>
      </div>

      <div class="ai-nav-foot">
        <div class="ai-nav-balance">
          <b>{{ status.balance }}</b>
          <span>{{ status.token_name }}</span>
          <el-button text type="primary" size="small" @click="$router.push('/wallet')">充值</el-button>
        </div>
        <div
          v-if="status.free_quota?.enabled"
          class="ai-nav-free"
          :class="{ exhausted: status.free_quota.calls_left === 0 }"
        >
          今日免费 {{ status.free_quota.calls_left < 0 ? '不限' : status.free_quota.calls_left }} 次
        </div>
        <button class="ai-nav-item ai-nav-back" type="button" @click="$router.push('/')">
          <span class="ai-nav-emoji">↩</span>
          <span class="ai-nav-label">返回站点</span>
        </button>
      </div>
    </aside>
    <div v-if="mobileSideOpen" class="ai-hub-mask" @click="mobileSideOpen = false" />

    <!-- ===== 主区 ===== -->
    <main class="ai-hub-main">
      <div class="ai-hub-topbar">
        <el-button class="ai-hub-menu-btn" text @click="mobileSideOpen = true">
          <el-icon><Menu /></el-icon>
        </el-button>
        <span class="ai-hub-current">{{ currentTitle }}</span>
        <span v-if="!status.ai_enabled" class="ai-hub-status">AI 未开启</span>

        <div class="ai-hub-topbar-right">
          <el-button
            v-if="activeTab === 'gallery'"
            size="small"
            text
            type="primary"
            @click="$router.push('/ai/gallery')"
          >
            独立页 ↗
          </el-button>
          <el-button v-if="activeTab === 'chat'" size="small" text @click="newChat">
            <el-icon><Plus /></el-icon> 新建
          </el-button>
          <el-tooltip
            :content="immersive ? '退出沉浸模式（Esc）' : '沉浸模式：隐藏侧栏与顶栏'"
            placement="bottom"
          >
            <el-button size="small" text @click="toggleImmersive">
              <el-icon><FullScreen /></el-icon>
            </el-button>
          </el-tooltip>
        </div>
      </div>

      <!-- 模型 / 智能体 / 灵感广场不依赖总开关：没开启时也能先看有什么 -->
      <AiModelSquare
        v-if="activeTab === 'models'"
        :status="status"
        :is-admin="isAdmin"
        @use="onUseModel"
      />
      <AiAgentsPanel
        v-else-if="activeTab === 'agents'"
        :agents="agents"
        :is-admin="isAdmin"
        @start="onStartAgent"
      />
      <AiGalleryPanel v-else-if="activeTab === 'gallery'" @remix="onRemix" />

      <!-- ===== 需要 AI 已开启的面板：给可操作引导 ===== -->
      <div v-else-if="!loading && !status.ai_enabled" class="ai-hub-blocked">
        <div class="ai-guide-card">
          <div class="ai-guide-emoji">🤖</div>
          <h2 class="ai-guide-title">这项功能需要先开启 AI</h2>
          <p class="ai-guide-desc">
            <template v-if="isAdmin">
              AI 是<strong>默认关闭</strong>的增值能力 —— 这不是故障。按下面三步配置，几分钟就能用起来。
            </template>
            <template v-else>
              站长还没有开放 AI 功能。你可以先去看看灵感广场里的作品，或者联系站长开通。
            </template>
          </p>

          <ol v-if="isAdmin" class="ai-guide-steps">
            <li>
              <span class="ai-guide-num">1</span>
              <div>
                <b>打开总开关</b>
                <p>在「AI 功能配置」里启用 AI，并决定是否发放每日免费额度。</p>
              </div>
            </li>
            <li>
              <span class="ai-guide-num">2</span>
              <div>
                <b>添加至少一个模型</b>
                <p>在「AI 模型池」用预设一键创建（DeepSeek / 通义千问 / 豆包 / GPT 等），再补上上游密钥。</p>
              </div>
            </li>
            <li>
              <span class="ai-guide-num">3</span>
              <div>
                <b>回来试一句</b>
                <p>对话、绘画、视频分别独立开关；只开对话也能先用起来。</p>
              </div>
            </li>
          </ol>

          <div class="ai-guide-actions">
            <template v-if="isAdmin">
              <el-button type="primary" size="large" @click="$router.push('/admin/ai-features')">
                去开启 AI
              </el-button>
              <el-button size="large" @click="$router.push('/admin/ai-pool')">配置模型池</el-button>
            </template>
            <template v-else>
              <el-button type="primary" size="large" @click="$router.push('/ai/gallery')">
                看看灵感广场
              </el-button>
            </template>
            <el-button text size="large" @click="goHome">返回工作台</el-button>
          </div>
        </div>
      </div>

      <!-- 已开启但没有任何模型：同样给可执行的下一步，而不是干说「暂无可用模型」 -->
      <div v-else-if="!loading && noModels" class="ai-hub-blocked">
        <div class="ai-guide-card">
          <div class="ai-guide-emoji">🧩</div>
          <h2 class="ai-guide-title">AI 已开启，但还没有可用模型</h2>
          <p class="ai-guide-desc">
            <template v-if="isAdmin">
              功能开关是开的，但「AI 模型池」里没有<strong>已启用</strong>的模型。
              模型池支持预设一键创建，填完密钥后把「启用」打开即可。
            </template>
            <template v-else>
              站长开启 AI 了，但还没配置可用模型。可以先逛逛灵感广场，稍后再来。
            </template>
          </p>
          <div class="ai-guide-actions">
            <template v-if="isAdmin">
              <el-button type="primary" size="large" @click="$router.push('/admin/ai-pool')">
                去配置模型
              </el-button>
              <el-button size="large" @click="$router.push('/admin/ai-features')">查看功能配置</el-button>
            </template>
            <template v-else>
              <el-button type="primary" size="large" @click="$router.push('/ai/gallery')">
                看看灵感广场
              </el-button>
            </template>
            <el-button text size="large" @click="goHome">返回工作台</el-button>
          </div>
        </div>
      </div>

      <template v-else>
        <AiChatPanel
          v-if="activeTab === 'chat'"
          :key="'chat-' + (activeSessionId || 'new-' + activeAgentId) + '-' + presetModelId"
          :status="status"
          :agents="agents"
          :session-id="activeSessionId"
          :agent-id="activeAgentId"
          :preset-model-id="presetModelId"
          @session-created="onSessionCreated"
          @balance-updated="onBalanceUpdated"
          @sessions-changed="loadSessions"
        />
        <AiImagePanel v-else-if="activeTab === 'image'" :status="status" :initial-prompt="remixPrompt" />
        <AiVideoPanel v-else-if="activeTab === 'video'" :status="status" />
        <MindMapPanel v-else :status="status" />
      </template>
    </main>

    <!-- 分享对话框 -->
    <el-dialog v-model="shareVisible" title="分享会话" width="520px">
      <template v-if="shareLink">
        <p class="share-tip">
          任何人通过此链接可查看该会话的只读快照（不含余额与费用信息）。取消分享后链接立即失效。
        </p>
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
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus, Close, Delete, Menu, EditPen, Share, ArrowLeft, FullScreen } from '@element-plus/icons-vue';
import { aiHubApi, type AiHubStatus, type AiAgent, type AiChatSession } from '@/api/aiHub';
import { useAuthStore } from '@/stores/auth';
import { appPrefix } from '@/utils/prefix';
import { AI_ENTRIES } from '@/components/ai/aiEntries';
import AiModelSquare from '@/components/ai/AiModelSquare.vue';
import AiAgentsPanel from '@/components/ai/AiAgentsPanel.vue';
import AiChatPanel from '@/components/ai/AiChatPanel.vue';
import AiImagePanel from '@/components/ai/AiImagePanel.vue';
import AiVideoPanel from '@/components/ai/AiVideoPanel.vue';
import AiGalleryPanel from '@/components/ai/AiGalleryPanel.vue';
import MindMapPanel from '@/components/ai/MindMapPanel.vue';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
/** 未就绪态的文案与按钮按角色分流：管理员看到的是「怎么开」，普通用户看到的是「找谁开」 */
const isAdmin = computed(() => authStore.user?.role === 'admin');
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
/** 从「大模型广场」点某个模型进来时，把它带给对话面板预选 */
const presetModelId = ref(0);

type HubTab = 'chat' | 'models' | 'image' | 'video' | 'gallery' | 'agents' | 'mind';
// 开门见山：进来直接是对话界面，不做中间门户页（参考站同样一进即功能页）
const activeTab = ref<HubTab>('chat');
const remixPrompt = ref('');
const mobileSideOpen = ref(false);
const loading = ref(true);

const activeSession = computed(() => sessions.value.find((s) => s.id === activeSessionId.value) || null);

/** 左侧竖向导航的入口（与门户宫格共用一份配置，避免两处漂移） */
const navEntries = computed(() => AI_ENTRIES);

const TAB_TITLES: Record<HubTab, string> = {
  chat: 'AI 对话',
  models: '大模型广场',
  image: '专业绘画',
  video: '视频创作',
  gallery: '灵感广场',
  agents: 'AI 智能体',
  mind: '思维导图',
};
const currentTitle = computed(() => {
  if (activeTab.value === 'chat' && activeSession.value) {
    return activeSession.value.agent_name || TAB_TITLES.chat;
  }
  return TAB_TITLES[activeTab.value];
});

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

/** 引导卡片里的「返回」：回到对话（原来回门户，门户已移除） */
function goHome() {
  activeTab.value = 'chat';
  mobileSideOpen.value = false;
}

/** 门户卡片跳转：会员与额度不在 Hub 内，出站到会员页（页内有充值入口） */
function onNavigate(tab: string) {
  mobileSideOpen.value = false;
  if (tab === 'membership') {
    router.push('/membership');
    return;
  }
  activeTab.value = tab as HubTab;
}

/** 未开启态里的「去开启 / 配模型」：直接进后台对应页 */
function onAdminJump(to: 'features' | 'pool') {
  router.push(to === 'features' ? '/admin/ai-features' : '/admin/ai-pool');
}

/** 模型广场点「开始对话」：带上模型预选，新开一个会话 */
function onUseModel(payload: { kind: 'chat' | 'image' | 'video'; modelId: number }) {
  if (payload.kind === 'chat') {
    presetModelId.value = payload.modelId;
    activeSessionId.value = null;
    activeAgentId.value = 0;
    activeTab.value = 'chat';
    return;
  }
  // 绘画 / 视频面板自带模型选择，切过去即可（模型池顺序里第一个仍是默认）
  activeTab.value = payload.kind === 'image' ? 'image' : 'video';
}

/** 智能体卡片：像从侧栏选智能体一样，新开对话 */
function onStartAgent(a: AiAgent) {
  presetModelId.value = 0;
  activeSessionId.value = null;
  activeAgentId.value = a.id;
  activeTab.value = 'chat';
  mobileSideOpen.value = false;
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

/** 左侧导航里会话条目的 ⋯ 菜单 */
function onSessionCmd(cmd: string, s: AiChatSession) {
  if (cmd === 'rename') return renameSession(s);
  if (cmd === 'share') return openShare(s);
  if (cmd === 'delete') return deleteSession(s);
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
  const validTabs: HubTab[] = ['chat', 'models', 'image', 'video', 'gallery', 'agents', 'mind'];
  if (validTabs.includes(tab as HubTab)) activeTab.value = tab as HubTab;
  const p = String(route.query.prompt || '');
  if (p) remixPrompt.value = p;
});

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown);
  applyImmersive(false);
});</script>

<style scoped>
/* ===== 容器：左侧 184px 导航 + 主区（尺寸参照 testlo）===== */
.ai-hub {
  display: flex;
  height: calc(100vh - 60px);
  min-height: 0;
  background: var(--bg-page, #f5f7fa);
}

/* ===== 左侧功能导航 ===== */
.ai-nav {
  width: 184px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--bg-card, #fff);
  border-right: 1px solid var(--border-color, #ebeef5);
}
.ai-nav-head {
  height: 48px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 14px;
  border-bottom: 1px solid var(--border-color, #ebeef5);
}
.ai-nav-logo {
  font-size: 16px;
  color: var(--el-color-primary, #6366f1);
}
.ai-nav-name {
  font-size: var(--text-base, 14px);
  font-weight: 600;
  color: var(--text-color, #1f2328);
}
.ai-nav-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 10px 8px;
  scrollbar-width: thin;
}
.ai-nav-new {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 10px;
  margin-bottom: 6px;
  border: none;
  border-radius: var(--radius-md, 10px);
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
  color: #fff;
  font-size: var(--text-sm, 13px);
  font-family: inherit;
  cursor: pointer;
  transition: filter 0.15s ease;
}
.ai-nav-new:hover { filter: brightness(1.06); }
.ai-nav-new-text { flex: 1; text-align: left; }
.ai-nav-kbd {
  font-size: 10px;
  padding: 0 4px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.22);
  font-family: inherit;
}

.ai-nav-group {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 8px 4px;
  font-size: var(--text-xs, 12px);
  color: var(--text-light, #9ca3af);
  letter-spacing: 0.02em;
}
.ai-nav-refresh {
  border: none;
  background: none;
  color: var(--text-light, #9ca3af);
  cursor: pointer;
  font-size: 12px;
  padding: 0 2px;
}
.ai-nav-refresh:hover { color: var(--el-color-primary, #6366f1); }

.ai-nav-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  padding: 0 10px;
  margin-bottom: 2px;
  border: none;
  border-radius: var(--radius-md, 10px);
  background: transparent;
  color: var(--text-color, #374151);
  font-size: var(--text-sm, 13px);
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}
.ai-nav-item:hover { background: var(--bg-page, #f5f6f8); }
.ai-nav-item.active {
  background: var(--el-color-primary-light-9, #eef2ff);
  color: var(--el-color-primary, #6366f1);
  font-weight: 600;
}
.ai-nav-item.is-dim { opacity: 0.66; }
.ai-nav-emoji { width: 16px; text-align: center; font-size: 13px; flex-shrink: 0; }
.ai-nav-label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ai-nav-badge {
  font-size: 9px;
  line-height: 1.5;
  padding: 0 4px;
  border-radius: 3px;
  color: #fff;
  background: var(--gradient-danger, linear-gradient(135deg, #f43f5e, #e11d48));
  flex-shrink: 0;
}
.ai-nav-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #f59e0b;
  flex-shrink: 0;
}

/* 历史会话条目 */
.ai-nav-empty {
  padding: 4px 10px 8px;
  font-size: var(--text-xs, 12px);
  color: var(--text-light, #9ca3af);
}
.ai-nav-session {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 30px;
  padding: 0 6px 0 10px;
  margin-bottom: 1px;
  border-radius: 8px;
  cursor: pointer;
  color: var(--text-color, #374151);
  font-size: var(--text-sm, 13px);
}
.ai-nav-session:hover { background: var(--bg-page, #f5f6f8); }
.ai-nav-session.active {
  background: var(--el-color-primary-light-9, #eef2ff);
  color: var(--el-color-primary, #6366f1);
}
.ai-nav-session-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ai-nav-share { font-size: 10px; opacity: 0.7; flex-shrink: 0; }
.ai-nav-session-more {
  border: none;
  background: none;
  color: var(--text-light, #9ca3af);
  cursor: pointer;
  font-size: 13px;
  line-height: 1;
  padding: 0 3px;
  border-radius: 4px;
  opacity: 0;
  flex-shrink: 0;
}
.ai-nav-session:hover .ai-nav-session-more { opacity: 1; }
.ai-nav-session-more:hover { background: rgba(0, 0, 0, 0.06); }

/* 底部账户区 */
.ai-nav-foot {
  flex-shrink: 0;
  padding: 8px;
  border-top: 1px solid var(--border-color, #ebeef5);
}
.ai-nav-balance {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 4px;
  font-size: var(--text-xs, 12px);
  color: var(--text-light, #6b7280);
}
.ai-nav-balance b { color: var(--text-color, #1f2328); font-size: var(--text-base, 14px); }
.ai-nav-free {
  margin: 2px 4px 4px;
  font-size: 11px;
  color: #16a34a;
}
.ai-nav-free.exhausted { color: #ef4444; }
.ai-nav-back { margin-top: 2px; }

/* ===== 主区 ===== */
.ai-hub-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.ai-hub-topbar {
  height: 56px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  background: var(--bg-card, #fff);
  border-bottom: 1px solid var(--border-color, #ebeef5);
}
.ai-hub-current {
  font-size: var(--text-md, 16px);
  font-weight: 600;
  color: var(--text-color, #1f2328);
}
.ai-hub-status {
  font-size: var(--text-xs, 12px);
  padding: 1px 8px;
  border-radius: 999px;
  color: #b45309;
  background: rgba(180, 83, 9, 0.1);
}
.ai-hub-topbar-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 6px;
}
.ai-hub-menu-btn { display: none; }
.ai-hub-mask { display: none; }

.ai-hub-blocked {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-6, 32px);
  overflow-y: auto;
}

/* ===== 未就绪引导卡片 ===== */
.ai-guide-card {
  max-width: 620px;
  background: var(--bg-card, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-lg, 16px);
  box-shadow: var(--shadow-sm, 0 2px 8px rgba(0, 0, 0, 0.06));
  padding: var(--space-6, 32px);
  text-align: center;
}
.ai-guide-emoji { font-size: 44px; line-height: 1; margin-bottom: var(--space-3, 12px); }
.ai-guide-title {
  margin: 0 0 var(--space-2, 8px);
  font-size: var(--text-xl, 20px);
  font-weight: 600;
  color: var(--text-color, #1f2328);
}
.ai-guide-desc {
  margin: 0;
  font-size: var(--text-base, 14px);
  line-height: 1.7;
  color: var(--text-light, #6b7280);
}
.ai-guide-steps {
  list-style: none;
  margin: var(--space-5, 24px) 0 0;
  padding: 0;
  text-align: left;
  display: flex;
  flex-direction: column;
  gap: var(--space-3, 12px);
}
.ai-guide-steps li {
  display: flex;
  gap: var(--space-3, 12px);
  padding: var(--space-3, 12px);
  border-radius: var(--radius-md, 10px);
  background: var(--bg-page, #f7f8fa);
}
.ai-guide-num {
  flex-shrink: 0;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: var(--text-xs, 12px);
  font-weight: 600;
  color: #fff;
  background: var(--gradient-primary, linear-gradient(135deg, #6366f1, #8b5cf6));
}
.ai-guide-steps b { font-size: var(--text-base, 14px); color: var(--text-color, #1f2328); }
.ai-guide-steps p {
  margin: 2px 0 0;
  font-size: var(--text-sm, 13px);
  line-height: 1.6;
  color: var(--text-light, #6b7280);
}
.ai-guide-actions {
  margin-top: var(--space-5, 24px);
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3, 12px);
  justify-content: center;
}

.share-tip {
  font-size: var(--text-sm, 13px);
  color: var(--text-light, #666);
  margin: 0 0 10px;
  line-height: 1.6;
}
.share-actions { margin-top: 10px; text-align: right; }

/* ===== 沉浸模式 ===== */
.ai-hub.ai-hub-immersive { height: 100vh; }
.ai-hub.ai-hub-immersive .ai-nav { display: none; }
.ai-hub.ai-hub-immersive .ai-hub-menu-btn { display: none; }
.ai-hub.ai-hub-immersive .ai-hub-topbar { height: 44px; padding: 0 12px; }

/* ===== 窄屏：导航抽屉化 ===== */
@media (max-width: 860px) {
  .ai-nav {
    position: fixed;
    top: 0;
    bottom: 0;
    left: 0;
    z-index: 20;
    transform: translateX(-100%);
    transition: transform 0.25s;
    box-shadow: 2px 0 12px rgba(0, 0, 0, 0.12);
  }
  .ai-nav-open { transform: translateX(0); }
  .ai-hub-menu-btn { display: inline-flex; }
  .ai-hub-mask {
    display: block;
    position: fixed;
    inset: 0;
    z-index: 10;
    background: rgba(0, 0, 0, 0.4);
  }
}

/* ===== 深色 ===== */
[data-theme="dark"] .ai-nav,
[data-theme="dark"] .ai-hub-topbar { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .ai-nav-name,
[data-theme="dark"] .ai-hub-current { color: #e2e8f0; }
[data-theme="dark"] .ai-nav-item { color: #cbd5e1; }
[data-theme="dark"] .ai-nav-item:hover,
[data-theme="dark"] .ai-nav-session:hover { background: #0f172a; }
[data-theme="dark"] .ai-nav-item.active,
[data-theme="dark"] .ai-nav-session.active { background: rgba(99, 102, 241, 0.18); }
[data-theme="dark"] .ai-guide-card { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .ai-guide-title { color: #e2e8f0; }
[data-theme="dark"] .ai-guide-steps li { background: #0f172a; }
</style>
