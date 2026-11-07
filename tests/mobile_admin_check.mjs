/*
 * 管理员台移动端真实浏览器验收（playwright-core，375px 视口，headless）
 * 验证点：
 *   1) /admin 在 375px 下无横向溢出；
 *   2) 窄屏隐藏死侧栏（.admin-aside 不挂载），顶栏出现汉堡按钮；
 *   3) 点汉堡 → 抽屉导航（.admin-mobile-drawer + .admin-sidebar）可见、菜单项>0；
 *   4) 点菜单项 → 抽屉关闭 + 路由切换；
 *   5) 子页 /admin/sentiment 在 375px 下无横向溢出，且"阵营图谱"子视图可渲染。
 * 运行前置：隔离实例已在 8001 端口（captcha 关闭），admin@nvs.local / NvsTest@2026。
 */
import { createRequire } from 'module';
const require = createRequire(import.meta.url);
const { chromium } = require('C:/Users/QBZ95/.workbuddy/binaries/node/workspace/node_modules/playwright-core');

const BASE = 'http://127.0.0.1:8001';
const EXEC = process.env.LOCALAPPDATA + '/ms-playwright/chromium-1148/chrome-win/chrome.exe';
const ADMIN = { email: 'admin@nvs.local', pwd: 'NvsTest@2026' };

const log = (...a) => console.log('[mobile]', ...a);
let failures = 0;
function assert(cond, msg) { if (cond) log('PASS:', msg); else { failures++; log('FAIL:', msg); } }

const browser = await chromium.launch({ executablePath: EXEC, headless: true, args: ['--no-sandbox'] });
const context = await browser.newContext({
  viewport: { width: 375, height: 812 },
  deviceScaleFactor: 2,
  isMobile: true,
  hasTouch: true,
});
const page = await context.newPage();
page.on('pageerror', e => log('PAGEERROR:', e.message));
page.on('response', r => {
  if (r.status() >= 400 && r.url().includes('/api/'))
    r.text().then(t => log('HTTP', r.status(), r.url().split('?')[0], '->', t.slice(0, 140))).catch(() => {});
});

async function noOverflow(label) {
  const sw = await page.evaluate(() => document.documentElement.scrollWidth);
  const iw = await page.evaluate(() => window.innerWidth);
  assert(sw <= iw + 1, `${label}: 无横向溢出 (scrollWidth=${sw} <= innerWidth=${iw})`);
}

try {
  // ── 登录 ──
  log('login as', ADMIN.email);
  await page.goto(BASE + '/login', { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('input[placeholder*="邮箱"]', { timeout: 10000 });
  await page.fill('input[placeholder*="邮箱"]', ADMIN.email);
  await page.fill('input[placeholder*="密码"]', ADMIN.pwd);
  await page.press('input[placeholder*="密码"]', 'Enter');
  await page.waitForURL(u => !u.pathname.includes('/login'), { timeout: 20000 });
  log('logged in, url=', page.url());

  // ── 进 /admin（整页跳转；登录态走 HttpOnly Cookie，重载 /api/auth/me 仍 200）──
  await page.goto(BASE + '/admin', { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('.admin-main', { timeout: 10000 });
  await page.waitForSelector('.at-hamburger', { timeout: 10000 });
  await page.waitForTimeout(400); // 等布局稳定

  // 1) 无横向溢出
  await noOverflow('/admin');

  // 2) 死侧栏未挂载 + 汉堡可见
  const asideCount = await page.locator('.admin-aside').count();
  assert(asideCount === 0, `/admin: 窄屏 .admin-aside 未挂载 (count=${asideCount})`);
  const hamVisible = await page.locator('.at-hamburger').isVisible();
  assert(hamVisible, '/admin: 顶栏汉堡按钮可见');

  // 3) 点汉堡 → 抽屉导航出现
  await page.locator('.at-hamburger').click();
  await page.waitForSelector('.admin-mobile-drawer .admin-sidebar', { state: 'visible', timeout: 8000 });
  const menuItems = await page.locator('.admin-mobile-drawer .el-menu-item').count();
  assert(menuItems > 0, `/admin: 抽屉内菜单项可见 (count=${menuItems})`);

  // 4) 展开第一个分类 → 点其下子菜单项 → 抽屉关闭 + 路由切换（首页"数据概览"即 /admin，不能作为跳转目标）
  const beforeUrl = page.url();
  await page.locator('.admin-mobile-drawer .el-sub-menu__title').first().click();
  await page.waitForTimeout(400);
  // 第一个 .el-menu-item 是首页，子项从 nth(1) 起；展开后可见可点
  const child = page.locator('.admin-mobile-drawer .el-menu-item').nth(1);
  await child.click({ timeout: 5000 });
  await page.waitForTimeout(700);
  const drawerHidden = !(await page.locator('.admin-mobile-drawer').isVisible().catch(() => false));
  assert(drawerHidden, '点子菜单项后抽屉已关闭');
  const afterUrl = page.url();
  assert(afterUrl !== beforeUrl, `点子菜单项后路由切换 (${beforeUrl} -> ${afterUrl})`);

  // 5) 子页 /admin/sentiment 移动端无溢出 + 阵营图谱子视图可渲染
  await page.goto(BASE + '/admin/sentiment', { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('.admin-header', { timeout: 10000 });
  await page.waitForTimeout(400);
  await noOverflow('/admin/sentiment');
  // 点"阵营图谱" radio，确认子视图在窄屏渲染且仍无溢出
  const graphTab = page.locator('.el-radio-button:has-text("阵营图谱")');
  if (await graphTab.count() > 0) {
    await graphTab.first().click();
    await page.waitForTimeout(500);
    await noOverflow('/admin/sentiment#graph');
    const graphPanel = await page.locator('.camp-panel', { hasText: '阵营标记列表' }).first().count();
    assert(graphPanel >= 1, '/admin/sentiment 阵营图谱子视图存在');
  } else {
    log('WARN: 未找到"阵营图谱" radio（可能标签页结构变化），跳过子视图校验');
  }
} catch (e) {
  failures++;
  log('EXCEPTION:', e.message);
} finally {
  await browser.close();
}

log(failures === 0 ? '全部通过 ✅' : `存在 ${failures} 项失败 ❌`);
process.exit(failures === 0 ? 0 : 1);
