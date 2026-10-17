<template>
  <div class="page-container">
    <div class="follow-header">
      <h2>特别关注</h2>
      <div class="header-sub">
        <span class="sub-hint">特别关注作者的最新动态，会汇总到动态流</span>
        <el-button size="small" @click="$router.push('/feed')">查看动态流</el-button>
      </div>
    </div>

    <div class="user-list" v-loading="loading">
      <div v-if="users.length === 0 && !loading" class="empty-hint">
        还没有特别关注任何人<br />
        在作者主页点「特别关注」，他的更新会优先出现在动态流
      </div>
      <div v-for="user in users" :key="user.id" class="user-card" @click="$router.push(`/author/${user.id}`)">
        <el-avatar :size="48" :src="user.avatar_url">{{ user.nickname?.[0] || user.username?.[0] }}</el-avatar>
        <div class="user-info">
          <div class="user-name">
            {{ user.nickname || user.username }}
            <el-tag size="small" type="warning" effect="light" round>特别关注</el-tag>
          </div>
          <div class="user-bio">{{ user.bio || '这位作者还没有简介' }}</div>
        </div>
        <el-button
          size="small"
          type="danger"
          plain
          :loading="cancelingId === user.id"
          @click.stop="doCancel(user.id)"
        >
          取消特别关注
        </el-button>
      </div>
    </div>

    <div class="pagination" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="loadData"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { specialApi, type SpecialFollowUser } from '@/api/social/special';
import { ElMessage } from 'element-plus';

const users = ref<SpecialFollowUser[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;
const loading = ref(false);
const cancelingId = ref(0);

async function loadData() {
  loading.value = true;
  try {
    const res = await specialApi.listSpecialFollows(page.value, pageSize);
    if (res.data.code === 0) {
      users.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
    }
  } catch { /* ignore */ }
  finally { loading.value = false; }
}

// 取消特别关注（保留普通关注关系）
async function doCancel(id: number) {
  cancelingId.value = id;
  try {
    const res = await specialApi.toggleSpecialFollow(id, false);
    if (res.data.code === 0) {
      ElMessage.success('已取消特别关注');
      // 当前页取消后若为空且不是第一页，回退一页
      if (users.value.length === 1 && page.value > 1) {
        page.value -= 1;
      }
      loadData();
    }
  } catch {
    ElMessage.error('操作失败');
  }
  cancelingId.value = 0;
}

onMounted(() => { loadData(); });
</script>

<style scoped>
.page-container { max-width: 700px; margin: 0 auto; padding: 24px; }
.follow-header { margin-bottom: 16px; }
.follow-header h2 { margin: 0 0 12px 0; }
.header-sub { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.sub-hint { color: var(--text-light, #999); font-size: 0.8rem; }
.user-list { display: flex; flex-direction: column; gap: 8px; }
.user-card {
  display: flex; align-items: center; gap: 14px;
  padding: 14px 16px; background: var(--el-bg-color-overlay, #fff); border-radius: 10px;
  cursor: pointer; transition: box-shadow 0.2s; border: 1px solid var(--border-color, #eee);
}
.user-card:hover { box-shadow: 0 2px 10px rgba(0,0,0,0.08); }
.user-info { flex: 1; }
.user-name { font-weight: 600; font-size: 0.95rem; display: flex; align-items: center; gap: 6px; }
.user-bio { color: var(--text-light, #999); font-size: 0.8rem; margin-top: 2px; }
.empty-hint { text-align: center; padding: 48px 16px; color: var(--text-light, #999); line-height: 1.8; }
.pagination { margin-top: 20px; display: flex; justify-content: center; }

/* ====== 移动端适配 ====== */
@media (max-width: 640px) {
  .user-card {
    padding: 12px;
    gap: 10px;
  }
  .user-card .el-button {
    flex-shrink: 0;
  }
  .header-sub { flex-direction: column; align-items: flex-start; }
  .pagination .el-pagination {
    flex-wrap: wrap;
    justify-content: center;
  }
}
</style>
