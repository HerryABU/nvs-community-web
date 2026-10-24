<template>
  <div class="live-page">
    <div class="live-head">
      <h2>{{ data?.title || '直播间' }}</h2>
      <el-tag v-if="data?.status" size="small" :type="statusType">{{ statusLabel }}</el-tag>
    </div>

    <el-skeleton v-if="loading" :rows="5" animated />

    <template v-else-if="data?.enabled">
      <!-- 需要登录 -->
      <el-empty v-if="data.need_login" description="本直播间需要登录后观看">
        <el-button type="primary" @click="$router.push('/login')">去登录</el-button>
      </el-empty>

      <template v-else>
        <!-- 直播中：优先 iframe（外部直播间），其次原生 video（直链） -->
        <div v-if="data.status === 'live'" class="live-player">
          <iframe
            v-if="data.embed_url"
            :src="data.embed_url"
            class="live-frame"
            allow="autoplay; fullscreen; picture-in-picture; encrypted-media"
            sandbox="allow-scripts allow-same-origin allow-presentation allow-forms"
            referrerpolicy="no-referrer-when-downgrade"
            allowfullscreen
          />
          <video v-else-if="data.video_url" :src="data.video_url" class="live-video" controls autoplay playsinline />
          <div v-else class="live-empty">已标记为直播中，但管理员还没填播放地址</div>
        </div>

        <!-- 未开播 / 预告：海报 + 开播时间 -->
        <div v-else class="live-offline">
          <img v-if="data.poster_url" :src="data.poster_url" class="live-poster" alt="直播海报" />
          <div class="live-offline-text">
            <div class="live-offline-title">{{ data.status === 'preview' ? '即将开播' : '当前未开播' }}</div>
            <div v-if="data.starts_at" class="live-countdown">
              开播时间：{{ fmt(data.starts_at) }}
              <span v-if="countdown" class="live-cd">（{{ countdown }}）</span>
            </div>
            <div v-if="data.video_url" class="live-replay">
              <el-button size="small" @click="playReplay">播放录像</el-button>
            </div>
          </div>
          <video v-if="replayOn" :src="data.video_url" class="live-video" controls autoplay playsinline />
        </div>

        <div v-if="data.notice" class="live-notice">{{ data.notice }}</div>

        <div class="live-actions">
          <el-button v-if="data.chat_group_id" size="small" @click="$router.push(`/group/${data.chat_group_id}`)">
            进入直播聊天群
          </el-button>
          <el-button size="small" text @click="reload">刷新状态</el-button>
        </div>
      </template>
    </template>

    <el-empty v-else description="直播未开启">
      <el-button size="small" @click="$router.push('/')">回首页</el-button>
    </el-empty>

    <p class="live-hint">
      说明：本站只提供**入口与嵌入**，不做推流/转码/HLS。推流由 OBS 等推给流媒体服务（SRS/云直播），
      再把播放地址或直播间页填到后台「运营位 → 直播」即可。
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import api from '@/api/index';

interface LiveData {
  enabled: boolean;
  title?: string;
  status?: string;
  notice?: string;
  starts_at?: string | null;
  embed_url?: string;
  video_url?: string;
  poster_url?: string;
  chat_group_id?: number;
  need_login?: boolean;
}

const data = ref<LiveData | null>(null);
const loading = ref(true);
const replayOn = ref(false);
const now = ref(Date.now());
let timer: number | null = null;

const statusLabel = computed(() => {
  const s = data.value?.status;
  return s === 'live' ? '直播中' : s === 'preview' ? '预告' : '未开播';
});
const statusType = computed(() => {
  const s = data.value?.status;
  return s === 'live' ? 'danger' : s === 'preview' ? 'warning' : 'info';
});

// 倒计时（只算到分钟，避免每秒重渲染整页）
const countdown = computed(() => {
  const t = data.value?.starts_at;
  if (!t) return '';
  const diff = new Date(t).getTime() - now.value;
  if (diff <= 0) return '已到开播时间';
  const m = Math.floor(diff / 60000);
  if (m < 60) return `${m} 分钟后`;
  const h = Math.floor(m / 60);
  return `${h} 小时 ${m % 60} 分钟后`;
});

function fmt(s?: string | null) {
  return s ? new Date(s).toLocaleString('zh-CN') : '';
}
function playReplay() {
  replayOn.value = true;
}

async function reload() {
  loading.value = true;
  try {
    const res = await api.get<{ code: number; data: LiveData }>('/live');
    data.value = res.data.data;
  } catch {
    data.value = { enabled: false };
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  reload();
  timer = window.setInterval(() => { now.value = Date.now(); }, 60000);
});
onUnmounted(() => {
  if (timer !== null) window.clearInterval(timer);
});
</script>

<style scoped>
.live-page { max-width: 1000px; margin: 0 auto; padding: 20px 16px 48px; }
.live-head { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.live-head h2 { margin: 0; }
.live-player { position: relative; background: #000; border-radius: 12px; overflow: hidden; }
.live-frame { width: 100%; aspect-ratio: 16 / 9; border: 0; display: block; }
.live-video { width: 100%; border-radius: 12px; display: block; background: #000; }
.live-empty { padding: 40px; color: #bbb; text-align: center; }
.live-offline {
  border: 1px solid var(--border-color); border-radius: 12px; padding: 16px;
  background: var(--bg-card); display: flex; flex-direction: column; gap: 12px;
}
.live-poster { width: 100%; max-height: 420px; object-fit: cover; border-radius: 10px; }
.live-offline-title { font-size: 1.05rem; font-weight: 700; }
.live-countdown { color: var(--text-muted); font-size: 0.86rem; margin-top: 4px; }
.live-cd { color: var(--el-color-primary); }
.live-replay { margin-top: 8px; }
.live-notice {
  margin-top: 12px; padding: 10px 12px; border-radius: 8px;
  background: var(--bg-muted); font-size: 0.86rem; white-space: pre-wrap;
}
.live-actions { margin-top: 12px; display: flex; gap: 8px; }
.live-hint { margin-top: 18px; font-size: 0.76rem; color: var(--text-muted); line-height: 1.6; }
</style>
