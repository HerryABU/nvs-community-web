# -*- coding: utf-8 -*-
"""从实际库结构 + 源码模型生成数据表文档。

产出：
  docs/DATABASE.md        总览：分域表清单（表名 / 用途 / 行数 / 主要字段）
  docs/DATABASE-SCHEMA.md 逐表字段明细（自动生成，勿手改）
"""
import json
import os
import re
import sqlite3
import subprocess
import sys

ROOT = r"C:\Users\QBZ95\Desktop\tools\codes\nvs"


def load_schema(db_path):
    """直接从 SQLite 读表结构（表 / 字段 / 索引 / 行数）。

    以前是从 C:\\tmp\\schema.json 快照读，容易忘记刷新导致新增表进不了文档；
    现在以「库即为真相」，参数给哪个库就读哪个库。
    """
    con = sqlite3.connect(db_path)
    schema = {}
    names = [
        r[0]
        for r in con.execute(
            "select name from sqlite_master where type='table' and name not like 'sqlite_%' order by name"
        )
    ]
    for name in names:
        cols = []
        for cid, cname, ctype, notnull, _dflt, pk in con.execute(f'PRAGMA table_info("{name}")'):
            cols.append({"n": cname, "t": (ctype or "").upper(), "nn": bool(notnull), "pk": pk})
        idx = []
        for row in con.execute(f'PRAGMA index_list("{name}")'):
            idx.append(row[1])
        try:
            rows = con.execute(f'select count(*) from "{name}"').fetchone()[0]
        except sqlite3.Error:
            rows = 0
        schema[name] = {"cols": cols, "rows": rows, "idx": idx}
    con.close()
    return schema


SCHEMA = {}

