<template>
  <div class="gq-card">
    <div class="gq-main">
      <canvas ref="canvasRef" class="gq-canvas" />
      <div class="gq-meta">
        <div class="gq-label">群号</div>
        <div class="gq-no">{{ group.group_no || '—' }}</div>
        <div class="gq-tip">扫码或输入群号即可申请加入</div>
        <div class="gq-actions">
          <el-button size="small" @click="copy(group.group_no, '群号已复制')">复制群号</el-button>
          <el-button size="small" type="primary" plain @click="copy(shareUrl, '链接已复制')">复制链接</el-button>
          <el-button v-if="inviteCode" size="small" @click="copy(inviteCode, '邀请码已复制')">
            邀请码 {{ inviteCode }}
          </el-button>
        </div>
      </div>
    </div>
    <div class="gq-foot">二维码由浏览器本地生成，不经过服务器</div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import QRCode from 'qrcode';
import { ElMessage } from 'element-plus';
import { appPrefix } from '@/utils/prefix';
import { groupShareUrl } from '@/utils/groupQr';

const props = defineProps<{
  group: { id: number; name: string; group_no: string };
  inviteCode?: string;
}>();

const canvasRef = ref<HTMLCanvasElement | null>(null);

const shareUrl = computed(() => {
  const origin = window.location.origin;
  const base = `${origin}${appPrefix || ''}`;
  return groupShareUrl(base, props.group.id, props.group.group_no);
});

async function render() {
  const el = canvasRef.value;
  if (!el || !props.group.group_no) return;
  try {
    await QRCode.toCanvas(el, shareUrl.value, {
      width: 168,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: { dark: '#111827', light: '#ffffff' },
    });
  } catch {
    /* 生成失败不阻塞页面 */
  }
}

async function copy(text: string, msg: string) {
  try {
    await navigator.clipboard.writeText(text);
    ElMessage.success(msg);
  } catch {
    ElMessage.warning('浏览器未授予剪贴板权限，请手动复制');
  }
}

onMounted(render);
watch(() => [props.group.id, props.group.group_no], render);
</script>

<style scoped>
.gq-card {
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 12px;
  padding: 14px;
  background: var(--card-bg, #fff);
}
.gq-main { display: flex; gap: 16px; align-items: center; flex-wrap: wrap; }
.gq-canvas { width: 168px; height: 168px; border-radius: 8px; background: #fff; }
.gq-label { font-size: 0.74rem; color: var(--text-light, #94a3b8); }
.gq-no { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 1.5rem; font-weight: 700; letter-spacing: 0.06em; }
.gq-tip { font-size: 0.76rem; color: var(--text-light, #94a3b8); margin-top: 2px; }
.gq-actions { display: flex; gap: 8px; margin-top: 10px; flex-wrap: wrap; }
.gq-foot { margin-top: 10px; font-size: 0.7rem; color: var(--text-light, #94a3b8); }
</style>
