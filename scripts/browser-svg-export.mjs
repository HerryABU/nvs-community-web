/**
 * SVG / mermaid 导出能力的真浏览器验收
 *
 * 验三件事：
 *  1. markdown 里的 mermaid 图渲染出工具条（SVG / PNG / 源码三个按钮）；
 *  2. 点 PNG 真的产出合法 PNG（校验文件头魔数），SVG 产出含 <svg 的文件，
 *     源码产出含 mermaid 源文的文件；
 *  3. AI 回复里的 SVG 代码块能被识别为可预览块，弹窗标题变成「SVG 预览」并能导出 PNG。
 *
 * 用法：NVS_BASE=http://127.0.0.1:8001 NVS_CAPTCHA_TOKEN=xxx node scripts/browser-svg-export.mjs
 */
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
const require = createRequire('C:/Users/QBZ95/.workbuddy/binaries/node/workspace/');
const { chromium } = require('playwright-core');

const BASE = process.env.NVS_BASE || 'http://127.0.0.1:8001';
const OUT = process.env.NVS_SHOT_DIR || 'C:/tmp/nvs_qa/shots-svg';
const MERMAID = `\`\`\`mermaid
flowchart TD
  A[登录] --> B{校验}
  B -->|通过| C[首页]
  B -->|失败| D[报错]
\`\`\``;

function findChrome() {
  const base = path.join(process.env.LOCALAPPDATA || '', 'ms-playwright');
  for (const d of fs.readdirSync(base).filter((x) => x.startsWith('chromium-')).sort().reverse()) {
    const exe = path.join(base, d, 'chrome-win', 'chrome.exe');
    if (fs.existsSync(exe)) return exe;
  }
}

const results = [];
const ok = (name, pass, detail = '') => {
  results.push({ name, pass, detail });
  console.log(`${pass ? '✅' : '❌'} ${name}${detail ? ' — ' + detail : ''}`);
};

const browser = await chromium.launch({ executablePath: findChrome(), args: ['--no-proxy-server'] });
const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 }, acceptDownloads: true });
const page = await ctx.newPage();
page.on('console', (m) => { if (m.type() === 'error' || m.text().includes('图')) console.log('   [console]', m.text().slice(0, 160)); });
page.on('pageerror', (e) => console.log('   [pageerror]', String(e).slice(0, 160)));

let fp = '';
page.on('request', (r) => { const h = r.headers()['x-device-fingerprint']; if (h && !fp) fp = h; });
await page.goto(BASE + '/login', { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(2500);

// ── 登录 ──
const login = await ctx.request.post(BASE + '/api/auth/login', {
  data: { email: 'admin@nvs.local', password: 'NTsuper123456', captcha_token: process.env.NVS_CAPTCHA_TOKEN || '' },
  headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest', 'X-Device-Fingerprint': fp },
});
const lb = await login.json().catch(() => ({}));
ok('登录（浏览器上下文，指纹一致）', lb.code === 0, JSON.stringify(lb).slice(0, 80));
if (lb.code !== 0) { await browser.close(); process.exit(2); }

// ── 1. 建一篇带 mermaid 的博客（走真实 API，文件与签名由服务端处理）──
const created = await ctx.request.post(BASE + '/api/blogs', {
  data: {
    title: 'QA 图表导出验收',
    summary: 'mermaid 导出工具条验收',
    content: `# 图表导出验收\n\n${MERMAID}\n\n结束。`,
  },
  headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest', 'X-Device-Fingerprint': fp },
});
const cb = await created.json().catch(() => ({}));
const blogId = cb?.data?.id || cb?.data?.blog?.id;
const blogAuthor = cb?.data?.author_id || 1;
ok('创建带 mermaid 的博客', !!blogId, `id=${blogId} ${JSON.stringify(cb).slice(0, 100)}`);

if (blogId) {
  await page.goto(`${BASE}/author/${blogAuthor}/blogs/${blogId}`, { waitUntil: 'networkidle', timeout: 45000 }).catch(() => {});
  // mermaid 是异步渲染的：等引擎 + mermaid.run() 都完成
  await page.waitForTimeout(4000);

  // ── 2. 工具条是否出现 ──
  const bar = await page.locator('.diagram-toolbar').first();
  const hasBar = await bar.count().catch(() => 0);
  ok('mermaid 图旁出现导出工具条', hasBar > 0);
  if (hasBar) {
    const labels = await bar.locator('.diagram-toolbar-btn').allTextContents();
    ok('工具条含 SVG / PNG / 源码 三个按钮', labels.length === 3, labels.join(','));
  }
  const svgCount = await page.locator('.mermaid-container svg').count();
  ok('mermaid 已渲染成 SVG', svgCount > 0, `svg=${svgCount}`);

  // ── 3. 三个导出按钮 ──
  for (const [label, ext, check] of [
    ['PNG', 'png', (buf) => buf.slice(0, 8).equals(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])) && buf.length > 1000],
    ['SVG', 'svg', (buf) => buf.toString('utf8').includes('<svg')],
    ['源码', 'mmd', (buf) => buf.toString('utf8').includes('flowchart')],
  ]) {
    const diag = await page.evaluate(() => {
      const bs = Array.from(document.querySelectorAll('.diagram-toolbar-btn'));
      return bs.map((b) => ({ t: b.textContent, connected: b.isConnected, pe: getComputedStyle(b).pointerEvents, op: getComputedStyle(b).opacity }));
    }).catch((e) => [{ err: String(e).slice(0, 80) }]);
    console.log('   [diag]', JSON.stringify(diag));
    const btn = page.locator('.diagram-toolbar-btn', { hasText: label }).first();
    if (!(await btn.count().catch(() => 0))) { ok(`导出 ${label}`, false, '按钮不存在'); continue; }
    try {
      const [download] = await Promise.all([
        page.waitForEvent('download', { timeout: 20000 }),
        btn.click(),
      ]);
      const p = path.join(OUT, `mermaid-${label.toLowerCase()}.${ext}`);
      fs.mkdirSync(OUT, { recursive: true });
      await download.saveAs(p);
      const buf = fs.readFileSync(p);
      ok(`导出 ${label}`, check(buf), `${buf.length} bytes → ${path.basename(p)}`);
    } catch (e) {
      ok(`导出 ${label}`, false, String(e).slice(0, 90));
    }
  }
}