# ---------------------------------------------------------------- 域划分
DOMAINS = [
    ("账号与用户", "注册、登录、审计、关注、私信、通知、黑名单", [
        "users", "user_audit_logs", "refresh_states", "checkin_records", "follows",
        "messages", "notifications", "blacklists", "hidden_contents", "reports",
        "malicious_reporters", "gate_device_bindings", "gate_invites",
    ]),
    ("小说与创作", "作品、章节、版本、协作、角色卡、阅读进度、评分、购买", [
        "novels", "novel_categories", "chapters", "chapter_versions", "chapter_branches",
        "novel_collaborators", "novel_collab_audits", "novel_characters", "novel_documents",
        "novel_likes", "novel_indexes", "bookshelf", "reading_notes", "ratings",
        "content_purchases", "comments",
    ]),
    ("作者博客", "博客文章、分类、标签、话题、点赞收藏、作者主页主题", [
        "author_blogs", "blog_categories", "blog_tags", "blog_tag_links", "blog_topics",
        "blog_topic_links", "blog_likes", "blog_favorites", "author_themes",
        "author_home_customize", "user_home_customize",
    ]),
    ("内容流（发现）", "图文/视频/音乐/微文动态、评论、点赞收藏、标签倒排", [
        "feed_posts", "feed_comments", "feed_comment_likes", "feed_likes",
        "feed_favorites", "feed_tags", "feed_danmaku",
    ]),
    ("论坛", "版块、主题、楼层、禁言与建版配额", [
        "forums", "threads", "posts", "forum_mutes", "user_forum_quotas",
    ]),
    ("群组", "群组、成员、群帖、群聊、群文件、已读位置、建群配额", [
        "groups", "group_members", "group_posts", "group_post_replies",
        "group_chat_messages", "group_files", "group_read_states", "user_group_quotas",
    ]),
    ("OC 角色扮演", "OC 角色卡、世界书、创作项目、AI 群组与剧情流", [
        "oc_characters", "oc_character_arts", "oc_emotes", "oc_world_books", "oc_world_entries",
        "oc_world_links", "oc_projects", "oc_project_members", "oc_project_access", "oc_groups",
        "oc_group_settings", "oc_group_members", "oc_agents", "oc_flows", "oc_turns",
        "oc_windows", "oc_budget_usage",
    ]),
    ("自托管", "运行时版本目录、源码/编译产物项目、审核留痕、反向代理路由", [
        "selfhost_runtimes", "selfhost_projects", "selfhost_review_logs", "selfhost_routes",
    ]),
    ("AI 中台", "厂商、模型、密钥池、提示词、调用计费、会话、媒体验成、安全闸", [
        "ai_vendors", "ai_models", "ai_model_keys", "ai_channel_keys", "ai_channel_accounts",
        "ai_category_prices", "ai_prompts", "ai_prompt_templates", "ai_prompt_template_versions",
        "ai_styles", "ai_call_logs", "ai_chat_sessions", "ai_chat_messages", "ai_media_tasks",
        "ai_agents", "agent_submissions", "ai_content_marks", "ai_judge_logs",
        "ai_safety_configs", "ai_safety_events", "ai_safety_profiles", "ai_bot_usages",
        "ai_novel_memories",
    ]),
    ("AI 写作 Agent", "写作工程（设定/人物/时间线/大纲）与生成章节草稿", [
        "authorai_projects", "authorai_details", "authorai_chapters",
    ]),
    ("AIGC 合规", "原图资产、内容寻址交付副本、取图授权、导出与豁免留痕", [
        "aigc_assets", "aigc_deliveries", "aigc_delivery_grants", "aigc_exports",
        "aigc_waiver_logs",
    ]),
    ("钱包与支付", "代币流水余额、充值提现、卡密、VIP、支付风控、作者收益", [
        "token_ledger", "user_token_balances", "recharge_orders", "withdrawal_requests",
        "card_keys", "vip_cards", "vip_applications", "payment_risk_events", "earnings_records",
    ]),
    ("存储与配额", "正文库内副本、HTML 作品空间、发布/AI 额度", [
        "content_blobs", "user_htmls", "html_quota_overrides", "user_web_quotas",
        "web_quota_requests", "user_publish_quotas", "user_publish_logs", "user_frames",
        "user_ai_free_quotas", "user_ai_free_usages", "user_ai_keys",
    ]),
    ("安全与风控", "敏感词、XSS、蜜罐、机审、舆情、API Key 限流与告警", [
        "bad_words", "bad_word_flags", "xss_rules", "honeypot_sessions", "honeypot_events",
        "honeypot_fingerprints", "honeypot_settings", "blacklist_ips", "moderation_config",
        "moderation_records", "sentiment_configs", "sentiment_terms", "sentiment_term_audits",
        "sentiment_hits", "sentiment_content_topics", "sentiment_camp_marks",
        "sentiment_camp_target_settings", "sentiment_delete_requests", "api_keys",
        "api_key_alarms", "api_access_logs", "user_api_policies", "rage_configs",
        "channel_balance_samples",
    ]),
    ("监管与处置", "警方取证、隔离墙、心理危机、软删除快照、紧急操作审计、漏洞登记", [
        "police_tickets", "police_ticket_logs", "police_evidence_requests",
        "police_evidence_request_logs", "supervised_users", "supervise_logs", "crisis_events",
        "softdel_users", "softdel_novels", "softdel_chapters", "softdel_blogs",
        "softdel_posts", "softdel_comments", "softdel_threads", "softdel_forums",
        "emergency_audit_logs", "vulnerabilities",
    ]),
    ("站点运营", "平台配置、公告、轮播、导航、首页区块、主题、搜索日志", [
        "platform_configs", "system_settings", "announcements", "carousels", "nav_menus",
        "home_blocks", "site_themes", "user_themes", "search_logs",
    ]),
    ("访问统计", "全站访问日志与分区进入日志", [
        "web_access_logs", "zone_access_logs",
    ]),
    ("联邦", "联邦站点、作品缓存、对端密钥、入站请求", [
        "federated_sites", "federated_novels", "federated_peer_keys", "federated_inbound",
    ]),
]

