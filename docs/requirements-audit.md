# 需求实现对照检查（NVS 后续阶段 + 国密改造）

> 检查时间：2026-10-23；代码基线：`v3` 分支（本次顺延至 3.8.0）。
> 口径：**✅ 已实现**（前后端都有，且真的生效）｜**🟡 部分**（有基础能力，缺文中指定细节）｜**⛔ 未做**（无代码）。
> 说明：两文档中的「站长责任」项（定价、资质、备案、密评、内容定位）不列为开发缺口。

---

# 一、《NVS 后续阶段需求文档》

## 1. 基础定位与边界
| 需求 | 状态 | 落点 / 缺口 |
|---|---|---|
| 类型文学定位、默认不预置爽文模板 | ✅ | `models/ai_prompt.go` 平台预设仅写作/编程/学习/翻译/绘画/视频，无爽文模板 |
| 提示词模板引擎（变量注入、版本管理、按分类挂载、测试预览、优先级） | 🟡 | 已有：提示词库 + `{{变量}}` 占位 + 分类 + 启用/排序；缺：版本管理、变量自动注入（作品上下文/角色/前文摘要/作者样本）、测试预览 |
| 站长责任边界声明 | ✅ | README + 各管理页提示文案 |

## 2. AI 写作与 AI 审核
| 需求 | 状态 | 落点 / 缺口 |
|---|---|---|
| 编辑区 AI：续写/润色/扩写/摘要/改写/自定义 | ✅ | `components/editor/AiAssistPanel.vue`；章节编辑器 + **博客编辑器（本轮新补）** |
| 生成大纲 / 角色设定 | 🟡 | 大纲：`AiNovelMemory(outline)` + 编辑区动作；角色设定：未做专用动作 |
| 章节摘要缓存 | ✅ | `models/ai_memory.go`（novel 大纲 + 章节摘要 + 预算组装） |
| AI 输出回填编辑器、保存时走审核 | ✅ | `@insert/@replace` → 编辑器；保存走统一内容审核管线 |
| AI 生成内容标识（显式+隐式） | 🟡 | `@ai` 回复有「AI」徽标；编辑区 AI 产出发布时**未统一打标**（缺显式标识字段） |
| AI 审核覆盖章节/评论/论坛/博客/群组/分享 | 🟡 | 违禁词库 + 举报 + 内容扫描已覆盖章节/评论/论坛/博客/分享；群组功能不存在故未覆盖 |
| 人工兜底 | ✅ | 管理端审核队列（`/admin/reports`、`/admin/html-review` 等） |

## 3. 合规底座
| 需求 | 状态 | 落点 |
|---|---|---|
| 实名五提供商 + 敏感凭据脱敏回显 | ✅ | `server/security/realname`、`/admin/realname` |
| 违禁词库（关键词/正则 × 标记/删除/替换 × 范围） | ✅ | `/admin/badwords` |
| 算法透明度说明 + 个性化推荐关闭权 | 🟡 | 关闭权有（推荐开关）；公开算法说明页未做 |
| 日志按小时切分 / SHA256 归档 / 6 个月 / 脱敏 | 🟡 | 有日切分 + 脱敏（`utils/dailylog`、`LOG_REDACT_SENSITIVE`）；SHA256 归档与 6 个月策略需站长侧配置，缺内置轮转清理 |
| 举报/申诉入口 | ✅ | 评论/内容举报 + 我的举报 |
| 等保/关基自测、漏洞整改闭环、检查材料导出 | 🟡 | `/admin/compliance`、`/admin/vulnerabilities` 已有；一键导出材料未做 |
| 未成年人保护（核验/minor_mode/额外限制） | ✅ | `/admin/realname`、minor 限制 |

## 4. 蜜罐与取证体系
| 需求 | 状态 | 说明 |
|---|---|---|
| 诱捕链（假 .env → 蜜罐 JWT → 假被攻破页 → 身份异常提示 → SQL 注入模拟） | ⛔ | **未实现**。当前只有「隔离墙 / zone-access / web-access 审计」与警方取管台取证包 |
| 蜜罐 JWT（独立密钥 + hid + 会话表） | ⛔ | 未实现 |
| SQL 注入蜜罐（预生成响应库） | ⛔ | 未实现（真实链路全部参数化查询，注入打不穿） |
| 蜜罐事件 append-only + 哈希对齐 + 导出审批 | ⛔ | 未实现 |
| 警方定位为「溯源辅助」 | ✅ | `/police-console` 已按此定位 |

> 该项是**整套子系统**，不是补丁式改动。建议单独立项：`server/honeypot`（假 .env 生成器 / 蜜罐 JWT 签发 / 会话表 / 载荷匹配响应库 / 事件表 / 威慑页）+ 管理端「蜜罐事件」面板 + 取证包导出审批。

