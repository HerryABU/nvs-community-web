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
└── routes.go       # 独立 gin 引擎（独立端口，不挂主站路由树）
```
- **独立进程**：`nvs-honeypot` 单独二进制 + 单独端口（默认 18081），主站只暴露一条反向代理规则；崩溃不影响主站（N-02 同样的隔离思路）。
- 蜜罐密钥：启动时 `honeypot/secret.key`（0600），与 `JWT_SECRET` 不共用；`hid` = HMAC(secret, 会话序号) 前 16 hex。
- 指纹页：Go 模板渲染（与主站同款 CSS 变量），内联脚本采集后 `POST /hp/fp`，响应恒 `{"ok":false,"reason":"risk_control"}`。

## 4. 管理端（新面板 `/admin/honeypot`）

- 会话列表：IP/ASN、首次/最近、UA 家族、命中诱饵、风险分、是否到过验证页
- 会话详情：事件时间轴（从 .env 命中 → 伪造 token → 越权尝试 → 验证绕过）、指纹卡、payload 结构对比
- 运营动作：分级封禁（IP/设备指纹/UA 规则）、取证包导出（zip：事件+指纹+日志哈希，**导出需二次确认并记录审批人**）、规则反哺（把命中的 XSS/爬虫特征写回 security 规则库）
- 与警方取管台联动：蜜罐事件作为证据链附件（不改写既有「溯源辅助」定位）

## 5. 分期

| 期 | 内容 | 验收 |
|---|---|---|
| **M1（2~3 天）** | 假 `.env` + 独立蜜罐进程 + 事件表 + 会话列表（先跑通「谁扫到了」） | 访问 `/hp/.env` 产生 1 条 session + 1 条 env_access；主站零影响 |
| **M2** | 蜜罐 JWT + 影子认证 + 低风险放行 + 越权返回「权限不足」并记录（简化版先不做验证页） | 用 `JWT_SECRET` 伪造 token 能拿到合成资料；访问 `/admin` 被拒并记录 jwt_forged/privilege_ops |
| **M3** | 「先甜后卡」：触发「身份异常」+ 特殊人机验证页（恒失败）+ 指纹前置采集 | 越权操作 → 验证页；指纹落库并与 session 关联；验证提交 100% 失败 |
| **M4** | 取证包导出 + 规则反哺 + 警方联动 + 90 天 TTL 归档 | 导出 zip 含哈希清单且审批留痕 |

## 6. 风险清单（必读）

1. **验证页是最大暴露面**：模板化/固定 JS 路径/风格不一致 → 一眼识破。必须复用主站 CSS 变量与组件样式。
2. **影子服务绝不能连真实库**：独立 DB 文件（`honeypot.db`），启动时断言 DSN 不含生产库名。
3. **不要收集超必要数据**：指纹字段白名单化，避免合规红线；导出审计。
4. **不要诱导提交真实凭据**：验证页只做「看起来像」，不索取账号密码以外的真实输入，也不把用户输入回显。
5. **运维成本真实存在**：M3 起需要有人持续看事件、调阈值；无人运维时停在 M2。
