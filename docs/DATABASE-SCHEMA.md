# 数据表字段明细

> 由 `scripts/gen-db-doc.py` 依据实际库结构 + 模型自动生成，**请勿手工修改**；
> 改模型后重新生成即可。用途说明见 [`DATABASE.md`](DATABASE.md)。

共 197 张表、2244 个字段。

## 账号与用户

### `users`

账号主体：邮箱/密码哈希、昵称头像、角色、会员等级、状态（行数 22）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `username` | TEXT |  |
| `email` | TEXT |  |
| `password_hash` | TEXT | 哈希 |
| `nickname` | TEXT |  |
| `avatar_url` | TEXT |  |
| `signature` | TEXT |  |
| `bio` | TEXT |  |
| `role` | TEXT |  |
| `signing_key` | TEXT |  |
| `real_name_verified` | NUMERIC |  |
| `registration_ip` | TEXT |  |
| `last_login_ip` | TEXT |  |
| `last_login_at` | DATETIME |  |
| `login_fail_count` | INTEGER |  |
| `locked_until` | DATETIME |  |
| `muted_until` | DATETIME |  |
| `mute_reason` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |
| `personalized_recommend` | NUMERIC |  |
| `police_granted_by` | TEXT |  |
| `police_granted_at` | DATETIME |  |
| `phone` | TEXT |  |
| `real_name` | TEXT |  |
| `id_card_hash` | TEXT | 哈希 |
| `id_card_masked` | TEXT |  |
| `real_name_at` | DATETIME |  |
| `birth_date` | DATETIME |  |
| `minor_protected` | NUMERIC |  |
| `we_chat_open_id` | TEXT |  |
| `we_chat_union_id` | TEXT |  |
| `require_follow_to_message` | NUMERIC |  |
| `member_until` | DATETIME |  |
| `risk_youth_mental` | NUMERIC |  |
| `risk_youth_mental_until` | DATETIME |  |
| `member_level` | INTEGER |  |

索引：`idx_users_member_until`、`idx_users_we_chat_open_id`、`idx_users_id_card_hash`、`idx_users_phone`、`idx_users_username`、`idx_users_email`、`idx_users_deleted_at`

### `user_audit_logs`

用户操作审计：登录、改密、提权等，含 IP/UA 与结果（行数 710）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `action` | TEXT |  |
| `ip` | TEXT |  |
| `user_agent` | TEXT |  |
| `detail` | TEXT |  |
| `result` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_user_audit_user_time`、`idx_user_audit_logs_action`

### `refresh_states`

当前有效的 refresh token 版本，用于强制下线（行数 16）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `user_id` | INTEGER | 主键 |
| `active_jti` | TEXT |  |
| `updated_at` | INTEGER | 时间 |

### `checkin_records`

每日签到与连签奖励（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `date` | TEXT |  |
| `streak` | INTEGER |  |
| `reward` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`uk_checkin_user_date`

### `follows`

关注关系，is_special 标记特别关注（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `follower_id` | INTEGER |  |
| `followed_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `is_special` | NUMERIC |  |

索引：`idx_follows_followed`、`idx_follows_follower`、`uk_follow`、`idx_follower`、`idx_followed`

### `messages`

私信：撤回标记 + 收发双向删除标记（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `sender_id` | INTEGER |  |
| `receiver_id` | INTEGER |  |
| `content` | TEXT |  |
| `read_at` | DATETIME |  |
| `recalled` | NUMERIC |  |
| `deleted_by_sender` | NUMERIC |  |
| `deleted_by_receiver` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_msg_pair`、`idx_msg_sender`、`idx_msg_receiver`、`idx_messages_created_at`

### `notifications`

站内通知（评论/点赞/关注/系统）（行数 61）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `actor_id` | INTEGER |  |
| `actor_name` | TEXT |  |
| `type` | TEXT |  |
| `title` | TEXT |  |
| `content` | TEXT |  |
| `link` | TEXT |  |
| `read` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_notifications_user_read_created`、`idx_notifications_type`

### `blacklists`

用户互拉黑名单（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `blocked_user_id` | INTEGER |  |
| `mode` | TEXT |  |
| `reason` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_blacklists_user_id`、`idx_blacklists_pair`、`idx_blacklists_blocked_user_id`

### `hidden_contents`

用户主动隐藏的内容（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `target_type` | TEXT |  |
| `target_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_hidden_user`

### `reports`

内容举报工单与处置状态（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `reporter_id` | INTEGER |  |
| `target_type` | TEXT |  |
| `target_id` | INTEGER |  |
| `reason_code` | TEXT |  |
| `reason` | TEXT |  |
| `detail` | TEXT |  |
| `status` | TEXT |  |
| `handler_id` | INTEGER |  |
| `verdict` | TEXT |  |
| `resolved_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_reports_reporter_id`、`idx_reports_target`、`idx_reports_status`

### `malicious_reporters`

恶意举报统计与举报封禁（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `total_reports` | INTEGER |  |
| `malicious_count` | INTEGER |  |
| `reporting_banned` | NUMERIC |  |
| `ban_reason` | TEXT |  |
| `banned_at` | DATETIME |  |
| `updated_at` | DATETIME | 时间 |

索引：`idx_malicious_reporters_user_id`

### `gate_device_bindings`

注册闸门：设备指纹与账号的一次性绑定（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `device_id` | TEXT |  |
| `user_id` | INTEGER |  |
| `ip` | TEXT |  |
| `ua_hash` | TEXT | 哈希 |
| `source` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_gate_device_bindings_device_id`、`idx_gate_device_bindings_user_id`、`idx_gate_device_bindings_ip`、`idx_gate_device_bindings_created_at`

### `gate_invites`

注册邀请码（次数、有效期）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `code` | TEXT |  |
| `created_by` | INTEGER |  |
| `max_uses` | INTEGER |  |
| `used` | INTEGER |  |
| `note` | TEXT |  |
| `expires_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_gate_invites_created_by`、`idx_gate_invites_code`

## 小说与创作

### `novels`

作品主体：作者、分类、标签、封面、价格、阅读权限（行数 16）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `author_id` | INTEGER |  |
| `title` | TEXT |  |
| `category` | TEXT |  |
| `tags` | JSON |  |
| `summary` | TEXT |  |
| `cover_url` | TEXT |  |
| `price_per_chapter` | DECIMAL(10,2) |  |
| `status` | TEXT |  |
| `source_type` | TEXT |  |
| `creation_method` | TEXT |  |
| `total_words` | INTEGER |  |
| `total_chapters` | INTEGER |  |
| `view_count` | INTEGER |  |
| `ban_reason` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |
| `deleted_by` | TEXT |  |
| `wall_enabled` | NUMERIC |  |
| `mode` | TEXT |  |
| `read_permission` | TEXT |  |
| `paid_price` | INTEGER |  |
| `derivative_type` | TEXT |  |
| `derivative_origin_id` | INTEGER |  |
| `derivative_origin_title` | TEXT |  |
| `derivative_origin_author` | TEXT |  |
| `derivative_external_source` | TEXT |  |
| `comments_enabled` | NUMERIC |  |
| `like_count` | INTEGER |  |

索引：`idx_novels_read_permission`、`idx_novels_mode`、`idx_novels_category`、`idx_novels_author_status`、`idx_novels_deleted_at`、`idx_novels_derivative_origin_id`

### `novel_categories`

作品与分类的多对多关联（行数 19）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `novel_id` | INTEGER |  |
| `category` | TEXT |  |

索引：`idx_novel_categories_novel`、`uk_novel_category`、`idx_novel_category_novel`、`idx_novel_category_cat`

### `chapters`

章节元数据（正文在文件或 content_blobs）（行数 2042）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `novel_id` | INTEGER |  |
| `chapter_number` | INTEGER |  |
| `title` | TEXT |  |
| `content_path` | TEXT |  |
| `content_hash` | TEXT | 哈希 |
| `content_signature` | TEXT | 哈希 |
| `word_count` | INTEGER |  |
| `status` | TEXT |  |
| `ban_reason` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |
| `deleted_by` | TEXT |  |
| `mode` | TEXT |  |
| `source_flow_id` | INTEGER |  |
| `author_id` | INTEGER |  |
| `review_note` | TEXT |  |
| `images` | JSON |  |

索引：`idx_chapters_author_id`、`idx_chapters_source_flow_id`、`idx_chapters_status`、`idx_chapters_novel_id`、`uk_novel_chapter`、`idx_chapters_deleted_at`

### `chapter_versions`

章节版本快照，支持回溯与合并（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `chapter_id` | INTEGER |  |
| `novel_id` | INTEGER |  |
| `chapter_num` | INTEGER |  |
| `branch` | TEXT |  |
| `version` | INTEGER |  |
| `parent_ver` | INTEGER |  |
| `merge_from` | TEXT |  |
| `merge_from_ver` | INTEGER |  |
| `title` | TEXT |  |
| `content_path` | TEXT |  |
| `content_hash` | TEXT | 哈希 |
| `word_count` | INTEGER |  |
| `message` | TEXT |  |
| `author_id` | INTEGER |  |
| `force` | NUMERIC |  |
| `deleted` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_ch_ver`、`idx_chapter_versions_novel_id`、`idx_chapter_versions_author_id`

### `chapter_branches`

章节分支（多结局/草稿线）及分支头版本（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `chapter_id` | INTEGER |  |
| `name` | TEXT |  |
| `head_ver` | INTEGER |  |
| `from_ver` | INTEGER |  |
| `from_branch` | TEXT |  |
| `author_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_branch`

### `novel_collaborators`

协作者与权限位（发布/审核/改他人稿）（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `novel_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `role` | TEXT |  |
| `status` | TEXT |  |
| `can_publish` | NUMERIC |  |
| `can_review` | NUMERIC |  |
| `can_edit_others` | NUMERIC |  |
| `revenue_share_bp` | INTEGER |  |
| `revenue_note` | TEXT |  |
| `byline_order` | INTEGER |  |
| `invited_by` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_novel_collaborators_user_id`、`idx_novel_collaborators_status`、`idx_novel_collab`

### `novel_collab_audits`

协作操作审计（前后值快照）（行数 7）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `novel_id` | INTEGER |  |
| `actor_id` | INTEGER |  |
| `action` | TEXT |  |
| `target_user_id` | INTEGER |  |
| `before` | TEXT |  |
| `after` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_novel_collab_audits_novel_id`、`idx_novel_collab_audits_actor_id`、`idx_novel_collab_audits_action`

### `novel_characters`

作品角色卡（阵营、简介、配色）（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `novel_id` | INTEGER |  |
| `name` | TEXT |  |
| `avatar` | TEXT |  |
| `color` | TEXT |  |
| `side` | TEXT |  |
| `intro` | TEXT |  |
| `sort_order` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_novel_characters_novel_id`

### `novel_documents`

设定集/世界观文档，可控制可见性（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `novel_id` | INTEGER |  |
| `kind` | TEXT |  |
| `title` | TEXT |  |
| `content` | TEXT |  |
| `sort_order` | INTEGER |  |
| `visibility` | TEXT |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_novel_doc_novel`

### `novel_likes`

作品点赞（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `novel_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`uk_novel_like`、`idx_novel_like_novel`、`idx_novel_likes_user_id`

### `novel_indexes`

index.json 的库内副本（与文件同字节 + Hash）（行数 16）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `novel_id` | INTEGER | 主键 |
| `data` | TEXT |  |
| `hash` | TEXT |  |
| `updated_at` | DATETIME | 时间 |

索引：`idx_novel_indexes_hash`

### `bookshelf`

书架与阅读进度（最后阅读章节）（行数 3）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `novel_id` | INTEGER |  |
| `last_read_chapter` | INTEGER |  |
| `added_at` | DATETIME |  |

索引：`idx_bookshelf_user`、`uk_user_novel`、`idx_bookshelf_user_id`、`idx_bookshelf_novel_id`

### `reading_notes`

划线笔记与批注（含锚点定位）（行数 15）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `novel_id` | INTEGER |  |
| `chapter_number` | INTEGER |  |
| `type` | TEXT |  |
| `content` | TEXT |  |
| `selected_text` | TEXT |  |
| `anchor` | INTEGER |  |
| `visibility` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_reading_notes_user_novel`、`idx_note_user_novel`、`idx_reading_notes_visibility`

