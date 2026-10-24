# 群组功能 · 设计与实现状态（v3.9.0 已实现 G1~G3）

> **实现落点**：领域层 `server/groups/`（group / member / post / file / quota / migrate / 测试）、
> 接口层 `server/handlers/group/`（group / member / post / file / ai / admin）、
> 前端 `api/group` + `components/group` + `views/group` + `views/admin/group`。
> 已完成：群号、可见性、三种加入方式、角色与拉人/踢人/禁言/公告、配额（按角色 + 按人）、
> 群内帖子先审后发、群文件隔离与 ZIP 炸弹检测、群内 `@ai`。
> 未完成：G4 实时消息（当前为异步发帖，WebSocket 需单独评估）。

> 目标：在公开社区（论坛/博客）之外补「半私密/私密」协作空间，复用既有审核、额度、举报、未成年保护体系。

## 1. 领域模型

```
groups(id, owner_id, name, slug, visibility[public|semi|private], category,
       join_mode[open|apply|invite], member_limit, post_limit_per_day,
       status[active|archived|banned], created_at)
group_members(group_id, user_id, role[owner|admin|member], joined_at, muted_until)
group_posts(id, group_id, user_id, title, content_path, content_hash,
            status[pending|published|removed], created_at)
group_files(id, group_id, uploader_id, name, size, sha256, mime, status, created_at)
group_reports(id, group_id, post_id, reporter_id, reason, status, created_at)
```

## 2. 权限与可见性

| 可见性 | 谁能看 | 谁能读内容 | 谁能发 |
|---|---|---|---|
| public | 所有人 | 所有人 | 成员 |
| semi | 所有人可见存在与简介，内容仅成员 | 成员 | 成员（可配置先审后发） |
| private | 仅成员 | 成员 | 成员（默认先审后发） |

- 群内角色：`owner`（解散/任命/踢人）、`admin`（审核/删帖/禁言）、`member`。
- **私密不豁免平台审核**：私密群内容仍过违禁词 + 抽查 + 举报；管理员进群取证需**群主授权或站长强制**，全程审计。

## 3. 额度与未成年保护（复用用户配额体系）

- 建群上限（默认普通 3 / VIP 10 / 管理员不限）、每群人数上限、每群每日发帖上限。
- 所有限额在 `/admin/quota`（等级 × 上限 + 按人覆盖），与发文额度同源。
- 未成年（minor_mode）：禁止建群、禁止加入 semi/private 群；群内额外内容限制沿用既有策略。

## 4. 文件安全

- 存储隔离：`data/groups/<group_id>/`，下载走鉴权路由（禁止直链静态目录）。
- 上传校验：MIME 嗅探 + 扩展名白名单 + 单文件/总量上限 + **ZIP 炸弹检测**（解压比 > 100 或解压后 > 500MB 拒绝）。
- 群解散/成员退出：文件软删除 → 7 天后清理（审计留痕）。

## 5. 接口草案

```
GET    /api/groups?scope=my|discover        # 列表（可见性过滤）
POST   /api/groups                          # 建群（额度+未成年校验）
GET    /api/groups/:id                      # 详情（非成员只看简介）
PATCH  /api/groups/:id                      # 群主/管理员改设置
POST   /api/groups/:id/join                 # 加入（open 直接进 / apply 待审 / invite 校验码）
DELETE /api/groups/:id/members/:uid         # 踢人/退出
GET    /api/groups/:id/posts                # 帖子列表（成员）
POST   /api/groups/:id/posts                # 发帖（过审核管线）
POST   /api/groups/:id/files                # 上传（ZIP 炸弹检测）
GET    /api/groups/:id/files/:fid/download  # 鉴权下载
```

管理端：`/admin/groups`（列表/封禁/强制审核）、群内举报进 `/admin/reports`。

## 6. 分期

| 期 | 内容 |
|---|---|
| G1 | 建群/加入/成员角色/帖子（公开 + 半公开），复用审核与额度 |
| G2 | 私密群 + 申请审核 + 群内举报 + 管理员进群取证（授权+审计） |
| G3 | 文件上传与鉴权下载（ZIP 炸弹检测、隔离存储、解散清理） |
| G4 | 实时能力评估：先用轮询（30s）验证需求，再决定是否上 SSE/WebSocket |

## 7. 与 `@ai` 机器人 / 蜜罐的关系

- `@ai`：群内先发后审 + 举报兜底；机器人权限 ≤ 普通成员；按「群 × 人」双维度限流；计费仍绑发起人。
- 蜜罐：群组**不做**诱捕面（避免与真实私密数据混淆），只在群文件下载路径复用统一风控日志。
