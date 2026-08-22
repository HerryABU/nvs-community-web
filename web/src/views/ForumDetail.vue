<template>
  <div class="page-container" v-loading="loading">
    <template v-if="forum">
      <div class="forum-header">
        <h1>{{ forum.name }}</h1>
        <p class="forum-desc">{{ forum.description }}</p>
        <div class="forum-actions">
          <el-button v-if="isModerator" size="small" @click="openMuteManage">禁言管理</el-button>
          <el-button type="primary" @click="showNewThread = true" v-if="authStore.isLoggedIn && !locked">
            发布新帖
          </el-button>
          <router-link v-else-if="!authStore.loading && !locked" to="/login" class="login-link">登录后即可发帖</router-link>
        </div>
      </div>

      <!-- 禁言管理弹窗（创建者/管理员） -->
      <el-dialog v-model="showMuteManage" title="🚫 禁言管理" width="560px">
        <el-empty v-if="!muteLoading && mutes.length === 0" description="暂无禁言记录" />
        <div v-else v-loading="muteLoading" class="mute-list">
          <div v-for="m in mutes" :key="m.id" class="mute-item">
            <div class="mute-main">
              <span class="mute-user">{{ m.username || ('用户#' + m.user_id) }}</span>
              <el-tag v-if="m.admin_ban" type="danger" size="small">管理员封禁</el-tag>
              <el-tag v-else type="warning" size="small">创建者禁言</el-tag>
              <span class="mute-reason">{{ m.reason || '未填原因' }}</span>
            </div>
            <div class="mute-meta">
              <span v-if="m.muted_until">{{ m.banned_by }} · 至 {{ formatMuteTime(m.muted_until) }}</span>
              <span v-else>{{ m.banned_by }} · 永久</span>
            </div>
            <el-button
              size="small"
              type="danger"
              plain
              :disabled="m.admin_ban && !isAdmin"
              :title="m.admin_ban && !isAdmin ? '管理员封禁的禁言仅管理员可解除' : ''"
              @click="unmuteUser(m)"
            >
              解禁
            </el-button>
          </div>
        </div>
      </el-dialog>

      <!-- 密码门禁 -->
      <div v-if="locked" class="password-gate">
        <el-result icon="lock" :title="needRoom ? '该论坛不可见' : '该论坛需要密码'" :sub-title="needRoom ? '请输入房间号（有密码的论坛还需密码）以进入' : '请输入进入密码以查看帖子'">
          <template #extra>
            <div style="display:flex;flex-direction:column;gap:8px;align-items:center">
              <el-input v-if="needRoom" v-model="roomInput" placeholder="房间号（创建者分享给你）" style="width:240px" @keyup.enter="submitPassword" />
              <el-input v-model="passwordInput" :type="forum?.has_password ? 'password' : 'text'" :placeholder="forum?.has_password ? '进入密码' : '无需密码，仅填房间号'" style="width:240px" show-password @keyup.enter="submitPassword" />
              <el-button type="primary" style="margin-left:8px" :loading="verifying" @click="submitPassword">进入</el-button>
            </div>
          </template>
        </el-result>
      </div>

      <!-- 发帖表单 -->
      <el-dialog v-model="showNewThread" title="发布新帖" width="600px" destroy-on-close>
        <el-form label-width="60px">
          <el-form-item label="标题" required>
            <el-input v-model="newTitle" placeholder="请输入标题" maxlength="100" />
          </el-form-item>
          <el-form-item label="内容" required>
            <el-input v-model="newContent" type="textarea" :rows="6" placeholder="请输入内容，支持 **粗体** *斜体* `代码` 等 Markdown 语法" maxlength="10000" show-word-limit />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="showNewThread = false">取消</el-button>
          <el-button type="primary" :loading="posting" @click="submitThread">发布</el-button>
        </template>
      </el-dialog>

      <!-- 子论坛 -->
      <div v-if="subForums && subForums.length > 0" class="sub-forums">
        <h3 class="section-title">子话题</h3>
        <div class="sub-forum-grid">
          <div
            v-for="sf in subForums"
            :key="sf.id"
            class="sub-forum-card"
            @click="$router.push(`/forum/${sf.id}`)"
          >
            <h4>{{ sf.name }}</h4>
            <p v-if="sf.description">{{ sf.description }}</p>
            <span class="sub-forum-meta">{{ sf.thread_count || 0 }} 帖子</span>
          </div>
        </div>
      </div>

      <!-- 帖子列表 -->
      <div class="thread-list">
        <div v-for="t in threads" :key="t.id" class="thread-item" @click="$router.push(`/thread/${t.id}`)">
          <div class="thread-main">
            <h3>{{ t.title }}</h3>
            <div class="thread-meta">
              <span>{{ t.user?.username || '匿名' }}</span>
              <span>{{ formatTime(t.created_at) }}</span>
              <span>{{ t.view_count }} 浏览</span>
              <span>{{ t.post_count }} 回复</span>
            </div>
          </div>
        </div>
        <el-empty v-if="threads.length === 0" description="暂无帖子" />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { novelApi, forumApi } from '@/api/novel';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

