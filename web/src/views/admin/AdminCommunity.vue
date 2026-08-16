<template>
  <div class="admin-page">
    <div class="admin-page-header">
      <el-button text @click="$router.push('/admin')"><el-icon><ArrowLeft /></el-icon> 返回后台</el-button>
      <h2>社区动态</h2>
    </div>
    <div class="admin-page-body">
      <el-row :gutter="20">
        <el-col :xs="24" :md="12">
          <h4 class="sec-title">👤 最近注册用户</h4>
          <div v-loading="loading">
            <el-empty v-if="!loading && data.recent_users?.length === 0" description="暂无数据" />
            <div v-else class="community-list">
              <div v-for="u in data.recent_users" :key="u.id" class="community-item">
                <el-avatar :size="32" :src="u.avatar_url">{{ u.nickname?.[0] || u.username?.[0] }}</el-avatar>
                <span>{{ u.nickname || u.username }}</span>
                <el-tag size="small" :type="u.role === 'vip_author' ? 'warning' : 'info'">{{ u.role }}</el-tag>
                <span class="time">{{ formatDate(u.created_at) }}</span>
              </div>
            </div>
          </div>
        </el-col>
        <el-col :xs="24" :md="12">
          <h4 class="sec-title">📖 最新作品</h4>
          <div v-loading="loading">
            <el-empty v-if="!loading && data.recent_novels?.length === 0" description="暂无数据" />
            <div v-else class="community-list">
              <div v-for="n in data.recent_novels" :key="n.id" class="community-item" @click="$router.push('/novel/'+n.id)" style="cursor:pointer">
                <span style="font-weight:500">{{ n.title }}</span>
                <span class="time">{{ n.author?.nickname || '未知' }}</span>
              </div>
            </div>
          </div>
        </el-col>
      </el-row>

      <el-row :gutter="20" style="margin-top:16px">
        <el-col :xs="24" :md="12">
          <h4 class="sec-title">💬 最近评论</h4>
          <div v-loading="loading">
            <el-empty v-if="!loading && data.recent_comments?.length === 0" description="暂无数据" />
            <div v-else class="community-list">
              <div v-for="c in data.recent_comments" :key="c.id" class="community-item" style="flex-direction:column;align-items:flex-start;gap:2px">
                <span class="content">{{ truncate(c.content, 60) }}</span>
                <span class="time">{{ c.username }} · 《{{ c.novel_title }}》 · {{ formatDate(c.created_at) }}</span>
              </div>
            </div>
          </div>
        </el-col>
        <el-col :xs="24" :md="12">
          <h4 class="sec-title">📋 论坛新帖</h4>
          <div v-loading="loading">
            <el-empty v-if="!loading && data.recent_threads?.length === 0" description="暂无数据" />
            <div v-else class="community-list">
              <div v-for="t in data.recent_threads" :key="t.id" class="community-item" @click="$router.push('/thread/'+t.id)" style="cursor:pointer">
                <span style="font-weight:500;font-size:0.85rem">{{ truncate(t.title, 40) }}</span>
                <span class="time">@{{ t.username }}</span>
              </div>
            </div>
          </div>
        </el-col>
      </el-row>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { ArrowLeft } from '@element-plus/icons-vue';
import { adminApi } from '@/api/admin';

const data = reactive({ recent_users: [] as any[], recent_novels: [] as any[], recent_comments: [] as any[], recent_threads: [] as any[] });
const loading = ref(false);

async function load() {
  loading.value = true;
  try {
    const res = await adminApi.getCommunity();
    Object.assign(data, res.data.data || {});
  } catch { /* ignore */ }
  loading.value = false;
}

function formatDate(d?: string) { return d ? new Date(d).toLocaleDateString('zh-CN') : ''; }
function truncate(s: string, n: number) { return s?.length > n ? s.slice(0, n) + '…' : s; }

onMounted(load);
</script>

<style scoped>
.admin-page { min-height: 100vh; padding: 24px; background: var(--bg-page, #f5f7fa); }
.admin-page-header { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; }
.admin-page-header h2 { margin: 0; font-size: 1.3rem; color: var(--text-color, #333); }
.admin-page-body { max-width: 1100px; }
.sec-title { margin: 0 0 8px 0; font-size: 0.95rem; color: var(--text-color, #333); }
.community-list { display: flex; flex-direction: column; gap: 8px; }
.community-item { display: flex; align-items: center; gap: 10px; padding: 8px 12px; background: #fff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.06); }
.time { font-size: 0.75rem; color: #999; margin-left: auto; }
.content { font-size: 0.85rem; line-height: 1.3; }
[data-theme="dark"] .admin-page { background: #0f172a; }
[data-theme="dark"] .community-item { background: #1e293b; }
[data-theme="dark"] .admin-page-header h2, [data-theme="dark"] .sec-title { color: #e2e8f0; }
</style>
