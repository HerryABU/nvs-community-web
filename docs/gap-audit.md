# 文档「未做项」核对报告（gap audit）

> 口径：把 `docs/` 里所有「未做 / 待做 / 暂不 / 后续 / M2-M4」逐条拉到代码里核对。
> 核对时间：2026-10-24（版本 v3.15.0）
> 结论分三类：**GAP（确实没做）** / **STALE（文档过时，其实已做）** / **BY DESIGN（明确取舍，不算缺口）**

---

## 0. 一句话结论

- **真实缺口 24 项**，其中安全合规 9 项、AI 平台能力 11 项、内容玩法 4 项。
- **文档过时 4 项**（多密钥卡池、沉浸模式、蜜罐 M2-M4、大纲记忆），已在本文标注并同步修正原文档。
- **明确取舍 5 项**（实时共同编辑、SQLi 蜜罐的「预生成响应库」路线、独立蜜罐 DB 等），不算缺口。

---

## 1. 安全 / 合规（9 项）

| # | 项 | 文档出处 | 代码核对 | 结论 | 优先级 |
|---|---|---|---|---|---|
| 1 | 蜜罐独立进程 / 独立端口 | `honeypot-design.md` §1 约束 5、§5 M1 | 只有 `honeypot.Register(r)` 内嵌主站；无 `cmd/honeypot` | GAP | **P0**（隔离失败影响主站） |
| 2 | 蜜罐独立 DB（`honeypot.db`）+ 启动断言 DSN 不含生产库 | 同上 §6 风险 2 | 与主库同库同表前缀 | GAP | **P0** |
| 3 | SQL 注入蜜罐（预生成响应库） | `requirements-audit.md` 蜜罐段 | 无 sqli 相关代码（真实链路全参数化，注入打不穿） | GAP | P1 |
| 4 | JA3 / JA4 指纹、WebRTC 内网 IP | `honeypot-design.md` §2 表字段 | `honeypot_fingerprints` 无 ja3/ja4/webrtc 列 | GAP | P1 |
| 5 | 90 天 TTL **冷归档**（而非只删） | 同上 §5 M4、§2 | `Cleanup()` 直接 `Delete`，无归档表/归档文件 | GAP | **P0**（合规留存） |
| 6 | 警方联动工单 | 同上 §5 M4 | 有 `police` 角色与取证下载，无工单流转 | GAP | P2 |
| 7 | 一键导出合规检查材料 | `requirements-audit.md` 等保条 | `/admin/compliance`、`/admin/vulnerabilities` 有页面；`grep func.*Export` 无命中 | GAP | **P0**（等保要材料） |
| 8 | 公开的算法透明度说明页 | 同上「算法透明度」条 | 只有后台 `AdminAlgorithms.vue`（配置用），无前台公开页 | GAP | **P0**（算法备案口径） |
| 9 | 充值异常 / 退款与风控联动 | `requirements-audit.md` 支付条 | 视频失败自动退款有；与蜜罐/风控无联动 | GAP | P1 |

补充：i18n 中英双语（`requirements-audit.md` N-03）—— 前端无 `vue-i18n`，**GAP**，但若只做国内站可降为 P2。

## 2. AI 平台能力（11 项）

| # | 项 | 文档出处 | 代码核对 | 结论 | 优先级 |
|---|---|---|---|---|---|
| 10 | 路由策略：最便宜 / 最快 / 场景自动 | `requirements-audit.md` 池子条 | 只有「用户手选模型 + 厂商筛选」；`grep 最便宜/cheapest/AutoRoute` 无命中 | GAP | **P0**（成本） |
| 11 | 故障自动降级到备用模型（且必须提示，不静默） | 同上 | 失败即报错、不扣款，但无自动切备用 | GAP | **P0** |
| 12 | 全站总预算上限、高峰排队 | 同上 | 有单次上限/每日限额/群级预算；无全站总预算与排队 | GAP | P1 |
| 13 | 转化漏斗（免费额度 → 付费） | 同上「成本监控」 | 有消耗统计与开关，无漏斗 | GAP | P2 |
| 14 | 注册后 N 小时门槛 / 设备指纹防刷 | 同上「防刷」 | 有每日限额 + 速率限制；`grep device_fingerprint` 无命中 | GAP | P1 |
| 15 | 三维交叉限流（人 / 模型 / 时间窗） | 同上 | 用户级速率 + 每日限额 + 单模型调用，未做三维交叉 | GAP | P2 |
| 16 | 角色设定专用 AI 动作 | 同上「生成大纲 / 角色设定」 | `AI_ACTIONS` 只有 continue/polish/expand/summarize/custom；大纲已由记忆类型支持 | GAP（角色设定） | P1 |
| 17 | 附件上传、历史导出（AI 出图） | 同上 | 有 max_tokens/比例/张数/风格，无附件与历史导出 | GAP | P2 |
| 18 | TTS / 音乐（音频模型） | `sparkai-reference.md` | 模型池有 kind 扩展位，无实现 | GAP | P2 |
| 19 | 思维导图 | 同上 | 只有 Reader 的 Mermaid 渲染（能画，但没有「生成思维导图」功能） | GAP | P2 |
| 20 | 自定义应用菜单（可挂外部） | 同上 | 有「扩展应用 /frames」，无自定义菜单配置 | GAP | P2 |