### `ratings`

多维度评分：完成度/叙事/思想/社区口碑/更新稳定（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `novel_id` | INTEGER |  |
| `type_completion` | INTEGER |  |
| `narrative_quality` | INTEGER |  |
| `thought_depth` | INTEGER |  |
| `community_reputation` | INTEGER |  |
| `update_stability` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_ratings_user_novel`、`idx_ratings_novel`、`uk_user_novel_rating`、`idx_ratings_novel_id`

### `content_purchases`

付费章节购买记录（幂等发放阅读权限）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `content_type` | TEXT |  |
| `content_id` | INTEGER |  |
| `price` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`uk_content_purchase`、`idx_content_purchases_user_id`

### `comments`

小说/博客评论，支持引文与偏移定位（行数 6）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `novel_id` | INTEGER |  |
| `blog_id` | INTEGER |  |
| `chapter_number` | INTEGER |  |
| `content` | TEXT |  |
| `quote_text` | TEXT |  |
| `quote_offset` | INTEGER |  |
| `parent_id` | INTEGER |  |
| `is_markdown` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |
| `deleted_by` | TEXT |  |
| `root_id` | INTEGER |  |
| `floor_no` | INTEGER |  |
| `reply_to_user_id` | INTEGER |  |
| `has_password` | NUMERIC |  |
| `password_hash` | TEXT | 哈希 |
| `created_micro` | INTEGER |  |
| `mentions_raw` | TEXT |  |
| `flagged_minor` | NUMERIC |  |
| `moderation_status` | TEXT |  |
| `region` | TEXT |  |
| `images_raw` | TEXT |  |
| `poll_id` | INTEGER |  |
| `show_vote` | NUMERIC |  |

索引：`idx_comments_poll_id`、`idx_comments_flagged_minor`、`idx_comments_has_password`、`idx_comments_reply_to_user_id`、`idx_comments_created_micro`、`idx_comments_floor`、`idx_comments_root`、`idx_comments_blog`、`idx_comments_novel_created`、`idx_comments_blog_id`、`idx_comments_parent`、`idx_comments_deleted_at`、`idx_comments_user_id`

## 作者博客

### `author_blogs`

博客文章元数据（正文在文件或 content_blobs）（行数 41）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `author_id` | INTEGER |  |
| `title` | TEXT |  |
| `content_path` | TEXT |  |
| `content_hash` | TEXT | 哈希 |
| `summary` | TEXT |  |
| `is_pinned` | NUMERIC |  |
| `view_count` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |
| `category_id` | INTEGER |  |
| `like_count` | INTEGER |  |
| `deleted_by` | TEXT |  |
| `read_permission` | TEXT |  |
| `paid_price` | INTEGER |  |
| `favorite_count` | INTEGER |  |
| `cover_url` | TEXT |  |
| `comments_enabled` | NUMERIC |  |

索引：`idx_author_blogs_read_permission`、`idx_blog_category`、`idx_author_blogs_created`、`idx_author_blogs_author`、`idx_author_blog`、`idx_author_blogs_is_pinned`、`idx_author_blogs_deleted_at`

### `blog_categories`

博客分类，含审核状态与驳回原因（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `parent_id` | INTEGER |  |
| `name` | TEXT |  |
| `sort_order` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `created_by` | INTEGER |  |
| `status` | TEXT |  |
| `reject_reason` | TEXT |  |

索引：`idx_blog_categories_status`、`idx_blog_categories_created_by`、`idx_blog_cat_parent`

### `blog_tags`

博客标签字典（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `created_by` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_blog_tags_name`

### `blog_tag_links`

博客与标签的关联（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `blog_id` | INTEGER | 主键 |
| `tag_id` | INTEGER | 主键 |

### `blog_topics`

博客话题（专题聚合）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `description` | TEXT |  |
| `created_by` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_blog_topics_name`

### `blog_topic_links`

博客与话题的关联（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `blog_id` | INTEGER | 主键 |
| `topic_id` | INTEGER | 主键 |

### `blog_likes`

博客点赞（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `blog_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`uk_blog_like`、`idx_blog_like_blog`、`idx_blog_likes_user_id`

### `blog_favorites`

博客收藏（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `blog_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`uk_blog_fav`、`idx_blog_fav_blog`、`idx_blog_favorites_user_id`

### `author_themes`

作者主页主题：CSS 与博客样式（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `author_id` | INTEGER | 主键 |
| `css` | TEXT |  |
| `updated_at` | DATETIME | 时间 |
| `blog_style` | TEXT |  |

### `author_home_customize`

作者主页自定义 HTML（审核后公开）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `author_id` | INTEGER | 主键 |
| `raw_html` | TEXT |  |
| `safe_html` | TEXT |  |
| `public_html` | TEXT |  |
| `review_status` | VARCHAR(16) |  |
| `had_issues` | NUMERIC |  |
| `review_note` | TEXT |  |
| `reviewed_by` | INTEGER |  |
| `reviewed_at` | DATETIME |  |
| `updated_at` | DATETIME | 时间 |

### `user_home_customize`

用户个人主页自定义 HTML（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `user_id` | INTEGER | 主键 |
| `raw_html` | TEXT |  |
| `updated_at` | DATETIME | 时间 |

## 内容流（发现）

### `feed_posts`

动态主体：图文/视频/音乐/微文，含封面与媒体地址（行数 30）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `kind` | TEXT |  |
| `title` | TEXT |  |
| `content` | TEXT |  |
| `video_url` | TEXT |  |
| `cover_url` | TEXT |  |
| `images` | TEXT |  |
| `tags` | TEXT |  |
| `duration` | INTEGER |  |
| `status` | TEXT |  |
| `latitude` | TEXT |  |
| `longitude` | TEXT |  |
| `like_count` | INTEGER |  |
| `comment_count` | INTEGER |  |
| `favorite_count` | INTEGER |  |
| `share_count` | INTEGER |  |
| `view_count` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `audio_url` | TEXT |  |

索引：`idx_feed_user`、`idx_feed_kind`、`idx_feed_posts_status`

### `feed_comments`

动态评论，parent_id 支持楼中楼（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `post_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `parent_id` | INTEGER |  |
| `content` | TEXT |  |
| `like_count` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `poll_id` | INTEGER |  |
| `show_vote` | NUMERIC |  |

索引：`idx_feed_comments_poll_id`、`idx_feed_comments_parent_id`、`idx_feed_comment`、`idx_feed_comments_user_id`

### `feed_comment_likes`

评论点赞（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `comment_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_feed_clike`

### `feed_likes`

动态点赞（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `post_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_feed_like`

### `feed_favorites`

动态收藏（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `post_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_feed_fav`

### `feed_tags`

标签倒排索引（唯一索引 post_id+tag）（行数 38）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `post_id` | INTEGER |  |
| `tag` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_feed_tag_post_tag`、`idx_feed_tag_post`、`idx_feed_tag_name`

### `feed_danmaku`

视频弹幕（按 time_ms 绑时间轴，独立于评论计数）（行数 17）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `post_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `content` | TEXT |  |
| `time_ms` | INTEGER |  |
| `mode` | TEXT |  |
| `color` | TEXT |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_danmaku_post_time`、`idx_feed_danmaku_user_id`、`idx_feed_danmaku_status`

## 论坛

### `forums`

版块：类型、分区、关联对象、层级（行数 14）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `description` | TEXT |  |
| `type` | TEXT |  |
| `zone` | TEXT |  |
| `ref_id` | TEXT |  |
| `parent_id` | INTEGER |  |
| `sort_order` | INTEGER |  |
| `thread_count` | INTEGER |  |
| `creator_id` | INTEGER |  |
| `is_public` | NUMERIC |  |
| `password_hash` | TEXT | 哈希 |
| `room_code` | TEXT |  |
| `review_status` | TEXT |  |
| `review_note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_forums_creator_id`、`idx_forums_room_code`、`idx_forums_review_status`、`idx_forums_deleted_at`、`idx_forums_type`、`idx_forums_parent_id`

### `threads`

主题帖：置顶/锁定/浏览量（行数 6）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `forum_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `title` | TEXT |  |
| `content` | TEXT |  |
| `is_pinned` | NUMERIC |  |
| `is_locked` | NUMERIC |  |
| `view_count` | INTEGER |  |
| `post_count` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_threads_forum_created`、`idx_threads_user_id`、`idx_threads_deleted_at`

### `posts`

楼层回复，parent_id/root_id 组织楼中楼（行数 26）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `thread_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `content` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `parent_id` | INTEGER |  |
| `root_id` | INTEGER |  |
| `floor_no` | INTEGER |  |
| `reply_count` | INTEGER |  |
| `poll_id` | INTEGER |  |
| `show_vote` | NUMERIC |  |

索引：`idx_posts_poll_id`、`idx_posts_floor`、`idx_posts_root`、`idx_posts_parent`、`idx_posts_user`、`idx_posts_thread_created`、`idx_posts_user_id`

### `forum_mutes`

版块禁言（到期时间、管理员封禁标记）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `forum_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `muted_until` | DATETIME |  |
| `reason` | TEXT |  |
| `admin_ban` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_forum_mutes_forum_user`

### `user_forum_quotas`

用户建版配额（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `max_forums` | INTEGER |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_user_forum_quotas_user_id`

## 群组

### `groups`

群组主体：群号、分类、可见性、加入方式（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `owner_id` | INTEGER |  |
| `group_no` | TEXT |  |
| `name` | TEXT |  |
| `description` | TEXT |  |
| `category` | TEXT |  |
| `visibility` | TEXT |  |
| `join_mode` | TEXT |  |
| `invite_code` | TEXT |  |
| `cover_url` | TEXT |  |
| `member_limit` | INTEGER |  |
| `post_limit` | INTEGER |  |
| `review_first` | NUMERIC |  |
| `status` | TEXT |  |
| `member_count` | INTEGER |  |
| `post_count` | INTEGER |  |
| `announcement` | TEXT |  |
| `announcement_by` | INTEGER |  |
| `announcement_at` | DATETIME |  |
| `allow_member_invite` | NUMERIC |  |
| `ai_enabled` | NUMERIC |  |
| `ai_agent_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_groups_category`、`idx_groups_visibility`、`idx_groups_status`、`idx_groups_deleted_at`、`idx_groups_owner_id`、`idx_groups_group_no`、`idx_groups_name`

### `group_members`

群成员与角色、禁言到期（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `group_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `role` | TEXT |  |
| `status` | TEXT |  |
| `muted_till` | DATETIME |  |
| `note` | TEXT |  |
| `joined_at` | DATETIME |  |
| `updated_at` | DATETIME | 时间 |

索引：`idx_group_member`、`idx_group_member_user`、`idx_group_members_status`

### `group_posts`

群内帖子（可 AI 辅助）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `group_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `title` | TEXT |  |
| `content` | TEXT |  |
| `status` | TEXT |  |
| `reply_cnt` | INTEGER |  |
| `ai_assisted` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_group_posts_group_id`、`idx_group_posts_user_id`、`idx_group_posts_status`、`idx_group_posts_deleted_at`

### `group_post_replies`

群帖回复（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `group_id` | INTEGER |  |
| `post_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `content` | TEXT |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_group_post_replies_group_id`、`idx_group_post_replies_post_id`、`idx_group_post_replies_user_id`、`idx_group_post_replies_status`、`idx_group_post_replies_deleted_at`

### `group_chat_messages`

群聊消息（支持引用回复）（行数 3）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `group_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `content` | TEXT |  |
| `reply_to_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_gcm_group`、`idx_group_chat_messages_user_id`

### `group_files`

群文件（SHA-256 去重 + 存储路径）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `group_id` | INTEGER |  |
| `uploader_id` | INTEGER |  |
| `name` | TEXT |  |
| `size` | INTEGER |  |
| `mime` | TEXT |  |
| `sha256` | TEXT |  |
| `storage_path` | TEXT |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_group_files_sha256`、`idx_group_files_status`、`idx_group_files_deleted_at`、`idx_group_files_group_id`、`idx_group_files_uploader_id`

