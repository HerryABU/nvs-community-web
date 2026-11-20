# 数据表总览

> 本文件按业务域盘点全部数据表；逐表字段明细见 [`DATABASE-SCHEMA.md`](DATABASE-SCHEMA.md)。
> 双模式存储/索引/交付副本的行为细节见 [`content-storage-schema.md`](content-storage-schema.md)。
> 表结构以源码里的 GORM 模型为准，`AutoMigrate` 在启动时创建表并补列。

```bash
# 结构有变动后重新生成这两份文档（参数为任意一个已迁移的 nvs.db）
python scripts/gen-db-doc.py data/nvs.db
```

当前共 **182 张表 / 2060 个字段**（行数为本机测试站实测，仅用于判断「有没有数据」，不代表生产规模）。

## 全局约定

| 约定 | 说明 |
| --- | --- |
| 时间列 | 统一 `created_at` / `updated_at`（部分表另有业务时间列），UTC+8 落库 |
| 软删除 | `deleted_at IS NULL` 为有效行；GORM 默认作用域自动过滤，统计时注意 `Unscoped()` |
| 密文列 | `*_enc` 为对称加密后的密文（模型密钥、渠道令牌、用户自带 Key） |
| 哈希列 | `*_hash` / `*_signature` 为单向哈希，用于校验或去重，不可逆 |
| 内容寻址 | `aigc_deliveries.object_key`、`content_blobs` 的 Hash 均按内容算，同内容只存一份 |
| 删除留痕 | 业务删除走 `softdel_*` 快照表，可原样恢复 |
| 配置 | 运行期开关在 `platform_configs`（改库即时生效，不读 .env） |

## 分域索引

