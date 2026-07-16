<template>
  <div class="page-container">
    <div class="page-header">
      <h2>🏪 HTML 应用广场</h2>
      <p class="subtitle">浏览社区公开的扩展HTML项目 · 作者专区 · 在线运行</p>
    </div>
    <div v-if="items.length===0&&!loading" class="empty"><el-empty description="还没有公开项目" /></div>
    <div v-else class="plaza-grid">
      <div v-for="h in items" :key="h.id" class="plaza-card" @click="openRunner(h)">
        <div class="card-thumb">
          <img v-if="h.thumb_url" :src="h.thumb_url" class="thumb-img" />
          <div v-else class="thumb-placeholder">🔌</div>
        </div>
        <div class="card-info">
          <div class="card-title">{{ h.name }}</div>
          <div class="card-author">by {{ h.user?.nickname || h.user?.username || '未知' }}</div>
          <div class="card-meta">
            <span>{{ h.file_count }} 文件</span>
            <span>{{ fmtSize(h.total_size) }}</span>
            <el-tag v-if="h.allow_wasm" type="warning" size="small">WASM</el-tag>
          </div>
          <div class="card-actions" @click.stop>
            <el-button size="small" type="primary" @click="openRunner(h)">▶ 运行</el-button>
            <el-button size="small" @click="openPreview(h)">👁 预览</el-button>
            <el-button v-if="h.is_downloadable" size="small" @click="download(h)">📥 下载</el-button>
          </div>
        </div>
      </div>
    </div>
    <div v-if="total>size" style="text-align:center;margin-top:20px">
      <el-pagination layout="prev,pager,next" :total="total" :page-size="size" v-model:current-page="page" @change="load" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import api from '@/api/index';

const router = useRouter();
const items = ref<any[]>([]);
const loading = ref(false);
const total = ref(0);
const page = ref(1);
const size = 20;

onMounted(() => load());

async function load() {
  loading.value = true;
  try {
    const r = await api.get('/htmls/public', { params: { page: page.value, size } });
    if (r.data.code === 0) { items.value = r.data.data.items; total.value = r.data.data.total; }
  } catch { /* */ }
  loading.value = false;
}

function openRunner(h: any) { window.open(`/app/${h.id}`, '_blank'); }
function openPreview(h: any) { window.open(`/api/userhtmls/${h.id}/preview`, '_blank'); }
function download(h: any) { window.open(`/api/userhtmls/${h.id}/download`, '_blank'); }
function fmtSize(b: number) { if (b<1024) return b+'B'; if (b<1048576) return (b/1024).toFixed(1)+'KB'; return (b/1048576).toFixed(1)+'MB'; }
</script>

<style scoped>
.page-container{max-width:1100px;margin:0 auto;padding:24px 20px}
.page-header{margin-bottom:20px}
.page-header h2{margin:0;font-size:1.5rem}
.subtitle{color:var(--text-secondary);font-size:.9rem}
.plaza-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(260px,1fr));gap:16px}
.plaza-card{background:var(--card-bg);border:1px solid var(--border-color);border-radius:12px;overflow:hidden;cursor:pointer;transition:box-shadow .2s}
.plaza-card:hover{box-shadow:0 4px 20px rgba(0,0,0,.1)}
.card-thumb{height:160px;display:flex;align-items:center;justify-content:center;background:var(--bg-color);overflow:hidden}
.thumb-img{width:100%;height:100%;object-fit:cover}
.thumb-placeholder{font-size:3rem;opacity:.3}
.card-info{padding:12px 14px}
.card-title{font-weight:600;font-size:1rem;margin-bottom:2px}
.card-author{color:var(--text-secondary);font-size:.85rem;margin-bottom:6px}
.card-meta{display:flex;align-items:center;gap:8px;font-size:.8rem;color:var(--text-secondary);margin-bottom:8px}
.card-actions{display:flex;gap:4px}
.empty{padding:60px 0;display:flex;justify-content:center}
</style>