### `group_read_states`

群已读位置（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `group_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `last_read_at` | DATETIME |  |
| `last_read_post_id` | INTEGER |  |
| `updated_at` | DATETIME | 时间 |

索引：`idx_group_read`

### `user_group_quotas`

用户建群配额（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `max_groups` | INTEGER |  |
| `note` | TEXT |  |
| `updated_at` | DATETIME | 时间 |

索引：`idx_user_group_quotas_user_id`

## OC 角色扮演

### `oc_characters`

OC 角色卡：人设、外观、口吻、场景（行数 13）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `name` | TEXT |  |
| `avatar_url` | TEXT |  |
| `persona` | TEXT |  |
| `appearance` | TEXT |  |
| `speech` | TEXT |  |
| `scenario` | TEXT |  |
| `first_message` | TEXT |  |
| `mes_example` | TEXT |  |
| `system_prompt` | TEXT |  |
| `post_history` | TEXT |  |
| `creator_notes` | TEXT |  |
| `greetings` | TEXT |  |
| `book_id` | INTEGER |  |
| `mood` | INTEGER |  |
| `affinity` | INTEGER |  |
| `energy` | INTEGER |  |
| `emote_map` | TEXT |  |
| `tags` | TEXT |  |
| `visibility` | TEXT |  |
| `review` | TEXT |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `oc_kind` | TEXT |  |
| `species` | TEXT |  |
| `species_traits` | TEXT |  |
| `anthro_type` | TEXT |  |
| `project_id` | INTEGER |  |

索引：`idx_oc_characters_project_id`、`idx_oc_characters_user_id`、`idx_oc_characters_name`、`idx_oc_characters_visibility`、`idx_oc_characters_review`

### `oc_character_arts`

角色形象图库（立绘/表情/设定参考/场景 + 强调特征）（行数 11）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `character_id` | INTEGER |  |
| `project_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `url` | TEXT |  |
| `title` | TEXT |  |
| `kind` | TEXT |  |
| `traits` | TEXT |  |
| `note` | TEXT |  |
| `sort` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_oc_character_arts_character_id`、`idx_oc_character_arts_project_id`、`idx_oc_character_arts_user_id`

### `oc_emotes`

OC 表情包（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `key` | TEXT |  |
| `url` | TEXT |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_oc_emotes_user_id`、`idx_oc_emotes_key`

### `oc_world_books`

世界书（扫描深度、Token 预算）（行数 29）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `group_id` | INTEGER |  |
| `title` | TEXT |  |
| `summary` | TEXT |  |
| `version` | INTEGER |  |
| `scan_depth` | INTEGER |  |
| `token_budget` | INTEGER |  |
| `recursive_scan` | NUMERIC |  |
| `is_official` | NUMERIC |  |
| `visibility` | TEXT |  |
| `review` | TEXT |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `project_id` | INTEGER |  |
| `kind` | TEXT |  |

索引：`idx_oc_world_books_kind`、`idx_oc_world_books_project_id`、`idx_oc_world_books_group_id`、`idx_oc_world_books_is_official`、`idx_oc_world_books_visibility`、`idx_oc_world_books_review`、`idx_oc_world_books_user_id`

### `oc_world_entries`

世界书条目（关键词、二次关键词、逻辑）（行数 10）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `book_id` | INTEGER |  |
| `kind` | TEXT |  |
| `key` | TEXT |  |
| `content` | TEXT |  |
| `secondary_keys` | TEXT |  |
| `selective` | NUMERIC |  |
| `logic` | INTEGER |  |
| `constant` | NUMERIC |  |
| `enabled` | NUMERIC |  |
| `probability` | INTEGER |  |
| `case_sensitive` | NUMERIC |  |
| `whole_word` | NUMERIC |  |
| `insertion_order` | INTEGER |  |
| `priority` | INTEGER |  |
| `position` | INTEGER |  |
| `depth` | INTEGER |  |
| `group` | TEXT |  |
| `sort_order` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_oc_world_entries_book_id`、`idx_oc_world_entries_kind`、`idx_oc_world_entries_key`

### `oc_world_links`

世界书之间的引用链（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `from_book_id` | INTEGER |  |
| `to_book_id` | INTEGER |  |
| `label` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_oc_world_links_from_book_id`、`idx_oc_world_links_to_book_id`

### `oc_projects`

OC 创作项目（可见性、访问密码）（行数 15）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `title` | TEXT |  |
| `summary` | TEXT |  |
| `cover_url` | TEXT |  |
| `category` | TEXT |  |
| `visibility` | TEXT |  |
| `password_hash` | TEXT | 哈希 |
| `world_count` | INTEGER |  |
| `member_count` | INTEGER |  |
| `review` | TEXT |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_oc_projects_user_id`、`idx_oc_projects_title`、`idx_oc_projects_visibility`、`idx_oc_projects_review`

### `oc_project_members`

项目成员与角色（行数 15）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `project_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `role` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_ocpm`

### `oc_project_access`

项目访问令牌（带过期时间）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `project_id` | INTEGER |  |
| `token` | TEXT |  |
| `expires_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_oc_project_access_project_id`、`idx_oc_project_access_token`、`idx_oc_project_access_expires_at`

### `oc_groups`

OC 群组（AI 角色群聊）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `name` | TEXT |  |
| `summary` | TEXT |  |
| `cover_url` | TEXT |  |
| `tags` | TEXT |  |
| `member_count` | INTEGER |  |
| `visibility` | TEXT |  |
| `review` | TEXT |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_oc_groups_user_id`、`idx_oc_groups_name`、`idx_oc_groups_visibility`、`idx_oc_groups_review`

### `oc_group_settings`

OC 群审计等级、窗口大小与时机（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `group_id` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `mode` | TEXT |  |
| `audit_level` | TEXT |  |
| `audit_timing` | TEXT |  |
| `window_size` | INTEGER |  |
| `window_rounds` | INTEGER |  |
| `daily_budget` | INTEGER |  |
| `flow_budget` | INTEGER |  |
| `max_turns` | INTEGER |  |
| `auto_run` | NUMERIC |  |
| `red_team_enabled` | NUMERIC |  |
| `red_team_note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_oc_group_settings_group_id`

### `oc_group_members`

群内绑定的 OC 角色（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `group_id` | INTEGER |  |
| `character_id` | INTEGER |  |
| `sort_order` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_ocgm`

### `oc_agents`

群内 AI 角色绑定的模型/模板/策略/预算（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `group_id` | INTEGER |  |
| `character_id` | INTEGER |  |
| `model_id` | INTEGER |  |
| `template_id` | INTEGER |  |
| `strategy` | TEXT |  |
| `enabled` | NUMERIC |  |
| `budget` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_oc_agents_group_id`、`idx_oc_agents_character_id`

### `oc_flows`

剧情流：一次连续演出（排期、轮次）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `group_id` | INTEGER |  |
| `starter_id` | INTEGER |  |
| `title` | TEXT |  |
| `book_id` | INTEGER |  |
| `schedule` | TEXT |  |
| `status` | TEXT |  |
| `turn_count` | INTEGER |  |
| `max_turns` | INTEGER |  |
| `stop_reason` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_oc_flows_group_id`、`idx_oc_flows_status`

### `oc_turns`

剧情轮次发言（角色/代理/用户）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `flow_id` | INTEGER |  |
| `group_id` | INTEGER |  |
| `seq` | INTEGER |  |
| `character_id` | INTEGER |  |
| `agent_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `content` | TEXT |  |
| `model_id` | INTEGER |  |
| `model_name` | TEXT |  |
| `tokens_in` | INTEGER |  |
| `tokens_out` | INTEGER |  |
| `cost` | INTEGER |  |
| `emotion` | TEXT |  |
| `review` | TEXT |  |
| `window_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_oc_turns_flow_id`、`idx_oc_turns_group_id`、`idx_oc_turns_seq`、`idx_oc_turns_review`、`idx_oc_turns_window_id`

### `oc_windows`

剧情窗口审核结果（区间、等级）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `flow_id` | INTEGER |  |
| `group_id` | INTEGER |  |
| `from_seq` | INTEGER |  |
| `to_seq` | INTEGER |  |
| `turn_count` | INTEGER |  |
| `level` | TEXT |  |
| `result` | TEXT |  |
| `hits` | TEXT |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_oc_windows_flow_id`、`idx_oc_windows_group_id`

### `oc_budget_usage`

OC 群每日预算消耗（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `group_id` | INTEGER |  |
| `day` | TEXT |  |
| `cost` | INTEGER |  |
| `turns` | INTEGER |  |
| `updated_at` | DATETIME | 时间 |

索引：`idx_oc_budget`

## 自托管

### `selfhost_runtimes`

自托管运行时版本目录（语言/版本/编译型或解释型，站长维护）（行数 9）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `language` | TEXT |  |
| `version` | TEXT |  |
| `label` | TEXT |  |
| `kind` | TEXT |  |
| `install_hint` | TEXT |  |
| `download_url` | TEXT |  |
| `checksum` | TEXT |  |
| `enabled` | NUMERIC |  |
| `sort` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `exec_path` | TEXT |  |

索引：`idx_selfhost_runtimes_enabled`、`idx_selfhost_runtimes_language`

### `selfhost_projects`

自托管项目（源码模式/编译版、源码与可执行文件、upstream、审核状态）（行数 23）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `name` | TEXT |  |
| `slug` | TEXT |  |
| `description` | TEXT |  |
| `language` | TEXT |  |
| `runtime_id` | INTEGER |  |
| `mode` | TEXT |  |
| `source_file` | TEXT |  |
| `binary_file` | TEXT |  |
| `source_name` | TEXT |  |
| `binary_name` | TEXT |  |
| `build_command` | TEXT |  |
| `start_command` | TEXT |  |
| `upstream` | TEXT |  |
| `visibility` | TEXT |  |
| `review_status` | TEXT |  |
| `review_note` | TEXT |  |
| `reviewed_by` | INTEGER |  |
| `reviewed_at` | DATETIME |  |
| `source_waived` | NUMERIC |  |
| `local_exec` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |
| `source_shared` | NUMERIC |  |
| `binary_shared` | NUMERIC |  |

索引：`idx_selfhost_projects_language`、`idx_selfhost_projects_runtime_id`、`idx_selfhost_projects_mode`、`idx_selfhost_projects_visibility`、`idx_selfhost_projects_review_status`、`idx_selfhost_projects_deleted_at`、`idx_selfhost_projects_user_id`、`idx_selfhost_projects_slug`

### `selfhost_review_logs`

自托管项目审核留痕（提交/通过/驳回/停止/恢复）（行数 4）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `project_id` | INTEGER |  |
| `operator_id` | INTEGER |  |
| `action` | TEXT |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_selfhost_review_logs_project_id`、`idx_selfhost_review_logs_operator_id`

### `selfhost_routes`

自托管反向代理路由（前缀最长匹配、upstream、启停与超时）（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `project_id` | INTEGER |  |
| `prefix` | TEXT |  |
| `target` | TEXT |  |
| `strip_prefix` | NUMERIC |  |
| `timeout` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `status` | TEXT |  |
| `last_error` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `cookie_hat` | TEXT |  |
| `drop_foreign` | NUMERIC |  |
| `tls_skip_verify` | NUMERIC |  |

索引：`idx_selfhost_routes_project_id`、`idx_selfhost_routes_prefix`、`idx_selfhost_routes_enabled`、`idx_selfhost_routes_status`

### `selfhost_file_grants`

