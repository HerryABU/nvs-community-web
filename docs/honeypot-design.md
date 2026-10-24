# 蜜罐与取证子系统 · 立项设计（P3）

> 目标：把「被动审计」升级为「主动诱捕 + 行为取证」。
> 定位：**溯源辅助**，不替代真实安全事件处置，也不作为独立案源。
> 前置原则：真业务链路全部参数化查询，蜜罐**不碰真实库**；蜜罐所有凭据独立生成、与真实系统零复用。

## 1. 设计要点（按「先甜后卡」节奏）

| 阶段 | 诱饵 | 攻击者看到 | 我们拿到 |
|---|---|---|---|
| ① 入口 | `/.env`（仿真泄露，含 `JWT_SECRET`、DB/Redis 占位凭据、运维备注） | 「捡到运维疏忽」 | 命中 IP/UA/ASN、扫描器指纹 |
| ② 身份 | 用该 `JWT_SECRET` 签出的 token **在蜜罐内验签通过**（影子认证） | 「这把 key 能用！」 | 伪造 payload（sub/role/is_admin 改动）、算法选择（HS256/alg:none/混淆） |
| ③ 甜头 | 低风险读接口正常返回合成数据（首页、自己的资料） | 「系统是活的」 | 请求序列、自动化节奏 |
| ④ 卡口 | 越权/写入/提权操作 → **「检测到异地登录，请重新验证」** + 特殊人机验证页 | 「有风控，得绕」 | 绕过尝试（改 token 重签、跳过验证端点、OCR/打码平台特征、切换 IP/工具） |
| ⑤ 沉底 | 验证页在**加载即采集**浏览器/系统指纹（不等提交） | 以为是风控采集 | Canvas/WebGL/字体/时区/Client Hints/WebRTC 内网 IP/headless 痕迹，与 ①④ 会话 ID 关联 |

关键设计约束（来自实战评估）：
1. **验证永不放行**：验证结果恒失败、状态无状态化、不落可重放凭据；验证接口不存在「返回 success」的分支。
2. **指纹前置采集**：页面 `DOMContentLoaded` 即采，攻击者关页面也已拿到；采集脚本与站点 JS 混编，不单独出现 `fingerprint.js`。
3. **采集范围向真实风控靠拢**：不读剪贴板、不异常精度枚举硬件，避免被指纹检测工具识别为「采集器」。headless 检测只做**标记**不直接拒绝（拒绝会立刻暴露）。
4. **阈值反机械**：触发条件 = 操作性质（越权/写入/提权）+ payload 特征 + 时间间隔 的组合，并带小概率随机延迟/放行，避免「第 N 次必触发」被探测。
5. **绝对隔离**：蜜罐进程独立二进制/独立端口/独立数据目录；出站流量白名单为空（禁 C2 回连、禁当跳板）；凭据全部一次性生成。

## 2. 数据模型

```
honeypot_sessions(id, hid, src_ip, src_port, ua_raw, ua_norm, asn, country,
                  first_seen, last_seen, ja3, ja4, verdict)
honeypot_events(id, session_id, kind, path, method, payload_hash, payload_excerpt,
                risk, created_at)              -- kind: env_access|login|jwt_forged|privilege_ops|
                                               --       blocked_action|verify_attempt|verify_page
honeypot_fingerprints(id, session_id, canvas_hash, webgl_hash, fonts_hash, tz, lang,
                      screen, dpr, cores, memory, client_hints, webrtc_ip, headless_flags, created_at)
```
- **append-only**：三表只 INSERT，不 UPDATE/DELETE（除 TTL 归档）；`payload_hash` 与日志归档 SHA256 对齐可验证。
- 敏感面处理：`payload_excerpt` 经洗白（去 Cookie/Authorization 原值，只留结构），**绝不记录真实用户 key**（过滤 `api_key`/`sk-`/`Bearer`）。
- 指纹属个人数据：默认 **90 天**保留、仅管理员/警方角色可读、导出需审批留痕。

## 3. 后端落点（Go）

```
server/honeypot/
├── decoy.go        # 假 .env 内容生成（每实例唯一 canary、运维口吻备注）
├── jwt.go          # 蜜罐独立密钥签发/验签（hid 写入 payload，与真实 JWT 密钥物理隔离）
├── shadowauth.go   # 影子认证：校验伪造 token、返回合成用户/资料
├── tripwire.go     # 越权/写入/提权判定 + 随机化节奏 + 「身份异常」响应
├── verify.go       # 特殊人机验证：恒失败、无状态、采集入口
├── fingerprint.go  # 指纹标准化（UA→家族/版本、JA3/JA4 解析、headless 标记）
├── event.go        # 事件写入（异步批量、失败落本地队列）
└── routes.go       # 诱饵路由（挂在主 gin 引擎上，与业务路由共存、互不干扰）
```
- **内嵌主程序**：蜜罐是主程序的一个包，诱饵/影子认证/验证页/SQLi 诱饵都注册到主 gin 引擎，
  **不产出第二个二进制**——站长只部署 `nvs-server.exe` 一个文件。
  （注：本项目 v3.16 曾提供独立进程 `nvs-honeypot` 形态，v3.20.1 按站长要求收回归内嵌，
  理由：单二进制部署更简单，且诱饵本身无状态、不需要独立扩缩容。）
