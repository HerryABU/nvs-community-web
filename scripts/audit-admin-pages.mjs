#!/usr/bin/env node
/**
 * 后台管理页「列表基础能力」审计
 *
 * 为什么要有这个脚本：
 * 50 个后台入口是逐年加出来的，同一个后台里表格密度、空态、错误处理各写各的。
 * 靠人肉复查必然漏，做成脚本才能看出"还剩多少页没统一"，也能防止新页面又退回旧写法。
 *
 * 用法：
 *   node scripts/audit-admin-pages.mjs                # 打印表格
 *   node scripts/audit-admin-pages.mjs --md           # 输出 markdown（可直接贴进文档）
 *   node scripts/audit-admin-pages.mjs --strict       # 有页面未统一时退出码 1（供 CI 用）
 */
import fs from 'node:fs';
import path from 'node:path';

const DIR = 'web/src/views/admin';
const args = process.argv.slice(2);
const asMarkdown = args.includes('--md');
const strict = args.includes('--strict');

if (!fs.existsSync(DIR)) {
  console.error(`目录不存在：${DIR}（请在仓库根目录执行）`);
  process.exit(2);
}

const files = fs.readdirSync(DIR).filter((f) => f.endsWith('.vue')).sort();

const rows = files.map((f) => {
  const s = fs.readFileSync(path.join(DIR, f), 'utf-8');
  const tables = (s.match(/<el-table\b/g) || []).length;
  return {
    name: f.replace(/\.vue$/, ''),
    tables,
    // 四态：StateBlock 组件（三态统一）、或至少给表格写了 #empty 插槽
    stateBlock: s.includes('StateBlock'),
    emptySlot: (s.match(/#empty/g) || []).length,
    elEmpty: (s.match(/el-empty/g) || []).length,
    skeleton: /el-skeleton|hms-skeleton|skeleton=/.test(s),
    loading: (s.match(/v-loading/g) || []).length,
    pagination: (s.match(/el-pagination/g) || []).length,
    filters: (s.match(/el-select|el-input|el-date-picker|el-radio-group/g) || []).length,
    // 错误提示：页面自己接了 loadError/error 态，或者用 AdminHeader
    // （AdminHeader 内置了全局「后台接口异常」横幅，axios 拦截器统一登记 5xx/网络错误）
    errorState: /loadError|state="error"/.test(s),
    globalBanner: /AdminHeader/.test(s),
    silentCatch: /catch\s*(\(\s*\w*\s*\))?\s*\{\s*(\/\*[^*]*\*\/)?\s*\}/.test(s),
  };
});

const n = rows.length;
const pct = (k) => `${Math.round((rows.filter((r) => r[k]).length / n) * 100)}%`;

if (asMarkdown) {
  console.log('| 页面 | 表格 | StateBlock | #empty 插槽 | 骨架 | v-loading | 分页 | 筛选项 | 错误态 |');
  console.log('|---|---:|---:|---:|---:|---:|---:|---:|:--:|');
  for (const r of rows) {
    console.log(
      `| ${r.name} | ${r.tables} | ${r.stateBlock ? '✓' : '—'} | ${r.emptySlot} | ${r.skeleton ? '✓' : '—'} | ${r.loading} | ${r.pagination} | ${r.filters} | ${r.errorState ? '页面内' : r.globalBanner ? '全局横幅' : '—'} |`,
    );
  }
  console.log('');
  console.log(`合计 ${n} 页；StateBlock ${pct('stateBlock')}；骨架 ${pct('skeleton')}；错误提示 ${Math.round((rows.filter((r) => r.errorState || r.globalBanner).length / n) * 100)}%`);
} else {
  const pad = (v, w) => String(v).padEnd(w);
  console.log(
    pad('页面', 26) + pad('表格', 6) + pad('StateBlock', 12) + pad('#empty', 8) + pad('骨架', 6) + pad('loading', 9) + pad('分页', 6) + pad('筛选', 6) + '错误态',
  );
  console.log('-'.repeat(96));
  for (const r of rows) {
    console.log(
      pad(r.name, 26) +
        pad(r.tables, 6) +
        pad(r.stateBlock ? 'yes' : '-', 12) +
        pad(r.emptySlot, 8) +
        pad(r.skeleton ? 'yes' : '-', 6) +
        pad(r.loading, 9) +
        pad(r.pagination, 6) +
        pad(r.filters, 6) +
        (r.errorState ? 'page' : r.globalBanner ? 'banner' : '-'),
    );
  }
  console.log('-'.repeat(96));
  console.log(
    `共 ${n} 页｜StateBlock ${pct('stateBlock')}｜骨架 ${pct('skeleton')}｜错误提示 ${Math.round((rows.filter((r) => r.errorState || r.globalBanner).length / n) * 100)}%｜表格总数 ${rows.reduce((a, r) => a + r.tables, 0)}`,
  );

  const needFix = rows.filter((r) => r.tables > 0 && !r.stateBlock && r.emptySlot === 0);
  if (needFix.length) {
    console.log('\n【优先修】有表格但既没 StateBlock 也没 #empty 插槽：');
    for (const r of needFix) console.log(`  - ${r.name}（${r.tables} 张表）`);
  }
  const silent = rows.filter((r) => r.silentCatch && !r.errorState);
  if (silent.length) {
    console.log('\n【静默失败】catch 里什么都没做、也没有错误态：');
    for (const r of silent) console.log(`  - ${r.name}`);
  }
}

if (strict) {
  const bad = rows.filter((r) => r.tables > 0 && !r.stateBlock && r.emptySlot === 0);
  if (bad.length) process.exit(1);
}
