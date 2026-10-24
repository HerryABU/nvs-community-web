<template>
  <div class="admin-page" v-loading="loading">
    <!-- 接口异常常驻提示：本页没有 AdminHeader，靠这一行拿到全局错误反馈 -->
    <AdminApiAlert />
    <SectionHeader title="AI 评论机器人" subtitle="评论区 @ai 召唤回复 · 限流防刷 · 发起人付费" />

    <el-card shadow="never" class="cfg-card">
      <el-form label-width="140px" style="max-width: 620px">
        <el-form-item label="启用机器人">
          <el-switch v-model="form.enabled" />
          <span class="muted small" style="margin-left: 10px">关闭后评论区 @ai 无任何效果</span>
        </el-form-item>
        <el-form-item label="每用户每日召唤">
          <el-radio-group v-model="limitMode">
            <el-radio-button value="limited">限制次数</el-radio-button>
            <el-radio-button value="unlimited">不限</el-radio-button>
            <el-radio-button value="off">禁止</el-radio-button>
          </el-radio-group>
          <el-input-number
            v-if="limitMode === 'limited'"
            v-model="form.daily_limit"
            :min="1" :max="100" :step="1"
            controls-position="right" style="margin-left: 10px"
          />
          <span class="muted small" style="margin-left: 10px">次 / 日（防刷）</span>
        </el-form-item>
        <el-form-item label="默认人设">
          <el-select v-model="form.agent_id" clearable placeholder="通用写作助手（不选智能体时）" style="width: 260px">
            <el-option v-for="a in agents" :key="a.id" :label="`${a.emoji} ${a.name}`" :value="a.id" />
          </el-select>
          <span class="muted small" style="margin-left: 10px">召唤语 @智能体名 可临时覆盖</span>
        </el-form-item>
        <el-form-item label="回复落款">
          <el-input v-model="form.signature" placeholder="如：—— AI 助手" maxlength="64" style="width: 260px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">保存（立即生效）</el-button>
        </el-form-item>
      </el-form>
      <p class="hint-text">
        用户在评论中输入 <b>@ai</b>（或 <b>@智能体名</b>）即可召唤 AI 回复：异步生成、以「AI 助手」身份
        挂靠同楼发布。费用由<b>召唤人</b>承担（免费额度优先 → 代币预检，余额不足时静默跳过）；
        同一楼 60 秒内仅触发一次。回复末尾按配置附加落款与本次消耗。
      </p>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue';
import { ElMessage } from 'element-plus';
import SectionHeader from '@/components/common/SectionHeader.vue';
import { aiApi } from '@/api/ai';
import { aiHubApi } from '@/api/aiHub';
import AdminApiAlert from '@/components/admin/AdminApiAlert.vue';

const loading = ref(false);
const saving = ref(false);
const agents = ref<{ id: number; name: string; emoji: string }[]>([]);

const form = reactive({ enabled: false, daily_limit: 3, agent_id: 0, signature: '—— AI 助手' });
const limitMode = ref<'limited' | 'unlimited' | 'off'>('limited');

watch(limitMode, (m) => {
  form.daily_limit = m === 'unlimited' ? -1 : m === 'off' ? 0 : 3;
});

async function load() {
  loading.value = true;
  try {
    const [cfg, ag] = await Promise.all([aiApi.adminGetBotConfig(), aiHubApi.adminListAgents()]);
    if (cfg.data.code === 0) {
      form.enabled = cfg.data.data.enabled;
      form.daily_limit = cfg.data.data.config.daily_limit;
      form.agent_id = cfg.data.data.config.agent_id;
      form.signature = cfg.data.data.config.signature || '';
      limitMode.value = form.daily_limit < 0 ? 'unlimited' : form.daily_limit === 0 ? 'off' : 'limited';
    }
    if (ag.data.code === 0) agents.value = ag.data.data.list || [];
  } catch { /* ignore */ }
  loading.value = false;
}

async function save() {
  saving.value = true;
  try {
    const res = await aiApi.adminSetBotConfig({
      enabled: form.enabled,
      daily_limit: form.daily_limit,
      agent_id: form.agent_id,
      signature: form.signature.trim(),
    });
    if (res.data.code === 0) ElMessage.success('已保存（立即生效）');
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败');
  }
  saving.value = false;
}

onMounted(load);
</script>

<style scoped>
.cfg-card { border-radius: 10px; }
.hint-text { color: var(--text-light, #888); font-size: 0.82rem; line-height: 1.7; }
.muted { color: var(--text-light, #999); }
.small { font-size: 0.78rem; }
</style>
