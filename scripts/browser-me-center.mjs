#!/usr/bin/env node
/**
 * 个人中心验收：下拉菜单瘦身 + 所有入口真的能在 /me 里找到，且链接都能落地。
 *
 * 断言：
 *  1. 头像下拉只剩「个人中心 / 我的主页 /(管理员) 管理员面板 /(警方) 警方取管台 / 退出登录」
 *  2. 被移除的 13 个入口在 /me 概览的「我的服务」里全部存在
 *  3. 每个服务链接点进去都能到达目标页（不 404、不被弹回登录）
 *  4. /me/decorate 有「更换头像」按钮（原来在下拉里的功能有落点）
 */
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire('C:/Users/QBZ95/.workbuddy/binaries/node/workspace/');
const { chromium } = require('playwright-core');

const BASE = process.env.NVS_BASE || 'http://127.0.0.1:8000';
const COOKIE_FILE = process.env.NVS_COOKIE || '';
const SHOT_DIR = 'C:/Users/QBZ95/Desktop/tools/tests/test_go/log/shots';

const REMOVED = [
  '更换头像', '模板设置', '我的举报', '格式使用文档', 'API Key 爬虫指南',
  '我的 API Key', '我的钱包', 'AI 工作台', '黑名单管理', '特别关注',
  '特别关注动态', '我的私信', '账号设置',
];

function chromePath() {
  const base = path.join(process.env.LOCALAPPDATA || '', 'ms-playwright');
  const dirs = fs.existsSync(base) ? fs.readdirSync(base).filter((d) => d.startsWith('chromium-')).sort().reverse() : [];
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

const rows = [];
function check(name, ok, detail = '') {
  rows.push({ name, ok, detail });
  console.log(`${ok ? '✅' : '❌'} ${name}${detail ? '  — ' + detail : ''}`);
}

(async () => {
  const browser = await chromium.launch({ executablePath: chromePath(), args: ['--no-proxy-server'] });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await ctx.addCookies(cookies());
  const page = await ctx.newPage();
  const pageErrors = [];
  page.on('pageerror', (e) => pageErrors.push(String(e).slice(0, 160)));

  // ---------- 1. 头像下拉内容 ----------
  await page.goto(BASE + '/', { waitUntil: 'networkidle' });
  await page.waitForSelector('.user-info', { timeout: 20000 });
  await page.click('.user-info');
  await page.waitForTimeout(600);
  // 用户菜单有专属 popper-class，避免和「作者工作台」「更多」等下拉混在一起
  const userMenu = await page.$$eval('.user-menu-popper .el-dropdown-menu__item', (els) =>
    els.map((e) => e.textContent.trim()).filter(Boolean),
  );
  check('头像下拉已瘦身（≤5 项）', userMenu.length <= 5, userMenu.join(' | '));
  const stillThere = REMOVED.filter((x) => userMenu.some((m) => m.includes(x)));
  check('13 个入口已从下拉移除', stillThere.length === 0, stillThere.length ? '仍在: ' + stillThere.join(',') : '');
  await page.keyboard.press('Escape');
  await page.screenshot({ path: path.join(SHOT_DIR, 'me-dropdown-1440.png') });

  // ---------- 2. /me 概览里的「我的服务」 ----------
  await page.goto(BASE + '/me', { waitUntil: 'networkidle' });
  await page.waitForSelector('.me-services', { timeout: 20000 });
  const svcText = await page.$eval('.me-services', (el) => el.innerText.replace(/\s+/g, ' '));
  const svcLinks = await page.$$eval('.me-svc-item', (els) => els.map((e) => ({ label: e.querySelector('.me-svc-label')?.textContent || '', href: e.getAttribute('href') || '' })));
  check('「我的服务」分组已渲染', svcLinks.length >= 13, `${svcLinks.length} 个入口`);
  const missing = REMOVED.filter((r) => !svcText.includes(r) && !(r === '更换头像' && svcText.includes('头像与资料')));
  check('被移除的入口在 /me 都能找到', missing.length === 0, missing.length ? '缺: ' + missing.join(',') : '');
  await page.screenshot({ path: path.join(SHOT_DIR, 'me-services-1440.png') });

  // ---------- 3. 每个入口落地可达 ----------
  const bad = [];
  for (const l of svcLinks) {
    const before = page.url();
    await page.goto(BASE + l.href, { waitUntil: 'domcontentloaded' });
    await page.waitForTimeout(400);
    const p = new URL(page.url()).pathname;
    const isLogin = p.startsWith('/login');
    const notFound = (await page.content()).includes('页面不存在');
    if (isLogin || notFound || p !== new URL(BASE + l.href).pathname) {
      bad.push(`${l.label} → ${l.href} (实际 ${p}${isLogin ? ' [被弹登录]' : ''}${notFound ? ' [404]' : ''})`);
    }
    await page.goto(before, { waitUntil: 'domcontentloaded' }).catch(() => {});
  }
  check('所有服务入口都能直达（不 404 / 不弹登录）', bad.length === 0, bad.slice(0, 3).join(' ; '));

  // ---------- 4. 头像上传落点 ----------
  await page.goto(BASE + '/me/decorate', { waitUntil: 'networkidle' });
  await page.waitForSelector('.prof-avatar-col', { timeout: 20000 });
  const hasBtn = await page.locator('.prof-avatar-btns button', { hasText: '更换头像' }).count();
  const hasInput = await page.locator('input[type=file]').count();
  check('装扮页有「更换头像」按钮与文件选择器', hasBtn > 0 && hasInput > 0, `btn=${hasBtn} input=${hasInput}`);

  // 头像可点跳转
  await page.goto(BASE + '/me', { waitUntil: 'networkidle' });
  await page.waitForSelector('.me-avatar', { timeout: 15000 });
  await page.click('.me-avatar');
  await page.waitForTimeout(800);
  check('个人中心头像可点进装扮页', new URL(page.url()).pathname === '/me/decorate', page.url());

  check('全程无未捕获异常', pageErrors.length === 0, pageErrors.slice(0, 2).join(' | '));

  await browser.close();
  const failed = rows.filter((r) => !r.ok);
  console.log(`\n个人中心验收 ${rows.length} 项，失败 ${failed.length} 项`);
  process.exit(failed.length ? 1 : 0);
})().catch((e) => {
  console.error('验收异常：', e);
  process.exit(2);
});
