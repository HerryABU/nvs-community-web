<template>
  <div class="page-container">
    <h1 class="page-title">社区论坛</h1>

    <!-- 作者工坊 -->
    <h2 class="section-title">作者工坊</h2>
    <div class="forum-grid" v-if="authorForums.length > 0">
      <el-card v-for="forum in authorForums" :key="forum.id" class="forum-card" shadow="hover" @click="enterForum(forum)">
        <h3>{{ forum.name }}</h3>
        <p>{{ forum.description }}</p>
        <div class="forum-meta"><span>{{ forum.thread_count }} 个帖子</span></div>
      </el-card>
    </div>
    <el-empty v-else description="暂无忧" :image-size="40" />

    <!-- 读者·作者互动 -->
    <h2 class="section-title" style="margin-top:32px">读者·作者互动</h2>
    <div class="forum-grid" v-if="raForums.length > 0">
      <el-card v-for="forum in raForums" :key="forum.id" class="forum-card" shadow="hover" @click="enterForum(forum)">
        <h3>{{ forum.name }}</h3>
        <p>{{ forum.description }}</p>
        <div class="forum-meta"><span>{{ forum.thread_count }} 个帖子</span></div>
      </el-card>
    </div>
    <el-empty v-else description="暂无" :image-size="40" />

    <!-- 读者交流广场 -->
    <h2 class="section-title" style="margin-top:32px">读者交流广场</h2>
    <div class="forum-grid" v-if="readerForums.length > 0">
      <el-card v-for="forum in readerForums" :key="forum.id" class="forum-card" shadow="hover" @click="enterForum(forum)">
        <h3>{{ forum.name }}</h3>
        <p>{{ forum.description }}</p>
        <div class="forum-meta"><span>{{ forum.thread_count }} 个帖子</span></div>
      </el-card>
    </div>
    <el-empty v-else description="暂无" :image-size="40" />

    <!-- 公共广场 -->
    <h2 class="section-title" style="margin-top:32px">公共广场</h2>
    <div class="forum-grid" v-if="generalForums.length > 0">
      <el-card v-for="forum in generalForums" :key="forum.id" class="forum-card" shadow="hover" @click="enterForum(forum)">
        <h3>{{ forum.name }}</h3>
        <p>{{ forum.description }}</p>
        <div class="forum-meta"><span>{{ forum.thread_count }} 个帖子</span></div>
      </el-card>
    </div>

    <!-- 自建区（用户自建论坛） -->
    <div class="self-built-header" style="margin-top:32px;display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:10px">
      <h2 class="section-title" style="margin:0">🏘️ 自建区</h2>
      <div style="display:flex;align-items:center;gap:8px">
        <el-input
          v-model="roomCodeInput"
          size="small"
          style="width:200px"
          placeholder="输入房间号进入（如 A3F9K2）"
          @keyup.enter="enterByRoomCode"
        />
        <el-button size="small" :loading="enteringRoom" @click="enterByRoomCode">进入</el-button>
        <el-button v-if="authStore.isLoggedIn" type="primary" size="small" @click="showCreateForum = true">
          创建论坛
        </el-button>
      </div>
    </div>
    <div class="forum-grid" v-if="customForums.length > 0">
      <el-card v-for="forum in customForums" :key="forum.id" class="forum-card" shadow="hover" @click="enterCustomForum(forum)">
        <h3>
          {{ forum.name }}
          <el-tag v-if="forum.has_password" size="small" type="warning" style="margin-left:6px">🔒 密码</el-tag>
          <el-tag v-if="forum.review_status === 'pending'" size="small" type="info" style="margin-left:6px">待审查</el-tag>
          <el-tag v-else-if="forum.review_status === 'rejected'" size="small" type="danger" style="margin-left:6px">已驳回</el-tag>
        </h3>
        <p>{{ forum.description || '暂无简介' }}</p>
        <div class="forum-meta">
          <span>{{ forum.thread_count }} 个帖子</span>
          <el-tag v-if="forum.creator_id === authStore.user?.id" size="small" type="success" style="margin-left:8px">我创建的</el-tag>
          <el-tag v-if="forum.creator_id === authStore.user?.id && forum.room_code" size="small" style="margin-left:8px;cursor:pointer" title="点击复制房间号" @click.stop="copyRoom(forum.room_code)">
            🚪 {{ forum.room_code }}
          </el-tag>
        </div>
      </el-card>
    </div>
    <el-empty v-else description="暂无自建论坛" :image-size="40" />

    <!-- 创建论坛弹窗 -->
    <el-dialog v-model="showCreateForum" title="创建自建论坛" width="520px" destroy-on-close @closed="onCreateClosed">
      <el-form :model="createForm" label-width="90px">
        <el-form-item label="论坛名称" required>
          <el-input v-model="createForm.name" maxlength="128" placeholder="给你的论坛起个名字" />
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="createForm.description" type="textarea" :rows="3" maxlength="512" />
        </el-form-item>
        <el-form-item label="进入密码">
          <el-input v-model="createForm.password" maxlength="64" placeholder="可选：设置后需密码进入" show-password />
        </el-form-item>
        <el-form-item label="房间号">
          <el-input
            v-model="createForm.room_code"
            maxlength="16"
            placeholder="留空自动生成；也可自定：4-16 位数字+英文字母"
            @input="onRoomCodeInput"
          />
          <span class="hint" style="margin-left:8px">房间号是进入论坛的凭证，发给好友即可凭号进入（有密码则还需密码）</span>
        </el-form-item>
        <el-form-item label="公开显示">
          <el-switch v-model="createForm.is_public" />
          <span class="hint" style="margin-left:8px">开启后在大厅「自建区」公开展示，关闭则隐藏（仅本人可见，他人凭房间号进入）</span>
        </el-form-item>
        <el-form-item v-if="createdRoomCode" label="房间号">
          <el-tag size="large" type="success" style="font-size:1rem;letter-spacing:2px">{{ createdRoomCode }}</el-tag>
          <el-button size="small" text type="primary" @click="copyRoomCode">复制</el-button>
          <span class="hint" style="margin-left:8px">把房间号分享给好友即可进入（有密码则还需密码）</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateForum = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="submitCreateForum">创建（可直接使用，后续由管理员审查）</el-button>
      </template>
    </el-dialog>

    <!-- 敏感分区论坛（带隔离墙确认） -->
    <h2 v-if="sensitiveForums.length > 0" class="section-title" style="margin-top:32px;color:#dc2626;">⚠ 敏感分区论坛</h2>
    <p v-if="sensitiveForums.length > 0" style="color:#999;font-size:0.85rem;margin-bottom:12px">以下论坛涉及敏感内容，点击后将触发确认弹窗</p>
    <div class="forum-grid" v-if="sensitiveForums.length > 0">
      <el-card v-for="forum in sensitiveForums" :key="forum.id"
        class="forum-card forum-card-sensitive" shadow="hover" @click="enterForum(forum)">
        <h3><span style="color:#dc2626;">⚠</span> {{ forum.name }}</h3>
        <p>{{ forum.description }}</p>
        <div class="forum-meta">
          <el-tag type="danger" size="small">敏感内容</el-tag>
          <span style="margin-left:8px">{{ forum.thread_count }} 个帖子</span>
        </div>
      </el-card>
    </div>

    <el-empty v-if="!loading && allForums.length === 0" description="暂无论坛" />

    <!-- 敏感分区确认弹窗 -->
    <SensitiveZoneGuard
      :visible="showZoneGuard"
      :zone-name="zoneGuardName"
      :is-cross-domain="zoneGuardCross"
      @confirm="onZoneConfirmed"
      @cancel="showZoneGuard = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { novelApi, forumApi } from '@/api/novel';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';