自托管文件访问授权（源码/产物，默认拒绝，所有者逐用户审批）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `project_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `kind` | TEXT |  |
| `status` | TEXT |  |
| `note` | TEXT |  |
| `reviewed_by` | INTEGER |  |
| `reviewed_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_selfhost_file_grants_user_id`、`idx_selfhost_file_grants_status`、`idx_selfhost_file_grants_project_id`

### `selfhost_direct_routes`

管理员端口直派路由（路径→本机端口，GWh 同款，免审核）（行数 10）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `prefix` | TEXT |  |
| `port` | INTEGER |  |
| `root_path` | TEXT |  |
| `enabled` | NUMERIC |  |
| `description` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `target_url` | TEXT |  |
| `cookie_hat` | TEXT |  |
| `drop_foreign` | NUMERIC |  |
| `tls_skip_verify` | NUMERIC |  |

索引：`idx_selfhost_direct_routes_prefix`、`idx_selfhost_direct_routes_enabled`

### `selfhost_port_maps`

四层 TCP 端口映射（监听端口→目标主机:端口）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `listen_port` | INTEGER |  |
| `target_host` | TEXT |  |
| `target_port` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `description` | TEXT |  |
| `last_error` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_selfhost_port_maps_listen_port`、`idx_selfhost_port_maps_enabled`

### `selfhost_usage_daily`

自托管项目按天流量统计（请求数、上下行字节）（行数 8）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `project_id` | INTEGER |  |
| `day` | TEXT |  |
| `requests` | INTEGER |  |
| `bytes_up` | INTEGER |  |
| `bytes_down` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_selfhost_usage_day`

### `selfhost_runtime_requests`

用户申请缺失运行时（管理员可改语言/版本后落地）（行数 5）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `project_id` | INTEGER |  |
| `language` | TEXT |  |
| `version` | TEXT |  |
| `note` | TEXT |  |
| `status` | TEXT |  |
| `final_language` | TEXT |  |
| `final_version` | TEXT |  |
| `admin_note` | TEXT |  |
| `reviewed_by` | INTEGER |  |
| `reviewed_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_selfhost_runtime_requests_user_id`、`idx_selfhost_runtime_requests_project_id`、`idx_selfhost_runtime_requests_status`

### `selfhost_iso_services`

路径隔离代理服务（pass-through / strip / isolated 三种模式）（行数 20）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `target` | TEXT |  |
| `mode` | TEXT |  |
| `timeout` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `description` | TEXT |  |
| `cookie_hat` | TEXT |  |
| `html_rewrite` | NUMERIC |  |
| `known_abs_prefix` | TEXT |  |
| `host_mode` | TEXT |  |
| `no_full_compat` | NUMERIC |  |
| `max_rewrite_bytes` | INTEGER |  |
| `drop_foreign` | NUMERIC |  |
| `pass_through_fix` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_selfhost_iso_services_name`、`idx_selfhost_iso_services_enabled`

## AI 中台

### `ai_vendors`

上游厂商标识与排序（行数 14）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `sort_order` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `prefix` | TEXT |  |

索引：`idx_ai_vendors_prefix`、`idx_ai_vendors_name`

### `ai_models`

模型注册：类型、单价、base_url、密钥密文（行数 24）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `kind` | TEXT |  |
| `model_id` | TEXT |  |
| `base_url` | TEXT |  |
| `api_key_enc` | TEXT | 密文 |
| `price_in` | INTEGER |  |
| `price_out` | INTEGER |  |
| `price_call` | INTEGER |  |
| `max_tokens` | INTEGER |  |
| `temperature` | REAL |  |
| `enabled` | NUMERIC |  |
| `priority` | INTEGER |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `api_kind` | TEXT |  |
| `provider_hint` | TEXT |  |
| `vendor` | TEXT |  |
| `thinking` | NUMERIC |  |
| `alias` | TEXT |  |
| `channel` | TEXT |  |
| `category_tags` | TEXT |  |
| `user_visible` | NUMERIC |  |
| `version` | TEXT |  |
| `sub_version` | TEXT |  |
| `token_user_id` | TEXT |  |
| `account_token_enc` | TEXT | 密文 |
| `balance_provider` | TEXT |  |
| `balance_url` | TEXT |  |
| `balance_field` | TEXT |  |
| `balance_divisor` | REAL |  |
| `balance_multiplier` | REAL |  |
| `balance_currency` | TEXT |  |
| `rate_real_amount` | REAL |  |
| `rate_virtual_amount` | REAL |  |
| `rate_real_currency` | TEXT |  |
| `search_endpoint` | NUMERIC |  |
| `search_native` | NUMERIC |  |

索引：`idx_ai_models_sub_version`、`idx_ai_models_version`、`idx_ai_models_channel`、`idx_ai_models_vendor`、`idx_ai_models_kind`

### `ai_model_keys`

模型多密钥池（权重、失败计数、熔断）（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `model_id` | INTEGER |  |
| `label` | TEXT |  |
| `key_enc` | TEXT | 密文 |
| `weight` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `fail_count` | INTEGER |  |
| `last_fail_at` | DATETIME |  |
| `last_used_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_ai_model_keys_model_id`

### `ai_channel_keys`

上游渠道密钥池（权重、失败次数）（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `base_url` | TEXT |  |
| `label` | TEXT |  |
| `key_enc` | TEXT | 密文 |
| `fingerprint` | TEXT |  |
| `weight` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `fail_count` | INTEGER |  |
| `success_total` | INTEGER |  |
| `fail_total` | INTEGER |  |
| `last_fail_at` | DATETIME |  |
| `last_used_at` | DATETIME |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_channel_key_url`

### `ai_channel_accounts`

账号级渠道授权（令牌密文与指纹）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `base_url` | TEXT |  |
| `label` | TEXT |  |
| `user_id` | TEXT |  |
| `access_token_enc` | TEXT | 密文 |
| `access_token_fp` | TEXT |  |
| `access_token_tail` | TEXT |  |
| `model_key_enc` | TEXT | 密文 |
| `model_key_fp` | TEXT |  |
| `model_key_tail` | TEXT |  |
| `enabled` | NUMERIC |  |
| `last_checked_at` | DATETIME |  |
| `last_ok` | NUMERIC |  |
| `last_models` | INTEGER |  |
| `last_balance` | REAL |  |
| `last_used` | REAL |  |
| `last_currency` | TEXT |  |
| `last_error` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_ai_channel_accounts_base_url`

### `ai_category_prices`

按内容类别的计费单价（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `kind` | TEXT | 主键 |
| `price_in` | INTEGER |  |
| `price_out` | INTEGER |  |
| `price_call` | INTEGER |  |
| `updated_at` | DATETIME | 时间 |

### `ai_prompts`

提示词（内置/用户自建）（行数 17）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `kind` | TEXT |  |
| `category` | TEXT |  |
| `title` | TEXT |  |
| `content` | TEXT |  |
| `builtin` | NUMERIC |  |
| `sort_order` | INTEGER |  |
| `used_count` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_ai_prompt_user`、`idx_ai_prompts_kind`

### `ai_prompt_templates`

提示词模板（场景、变量）（行数 25）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `key` | TEXT |  |
| `title` | TEXT |  |
| `scene` | TEXT |  |
| `category` | TEXT |  |
| `kind` | TEXT |  |
| `content` | TEXT |  |
| `variables` | TEXT |  |
| `version` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `priority` | INTEGER |  |
| `builtin` | NUMERIC |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_ai_prompt_templates_key`、`idx_ai_prompt_templates_scene`、`idx_ai_prompt_templates_category`、`idx_ai_prompt_templates_enabled`

### `ai_prompt_template_versions`

提示词模板版本历史（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `template_id` | INTEGER |  |
| `version` | INTEGER |  |
| `content` | TEXT |  |
| `variables` | TEXT |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_ai_prompt_template_versions_template_id`

### `ai_styles`

写作风格预设（行数 14）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `kind` | TEXT |  |
| `name` | TEXT |  |
| `sort_order` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_ai_styles_kind`

### `ai_call_logs`

每次模型调用：Token、费用、耗时、结果（行数 215）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `model_id` | INTEGER |  |
| `model_name` | TEXT |  |
| `action` | TEXT |  |
| `ref_type` | TEXT |  |
| `ref_id` | INTEGER |  |
| `tokens_in` | INTEGER |  |
| `tokens_out` | INTEGER |  |
| `cost` | INTEGER |  |
| `status` | TEXT |  |
| `price_snapshot` | TEXT |  |
| `free_quota` | NUMERIC |  |
| `error` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `settled_at` | DATETIME |  |
| `latency_ms` | INTEGER |  |

索引：`idx_ai_call_logs_model_id`、`idx_ai_call_logs_status`、`idx_ai_call_user`

### `ai_chat_sessions`

对话会话（用户/智能体/模型）（行数 101）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `agent_id` | INTEGER |  |
| `agent_name` | TEXT |  |
| `model_id` | INTEGER |  |
| `title` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `third_conv_id` | TEXT |  |
| `share_token` | TEXT |  |
| `shared_at` | DATETIME |  |
| `locked` | NUMERIC |  |
| `locked_at` | DATETIME |  |
| `lock_reason` | TEXT |  |
| `locked_by` | INTEGER |  |
| `contaminated` | NUMERIC |  |
| `contaminated_at` | DATETIME |  |
| `active_leaf_id` | INTEGER |  |

索引：`idx_ai_chat_sessions_contaminated`、`idx_ai_chat_sessions_locked`、`idx_ai_chat_sessions_agent_id`、`idx_ai_session_user`

### `ai_chat_messages`

对话消息与计费明细（行数 405）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `session_id` | INTEGER |  |
| `role` | TEXT |  |
| `content` | TEXT |  |
| `tokens_in` | INTEGER |  |
| `tokens_out` | INTEGER |  |
| `cost` | INTEGER |  |
| `call_log_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `parent_id` | INTEGER |  |
| `safety_blocked` | NUMERIC |  |
| `reasoning` | TEXT |  |

索引：`idx_ai_chat_messages_parent_id`、`idx_ai_msg_session`

### `ai_media_tasks`

图片/视频生成任务与参数（行数 15）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `kind` | TEXT |  |
| `model_id` | INTEGER |  |
| `model_name` | TEXT |  |
| `prompt` | TEXT |  |
| `params` | TEXT |  |
| `status` | TEXT |  |
| `remote_id` | TEXT |  |
| `result` | TEXT |  |
| `cost` | INTEGER |  |
| `call_log_id` | INTEGER |  |
| `error` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `attachments` | TEXT |  |
| `is_public` | NUMERIC |  |
| `title` | TEXT |  |
| `style` | TEXT |  |
| `likes` | INTEGER |  |
| `refunded` | NUMERIC |  |

索引：`idx_ai_media_tasks_is_public`、`idx_ai_media_tasks_style`、`idx_ai_media_user`、`idx_ai_media_tasks_kind`、`idx_ai_media_tasks_status`

### `ai_agents`

内置智能体（人设、开场白、启用状态）（行数 4）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `emoji` | TEXT |  |
| `description` | TEXT |  |
| `system_prompt` | TEXT |  |
| `greeting` | TEXT |  |
| `builtin` | NUMERIC |  |
| `enabled` | NUMERIC |  |
| `sort_order` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `category` | TEXT |  |
| `used_count` | INTEGER |  |
| `upstream` | TEXT |  |
| `bot_id` | TEXT |  |
| `coze_base` | TEXT |  |
| `token_enc` | TEXT | 密文 |
| `submitted_by` | INTEGER |  |
| `persona` | TEXT |  |
| `appearance` | TEXT |  |
| `speech` | TEXT |  |

索引：`idx_ai_agents_submitted_by`、`idx_ai_agents_category`

### `agent_submissions`

用户提交的智能体（待审核上架）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `user_name` | TEXT |  |
| `name` | TEXT |  |
| `emoji` | TEXT |  |
| `description` | TEXT |  |
| `greeting` | TEXT |  |
| `category` | TEXT |  |
| `system_prompt` | TEXT |  |
| `status` | TEXT |  |
| `review_note` | TEXT |  |
| `reviewer_id` | INTEGER |  |
| `reviewed_at` | DATETIME |  |
| `agent_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_agent_submissions_user_id`、`idx_agent_submissions_status`

### `ai_content_marks`

内容 AI 标识登记（目标类型 + 模型）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `target_type` | TEXT |  |
| `target_id` | INTEGER |  |
| `ai_model` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_ai_mark_target`

