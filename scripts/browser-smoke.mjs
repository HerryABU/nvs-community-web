#!/usr/bin/env node
/**
 * NVS 真浏览器验收（v3.21.0 新页面）
 *
 * 为什么要脚本：`vite build` 成功 ≠ 页面能跑。本仓库踩过的坑里，
 * 白屏（模板引用未定义变量）、静默失效（图标没导入）、横向溢出（overflow 加错容器）
 * 全都是构建期无感的，只有真浏览器 + console 监听 + 几何量化才能发现。
 *
 * 覆盖页面：主页资料卡 / 作者主页分栏 / 小说详情（目录工具条+评论排序）/ 阅读器（朗读）
 * 覆盖宽度：375 / 768 / 1024 / 1440（每档都量 scrollWidth 溢出与关键元素可见性）
 *
 * 用法：
 *   node scripts/browser-smoke.mjs                # 全量
 *   node scripts/browser-smoke.mjs --only=novel   # 只跑某页
 */
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire('C:/Users/QBZ95/.workbuddy/binaries/node/workspace/');
const { chromium } = require('playwright-core');

const BASE = process.env.NVS_BASE || 'http://127.0.0.1:8000';
const COOKIE_FILE = process.env.NVS_COOKIE || '/tmp/nvs_cookie.txt';
const SHOT_DIR = process.env.NVS_SHOT_DIR || 'C:/Users/QBZ95/Desktop/tools/tests/test_go/log/shots';
const ONLY = (process.argv.find((a) => a.startsWith('--only=')) || '').split('=')[1] || '';

// 作品/作者：取自 test_go 里章节最多的作品（赤色因果律，author_id=3）
const NOVEL_ID = Number(process.env.NVS_NOVEL || 1);
const AUTHOR_ID = Number(process.env.NVS_AUTHOR || 3);
const CHAPTER = Number(process.env.NVS_CHAPTER || 1);

const WIDTHS = [375, 768, 1024, 1440];

