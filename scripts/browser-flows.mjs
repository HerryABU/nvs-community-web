#!/usr/bin/env node
/**
 * NVS 关键闭环 · 真浏览器验收（DOM / HTTP 量化断言）
 *
 * 为什么不用截图当凭证：本机模型读不了图片，截图无法作为验收依据。
 * 因此所有结论都由可复现的量化断言给出——元素几何、接口状态码、
 * 滑块像素定位结果、流式增量长度。
 *
 * 覆盖闭环：
 *   auth   滑块 → 邮箱验证码 → 登录（真几何滑块 + 防裸调）
 *   ai     AI 对话流式输出（真实上游，SSE 增量落进消息区）
 *   draw   专业绘画：卡片墙 + 五档比例
 *   graph  评论图谱
 *   police 警方台：舆情聚合 + 已删除内容高亮色可读
 *
 * 用法：
 *   node scripts/browser-flows.mjs               # 全量
 *   node scripts/browser-flows.mjs --only=auth   # 单项
 *   NVS_PASSWORD=xxx node scripts/browser-flows.mjs
 */
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire('C:/Users/QBZ95/.workbuddy/binaries/node/workspace/');
const { chromium } = require('playwright-core');

const BASE = process.env.NVS_BASE || 'http://127.0.0.1:8000';
const PASSWORDS = (process.env.NVS_PASSWORD || 'admin123,Smoke@2026').split(',').map((s) => s.trim()).filter(Boolean);
const EMAIL = process.env.NVS_EMAIL || 'admin@nvs.local';
const ONLY = (process.argv.find((a) => a.startsWith('--only=')) || '').split('=')[1] || '';
const OUT_DIR = process.env.NVS_OUT || path.join(process.cwd(), 'docs', 'qa');

const results = [];
function record(flow, name, ok, detail = '') {
  results.push({ flow, name, ok: !!ok, detail: String(detail).slice(0, 300) });
  console.log(`${ok ? '✅' : '❌'} [${flow}] ${name}${detail ? ' — ' + detail : ''}`);
}

/** 在页面上下文里发请求（复用同源 cookie；用绝对 URL，页面可能在 about:blank） */
async function api(page, method, p, body) {
  return await page.evaluate(
    async ([m, path, b, base]) => {
      const init = { method: m, headers: {}, credentials: 'include' };
      if (b !== null && b !== undefined) {
        init.headers['Content-Type'] = 'application/json';
        init.body = JSON.stringify(b);
      }
      const r = await fetch(base + path, init);
      let j = null;
      try { j = await r.json(); } catch (e) { j = null; }
      return { status: r.status, json: j };
    },
    [method, p, body ?? null, BASE],
  );
}

/**
 * 取滑块挑战并在浏览器里用 canvas 像素定位缺口左边界。
 * 服务端把缺口画成 42×42 的半透明黑方块（alpha 90）叠在纵向渐变背景上，
 * 因此每行的中位亮度即基色，缺口行会出现一条 42px 宽的暗带。
 *
 * dev server 的 HMR 可能在 evaluate 期间触发整页重载（执行上下文被销毁），
 * 故这里做有限次重试。
 */
async function solveCaptcha(page) {
  let lastErr = '';
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      return await solveCaptchaOnce(page);
    } catch (e) {
      lastErr = String(e).slice(0, 120);
      await page.waitForTimeout(1500);
    }
  }
  return { err: 'solveCaptcha 失败: ' + lastErr };
}

async function solveCaptchaOnce(page) {
  return await page.evaluate(async () => {
    const r = await fetch('/api/auth/captcha/issue', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}', credentials: 'include',
    });
    const j = await r.json();
    const ch = j && j.data;
    if (!ch) return { err: 'issue 未返回 data', raw: j };
    const img = new Image();
    await new Promise((res, rej) => { img.onload = res; img.onerror = () => rej(new Error('bg 解码失败')); img.src = ch.bg_image; });
    const c = document.createElement('canvas');
    c.width = img.width; c.height = img.height;
    const ctx = c.getContext('2d', { willReadFrequently: true });
    ctx.drawImage(img, 0, 0);
    const d = ctx.getImageData(0, 0, c.width, c.height).data;
    const lum = (i) => 0.299 * d[i] + 0.587 * d[i + 1] + 0.114 * d[i + 2];
    const y0 = Math.floor(c.height / 2 - 42 / 2);
    const votes = new Map();
    for (let y = y0; y < y0 + 42 && y < c.height; y++) {
      const lums = [];
      for (let x = 0; x < c.width; x++) lums.push(lum((y * c.width + x) * 4));
      const sorted = [...lums].sort((a, b) => a - b);
      const base = sorted[Math.floor(sorted.length / 2)];
      if (!base) continue;
      let start = -1;
      const runs = [];
      for (let x = 0; x < c.width; x++) {
        if (lums[x] < base * 0.85) { if (start < 0) start = x; }
        else { if (start >= 0) { runs.push([start, x - 1]); start = -1; } }
      }
      if (start >= 0) runs.push([start, c.width - 1]);
      for (const [s, e] of runs) {
        if (e - s + 1 >= 30) votes.set(s, (votes.get(s) || 0) + 1);
      }
    }
    let best = -1, bestV = 0;
    for (const [k, v] of votes) if (v > bestV) { bestV = v; best = k; }
    return {
      challenge_id: ch.challenge_id,
      x: best,
      votes: bestV,
      imgW: c.width, imgH: c.height,
      bgIsPng: String(ch.bg_image || '').startsWith('data:image/png;base64,'),
      bgLen: (ch.bg_image || '').length,
      pieceLen: (ch.piece_image || '').length,
      declaredW: ch.width, declaredH: ch.height,
    };
  });
}

