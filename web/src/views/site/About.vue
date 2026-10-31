<template>
  <div class="page-container about-page">
    <div class="page-header">
      <h2>关于{{ siteName || '本站' }}</h2>
      <p class="subtitle">站点简介 · 备案信息 · 合规公示</p>
    </div>

    <!-- 站点简介：优先取平台配置 guidelines（后端已消毒），未配置时展示默认文案 -->
    <el-card class="about-card">
      <template #header><h3 class="about-card-title">📖 站点简介</h3></template>
      <div class="about-desc" v-html="descriptionHtml"></div>
    </el-card>

    <!-- 备案信息：ICP / 公安联网备案 / 其他备案（均读不到时不渲染对应项） -->
    <el-card v-if="hasFiling" class="about-card">
      <template #header><h3 class="about-card-title">🪪 备案信息</h3></template>
      <SiteFiling layout="stack" />
    </el-card>

    <!-- 平台合规说明（公安部令第176号：举报渠道 / 未成年人保护 / 算法公示入口） -->
    <el-card class="about-card">
      <template #header><h3 class="about-card-title">🛡️ 平台合规</h3></template>
      <ul class="about-compliance">
        <li>本平台依法履行网络信息服务提供者义务，落实公安部令第176号相关要求。</li>
        <li>如发现违法不良信息，请通过页内「举报」按钮或联系管理员反馈，我们会及时核实处理。</li>
        <li>平台设有未成年人保护模式与内容分区确认机制，过滤不适合未成年人的内容。</li>
        <li>首页推荐逻辑说明可在首页「推荐说明」弹窗中查看（算法透明度）。</li>
      </ul>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { storeToRefs } from 'pinia';
import { useAppStore } from '@/stores/app';
import SiteFiling from '@/components/site/SiteFiling.vue';

const appStore = useAppStore();
const { siteName, siteInfo } = storeToRefs(appStore);

// 站点简介：平台指南（注册页同款配置，后端 bluemonday 已消毒）→ 默认文案
const DEFAULT_DESCRIPTION = `<p>本站是一个聚焦高质量文学创作与阅读的社区平台，致力于连接创作者与读者。</p>
<p>在这里，作者可以发布作品、管理章节、与读者互动；读者可以发现好故事、参与评论、加入论坛讨论。</p>`;

const descriptionHtml = computed(() => siteInfo.value?.guidelines || DEFAULT_DESCRIPTION);

// 备案信息是否存在（与 SiteFiling 组件的渲染规则保持一致：ICP/公安/其他备案任一非空即展示）
const hasFiling = computed(() => {
  const info = siteInfo.value || {};
  return !!(info.site_icp || info.site_psb || info.site_net_culture_license || info.site_app_filing || info.site_broadcast_license || info.site_idc_license);
});

// siteInfo 由应用级 bootstrap 的 /api/batch 一并带回（幂等，多处安全调用）
onMounted(() => {
  appStore.bootstrap();
});
</script>

<style scoped>
.about-page { padding-top: 24px; padding-bottom: 40px; }
.about-card { margin-bottom: 16px; }
.about-card-title { margin: 0; font-size: 1rem; }
.about-desc { line-height: 1.9; font-size: 0.95rem; }
/* v-html 注入的后端指南 HTML 需要 :deep 才能命中 */
.about-desc :deep(h3), .about-desc :deep(h4) { color: var(--primary-color); margin: 14px 0 8px; }
.about-desc :deep(p) { margin: 8px 0; }
.about-desc :deep(ul), .about-desc :deep(ol) { padding-left: 20px; }
.about-desc :deep(li) { margin-bottom: 4px; }
.about-desc :deep(a) { color: var(--el-color-primary); }
.about-desc :deep(strong) { color: var(--text-color, #333); }
.about-compliance { margin: 0; padding-left: 20px; line-height: 2; font-size: 0.92rem; color: var(--text-light, #666); }
[data-theme="dark"] .about-compliance { color: #94a3b8; }
</style>
