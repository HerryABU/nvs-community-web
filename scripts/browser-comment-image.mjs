/**
 * 评论附图 + 楼加密可见性的真浏览器验收
 *
 * 验三件事：
 *  1. 评论框能上传图片（走 /api/author/image），出现缩略图，可移除；
 *  2. 提交后评论带图落库，列表里能渲染出 <img>，点击可放大预览；
 *  3. 楼加密：未解锁时楼正文显示 🔒 占位，但**楼内回复（评论区）照常可见**
 *     —— 这是「楼加密 ≠ 评论区加密」的核心断言。
 *
 * 用法：NVS_BASE=http://127.0.0.1:8001 NVS_CAPTCHA_TOKEN=xxx node scripts/browser-comment-image.mjs
 */
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
const require = createRequire('C:/Users/QBZ95/.workbuddy/binaries/node/workspace/');
const { chromium } = require('playwright-core');

const BASE = process.env.NVS_BASE || 'http://127.0.0.1:8001';
const OUT = process.env.NVS_SHOT_DIR || 'C:/tmp/nvs_qa/shots-comment-img';
fs.mkdirSync(OUT, { recursive: true });

function findChrome() {
  const base = path.join(process.env.LOCALAPPDATA || '', 'ms-playwright');
  for (const d of fs.readdirSync(base).filter((x) => x.startsWith('chromium-')).sort().reverse()) {
    const exe = path.join(base, d, 'chrome-win', 'chrome.exe');
    if (fs.existsSync(exe)) return exe;
  }
}

// 造一张真 PNG（16×16 渐变）。
// ⚠️ 不能手拼 base64：CRC 一旦不对，Go 的 png 解码器会直接报「文件已损坏」，
// 而服务端 UploadImage 是「解码 → 重编码防图片马」，解码失败就整张拒收。
// 这里用 zlib 现场编码，保证 chunk CRC 正确。
function makeTestPng() {
  const zlib = require('node:zlib');
  const W = 16, H = 16;
  const raw = Buffer.alloc(H * (1 + W * 3));
  let o = 0;
  for (let y = 0; y < H; y++) {
    raw[o++] = 0; // filter: none
    for (let x = 0; x < W; x++) {
      raw[o++] = (x * 16) % 256;
      raw[o++] = (y * 16) % 256;
      raw[o++] = 128;
    }
  }
  const chunk = (type, data) => {
    const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
    const body = Buffer.concat([Buffer.from(type, 'ascii'), data]);
    const crc = Buffer.alloc(4); crc.writeUInt32BE(zlib.crc32 ? zlib.crc32(body) >>> 0 : crc32(body));
    return Buffer.concat([len, body, crc]);
  };
  // Node 20+ 有 zlib.crc32；老版本退回手算
  function crc32(buf) {
    let c, crc = 0xffffffff;
    for (let n = 0; n < buf.length; n++) {
      c = (crc ^ buf[n]) & 0xff;
      for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
      crc = (crc >>> 8) ^ c;
    }
    return (crc ^ 0xffffffff) >>> 0;
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(W, 0); ihdr.writeUInt32BE(H, 4);
  ihdr[8] = 8; ihdr[9] = 2; ihdr[10] = 0; ihdr[11] = 0; ihdr[12] = 0;
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', zlib.deflateSync(raw)),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

const results = [];
const ok = (name, pass, detail = '') => {
  results.push({ name, pass, detail });
  console.log(`${pass ? '✅' : '❌'} ${name}${detail ? ' — ' + detail : ''}`);
};

const browser = await chromium.launch({ executablePath: findChrome(), args: ['--no-proxy-server'] });
const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 }, acceptDownloads: true });
const page = await ctx.newPage();
page.on('pageerror', (e) => console.log('   [pageerror]', String(e).slice(0, 200)));
page.on('response', async (r) => {
  if (r.url().includes('/api/author/image')) {
    console.log('   [upload]', r.status(), (await r.text().catch(() => '')).slice(0, 160));
  }
});

