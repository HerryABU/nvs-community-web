import { createApp } from 'vue';
import { createPinia } from 'pinia';

import App from './App.vue';
import router from './router';
import './styles/global.css';
import './styles/admin.css';

// ====== Element Plus 命令式组件样式（按需引入模式下必须显式引入） ======
// 模板组件（<el-dialog> 等）由 unplugin-vue-components 自动注入样式；
// 但代码中显式 `import { ElMessage } from 'element-plus'` 的 API 组件
// 不会被解析器处理，需在此全局引入其样式，否则弹窗无样式（裸文本）。
import 'element-plus/es/components/message/style/css';
import 'element-plus/es/components/message-box/style/css';
import 'element-plus/es/components/notification/style/css';
import 'element-plus/es/components/loading/style/css';

// ====== 创建并挂载应用（核心，不依赖重量级组件） ======
try {
  const app = createApp(App);
  (window as any).__nvs_app = app; // 供动态导入的模块注册全局组件
  app.use(createPinia());
  app.use(router);
  app.mount('#app');
} catch (e: any) {
  console.error('[NVS] 应用初始化失败:', e);
  const el = document.getElementById('app');
  if (el) {
    el.innerHTML = '<div style="padding:40px;font-family:sans-serif;color:#ef4444;background:#fef2f2;border:2px solid #fecaca;border-radius:12px;margin:40px;text-align:center">'
      + '<h2 style="margin-bottom:12px">⚠️ 应用初始化失败</h2>'
      + '<pre style="white-space:pre-wrap;word-break:break-all;text-align:left;background:#fff;padding:16px;border-radius:8px;max-height:400px;overflow:auto;font-size:13px">'
      + (e?.stack || e?.message || String(e))
      + '</pre></div>';
  }
}

// ====== 异步加载非核心组件（失败不影响首页） ======

// Markdown 编辑器 & 预览器（Cherry Markdown）
// CSS 通过 index.ts 的 side-effect import 自动加载
import('./markdown').catch(e => {
  console.warn('[NVS] Markdown 模块加载失败:', e);
});

// ECharts（vue-echarts）
import('vue-echarts').then(({ default: ECharts }) => {
  return import('echarts/core').then(({ use }) => {
    return Promise.all([
      import('echarts/renderers'),
      import('echarts/charts'),
      import('echarts/components'),
    ]).then(([renderers, charts, components]) => {
      use([
        renderers.CanvasRenderer,
        charts.LineChart, charts.BarChart, charts.PieChart, charts.GaugeChart, charts.GraphChart,
        components.GridComponent, components.TooltipComponent, components.LegendComponent, components.TitleComponent,
      ]);
      // 从已挂载的实例注册全局组件
      const app = (window as any).__nvs_app;
      if (app) app.component('v-chart', ECharts);
    });
  });
}).catch(e => {
  console.warn('[NVS] ECharts 加载失败，统计图表不可用:', e);
});
