<template>
  <div class="page-container collab-page" v-loading="loading">
    <h1 class="cp-title">我的协作</h1>
    <p class="cp-sub">作为协作者参与的作品、待处理的邀请，以及收益分成与署名顺序</p>

    <!-- 待处理邀请 -->
    <section class="cp-section">
      <h2 class="cp-section-title">
        待处理邀请
        <el-tag v-if="invites.length" size="small" type="warning">{{ invites.length }}</el-tag>
      </h2>
      <el-empty v-if="invites.length === 0" description="暂无邀请" :image-size="60" />
      <div v-else class="cp-invite-list">
        <div v-for="it in invites" :key="it.id" class="cp-invite-item">
          <div class="cp-invite-main">
            <div class="cp-invite-title">{{ it.novel_title || `作品 #${it.novel_id}` }}</div>
            <div class="cp-invite-meta">
              邀请角色：<el-tag size="small">{{ roleLabel(it.role) }}</el-tag>
              <span class="cp-tip">接受后按角色获得相应权限</span>
            </div>
          </div>
          <div class="cp-invite-actions">
            <el-button size="small" type="primary" @click="respond(it.novel_id, true)">接受</el-button>
            <el-button size="small" @click="respond(it.novel_id, false)">拒绝</el-button>
          </div>
        </div>
      </div>
    </section>

    <!-- 参与的协作作品 -->
    <section class="cp-section">
      <h2 class="cp-section-title">参与协作的作品</h2>
      <el-empty v-if="works.length === 0" description="还没有参与任何协作作品" :image-size="60" />
      <div v-else class="cp-work-grid">
        <div v-for="w in works" :key="w.novel_id" class="cp-work-card">
          <div class="cp-work-cover">
            <el-image v-if="w.cover_url" :src="w.cover_url" fit="cover" class="cp-cover-img">
              <template #error><div class="cp-cover-ph">📖</div></template>
            </el-image>
            <div v-else class="cp-cover-ph">📖</div>
          </div>
          <div class="cp-work-body">
            <h3 class="cp-work-title">{{ w.title }}</h3>
            <div class="cp-work-meta">
              <el-tag size="small" type="primary">{{ w.role_label || roleLabel(w.role) }}</el-tag>
              <span>主笔：{{ w.owner_name || '—' }}</span>
            </div>
            <div class="cp-work-meta">
              <span>我写了 {{ w.chapter_count }} 章</span>
              <span>分成 {{ (w.revenue_share_bp / 100).toFixed(2) }}%</span>
              <span>署名次 {{ w.byline_order }}</span>
            </div>
          </div>
          <div class="cp-work-actions">
            <el-button size="small" type="primary" plain @click="goWrite(w)">
              <el-icon><EditPen /></el-icon>去写作
            </el-button>
            <el-button size="small" plain @click="$router.push(`/novel/${w.novel_id}`)">
              <el-icon><View /></el-icon>查看
            </el-button>
            <el-popconfirm title="确认退出该作品的协作？历史署名保留" @confirm="leave(w.novel_id)">
              <template #reference>
                <el-button size="small" type="danger" plain>退出</el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { EditPen, View } from '@element-plus/icons-vue';
import { collabApi, COLLAB_ROLE_LABEL, type Collaborator, type CollabWork } from '@/api/collab';

const router = useRouter();
const loading = ref(false);
const invites = ref<Collaborator[]>([]);
const works = ref<CollabWork[]>([]);

function roleLabel(role: string) {
  return COLLAB_ROLE_LABEL[role] || role;
}

async function load() {
  loading.value = true;
  try {
    const [invRes, workRes] = await Promise.all([collabApi.invites(), collabApi.works()]);
    invites.value = invRes.data.data?.list || [];
    works.value = workRes.data.data?.list || [];
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败');
  } finally {
    loading.value = false;
  }
}

async function respond(novelId: number, accept: boolean) {
  try {
    await collabApi.respond(novelId, accept);
    ElMessage.success(accept ? '已接受邀请' : '已拒绝邀请');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败');
  }
}

async function leave(novelId: number) {
  try {
    await collabApi.leave(novelId);
    ElMessage.success('已退出协作');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '退出失败');
  }
}

function goWrite(w: CollabWork) {
  router.push(`/author/editor/${w.novel_id}/chapter/1`);
}

onMounted(load);
</script>

<style scoped>
.collab-page {
  max-width: 1100px;
  margin: 0 auto;
  padding: 24px 16px 48px;
}
.cp-title {
  font-size: 1.6rem;
  margin: 0 0 4px;
}
.cp-sub {
  color: var(--text-muted);
  font-size: 0.85rem;
  margin: 0 0 20px;
}
.cp-section {
  margin-bottom: 28px;
}
.cp-section-title {
  font-size: 1.05rem;
  margin: 0 0 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.cp-invite-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.cp-invite-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-card);
}
.cp-invite-title {
  font-weight: 600;
  font-size: 0.95rem;
}
.cp-invite-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.8rem;
  color: var(--text-muted);
  margin-top: 4px;
}
.cp-invite-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}
.cp-work-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}
.cp-work-card {
  border: 1px solid var(--border-color);
  border-radius: 10px;
  overflow: hidden;
  background: var(--bg-card);
  display: flex;
  flex-direction: column;
}
.cp-work-cover {
  height: 120px;
  background: var(--bg-soft);
}
.cp-cover-img,
.cp-cover-ph {
  width: 100%;
  height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 2rem;
}
.cp-work-body {
  padding: 12px 14px;
  flex: 1;
}
.cp-work-title {
  font-size: 1rem;
  margin: 0 0 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cp-work-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  font-size: 0.78rem;
  color: var(--text-muted);
  margin-top: 6px;
}
.cp-work-actions {
  display: flex;
  gap: 8px;
  padding: 10px 14px;
  border-top: 1px solid var(--border-color);
  flex-wrap: wrap;
}
.cp-tip {
  color: var(--text-muted);
}

@media (max-width: 768px) {
  .collab-page {
    padding: 16px 10px 40px;
  }
  .cp-invite-item {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
