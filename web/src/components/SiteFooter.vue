<template>
  <footer v-if="icp || psb || siteName" class="site-footer">
    <div class="footer-inner">
      <span v-if="siteName">{{ siteName }}</span>
      <a v-if="icp" href="https://beian.miit.gov.cn" target="_blank" rel="noopener" class="footer-link">{{ icp }}</a>
      <a v-if="psb && psbLink" :href="psbLink" target="_blank" rel="noopener" class="footer-link">{{ psb }}</a>
      <a v-else-if="psb" href="https://beian.cyberpolice.cn" target="_blank" rel="noopener" class="footer-link">{{ psb }}</a>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { publicApi } from '@/api/admin';

// 站点页脚（176号令 第七条第一项：联网备案号自动展示）
const siteName = ref('');
const icp = ref('');
const psb = ref('');
const psbLink = ref('');

onMounted(async () => {
  try {
    const res = await publicApi.getSiteInfo();
    const d = res.data?.data || {};
    siteName.value = d.site_name || '';
    icp.value = d.site_icp || '';
    psb.value = d.site_psb || '';
    psbLink.value = d.site_psb_link || '';
  } catch { /* 静默 */ }
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
  gap: 6px 18px;
  font-size: 0.82rem;
  color: var(--text-light, #888);
}
.footer-link {
  color: var(--text-light, #888);
  text-decoration: none;
}
.footer-link:hover {
  color: var(--el-color-primary);
  text-decoration: underline;
}
[data-theme="dark"] .site-footer { background: #0f172a; border-color: #1e293b; }
</style>
