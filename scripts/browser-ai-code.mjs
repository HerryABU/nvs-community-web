#!/usr/bin/env node
/**
 * AI 代码块验收（真浏览器 · 量化断言）
 *
 * 覆盖：
 *   1. 代码块工具条：语言标签 / 复制 / HTML 专属的「预览」「发布」
 *   2. 语法高亮：hljs span 真的生成且带非默认颜色
 *   3. 复制：写进剪贴板的内容 == 原始代码（不含工具条文字）
 *   4. HTML 预览：沙箱 iframe 里 DOM 真的渲染出来（Playwright 可穿透 sandbox 读 DOM）
 *   5. 发布到 HTML 广场：单文件 ZIP 被后端接受（这条最重要 ——
 *      miniZip 的字节结构错了只有真实上传才暴露）
 *
 * 为什么用假 SSE：本脚本验的是「前端渲染 + 上传」链路，不是上游模型。
 * 用 route 拦截 /api/ai/chat/stream 注入固定代码块，断言可重复、不耗额度。
 *
 * 用法：
 *   node scripts/browser-ai-code.mjs
 *   NVS_PASSWORD=xxx node scripts/browser-ai-code.mjs
 *   node scripts/browser-ai-code.mjs --skip-publish   # 不真的发一条广场记录
 */
import path from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire('C:/Users/QBZ95/.workbuddy/binaries/node/workspace/');
const { chromium } = require('playwright-core');

const BASE = process.env.NVS_BASE || 'http://127.0.0.1:8000';
const PASSWORDS = (process.env.NVS_PASSWORD || 'admin123,Smoke@2026')
  .split(',')
  .map((s) => s.trim())
  .filter(Boolean);
const EMAIL = process.env.NVS_EMAIL || 'admin@nvs.local';
const SKIP_PUBLISH = process.argv.includes('--skip-publish');
/** --real：不拦 SSE，真的调一次上游模型（验证真实输出的写法也识别得出来） */
const REAL = process.argv.includes('--real');

const results = [];
function record(name, ok, detail = '') {
  results.push({ name, ok: !!ok, detail: String(detail).slice(0, 400) });
  console.log(`${ok ? '✅' : '❌'} ${name}${detail ? ' — ' + detail : ''}`);
}

