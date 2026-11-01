#!/usr/bin/env node
/**
 * NVS 移动端专项验收（真浏览器）
 *
 * 为什么单独写一个：browser-smoke.mjs 关注「能不能用 + 有没有横向溢出」，
 * 但移动端的体验债大多不体现为溢出，而是：
 *   - 触控目标太小（手机上点不准）
 *   - 文字太小、行距太挤
 *   - 工具条挤成一团（按钮换行/被压扁）
 *   - 固定定位元素挡住正文
 * 这些必须量几何，不能靠看截图。
 *
 * 登录：直接在浏览器上下文里调 /api/auth/login。
 *   ⚠️ 访问令牌与「设备指纹 + 登录地」强绑定，用 curl 登录会让浏览器里的
 *      /api/auth/me 全部 401（指纹对不上）。所以必须在页面上下文里登录，
 *      并带上页面自己算出来的 X-Device-Fingerprint。
 *
 * 用法：
 *   NVS_BASE=http://127.0.0.1:8001 node scripts/browser-mobile.mjs
 *   NVS_BASE=... NVS_MOBILE_WIDTHS=375,390 node scripts/browser-mobile.mjs
 */
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire('C:/Users/QBZ95/.workbuddy/binaries/node/workspace/');
const { chromium } = require('playwright-core');

const BASE = process.env.NVS_BASE || 'http://127.0.0.1:8001';
const SHOT_DIR = process.env.NVS_SHOT_DIR || 'C:/tmp/nvs_qa/shots-mobile';
const WIDTHS = (process.env.NVS_MOBILE_WIDTHS || '375,390,414')
  .split(',')
  .map((s) => Number(s.trim()))
  .filter(Boolean);
const ONLY = (process.argv.find((a) => a.startsWith('--only=')) || '').split('=')[1] || '';

// 触控目标最小边长（px）。低于此值在手机上容易点错。
const MIN_TAP = 36;
// 字号下限（px）。低于此值在手机上阅读吃力。
const MIN_FONT = 11;

const PAGES = [
  { key: 'home', name: '首页', url: '/', ready: 'body' },
  { key: 'reader', name: '阅读器', url: '/author/3/novel/1/read/1', ready: '.reader-page, .reader, .page-container' },
  { key: 'novel', name: '小说详情', url: '/author/3/novel/1', ready: '.novel-detail, .page-container' },
  { key: 'author', name: '作者主页', url: '/author/3', ready: '.ah-profile, .page-container' },
  { key: 'forums', name: '论坛列表', url: '/forums', ready: '.page-container' },
  { key: 'thread', name: '帖子详情', url: '/forum/1', ready: '.page-container' },
  { key: 'category', name: '分类页', url: '/category/all', ready: '.page-container' },
  { key: 'membership', name: '会员页', url: '/membership', ready: '.mb-page' },
  { key: 'ai', name: 'AI 工作台', url: '/ai', ready: '.ai-hub' },
  { key: 'admin', name: '后台首页', url: '/admin', ready: '.dashboard-bg, .admin-page' },
];

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

