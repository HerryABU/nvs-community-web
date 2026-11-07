// 舆论图谱 scope（全局/评论区/议题）+ 强中立提示词默认常驻 验收
// 针对隔离实例 8001（captcha 已关，admin 密码已重置）。复用 mobile 脚本的登录写法。
import { createRequire } from 'module';
const require = createRequire(import.meta.url);
const { chromium } = require('C:/Users/QBZ95/.workbuddy/binaries/node/workspace/node_modules/playwright-core');

const BASE = 'http://127.0.0.1:8001';
const EMAIL = 'admin@nvs.local';
const PW = 'NvsTest@2026';
const EXE = process.env.LOCALAPPDATA + '/ms-playwright/chromium-1148/chrome-win/chrome.exe';

let pass = 0, fail = 0;
function assert(cond, msg) {
  if (cond) { pass++; console.log('  ✅', msg); }
  else { fail++; console.log('  ❌', msg); }
}

const browser = await chromium.launch({ executablePath: EXE, headless: true });

async function login(p) {
  await p.goto(BASE + '/login', { waitUntil: 'domcontentloaded' });
  await p.waitForSelector('input[placeholder*="邮箱"]', { timeout: 10000 });
  await p.fill('input[placeholder*="邮箱"]', EMAIL);
  await p.fill('input[placeholder*="密码"]', PW);
  await p.press('input[placeholder*="密码"]', 'Enter');
  try { await p.waitForURL(u => !u.pathname.includes('/login'), { timeout: 20000 }); }
  catch { console.log('  (登录超时，可能 429)'); }
}

try {
  // ── 桌面视口功能验证 ──
  {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const p = await ctx.newPage();
    await login(p);
    console.log('  logged in url=', p.url());
    await p.goto(BASE + '/admin/sentiment', { waitUntil: 'domcontentloaded' });
    await p.waitForSelector('.el-radio-button:has-text("阵营图谱")', { timeout: 10000 });
    await p.locator('.el-radio-button:has-text("阵营图谱")').click();
    await p.waitForSelector('.camp-wrap', { timeout: 10000 });
    await p.waitForSelector('text=阵营标记列表', { timeout: 8000 });

    // 1) 强中立提示词默认可见
    const promptVisible = await p.locator('.camp-prompt pre').first().isVisible().catch(() => false);
    const promptText = (await p.locator('.camp-prompt pre').first().innerText().catch(() => '')).slice(0, 400);
    assert(promptVisible, '强中立提示词默认常驻显示（未点「查看」即展开）');
    assert(/严禁判断谁对谁错|结构性|只标记/.test(promptText), `提示词内容含强中立约束（${promptText.slice(0, 24)}…）`);

    // 2) scope 三维度存在
    for (const t of ['全局总览', '按评论区', '按议题']) {
      const c = await p.locator(`.el-radio-button:has-text("${t}")`).count();
      assert(c === 1, `scope 维度「${t}」存在`);
    }

    // 3) 默认范围标题 = 全局总览
    const titleAll = (await p.locator('.camp-scope-title').innerText().catch(() => '')).trim();
    assert(/全局总览/.test(titleAll), `默认范围标题含「全局总览」（${titleAll}）`);

    // 4) 切「按评论区」：出现 content_type 下拉 + content_id 输入，填 ID 后标题更新
    await p.locator('.el-radio-button:has-text("按评论区")').click();
    await p.waitForTimeout(300);
    const hasSelect = await p.locator('.camp-scope .el-select').count();
    const hasNum = await p.locator('.camp-scope .el-input-number').count();
    assert(hasSelect >= 1 && hasNum >= 1, '按评论区：出现内容类型下拉 + 内容ID输入');
    // 用「+」按钮把内容 ID 设为 1（el-input-number 直接 fill 原生 input 不触发 @change）
    await p.locator('.camp-scope .el-input-number__increase').click();
    await p.waitForFunction(
      () => { const el = document.querySelector('.camp-scope-title'); return el && /评论区/.test(el.textContent || ''); },
      { timeout: 6000 }
    ).catch(() => {});
    const titleContent = (await p.locator('.camp-scope-title').innerText().catch(() => '')).trim();
    assert(/评论区/.test(titleContent), `切到评论区并填 ID 后范围标题更新（${titleContent}）`);

    // 5) 切回全局 → 切「按议题」：出现议题下拉
    await p.locator('.el-radio-button:has-text("全局总览")').click();
    await p.waitForTimeout(200);
    await p.locator('.el-radio-button:has-text("按议题")').click();
    await p.waitForTimeout(300);
    const topicSelect = await p.locator('.camp-scope .el-select').count();
    assert(topicSelect >= 1, '按议题：出现议题标签下拉');

    // 6) 桌面端无横向溢出
    const ofD = await p.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    assert(ofD <= 2, `桌面端无横向溢出（溢出 ${ofD}px）`);
    await ctx.close();
  }

  // ── 移动端 375px 无横向溢出 + 提示词可见 ──
  {
    const ctx = await browser.newContext({ viewport: { width: 375, height: 812 } });
    const p = await ctx.newPage();
    await login(p);
    await p.goto(BASE + '/admin/sentiment', { waitUntil: 'domcontentloaded' });
    await p.waitForSelector('.el-radio-button:has-text("阵营图谱")', { timeout: 10000 });
    await p.locator('.el-radio-button:has-text("阵营图谱")').click();
    await p.waitForSelector('.camp-wrap', { timeout: 10000 });
    await p.waitForTimeout(400);
    const ofM = await p.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    assert(ofM <= 2, `移动端 375px 无横向溢出（溢出 ${ofM}px）`);
    const promptM = await p.locator('.camp-prompt pre').first().isVisible().catch(() => false);
    assert(promptM, '移动端：强中立提示词默认可见');
    await ctx.close();
  }
} catch (e) {
  console.error('运行异常：', e.message);
  fail++;
} finally {
  await browser.close();
}

console.log(`\n=== 结果：${pass} 通过 / ${fail} 失败 ===`);
process.exit(fail === 0 ? 0 : 1);
