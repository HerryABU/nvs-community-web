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

    <!-- 我的服务：原先散落在导航栏下拉里的入口，统一收到个人中心（下拉只剩 5 项） -->
    <h2 class="me-section-title">我的服务</h2>
    <div class="me-services">
      <div v-for="g in serviceGroups" :key="g.title" class="me-svc-group">
        <div class="me-svc-head">
          <span class="me-svc-emoji">{{ g.icon }}</span>{{ g.title }}
        </div>
        <router-link
          v-for="it in g.items"
          :key="it.to"
          :to="it.to"
          class="me-svc-item"
        >
          <span class="me-svc-label">{{ it.label }}</span>
          <span class="me-svc-desc">{{ it.desc }}</span>
          <span class="me-svc-arrow">→</span>
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, ref, onMounted, type Ref } from 'vue';
import { homeCustomizeApi } from '@/api/homeCustomize';
import { useAuthStore } from '@/stores/auth';

const profile = inject<Ref<any>>('meProfile');
const authStore = useAuthStore();

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

// 我的服务：把导航栏下拉里原来的 14 个入口按语义分组搬进来。
// 「我的主页」是公开页（/author/<我的id>），必须带 id，所以整体做成 computed。
const serviceGroups = computed(() => [
  {
    icon: '🪪',
    title: '账号与资料',
    items: [
      { label: '头像与资料', desc: '换头像 / 昵称 / 签名 / 简介', to: '/me/decorate' },
      { label: '账号设置', desc: '密码、绑定、隐私偏好', to: '/settings' },
      { label: '我的主页', desc: '别人看到的公开主页', to: `/author/${authStore.user?.id ?? ''}` },
    ],
  },
  {
    icon: '💬',
    title: '互动与关注',
    items: [
      { label: '我的私信', desc: '一对一会话', to: '/user/messages' },
      { label: '特别关注', desc: '重点盯的作者', to: '/user/special-follows' },
      { label: '特别关注动态', desc: '他们更新了什么', to: '/feed' },
      { label: '黑名单管理', desc: '屏蔽/隐藏名单', to: '/user/blacklist' },
      { label: '我的举报', desc: '我提交的举报进度', to: '/reports' },
    ],
  },
  {
    icon: '💰',
    title: '资产与开放能力',
    items: [
      { label: '我的钱包', desc: '余额、充值、消费记录', to: '/wallet' },
      { label: '我的 API Key', desc: '签发与吊销', to: '/my-keys' },
      { label: 'API Key 爬虫指南', desc: '接入示例与限制', to: '/api-spider' },
      { label: 'AI 工作台', desc: '对话 / 绘画 / 视频', to: '/ai' },
    ],
  },
  {
    icon: '🎨',
    title: '外观与帮助',
    items: [
      { label: '模板设置', desc: '作者主页 UI 模板', to: '/author/templates' },
      { label: '格式使用文档', desc: 'Markdown / 话本体语法', to: '/docs' },
      { label: '我的精读', desc: '书签与划线批注', to: '/reading' },
    ],
  },
]);

onMounted(loadMyCustom);
</script>

<style scoped>
.me-rating { margin-bottom: 16px; }
.me-cards { margin-top: 4px; }
.me-card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 20px;
  text-align: center;
  cursor: pointer;
  transition: box-shadow 0.2s, transform 0.2s;
  margin-bottom: 16px;
}
.me-card:hover { box-shadow: 0 4px 14px rgba(0, 0, 0, 0.1); transform: translateY(-2px); }
.me-card-icon { font-size: 1.8rem; margin-bottom: 8px; }
.me-card-title { font-weight: 600; font-size: 1rem; color: var(--text-color); }
.me-card-sub { font-size: 0.8rem; color: var(--text-muted); margin-top: 4px; }

/* ===== 我的服务（从导航栏下拉搬进来的入口） ===== */
.me-section-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-color);
  margin: 8px 0 12px;
}
.me-services {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}
.me-svc-group {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 14px 12px 8px;
}
.me-svc-head {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--text-color);
  padding-bottom: 8px;
  margin-bottom: 4px;
  border-bottom: 1px solid var(--border-color);
}
.me-svc-emoji { font-size: 1rem; }
.me-svc-item {
  display: grid;
  grid-template-columns: 1fr auto;
  grid-template-areas: 'label arrow' 'desc arrow';
  align-items: center;
  gap: 0 8px;
  padding: 8px 6px;
  border-radius: 8px;
  text-decoration: none;
  color: inherit;
  transition: background 0.15s;
}
.me-svc-item:hover { background: var(--bg-soft, #f6f8fb); }
.me-svc-label {
  grid-area: label;
  font-size: 0.86rem;
  color: var(--text-color);
}
.me-svc-desc {
  grid-area: desc;
  font-size: 0.72rem;
  color: var(--text-light);
  line-height: 1.5;
}
.me-svc-arrow {
  grid-area: arrow;
  color: var(--text-light);
  font-size: 0.85rem;
  opacity: 0;
  transition: opacity 0.15s, transform 0.15s;
}
.me-svc-item:hover .me-svc-arrow { opacity: 1; transform: translateX(2px); }

@media (max-width: 1100px) {
  .me-services { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 640px) {
  .me-services { grid-template-columns: 1fr; }
}
</style>