# ---------------------------------------------------------------- 用途
PURPOSE = {
    # 账号与用户
    "users": "账号主体：邮箱/密码哈希、昵称头像、角色、会员等级、状态",
    "user_audit_logs": "用户操作审计：登录、改密、提权等，含 IP/UA 与结果",
    "refresh_states": "当前有效的 refresh token 版本，用于强制下线",
    "checkin_records": "每日签到与连签奖励",
    "follows": "关注关系，is_special 标记特别关注",
    "messages": "私信：撤回标记 + 收发双向删除标记",
    "notifications": "站内通知（评论/点赞/关注/系统）",
    "blacklists": "用户互拉黑名单",
    "hidden_contents": "用户主动隐藏的内容",
    "reports": "内容举报工单与处置状态",
    "malicious_reporters": "恶意举报统计与举报封禁",
    "gate_device_bindings": "注册闸门：设备指纹与账号的一次性绑定",
    "gate_invites": "注册邀请码（次数、有效期）",
    # 小说
    "novels": "作品主体：作者、分类、标签、封面、价格、阅读权限",
    "novel_categories": "作品与分类的多对多关联",
    "chapters": "章节元数据（正文在文件或 content_blobs）",
    "chapter_versions": "章节版本快照，支持回溯与合并",
    "chapter_branches": "章节分支（多结局/草稿线）及分支头版本",
    "novel_collaborators": "协作者与权限位（发布/审核/改他人稿）",
    "novel_collab_audits": "协作操作审计（前后值快照）",
    "novel_characters": "作品角色卡（阵营、简介、配色）",
    "novel_documents": "设定集/世界观文档，可控制可见性",
    "novel_likes": "作品点赞",
    "novel_indexes": "index.json 的库内副本（与文件同字节 + Hash）",
    "bookshelf": "书架与阅读进度（最后阅读章节）",
    "reading_notes": "划线笔记与批注（含锚点定位）",
    "ratings": "多维度评分：完成度/叙事/思想/社区口碑/更新稳定",
    "content_purchases": "付费章节购买记录（幂等发放阅读权限）",
    "comments": "小说/博客评论，支持引文与偏移定位",
    # 博客
    "author_blogs": "博客文章元数据（正文在文件或 content_blobs）",
    "blog_categories": "博客分类，含审核状态与驳回原因",
    "blog_tags": "博客标签字典",
    "blog_tag_links": "博客与标签的关联",
    "blog_topics": "博客话题（专题聚合）",
    "blog_topic_links": "博客与话题的关联",
    "blog_likes": "博客点赞",
    "blog_favorites": "博客收藏",
    "author_themes": "作者主页主题：CSS 与博客样式",
    "author_home_customize": "作者主页自定义 HTML（审核后公开）",
    "user_home_customize": "用户个人主页自定义 HTML",
    # 内容流
    "feed_posts": "动态主体：图文/视频/音乐/微文，含封面与媒体地址",
    "feed_comments": "动态评论，parent_id 支持楼中楼",
    "feed_comment_likes": "评论点赞",
    "feed_likes": "动态点赞",
    "feed_favorites": "动态收藏",
    "feed_tags": "标签倒排索引（唯一索引 post_id+tag）",
    "feed_danmaku": "视频弹幕（按 time_ms 绑时间轴，独立于评论计数）",
    # 论坛
    "forums": "版块：类型、分区、关联对象、层级",
    "threads": "主题帖：置顶/锁定/浏览量",
    "posts": "楼层回复，parent_id/root_id 组织楼中楼",
    "forum_mutes": "版块禁言（到期时间、管理员封禁标记）",
    "user_forum_quotas": "用户建版配额",
    # 群组
    "groups": "群组主体：群号、分类、可见性、加入方式",
    "group_members": "群成员与角色、禁言到期",
    "group_posts": "群内帖子（可 AI 辅助）",
    "group_post_replies": "群帖回复",
    "group_chat_messages": "群聊消息（支持引用回复）",
    "group_files": "群文件（SHA-256 去重 + 存储路径）",
    "group_read_states": "群已读位置",
    "user_group_quotas": "用户建群配额",
    # OC
    "oc_characters": "OC 角色卡：人设、外观、口吻、场景",
    "oc_character_arts": "角色形象图库（立绘/表情/设定参考/场景 + 强调特征）",
    # 自托管
    "selfhost_runtimes": "自托管运行时版本目录（语言/版本/编译型或解释型，站长维护）",
    "selfhost_projects": "自托管项目（源码模式/编译版、源码与可执行文件、upstream、审核状态）",
    "selfhost_review_logs": "自托管项目审核留痕（提交/通过/驳回/停止/恢复）",
    "selfhost_routes": "自托管反向代理路由（前缀最长匹配、upstream、启停与超时）",
    "oc_emotes": "OC 表情包",
    "oc_world_books": "世界书（扫描深度、Token 预算）",
    "oc_world_entries": "世界书条目（关键词、二次关键词、逻辑）",
    "oc_world_links": "世界书之间的引用链",
    "oc_projects": "OC 创作项目（可见性、访问密码）",
    "oc_project_members": "项目成员与角色",
    "oc_project_access": "项目访问令牌（带过期时间）",
    "oc_groups": "OC 群组（AI 角色群聊）",
    "oc_group_settings": "OC 群审计等级、窗口大小与时机",
    "oc_group_members": "群内绑定的 OC 角色",
    "oc_agents": "群内 AI 角色绑定的模型/模板/策略/预算",
    "oc_flows": "剧情流：一次连续演出（排期、轮次）",
    "oc_turns": "剧情轮次发言（角色/代理/用户）",
    "oc_windows": "剧情窗口审核结果（区间、等级）",
    "oc_budget_usage": "OC 群每日预算消耗",
    # AI 中台
    "ai_vendors": "上游厂商标识与排序",
    "ai_models": "模型注册：类型、单价、base_url、密钥密文",
    "ai_model_keys": "模型多密钥池（权重、失败计数、熔断）",
    "ai_channel_keys": "上游渠道密钥池（权重、失败次数）",
    "ai_channel_accounts": "账号级渠道授权（令牌密文与指纹）",
    "ai_category_prices": "按内容类别的计费单价",
    "ai_prompts": "提示词（内置/用户自建）",
    "ai_prompt_templates": "提示词模板（场景、变量）",
    "ai_prompt_template_versions": "提示词模板版本历史",
    "ai_styles": "写作风格预设",
    "ai_call_logs": "每次模型调用：Token、费用、耗时、结果",
    "ai_chat_sessions": "对话会话（用户/智能体/模型）",
    "ai_chat_messages": "对话消息与计费明细",
    "ai_media_tasks": "图片/视频生成任务与参数",
    "ai_agents": "内置智能体（人设、开场白、启用状态）",
    "agent_submissions": "用户提交的智能体（待审核上架）",
    "ai_content_marks": "内容 AI 标识登记（目标类型 + 模型）",
    "ai_judge_logs": "AI 审核判定日志（触发点、模型、结论）",
    "ai_safety_configs": "三层安全闸配置与阈值",
    "ai_safety_events": "安全闸事件（方向、动作、等级、分类）",
    "ai_safety_profiles": "用户风险画像（评分、命中、冷却期）",
    "ai_bot_usages": "机器人每日调用次数配额",
    "ai_novel_memories": "长篇记忆摘要与 RAG 切片（按章节）",
    # AI 写作
    "authorai_projects": "写作工程：设定、人物、时间线、大纲（与 AI 中台数据隔离）",
    "authorai_details": "工程内各模块条目（按 module 分组）",
    "authorai_chapters": "生成的章节草稿与状态",
    # AIGC
    "aigc_assets": "原始生成资产（图/视频/音频）",
    "aigc_deliveries": "交付副本：内容寻址键、标识结果、公开与命中数",
    "aigc_delivery_grants": "取图授权（object_key + user_id 唯一）",
    "aigc_exports": "导出记录：ProduceID 与模型名映射（国标编号）",
    "aigc_waiver_logs": "显式标识豁免知情记录（原文哈希、协议版本、结果）",
    # 钱包
    "token_ledger": "代币流水（类型、金额、变动后余额、业务引用）",
    "user_token_balances": "代币余额（一人一行）",
    "recharge_orders": "充值订单（渠道、金额分、到账代币）",
    "withdrawal_requests": "提现申请与审核状态",
    "card_keys": "通用卡密（代币），存储哈希与前缀",
    "vip_cards": "VIP 卡密（等级、天数、批次）",
    "vip_applications": "VIP 申请与审核",
    "payment_risk_events": "支付风控事件（评分、决策、原因）",
    "earnings_records": "作者收益流水",
    # 存储与配额
    "content_blobs": "正文库内副本（chapter/blog），双模式存储的核心表",
    "user_htmls": "用户上传的 HTML 作品（解压目录、入口文件）",
    "html_quota_overrides": "HTML 空间配额覆盖（按用户）",
    "user_web_quotas": "用户网站空间配额",
    "web_quota_requests": "空间扩容申请与审批",
    "user_publish_quotas": "发布配额（篇数、字数上限）",
    "user_publish_logs": "发布流水（用于配额核算）",
    "user_frames": "作品框/装扮",
    "user_ai_free_quotas": "AI 免费额度配置（每日次数与 Token）",
    "user_ai_free_usages": "AI 免费额度当日用量",
    "user_ai_keys": "用户自带模型密钥（密文存储）",
    # 安全
    "bad_words": "敏感词库（匹配方式、处置动作、作用域）",
    "bad_word_flags": "敏感词命中标记与处置状态",
    "xss_rules": "XSS 过滤规则",
    "honeypot_sessions": "蜜罐会话（来源 IP、UA 归一、事件计数）",
    "honeypot_events": "蜜罐事件（诱饵、方法、路径、指纹）",
    "honeypot_fingerprints": "蜜罐采集的浏览器指纹（canvas/WebGL/字体）",
    "honeypot_settings": "蜜罐开关与采集粒度",
    "blacklist_ips": "IP 黑名单（含到期时间）",
    "moderation_config": "机审配置（模型、触发时机、批量间隔）",
    "moderation_records": "机审记录（范围、触发点、结论）",
    "sentiment_configs": "舆情总开关与阈值",
    "sentiment_terms": "舆情词库（类别、严重度、动作覆盖）",
    "sentiment_term_audits": "舆情词库变更审计",
    "sentiment_hits": "舆情命中记录与处置决策",
    "sentiment_content_topics": "内容舆情画像（标签、风险、舆论分）",
    "sentiment_camp_marks": "阵营标记（内容/评论维度的立场）",
    "sentiment_camp_target_settings": "单个内容的阵营识别开关",
    "sentiment_delete_requests": "舆情触发的删除申请",
    "api_keys": "API Key（哈希、前缀、角色、每分钟配额、IP 白名单）",
    "api_key_alarms": "API Key 异常告警与确认",
    "api_access_logs": "API 访问日志（身份、路径、状态码、IP）",
    "user_api_policies": "用户限流策略与 Key 封禁状态",
    "rage_configs": "限流/长连接参数（批量间隔、并发、WS/MQTT 开关）",
    "channel_balance_samples": "上游渠道余额采样（用于额度预警）",
    # 监管
    "police_tickets": "警方工单（类型、优先级、状态、对象用户）",
    "police_ticket_logs": "警方工单流转日志（状态变更）",
    "police_evidence_requests": "取证申请：申请→站长批准→导出",
    "police_evidence_request_logs": "取证申请流转日志",
    "supervised_users": "隔离墙名单（等级、评分、来源、证据引用）",
    "supervise_logs": "隔离墙处置日志（等级变更）",
    "crisis_events": "心理危机事件与干预动作",
    "softdel_users": "软删除用户快照",
    "softdel_novels": "软删除作品快照",
    "softdel_chapters": "软删除章节快照",
    "softdel_blogs": "软删除博客快照",
    "softdel_posts": "软删除帖子快照",
    "softdel_comments": "软删除评论快照",
    "softdel_threads": "软删除主题快照",
    "softdel_forums": "软删除版块快照",
    "emergency_audit_logs": "紧急操作（封站/解锁等）审计",
    "vulnerabilities": "漏洞登记（严重度、影响面、修复时间）",
    # 站点运营
    "platform_configs": "平台配置键值（storage_mode、阅读权限、开关等）",
    "system_settings": "系统设置键值",
    "announcements": "站内公告（置顶、发布状态）",
    "carousels": "首页轮播图（投放时间、目标）",
    "nav_menus": "导航菜单（层级、图标、可见性）",
    "home_blocks": "首页区块（类型、配置 JSON、排序）",
    "site_themes": "全站主题：CSS、自定义 JS 与加载模式",
    "user_themes": "用户个人主题覆盖",
    "search_logs": "站内搜索日志（关键词、结果数、来源）",
    # 访问统计
    "web_access_logs": "全站访问日志（UA 族、路径、IP、身份）",
    "zone_access_logs": "分区进入日志（zone、IP、UA）",
    # 联邦
    "federated_sites": "联邦站点登记（API 地址、状态、同步时间）",
    "federated_novels": "联邦作品缓存（远端 ID 映射）",
    "federated_peer_keys": "联邦对端密钥（方向、哈希、密文）",
    "federated_inbound": "入站请求统计（来源、端点、计数）",
}

