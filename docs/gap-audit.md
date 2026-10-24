# 文档「未做项」核对报告（gap audit）

> 口径：把 `docs/` 里所有「未做 / 待做 / 暂不 / 后续 / M2-M4」逐条拉到代码里核对。
> 核对时间：2026-10-24（版本 v3.15.0）
> 结论分三类：**GAP（确实没做）** / **STALE（文档过时，其实已做）** / **BY DESIGN（明确取舍，不算缺口）**

---

## 0. 一句话结论

- **真实缺口 0 项**（v3.19.0 修掉 15 项 + v3.20.0 收口最后 3 项；P0/P1/P2 全清）。
- **文档过时/核对误判 5 项**（多密钥卡池、沉浸模式、蜜罐 M2-M4、大纲记忆、算法透明度页），已标注并修正。
- **明确取舍 4 项**（实时共同编辑、验证页放行分支、headless 直接拒绝、蜜罐记录真实凭据），不算缺口。

> 🔄 2026-10-24 第二轮更新（v3.16.0）
> - ✅ 已完成：#1 蜜罐独立进程、#2 独立 DB + 同路径断言、#5 冷归档、#7 合规材料一键导出
> - ✅ v3.17.0 完成：#10 路由策略（最便宜/最快/按场景）、#11 故障自动降级（必须提示，不静默）
> - ✅ v3.18.0 完成：#16 角色设定 AI 动作、#21 主笔转让
> - ✅ v3.20.0 完成：#17 出图附件与历史导出、#22 气泡长按互动、#23 直播入口（剩余 0 项）
> - ✅ v3.19.0 完成：#3 SQLi 蜜罐、#4 JA3/JA4+WebRTC、#6 警方工单、#9 充值风控、#12 总预算+排队、
>   #13 转化漏斗、#14 注册门槛+设备指纹、#15 三维限流、#18 TTS/音乐、#19 思维导图、
>   #20 自定义菜单、#22 表情包（长按互动除外）、#23 轮播/签到/访客（直播除外）、#24 OC 卡互通，
>   外加「告警口径」：API Key HMAC 审计指纹 + 多 IP 提醒 + 解密失败 + 封禁后仍使用
> - ⚠️ 撤回误判：#8「公开算法透明度页」**其实早已实现**——首页 `HomeNoticeBar.vue` 的「📊 推荐说明」弹窗 +
>   公开接口 `/api/algorithms/description`（第一版只 grep 路由与后台页，漏了组件内弹窗）
> - 仍在清单：#3 #4 #6 #9 #10 #11 #12 #13 #14 #15 #16 #17 #18 #19 #20 #21 #22 #23 #24

---

## 1. 安全 / 合规（9 项）

| # | 项 | 文档出处 | 代码核对 | 结论 | 优先级 |
|---|---|---|---|---|---|
| 1 | 蜜罐独立进程 / 独立端口 | `honeypot-design.md` §1 约束 5 | ✅ v3.16.0：`cmd/honeypot`（`nvs-honeypot.exe`，默认 18081），只挂诱饵/影子/验证页 + `/__hp/health` | 已做 | — |
| 2 | 蜜罐独立 DB + 启动断言 | 同上 §6 风险 2 | ✅ v3.16.0：`NVS_HONEYPOT_DB_PATH` 切独立库，`SetDB` 断言「蜜罐库 ≠ 主库」，同路径拒绝启动 | 已做 | — |
| 3 | SQL 注入蜜罐（预生成响应库） | `requirements-audit.md` 蜜罐段 | ✅ v3.19.0：`honeypot/sqli.go` 假 SQL 入口 + 预生成响应库（假表/假用户/金丝雀）+ 10 类注入特征；事件 sqli_hit/probe | 已做 | — |
| 4 | JA3 / JA4 指纹、WebRTC 内网 IP | `honeypot-design.md` §2 表字段 | ✅ v3.19.0：反代头 X-JA3-Hash/X-JA4 采集（缺失记 missing 不伪造）+ 指纹表加 ja3/ja4/tls_source/tls_version/webrtc_ips + 验证页 RTCPeerConnection 采内网 IP | 已做 | — |
| 5 | 90 天 TTL **冷归档**（而非只删） | 同上 §5 M4 | ✅ v3.16.0：`ArchiveAndCleanup()` 先写 `archive/honeypot-events-YYYYMM.jsonl.gz`（fsync）再分批删；`GET /honeypot/archives` | 已做 | — |
| 6 | 警方联动工单 | 同上 §5 M4 | ✅ v3.19.0：`server/police` 建单/派单/进展/结案（结单必填结论）+ 超期与未派单口径 + 操作日志 | 已做 | — |
| 7 | 一键导出合规检查材料 | `requirements-audit.md` 等保条 | ✅ v3.16.0：`GET /api/admin/compliance/bundle.zip`（报告+算法说明+定级+漏洞+用户汇总+蜜罐摘要+日志清单+SHA256 manifest），导出写审计 | 已做 | — |
| 8 | ~~公开的算法透明度说明页~~ | 同上「算法透明度」条 | **误判**：首页 `HomeNoticeBar.vue`「📊 推荐说明」弹窗 + `GET /api/algorithms/description` 早已存在 | STALE（我漏查组件） | — |
| 9 | 充值异常 / 退款与风控联动 | `requirements-audit.md` 支付条 | ✅ v3.19.0：`server/risk` 入账前打分（蜜罐 IP/同设备多账号/高频/退款率/新号大额），allow/hold/block 三处置，回调接入 | 已做 | — |

补充：i18n 中英双语（`requirements-audit.md` N-03）—— 前端无 `vue-i18n`，**GAP**，但若只做国内站可降为 P2。

## 2. AI 平台能力（11 项）

