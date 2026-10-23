<template>
  <!-- 灵感广场：公开作品流（图像 / AI 视频），支持风格筛选、点赞、一键复刻 -->
  <div class="ai-gallery">
    <div class="gal-top">
      <el-radio-group v-model="kind" size="small" @change="reload">
        <el-radio-button value="image">图像</el-radio-button>
        <el-radio-button value="video">AI 视频</el-radio-button>
      </el-radio-group>
      <div class="gal-styles">
        <span :class="['gal-chip', { active: !style }]" @click="pickStyle('')">全部</span>
        <span
          v-for="s in styles"
          :key="s"
          :class="['gal-chip', { active: style === s }]"
          @click="pickStyle(s)"
        >{{ s }}</span>
      </div>
      <el-button size="small" text :loading="loading" @click="reload">刷新</el-button>
    </div>

    <div v-loading="loading" class="gal-body">
      <el-empty v-if="!list.length && !loading" description="还没有公开作品，去绘画页发布第一张吧" :image-size="70" />
      <div class="gal-grid">
        <div v-for="it in list" :key="it.id" class="gal-card">
          <div class="gal-media">
            <video v-if="it.kind === 'video' && it.video" :src="it.video" controls preload="metadata" />
            <img v-else-if="it.images.length" :src="it.images[0]" :alt="it.title" />
            <div v-else class="gal-broken">无预览</div>
            <span class="gal-model">{{ it.model_name }}</span>
            <span v-if="it.kind === 'video'" class="gal-kind">视频</span>
          </div>
          <div class="gal-meta">
            <div class="gal-title">{{ it.title || '未命名作品' }}</div>
            <div class="gal-prompt" :title="it.prompt">{{ it.prompt }}</div>
            <div class="gal-actions">
              <button type="button" class="gal-btn" @click="remix(it)">
                <el-icon><MagicStick /></el-icon> 复刻同款
              </button>
              <button type="button" class="gal-btn gal-like" @click="like(it)">
                ♥ {{ it.likes }}
              </button>
            </div>
          </div>
        </div>
      </div>
      <div v-if="total > list.length" class="gal-more">
        <el-button size="small" :loading="loading" @click="loadMore">加载更多（{{ list.length }}/{{ total }}）</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { MagicStick } from '@element-plus/icons-vue';
import { aiHubApi } from '@/api/aiHub';

const emit = defineEmits<{ (e: 'remix', payload: { kind: string; prompt: string }): void }>();

interface GalItem {
  id: number;
  kind: 'image' | 'video';
  title: string;
  prompt: string;
  model_name: string;
  style: string;
  likes: number;
  images: string[];
  video: string;
}

const kind = ref<'image' | 'video'>('image');
const style = ref('');
const styles = ref<string[]>([]);
const list = ref<GalItem[]>([]);
const total = ref(0);
const page = ref(1);
const loading = ref(false);
const liked = new Set<number>();

async function fetchPage(p: number) {
  loading.value = true;
  try {
    const res = await aiHubApi.gallery({ kind: kind.value, style: style.value || undefined, page: p });
    if (res.data.code === 0) {
      const d = res.data.data;
      total.value = d.total;
      styles.value = d.styles || [];
      list.value = p === 1 ? d.list : [...list.value, ...d.list];
    }
  } catch { /* ignore */ }
  loading.value = false;
}

function reload() {
  page.value = 1;
  fetchPage(1);
}

function loadMore() {
  page.value += 1;
  fetchPage(page.value);
}

function pickStyle(s: string) {
  style.value = s;
  reload();
}

function remix(it: GalItem) {
  emit('remix', { kind: it.kind, prompt: it.prompt });
}

async function like(it: GalItem) {
  if (liked.has(it.id)) {
    ElMessage.info('已点过赞');
    return;
  }
  try {
    const res = await aiHubApi.likeMedia(it.id);
    if (res.data.code === 0) {
      it.likes += 1;
      liked.add(it.id);
    }
  } catch (e: any) {
    ElMessage.warning(e?.response?.data?.message || '点赞失败');
  }
}

onMounted(reload);
</script>

<style scoped>
.ai-gallery { padding: 16px; display: flex; flex-direction: column; min-height: 0; height: 100%; }
.gal-top { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; }
.gal-styles { display: flex; gap: 6px; flex-wrap: wrap; flex: 1; }
.gal-chip {
  font-size: 0.74rem; padding: 2px 10px; border-radius: 999px; cursor: pointer;
  background: var(--el-fill-color-light, #f5f7fa); color: var(--text-light, #666);
}
.gal-chip.active { background: var(--el-color-primary, #409eff); color: #fff; }
.gal-body { flex: 1; overflow-y: auto; min-height: 0; }
.gal-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); gap: 14px; }
.gal-card {
  border: 1px solid var(--el-border-color-lighter, #ebeef5); border-radius: 12px; overflow: hidden;
  background: var(--el-bg-color, #fff); display: flex; flex-direction: column;
}
.gal-media { position: relative; aspect-ratio: 1; background: var(--el-fill-color-light, #f5f7fa); }
.gal-media img, .gal-media video { width: 100%; height: 100%; object-fit: cover; display: block; }
.gal-broken { display: flex; align-items: center; justify-content: center; height: 100%; color: #999; font-size: 0.8rem; }
.gal-model {
  position: absolute; top: 8px; left: 8px; padding: 1px 8px; border-radius: 999px;
  font-size: 0.64rem; color: #fff; background: rgba(0, 0, 0, 0.55); max-width: 70%;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.gal-kind {
  position: absolute; top: 8px; right: 8px; padding: 1px 8px; border-radius: 999px;
  font-size: 0.64rem; color: #fff; background: rgba(64, 158, 255, 0.85);
}
.gal-meta { padding: 10px 12px 12px; display: flex; flex-direction: column; gap: 6px; }
.gal-title { font-size: 0.84rem; font-weight: 600; color: var(--text-color, #333); }
.gal-prompt {
  font-size: 0.72rem; color: var(--text-light, #888); line-height: 1.5;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.gal-actions { display: flex; align-items: center; gap: 8px; margin-top: 2px; }
.gal-btn {
  display: inline-flex; align-items: center; gap: 3px; cursor: pointer;
  border: 1px solid var(--el-border-color, #e5e7eb); background: transparent;
  border-radius: 999px; padding: 2px 10px; font-size: 0.72rem; color: var(--el-color-primary, #409eff);
}
.gal-like { color: #ef4444; border-color: rgba(239, 68, 68, 0.3); margin-left: auto; }
.gal-more { text-align: center; padding: 16px 0; }
</style>
