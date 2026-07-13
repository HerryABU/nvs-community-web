<template>
  <div class="comment-section">
    <!-- 发表评论 -->
    <div class="comment-form" v-if="authStore.isLoggedIn">
      <el-input
        v-model="newComment"
        type="textarea"
        :rows="3"
        placeholder="写下你的评论... 支持 **粗体** *斜体* `代码` 等 Markdown 语法"
        maxlength="2000"
        show-word-limit
      />
      <div class="form-actions">
        <span class="tip-text">文明评论，理性表达</span>
        <el-button type="primary" size="small" :loading="submitting" @click="submitComment">
          发表评论
        </el-button>
      </div>
    </div>
    <div v-else class="comment-login-hint">
      <router-link to="/login">登录</router-link> 后即可发表评论
    </div>

    <!-- 评论列表（嵌套展示） -->
    <div class="comment-list" v-if="displayComments.length > 0">
      <template v-for="comment in displayComments" :key="comment.id">
        <CommentItem
          :comment="comment"
          @reply="handleReply"
          @delete="handleDelete"
          @refresh="fetchComments(true)"
        />
      </template>
    </div>
    <el-empty v-else description="暂无评论" />

    <!-- 加载更多 -->
    <div class="load-more" v-if="hasMore">
      <el-button text :loading="loadingMore" @click="loadMore">加载更多</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { novelApi, commentApi, type Comment } from '@/api/novel';
import { useAuthStore } from '@/stores/auth';
import { ElMessage, ElMessageBox } from 'element-plus';
import { shouldShowGuard } from '@/utils/sensitiveZone';
import CommentItem from './CommentItem.vue';

const props = defineProps<{
  novelId: number;
  chapterNumber?: number;
  novelCategory?: string;
  blogId?: number;
}>();

const authStore = useAuthStore();

const comments = ref<Comment[]>([]);
const newComment = ref('');
const submitting = ref(false);
const page = ref(1);
const hasMore = ref(false);
const loadingMore = ref(false);

// 将平铺评论转为嵌套结构：parent_id 非空的作为子评论挂到父评论下
const displayComments = computed(() => {
  const all = comments.value;
  const parentMap = new Map<number, Comment & { children: Comment[] }>();

  // 第一遍：收集所有父评论（parent_id 为空/0）
  for (const c of all) {
    if (!c.parent_id) {
      parentMap.set(c.id, { ...c, children: [] });
    }
  }

  // 第二遍：将子评论挂到对应父评论下
  for (const c of all) {
    if (c.parent_id && parentMap.has(c.parent_id)) {
      parentMap.get(c.parent_id)!.children!.push({ ...c });
    }
  }

  // 返回父评论列表（带 children）
  return Array.from(parentMap.values());
});

async function fetchComments(reset = false) {
  if (reset) page.value = 1;
  try {
    const res = await commentApi.getComments({
      novel_id: props.blogId ? 0 : props.novelId,
      blog_id: props.blogId || 0,
      chapter_number: props.chapterNumber,
      page: page.value,
    });
    const data = res.data.data;
    if (reset) {
      comments.value = data.list || [];
    } else {
      comments.value.push(...(data.list || []));
    }
    hasMore.value = (data.list?.length || 0) >= 20;
  } catch (e) {
    console.error(e);
  }
}

// 跨域评论警告：敏感分区评论需两级确认（文化差异 + 法律风险）
async function crossDomainCheck(): Promise<boolean> {
  const cat = props.novelCategory;
  if (!cat) return true;

  const guard = shouldShowGuard(cat);
  if (!guard?.needed) return true;

  const key = `nvs-cross-comment-warned-${cat}`;
  if (localStorage.getItem(key) === '1') return true;

  // 第1级：文化差异提醒
  try {
    await ElMessageBox.confirm(
      `您正在「${cat}」发表评论。该区域具有特定的文化语境和社群规范，请确保您的评论尊重该区域的文化背景和读者感受。`,
      '跨区评论提醒 (1/2)',
      { confirmButtonText: '我了解，继续评论', cancelButtonText: '取消', type: 'warning' }
    );
  } catch {
    return false;
  }

  // 第2级：法律风险告知（评论场景不禁止粘贴）
  try {
    await ElMessageBox.prompt(
      `您确认在「${cat}」发表评论。请手动输入"我承诺承担法律责任"以继续：`,
      '法律风险告知 (2/2)',
      {
        confirmButtonText: '确认',
        cancelButtonText: '取消',
        inputPattern: /^我承诺承担法律责任$/,
        inputErrorMessage: '请准确输入"我承诺承担法律责任"',
        type: 'warning',
      }
    );
  } catch {
    return false;
  }

  localStorage.setItem(key, '1');
  return true;
}

async function submitComment() {
  if (!newComment.value.trim()) return;

  // 跨域评论检查
  const allowed = await crossDomainCheck();
  if (!allowed) return;

  submitting.value = true;
  try {
    await commentApi.createComment({
      novel_id: props.blogId ? 0 : props.novelId,
      blog_id: props.blogId || 0,
      chapter_number: props.chapterNumber,
      content: newComment.value.trim(),
    });
    newComment.value = '';
    ElMessage.success('评论发表成功');
    fetchComments(true);
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '发表失败');
  } finally {
    submitting.value = false;
  }
}

async function handleReply(parentId: number, content: string) {
  const allowed = await crossDomainCheck();
  if (!allowed) return;
  try {
    await commentApi.createComment({
      novel_id: props.blogId ? 0 : props.novelId,
      blog_id: props.blogId || 0,
      chapter_number: props.chapterNumber,
      content,
      parent_id: parentId,
    });
    ElMessage.success('回复成功');
    fetchComments(true);
  } catch (e: any) {
    ElMessage.error('回复失败');
  }
}

async function handleDelete(id: number) {
  try {
    await ElMessageBox.confirm('确定删除这条评论？', '删除确认', {
      confirmButtonText: '确认删除',
      type: 'warning',
    });
    await commentApi.deleteComment(id);
    comments.value = comments.value.filter(c => c.id !== id);
    ElMessage.success('已删除');
  } catch {
    // 取消
  }
}

async function loadMore() {
  loadingMore.value = true;
  page.value++;
  await fetchComments(false);
  loadingMore.value = false;
}

onMounted(() => fetchComments(true));
</script>

<style scoped>
.comment-form {
  margin-bottom: 24px;
}

.form-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 8px;
}

.tip-text {
  font-size: 0.8rem;
  color: var(--text-light);
}

.comment-login-hint {
  text-align: center;
  padding: 24px;
  color: var(--text-light);
  font-size: 0.9rem;
}

.comment-list {
  margin-top: 16px;
}

.load-more {
  text-align: center;
  margin-top: 16px;
}
</style>

<style>
[data-theme="dark"] .comment-login-hint {
  color: #94a3b8;
}
[data-theme="dark"] .tip-text {
  color: #94a3b8;
}
</style>
