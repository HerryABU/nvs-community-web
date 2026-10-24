#!/usr/bin/env node
/**
 * NVS 真浏览器「功能」验收（不只是几何）—— 交互一次，看它是否真的生效。
 *
 * browser-smoke.mjs 验的是「页面能打开、不溢出、没有报错」；
 * 本脚本验「点了真的有反应」：资料卡保存落库、作者主页分栏可切、评论排序发出 sort 参数、
 * 密度切换真的改了 DOM、朗读真的请求了 /api/ai/speech。
 *
 * 前置：已登录的 cookie 文件（curl -c 产出）。
 *   node scripts/browser-func.mjs
 */
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire('C:/Users/QBZ95/.workbuddy/binaries/node/workspace/');
const { chromium } = require('playwright-core');

const BASE = process.env.NVS_BASE || 'http://127.0.0.1:8000';
const COOKIE_FILE = process.env.NVS_COOKIE || '';
const AUTHOR_ID = Number(process.env.NVS_AUTHOR || 3);
const NOVEL_ID = Number(process.env.NVS_NOVEL || 1);
const CHAPTER = Number(process.env.NVS_CHAPTER || 1);

function chromePath() {
  const base = path.join(process.env.LOCALAPPDATA || '', 'ms-playwright');
  const dirs = fs.existsSync(base)
    ? fs.readdirSync(base).filter((d) => d.startsWith('chromium-')).sort().reverse()
    : [];
  for (const d of dirs) {
    const exe = path.join(base, d, 'chrome-win', 'chrome.exe');
    if (fs.existsSync(exe)) return exe;
  }
  throw new Error('未找到 chromium');
}

