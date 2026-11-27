# 架构与数据字典

> 目录结构、分层约定、数据库表与索引说明。
> 返回 [README](../README.md) · [更新日志](CHANGELOG.md) · [API](API.md) · [配置](CONFIG.md) · [开发](DEVELOPMENT.md) · [特性详解](FEATURES.md)

```
nvs/
│
├── nvs-server.exe                 # 预编译单文件（下载即用）
├── build.bat                      # 一键构建脚本
├── build-all.ps1                  # 全平台交叉编译
├── .env.example                   # 环境变量模板
├── .env                           # [自动生成] 运行时配置
├── docker-compose.yml             # Docker 容器编排
├── Makefile                       # check / check-all / dist / build / version 等目标
├──
├── scripts/                       # 质量门禁与真浏览器验收（可在本地重复执行）
│   ├── check-spa-roots.sh         # 路由首段 vs knownSPARoots 对撞（防深链 404）
│   ├── audit-unresolved-components.mjs  # 模板用了但没 import 的组件
│   ├── audit-admin-pages.mjs      # 后台页四态体检（--strict）
│   ├── fix-table-empty-slots.mjs  # 清理列级 #empty 死插槽（--write）
│   ├── scan-frontend-secrets.sh   # 前端产物密钥扫描
│   ├── browser-smoke.mjs          # 四档宽度几何回归（375/768/1024/1440）
│   ├── browser-mobile.mjs         # 移动端专项（触控目标/字号下限/横向溢出/固定层）
│   ├── live-adro-replay.mjs       # 真站越狱回放（同会话逐轮打，验安全闸）
│   ├── browser-svg-export.mjs     # SVG / mermaid 导出验收（PNG 魔数 + SVG 源文）
│   ├── browser-comment-image.mjs  # 评论附图 + 楼加密可见性（上传/渲染/脱敏/评论区不连带）
│   ├── browser-func.mjs           # 功能级回归（排序参数/密度/朗读/落库）
│   └── browser-me-center.mjs      # 个人中心与下拉菜单专项
│
├── server/                        # Go 后端
│   ├── main.go                    # 入口（501 行）：配置初始化 → 数据库 → 子包 Migrate → 组装路由 → 启动
│   ├── embed.go                   # 发布轨：//go:embed all:dist（需先 make dist）
│   ├── embed_nodist.go            # 业务轨（-tags nodist）：嵌入提示页，不依赖前端产物即可编译/测试
│   ├── nodist_placeholder/        # 上面那轨嵌的提示页（告诉你怎么构建真正的发布物）
│   ├── routes/                    # 路由装配（从 main.go 抽出，按分组分文件，最大 492 行）
│   │   ├── routes.go              # Register：中间件链 + /health + 建站向导，依次调用下面五个
│   │   ├── public.go              # /api 公开组
│   │   ├── protected.go           # /api 登录组
│   │   ├── admin.go               # /api/admin 后台组
│   │   ├── police.go              # /api/police 取管台
│   │   ├── spa.go                 # /sandbox 静态资源 + NoRoute（SPA 回退）
│   │   └── guards.go              # setupGuard / setupLocalOnlyGuard
│   ├── Dockerfile
│   ├── init.sql                   # MySQL 手动建表脚本
│   ├── go.mod / go.sum
│   │
│   ├── config/
│   │   ├── config.go              # 自动创建 .env + 加载 + IPv4/IPv6 检测 + URL_PREFIX 解析
│   │   ├── secure.go              # 数据库配置 AES-256-GCM 加密存储（.secret.key + secure-config.json）
│   │   ├── redact_config.go       # 日志脱敏配置（RedactManager：.env 种子 + DB 持久化 + 热更新）
│   │   ├── envfile.go             # .env 键映射与自动补写
│   │   └── setup.go               # 建站向导状态 / 一次性安装密钥
│   │
│   ├── models/                    # 数据层（133 个文件 / 120+ 结构体）
│   │   ├── user.go                # User（角色含 police / 会员有效期 / 实名 / 未成年保护 / 禁言）
│   │   ├── novel.go               # Novel + Chapter + NovelCategory（软删除）
│   │   ├── comment.go             # Comment：楼层模型（root_id / floor_no）+ 加密楼 + 排序白名单
│   │   ├── comment_policy.go      # 评论区策略：楼加密是否连带隐藏楼内回复（平台级开关）
│   │   ├── forum.go / groups.go   # 论坛（Forum/Thread/Post）与群组（Group/GroupPost/成员/公告）
│   │   ├── blog*.go               # 作者博客 / 三级分类 / 标签 / 话题
│   │   ├── bookshelf.go / rating.go / follow.go / reading_note.go  # 书架·评分·关注·书签批注
│   │   ├── ai*.go                 # 模型池 / 密钥池 / 路由策略 / 调用日志 / 会话 / 媒体任务 / 成本
│   │   ├── membership*.go         # 会员套餐 / 充值订单 / 代币流水 / 免费额度
│   │   ├── chapter_vcs.go         # 章节版本（内置实现 / git 后端可切）
│   │   ├── collab.go / oc*.go     # 协作成员与主笔 / OC 角色卡·世界书·多智能体会话
│   │   ├── prompt_template.go     # 提示词模板 + 变量白名单
│   │   ├── live.go / realtime/    # 直播状态与 SSE Hub
│   │   ├── softdel.go             # softdel_* 留证表统一写入（8 类内容）
│   │   ├── zone_access.go         # ZoneAccessLog 隔离墙进入记录（176号令专项）
│   │   ├── web_access.go          # WebAccessLog 攻击流量/爬虫统计
│   │   ├── police*.go             # 警方工单 / 取证
│   │   ├── payment.go / risk.go   # 支付通道 / 充值风控
│   │   ├── platform*.go           # 站长配置 / 公告 / 隔离墙 / 联邦站点 / 备案与合规
│   │   ├── xss_rule.go / badword.go / vulnerability.go / log_archive.go
│   │   └── db.go                  # AllModels 迁移 + 跨库复制 + 热切换
│   │
│   ├── handlers/                  # API 处理器（根 133 个文件 + 11 个子包，按功能拆分）
│   │   ├── auth_*.go              # 注册 / 登录 / 验证码与改密（含账号锁定与审计）
│   │   ├── novel*.go / chapter*.go# 作品与章节读写（SHA256 + HMAC 签名、软删除归档）
│   │   ├── comment.go             # 评论（楼层分页 + sort=new/old/hot + 加密楼 + 取证视图 + 附图审核）
│   │   ├── admin_*.go             # 后台：用户 / 举报 / 论坛 / 站点 / 合规 / 漏洞 / 额度 / 密码与国密
│   │   ├── police.go              # 警方取管台：软删除查看 / 凭证下载 / 全量取证包 / 论坛巡查
│   │   ├── social_*.go            # 关注 / 博客 / 话题 / 私信 / 黑名单 / 特别关注
│   │   ├── home_customize.go      # 个人主页装扮（双作用域 + 消毒 + 站长审核）
│   │   ├── emergency.go           # 内容抢救台（二级密码 + 硬删除 + 审计）
│   │   ├── setup.go / db_admin.go # 建站向导 / 数据库健康·热迁移·备份恢复
│   │   ├── export.go              # 子包桥接：把 CurrentUserID / EnsureAdmin 等导出给子包用
│   │   ├── context.go             # 鉴权上下文统一封装（currentUserID / UserRole 等）
│   │   ├── ai/                    # AI 域（41 个文件：对话流式 / 绘画 / 视频 / 智能体 / 模型池 /
│   │   │                          #   渠道监控 / 定价 / 成本 / 安全闸 / AIGC 标识导出）
│   │   ├── group/ honeypot/ oc/   # 群组（11）/ 蜜罐（2）/ OC 模式（2）
│   │   └── authors/ blogs/ log/   # 〔运行时产物目录〕上传与日志误落此处，非 Go 包，勿当源码改
│   │
│   ├── aiclient/  aiapi/  aicall/ # 多厂商模型协议（OpenAI 兼容 / Anthropic / Gemini / MJ / Kling / Coze）
│   ├── chaptervcs/  collab/       # 章节版本控制后端 / 协作
│   ├── groups/  oc/  prompttpl/  realtime/   # 群组 / OC 模式 / 提示词模板 / SSE 推送
│   ├── honeypot/  gate/  risk/    # 蜜罐（诱饵·影子认证·指纹·取证）/ 准入闸门 / 充值风控
│   ├── aigc/  aisafety/  sentiment/  crisis/ # AIGC 标识（角标 + XMP）/ 三层安全闸（+ 跨轮拼装链检测 + 绘画审核）/ 舆情 / 心理危机
│   ├── ipgeo/  doctext/  moderation/         # 离线 IP 归属 / 文档正文抽取 / 审核规则
│   ├── payment/  police/  audit/  # 支付通道 / 警方取证 / 审计
│   ├── middleware/                # 鉴权 / CSRF / 访问日志 / 脱敏 / 前缀剥离 / 沙盒头 / 端口代理
│   ├── security/                  # 限流 / XSS 净化 / 内容签名 / 内容扫描 / 违禁词 / 滑块验证码 / 正则安全
│   ├── markdown/  scriptmode/     # Markdown 渲染（Mermaid/chemfig）/ 话本气泡体解析
│   ├── realname/  crypto/  tlsconfig/  cert/  gmcmd/  utils/  version/
│   │
│   └── dist/                      # [自动生成] 前端构建产物（Go embed 使用）
│
├── web/                           # Vue 3 前端
│   ├── vite.config.ts             # outDir → ../dist / dev 代理 /api → 8080 / 按需导入 Element Plus
│   ├── package.json               # TipTap · ECharts · KaTeX · Mermaid · Element Plus · three
│   └── src/
│       ├── router/index.ts        # 117 条路由 + 角色守卫（requiresAdmin / requiresPolice）+ base 自动感知前缀
│       ├── stores/                # Pinia：auth（幂等 fetchUser）/ theme / app（站点配置与未读数）
│       ├── api/                   # 43 个模块：axios 封装（前缀 + CSRF 头）、各业务域
│       ├── utils/                 # prefix / sensitiveZone / userKey / adminHealth / editorShortcuts / turndown / miniZip / clipboard
│       ├── styles/                # global.css（设计令牌）/ scale.css（间距·字号·容器档位）/ admin.css / mobile.css（窄屏统一修补）/ ai-code.css（代码块高亮与工具条）
│       ├── views/                 # 页面（顶层 0 个文件，全部按业务域归入 15 个子目录）
│       │   ├── ai/                # AiHub（竖列：对话/广场/绘画/视频/灵感/智能体/思维导图）/ AiGallery / AiPricing / AiShared
│       │   ├── auth/              # Login / Register / SetupWizard
│       │   ├── novel/             # NovelDetail / Reader（精读·朗读·划线）/ Editor / ChapterEditor / CategoryView
│       │   ├── blog/              # BlogList / BlogDetail / BlogEditor / BlogHome / BlogManager / BlogCategory*
│       │   ├── forum/             # Forums / ForumDetail / ThreadDetail / Topics
│       │   ├── author/            # AuthorDashboard / AuthorHome（封面+分栏）/ AuthorBlogs / AuthorTemplateSettings
│       │   ├── home/              # Home / HomeSwitch / Feed
│       │   ├── payment/           # Membership / PayMock / PayResult / Wallet
│       │   ├── comment/           # CommentNetworkDrawer / CommentNetworkPage（评论关系网）
│       │   ├── me/                # MeLayout + 概况/作品/博客/留言/数据/装扮（「我的服务」四组入口）
│       │   ├── site/              # About / DocsPage / Settings / ThemeSettings / Live / HtmlPlaza / APIKeySpider
│       │   ├── admin/             # 后台：AdminDashboard + adminMenuData + 6 个子目录
│       │   │   ├── site/          # 站点设置 / 登录方式 / 主页区块 / 主题 / 狂暴模式 / 数据库 / 健康 / 互通 …
│       │   │   ├── security/      # 合规 / 漏洞 / 日志脱敏 / 密码与国密 / 蜜罐 / 门槛 / 风控 / 隔离墙 / 网络诊断 …
│       │   │   ├── content/       # 审核队列 / 违禁词 / 封禁 / 留证 / 抢救台 / HTML 审核 / 举报 / 分类 / 舆情 / 危机
│       │   │   ├── ai/            # 模型池 / 定价 / 成本 / 安全闸 / 审核 / 日志 / 额度 / 智能体 / 算法
│       │   │   ├── business/      # 支付 / 代币 / 发布额度 / 网页额度 / 用户管理
│       │   │   └── police/        # 取管台 / 工单 / 论坛巡查
│       │   ├── user/              # 私信 / 黑名单 / 特别关注 / 微信绑定 / 网页额度
│       │   ├── group/             # 群组列表 / 详情 / 发帖
│       │   └── frame/             # FrameManager / HTMLManager / HtmlRunner / WasmRunner / SandboxPreview
│       └── components/            # 70 个通用组件
│           ├── NavBar（下拉只剩 5 项）/ CommentSection / FloorComments（楼层模型）
│           ├── StateBlock（骨架/空/错误三态）/ AdminHeader / AdminFilterBar / AdminApiAlert
│           ├── DashboardCharts / StarRating / SensitiveZoneGuard / SlideCaptcha / MentionTextarea
│           ├── ai/ — AiChatPanel / AiImagePanel / AiVideoPanel / AiModelSquare / AiAgentsPanel / MindMapPanel
│           ├── admin/ · author/ · editor/ · group/ · message/ · user/（按域分子目录）
│
├── dist/                          # [自动生成] 前端构建产物（项目根目录）
├── data/                          # [运行时生成]
│   ├── nvs.db                     # SQLite 数据库
│   ├── novels/                    # 小说 HTML 正文
│   └── uploads/                   # 上传资源
│
└── nginx/
    └── nginx.conf                 # Docker 模式 Nginx 配置
```

