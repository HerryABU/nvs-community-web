<template>
  <el-dialog
    :model-value="visible"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :show-close="false"
    width="540px"
    top="10vh"
  >
    <!-- 第 1 步：介绍 -->
    <div v-if="step === 1">
      <div class="sz-title">⚠️ 内容提醒</div>
      <div class="sz-body">
        <p>{{ detail.intro_text || `您即将进入「${zoneName}」分区。` }}</p>
        <p>该分区内容属于敏感题材，请确认您已做好心理准备，并愿意以开放心态阅读。</p>
      </div>
      <div style="text-align:right;margin-top:20px">
        <el-button @click="$emit('cancel')">离开</el-button>
        <el-button type="primary" @click="currentStep = 2">我已知晓，继续</el-button>
      </div>
    </div>

    <!-- 警告步骤 -->
    <div v-if="step === 2">
      <div class="sz-title sz-title-warn">🚫 警告</div>
      <div class="sz-body">
        <p>{{ currentWarning }}</p>
        <p style="margin-top:12px;color:#999;font-size:0.85rem">
          这是第 <strong>{{ currentStep }}</strong> 步，共 <strong>{{ totalSteps }}</strong> 步。
        </p>
      </div>
      <div style="text-align:right;margin-top:20px">
        <el-button @click="$emit('cancel')">离开</el-button>
        <el-button type="warning" @click="currentStep++">继续</el-button>
      </div>
    </div>

    <!-- 最后一步：输入确认 -->
    <div v-if="step === 3">
      <div class="sz-title sz-title-danger">🔴 最终确认</div>
      <div class="sz-body">
        <p>您必须手动输入以下确认语以继续：</p>
        <p class="sz-confirm-text">{{ detail.confirm_text || '我承诺承担全部阅读责任' }}</p>
        <el-input
          v-model="confirmInput"
          :placeholder="`请在此输入确认语（不可粘贴，请手动输入）`"
          size="large"
          style="margin-top:12px"
          @paste.prevent
          @drop.prevent
        />
        <p style="margin-top:8px;color:#999;font-size:0.8rem">
          输入即视为您已充分了解该分区内容的性质，并自愿承担一切法律与心理后果。
        </p>
      </div>
      <div style="text-align:right;margin-top:20px">
        <el-button @click="$emit('cancel')">离开</el-button>
        <el-button
          type="danger"
          :disabled="confirmInput !== (detail.confirm_text || '我承诺承担全部阅读责任')"
          @click="confirm"
        >
          确认进入
        </el-button>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import api from '@/api/index';

const props = defineProps<{
  visible: boolean;
  zoneName: string;
  isCrossDomain: boolean;
}>();

const emit = defineEmits<{
  confirm: [];
  cancel: [];
}>();

// 从 API 获取的 zone 详细配置
const detail = ref<{
  steps: number;
  confirm_text: string;
  warnings: string[];
  intro_text: string;
  cross_domain_extra: number;
}>({
  steps: 3,
  confirm_text: '我承诺承担全部阅读责任',
  warnings: ['该分区内容属于敏感题材。'],
  intro_text: '',
  cross_domain_extra: 2,
});

async function fetchZoneDetail() {
  try {
    const res = await api.get(`/wall-zone/${encodeURIComponent(props.zoneName)}`);
    if (res.data.code === 0) {
      const d = res.data.data;
      detail.value = {
        steps: d.steps || 3,
        confirm_text: d.confirm_text || '我承诺承担全部阅读责任',
        warnings: Array.isArray(d.warnings) && d.warnings.length > 0 ? d.warnings : ['该分区内容属于敏感题材。'],
        intro_text: d.intro_text || `您即将进入「${props.zoneName}」分区。该分区内容属于敏感题材，可能包含以下特点：`,
        cross_domain_extra: d.cross_domain_extra ?? 2,
      };
    }
  } catch { /* use defaults */ }
}

// 总步数 = zone 配置步数 + 跨域额外步数
const totalSteps = computed(() => {
  const base = detail.value.steps || 3;
  const extra = props.isCrossDomain ? (detail.value.cross_domain_extra || 2) : 0;
  return Math.min(base + extra, 7);
});

const currentStep = ref(1);
const confirmInput = ref('');

const currentWarning = computed(() => {
  const idx = currentStep.value - 2;
  if (idx >= 0 && idx < detail.value.warnings.length) {
    return detail.value.warnings[idx];
  }
  // 如果警告不够，用跨域额外警告填充
  const crossIdx = idx - detail.value.warnings.length;
  if (crossIdx >= 0 && props.isCrossDomain) {
    const crossWarnings = [
      '您正在从一个分区跨越到另一个敏感分区。请确保您理解并尊重不同分区的文化差异。',
      '跨域移动意味着您自愿承担一切潜在的法律与心理后果。',
    ];
    if (crossIdx < crossWarnings.length) return crossWarnings[crossIdx];
  }
  return '请确认您已充分理解并愿意承担相关风险。';
});

// step 计算：1=介绍, 2=警告循环, 3=最终确认
const step = computed(() => {
  if (currentStep.value === 1) return 1;
  if (currentStep.value < totalSteps.value) return 2;
  return 3;
});

function confirm() {
  emit('confirm');
}

// 弹窗打开时获取 zone 详情并重置
watch(() => props.visible, async (val) => {
  if (val) {
    currentStep.value = 1;
    confirmInput.value = '';
    await fetchZoneDetail();
  }
});
</script>

<style scoped>
.sz-title {
  font-size: 1.3rem;
  font-weight: 700;
  color: #e6a23c;
  margin-bottom: 16px;
}
.sz-title-warn {
  color: #f56c6c;
}
.sz-title-danger {
  color: #f56c6c;
}
.sz-body {
  line-height: 1.8;
  font-size: 0.95rem;
  color: #333;
}
.sz-body ul {
  padding-left: 20px;
}
.sz-body li {
  margin: 6px 0;
}
.sz-confirm-text {
  display: inline-block;
  background: #fef0f0;
  border: 1px solid #fde2e2;
  border-radius: 4px;
  padding: 6px 16px;
  font-weight: 700;
  color: #f56c6c;
  font-size: 1.1rem;
  letter-spacing: 2px;
}
</style>

<style>
[data-theme="dark"] .sz-title {
  color: #fbbf24;
}
[data-theme="dark"] .sz-title-warn {
  color: #f87171;
}
[data-theme="dark"] .sz-title-danger {
  color: #f87171;
}
[data-theme="dark"] .sz-body {
  color: #cbd5e1;
}
[data-theme="dark"] .sz-confirm-text {
  background: rgba(248, 113, 113, 0.12);
  border-color: rgba(248, 113, 113, 0.3);
  color: #f87171;
}
</style>