- 蜜罐密钥：启动时 `honeypot/secret.key`（0600），与 `JWT_SECRET` 不共用；`hid` = HMAC(secret, 会话序号) 前 16 hex。
- 指纹页：Go 模板渲染（与主站同款 CSS 变量），内联脚本采集后 `POST /hp/fp`，响应恒 `{"ok":false,"reason":"risk_control"}`。

## 4. 管理端（新面板 `/admin/honeypot`）

- 会话列表：IP/ASN、首次/最近、UA 家族、命中诱饵、风险分、是否到过验证页
- 会话详情：事件时间轴（从 .env 命中 → 伪造 token → 越权尝试 → 验证绕过）、指纹卡、payload 结构对比
- 运营动作：分级封禁（IP/设备指纹/UA 规则）、取证包导出（zip：事件+指纹+日志哈希，**导出需二次确认并记录审批人**）、规则反哺（把命中的 XSS/爬虫特征写回 security 规则库）
- 与警方取管台联动：蜜罐事件作为证据链附件（不改写既有「溯源辅助」定位）

## 5. 分期

> 下表是**原计划**；实际落地状态见紧随其后的 §5.1（M1 取内嵌形态，M2/M3/M4 已于 v3.14~v3.15 实现）。

| 期 | 内容 | 验收 |
|---|---|---|
| **M1（2~3 天）** | 假 `.env` + 独立蜜罐进程 + 事件表 + 会话列表（先跑通「谁扫到了」） | 访问 `/hp/.env` 产生 1 条 session + 1 条 env_access；主站零影响 |
| **M2** | 蜜罐 JWT + 影子认证 + 低风险放行 + 越权返回「权限不足」并记录（简化版先不做验证页） | 用 `JWT_SECRET` 伪造 token 能拿到合成资料；访问 `/admin` 被拒并记录 jwt_forged/privilege_ops |
| **M3** | 「先甜后卡」：触发「身份异常」+ 特殊人机验证页（恒失败）+ 指纹前置采集 | 越权操作 → 验证页；指纹落库并与 session 关联；验证提交 100% 失败 |
| **M4** | 取证包导出 + 规则反哺 + 警方联动 + 90 天 TTL 归档 | 导出 zip 含哈希清单且审批留痕 |

## 5.1 实现现状（v1，`server/honeypot`）

v1 先落「主站内嵌诱饵」形态（不启独立进程/端口），把「谁在扫、扫到什么、要不要封」跑通：

**等级（站长/管理员可配，`honeypot_settings.level`）**

| 等级 | 挂载诱饵 | 行为 |
|---|---|---|
| `off` | 无 | 所有诱饵路径返回与普通 404 一致的响应，蜜罐存在不可见 |
| `low` | `/.env` | 只记录 |
| `medium` | `/.env`、`/.git/config`、`/admin.php`、`/robots-disallow.txt` | 只记录 |
| `high` | 全量（+`/phpinfo.php`、`/actuator/env`、`/wp-login.php`、`/backup/`） | 可选延迟响应（≤2s）+ 命中即封禁 |

- 诱饵路径**无条件注册**，命中后再按当前等级判定是否生效；等级热更新（15s 缓存）。
- 探路记录：`/.git/HEAD`、`/phpmyadmin/`、`/swagger-ui.html` 等常见扫描路径，
  以及 NoRoute 里命中 `.php/.env/.git/admin/backup/config/actuator/swagger` 的可疑 404，都静默记一条 `decoy_probe`。
- 高等级 `auto_ban` 命中即封（进程内黑名单，重启清空；导出/解封在后台）。

**日志记录字段（管理员勾选，`honeypot_settings` 同表）**

| 字段 | 默认 | 说明 |
|---|---|---|
| `record_ua` | ✅ | UA |
| `record_referer` | ✅ | Referer |
| `record_xff` | ❌ | X-Forwarded-For（含代理链与内网信息，默认不记） |
| `record_query` | ✅ | Query |
| `record_headers` | ❌ | 请求头，仅白名单；`Cookie/Authorization/X-API-Key` **强制永不记录** |
| `record_body_snippet` | ❌ | Body 摘要，含 password/token/secret 字样时整段省略 |

- 这套策略**同时作用于访问日志**（`middleware/access_log.go`）：关掉的字段不落库。
- 区别于「日志脱敏」（`/admin/log-redact`）：脱敏管「记了之后遮」，本策略管「要不要记」。

