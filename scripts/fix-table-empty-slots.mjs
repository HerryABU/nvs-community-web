#!/usr/bin/env node
/**
 * 清理「列级 #empty 插槽」—— Element Plus 的 el-table-column **没有** empty 插槽，
 * 只有 el-table 有。早期为了给每页加四态，把 <template #empty><StateBlock/></template>
 * 复制进了每个 el-table-column 里：这些块永远不会渲染，纯死代码（AdminGate 一页 12 处）。
 * 它们还会让人误以为"空态已经处理了"，实际表格空时仍然空白。
 *
 * 用法：
 *   node scripts/fix-table-empty-slots.mjs           # 只报告
 *   node scripts/fix-table-empty-slots.mjs --write   # 实际删除
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const SRC = path.join(REPO_ROOT, 'web/src');
const WRITE = process.argv.includes('--write');

function walk(dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (e.name.endsWith('.vue')) out.push(p);
  }
  return out;
}

/** 计算 el-table-column 区块范围：从开标签到最近的闭标签（列不嵌套，安全） */
function columnSpans(src) {
  const spans = [];
  const open = /<el-table-column\b/g;
  let m;
  while ((m = open.exec(src))) {
    const close = src.indexOf('</el-table-column>', m.index);
    if (close < 0) break;
    spans.push([m.index, close + '</el-table-column>'.length]);
  }
  return spans;
}

let totalRemoved = 0;
const changed = [];

for (const file of walk(SRC)) {
  let src = fs.readFileSync(file, 'utf8');
  const before = src;
  const spans = columnSpans(src);

  // 从后往前处理，避免偏移失效
  for (const [start, end] of spans.reverse()) {
    let block = src.slice(start, end);
    let removedInBlock = 0;
    // 删除列内的 #empty 插槽（非贪婪匹配到第一个 </template>）
    block = block.replace(/\n?[ \t]*<template\s+#empty\s*>[\s\S]*?<\/template>\n?/g, (mm) => {
      // 只删包含 StateBlock / el-empty / 文本提示的空态块
      if (/StateBlock|el-empty|暂无|没有/.test(mm)) {
        removedInBlock++;
        return '\n';
      }
      return mm;
    });
    if (removedInBlock > 0) {
      src = src.slice(0, start) + block + src.slice(end);
      totalRemoved += removedInBlock;
    }
  }

  if (src !== before) {
    changed.push(`${path.relative(REPO_ROOT, file).replace(/\\/g, '/')}`);
    if (WRITE) fs.writeFileSync(file, src, 'utf8');
  }
}

if (changed.length === 0) {
  console.log('✅ 没有列级 #empty 插槽');
  process.exit(0);
}

console.log(`${WRITE ? '已清理' : '可清理'} ${totalRemoved} 处列级 #empty 插槽，涉及 ${changed.length} 个文件：`);
for (const c of changed) console.log('  - ' + c);
if (!WRITE) console.log('\n加 --write 实际执行。');
