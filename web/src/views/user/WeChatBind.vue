<template>
  <!-- 微信账号绑定（F21.2）：绑定状态展示 / 发起绑定 / 解绑（二次确认）。
       绑定走「授权 URL 跳转 + 微信回调」：点击绑定先取授权 URL 并把 state 存入
       sessionStorage，再跳转微信；确认后微信 302 到后端 callback，
       后端把微信写入当前登录态并 302 回本页（?wechat=bound / ?wechat=failed&reason=...）。 -->
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2>💬 微信绑定</h2>
        <p class="subtitle">
          绑定微信后可使用扫码登录；一个微信只能绑定一个账号，一个账号可绑定一个微信（OpenID/UnionID）。
        </p>
      </div>
      <el-button v-if="!bound" type="primary" :loading="binding" @click="startBind">绑定微信</el-button>
      <el-button v-else type="danger" plain :loading="unbinding" @click="unbind">解绑微信</el-button>
    </div>

    <el-card v-loading="loading" class="bind-body">
      <template v-if="bound">
        <div class="bind-row">
          <el-avatar :size="48" :src="status.avatar">{{ (status.nickname || '?')[0] }}</el-avatar>
          <div class="bind-meta">
            <span class="bind-name">{{ status.nickname || '微信用户' }}</span>
            <el-tag size="small" type="success">已绑定</el-tag>
          </div>
        </div>
        <p class="bind-hint">绑定后可在登录页使用微信扫码直接登录本账号。</p>
      </template>
      <el-empty v-else-if="!loading" description="尚未绑定微信">
        <template #description>
          <p>尚未绑定微信</p>
          <p class="bind-hint">点击右上角「绑定微信」，扫码确认后即可使用微信扫码登录。</p>
        </template>
      </el-empty>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { wechatApi, type WeChatBindStatusResp } from '@/api/wechat';

// sessionStorage key：绑定发起时保存 state（刷新/重进可校验，避免流程中断）
const BIND_STATE_KEY = 'nvs:wechat:bind:state';

const route = useRoute();
const loading = ref(false);
const binding = ref(false);
const unbinding = ref(false);
const bound = ref(false);
const status = reactive<WeChatBindStatusResp>({ bound: false, nickname: '', avatar: '' });

// 回调失败原因 → 中文提示（与后端 WeChatCallback 的 reason 一致）
const FAIL_REASONS: Record<string, string> = {
  cancel: '已取消微信授权',
  state: '绑定状态已过期，请重新发起绑定',
  exchange: '微信授权失败，请重试',
  userinfo: '获取微信信息失败，请重试',
  taken: '该微信已绑定其他账号，请先在其账号下解绑',
  locked: '账号已锁定，暂无法绑定',
  server: '服务器错误，请稍后重试',
  register: '账号创建失败，请稍后重试',
};

async function loadStatus() {
  loading.value = true;
  try {
    const res = await wechatApi.getBindStatus();
    if (res.data.code === 0) {
      bound.value = res.data.data.bound;
      status.bound = res.data.data.bound;
      status.nickname = res.data.data.nickname || '';
      status.avatar = res.data.data.avatar || '';
    }
  } catch {
    /* 拦截器已输出错误信息 */
  } finally {
    loading.value = false;
  }
}

// 发起绑定：先取授权 URL（purpose=bind），state 存入 sessionStorage 后跳转微信
async function startBind() {
  binding.value = true;
  try {
    const res = await wechatApi.getAuthUrl('bind');
    if (res.data.code === 0 && res.data.data?.url) {
      sessionStorage.setItem(BIND_STATE_KEY, res.data.data.state);
      window.location.href = res.data.data.url;
    } else {
      ElMessage.error(res.data.message || '获取微信授权链接失败');
    }
  } catch (e: any) {
    ElMessage.error(e.response?.data?.message || '获取微信授权链接失败，请稍后重试');
  } finally {
    binding.value = false;
  }
}

// 解绑：二次确认
async function unbind() {
  try {
    await ElMessageBox.confirm('确定解绑微信吗？解绑后将无法使用微信扫码登录本账号。', '解绑微信', {
      type: 'warning',
      confirmButtonText: '解绑',
      cancelButtonText: '取消',
    });
  } catch {
    return; // 用户取消
  }
  unbinding.value = true;
  try {
    const res = await wechatApi.unbindWeChat();
    if (res.data.code === 0) {
      ElMessage.success('已解绑微信');
      sessionStorage.removeItem(BIND_STATE_KEY);
      loadStatus();
    }
  } catch {
    /* 拦截器已输出错误信息 */
  } finally {
    unbinding.value = false;
  }
}

// 处理微信回调重定向带回的结果（?wechat=bound / ?wechat=failed&reason=...）
function handleCallbackQuery() {
  const result = route.query.wechat as string | undefined;
  if (!result) return;
  if (result === 'bound') {
    ElMessage.success('微信绑定成功');
    sessionStorage.removeItem(BIND_STATE_KEY);
  } else if (result === 'failed') {
    const reason = (route.query.reason as string) || '';
    ElMessage.error(FAIL_REASONS[reason] || '微信绑定失败，请重试');
  }
}

onMounted(() => {
  handleCallbackQuery();
  loadStatus();
});
</script>

<style scoped>
.page-container { max-width: 720px; margin: 0 auto; padding: 24px 20px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px; flex-wrap: wrap; gap: 12px; }
.page-header h2 { margin: 0; font-size: 1.4rem; }
.subtitle { color: var(--text-secondary); margin: 4px 0 0; font-size: 0.85rem; max-width: 560px; }
.bind-body { min-height: 160px; }
.bind-row { display: flex; align-items: center; gap: 14px; }
.bind-meta { display: flex; flex-direction: column; gap: 6px; align-items: flex-start; }
.bind-name { font-weight: 600; font-size: 1rem; }
.bind-hint { color: var(--text-secondary); font-size: 0.82rem; margin: 12px 0 0; }

@media (max-width: 640px) {
  .page-container { padding: 16px 12px; }
}
</style>