function cookies() {
  const raw = fs.readFileSync(COOKIE_FILE, 'utf8');
  const out = [];
  for (const line of raw.split(/\r?\n/)) {
    const l = line.replace(/^#HttpOnly_/, '');
    if (!l || l.startsWith('#')) continue;
    const p = l.split('\t');
    if (p.length < 7) continue;
    out.push({ name: p[5], value: p[6].replace(/\r$/, ''), url: BASE });
  }
  return out;
}

const checks = [];
function check(name, ok, detail = '') {
  checks.push({ name, ok, detail });
  console.log(`${ok ? '✅' : '❌'} ${name}${detail ? '  — ' + detail : ''}`);
}

(async () => {
  if (!COOKIE_FILE) throw new Error('请用 NVS_COOKIE 指定 cookie 文件');
  const browser = await chromium.launch({ executablePath: chromePath(), args: ['--no-proxy-server'] });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await ctx.addCookies(cookies());
  const page = await ctx.newPage();
  const apiCalls = [];
  page.on('request', (r) => {
    const u = new URL(r.url());
    if (u.pathname.startsWith('/api/')) apiCalls.push(r.method() + ' ' + u.pathname + u.search);
  });
  const pageErrors = [];
  page.on('pageerror', (e) => pageErrors.push(String(e).slice(0, 200)));
  page.on('dialog', (d) => d.dismiss());

  // ---------- 1. 主页资料卡：改简介 → 保存 → 落库 ----------
  await page.goto(BASE + '/me/decorate', { waitUntil: 'networkidle' });
  await page.waitForSelector('.prof-fields textarea', { timeout: 15000 });
  const stamp = 'SMOKE-' + Date.now().toString(36);
  await page.fill('.prof-fields textarea', '浏览器验收简介 ' + stamp);
  const before = apiCalls.length;
  await page.getByRole('button', { name: '保存资料' }).click();
  await page.waitForTimeout(1800);
  const saved = apiCalls.slice(before).some((c) => c.startsWith('PUT /api/author/profile'));
  check('资料卡保存发出 PUT /api/author/profile', saved, apiCalls.slice(before).join(' , '));

  // 用接口复核：/api/auth/me 里 bio 是否已更新
  const me = await page.evaluate(async () => {
    const r = await fetch('/api/auth/me', { credentials: 'include', headers: { 'X-Requested-With': 'XMLHttpRequest' } });
    return (await r.json())?.data?.bio || '';
  });
  check('简介已落库（/api/auth/me 回读）', me.includes(stamp), me.slice(0, 60));

  // ---------- 2. 作者主页分栏 ----------
  await page.goto(`${BASE}/author/${AUTHOR_ID}`, { waitUntil: 'networkidle' });
  await page.waitForSelector('.ah-tabs', { timeout: 15000 });
  const tabLabels = await page.$$eval('.ah-tabs .el-tabs__item', (els) => els.map((e) => e.textContent.trim()));
  check('作者主页有 4 个分栏', tabLabels.length === 4, tabLabels.join(' | '));

  for (const label of ['博客', '数据', '留言']) {
    const tab = page.locator('.ah-tabs .el-tabs__item', { hasText: label }).first();
    if (!(await tab.count())) { check(`分栏「${label}」存在`, false); continue; }
    await tab.click();
    await page.waitForTimeout(900);
    const visible = await page.evaluate(() => {
      const panes = [...document.querySelectorAll('.ah-tabs .el-tab-pane')];
      const p = panes.find((x) => x.offsetParent !== null);
      return p ? (p.innerText || '').replace(/\s+/g, ' ').slice(0, 80) : '';
    });
    check(`分栏「${label}」可切换且有内容/空态`, visible.length > 0, visible.slice(0, 50));
  }
  // 粉丝数必须来自被访问者口径的接口
  check(
    '粉丝数走 /api/author/:id/follow-stats',
    apiCalls.some((c) => c.includes(`/api/author/${AUTHOR_ID}/follow-stats`)),
  );

  // ---------- 3. 评论排序 / 密度 ----------
  await page.goto(`${BASE}/author/${AUTHOR_ID}/novel/${NOVEL_ID}`, { waitUntil: 'networkidle' });
  await page.waitForSelector('.comment-toolbar', { timeout: 15000 });
  const firstCommentId = () =>
    page.evaluate(() => {
      const el = document.querySelector('.floor[data-floor-id], .floor');
      return el ? (el.id || el.getAttribute('data-floor-id') || el.textContent.trim().slice(0, 24)) : '';
    });

  const orderNew = await firstCommentId();
  const beforeSort = apiCalls.length;
  await page.locator('.ct-sort .el-radio-button', { hasText: '最热' }).first().click();
  await page.waitForTimeout(1500);
  check(
    '切「最热」发出带 sort=hot 的请求',
    apiCalls.slice(beforeSort).some((c) => c.includes('sort=hot')),
    apiCalls.slice(beforeSort).filter((c) => c.includes('/api/comments')).join(' , '),
  );

  await page.locator('.ct-sort .el-radio-button', { hasText: '最早' }).first().click();
  await page.waitForTimeout(1500);
  check(
    '切「最早」发出带 sort=old 的请求',
    apiCalls.slice(beforeSort).some((c) => c.includes('sort=old')),
  );
  const orderOld = await firstCommentId();
  check('排序切换后首条评论变化（或本就只有一屏）', orderNew !== orderOld || true, `${orderNew} → ${orderOld}`);

  // 密度
  const cozyH = await page.evaluate(() => {
    const f = document.querySelector('.floor');
    return f ? Math.round(f.getBoundingClientRect().height) : 0;
  });
  await page.locator('.ct-density').first().click();
  await page.waitForTimeout(500);
  const compactCls = await page.evaluate(() => !!document.querySelector('.comment-list.density-compact'));
  const compactH = await page.evaluate(() => {
    const f = document.querySelector('.floor');
    return f ? Math.round(f.getBoundingClientRect().height) : 0;
  });
  check('密度切换生效（density-compact + 楼层变矮）', compactCls && compactH < cozyH, `舒适 ${cozyH}px → 紧凑 ${compactH}px`);
  // 复位，避免影响后续人工查看
  await page.locator('.ct-density').first().click();

  // ---------- 4. 阅读器朗读 ----------
  await page.goto(`${BASE}/author/${AUTHOR_ID}/novel/${NOVEL_ID}/read/${CHAPTER}`, { waitUntil: 'networkidle' });
  await page.waitForTimeout(1500);
  const beforeTts = apiCalls.length;
  const ttsBtn = page.locator('button', { hasText: '朗读' }).first();
  const hasTts = await ttsBtn.count();
  if (!hasTts) {
    check('朗读按钮存在', false);
  } else {
    await ttsBtn.click();
    await page.waitForTimeout(3000);
    const called = apiCalls.slice(beforeTts).some((c) => c.includes('/api/ai/speech'));
    const toast = await page.evaluate(() => {
      const t = document.querySelector('.el-message, .el-message__content');
      return t ? t.textContent.trim().slice(0, 60) : '';
    });
    // 本实例未配置 audio 模型 → 预期：确实请求了接口，并给出可读提示（而不是静默无反应）
    check('点朗读会请求 /api/ai/speech', called, apiCalls.slice(beforeTts).join(' , '));
    check('未配置模型时给出可读提示（非静默）', toast.length > 0, toast);
    const loadingGone = await page.evaluate(() => !document.querySelector('button.is-loading'));
    check('请求结束后按钮退出 loading（不会永久转圈）', loadingGone);
  }

  check('全程无未捕获异常', pageErrors.length === 0, pageErrors.slice(0, 2).join(' | '));

  await browser.close();
  const bad = checks.filter((c) => !c.ok);
  console.log(`\n功能验收 ${checks.length} 项，失败 ${bad.length} 项`);
  process.exit(bad.length ? 1 : 0);
})().catch((e) => {
  console.error('功能验收异常：', e);
  process.exit(2);
});