## 5. AI 池子与定价
| 需求 | 状态 | 落点 |
|---|---|---|
| 文本池 + 绘画池（视频池） | ✅ | `AiModel.kind = chat/image/video` |
| 模型身份卡（名称/提供商/模型 ID/能力/单价/上下文/流式/健康/优先级） | ✅ | 模型池 + **vendor 厂商字典** + `api_kind` + `priority` |
| Key AES-256-GCM 加密、日志/审计不出现明文 | ✅ | `EncryptField/DecryptField`；接口只回传 `has_api_key` 与尾 4 位 hint |
| **多密钥卡池 + 权重 + 健康状态** | ✅ | 本轮新增 `ai_model_keys`：权重轮询、失败计数、连续 3 次自动停用、成功清零、探活测试、池空回退 |
| 价格按输入/输出或按次 | ✅ | `price_in/price_out/price_call` |
| 路由策略（最便宜/最快/用户可选/场景自动） | 🟡 | 用户可选（面板选模型）+ 厂商筛选；**最便宜/最快自动路由未做** |
| 故障降级必须提示、不静默 | 🟡 | 失败会明确报错且不扣款；**自动降级到备用模型未做**（需先做路由策略） |

## 6. 免费 AI
| 需求 | 状态 | 落点 |
|---|---|---|
| 平台免费额度 + 按人覆盖 | ✅ | `/admin/ai-free-quota`，`AiFreeQuotaConfig` / `UserAiFreeQuota` |
| 额度/能力限制、免费优先于扣费 | ✅ | `CheckAiFreeQuota` → 全链路（对话/绘画/视频/思维导图） |
| 总预算上限、高峰排队 | ⛔ | 未做 |
| 防刷（注册后 N 小时、设备指纹、验证后可用） | 🟡 | 有每日限额 + 速率限制；设备指纹与注册时长门槛未做 |
| 成本监控（消耗/剩余/转化/暂停开关） | 🟡 | 有消耗统计（审计页）+ 开关（允许/不允许）；转化漏斗未做 |

## 7. 代币、卡密与支付
| 需求 | 状态 | 落点 |
|---|---|---|
| 代币 = 平台内积分（不可提现/转账） | ✅ | `token.go` + 钱包页文案 |
| 与作者收益两套账 | ✅ | `TokenLedger` 与收益账分离 |
| 聚合支付（微信/支付宝）+ 回调验签 + 幂等 + 对账 | ✅ | `server/payment`（AggregateProvider + MockProvider）、`MarkOrderPaid` 幂等 |
| 卡密（随机/哈希/一次性展示/有效期/比例固定） | ✅ | `/admin/token` 卡密管理 + 兑换 |
| 账本可查/可对账/不可篡改（append-only） | ✅ | `AppendTokenLedger` + 流水页 |
| 计费：预估 → 确认 → 扣款 → 快照 → 单次上限 | 🟡 | 有预估/预检/快照/流水；**单次上限 + 二次确认弹窗**未做 |
| 退款规则、异常充值风控联动 | 🟡 | 视频失败自动退款；支付退款流程与蜜罐联动未做 |

## 8. 用户自备 API Key
| 需求 | 状态 | 说明 |
|---|---|---|
| 后端解密路径（加密存储、运行时短暂解密、不落盘） | ✅ | `UserAiKey`（AES-256-GCM）+ 只写不读 + 上限 5 + 默认密钥 |
| **前端本地解密路径**（密码派生密钥、服务端不可见明文） | ⛔ | 未实现（当前是「后端解密」单路径） |
| 用户密码派生（Argon2id/scrypt/PBKDF2） | ⛔ | 现用服务端机器密钥（`utils.EncryptField`），非用户密码派生 |
| 审计用 HMAC 指纹、异常解密告警、多 IP 提醒 | 🟡 | 有 `LastUsedAt` 与使用上限；HMAC 指纹与告警未做 |

## 9. 个人信息泄漏提示
| 需求 | 状态 |
|---|---|
| 公开输入区格式类（手机/邮箱/SK-/身份证）拦截提示、语义类提示但允许发送、检测不留痕、与 API Key 体系联动 | ⛔ **未做**（整项缺失） |

> 低成本落地建议：前端统一 `useLeakGuard()` 组合式函数 + 提交前本地正则扫描（不上报内容，只上报「命中类型计数」），评论区/发帖/章节/简介挂载。

## 10. 群组功能
| 需求 | 状态 |
|---|---|
| 群组可见性/角色/治理/额度/未成年人限制/文件隔离/实时消息 | ⛔ **未做**（论坛为公开版块模型，无群组实体） |