### `ai_judge_logs`

AI 审核判定日志（触发点、模型、结论）（行数 35）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `username` | TEXT |  |
| `session_id` | INTEGER |  |
| `turn` | INTEGER |  |
| `trigger` | TEXT |  |
| `model_id` | INTEGER |  |
| `model_name` | TEXT |  |
| `jailbroken` | NUMERIC |  |
| `score` | INTEGER |  |
| `categories` | TEXT |  |
| `reason` | TEXT |  |
| `blocked` | NUMERIC |  |
| `released` | NUMERIC |  |
| `latency_ms` | INTEGER |  |
| `failed` | NUMERIC |  |
| `raw_reply` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_ai_judge_logs_created_at`、`idx_ai_judge_logs_user_id`、`idx_ai_judge_logs_session_id`、`idx_ai_judge_logs_jailbroken`

### `ai_safety_configs`

三层安全闸配置与阈值（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `enabled` | NUMERIC |  |
| `guard_input` | NUMERIC |  |
| `guard_output` | NUMERIC |  |
| `input_block_score` | INTEGER |  |
| `output_block_score` | INTEGER |  |
| `review_score` | INTEGER |  |
| `watch_block_score` | INTEGER |  |
| `watch_review_score` | INTEGER |  |
| `prebuffer_chars` | INTEGER |  |
| `flush_chars` | INTEGER |  |
| `watch_after_hits` | INTEGER |  |
| `watch_hours` | INTEGER |  |
| `cooldown_hits` | INTEGER |  |
| `cooldown_minutes` | INTEGER |  |
| `score_decay_hours` | INTEGER |  |
| `scan_context_turns` | INTEGER |  |
| `skip_first_turn` | NUMERIC |  |
| `protect_system` | NUMERIC |  |
| `allow_fiction_relief` | NUMERIC |  |
| `allow_harm_relief` | NUMERIC |  |
| `record_excerpt` | NUMERIC |  |
| `excerpt_chars` | INTEGER |  |
| `system_guard_prompt` | TEXT |  |
| `refusal_text` | TEXT |  |
| `cooldown_text` | TEXT |  |
| `watch_notice_text` | TEXT |  |
| `extra_terms` | TEXT |  |
| `judge_enabled` | NUMERIC |  |
| `judge_model_id` | INTEGER |  |
| `judge_every_turns` | INTEGER |  |
| `judge_on_suspect` | NUMERIC |  |
| `judge_review_blocks` | NUMERIC |  |
| `judge_block_score` | INTEGER |  |
| `judge_release_score` | INTEGER |  |
| `judge_max_chars` | INTEGER |  |
| `judge_timeout_sec` | INTEGER |  |
| `judge_fail_open` | NUMERIC |  |
| `judge_max_turns` | INTEGER |  |
| `judge_prompt` | TEXT |  |
| `schema_version` | INTEGER |  |
| `updated_at` | DATETIME | 时间 |
| `refusal_jailbreak_text` | TEXT |  |
| `terminate_on_block` | NUMERIC |  |
| `terminate_score` | INTEGER |  |
| `terminate_mode` | TEXT |  |
| `session_terminated_text` | TEXT |  |
| `contaminated_notice_text` | TEXT |  |
| `moderate_blocked_text` | TEXT |  |
| `minor_crisis_enabled` | NUMERIC |  |
| `minor_crisis_hotline` | TEXT |  |
| `minor_crisis_comfort` | TEXT |  |
| `minor_observation_hours` | INTEGER |  |
| `minor_crisis_prompt` | TEXT |  |
| `guard_leak` | NUMERIC |  |
| `image_moderation_prompt` | TEXT |  |
| `image_gen_guard` | NUMERIC |  |
| `image_output_guard` | NUMERIC |  |
| `image_output_prompt` | TEXT |  |
| `appease_display_mode` | TEXT |  |
| `appease_replace_text` | TEXT |  |
| `appease_soft_card_text` | TEXT |  |
| `appease_escalate_score` | INTEGER |  |
| `appease_system_prompt` | TEXT |  |

### `ai_safety_events`

安全闸事件（方向、动作、等级、分类）（行数 41）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `username` | TEXT |  |
| `direction` | TEXT |  |
| `action` | TEXT |  |
| `level` | TEXT |  |
| `score` | INTEGER |  |
| `categories` | TEXT |  |
| `hits` | TEXT |  |
| `excerpt` | TEXT |  |
| `model_id` | INTEGER |  |
| `session_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_ai_safety_events_user_id`、`idx_ai_safety_events_direction`、`idx_ai_safety_events_action`、`idx_ai_safety_events_created_at`

### `ai_safety_profiles`

用户风险画像（评分、命中、冷却期）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `user_id` | INTEGER | 主键 |
| `username` | TEXT |  |
| `risk_score` | INTEGER |  |
| `block_count` | INTEGER |  |
| `hit_count` | INTEGER |  |
| `watch_until` | DATETIME |  |
| `cooldown_until` | DATETIME |  |
| `last_hit_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

### `ai_bot_usages`

机器人每日调用次数配额（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `day` | TEXT |  |
| `calls_used` | INTEGER |  |
| `updated_at` | DATETIME | 时间 |

索引：`idx_ai_bot_usage`

### `ai_novel_memories`

长篇记忆摘要与 RAG 切片（按章节）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `novel_id` | INTEGER |  |
| `kind` | TEXT |  |
| `chapter_id` | INTEGER |  |
| `content` | TEXT |  |
| `tokens` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_ai_memory`

## AI 写作 Agent

### `authorai_projects`

写作工程：设定、人物、时间线、大纲（与 AI 中台数据隔离）（行数 4）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `novel_id` | INTEGER |  |
| `title` | TEXT |  |
| `setting` | TEXT |  |
| `characters` | TEXT |  |
| `timeline` | TEXT |  |
| `outline` | TEXT |  |
| `style_preset` | TEXT |  |
| `style_detail` | TEXT |  |
| `body_draft` | TEXT |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `setting_input` | TEXT |  |
| `character_input` | TEXT |  |
| `outline_input` | TEXT |  |
| `timeline_input` | TEXT |  |
| `seed_topic` | TEXT |  |
| `storyline` | TEXT |  |
| `storyline_input` | TEXT |  |

索引：`idx_authorai_user`、`idx_authorai_projects_novel_id`

### `authorai_details`

工程内各模块条目（按 module 分组）（行数 5）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `project_id` | INTEGER |  |
| `module` | TEXT |  |
| `title` | TEXT |  |
| `content` | TEXT |  |
| `sort_order` | INTEGER |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_authorai_detail_project`、`idx_authorai_detail_module`

### `authorai_chapters`

生成的章节草稿与状态（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `project_id` | INTEGER |  |
| `order` | INTEGER |  |
| `title` | TEXT |  |
| `outline` | TEXT |  |
| `body` | TEXT |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_authorai_chapter_project`

## AIGC 合规

### `aigc_assets`

原始生成资产（图/视频/音频）（行数 7）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `task_id` | INTEGER |  |
| `source` | TEXT |  |
| `index` | INTEGER |  |
| `model_name` | TEXT |  |
| `file_path` | TEXT |  |
| `mime` | TEXT |  |
| `width` | INTEGER |  |
| `height` | INTEGER |  |
| `size_bytes` | INTEGER |  |
| `sha256` | TEXT |  |
| `from_url` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_aigc_assets_user_id`、`idx_aigc_assets_task_id`、`idx_aigc_assets_source`、`idx_aigc_assets_created_at`

### `aigc_deliveries`

交付副本：内容寻址键、标识结果、公开与命中数（行数 3）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `object_key` | TEXT |  |
| `produce_id` | TEXT |  |
| `source_hash` | TEXT | 哈希 |
| `file_path` | TEXT |  |
| `mime` | TEXT |  |
| `ext` | TEXT |  |
| `width` | INTEGER |  |
| `height` | INTEGER |  |
| `size_bytes` | INTEGER |  |
| `labeled` | NUMERIC |  |
| `public` | NUMERIC |  |
| `hits` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_aigc_deliveries_produce_id`、`idx_aigc_deliveries_source_hash`、`idx_aigc_deliveries_public`、`idx_aigc_deliveries_created_at`、`idx_aigc_deliveries_object_key`

### `aigc_delivery_grants`

取图授权（object_key + user_id 唯一）（行数 3）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `object_key` | TEXT |  |
| `user_id` | INTEGER |  |
| `task_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_aigc_grant`、`idx_aigc_delivery_grants_user_id`

### `aigc_exports`

导出记录：ProduceID 与模型名映射（国标编号）（行数 15）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `produce_id` | TEXT |  |
| `asset_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `model_name` | TEXT |  |
| `format` | TEXT |  |
| `width` | INTEGER |  |
| `height` | INTEGER |  |
| `waived` | NUMERIC |  |
| `labeled` | NUMERIC |  |
| `label_text` | TEXT |  |
| `label_font_size` | INTEGER |  |
| `content_producer` | TEXT |  |
| `reserved_code1` | TEXT |  |
| `prompt_attached` | NUMERIC |  |
| `aigc_json` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_aigc_exports_created_at`、`idx_aigc_exports_user_id`、`idx_aigc_exports_asset_id`、`idx_aigc_exports_produce_id`

### `aigc_waiver_logs`

显式标识豁免知情记录（原文哈希、协议版本、结果）（行数 4）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `user_name` | TEXT |  |
| `produce_id` | TEXT |  |
| `request_time` | DATETIME |  |
| `agreement_version` | TEXT |  |
| `input_text` | TEXT |  |
| `input_hash` | TEXT | 哈希 |
| `response_result` | TEXT |  |
| `deny_reason` | TEXT |  |
| `ip` | TEXT |  |
| `ua` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_aigc_waiver_logs_user_id`、`idx_aigc_waiver_logs_produce_id`、`idx_aigc_waiver_logs_created_at`

## 钱包与支付

### `token_ledger`

代币流水（类型、金额、变动后余额、业务引用）（行数 9）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `type` | TEXT |  |
| `amount` | INTEGER |  |
| `balance_after` | INTEGER |  |
| `ref_type` | TEXT |  |
| `ref_id` | INTEGER |  |
| `remark` | TEXT |  |
| `price_snapshot` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_token_ledger_user`、`idx_token_ledger_type`、`idx_token_ledger_ref_id`

### `user_token_balances`

代币余额（一人一行）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `user_id` | INTEGER | 主键 |
| `balance` | INTEGER |  |
| `updated_at` | DATETIME | 时间 |

### `recharge_orders`

充值订单（渠道、金额分、到账代币）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `order_no` | TEXT |  |
| `user_id` | INTEGER |  |
| `channel` | TEXT |  |
| `provider` | TEXT |  |
| `amount_fen` | INTEGER |  |
| `token_amount` | INTEGER |  |
| `status` | TEXT |  |
| `pay_url` | TEXT |  |
| `transaction_id` | TEXT |  |
| `expires_at` | DATETIME |  |
| `paid_at` | DATETIME |  |
| `notify_raw` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `member_days` | INTEGER |  |

索引：`idx_recharge_orders_user_id`、`idx_recharge_orders_status`、`idx_recharge_orders_transaction_id`、`idx_recharge_orders_expires_at`、`idx_recharge_orders_created_at`、`idx_recharge_orders_order_no`

### `withdrawal_requests`

提现申请与审核状态（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `amount` | REAL |  |
| `method` | TEXT |  |
| `account` | TEXT |  |
| `status` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_withdrawal_requests_user_id`

### `card_keys`

