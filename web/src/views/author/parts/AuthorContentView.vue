<template>
  <div class="content-view">
    <!-- 我创建的论坛（可管理：禁言/解禁，管理员封禁的仅管理员可解） -->
    <div class="novels-section" v-if="myForums.length > 0">
      <h2 class="section-title-bar">
        <span class="title-bar-text">我创建的论坛</span>
        <span class="title-bar-count">{{ myForums.length }} 个</span>
      </h2>
      <div class="forum-admin-list">
        <div v-for="f in myForums" :key="f.id" class="forum-admin-item">
          <div class="forum-admin-main">
            <span class="forum-admin-name">{{ f.name }}</span>
            <el-tag v-if="f.room_code" size="small" style="cursor:pointer" title="点击复制房间号" @click="copyRoomCode(f.room_code)">🚪 {{ f.room_code }}</el-tag>
            <el-tag v-if="f.has_password" size="small" type="warning">🔒 密码</el-tag>
            <el-tag v-if="f.review_status === 'pending'" size="small" type="info">待审查</el-tag>
            <el-tag v-if="f.review_status === 'rejected'" size="small" type="danger">已驳回</el-tag>
            <span class="forum-admin-meta">{{ f.thread_count }} 个帖子</span>
          </div>
          <el-button size="small" type="primary" plain @click="$router.push(`/forum/${f.id}`)">管理</el-button>
        </div>
      </div>
    </div>

    <!-- 回收站（用户自删内容，可恢复；管理员封禁的不可自行恢复） -->
    <div class="novels-section" v-if="recycle.novels.length > 0 || recycle.blogs.length > 0">
      <h2 class="section-title-bar">
        <span class="title-bar-text">🗑 回收站</span>
        <span class="title-bar-count">{{ recycle.novels.length + recycle.blogs.length }} 项</span>
      </h2>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px"
        title="以下内容由您自己删除，可随时恢复；被管理员封禁的内容不在此处，请联系平台申诉。" />
      <div class="recycle-list">
        <div v-for="n in recycle.novels" :key="'rn' + n.id" class="recycle-item">
          <el-tag type="warning" size="small">小说</el-tag>
          <span class="recycle-title">《{{ n.title }}》</span>
          <span class="recycle-meta">{{ n.total_chapters || 0 }} 章 · {{ (n.total_words || 0).toLocaleString() }} 字</span>
          <el-button size="small" type="success" plain @click="restoreRecycle('novel', n.id)">恢复</el-button>
        </div>
        <div v-for="b in recycle.blogs" :key="'rb' + b.id" class="recycle-item">
          <el-tag type="warning" size="small">博客</el-tag>
          <span class="recycle-title">{{ b.title }}</span>
          <span class="recycle-meta">{{ formatDate(b.created_at) }}</span>
          <el-button size="small" type="success" plain @click="restoreRecycle('blog', b.id)">恢复</el-button>
        </div>
      </div>
    </div>

    <!-- 封禁内容（176号令合规告知） -->
    <div class="novels-section banned-section" v-if="bannedContent.novels.length > 0 || bannedContent.chapters.length > 0">
      <h2 class="section-title-bar">
        <span class="title-bar-text">已封禁内容</span>
        <span class="title-bar-count">{{ bannedContent.novels.length + bannedContent.chapters.length }} 项</span>
      </h2>
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 12px"
        title="以下内容因违反平台合规规范（176号令）被封禁，如有异议请联系平台申诉。" />
      <div class="banned-list">
        <div v-for="n in bannedContent.novels" :key="'n' + n.id" class="banned-item">
          <div class="banned-head">
            <el-tag type="danger" size="small">小说</el-tag>
            <span class="banned-title">《{{ n.title }}》</span>
          </div>
          <div class="banned-reason"><span class="banned-label">封禁原因：</span>{{ n.ban_reason || '未填写' }}</div>
          <div class="banned-time">封禁时间：{{ formatDate(n.banned_at) }}</div>
        </div>
        <div v-for="c in bannedContent.chapters" :key="'c' + c.id" class="banned-item">
          <div class="banned-head">
            <el-tag type="danger" size="small">章节</el-tag>
            <span class="banned-title">《{{ c.novel_title }}》第{{ c.chapter_number }}章 {{ c.title }}</span>
          </div>
          <div class="banned-reason"><span class="banned-label">封禁原因：</span>{{ c.ban_reason || '未填写' }}</div>
          <div class="banned-time">封禁时间：{{ formatDate(c.banned_at) }}</div>
        </div>
      </div>
    </div>

    <el-empty
      v-if="!myForums.length && !recycle.novels.length && !recycle.blogs.length && !bannedContent.novels.length && !bannedContent.chapters.length"
      description="暂无论坛 / 回收站 / 封禁记录"
    />
  </div>
</template>

<script setup lang="ts">
import { authorDataVersion, bumpAuthorData } from './authorRefresh';
import { ref, onMounted, watch } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { novelApi } from '@/api/novel';
import { forumApi } from '@/api/novel';