/** 页面内度量：溢出元素 / 触控目标 / 字号 / 固定层遮挡 */
async function measure(page) {
  return page.evaluate(
    ({ MIN_TAP, MIN_FONT }) => {
      const vw = window.innerWidth;
      const de = document.documentElement;
      const out = {
        vw,
        overflowX: de.scrollWidth - vw,
        offenders: [],
        smallTaps: [],
        tinyText: [],
        fixedBars: [],
        hasHScroll: false,
      };
      out.hasHScroll = de.scrollWidth > vw + 1;

      const desc = (el) => {
        const cls = (el.className && el.className.baseVal !== undefined
          ? el.className.baseVal
          : el.className || '')
          .toString()
          .trim()
          .split(/\s+/)
          .slice(0, 2)
          .join('.');
        const id = el.id ? `#${el.id}` : '';
        const tag = el.tagName.toLowerCase();
        return `${tag}${id}${cls ? '.' + cls : ''}`;
      };

      const all = Array.from(document.querySelectorAll('body *'));
      for (const el of all) {
        const r = el.getBoundingClientRect();
        if (r.width === 0 || r.height === 0) continue;
        const cs = getComputedStyle(el);
        if (cs.visibility === 'hidden' || cs.display === 'none' || cs.opacity === '0') continue;

        // 1) 越过右边界（横向溢出的元凶）
        if (r.right > vw + 1 && out.offenders.length < 12) {
          const txt = (el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 30);
          out.offenders.push({
            sel: desc(el),
            right: Math.round(r.right),
            w: Math.round(r.width),
            txt,
          });
        }

        // 2) 触控目标过小
        const tag = el.tagName.toLowerCase();
        const isTap =
          tag === 'button' ||
          // 行内文字链接（display:inline）不算独立触控目标：
          // 它就在正文里，命中区域就是文字本身，强行放大反而会破坏排版
          (tag === 'a' && el.getAttribute('href') && cs.display !== 'inline') ||
          el.getAttribute('role') === 'button' ||
          el.classList.contains('el-button') ||
          el.classList.contains('el-checkbox') ||
          el.classList.contains('el-radio') ||
          el.classList.contains('el-switch');
        if (isTap && (r.height < MIN_TAP || r.width < MIN_TAP) && out.smallTaps.length < 10) {
          out.smallTaps.push({
            sel: desc(el),
            w: Math.round(r.width),
            h: Math.round(r.height),
            txt: (el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 20),
          });
        }

        // 3) 字号过小（只看有直接文本的元素）
        const fs = parseFloat(cs.fontSize) || 0;
        const hasOwnText = Array.from(el.childNodes).some(
          (n) => n.nodeType === 3 && n.textContent.trim().length > 0,
        );
        if (hasOwnText && fs > 0 && fs < MIN_FONT && out.tinyText.length < 8) {
          out.tinyText.push({ sel: desc(el), fs, txt: (el.textContent || '').trim().slice(0, 20) });
        }

        // 4) 固定定位条（可能挡住内容）
        if ((cs.position === 'fixed' || cs.position === 'sticky') && r.height > 0) {
          out.fixedBars.push({ sel: desc(el), pos: cs.position, h: Math.round(r.height) });
        }
      }
      return out;
    },
    { MIN_TAP, MIN_FONT },
  );
}

const results = [];

