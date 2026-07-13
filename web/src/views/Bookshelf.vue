<template>
  <div class="bookshelf-page" v-loading="shelfLoading">
    <div class="page-container">
      <div class="shelf-header">
        <h1 class="section-title">
          📚 我的书架
          <span class="shelf-count" v-if="shelfTotal > 0">（{{ shelfTotal }} 部）</span>
        </h1>
      </div>

      <!-- 标签切换 -->
      <el-tabs v-model="activeTab" class="shelf-tabs" @tab-change="onTabChange">
        <el-tab-pane label="书架" name="shelf" />
        <el-tab-pane label="我的关注" name="following" />
        <el-tab-pane label="关注我的人" name="followers" />
      </el-tabs>

      <!-- 书架 -->
      <template v-if="activeTab === 'shelf'">
        <el-empty
          v-if="!shelfLoading && items.length === 0"
          description="书架空空如也，去发现好作品吧"
          :image-size="160"
        >
          <el-button type="primary" @click="$router.push('/')">去首页看看</el-button>
        </el-empty>

        <div v-else class="shelf-grid">
          <div v-for="item in items" :key="item.id" class="shelf-card" @click="goRead(item)">
            <div class="shelf-cover">
              <img v-if="item.novel.cover_url" :src="item.novel.cover_url" :alt="item.novel.title" />
              <div v-else class="cover-placeholder">{{ item.novel.title[0] }}</div>
            </div>
            <div class="shelf-info">
              <h3 class="shelf-title">{{ item.novel.title }}</h3>
              <p class="shelf-author">作者：{{ item.novel.author?.nickname || item.novel.author?.username || '未知' }}</p>
              <div class="shelf-meta">
                <span class="meta-tag">{{ item.novel.category }}</span>
                <span>{{ item.novel.total_chapters }} 章</span>
                <span>{{ formatWords(item.novel.total_words) }}</span>
              </div>
              <div class="shelf-progress" v-if="item.last_read_chapter > 0">
                <el-progress
                  :percentage="item.novel.total_chapters > 0 ? Math.round((item.last_read_chapter / item.novel.total_chapters) * 100) : 0"
                  :stroke-width="6" :show-text="false"
                />
                <span class="progress-text">已读至第 {{ item.last_read_chapter }} 章（{{ item.last_read_chapter }}/{{ item.novel.total_chapters }}）</span>
              </div>
              <div class="shelf-progress" v-else>
                <span class="progress-text new-text">新添加到书架</span>
              </div>
              <div class="shelf-status">
                <el-tag :type="item.novel.status === 'completed' ? 'success' : ''" size="small">
                  {{ item.novel.status === 'completed' ? '已完结' : '连载中' }}
                </el-tag>
              </div>
            </div>
            <el-popconfirm title="确认从书架移除？" confirm-button-text="移除" cancel-button-text="取消" @confirm.stop="handleRemove(item.novel_id)">
              <template #reference>
                <el-button class="shelf-remove-btn" size="small" circle :icon="Delete" @click.stop />
              </template>
            </el-popconfirm>
          </div>
        </div>

        <div class="shelf-pagination" v-if="shelfTotal > pageSize">
          <el-pagination v-model:current-page="shelfPage" :page-size="pageSize" :total="shelfTotal" layout="prev, pager, next" @current-change="loadBookshelf" />
        </div>
      </template>

      <!-- 关注 -->
      <template v-else>
        <div class="follow-list" v-loading="followLoading">
          <div v-if="followUsers.length === 0 && !followLoading" class="empty-hint">
            {{ activeTab === 'following' ? '还没有关注任何人' : '还没有人关注你' }}
          </div>
          <div v-for="user in followUsers" :key="user.id" class="follow-card" @click="$router.push(`/author/${user.id}`)">
            <el-avatar :size="48" :src="user.avatar_url">{{ user.nickname?.[0] || user.username?.[0] }}</el-avatar>
            <div class="follow-info">
              <div class="follow-name">{{ user.nickname || user.username }}</div>
              <div class="follow-bio">{{ user.bio || '' }}</div>
            </div>
            <el-button v-if="activeTab === 'following'" size="small" type="danger" plain @click.stop="doUnfollow(user.id)" :loading="unfollowingId === user.id">
              取消关注
            </el-button>
          </div>
        </div>
        <div class="shelf-pagination" v-if="followTotal > 20">
          <el-pagination v-model:current-page="followPage" :page-size="20" :total="followTotal" layout="prev, pager, next" @current-change="loadFollowData" />
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { bookshelfApi, type ShelfItem } from '@/api/bookshelf';
import { followApi } from '@/api/social';
import { ElMessage } from 'element-plus';
import { Delete } from '@element-plus/icons-vue';

const router = useRouter();

// 书架
const items = ref<ShelfItem[]>([]);
const shelfTotal = ref(0);
const shelfLoading = ref(false);
const shelfPage = ref(1);
const pageSize = 50;

// 关注
const activeTab = ref('shelf');
const followUsers = ref<any[]>([]);
const followTotal = ref(0);
const followLoading = ref(false);
const followPage = ref(1);
const unfollowingId = ref(0);

