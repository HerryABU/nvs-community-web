<template>
  <div class="emote-mgr">
    <div class="em-head">
      <div>
        <div class="em-title">我的自定义表情</div>
        <div class="em-hint">
          气泡里写 <code>[键]</code> 就会渲染成图片。表情键是<b>全局唯一</b>的——气泡由服务端渲染，
          没有「当前用户」上下文，所以别人占了同名键必须换名（不会静默覆盖）。
        </div>
      </div>
    </div>

    <div class="em-form">
      <el-input v-model="key" placeholder="表情键（1~8 字，不含方括号）" style="width: 200px" />
      <el-upload
        :show-file-list="false"
        :http-request="uploadEmote"
        accept="image/png,image/jpeg,image/gif,image/webp"
      >
        <el-button :loading="uploading">上传图片并添加</el-button>
      </el-upload>
      <span class="em-hint">或</span>
      <el-input v-model="url" placeholder="已有图片地址 /uploads/..." style="width: 260px" />
      <el-button type="primary" :loading="creating" @click="createByUrl">添加</el-button>
    </div>

    <div v-if="list.length" class="em-grid">
      <div v-for="e in list" :key="e.id" class="em-item">
        <img :src="e.url" :alt="e.key" class="em-img" />
        <div class="em-key">[{{ e.key }}]</div>
        <el-button text size="small" type="danger" @click="remove(e)">删除</el-button>
      </div>
    </div>
    <el-empty v-else description="还没有自定义表情" :image-size="70" />

    <div class="em-builtin">
      <div class="em-title">内置表情（无需上传，直接可用）</div>
      <div class="em-keys">
        <el-tag v-for="k in builtin" :key="k" size="small" class="em-tag">[{{ k }}]</el-tag>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import api from '@/api/index';

interface Emote {
  id: number;
  key: string;
  url: string;
  note: string;
}

const list = ref<Emote[]>([]);
const builtin = ref<string[]>([]);
const key = ref('');
const url = ref('');
const creating = ref(false);
const uploading = ref(false);

async function load() {
  const res = await api.get<{ code: number; data: { list: Emote[]; builtin_keys: string[] } }>('/oc/emotes');
  list.value = res.data.data?.list || [];
  builtin.value = res.data.data?.builtin_keys || [];
}

async function uploadEmote(opt: any) {
  if (!key.value.trim()) {
    ElMessage.warning('请先填表情键');
    return;
  }
  uploading.value = true;
  try {
    const fd = new FormData();
    fd.append('file', opt.file);
    const up = await api.post<{ code: number; data: { url: string } }>('/upload', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    const u = up.data.data?.url;
    if (!u) throw new Error('上传未返回地址');
    await create(u);
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '上传失败');
  } finally {
    uploading.value = false;
  }
}

async function create(u: string) {
  creating.value = true;
  try {
    await api.post('/oc/emotes', { key: key.value.trim(), url: u, note: '' });
    ElMessage.success(`已添加，可在气泡里用 [${key.value.trim()}]`);
    key.value = '';
    url.value = '';
    await load();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '添加失败');
  } finally {
    creating.value = false;
  }
}

async function createByUrl() {
  if (!key.value.trim() || !url.value.trim()) {
    ElMessage.warning('请填表情键与图片地址');
    return;
  }
  await create(url.value.trim());
}

async function remove(e: Emote) {
  await ElMessageBox.confirm(`删除表情 [${e.key}]？`, '确认', { type: 'warning' });
  await api.delete(`/oc/emotes/${e.id}`);
  ElMessage.success('已删除');
  await load();
}

onMounted(load);
</script>

<style scoped>
.emote-mgr { padding: 4px; }
.em-head { margin-bottom: 10px; }
.em-title { font-size: 0.92rem; font-weight: 600; }
.em-hint { color: var(--text-muted); font-size: 0.76rem; line-height: 1.6; }
.em-form { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 14px; }
.em-grid { display: flex; flex-wrap: wrap; gap: 12px; }
.em-item {
  width: 104px; border: 1px solid var(--border-color); border-radius: 10px;
  padding: 8px; text-align: center;
}
.em-img { width: 72px; height: 72px; object-fit: contain; display: block; margin: 0 auto 4px; }
.em-key { font-size: 0.72rem; color: var(--text-muted); margin-bottom: 2px; word-break: break-all; }
.em-builtin { margin-top: 18px; }
.em-keys { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; }
.em-tag { font-family: ui-monospace, monospace; }
code { background: var(--bg-muted); padding: 0 3px; border-radius: 3px; }
</style>