# 关键字段补充说明（可选，覆盖到就写进「主要字段」列）
KEY_FIELDS = {
    "users": "role / member_level / status",
    "novels": "author_id / read_permission / paid_price",
    "chapters": "novel_id / chapter_number / content_path",
    "content_blobs": "(ref_type,ref_id) 主键 / content / content_hash",
    "novel_indexes": "novel_id 主键 / data / hash",
    "feed_tags": "(post_id,tag) 唯一 / tag 索引",
    "feed_danmaku": "(post_id,time_ms) 联合索引 / status 索引",
    "aigc_deliveries": "object_key 唯一 / produce_id / labeled / public",
    "aigc_delivery_grants": "(object_key,user_id) 唯一",
    "platform_configs": "key 主键 / value",
    "chapters": "novel_id / chapter_number / content_path / deleted_at",
}


def camel_to_words(t):
    return t


def collect_sources():
    """表名 → 源码位置（models 里的 TableName 定义处）"""
    hits = {}
    for base in ("server",):
        for dirpath, _dirs, files in os.walk(os.path.join(ROOT, base)):
            for f in files:
                if not f.endswith(".go"):
                    continue
                p = os.path.join(dirpath, f)
                try:
                    text = open(p, encoding="utf-8").read()
                except OSError:
                    continue
                for m in re.finditer(r'TableName\(\)\s*string\s*\{\s*return\s+"([^"]+)"', text):
                    rel = os.path.relpath(p, ROOT).replace("\\", "/")
                    hits.setdefault(m.group(1), rel)
    return hits