function formatWords(n: number): string {
  if (n >= 10000) return (n / 10000).toFixed(1) + '万字';
  if (n >= 1000) return (n / 1000).toFixed(1) + '千字';
  return n + '字';
}

function goRead(item: ShelfItem) {
  const ch = item.last_read_chapter > 0 ? item.last_read_chapter : 1;
  router.push(`/novel/${item.novel_id}/read/${ch}`);
}

async function loadBookshelf() {
  shelfLoading.value = true;
  try {
    const res = await bookshelfApi.getList(shelfPage.value, pageSize);
    if (res.data.code === 0) {
      items.value = res.data.data.list || [];
      shelfTotal.value = res.data.data.total || 0;
    }
  } catch {
    ElMessage.error('加载书架失败');
  } finally {
    shelfLoading.value = false;
  }
}

async function handleRemove(novelId: number) {
  try {
    await bookshelfApi.remove(novelId);
    items.value = items.value.filter(i => i.novel_id !== novelId);
    shelfTotal.value--;
    ElMessage.success('已从书架移除');
  } catch {
    ElMessage.error('操作失败');
  }
}

async function loadFollowData() {
  followLoading.value = true;
  try {
    const fn = activeTab.value === 'following' ? followApi.listFollowing : followApi.listFollowers;
    const res = await fn(followPage.value);
    if (res.data.code === 0) {
      followUsers.value = res.data.data.list || [];
      followTotal.value = res.data.data.total || 0;
    }
  } catch { /* ignore */ }
  finally { followLoading.value = false; }
}

async function doUnfollow(id: number) {
  unfollowingId.value = id;
  try {
    await followApi.unfollow(id);
    ElMessage.success('已取消关注');
    loadFollowData();
  } catch {
    ElMessage.error('操作失败');
  }
  unfollowingId.value = 0;
}

function onTabChange(tab: string) {
  if (tab !== 'shelf') {
    followPage.value = 1;
    loadFollowData();
  }
}

onMounted(() => {
  loadBookshelf();
});
</script>

<style scoped>
.bookshelf-page {
  min-height: calc(100vh - 60px);
  background: var(--bg-color);
  padding-top: 20px;
}

.shelf-header { margin-bottom: 8px; }
.shelf-header .section-title { font-size: 1.4rem; }
.shelf-count { font-size: 0.9rem; color: var(--text-light); font-weight: 400; }

.shelf-tabs { margin-bottom: 16px; }

.shelf-grid { display: flex; flex-direction: column; gap: 12px; }

.shelf-card {
  position: relative;
  display: flex; gap: 16px;
  padding: 16px;
  background: #fff; border-radius: var(--radius-sm);
  box-shadow: var(--shadow-card);
  cursor: pointer;
  transition: box-shadow var(--transition-fast), transform var(--transition-fast);
  border: 1px solid var(--border-color);
}
.shelf-card:hover { box-shadow: var(--shadow-hover); transform: translateY(-2px); }

.shelf-cover {
  flex-shrink: 0; width: 80px; height: 106px;
  border-radius: 6px; overflow: hidden;
}
.shelf-cover img { width: 100%; height: 100%; object-fit: cover; }
.cover-placeholder {
  width: 100%; height: 100%;
  background: var(--bg-hover);
  display: flex; align-items: center; justify-content: center;
  font-size: 2rem; font-weight: 700;
  color: var(--text-light);
}

.shelf-info { flex: 1; min-width: 0; }
.shelf-title { font-size: 1rem; font-weight: 600; margin-bottom: 4px; }
.shelf-author { font-size: 0.8rem; color: var(--text-light); margin-bottom: 4px; }
.shelf-meta { display: flex; gap: 8px; font-size: 0.8rem; color: var(--text-light); margin-bottom: 8px; }
.meta-tag {
  background: var(--primary-light); color: var(--primary-color);
  padding: 0 6px; border-radius: 4px; font-size: 0.75rem;
}
.shelf-progress { margin-bottom: 6px; }
.progress-text { font-size: 0.75rem; color: var(--text-light); }
.new-text { color: var(--primary-color); font-weight: 500; }

.shelf-remove-btn {
  position: absolute; top: 8px; right: 8px;
  opacity: 0; transition: opacity 0.2s;
}
.shelf-card:hover .shelf-remove-btn { opacity: 1; }

.shelf-pagination { margin-top: 20px; display: flex; justify-content: center; }

/* 关注列表 */
.follow-list { display: flex; flex-direction: column; gap: 8px; }
.follow-card {
  display: flex; align-items: center; gap: 14px;
  padding: 14px 16px; background: #fff; border-radius: 10px;
  cursor: pointer; transition: box-shadow 0.2s;
  box-shadow: var(--shadow-card);
  border: 1px solid var(--border-color);
}
.follow-card:hover { box-shadow: var(--shadow-hover); }
.follow-info { flex: 1; }
.follow-name { font-weight: 600; font-size: 0.95rem; }
.follow-bio { color: #999; font-size: 0.8rem; margin-top: 2px; }
.empty-hint { text-align: center; padding: 48px; color: #999; }
</style>