let fp = '';
page.on('request', (r) => { const h = r.headers()['x-device-fingerprint']; if (h && !fp) fp = h; });
await page.goto(BASE + '/login', { waitUntil: 'domcontentloaded' });
await page.waitForTimeout(2500);

const login = await ctx.request.post(BASE + '/api/auth/login', {
  data: { email: 'admin@nvs.local', password: 'NTsuper123456', captcha_token: process.env.NVS_CAPTCHA_TOKEN || '' },
  headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest', 'X-Device-Fingerprint': fp },
});
const lb = await login.json().catch(() => ({}));
ok('登录（浏览器上下文，指纹一致）', lb.code === 0, JSON.stringify(lb).slice(0, 100));
if (lb.code !== 0) { await browser.close(); process.exit(2); }

const H = { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest', 'X-Device-Fingerprint': fp };

// ── 准备：建一篇博客作为评论宿主 ──
const created = await ctx.request.post(BASE + '/api/blogs', {
  data: { title: 'QA 评论附图验收', summary: '评论附图 + 楼加密可见性', content: '评论附图验收用博客。' },
  headers: H,
});
const cb = await created.json().catch(() => ({}));
const blogId = cb?.data?.id || cb?.data?.blog?.id;
const blogAuthor = cb?.data?.author_id || 1;
ok('创建评论宿主博客', !!blogId, `blogId=${blogId}`);
if (!blogId) { await browser.close(); process.exit(2); }

// ── 1. 后台：评论区策略默认「不连带隐藏楼内回复」──
const polRes = await ctx.request.get(BASE + '/api/admin/comment-policy', { headers: H });
const pol = await polRes.json().catch(() => ({}));
const hides = pol?.data?.config?.lock_hides_replies;
ok('评论区策略可读且默认不连带隐藏', hides === false, `lock_hides_replies=${hides}`);

// ── 2. 楼加密场景：造一条加密楼 + 一条楼内回复 ──
const floorRes = await ctx.request.post(BASE + '/api/comments', {
  data: { blog_id: blogId, content: '这是一条加密楼，验证楼内回复是否可见。' },
  headers: H,
});
const fr = await floorRes.json().catch(() => ({}));
const floorId = fr?.data?.id || fr?.data?.comment?.id;
ok('创建加密楼', !!floorId, `floorId=${floorId}`);

// 给该楼设密码
if (floorId) {
  const pwdRes = await ctx.request.post(`${BASE}/api/comments/${floorId}/password`, {
    data: { password: 'qa-pass-123' },
    headers: H,
  });
  const pr = await pwdRes.json().catch(() => ({}));
  ok('设置楼访问密码', pr?.code === 0, JSON.stringify(pr).slice(0, 100));

  // 用另一个用户身份回复（楼内回复）
  const replyRes = await ctx.request.post(BASE + '/api/comments', {
    data: { blog_id: blogId, content: '这是一条楼内回复，应该在评论区照常可见。', parent_id: floorId },
    headers: H,
  });
  const rr = await replyRes.json().catch(() => ({}));
  ok('创建楼内回复', rr?.code === 0, JSON.stringify(rr).slice(0, 120));
}

// ── 3. 评论附图：上传 → 预览 → 提交 ──
await page.goto(`${BASE}/author/${blogAuthor}/blogs/${blogId}`, { waitUntil: 'networkidle', timeout: 45000 }).catch(() => {});
await page.waitForTimeout(3000);

const uploadInput = page.locator('.cc-img-upload input[type=file]').first();
const hasUpload = await uploadInput.count();
ok('评论框出现「附图」上传入口', hasUpload > 0, `count=${hasUpload}`);

if (hasUpload > 0) {
  const tmp = 'C:/tmp/nvs_qa/qa-comment-image.png';
  fs.mkdirSync('C:/tmp/nvs_qa', { recursive: true });
  fs.writeFileSync(tmp, makeTestPng());
  await uploadInput.setInputFiles(tmp);
  await page.waitForTimeout(2500);

  const thumbCount = await page.locator('.cc-img-item img').count();
  ok('上传后出现缩略图预览', thumbCount > 0, `缩略图 ${thumbCount} 个`);

  const thumbSrc = thumbCount > 0 ? (await page.locator('.cc-img-item img').first().getAttribute('src')) || '' : '';
  ok('缩略图指向站内 /uploads/images/', thumbSrc.startsWith('/uploads/images/'), thumbSrc);

  // 提交评论（带图）
  const ta = page.locator('.comment-section textarea').first();
  if (await ta.count()) {
    await ta.fill('这是一条带附图的评论。');
  }
  const submit = page.locator('.comment-form button', { hasText: '发表评论' }).first();
  await submit.click();
  await page.waitForTimeout(3000);

  const imgCount = await page.locator('.comment-images .comment-image').count();
  ok('提交后评论列表渲染出附图', imgCount > 0, `渲染 ${imgCount} 张`);

  // 移除按钮存在性（再上传一张 → 移除 → 缩略图应减少）
  if (thumbCount > 0) {
    await uploadInput.setInputFiles(tmp);
    await page.waitForTimeout(2500);
    const before = await page.locator('.cc-img-item img').count();
    await page.locator('.cc-img-del').first().click();
    await page.waitForTimeout(600);
    const after = await page.locator('.cc-img-item img').count();
    ok('移除按钮能删掉待发附图', after === before - 1, `${before} → ${after}`);
  }
}

// ── 4. 楼加密：未解锁时楼正文占位，但楼内回复可见 ──
if (floorId) {
  await page.reload({ waitUntil: 'networkidle' }).catch(() => {});
  await page.waitForTimeout(3000);

  const lockedShown = await page.locator('.floor-locked, .locked-text').count();
  ok('未解锁时楼正文显示 🔒 占位', lockedShown > 0, `占位元素 ${lockedShown} 个`);

  // 楼内回复内容应可见（这是核心断言）
  const replyVisible = await page.locator('.floor-replies .reply-body', { hasText: '楼内回复' }).count();
  ok('楼内回复（评论区）在楼加密时照常可见', replyVisible > 0, `可见回复 ${replyVisible} 条`);

  // 楼正文内容不应泄露。
  // ⚠️ 这里当前身份是「楼作者本人」（admin），作者本来就该看到自己的全文，
  // 用页面断言会误报「泄露」。真正的「他人视角」要在 API 层测（无密码、无授权）。
  const anonCtx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const anonPage = await anonCtx.newPage();
  await anonPage.goto(`${BASE}/author/${blogAuthor}/blogs/${blogId}`, { waitUntil: 'networkidle', timeout: 45000 }).catch(() => {});
  await anonPage.waitForTimeout(2500);
  const anonBodies = await anonPage.locator('.floor-content').allInnerTexts().catch(() => []);
  const anonLeaked = anonBodies.some((t) => t.includes('这是一条加密楼'));
  ok('未解锁（他人视角）时楼正文内容不泄露', !anonLeaked, anonLeaked ? '泄露了！' : '已脱敏');

  const anonReplies = await anonPage.locator('.floor-replies .reply-body', { hasText: '楼内回复' }).count();
  ok('未解锁（他人视角）时楼内回复仍可见', anonReplies > 0, `可见回复 ${anonReplies} 条`);
  await anonCtx.close();
}

// ── 5. 绘画：优化提示词按钮存在 ──
await page.goto(`${BASE}/ai`, { waitUntil: 'networkidle', timeout: 45000 }).catch(() => {});
await page.waitForTimeout(2500);
const aiImgTab = page.locator('text=绘画').first();
if (await aiImgTab.count()) {
  await aiImgTab.click().catch(() => {});
  await page.waitForTimeout(1500);
}
const optimizeBtn = page.locator('.pd-link', { hasText: '优化' }).first();
ok('绘画面板出现「优化」按钮', (await optimizeBtn.count()) > 0);

await page.screenshot({ path: path.join(OUT, 'final.png'), fullPage: false });

await browser.close();

const failed = results.filter((r) => !r.pass);
console.log(`\n总计 ${results.length} 项，通过 ${results.length - failed.length}，失败 ${failed.length}`);
if (failed.length) {
  failed.forEach((f) => console.log(`  ❌ ${f.name} — ${f.detail}`));
  process.exit(1);
}