def col_note(col, indexes):
    notes = []
    if col["pk"]:
        notes.append("主键")
    n = col["n"]
    if n.endswith("_enc"):
        notes.append("密文")
    if n.endswith("_hash") or n.endswith("_signature"):
        notes.append("哈希")
    if n == "deleted_at":
        notes.append("软删除")
    if n in ("created_at", "updated_at", "started_at", "finished_at", "sampled_at"):
        notes.append("时间")
    return "、".join(notes)


def main():
    global SCHEMA
    db = sys.argv[1] if len(sys.argv) > 1 else os.path.join(ROOT, "data", "nvs.db")
    if not os.path.exists(db):
        raise SystemExit(f"!! 找不到数据库：{db}\n用法：python scripts/gen-db-doc.py <nvs.db>")
    SCHEMA = load_schema(db)

    sources = collect_sources()
    assigned = []
    for _name, _desc, tables in DOMAINS:
        assigned.extend(tables)
    missing = sorted(set(SCHEMA) - set(assigned))
    dup = sorted({t for t in assigned if assigned.count(t) > 1})
    unknown = sorted(set(assigned) - set(SCHEMA))
    if missing or dup or unknown:
        print("!! 未归类:", missing)
        print("!! 重复:", dup)
        print("!! 库里不存在:", unknown)
    no_purpose = [t for t in SCHEMA if t not in PURPOSE]
    if no_purpose:
        print("!! 缺用途说明:", no_purpose)

    total_cols = sum(len(v["cols"]) for v in SCHEMA.values())
    total_rows = sum(max(v["rows"], 0) for v in SCHEMA.values())

    # ---------------- 总览
    L = []
    L.append("# 数据表总览\n")
    L.append("> 本文件按业务域盘点全部数据表；逐表字段明细见 [`DATABASE-SCHEMA.md`](DATABASE-SCHEMA.md)。")
    L.append("> 双模式存储/索引/交付副本的行为细节见 [`content-storage-schema.md`](content-storage-schema.md)。")
    L.append("> 表结构以源码里的 GORM 模型为准，`AutoMigrate` 在启动时创建表并补列。\n")
    L.append("```bash")
    L.append("# 结构有变动后重新生成这两份文档（参数为任意一个已迁移的 nvs.db）")
    L.append("python scripts/gen-db-doc.py data/nvs.db")
    L.append("```\n")
    L.append(f"当前共 **{len(SCHEMA)} 张表 / {total_cols} 个字段**"
             f"（行数为本机测试站实测，仅用于判断「有没有数据」，不代表生产规模）。\n")
    L.append("## 全局约定\n")
    L.append("| 约定 | 说明 |")
    L.append("| --- | --- |")
    L.append("| 时间列 | 统一 `created_at` / `updated_at`（部分表另有业务时间列），UTC+8 落库 |")
    L.append("| 软删除 | `deleted_at IS NULL` 为有效行；GORM 默认作用域自动过滤，统计时注意 `Unscoped()` |")
    L.append("| 密文列 | `*_enc` 为对称加密后的密文（模型密钥、渠道令牌、用户自带 Key） |")
    L.append("| 哈希列 | `*_hash` / `*_signature` 为单向哈希，用于校验或去重，不可逆 |")
    L.append("| 内容寻址 | `aigc_deliveries.object_key`、`content_blobs` 的 Hash 均按内容算，同内容只存一份 |")
    L.append("| 删除留痕 | 业务删除走 `softdel_*` 快照表，可原样恢复 |")
    L.append("| 配置 | 运行期开关在 `platform_configs`（改库即时生效，不读 .env） |\n")

    L.append("## 分域索引\n")
    L.append("| 域 | 表数 | 职责 |")
    L.append("| --- | --- | --- |")
    for idx, (name, desc, tables) in enumerate(DOMAINS, 1):
        L.append(f"| [{name}](#domain-{idx}) | {len(tables)} | {desc} |")
    L.append("")

    for idx, (name, desc, tables) in enumerate(DOMAINS, 1):
        L.append(f'<a id="domain-{idx}"></a>\n')
        L.append(f"## {name}\n")
        L.append(f"{desc}。\n")
        L.append("| 表名 | 用途 | 主要字段 | 行数 | 源码 |")
        L.append("| --- | --- | --- | --- | --- |")
        for t in tables:
            info = SCHEMA.get(t)
            if not info:
                continue
            kf = KEY_FIELDS.get(t, "")
            if not kf:
                cols = [c["n"] for c in info["cols"]][:4]
                kf = " / ".join(cols)
            src = sources.get(t, "—（无模型定义）")
            short = t.replace("_", "\\_") if False else t
            L.append(f"| `{short}` | {PURPOSE.get(t, '')} | {kf} | {info['rows']} | `{src}` |")
        L.append("")

    # ---------------- 明细
    S = []
    S.append("# 数据表字段明细\n")
    S.append("> 由 `scripts/gen-db-doc.py` 依据实际库结构 + 模型自动生成，**请勿手工修改**；")
    S.append("> 改模型后重新生成即可。用途说明见 [`DATABASE.md`](DATABASE.md)。\n")
    S.append(f"共 {len(SCHEMA)} 张表、{total_cols} 个字段。\n")
    for name, desc, tables in DOMAINS:
        S.append(f"## {name}\n")
        for t in tables:
            info = SCHEMA.get(t)
            if not info:
                continue
            S.append(f"### `{t}`\n")
            S.append(f"{PURPOSE.get(t, '')}（行数 {info['rows']}）\n")
            S.append("| 字段 | 类型 | 说明 |")
            S.append("| --- | --- | --- |")
            for c in info["cols"]:
                S.append(f"| `{c['n']}` | {c['t'] or '—'} | {col_note(c, info['idx'])} |")
            extra = [i for i in info["idx"] if not i.startswith("sqlite_autoindex")]
            if extra:
                S.append("")
                S.append("索引：" + "、".join(f"`{i}`" for i in extra))
            S.append("")

    os.makedirs(os.path.join(ROOT, "docs"), exist_ok=True)
    open(os.path.join(ROOT, "docs", "DATABASE.md"), "w", encoding="utf-8", newline="\n").write("\n".join(L))
    open(os.path.join(ROOT, "docs", "DATABASE-SCHEMA.md"), "w", encoding="utf-8", newline="\n").write("\n".join(S))
    print(f"OK tables={len(SCHEMA)} cols={total_cols} rows={total_rows}")
    print("DATABASE.md 行数:", len(L), " DATABASE-SCHEMA.md 行数:", len(S))


if __name__ == "__main__":
    main()