// 论坛 / 回收站 / 封禁内容视图：与作品、博客分开的独立视图。
// 恢复回收站内容后，需要通知其它视图刷新（父组件通过 refreshKey 重新挂载当前视图）。
const myForums = ref<any[]>([]);
const forumQuota = ref<any>(null);

// 回收站（用户自删内容，可恢复；管理员封禁的不可自行恢复）
const recycle = ref<{ novels: any[]; blogs: any[] }>({ novels: [], blogs: [] });

// 被封禁（176号令）的小说/章节及原因
const bannedContent = ref<{ novels: any[]; chapters: any[] }>({ novels: [], chapters: [] });

function formatDate(d?: string) { if (!d) return ''; return new Date(d).toLocaleString('zh-CN'); }

async function loadMyForums() {
  try {
    const res = await forumApi.listMyForums();
    if (res.data.code === 0) {
      const d = res.data.data || {};
      // 后端返回 { list, quota }；兼容历史裸数组
      myForums.value = Array.isArray(d) ? d : (d.list || []);
      forumQuota.value = Array.isArray(d) ? null : (d.quota || null);
    }
  } catch { /* ignore */ }
}

async function loadRecycle() {
  try {
    const res = await novelApi.getRecycle();
    if (res.data.code === 0) {
      const d = res.data.data || {};
      recycle.value = {
        novels: Array.isArray(d.novels) ? d.novels : [],
        blogs: Array.isArray(d.blogs) ? d.blogs : [],
      };
    }
  } catch { /* ignore */ }
}

async function loadBannedContent() {
  try {
    const res = await novelApi.getBannedContent();
    if (res.data.code === 0) {
      const d = res.data.data || {};
      bannedContent.value = {
        novels: Array.isArray(d.novels) ? d.novels : [],
        chapters: Array.isArray(d.chapters) ? d.chapters : [],
      };
    }
  } catch { /* ignore */ }
}

async function restoreRecycle(type: string, id: number) {
  try {
    await ElMessageBox.confirm('确定恢复这项内容吗？', '恢复确认', {
      confirmButtonText: '恢复', type: 'warning',
    });
    await novelApi.restoreRecycle({ type, id });
    ElMessage.success('已恢复');
    loadRecycle();
    // 通知其它视图（我的作品 / 我的博客）重新拉取
    bumpAuthorData();
  } catch { /* 取消 */ }
}

async function copyRoomCode(code: string) {
  try {
    await navigator.clipboard.writeText(code);
    ElMessage.success('房间号已复制：' + code);
  } catch {
    ElMessage.warning('复制失败，请手动复制：' + code);
  }
}

onMounted(() => {
  loadMyForums();
  loadRecycle();
  loadBannedContent();
});

// 其它视图改动了数据（如回收站恢复作品）→ 重新拉取
watch(authorDataVersion, () => {
  loadMyForums();
  loadRecycle();
  loadBannedContent();
});
</script>

<style scoped>
@import './section.css';


/* ===== 封禁内容（176号令） ===== */
.banned-section { margin-top: 32px; }
.banned-list { display: flex; flex-direction: column; gap: 12px; }

/* ===== 回收站 ===== */
.recycle-list { display: flex; flex-direction: column; gap: 10px; }
.recycle-item {
  display: flex; align-items: center; gap: 10px;
  padding: 12px 16px; border-radius: 10px;
  background: rgba(245, 158, 11, 0.06);
  border: 1px solid rgba(245, 158, 11, 0.3);
  flex-wrap: wrap;
}
.recycle-title { font-weight: 600; color: #e2e8f0; flex: 1; min-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
[data-theme="light"] .recycle-title { color: var(--text-color); }
.recycle-meta { font-size: 0.78rem; color: var(--text-light, #999); white-space: nowrap; }
.forum-admin-list { display: flex; flex-direction: column; gap: 10px; }
.forum-admin-item {
  display: flex; align-items: center; justify-content: space-between; gap: 10px;
  padding: 12px 16px; border-radius: 10px;
  background: var(--bg-card, #fff); border: 1px solid var(--glass-border, #e5e7eb);
}
.forum-admin-main { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; min-width: 0; }
.forum-admin-name { font-weight: 600; color: var(--text-color, #333); font-size: 0.95rem; }
.forum-admin-meta { font-size: 0.78rem; color: var(--text-light, #999); }
.banned-item {
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid rgba(239, 68, 68, 0.35);
  background: rgba(239, 68, 68, 0.08);
}
.banned-head { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.banned-title { font-weight: 600; color: #e2e8f0; }
[data-theme="light"] .banned-title { color: var(--text-color); }
.banned-reason { font-size: 0.88rem; color: #fca5a5; line-height: 1.5; }
[data-theme="light"] .banned-reason { color: #b91c1c; }
.banned-label { font-weight: 600; }
.banned-time { font-size: 0.78rem; color: rgba(255,255,255,0.45); margin-top: 4px; }

@media (max-width: 768px) {
  /* 论坛管理列表：行式改纵向 */
  .forum-admin-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
}
</style>