// 论坛管理者：管理员 或 论坛创建者
const isAdmin = computed(() => authStore.user?.role === 'admin');
const isModerator = computed(() => {
  const f = forum.value;
  if (!f) return false;
  if (isAdmin.value) return true;
  return f.type === 'custom' && f.creator_id && f.creator_id === authStore.user?.id;
});

// ====== 禁言管理 ======
const showMuteManage = ref(false);
const mutes = ref<any[]>([]);
const muteLoading = ref(false);

async function openMuteManage() {
  showMuteManage.value = true;
  muteLoading.value = true;
  mutes.value = [];
  try {
    const res = await forumApi.listMutes(Number(route.params.id));
    if (res.data.code === 0) mutes.value = res.data.data || [];
  } catch { ElMessage.error('加载禁言列表失败'); }
  finally { muteLoading.value = false; }
}

function formatMuteTime(iso: string) {
  if (!iso) return '';
  const d = new Date(iso);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

async function unmuteUser(m: any) {
  try {
    await ElMessageBox.confirm(`确定解除「${m.username || ('用户#' + m.user_id)}」的禁言？`, '解除禁言', { type: 'warning' });
    await forumApi.unmuteUser(Number(route.params.id), m.user_id);
    ElMessage.success('已解除禁言');
    openMuteManage();
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '解禁失败');
  }
}
const loading = ref(false);
const forum = ref<any>(null);
const threads = ref<any[]>([]);
const subForums = ref<any[]>([]);
const showNewThread = ref(false);
const newTitle = ref('');
const newContent = ref('');
const posting = ref(false);

// 密码门禁
const locked = ref(false);
const passwordInput = ref('');
const roomInput = ref('');
const verifying = ref(false);
// 不可见（非公开）论坛需凭房间号进入；管理员/创建者豁免
const needRoom = computed(() => {
  const f = forum.value;
  if (isModerator.value) return false;
  return !!f && !f.is_public && f.type === 'custom';
});

function formatTime(s: string) {
  return new Date(s).toLocaleDateString('zh-CN');
}

