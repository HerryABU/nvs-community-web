# 章节版本控制（chaptervcs）· 设计与实现

> 目标：多人协作写同一章时不再互相覆盖。给每章一条 **git 语义的版本链**：
> 提交、分支、三方合并、冲突检测、历史 diff、回滚、作者可删历史。

---

## 1. 为什么不是真 git

章节正文现存在 `主笔目录/{作品ID}/{章节号}.html`，哈希与作者签名都基于这个文件。
引入 go-git 会变成「文件 + 仓库」双写，还要背上较重的二进制依赖与 Windows 运维成本。
因此这里**自建一条 git 语义的版本链**，语义对齐（commit / parent / branch / head / merge / revert），
存储沿用既有目录结构：

```
<NovelDataDir>/authors/{主笔ID}/{作品ID}/.versions/{章节号}/{分支}/{版本号}.md
```

DB 只存元数据 + 哈希，正文不进库（历史膨胀不拖垮 SQLite）。

## 2. 数据模型

| 表 | 作用 |
|---|---|
| `chapter_versions` | 一次提交：章节 + 分支 + 版本号 + 父版本 + 作者 + 说明 + 快照路径 + 哈希 + 字数 |
| `chapter_branches` | 分支指针：head 版本号 + 分叉点（`from_branch` / `from_ver`，三方合并的 base） |

关键字段：

- `parent_ver`：主父版本（0 = 首次提交）
- `merge_from` / `merge_from_ver`：合并提交的第二父，用于追溯「这一版是合了哪个分支来的」
- `force`：标记「冲突后以我的为准」
- `deleted`：作者删除了历史快照；**只清正文文件，元数据保留**（谁在什么时候写了什么仍可查）

## 3. 冲突检测（保存时）

```
客户端保存携带 base_version（打开编辑器时拿到的版本号）
  ├─ head == base_version        → 正常提交
  ├─ head != base_version        → 409 冲突：回传 { base_content, server_content, mine_content }
  │                                前端并排对比，作者选「以我的为准」（force=true）或「放弃并重载」
  └─ base_version 未传（0）       → 不校验（兼容旧客户端 / 脚本导入）
```

- `force=true` **仍然产生一个新版本**，不是直接覆盖：冲突现场留痕，可回溯。
- 内容哈希与标题都没变 → 不产生新版本（与 git 一致，避免刷历史）。

## 4. 分支与三方合并

- 开分支 = 以某版本内容为新分支 v1，并记录分叉点 `from_ver`
- 合并 = 以分叉点为 base 做行级三方合并（base / ours=目标分支 / theirs=来源分支）
  - 只有一侧改 → 直接采用
  - 两侧改成一样 → 采用
  - 两侧同处不同改法 → **报冲突，不写库**，回传 `{ base, ours, theirs }` 三段给前端确认
- 差异过大（单侧剩余 > 1200 行）→ 降级为整块替换，避免 DP 内存爆炸

## 5. 回滚与删除

| 操作 | 语义 |
|---|---|
| 回滚（revert） | 以历史版本内容**再提交一次**，历史不被抹掉 |
| 删除历史 | 仅删正文快照，保留元数据；**不能删分支 head**（避免「有章节无内容」空洞） |
| 自动裁剪 | 单章快照超过 200 个时清掉最老的正文（元数据保留） |

## 6. 接口

| 方法 | 路径 |
|---|---|
| GET | `/api/novels/:id/chapters/:num/versions` |
| GET | `/api/novels/:id/chapters/:num/versions/:ver` |
| GET | `/api/novels/:id/chapters/:num/versions/:ver/diff?to=` |
| POST | `/api/novels/:id/chapters/:num/versions/:ver/revert` |
| DELETE | `/api/novels/:id/chapters/:num/versions/:ver` |
| GET | `/api/novels/:id/chapters/:num/branches` |
| POST | `/api/novels/:id/chapters/:num/branches` |
| POST | `/api/novels/:id/chapters/:num/merge` |

保存接口 `PUT /api/novels/:id/chapters/:num` 新增字段：`base_version` / `branch` / `force` / `message`。

## 7. 权限（复用 collab）

- 查看/回滚/删除历史、开分支、合并：需 `collab.Actor.CanEditChapter`（主笔 / 本章作者 / 有 `edit_others`）
- 删除历史：主笔或本章作者（避免协作者互删对方的稿）
- 所有写操作走 `collab` 审计（`novel_collab_audits`）

## 8. 可插拔后端：自研版本链 / 真实 git 仓库

`chaptervcs.Backend` 抽象存储；业务侧一律调包级函数，不感知差异。

| 后端 | 标识 | 启用方式 | 说明 |
|---|---|---|---|
| 自研版本链（默认） | `chain` | 默认；或 `NVS_VCS_BACKEND=chain` | 快照文件 + DB 元数据，零外部依赖 |
| 真实 git 仓库 | `git` | `go build -tags gitvcs` + `NVS_VCS_BACKEND=git` | go-git；仓库 `作品目录/.vcsrepo`，章节文件 `chapters/{章节号}.md`，分支 `ch{章节号}/{分支名}` |

- 启动时 `chaptervcs.Init()` 读环境变量切换；git 后端不可用（未带标签 / 初始化失败）自动回落 `chain`，不阻断启动。
- **差异**：git 后端历史**不可删除**（提交即留痕），`DeleteVersion` 返回明确错误并引导使用「回滚」；默认后端允许作者清理历史快照正文（元数据保留）。
- 冲突检测、三方合并、回滚在两个后端上语义一致（合并冲突仍不写库、回传三段内容）。
- 构建：`make build-gitvcs`（产物 `nvs-server-gitvcs.exe`）；测试：`make test-gitvcs`。

---

**版本**：v1（2026-10-24）　**状态**：已实现（领域层 + 接口层 + 前端 + 双后端，含单测）
