<template>
  <div class="page-container" v-loading="loading">
    <template v-if="thread">
      <div class="thread-header">
        <h1>{{ thread.title }}</h1>
        <div class="thread-meta">
          <span>{{ thread.user?.username || '匿名' }}</span>
          <span>{{ formatTime(thread.created_at) }}</span>
          <span>{{ thread.view_count }} 浏览 · {{ thread.post_count }} 回复</span>
        </div>
      </div>

      <!-- 主帖内容 -->
      <div class="thread-content">
        <div v-html="renderMarkdown(thread.content)"></div>
      </div>

      <!-- 回复列表 -->
      <h2 class="section-title" style="margin-top: 32px">回复（{{ total }}）</h2>
      <div class="post-list">
        <div v-for="p in posts" :key="p.id" class="post-item">
          <div class="post-header">
            <strong>{{ p.user?.username || '匿名' }}</strong>
            <span class="post-time">{{ formatTime(p.created_at) }}</span>
          </div>
          <div class="post-content">
            <div v-html="renderMarkdown(p.content)"></div>
          </div>
        </div>
        <el-empty v-if="posts.length === 0" description="暂无回复" />
      </div>

      <!-- 回复表单 -->
      <div class="reply-form" v-if="authStore.isLoggedIn">
        <el-input v-model="replyContent" type="textarea" :rows="3" placeholder="写下你的回复... 支持 **粗体** *斜体* `代码` 等 Markdown 语法" maxlength="2000" show-word-limit />
        <div class="reply-actions">
          <el-button type="primary" :loading="replying" @click="submitReply">发表回复</el-button>
        </div>
      </div>
      <div v-else-if="!authStore.loading" class="login-hint">
        <router-link to="/login">登录</router-link> 后即可回复
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { novelApi, forumApi } from '@/api/novel';
import { useAuthStore } from '@/stores/auth';
import { ElMessage } from 'element-plus';
import { renderMarkdown } from '@/markdown/renderer';

const route = useRoute();
const authStore = useAuthStore();
const loading = ref(false);
const thread = ref<any>(null);
const posts = ref<any[]>([]);
const total = ref(0);
const replyContent = ref('');
const replying = ref(false);

function formatTime(s: string) {
  return new Date(s).toLocaleDateString('zh-CN');
}

async function load() {
  loading.value = true;
  try {
    const res = await forumApi.getThread(Number(route.params.id));
    const d = res.data.data;
    thread.value = d.thread;
    posts.value = d.posts || [];
    total.value = d.total || 0;
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

async function submitReply() {
  if (!replyContent.value.trim()) return;
  replying.value = true;
  try {
    await forumApi.createPost(Number(route.params.id), { content: replyContent.value });
    ElMessage.success('回复成功');
    replyContent.value = '';
    load();
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '回复失败');
  } finally {
    replying.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
.thread-header {
  margin-bottom: 24px;
}

.thread-header h1 {
  font-size: 1.5rem;
  color: var(--primary-color);
  margin-bottom: 8px;
}

.thread-meta {
  display: flex;
  gap: 16px;
  font-size: 0.85rem;
  color: var(--text-light);
}

.thread-content {
  background: #fafafa;
  padding: 20px;
  border-radius: 8px;
  line-height: 1.8;
  white-space: pre-wrap;
  font-size: 0.95rem;
}

.post-item {
  padding: 16px 0;
  border-bottom: 1px solid #f0f0f0;
}

.post-header {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 6px;
}

.post-time {
  font-size: 0.8rem;
  color: var(--text-light);
}

.post-content {
  line-height: 1.7;
  white-space: pre-wrap;
}

.reply-form {
  margin-top: 24px;
}

.reply-actions {
  margin-top: 8px;
}

.login-hint {
  text-align: center;
  padding: 20px;
  color: var(--text-light);
}
</style>

<style>
[data-theme="dark"] .thread-header h1 {
  color: #e2e8f0;
}
[data-theme="dark"] .thread-meta {
  color: #94a3b8;
}
[data-theme="dark"] .thread-content {
  background: #1e293b;
  color: #cbd5e1;
}
[data-theme="dark"] .post-item {
  border-bottom-color: rgba(255,255,255,.06);
}
[data-theme="dark"] .post-header strong {
  color: #e2e8f0;
}
[data-theme="dark"] .post-time {
  color: #94a3b8;
}
[data-theme="dark"] .post-content {
  color: #cbd5e1;
}
[data-theme="dark"] .login-hint {
  color: #94a3b8;
}
</style>
