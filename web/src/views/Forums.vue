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
    <div class="self-built-header" style="margin-top:32px;display:flex;align-items:center;justify-content:space-between">
      <h2 class="section-title" style="margin:0">🏘️ 自建区</h2>
      <el-button v-if="authStore.isLoggedIn" type="primary" size="small" @click="showCreateForum = true">
        创建论坛
      </el-button>
      <router-link v-else to="/login" class="create-forum-login">登录后可创建论坛</router-link>
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
        </div>
      </el-card>
    </div>
    <el-empty v-else description="暂无自建论坛" :image-size="40" />

    <!-- 创建论坛弹窗 -->
    <el-dialog v-model="showCreateForum" title="创建自建论坛" width="520px" destroy-on-close>
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
        <el-form-item label="公开显示">
          <el-switch v-model="createForm.is_public" />
          <span class="hint" style="margin-left:8px">开启后在大厅「自建区」公开展示，关闭则隐藏（仅本人可见）</span>
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
const createForm = ref({ name: '', description: '', password: '', is_public: true });

async function submitCreateForum() {
  if (!createForm.value.name.trim()) { ElMessage.warning('请填写论坛名称'); return; }
  creating.value = true;
  try {
    const res = await forumApi.createForum({
      name: createForm.value.name.trim(),
      description: createForm.value.description,
      password: createForm.value.password,
      is_public: createForm.value.is_public,
    });
    if (res.data.code === 0) {
      ElMessage.success('论坛已创建，可直接使用（后续由管理员审查）');
      showCreateForum.value = false;
      createForm.value = { name: '', description: '', password: '', is_public: true };
      loadForums();
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '创建失败');
  } finally {
    creating.value = false;
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