### AI 安全闸：三层 + 跨轮拼装链

```
用户输入 ──▶ 规则引擎（scan.go）
              │  物质（宽口径：含工业前体，权重 25）
              │  意图 / 工艺 / 参数（依赖物质存在才计分）
              │  跨轮拼装链（chain.go）
              └─▶ 蓝队判定模型 B（judge.go，整段对话送审，批次制）
                        └─▶ 输出闸门（store.go OutputGuard，增量扫）
                             拒绝后泄漏判定（leak.go）
```

四道闸门各自的职责与失效边界：

| 闸门 | 判什么 | 失效时 |
|---|---|---|
| 规则引擎 | 单轮信号组合 + **跨轮家族拼装** | 确定性，不依赖模型 |
| 蓝队模型 B | 整段对话的累积意图（ADRO 阶梯） | 上游不可用时按配置 fail-open / fail-closed |
| 输出闸门 | 模型输出的工艺方向 / 反应方程式 / 同族多物质 | 话题不敏感时不启用（避免误杀科普） |
| 用户画像 | 该账号索取危险品的累积程度 | 达到阈值进入观察期 → 冷却 |

**跨轮拼装检测（chain.go）** 是这套设计里最关键的一环。ADRO 类越狱（Anchor 锚定 →
Deconstruct 拆解 → Recur 循环 → Output 输出）会把一条合成链拆成十几个单看无害的提问，
点状规则永远判不出来。判定条件：