| # | 项 | 文档出处 | 代码核对 | 结论 | 优先级 |
|---|---|---|---|---|---|
| 10 | 路由策略：最便宜 / 最快 / 场景自动 | `requirements-audit.md` 池子条 | ✅ v3.17.0：`models/ai_router.go`（cheapest=价+价、fastest=近 7 天实测延迟、scene=场景覆盖、manual=优先级）+ 后台「AI 模型池 → 路由策略」 | 已做 | — |
| 11 | 故障自动降级（必须提示，不静默） | 同上 | ✅ v3.17.0：`aicall.Complete` 降级链 + `Result.Notice()`；OC 发言与群内 AI 已接入，正文尾部标注「模型已降级：A → B（原因）」 | 已做 | — |
| 12 | 全站总预算上限、高峰排队 | 同上 | ✅ v3.19.0：`models/ai_budget.go` 日/月上限（HardStop 可软可硬）+ 并发闸门与排队超时 + 峰时上限 | 已做 | — |
| 13 | 转化漏斗（免费额度 → 付费） | 同上「成本监控」 | ✅ v3.19.0：`GET /api/admin/ai/funnel` 六段漏斗（去重用户）+ 最弱一跳洞察 | 已做 | — |
| 14 | 注册后 N 小时门槛 / 设备指纹防刷 | 同上「防刷」 | ✅ v3.19.0：`server/gate` 邀请码 + 设备指纹绑定 + 账号年龄 + 新号每日上限 + 临时邮箱 + 蜜罐 IP；注册/评论/发文三处生效 | 已做 | — |
| 15 | 三维交叉限流（人 / 模型 / 时间窗） | 同上 | ✅ v3.19.0：`models/ai_throttle.go` 用户×模型 / 用户 / 模型 三窗口 + 快照 | 已做 | — |
| 16 | 角色设定专用 AI 动作 | 同上「生成大纲 / 角色设定」 | ✅ v3.18.0：`AI_ACTIONS` 新增 `character`（后端 `aiSystemPrompt` + 流水标签），输出结构化角色卡 | 已做 | — |
| 17 | 附件上传、历史导出（AI 出图） | 同上 | ✅ v3.20.0：`POST /ai/media/attach` 附件落库（`AiMediaTask.Attachments`）+ `GET /ai/export/media.zip`（items.json + files/ + manifest SHA256）+ 单任务导出；参考图同时上传站内副本 | 已做 | — |
| 18 | TTS / 音乐（音频模型） | `sparkai-reference.md` | ✅ v3.19.0：`aiclient/audio.go` + `POST /api/ai/speech`（kind=audio，2000 字上限，落 uploads）+ 阅读器朗读按钮 | 已做 | — |
| 19 | 思维导图 | 同上 | ✅ v3.19.0：AI_ACTIONS 新增 mindmap（约束输出 mermaid mindmap，阅读器已能渲染） | 已做 | — |
| 20 | 自定义应用菜单（可挂外部） | 同上 | ✅ v3.19.0：`models/nav_menu.go` 两级菜单 + 协议白名单 + 登录/角色可见性 + 导航栏渲染 | 已做 | — |

## 3. 内容 / 玩法（4 项）

| # | 项 | 文档出处 | 代码核对 | 结论 | 优先级 |
|---|---|---|---|---|---|
| 21 | 主笔转让 | `novel-collaboration.md` §9 | ✅ v3.18.0：`collab.Transfer` + `POST /api/novels/:id/transfer`；受让人须为已接受协作者，事务内换主笔+原主笔降 editor，二次确认输入标题，审计留痕 | 已做 | — |
| 22 | 气泡头像自定义 / 长按互动 / 表情包上传 | `novel-script-mode.md` §7 | ✅ v3.20.0 全量：表情包（v3.19）+ 气泡 `data-*` 属性 + 阅读器长按/右键动作面板（复制/引用评论/收藏本章） | 已做 | — |
| 23 | 画廊轮播图 / 直播入口 / 签到奖励 / 访客设置 | `admin-reference.md` 运营条 | ✅ v3.20.0 全量：轮播/签到/访客（v3.19）+ 直播入口（iframe 仅 https + sandbox / 直链 video / 海报与倒计时 / 关联群聊） | 已做 | — |
| 24 | OC 角色卡导出/导入（酒馆 V2 卡互通） | `oc-mode.md` §11.1（借鉴对照时隐含） | ✅ v3.19.0：`oc/card_io.go` V2/V1 双向 + character_book 互通 + extensions.nvs 保留 + 未知字段留存 | 已做 | — |

## 4. 文档过时（STALE，其实已做）

| 项 | 文档现在写的 | 实际 |
|---|---|---|
| 模型多密钥卡池 | `admin-reference.md`「无（每模型单密钥）」 | `models/ai_model_keys`（Label/Weight/FailCount/Enabled）+ `AcquireModelKey` 加权轮询已支持 |
| 沉浸模式 | `sparkai-reference.md`「⏳ 未做」 | `AiHub.vue` 的 `immersive` + `global.css .ai-immersive` 已实现 |
| 蜜罐 JWT / 影子认证 / append-only / 导出审批 | `requirements-audit.md` 四行写 ⛔ | v3.15.0 已落地（`honeypot/jwt.go`、`shadowauth.go`、`forensics.go`、`export` 事件留痕） |
| 生成大纲 | `requirements-audit.md` 写「角色设定未做」（大纲已做） | 大纲：`AiNovelMemory(outline)` 已可用；**只有角色设定动作仍缺** |
| 算法透明度公开页 | 本报告第一版列为 P0 GAP | 实际已有（首页「推荐说明」弹窗 + 公开接口），属**核对误判** |

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
