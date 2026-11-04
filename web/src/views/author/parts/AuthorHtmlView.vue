<template>
  <div class="novels-section">
    <h2 class="section-title-bar">
      <span class="title-bar-text">🎨 自定义 HTML 管理</span>
      <span class="title-bar-count">{{ htmlStats.frames }} 模板 · {{ htmlStats.htmls }} 扩展</span>
    </h2>
    <div class="html-mgr-grid">
      <div class="html-mgr-card" @click="$router.push('/author/templates')">
        <div class="hmc-icon">🎨</div>
        <div class="hmc-body">
          <div class="hmc-title">主题模板（UI 美化）</div>
          <div class="hmc-desc">阅读页 / 作者主页模板，支持沙盒与非沙盒注入</div>
          <div class="hmc-meta">
            <el-tag size="small" type="info">{{ htmlStats.frames }} 个</el-tag>
            <el-tag v-if="htmlStats.framePending > 0" size="small" type="warning">{{ htmlStats.framePending }} 待复核</el-tag>
          </div>
        </div>
      </div>
      <div class="html-mgr-card" @click="$router.push('/htmls')">
        <div class="hmc-icon">🔌</div>
        <div class="hmc-body">
          <div class="hmc-title">扩展应用（HTML 托管）</div>
          <div class="hmc-desc">ZIP 上传完整 HTML 应用，沙盒运行 / 广场发布</div>
          <div class="hmc-meta">
            <el-tag size="small" type="info">{{ htmlStats.htmls }} 个</el-tag>
            <el-tag v-if="htmlStats.htmlPending > 0" size="small" type="warning">{{ htmlStats.htmlPending }} 待复核</el-tag>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { authorDataVersion } from './authorRefresh';
import { ref, onMounted, watch } from 'vue';
import { frameApi, htmlApi } from '@/api/home/frame';

// 自定义 HTML 视图：主题模板（frames）与扩展应用（htmls）的入口与统计。
const htmlStats = ref({ frames: 0, htmls: 0, framePending: 0, htmlPending: 0 });

async function loadHtmlStats() {
  try {
    const [fr, hr] = await Promise.all([frameApi.list(), htmlApi.list()]);
    if (fr.data.code === 0) {
      const frames = fr.data.data || [];
      htmlStats.value.frames = frames.length;
      htmlStats.value.framePending = frames.filter((f: any) => f.review_status === 'pending').length;
    }
    if (hr.data.code === 0) {
      const htmls = hr.data.data || [];
      htmlStats.value.htmls = htmls.length;
      htmlStats.value.htmlPending = htmls.filter((h: any) => h.review_status === 'pending').length;
    }
  } catch { /* ignore */ }
}

onMounted(loadHtmlStats);

// 其它视图改动了数据（如回收站恢复作品）→ 重新拉取
watch(authorDataVersion, () => { loadHtmlStats });
</script>

<style scoped>
@import './section.css';

/* 自定义 HTML 管理入口 */
.html-mgr-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}
.html-mgr-card {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  padding: 20px;
  background: rgba(255,255,255,0.05);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 14px;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s, border-color 0.2s;
}
.html-mgr-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 8px 24px rgba(0,0,0,0.25);
  border-color: rgba(255,255,255,0.16);
}
[data-theme="light"] .html-mgr-card {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}
[data-theme="light"] .html-mgr-card:hover {
  box-shadow: var(--shadow-lg);
}
.hmc-icon { font-size: 1.8rem; line-height: 1; }
.hmc-body { flex: 1; min-width: 0; }
.hmc-title { font-weight: 600; font-size: 1rem; color: #e2e8f0; }
[data-theme="light"] .hmc-title { color: var(--text-color); }
.hmc-desc { font-size: 0.82rem; color: rgba(255,255,255,0.45); margin: 4px 0 8px; }
[data-theme="light"] .hmc-desc { color: var(--text-light); }
.hmc-meta { display: flex; gap: 6px; }

@media (max-width: 768px) {
  .html-mgr-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }
}
</style>