1. 同一**合成家族**（`ruleGroup.Family`）在对话中出现 ≥2 个成员；
2. 其中至少一个成员本身是**管制物质**（`leakTopicTerms`）；
3. 当前轮在问「怎么做」（工艺词 / 追问词 / 制造意图 / 转化句式）。

三者同时成立才判定为拼装攻击并加分。工业前体（丁二醇 / 丁二烯 / 丁烷 / 丁烯 / 顺丁烯二酸酐）
单独出现**不构成命中**（权重 25），但能参与拼合检测 —— 否则攻击者换一个前体，链就断了。

**泄漏判定（leak.go）** 区分两种形态：

- *拒绝后泄漏*：模型先拒绝、再给方向（试剂 + 工艺 + 参数）—— 需话题敏感 + 拒绝语；
- *硬信号*：给出**化学反应方程式**，或输出里同族多个管制物质同现 —— 无需拒绝语即定性，
  因为模型有时根本不拒绝、直接照答。

拒绝语检测除固定短语外还认软性拒绝句式（「不会涉及具体操作参数」「仅供理论认知」这类）——
只匹配固定短语时这类输出会整段漏判。

### 外键约束
- SQLite 连接通过 DSN `_pragma=foreign_keys(1)` **每连接自动启用外键约束**（启动时校验 `PRAGMA foreign_keys = 1` 否则拒绝启动）；MySQL 原生启用。
- 热迁移（SQLite ↔ MySQL）复制期间临时关闭外键检查（`PRAGMA foreign_keys=OFF` / `SET FOREIGN_KEY_CHECKS=0`），完成后恢复，避免表复制顺序触发外键失败。
- 启用后删除被引用记录会被拒绝（如删除仍有回帖/评论的用户），由业务层先级联处理或归档。

