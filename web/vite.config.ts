import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { resolve } from 'path';
import AutoImport from 'unplugin-auto-import/vite';
import Components from 'unplugin-vue-components/vite';
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers';
import { compression } from 'vite-plugin-compression2';

export default defineConfig({
  plugins: [
    vue(),
    // 按需自动引入 Element Plus 组件与 API，避免全量注册导致主包臃肿
    // 注：auto-imports.d.ts / components.d.ts 为插件自动生成的类型声明文件
    AutoImport({ resolvers: [ElementPlusResolver()] }),
    Components({ resolvers: [ElementPlusResolver()] }),
    // 产物压缩：gzip + brotli，显著降低传输体积（不影响功能）
    compression({ algorithm: 'gzip', threshold: 1024 }),
    compression({ algorithm: 'brotliCompress', threshold: 1024 }),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
    // 去重：确保 katex 和 mermaid 使用根目录版本，避免 Cherry 内部版本冲突
    dedupe: ['katex', 'mermaid'],
  },
  // 🔀 相对 base（顶层配置）：资源引用（首屏 js/css、动态 chunk、modulepreload）生成相对路径，
  // 由后端在 index.html 注入 <base href="/{URL_PREFIX}/"> 统一解析——
  // 支持反向代理前缀挂载（如 :8088/go-nvs/login），js/css 等静态资源同步带前缀。
  base: './',
  build: {
    outDir: '../dist',
    // ⚠️ 必须清空：outDir 在项目根之外，默认不清空会**累积历史 chunk**
    // （实测曾出现两份 vendor-mermaid 同时存在，dist 39M、内嵌 exe 从 52M 涨到 60M）。
    // 若某些受限环境拦截删除，构建前先 `mv dist dist_old_$(date +%s)` 再跑。
    emptyOutDir: true,
    // 体积告警阈值放宽（重型库本就独立分包）
    chunkSizeWarningLimit: 4000,
    rollupOptions: {
      output: {
        // 把体积大且变动少的重型库单独分包，提升浏览器缓存命中率
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('echarts')) return 'vendor-echarts';
            if (id.includes('mermaid')) return 'vendor-mermaid';
            if (id.includes('cherry-markdown')) return 'vendor-cherry';
            if (id.includes('cytoscape') || id.includes('smiles-drawer')) return 'vendor-graph';
          }
          return undefined;
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});