/** 从 curl -c 的 Netscape cookie 文件解析（HttpOnly 是 #HttpOnly_ 前缀、可能带 CRLF） */
function readCookies() {
  const raw = fs.readFileSync(COOKIE_FILE, 'utf8');
  const out = [];
  for (const line of raw.split(/\r?\n/)) {
    const l = line.replace(/^#HttpOnly_/, '');
    if (!l || l.startsWith('#')) continue;
    const parts = l.split('\t');
    if (parts.length < 7) continue;
    const [domain, , p, secure, expires, name, value] = parts;
    out.push({
      name,
      value: value.replace(/\r$/, ''),
      domain: domain.replace(/^\./, ''),
      path: p || '/',
      expires: Number(expires) || -1,
      httpOnly: /^#HttpOnly_/.test(line),
      secure: secure === 'TRUE',
    });
  }
  return out;
}

/** 页面级度量 */
async function measure(page, keySelectors) {
  return page.evaluate((sels) => {
    const de = document.documentElement;
    const res = {
      title: document.title,
      url: location.pathname,
      innerWidth: window.innerWidth,
      overflowX: de.scrollWidth - window.innerWidth,
      bodyText: (document.body.innerText || '').slice(0, 160).replace(/\s+/g, ' '),
      missing: [],
      found: {},
    };
    for (const [k, sel] of Object.entries(sels)) {
      const el = document.querySelector(sel);
      if (!el) { res.missing.push(k + ' → ' + sel); continue; }
      const r = el.getBoundingClientRect();
      res.found[k] = {
        w: Math.round(r.width), h: Math.round(r.height),
        visible: r.width > 0 && r.height > 0,
        overRight: r.right > window.innerWidth + 1,
      };
    }
    return res;
  }, keySelectors);
}

const PAGES = [
  {
    key: 'decorate',
    name: '主页资料卡',
    url: '/me/decorate',
    ready: '.decorate-page',
    sels: {
      资料卡: '.dec-card',
      头像: '.prof-avatar-col .el-avatar',
      昵称输入: '.prof-fields input',
      简介框: '.prof-fields textarea',
    },
  },
  {
    key: 'author',
    name: '作者主页分栏',
    url: `/author/${AUTHOR_ID}`,
    ready: '.ah-profile',
    sels: {
      封面: '.ah-cover',
      资料头: '.ah-profile',
      分栏: '.ah-tabs',
      粉丝数: '.ah-stats',
    },
  },
  {
    key: 'novel',
    name: '小说详情',
    url: `/author/${AUTHOR_ID}/novel/${NOVEL_ID}`,
    ready: '.novel-detail',
    sels: {
      标题: '.novel-title',
      目录工具条: '.chapter-toolbar',
      章节搜索: '.ct-controls input',
      排序组: '.ct-controls .el-radio-group',
      评论工具条: '.comment-toolbar',
      评论排序: '.ct-sort',
    },
  },
  {
    key: 'reader',
    name: '阅读器（朗读）',
    url: `/author/${AUTHOR_ID}/novel/${NOVEL_ID}/read/${CHAPTER}`,
    ready: '.reader-page, .reader, .page-container',
    sels: {
      正文区: '.reader-md',
      朗读按钮: 'button:has(.el-icon)',
    },
  },
];

const results = [];
const consoleErrors = [];

(async () => {
  fs.mkdirSync(SHOT_DIR, { recursive: true });
  const cookies = readCookies();
  console.log(`cookie 数=${cookies.length}`);

  const browser = await chromium.launch({
    executablePath: process.env.NVS_CHROME || findChrome(),
    args: ['--no-proxy-server', '--disable-gpu'],
  });

  const targets = ONLY ? PAGES.filter((p) => p.key === ONLY) : PAGES;

  for (const p of targets) {
    for (const w of WIDTHS) {
      const ctx = await browser.newContext({
        viewport: { width: w, height: 900 },
        deviceScaleFactor: 1,
      });
      // Playwright 的 cookie 只接受「url」或「domain+path」二选一。
      // 实测两者同时给会抛 "Cookie should have either url or domain"，
      // 这里统一用 url（对 127.0.0.1 这类裸 IP 最稳），失败再退回 domain 形式。
      try {
        await ctx.addCookies(cookies.map((c) => ({ name: c.name, value: c.value, url: BASE })));
      } catch {
        await ctx.addCookies(
          cookies.map((c) => ({
            name: c.name,
            value: c.value,
            domain: c.domain,
            path: c.path,
            httpOnly: c.httpOnly,
            secure: c.secure,
          })),
        );
      }
      const page = await ctx.newPage();
      const errs = [];
      const warns = [];
      page.on('console', (m) => {
        if (m.type() !== 'error') return;
        const t = m.text().slice(0, 220);
        // 浏览器对「图片 404」也只给一条泛化的 console error，且不带 URL。
        // 真正的 API 失败已由下面的 response 钩子按 URL 记录，所以这里归入提示即可，
        // 否则 test_go 里那条含 ![x](x"onerror=...) 的脏评论会让每次验收都「失败」。
        if (t.startsWith('Failed to load resource')) warns.push('[console] ' + t);
        else errs.push('[console] ' + t);
      });
      page.on('pageerror', (e) => errs.push(`[pageerror] ${String(e).slice(0, 220)}`));
      // 失败的接口要连 URL 一起记：只知道「404」等于没信息。
      // 分类：/api/* 的失败算「阻断」；其它（图片等）仅记为「提示」——
      // test_go 里存着 XSS 测试数据，cover_url 会故意写成 /x"onerror=... 这种必然 404 的值。
      page.on('response', (r) => {
        if (r.status() < 400) return;
        const u = r.url().replace(BASE, '');
        const rec2 = `[http ${r.status()}] ${u}`;
        if (u.startsWith('/api/')) errs.push('API ' + rec2);
        else warns.push('ASSET ' + rec2);
      });

      let rec = { key: p.key, name: p.name, width: w, ok: false, error: '', metrics: null, errs, warns };
      try {
        await page.goto(BASE + p.url, { waitUntil: 'networkidle', timeout: 45000 });
        await page.waitForSelector(p.ready, { timeout: 20000 });
        // 给异步区块（评论/图表/装扮）一点渲染时间
        await page.waitForTimeout(1200);
        rec.metrics = await measure(page, p.sels);
        rec.ok = rec.metrics.overflowX <= 1 && rec.metrics.missing.length === 0 && errs.length === 0;
        await page.screenshot({
          path: path.join(SHOT_DIR, `${p.key}-${w}.png`),
          fullPage: false,
        });
      } catch (e) {
        rec.error = String(e).slice(0, 300);
      }
      if (errs.length) consoleErrors.push({ page: p.key, width: w, errs });
      results.push(rec);
      console.log(
        `${rec.ok ? '✅' : '❌'} ${p.name} @${w}  overflow=${rec.metrics?.overflowX ?? 'n/a'}  ` +
          `missing=${rec.metrics?.missing.join(', ') || '-'}  errs=${errs.length}  assets404=${warns.length}` +
          (rec.error ? `  err=${rec.error}` : ''),
      );
      await ctx.close();
    }
  }

  await browser.close();

  // 明细落到文件，便于逐条看
  const report = { base: BASE, at: new Date().toISOString(), results };
  fs.writeFileSync(path.join(SHOT_DIR, 'report.json'), JSON.stringify(report, null, 2), 'utf8');

  const bad = results.filter((r) => !r.ok);
  console.log(`\n总计 ${results.length} 项，失败 ${bad.length} 项；截图与 report.json → ${SHOT_DIR}`);
  if (bad.length) {
    for (const b of bad) {
      console.log(`\n— ${b.name} @${b.width}`);
      if (b.error) console.log('  error: ' + b.error);
      if (b.metrics?.missing?.length) console.log('  缺失元素: ' + b.metrics.missing.join(' | '));
      if (b.metrics && b.metrics.overflowX > 1) console.log('  横向溢出: ' + b.metrics.overflowX + 'px');
      for (const e of b.errs.slice(0, 6)) console.log('  ' + e);
    }
    process.exit(1);
  }
})().catch((e) => {
  console.error('验收脚本异常：', e);
  process.exit(2);
});

function findChrome() {
  const base = path.join(process.env.LOCALAPPDATA || '', 'ms-playwright');
  const dirs = fs.existsSync(base)
    ? fs.readdirSync(base).filter((d) => d.startsWith('chromium-')).sort().reverse()
    : [];
  for (const d of dirs) {
    const exe = path.join(base, d, 'chrome-win', 'chrome.exe');
    if (fs.existsSync(exe)) return exe;
  }
  throw new Error('未找到 chromium，可用 NVS_CHROME 指定');
}
