<template>
  <div class="page-container me-center" v-loading="loading">
    <template v-if="profile">
      <!-- 个人信息头（瘦：仅头部 + 计数，不渲染图表/评论/博客列表） -->
      <div class="me-header">
        <el-avatar :size="80" :src="author.avatar_url || ''">
          {{ initial }}
        </el-avatar>
        <div class="me-info">
          <h1>{{ author.nickname || author.username }}</h1>
          <p class="me-bio" v-if="author.bio">{{ author.bio }}</p>
          <p class="me-signature" v-if="author.signature">✍️ {{ author.signature }}</p>
          <div class="me-stats">
            <span>作品：{{ profile.total_novels || 0 }}</span>
            <span>总字数：{{ (profile.total_words || 0).toLocaleString() }}</span>
            <span>总章节：{{ profile.total_chapters || 0 }}</span>
            <span>评论：{{ (profile.total_comments || 0).toLocaleString() }}</span>
            <el-tag v-if="author.role === 'vip_author'" type="warning" size="small">VIP作者</el-tag>
            <el-tag v-else-if="author.role === 'author'" size="small">作者</el-tag>
          </div>
          <div class="me-actions">
            <span style="color:var(--text-muted);font-size:0.85rem">{{ followStats.followers }} 粉丝 · {{ followStats.following }} 关注</span>
            <el-button type="primary" text size="small" @click="goForum">进入我的讨论区</el-button>
          </div>
        </div>
      </div>

      <!-- 功能类型 tab（路由切换，子页各自懒加载自身数据） -->
      <el-tabs v-model="activeTab" class="me-tabs" @tab-change="onTab">
        <el-tab-pane label="概况" name="overview" />
        <el-tab-pane label="作品" name="works" />
        <el-tab-pane label="博客" name="blogs" />
        <el-tab-pane label="留言" name="comments" />
        <el-tab-pane label="数据" name="stats" />
        <el-tab-pane label="装扮" name="decorate" />
      </el-tabs>

      <router-view />
    </template>
    <el-empty v-else-if="!loading" description="加载失败" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, provide, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { publicApi } from '@/api/admin';
import { followApi } from '@/api/social';
import { useAuthStore } from '@/stores/auth';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

// 当前登录用户即个人中心主体（/me 不带参数，永远指向自己）
const authorKey = ref(String(authStore.user?.id ?? ''));
const authorId = ref(0);
const profile = ref<any>(null);
const loading = ref(false);
const followStats = ref({ followers: 0, following: 0 });

const author = computed(() => profile.value?.author || {});
const initial = computed(() => (author.value.nickname?.[0] || author.value.username?.[0] || 'A'));

// 子页通过 inject 复用头部已拉取的 profile（避免重复请求）
provide('meProfile', profile);
provide('meAuthorId', authorId);
provide('meAuthorKey', authorKey);

const activeTab = computed({
  get: () => (route.name === 'me' ? 'overview' : String(route.name).replace('me-', '')),
  set: (t: string) => t,
});

function onTab(name: string) {
  router.push(name === 'overview' ? '/me' : `/me/${name}`);
}

async function load() {
  loading.value = true;
  try {
    const res = await publicApi.getAuthorProfile(authorKey.value);
    if (res.data.code === 0) {
      const d = res.data.data;
      profile.value = d;
      authorId.value = d.author_id || d.author?.id || 0;
    }
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
  // 关注计数（自己页也展示粉丝/关注数）
  try {
    const r = await followApi.stats();
    followStats.value = r.data.data || { followers: 0, following: 0 };
  } catch { /* ignore */ }
}

async function goForum() {
  if (!authorId.value) return;
  try {
    const res = await publicApi.getAuthorForum(authorId.value);
    const forumId = res.data.data?.forum?.id;
    if (forumId) router.push(`/forum/${forumId}`);
  } catch { /* ignore */ }
}

onMounted(load);
// 登录态切换后重新拉取（理论上 /me 已 requiresAuth）
watch(() => authStore.user?.id, (v) => { if (v) { authorKey.value = String(v); load(); } });
</script>

<style scoped>
.me-header {
  display: flex;
  gap: 24px;
  align-items: flex-start;
  margin-bottom: 24px;
  padding: 24px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 1px 6px rgba(0, 0, 0, 0.06);
}
[data-theme="dark"] .me-header {
  background: #1e293b;
  box-shadow: 0 1px 6px rgba(0, 0, 0, 0.25);
}
.me-info h1 {
  font-size: 1.5rem;
  color: var(--primary-color);
  margin-bottom: 6px;
}
.me-bio {
  color: #666;
  margin-bottom: 6px;
  max-width: 500px;
}
[data-theme="dark"] .me-bio { color: #94a3b8; }
.me-signature { color: var(--text-light); margin-bottom: 6px; font-size: 0.9rem; }
.me-stats {
  display: flex;
  gap: 16px;
  font-size: 0.85rem;
  color: var(--text-light);
  flex-wrap: wrap;
  align-items: center;
  margin-bottom: 8px;
}
.me-actions {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}
.me-tabs { margin-bottom: 16px; }

@media (max-width: 640px) {
  .me-header {
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 14px;
    padding: 20px 16px;
  }
  .me-stats { justify-content: center; }
  .me-info h1 { font-size: 1.3rem; }
}
</style>