### posts 表删除语义（方案A：统一走 softdel_posts）
- **posts 表不再有 `deleted_at` 列**：回帖删除 = 先 `ArchiveSoftDelete` 归档完整快照到 `softdel_posts` 留证表，再物理删除原始记录。
- 与 novels/chapters/comments/threads 等「软删除 + softdel_* 留证」双轨逻辑语义一致，消除 `deleted_at` 与 `softdel_posts` 两套删除逻辑并存的混乱。
- 升级迁移：启动时 `DropPostDeletedAt` 自动检测并重建 posts 表（SQLite 建新表→拷贝→重命名，保留全部数据与自增序列；MySQL 直接 `DROP COLUMN`），幂等可重复执行。
- `softdel_posts` 及其余 `softdel_*` 留证表保持不动。
- ⚠️ 回帖删除后不可解封恢复（物理删除），管理员「解封/恢复」对 post 类型仅清除封禁原因标记。

### 数据表
全部 **194 张表 / 2205 个字段** 已按业务域整理成两份专门文档：

- [`DATABASE.md`](DATABASE.md)：分域总览（表名 / 用途 / 主要字段 / 行数 / 源码位置）+ 全局约定
- [`DATABASE-SCHEMA.md`](DATABASE-SCHEMA.md)：逐表字段明细（由 `python scripts/gen-db-doc.py data/nvs.db` 自动生成）

