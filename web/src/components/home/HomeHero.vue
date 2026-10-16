<template>
  <!-- ====== 极简 Hero（F15：从头图区块渲染） ====== -->
  <div class="hero-section">
    <div class="hero-bg"></div>
    <div class="hero-glass">
      <h1>欢迎来到 {{ siteName || '星海文学' }}</h1>
      <p>发现好故事，连接创作者与读者的文学社区</p>
      <div v-if="showCategories" class="hero-quick-links">
        <span v-for="cat in categories.slice(0, 4)" :key="cat"
          class="hero-cat-chip" @click="$router.push(`/category/${encodeURIComponent(cat)}`)">
          {{ cat }}
        </span>
        <span class="hero-cat-chip hero-cat-more" @click="$router.push('/forums')">论坛社区</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  siteName: string;
  categories: string[];
  showCategories?: boolean;
}>();
</script>

<style scoped>
.hero-section {
  position: relative;
  height: 240px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  margin-bottom: 32px;
}
.hero-bg {
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, #e8ecf1 0%, #dce3ed 50%, #cfd9e8 100%);
  z-index: 0;
}
.hero-bg::after {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse at 50% 80%, rgba(99,102,241,.12) 0%, transparent 60%),
              radial-gradient(ellipse at 20% 20%, rgba(139,92,246,.08) 0%, transparent 50%);
}
.hero-glass {
  position: relative;
  z-index: 1;
  text-align: center;
  padding: 32px 40px;
  background: rgba(255,255,255,.55);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(0,0,0,.06);
  border-radius: var(--radius-lg);
  max-width: 640px;
  width: 90%;
  box-shadow: 0 4px 24px rgba(0,0,0,.04);
}
.hero-glass h1 {
  font-size: 2rem;
  font-weight: 300;
  color: var(--primary-color);
  margin-bottom: 6px;
  letter-spacing: -.5px;
}
.hero-glass p {
  font-size: .95rem;
  color: var(--text-light);
  margin-bottom: 16px;
  font-weight: 300;
}
.hero-quick-links {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: center;
}
.hero-cat-chip {
  padding: 5px 16px;
  border-radius: 20px;
  font-size: .82rem;
  font-weight: 500;
  cursor: pointer;
  transition: all .2s;
  background: rgba(99,102,241,.08);
  color: #4f46e5;
  border: 1px solid rgba(99,102,241,.2);
}
.hero-cat-chip:hover {
  background: rgba(99,102,241,.16);
  border-color: rgba(99,102,241,.35);
}
.hero-cat-more {
  background: rgba(99,102,241,.16);
  border-color: rgba(99,102,241,.35);
  color: #4338ca;
}

/* ====== Dark Mode Hero ====== */
[data-theme="dark"] .hero-bg {
  background: linear-gradient(135deg, #0f1117 0%, #131620 50%, #0f172a 100%);
}
[data-theme="dark"] .hero-bg::after {
  background: radial-gradient(ellipse at 50% 80%, rgba(99,102,241,.18) 0%, transparent 60%),
              radial-gradient(ellipse at 20% 20%, rgba(139,92,246,.10) 0%, transparent 50%);
}
[data-theme="dark"] .hero-glass {
  background: rgba(255,255,255,.04);
  border-color: rgba(255,255,255,.08);
  box-shadow: none;
}
[data-theme="dark"] .hero-glass h1 { color: #fff; }
[data-theme="dark"] .hero-glass p { color: rgba(255,255,255,.65); }
[data-theme="dark"] .hero-cat-chip {
  background: rgba(255,255,255,.1);
  color: rgba(255,255,255,.85);
  border-color: rgba(255,255,255,.15);
}
[data-theme="dark"] .hero-cat-chip:hover {
  background: rgba(255,255,255,.22);
  border-color: rgba(255,255,255,.35);
}
[data-theme="dark"] .hero-cat-more {
  background: rgba(99,102,241,.25);
  border-color: rgba(99,102,241,.4);
  color: rgba(255,255,255,.85);
}

/* ====== 移动端适配 ====== */
@media (max-width: 768px) {
  .hero-section { height: 150px; margin-bottom: 16px; }
  .hero-glass { padding: 18px 16px; width: calc(100% - 24px); }
  .hero-glass h1 { font-size: 1.3rem; margin-bottom: 4px; }
  .hero-glass p { font-size: 0.8rem; margin-bottom: 10px; }
  .hero-cat-chip { padding: 4px 12px; font-size: 0.75rem; }
}
</style>
