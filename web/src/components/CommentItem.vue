<template>
  <div class="comment-item" :class="{ 'child-comment': depth > 0 }">
    <div class="comment-header">
      <span class="comment-user">{{ comment.username || '匿名用户' }}</span>
      <span class="comment-time">{{ formatTime(comment.created_at) }}</span>
      <span v-if="comment.chapter_number > 0" class="comment-chapter">
        第{{ comment.chapter_number }}章
      </span>
    </div>
    <div v-if="comment.quote_text" class="comment-quote">
      「{{ comment.quote_text }}」
    </div>
    <div class="comment-content">
      <v-md-preview :text="comment.content" />
    </div>
    <div class="comment-actions">
      <el-button v-if="authStore.isLoggedIn" size="small" text @click="showReply = !showReply">
        {{ showReply ? '取消回复' : '回复' }}
      </el-button>
      <el-button
        v-if="authStore.user?.id === comment.user_id"
        size="small"
        text
        type="danger"
        @click="$emit('delete', comment.id)"
      >
        删除
      </el-button>
    </div>

    <!-- 回复表单 -->
    <div v-if="showReply && authStore.isLoggedIn" class="reply-form">
      <el-input
        v-model="replyContent"
        type="textarea"
        :rows="2"
        placeholder="回复... 支持 Markdown 语法"
        maxlength="2000"
      />
      <div class="reply-form-actions">
        <el-button size="small" @click="showReply = false; replyContent = ''">取消</el-button>
        <el-button size="small" type="primary" @click="doReply">回复</el-button>
      </div>
    </div>

    <!-- 子评论列表（二级嵌套） -->
    <div v-if="comment.children && comment.children.length > 0" class="child-comments">
      <CommentItem
        v-for="child in comment.children"
        :key="child.id"
        :comment="child"
        :depth="(depth || 0) + 1"
        @reply="(parentId: number, content: string) => $emit('reply', parentId, content)"
        @delete="(id: number) => $emit('delete', id)"
        @refresh="$emit('refresh')"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useAuthStore } from '@/stores/auth';
import type { Comment } from '@/api/novel';
import VMdPreview from '@kangc/v-md-editor/lib/preview';

const props = defineProps<{
  comment: Comment & { children?: Comment[] };
  depth?: number;
}>();

const emit = defineEmits<{
  reply: [parentId: number, content: string];
  delete: [id: number];
  refresh: [];
}>();

const authStore = useAuthStore();
const showReply = ref(false);
const replyContent = ref('');

function formatTime(dateStr: string) {
  const d = new Date(dateStr);
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  if (diff < 60000) return '刚刚';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
  return d.toLocaleDateString('zh-CN');
}

function doReply() {
  if (!replyContent.value.trim()) return;
  emit('reply', props.comment.id, replyContent.value.trim());
  replyContent.value = '';
  showReply.value = false;
}
</script>

<style scoped>
.comment-item {
  padding: 16px 0;
  border-bottom: 1px solid #f0f0f0;
}

.comment-item:last-child {
  border-bottom: none;
}

.child-comment {
  margin-left: 32px;
  padding-left: 12px;
  border-left: 2px solid #e9ecef;
}

.comment-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
}

.comment-user {
  font-weight: 600;
  font-size: 0.9rem;
  color: var(--primary-color);
}

.comment-time {
  font-size: 0.8rem;
  color: var(--text-light);
}

.comment-chapter {
  font-size: 0.75rem;
  color: var(--accent-color);
  background: #fdf2e9;
  padding: 1px 6px;
  border-radius: 4px;
}

.comment-quote {
  background: #f5f7fa;
  padding: 6px 10px;
  border-left: 3px solid var(--accent-color);
  margin-bottom: 6px;
  font-size: 0.85rem;
  color: #666;
  border-radius: 0 4px 4px 0;
}

.comment-content {
  font-size: 0.95rem;
  line-height: 1.7;
  color: #333;
}

.comment-actions {
  margin-top: 6px;
}

.reply-form {
  margin-top: 10px;
}

.reply-form-actions {
  display: flex;
  gap: 6px;
  justify-content: flex-end;
  margin-top: 6px;
}

.child-comments {
  margin-top: 4px;
}
</style>

<style>
[data-theme="dark"] .comment-item {
  border-bottom-color: rgba(255,255,255,.06);
}
[data-theme="dark"] .child-comment {
  border-left-color: rgba(255,255,255,.1);
}
[data-theme="dark"] .comment-user {
  color: #e2e8f0;
}
[data-theme="dark"] .comment-time {
  color: #94a3b8;
}
[data-theme="dark"] .comment-chapter {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
}
[data-theme="dark"] .comment-quote {
  background: #1e293b;
  border-left-color: #f59e0b;
  color: #94a3b8;
}
[data-theme="dark"] .comment-content {
  color: #cbd5e1;
}
</style>
