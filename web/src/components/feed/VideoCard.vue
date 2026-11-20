<template>
  <article class="vcard" @click="open">
    <div class="vcard-cover" :class="{ empty: !post.cover_url }">
      <img v-if="post.cover_url" :src="mediaUrl(post.cover_url)" :alt="post.title || '封面'" loading="lazy" />
      <span v-else class="vcard-ph">🎬</span>
      <span v-if="durationText" class="vcard-dur">{{ durationText }}</span>
      <span class="vcard-views">▶ {{ formatCount(post.view_count) }}</span>
    </div>
    <div class="vcard-body">
      <h3 class="vcard-title">{{ post.title || truncate(post.content) || '未命名视频' }}</h3>
      <div class="vcard-meta">
        <el-avatar :size="20" :src="mediaUrl(post.avatar_url)">{{ (post.author_name || '?')[0] }}</el-avatar>
        <span class="vcard-author">{{ post.author_name || '用户' + post.user_id }}</span>
        <span class="vcard-dot">·</span>
        <span>{{ formatWhen(post.created_at) }}</span>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';
import type { FeedPost } from '@/api/feed';
import { mediaUrl } from '@/utils/prefix';
import { formatCount, formatDuration, formatWhen } from '@/utils/feedFormat';

const props = defineProps<{ post: FeedPost }>();
const router = useRouter();

const durationText = computed(() => formatDuration(props.post.duration));
function truncate(s?: string) {
  const t = (s || '').replace(/[#*`>\[\]]/g, '').trim();
  return t ? t.slice(0, 36) : '';
}
function open() {
  router.push(`/video/${props.post.id}`);
}
</script>

<style scoped>
.vcard { cursor: pointer; display: flex; flex-direction: column; }
.vcard-cover {
  position: relative; border-radius: 10px; overflow: hidden; aspect-ratio: 16 / 9;
  background: #e5e7eb;
}
.vcard-cover img { width: 100%; height: 100%; object-fit: cover; display: block; transition: transform 0.25s; }
.vcard:hover .vcard-cover img { transform: scale(1.04); }
.vcard-cover.empty { display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #1e2a5a, #2b3a67); }
.vcard-ph { font-size: 2rem; opacity: 0.7; }
.vcard-dur {
  position: absolute; right: 6px; bottom: 6px; background: rgba(0, 0, 0, 0.7); color: #fff;
  font-size: 0.72rem; padding: 1px 6px; border-radius: 4px; letter-spacing: 0.3px;
}
.vcard-views {
  position: absolute; left: 6px; bottom: 6px; color: #fff; font-size: 0.72rem;
  background: rgba(0, 0, 0, 0.55); padding: 1px 6px; border-radius: 4px;
}
.vcard-body { padding: 8px 2px 0; }
.vcard-title {
  margin: 0 0 6px; font-size: 0.92rem; font-weight: 500; line-height: 1.45; color: var(--text-color);
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.vcard-meta { display: flex; align-items: center; gap: 5px; font-size: 0.76rem; color: var(--text-muted, #6b7280); }
.vcard-author { max-width: 110px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.vcard-dot { opacity: 0.6; }
</style>
