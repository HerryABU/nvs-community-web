<template>
  <div class="page-container">
    <h1 class="page-title">社区论坛</h1>

    <!-- 作者工坊 -->
    <h2 class="section-title">作者工坊</h2>
    <div class="forum-grid" v-if="authorForums.length > 0">
      <el-card v-for="forum in authorForums" :key="forum.id" class="forum-card" shadow="hover" @click="enterForum(forum)">
        <h3>{{ forum.name }}</h3>
        <p>{{ forum.description }}</p>
        <div class="forum-meta"><span>{{ forum.thread_count }} 个帖子</span></div>
      </el-card>
    </div>
    <el-empty v-else description="暂无忧" :image-size="40" />

    <!-- 读者·作者互动 -->
    <h2 class="section-title" style="margin-top:32px">读者·作者互动</h2>
    <div class="forum-grid" v-if="raForums.length > 0">
      <el-card v-for="forum in raForums" :key="forum.id" class="forum-card" shadow="hover" @click="enterForum(forum)">
        <h3>{{ forum.name }}</h3>
        <p>{{ forum.description }}</p>
        <div class="forum-meta"><span>{{ forum.thread_count }} 个帖子</span></div>
      </el-card>
    </div>
    <el-empty v-else description="暂无" :image-size="40" />

    <!-- 读者交流广场 -->
    <h2 class="section-title" style="margin-top:32px">读者交流广场</h2>
    <div class="forum-grid" v-if="readerForums.length > 0">
      <el-card v-for="forum in readerForums" :key="forum.id" class="forum-card" shadow="hover" @click="enterForum(forum)">
        <h3>{{ forum.name }}</h3>
        <p>{{ forum.description }}</p>
        <div class="forum-meta"><span>{{ forum.thread_count }} 个帖子</span></div>
      </el-card>
    </div>
    <el-empty v-else description="暂无" :image-size="40" />

    <!-- 公共广场 -->
    <h2 class="section-title" style="margin-top:32px">公共广场</h2>
    <div class="forum-grid" v-if="generalForums.length > 0">
      <el-card v-for="forum in generalForums" :key="forum.id" class="forum-card" shadow="hover" @click="enterForum(forum)">
        <h3>{{ forum.name }}</h3>
        <p>{{ forum.description }}</p>
        <div class="forum-meta"><span>{{ forum.thread_count }} 个帖子</span></div>
      </el-card>
    </div>
    <el-empty v-if="!loading && allForums.length === 0" description="暂无论坛" />

    <!-- 敏感分区确认弹窗 -->
    <SensitiveZoneGuard
      :visible="showZoneGuard"
      :zone-name="zoneGuardName"
      :is-cross-domain="zoneGuardCross"
      @confirm="onZoneConfirmed"
      @cancel="showZoneGuard = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { novelApi } from '@/api/novel';
import SensitiveZoneGuard from '@/components/SensitiveZoneGuard.vue';
import { shouldShowGuard, markZoneConfirmed, setLastZone } from '@/utils/sensitiveZone';

interface Forum {
  id: number;
  name: string;
  description: string;
  type: string;
  thread_count: number;
}

const router = useRouter();
const allForums = ref<Forum[]>([]);
const loading = ref(false);

const authorForums = computed(() => allForums.value.filter(f => f.type === 'author' || f.type === 'author_sub'));
const raForums = computed(() => allForums.value.filter(f => f.type === 'reader_author'));
const readerForums = computed(() => allForums.value.filter(f => f.type === 'reader'));
const generalForums = computed(() => allForums.value.filter(f => f.type === 'general'));

// 敏感分区确认
const showZoneGuard = ref(false);
const zoneGuardName = ref('');
const zoneGuardCross = ref(false);
let pendingForumId: number | null = null;

function enterForum(forum: Forum) {
  const guard = shouldShowGuard(forum.name);
  if (guard?.needed) {
    zoneGuardName.value = forum.name;
    zoneGuardCross.value = guard.isCrossDomain;
    pendingForumId = forum.id;
    showZoneGuard.value = true;
    return;
  }
  router.push(`/forum/${forum.id}`);
}

function onZoneConfirmed() {
  markZoneConfirmed(zoneGuardName.value);
  setLastZone(zoneGuardName.value);
  showZoneGuard.value = false;
  if (pendingForumId !== null) {
    router.push(`/forum/${pendingForumId}`);
    pendingForumId = null;
  }
}

onMounted(async () => {
  loading.value = true;
  try {
    const res = await novelApi.getForums('all');
    allForums.value = res.data.data || [];
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.forum-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.forum-card {
  cursor: pointer;
}

.forum-card h3 {
  font-size: 1.1rem;
  color: var(--primary-color);
  margin-bottom: 8px;
}

.forum-card p {
  color: #666;
  font-size: 0.9rem;
  margin-bottom: 12px;
}

.forum-meta {
  font-size: 0.8rem;
  color: var(--text-light);
}
</style>

<style>
[data-theme="dark"] .forum-card {
  background: #1e293b;
  border-color: rgba(255,255,255,.08);
}
[data-theme="dark"] .forum-card h3 {
  color: #e2e8f0;
}
[data-theme="dark"] .forum-card p {
  color: #94a3b8;
}
[data-theme="dark"] .forum-meta {
  color: #94a3b8;
}
</style>
