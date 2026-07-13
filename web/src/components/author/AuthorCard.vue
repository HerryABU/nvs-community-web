<template>
  <div class="glass-stat-card" style="padding:20px 28px;margin-bottom:24px;display:flex;align-items:center;gap:24px;flex-wrap:wrap">
    <div style="position:relative;cursor:pointer" @click="triggerAvatar">
      <el-avatar :size="72" :src="authStore.user?.avatar_url">
        <el-icon :size="32"><UserFilled /></el-icon>
      </el-avatar>
      <div style="position:absolute;bottom:0;right:0;background:var(--primary-color);border-radius:50%;width:24px;height:24px;display:flex;align-items:center;justify-content:center">
        <el-icon :size="14" color="#fff"><Camera /></el-icon>
      </div>
    </div>
    <input ref="avatarInput" type="file" accept="image/*" style="display:none" @change="onAvatarChange" />
    <input ref="imageInput" type="file" accept="image/*" style="display:none" @change="onImageChange" />
    <el-button size="small" @click="triggerImage" style="margin-left:8px">
      <el-icon><Picture /></el-icon> 传图
    </el-button>

    <div style="flex:1;min-width:200px">
      <div style="font-size:1.1rem;font-weight:600;margin-bottom:4px">{{ authStore.user?.nickname || authStore.user?.username }}</div>
      <div style="color:#999;font-size:0.85rem">{{ authStore.user?.email }}</div>
      <div style="margin-top:8px;font-size:0.85rem">
        角色：<el-tag :type="authStore.user?.role === 'vip_author' ? 'warning' : 'success'" size="small">{{ authStore.user?.role === 'vip_author' ? 'VIP 作者' : '作者' }}</el-tag>
      </div>
      <div style="margin-top:6px;display:flex;align-items:center;gap:6px">
        <span v-if="!editingSig" style="color:#999;font-size:0.82rem;cursor:pointer;max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" @click="startEditSig">
          {{ authStore.user?.signature || '点击设置个性签名…' }}
        </span>
        <el-input
          v-else
          v-model="sigDraft"
          size="small"
          maxlength="60"
          placeholder="输入个性签名（最多60字）"
          style="width:220px"
          @blur="saveSignature"
          @keyup.enter="saveSignature"
        />
      </div>
    </div>
    <div style="min-width:180px;padding:12px 16px;background:rgba(64,158,255,0.06);border-radius:8px">
      <div style="font-size:0.8rem;color:#999;margin-bottom:4px">密码学签名</div>
      <div v-if="sigStatus">
        <div style="font-weight:600;font-size:0.95rem">
          {{ sigStatus.has_signing_key ? '🔐 已启用' : '⚠ 未启用' }}
        </div>
        <div style="font-size:0.75rem;color:#999;margin-top:4px">
          已签名 {{ sigStatus.signed_chapters }}/{{ sigStatus.total_chapters }} 章
          <span v-if="sigStatus.total_chapters > 0">
            ({{ sigStatus.coverage_percent.toFixed(0) }}%)
          </span>
        </div>
      </div>
      <div v-else style="color:#999;font-size:0.85rem">加载中...</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useAuthStore } from '@/stores/auth';
import { authApi } from '@/api/auth';
import { ElMessage } from 'element-plus';
import { UserFilled, Camera, Picture } from '@element-plus/icons-vue';

const authStore = useAuthStore();

const avatarInput = ref<HTMLInputElement>();
const imageInput = ref<HTMLInputElement>();
const editingSig = ref(false);
const sigDraft = ref('');

interface SigStatus { has_signing_key: boolean; total_chapters: number; signed_chapters: number; coverage_percent: number; }
const sigStatus = ref<SigStatus | null>(null);

function triggerAvatar() { avatarInput.value?.click(); }
function triggerImage() { imageInput.value?.click(); }

async function onAvatarChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0];
  if (!file) return;
  const fd = new FormData(); fd.append('avatar', file);
  try {
    const res = await authApi.uploadAvatar(fd);
    authStore.user!.avatar_url = res.data.data.avatar_url;
    ElMessage.success('头像已更新');
  } catch { ElMessage.error('上传失败'); }
}

async function onImageChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0];
  if (!file) return;
  const fd = new FormData(); fd.append('image', file);
  try {
    const res = await authApi.uploadImage(fd);
    ElMessage.success({ message: '图片上传成功', type: 'success', duration: 2000 });
    navigator.clipboard.writeText(res.data.data.url);
    ElMessage.info('图片 URL 已复制到剪贴板');
  } catch { ElMessage.error('上传失败'); }
}

function startEditSig() {
  sigDraft.value = authStore.user?.signature || '';
  editingSig.value = true;
}

async function saveSignature() {
  editingSig.value = false;
  const sig = sigDraft.value.trim();
  if (sig === (authStore.user?.signature || '')) return;
  try {
    await authApi.updateProfile({ signature: sig });
    authStore.user!.signature = sig;
  } catch { /* ignore */ }
}

async function loadSigStatus() {
  try {
    const res = await authApi.getSignatureStatus();
    sigStatus.value = res.data.data;
  } catch { /* ignore */ }
}

onMounted(() => { loadSigStatus(); });
</script>
