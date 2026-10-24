#!/usr/bin/env node
/**
 * 审计「模板里用了、脚本里没定义」的组件标签。
 *
 * 为什么需要：本项目 Element Plus 走按需导入（unplugin-vue-components），
 * 只有 `El*` 前缀会被自动解析；`<ArrowDown />`、`<Sort />` 这类图标组件
 * 必须显式 `import { ArrowDown } from '@element-plus/icons-vue'`。
 * 漏导入时 Vue 只会 `console.warn` 并渲染成一个空的自定义元素 —— 页面不报错、
 * 构建照样成功，但图标/组件静默消失，极难发现（Reader 白屏那次是它的加强版）。
 *
 * 用法：
 *   node scripts/audit-unresolved-components.mjs           # 报告
 *   node scripts/audit-unresolved-components.mjs --strict  # 有问题则退出码 1
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// 以脚本自身位置定位仓库根（不依赖 cwd，才能在 web/ 或仓库根下都跑对）
const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const ROOT = path.join(REPO_ROOT, 'web/src');
const AUTO_DTS = path.join(REPO_ROOT, 'web/components.d.ts');
const STRICT = process.argv.includes('--strict');

// Vue 内置组件：无需导入
const BUILTIN = new Set([
  'Transition', 'TransitionGroup', 'KeepAlive', 'Teleport', 'Suspense',
  'Component', 'Slot', 'Template', 'RouterView', 'RouterLink',
]);

// 自动导入的 Element Plus 组件前缀（unplugin ElementPlusResolver 负责）
const autoImported = new Set();
if (fs.existsSync(AUTO_DTS)) {
  const dts = fs.readFileSync(AUTO_DTS, 'utf8');
  // components.d.ts: `ElButton: typeof import('element-plus/es')['ElButton']`
  for (const m of dts.matchAll(/^\s{2,}(\w+):\s*typeof import/gm)) autoImported.add(m[1]);
}

function walk(dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (e.name.endsWith('.vue')) out.push(p);
  }
  return out;
}

// 只保留模板区（第一个 <template> ... 最后一个 </template>）
function templateOf(src) {
  const start = src.indexOf('<template');
  const end = src.lastIndexOf('</template>');
  if (start < 0 || end < 0) return '';
  return src.slice(src.indexOf('>', start) + 1, end);
}

// 脚本区（<script ...> ... </script>）
function scriptOf(src) {
  const m = src.match(/<script[^>]*>([\s\S]*?)<\/script>/);
  return m ? m[1] : '';
}

const problems = [];
for (const file of walk(ROOT)) {
  const src = fs.readFileSync(file, 'utf8');
  const tpl = templateOf(src);
  const script = scriptOf(src);
  if (!tpl) continue;

  // 模板中的 PascalCase 标签（含自闭合）
  const tags = new Set();
  for (const m of tpl.matchAll(/<([A-Z][A-Za-z0-9]*)[\s/>]/g)) tags.add(m[1]);

  for (const tag of tags) {
    if (BUILTIN.has(tag) || autoImported.has(tag)) continue;
    // 脚本里出现过同名标识（import / const / components 注册）即认为已定义
    const declared = new RegExp(`(?:^|[^\\w$])${tag}(?:[^\\w$]|$)`).test(script);
    if (!declared) {
      const rel = path.relative(REPO_ROOT, file).replace(/\\/g, '/');
      problems.push(`${rel}\t<${tag}>`);
    }
  }
}

if (problems.length === 0) {
  console.log('✅ 未发现「模板使用但脚本未定义」的组件');
  process.exit(0);
}

console.log(`❌ 发现 ${problems.length} 处模板使用了未定义的组件：\n`);
for (const p of problems) console.log('  ' + p);
console.log('\n修法：在 <script setup> 里显式导入（图标来自 @element-plus/icons-vue）。');
if (STRICT) process.exit(1);