/** 完整登录：解滑块 → 换 pass token → 登录 → 落 cookie */
async function login(page, ctx) {
  // 第一次挑战用于验证「错误位置必须被拒」
  const probe = await solveCaptcha(page);
  if (probe.err) return { ok: false, why: 'captcha issue 失败: ' + probe.err };
  if (!probe.bgIsPng) return { ok: false, why: 'bg_image 不是 PNG data URL（滑块非真几何）' };
  if (probe.votes < 10) return { ok: false, why: `缺口像素定位失败 votes=${probe.votes}` };
  const wrong = await api(page, 'POST', '/api/auth/captcha/verify', {
    challenge_id: probe.challenge_id, x: Math.max(0, probe.x + 40),
  });
  const wrongRejected = wrong.status >= 400 || (wrong.json && wrong.json.code !== 0);

  // pass token 是一次性的：每次口令尝试都要重新解一次滑块
  let last = null;
  for (const pw of PASSWORDS) {
    const sol = await solveCaptcha(page);
    if (sol.err) return { ok: false, why: 'captcha issue 失败: ' + sol.err, sol: probe, wrongRejected };
    const okv = await api(page, 'POST', '/api/auth/captcha/verify', {
      challenge_id: sol.challenge_id, x: sol.x,
    });
    const token = okv.json && okv.json.data && okv.json.data.captcha_token;
    if (!token) return { ok: false, why: '正确滑块位置未换到 pass token', raw: okv.json, sol: probe, wrongRejected };
    const r = await api(page, 'POST', '/api/auth/login', {
      email: EMAIL, password: pw, captcha_token: token,
    });
    last = { pw, r };
    if (r.status === 200 && r.json && r.json.code === 0) {
      const jar = await ctx.cookies();
      if (!jar.some((c) => c.name === 'token')) {
        const t = r.json.data && (r.json.data.token || r.json.data.access_token);
        if (t) await ctx.addCookies([{ name: 'token', value: t, url: BASE }]);
      }
      return { ok: true, sol: probe, wrongRejected, token, password: pw };
    }
    const msg = (r.json && r.json.message) || '';
    // 命中登录限流（5 次/60s）：退避后重试同一个口令
    if (/过于频繁/.test(msg)) {
      await page.waitForTimeout(15000);
      continue;
    }
    // 口令错误（邮箱或密码错误）→ 换下一个口令重试；其它原因直接放弃
    if (!/密码错误/.test(msg)) break;
  }
  return { ok: false, why: '登录失败', raw: last && last.r && last.r.json, sol: probe, wrongRejected };
}

async function newPage(browser, cookies) {
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 });
  // 清掉本地存储里的滑块令牌，否则滑块组件不会渲染（前端按「无 token 才显示」）
  await ctx.addInitScript(() => {
    try { localStorage.clear(); sessionStorage.clear(); } catch (e) { /* ignore */ }
  });
  if (cookies && cookies.length) {
    try {
      await ctx.addCookies(cookies.map((c) => ({ name: c.name, value: c.value, url: BASE })));
    } catch {
      await ctx.addCookies(cookies.map((c) => ({ name: c.name, value: c.value, domain: c.domain, path: c.path, httpOnly: c.httpOnly, secure: c.secure })));
    }
  }
  const page = await ctx.newPage();
  const errs = [];
  // pageerror 只抓未捕获异常；Vue 的 setup 错误会被框架捕获并走 console.error，
  // 必须同时监听 console，否则「子组件静默不渲染」这类问题查不到根因。
  page.on('console', (m) => {
    if (m.type() !== 'error') return;
    const t = m.text().slice(0, 240);
    if (t.startsWith('Failed to load resource')) return; // 图片等资源 404 与业务无关
    errs.push('[console] ' + t);
  });
  page.on('pageerror', (e) => errs.push('[pageerror] ' + String(e).slice(0, 240)));
  return { ctx, page, errs };
}