async function load() {
  loading.value = true;
  try {
    const password = (route.query.password as string) || '';
    const room = (route.query.room as string) || '';
    const res = await forumApi.getForum(Number(route.params.id), undefined, password, room);
    const d = res.data.data;
    forum.value = d.forum;
    locked.value = !!d.locked;
    threads.value = d.threads || [];
    subForums.value = d.sub_forums || [];
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

async function submitPassword() {
  const f = forum.value;
  if (f?.has_password && !passwordInput.value.trim()) { ElMessage.warning('请输入密码'); return; }
  if (needRoom.value && !roomInput.value.trim()) { ElMessage.warning('请输入房间号'); return; }
  verifying.value = true;
  try {
    // 不可见论坛：先校验房间号（enter-by-code），有密码则顺带校验
    if (needRoom.value) {
      const code = roomInput.value.trim().toUpperCase();
      const r = await forumApi.enterByRoomCode(code, passwordInput.value);
      if (r.data.code === 0) {
        locked.value = false;
        router.replace({ query: { room: code, password: passwordInput.value || undefined } });
        await load();
        return;
      }
    }
    const res = await forumApi.verifyPassword(Number(route.params.id), passwordInput.value);
    if (res.data.code === 0) {
      locked.value = false;
      router.replace({ query: { password: passwordInput.value } });
      await load();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '密码错误');
  } finally {
    verifying.value = false;
  }
}

async function submitThread() {
  if (!newTitle.value.trim() || !newContent.value.trim()) {
    ElMessage.warning('请填写标题和内容');
    return;
  }
  posting.value = true;
  try {
    await forumApi.createThread(Number(route.params.id), { title: newTitle.value, content: newContent.value });
    ElMessage.success('发布成功');
    showNewThread.value = false;
    newTitle.value = '';
    newContent.value = '';
    load();
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '发布失败');
  } finally {
    posting.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
.sub-forums {
  margin-bottom: 24px;
}

.section-title {
  font-size: 1.1rem;
  color: var(--text-color, #333);
  margin-bottom: 12px;
  padding-bottom: 6px;
  border-bottom: 1px solid #f0f0f0;
}

.sub-forum-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}

.sub-forum-card {
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 8px;
  padding: 14px;
  cursor: pointer;
  transition: box-shadow 0.2s, transform 0.2s;
}

.sub-forum-card:hover {
  box-shadow: 0 2px 8px rgba(0,0,0,0.08);
  transform: translateY(-1px);
}

.sub-forum-card h4 {
  font-size: 1rem;
  color: var(--primary-color);
  margin: 0 0 6px 0;
}

.sub-forum-card p {
  font-size: 0.85rem;
  color: var(--text-light);
  margin: 0 0 8px 0;
  line-height: 1.4;
}

.sub-forum-meta {
  font-size: 0.75rem;
  color: #999;
}

.forum-header {
  margin-bottom: 24px;
}

.forum-header h1 {
  font-size: 1.5rem;
  color: var(--primary-color);
  margin-bottom: 8px;
}

.forum-desc {
  color: var(--text-light);
  margin-bottom: 16px;
}

.forum-actions { display: flex; align-items: center; gap: 8px; }
.mute-list { display: flex; flex-direction: column; gap: 8px; }
.mute-item {
  display: flex; align-items: center; gap: 10px; padding: 10px 12px;
  border-radius: 8px; background: var(--bg-card, #f7f7f7);
  border: 1px solid var(--glass-border, #e5e7eb);
}
.mute-main { flex: 1; min-width: 0; display: flex; align-items: center; gap: 8px; }
.mute-user { font-weight: 600; color: var(--text-color, #333); }
.mute-reason { font-size: 0.82rem; color: var(--text-light, #666); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mute-meta { font-size: 0.75rem; color: #999; flex-shrink: 0; }

.thread-item {
  padding: 16px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background 0.2s;
}

.thread-item:hover {
  background: #f9f9f9;
}

.thread-main h3 {
  font-size: 1rem;
  color: var(--primary-color);
  margin-bottom: 6px;
}

.thread-meta {
  display: flex;
  gap: 16px;
  font-size: 0.8rem;
  color: var(--text-light);
}

.login-link {
  color: var(--primary-color);
  font-size: 0.9rem;
}

.password-gate {
  padding: 40px 0;
  text-align: center;
}

/* ====== 移动端论坛详情适配 ====== */
@media (max-width: 640px) {
  .sub-forum-grid {
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
  .thread-meta {
    flex-wrap: wrap;
    gap: 6px 12px;
  }
  .forum-actions {
    flex-wrap: wrap;
  }
  .mute-main {
    flex-wrap: wrap;
  }
}
</style>

<style>
[data-theme="dark"] .forum-header h1 {
  color: #e2e8f0;
}
[data-theme="dark"] .forum-desc {
  color: #94a3b8;
}
[data-theme="dark"] .thread-item {
  border-bottom-color: rgba(255,255,255,.06);
}
[data-theme="dark"] .thread-item:hover {
  background: #1e293b;
}
[data-theme="dark"] .thread-main h3 {
  color: #e2e8f0;
}
[data-theme="dark"] .thread-meta {
  color: #94a3b8;
}
[data-theme="dark"] .section-title {
  color: #e2e8f0;
  border-bottom-color: rgba(255,255,255,.06);
}
[data-theme="dark"] .sub-forum-card {
  background: #1e293b;
  border-color: rgba(255,255,255,.08);
}
[data-theme="dark"] .sub-forum-card:hover {
  box-shadow: 0 2px 8px rgba(0,0,0,0.4);
}
[data-theme="dark"] .sub-forum-card h4 {
  color: #e2e8f0;
}
[data-theme="dark"] .sub-forum-card p {
  color: #94a3b8;
}
[data-theme="dark"] .sub-forum-meta {
  color: #64748b;
}
</style>
