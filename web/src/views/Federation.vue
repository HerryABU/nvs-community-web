<template>
  <div class="page-container federation-view">
    <!-- 未开启独立页：占位（站长可在「远程站点互通」开启「独立友链页」） -->
    <div v-if="!independentEnabled" class="fed-disabled">
      <span class="fed-disabled-icon">🌐</span>
      <h2>联邦独立页未开启</h2>
      <p>本站暂未对外开放互联站点目录。如需要，请站长在后台「远程站点互通」中开启「独立友链页」。</p>
    </div>

    <template v-else>
      <!-- 页头 -->
      <div class="fedv-header">
        <div class="fedv-title">
          <span class="fedv-icon">🌐</span>
          <div>
            <h1>联邦互联</h1>
            <p class="fedv-sub">NVS 站点互相连接，共享作品与社区 · 内容即文件 · 哈希可验证</p>
          </div>
        </div>
      </div>

      <!-- 站点目录（友链风格） -->
      <section class="fedv-section">
        <h2 class="fedv-h2"><span>互联站点目录</span><small>{{ sites.length }} 个已连接站点</small></h2>
        <div v-loading="loadingSites" class="fedv-sites">
          <div v-for="s in sites" :key="s.id" class="fedv-site" @click="openSite(s)">
            <div class="fedv-site-head">
              <span class="fedv-site-name">{{ s.name }}</span>
              <el-tag size="small" :type="s.status === 'active' ? 'success' : 'info'">
                {{ s.status === 'active' ? '已连接' : '停用' }}
              </el-tag>
            </div>
            <p class="fedv-site-desc">{{ s.description || '暂无简介' }}</p>
            <div class="fedv-site-foot">
              <span>📚 {{ s.novel_count || 0 }} 部作品</span>
              <span class="fedv-site-go">前往 →</span>
            </div>
          </div>
          <div v-if="!loadingSites && sites.length === 0" class="fedv-site-empty">
            还没有互联站点，请站长在后台「远程站点互通」中添加并同步。
          </div>
        </div>
      </section>

      <!-- 跨站作品流 -->
      <section class="fedv-section">
        <h2 class="fedv-h2"><span>跨站作品流</span><small>来自各互联站点的精选作品</small></h2>
        <div v-loading="loadingNovels" class="fedv-grid">
          <a
            v-for="item in novels"
            :key="item.site_id + '-' + item.remote_id"
            class="fedv-card"
            :href="item.source_url"
            target="_blank"
            rel="noopener noreferrer"
          >
            <div class="fedv-cover">
              <img v-if="item.cover_url" :src="item.cover_url" :alt="item.title" loading="lazy" />
              <span v-else class="fedv-cover-ph">📖</span>
              <span v-if="item.site" class="fedv-site-badge">{{ item.site.name }}</span>
            </div>
            <div class="fedv-card-body">
              <h4 class="fedv-card-title">{{ item.title }}</h4>
              <p class="fedv-card-author">{{ item.author || '佚名' }}<span v-if="item.category"> · {{ item.category }}</span></p>
              <p v-if="item.summary" class="fedv-card-summary">{{ item.summary }}</p>
            </div>
          </a>
        </div>

        <div v-if="!loadingNovels && novels.length === 0" class="fedv-empty">
          暂无跨站作品缓存。
        </div>

        <div v-if="hasMore" class="fedv-more">
          <el-button :loading="loadingMore" @click="loadMore">加载更多</el-button>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { storeToRefs } from 'pinia';
import { useAppStore } from '@/stores/app';
import { publicApi } from '@/api/admin';
import { appPrefix } from '@/utils/prefix';

const appStore = useAppStore();
const { federationModes, federatedSites } = storeToRefs(appStore);

const independentEnabled = computed(() => !!federationModes.value.independent_enabled);
const sites = computed(() => federatedSites.value || []);

const novels = ref<any[]>([]);
const loadingSites = ref(false);
const loadingNovels = ref(false);
const loadingMore = ref(false);
const page = ref(1);
const pageSize = 24;
const total = ref(0);

const hasMore = computed(() => novels.value.length < total.value);

function openSite(s: any) {
  // 通过本地代理路径访问外部站点内容（前缀挂载场景需带 /go-nvs）
  window.open(`${appPrefix}/${s.id}`, '_blank');
}

async function loadNovels(reset = true) {
  if (reset) {
    loadingNovels.value = true;
    page.value = 1;
  } else {
    loadingMore.value = true;
  }
  try {
    const res = await publicApi.getFederatedNovels({ page: page.value, page_size: pageSize });
    const list = res.data?.data?.list || [];
    const t = res.data?.data?.total || 0;
    total.value = t;
    if (reset) novels.value = list;
    else novels.value = novels.value.concat(list);
  } catch {
    if (reset) novels.value = [];
  } finally {
    loadingNovels.value = false;
    loadingMore.value = false;
  }
}

async function loadMore() {
  page.value += 1;
  await loadNovels(false);
}