const HTML_DOC = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>打地鼠小游戏</title>
<style>body{margin:0;display:grid;place-items:center;height:100vh;background:#0f172a;color:#e2e8f0;font-family:system-ui}
h1{font-size:20px}.grid{display:grid;grid-template-columns:repeat(3,80px);gap:8px}
.hole{width:80px;height:80px;border-radius:12px;background:#1e293b;cursor:pointer}
.hole.hit{background:#22c55e}</style>
</head>
<body>
<h1 id="tip">点方块得分</h1>
<div class="grid">
  <div class="hole" data-i="0"></div><div class="hole" data-i="1"></div><div class="hole" data-i="2"></div>
  <div class="hole" data-i="3"></div><div class="hole" data-i="4"></div><div class="hole" data-i="5"></div>
</div>
<script>
let score = 0;
document.querySelectorAll('.hole').forEach(function (el) {
  el.addEventListener('click', function () {
    score += 1;
    el.classList.add('hit');
    document.getElementById('tip').textContent = '得分 ' + score;
    // 沙箱内没有真实 localStorage，验证内存兜底是否生效
    try { localStorage.setItem('score', String(score)); } catch (e) { document.title = 'storage-error'; }
  });
});
</script>
</body>
</html>`;

const PY_CODE = `def fib(n):
    a, b = 0, 1
    for _ in range(n):
        yield a
        a, b = b, a + b

print(list(fib(8)))`;

const JS_SNIPPET = `const sum = (a, b) => a + b;
console.log(sum(1, 2));`;

/** 没有语言标注、但确实是 HTML —— AI 输出里极常见，必须也能预览/发布 */
const BARE_HTML = `<!DOCTYPE html>
<html><body><h2 id="banner">未标注的 HTML 也能预览</h2></body></html>`;

const ANSWER = [
  '给你一个可以直接玩的小游戏：',
  '',
  '```html',
  HTML_DOC,
  '```',
  '',
  '对应的 Python 版本：',
  '',
  '```python',
  PY_CODE,
  '```',
  '',
  '调用方式：',
  '',
  '```',
  JS_SNIPPET,
  '```',
  '',
  '顺手一个（没写语言标注）：',
  '',
  '```',
  BARE_HTML,
  '```',
].join('\n');

// ── 滑块 + 登录 ──
// 算法与 scripts/browser-flows.mjs 完全一致（缺口是 42×42 半透明黑方块，
// 在图片纵向中带的 42px 窗口里按列投票找左边界）。此处内联而非抽公共模块，
// 是为了不动既有 CI 脚本；两边如需一起改，以 browser-flows.mjs 为准。
async function solveCaptcha(page) {
  const attempt = async () =>
    await page.evaluate(async () => {
      const r = await fetch('/api/auth/captcha/issue', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
        credentials: 'include',
      });
      const j = await r.json();
      const ch = j && j.data;
      if (!ch) return { err: 'issue 未返回 data' };
      const img = new Image();
      await new Promise((res, rej) => {
        img.onload = res;
        img.onerror = () => rej(new Error('bg 解码失败'));
        img.src = ch.bg_image;
      });
      const c = document.createElement('canvas');
      c.width = img.width;
      c.height = img.height;
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
          if (lums[x] < base * 0.85) {
            if (start < 0) start = x;
          } else if (start >= 0) {
            runs.push([start, x - 1]);
            start = -1;
          }
        }
        if (start >= 0) runs.push([start, c.width - 1]);
        for (const [s, e] of runs) {
          if (e - s + 1 >= 30) votes.set(s, (votes.get(s) || 0) + 1);
        }
      }
      let best = -1;
      let bestV = 0;
      for (const [k, v] of votes) {
        if (v > bestV) {
          bestV = v;
          best = k;
        }
      }
      return { challenge_id: ch.challenge_id, x: best, votes: bestV };
    });

  let lastErr = '';
  for (let i = 0; i < 4; i++) {
    try {
      const s = await attempt();
      if (!s.err && s.x >= 0 && s.votes >= 10) return s;
      lastErr = s.err || `x=${s.x} votes=${s.votes}`;
    } catch (e) {
      lastErr = String(e).slice(0, 120);
    }
    await page.waitForTimeout(1200);
  }
  return { err: lastErr || '滑块定位失败' };
}

/**
 * 设备指纹：登录请求必须和后续 axios 请求带**同一个**指纹。
 * 访问令牌与「指纹 + 登录地」强绑定，登录时没带指纹的话，
 * 之后任何带指纹的请求都会被判「登录环境已变更」——验收脚本自己会把自己踢掉。
 * 做法：先在未登录状态打开 /ai 触发一次 axios 请求，从请求头里把它真实用的指纹抓出来。
 */
let FINGERPRINT = '';

async function apiFetch(page, method, p, body) {
  return await page.evaluate(
    async ([m, pp, b, base, fp]) => {
      const headers = { 'X-Requested-With': 'XMLHttpRequest' };
      if (fp) headers['X-Device-Fingerprint'] = fp;
      const init = { method: m, headers, credentials: 'include' };
      if (b !== null && b !== undefined) {
        init.headers['Content-Type'] = 'application/json';
        init.body = JSON.stringify(b);
      }
      const r = await fetch(base + pp, init);
      let j = null;
      try {
        j = await r.json();
      } catch {
        j = null;
      }
      return { status: r.status, json: j };
    },
    [method, p, body ?? null, BASE, FINGERPRINT],
  );
}

