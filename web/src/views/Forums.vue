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

    <!-- 敏感分区论坛（带隔离墙确认） -->
    <h2 v-if="sensitiveForums.length > 0" class="section-title" style="margin-top:32px;color:#dc2626;">⚠ 敏感分区论坛</h2>
    <p v-if="sensitiveForums.length > 0" style="color:#999;font-size:0.85rem;margin-bottom:12px">以下论坛涉及敏感内容，点击后将触发确认弹窗</p>
    <div class="forum-grid" v-if="sensitiveForums.length > 0">
      <el-card v-for="forum in sensitiveForums" :key="forum.id"
        class="forum-card forum-card-sensitive" shadow="hover" @click="enterForum(forum)">
        <h3><span style="color:#dc2626;">⚠</span> {{ forum.name }}</h3>
        <p>{{ forum.description }}</p>
        <div class="forum-meta">
          <el-tag type="danger" size="small">敏感内容</el-tag>
          <span style="margin-left:8px">{{ forum.thread_count }} 个帖子</span>
        </div>
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
import { novelApi, forumApi } from '@/api/novel';
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
const sensitiveForums = computed(() => allForums.value.filter(f => f.type === 'sensitive'));

// 敏感分区确认
const showZoneGuard = ref(false);
const zoneGuardName = ref('');
const zoneGuardCross = ref(false);
let pendingForumId: number | null = null;

async function enterForum(forum: Forum) {
  // type===sensitive 直接触发隔离，否则走系统分区匹配
  if (forum.type === 'sensitive') {
    zoneGuardName.value = (forum as any).zone || forum.name;
    zoneGuardCross.value = false;
    pendingForumId = forum.id;
    showZoneGuard.value = true;
    return;
  }
  const guard = await shouldShowGuard((forum as any).zone || forum.name);
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
    const res = await forumApi.getForums('all');
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

.forum-card-sensitive {
  border-color: rgba(220, 38, 38, 0.3);
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
  display: flex;
  align-items: center;
}
</style>

<style>
[data-theme="dark"] .forum-card {
  background: #1e293b;
  border-color: rgba(255,255,255,.08);
}
[data-theme="dark"] .forum-card-sensitive {
  border-color: rgba(220, 38, 38, 0.4);
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
