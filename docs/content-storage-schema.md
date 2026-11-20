# 内容存储与索引表结构

本文描述「正文存储双模式」「小说章节清单入库」「标签倒排索引」「AIGC 交付副本」这四块功能涉及的**表结构**：
每张表存什么、字段含义、索引怎么走、谁写谁读、以及迁移/运维时怎么自查。

> 配套阅读：`docs/ARCHITECTURE.md`（分层约定）、`docs/CONFIG.md`（配置项）。
> 迁移任务本身不落表，它的进度是进程内对象，见文末「迁移任务进度」。

---

## 1. `content_blobs` —— 正文副本（章节 / 博客）

正文的唯一「库内副本」。混合模式下它是文件的镜像，纯数据库模式下它就是正文本身。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `ref_type` | string(32) | **联合主键** | 内容类别，取值 `chapter` / `blog`（常量 `contentstore.RefChapter` / `RefBlog`） |
| `ref_id` | uint | **联合主键** | 对应 `chapters.id` 或 `author_blogs.id` |
| `content` | text | not null | 正文原文（HTML/Markdown 原样，不压缩、不改写） |
| `content_hash` | string(64) | — | `sha256(content)`，用于一致性比对与「是否变过」判断 |
| `updated_at` | datetime | — | 每次写入更新 |

**索引**：主键 `(ref_type, ref_id)` 即覆盖「按类别 + 内容 ID 精确取一条」的全部读路径，不额外建索引。

**写入方（唯一入口 `server/contentstore`）**

| 调用 | 行为 |
| --- | --- |
| `Put(refType, refID, path, content)` | `hybrid`：先写文件再写库；`database`：只写库。主键冲突时按 `Save` 语义做 UPDATE |
| `Get(refType, refID, path)` | `database`：只读库；`hybrid`：先读文件，文件缺失回退库。**两种模式下库内缺失都会回退读文件**，避免历史漏迁直接 500 |
| `LoadDB(...)` | 强制只读库，不受当前模式影响（迁移与校验用） |
| `Remove(...)` | 删库内副本（文件由业务层处理） |

**读方**：章节阅读（`server/handlers/novel/chapter_read.go`）、小说导出（`novel_export.go`）、
博客阅读（`server/handlers/blog/social_blog.go`）、迁移任务、健康自查。

⚠️ 注意：`content` 为空字符串是**合法状态**——源章节文件本身 0 字节时，副本如实为空。
空副本 ≠ 迁移失败，只有「库内无副本 + 磁盘无文件」才是真丢失。

---

## 2. `novel_indexes` —— 小说章节清单（index.json）的库内副本

文件系统里的 `index.json` 用于静态直出；本表是**同一份 JSON 字节**的镜像，供纯数据库模式与快速校验使用。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `novel_id` | uint | **主键** | 小说 ID，一本一条 |
| `data` | text | not null | `index.json` 的**原始字节**（不做二次 marshal，保证与文件逐字节一致） |
| `hash` | string(64) | index | `sha256(data)`，可据此发现文件与库不一致 |
| `updated_at` | datetime | — | 最近一次重建时间 |

**写入方**：`server/novelindex`（发布/修改章节、改书名、删除整本时调用）。
**读方**：`GET /api/novels/:id/index.json` —— 优先读库，老数据（无副本）现场重建后返回。

**一致性语义**：库与文件写的是同一段字节，因此「文件存在且 hash 相同」= 未漂移；
纯数据库模式下允许文件不存在，接口仍可用。

---

## 3. `feed_tags` —— 动态标签倒排索引

`posts.tags` 是逗号分隔字符串，用 `LIKE '%标签%'` 会误命中（「测试」命中「测试版」）且无法走索引。
本表把标签拆成「一行一个标签」的倒排结构，使 `/tag/:tag` 精确筛选可走索引。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | uint | 主键 | 自增 |
| `post_id` | uint | `idx_feed_tag_post_tag`（唯一）、`idx_feed_tag_post` | 动态 ID |
| `tag` | string(64) | `idx_feed_tag_post_tag`（唯一）、`idx_feed_tag_name` | 标签名（去 `#`、去空白） |
| `created_at` | datetime | — | 入索引时间 |

**唯一索引 `(post_id, tag)`** 保证同一条动态同一标签只有一行，重复同步是幂等的。

**写入方**：`feed.SyncPostTags(postID, rawTags)` —— 发布/编辑动态时**先删后插**，调用方无需事务包裹也可安全重试。
**回填**：`feed.EnsureTagIndex()` 首次访问时 `sync.Once` 全量回填老数据；后台可调 `RebuildTagIndex()` 重建。
**查询**：`ListOptions.Tag` 生成 `id IN (SELECT post_id FROM feed_tags WHERE tag = ?)`，走 `idx_feed_tag_name`。

标签解析规则见 `feed.ParseTags`：中英文逗号都算分隔符、去空去重、单标签最长 32 字符（超出截断）。

---

## 4. `aigc_deliveries` / `aigc_delivery_grants` —— AIGC 交付副本与取图授权

出图/出视频成功后，系统同步生成一份**已烧标识**的交付副本并以内容寻址落盘；
前端拿到的不是原图地址，而是带 HMAC 签名的副本 URL。