async function login(page, ctx) {
  const trace = [];
  for (const pw of PASSWORDS) {
    const sol = await solveCaptcha(page);
    if (sol.err) {
      trace.push(`滑块: ${sol.err}`);
      continue;
    }
    const v = await apiFetch(page, 'POST', '/api/auth/captcha/verify', {
      challenge_id: sol.challenge_id,
      x: sol.x,
    });
    const token = v.json?.data?.captcha_token;
    if (!token) {
      trace.push(`verify x=${sol.x} votes=${sol.votes} → ${v.status} ${JSON.stringify(v.json)?.slice(0, 120)}`);
      continue;
    }
    const r = await apiFetch(page, 'POST', '/api/auth/login', { email: EMAIL, password: pw, captcha_token: token });
    if (r.status === 200 && r.json?.code === 0) {
      const t = r.json.data?.token || r.json.data?.access_token;
      const jar = await ctx.cookies();
      if (t && !jar.some((c) => c.name === 'token')) {
        await ctx.addCookies([{ name: 'token', value: t, url: BASE }]);
      }
      return { ok: true, password: pw };
    }
    trace.push(`login ${pw} → ${r.status} ${JSON.stringify(r.json)?.slice(0, 160)}`);
    if (/过于频繁/.test(r.json?.message || '')) {
      await page.waitForTimeout(15000);
    }
  }
  return { ok: false, why: trace.join(' | ') || '所有口令均登录失败' };
}

// ── 主流程 ──────────────────────────────────────────────────────────
const browser = await chromium.launch({
  executablePath:
    process.env.NVS_CHROME ||
    'C:/Users/QBZ95/AppData/Local/ms-playwright/chromium-1148/chrome-win/chrome.exe',
  args: ['--no-sandbox', '--disable-dev-shm-usage'],
});
const ctx = await browser.newContext({
  viewport: { width: 1440, height: 940 },
  permissions: ['clipboard-read', 'clipboard-write'],
});
await ctx.addInitScript(() => {
  try {
    localStorage.clear();
    sessionStorage.clear();
  } catch {
    /* ignore */
  }
});

const page = await ctx.newPage();
const errs = [];
page.on('console', (m) => {
  if (m.type() !== 'error') return;
  const t = m.text();
  if (t.startsWith('Failed to load resource')) return;
  errs.push('[console] ' + t.slice(0, 200));
});
page.on('pageerror', (e) => errs.push('[pageerror] ' + String(e).slice(0, 200)));

