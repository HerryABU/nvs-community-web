# 后台管理页体检报告

> 生成方式：`node scripts/audit-admin-pages.mjs --md`（可随时重跑，数字永远是最新的）
> 审计范围：`web/src/views/admin/*.vue` 共 48 个页面、455 个 `el-table`

## 一、结论先说

| 指标 | 体检前 | 现在 |
|---|---|---|
| StateBlock（统一三态组件）覆盖 | 0 / 48（0%） | 35 / 48（73%） |
| 表格有骨架/加载态 | 0 / 48（0%） | 18 / 48（38%） |
| 空表格有可读空态 | 6 / 48 有 `el-empty`，其余只有默认"暂无数据" | 27 / 48 有明确空态（含下一步提示） |
| 接口失败有可见提示 | 0 / 48（只有一闪而过的 toast） | 38 / 48（AdminHeader 全局横幅）+ 1 页面内 |
| 表格行高/表头/字号一致 | 415 / 455 张表未设密度，各写各的 | 全部统一（一组全局规则） |

**"逐页看都觉得粗糙"的真实原因不是配色，是这三件事：**
1. 455 张表里 415 张走默认大行高（12~14px 上下留白），表头没有底色与分隔线；
2. 空表格只有 Element Plus 默认的"暂无数据"四个字，分不清"没数据"和"还没加载回来"；
3. 接口失败时页面 catch 里基本是 `/* ignore */`，用户看到的就是"表格空着"。

## 二、这一轮改了什么（都收口到一处，不是逐页手改）

### 1. 表格密度 / 表头 / 空态：`web/src/styles/admin.css`
作用域限定 `.admin-page`，一处生效于全部 455 张表：
- 单元格 `padding: 8px 0`、表头 `9px 0` + 底色 + 底部分隔线 + 12px 灰字（扫读不串行）
- 表格字号统一 13px；行内操作按钮间距 12px → 4px
- 空态块压到 120px、文字 13px（原来 60px 插画 + 大留白）
- 深色：表头 `#0f172a`、分隔线 `#334155`、文字 `#e2e8f0`

> ⚠️ 踩坑：这些属性必须 `!important`。Element Plus 自带的 `.el-table .el-table__cell`
> 特异性与本规则接近且加载顺序更靠后，不加的话实测仍是 14px（已验证）。

### 2. 空态 / 骨架：18 个页面注入 `#empty` 插槽（148 处）
统一内容为 `StateBlock`（`loading` 时渲染骨架，否则渲染紧凑空态 + "换个筛选条件试试"）。
只给原本**既没有 StateBlock 也没有 #empty** 的页面注入，已有手写三态的页面不动。

### 3. 接口异常：`web/src/utils/adminHealth.ts` + `AdminHeader`
- axios 响应拦截器：后台请求出现 **5xx / 网络错误** 时登记
  （4xx 属业务语义，登记只会变噪音；`/admin/health`、`/admin/ping`、`/setup/*` 排除）
- `AdminHeader` 顶部常驻横幅：状态码 + 请求路径 + 消息 + 「重新加载」
- 任意后台请求成功即自动清除；也可手动「忽略」
- 覆盖 38 个使用 AdminHeader 的页面（剩 10 个设置型页面见下）

### 4. 审计脚本：`scripts/audit-admin-pages.mjs`
```
node scripts/audit-admin-pages.mjs          # 表格视图
node scripts/audit-admin-pages.mjs --md     # markdown（本文表格即由它生成）
node scripts/audit-admin-pages.mjs --strict # 有新页面未接四态时退出码 1（可接 CI）
```

## 三、逐页明细

