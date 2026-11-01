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
│   │   ├── comment.go             # 评论（楼层分页 + sort=new/old/hot + 加密楼 + 取证视图）
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
│   ├── aigc/  aisafety/  sentiment/  crisis/ # AIGC 标识（角标 + XMP）/ 三层安全闸（+ 跨轮拼装链检测）/ 舆情 / 心理危机
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

### 预留表 / 字段（功能未开发）
以下表与字段当前仅建表占位，**无业务代码**，请勿误用：

| 表 | 字段/表 | 状态 | 说明 |
|----|---------|------|------|
| users | `real_name` / `id_card` / `bank_card` | 预留 | 企业版实名认证与支付绑定，暂未启用 |
| vip_applications | 整表 | 预留 | VIP 会员申请体系，暂未开发 |
| earnings_records | 整表 | 预留 | 作者收益记录，暂未开发 |
| withdrawal_requests | 整表 | 预留 | 提现申请流程，暂未开发 |