import SensitiveZoneGuard from '@/components/SensitiveZoneGuard.vue';
import { shouldShowGuard, markZoneConfirmed, setLastZone } from '@/utils/sensitiveZone';

interface Forum {
  id: number;
  name: string;
  description: string;
  type: string;
  thread_count: number;
  has_password?: boolean;
  review_status?: string;
  creator_id?: number;
  zone?: string;
}

const router = useRouter();
const authStore = useAuthStore();
const allForums = ref<Forum[]>([]);
const loading = ref(false);

const authorForums = computed(() => allForums.value.filter(f => f.type === 'author' || f.type === 'author_sub'));
const raForums = computed(() => allForums.value.filter(f => f.type === 'reader_author'));
const readerForums = computed(() => allForums.value.filter(f => f.type === 'reader'));
const generalForums = computed(() => allForums.value.filter(f => f.type === 'general'));
const sensitiveForums = computed(() => allForums.value.filter(f => f.type === 'sensitive'));
const customForums = computed(() => allForums.value.filter(f => f.type === 'custom'));

// 创建论坛
const showCreateForum = ref(false);
const creating = ref(false);
const createForm = ref({ name: '', description: '', password: '', is_public: true, room_code: '' });
const createdRoomCode = ref('');

// 房间号输入：自动转大写并过滤非法字符
function onRoomCodeInput() {
  createForm.value.room_code = (createForm.value.room_code || '').toUpperCase().replace(/[^A-Z0-9]/g, '');
}

async function copyRoomCode() {
  try {
    await navigator.clipboard.writeText(createdRoomCode.value);
    ElMessage.success('房间号已复制');
  } catch {
    ElMessage.warning('复制失败，请手动复制：' + createdRoomCode.value);
  }
}

async function copyRoom(code: string) {
  try {
    await navigator.clipboard.writeText(code);
    ElMessage.success('房间号已复制：' + code);
  } catch {
    ElMessage.warning('复制失败，请手动复制：' + code);
  }
}

function onCreateClosed() {
  createdRoomCode.value = '';
  createForm.value = { name: '', description: '', password: '', is_public: true, room_code: '' };
}

