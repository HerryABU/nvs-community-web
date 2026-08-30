# NVS 路线图 · 3.5.x 合规与运维增强

> 目标版本：**3.5.1**（当前代码基线 `server/version/version.go` = `3.3.2`）。
> 工作流：每个功能实现完成 → `git add` → `git commit`，版本号顺延至 3.5.1 区间。
> 合规基线：176 号令（互联网信息服务算法推荐管理规定 / 用户信息追溯）。

---

## 现有地基（已实现，路线图在其上扩展）

| 模块 | 位置 | 说明 |
|------|------|------|
| 隔离墙配置 | `server/models/platform_wall.go` | 敏感分区（同人区/政治文学区）多级确认墙，配置存 `platform_config.wall_config` |
| 隔离墙进入记录 | `server/handlers/...RecordZoneEnter` + `ListZoneAccessLogs` | 路由 `POST /api/zone/enter`、`GET /api/admin/zone-access`（176 号令专项） |
| 访问日志中间件 | `server/middleware/access_log.go` | 记录 `user_id / method / path / query / 请求头 / 请求体`（脱敏+64KB 截断） |
| API 访问日志 | `server/models/api_key.go` `ApiAccessLog` | 含 `IdentityType`、UA、保留天数 `LogRetentionDays` |
| 请求头脱敏 | `server/middleware/redact.go` `RedactHeaders` | 需求 3.3.1 |
| 软删除 | `server/models/softdel.go` + `server/handlers/admin_softdel.go` + `web/src/views/admin/AdminSoftDel.vue` | 封禁/删除用户采用软删除（可追溯） |
| 用户角色 | `server/models/user.go` `Role string` | 现仅 `reader` / `author` / `vip_author` / `admin`；注册恒为 `reader` |

---

## 功能一：隔离墙「谁进入 / 何时 / 请求头」访问仪表盘

**定位**：把原本只在数据库/接口里看到的隔离墙进入记录，在前台做成可视化仪表盘。
**后端**：`ListZoneAccessLogs` 已存在，需补充分页/筛选/导出（按分区、时间段、用户）。
**前端**：新增 `web/src/views/admin/AdminWallAccess.vue`（或并入 `AdminWall.vue` 新增 Tab）：
- 表格：用户(脱敏)、分区、进入时间、请求头摘要、来源 IP。
- 统计卡片：各分区进入人次、按日趋势（ECharts）。
- 详情抽屉：单条记录的完整请求头（走 `admin_redact.go` 的 header 预览，保持脱敏）。
**验收**：管理员可在前台查看隔离墙进入全量记录，无需直接查库。

## 功能二：请求头 / IP 分类诊断面板（名义"用户分布 / 网络诊断"，含防爬与攻击流量）

**定位**：独立面板，基于 `AccessLog` + `ApiAccessLog` 的 `请求头 / IP / UA / TTL` 等做分类。
**后端**：
- 新增聚合接口：按 `IP` 去重计数、按 `UA` 归类（识别爬虫特征）、按 `TTL`/ASN 估算网络归属。
- 攻击/爬虫标记：高频同 IP、非常规 UA、敏感路径扫描、速率超限 → 标红"疑似攻击流量"。
- 复用 `security/ratelimit.go` 与现有爬虫规则。
**前端**：新增 `web/src/views/admin/AdminNetDiag.vue`：
- 地图/柱状：用户地理与网络分布（IP 段聚合）。
- 攻击流量清单：标红高风险 IP，可一键加黑名单（联动 `AdminBan`）。
- 名义入口文案为"网络诊断 / 用户分布"，实际承载防爬与攻击可见性。
**验收**：可看到请求头、IP 分类，并能识别并处置攻击流量。

## 功能三：警方用户（警方取管台）

**定位**：新增角色 `police`，类管理员但独立命名为「警方取管台」，可见软删除数据并下载凭证/取证包；**不可自助注册**，须由管理员在控制台提权。
**后端**：
- `User.Role` 新增 `"police"`；注册流程（`auth_register.go`）仍恒为 `reader`，禁止自注册为 police。
- `server/handlers/admin_user.go` 新增"提权至警方用户"操作（仅 `admin` 可操作），写入审计日志。
- 权限模型：police 共享 admin 视角的合规/取证能力，但 UI 入口为独立「警方取管台」。
- 警方取管台能力：查看 `AdminSoftDel` 软删除全量、导出取证 ZIP（复用 `admin_toolbox.go` 的 `export-logs`）、凭证下载。
- 合规收益：176 号令追溯 + 便于"提前 2–3 天通知准备"，警方以专属账号导出。
**前端**：
- 新增 `web/src/views/admin/AdminPoliceConsole.vue`（警方取管台）。
- `AdminUser.vue` 用户管理增加"角色"列与"提权为警方"按钮（仅 admin 可见）。
**验收**：普通用户无法自注册为警方；管理员可提权；警方账号可看软删除与下载凭证。

## 功能四：管理员搜索 / 筛选（按分类等）

**定位**：用户管理与内容管理的检索增强。
**后端**：`admin_user.go` / `admin_forum.go` 等列表接口补充多条件筛选（角色、状态、分类、时间）。
**前端**：`AdminUser.vue` 与内容管理面板增加搜索框 + 分类下拉 + 状态筛选。
**验收**：可按角色/分类/状态组合筛选，结果分页。

---

## 版本与提交节奏

- 当前：`3.3.2` → 目标：`3.5.1`。
- 每个功能完成后：`git add` → `git commit`，版本顺延（建议 3.4.0 → 3.4.x → 3.5.1）。
- 每次发布前：通读全代码 → 同步 `README.md` 版本更新记录 → `server/version/version.go` 与 `web/package.json` 三处版本一致。
- 严禁前端硬编码 `/go-nvs/` 前缀；新面板须走运行时前缀注入。

## 风险与注意

- 警方角色属高权限，提权操作必须留审计；切勿暴露自注册入口。
- 请求头/日志含敏感字段，前端展示须沿用 `RedactHeaders` 脱敏规则。
- 攻击流量识别为辅助判断，处置动作（封禁）需二次确认，避免误杀。
