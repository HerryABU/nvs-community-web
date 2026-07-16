<template>
  <div class="page-container">
    <div class="page-header">
      <div class="header-left">
        <h2>⚙️ WASM 扩展说明</h2>
        <p class="subtitle">WebAssembly 模块通过「扩展HTML应用」的ZIP包上传，与HTML/CSS/JS一起打包运行</p>
      </div>
    </div>

    <el-card shadow="hover" style="margin-bottom:20px">
      <template #header><span>📦 如何部署 WASM 应用</span></template>
      <el-steps :active="4" align-center finish-status="success">
        <el-step title="编写应用" description="HTML + JS + .wasm" />
        <el-step title="打包ZIP" description="将所有文件放入ZIP" />
        <el-step title="上传" description="扩展HTML → 上传ZIP" />
        <el-step title="沙盒运行" description="iframe隔离 + WASM支持" />
      </el-steps>
    </el-card>

    <el-row :gutter="20" style="margin-bottom:24px">
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header><span>🛡️ 安全策略</span></template>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="iframe sandbox">allow-scripts allow-same-origin allow-forms</el-descriptions-item>
            <el-descriptions-item label="CSP">'wasm-unsafe-eval' 启用</el-descriptions-item>
            <el-descriptions-item label="COEP">require-corp（WASM共享内存）</el-descriptions-item>
            <el-descriptions-item label="ZIP限制">≤20MB 压缩 · ≤50MB 解压 · ≤100:1 压缩比 · ≤500文件</el-descriptions-item>
            <el-descriptions-item label="文件类型白名单">html/css/js/wasm/json/svg/图片/字体</el-descriptions-item>
            <el-descriptions-item label="路径穿越">自动检测拦截</el-descriptions-item>
            <el-descriptions-item label="ZIP炸弹">压缩比 > 100:1 自动拒绝</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header><span>🚀 快速开始示例</span></template>
          <p style="color:var(--text-secondary);margin-bottom:12px">典型的 ZIP 包结构：</p>
          <pre class="zip-struct">my-app.zip
├── index.html          ← 入口HTML
├── style.css
├── app.js
├── module.wasm         ← WASM模块
└── assets/
    ├── icon.png
    └── data.json</pre>
          <el-button type="primary" style="margin-top:12px" @click="$router.push('/htmls')" :icon="Upload">
            去上传ZIP
          </el-button>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { Upload } from '@element-plus/icons-vue';
</script>

<style scoped>
.page-container { max-width: 1100px; margin: 0 auto; padding: 24px 20px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; }
.header-left h2 { margin: 0; font-size: 1.4rem; }
.subtitle { color: var(--text-secondary); margin: 4px 0 0; font-size: 0.9rem; }
.zip-struct { background: var(--card-bg); border: 1px solid var(--border-color); border-radius: 8px; padding: 14px; font-family: 'SF Mono', 'Fira Code', monospace; font-size: 0.85rem; line-height: 1.7; overflow-x: auto; }
</style>