## 11. `@ai` 机器人
| 需求 | 状态 | 落点 |
|---|---|---|
| 系统身份、权限白名单、独立限流、独立审计 | ✅ | `ai_assistant` 系统用户 + `ai_bot_config` + `AiBotUsage` |
| 消耗发起人卡密/代币、流水可对账 | ✅ | 按发起人 `userID` 计费入账 |
| 公开区先审后发 / 群组先发后审 | 🟡 | 仅公开评论区（无群组）；评论走统一审核 |
| AI 回复过违禁词/扫描/XSS 净化 + AI 标识 + 可举报删除 | 🟡 | 有 XSS 净化 + AI 徽标 + 可删除；AI 回复未单独跑违禁词扫描（依赖评论管线） |
| Prompt 注入防护（输入/输出统一过滤） | 🟡 | 有长度/去重/限流；**注入特征过滤未专门实现** |

## 12. AI 工作台
| 需求 | 状态 | 落点 |
|---|---|---|
| 多轮对话 / 会话管理 / 分享到社区 | ✅ | `/ai`（会话、重命名、分享令牌、公开只读页） |
| 文本 + 绘画 + 视频 | ✅ | 三 Tab + 混图/换脸（可关） |
| 文件上传、历史导出、参数配置 | 🟡 | 有 `max_tokens`、比例、张数、风格；**附件上传与历史导出未做** |
| 私密 → 公开须重新审核 | 🟡 | 分享是只读快照；作品发布**未强制再过审核**（缺发布审核钩子） |
| 图片独立存储 + 权限 + 水印 + 色情/暴力检测 | ⛔ | 图片为上游 URL 或 data URL 直传，无水印/无内容检测 |
| 三维限流（人/模型/时间窗口） | 🟡 | 有用户级速率 + 每日限额 + 单模型调用；三维交叉未做 |

## 13. 场景化提示词与编辑区 AI
| 需求 | 状态 | 落点 |
|---|---|---|
| 模板引擎 + 类型文学默认框架 | 🟡 | 提示词库（预设 19 条、分类、变量占位）；缺引擎（见 1 节） |
| 编辑区 AI 计费（与 @ai、工作台共用账本） | ✅ | 同一 `AiCallLog` / 代币账本 |
| 每日发文限额按等级差异化 + 可选购买额度 | ✅ | 早期 F 阶段配额体系（等级 × 上限，可按人覆盖） |
| AI 辅助内容计入每日发文限额 | 🟡 | 计费与发布都记在用户头上，但**未把「AI 生成标记」作为限额判定依据** |
| 与群组/Web 额度统一配额体系 | 🟡 | 统一走用户配额；群组维度无 |

## 14–15. 全站目标与验收要点
- 价格/额度/等级/模型池/支付/限额**后台可配**：✅（`/admin/ai-features`、`/admin/ai-pool`、`/admin/payment` 等）
- AI 调用可预估/可确认/可流水/可快照：🟡（缺二次确认）
- 蜜罐事件 append-only/可验证/导出留痕：⛔
- 用户 key 加密/脱敏/蜜罐过滤/指纹审计：🟡（加密与脱敏 ✅，指纹与蜜罐过滤 ⛔）

---

# 二、《国密（SM）改造需求文档》