本节以下只列**经过设计、需要额外说明**的表与索引，其余按名自解释。

### 核心索引
启动迁移自动创建缺失索引并清理被替代的旧索引：

| 表 | 索引 | 用途 |
|----|------|------|
| novels | `idx_novels_author_status` | 按作者查作品并筛选状态 |
| posts | `idx_posts_thread_created` / `idx_posts_user_id` | 帖内回帖列表 / 按用户查回帖 |
| threads | `idx_threads_forum_created` | 按版块查主题帖 |
| comments | `idx_comments_novel_created` / `idx_comments_parent` | 作品评论列表 / 子评论 |
| notifications | `idx_notifications_user_read_created` | 未读通知倒序 |
| reports | `idx_reports_target` | 按目标查举报 |
| user_audit_logs | `idx_user_audit_user_time` | 按用户查审计日志 |
| zone_access_logs | `idx_zone_access_user_zone` | 按用户+分区查隔离墙记录 |
| forum_mutes | `idx_forum_mutes_forum_user` | 论坛内禁言查询 |

（chapters 由唯一复合索引 `uk_novel_chapter(novel_id, chapter_number)` 覆盖，无需重复索引；follows / bookshelf / ratings / federated_novels 已有等价索引。）

### 视频弹幕（feed_danmaku）
- **单独建表，不混 `feed_comments`**：弹幕绑在视频时间轴（`time_ms`）上、量大、读多写少，且不应计入评论数、不进楼中楼列表。
- **联合索引 `idx_danmaku_post_time(post_id, time_ms)`**：按视频 + 时间窗拉取（`from_ms`/`to_ms` 闭区间），一次加载整片弹幕；`status` 单列索引用于隐藏/审核。
- **只存纯文本**：`content` 上限 80 字（列宽 255 字节），前端用 `textContent` 写入而非 `v-html`，天然免疫 XSS；颜色仅接受 `#rrggbb`。
- **时间点校验**：写入时校验目标必须是 `kind=video` 且处于发布态，`time_ms` 不得超过 `(duration+5)*1000`（时长未知时放行，允许先发后取元数据）。
- **配额**：单次拉取上限 3000 条；发送走 60 秒 30 条限流。
- **渲染**：前端 `components/feed/DanmakuLayer.vue` 由 rAF 逐帧对齐 `currentTime`（`timeupdate` 仅 4 次/秒，会成串冒出），轨道分配按「同轨上一条尾巴让出间距」的时刻判定；暂停时把 CSS 动画 `animation-play-state` 置 `paused`，回退超过 0.8 秒清屏并按二分重算游标。
- **播放器**：`components/feed/VideoPlayer.vue` 自绘控制条（`controls` 属性关闭），详情页与竖滑沉浸模式共用同一组件。

### 亚文化创作区：世界（立项制）与角色形象图库
- **两层容器**：`oc_projects`（世界/企划，顶层）→ `oc_world_books`（世界书，可多本）→ `oc_world_entries`（按键触发的设定条目）；角色卡 `oc_characters` 同时挂 `project_id`（归属世界）与 `book_id`（归属世界书）。
- **世界有自己的路由**：`/subculture/world/:id`（`views/subculture/WorldDetail.vue`），分五个区块——角色 / 形象图库 / 世界书 / 成员与跨世界链接 / 设置。列表页（`components/oc/ProjectManager.vue`）只负责创建与进入，不再用抽屉承载详情，避免两套 UI 维护同一批数据。
- **默认世界书**：角色必须归属世界书，而新世界初始没有任何书；世界页在 `worlds.length === 0` 时自动补一本同名书（失败静默，只读成员建不了）。补完后要通知 `CharacterManager` 重取书列表（`defineExpose({ reloadBooks })`），否则「新建角色卡」会被误判成「还没有世界」。
- **角色按世界过滤**：`GET /api/oc/characters?scope=project&project_id=N`。`scope=project` 返回世界内全部角色（含其他成员创建的），因此 **handler 必须先用 `oc.GetProject` 验项目可见性**，否则可以靠猜 id 列出别人私有世界里的角色。
- **形象图库 `oc_character_arts`**：一张角色卡可挂多张图，`kind` 白名单 `portrait/emote/ref/scene`；`project_id` 从角色回填（不信任前端）；每张图可写 `traits`（这张图强调的外貌特征），与角色卡的 `appearance`/`persona` 一起构成「样子 + 性格」。图片地址复用 `ValidateEmoteURL` 的协议白名单（拒 `javascript:`/`data:`），本体仍走既有上传接口。
- **写权限**：角色所有者 / 项目 owner·editor / 平台管理员（`canEditArt`）；世界聚合查询 `GET /api/oc/projects/:pid/arts` 先验项目可见性。
- **世界书条目提交必须用 camelCase**：`SaveEntry` 的绑定结构体没有 json tag，`encoding/json` 只做大小写不敏感字段名匹配，`secondary_keys` / `insertion_order` 这类 snake_case 会被静默丢弃（已修）。

