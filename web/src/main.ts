import { createApp } from 'vue';
import { createPinia } from 'pinia';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';

// Markdown 编辑器
import VMdEditor from '@kangc/v-md-editor';
import '@kangc/v-md-editor/lib/style/base-editor.css';
import vuepressTheme from '@kangc/v-md-editor/lib/theme/vuepress.js';
import '@kangc/v-md-editor/lib/theme/style/vuepress.css';

// 插件：LaTeX 数学公式
import createKatexPlugin from '@kangc/v-md-editor/lib/plugins/katex/npm';
// 插件：Mermaid 流程图
import createMermaidPlugin from '@kangc/v-md-editor/lib/plugins/mermaid/npm';
// 插件：行号
import createLineNumberPlugin from '@kangc/v-md-editor/lib/plugins/line-number/index';
// 插件：复制代码
import createCopyCodePlugin from '@kangc/v-md-editor/lib/plugins/copy-code/index';
// 插件：高亮行
import createHighlightLinesPlugin from '@kangc/v-md-editor/lib/plugins/highlight-lines/index';
// 插件：Emoji
import createEmojiPlugin from '@kangc/v-md-editor/lib/plugins/emoji/index';
// 插件：提示块
import createTipPlugin from '@kangc/v-md-editor/lib/plugins/tip/index';
// 插件：任务列表
import createTodoListPlugin from '@kangc/v-md-editor/lib/plugins/todo-list/index';

import Prism from 'prismjs';
import VMdPreview from '@kangc/v-md-editor/lib/preview';
// 代码高亮语言扩展
import 'prismjs/components/prism-python';
import 'prismjs/components/prism-java';
import 'prismjs/components/prism-c';
import 'prismjs/components/prism-cpp';
import 'prismjs/components/prism-go';
import 'prismjs/components/prism-rust';
import 'prismjs/components/prism-typescript';
import 'prismjs/components/prism-bash';
import 'prismjs/components/prism-sql';
import 'prismjs/components/prism-yaml';
import 'prismjs/components/prism-json';
import 'prismjs/components/prism-markdown';
import 'prismjs/components/prism-latex';

// ECharts
import ECharts from 'vue-echarts';
import { use } from 'echarts/core';
import { CanvasRenderer } from 'echarts/renderers';
import { LineChart, BarChart, PieChart, GaugeChart } from 'echarts/charts';
import { GridComponent, TooltipComponent, LegendComponent, TitleComponent } from 'echarts/components';

// 注册 ECharts 必需组件
use([CanvasRenderer, LineChart, BarChart, PieChart, GaugeChart, GridComponent, TooltipComponent, LegendComponent, TitleComponent]);

import App from './App.vue';
import router from './router';
import './styles/global.css';

// Katex CSS
import 'katex/dist/katex.min.css';
// Copy code CSS
import '@kangc/v-md-editor/lib/plugins/copy-code/copy-code.css';
// Tip CSS
import '@kangc/v-md-editor/lib/plugins/tip/tip.css';
// Mermaid CSS
import '@kangc/v-md-editor/lib/plugins/mermaid/mermaid.css';

// 使用主题（扩展 markdown-it 以支持更多语法）
import markdownItMark from 'markdown-it-mark';

VMdEditor.use(vuepressTheme, {
  Prism,
  extend(md) {
    // 支持 ==高亮文本== 语法
    md.use(markdownItMark);

    // ====== 安全增强：img 标签 XSS 防护 ======
    // 重写图片渲染器，移除危险属性（onerror/onload 等事件处理器）
    const defaultImageRender = md.renderer.rules.image || function (tokens, idx, options, env, self) {
      return self.renderToken(tokens, idx, options);
    };
    md.renderer.rules.image = function (tokens, idx, options, env, self) {
      const token = tokens[idx];
      // 只保留安全的 src、alt、title 属性
      const src = token.attrGet('src') || '';
      const alt = token.attrGet('alt') || '';
      const title = token.attrGet('title') || '';

      // 过滤 javascript: 和 data: 协议（data: 在 Markdown 上下文中高危）
      let safeSrc = src;
      if (/^\s*javascript\s*:/i.test(safeSrc)) {
        safeSrc = '';
      }

      // 构建安全的 img 标签
      let result = '<img';
      if (safeSrc) result += ` src="${safeSrc.replace(/"/g, '&quot;')}"`;
      if (alt) result += ` alt="${alt.replace(/"/g, '&quot;')}"`;
      if (title) result += ` title="${title.replace(/"/g, '&quot;')}"`;
      result += ' loading="lazy" referrerpolicy="no-referrer"';
      result += ' />';
      return result;
    };

    // ====== 安全增强：HTML 输出净化 ======
    // 过滤渲染后 HTML 中的危险事件处理器和危险协议
    const origHtmlInline = md.renderer.rules.html_inline || function (tokens, idx, options, env, self) {
      return self.renderToken(tokens, idx, options);
    };
    md.renderer.rules.html_inline = function (tokens, idx, options, env, self) {
      const token = tokens[idx];
      let content = token.content;

      // 过滤危险事件处理器
      content = content.replace(/\s+on\w+\s*=\s*["'][^"']*["']/gi, '');
      content = content.replace(/\s+on\w+\s*=\s*[^\s>]*/gi, '');

      // 过滤 javascript: 协议
      content = content.replace(/href\s*=\s*["']\s*javascript\s*:[^"']*["']/gi, 'href="#"');
      content = content.replace(/src\s*=\s*["']\s*javascript\s*:[^"']*["']/gi, 'src=""');

      // 匹配 <kbd> 标签，自动添加 class
      if (content.match(/^<kbd/i)) {
        content = content.replace(
          /<kbd([^>]*)>/gi,
          '<kbd$1 class="kbd-key">'
        );
      }

      token.content = content;
      return origHtmlInline(tokens, idx, options, env, self);
    };

    // 同样处理 html_block
    const origHtmlBlock = md.renderer.rules.html_block || function (tokens, idx, options, env, self) {
      return self.renderToken(tokens, idx, options);
    };
    md.renderer.rules.html_block = function (tokens, idx, options, env, self) {
      const token = tokens[idx];
      let content = token.content;

      // 过滤危险事件处理器
      content = content.replace(/\s+on\w+\s*=\s*["'][^"']*["']/gi, '');
      content = content.replace(/\s+on\w+\s*=\s*[^\s>]*/gi, '');

      // 过滤危险协议
      content = content.replace(/href\s*=\s*["']\s*javascript\s*:[^"']*["']/gi, 'href="#"');
      content = content.replace(/src\s*=\s*["']\s*javascript\s*:[^"']*["']/gi, 'src=""');

      if (content.match(/<kbd/i)) {
        content = content.replace(
          /<kbd([^>]*)>/gi,
          '<kbd$1 class="kbd-key">'
        );
      }

      token.content = content;
      return origHtmlBlock(tokens, idx, options, env, self);
    };
  },
});

// 注册插件
VMdEditor.use(createKatexPlugin());
VMdEditor.use(createMermaidPlugin());
VMdEditor.use(createLineNumberPlugin());
VMdEditor.use(createCopyCodePlugin());
VMdEditor.use(createHighlightLinesPlugin());
VMdEditor.use(createEmojiPlugin());
VMdEditor.use(createTipPlugin());
VMdEditor.use(createTodoListPlugin());

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(ElementPlus);
app.use(VMdEditor);
app.use(VMdPreview);
app.component('v-chart', ECharts);
app.mount('#app');
