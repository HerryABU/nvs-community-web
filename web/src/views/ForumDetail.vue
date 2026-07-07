<template>
  <div class="page-container" v-loading="loading">
    <template v-if="forum">
      <div class="forum-header">
        <h1>{{ forum.name }}</h1>
        <p class="forum-desc">{{ forum.description }}</p>
        <el-button type="primary" @click="showNewThread = true" v-if="authStore.isLoggedIn">
          发布新帖
        </el-button>
        <router-link v-else-if="!authStore.loading" to="/login" class="login-link">登录后即可发帖</router-link>
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
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { novelApi } from '@/api/novel';
import { useAuthStore } from '@/stores/auth';
import { ElMessage } from 'element-plus';

const route = useRoute();
const authStore = useAuthStore();
const loading = ref(false);
const forum = ref<any>(null);
const threads = ref<any[]>([]);
const subForums = ref<any[]>([]);
const showNewThread = ref(false);
const newTitle = ref('');
const newContent = ref('');
const posting = ref(false);

function formatTime(s: string) {
  return new Date(s).toLocaleDateString('zh-CN');
}

async function load() {
  loading.value = true;
  try {
    const res = await novelApi.getForum(Number(route.params.id));
    const d = res.data.data;
    forum.value = d.forum;
    threads.value = d.threads || [];
    subForums.value = d.sub_forums || [];
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
}

async function submitThread() {
  if (!newTitle.value.trim() || !newContent.value.trim()) {
    ElMessage.warning('请填写标题和内容');
    return;
  }
  posting.value = true;
  try {
    await novelApi.createThread(Number(route.params.id), { title: newTitle.value, content: newContent.value });
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
