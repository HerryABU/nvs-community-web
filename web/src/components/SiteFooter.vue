<template>
  <footer class="site-footer">
    <div class="footer-inner">
      <span v-if="siteName" class="site-name">{{ siteName }}</span>
      <a v-if="icp" href="https://beian.miit.gov.cn" target="_blank" rel="noopener" class="footer-link">{{ icp }}</a>
      <a v-if="psb && psbLink" :href="psbLink" target="_blank" rel="noopener" class="footer-link">{{ psb }}</a>
      <a v-else-if="psb" href="https://beian.cyberpolice.cn" target="_blank" rel="noopener" class="footer-link">{{ psb }}</a>
      <!-- 开源署名（参考 alist「Powered by」风格）：由 NVS 驱动 + 仓库链接 -->
      <span class="powered">
        由
        <a href="https://github.com/HerryABU/nvs-community-web" target="_blank" rel="noopener" class="footer-link">
          <strong>NVS</strong>
        </a>
        驱动
        <span class="ver">v{{ appVersion }}</span>
      </span>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { storeToRefs } from 'pinia';
import { useAppStore } from '@/stores/app';
// 版本号与 package.json 保持一致（单一来源，构建时注入），与后端 /api/health version 同步
import pkg from '../../package.json';

// 站点页脚（176号令 第七条第一项：联网备案号自动展示 + 开源署名）
const appVersion = pkg.version;
const appStore = useAppStore();
const { siteName, siteInfo } = storeToRefs(appStore);

// 备案信息来自应用级聚合 batch（/api/site-info），不再单独发请求
const icp = computed(() => siteInfo.value?.site_icp || '');
const psb = computed(() => siteInfo.value?.site_psb || '');
const psbLink = computed(() => siteInfo.value?.site_psb_link || '');

onMounted(() => {
  appStore.bootstrap(); // 幂等
});
</script>

<style scoped>
.site-footer {
  border-top: 1px solid var(--border-color, #e5e7eb);
  background: var(--bg-card, #fff);
  padding: 14px 24px;
  margin-top: 32px;
}
.footer-inner {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: center;
  gap: 6px 18px;
  font-size: 0.82rem;
  color: var(--text-light, #888);
}
.site-name {
  font-weight: 600;
  color: var(--text-color, #333);
}
.powered {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.powered .ver {
  font-size: 0.75rem;
  opacity: 0.7;
}
.footer-link {
  color: var(--text-light, #888);
  text-decoration: none;
}
.footer-link strong {
  color: var(--el-color-primary, #5b5fe3);
  font-weight: 600;
}
.footer-link:hover {
  color: var(--el-color-primary);
  text-decoration: underline;
}
[data-theme="dark"] .site-footer { background: #0f172a; border-color: #1e293b; }
</style>
