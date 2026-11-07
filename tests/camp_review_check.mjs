// 舆论阵营图谱「审核后自动生成」验收：
//   1) 图谱声明仅含已审核标记（提示/说明含「已审核通过」）
//   2) 待审核(pending)标记出现在列表但【不】进入图谱
//   3) 超级管理员审核通过(pending→approved)后，标记【自动】进入图谱
//   4) 超级管理员手动新增标记直接 approved，立即进入图谱
//   5) 列表状态列 + 超管「通过/驳回」按钮存在
// 针对隔离实例 8001（captcha 关，admin@nvs.local / NvsTest@2026）。
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

// 读取当前图谱里是否含某 camp_id 节点
async function graphHas(p, campId) {
  return await p.evaluate((cid) => {
    // 图谱数据通过组件状态渲染 SVG 节点；节点 label 文本含阵营显示名/标识
    const txt = document.querySelector('.camp-svg-wrap')?.innerText || '';
    return txt.includes(cid);
  }, campId);
}

try {
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const p = await ctx.newPage();
  await login(p);
  console.log('  logged in url=', p.url());
  await p.goto(BASE + '/admin/sentiment', { waitUntil: 'domcontentloaded' });
  await p.waitForSelector('.el-radio-button:has-text("阵营图谱")', { timeout: 10000 });
  await p.locator('.el-radio-button:has-text("阵营图谱")').click();
  await p.waitForSelector('.camp-wrap', { timeout: 10000 });
  await p.waitForTimeout(800);
  // 调试：等表格行出现并输出标记面板文本
  await p.waitForSelector('.camp-panel:has-text("阵营标记列表") .el-table__row', { timeout: 8000 }).catch(() => {});
  const dbg = await p.locator('.camp-panel:has-text("阵营标记列表")').innerText().catch(() => '');
  console.log('  [DEBUG 标记面板]\n' + dbg.split('\n').map(s => '    ' + s).join('\n'));

  // 2) 注入的待审标记 PENDGATE 出现在列表（待审核），但不进入图谱
  const listText0 = await p.locator('.camp-panel:has-text("阵营标记列表")').innerText().catch(() => '');
  assert(/PENDGATE/.test(listText0), '待审标记 PENDGATE 出现在阵营标记列表中');
  assert(/待审核/.test(listText0), '列表中 PENDGATE 状态为「待审核」');
  const inGraphPending = await graphHas(p, 'PENDOK');
  assert(!inGraphPending, '待审标记 PENDGATE【不】进入图谱（审核前）');

  // 5) 状态列 + 超管「通过/驳回」按钮存在
  const hasPass = await p.locator('.camp-panel:has-text("阵营标记列表") .el-button:has-text("通过")').count();
  const hasReject = await p.locator('.camp-panel:has-text("阵营标记列表") .el-button:has-text("驳回")').count();
  assert(hasPass >= 1 && hasReject >= 1, `超管可对待审标记看到「通过/驳回」按钮（通过 ${hasPass} / 驳回 ${hasReject}）`);

  // 3) 审核通过 PENDGATE → 自动进入图谱
  const pendRow = p.locator('.el-table__row', { hasText: 'PENDGATE' }).first();
  await pendRow.locator('.el-button:has-text("通过")').click();
  await p.waitForTimeout(1800);
  await p.locator('.el-button:has-text("刷新图谱")').click();
  await p.waitForTimeout(1200);
  const inGraphApproved = await graphHas(p, 'PENDOK');
  assert(inGraphApproved, '审核通过后 PENDGATE【自动】进入图谱');

  // 1) 图谱说明声明仅含已审核标记（此时图谱已有节点，声明会渲染）
  const notice = await p.locator('.camp-disclaimer').innerText().catch(() => '');
  assert(/已审核通过/.test(notice), `图谱说明声明仅含已审核标记（${notice.slice(0, 28)}…）`);

  // 4) 超管手动新增标记 → 直接 approved → 立即进入图谱
  await p.locator('.camp-panel:has-text("手动标记") input[placeholder*="支持方"]').fill('SUPERADD');
  await p.locator('.camp-panel:has-text("手动标记") input[placeholder*="中性短名"]').fill('SUPADD');
  await p.locator('.camp-panel:has-text("手动标记") .el-button:has-text("新增标记")').click();
  await p.waitForTimeout(1500);
  await p.locator('.el-button:has-text("刷新图谱")').click();
  await p.waitForTimeout(1200);
  const inGraphSuper = await graphHas(p, 'SUPADD');
  assert(inGraphSuper, '超管手动新增标记 SUPERADD 直接审核通过并进入图谱');

  await ctx.close();
} catch (e) {
  console.error('运行异常：', e.message);
  fail++;
} finally {
  await browser.close();
}

console.log(`\n=== 结果：${pass} 通过 / ${fail} 失败 ===`);
process.exit(fail === 0 ? 0 : 1);
