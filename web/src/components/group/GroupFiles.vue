<template>
  <div class="gf">
    <div class="gf-bar">
      <div class="gf-used">
        已用 {{ human(used) }} / {{ human(quota) }}
        <el-progress :percentage="pct" :stroke-width="5" :show-text="false" style="width:160px;margin-top:6px" />
      </div>
      <div class="gf-spacer" />
      <el-button type="primary" size="small" :loading="uploading" @click="pick">
        <el-icon><Upload /></el-icon> 上传文件
      </el-button>
      <input ref="fileRef" type="file" hidden @change="onFile" />
    </div>
    <div v-if="uploading" class="gf-progress">上传中 {{ progress }}%</div>

    <div v-loading="loading" class="gf-list">
      <div v-for="f in list" :key="f.id" class="gf-item">
        <span class="gf-icon">{{ iconOf(f.name) }}</span>
        <div class="gf-info">
          <div class="gf-name">{{ f.name }}</div>
          <div class="gf-sub">
            <span>{{ human(f.size) }}</span>
            <span>{{ f.uploader_name }}</span>
            <span>{{ formatDate(f.created_at) }}</span>
          </div>
        </div>
        <div class="gf-actions">
          <el-button size="small" text type="primary" @click="download(f)">下载</el-button>
          <el-button v-if="canManage || f.uploader_id === auth.user?.id" size="small" text type="danger" @click="remove(f)">删除</el-button>
        </div>
      </div>
      <EmptyState v-if="!loading && !list.length" icon="📎" text="还没有群文件" />
    </div>
    <div class="gf-note">单文件 ≤ 50MB，单群 ≤ 512MB；压缩包会做 ZIP 炸弹检测；文件仅群成员可下载。</div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Upload } from '@element-plus/icons-vue';
import { groupApi, type GroupFile } from '@/api/group';
import { useAuthStore } from '@/stores/auth';
import EmptyState from '@/components/common/EmptyState.vue';

const props = defineProps<{ groupId: number; canManage: boolean }>();
const auth = useAuthStore();

const list = ref<GroupFile[]>([]);
const loading = ref(false);
const uploading = ref(false);
const progress = ref(0);
const used = ref(0);
const quota = ref(512 * 1024 * 1024);
const fileRef = ref<HTMLInputElement | null>(null);

const pct = computed(() => (quota.value ? Math.min(100, Math.round((used.value / quota.value) * 100)) : 0));

function human(n: number) {
  if (!n) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`;
}
function formatDate(s: string) {
  if (!s) return '';
  const d = new Date(s);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
function iconOf(name: string) {
  const ext = name.slice(name.lastIndexOf('.')).toLowerCase();
  if (['.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp'].includes(ext)) return '🖼️';
  if (ext === '.pdf') return '📕';
  if (['.doc', '.docx'].includes(ext)) return '📄';
  if (['.xls', '.xlsx', '.csv'].includes(ext)) return '📊';
  if (['.ppt', '.pptx'].includes(ext)) return '📽️';
  if (['.zip', '.7z', '.rar'].includes(ext)) return '🗜️';
  if (['.mp4', '.webm', '.mp3', '.wav', '.m4a'].includes(ext)) return '🎬';
  return '📎';
}

async function load() {
  loading.value = true;
  try {
    const res = await groupApi.files(props.groupId, { page: 1, size: 100 });
    if (res.data.code === 0) {
      list.value = res.data.data.list || [];
      used.value = res.data.data.used || 0;
      quota.value = res.data.data.quota || quota.value;
    }
  } catch { /* ignore */ }
  loading.value = false;
}

function pick() {
  fileRef.value?.click();
}

async function onFile(e: Event) {
  const input = e.target as HTMLInputElement;
  const f = input.files?.[0];
  input.value = '';
  if (!f) return;
  if (f.size > 50 * 1024 * 1024) {
    ElMessage.warning('单文件不能超过 50MB');
    return;
  }
  uploading.value = true;
  progress.value = 0;
  try {
    const res = await groupApi.uploadFile(props.groupId, f, (p) => (progress.value = p));
    if (res.data.code === 0) {
      ElMessage.success('上传成功');
      await load();
    }
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.message || '上传失败');
  }
  uploading.value = false;
}

async function download(f: GroupFile) {
  try {
    await groupApi.downloadFile(props.groupId, f.id, f.name);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '下载失败');
  }
}

async function remove(f: GroupFile) {
  try {
    await ElMessageBox.confirm(`删除文件「${f.name}」？`, '删除确认', { type: 'warning' });
  } catch { return; }
  try {
    await groupApi.deleteFile(props.groupId, f.id);
    ElMessage.success('已删除');
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败');
  }
}

load();
defineExpose({ reload: load });
</script>

<style scoped>
.gf-bar { display: flex; align-items: center; gap: 12px; margin-bottom: 10px; flex-wrap: wrap; }
.gf-used { font-size: 0.76rem; color: var(--text-light, #64748b); }
.gf-spacer { flex: 1; }
.gf-progress { font-size: 0.74rem; color: var(--primary-color, #409eff); margin-bottom: 8px; }
.gf-list { display: flex; flex-direction: column; gap: 6px; min-height: 100px; }
.gf-item {
  display: flex; align-items: center; gap: 10px; padding: 9px 12px;
  border: 1px solid var(--border-color, #eee); border-radius: 10px; background: var(--card-bg, #fff);
}
.gf-icon { font-size: 1.1rem; }
.gf-info { flex: 1; min-width: 0; }
.gf-name { font-size: 0.86rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.gf-sub { display: flex; gap: 10px; font-size: 0.72rem; color: var(--text-light, #94a3b8); margin-top: 2px; }
.gf-actions { display: flex; gap: 2px; }
.gf-note { margin-top: 10px; font-size: 0.72rem; color: var(--text-light, #94a3b8); }
</style>