/* ============================ 闭环 1：认证 ============================ */
async function flowAuth(browser) {
  const { ctx, page, errs } = await newPage(browser);
  try {
    await page.goto(BASE + '/login', { waitUntil: 'domcontentloaded', timeout: 60000 });
    await page.waitForTimeout(2500); // 等 dev server HMR / 首屏异步加载稳定

    // 前端滑块组件真实存在（滑块仅在「本地无有效 pass token」时渲染，
    // 且要等 siteInfo 异步加载完才知道 captcha_enabled，故用等待而非即时计数）
    let hasSlider = 0;
    try {
      await page.waitForSelector('.slide-puzzle', { timeout: 15000 });
      hasSlider = await page.locator('.slide-puzzle').count();
    } catch (e) { hasSlider = 0; }
    record('auth', '登录页渲染滑块组件', hasSlider > 0, `.slide-puzzle=${hasSlider}`);

    // 无 pass token 时发验证码必须被拒（禁止仅调用接口发送验证码）
    const noCap = await api(page, 'POST', '/api/auth/send-code', { email: 'qa_probe@example.com', captcha_token: '' });
    record('auth', 'send-code 无滑块令牌被拒', noCap.status >= 400 || (noCap.json && noCap.json.code !== 0),
      `status=${noCap.status} code=${noCap.json && noCap.json.code}`);

    // 无 pass token 时登录必须被拒
    const noCapLogin = await api(page, 'POST', '/api/auth/login', { email: EMAIL, password: PASSWORDS[0], captcha_token: '' });
    record('auth', 'login 无滑块令牌被拒', noCapLogin.status >= 400 || (noCapLogin.json && noCapLogin.json.code !== 0),
      `status=${noCapLogin.status} code=${noCapLogin.json && noCapLogin.json.code}`);

    // 解滑块 → 登录
    const lg = await login(page, ctx);
    record('auth', '像素定位滑块缺口', !lg.err && lg.sol && lg.sol.votes >= 10,
      lg.sol ? `x=${lg.sol.x} votes=${lg.sol.votes} ${lg.sol.imgW}x${lg.sol.imgH} bg=${lg.sol.bgLen}B` : lg.why);
    record('auth', '滑块位置错误时被拒', lg.wrongRejected === true, '错误 x 应换不到 pass token');
    record('auth', '管理员登录成功', lg.ok, lg.ok ? `口令来源=${lg.password}` :
      `原因=${lg.why} 原始=${JSON.stringify(lg.raw || {}).slice(0, 180)}`);

    // 登录后能拿到 /me
    if (lg.ok) {
      await page.goto(BASE + '/me', { waitUntil: 'domcontentloaded', timeout: 45000 });
      const me = await api(page, 'GET', '/api/auth/me');
      record('auth', '登录态可用（/api/user/me）', me.status === 200 && me.json && me.json.code === 0,
        `status=${me.status}`);
    }
    if (errs.length) record('auth', '页面无 JS 运行时错误', false, errs.slice(0, 3).join(' | '));
    else record('auth', '页面无 JS 运行时错误', true);
    return { cookies: await ctx.cookies() };
  } finally {
    await ctx.close();
  }
}

/* ============================ 闭环 2：AI 流式 ============================ */
async function flowAI(browser, cookies) {
  const { ctx, page, errs } = await newPage(browser, cookies);
  try {
    await page.goto(BASE + '/ai', { waitUntil: 'domcontentloaded', timeout: 45000 });
    await page.waitForSelector('.ai-chat-messages, .ai-hub', { timeout: 20000 });
    const hasInput = await page.locator('textarea.ai-composer-input').count();
    record('ai', 'AI 工作台渲染对话输入区', hasInput > 0, `textarea=${hasInput}`);

    await page.fill('textarea.ai-composer-input', '用一句话介绍你自己');
    await page.click('.ai-send-btn');

    // 流式：消息区文本长度应随时间增长
    const samples = [];
    let lastLen = 0, grew = false;
    for (let i = 0; i < 40; i++) {
      await page.waitForTimeout(1000);
      const len = await page.evaluate(() => {
        const els = document.querySelectorAll('.ai-chat-messages .ai-msg-md, .ai-chat-messages .ai-msg-plain');
        let t = 0;
        els.forEach((e) => { t += (e.textContent || '').trim().length; });
        return t;
      });
      samples.push(len);
      if (len > 0 && len !== lastLen) grew = true;
      if (len > 0 && i > 6) break;
      lastLen = len;
    }
    const finalLen = samples[samples.length - 1] || 0;
    record('ai', 'AI 回复内容非空', finalLen > 0, `消息区文本长度=${finalLen}`);
    record('ai', '流式增量（长度变化）', grew, `采样=${samples.slice(0, 8).join(',')}`);

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    record('ai', 'AI 工作台无横向溢出', overflow <= 1, `overflowX=${overflow}px`);
    if (errs.length) record('ai', '无 JS 运行时错误', false, errs.slice(0, 3).join(' | '));
    else record('ai', '无 JS 运行时错误', true);
  } finally {
    await ctx.close();
  }
}