| 域 | 表数 | 职责 |
| --- | --- | --- |
| [账号与用户](#domain-1) | 13 | 注册、登录、审计、关注、私信、通知、黑名单 |
| [小说与创作](#domain-2) | 16 | 作品、章节、版本、协作、角色卡、阅读进度、评分、购买 |
| [作者博客](#domain-3) | 11 | 博客文章、分类、标签、话题、点赞收藏、作者主页主题 |
| [内容流（发现）](#domain-4) | 6 | 图文/视频/音乐/微文动态、评论、点赞收藏、标签倒排 |
| [论坛](#domain-5) | 5 | 版块、主题、楼层、禁言与建版配额 |
| [群组](#domain-6) | 8 | 群组、成员、群帖、群聊、群文件、已读位置、建群配额 |
| [OC 角色扮演](#domain-7) | 16 | OC 角色卡、世界书、创作项目、AI 群组与剧情流 |
| [AI 中台](#domain-8) | 23 | 厂商、模型、密钥池、提示词、调用计费、会话、媒体验成、安全闸 |
| [AI 写作 Agent](#domain-9) | 3 | 写作工程（设定/人物/时间线/大纲）与生成章节草稿 |
| [AIGC 合规](#domain-10) | 5 | 原图资产、内容寻址交付副本、取图授权、导出与豁免留痕 |
| [钱包与支付](#domain-11) | 9 | 代币流水余额、充值提现、卡密、VIP、支付风控、作者收益 |
| [存储与配额](#domain-12) | 11 | 正文库内副本、HTML 作品空间、发布/AI 额度 |
| [安全与风控](#domain-13) | 24 | 敏感词、XSS、蜜罐、机审、舆情、API Key 限流与告警 |
| [监管与处置](#domain-14) | 17 | 警方取证、隔离墙、心理危机、软删除快照、紧急操作审计、漏洞登记 |
| [站点运营](#domain-15) | 9 | 平台配置、公告、轮播、导航、首页区块、主题、搜索日志 |
| [访问统计](#domain-16) | 2 | 全站访问日志与分区进入日志 |
| [联邦](#domain-17) | 4 | 联邦站点、作品缓存、对端密钥、入站请求 |

<a id="domain-1"></a>

## 账号与用户

注册、登录、审计、关注、私信、通知、黑名单。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `users` | 账号主体：邮箱/密码哈希、昵称头像、角色、会员等级、状态 | role / member_level / status | 22 | `server/models/user.go` |
| `user_audit_logs` | 用户操作审计：登录、改密、提权等，含 IP/UA 与结果 | id / user_id / action / ip | 514 | `server/models/apikey/user_audit.go` |
| `refresh_states` | 当前有效的 refresh token 版本，用于强制下线 | user_id / active_jti / updated_at | 17 | `—（无模型定义）` |
| `checkin_records` | 每日签到与连签奖励 | id / user_id / date / streak | 2 | `server/models/siteops/site_ops.go` |
| `follows` | 关注关系，is_special 标记特别关注 | id / follower_id / followed_id / created_at | 0 | `server/models/social/follow.go` |
| `messages` | 私信：撤回标记 + 收发双向删除标记 | id / sender_id / receiver_id / content | 0 | `server/models/social/message.go` |
| `notifications` | 站内通知（评论/点赞/关注/系统） | id / user_id / actor_id / actor_name | 61 | `server/models/notification.go` |
| `blacklists` | 用户互拉黑名单 | id / user_id / blocked_user_id / mode | 0 | `server/models/security/blacklist.go` |
| `hidden_contents` | 用户主动隐藏的内容 | id / user_id / target_type / target_id | 0 | `server/moderation/models.go` |
| `reports` | 内容举报工单与处置状态 | id / reporter_id / target_type / target_id | 0 | `server/models/platform/platform.go` |
| `malicious_reporters` | 恶意举报统计与举报封禁 | id / user_id / total_reports / malicious_count | 0 | `server/moderation/models.go` |
| `gate_device_bindings` | 注册闸门：设备指纹与账号的一次性绑定 | id / device_id / user_id / ip | 1 | `server/gate/gate.go` |
| `gate_invites` | 注册邀请码（次数、有效期） | id / code / created_by / max_uses | 0 | `server/gate/gate.go` |

<a id="domain-2"></a>

## 小说与创作

作品、章节、版本、协作、角色卡、阅读进度、评分、购买。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `novels` | 作品主体：作者、分类、标签、封面、价格、阅读权限 | author_id / read_permission / paid_price | 16 | `server/models/novel.go` |
| `novel_categories` | 作品与分类的多对多关联 | id / novel_id / category | 19 | `server/models/novel.go` |
| `chapters` | 章节元数据（正文在文件或 content_blobs） | novel_id / chapter_number / content_path / deleted_at | 2042 | `server/models/novel.go` |
| `chapter_versions` | 章节版本快照，支持回溯与合并 | id / chapter_id / novel_id / chapter_num | 2 | `server/chaptervcs/model.go` |
| `chapter_branches` | 章节分支（多结局/草稿线）及分支头版本 | id / chapter_id / name / head_ver | 2 | `server/chaptervcs/model.go` |
| `novel_collaborators` | 协作者与权限位（发布/审核/改他人稿） | id / novel_id / user_id / role | 2 | `server/collab/collab.go` |
| `novel_collab_audits` | 协作操作审计（前后值快照） | id / novel_id / actor_id / action | 7 | `server/collab/collab.go` |
| `novel_characters` | 作品角色卡（阵营、简介、配色） | id / novel_id / name / avatar | 2 | `server/models/novel_character.go` |
| `novel_documents` | 设定集/世界观文档，可控制可见性 | id / novel_id / kind / title | 0 | `server/models/novel_document.go` |
| `novel_likes` | 作品点赞 | id / novel_id / user_id / created_at | 0 | `server/models/novel.go` |
| `novel_indexes` | index.json 的库内副本（与文件同字节 + Hash） | novel_id 主键 / data / hash | 16 | `server/models/novel_index.go` |
| `bookshelf` | 书架与阅读进度（最后阅读章节） | id / user_id / novel_id / last_read_chapter | 3 | `server/models/social/bookshelf.go` |
| `reading_notes` | 划线笔记与批注（含锚点定位） | id / user_id / novel_id / chapter_number | 15 | `server/models/social/reading_note.go` |
| `ratings` | 多维度评分：完成度/叙事/思想/社区口碑/更新稳定 | id / user_id / novel_id / type_completion | 0 | `server/models/social/rating.go` |
| `content_purchases` | 付费章节购买记录（幂等发放阅读权限） | id / user_id / content_type / content_id | 0 | `server/readperm/purchase.go` |
| `comments` | 小说/博客评论，支持引文与偏移定位 | id / user_id / novel_id / blog_id | 6 | `server/models/comment.go` |

<a id="domain-3"></a>

## 作者博客

博客文章、分类、标签、话题、点赞收藏、作者主页主题。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `author_blogs` | 博客文章元数据（正文在文件或 content_blobs） | id / author_id / title / content_path | 41 | `server/models/blog/blog.go` |
| `blog_categories` | 博客分类，含审核状态与驳回原因 | id / parent_id / name / sort_order | 1 | `server/models/blog/blog_category.go` |
| `blog_tags` | 博客标签字典 | id / name / created_by / created_at | 1 | `server/models/blog/blog_tag.go` |
| `blog_tag_links` | 博客与标签的关联 | blog_id / tag_id | 1 | `server/models/blog/blog_tag.go` |
| `blog_topics` | 博客话题（专题聚合） | id / name / description / created_by | 0 | `server/models/blog/blog_topic.go` |
| `blog_topic_links` | 博客与话题的关联 | blog_id / topic_id | 0 | `server/models/blog/blog_topic.go` |
| `blog_likes` | 博客点赞 | id / blog_id / user_id / created_at | 2 | `server/models/blog/blog_category.go` |
| `blog_favorites` | 博客收藏 | id / blog_id / user_id / created_at | 0 | `server/models/blog/blog_category.go` |
| `author_themes` | 作者主页主题：CSS 与博客样式 | author_id / css / updated_at / blog_style | 0 | `server/models/theme/author_theme.go` |
| `author_home_customize` | 作者主页自定义 HTML（审核后公开） | author_id / raw_html / safe_html / public_html | 0 | `server/models/theme/home_customize.go` |
| `user_home_customize` | 用户个人主页自定义 HTML | user_id / raw_html / updated_at | 0 | `server/models/theme/home_customize.go` |

<a id="domain-4"></a>

## 内容流（发现）

图文/视频/音乐/微文动态、评论、点赞收藏、标签倒排。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `feed_posts` | 动态主体：图文/视频/音乐/微文，含封面与媒体地址 | id / user_id / kind / title | 30 | `server/models/feed/model.go` |
| `feed_comments` | 动态评论，parent_id 支持楼中楼 | id / post_id / user_id / parent_id | 1 | `server/models/feed/model.go` |
| `feed_comment_likes` | 评论点赞 | id / comment_id / user_id / created_at | 0 | `server/models/feed/model.go` |
| `feed_likes` | 动态点赞 | id / post_id / user_id / created_at | 0 | `server/models/feed/model.go` |
| `feed_favorites` | 动态收藏 | id / post_id / user_id / created_at | 0 | `server/models/feed/model.go` |
| `feed_tags` | 标签倒排索引（唯一索引 post_id+tag） | (post_id,tag) 唯一 / tag 索引 | 38 | `server/models/feed/tag.go` |

<a id="domain-5"></a>

## 论坛

版块、主题、楼层、禁言与建版配额。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `forums` | 版块：类型、分区、关联对象、层级 | id / name / description / type | 14 | `server/models/forum.go` |
| `threads` | 主题帖：置顶/锁定/浏览量 | id / forum_id / user_id / title | 6 | `server/models/forum.go` |
| `posts` | 楼层回复，parent_id/root_id 组织楼中楼 | id / thread_id / user_id / content | 26 | `server/models/forum.go` |
| `forum_mutes` | 版块禁言（到期时间、管理员封禁标记） | id / forum_id / user_id / muted_until | 0 | `server/models/forum.go` |
| `user_forum_quotas` | 用户建版配额 | id / user_id / max_forums / note | 0 | `server/models/forum.go` |

<a id="domain-6"></a>

## 群组

群组、成员、群帖、群聊、群文件、已读位置、建群配额。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `groups` | 群组主体：群号、分类、可见性、加入方式 | id / owner_id / group_no / name | 1 | `server/groups/group.go` |
| `group_members` | 群成员与角色、禁言到期 | id / group_id / user_id / role | 1 | `server/groups/group_member.go` |
| `group_posts` | 群内帖子（可 AI 辅助） | id / group_id / user_id / title | 0 | `server/groups/group_post.go` |
| `group_post_replies` | 群帖回复 | id / group_id / post_id / user_id | 0 | `server/groups/group_post.go` |
| `group_chat_messages` | 群聊消息（支持引用回复） | id / group_id / user_id / content | 3 | `server/groups/group_chat.go` |
| `group_files` | 群文件（SHA-256 去重 + 存储路径） | id / group_id / uploader_id / name | 0 | `server/groups/group_file.go` |
| `group_read_states` | 群已读位置 | id / group_id / user_id / last_read_at | 1 | `server/groups/read.go` |
| `user_group_quotas` | 用户建群配额 | id / user_id / max_groups / note | 0 | `server/groups/group_quota.go` |

<a id="domain-7"></a>

## OC 角色扮演

OC 角色卡、世界书、创作项目、AI 群组与剧情流。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `oc_characters` | OC 角色卡：人设、外观、口吻、场景 | id / user_id / name / avatar_url | 1 | `server/oc/model.go` |
| `oc_emotes` | OC 表情包 | id / user_id / key / url | 0 | `server/oc/emote.go` |
| `oc_world_books` | 世界书（扫描深度、Token 预算） | id / user_id / group_id / title | 15 | `server/oc/model.go` |
| `oc_world_entries` | 世界书条目（关键词、二次关键词、逻辑） | id / book_id / kind / key | 0 | `server/oc/model.go` |
| `oc_world_links` | 世界书之间的引用链 | id / from_book_id / to_book_id / label | 0 | `server/oc/project.go` |
| `oc_projects` | OC 创作项目（可见性、访问密码） | id / user_id / title / summary | 0 | `server/oc/project.go` |
| `oc_project_members` | 项目成员与角色 | id / project_id / user_id / role | 0 | `server/oc/project.go` |
| `oc_project_access` | 项目访问令牌（带过期时间） | id / project_id / token / expires_at | 0 | `server/oc/project.go` |
| `oc_groups` | OC 群组（AI 角色群聊） | id / user_id / name / summary | 1 | `server/oc/ocgroup.go` |
| `oc_group_settings` | OC 群审计等级、窗口大小与时机 | id / group_id / enabled / mode | 1 | `server/oc/model.go` |
| `oc_group_members` | 群内绑定的 OC 角色 | id / group_id / character_id / sort_order | 0 | `server/oc/ocgroup.go` |
| `oc_agents` | 群内 AI 角色绑定的模型/模板/策略/预算 | id / group_id / character_id / model_id | 0 | `server/oc/model.go` |
| `oc_flows` | 剧情流：一次连续演出（排期、轮次） | id / group_id / starter_id / title | 0 | `server/oc/model.go` |
| `oc_turns` | 剧情轮次发言（角色/代理/用户） | id / flow_id / group_id / seq | 0 | `server/oc/model.go` |
| `oc_windows` | 剧情窗口审核结果（区间、等级） | id / flow_id / group_id / from_seq | 0 | `server/oc/model.go` |
| `oc_budget_usage` | OC 群每日预算消耗 | id / group_id / day / cost | 0 | `server/oc/model.go` |

<a id="domain-8"></a>

## AI 中台

厂商、模型、密钥池、提示词、调用计费、会话、媒体验成、安全闸。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `ai_vendors` | 上游厂商标识与排序 | id / name / sort_order / enabled | 14 | `server/models/aiops/ai_vendor.go` |
| `ai_models` | 模型注册：类型、单价、base_url、密钥密文 | id / name / kind / model_id | 24 | `server/models/ai/ai_pool.go` |
| `ai_model_keys` | 模型多密钥池（权重、失败计数、熔断） | id / model_id / label / key_enc | 2 | `server/models/aihub/ai_model_key.go` |
| `ai_channel_keys` | 上游渠道密钥池（权重、失败次数） | id / base_url / label / key_enc | 2 | `server/models/aihub/ai_channel_key.go` |
| `ai_channel_accounts` | 账号级渠道授权（令牌密文与指纹） | id / base_url / label / user_id | 1 | `server/models/aihub/ai_channel_account.go` |
| `ai_category_prices` | 按内容类别的计费单价 | kind / price_in / price_out / price_call | 2 | `server/models/ai/ai_category_price.go` |
| `ai_prompts` | 提示词（内置/用户自建） | id / user_id / kind / category | 17 | `server/models/aiops/ai_prompt.go` |
| `ai_prompt_templates` | 提示词模板（场景、变量） | id / key / title / scene | 25 | `server/prompttpl/template.go` |
| `ai_prompt_template_versions` | 提示词模板版本历史 | id / template_id / version / content | 0 | `server/prompttpl/template.go` |
| `ai_styles` | 写作风格预设 | id / kind / name / sort_order | 14 | `server/models/aihub/ai_style.go` |
| `ai_call_logs` | 每次模型调用：Token、费用、耗时、结果 | id / user_id / model_id / model_name | 215 | `server/models/ai/ai_billing.go` |
| `ai_chat_sessions` | 对话会话（用户/智能体/模型） | id / user_id / agent_id / agent_name | 101 | `server/models/ai/ai_chat.go` |
| `ai_chat_messages` | 对话消息与计费明细 | id / session_id / role / content | 405 | `server/models/ai/ai_chat.go` |
| `ai_media_tasks` | 图片/视频生成任务与参数 | id / user_id / kind / model_id | 15 | `server/models/ai/ai_media.go` |
| `ai_agents` | 内置智能体（人设、开场白、启用状态） | id / name / emoji / description | 4 | `server/models/ai/ai_agent.go` |
| `agent_submissions` | 用户提交的智能体（待审核上架） | id / user_id / user_name / name | 0 | `server/models/ai/agent_submission.go` |
| `ai_content_marks` | 内容 AI 标识登记（目标类型 + 模型） | id / target_type / target_id / ai_model | 0 | `server/models/aihub/ai_content_mark.go` |
| `ai_judge_logs` | AI 审核判定日志（触发点、模型、结论） | id / user_id / username / session_id | 35 | `server/aisafety/models.go` |
| `ai_safety_configs` | 三层安全闸配置与阈值 | id / enabled / guard_input / guard_output | 1 | `server/aisafety/models.go` |
| `ai_safety_events` | 安全闸事件（方向、动作、等级、分类） | id / user_id / username / direction | 41 | `server/aisafety/models.go` |
| `ai_safety_profiles` | 用户风险画像（评分、命中、冷却期） | user_id / username / risk_score / block_count | 1 | `server/aisafety/models.go` |
| `ai_bot_usages` | 机器人每日调用次数配额 | id / user_id / day / calls_used | 0 | `server/models/ai/ai_bot.go` |
| `ai_novel_memories` | 长篇记忆摘要与 RAG 切片（按章节） | id / novel_id / kind / chapter_id | 0 | `server/models/aiops/ai_memory.go` |

<a id="domain-9"></a>

## AI 写作 Agent

写作工程（设定/人物/时间线/大纲）与生成章节草稿。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `authorai_projects` | 写作工程：设定、人物、时间线、大纲（与 AI 中台数据隔离） | id / user_id / novel_id / title | 4 | `server/models/authorai/project.go` |
| `authorai_details` | 工程内各模块条目（按 module 分组） | id / project_id / module / title | 5 | `server/models/authorai/detail.go` |
| `authorai_chapters` | 生成的章节草稿与状态 | id / project_id / order / title | 0 | `server/models/authorai/chapter.go` |

<a id="domain-10"></a>

## AIGC 合规

原图资产、内容寻址交付副本、取图授权、导出与豁免留痕。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `aigc_assets` | 原始生成资产（图/视频/音频） | id / user_id / task_id / source | 7 | `server/models/aiops/aigc.go` |
| `aigc_deliveries` | 交付副本：内容寻址键、标识结果、公开与命中数 | object_key 唯一 / produce_id / labeled / public | 3 | `server/models/aiops/aigc.go` |
| `aigc_delivery_grants` | 取图授权（object_key + user_id 唯一） | (object_key,user_id) 唯一 | 3 | `server/models/aiops/aigc.go` |
| `aigc_exports` | 导出记录：ProduceID 与模型名映射（国标编号） | id / produce_id / asset_id / user_id | 15 | `server/models/aiops/aigc.go` |
| `aigc_waiver_logs` | 显式标识豁免知情记录（原文哈希、协议版本、结果） | id / user_id / user_name / produce_id | 4 | `server/models/aiops/aigc.go` |

<a id="domain-11"></a>

## 钱包与支付

代币流水余额、充值提现、卡密、VIP、支付风控、作者收益。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `token_ledger` | 代币流水（类型、金额、变动后余额、业务引用） | id / user_id / type / amount | 9 | `server/models/wallet/token.go` |
| `user_token_balances` | 代币余额（一人一行） | user_id / balance / updated_at | 1 | `server/models/wallet/token.go` |
| `recharge_orders` | 充值订单（渠道、金额分、到账代币） | id / order_no / user_id / channel | 0 | `server/models/wallet/payment.go` |
| `withdrawal_requests` | 提现申请与审核状态 | id / user_id / amount / method | 0 | `server/models/platform/platform.go` |
| `card_keys` | 通用卡密（代币），存储哈希与前缀 | id / code_hash / code_prefix / value | 3 | `server/models/wallet/token.go` |
| `vip_cards` | VIP 卡密（等级、天数、批次） | id / code_hash / code_prefix / level | 1 | `server/vipcard/model.go` |
| `vip_applications` | VIP 申请与审核 | id / user_id / status / reason | 0 | `server/models/platform/platform_vip.go` |
| `payment_risk_events` | 支付风控事件（评分、决策、原因） | id / order_no / user_id / score | 0 | `server/risk/risk.go` |
| `earnings_records` | 作者收益流水 | id / user_id / novel_id / amount | 0 | `server/models/platform/platform.go` |

<a id="domain-12"></a>

## 存储与配额

正文库内副本、HTML 作品空间、发布/AI 额度。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `content_blobs` | 正文库内副本（chapter/blog），双模式存储的核心表 | (ref_type,ref_id) 主键 / content / content_hash | 2069 | `server/models/content_blob.go` |
| `user_htmls` | 用户上传的 HTML 作品（解压目录、入口文件） | id / user_id / novel_id / name | 1 | `server/models/softdel/userframe.go` |
| `html_quota_overrides` | HTML 空间配额覆盖（按用户） | id / user_id / max_projects / max_total_size_mb | 0 | `server/models/quota/html_quota.go` |
| `user_web_quotas` | 用户网站空间配额 | id / user_id / quota / note | 0 | `server/models/quota/web_quota.go` |
| `web_quota_requests` | 空间扩容申请与审批 | id / user_id / target_type / target_id | 0 | `server/models/quota/web_quota.go` |
| `user_publish_quotas` | 发布配额（篇数、字数上限） | id / user_id / count / word_cap | 0 | `server/models/quota/publish_quota.go` |
| `user_publish_logs` | 发布流水（用于配额核算） | id / user_id / kind / word_count | 5 | `server/models/quota/publish_quota.go` |
| `user_frames` | 作品框/装扮 | id / user_id / novel_id / name | 0 | `server/models/softdel/userframe.go` |
| `user_ai_free_quotas` | AI 免费额度配置（每日次数与 Token） | id / user_id / daily_calls / daily_tokens | 0 | `server/models/aihub/ai_free_quota.go` |
| `user_ai_free_usages` | AI 免费额度当日用量 | id / user_id / day / calls_used | 11 | `server/models/aihub/ai_free_quota.go` |
| `user_ai_keys` | 用户自带模型密钥（密文存储） | id / user_id / label / base_url | 0 | `server/models/aicost/user_ai_key.go` |

<a id="domain-13"></a>

## 安全与风控

敏感词、XSS、蜜罐、机审、舆情、API Key 限流与告警。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `bad_words` | 敏感词库（匹配方式、处置动作、作用域） | id / name / match_type / pattern | 72 | `server/models/security/badword.go` |
| `bad_word_flags` | 敏感词命中标记与处置状态 | id / content_type / content_id / user_id | 2 | `server/models/security/badword.go` |
| `xss_rules` | XSS 过滤规则 | id / name / pattern / action | 0 | `server/models/security/xss_rule.go` |
| `honeypot_sessions` | 蜜罐会话（来源 IP、UA 归一、事件计数） | id / h_id / src_ip / ua_norm | 15 | `server/honeypot/model.go` |
| `honeypot_events` | 蜜罐事件（诱饵、方法、路径、指纹） | id / session_id / kind / decoy | 367 | `server/honeypot/model.go` |
| `honeypot_fingerprints` | 蜜罐采集的浏览器指纹（canvas/WebGL/字体） | id / session_id / canvas / web_gl | 0 | `server/honeypot/fingerprint.go` |
| `honeypot_settings` | 蜜罐开关与采集粒度 | id / enabled / level / record_ua | 1 | `server/honeypot/model.go` |
| `blacklist_ips` | IP 黑名单（含到期时间） | id / ip / reason / expires_at | 0 | `server/models/platform/platform.go` |
| `moderation_config` | 机审配置（模型、触发时机、批量间隔） | id / enabled / model_id / novel_blog_on_write | 1 | `server/moderation/models.go` |
| `moderation_records` | 机审记录（范围、触发点、结论） | id / scope / target_type / target_id | 19 | `server/moderation/models.go` |
| `sentiment_configs` | 舆情总开关与阈值 | id / enabled / block_risk / flag_risk | 1 | `server/sentiment/store/config.go` |
| `sentiment_terms` | 舆情词库（类别、严重度、动作覆盖） | id / word / cat / escalate_under | 0 | `server/sentiment/store/models.go` |
| `sentiment_term_audits` | 舆情词库变更审计 | id / term_id / word / action | 0 | `server/sentiment/store/models.go` |
| `sentiment_hits` | 舆情命中记录与处置决策 | id / content_type / content_id / comment_id | 0 | `server/sentiment/store/models.go` |
| `sentiment_content_topics` | 内容舆情画像（标签、风险、舆论分） | id / content_type / content_id / tags | 0 | `server/sentiment/store/models.go` |
| `sentiment_camp_marks` | 阵营标记（内容/评论维度的立场） | id / content_type / content_id / comment_id | 0 | `server/sentiment/store/models.go` |
| `sentiment_camp_target_settings` | 单个内容的阵营识别开关 | id / content_type / content_id / enabled | 0 | `server/sentiment/store/models.go` |
| `sentiment_delete_requests` | 舆情触发的删除申请 | id / action / target_type / target_id | 0 | `server/sentiment/store/models.go` |
| `api_keys` | API Key（哈希、前缀、角色、每分钟配额、IP 白名单） | id / user_id / name / role | 0 | `server/models/apikey/api_key.go` |
| `api_key_alarms` | API Key 异常告警与确认 | id / kind / key_id / user_id | 0 | `server/models/apikey/api_key_alarm.go` |
| `api_access_logs` | API 访问日志（身份、路径、状态码、IP） | id / identity_type / identity_id / key_prefix | 109 | `server/models/apikey/api_key.go` |
| `user_api_policies` | 用户限流策略与 Key 封禁状态 | id / user_id / quota_per_min / window_secs | 0 | `server/models/apikey/api_key.go` |
| `rage_configs` | 限流/长连接参数（批量间隔、并发、WS/MQTT 开关） | id / enabled / batch_interval_sec / batch_size | 1 | `server/models/siteops/rage_config.go` |
| `channel_balance_samples` | 上游渠道余额采样（用于额度预警） | id / base_url / remaining / used | 574 | `server/models/aicost/channel_balance_sample.go` |

<a id="domain-14"></a>

## 监管与处置

警方取证、隔离墙、心理危机、软删除快照、紧急操作审计、漏洞登记。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `police_tickets` | 警方工单（类型、优先级、状态、对象用户） | id / no / kind / priority | 0 | `server/police/police.go` |
| `police_ticket_logs` | 警方工单流转日志（状态变更） | id / ticket_id / actor_id / action | 0 | `server/police/police.go` |
| `police_evidence_requests` | 取证申请：申请→站长批准→导出 | id / no / requester_id / target_user_id | 1 | `server/police/evidence_request.go` |
| `police_evidence_request_logs` | 取证申请流转日志 | id / evidence_request_id / actor_id / action | 2 | `server/police/evidence_request.go` |
| `supervised_users` | 隔离墙名单（等级、评分、来源、证据引用） | id / user_id / username / tier | 2 | `server/supervise/supervise.go` |
| `supervise_logs` | 隔离墙处置日志（等级变更） | id / supervised_id / user_id / actor_id | 9 | `server/supervise/supervise.go` |
| `crisis_events` | 心理危机事件与干预动作 | id / user_id / username / source | 0 | `server/models/security/crisis.go` |
| `softdel_users` | 软删除用户快照 | id / original_id / deleted_by / operator | 0 | `server/models/softdel/softdel.go` |
| `softdel_novels` | 软删除作品快照 | id / original_id / deleted_by / operator | 0 | `server/models/softdel/softdel.go` |
| `softdel_chapters` | 软删除章节快照 | id / original_id / deleted_by / operator | 0 | `server/models/softdel/softdel.go` |
| `softdel_blogs` | 软删除博客快照 | id / original_id / deleted_by / operator | 0 | `server/models/softdel/softdel.go` |
| `softdel_posts` | 软删除帖子快照 | id / original_id / deleted_by / operator | 2 | `server/models/softdel/softdel.go` |
| `softdel_comments` | 软删除评论快照 | id / original_id / deleted_by / operator | 0 | `server/models/softdel/softdel.go` |
| `softdel_threads` | 软删除主题快照 | id / original_id / deleted_by / operator | 2 | `server/models/softdel/softdel.go` |
| `softdel_forums` | 软删除版块快照 | id / original_id / deleted_by / operator | 0 | `server/models/softdel/softdel.go` |
| `emergency_audit_logs` | 紧急操作（封站/解锁等）审计 | id / operator_id / operator / ip | 15 | `server/models/siteops/emergency_audit.go` |
| `vulnerabilities` | 漏洞登记（严重度、影响面、修复时间） | id / name / description / severity | 0 | `server/models/security/vulnerability.go` |

<a id="domain-15"></a>

## 站点运营

平台配置、公告、轮播、导航、首页区块、主题、搜索日志。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `platform_configs` | 平台配置键值（storage_mode、阅读权限、开关等） | key 主键 / value | 59 | `server/models/platform_config.go` |
| `system_settings` | 系统设置键值 | key / value / updated_at | 1 | `server/models/siteops/system_setting.go` |
| `announcements` | 站内公告（置顶、发布状态） | id / title / content / pinned | 1 | `server/models/siteops/announcement.go` |
| `carousels` | 首页轮播图（投放时间、目标） | id / title / image_url / link_url | 0 | `server/models/siteops/site_ops.go` |
| `nav_menus` | 导航菜单（层级、图标、可见性） | id / parent_id / title / icon | 0 | `server/models/siteops/nav_menu.go` |
| `home_blocks` | 首页区块（类型、配置 JSON、排序） | id / type / title / config | 6 | `server/models/theme/home_block.go` |
| `site_themes` | 全站主题：CSS、自定义 JS 与加载模式 | id / css / updated_at / custom_js | 1 | `server/models/theme/site_theme.go` |
| `user_themes` | 用户个人主题覆盖 | id / user_id / css / updated_at | 0 | `server/models/theme/user_theme.go` |
| `search_logs` | 站内搜索日志（关键词、结果数、来源） | id / query / user_id / result_count | 0 | `server/models/search/searchlog.go` |

<a id="domain-16"></a>

## 访问统计

全站访问日志与分区进入日志。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `web_access_logs` | 全站访问日志（UA 族、路径、IP、身份） | id / user_id / identity_type / ua_family | 40974 | `server/models/quota/web_access.go` |
| `zone_access_logs` | 分区进入日志（zone、IP、UA） | id / user_id / zone / entered_at | 5 | `server/models/platform/zone_access.go` |

<a id="domain-17"></a>

## 联邦

联邦站点、作品缓存、对端密钥、入站请求。

| 表名 | 用途 | 主要字段 | 行数 | 源码 |
| --- | --- | --- | --- | --- |
| `federated_sites` | 联邦站点登记（API 地址、状态、同步时间） | id / name / url / api_url | 0 | `server/models/platform/platform_federated.go` |
| `federated_novels` | 联邦作品缓存（远端 ID 映射） | id / site_id / remote_id / title | 0 | `server/models/platform/platform_federated.go` |
| `federated_peer_keys` | 联邦对端密钥（方向、哈希、密文） | id / site_id / direction / peer_name | 0 | `server/models/apikey/api_key.go` |
| `federated_inbound` | 入站请求统计（来源、端点、计数） | id / name / url / ip | 15 | `server/models/platform/platform_federated.go` |