try {
  // 先抓页面真实指纹（未登录也会发公开接口请求，拦截器对它们同样附加指纹头）
  page.on('request', (r) => {
    const h = r.headers()['x-device-fingerprint'];
    if (h) FINGERPRINT = h;
  });
  await page.goto(`${BASE}/ai`, { waitUntil: 'domcontentloaded' });
  await page.waitForTimeout(1600);
  console.log(`   ↳ 设备指纹：${FINGERPRINT ? FINGERPRINT.slice(0, 24) + '…' : '(页面未产生，按无指纹登录)'}`);

  await page.goto(`${BASE}/login`, { waitUntil: 'domcontentloaded' });
  const lg = await login(page, ctx);
  record('登录测试站', lg.ok, lg.ok ? `口令=${lg.password}` : lg.why);
  if (!lg.ok) throw new Error('登录失败，终止验收');

  // 注入假 SSE：4 个代码块（html 完整文档 / python / 无标注 JS / 无标注 HTML）
  // --real 时不拦截，让请求真的打到上游
  if (!REAL) {
    await page.route('**/api/ai/chat/stream', async (route) => {
      const chunks = ANSWER.match(/[\s\S]{1,64}/g) || [];
      let body = '';
      for (const c of chunks) {
        body += `data: ${JSON.stringify({ content: c })}\n\n`;
      }
      body += `data: ${JSON.stringify({ done: true, cost: 0, free: true, balance_after: 0, session_id: 0 })}\n\n`;
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
        body,
      });
    });
  }

  await page.goto(`${BASE}/ai`, { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('.ai-chat textarea', { timeout: 20000 });

  // 新建对话（避免污染历史会话）
  const newBtn = page.locator('button:has-text("新对话"), button:has-text("新建对话")').first();
  if (await newBtn.count()) {
    await newBtn.click().catch(() => {});
    await page.waitForTimeout(400);
  }

  const prompt = REAL
    ? '只输出一个 HTML 代码块（```html 开头），页面上放一个按钮和一个数字，点按钮数字加一。不要任何解释。'
    : '给我一个打地鼠小游戏的 HTML，再给个 Python 版';

  // 真实模式下先探一次 stream 端点：拿到明确的 status + 响应体，
  // 否则「等了 120 秒没内容」这种失败根本看不出是认证、限流还是上游超时。
  if (REAL) {
    const probe = await page.evaluate(async (fp) => {
      const headers = { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' };
      if (fp) headers['X-Device-Fingerprint'] = fp;
      const r = await fetch('/api/ai/chat/stream', {
        method: 'POST',
        headers,
        credentials: 'include',
        body: JSON.stringify({ model_id: 0, message: '' }),
      });
      let body = '';
      try {
        body = (await r.text()).slice(0, 200);
      } catch {
        /* ignore */
      }
      return { status: r.status, body };
    }, FINGERPRINT);
    console.log(`   ↳ stream 预检：status=${probe.status} body=${probe.body}`);
    record('令牌绑定校验通过（预检非 401）', probe.status !== 401, `status=${probe.status} ${probe.body.slice(0, 80)}`);
  }

  await page.locator('.ai-chat textarea').first().fill(prompt);
  await page.locator('.ai-send-btn').first().click();

  // 等流式结束：假数据 4 个代码块；真实上游只要有 1 个带 HTML 的块即可
  await page.waitForFunction(
    (min) => document.querySelectorAll('.ai-msg-md pre').length >= min,
    REAL ? 1 : 4,
    { timeout: REAL ? 240000 : 30000 },
  );
  if (REAL) {
    // 真实上游继续吐 delta，等「停止」按钮消失才算生成完
    await page
      .waitForFunction(() => !document.querySelector('.ai-send-btn.is-stop'), null, { timeout: 120000 })
      .catch(() => {});
  }
  await page.waitForTimeout(900); // 等高亮调度（节流 260ms）跑完

  // ── 1. 工具条结构 ──
  const bars = await page.evaluate(() => {
    return [...document.querySelectorAll('.ai-msg-md pre')].map((pre) => {
      const bar = pre.querySelector('.code-bar');
      const acts = [...(bar?.querySelectorAll('.code-act') || [])].map((b) => b.dataset.act);
      return {
        lang: bar?.querySelector('.code-lang')?.textContent || '',
        acts,
        hasCode: !!pre.querySelector('code'),
        barInsidePre: !!bar,
        dataLang: pre.closest('[data-lang]')?.getAttribute('data-lang') ?? '(无属性)',
        textLen: (pre.querySelector('code')?.textContent || '').length,
      };
    });
  });
  const hIdx = REAL ? bars.findIndex((b) => b.acts.includes('preview')) : 0;

  record(REAL ? '真实上游返回代码块' : '代码块数量 = 4', REAL ? bars.length >= 1 : bars.length === 4, `实际 ${bars.length}`);
  record('每个代码块都有工具条', bars.every((b) => b.barInsidePre), JSON.stringify(bars.map((b) => b.lang)));
  if (!REAL) {
    record(
      '语言标签正确（HTML/PYTHON/空/HTML）',
      bars[0]?.lang === 'HTML' && bars[1]?.lang === 'PYTHON' && bars[2]?.lang === '' && bars[3]?.lang === 'HTML',
      JSON.stringify(bars.map((b) => b.lang)),
    );
  }
  record(
    'HTML 块有 预览+发布+复制',
    hIdx >= 0 && JSON.stringify(bars[hIdx].acts) === JSON.stringify(['preview', 'publish', 'copy']),
    hIdx >= 0 ? JSON.stringify(bars[hIdx].acts) : '没有找到 HTML 块',
  );
  if (!REAL) {
    record(
      '非 HTML 块只有 复制',
      JSON.stringify(bars[1]?.acts) === JSON.stringify(['copy']) && JSON.stringify(bars[2]?.acts) === JSON.stringify(['copy']),
      JSON.stringify([bars[1]?.acts, bars[2]?.acts]),
    );
    record(
      '未标注语言的 HTML 也能识别（不被 javascript fallback 骗）',
      JSON.stringify(bars[3]?.acts) === JSON.stringify(['preview', 'publish', 'copy']),
      `data-lang=${JSON.stringify(bars[3]?.dataLang)} acts=${JSON.stringify(bars[3]?.acts)}`,
    );
  }

  // ── 2. 语法高亮 ──
  const hl = await page.evaluate(() => {
    const out = [];
    for (const pre of document.querySelectorAll('.ai-msg-md pre')) {
      const spans = [...pre.querySelectorAll('span[class*="hljs-"]')];
      const colored = spans.filter((s) => {
        const c = getComputedStyle(s).color;
        return c && c !== 'rgb(226, 232, 240)' && c !== 'rgba(0, 0, 0, 0)';
      });
      out.push({
        spans: spans.length,
        colored: colored.length,
        sampleColor: colored[0] ? getComputedStyle(colored[0]).color : '',
        sampleClass: colored[0]?.className || '',
      });
    }
    return out;
  });
  record('hljs 生成高亮 span', hl.every((h) => h.spans > 0), JSON.stringify(hl.map((h) => h.spans)));
  record(
    '高亮配色生效（非默认白）',
    hl.every((h) => h.colored > 0),
    hl.map((h) => `${h.sampleClass}=${h.sampleColor}`).join(' | '),
  );

  // ── 3. 复制（内容必须等于源码，不含工具条文字）──
  const firstPre = page.locator('.ai-msg-md pre').nth(Math.max(hIdx, 0));
  await firstPre.locator('.code-act[data-act="copy"]').click();
  await page.waitForTimeout(260);
  const copied = await page.evaluate(() => navigator.clipboard.readText());
  const copyOk = REAL
    ? copied.length > 60 && copied.includes('<') && !copied.includes('已复制') && !copied.includes('预览')
    : copied.includes('打地鼠小游戏') &&
      copied.includes('document.querySelectorAll') &&
      !copied.includes('已复制') &&
      !copied.includes('预览');
  record('复制内容 = 代码原文（无工具条文字）', copyOk, `长度 ${copied.length}`);
  const btnText = await firstPre.locator('.code-act[data-act="copy"]').textContent();
  record('复制按钮有成功反馈', btnText.trim() === '已复制', btnText.trim());

  // ── 4. HTML 预览 ──
  await firstPre.locator('.code-act[data-act="preview"]').click();
  await page.waitForSelector('.ai-html-preview-dialog iframe.hp-frame', { timeout: 8000 });
  await page.waitForTimeout(1200);

  const sandboxAttr = await page.locator('iframe.hp-frame').first().getAttribute('sandbox');
  record(
    '预览 iframe 沙箱不含 allow-same-origin',
    !!sandboxAttr && sandboxAttr.includes('allow-scripts') && !sandboxAttr.includes('allow-same-origin'),
    sandboxAttr || '(无 sandbox，危险)',
  );

  // Playwright 走 CDP，可以穿透 sandbox 读 iframe 内 DOM
  const frames = page.frames().filter((f) => f !== page.mainFrame());
  let frameText = '';
  for (const f of frames) {
    try {
      frameText += await f.evaluate(() => document.body?.innerText || '');
    } catch {
      /* 已被替换的 frame */
    }
  }
  // 真实上游的文案不可控，这里只验「iframe 里确实渲染出了可见文本」
  const expectText = REAL ? frameText.trim().length > 0 : frameText.includes('点方块得分');
  record('预览 iframe 真的渲染出内容', expectText, `innerText=${JSON.stringify(frameText.slice(0, 60))}`);

  // 验证沙箱内脚本真的在跑：点击页面里的可交互元素，DOM 有变化即说明 JS 生效。
  // 假数据用例点 .hole（顺带验证 localStorage 兜底），真实上游的页面长什么样不可控，
  // 退化为「点第一个 button 看 innerText 变没变」。
  let clickOk = false;
  let clickNote = '';
  for (const f of frames) {
    try {
      const hole = f.locator('.hole').first();
      if (await hole.count()) {
        await hole.click({ timeout: 2000 });
        await f.waitForTimeout(120);
        const tip = await f.locator('#tip').innerText();
        const title = await f.title();
        clickOk = tip.includes('得分 1') && title !== 'storage-error';
        clickNote = '得分已更新且 localStorage 兜底生效';
        break;
      }
      const btn = f.locator('button').first();
      if (await btn.count()) {
        const before = await f.evaluate(() => document.body.innerText);
        await btn.click({ timeout: 2000 });
        await f.waitForTimeout(180);
        const after = await f.evaluate(() => document.body.innerText);
        clickOk = before !== after;
        clickNote = clickOk ? '点击后 DOM 已变化' : '点击后 DOM 无变化';
        break;
      }
    } catch (e) {
      clickNote = String(e).slice(0, 80);
    }
  }
  record('沙箱内脚本可执行', clickOk, clickNote);

  await page.keyboard.press('Escape');
  await page.locator('.ai-html-preview-dialog .hp-actions button:has-text("关闭")').first().click().catch(() => {});
  await page.waitForTimeout(400);

  // ── 5. 发布到广场（真实上传，验证 ZIP 字节结构）──
  await firstPre.locator('.code-act[data-act="publish"]').click();
  await page.waitForSelector('.ai-html-publish-dialog', { timeout: 8000 });
  await page.waitForTimeout(300);
  const nameVal = await page.locator('.ai-html-publish-dialog input').first().inputValue();
  record(
    '发布弹窗有默认名称',
    nameVal.trim().length > 0,
    nameVal + (REAL ? '' : nameVal.includes('打地鼠') ? '（取自 <title>）' : '（未取到 title）'),
  );

  if (SKIP_PUBLISH) {
    record('发布（已跳过）', true, '--skip-publish');
  } else {
    await page.locator('.ai-html-publish-dialog .el-dialog__footer button:has-text("发布")').first().click();
    // 等结果：成功文案或错误提示
    const outcome = await page
      .waitForFunction(
        () => {
          const d = document.querySelector('.ai-html-publish-dialog');
          if (!d) return null;
          const err = d.querySelector('.el-alert--error .el-alert__title');
          const okEl = d.querySelector('.el-alert--success .el-alert__title');
          const warnEl = d.querySelector('.el-alert--warning .el-alert__title');
          if (err) return { kind: 'error', text: err.textContent };
          if (okEl) return { kind: 'ok', text: okEl.textContent };
          if (warnEl) return { kind: 'warn', text: warnEl.textContent };
          return null;
        },
        null,
        { timeout: 45000 },
      )
      .then((h) => h.jsonValue())
      .catch(() => null);
    record('单文件 ZIP 被后端接受（发布成功）', outcome && outcome.kind !== 'error', JSON.stringify(outcome));

    // 事后清理：按「刚发布的那个名字」精确删，绝不按前缀/模糊匹配误删用户数据
    if (outcome && outcome.kind !== 'error') {
      const list = await apiFetch(page, 'GET', '/api/userhtmls');
      const d = list.json?.data;
      const items = Array.isArray(d) ? d : Array.isArray(d?.items) ? d.items : [];
      const mine = items.find((x) => String(x.name || '') === nameVal.trim());
      if (mine?.id) {
        const del = await apiFetch(page, 'DELETE', `/api/userhtmls/${mine.id}`);
        record('清理验收记录', del.status === 200 || del.json?.code === 0, `id=${mine.id} status=${del.status}`);
      } else {
        record('清理验收记录', false, `列表里没找到 name=${nameVal.trim()}（共 ${items.length} 条），请手动清理`);
      }
    }
  }

  // ── 控制台无报错 ──
  record('页面无 JS 异常', errs.length === 0, errs.slice(0, 3).join(' | '));
} catch (e) {
  // 失败时把页面尾部文本一起打出来：ElMessage 的错误提示是唯一能区分
  // 「上游超时 / 额度不足 / 内容拦截 / 模型未配置」的线索
  const dump = await page
    .evaluate(() => document.body.innerText.replace(/\s+/g, ' ').slice(-420))
    .catch(() => '');
  console.log('   ↳ 页面尾部：' + dump);
  record('验收流程异常', false, String(e).slice(0, 200));
} finally {
  await browser.close();
}

const bad = results.filter((r) => !r.ok);
console.log('\n────────────────────────────');
console.log(`总计 ${results.length} 项，失败 ${bad.length} 项`);
if (bad.length) {
  for (const b of bad) console.log(`  ❌ ${b.name} — ${b.detail}`);
  process.exitCode = 1;
}
