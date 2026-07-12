/**
 * Markdown 编辑器 & 预览器 — 统一使用 Cherry Markdown
 *
 * Cherry Markdown 是腾讯开源的 JS Markdown 编辑器，
 * 内置 KaTeX / Mermaid / 代码高亮 / DOMPurify XSS 防护。
 *
 * 模块拆分：
 *   renderer.ts    — CherryEngine 纯渲染（Reader / 评论 / 论坛）
 *   global-fix.css — Cherry 预览区和全局 Markdown 样式
 *
 * 使用方式（在 main.ts 中）：
 *   import('./markdown').then(...)
 *
 * 编辑器组件（ChapterEditor.vue）直接 import Cherry 并挂载到 DOM，
 * 无需通过 Vue plugin 系统注册。
 */

// Cherry Markdown 完整 CSS（编辑器 + 预览）
import 'cherry-markdown/dist/cherry-markdown.min.css';

// KaTeX CSS（Cherry 引擎需要）
import 'katex/dist/katex.min.css';

// 全局 Markdown 渲染修复样式
import './global-fix.css';

// 导出渲染器（供 Reader / Comment / Thread 使用）
export { renderMarkdown, renderMermaidBlocks } from './renderer';

/**
 * Cherry 编辑器工厂函数
 *
 * 动态导入 Cherry（避免阻塞首页加载），
 * 并返回 Cherry 类和初始化方法。
 */
export async function loadCherryEditor() {
  const Cherry = (await import('cherry-markdown')).default;
  return Cherry;
}