### 自托管：运行时目录 / 项目托管 / 反向代理
- **安全边界**：平台**不在本机执行用户代码**。作者自带服务（`upstream` 填自己的 `host:port`），平台只做源码/编译产物托管、人工审核与反向代理。`Project.LocalExec` 是预留字段，当前恒 `false`，接口不接受置真。
- **四张表**：`selfhost_runtimes`（语言/版本/编译型或解释型，站长维护，平台不负责安装）→ `selfhost_projects`（源码模式 / 编译版，`source_file`/`binary_file` 落 `data/selfhost/<uid>/<pid>/`，`review_status`、`visibility`、作者自带 `upstream`）→ `selfhost_routes`（一个项目一条对外前缀路由）→ `selfhost_review_logs`（提交/通过/驳回/停止/恢复留痕）。
- **审核门禁**：新项目一律 `draft` + `private`；`SubmitForReview` 再验一次源码与可执行文件；只有 `approved` 才能设公开、才能建/启用代理路由。项目内容或文件一改就退回 `draft`（先审后改绕过防护）。站长可停止项目，路由随之立即失效。
- **源码强制规则**：`platform_configs.selfhost_source_required`（默认 `1`，DB 优先于 env）。`SourceRequired(lang, waived)` 按运行时 `kind` 与语言白名单（Java/Go/.NET/Rust/C/C++…）判定；管理员可在审核时逐项目 `source_waived=true` 豁免。后台「反向代理管理」页有全局开关。
- **SSRF 边界**：`ValidateUpstream` 只收 http/https，拒 `localhost`、`127/8`、`::1`、`169.254/16` 与裸端口写法；代理表加载时再用 `target.IsSelf(selfPort)` 挡「指向本平台监听端口」的路由，配合跳数守卫（超 5 跳返回 508）。
- **反代核心来自 gatewayhub v1.6.0**（同作者、同为 AGPL-3.0）：`server/selfhost/proxycore` 整体移植 GWh 的 `manager/match/entry/handler/helper`（最长前缀匹配、`Upsert/Remove` 热加载、双 Transport 连接池、32KB buffer pool、508 跳数守卫、WebSocket 免超时、502/504 统一错误、statusWriter），加上 `rewrite`（前缀 shim / HTML·CSS·JS 字面量改写 / Set-Cookie Path 收紧 / 自算 ETag）、`hopguard`、`target`。除 `gin` 外只依赖标准库，**连同 GWh 的原始单元测试整体搬入并通过**。与 GWh 一样是**路径级反向代理**：前缀直接挂在根路径（如 `/java-order/...`、`/team/app/...`），在 `NoRoute` 里先 `Matchable()` 再交给 `proxycore.Manager.Handle()`，未命中才走 NVS 原有的严格 SPA 回退。TLS 由外层网关终止，平台自身不碰证书。
- **文件访问默认拒绝**：`Project.SourceShared` / `BinaryShared` 默认 false，公开项目**不等于**公开源码。`CanDownload()` 只放行所有者/管理员，或显式共享开关，或 `selfhost_file_grants` 里已批准的用户；匿名一律拒绝。前端下载必须走 axios（带 `X-Device-Fingerprint`），普通 `<a href>` 会被令牌绑定拦成 401。
- **授权留痕**：`POST /projects/:id/access` 申请 → 所有者 `POST /grants/:id/review` 批准/驳回 → 可撤销；列表响应带服务端计算的 `can_source` / `can_binary`，前端不自行推断。
- **特殊转发剥离（与 GWh 的 `stripDeployPrefix` 同语义）**：`middleware.SmartStripURLPrefixHandler` 包在 Gin 引擎外层、路由匹配之前。显式配置的 `URL_PREFIX` 优先剥离；未命中时，仅当首段既非保留字（`api/assets/static/favicon/uploads/sandbox/health/hp`）、非前端 SPA 根段、非已注册代理路由首段，且路径含第二段，才判定为反代透传的部署前缀并剥掉。因此 `/{name}/java-order/x` 能命中代理路由，而 `/java-order/x` 本身不会被误吃。
- **保留段**：代理首段不能是 API / 静态资源 / 前端 SPA 根段（`ValidatePrefix` 里 `reservedProxyPrefixes`），否则会劫持后台与页面。
- **热加载**：路由增删改与审核状态变化时 `InvalidateRoutes()` 重建内存表；`SortedEnabledRoutes()` 只返回「已审核通过 + 已启用」的路由，并按前缀长度降序，避免 `/a` 抢掉 `/a/b`。WebSocket 升级请求跳过超时，避免长连接被掐断。
- **后台三块独立页**：`/admin/selfhost/runtimes`、`/admin/selfhost/projects`、`/admin/selfhost/routes`；前台 `/selfhost/projects`（我的项目 + 文件上传 + 提交审核）与 `/selfhost/plaza`（仅已审核公开项目）。

