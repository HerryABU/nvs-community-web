<template>
  <div class="novels-section">
    <h2 class="section-title-bar">
      <span class="title-bar-text">我的作品</span>
      <span class="title-bar-count">{{ novels.length }} 部</span>
    </h2>
    <div v-loading="loadingNovels">
      <el-empty v-if="!loadingNovels && novels.length === 0" description="还没有作品，快去创作吧！">
        <el-button type="primary" @click="$router.push('/author/editor')">新建作品</el-button>
      </el-empty>

      <div class="novel-card-grid" v-else>
        <div v-for="novel in novels" :key="novel.id" class="novel-card glass-novel-card">
          <div class="novel-card-cover">
            <el-image v-if="novel.cover_url" :src="novel.cover_url" fit="cover" class="cover-img">
              <template #error>
                <div class="cover-placeholder"><el-icon :size="32"><Document /></el-icon></div>
              </template>
            </el-image>
            <div v-else class="cover-placeholder"><el-icon :size="32"><Document /></el-icon></div>
            <div class="novel-status-badge" :class="novel.status">
              {{ novel.status === 'published' ? '已发布' : novel.status === 'draft' ? '草稿' : '已隐藏' }}
            </div>
          </div>
          <div class="novel-card-body">
            <h3 class="novel-card-title">{{ novel.title }}</h3>
            <div class="novel-card-tags">
              <template v-if="novel.categories && novel.categories.length > 0">
                <el-tag size="small" v-for="cat in novel.categories" :key="cat" class="cat-tag">{{ cat }}</el-tag>
              </template>
              <el-tag size="small" v-else class="cat-tag">{{ novel.category }}</el-tag>
            </div>
            <!-- 隔离墙开关（仅敏感分类作品显示） -->
            <div v-if="isSensitiveCategory(novel)" class="wall-toggle-row">
              <span style="font-size:0.8rem;color:#999">隔离确认</span>
              <el-switch
                :model-value="novel.wall_enabled !== false"
                size="small"
                :loading="wallToggling === novel.id"
                @change="(val: boolean) => toggleWall(novel, val)"
              />
              <span style="font-size:0.75rem" :style="{color: novel.wall_enabled !== false ? '#22c55e' : '#999'}">
                {{ novel.wall_enabled !== false ? '已启用' : '已关闭' }}
              </span>
            </div>
            <div class="novel-card-meta">
              <span><el-icon><Collection /></el-icon>{{ novel.total_chapters || 0 }} 章</span>
              <span><el-icon><MagicStick /></el-icon>{{ (novel.total_words || 0).toLocaleString() }} 字</span>
            </div>
          </div>
          <div class="novel-card-actions">
            <el-button size="small" type="primary" plain @click="$router.push(`/author/editor/${novel.id}`)">
              <el-icon><Edit /></el-icon>编辑
            </el-button>
            <el-button size="small" plain @click="$router.push(`/novel/${novel.id}`)">
              <el-icon><View /></el-icon>查看
            </el-button>
            <el-button size="small" plain @click="openCollab(novel.id)">
              <el-icon><UserFilled /></el-icon>协作
            </el-button>
            <el-dropdown trigger="click" @command="(fmt: string) => handleExport(novel.id, fmt)">
              <el-button size="small" type="warning" plain :loading="exportingId === novel.id">
                <el-icon><Download /></el-icon>导出<el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="zip">ZIP 打包</el-dropdown-item>
                  <el-dropdown-item command="epub">EPUB 电子书</el-dropdown-item>
                  <el-dropdown-item command="docx">Word 文档</el-dropdown-item>
                  <el-dropdown-item command="md">合并 Markdown</el-dropdown-item>
                  <el-dropdown-item command="txt">纯文本 TXT</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button size="small" type="danger" plain @click="handleDelete(novel)">
              <el-icon><Delete /></el-icon>删除
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 多人协作：协作者 / 角色 / 分成 / 署名（docs/novel-collaboration.md） -->
    <CollaboratorsPanel v-model:visible="collabVisible" :novel-id="collabNovelId" :novel-title="collabNovelTitle" />
  </div>
</template>

<script setup lang="ts">
import { authorDataVersion } from './authorRefresh';
import { ref, onMounted, watch } from 'vue';
import {
  ArrowDown, Document, Collection, MagicStick, UserFilled, Edit, View, Download, Delete,
} from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { novelApi, type Novel } from '@/api/novel';
import { authApi } from '@/api/auth';
import CollaboratorsPanel from '@/components/collab/CollaboratorsPanel.vue';

// 作品视图：作品卡片 + 导出 / 删除 / 协作 + 敏感作品隔离墙开关。
const novels = ref<Novel[]>([]);
const loadingNovels = ref(false);
const exportingId = ref(0);

// 多人协作：作品级协作者管理面板
const collabVisible = ref(false);
const collabNovelId = ref(0);
const collabNovelTitle = ref('');
function openCollab(novelId: number) {
  collabNovelId.value = novelId;
  const n = novels.value.find((x: any) => x.id === novelId);
  collabNovelTitle.value = n?.title || '';
  collabVisible.value = true;
}

// 隔离墙开关
const wallToggling = ref(0);
const sensitiveZones = ref<string[]>([]);