onMounted(async () => {
  loadingSites.value = true;
  await appStore.bootstrap(); // 拉取联邦模式与站点列表（幂等）
  loadingSites.value = false;
});

// ⚠️ bootstrap 异步回填 federationModes：挂载瞬间 independent_enabled 可能仍是默认 false，
// 直接 if 判断会漏拉作品流且不再重跑。改为 watch（immediate 覆盖「已就绪」场景）。
watch(independentEnabled, (v) => {
  if (v) loadNovels(true);
}, { immediate: true });
</script>

<style scoped>
.federation-view { max-width: 1280px; }

.fed-disabled {
  max-width: 560px;
  margin: 80px auto;
  text-align: center;
  padding: 40px 24px;
  border: 1px dashed var(--border-color, #e5e7eb);
  border-radius: var(--radius-lg, 16px);
  background: var(--bg-card, #fff);
}
.fed-disabled-icon { font-size: 3rem; opacity: 0.6; }
.fed-disabled h2 { margin: 16px 0 8px; font-size: 1.3rem; color: var(--text-color, #1a1a2e); font-weight: 600; }
.fed-disabled p { color: var(--text-light, #888); font-size: 0.9rem; line-height: 1.7; }

.fedv-header { margin-bottom: 28px; }
.fedv-title { display: flex; align-items: center; gap: 16px; }
.fedv-icon { font-size: 2.4rem; line-height: 1; }
.fedv-title h1 { margin: 0 0 4px; font-size: 1.8rem; font-weight: 300; color: var(--text-color, #1a1a2e); letter-spacing: -0.02em; }
.fedv-sub { margin: 0; font-size: 0.9rem; color: var(--text-light, #888); }

.fedv-section { margin-bottom: 36px; }
.fedv-h2 {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin: 0 0 16px;
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-color, #1a1a2e);
}
.fedv-h2 small { font-size: 0.8rem; font-weight: 400; color: var(--text-light, #999); }

/* 站点目录（友链风格） */
.fedv-sites {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 16px;
}
.fedv-site {
  padding: 16px;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-md, 12px);
  background: var(--bg-card, #fff);
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}
.fedv-site:hover {
  transform: translateY(-3px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
  border-color: var(--primary-color, #6366f1);
}
.fedv-site-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 8px; }
.fedv-site-name { font-size: 1.05rem; font-weight: 600; color: var(--text-color, #1a1a2e); }
.fedv-site-desc {
  margin: 0 0 12px;
  font-size: 0.85rem;
  color: var(--text-light, #888);
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 2.7em;
}
.fedv-site-foot { display: flex; align-items: center; justify-content: space-between; font-size: 0.8rem; color: var(--text-light, #999); }
.fedv-site-go { color: var(--primary-color, #6366f1); font-weight: 500; }
.fedv-site-empty, .fedv-empty {
  padding: 20px;
  text-align: center;
  font-size: 0.88rem;
  color: var(--text-light, #999);
  border: 1px dashed var(--border-color, #e5e7eb);
  border-radius: var(--radius-md, 12px);
}

/* 跨站作品流 */
.fedv-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
  min-height: 80px;
}
.fedv-card {
  display: flex;
  flex-direction: column;
  border-radius: var(--radius-md, 12px);
  overflow: hidden;
  background: var(--bg-soft, #f7f8fa);
  border: 1px solid var(--border-color, #eee);
  text-decoration: none;
  color: inherit;
  transition: transform 0.18s ease, box-shadow 0.18s ease;
}
.fedv-card:hover { transform: translateY(-3px); box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1); }
.fedv-cover {
  position: relative;
  aspect-ratio: 3 / 4;
  background: linear-gradient(135deg, #e0e7ff, #f5f3ff);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.fedv-cover img { width: 100%; height: 100%; object-fit: cover; }
.fedv-cover-ph { font-size: 2.4rem; opacity: 0.5; }
.fedv-site-badge {
  position: absolute;
  left: 8px;
  top: 8px;
  font-size: 0.68rem;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(17, 24, 39, 0.78);
  color: #fff;
  backdrop-filter: blur(4px);
}
.fedv-card-body { padding: 10px 12px 12px; display: flex; flex-direction: column; gap: 4px; }
.fedv-card-title {
  margin: 0;
  font-size: 0.92rem;
  font-weight: 600;
  color: var(--text-color, #1a1a2e);
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.fedv-card-author { margin: 0; font-size: 0.76rem; color: var(--text-light, #888); }
.fedv-card-summary {
  margin: 0;
  font-size: 0.76rem;
  color: var(--text-light, #999);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.fedv-more { display: flex; justify-content: center; margin-top: 20px; }

[data-theme="dark"] .fedv-site, .fedv-card { background: #1e293b; border-color: #334155; }
[data-theme="dark"] .fedv-site-name, .fedv-card-title { color: #e2e8f0; }
[data-theme="dark"] .fedv-cover { background: linear-gradient(135deg, #1e293b, #312e81); }
[data-theme="dark"] .fed-disabled, .fedv-site-empty, .fedv-empty { background: #1e293b; border-color: #334155; }
</style>