| 页面 | 表格 | StateBlock | #empty 插槽 | 骨架 | v-loading | 分页 | 筛选项 | 错误态 |
|---|---:|---:|---:|---:|---:|---:|---:|:--:|
| AdminAiAgents | 9 | ✓ | 1 | — | 1 | 0 | 13 | 全局横幅 |
| AdminAiBot | 0 | — | 0 | — | 1 | 0 | 6 | — |
| AdminAiCost | 9 | ✓ | 3 | ✓ | 0 | 0 | 12 | — |
| AdminAiFeatures | 11 | ✓ | 2 | — | 2 | 0 | 15 | 全局横幅 |
| AdminAiFreeQuota | 7 | ✓ | 5 | ✓ | 2 | 0 | 14 | — |
| AdminAiLogs | 29 | ✓ | 3 | — | 3 | 3 | 7 | 全局横幅 |
| AdminAiPool | 19 | ✓ | 2 | — | 2 | 0 | 29 | 全局横幅 |
| AdminAlgorithms | 0 | — | 0 | — | 0 | 0 | 5 | 全局横幅 |
| AdminAnnouncements | 6 | ✓ | 1 | — | 1 | 0 | 4 | 全局横幅 |
| AdminBadWords | 21 | ✓ | 2 | — | 2 | 1 | 17 | 全局横幅 |
| AdminBan | 16 | ✓ | 2 | — | 2 | 1 | 2 | 全局横幅 |
| AdminBlogCategories | 0 | — | 0 | — | 2 | 0 | 11 | — |
| AdminCategories | 0 | — | 0 | — | 0 | 0 | 1 | 全局横幅 |
| AdminCommunity | 0 | — | 0 | — | 4 | 0 | 0 | 全局横幅 |
| AdminCompliance | 4 | ✓ | 2 | ✓ | 0 | 0 | 4 | 全局横幅 |
| AdminDatabase | 6 | ✓ | 1 | — | 1 | 0 | 8 | 全局横幅 |
| AdminEmergency | 8 | ✓ | 1 | — | 0 | 0 | 8 | 全局横幅 |
| AdminForums | 15 | ✓ | 8 | ✓ | 2 | 0 | 10 | 全局横幅 |
| AdminGate | 18 | ✓ | 12 | ✓ | 0 | 0 | 14 | — |
| AdminHealth | 0 | — | 0 | — | 0 | 0 | 0 | 全局横幅 |
| AdminHomeBlocks | 7 | ✓ | 5 | ✓ | 2 | 0 | 5 | 全局横幅 |
| AdminHomeCustomizeReview | 6 | ✓ | 1 | — | 2 | 0 | 3 | 全局横幅 |
| AdminHoneypot | 19 | ✓ | 3 | — | 1 | 0 | 10 | — |
| AdminHotWords | 0 | — | 0 | — | 4 | 0 | 5 | 全局横幅 |
| AdminHtmlReview | 19 | ✓ | 2 | — | 3 | 0 | 9 | 全局横幅 |
| AdminLogRedact | 0 | — | 0 | — | 0 | 0 | 6 | 全局横幅 |
| AdminLoginMethods | 0 | — | 0 | — | 0 | 0 | 19 | 全局横幅 |
| AdminNetDiag | 26 | ✓ | 8 | ✓ | 3 | 1 | 11 | 全局横幅 |
| AdminOps | 22 | ✓ | 10 | ✓ | 0 | 0 | 23 | — |
| AdminPayment | 18 | ✓ | 16 | ✓ | 2 | 1 | 20 | — |
| AdminPoliceConsole | 9 | ✓ | 6 | ✓ | 1 | 0 | 2 | 全局横幅 |
| AdminPublishQuota | 11 | ✓ | 5 | ✓ | 2 | 0 | 6 | 全局横幅 |
| AdminRage | 0 | — | 0 | — | 1 | 0 | 3 | 全局横幅 |
| AdminRateLimit | 26 | ✓ | 22 | ✓ | 3 | 1 | 14 | 全局横幅 |
| AdminRealName | 8 | ✓ | 1 | — | 2 | 1 | 3 | 全局横幅 |
| AdminReports | 8 | ✓ | 1 | — | 1 | 0 | 7 | 全局横幅 |
| AdminRisk | 8 | ✓ | 5 | ✓ | 0 | 0 | 12 | — |
| AdminSecurity | 9 | ✓ | 1 | — | 1 | 0 | 11 | 全局横幅 |
| AdminSettings | 0 | — | 0 | — | 0 | 0 | 12 | 全局横幅 |
| AdminSites | 26 | ✓ | 20 | ✓ | 2 | 0 | 5 | 全局横幅 |
| AdminSoftDel | 9 | ✓ | 1 | — | 1 | 0 | 2 | 全局横幅 |
| AdminTheme | 0 | — | 0 | — | 2 | 0 | 3 | 全局横幅 |
| AdminTickets | 7 | ✓ | 5 | ✓ | 0 | 0 | 20 | — |
| AdminToken | 9 | ✓ | 1 | — | 1 | 0 | 11 | 全局横幅 |
| AdminVulnerabilities | 8 | ✓ | 5 | ✓ | 0 | 0 | 10 | 全局横幅 |
| AdminWall | 0 | — | 0 | — | 0 | 0 | 6 | 全局横幅 |
| AdminWallAccess | 7 | ✓ | 5 | ✓ | 2 | 1 | 5 | 全局横幅 |
| AdminWebQuota | 15 | ✓ | 8 | ✓ | 2 | 0 | 9 | 全局横幅 |


## 四、还没做的（明确列出，不做隐性承诺）

| 项 | 现状 | 影响 | 建议 |
|---|---|---|---|
| 10 个页面没有 AdminHeader（AdminAiBot / AdminAiCost / AdminAiFreeQuota / AdminBlogCategories / AdminGate / AdminHoneypot / AdminOps / AdminPayment / AdminRisk / AdminTickets） | 拿不到全局异常横幅 | 接口失败时仍只有 toast | 属"设置面板"型页面（自带页内标题）；下一步给它们各加页内错误态 |
| 13 个页面未接 StateBlock | 多为表单/看板型，无表格或仅 1 张 | 空态仍是默认文案 | 低优先，按实际使用频率补 |
| 分页只有 8 个页面有 | 其余多为一次性全量返回 | 数据量大时列表很长 | 需后端配合改分页接口，属功能改造 |
| 筛选项密度差很大（1~29 控件） | 见上表"筛选项"列 | 高密度页（AdminAiPool 29、AdminOps 23、AdminPayment 20）工具条过长 | 抽「常用筛选 + 更多筛选」折叠 |
| 骨架只覆盖 18 页 | 其余靠 `v-loading` 遮罩 | 首屏仍会跳一下 | 复用本轮 `#empty` + `state=loading` 写法逐步替换 |

## 五、怎么验收的

真浏览器（playwright-core + 本地 chromium，登录态，1440×950）10 项断言全 PASS：
表头 padding 9px / 单元格 8px / 表头有底色 / 字号 13px / 空表格有可读空态 /
接口 500 时页头出横幅（含路径与消息）/ 有重试按钮 / 深色表头 `rgb(15,17,23)` /
深色单元格文字 `rgb(226,232,240)` / 页面 JS 错误 0。