const exportFormats: Record<string, { label: string; ext: string; mime: string; fn: (id: number) => Promise<any> }> = {
  zip: { label: 'ZIP 打包', ext: 'zip', mime: 'application/zip', fn: novelApi.exportNovel },
  epub: { label: 'EPUB 电子书', ext: 'epub', mime: 'application/epub+zip', fn: novelApi.exportNovelEPUB },
  docx: { label: 'Word 文档', ext: 'docx', mime: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', fn: novelApi.exportNovelDOCX },
  md: { label: '合并 Markdown', ext: 'md', mime: 'text/markdown', fn: novelApi.exportNovelMarkdown },
  txt: { label: '纯文本 TXT', ext: 'txt', mime: 'text/plain', fn: novelApi.exportNovelTXT },
};

async function loadNovels() {
  loadingNovels.value = true;
  try {
    const res = await novelApi.getMyNovels();
    novels.value = res.data.data || [];
  } catch (e) {
    console.error(e);
  } finally {
    loadingNovels.value = false;
  }
}

async function handleExport(id: number, format: string = 'zip') {
  const fmt = exportFormats[format] || exportFormats.zip;
  exportingId.value = id;
  try {
    const res = await fmt.fn.call(novelApi, id);
    const blob = new Blob([res.data], { type: fmt.mime });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `novel_${id}.${fmt.ext}`;
    a.click();
    URL.revokeObjectURL(url);
    ElMessage.success(`${fmt.label}导出成功`);
  } catch {
    ElMessage.error(`${fmt.label}导出失败`);
  } finally {
    exportingId.value = 0;
  }
}

async function handleDelete(novel: Novel) {
  try {
    await ElMessageBox.confirm(`确定要删除《${novel.title}》吗？此操作不可恢复！`, '删除确认', {
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
      type: 'warning',
    });
    await novelApi.deleteNovel(novel.id);
    ElMessage.success('已删除');
    await loadNovels();
  } catch { /* cancelled */ }
}

function matchZone(cat: string): boolean {
  if (sensitiveZones.value.includes(cat)) return true;
  for (const zone of sensitiveZones.value) {
    if (cat.includes(zone) || (zone.includes(cat) && cat.length >= 2)) return true;
  }
  return false;
}

function isSensitiveCategory(novel: any): boolean {
  const cats = novel.categories?.length ? novel.categories : (novel.category ? [novel.category] : []);
  return cats.some((c: string) => matchZone(c));
}

async function toggleWall(novel: any, enabled: boolean) {
  wallToggling.value = novel.id;
  try {
    await authApi.put('/author/profile', { wall_enabled: enabled, novel_id: novel.id });
    novel.wall_enabled = enabled;
    ElMessage.success(enabled ? '隔离确认已启用' : '隔离确认已关闭');
  } catch (e: any) {
    ElMessage.error('操作失败');
  }
  wallToggling.value = 0;
}

async function loadSensitiveZones() {
  try {
    const res = await authApi.get('/site-info');
    const d = res.data?.data;
    // 必须 wall_enabled=true 且 wall_zones 存在才启用隔离墙
    if (d?.wall_enabled && Array.isArray(d.wall_zones)) {
      sensitiveZones.value = d.wall_zones;
    }
  } catch {}
}

onMounted(() => {
  loadNovels();
  loadSensitiveZones();
});

// 其它视图改动了数据（如回收站恢复作品）→ 重新拉取
watch(authorDataVersion, () => {
  loadNovels();
  loadSensitiveZones();
});
</script>

<style scoped>
@import './section.css';


/* ===== 作品卡片 ===== */
.novel-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}

.glass-novel-card {
  border-radius: 16px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: transform 0.3s ease, box-shadow 0.3s ease, border-color 0.3s ease;
}

[data-theme="light"] .glass-novel-card {
  background: #fff;
  border-color: var(--border-color);
  box-shadow: var(--shadow-card);
}

.glass-novel-card:hover {
  transform: translateY(-6px);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.35);
  border-color: rgba(255, 255, 255, 0.15);
}

[data-theme="light"] .glass-novel-card:hover {
  box-shadow: var(--shadow-lg);
  border-color: var(--border-color);
}

.novel-card-cover {
  position: relative;
  height: 200px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.15), rgba(139, 92, 246, 0.1));
  overflow: hidden;
}

.cover-img {
  width: 100%;
  height: 100%;
}

.cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.2);
}

[data-theme="light"] .cover-placeholder {
  color: rgba(0, 0, 0, 0.15);
}

.novel-status-badge {
  position: absolute;
  top: 12px;
  right: 12px;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 0.75rem;
  font-weight: 600;
  color: #fff;
  backdrop-filter: blur(6px);
}

.novel-status-badge.published {
  background: rgba(39, 174, 96, 0.85);
}

.novel-status-badge.draft {
  background: rgba(149, 165, 166, 0.85);
}

.novel-status-badge.hidden {
  background: rgba(231, 76, 60, 0.85);
}

.novel-card-body {
  padding: 16px;
  flex: 1;
}

.novel-card-title {
  font-size: 1.1rem;
  font-weight: 600;
  color: #e2e8f0;
  margin: 0 0 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

[data-theme="light"] .novel-card-title {
  color: var(--text-color);
}

.novel-card-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 10px;
}

.cat-tag {
  font-size: 0.75rem;
}

.novel-card-meta {
  display: flex;
  gap: 16px;
  font-size: 0.85rem;
  color: rgba(255, 255, 255, 0.45);
}

[data-theme="light"] .novel-card-meta {
  color: var(--text-light);
}

.novel-card-meta span {
  display: flex;
  align-items: center;
  gap: 4px;
}

.novel-card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 12px 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

[data-theme="light"] .novel-card-actions {
  border-top-color: var(--border-color);
}

@media (max-width: 768px) {
  /* 作品卡片网格：窄屏单列 */
  .novel-card-grid {
    grid-template-columns: 1fr;
    gap: 14px;
  }
}
</style>