通用卡密（代币），存储哈希与前缀（行数 3）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `code_hash` | TEXT | 哈希 |
| `code_prefix` | TEXT |  |
| `value` | INTEGER |  |
| `status` | TEXT |  |
| `batch_id` | TEXT |  |
| `created_by` | INTEGER |  |
| `used_by` | INTEGER |  |
| `used_at` | DATETIME |  |
| `expires_at` | DATETIME |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_card_keys_code_hash`、`idx_card_keys_status`、`idx_card_keys_batch_id`

### `vip_cards`

VIP 卡密（等级、天数、批次）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `code_hash` | TEXT | 哈希 |
| `code_prefix` | TEXT |  |
| `level` | INTEGER |  |
| `days` | INTEGER |  |
| `status` | TEXT |  |
| `batch_id` | TEXT |  |
| `created_by` | INTEGER |  |
| `used_by` | INTEGER |  |
| `used_at` | DATETIME |  |
| `expires_at` | DATETIME |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_vip_cards_code_hash`、`idx_vip_cards_level`、`idx_vip_cards_status`、`idx_vip_cards_batch_id`

### `vip_applications`

VIP 申请与审核（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `status` | TEXT |  |
| `reason` | TEXT |  |
| `reviewed_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_vip_applications_user_id`

### `payment_risk_events`

支付风控事件（评分、决策、原因）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `order_no` | TEXT |  |
| `user_id` | INTEGER |  |
| `score` | INTEGER |  |
| `decision` | TEXT |  |
| `reasons` | TEXT |  |
| `payload` | TEXT |  |
| `status` | TEXT |  |
| `handled_by` | INTEGER |  |
| `handled_at` | DATETIME |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_payment_risk_events_order_no`、`idx_payment_risk_events_user_id`、`idx_payment_risk_events_score`、`idx_payment_risk_events_decision`、`idx_payment_risk_events_status`、`idx_payment_risk_events_created_at`

### `earnings_records`

作者收益流水（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `novel_id` | INTEGER |  |
| `amount` | REAL |  |
| `type` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_earnings_records_user_id`

## 存储与配额

### `content_blobs`

正文库内副本（chapter/blog），双模式存储的核心表（行数 2069）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `ref_type` | TEXT | 主键 |
| `ref_id` | INTEGER | 主键 |
| `content` | TEXT |  |
| `content_hash` | TEXT | 哈希 |
| `updated_at` | DATETIME | 时间 |

### `user_htmls`

用户上传的 HTML 作品（解压目录、入口文件）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `novel_id` | INTEGER |  |
| `name` | TEXT |  |
| `description` | TEXT |  |
| `extract_dir` | TEXT |  |
| `entry_file` | TEXT |  |
| `file_count` | INTEGER |  |
| `total_size` | INTEGER |  |
| `file_path` | TEXT |  |
| `zip_path` | TEXT |  |
| `port` | INTEGER |  |
| `project_name` | TEXT |  |
| `is_active` | NUMERIC |  |
| `allow_wasm` | NUMERIC |  |
| `is_public` | NUMERIC |  |
| `is_downloadable` | NUMERIC |  |
| `thumb_url` | TEXT |  |
| `review_status` | TEXT |  |
| `review_note` | TEXT |  |
| `scan_warnings` | TEXT |  |
| `run_mode` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |
| `is_extracted` | NUMERIC |  |
| `quarantine_path` | TEXT |  |
| `scan_blocked` | NUMERIC |  |
| `scan_block_reason` | TEXT |  |
| `reviewed_by` | INTEGER |  |
| `reviewed_at` | DATETIME |  |
| `blog_id` | INTEGER |  |
| `frame_width` | INTEGER |  |
| `frame_height` | INTEGER |  |

索引：`idx_user_htmls_blog_id`、`idx_user_htmls_user_id`、`idx_user_htmls_novel_id`、`idx_user_htmls_project_name`、`idx_user_htmls_review_status`、`idx_user_htmls_run_mode`、`idx_user_htmls_deleted_at`

### `html_quota_overrides`

HTML 空间配额覆盖（按用户）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `max_projects` | INTEGER |  |
| `max_total_size_mb` | INTEGER |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_html_quota_overrides_user_id`

### `user_web_quotas`

用户网站空间配额（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `quota` | INTEGER |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `quota_size_mb` | INTEGER |  |

索引：`idx_user_web_quotas_user_id`

### `web_quota_requests`

空间扩容申请与审批（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `target_type` | TEXT |  |
| `target_id` | INTEGER |  |
| `status` | TEXT |  |
| `reason` | TEXT |  |
| `review_note` | TEXT |  |
| `reviewed_by` | INTEGER |  |
| `reviewed_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_web_quota_requests_status`、`idx_web_quota_requests_user_id`、`idx_web_quota_requests_target_type`、`idx_web_quota_requests_target_id`

### `user_publish_quotas`

发布配额（篇数、字数上限）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `count` | INTEGER |  |
| `word_cap` | INTEGER |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_user_publish_quotas_user_id`

### `user_publish_logs`

发布流水（用于配额核算）（行数 5）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `kind` | TEXT |  |
| `word_count` | INTEGER |  |
| `target_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_publish_user_day`、`idx_user_publish_logs_kind`、`idx_user_publish_logs_target_id`

### `user_frames`

作品框/装扮（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `novel_id` | INTEGER |  |
| `name` | TEXT |  |
| `description` | TEXT |  |
| `file_path` | TEXT |  |
| `thumb_url` | TEXT |  |
| `is_active` | NUMERIC |  |
| `is_public` | NUMERIC |  |
| `tags` | TEXT |  |
| `version` | INTEGER |  |
| `has_controls` | NUMERIC |  |
| `uses_novel_api` | NUMERIC |  |
| `sandbox_level` | TEXT |  |
| `frame_type` | TEXT |  |
| `review_status` | TEXT |  |
| `review_note` | TEXT |  |
| `scan_warnings` | TEXT |  |
| `run_mode` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |
| `frame_width` | INTEGER |  |
| `frame_height` | INTEGER |  |
| `position` | TEXT |  |
| `blog_id` | INTEGER |  |

索引：`idx_user_frames_user_id`、`idx_user_frames_deleted_at`、`idx_user_frames_run_mode`、`idx_user_frames_review_status`、`idx_user_frames_frame_type`、`idx_user_frames_blog_id`、`idx_user_frames_novel_id`

### `user_ai_free_quotas`

AI 免费额度配置（每日次数与 Token）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `daily_calls` | INTEGER |  |
| `daily_tokens` | INTEGER |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_user_ai_free_quotas_user_id`

### `user_ai_free_usages`

AI 免费额度当日用量（行数 11）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `day` | TEXT |  |
| `calls_used` | INTEGER |  |
| `tokens_used` | INTEGER |  |
| `updated_at` | DATETIME | 时间 |

索引：`idx_ai_free_usage`

### `user_ai_keys`

用户自带模型密钥（密文存储）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `label` | TEXT |  |
| `base_url` | TEXT |  |
| `key_enc` | TEXT | 密文 |
| `model_id` | TEXT |  |
| `enabled` | NUMERIC |  |
| `is_default` | NUMERIC |  |
| `last_used_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_user_ai_keys_user_id`

## 安全与风控

### `bad_words`

敏感词库（匹配方式、处置动作、作用域）（行数 72）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `match_type` | TEXT |  |
| `pattern` | TEXT |  |
| `action` | TEXT |  |
| `replacement` | TEXT |  |
| `scope` | TEXT |  |
| `enabled` | NUMERIC |  |
| `description` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `category` | TEXT |  |
| `tags` | TEXT |  |

### `bad_word_flags`

敏感词命中标记与处置状态（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `content_type` | TEXT |  |
| `content_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `content_title` | TEXT |  |
| `snippet` | TEXT |  |
| `matched_words` | TEXT |  |
| `status` | TEXT |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_bad_word_flags_content_type`、`idx_bad_word_flags_user_id`、`idx_bad_word_flags_status`

### `xss_rules`

XSS 过滤规则（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `pattern` | TEXT |  |
| `action` | TEXT |  |
| `replacement` | TEXT |  |
| `enabled` | NUMERIC |  |
| `description` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

### `honeypot_sessions`

蜜罐会话（来源 IP、UA 归一、事件计数）（行数 16）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `h_id` | TEXT |  |
| `src_ip` | TEXT |  |
| `ua_norm` | TEXT |  |
| `referer` | TEXT |  |
| `xff` | TEXT |  |
| `country` | TEXT |  |
| `event_count` | INTEGER |  |
| `max_risk` | INTEGER |  |
| `banned` | NUMERIC |  |
| `first_seen` | DATETIME |  |
| `last_seen` | DATETIME |  |

索引：`idx_honeypot_sessions_last_seen`、`idx_honeypot_sessions_h_id`、`idx_honeypot_sessions_src_ip`、`idx_honeypot_sessions_max_risk`

### `honeypot_events`

蜜罐事件（诱饵、方法、路径、指纹）（行数 536）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `session_id` | INTEGER |  |
| `kind` | TEXT |  |
| `decoy` | TEXT |  |
| `method` | TEXT |  |
| `path` | TEXT |  |
| `query` | TEXT |  |
| `ua` | TEXT |  |
| `referer` | TEXT |  |
| `xff` | TEXT |  |
| `headers` | TEXT |  |
| `body_snippet` | TEXT |  |
| `risk` | INTEGER |  |
| `action` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_honeypot_events_session_id`、`idx_honeypot_events_kind`、`idx_honeypot_events_path`、`idx_honeypot_events_created_at`

### `honeypot_fingerprints`

蜜罐采集的浏览器指纹（canvas/WebGL/字体）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `session_id` | INTEGER |  |
| `canvas` | TEXT |  |
| `web_gl` | TEXT |  |
| `fonts` | TEXT |  |
| `tz` | TEXT |  |
| `lang` | TEXT |  |
| `screen` | TEXT |  |
| `dpr` | TEXT |  |
| `cores` | TEXT |  |
| `memory` | TEXT |  |
| `hints` | TEXT |  |
| `headless` | TEXT |  |
| `ja3` | TEXT |  |
| `ja4` | TEXT |  |
| `tls_source` | TEXT |  |
| `tls_ver` | TEXT |  |
| `web_rtc_ips` | TEXT |  |
| `raw` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_honeypot_fingerprints_session_id`、`idx_honeypot_fingerprints_ja3`、`idx_honeypot_fingerprints_created_at`

### `honeypot_settings`

蜜罐开关与采集粒度（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `enabled` | NUMERIC |  |
| `level` | TEXT |  |
| `record_ua` | NUMERIC |  |
| `record_referer` | NUMERIC |  |
| `record_xff` | NUMERIC |  |
| `record_query` | NUMERIC |  |
| `record_headers` | NUMERIC |  |
| `record_body_snippet` | NUMERIC |  |
| `body_snippet_limit` | INTEGER |  |
| `header_allowlist` | TEXT |  |
| `auto_ban` | NUMERIC |  |
| `ban_minutes` | INTEGER |  |
| `delay_ms` | INTEGER |  |
| `retention_days` | INTEGER |  |
| `updated_at` | DATETIME | 时间 |
| `annotation` | TEXT |  |
| `custom_decoys` | TEXT |  |
| `decoy_profile` | TEXT |  |

### `blacklist_ips`

IP 黑名单（含到期时间）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `ip` | TEXT |  |
| `reason` | TEXT |  |
| `expires_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_blacklist_ips_ip`

### `moderation_config`

机审配置（模型、触发时机、批量间隔）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `enabled` | NUMERIC |  |
| `model_id` | INTEGER |  |
| `novel_blog_on_write` | NUMERIC |  |
| `forum_comment_batch` | NUMERIC |  |
| `batch_interval_hours` | INTEGER |  |
| `auto_hide_on_fail` | NUMERIC |  |
| `malicious_threshold` | INTEGER |  |
| `recheck_report_count` | INTEGER |  |
| `stop_review_after` | INTEGER |  |
| `hide_from_reporter` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

### `moderation_records`

机审记录（范围、触发点、结论）（行数 19）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `scope` | TEXT |  |
| `target_type` | TEXT |  |
| `target_id` | INTEGER |  |
| `trigger` | TEXT |  |
| `model_id` | INTEGER |  |
| `model_name` | TEXT |  |
| `verdict` | TEXT |  |
| `compliant` | NUMERIC |  |
| `reason` | TEXT |  |
| `categories` | TEXT |  |
| `report_count` | INTEGER |  |
| `re_triggered` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |
| `review_status` | TEXT |  |
| `review_action` | TEXT |  |
| `reviewer_id` | INTEGER |  |
| `review_note` | TEXT |  |
| `reviewed_at` | DATETIME |  |

索引：`idx_moderation_records_review_status`、`idx_mod_target`

### `sentiment_configs`

舆情总开关与阈值（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `enabled` | NUMERIC |  |
| `block_risk` | TEXT |  |
| `flag_risk` | TEXT |  |
| `flood_block` | NUMERIC |  |
| `learn_from_ai` | NUMERIC |  |
| `search_enabled` | NUMERIC |  |
| `search_model_id` | INTEGER |  |
| `search_path` | TEXT |  |
| `search_query` | TEXT |  |
| `search_budget_per_day` | INTEGER |  |
| `recency_days` | INTEGER |  |
| `auto_update_hour` | INTEGER |  |
| `daily_quota` | INTEGER |  |
| `require_review` | NUMERIC |  |
| `min_confidence` | INTEGER |  |
| `fold_chars` | INTEGER |  |
| `schema_version` | INTEGER |  |
| `updated_at` | DATETIME | 时间 |

### `sentiment_terms`

舆情词库（类别、严重度、动作覆盖）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `word` | TEXT |  |
| `cat` | TEXT |  |
| `escalate_under` | TEXT |  |
| `severity` | INTEGER |  |
| `action_override` | TEXT |  |
| `source` | TEXT |  |
| `status` | TEXT |  |
| `hit_count` | INTEGER |  |
| `confidence` | INTEGER |  |
| `review_cycle_days` | INTEGER |  |
| `reviewed_at` | DATETIME |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_sentiment_terms_word`、`idx_sentiment_terms_cat`、`idx_sentiment_terms_source`、`idx_sentiment_terms_status`

