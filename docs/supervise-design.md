# 绥靖模式与以疏代堵名单子系统 · 立项设计（v4.8.0）

> 目标：把 AI 安全闸从「一律硬拦」升级为「**以疏代堵**」——对高危但非 imminent 危险的用户，
> 在**用户完全无感**的前提下完成疏导、入档、升级、取证、对接警方。
> 设计哲学：能疏不堵、能导不打；既不误伤正常创作，也不放过真正恶意的账号。

## 1. 为什么需要绥靖模式

真实站点上此前只有「严格」模式真正生效：命中即整段拒绝、整对话终止。这带来两类问题：

1. **误伤严重**：写小说查资料、科普讨论、讲危害/举报，都会被一刀切拦掉，创作者体验崩坏。
2. **打草惊蛇**：真正想干坏事的人被一拦就换号、换说法、换工具，平台彻底失去视线。

「绥靖模式」反过来：用户看起来一切正常（收到的是引导内容而非拒绝），但他的危险行为已被
**静默记入名单**。平台继续观察，等到证据充分（监管模型 B 判定分越线）再升级处置。
这与「中档模式」（用户可见、注入提示）和「严格模式」（用户可见、终止）形成三档梯度。

| 模式 | 用户感知 | 系统动作 | 适用 |
|---|---|---|---|
| strict 严格 | 可见：终止提示 | 整对话锁死 | 真实泄露已发生 |
| moderate 中档 | 可见：注入提示 | 对话继续但上下文被标记不可信 | 高误判率场景的折中 |
| loose 宽松 | 基本无感 | 只拦当轮，不注入、不终止 | 误判率更高的场景 |
| **appease 绥靖** | **完全无感** | 危险内容改写为引导内容 + 静默入档 | **以疏代堵：既不误伤也不放跑** |

## 2. 用户无感是怎么做到的（关键）

SSE 流式契约里，拦截分支会下发 `blocked / terminated / replace` 字段，前端 `aiHub.ts` 收到
`ev.replace` 会**强制**把本轮标记为拦截并弹警告 toast。只要走这条路，用户就不可能「无感」。

因此绥靖模式**不复用**拦截分支，而是走一条独立响应路径：

- **输入侧命中**：`ai_hub.go` 在拦截判定前先识别 `mode == appease`，调用
  `serveAppeaseResponse()`——把用户提问与「引导回答」都按**普通助手消息**落库
  （`AppendAiChatMessageWithParent`，cost=0），只下发 `content` / `done`，**没有** blocked/terminated/replace。
- **输出侧命中**：内联分支直接 `sseEvent({content})` + `sseEvent({done})`，同样不带拦截标记。

用户看到的就是一段「看似正常」的引导文字（科普 / 法律后果 / 求助渠道），完全不知道已被记录。

### 主模型提示词放松（减少误伤）

绥靖模式下，主模型的系统提示词改用 `RelaxedSystemPrompt(cfg)`：去掉常规模式里激进的
「凡事预设可疑」框架，只在确有危险语义时按内置默认引导，**显著降低误伤**。管理员也可在后台
`appease_system_prompt` 字段自定义宽松准则（留空回退内置默认）。

## 3. 名单子系统（两级）

```
supervised_users(
  id, user_id, username, tier,        -- tier: suspect(可疑) | malicious(恶意)
  score, source, reasons(JSON),       -- 入档原因数组
  evidence_refs(JSON),                -- 证据引用（不存正文，正文按 TTL 过期）
  status, handled_by, note,           -- status: active | cleared
  first_seen, last_event, created_at, updated_at
)
supervise_logs(
  id, supervised_id, user_id, actor_id, action,  -- upsert|escalate|clear|set_tier|note
  from_to, note, created_at
)
```

- **只升不降**：系统自动 `suspect → malicious`（监管模型 B 判定分 ≥ `appease_escalate_score`）。
  人工降级必须走 `set_tier` 并强制留 `note`（审计合规）。
- **证据引用而非正文**：名单与取证只存引用，证据正文按 TTL 过期，避免无限留存 PII。
- **清除 ≠ 删除**：`clear` 把 `status` 置 `cleared`，历史原因与全部日志**保留**，符合 176 号令对
  处置留痕的要求。

### 升级触发

`ai_terminate.go` 的 `escalateAppearanceIfNeeded()`：当监管模型 B 判定 `v.Score >= cfg.AppeaseEscalateScore`
时，先 `Upsert(... SrcJudge ...)`（记录判定来源），再 `Escalate()` 升级为恶意名单。

## 4. 取证包导出（证据 by reference）

`supervise.BuildSuperviseForensicsZip(targetUserID, operatorID)` 产出含以下文件的 zip：

