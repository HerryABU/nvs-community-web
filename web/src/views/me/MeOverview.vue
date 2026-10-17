<template>
  <div class="me-overview">
    <!-- 我的主页装扮（仅本人可见，原始 HTML，等同 alist 个人自定义） -->
    <div
      v-if="myRawHtml"
      class="me-custom-html"
      v-html="myRawHtml"
    />

    <!-- 评分 -->
    <div class="me-rating" v-if="avgRating > 0">
      <el-rate v-model="avgRating" disabled show-score text-color="#f59e0b" />
    </div>

    <!-- 功能入口卡片（点击进入对应 /me/功能类型 子页） -->
    <el-row :gutter="16" class="me-cards">
      <el-col :xs="12" :sm="12" :md="6">
        <div class="me-card" @click="$router.push('/me/works')">
          <div class="me-card-icon">📚</div>
          <div class="me-card-title">作品</div>
          <div class="me-card-sub">{{ profile?.total_novels || 0 }} 部</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <div class="me-card" @click="$router.push('/me/blogs')">
          <div class="me-card-icon">✍️</div>
          <div class="me-card-title">博客</div>
          <div class="me-card-sub">查看我的文章</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <div class="me-card" @click="$router.push('/me/comments')">
          <div class="me-card-icon">💬</div>
          <div class="me-card-title">留言</div>
          <div class="me-card-sub">{{ (profile?.total_comments || 0).toLocaleString() }} 条</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <div class="me-card" @click="$router.push('/me/stats')">
          <div class="me-card-icon">📈</div>
          <div class="me-card-title">数据</div>
          <div class="me-card-sub">创作趋势</div>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, ref, onMounted, type Ref } from 'vue';
import { homeCustomizeApi } from '@/api/homeCustomize';

const profile = inject<Ref<any>>('meProfile');

const myRawHtml = ref('');

const avgRating = computed(() => {
  const v = profile?.value?.avg_rating;
  if (!v || v <= 0) return 0;
  return Math.min(5, Math.round(v * 2) / 2);
});

// 拉取本人的主页装扮原始 HTML（仅本人可见），非空才渲染
async function loadMyCustom() {
  try {
    const res = await homeCustomizeApi.getMyHomeCustomize();
    if (res.data.code === 0) myRawHtml.value = res.data.data.my_raw_html || '';
  } catch { /* ignore */ }
}

onMounted(loadMyCustom);
</script>

<style scoped>
.me-rating { margin-bottom: 16px; }
.me-cards { margin-top: 4px; }
.me-card {
  background: #fff;
  border: 1px solid #eee;
  border-radius: 12px;
  padding: 20px;
  text-align: center;
  cursor: pointer;
  transition: box-shadow 0.2s, transform 0.2s;
  margin-bottom: 16px;
}
.me-card:hover { box-shadow: 0 4px 14px rgba(0, 0, 0, 0.1); transform: translateY(-2px); }
[data-theme="dark"] .me-card { background: #1e293b; border-color: #334155; }
.me-card-icon { font-size: 1.8rem; margin-bottom: 8px; }
.me-card-title { font-weight: 600; font-size: 1rem; color: var(--text-color); }
.me-card-sub { font-size: 0.8rem; color: var(--text-muted); margin-top: 4px; }
</style>