### 预留表 / 字段（功能未开发）
以下表与字段当前仅建表占位，**无业务代码**，请勿误用：

| 表 | 字段/表 | 状态 | 说明 |
|----|---------|------|------|
| users | `real_name` / `id_card` / `bank_card` | 预留 | 企业版实名认证与支付绑定，暂未启用 |
| vip_applications | 整表 | 预留 | VIP 会员申请体系，暂未开发 |
| earnings_records | 整表 | 预留 | 作者收益记录，暂未开发 |
| withdrawal_requests | 整表 | 预留 | 提现申请流程，暂未开发 |

### 端口直派、TCP 映射、用量与空间隔离
- **端口直派 `selfhost_direct_routes`**：管理员填对外前缀 + 本机端口（+ 可选后端根路径），保存即写入内存路由表，与项目审核解耦；上游固定为 `127.0.0.1:<port>`，同样走 proxycore 的最长前缀/改写/508 跳数守卫。保留段校验与项目路由共用。
- **TCP 端口映射 `selfhost_port_maps`**：`server/selfhost/portmap` 的运行时在 Gin 之外监听端口，双向 `io.Copy` + `CloseWrite` 转发；`Reload()` 只变更新增/修改/删除的监听，监听失败回写 `last_error`。禁止与平台自身端口冲突、禁止 127.0.0.1 同端口自环。仅管理员可配置。
- **用量 `selfhost_usage_daily`**：proxycore 在请求体与响应写入处计数，按代理前缀归属到项目，按天累计请求数/上下行字节；体积由源码包+可执行文件实际落盘字节实时求和。`GET /api/admin/selfhost/usage` 给全站报表。
- **空间隔离**：`CheckIsolation()` 在提交审核、建/启用路由前阻断三类跨空间配置——上游指向本机回环、指向平台自身端口、与其他项目上游完全相同或指向另一个项目的代理前缀；后台「用量与隔离」页提供自检列表。
- **启动参数**：上传可执行文件的项目必须填写 `start_command`，前后端双向校验；无启动参数无法提交审核。

### 运行时扫描与申请闭环
- `Runtime.ExecPath` 保存可执行位置：`POST /api/admin/selfhost/runtimes/scan` 由管理员点击触发，扫描 PATH 中的 java/go/node/python/php/dotnet/rust/gcc/maven，固定命令表、不走 shell、3 秒超时、输出截断；结果可一键添加，也可在运行时表单里手动填写路径。
- `selfhost_runtime_requests`：用户提交缺失的语言+版本与用途说明；管理员看到程序/代码说明后 `POST .../review`，可以按原样批准，也可以**直接修改语言/版本与可执行路径**再落地为运行时；被驳回的记录保留管理员意见。
- 前台运行时下拉由 `GET /api/selfhost/runtimes` 提供（仅启用项），用户页有「运行时申请」入口。

- **自指与嵌套**：管理员直派可填 `target_url`（如 `http://127.0.0.1:8000/other`），实现 GWh 同款的路径别名/嵌套转发；`X-GatewayHub-Hops` 每跳 +1，超过 5 跳返回 508，死配置不会变成请求生成器。用户项目路由仍禁止 localhost/回环，避免审核后借代理访问平台自身。
- **Java 验收样本**：`C://Users//QBZ95//Desktop//tools//tests//spt//sports-2.6.8.jar` 用 `java -jar ... --server.port=19102` 启动，PATH 扫描能识别到 Temurin JDK 21.0.10；直派 `/sports-qa/` 指向 19102 可正常访问，别名 `/alias-qa/` → `/sports-qa/` 走两跳仍 200，`/loop-qa/` 自环返回 508。