## 3. 内容 / 玩法（4 项）

| # | 项 | 文档出处 | 代码核对 | 结论 | 优先级 |
|---|---|---|---|---|---|
| 21 | 主笔转让 | `novel-collaboration.md` §9 | `server/collab` 无 transfer 字段与接口，前端无入口（文档写「字段与文档预留」，实际**字段也没建**） | GAP | P1 |
| 22 | 气泡头像自定义 / 长按互动 / 表情包上传 | `novel-script-mode.md` §7 | 只有 `emote_map`（情绪→URL）与内置表情词典，无上传与管理 | GAP | P2 |
| 23 | 画廊轮播图 / 直播入口 / 签到奖励 / 访客设置 | `admin-reference.md` 运营条 | `grep 签到/carousel/签到` 无命中 | GAP | P2（运营侧，非主线） |
| 24 | OC 角色卡导出/导入（酒馆 V2 卡互通） | `oc-mode.md` §11.1（借鉴对照时隐含） | 无 import/export 卡片接口 | GAP | P2 |

## 4. 文档过时（STALE，其实已做）

| 项 | 文档现在写的 | 实际 |
|---|---|---|
| 模型多密钥卡池 | `admin-reference.md`「无（每模型单密钥）」 | `models/ai_model_keys`（Label/Weight/FailCount/Enabled）+ `AcquireModelKey` 加权轮询已支持 |
| 沉浸模式 | `sparkai-reference.md`「⏳ 未做」 | `AiHub.vue` 的 `immersive` + `global.css .ai-immersive` 已实现 |
| 蜜罐 JWT / 影子认证 / append-only / 导出审批 | `requirements-audit.md` 四行写 ⛔ | v3.15.0 已落地（`honeypot/jwt.go`、`shadowauth.go`、`forensics.go`、`export` 事件留痕） |
| 生成大纲 | `requirements-audit.md` 写「角色设定未做」（大纲已做） | 大纲：`AiNovelMemory(outline)` 已可用；**只有角色设定动作仍缺** |

## 5. 明确取舍（BY DESIGN，不算缺口）

| 项 | 文档口径 | 说明 |
|---|---|---|
| 实时共同编辑（OT/CRDT、光标同步） | `novel-collaboration.md` §9「暂不做」 | 与「章节正文落文件」架构冲突，成本高；当前用版本控制 + 冲突检测替代 |
| 验证页「放行」分支 | `honeypot-design.md` §1 约束 1 | 永不实现（恒失败是设计） |
| headless 检测直接拒绝 | 同上 约束 3 | 只标记不拒绝（拒绝会暴露蜜罐） |
| 蜜罐拒绝记录真实凭据 | 同上 约束 4 | 永不实现（合规红线） |
| 国密 gm 轨端到端握手 / 故障注入 | `gm-mapping.md` §未做 | 属**站长环境验证**（Windows 开发机无 Tongsuo），非代码缺口 |

---

## 6. 建议的下一步顺序

**第一批（P0，安全合规闭环，约 1 个版本）**
1. 蜜罐独立进程 + 独立 DB（`cmd/honeypot`，端口 18081，DSN 断言）
2. 蜜罐冷归档（事件超期转 `honeypot_archive` / 归档文件，删前可校验）
3. 合规材料一键导出（等保自测 + 漏洞整改 + 审计摘要 → zip）
4. 前台算法透明度说明页（可后台编辑，走审核）
5. AI 路由策略 + 自动降级（先做「按价格排序 + 失败切备用 + 明确提示」）

**第二批（P1，能力补齐）**
6. 全站总预算 + 高峰排队
7. 注册时长门槛 + 设备指纹（复用蜜罐指纹表，需用户授权口径）
8. 主笔转让（补字段 + 接口 + 二次确认）
9. 角色设定 AI 动作
10. JA3/JA4 与 SQLi 蜜罐（取证深度）

**第三批（P2，运营/体验）**
11. TTS/音乐、思维导图、自定义菜单、画廊轮播、签到、访客设置
12. 气泡头像/表情包上传、OC 卡导入导出
13. 转化漏斗、三维限流、警方工单

---

**维护约定**：本文与 `requirements-audit.md` 冲突时，以本文（最新核对）为准；
每轮版本发布前把「已完成」项从 GAP 移到 STALE/已完成并回填原文档。