### `sentiment_term_audits`

舆情词库变更审计（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `term_id` | INTEGER |  |
| `word` | TEXT |  |
| `action` | TEXT |  |
| `operator_id` | INTEGER |  |
| `operator` | TEXT |  |
| `note` | TEXT |  |
| `before` | TEXT |  |
| `after` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_sentiment_term_audits_term_id`、`idx_sentiment_term_audits_created_at`

### `sentiment_hits`

舆情命中记录与处置决策（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `content_type` | TEXT |  |
| `content_id` | INTEGER |  |
| `comment_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `username` | TEXT |  |
| `decision` | TEXT |  |
| `risk` | TEXT |  |
| `groups` | TEXT |  |
| `matched_words` | TEXT |  |
| `variant_words` | TEXT |  |
| `flood` | TEXT |  |
| `reason` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_sentiment_hits_content_id`、`idx_sentiment_hits_comment_id`、`idx_sentiment_hits_user_id`、`idx_sentiment_hits_decision`、`idx_sentiment_hits_risk`、`idx_sentiment_hits_groups`、`idx_sentiment_hits_created_at`、`idx_sentiment_hits_content_type`

### `sentiment_content_topics`

内容舆情画像（标签、风险、舆论分）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `content_type` | TEXT |  |
| `content_id` | INTEGER |  |
| `tags` | TEXT |  |
| `groups` | TEXT |  |
| `base_risk` | TEXT |  |
| `risk` | TEXT |  |
| `public_opinion_score` | INTEGER |  |
| `public_opinion_high` | NUMERIC |  |
| `source` | TEXT |  |
| `confidence` | INTEGER |  |
| `review_status` | TEXT |  |
| `policy` | TEXT |  |
| `reviewed_by` | INTEGER |  |
| `reviewed_at` | DATETIME |  |
| `updated_at` | DATETIME | 时间 |
| `created_at` | DATETIME | 时间 |

索引：`idx_sentiment_topic`、`idx_sentiment_content_topics_public_opinion_high`、`idx_sentiment_content_topics_review_status`

### `sentiment_camp_marks`

阵营标记（内容/评论维度的立场）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `content_type` | TEXT |  |
| `content_id` | INTEGER |  |
| `comment_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `camp_id` | TEXT |  |
| `camp_label` | TEXT |  |
| `position` | TEXT |  |
| `lang_format` | TEXT |  |
| `lang_evidence` | TEXT |  |
| `confidence` | INTEGER |  |
| `note` | TEXT |  |
| `source` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `status` | TEXT |  |

索引：`idx_sentiment_camp_marks_status`、`idx_sentiment_camp_marks_content_type`、`idx_sentiment_camp_marks_content_id`、`idx_sentiment_camp_marks_comment_id`、`idx_sentiment_camp_marks_user_id`、`idx_sentiment_camp_marks_camp_id`、`idx_sentiment_camp_marks_lang_format`、`idx_sentiment_camp_marks_created_at`

### `sentiment_camp_target_settings`

单个内容的阵营识别开关（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `content_type` | TEXT |  |
| `content_id` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `operator_id` | INTEGER |  |
| `operator` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_camp_target`

### `sentiment_delete_requests`

舆情触发的删除申请（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `action` | TEXT |  |
| `target_type` | TEXT |  |
| `target_id` | INTEGER |  |
| `target_label` | TEXT |  |
| `requester_id` | INTEGER |  |
| `requester` | TEXT |  |
| `reason` | TEXT |  |
| `before` | TEXT |  |
| `after` | TEXT |  |
| `status` | TEXT |  |
| `reviewer_id` | INTEGER |  |
| `reviewed_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_sentiment_delete_requests_status`、`idx_sentiment_delete_requests_created_at`、`idx_sentiment_delete_requests_action`、`idx_sentiment_delete_requests_target_type`、`idx_sentiment_delete_requests_target_id`、`idx_sentiment_delete_requests_requester_id`

### `api_keys`

API Key（哈希、前缀、角色、每分钟配额、IP 白名单）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `name` | TEXT |  |
| `role` | TEXT |  |
| `key_hash` | TEXT | 哈希 |
| `key_prefix` | TEXT |  |
| `quota_per_min` | INTEGER |  |
| `allowed_ips` | TEXT |  |
| `expires_at` | DATETIME |  |
| `last_used_at` | DATETIME |  |
| `is_active` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `total_calls` | INTEGER |  |
| `is_banned` | NUMERIC |  |
| `ban_reason` | TEXT |  |
| `banned_at` | DATETIME |  |
| `banned_by` | INTEGER |  |

索引：`idx_api_keys_user_id`、`idx_api_keys_key_hash`

### `api_key_alarms`

API Key 异常告警与确认（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `kind` | TEXT |  |
| `key_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `detail` | TEXT |  |
| `acked` | NUMERIC |  |
| `acked_by` | INTEGER |  |
| `acked_at` | DATETIME |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_api_key_alarms_kind`、`idx_api_key_alarms_key_id`、`idx_api_key_alarms_user_id`、`idx_api_key_alarms_acked`、`idx_api_key_alarms_created_at`

### `api_access_logs`

API 访问日志（身份、路径、状态码、IP）（行数 123）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `identity_type` | TEXT |  |
| `identity_id` | INTEGER |  |
| `key_prefix` | TEXT |  |
| `method` | TEXT |  |
| `path` | TEXT |  |
| `status_code` | INTEGER |  |
| `request_ip` | TEXT |  |
| `user_agent` | TEXT |  |
| `bot_reason` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_api_access_logs_identity_type`、`idx_api_access_logs_created_at`

### `user_api_policies`

用户限流策略与 Key 封禁状态（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `quota_per_min` | INTEGER |  |
| `window_secs` | INTEGER |  |
| `max_keys` | INTEGER |  |
| `keys_banned` | NUMERIC |  |
| `ban_reason` | TEXT |  |
| `ban_until` | DATETIME |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_user_api_policies_user_id`

### `rage_configs`

限流/长连接参数（批量间隔、并发、WS/MQTT 开关）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `enabled` | NUMERIC |  |
| `batch_interval_sec` | INTEGER |  |
| `batch_size` | INTEGER |  |
| `max_conns` | INTEGER |  |
| `updated_at` | DATETIME | 时间 |
| `ws_enabled` | NUMERIC |  |
| `mqtt_enabled` | NUMERIC |  |
| `mqtt_bind` | TEXT |  |
| `mqtt_port` | INTEGER |  |
| `grpc_enabled` | NUMERIC |  |
| `grpc_bind` | TEXT |  |
| `grpc_port` | INTEGER |  |

### `channel_balance_samples`

上游渠道余额采样（用于额度预警）（行数 1060）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `base_url` | TEXT |  |
| `remaining` | REAL |  |
| `used` | REAL |  |
| `granted` | REAL |  |
| `currency` | TEXT |  |
| `supported` | NUMERIC |  |
| `sampled_at` | DATETIME | 时间 |

索引：`idx_channel_balance_samples_base_url`、`idx_channel_balance_samples_sampled_at`

## 监管与处置

### `police_tickets`

警方工单（类型、优先级、状态、对象用户）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `no` | TEXT |  |
| `kind` | TEXT |  |
| `priority` | TEXT |  |
| `status` | TEXT |  |
| `title` | TEXT |  |
| `summary` | TEXT |  |
| `subject_user_id` | INTEGER |  |
| `subject_novel_id` | INTEGER |  |
| `subject_post_id` | INTEGER |  |
| `session_id` | INTEGER |  |
| `evidence` | TEXT |  |
| `requester_id` | INTEGER |  |
| `handler_id` | INTEGER |  |
| `due_at` | DATETIME |  |
| `closed_at` | DATETIME |  |
| `result` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_police_tickets_priority`、`idx_police_tickets_status`、`idx_police_tickets_subject_user_id`、`idx_police_tickets_requester_id`、`idx_police_tickets_handler_id`、`idx_police_tickets_no`、`idx_police_tickets_kind`

### `police_ticket_logs`

警方工单流转日志（状态变更）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `ticket_id` | INTEGER |  |
| `actor_id` | INTEGER |  |
| `action` | TEXT |  |
| `from_to` | TEXT |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_police_ticket_logs_ticket_id`、`idx_police_ticket_logs_created_at`

### `police_evidence_requests`

取证申请：申请→站长批准→导出（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `no` | TEXT |  |
| `requester_id` | INTEGER |  |
| `target_user_id` | INTEGER |  |
| `target_name` | TEXT |  |
| `reason` | TEXT |  |
| `status` | TEXT |  |
| `reviewer_id` | INTEGER |  |
| `review_note` | TEXT |  |
| `reviewed_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_police_evidence_requests_no`、`idx_police_evidence_requests_requester_id`、`idx_police_evidence_requests_target_user_id`、`idx_police_evidence_requests_status`

### `police_evidence_request_logs`

取证申请流转日志（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `evidence_request_id` | INTEGER |  |
| `actor_id` | INTEGER |  |
| `action` | TEXT |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_police_evidence_request_logs_evidence_request_id`

### `supervised_users`

隔离墙名单（等级、评分、来源、证据引用）（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `username` | TEXT |  |
| `tier` | TEXT |  |
| `score` | INTEGER |  |
| `source` | TEXT |  |
| `reasons` | TEXT |  |
| `evidence_refs` | TEXT |  |
| `status` | TEXT |  |
| `handled_by` | INTEGER |  |
| `handled_at` | DATETIME |  |
| `note` | TEXT |  |
| `first_seen` | DATETIME |  |
| `last_event` | DATETIME |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_supervised_users_status`、`idx_supervised_users_user_id`、`idx_supervised_users_tier`、`idx_supervised_users_source`

### `supervise_logs`

隔离墙处置日志（等级变更）（行数 9）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `supervised_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `actor_id` | INTEGER |  |
| `action` | TEXT |  |
| `from_to` | TEXT |  |
| `note` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_supervise_logs_supervised_id`、`idx_supervise_logs_user_id`、`idx_supervise_logs_created_at`

### `crisis_events`

心理危机事件与干预动作（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `username` | TEXT |  |
| `source` | TEXT |  |
| `content_type` | TEXT |  |
| `content_id` | INTEGER |  |
| `snippet` | TEXT |  |
| `action` | TEXT |  |
| `reason` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_crisis_events_source`、`idx_crisis_events_content_id`、`idx_crisis_user_created`

