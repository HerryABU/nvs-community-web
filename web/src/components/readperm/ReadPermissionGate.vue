<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { READ_PERM_REASONS, readPermApi } from '@/api/readperm';

// 读者端权限拦截卡片：小说阅读器 / 博客详情共用。
// 由父组件在捕获到 401/403 且响应带 data.reason 时传入，避免只看 toast。
const props = defineProps<{
  reason: string;
  message: string;
  authorId?: number;
  contentType?: string; // #67 付费内容：novel / blog
  contentId?: number;
}>();
const emit = defineEmits<{
  (e: 'close'): void;
  (e: 'purchased'): void;
}>();

const router = useRouter();
const route = useRoute();
const price = ref(0);
const buying = ref(false);

const visible = computed(() => !!props.reason);
const canPurchase = computed(() =>
  props.reason === READ_PERM_REASONS.paid && !!props.contentType && !!props.contentId);

watch(
  () => [props.reason, props.contentType, props.contentId],
  async () => {
    if (!canPurchase.value || !props.contentType || !props.contentId) return;
    try {
      const res = await readPermApi.purchaseStatus(props.contentType, props.contentId);
      price.value = res.data?.data?.price || 0;
    } catch { price.value = 0; }
  },
  { immediate: true },
);

const title = computed(() => {
  switch (props.reason) {
    case READ_PERM_REASONS.login: return '登录后继续阅读';
    case READ_PERM_REASONS.follower: return '粉丝专享内容';
    case READ_PERM_REASONS.vip: return '会员专享内容';
    case READ_PERM_REASONS.paid: return '付费内容';
    case READ_PERM_REASONS.private: return '私密内容';
    default: return '暂无阅读权限';
  }
});

const actionLabel = computed(() => {
  switch (props.reason) {
    case READ_PERM_REASONS.login: return '去登录';
    case READ_PERM_REASONS.follower: return '去看看作者主页';
    case READ_PERM_REASONS.vip: return '开通会员';
    case READ_PERM_REASONS.paid: return '购买解锁';
    default: return '知道了';
  }
});

async function doPurchase() {
  if (!props.contentType || !props.contentId) return;
  buying.value = true;
  try {
    const res = await readPermApi.purchase(props.contentType, props.contentId);
    if (res.data.code === 0) {
      ElMessage.success(res.data.data.already ? '已解锁，正在刷新' : '购买成功，正在刷新');
      emit('purchased');
      return;
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '购买失败');
  } finally {
    buying.value = false;
  }
}

function act() {
  switch (props.reason) {
    case READ_PERM_REASONS.login:
      router.push(`/login?redirect=${encodeURIComponent(route.fullPath)}`);
      break;
    case READ_PERM_REASONS.follower:
      if (props.authorId) router.push(`/author/${props.authorId}`);
      else ElMessage.info('请前往作者主页关注后再阅读');
      break;
    case READ_PERM_REASONS.vip:
      router.push('/membership');
      break;
    case READ_PERM_REASONS.paid:
      if (canPurchase.value) doPurchase();
      else router.push('/wallet');
      break;
    default:
      emit('close');
      break;
  }
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    :show-close="false"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    width="420px"
    align-center
    @update:model-value="emit('close')"
  >
    <template #header>
      <div class="rpg-title"><span class="rpg-lock">🔒</span>{{ title }}</div>
    </template>
    <p class="rpg-msg">{{ message }}</p>
    <p v-if="canPurchase && price > 0" class="rpg-note">
      解锁价格 <b>{{ price }}</b> 代币，从钱包余额扣除；购买后本内容永久可读。
    </p>
    <p v-else-if="reason === 'paid_required'" class="rpg-note">
      付费解锁由站长在后台开放，若尚未启用请联系站点管理员。
    </p>
    <template #footer>
      <el-button @click="emit('close')">返回</el-button>
      <el-button type="primary" :loading="buying" @click="act">{{ actionLabel }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.rpg-title { font-size: 1.05rem; font-weight: 600; display: flex; align-items: center; gap: 8px; }
.rpg-lock { font-size: 1.1rem; }
.rpg-msg { margin: 4px 0 0; line-height: 1.7; color: var(--text-regular, #606266); }
.rpg-note { margin: 10px 0 0; font-size: 0.82rem; color: var(--text-light, #999); line-height: 1.6; }
</style>