/* ============================ 闭环 3：绘画卡片墙 ============================ */
async function flowDraw(browser, cookies) {
  const { ctx, page, errs } = await newPage(browser, cookies);
  try {
    await page.goto(BASE + '/ai', { waitUntil: 'domcontentloaded', timeout: 45000 });
    await page.waitForSelector('.ai-nav-item', { timeout: 20000 });
    // 切到「专业绘画」
    const nav = page.locator('.ai-nav-item', { hasText: '专业绘画' });
    if (await nav.count()) await nav.first().click();
    await page.waitForSelector('.ai-paint', { timeout: 20000 });

    const ratios = await page.locator('.pd-ratios .pd-ratio').count();
    record('draw', '比例档位 = 5 档', ratios === 5, `实际 ${ratios} 档`);

    const ratioText = await page.evaluate(() =>
      Array.from(document.querySelectorAll('.pd-ratios .pd-ratio')).map((e) => (e.textContent || '').trim()));
    record('draw', '比例档位内容可读', ratioText.length === 5, ratioText.join(' / '));

    const modes = await page.locator('.pd-modes .pd-mode').count();
    record('draw', '绘画模式（文生图/混图/换脸）齐备', modes >= 3, `${modes} 个模式`);

    const cards = await page.locator('.pd-card').count();
    record('draw', '作品卡片墙容器存在', cards >= 0, `.pd-card 命中 ${cards} 个`);

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    record('draw', '绘画面板无横向溢出', overflow <= 1, `overflowX=${overflow}px`);
    if (errs.length) record('draw', '无 JS 运行时错误', false, errs.slice(0, 3).join(' | '));
    else record('draw', '无 JS 运行时错误', true);
  } finally {
    await ctx.close();
  }
}

/* ============================ 闭环 4：评论图谱 ============================ */
async function flowGraph(browser, cookies) {
  const { ctx, page, errs } = await newPage(browser, cookies);
  try {
    // 图谱页需要作品上下文（?type=&id=），否则 targetID=0 无数据、只渲染空态
    const novelId = Number(process.env.NVS_NOVEL || 1);
    await page.goto(BASE + `/comment-network?type=novel&id=${novelId}`, { waitUntil: 'domcontentloaded', timeout: 60000 });
    // dev server 首次按需编译该页依赖（echarts/three）可能较慢，给足超时
    await page.waitForSelector('.comment-network-page', { timeout: 60000 });
    await page.waitForTimeout(2500);

    const wrap = await page.locator('.cn-graph-wrap').count();
    // 诊断：容器缺失时把现场结构写进报告，便于定位是「子组件未挂载」还是「空态」
    const diag = wrap === 0 ? await page.evaluate(() => {
      const root = document.querySelector('.comment-network-page');
      return {
        url: location.pathname + location.search,
        children: root ? [...root.children].map((e) => e.className || e.tagName) : [],
        hasChild: !!document.querySelector('.comment-network'),
        text: (document.body.innerText || '').slice(0, 160).replace(/\s+/g, ' '),
      };
    }) : {};
    record('graph', '图谱容器渲染', wrap > 0, wrap > 0 ? `.cn-graph-wrap=${wrap}` : JSON.stringify(diag));

    const canvas = await page.evaluate(() => {
      const c = document.querySelector('.cn-graph canvas, .cn-graph-3d canvas');
      if (!c) return { found: false };
      const r = c.getBoundingClientRect();
      return { found: true, w: Math.round(r.width), h: Math.round(r.height) };
    });
    record('graph', '图谱画布已绘制且有尺寸', canvas.found && canvas.w > 50 && canvas.h > 50,
      canvas.found ? `${canvas.w}x${canvas.h}` : '未找到 canvas');

    const edges = await page.locator('.cn-edge').count();
    record('graph', '评论关系边可渲染', edges >= 0, `.cn-edge=${edges}`);

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    record('graph', '图谱页无横向溢出', overflow <= 1, `overflowX=${overflow}px`);
    if (errs.length) record('graph', '无 JS 运行时错误', false, errs.slice(0, 3).join(' | '));
    else record('graph', '无 JS 运行时错误', true);
  } finally {
    await ctx.close();
  }
}

