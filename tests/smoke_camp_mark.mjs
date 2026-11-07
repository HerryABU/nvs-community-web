/*
 * 阵营标记「修改/删除权限分层」真实浏览器验收脚本（playwright-core，headless）
 * 覆盖闭环：
 *   超管新增 → 普通管理员申请修改(待批、未生效) → 超管批准生效
 *          → 普通管理员申请删除(待批、未删) → 超管批准真删
 *          → 普通管理员申请删除 → 超管驳回(标记仍在、状态已驳回)
 * 运行前置：
 *   1) 用 nvs-server.exe 起一个隔离实例（建议 8001 端口），复制 .env / .jwt_secret /
 *      .secret.key / token-vault.key / secure-config.json / nvs.db 到隔离目录；
 *   2) 隔离库 platform_configs.captcha_enabled 置 false（避免滑块验证码拦截登录）；
 *   3) 准备两个账户：admin@nvs.local（role=admin，站点最小 id 即超管）与 reg@nvs.local（role=admin 普通管理员）；
 *   4) BASE / EXEC 常量按本机调整；node tests/smoke_camp_mark.mjs 即可。
 * 注意：登录接口有 RateLimit(5,60)，脚本内置 429 退避重试。
 */
import { createRequire } from 'module';
const require = createRequire(import.meta.url);
const { chromium } = require('C:/Users/QBZ95/.workbuddy/binaries/node/workspace/node_modules/playwright-core');

const BASE = 'http://127.0.0.1:8001';
const EXEC = process.env.LOCALAPPDATA + '/ms-playwright/chromium-1148/chrome-win/chrome.exe';
const SHOT = 'C:/Users/QBZ95/Desktop/tools/tests/test_go_smoke';
const ADMIN = { email: 'admin@nvs.local', pwd: 'NvsTest@2026' };
const REG = { email: 'reg@nvs.local', pwd: 'NvsTest@2026' };

const log = (...a) => console.log('[smoke]', ...a);
let failures = 0;
function assert(cond, msg) { if (cond) log('PASS:', msg); else { failures++; log('FAIL:', msg); } }
const ts = Date.now().toString().slice(-6);
const MARK_ID = `SMK${ts}`;
const MARK_NAME = `烟雾${ts}方`;
const MARK_NAME2 = `烟雾${ts}改`;

const browser = await chromium.launch({ executablePath: EXEC, headless: true, args: ['--no-sandbox'] });
let curCtx = null;