```
supervised.json     # 该用户的名单记录
sessions.json       # AI 会话列表
messages.json       # 会话消息（含被绥靖替换的内容，证明「以疏代堵」已发生）
judge_logs.json     # 监管模型 B 判定记录
safety_events.json  # 安全事件
manifest.json       # 上述每个文件的 SHA256，便于校验完整性
```

zip 写临时目录（`os.MkdirTemp`），handler 读回后 `defer os.Remove` 清理，不下落持久副本。

## 5. 警方对接（申请 → 审批 → 导出）

设计要点：**站长掌握审批权**，警方不能直接拖走数据，必须先申请、获批后才能导出指定用户。

```
police_evidence_requests(
  id, no(ER-YYYYMMDD-XXXXXX), requester_id, target_user_id, target_name,
  reason, status(pending|approved|rejected),
  reviewer_id, review_note, reviewed_at, created_at, updated_at
)
police_evidence_request_logs(...)   -- 审批动作日志
```

流程：

1. **警方提交申请** `POST /api/police/evidence-requests`（target_user_id + reason）。
2. **站长审批** `POST /api/admin/police/evidence-requests/:id/review`（approve / reject；驳回须填理由）。
3. **警方导出** `GET /api/police/export-user?user_id=X`——handler 校验
   `police.HasApprovedRequest(policeUID, target)`，无批准直接 `403`。
   **站长（`role=admin`）始终可直接导出**，不经申请。
4. 站长自身的「导出全部取证包」（`/api/police/export`）同样受「须先获批」约束（已有逻辑）。

权限口径（前后端一致）：`/police/*` 整段 `requiresPolice`；`/admin/police-evidence` 仅 `requiresAdmin`；
`canAccessPath` 与后端 `AdminOrPoliceGovernance` / `ensurePoliceOrAdmin` 同口径，越权请求 403 并记审计。

## 6. 后端落点（Go）

```
server/aisafety/config.go        # appease 配置项（展示模式/替代文案/软卡文案/升级线/宽松提示词）
server/handlers/ai/ai_terminate.go  # applyTermination + escalateAppearanceIfNeeded + appeaseContent
server/handlers/ai/ai_hub.go        # 输入/输出拦截前的 appease 分支（独立无感响应路径）
server/handlers/ai/ai_safety.go     # serveAppeaseResponse（普通助手消息落库，无 blocked 标记）
server/supervise/supervise.go       # Upsert/Escalate/List/Get/SetTier/Clear/AddNote/Stats/Migrate
server/supervise/forensics.go       # BuildSuperviseForensicsZip（证据 by reference + SHA256 manifest）
server/police/evidence_request.go   # 取证申请模型 + Create/List/Review/HasApproved*
server/handlers/police_evidence.go  # 警方侧：创建/我的申请/导出（含审批校验）
server/handlers/admin_police_evidence.go  # 站长侧：列表/审批
server/handlers/ai/ai_supervise.go  # 站长侧名单管理 API（仅 admin）
server/routes/{admin,police}.go     # 路由注册
```

## 7. 前端落点（Vue 3 + Element Plus）

```
web/src/api/aiSafety.ts                       # supervise* / evidenceRequest* 接口 + 类型
web/src/views/admin/ai/AdminAiSafety.vue      # 终止模式选择 + 绥靖展示模式 + 绥靖文案配置
web/src/views/admin/ai/AdminSupervise.vue     # 站长：名单列表/详情/改档/清除/加注/统计
web/src/views/admin/police/AdminPoliceEvidence.vue     # 警方：发起申请/我的申请/导出取证包
web/src/views/admin/police/AdminPoliceEvidenceReview.vue  # 站长：申请审批（批准/驳回）
web/src/router/index.ts + adminMenuData.ts    # 路由与菜单（内容治理「绥靖名单」+ 警方取证「取证申请审批」）
```

## 8. 配置项一览（后台 AI 防越狱 → 终止处置 → 绥靖模式）

| 字段 | 含义 | 默认 |
|---|---|---|
| `terminate_mode` | `appease` 启用绥靖 | — |
| `appease_display_mode` | replace / softcard / both | replace |
| `appease_replace_text` | 替代回答文案（科普/法律后果/求助引导） | 内置默认 |
| `appease_soft_card_text` | 软警示卡片文案 | 内置默认 |
| `appease_escalate_score` | 监管模型 B 判定分 ≥ 此值则升恶意名单 | 同判定拦截线量级 |
| `appease_system_prompt` | 绥靖模式主模型宽松准则（空回退内置默认） | 空 |

## 9. 合规与边界

- 名单/取证全部参数化查询，证据正文按 TTL 过期，不无限留存 PII。
- 所有人工干预（改档/清除/加注/审批）强制写日志，含操作人，`set_tier`/`clear` 强制填理由。
- 警方取证必须经站长审批，越权导出 403 并记审计；zip 临时文件用后删除。
- 「以疏代堵」不是「放任」：升级为恶意名单后，站长可结合既有封禁/留证能力处置。
