<template>
  <!-- ====== 站点公告栏 + 公告弹窗 + 推荐说明（F15：从公告栏区块渲染；176号令合规通知） ====== -->
  <div v-if="announcements.length" class="notice-bar">
    <span class="notice-icon">📢</span>
    <div class="notice-items">
      <span
        v-for="a in announcements.slice(0, maxItems)"
        :key="a.id"
        class="notice-item"
        @click="openNotice(a)"
      >
        <el-tag v-if="a.pinned" type="danger" size="small" effect="plain" class="notice-pin-tag">置顶</el-tag>
        <span class="notice-item-title">{{ a.title }}</span>
        <span class="notice-item-date">{{ formatDate(a.created_at) }}</span>
      </span>
    </div>
    <span class="notice-more" @click="showAllNotice = true">查看全部</span>
  </div>

  <!-- 公告详情弹窗（内容经 cherry-markdown 渲染，与评论区同款样式） -->
  <el-dialog v-model="noticeDlg.visible" :title="noticeDlg.item?.title || '公告'" width="600px">
    <div class="notice-dlg-content cherry-markdown cherry-previewer">
      <div v-html="noticeHtml"></div>
    </div>
    <div class="notice-dlg-date">发布于 {{ formatDate(noticeDlg.item?.created_at) }}</div>
  </el-dialog>
  <el-dialog v-model="showAllNotice" title="📢 全部公告" width="600px">
    <div v-if="!announcements.length" style="color: var(--text-light)">暂无公告</div>
    <div v-for="a in announcements" :key="a.id" class="notice-list-item">
      <el-tag v-if="a.pinned" type="danger" size="small" effect="plain" style="margin-right:6px">置顶</el-tag>
      <span class="notice-list-title" @click="openNotice(a); showAllNotice = false">{{ a.title }}</span>
      <span class="notice-list-date">{{ formatDate(a.created_at) }}</span>
    </div>
  </el-dialog>

  <!-- 推荐说明（176号令 第七条第九项 算法透明度） -->
  <el-dialog v-model="showAlgo" title="📊 推荐说明" width="560px">
    <div class="algo-desc" style="white-space: pre-wrap; line-height: 1.8">{{ algoDescription }}</div>
    <template #footer><el-button type="primary" @click="showAlgo = false">知道了</el-button></template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { useAppStore } from '@/stores/app';
import { renderMarkdown } from '@/markdown/renderer';

const props = defineProps<{
  /** 最多展示几条公告 */
  maxItems?: number;
}>();

const appStore = useAppStore();
const announcements = ref<any[]>([]);
const showAllNotice = ref(false);
const noticeDlg = reactive({ visible: false, item: null as any });
const algoDescription = ref('');
const showAlgo = ref(false);
const maxItems = computed(() => props.maxItems || 3);
const noticeHtml = computed(() => renderMarkdown(noticeDlg.item?.content || ''));

function formatDate(d?: string) { return d ? new Date(d).toLocaleDateString('zh-CN') : ''; }

// 多公告排序：置顶优先，其余按发布时间倒序（后端已排序，前端二次保险）
function sortAnnouncements(list: any[]) {
  return [...list].sort((a, b) => {
    if (!!a.pinned !== !!b.pinned) return a.pinned ? -1 : 1;
    return new Date(b.created_at || 0).getTime() - new Date(a.created_at || 0).getTime();
  });
}

function openNotice(a: any) {
  noticeDlg.item = a;
  noticeDlg.visible = true;
}

// 公告与算法说明来自应用级聚合 batch（首屏 1 次请求，SPA 内复跑 ensureHomeLoaded 兜底）
onMounted(async () => {
  try {
    await appStore.ensureHomeLoaded();
    const d = appStore.homeData;
    if (!d) return;
    announcements.value = sortAnnouncements(d.announcements || []);
    if (d.algo) algoDescription.value = d.algo;
  } catch (e) {
    console.error(e);
  }
});

defineExpose({ openAlgo: () => (showAlgo.value = true) });
</script>

<style scoped>
/* ====== 公告栏（176号令：合规通知/安全公告） ====== */
.notice-bar {
  max-width: 1200px; margin: 14px auto 0; padding: 8px 16px;
  display: flex; align-items: center; gap: 12px;
  background: linear-gradient(90deg, rgba(250,204,21,0.12), rgba(250,204,21,0.03));
  border: 1px solid rgba(250,204,21,0.35); border-radius: 10px;
  font-size: 0.9rem;
}
.notice-icon { flex-shrink: 0; }
.notice-items { flex: 1; display: flex; flex-wrap: wrap; gap: 6px 18px; overflow: hidden; }
.notice-item { cursor: pointer; display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 360px; }
.notice-item-title { overflow: hidden; text-overflow: ellipsis; }
.notice-item-date { flex-shrink: 0; font-size: 0.75rem; color: var(--text-light, #aaa); }
.notice-item:hover { text-decoration: underline; }
.notice-pin-tag { flex-shrink: 0; }
.notice-more { flex-shrink: 0; cursor: pointer; color: var(--el-color-primary); font-size: 0.8rem; }
/* 公告详情弹窗：内容经 cherry-markdown 渲染，长文限高滚动 */
.notice-dlg-content { max-height: 60vh; overflow-y: auto; }
.notice-dlg-date { color: var(--text-light, #999); font-size: 0.8rem; margin-top: 8px; }
.notice-list-item { display: flex; align-items: center; padding: 8px 0; border-bottom: 1px dashed var(--el-border-color); }
.notice-list-title { flex: 1; cursor: pointer; }
.notice-list-title:hover { color: var(--el-color-primary); }
.notice-list-date { color: var(--text-light, #999); font-size: 0.8rem; }
</style>