// ── 4. AI 回复里的 SVG 代码块 → 可预览 + 导出 PNG ──
await page.goto(BASE + '/ai', { waitUntil: 'networkidle', timeout: 45000 }).catch(() => {});
await page.waitForTimeout(2000);
const input = page.locator('textarea').first();
if (await input.count().catch(() => 0)) {
  await input.fill('请只输出一个 SVG 代码块，画一个半径 60 的红色实心圆，不要别的解释文字。');
  await input.press('Enter');
  // 等 AI 回复（最多 90 秒）
  await page.waitForTimeout(9000);
  const previewBtns = await page.locator('.code-act', { hasText: '预览' }).count();
  ok('AI 的 SVG 代码块出现「预览」按钮', previewBtns > 0, `预览按钮数=${previewBtns}`);
  if (previewBtns > 0) {
    await page.locator('.code-act', { hasText: '预览' }).last().click();
    await page.waitForTimeout(1500);
    const title = await page.locator('.ai-html-preview-dialog .hp-title').textContent().catch(() => '');
    ok('预览弹窗标题识别为 SVG 预览', /SVG/.test(title || ''), (title || '').trim());
    const pngBtn = page.locator('.ai-html-preview-dialog button', { hasText: '导出 PNG' });
    ok('预览弹窗有「导出 PNG」按钮', (await pngBtn.count().catch(() => 0)) > 0);
    if (await pngBtn.count().catch(() => 0)) {
      try {
        const [download] = await Promise.all([
          page.waitForEvent('download', { timeout: 20000 }),
          pngBtn.click(),
        ]);
        const p = path.join(OUT, 'ai-svg.png');
        fs.mkdirSync(OUT, { recursive: true });
        await download.saveAs(p);
        const buf = fs.readFileSync(p);
        const isPng = buf.slice(0, 8).equals(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]));
        ok('SVG 预览导出 PNG', isPng && buf.length > 500, `${buf.length} bytes`);
      } catch (e) {
        ok('SVG 预览导出 PNG', false, String(e).slice(0, 90));
      }
    }
  }
} else {
  ok('AI 输入框存在', false);
}

fs.mkdirSync(OUT, { recursive: true });
await page.screenshot({ path: path.join(OUT, 'final.png') }).catch(() => {});
await browser.close();

const bad = results.filter((r) => !r.pass);
console.log(`\n共 ${results.length} 项，失败 ${bad.length} 项；截图与产物 → ${OUT}`);
process.exit(bad.length ? 1 : 0);
