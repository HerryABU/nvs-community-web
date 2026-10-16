<template>
  <!-- 站点备案信息区（公安部令第176号 第七条第一项：联网备案号对外展示）
       数据来源：应用级聚合 batch 的 /api/site-info（stores/app.ts → siteInfo）
       渲染规则：ICP / 公安备案仅对应配置非空才出现；两者皆空时整个区块不渲染 -->
  <div v-if="hasFiling" class="site-filing" :class="`site-filing--${layout}`">
    <!-- ICP 备案：跳转工信部备案管理系统 -->
    <a
      v-if="icp"
      class="filing-item filing-icp"
      href="https://beian.miit.gov.cn"
      target="_blank"
      rel="noopener"
      :title="`${icp}（工业和信息化部备案管理系统）`"
    >{{ icp }}</a>

    <!-- 公安联网备案：优先站长配置的链接，为空则跳全国公安联网备案官方平台 -->
    <a
      v-if="psb"
      class="filing-item filing-psb"
      :href="psbLink"
      target="_blank"
      rel="noopener"
      :title="`${psb}（全国公安联网备案信息平台）`"
    >
      <PoliceBadge class="filing-police-icon" />
      {{ psb }}
    </a>

    <!-- 其他备案信息占位（网络文化经营许可证 / App 备案等）：
         后端 site-info 暂未下发这些字段，读到时才渲染，读不到不占位 -->
    <span v-for="f in otherFilings" :key="f.label" class="filing-item filing-other">
      {{ f.label }}：{{ f.value }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { storeToRefs } from 'pinia';
import { useAppStore } from '@/stores/app';
import PoliceBadge from './PoliceBadge.vue';

// layout: inline = 页脚横排小字号；stack = 关于页块级列表
defineProps<{ layout?: 'inline' | 'stack' }>();

const appStore = useAppStore();
const { siteInfo } = storeToRefs(appStore);

// 备案号来自应用级聚合 batch（/api/site-info），无需单独发请求
const icp = computed(() => siteInfo.value?.site_icp || '');
const psb = computed(() => siteInfo.value?.site_psb || '');
// 公安备案链接：站长未配置时落到全国公安联网备案官方平台
const psbLink = computed(() => siteInfo.value?.site_psb_link || 'https://beian.mps.gov.cn/');

// 其他备案占位：仅当 site-info 下发对应字段时才展示（后端未提供则不渲染）
const otherFilings = computed(() => {
  const info = siteInfo.value || {};
  const candidates: { key: string; label: string }[] = [
    { key: 'site_net_culture_license', label: '网络文化经营许可证' },
    { key: 'site_app_filing', label: 'App 备案' },
    { key: 'site_broadcast_license', label: '广播电视节目制作经营许可证' },
    { key: 'site_idc_license', label: 'IDC/ISP 许可证' },
  ];
  return candidates
    .map(({ key, label }) => ({ label, value: String(info[key] || '').trim() }))
    .filter((f) => f.value);
});

// ICP / 公安至少配了一项才展示备案区
const hasFiling = computed(() => !!icp.value || !!psb.value || otherFilings.value.length > 0);
</script>

<style scoped>
.site-filing {
  color: var(--text-light, #888);
}

/* 页脚横排 */
.site-filing--inline {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: center;
  gap: 4px 16px;
  font-size: 0.78rem;
}

/* 关于页块级列表 */
.site-filing--stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
  font-size: 0.92rem;
}

.filing-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--text-light, #888);
  text-decoration: none;
  line-height: 1.5;
}

.filing-item:hover {
  color: var(--el-color-primary);
  text-decoration: underline;
}

.filing-police-icon {
  flex-shrink: 0;
  color: #4a7fd4;
}
</style>