### GWh 完整吞并：三种去头模式与隔离引擎
- **整包移植**：`server/selfhost/isolation/` 是 gatewayhub `internal/isolation` 的完整移植（engine 517 行 + response 322 行 + breaker/config/cookie/diag/forward/matcher/metrics/respond，含 11 个原始单测全过）；`server/selfhost/accesslog/` 与 `target/backend.go` 一并移植。导入路径 `gatewayhub/internal/*` → `nvs-server/selfhost/*`。
- **三种模式**（`isolation/config/mode.go`，与 GWh 语义完全一致）：
  - `pass-through`：原样透传 `/{服务名}/...`，后端自己认前缀；只做头层修正，不改 body。
  - `strip`：剥离 `/{服务名}` 前缀（空则置 `/`，`path.Clean` 防越界、保留尾斜杠），**允许**响应体字面量改写。
  - `isolated`：剥离前缀，但**响应体一个字节都不改**（降级模式，仅头层 Cookie/Location 修正）。
  - `ModeParse` 收敛常见别名（passthrough/pass/transparent、prefix-strip、isolate/isolation/headers-only）。
- **完整能力**：Cookie 帽子（`{服务名}__` 前缀防跨服务串号）、外来 Cookie 丢弃、Location 前缀补齐、HTML/JS/CSS 绝对链接改写（br/zstd 不解压原样透传、gzip 逐跳解包重压、ETag 带跳级 scope）、熔断（breaker）、每服务指标、审计日志、跳数守卫（自指/嵌套 >5 跳 508）、自指重定向环检测。
- **NVS 接入**：`selfhost_iso_services` 表存配置（含 cookie_hat/html_rewrite/host_mode/known_abs_prefixes/max_rewrite_bytes 等），`isoEngineTable()` 映射成 `config.Table` 快照，`engine.Engine.Reload()` 热加载；`Matchable()/Handler()/HasRoutePrefix()` 由完整引擎驱动。`MaxRewriteBytes<=0` 归一到 GWh 默认 2MB。
- **管理接口**：列表带运行状态快照（熔断/指标）、`/api/admin/selfhost/iso-metrics`、`/api/admin/selfhost/iso-audit`；后台「路径隔离代理」页展示审计与命中数，配置表单可改全部高级字段。
- ⚠️ 旧的 215 行简化 `selfhost/iso/` 已移除，不再有双实现。

### 跨系统嵌套代理（NVS ↔ GWh）
- **问题**：`NVS(/gwh/) → GWh(/go-nvs/) → 后端` 这类双向嵌套里，内层网关会先注入 shim（P=局部前缀）与 `<base href=局部>`；外层若因「已有 shim」就跳过，页面会留下 base=`/gwh/` 与 P=`/go-nvs` 不一致，相对资源解析到 `/gwh/assets` 而不是 `/gwh/go-nvs/assets`，整站 404。
- **修复**：`rewrite.injectShim` 遇到已注入的 shim 时不再跳过，而是用 `shimBlockRe` 整块替换为**当前完整链**的前缀（`<base>` 本就是覆盖式改写）。GWh 侧 `internal/proxy/rewrite/shim.go` 同步修复，两边行为一致。
- **完整链传递**：`X-GatewayHub-Prefix-Chain` 请求侧逐跳累加、响应侧由最内层写回，跨进程 HTTP 实测 `NVS→GWh→NVS` 得到 `/gwh/go-nvs`、反向得到 `/go-nvs/gwh`；两侧 shim 与 base 始终一致。跳数守卫 `X-GatewayHub-Hops` 超 5 跳 508。
- **openlist**：`data-src`（vite legacy）不由静态改写处理（避免二次拼前缀），靠 shim 运行期 hook；`base_path: '/'` 由 `PrepareHTML` 改成网关前缀。NVS 与 GWh 对 `/opls/` 的输出逐字节一致。

### 三种去头模式智能识别
- **问题**：同一个 openlist，配成 strip 还是 pass-through 取决于后端是否自己配了 base path；选错的症状是白屏或 302 自环，人工试错成本高。
- **探测包** `server/selfhost/isolation/probe`：只读请求后端根路径（不跟随 302），据三类信号判定——302 Location 带服务名前缀 / 内联 `base_path` 非根 / `<base href>` 带前缀 ⇒ `pass-through`（后端已认前缀）；HTML 且 `base_path='/'` + 根相对资源 ⇒ `strip`（网关剥前缀并改写）；非 HTML 或信号不足 ⇒ `isolated`（最保守）。返回 `mode/reason/signals/confidence`。
- **接口** `POST /api/admin/selfhost/iso-probe`；后台「路径隔离代理」页新增服务时可点「智能识别模式」，自动填入推荐模式并展示证据。GWh 侧同款探测包与 `/api/isolation/probe`，隔离页每行可一键识别。
- **全局 gzip 与代理**：代理路径跳过全局 `gin-contrib/gzip`（`routes.conditionalGzip`），由代理层自己解压→改写→重压。gin-contrib/gzip 在 defer 用压缩前 Size 回填 CL，会让浏览器按错误 CL 解压缩字节（`ERR_CONTENT_LENGTH_MISMATCH`），openlist 并发加载曾因此白屏。