### `softdel_users`

软删除用户快照（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `original_id` | INTEGER |  |
| `deleted_by` | INTEGER |  |
| `operator` | TEXT |  |
| `reason` | TEXT |  |
| `source` | TEXT |  |
| `summary` | TEXT |  |
| `snapshot` | TEXT |  |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_softdel_users_deleted_by`、`idx_softdel_users_original_id`

### `softdel_novels`

软删除作品快照（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `original_id` | INTEGER |  |
| `deleted_by` | INTEGER |  |
| `operator` | TEXT |  |
| `reason` | TEXT |  |
| `source` | TEXT |  |
| `summary` | TEXT |  |
| `snapshot` | TEXT |  |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_softdel_novels_original_id`、`idx_softdel_novels_deleted_by`

### `softdel_chapters`

软删除章节快照（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `original_id` | INTEGER |  |
| `deleted_by` | INTEGER |  |
| `operator` | TEXT |  |
| `reason` | TEXT |  |
| `source` | TEXT |  |
| `summary` | TEXT |  |
| `snapshot` | TEXT |  |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_softdel_chapters_original_id`、`idx_softdel_chapters_deleted_by`

### `softdel_blogs`

软删除博客快照（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `original_id` | INTEGER |  |
| `deleted_by` | INTEGER |  |
| `operator` | TEXT |  |
| `reason` | TEXT |  |
| `source` | TEXT |  |
| `summary` | TEXT |  |
| `snapshot` | TEXT |  |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_softdel_blogs_original_id`、`idx_softdel_blogs_deleted_by`

### `softdel_posts`

软删除帖子快照（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `original_id` | INTEGER |  |
| `deleted_by` | INTEGER |  |
| `operator` | TEXT |  |
| `reason` | TEXT |  |
| `source` | TEXT |  |
| `summary` | TEXT |  |
| `snapshot` | TEXT |  |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_softdel_posts_original_id`、`idx_softdel_posts_deleted_by`

### `softdel_comments`

软删除评论快照（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `original_id` | INTEGER |  |
| `deleted_by` | INTEGER |  |
| `operator` | TEXT |  |
| `reason` | TEXT |  |
| `source` | TEXT |  |
| `summary` | TEXT |  |
| `snapshot` | TEXT |  |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_softdel_comments_original_id`、`idx_softdel_comments_deleted_by`

### `softdel_threads`

软删除主题快照（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `original_id` | INTEGER |  |
| `deleted_by` | INTEGER |  |
| `operator` | TEXT |  |
| `reason` | TEXT |  |
| `source` | TEXT |  |
| `summary` | TEXT |  |
| `snapshot` | TEXT |  |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_softdel_threads_original_id`、`idx_softdel_threads_deleted_by`

### `softdel_forums`

软删除版块快照（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `original_id` | INTEGER |  |
| `deleted_by` | INTEGER |  |
| `operator` | TEXT |  |
| `reason` | TEXT |  |
| `source` | TEXT |  |
| `summary` | TEXT |  |
| `snapshot` | TEXT |  |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_softdel_forums_original_id`、`idx_softdel_forums_deleted_by`

### `emergency_audit_logs`

紧急操作（封站/解锁等）审计（行数 15）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `operator_id` | INTEGER |  |
| `operator` | TEXT |  |
| `ip` | TEXT |  |
| `action` | TEXT |  |
| `target_type` | TEXT |  |
| `target_id` | INTEGER |  |
| `detail` | TEXT |  |
| `created_at` | DATETIME | 时间 |

### `vulnerabilities`

漏洞登记（严重度、影响面、修复时间）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `description` | TEXT |  |
| `severity` | TEXT |  |
| `status` | TEXT |  |
| `affected` | TEXT |  |
| `reported_by` | TEXT |  |
| `fixed_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

## 站点运营

### `platform_configs`

平台配置键值（storage_mode、阅读权限、开关等）（行数 68）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `key` | TEXT |  |
| `value` | TEXT |  |
| `updated_at` | DATETIME | 时间 |

索引：`idx_platform_configs_key`

### `system_settings`

系统设置键值（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `key` | TEXT | 主键 |
| `value` | TEXT |  |
| `updated_at` | DATETIME | 时间 |

### `announcements`

站内公告（置顶、发布状态）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `title` | TEXT |  |
| `content` | TEXT |  |
| `pinned` | NUMERIC |  |
| `published` | NUMERIC |  |
| `created_by` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |

### `carousels`

首页轮播图（投放时间、目标）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `title` | TEXT |  |
| `image_url` | TEXT |  |
| `link_url` | TEXT |  |
| `target` | TEXT |  |
| `sort_order` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `start_at` | DATETIME |  |
| `end_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

### `nav_menus`

导航菜单（层级、图标、可见性）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `parent_id` | INTEGER |  |
| `title` | TEXT |  |
| `icon` | TEXT |  |
| `url` | TEXT |  |
| `target` | TEXT |  |
| `sort_order` | INTEGER |  |
| `visible` | NUMERIC |  |
| `require_login` | NUMERIC |  |
| `admin_only` | NUMERIC |  |
| `badge` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_nav_menus_parent_id`、`idx_nav_menus_sort_order`

### `home_blocks`

首页区块（类型、配置 JSON、排序）（行数 6）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `type` | TEXT |  |
| `title` | TEXT |  |
| `config` | TEXT |  |
| `sort_order` | INTEGER |  |
| `enabled` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `deleted_at` | DATETIME | 软删除 |

索引：`idx_home_blocks_type`、`idx_home_blocks_sort_order`、`idx_home_blocks_deleted_at`

### `site_themes`

全站主题：CSS、自定义 JS 与加载模式（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `css` | TEXT |  |
| `updated_at` | DATETIME | 时间 |
| `custom_js` | TEXT |  |
| `custom_js_mode` | TEXT |  |
| `custom_js_delay` | INTEGER |  |
| `settings` | TEXT |  |

### `user_themes`

用户个人主题覆盖（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `css` | TEXT |  |
| `updated_at` | DATETIME | 时间 |
| `custom_js` | TEXT |  |
| `custom_js_mode` | TEXT |  |
| `custom_js_delay` | INTEGER |  |

索引：`idx_user_themes_user_id`

### `search_logs`

站内搜索日志（关键词、结果数、来源）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `query` | TEXT |  |
| `user_id` | INTEGER |  |
| `result_count` | INTEGER |  |
| `source` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_search_logs_query`、`idx_search_logs_user_id`、`idx_search_logs_created_at`

## 访问统计

### `web_access_logs`

全站访问日志（UA 族、路径、IP、身份）（行数 59557）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `identity_type` | TEXT |  |
| `ua_family` | TEXT |  |
| `method` | TEXT |  |
| `path` | TEXT |  |
| `query` | TEXT |  |
| `ip` | TEXT |  |
| `x_forwarded_for` | TEXT |  |
| `proxy_hops` | INTEGER |  |
| `ua` | TEXT |  |
| `status_code` | INTEGER |  |
| `size` | INTEGER |  |
| `latency_ms` | INTEGER |  |
| `is_bot` | NUMERIC |  |
| `bot_reason` | TEXT |  |
| `created_at` | DATETIME | 时间 |
| `os_name` | TEXT |  |
| `os_version` | TEXT |  |
| `browser_name` | TEXT |  |
| `browser_version` | TEXT |  |
| `device_type` | TEXT |  |
| `device_model` | TEXT |  |
| `host` | TEXT |  |
| `scheme` | TEXT |  |

索引：`idx_web_access_logs_host`、`idx_web_access_logs_device_type`、`idx_web_access_logs_browser_name`、`idx_web_access_logs_os_name`、`idx_web_access_logs_path`、`idx_web_access_logs_ip`、`idx_web_access_logs_status_code`、`idx_web_access_logs_is_bot`、`idx_web_access_logs_created_at`、`idx_web_access_logs_identity_type`、`idx_web_access_logs_ua_family`

### `zone_access_logs`

分区进入日志（zone、IP、UA）（行数 5）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `user_id` | INTEGER |  |
| `zone` | TEXT |  |
| `entered_at` | DATETIME |  |
| `ip` | TEXT |  |
| `user_agent` | TEXT |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_zone_access_user_zone`、`idx_zone_access_logs_entered_at`

## 联邦

### `federated_sites`

联邦站点登记（API 地址、状态、同步时间）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `url` | TEXT |  |
| `api_url` | TEXT |  |
| `description` | TEXT |  |
| `status` | TEXT |  |
| `last_sync_at` | DATETIME |  |
| `novel_count` | INTEGER |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_federated_sites_status`

### `federated_novels`

联邦作品缓存（远端 ID 映射）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `site_id` | INTEGER |  |
| `remote_id` | INTEGER |  |
| `title` | TEXT |  |
| `category` | TEXT |  |
| `author` | TEXT |  |
| `summary` | TEXT |  |
| `cover_url` | TEXT |  |
| `source_url` | TEXT |  |
| `cached_at` | DATETIME |  |
| `kind` | TEXT |  |
| `proxy_path` | TEXT |  |
| `origin_site` | TEXT |  |
| `source_chain` | TEXT |  |
| `hop_count` | INTEGER |  |

索引：`idx_fed_kind`、`idx_federated_novels_remote`、`idx_federated_novels_site`、`idx_federated_novels_site_id`、`idx_federated_novels_remote_id`

### `federated_peer_keys`

联邦对端密钥（方向、哈希、密文）（行数 0）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `site_id` | INTEGER |  |
| `direction` | TEXT |  |
| `peer_name` | TEXT |  |
| `key_hash` | TEXT | 哈希 |
| `key_prefix` | TEXT |  |
| `key_cipher` | TEXT |  |
| `issuer_name` | TEXT |  |
| `issuer_url` | TEXT |  |
| `quota_per_min` | INTEGER |  |
| `allowed_ips` | TEXT |  |
| `expires_at` | DATETIME |  |
| `last_used_at` | DATETIME |  |
| `is_active` | NUMERIC |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |
| `total_calls` | INTEGER |  |

索引：`idx_federated_peer_keys_site_id`、`idx_federated_peer_keys_direction`、`idx_federated_peer_keys_key_hash`

### `federated_inbound`

入站请求统计（来源、端点、计数）（行数 17）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `name` | TEXT |  |
| `url` | TEXT |  |
| `ip` | TEXT |  |
| `user_agent` | TEXT |  |
| `endpoint` | TEXT |  |
| `count` | INTEGER |  |
| `first_seen` | DATETIME |  |
| `last_seen` | DATETIME |  |
| `key_prefix` | TEXT |  |
| `issuer` | TEXT |  |

索引：`idx_federated_inbound_ip`

### `polls`

议题投票（载体类型/ID、作者、状态）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `target_kind` | TEXT |  |
| `target_id` | INTEGER |  |
| `author_id` | INTEGER |  |
| `question` | TEXT |  |
| `description` | TEXT |  |
| `multiple` | NUMERIC |  |
| `anonymous` | NUMERIC |  |
| `status` | TEXT |  |
| `closed_at` | DATETIME |  |
| `created_at` | DATETIME | 时间 |
| `updated_at` | DATETIME | 时间 |

索引：`idx_polls_status`、`idx_poll_target`、`idx_polls_author_id`

### `poll_options`

投票选项（标签、排序、票数）（行数 2）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `poll_id` | INTEGER |  |
| `label` | TEXT |  |
| `sort` | INTEGER |  |
| `votes` | INTEGER |  |

索引：`idx_poll_options_poll_id`

### `poll_votes`

用户投票记录（用户/选项，可覆盖）（行数 1）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 主键 |
| `poll_id` | INTEGER |  |
| `option_id` | INTEGER |  |
| `user_id` | INTEGER |  |
| `created_at` | DATETIME | 时间 |

索引：`idx_poll_vote`、`idx_poll_votes_option_id`