async function submitCreateForum() {
  if (!createForm.value.name.trim()) { ElMessage.warning('请填写论坛名称'); return; }
  creating.value = true;
  try {
    const res = await forumApi.createForum({
      name: createForm.value.name.trim(),
      description: createForm.value.description,
      password: createForm.value.password,
      is_public: createForm.value.is_public,
      room_code: createForm.value.room_code,
    });
    if (res.data.code === 0) {
      createdRoomCode.value = res.data.data?.room_code || '';
      ElMessage.success('论坛已创建，可直接使用（后续由管理员审查）');
      loadForums();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '创建失败');
  } finally {
    creating.value = false;
  }
}

// ====== 房间号进入（不可见论坛的凭证） ======
const roomCodeInput = ref('');
const enteringRoom = ref(false);

async function enterByRoomCode() {
  const code = roomCodeInput.value.trim().toUpperCase();
  if (!code) { ElMessage.warning('请输入房间号'); return; }
  enteringRoom.value = true;
  try {
    // 第一步：仅验证房间号，返回论坛信息与是否需要密码
    const res = await forumApi.enterByRoomCode(code);
    if (res.data.code === 0) {
      const d = res.data.data;
      if (d.need_password) {
        // 第二步：进入密码墙——输入正确密码后才进入
        try {
          const { value } = await ElMessageBox.prompt(
            `论坛「${d.name}」设置了进入密码，请输入密码：`, '进入密码',
            { confirmButtonText: '进入', cancelButtonText: '取消', inputType: 'password' }
          );
          if (!value) return;
          const res2 = await forumApi.enterByRoomCode(code, value);
          if (res2.data.code === 0) {
            router.push(`/forum/${d.forum_id}?room=${code}&password=${encodeURIComponent(value)}`);
          }
        } catch (e: any) {
          if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '密码错误');
        }
      } else {
        router.push(`/forum/${d.forum_id}?room=${code}`);
      }
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '房间号无效');
  } finally {
    enteringRoom.value = false;
  }
}

// 敏感分区确认
const showZoneGuard = ref(false);
const zoneGuardName = ref('');
const zoneGuardCross = ref(false);
let pendingForumId: number | null = null;

async function enterForum(forum: Forum) {
  // type===sensitive 直接触发隔离，否则走系统分区匹配
  if (forum.type === 'sensitive') {
    zoneGuardName.value = (forum as any).zone || forum.name;
    zoneGuardCross.value = false;
    pendingForumId = forum.id;
    showZoneGuard.value = true;
    return;
  }
  const guard = await shouldShowGuard((forum as any).zone || forum.name);
  if (guard?.needed) {
    zoneGuardName.value = forum.name;
    zoneGuardCross.value = guard.isCrossDomain;
    pendingForumId = forum.id;
    showZoneGuard.value = true;
    return;
  }
  router.push(`/forum/${forum.id}`);
}

// 进入自建论坛（有密码则先验证）
async function enterCustomForum(forum: Forum) {
  if (forum.creator_id === authStore.user?.id || authStore.user?.role === 'admin') {
    router.push(`/forum/${forum.id}`);
    return;
  }
  if (forum.has_password) {
    try {
      const { value } = await ElMessageBox.prompt('该论坛需要密码才能进入，请输入进入密码：', '进入密码', {
        confirmButtonText: '进入', cancelButtonText: '取消', inputType: 'password',
      });
      if (!value) return;
      const res = await forumApi.verifyPassword(forum.id, value);
      if (res.data.code === 0) {
        router.push(`/forum/${forum.id}?password=${encodeURIComponent(value)}`);
      }
    } catch (e: any) {
      if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '密码验证失败');
    }
    return;
  }
  router.push(`/forum/${forum.id}`);
}

function onZoneConfirmed() {
  markZoneConfirmed(zoneGuardName.value);
  setLastZone(zoneGuardName.value);
  showZoneGuard.value = false;
  if (pendingForumId !== null) {
    router.push(`/forum/${pendingForumId}`);
    pendingForumId = null;
  }
}

async function loadForums() {
  loading.value = true;
  try {
    const res = await forumApi.getForums('all');
    allForums.value = res.data.data || [];
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

onMounted(loadForums);
</script>

<style scoped>
.forum-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.forum-card {
  cursor: pointer;
}

.forum-card-sensitive {
  border-color: rgba(220, 38, 38, 0.3);
}

.forum-card h3 {
  font-size: 1.1rem;
  color: var(--primary-color);
  margin-bottom: 8px;
}

.forum-card p {
  color: #666;
  font-size: 0.9rem;
  margin-bottom: 12px;
}

.forum-meta {
  font-size: 0.8rem;
  color: var(--text-light);
  display: flex;
  align-items: center;
}

/* ====== 移动端论坛广场适配 ====== */
@media (max-width: 640px) {
  .forum-grid {
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 10px;
  }
  .forum-card h3 {
    font-size: 1rem;
  }
}
</style>

<style>
[data-theme="dark"] .forum-card {
  background: #1e293b;
  border-color: rgba(255,255,255,.08);
}
[data-theme="dark"] .forum-card-sensitive {
  border-color: rgba(220, 38, 38, 0.4);
}
[data-theme="dark"] .forum-card h3 {
  color: #e2e8f0;
}
[data-theme="dark"] .forum-card p {
  color: #94a3b8;
}
[data-theme="dark"] .forum-meta {
  color: #94a3b8;
}
</style>