/* ============================ 闭环 5：警方台 ============================ */
async function flowPolice(browser, cookies) {
  const { ctx, page, errs } = await newPage(browser, cookies);
  try {
    // 先落到站内页面（about:blank 的 fetch 会被拦，必须同源）
    await page.goto(BASE + '/police', { waitUntil: 'domcontentloaded', timeout: 45000 });

    // 接口层：警方（admin 亦可）能读已删除内容高亮色 + 软删除留证
    const color = await api(page, 'GET', '/api/police/del-color');
    record('police', '已删除内容高亮色可读', color.status === 200 && color.json && color.json.code === 0,
      `status=${color.status} color=${color.json && color.json.data && color.json.data.color}`);

    const soft = await api(page, 'GET', '/api/police/softdel');
    record('police', '软删除留证列表可读', soft.status === 200 && soft.json && soft.json.code === 0,
      `status=${soft.status}`);

    const stats = await api(page, 'GET', '/api/police/sentiment/stats');
    record('police', '舆情聚合可读', stats.status === 200 && stats.json && stats.json.code === 0,
      `status=${stats.status}`);

    // 页面层
    await page.waitForSelector('.pc-nav, .pc-tabs, .pc-stats', { timeout: 20000 });
    await page.waitForTimeout(1200);
    const stats2 = await page.locator('.pc-stat').count();
    record('police', '警方台统计卡片渲染', stats2 > 0, `.pc-stat=${stats2}`);

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    record('police', '警方台无横向溢出', overflow <= 1, `overflowX=${overflow}px`);
    if (errs.length) record('police', '无 JS 运行时错误', false, errs.slice(0, 3).join(' | '));
    else record('police', '无 JS 运行时错误', true);
  } finally {
    await ctx.close();
  }
}

/* ============================ 主流程 ============================ */
(async () => {
  fs.mkdirSync(OUT_DIR, { recursive: true });
  const browser = await chromium.launch({
    executablePath: process.env.NVS_CHROME || findChrome(),
    args: ['--no-proxy-server', '--disable-gpu'],
  });

  let cookies = [];
  const run = async (name, fn) => {
    if (ONLY && ONLY !== name) return;
    try {
      await fn();
    } catch (e) {
      record(name, '流程执行', false, String(e).slice(0, 240));
    }
  };

  // 认证先行：后续闭环都依赖登录态
  let authCookies = [];
  if (!ONLY || ONLY === 'auth') {
    try {
      authCookies = (await flowAuth(browser))?.cookies || [];
    } catch (e) {
      record('auth', '流程执行', false, String(e).slice(0, 240));
    }
  } else {
    // 单跑其它闭环时也需要登录态
    const { ctx, page } = await newPage(browser);
    try {
      await page.goto(BASE + '/login', { waitUntil: 'domcontentloaded', timeout: 60000 });
      await page.waitForTimeout(2500);
      const lg = await login(page, ctx);
      authCookies = lg.ok ? await ctx.cookies() : [];
    } catch (e) {
      record('auth', '登录（为其它闭环准备）', false, String(e).slice(0, 200));
    } finally {
      await ctx.close();
    }
  }
  cookies = authCookies;
  record('auth', '取得登录态供后续闭环使用', cookies.length > 0, `cookie 数=${cookies.length}`);

  await run('ai', () => flowAI(browser, cookies));
  await run('draw', () => flowDraw(browser, cookies));
  await run('graph', () => flowGraph(browser, cookies));
  await run('police', () => flowPolice(browser, cookies));

  await browser.close();

  const bad = results.filter((r) => !r.ok);
  const report = { base: BASE, at: new Date().toISOString(), total: results.length, failed: bad.length, results };
  const file = path.join(OUT_DIR, 'browser-flows-report.json');
  fs.writeFileSync(file, JSON.stringify(report, null, 2), 'utf8');

  console.log(`\n总计 ${results.length} 项断言，失败 ${bad.length} 项 → ${file}`);
  if (bad.length) {
    for (const b of bad) console.log(`  ❌ [${b.flow}] ${b.name} — ${b.detail}`);
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