(async () => {
  fs.mkdirSync(SHOT_DIR, { recursive: true });
  const browser = await chromium.launch({
    executablePath: process.env.NVS_CHROME || findChrome(),
    args: ['--no-proxy-server', '--disable-gpu'],
  });
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 1 });

  // ── 1) 抓页面自己算出来的设备指纹（登录要用同一个值）──
  const probe = await ctx.newPage();
  let fingerprint = '';
  probe.on('request', (r) => {
    const h = r.headers()['x-device-fingerprint'];
    if (h && !fingerprint) fingerprint = h;
  });
  await probe.goto(BASE + '/login', { waitUntil: 'domcontentloaded', timeout: 45000 });
  await probe.waitForTimeout(2500);
  await probe.close();
  if (!fingerprint) {
    console.error('⚠️ 未抓到设备指纹，登录可能被拒（令牌绑定指纹）');
  }

  // ── 2) 在浏览器上下文里登录（cookie 直接进 context）──
  const token = process.env.NVS_CAPTCHA_TOKEN || '';
  const login = await ctx.request.post(BASE + '/api/auth/login', {
    data: {
      email: process.env.NVS_EMAIL || 'admin@nvs.local',
      password: process.env.NVS_PASSWORD || 'NTsuper123456',
      captcha_token: token,
    },
    headers: {
      'Content-Type': 'application/json',
      'X-Requested-With': 'XMLHttpRequest',
      ...(fingerprint ? { 'X-Device-Fingerprint': fingerprint } : {}),
    },
  });
  const loginBody = await login.json().catch(() => ({}));
  if (loginBody.code !== 0) {
    console.error(`❌ 登录失败：${JSON.stringify(loginBody).slice(0, 200)}`);
    console.error('   （需先跑 nvs_qa_login.sh 取一个一次性 captcha token，用 NVS_CAPTCHA_TOKEN 传入）');
    await browser.close();
    process.exit(2);
  }
  console.log('✅ 登录成功（浏览器上下文内，指纹一致）');

  // ── 3) 逐页逐宽度度量 ──
  const targets = ONLY ? PAGES.filter((p) => p.key === ONLY) : PAGES;
  for (const p of targets) {
    for (const w of WIDTHS) {
      const page = await ctx.newPage();
      await page.setViewportSize({ width: w, height: 844 });
      const errs = [];
      page.on('pageerror', (e) => errs.push('[pageerror] ' + String(e).slice(0, 160)));
      page.on('response', (r) => {
        if (r.status() < 400) return;
        const u = r.url().replace(BASE, '');
        if (u.startsWith('/api/')) errs.push(`API [${r.status()}] ${u}`);
      });
      const rec = { key: p.key, name: p.name, width: w, errs, m: null, error: '' };
      try {
        await page.goto(BASE + p.url, { waitUntil: 'networkidle', timeout: 45000 });
        await page.waitForSelector(p.ready, { timeout: 20000 });
        await page.waitForTimeout(1200);
        rec.m = await measure(page);
        await page.screenshot({ path: path.join(SHOT_DIR, `${p.key}-${w}.png`) });
      } catch (e) {
        rec.error = String(e).slice(0, 220);
      }
      results.push(rec);
      const m = rec.m;
      console.log(
        `${m && m.overflowX > 1 ? '❌' : '✅'} ${p.name} @${w}  ` +
          `溢出=${m ? m.overflowX : 'n/a'}px  小触控点=${m ? m.smallTaps.length : '-'}  ` +
          `小字号=${m ? m.tinyText.length : '-'}  固定层=${m ? m.fixedBars.length : '-'}  errs=${errs.length}` +
          (rec.error ? `  err=${rec.error}` : ''),
      );
      await page.close();
    }
  }

  await browser.close();
  fs.writeFileSync(path.join(SHOT_DIR, 'report.json'), JSON.stringify({ base: BASE, results }, null, 2), 'utf8');

  // ── 4) 汇总问题清单 ──
  const issues = [];
  for (const r of results) {
    if (!r.m) continue;
    if (r.m.overflowX > 1) {
      issues.push(`[溢出] ${r.name} @${r.width}：横向多出 ${r.m.overflowX}px`);
      for (const o of r.m.offenders.slice(0, 5)) {
        issues.push(`        → ${o.sel} right=${o.right} w=${o.w} "${o.txt}"`);
      }
    }
    if (r.m.smallTaps.length) {
      issues.push(`[触控点过小] ${r.name} @${r.width}：${r.m.smallTaps.length} 个`);
      for (const s of r.m.smallTaps.slice(0, 4)) {
        issues.push(`        → ${s.sel} ${s.w}×${s.h} "${s.txt}"`);
      }
    }
    if (r.m.tinyText.length) {
      issues.push(`[字号过小] ${r.name} @${r.width}：${r.m.tinyText.length} 处`);
      for (const t of r.m.tinyText.slice(0, 4)) {
        issues.push(`        → ${t.sel} ${t.fs}px "${t.txt}"`);
      }
    }
    for (const e of r.errs.slice(0, 3)) issues.push(`[错误] ${r.name} @${r.width}：${e}`);
  }
  console.log('\n===== 问题清单 =====');
  console.log(issues.length ? issues.join('\n') : '无');
  console.log(`\n截图与 report.json → ${SHOT_DIR}`);
})().catch((e) => {
  console.error('审计脚本异常：', e);
  process.exit(3);
});