**接口**：`GET/PUT /api/admin/honeypot/config`、`GET /honeypot/sessions`、`GET /honeypot/sessions/:sid`、
`GET /honeypot/export`（导出留痕）、`POST /honeypot/cleanup`、`GET /honeypot/bans`、`POST /honeypot/ban|unban`。

**M2/M3/M4 已落地（v3.15.0）**

| 模块 | 实现 | 关键约束 |
|---|---|---|
| 影子认证 | `jwt.go` + `shadowauth.go`：蜜罐独立密钥 `data/honeypot/secret.key`（0600），诱饵 `.env` 暴露的 `JWT_SECRET` **就是这把密钥**；`/api/v2/*` 下读接口返回合成数据，写入/提权返回 403「检测到异地登录，请重新验证」+ 验证页地址 | 与真实 `JWT_SECRET` 物理隔离；`alg=none` 拒绝；卡口**没有成功分支** |
| 验证页 | `verify.go`：`/hp/verify`（内联采集脚本，`DOMContentLoaded` 即采）+ `/hp/fp`（上报恒失败）+ `/hp/verify`（提交恒失败、`retry:true` 诱导继续） | 页面不含任何 success 分支；检测到 headless **只标记不拒绝**（拒绝会暴露蜜罐） |
| 指纹 | `honeypot_fingerprints`：canvas/webgl/fonts/tz/lang/screen/dpr/cores/memory/hints/headless/raw | 同会话同指纹去重；仅管理员可读；随留存天数清理 |
| 取证包 | `forensics.go`：`GET /api/admin/honeypot/export.zip` → `session.json`/`events.jsonl`/`fingerprints.json`/`manifest.json`（SHA256 清单），响应头带 `X-Bundle-SHA256`，下载后删临时包 | 导出写一条 `export` 事件（审批留痕） |
| 规则反哺 | `feedback.go`：从风险 ≥60 的事件提炼 UA/路径特征 → 写入爬虫阻断名单并 `security.ReloadBotRules()` 即时生效 | 只反哺明确恶意特征；关键词长度 ≥4 防误伤；重复反哺幂等；写 `feedback` 事件留痕 |

**进程形态与留存（v3.20.1 收口：内嵌主程序）**

- **进程**：蜜罐**内嵌主程序**（无独立二进制、无独立端口）。
  - 诱饵、SQLi 假入口、影子认证 `/api/v2/*`、验证页 `/hp/*` 全部由 `main.go` 注册到主 gin 引擎；
  - 等级 `off` 时这些路径与普通 404 无差别，攻击者看不出蜜罐存在；
  - 因为内嵌，**不提供公开的 `/__hp/health` 自检端点**——那等于自报家门；
    运行状态改由管理端确认：`GET /api/admin/honeypot/db`（是否独立库/路径）、
    `GET /api/admin/honeypot/overview`（会话/事件/高风险计数）。
- **独立库（数据隔离，非进程隔离）**：`NVS_HONEYPOT_DB_PATH`（如 `data/honeypot/honeypot.db`）。
  主站管理接口读的也是它（后台照旧能看），但蜜罐数据与业务库**物理隔离**：
  即使诱饵被灌脏数据，也不会污染业务表。启动断言「蜜罐库 ≠ 主库」，同路径直接拒绝启动。
  未设该变量时表落在主库（零配置）。
- **冷归档**：`Cleanup` 改为 `ArchiveAndCleanup` —— 超期事件先写
  `data/honeypot/archive/honeypot-events-YYYYMM.jsonl.gz`，`fsync` 成功后才分批删库；
  `GET /api/admin/honeypot/archives` 可看归档文件。

**仍未做**：JA3/JA4 与 WebRTC 内网 IP 采集（需 TLS/浏览器侧支持）、警方联动工单。

### 5.1.1 影子路由前缀为什么是 `/api/v2/*`

真实站点没有 v2 前缀（不会与主站路由冲突），但对攻击者而言「发现了新版 API」本身就是甜头：
先给合成用户/统计（甜），一旦碰写入就落到卡口（卡）。所有合成数据都是假邮箱/假计数，
**没有任何一条能用于真实站点**，也不会写入真实用户表。

## 6. 风险清单（必读）

1. **验证页是最大暴露面**：模板化/固定 JS 路径/风格不一致 → 一眼识破。必须复用主站 CSS 变量与组件样式。
2. **影子服务绝不能连真实库**：独立 DB 文件（`honeypot.db`），启动时断言 DSN 不含生产库名。
3. **不要收集超必要数据**：指纹字段白名单化，避免合规红线；导出审计。
4. **不要诱导提交真实凭据**：验证页只做「看起来像」，不索取账号密码以外的真实输入，也不把用户输入回显。
5. **运维成本真实存在**：M3 起需要有人持续看事件、调阈值；无人运维时停在 M2。