| 章节 | 需求 | 状态 | 落点 / 说明 |
|---|---|---|---|
| 3 | go.mod 默认零国密依赖 | ✅ | `server/go.mod` 无 gmsm/zhenyi/tongsuo；`make build` 可证 |
| 3 | 构建标签隔离 `build_intl.go` / `build_gm.go` | ✅ | `server/crypto/intl.go`（`!gm`）、`gm.go`（`gm`）、`gm_purego.go`（`gm && !tongsuo`）、`gm_tongsuo.go`（`gm && tongsuo`） |
| 3 | `go.gm.mod` + Makefile 双轨目标 | ✅ | 本轮新增根 `Makefile`（build / build-gm / build-gm-cgo / test-gm / tidy-gm）与 `go.gm.mod` |
| 4.1 | `config.CryptoConfig` + 轨道 fail-fast | ✅ | `server/config/crypto.go`（track=gm/dual 未选 provider 直接启动失败） |
| 4.2 | `crypto.Provider` 接口（Name / BuildServerTLSConfig / SupportedCipherSuites） | ✅ | `server/crypto/provider.go` |
| 4.2 | 国际轨实现（标准 crypto/tls） | ✅ | `crypto/intl.go` |
| 4.2 | 国密轨实现（Tongsuo / 纯 Go） | ✅ | `crypto/gm.go` 按 `GMProvider` 分发 |
| 5.1 | `cert.LoadDualCertificate` + SM2 OID 识别（1.2.156.10197.1.301） | ✅ | `server/cert/dual.go` + `dual_gm.go`（gmsm/smx509） |
| 5.2 | 纯 Go gmtls 双证书 + 自适应协商（E011/E051） | ✅ | `crypto/gm_purego.go`（zgmtls `NewBasicAutoSwitchConfig` / `NewListener`） |
| 5.3 | Tongsuo CGO 路径（init 检测 TONGSUO_HOME、ctx 双证书） | ✅ | `crypto/gm_tongsuo.go` |
| 5.4 | 握手可观测 `HandshakeRecord` + JSONL | ✅ | `server/tlsconfig/record.go`（经 VerifyConnection 采集 + 内存统计） |
| 6.1 | `crypto.SecretSource`（env / file 0600） | ✅ | `server/crypto/secret/{env,file,chain}.go` |
| 6.2 | `aiapi.Client` 后端路由 + 前端不持凭据 + 扫描脚本 | ✅ | `server/aiapi/client.go` + `scripts/scan-frontend-secrets.sh`（本轮新增，当前产物 0 命中） |
| 6.3 | `CredentialAgeReport` + CLI 子命令 | ✅ | `server/audit/credential_age.go` + `gmcmd`（`go run . credential-age`） |
| 7.1 | `config.AIConfig`（endpoint/api_key_env/provider_hint/log_retention） | 🟡 | `aiapi` 具备 Endpoint/Secret/ProviderHint；**log_retention_days 未接轮转清理** |
| 7.2 | 合规提示（overseas WARN / domestic INFO / 空 WARN，各一次） | ✅ | `aiapi` 提示去重 |
| 7.3 | 调用元数据日志（不含正文）+ 错误分类 | ✅ | `aiapi` 记录时间/状态码/错误类型（auth/rate_limit/timeout/content_filter/network） |
| 8.1 | `gm-audit` 报告字段齐全（Markdown） | ✅ | `server/audit/gmaudit.go` + `gmcmd` |
| 8.2 | `docs/gm-mapping.md` 映射表 + 免责声明 | ✅ | 本轮新增 |
| 9 N-01 | 默认构建零国密符号 | ✅ | `strings`/`go version -m` 无国密模块 |
| 9 N-02 | dual 模式协议隔离、国密握手异常不影响国际轨 | 🟡 | 代码分支隔离已做；**缺故障注入测试**（见 gm-mapping 第 4 节） |
| 9 N-03 | i18n 中英双语 | ⛔ | 未做（当前中文文案为主） |
| 9 N-04 | 日志本地化、无远程上报 | ✅ | 全部本地文件/stdout |
| 9 N-05 | 不内置地区策略、提供审计钩子 | ✅ | 国密模块不含实名/过滤；NVS 应用层另行实现 |
| 10 | 验收 1（默认构建/测试通过） | ✅ | `go build ./...` + `go test ./...` 全绿 |
| 10 | 验收 2/3/4（gm 轨握手、CGO、回退隔离） | ⛔ 未验证 | 需 Linux + Tongsuo + 首次 `make tidy-gm` 下载国密依赖；Windows 开发机无法完成 |
| 10 | 验收 5（前端扫描 0 命中） | ✅ | `bash scripts/scan-frontend-secrets.sh server/dist` → 0 命中 |
| 10 | 验收 6（gm-audit Markdown 字段齐全） | ✅ | 见 `gm-audit` 输出结构 |
| 11 | 责任划分 | ✅ | 文档 + 管理端提示 |

---

# 三、下一步建议（按性价比排序）

| 优先级 | 事项 | 说明 |
|---|---|---|
| P0 | **gm 轨实机验收**（验收 2/3/4） | 需 Linux + Tongsuo；本机只能保证编译契约 |
| P1 | 单次调用上限 + 二次确认弹窗 | 经济系统风险点，改动小（前端弹窗 + 后端上限校验） |
| P1 | AI 内容发布标识（显式字段 + 展示） | 176 号令合规直接相关，改动中等 |
| P2 | 个人信息泄漏提示（前端统一组合式函数） | 整项缺失，纯前端可落地 |
| P2 | 提示词模板引擎（变量注入 + 版本 + 预览） | 在现有提示词库上加一层 |
| P3 | 蜜罐与取证子系统 | 独立立项，工作量最大 |
| P3 | 群组功能 | 独立立项，涉及权限/文件/实时消息 |
| P3 | 用户自备 Key 的「前端解密」路径 | 需要 Argon2id 派生 + 本地存储方案 |
