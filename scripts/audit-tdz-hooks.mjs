#!/usr/bin/env node
/**
 * 审计：<script setup> 里的「暂时性死区（TDZ）」致命错误。
 *
 * 背景：Vue SFC 的 <script setup> 顶层是**按书写顺序执行**的。
 * `watch(树状图配置, cb)` 的第一个参数会**在调用处立即求值**，
 * 若该 const 声明在后面（const 的 TDZ），setup 阶段直接抛
 * `ReferenceError: Cannot access 'x' before initialization`，
 * 整个子组件不渲染 → 页面只剩外壳、整页白屏。
 * 而 vite build 完全无感（构建期不执行 setup），只有真浏览器才暴露。
 *
 * 检测规则（只报「顶层立即求值」的用法，函数体内的延迟引用是安全的）：
 *   1. 收集文件内所有顶层 const/let 声明及其行号；
 *   2. 找出顶层 `watch(<标识符>` / `watch([<标识符>` / `watchEffect(`；
 *   3. 若被引用标识符的声明行号 > 使用行号 ⇒ TDZ 风险。
 *
 * 用法：node scripts/audit-tdz-hooks.mjs [--strict]
 *   --strict：有命中时以退出码 1 结束（用于 CI）
 */
import fs from 'node:fs';
import path from 'node:path';

const ROOT = process.cwd();
const SRC = path.join(ROOT, 'web', 'src');
const STRICT = process.argv.includes('--strict');

/** 递归收集 .vue 文件 */
function collectVue(dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) collectVue(p, out);
    else if (e.name.endsWith('.vue')) out.push(p);
  }
  return out;
}

/** 取出 <script setup> 段（若没有则返回 null） */
function scriptSetup(src) {
  const m = src.match(/<script\s+setup[^>]*>([\s\S]*?)<\/script>/i);
  return m ? m[1] : null;
}

/** 顶层声明的标识符 → 行号（相对 script 段） */
function topLevelDecls(code) {
  const decls = new Map();
  const lines = code.split(/\r?\n/);
  lines.forEach((ln, i) => {
    // 只认「顶层」声明：行首无缩进
    const m = ln.match(/^(?:export\s+)?(?:const|let)\s+([A-Za-z_$][\w$]*)/);
    if (m) {
      // 排除解构声明 const { a, b } = ...
      if (!/^(?:export\s+)?(?:const|let)\s*\{/.test(ln)) decls.set(m[1], i);
    }
  });
  return decls;
}

const findings = [];
const files = collectVue(SRC);
for (const f of files) {
  const src = fs.readFileSync(f, 'utf8');
  const code = scriptSetup(src);
  if (!code) continue;
  const decls = topLevelDecls(code);
  const lines = code.split(/\r?\n/);
  lines.forEach((ln, i) => {
    // 顶层 watch(标识符  /  watch([标识符
    const m = ln.match(/^watch\s*\(\s*\[?\s*([A-Za-z_$][\w$]*)/);
    if (!m) return;
    const name = m[1];
    const at = decls.get(name);
    if (at !== undefined && at > i) {
      findings.push({
        file: path.relative(ROOT, f),
        line: i + 1,
        name,
        declLine: at + 1,
        code: ln.trim().slice(0, 120),
      });
    }
  });
}

if (findings.length) {
  console.log(`发现 ${findings.length} 处 TDZ 风险（顶层 watch 首参引用了尚未声明的 const）：\n`);
  for (const x of findings) {
    console.log(`  ${x.file}:${x.line}  watch(${x.name}) → ${x.name} 声明在第 ${x.declLine} 行`);
    console.log(`      ${x.code}`);
  }
  console.log('\n修法：把该 watch 移到被引用 const 的声明之后（或改用 getter 延迟取值）。');
} else {
  console.log(`✔ 未发现顶层 TDZ 风险（扫描 ${files.length} 个 .vue）`);
}
if (STRICT && findings.length) process.exit(1);
