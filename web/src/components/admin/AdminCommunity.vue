<template>
  <el-collapse-item title="社区动态" name="community">
    <div class="panel-body">
      <el-row :gutter="20">
        <!-- 最近注册用户 -->
        <el-col :xs="24" :md="12">
          <h4 style="margin:0 0 8px 0;font-size:0.95rem">👤 最近注册用户</h4>
          <div v-loading="loading">
            <el-empty v-if="!loading && data.recent_users?.length === 0" description="暂无数据" />
            <div v-else class="community-list">
              <div v-for="u in data.recent_users" :key="u.id" class="community-item">
                <el-avatar :size="32" :src="u.avatar_url">{{ u.nickname?.[0] || u.username?.[0] }}</el-avatar>
                <span>{{ u.nickname || u.username }}</span>
                <el-tag size="small" :type="u.role === 'vip_author' ? 'warning' : 'info'">{{ u.role }}</el-tag>
                <span style="font-size:0.75rem;color:#999;margin-left:auto">{{ formatDate(u.created_at) }}</span>
              </div>
            </div>
          </div>
        </el-col>

        <!-- 最新作品 -->
        <el-col :xs="24" :md="12">
          <h4 style="margin:0 0 8px 0;font-size:0.95rem">📖 最新作品</h4>
          <div v-loading="loading">
            <el-empty v-if="!loading && data.recent_novels?.length === 0" description="暂无数据" />
            <div v-else class="community-list">
              <div v-for="n in data.recent_novels" :key="n.id" class="community-item" @click="$router.push('/novel/'+n.id)" style="cursor:pointer">
                <span style="font-weight:500">{{ n.title }}</span>
                <span style="font-size:0.8rem;color:#999;margin-left:auto">{{ n.author?.nickname || '未知' }}</span>
              </div>
            </div>
          </div>
        </el-col>
      </el-row>

      <el-row :gutter="20" style="margin-top:16px">
        <!-- 最近评论 -->
        <el-col :xs="24" :md="12">
          <h4 style="margin:0 0 8px 0;font-size:0.95rem">💬 最近评论</h4>
          <div v-loading="loading">
            <el-empty v-if="!loading && data.recent_comments?.length === 0" description="暂无数据" />
            <div v-else class="community-list">
              <div v-for="c in data.recent_comments" :key="c.id" class="community-item" style="flex-direction:column;align-items:flex-start;gap:2px">
                <span style="font-size:0.85rem;line-height:1.3">{{ truncate(c.content, 60) }}</span>
                <span style="font-size:0.75rem;color:#999">
                  {{ c.username }} · 《{{ c.novel_title }}》 · {{ formatDate(c.created_at) }}
                </span>
              </div>
            </div>
          </div>
        </el-col>

        <!-- 最近帖子 -->
        <el-col :xs="24" :md="12">
          <h4 style="margin:0 0 8px 0;font-size:0.95rem">📋 论坛新帖</h4>
          <div v-loading="loading">
            <el-empty v-if="!loading && data.recent_threads?.length === 0" description="暂无数据" />
            <div v-else class="community-list">
              <div v-for="t in data.recent_threads" :key="t.id" class="community-item" @click="$router.push('/thread/'+t.id)" style="cursor:pointer">
                <span style="font-weight:500;font-size:0.85rem">{{ truncate(t.title, 40) }}</span>
                <span style="font-size:0.75rem;color:#999;margin-left:auto">@{{ t.username }}</span>
              </div>
            </div>
          </div>
        </el-col>
      </el-row>
    </div>
  </el-collapse-item>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
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

onMounted(() => { load(); });
</script>