### 4.1 `aigc_deliveries`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | uint | 主键 | — |
| `object_key` | string(64) | **唯一** | 内容寻址键：`sha256(源内容 + 格式 + 合规配置)`，**提示词不进键**，因此同图跨用户只存一份 |
| `produce_id` | string(64) | index | 国标「内容编号」；同一内容复用同一编号 |
| `source_hash` | string(64) | index | 源文件 SHA-256，用于归并「同源不同参数」的导出 |
| `file_path` | string(512) | not null | 副本落盘路径（不对外暴露） |
| `mime` / `ext` | string | — | 副本类型与扩展名 |
| `width` / `height` | int | — | 像素尺寸（视频/音频为 0） |
| `size_bytes` | int64 | — | 副本字节数 |
| `labeled` | bool | — | 是否真的烧上了显式角标（无中文字体时降级为 false，仅写 metadata） |
| `public` | bool | index | 是否随作品公开（公开允许匿名取图） |
| `hits` | int64 | — | 去重命中次数，用于估算省下的存储/带宽 |
| `created_at` | datetime | index | — |

### 4.2 `aigc_delivery_grants`

谁有权取这份副本。唯一索引 `(object_key, user_id)`，同一用户同一副本只记一次。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | uint | 主键 | — |
| `object_key` | string(64) | `idx_aigc_grant`（唯一） | 指向交付副本 |
| `user_id` | uint | `idx_aigc_grant`（唯一）、index | 被授权的用户 |
| `task_id` | uint | default 0 | 产生该副本的 AI 任务 |
| `created_at` | datetime | — | — |

**取图端点只认签名**：`/api/ai/aigc/file/<64hex>?e=&u=&s=`。`<img>` 发不出设备指纹头，
所以这条路径不能用常规登录中间件；无签名或伪造签名一律 404。
**读历史与画廊时重新签发**（`resignResult()`），避免 URL 过期导致成片裂图。

---

## 5. 状态位：`platform_configs.storage_mode`

`content_blobs` 决定「能不能只靠库读」，`storage_mode` 决定「写的时候要不要落文件」。

| key | 取值 | 默认 | 说明 |
| --- | --- | --- | --- |
| `storage_mode` | `hybrid` / `database` | `hybrid` | 缺省与非法值一律按 `hybrid` 处理，保持老部署行为不变 |

读取入口：`contentstore.Mode()` / `IsDatabase()`。写入入口：后台「数据库管理 → 正文存储模式」
（`POST /api/admin/db/storage-mode`，异步任务 + 迁移期禁写）。

---

## 6. 迁移任务进度（不落表）

数据库迁移 / 备份 / 恢复 / 正文存储迁移四类任务的进度是**进程内对象**（`dbJob`），重启即丢，
不做持久化——它们的生命周期本就不跨重启。

对外只有一个读取入口：`GET /api/admin/db/jobs?limit=N`，字段：

| 字段 | 说明 |
| --- | --- |
| `type` | `migrate` / `backup` / `restore` / `storage-mode` |
| `status` | `running` / `done` / `error` |
| `stage` | 当前阶段：准备中 / 连接目标库 / 建立表结构 / 复制数据 / 迁移正文 / 重建小说索引 / 切换存储模式 / 完成 |
| `percent` | 0–100，按**真实任务量**推进（表格数或正文条数，不是作者数） |
| `status_text` | 人读的内容状态，例：`作者 1/3：已处理 840/2071 条正文 → database` |
| `detail` | 结构化明细：`total_items` / `ok_items` / `empty_items` / `skipped_items` / `missing_before` / `missing_after` / `content_bytes` |

---

## 7. 运维自查（判断迁移是否成功）

**不要看数据库文件体积。** SQLite 复写页面不按内容线性增长，重复执行迁移是幂等的
（内容已在库内时不会再次写入），所以「体积没变」既可能是一切正常，也可能是压根没跑。

后台「数据库管理 → 正文存储模式 → 入库健康自查」直接给条数与字节；对应接口
`GET /api/admin/db/storage-health`，返回：

| 字段 | 含义 |
| --- | --- |
| `chapter.total` / `chapter.blob` | 未删除章节数 / 已有库内副本数 |
| `chapter.empty` | 副本存在但正文为空（源文件本身 0 字节，正常） |
| `chapter.missing` | 无库内副本的章节数（**纯数据库模式下这些章节读不到正文**） |
| `blog.*` | 同上，针对作者博客 |
| `content_bytes` | 库内正文真实字节（按 BLOB 长度统计，中文不折半） |
| `db_file_bytes` | 数据库文件占用（含索引与访问日志，仅供容量参考） |
| `missing_sample` | 缺失样本：章节 ID、作品、章号、标题、`file_exists` |

等价的 SQL（SQLite）：

```sql
-- 库内副本条数与真实字节
SELECT ref_type,
       COUNT(*)                             AS copied,
       SUM(LENGTH(CAST(content AS BLOB)))   AS bytes
FROM content_blobs
GROUP BY ref_type;

-- 未删除但缺副本的章节（应为 0；软删除章节不参与迁移，也不算缺失）
SELECT COUNT(*) FROM chapters
WHERE deleted_at IS NULL
  AND id NOT IN (SELECT ref_id FROM content_blobs WHERE ref_type = 'chapter');

-- 小说清单副本
SELECT COUNT(*) FROM novel_indexes;

-- 标签索引与实际标签数对比
SELECT COUNT(DISTINCT tag) FROM feed_tags;
```

**已知的正常现象**

- `chapter.empty > 0`：测试站存在大量 0 字节占位章节，副本如实为空。
- `chapter.missing` 含「软删除章节」不算缺失（默认作用域自动排除），只有
  `deleted_at IS NULL` 且磁盘也无文件的才是真孤儿，需要在 `missing_sample` 里逐个处理。
- `db_file_bytes` 远大于 `content_bytes` 属正常：里面还有访问日志等其它大表。