// 每次登录用全新上下文（规避同上下文多次登录后登录表单提交不导航的浏览器侧状态累积问题）
async function uiLogin(acct) {
  if (curCtx) { await curCtx.close().catch(() => {}); curCtx = null; } // 关闭上一个上下文，避免累积耗尽浏览器资源
  const context = await browser.newContext();
  curCtx = context;
  const page = await context.newPage();
  page.on('pageerror', e => log('PAGEERROR:', e.message));
  // 捕获登录接口响应，供 429 退避重试判断
  let loginStatus = 0;
  page.on('response', r => {
    if (r.url().includes('/api/auth/login')) loginStatus = r.status();
    else if (r.status() >= 400 && r.url().includes('/api/')) r.text().then(t => log('HTTP', r.status(), r.url().split('?')[0], '->', t.slice(0, 120))).catch(() => {});
  });
  const doLogin = async (retry) => {
    loginStatus = 0;
    await page.goto(BASE + '/login', { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('input[placeholder*="邮箱"]', { timeout: 10000 });
    await page.fill('input[placeholder*="邮箱"]', acct.email);
    await page.fill('input[placeholder*="密码"]', acct.pwd);
    await page.press('input[placeholder*="密码"]', 'Enter');
    try {
      await page.waitForURL(u => !u.pathname.includes('/login'), { timeout: 20000 });
      return true;
    } catch {
      if (loginStatus === 429 && !retry) {
        log('登录被限流(429)，等待 65s 过窗口后重试…');
        await page.waitForTimeout(65000);
        return false; // 触发外层重试
      }
      const emailVisible = await page.locator('input[placeholder*="邮箱"]').count().catch(() => -1);
      log('LOGIN-TIMEOUT url=', page.url(), 'loginStatus=', loginStatus, 'emailInput=', emailVisible);
      throw new Error('login timeout');
    }
  };
  let ok = await doLogin(false);
  if (!ok) ok = await doLogin(true); // 429 退避后重试一次
  if (!ok) throw new Error('login failed after retry');
  log('logged in as', acct.email);
  return page;
}

// 客户端路由导航（不整页刷新，保留内存 token + cookie），规避重载 401
async function gotoSentiment(page) {
  await page.evaluate(() => window.__nvsRouter__.push('/admin/sentiment'));
  await page.waitForSelector('.el-radio-button:has-text("阵营图谱")', { timeout: 20000 });
  await page.locator('.el-radio-button:has-text("阵营图谱")').click();
  await page.waitForSelector('text=阵营标记列表', { timeout: 20000 });
  await page.waitForTimeout(1000);
}

async function mbConfirm(page) {
  const box = page.locator('.el-message-box').last();
  await box.waitFor({ state: 'visible', timeout: 8000 });
  await box.locator('button.el-button--primary').click({ timeout: 8000 });
}
function markPanel(page) { return page.locator('.camp-panel', { hasText: '阵营标记列表' }); }
function markRow(page, name) { return markPanel(page).locator('tr', { hasText: name }); }

(async () => {
  try {
    // —— 阶段A：超级管理员：建标记，看直改/直删按钮 ——
    let page = await uiLogin(ADMIN);
    await gotoSentiment(page);
    await page.screenshot({ path: SHOT + '/A_admin.png', fullPage: true });
    assert(await page.locator('h4:has-text("阵营标记列表")').count() > 0, 'A1 超管可见「阵营标记列表」面板');
    assert(await page.locator('h4:has-text("修改 / 删除申请")').count() > 0, 'A2 超管可见「修改 / 删除申请（修改项目）」面板');
    assert(await markPanel(page).getByRole('button', { name: '修改', exact: true }).count() > 0, 'A3 超管可见「修改/删除」直改按钮');
    assert(await markPanel(page).getByRole('button', { name: '删除', exact: true }).count() > 0, 'A3b 超管可见直删按钮');
    await page.fill('input[placeholder*="如 A"]', MARK_ID);
    await page.fill('input[placeholder*="中性短名"]', MARK_NAME);
    await page.click('button:has-text("新增标记")');
    await page.waitForTimeout(1500);
    await gotoSentiment(page);
    assert(await markRow(page, MARK_NAME).count() > 0, 'A4 超管新增标记成功（列表出现「' + MARK_NAME + '」）');

    // —— 阶段B：普通管理员：只能申请修改/删除 ——
    page = await uiLogin(REG);
    await gotoSentiment(page);
    await page.screenshot({ path: SHOT + '/B_reg.png', fullPage: true });
    assert(await markPanel(page).getByRole('button', { name: '申请修改', exact: true }).count() > 0, 'B1 普通管理员可见「申请修改/申请删除」按钮');
    assert(await markPanel(page).getByRole('button', { name: '修改', exact: true }).count() === 0, 'B2 普通管理员看不到超管直改「修改」按钮（权限分层生效）');
    await page.locator(`tr:has-text("${MARK_NAME}") button:has-text("申请修改")`).first().click();
    await page.waitForSelector('text=申请修改阵营标记', { timeout: 8000 });
    await page.locator('.el-dialog input[placeholder="中性短名"]').fill(MARK_NAME2);
    await page.click('.el-dialog button:has-text("提交")');
    await page.waitForTimeout(1500);
    await page.screenshot({ path: SHOT + '/B_requested.png', fullPage: true });
    assert(await page.locator(`tr:has-text("待批")`).count() > 0, 'B3 申请列表出现「待批」');
    assert(await markRow(page, MARK_NAME).count() > 0 && await markRow(page, MARK_NAME2).count() === 0, 'B4 批准前标记未变（仍为「' + MARK_NAME + '」，无「' + MARK_NAME2 + '」）');

    // —— 阶段C：超级管理员：批准修改申请 → 生效 ——
    page = await uiLogin(ADMIN);
    await gotoSentiment(page);
    await page.waitForTimeout(800);
    const approveBtn = page.locator(`tr:has-text("待批"):has-text("${MARK_NAME}") button:has-text("批准")`).first();
    assert(await approveBtn.count() > 0, 'C1 超管可见「批准/驳回」按钮');
    await approveBtn.click();
    await mbConfirm(page);
    await page.waitForTimeout(2000);
    await gotoSentiment(page);
    await page.screenshot({ path: SHOT + '/C_approved.png', fullPage: true });
    assert(await markRow(page, MARK_NAME2).count() > 0, 'C2 超管批准后标记更新为「' + MARK_NAME2 + '」');
    assert(await page.locator(`tr:has-text("待批"):has-text("${MARK_NAME}")`).count() === 0, 'C3 批准后本标记无残留「待批」');

    // —— 阶段D：普通管理员申请删除 → 超管批准真删 ——
    page = await uiLogin(REG);
    await gotoSentiment(page);
    await page.locator(`tr:has-text("${MARK_NAME2}") button:has-text("申请删除")`).first().click();
    await mbConfirm(page);
    await page.waitForTimeout(600);
    await mbConfirm(page);
    await page.waitForTimeout(1500);
    await page.screenshot({ path: SHOT + '/D_request_del.png', fullPage: true });
    assert(await page.locator(`tr:has-text("待批"):has-text("${MARK_NAME2}")`).count() > 0, 'D1 删除申请进入「待批」');
    assert(await markRow(page, MARK_NAME2).count() > 0, 'D2 批准前标记仍在（未删）');
    page = await uiLogin(ADMIN);
    await gotoSentiment(page);
    await page.waitForTimeout(800);
    await page.locator(`tr:has-text("待批"):has-text("${MARK_NAME2}") button:has-text("批准")`).first().click();
    await mbConfirm(page);
    await page.waitForTimeout(2000);
    await gotoSentiment(page);
    await page.screenshot({ path: SHOT + '/E_deleted.png', fullPage: true });
    assert(await markRow(page, MARK_NAME2).count() === 0, 'D3 超管批准后标记被真正删除（列表无「' + MARK_NAME2 + '」）');

    // —— 阶段E：驳回路径验证（用户强调站长可驳回）——
    page = await uiLogin(ADMIN);
    await gotoSentiment(page);
    const EXTRA = `驳回测${ts}`;
    await page.fill('input[placeholder*="如 A"]', `EX${ts}`);
    await page.fill('input[placeholder*="中性短名"]', EXTRA);
    await page.click('button:has-text("新增标记")');
    await page.waitForTimeout(1500);
    await gotoSentiment(page);
    assert(await markRow(page, EXTRA).count() > 0, 'E1 超管新增驳回测试标记「' + EXTRA + '」');
    page = await uiLogin(REG);
    await gotoSentiment(page);
    await page.locator(`tr:has-text("${EXTRA}") button:has-text("申请删除")`).first().click();
    await mbConfirm(page);
    await page.waitForTimeout(600);
    await mbConfirm(page);
    await page.waitForTimeout(1500);
    await gotoSentiment(page);
    assert(await page.locator(`tr:has-text("待批"):has-text("${EXTRA}")`).count() > 0, 'E2 普通管理员删除申请进入「待批」');
    page = await uiLogin(ADMIN);
    await gotoSentiment(page);
    await page.locator(`tr:has-text("待批"):has-text("${EXTRA}") button:has-text("驳回")`).first().click();
    await mbConfirm(page);
    await page.waitForTimeout(1500);
    await gotoSentiment(page);
    assert(await page.locator(`tr:has-text("已驳回"):has-text("${EXTRA}")`).count() > 0, 'E3 超管驳回后申请状态为「已驳回」');
    assert(await markRow(page, EXTRA).count() > 0, 'E4 驳回不删除：标记仍在列表');
    assert(await page.locator(`tr:has-text("已驳回"):has-text("${EXTRA}") button:has-text("批准")`).count() === 0, 'E5 已驳回申请不再有「批准」操作（已处理）');
    await markRow(page, EXTRA).locator('button:has-text("删除")').first().click();
    await mbConfirm(page);
    await page.waitForTimeout(1500);
    await gotoSentiment(page);
    assert(await markRow(page, EXTRA).count() === 0, 'E6 超管直删驳回测试标记成功（清理）');

    log('==== 结论：', failures === 0 ? '全部通过 ✅' : (failures + ' 项失败 ❌'));
  } catch (e) {
    log('EXCEPTION:', e.message);
    log('==== 结论：', (failures + 1) + ' 项失败 ❌');
    failures++;
  } finally {
    await browser.close();
    process.exit(failures === 0 ? 0 : 1);
  }
})();
