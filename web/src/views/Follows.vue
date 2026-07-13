<template>
  <div class="page-container">
    <div class="follow-header">
      <h2>我的关注</h2>
      <el-tabs v-model="activeTab" @tab-change="loadData">
        <el-tab-pane label="我关注的人" name="following" />
        <el-tab-pane label="关注我的人" name="followers" />
      </el-tabs>
    </div>

    <div class="user-list" v-loading="loading">
      <div v-if="users.length === 0 && !loading" class="empty-hint">
        {{ activeTab === 'following' ? '还没有关注任何人' : '还没有人关注你' }}
      </div>
      <div v-for="user in users" :key="user.id" class="user-card" @click="$router.push(`/author/${user.id}`)">
        <el-avatar :size="48" :src="user.avatar_url">{{ user.nickname?.[0] || user.username?.[0] }}</el-avatar>
        <div class="user-info">
          <div class="user-name">{{ user.nickname || user.username }}</div>
          <div class="user-bio">{{ user.bio || '' }}</div>
        </div>
        <el-button v-if="activeTab === 'following'" size="small" type="danger" plain @click.stop="doUnfollow(user.id)" :loading="unfollowingId === user.id">
          取消关注
        </el-button>
      </div>
    </div>

    <div class="pagination" v-if="total > 20">
      <el-pagination
        v-model:current-page="page"
        :page-size="20"
        :total="total"
        layout="prev, pager, next"
        @current-change="loadData"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { followApi } from '@/api/social';
import { ElMessage } from 'element-plus';

interface UserBrief {
  id: number;
  username: string;
  nickname: string;
  avatar_url: string;
  bio: string;
}

const activeTab = ref('following');
const users = ref<UserBrief[]>([]);
const total = ref(0);
const page = ref(1);
const loading = ref(false);
const unfollowingId = ref(0);

async function loadData() {
  loading.value = true;
  try {
    const fn = activeTab.value === 'following' ? followApi.listFollowing : followApi.listFollowers;
    const res = await fn(page.value);
    if (res.data.code === 0) {
      users.value = res.data.data.list || [];
      total.value = res.data.data.total || 0;
    }
  } catch { /* ignore */ }
  finally { loading.value = false; }
}

async function doUnfollow(id: number) {
  unfollowingId.value = id;
  try {
    await followApi.unfollow(id);
    ElMessage.success('已取消关注');
    loadData();
  } catch {
    ElMessage.error('操作失败');
  }
  unfollowingId.value = 0;
}

onMounted(() => { loadData(); });
</script>

<style scoped>
.page-container { max-width: 700px; margin: 0 auto; padding: 24px; }
.follow-header { margin-bottom: 16px; }
.follow-header h2 { margin: 0 0 12px 0; }
.user-list { display: flex; flex-direction: column; gap: 8px; }
.user-card {
  display: flex; align-items: center; gap: 14px;
  padding: 14px 16px; background: #fff; border-radius: 10px;
  cursor: pointer; transition: box-shadow 0.2s; border: 1px solid #eee;
}
.user-card:hover { box-shadow: 0 2px 10px rgba(0,0,0,0.08); }
.user-info { flex: 1; }
.user-name { font-weight: 600; font-size: 0.95rem; }
.user-bio { color: #999; font-size: 0.8rem; margin-top: 2px; }
.empty-hint { text-align: center; padding: 48px; color: #999; }
.pagination { margin-top: 20px; display: flex; justify-content: center; }
</style>