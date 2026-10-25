<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3.4-4FC08D?style=flat&logo=vuedotjs" alt="Vue">
  <img src="https://img.shields.io/badge/SQLite-3-003B57?style=flat&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/version-4.1.0-4F46E5?style=flat" alt="Version">
  <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-blue?style=flat" alt="Platform">
  <img src="https://img.shields.io/badge/CPU-x86%20%7C%20ARM%20%7C%20MIPS64%20%7C%20RISC--V-orange?style=flat" alt="CPU">
  <img src="https://img.shields.io/badge/license-AGPL%20v3-blue.svg" alt="License">
</p>

> 版本徽章与 `server/version/version.go`、`web/package.json` 必须三处一致（`make version` 会校验）；
> 发版时同步更新本徽章与下方「版本更新记录」。

<h1 align="center">📚 NVS — 去中心化网络小说平台</h1>

<p align="center">
  <strong>全平台 · 全CPU架构 · 下载即用 · 默认端口 8080</strong><br>
  聚焦高质量类型文学 · 作者自主定价 · 版权归属作者 · 文件系统存储
</p>

---

## ⚡ 为什么选 NVS？

**一个二进制文件，下载即运行。** 不需要装 MySQL，不需要配 Nginx，不需要 npm install —— 前端、后端、数据库全部内嵌。

> 首次启动自动创建 `.env` 配置文件（含数据库、端口、IPv4/IPv6），按需修改后重启即可。

| 对比 | 传统小说站点 | NVS |
|------|------------|-----|
| 部署 | 服务器 + 数据库 + 缓存 + 前端 | 一个二进制文件，下载即用 |
| 平台 | 通常仅 x86 Linux | **Windows / Linux / macOS 全平台** |
| CPU | 仅 x86_64 | **x86 · ARM · ARM64 · MIPS64 · RISC-V** |
| 默认端口 | 各组件端口不一 | **统一 8080** |
| 前端 | 需额外部署和配置 | **内嵌在二进制中** |
| 数据库 | 必须安装 MySQL/PostgreSQL | SQLite 自动创建，零配置 |
| 许可证 | 通常闭源 | **AGPL v3 —— 永远开源** |

---

## ✨ 特性

### 部署与运维
- 🌍 **全平台全CPU支持** — Windows / Linux / macOS × x86 / x64 / ARM / ARM64 / MIPS64 / RISC-V（10 个预编译目标；`windows/386`、`windows/arm`、`linux/mips64` 受限于底层 SQLite CGo 库暂不支持）
- 🚀 **下载即用** — 选择对应平台二进制文件，直接运行，零依赖安装（31.6 MB）
- ⚙️ **自动创建 .env** — 首次启动自动生成配置文件，自动检测本机 IPv4/IPv6 地址填入
- 🔌 **配置化绑定** — 支持配置 `BIND_IPV4` / `BIND_IPV6` / `ENABLE_IPV6`，灵活控制监听网卡
- 🌐 **启动时列出所有访问地址** — 扫描所有网卡，输出 `localhost` + 局域网 IP 的完整 URL 列表
- 🛡️ **Windows 防火墙自动配置** — 尝试自动添加 `netsh` 入站规则，管理员权限不足时输出命令行供复制
- 📦 **前端内嵌** — Vue 3 前端编译进 Go 二进制，单文件部署
- 🗄️ **SQLite 零配置** — 首次运行时自动创建数据库，无需安装或配置
- 🧭 **建站向导** — 首次运行进入可视化安装向导（环境检查 / 数据库连通测试 / 站点名与管理员设置），一次性安装密钥仅控制台显示一次
- 🔐 **数据库配置加密** — 数据库连接信息（含密码）经 AES-256-GCM 加密落盘 `secure-config.json`，不写明文 `.env`
- 🔄 **数据库在线热迁移** — SQLite ↔ MySQL 双向热切换，全程无需重启，逐表复制并上报进度
- 💾 **备份 / 恢复 / 定时备份** — SQLite `VACUUM INTO` 无损快照 / MySQL 全表 JSON 快照，支持按间隔定时调度
- 🔀 **站点路径前缀挂载（反向代理）** — 通过 `URL_PREFIX`（如 `/go-nvs`）将整站挂到任意子路径（`http://网关:8088/go-nvs/login`），供 nginx / 自建网关等反向代理使用；前端**自动感知前缀**（Vue Router base / axios baseURL / 资源绝对前缀 / 跳转），支持「保留前缀」「去前缀」两种转发模式，**前缀名任意配置、严禁硬编码**；反向代理仅改写 `<base href>` 时前端自动从 `document.baseURI` 推导实际挂载前缀
- ⚡ **静态资源强缓存** — Vite 产物 `assets/*`（文件名即内容指纹）→ `Cache-Control: immutable` 一年；`index.html` → `no-cache` 每次重新验证，发布新版本后浏览器立即拿到，杜绝重复全量下载 js/css
- 🏷️ **页脚开源署名** — 参考 alist「Powered by」风格，站底常驻「由 **NVS** 驱动 vX.Y.Z」并链接 GitHub 仓库（`HerryABU/nvs-community-web`），与 ICP/公安备案号同排展示；版本号统一管理（Go `version` 包 + 前端 `package.json` + README 顶部徽章，`/api/setup/status`、建站向导、健康页、启动日志同步一致）

### 用户系统
- 🔐 **JWT HttpOnly Cookie 认证** — `SameSite=Lax`，72h 过期
- 🔑 **bcrypt 密码哈希**
- 📧 **邮箱验证码** — 支持 QQ邮箱/Gmail/163 SMTP，6位数字验证码，10分钟有效
- 🧩 **滑块验证码** — `ENABLE_CAPTCHA` 开关控制
- 👥 **用户分级** — 普通用户（reader）/ 作者（author）/ VIP作者（vip_author）/ **警方（police）** / 仲裁员（arbitrator）/ 管理员（admin）；其中 **警方不可自注册**，只能由管理员在「用户管理 → 更多」提权，撤销/提权均写审计留痕
- 🪪 **个人中心统一入口** — 头像下拉只保留 5 项（个人中心 / 我的主页 / 管理员面板 / 警方取管台 / 退出登录）；更换头像、账号设置、钱包、API Key、私信、黑名单、特别关注、举报、模板设置、文档等 **14 个入口全部收进 `/me` 的「我的服务」**，按「账号与资料 / 互动与关注 / 资产与开放能力 / 外观与帮助」四组归类
- 📍 **注册/登录 IP 留存** — 记录 `registration_ip` / `last_login_ip` / `last_login_at`（176号令合规）
- 🕵️ **用户行为审计日志** — 登录成功/失败、登出、改密、注册全记录（含 IP / User-Agent）

### 作品创作
- 📝 **12 大分类** — 硬科幻 / 奇幻 / 推演文学 / 架空历史 / 现实主义 / 悬疑推理 / 实验文学 / 同人区 / 政治区 / 讽刺文学 / 泛二次元区 / 其他
- 🎨 **Cherry Markdown 编辑器（0.11.9 完整版）** — 统一渲染引擎（编辑器 = 阅读器 100% 同源），支持 Markdown / KaTeX 公式（含 physics 物理宏包 + mhchem 化学式）/ Mermaid 图表 / chemfig 化学结构图（SMILES）/ 颜色标记 · 一键切换富文本模式；阅读器 / 博客 / 评论 / 论坛浏览端与编辑器预览共用同一 Cherry 主题样式，章节标题以大字显示（h1 2em 居中，h1-h6 全阶统一）
- 📂 **文件系统存储** — 小说正文以 HTML 存于文件系统，数据库仅存路径索引
- 🔗 **内容可验证** — 每章自动计算 SHA256 哈希 + HMAC-SHA256 作者签名，前端可验证章节完整性
- 📤 **多格式导出** — ZIP / Markdown / EPUB / TXT 一键导出
- 📥 **多格式导入** — 支持 ZIP / Markdown / TXT 导入，自动解析章节结构，支持预览确认
- 📦 **批量导入** — 小说批量导入支持三种模式：**一个文件一章**（标题取文件名，默认）/ 自定义正则拆分（快捷规则：第X章 / Markdown 标题 / 分割线）/ 自动识别标题；可选项「章节标题加第N章前缀」；**编辑模式下导入/批量导入自动追加到当前作品末尾，不会新建书本**（单文件导入同样支持追加）；**博客批量导入**：多文件一键创建多篇博客

### 🤖 AI 工作台（对话 / 绘画 / 视频 / 智能体）

> 默认**关闭**（`ai_enabled=false`）—— 未开启时入口仍可见并标注「未开启」，管理员可就地开启并配置模型池。

- 🧭 **竖列式入口**（184px 左导航 + 56px 顶栏）：AI 对话 / 大模型广场 / 专业绘画 / 视频创作 / 灵感广场 / AI 智能体 / 思维导图 / 会员与额度，**打开即落地对话页**（不做九宫格门户）
- 💬 **标准对话页**：空态大标题 + 推荐词卡；圆角自适应输入框（聚焦描边）+ 行内「厂商/模型/长度」+ 圆形发送/停止；历史会话、代码块一键复制、**真正的重新生成**（先截断再重发，不会一问多答）
- 🧠 **思考型模型**（DeepSeek-R1 / GLM / Qwen thinking）—— 上游把正文放在 `reasoning` / `reasoning_content` 字段，两种命名都识别；推理过程进「思考过程」折叠块、正文不重复推送，`answer_in_reasoning` 时自动落正文
- 🎨 **专业绘画**：左 320 参数栏 + 右纵向作品流（每任务一卡：模型/时间/提示词/图片栅格/参考图数），**不自建「本次结果」区块**（结果就是流里最新一项）
- 🎬 **视频创作**：与绘画页同构；进行中有状态条（排队/生成 + 自动刷新），支持**停止等待（取消 + 原路退款）**与**失败重新提交**；刷新页面自动恢复轮询；终态判定统一走 `IsAiMediaTerminal`（含 `cancelled`）
- 🧩 **智能体 / 思维导图 / 灵感广场 / 提示词库**：智能体卡片墙、mindmap 面板、灵感广场（图文/视频）、提示词模板一键填入
- 🎛 **模型池与路由**：多厂商协议（OpenAI 兼容 / Anthropic / Gemini / MJ / Kling / Coze…）、密钥池轮询与失败熔断、**路由策略**（最便宜/最快/按场景）+ 故障自动降级、成本闸门与按次计费、`thinking` 标记、15 个开箱预设（`base_url` 为空的预设必须先填中转地址）
- 🔑 **BYOK**：用户可绑定自己的模型 Key（加密存储，仅本人可用），不占站内额度
- 📊 **成本与漏斗台**：调用量 / 成功率 / 首字延迟 / 花费排行 / 模型维度漏斗

### 📖 阅读体验（精读 · 朗读 · 评论）

- 🔤 **阅读器**：字号三档（小/中/大，`localStorage` 记忆）、顶部 2px 阅读进度条、目录抽屉、上/下一章、书签与阅读进度同步书架
- ✍️ **划线批注**：选中正文 → 浮动按钮 → 写批注（`reading_notes.type=annotation`），本章划线面板可列出「我的 + 他人公开」批注，点击定位并闪烁；同一句多处出现按次序落位，不会互相覆盖；支持「仅自己可见」
- 🔊 **语音朗读**：服务端合成（`POST /api/ai/speech`），按句切段（≤1800 字/段、单次 ≤8 段）顺序播放，可随时停止，离开页面自动释放音频；未配置 audio 模型时给出明确提示而非静默
- 💬 **楼层式评论区**（公众号评论区 + Discuz 楼中楼）：回复主内容 = 开新楼（`floor_no` 递增），回复楼内某条 = 挂靠该楼并显示「X 回复 Y」可折叠
- 🔃 **评论排序**：`new`（最新楼在前，默认）/ `old`（最早在前，追更补读）/ `hot`（楼内回复多的在前，排除已删除回复）；排序只影响「楼」的顺序，**楼内回复恒按时间正序**
- 🎚 **评论密度**：紧凑 / 舒适一键切换（`localStorage` 记忆），四态齐全（骨架 / 空态引导 / 错误重试 / 内容）
- 🕸 **评论网（社交图谱）**：按「谁回复了谁」构建力导向图，识别互刷、抱团与孤岛
- 🔒 **加密楼**：楼可设访问密码（入口限制，内容不加密存储），未解锁者只见占位、楼内回复不外泄；凭 `X-Floor-Password` 头解锁，警方/管理员豁免

### 👥 群组与实时消息

- 🏘 **群组**：群号/邀请码可搜可预览、成员管理、群公告、群内发帖与回帖、未读红点
- 📡 **SSE 实时推送**（`realtime.Hub`，每业务域独立实例，不蹭「狂暴模式」的 DefaultHub）：`post.created` / `post.reviewed` / `reply.created` / `announcement` / `member.joined` / `member.left`
- 🔔 **未读计数**：`MarkGroupRead` 记 `last_read_post_id`，判定走 `id > last_read_post_id`（ID 优先、时间兜底且加容差）——纯时间戳口径在「同一毫秒入群 + 发帖」时会永久漏掉一条
- 💌 **私信**：一对一会话、撤回（2 分钟内）、单方删除、未读数、互相关注才能发消息（陌生人可发 2 条）；支持 `/user/messages?peer=<id>` 直达会话
- 📣 **消息中心**：评论回复 / 帖子回复 / 作品评论 / 博客评论聚合，导航栏铃铛未读徽标，`nvs:unread` 事件跨组件同步

### 🎭 话本 · 协作 · 版本控制 · OC 模式

- 💬 **话本气泡体**：散文体 / 话本体 / 章节级混排三种形态；气泡对话支持长按互动（复制、引用为评论、存为书签）、表情与图片、AI 模板生成
- 👥 **多人协作**：按作品邀请协作者、主笔转让（权限与署名同步）、协作台集中查看
- 🗂 **章节版本控制**：每次保存留版本，可视化 diff 与回滚；默认内置实现，`make build-gitvcs` + `NVS_VCS_BACKEND=git` 可切真实 git 仓库后端
- 🎭 **OC 模式群组**：角色卡（性格/口吻/开场白）、世界书（设定注入）、多智能体角色扮演、一键把对话发布为话本章节
- 🧾 **提示词模板引擎**：模板 CRUD + 变量白名单；上下文自动注入（作品/大纲/角色/摘要/前文/样本），**客户端变量优先**，只回传模板实际用到的变量

### 💎 会员与额度

- 💳 **代币经济**：充值档位（纯充值 / 充值+会员天数）、代币消费（AI 调用、绘画、视频）、余额与流水
- 👑 **会员套餐**：`PaymentTier{fen,bonus,label,days,badge,features}`；下单时把 `member_days` 记成**快照**，事后改档位不影响已下单订单；到账与会员延长在**同一事务**（`MarkOrderPaid → extendMembershipTx`），续费起点取 `max(now, 到期)`
- 🎁 **每日免费 AI 额度**：游客 / 注册 / 会员三档，会员为注册用户 ×2；**站长按人覆盖优先于会员倍数**
- 🧾 **会员页**：3 列套餐 + 权益对比表（游客/注册/会员，会员列整列高亮），数字全部来自 `/api/membership`，站内不存在的权益不写进表
- 🔁 **支付通道**：在线支付 + 模拟收银台（`/pay/mock`）便于本地联调；风控台可配「大额人工复核 / 短时高频拦截 / 退款率上限」

### 社区互动
- 💬 **评论系统** — 章评/书评双轨，**楼层模型**（新楼 + 楼中楼折叠）、排序（最新/最早/最热）、密度（紧凑/舒适）、加密楼、评论网社交图谱、30s 新评论提示（不打断阅读）；IP 限流 + 违禁词 + 黑名单 hide/block 双模式
- ⭐ **多维度评分** — 类型完成度 / 叙事质量 / 思想深度 / 社区口碑 / 更新稳定性
- 📚 **书架** — 收藏追读，自动记录阅读进度（与阅读器双向同步）
- 🏠 **论坛双轨制** — 公共论坛广场 + 作品专属子论坛 + 作者专属论坛
- 👑 **作者工作台** — 仪表盘（多指标可切换图表：章节增长/字数增长/评论趋势/收藏/阅读·折线⇄柱状二选一）、作品管理、收益统计、提现申请
- 📝 **作者博客** — 认证作者可发布博客文章，支持置顶、**点赞**、评论，**CSDN 式一/二级分类**（管理员可任意管理），MD 文章右侧**可折叠目录**（#~######）；作者删除的内容进入**回收站**可自行恢复（区分管理员封禁，被封禁仅管理员可恢复、管理员可在「回收站监管」查看全站用户自删内容）
- 🛡️ **管理员面板** — 数据大屏（2×2 ECharts 图表）、社区动态仪表盘、用户管理、分类管理、举报处理、站长配置
- 🏘️ **社区动态** — 管理员可查看最近注册用户、最新作品、最近评论、论坛新帖

### 安全与合规
- 🧱 **敏感区三级隔离墙** — 系统级（管理员配置敏感分区）+ 论坛级（敏感论坛自动触发）+ 作者级（自定义警告语），3~5步确认弹窗含手动输入，动态配置步骤数/警告语/跨域额外步数
- 🚫 **IP 黑名单** — 管理员可封禁 IP，支持过期时间
- 🚦 **IP 限流** — 滑动窗口算法，内存存储，登录 5次/60s，评论 20次/60s
- 🔑 **四层身份 API Key 体系** — 站长 / 作者 / 读者 / 联邦 四种身份 Key + 匿名/爬虫通道，各自独立配额（管理员全局可调）。配额按身份从高到低：**带 Key** ≫ **匿名（无 Key）** > **爬虫（识别命中）** > **已知名头爬虫（直接 403 阻断）**；无 Key 的匿名访问只给低配额，被识别为爬虫后更低，**爬虫脚本想提速的唯一途径是申请并携带 API Key**（进而可审计、可吊销）。**已知名头爬虫阻断**（AI 训练爬虫 GPTBot/MetaWebIndexer/ClaudeBot/Bytespider 等 + 商业 SEO 采集器）遇即 403，名单管理员可追加（如某些搜索引擎爬虫），主流搜索引擎默认放行。**爬虫识别**（无头 UA / 浏览器 UA 缺 Sec-Fetch 头伪装检测 / 单 IP 高频行为降级）进入比匿名更低的爬虫配额；用户可在管理员角色上限内自建/自管 Key（配额/过期/IP 白名单），完整 Key 仅创建时展示一次、库中只存 SHA-256 哈希
- 🤝 **联邦 Key 双向可见** — 两站互换 API Key：本站签发的 Key 对方同步时携带，被链接的站可在入站记录中识别**对方用的是哪个 Key、是谁签发的**；出站同步自动携带对方签发的 Key（AES-256-GCM 加密落库），免除匿名限流干扰
- 🧹 **XSS 防护** — bluemonday 白名单净化 + 动态正则规则引擎（管理员后台可维护、可测试）+ 正则移除危险标签和事件处理器
- 🚫 **违禁词库** — 管理员后台动态维护，**三种动作**：标记（内容正常发布但进入待审列表，管理员确认违规[可连带软删留证]/放行）/ 删除（直接移除匹配内容）/ 替换（替换为指定文本）；**双匹配模式**：关键词（字面匹配自动转义，填词即用）与正则（Go RE2 全方位：`(?i)` 忽略大小写、`(?m)` 多行、`\p{Han}` Unicode 属性）；**可配置适用范围**（章节 / 评论 / 论坛 / 博客 / 全部）；规则即时生效无需改代码，内置实时测试工具，mark 命中自动写入待审记录（去重）
- 🔏 **内容签名** — HMAC-SHA256 作者签名，章节完整性可验证
- 📋 **举报系统** — 支持举报作品/评论/帖子，管理员处理
- 🗑️ **内容软删除** — 8 张内容表软删除（`deleted_at`），删除不物理移除、正文归档到 `.deleted/`，随时可追溯原始内容；**所有统计均排除软删除数据**（分类分布 / 章节趋势 / 字数趋势 / 评论趋势等管理端与作者端统计全部 `JOIN novels` + `deleted_at IS NULL` 双过滤，已删除作品的章节/字数/评论不再计入）
- 📊 **访问日志关联 user_id** — JSON 一行含 `user_id`/方法/路径/IP/状态/耗时，可追溯到具体用户；**按小时切分**（每天内每小时一个文件），且 **request / database 分文件夹**（`log/requests/<日期>/<小时>.log` 与 `log/database/<日期>/<小时>.log`）
- 🔐 **日志完整性保护** — 每小时文件每日 SHA256 校验 + gzip 归档 + 只读权限（0444）+ 6 个月保留
- 🛡️ **内容安全扫描（23 条规则）** — 提权 / Shell注入 / 木马 / 挖矿 / 僵尸网络 / 数据窃取 / 钓鱼，上传即自动审查
- 🚨 **内容抢救台** — 二级密码 + 短时 token 应急窗口，粘贴 URL 即可预览全部评论/回复并物理硬删除，全程审计留痕
- 🧾 **日志敏感信息脱敏** — 生产模式自动脱敏 Cookie / Authorization / body / query 中的敏感字段（密码类固定 `******`，其余可配置掩码与字段列表），管理后台可视化配置 + 预览工具，运行时热更新无需重启（176号令合规）
- 🛡️ **CSRF 防护** — 登录态写操作强制校验 `X-Requested-With` 头，前端 axios 统一附加，跨站表单/脚本无法伪造写请求
- 🔑 **登录账号级锁定** — 单账号 5 次失败 / 15 分钟锁定（配合 IP 限流双层防爆破）
- 🔐 **JWT 随机密钥** — 默认自动生成随机密钥并持久化 `data/.jwt_secret`，杜绝公开默认密钥伪造管理员 Token
- 📢 **公告栏** — 站点公告管理端 CRUD + 首页公告条 + 公告列表/详情弹窗（置顶优先）
- 🧭 **176号令合规工具箱** — 算法推荐透明度（公开说明 + 用户个性化推荐关闭权）、联网备案引导（ICP/公安备案字段 + 页脚展示）、等保/关基自测定级、漏洞整改闭环管理、检查材料一键导出（日志 ZIP / 用户 CSV / 合规自查报告）
- 🕵️ **蜜罐体系（四档可调）** — 诱饵路径 + 影子认证 + 验证页与浏览器指纹 + 取证包 + 规则反哺；**全部内嵌主程序**（不再产出第二个二进制），每次命中记录来源 IP / UA / 载荷 / 处置结果
- 🧱 **隔离墙访问仪表盘** — 谁在什么时候试图进入哪个敏感分区、从哪个 IP / UA 来（`ZoneAccessLog` 可视化 + 分页筛选 + 聚合统计），用于举证与反驳
- 📡 **网络诊断与攻击流量面板** — 独立 `WebAccessLog`（中间件异步批量写入），支持 Top IP / Top 路径 / 状态码分布 / UA 家族 / 爬虫原因分布；TTL 以 `X-Forwarded-For` 跳数近似并**在界面上标注该限制**
- 🚨 **警方取管台（police 角色）** — 只看取证相关：软删除留证全量查看（按类型筛选 + 关键词 + 分页）、单条凭证下载、**全量取证包导出（二次确认并写明含个人信息、仅限依法调取）**、论坛巡查（含隐藏/加密/已删除）；**无**普通管理权，角色只能由管理员提权
- 🧯 **AI 内容可追溯** — 章节/博客记录「AI 参与」标记与生成模型名，评论可 @ai 召唤助手（配额可控、每日上限可关）

### 站点互通
- 🌐 **联邦站点** — 支持远程站点互联与跨站作品同步（模仿 alist）
- 🔗 **公开 API** — `/api/site-info`、`/api/federated/novels`、`/api/federated/sites`
- 📡 **联邦入站记录** — 记录「谁连接了我」（对方站点名/地址/IP/User-Agent/访问次数），被连方可见连接来源
- 🔁 **联邦同步自报身份** — 同步时向对方自报本站名与地址，便于对方追溯

### 📱 移动端浏览支持
- 🍔 **响应式导航栏** — 窄屏自动收起为汉堡菜单 + 侧滑抽屉（分类 / 论坛 / 博客 / 书架 / 搜索 / 账号全入口）
- 📖 **阅读器移动优化** — 工具栏图标化、正文大字大行距、底部全宽翻页按钮、目录抽屉视口适配
- 📐 **全站响应式** — 首页 / 作品详情 / 分类 / 书架 / 论坛 / 博客 / 消息 / 登录注册等 30+ 页面窄屏布局适配，卡片双列、行式布局纵向堆叠
- 🎛️ **控制面板移动适配** — 管理后台（15 个 admin 页面）+ 作者工作台 / 管理员仪表盘 + 作品 / 章节 / 博客编辑器窄屏可用：工具条自动换行、表单栅格单列、步骤条可横滚、输入框限宽、统计卡双列
- 💬 **弹窗限宽** — 所有 Dialog / Drawer 在窄屏自动限制在视口内，杜绝横向溢出
- 📊 **表格横滚** — 管理后台与列表表格窄屏自动启用横向滚动
- 🧩 **触控友好** — 翻页按钮 / 操作按钮加大、底部安全区适配（`env(safe-area-inset-bottom)`）、表单标签置顶、分页换行居中

### 🎨 自定义 HTML-UI 美化

NVS 允许作者通过自定义 HTML 模板深度美化作品展示页面和作者主页。模板在独立 iframe 沙盒中运行，安全隔离。

**模板类型**

| 类型 | 显示位置 | 说明 |
|------|----------|------|
| 📖 阅读模板 | 小说阅读页（章节正文上方/下方） | 可调用平台小说 API，支持按钮/表单等交互控件 |
| 👤 作者展现模板 | 作者主页 `/author/:id` | 纯展示或交互，不依赖具体作品 |

**平台 JavaScript API（`window.NVS`）**

| 方法 | 说明 |
|------|------|
| `window.NVS.getNovelData(novelId?)` | 获取小说元数据+完整章节列表，返回 `Promise` |
| `window.NVS.getCurrentChapter()` | 从 iframe URL 解析当前阅读章节号 |
| `window.NVS.sendMessage(type, data)` | 通过 `postMessage` 向父页面发送消息 |
| `window.addEventListener('nvs-message', fn)` | 监听父页面发来的消息 |

**沙盒安全级别**

| 级别 | iframe sandbox 属性 | 适用 |
|------|---------------------|------|
| strict（只读） | `allow-scripts allow-same-origin` | 纯展示 |
| interactive（交互） | + `allow-forms allow-popups` | 含按钮/表单 |

**模板设置入口**：**个人中心 → 我的服务 → 模板设置**（`/author/templates`）→ 分 Tab 管理阅读模板与作者展现模板
（v3.21.1 起头像下拉只保留 5 项，其余入口统一收进个人中心）

---

### 📦 自定义 HTML 托管（扩展应用）

上传 ZIP 包将完整的 HTML 应用部署到 NVS 平台，在独立沙盒中运行。支持任意前端技术栈（Vue/React/原生 JS），可包含 WASM 模块。

**上传流程**

```
编写应用（HTML+CSS+JS+WASM）→ 打包 ZIP → 上传到扩展应用 → 自动解压+安全扫描 → 沙盒运行
```

**ZIP 包规范**

| 限制项 | 值 |
|--------|-----|
| 压缩包大小 | ≤ 20 MB |
| 解压后总大小 | ≤ 50 MB |
| 压缩比 | ≤ 100:1（防 ZIP 炸弹） |
| 文件数量 | ≤ 500 |
| 允许扩展名 | .html .css .js .wasm .json .png .jpg .svg .woff2 等 |
| 入口文件 | 默认 index.html，可手动选择 |

**运行方式**

| 方式 | 路径 | 说明 |
|------|------|------|
| 完整运行 | `/app/:id` | 全屏沙盒，虚拟路径隐藏真实存储 |
| 弹窗预览 | 卡片「预览」按钮 | 对话框内 iframe 预览 |
| 广场运行 | 广场卡片点击 | 新标签页全屏运行 |

**HTML 应用广场**

作者设置项目为「公开」后，项目出现在广场 Tab 中。上传 ZIP 时自动提取首张图片作为缩略图。作者可设置「允许下载」，访客可下载原始 ZIP 包。

**端口代理**

扩展应用可分配独立端口（49152-65535 高位优先），通过命名路径 `/sandbox/proxy/{作者ID}/{项目名}/` 反向代理访问，端口号不在 URL 中暴露。

**虚拟文件系统 (VFS)**

沙盒应用可通过 REST API 在内存中读写数据，完全隔离于真实文件系统：

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/api/sandbox-vfs/:id/write` | 写入 `{key, value}`，单值≤1MB，总量≤10MB/100键 |
| GET | `/api/sandbox-vfs/:id/read?key=` | 读取键值 |
| GET | `/api/sandbox-vfs/:id/list` | 列出所有键 |

**多层安全防护**

1. **上传前** — 23条内容安全扫描（提权命令/Shell注入/木马/挖矿/僵尸网络/数据窃取/钓鱼）
2. **解压中** — ZIP 炸弹检测（压缩比+总大小+文件数+路径穿越）
3. **解压后** — 目录锁定（文件→0444只读，目录→0755可写）
4. **运行时** — 写入拦截中间件（允许数据文件，禁止脚本文件）
5. **通信** — CSP + iframe sandbox 双重隔离
6. **路径隐藏** — `/app/:id` 虚拟路径，沙盒内无法探测真实存储位置

**WASM 支持**

- 上传时勾选「允许 WASM」
- 系统自动设置 `Cross-Origin-Embedder-Policy: require-corp`（启用 SharedArrayBuffer）
- 自动设置 `Cross-Origin-Opener-Policy: same-origin`
- CSP 启用 `'wasm-unsafe-eval'`
- `.wasm` 文件返回正确的 `Content-Type: application/wasm`

**展开/收起控件**

Markdown/富文本编辑器支持 `[expand title="标题"]...[/expand]` 语法，渲染为可点击展开的 `<details>` 折叠块。

---

## 🛠 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.25 · Gin · GORM (SQLite/MySQL) |
| 前端 | Vue 3.4 · Vite 5 · Element Plus 2.7（按需导入） · TypeScript 5.5 · TipTap · Cherry Markdown 0.11.9（完整版，编辑器/阅读器同源引擎） · ECharts 6 · KaTeX (physics/mhchem) · Mermaid · smiles-drawer (chemfig) · three.js / 3d-force-graph（评论网图谱） |
| AI | 多厂商协议（OpenAI 兼容 / Anthropic / Gemini / MJ / Kling / Coze）· SSE 流式 · 密钥池与熔断 · 路由策略与自动降级 · 思考型模型 reasoning 透传 · 语音合成 |
| 数据库 | SQLite（默认，零配置） / MySQL 8.0（支持在线热迁移） |
| 缓存 | Redis 7（可选，MySQL 模式推荐） |
| 实时 | SSE（每业务域独立 Hub；单向广播、过反代简单、Cookie 鉴权无需 token 进 URL） |
| 存储 | 文件系统（HTML + JSON） + 内存 VFS（沙盒虚拟文件系统） |
| 扩展 | 自定义 HTML-UI 模板 · ZIP 托管 · WASM · 端口代理 · 虚拟路径 |
| 安全 | 内容扫描（23条规则）· 动态 XSS 规则引擎 · 违禁词库（标记/删除/替换 + 关键词/正则）· 软删除（8表）· 审计日志 · 访问日志关联 user_id · 日志完整性保护 · 日志脱敏 · 滑块验证码 · CSRF · ZIP炸弹检测 · CSP · iframe sandbox · 目录锁定 · 数据库配置 AES-256 加密 |
| 质量 | `make check`（SPA 白名单 + 组件解析审计 + go build/vet/test + 前端 build）· Playwright-core 四档宽度与功能级真浏览器验收 · 仓库内 git hooks · GitHub Actions（CI + 多平台 Release） |
| 构建 | Go `embed` 前端 → 单二进制文件 |

---

## 📦 快速开始

### 方式一：直接运行（推荐）

从 [Releases](https://github.com/HerryABU/nvs-community-web/releases) 下载对应平台的文件：

| 平台 | 文件 | 适用设备 |
|------|------|----------|
| Windows | `nvs-win-x64.exe` | 64位 Windows（最常用） |
| Windows | `nvs-win-arm64.exe` | ARM64 Windows（Surface Pro X 等） |
| Linux | `nvs-linux-x64` | 64位 Linux（x86_64） |
| Linux | `nvs-linux-x32` | 32位 Linux |
| Linux | `nvs-linux-arm64` | ARM64 Linux（树莓派4/5、鲲鹏等） |
| Linux | `nvs-linux-arm32` | ARM32 v7 Linux（树莓派3/4） |
| Linux | `nvs-linux-armv6` | ARM32 v6 Linux（树莓派1/2/Zero） |
| Linux | `nvs-linux-riscv64` | RISC-V 64（VisionFive、LicheePi 等） |
| macOS | `nvs-mac-x64` | Intel Mac |
| macOS | `nvs-mac-arm64` | Apple Silicon Mac (M1/M2/M3) |

> 注：`windows/386`、`windows/arm`、`linux/mips64` 三个目标因 `modernc.org/sqlite` CGo 库暂不支持对应架构而不可用。

下载后运行即可：

```bash
# Linux / macOS
chmod +x nvs-linux-x64
./nvs-linux-x64

# Windows
nvs-win-x64.exe
```

首次启动自动创建 `.env` 文件，并进入**建站向导**。控制台会打印**一次性安装密钥**（仅显示一次），浏览器打开日志中列出的地址完成安装：

```
  首次运行：进入建站向导模式
  向导地址: http://localhost:8080/install
  安装密钥（一次性，仅显示一次，切勿泄露）: xxxxxxxxxxxxxxxx
```

向导中依次完成：环境检查 → 数据库设置（SQLite 零配置 / MySQL）→ 站点名称与管理员账号。安装完成后数据库连接信息会 **AES-256-GCM 加密落盘**（不写明文 `.env`），随后正常启动：

```
  可访问地址：
    本机:   http://localhost:8080
    本机:   http://127.0.0.1:8080
    局域网: http://192.168.1.3:8080        ← 局域网内其他设备用这个
```

> 无需安装 Go、Node.js、MySQL、Redis、Nginx。SQLite 数据库自动创建，前端已内嵌于 exe 中。

### 方式二：Docker

```bash
git clone https://github.com/HerryABU/nvs-community-web.git
cd nvs
cp .env.example .env
docker-compose up -d
```

### 方式三：从源码构建

```bash
git clone https://github.com/HerryABU/nvs-community-web.git
cd nvs

# 一键构建（Windows）
build.bat

# 或手动：
cd web && npm install && npm run build && cd ..
xcopy /E /I /Y dist server\dist
cd server && go build -ldflags="-s -w" -o nvs-server.exe . && cd ..
```

> **构建说明**：前端 `vite build` 输出到项目根目录 `dist/`，再由 `build.bat` 同步到 `server/dist/` 供 Go `embed` 嵌入。

---

## 🔐 默认管理员

> 通过**建站向导**安装时，可在向导中自定义管理员账号与密码（推荐）。

若未自定义（如直接使用已有数据库初始化），系统自动回退创建默认管理员：

| 字段 | 值 |
|------|-----|
| 邮箱 | `admin@nvs.local` |
| 密码 | `admin123` |

> ⚠️ 登录后请立即修改密码！

---

## 🧑‍💻 源码开发

### 环境要求

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | ≥ 1.25（见 `server/go.mod`） | 后端编译 |
| Node.js | ≥ 20（CI 用 22） | 前端构建（Vite 5） |
| npm | ≥ 9 | 依赖管理 |
| Git | ≥ 2.9 | 须支持 `core.hooksPath`（仓库内 hooks 依赖它） |
| make | 可选 | `make check/dist/build` 等便捷目标；**Windows 的 Git Bash 通常不带 make**，等价命令见各小节 |

> 真浏览器验收脚本需要 `playwright-core` 与已下载的 Chromium（不在 `web/package.json` 里，
> 由运行环境提供；脚本默认从 `%LOCALAPPDATA%\ms-playwright` 找浏览器，可用 `NVS_CHROME` 指定）。

### 本地开发

```bash
# 终端 1：启动 Go 后端（监听 :8080）
cd server
go run main.go

# 终端 2：启动 Vue 前端（监听 :5173，API 自动代理到 8080）
cd web
npm run dev
```

浏览器打开 **http://localhost:5173** 进行前端开发（热重载）。Vite 自动将 `/api` 请求代理到 `localhost:8080`。

### Git hooks（提交门禁，新克隆必做一次）

hooks 随仓库走（`.githooks/`），不需要问"你本地装没装"：

```bash
bash .githooks/install.sh        # Linux / macOS / Git Bash / WSL
# 或 Windows PowerShell
powershell -ExecutionPolicy Bypass -File .githooks\install.ps1
# 或
make hooks
```

| hook | 检查内容 | 耗时 |
|---|---|---|
| `pre-commit` | 禁提交 `.env` / `data/` / `dist/` / `*.exe` / 证书、>2MB 大文件、冲突标记、`gofmt`、**前端硬编码 `/go-nvs`**、**疑似明文密钥**、`debugger` | < 2s |
| `commit-msg` | 约定式提交 `type(scope): 说明`，首行 ≤120 字符 | < 0.2s |
| `pre-push` | `go build` + `go vet` + `go test ./...`（前端由 CI 兜底，`CHECK_WEB=1` 可强制本地构建） | 1~3 min |

为什么 commit 阶段不跑编译：提交是高频动作，3 分钟的门会让人人 `--no-verify`，等于没有门。
提交只拦"事后无法挽回"的错（密钥进历史、二进制进历史、前缀硬编码），正确性验证放在 push 与 CI。

需要绕过时（请别养成习惯）：

```bash
SKIP_HOOKS=1 git commit -m "..."   # 全部跳过
SKIP_TESTS=1 git push              # push 只 build + vet
CHECK_WEB=1 git push               # push 额外构建前端
```

详见 `.githooks/README.md`。

### 已提供的构建目标

```bash
make check        # 本地等价 CI：SPA 白名单 + 组件审计 + 后端 build/vet/test（nodist 轨）+ 前端 build
make check-all    # check + 同步 embed + 产物自检 + 前端产物密钥扫描
make dist         # 只构建前端并同步到 server/dist（embed 源）
make dist-check   # 校验 server/dist 有 index.html 且资源数正常（防"空 embed 打进 exe"）
make version      # 校验 version.go 与 package.json 版本一致
make hooks        # 启用 .githooks
make test         # 后端单测（nodist 轨，**不需要**先构建前端）
make test-embed   # 带真实 embed 产物的单测（需先 make dist）
make build        # 单平台（当前系统）产出 nvs-server.exe
make build-gitvcs # 章节版本控制走真实 git 仓库（-tags gitvcs，运行时 NVS_VCS_BACKEND=git）
make build-gm     # 国密轨（-tags gm，依赖 go.gm.mod）
```

> **构建顺序不能反**：`npm run build`（产出 `dist/`）→ 同步到 `server/dist/` → `go build`。
> `//go:embed all:dist` 是**编译期**打包，顺序反了会把旧前端（或空目录）嵌进 exe。

> 🏷 **两条编译轨（`nodist` 标签）**：`server/dist` 是构建产物、不入库，而 `//go:embed all:dist`
> 要求目录非空 —— 所以**全新 clone 里直接 `go build ./...` 会报 `pattern all:dist: no matching files found`**。
> - **发布轨**（默认，不带标签）：必须先 `make dist`，产出物才内含前端（`make build` / Release / 内嵌冒烟用它）
> - **业务轨**（`-tags nodist`）：嵌入一个提示页，只用于 `go build` / `go vet` / `go test`（CI 后端 job、
>   `make check` / `make test`、pre-push 钩子都用它）；该轨产物启动时会打印醒目警告，**不可作为发布物**
>
> 两条轨并存还配了一道防回归：CI 会额外用「最小 dist 占位 + 不带标签」编译一次，确保 nodist 开关没把发布轨弄坏。

### 质量门禁与验收脚本

`scripts/` 下是**可重复执行**的静态检查与真浏览器验收脚本（不是"我本地跑过一次"的口头结论）：

| 脚本 | 查什么 | 何时跑 |
|---|---|---|
| `scripts/check-spa-roots.sh` | 前端**路由首段**是否全部登记到 `server/main.go` 的 `knownSPARoots`（漏登记 = 点链接能进、刷新 404） | `make check` / CI |
| `scripts/audit-unresolved-components.mjs` | 模板里用了、`<script setup>` 里没导入的组件（Element Plus 按需导入只解析 `El*`，`<ArrowDown />` 少了 import 会**静默不渲染**） | `make check` / CI |
| `scripts/audit-admin-pages.mjs` | 后台页四态体检（表格 / StateBlock / `#empty` / 骨架 / 分页 / 筛选 / 错误态），`--strict` 有表格无四态即退出码 1 | 改后台页后 |
| `scripts/fix-table-empty-slots.mjs` | 清理**列级 `#empty` 死插槽**（`el-table-column` 没有 empty 插槽，写了永远不渲染）；`--write` 才实改 | 需要时手动 |
| `scripts/scan-frontend-secrets.sh` | 前端产物里是否混入密钥/令牌 | `make check-all` / CI |
| `scripts/browser-smoke.mjs` | 真浏览器 **四档宽度（375/768/1024/1440）**回归：无横向溢出、关键元素存在且可见、`/api/*` 无失败、console 无错误；截图 + `report.json` 落盘 | 改前台页面后 |
| `scripts/browser-func.mjs` | **功能级**回归：保存资料并回读落库、作者主页四分栏可切、评论排序真带 `sort=` 参数、密度切换真改 DOM 高度、朗读真请求 `/api/ai/speech` 且失败有可读提示 | 改交互逻辑后 |
| `scripts/browser-me-center.mjs` | 个人中心专项：下拉只剩 5 项、13 个入口已迁出、`/me` 服务链接逐个直达不 404 不弹登录、头像上传落点 | 改导航/个人中心后 |

三个浏览器脚本都用 `playwright-core` + 已登录 cookie：

```bash
# 1) 登录拿 cookie（测试实例若开了滑块，需临时把 captcha_enabled 置 false，跑完改回）
curl -s --noproxy '*' -c /tmp/nvs_cookie.txt \
  -H 'Content-Type: application/json' -H 'X-Requested-With: XMLHttpRequest' \
  -d '{"email":"admin@nvs.local","password":"你的密码"}' \
  http://127.0.0.1:8000/api/auth/login

# 2) 跑验收（Windows 下 cookie 路径要传 Windows 绝对路径，Node 不认 MSYS 的 /tmp）
NVS_COOKIE="C:/Users/<你>/AppData/Local/Temp/nvs_cookie.txt" \
NVS_BASE=http://127.0.0.1:8000 node scripts/browser-smoke.mjs
```

> 为什么必须有真浏览器：`vite build` 成功 **≠** 页面能跑。本仓库踩过的坑里，
> 模板引用未定义变量导致整页白屏、图标漏 import 静默消失、`overflow` 加错容器导致头像被挤出视野，
> 全都是构建期无感的。另：非 `/api` 的 404 只会产生一条不带 URL 的 `Failed to load resource`，
> 静态检查与肉眼都定位不到。

### 自动构建（GitHub Actions）

| workflow | 触发 | 做什么 |
|---|---|---|
| `ci.yml` | push 到 `v4` / `v3` / `main` / `master`、PR、手动 | ① 后端 `build` + `vet` + `test` + `gitvcs` 标签轨（均走 `nodist` 轨，不依赖前端产物）+ 发布轨占位编译；② 前端 `npm ci` + `vite build`；③ **内嵌产物冒烟**（拼出单 exe，真启动，断言首页 200、off 档诱饵 404、日志无 panic）；④ 推主干产出 `linux-amd64` 快照 artifact |
| `release.yml` | push tag `v*` | 校验 **tag = version.go = package.json**；多平台交叉编译（Win x64/ARM64、Linux x64/ARM64/ARMv7、macOS Intel/ARM64）；打包 zip/tar.gz + `SHA256SUMS`；创建 GitHub Release |

发布流程（版本号三处同步后打 tag）：

```bash
# 1) 改版本（三处必须一致）
#    server/version/version.go  var AppVersion = "x.y.z"
#    web/package.json           "version": "x.y.z"
#    README.md                  顶部 version 徽章 + 版本更新记录新增段落

# 2) 本地先过一遍
make version && make check-all

# 3) 提交 + 打 tag（tag 必须与版本一致，否则 release 直接失败）
git add -A && git commit -m "chore(release): x.y.z <摘要>"
git tag vx.y.z && git push origin v3 --tags
#   tag 命名两种都认：v3.20.1（推荐）或 3.20.1（兼容本仓库历史 tag）
```

Release 里的二进制**已内嵌前端**，解压即用，无需 Node / Go 环境；Windows 包为 zip，其余为 tar.gz，附 `SHA256SUMS` 与各自 `.sha256`。

### 全平台交叉编译

项目提供 `build-all.ps1` 脚本，自动为 **13 个目标平台** 生成预编译二进制文件（本地发布用；CI 的 Release 覆盖 7 个主流平台）：

```bash
# Windows（需 PowerShell）
./build-all.ps1
```

输出到 `release/` 目录：

```
release/
├── nvs-win-x64.exe          # Windows 64-bit (x86_64)
├── nvs-win-arm64.exe        # Windows ARM64
├── nvs-linux-x32            # Linux 32-bit
├── nvs-linux-x64            # Linux 64-bit
├── nvs-linux-arm32          # Linux ARM32 (v7)
├── nvs-linux-armv6          # Linux ARM32 (v6 — 树莓派1/Zero)
├── nvs-linux-arm64          # Linux ARM64 (树莓派4/5)
├── nvs-linux-riscv64        # Linux RISC-V 64
├── nvs-mac-x64              # macOS Intel
└── nvs-mac-arm64            # macOS Apple Silicon
```

手动单目标编译：

```bash
cd server
GOOS=linux   GOARCH=arm64  go build -ldflags="-s -w" -o nvs-linux-arm64 .
GOOS=windows GOARCH=amd64  go build -ldflags="-s -w" -o nvs-win-x64.exe .
GOOS=darwin  GOARCH=arm64  go build -ldflags="-s -w" -o nvs-mac-arm64 .
GOOS=linux   GOARCH=riscv64 go build -ldflags="-s -w" -o nvs-linux-riscv64 .
```

> **关于 `embed.go`**：`//go:embed all:dist` 将 `server/dist/` 全部嵌入二进制。**前端必须先 `npm run build`，否则编译失败。**

---

## ⚙️ 配置参考（.env）

启动时自动生成（模板见 `server/config/config.go` 的 `defaultEnvTemplate`），也可复制仓库根的 **`.env.example`** 改名 `.env` 后填写。

> 密钥类项在示例文件中**留空**：留空时启动会自动生成/派生（JWT 密钥落盘 `data/.jwt_secret`）。生产环境请显式填写。

### 数据库
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DB_DRIVER` | `sqlite` | 数据库驱动（`sqlite` / `mysql`） |
| `DB_PATH` | `data/nvs.db` | SQLite 文件路径（**装 DLL/容器挂载卷时务必设成绝对路径**） |
| `DB_MEMORY` | `false` | SQLite 内存模式（测试用，进程退出即丢） |
| `DB_HOST` | `127.0.0.1` | MySQL 主机 |
| `DB_PORT` | `3306` | MySQL 端口 |
| `DB_USER` | `nvs_user` | MySQL 用户名 |
| `DB_PASSWORD` | 空 | MySQL 密码 |
| `DB_NAME` | `nvs` | MySQL 数据库名 |
| `DB_ROOT_PASSWORD` | 空 | 仅供建站向导创建库/用户时使用，创建后可删 |

> 🔐 **数据库密码加密存储**：通过建站向导或后台「数据库迁移」设置的 MySQL 连接信息（含密码）会经 **AES-256-GCM** 加密写入 `data/secure-config.json`（密钥存 `data/.secret.key`，0600 权限仅本机可读），并自动从 `.env` 移除明文 `DB_*` 行，避免密码明文残留。`config.Init()` 启动时会优先解密加载该加密配置覆盖内存变量。
>
> ⚠️ `config.IsInstalled()` 的判据是 **`data/secure-config.json` 是否存在**（进程内 once 缓存）：把它挪走，哪怕数据库已建站，实例也会退回建站向导。

### Redis（可选，SQLite 模式下不需要）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `REDIS_HOST` | `127.0.0.1` | Redis 主机 |
| `REDIS_PORT` | `6379` | Redis 端口 |
| `REDIS_PASSWORD` | 空 | Redis 密码 |
| `REDIS_DB` | `0` | 库序号 |

### 网络与端口
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SERVER_PORT` | `8080` | 服务端口 |
| `BIND_IPV4` | `0.0.0.0` | IPv4 绑定地址（`0.0.0.0`=所有网卡） |
| `BIND_IPV6` | `::` | IPv6 绑定地址 |
| `ENABLE_IPV6` | `false` | 是否启用 IPv6 双栈 |
| `LOCAL_IPV4` | 自动检测 | 本机 IPv4（联邦互通用） |
| `LOCAL_IPV6` | 自动检测 | 本机 IPv6（联邦互通用） |

### 站点路径前缀（反向代理挂载）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `URL_PREFIX` | 空 | 站点路径前缀，如 `/go-nvs`（必须 `/` 开头、不带尾斜杠；留空=挂在域名根）。用于反向代理场景：nginx/网关将 `/go-nvs/` 前缀转发到本站时，访问 `http://网关:8088/go-nvs/login` 即可正常使用（自动剥离前缀 + 前端路由/API/资源/跳转全自适应，前缀名任意） |

> 🔀 **两种代理转发模式均可用**：① `proxy_pass http://后端;`（不带 URI，保留前缀，后端自动剥离）；② `proxy_pass http://后端/;`（带尾部斜杠，去前缀，后端收 `/assets/x.js` 等原始路径）。反向代理仅改写 `<base href>` 而不注入前缀变量时，前端自动从 `document.baseURI` 推导前缀，无需后端配置。

### JWT
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `JWT_SECRET` | 空（自动生成） | JWT 签名密钥；留空启动自动生成随机密钥并持久化，**务必不要用示例值上线** |
| `JWT_EXPIRE_HOURS` | `72` | 访问令牌过期时间（小时） |
| `JWT_REFRESH_EXPIRE_HOURS` | `720` | 刷新令牌过期时间（小时，默认 30 天，用于无感续期） |

### SMTP 邮件
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SMTP_HOST` | `smtp.qq.com` | SMTP 服务器 |
| `SMTP_PORT` | `587` | SMTP 端口 |
| `SMTP_USER` | — | 发件邮箱 |
| `SMTP_PASSWORD` | — | SMTP 授权码 |
| `SMTP_FROM` | — | 发件人地址 |

### 短信网关（手机号登录 / 验证码）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SMS_GATEWAY` | `log` | `log`=开发网关（验证码只写服务日志，不真发）；`http`=通用 HTTP 网关 |
| `SMS_HTTP_URL` | 空 | `http` 网关地址（POST JSON `{phone,code,sign_name,template_code}`） |
| `SMS_HTTP_TOKEN` | 空 | 网关令牌（请求头 `Authorization: Bearer <token>`） |
| `SMS_SIGN_NAME` | `星海文学` | 短信签名（需在短信平台报备） |
| `SMS_TEMPLATE_CODE` | 空 | 模板 Code（模板内容须含验证码占位符） |
| `SMS_TIMEOUT_SEC` | `5` | 网关超时（秒） |

### 微信扫码登录（开放平台网站应用）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `WECHAT_APP_ID` | 空 | 开放平台 AppID |
| `WECHAT_APP_SECRET` | 空 | 开放平台 AppSecret |
| `WECHAT_CALLBACK` | 空 | 完整回调地址（须与平台登记域名一致，含路径前缀），如 `https://example.com/go-nvs/api/auth/wechat/callback` |

### 存储与用户自定义内容
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `NOVEL_DATA_DIR` | `./data/novels` | 小说文件存储目录 |
| `UPLOAD_DIR` | `./data/uploads` | 上传文件存储目录 |
| `USER_FRAME_DIR` | `./data/userframe` | 自定义 HTML-UI 模板目录 |
| `USER_HTML_DIR` | `./data/userhtml` | 扩展应用（HTML 托管）目录 |

### 平台功能
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PLATFORM_FEE_RATE` | `0.10` | 平台抽成比例 |
| `SITE_NAME` | `NVS 类型文学平台` | 站点名称 |
| `EMAIL_VERIFY_ENABLED` | `false` | 邮箱验证开关 |
| `CAPTCHA_ENABLED` | `false` | 滑块验证码开关（**开站后建议开启**，登录/注册/发码/找回密码统一走一次性 pass token） |

### 安全 / 反向代理
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TRUSTED_PROXIES` | 空 | 信任的反向代理地址（逗号分隔 IP/CIDR）。留空=不信任任何代理头，客户端 IP 取真实 TCP 对端（防 `X-Forwarded-For` 伪造越权） |
| `COOKIE_SECURE` | `false` | JWT Cookie 强制 Secure（仅 HTTPS 传输；生产启用 HTTPS 后务必 `true`） |
| `SETUP_LOCAL_ONLY` | `true` | 建站向导仅允许本机 localhost 访问（防公网/反向代理越权进入安装向导） |
| `REAL_NAME_ENC_KEY` | 空 | 实名信息（姓名/证件）AES-256-GCM 密钥（hex 或 base64 的 32 字节）。留空则从 `JWT_SECRET` 派生 —— **轮换 JWT_SECRET 会导致历史实名数据不可解密**，生产请独立配置 |

### 国密 / TLS 双轨（可选模块）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `NVS_CRYPTO_TRACK` | `intl` | `intl`=标准 RSA/ECC+AES（零国密依赖）/ `gm`=纯国密 SM2/SM3/SM4+TLCP / `dual`=双轨自适应协商 |
| `NVS_CRYPTO_INTL_CERT` / `NVS_CRYPTO_INTL_KEY` | 空 | 国际轨证书；都不配 = 纯 HTTP（适合反代终止 TLS） |
| `NVS_CRYPTO_GM_PROVIDER` | 空 | 国密实现：`tongsuo`（CGO，官方绑定）/ `purego`（纯 Go，无 CGO） |
| `NVS_CRYPTO_SM_SIGN_CERT` / `_SIGN_KEY` / `_ENC_CERT` / `_ENC_KEY` | 空 | 国密双证书四件套（TLCP 需签名+加密两套） |
| `NVS_CRYPTO_SESSION_TICKETS_DISABLED` | `false` | 禁用 TLS 会话复用 |
| `NVS_TLS_HANDSHAKE_LOG` | 空 | 握手记录 JSONL 路径（记录协议/套件/SNI，不含业务数据） |

> 责任边界：框架不代做等保/密评；是否启用及合规责任由站长自行承担（见 `docs/gm-mapping.md`）。

### 日志敏感信息脱敏（176号令合规）
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LOG_REDACT_SENSITIVE` | `false` | 日志脱敏开关：`true`=生产模式（自动脱敏 Cookie/Authorization 令牌、body/query 敏感字段）；`false`=开发模式（保留原始数据，方便调试） |
| `LOG_REDACT_MASK` | `***REDACTED***` | 脱敏掩码（密码类固定 `******`，其余字段可用此掩码） |
| `LOG_SENSITIVE_FIELDS` | `password,token,authorization,cookie,secret,apikey` | 需脱敏的字段名列表（逗号分隔，可扩展） |

> 🧾 脱敏配置支持管理后台可视化维护（`/admin/log-redact`），保存后运行时热更新，无需重启；`.env` 仅为种子默认值。

---

## 📝 版本更新记录（文件级改动说明）

### v4.1.0 AI 工作台成型 + 内容安全闸 + 话本气泡体（2026-10-25）

v4 线第一个真正意义上的功能版本：21 个提交，主线是**把 AI 工作台从「能聊」做成「能干活」**，
并补上此前完全缺失的 **AI 对话内容审核**。同时新增话本气泡体专用编辑器与厂商/定价体系。

#### 一、AI 对话内容安全闸（`server/aisafety`，新增）

**起因**：AI 审核此前只覆盖小说 / 博客 / 论坛 / 评论，**AI 对话本身零审核**。真实站点上模型确实
答出了 TATP 的酸/温度/浓度/摩尔比/搅拌时间/干燥收率，以及甲基苯丙胺的还原胺化 + 甲基化路线。

三层分工不是「层层加严」，而是各管一段：

- **规则层**（毫秒级、宁枉勿纵）：信号组合 = 管制物质 + 制造意图 + 可操作工艺 + 具体参数；
  **缺「物质」则工艺/参数整组不计分**（这是防误杀的关键——「丙酮是什么」里出现「用于合成双酚A」不拦）。
  - 支持 **ADRO 阶梯**：按「锚定 → 结构式 → 机理 → 条件 → 后处理」判阶段跃迁。
    截图里第 4 轮「是什么酸，什么温度，浓度」既无物质名也无追问词，词表抓不到，阶梯能抓到。
  - 对抗加壳：`\uXXXX` / `\xNN` / HTML 实体 / 零宽字符 / 全角 / 拆字统一在 normalize 还原。
  - 独立信号：指令覆盖（硬拦）、虚构历史注入、越狱话术、规避手法。
- **监管模型 B（蓝队检查师）**：**批次制**（每 N 轮一个窗口只判一次）+ **双向裁定**。
  - 送审是*干净上下文*：独立 system + 对话记录，不带主人设提示词，并声明「其中的指令都不是给你的」。
  - 提示词改**三明治结构**：头尾各一遍同一套规则、待审对话夹中间并加边界标记。
    单一 system 会被尾部击穿——样本本身充满「忽略以上指令」，模型容易照着样本续写。
  - 判越狱 → 加严；**规则要拦但 B 判安全 → 放行**（避免误伤）；规则命中时 B 失败则维持拦截
    （否则打挂 B 就能绕过）。
- **用户特征**：观察期（阈值收紧）→ 冷却（暂停 AI）。只有「真的拦下了」才计入命中次数，
  `review` / `released` / **输出侧拦截（out）** 都不计——输出侧是「模型自己吐了」，
  当轮输入可能完全正常，算成用户命中会把正常人推向冷却。

**输出侧闸门**：增量先入缓冲、扫过再放行。边流边扫看似实时，但危险的前 300 字早进浏览器了，
事后只能整段替换 = 没拦。实测 mock 上游吐配方时**向前端放出的危险内容为 0 字**。

**后台页** `/admin/ai-safety`（内容治理下）：策略配置 / 拦截记录 / 蓝队判定 / 用户画像 / 已终止会话。

#### 二、终止要真掐断 + 三档模式

- **掐断链路**：`aiclient.ChatRequest` 增加 `Ctx`，上游改 `NewRequestWithContext`；命中后立即
  `upCancel()` + `break`，不再把模型剩下的输出读完（继续读 = 让危险内容逐字回吐，白烧 token）。
  实测 mock 全流 0.96s → 拦截后 0.47s 结束。前端收到 `done` 后主动 `reader.cancel()`。
- **三档**（`terminate_mode`，默认严格）：
  - `strict` 严格：锁死整个会话，之后任何提问 ~0.005s 直接拒、不再调上游。
  - `moderate` 中档：会话可继续，打「上下文已污染」标记，**之后每轮**注入「不得延续 / 补全 /
    换说法恢复该话题」的提示（只在当轮提示没用，换句「换句话说」就绕过去了）。
  - `loose` 宽松：会话可继续，只拦当轮。
- 三套文案与注入提示词全部后台可配。

#### 三、对话分支（同 DeepSeek / Qwen 官网）

- `AiChatMessage` 增 `parent_id`，`AiChatSession` 增 `active_leaf_id`；`ListAiChatThread` 按叶子回溯。
  **不能再用「整表按时间排序」**——有分支后时间序会把兄弟分支串成「问一次、答两次」的错乱上下文。
- 「重新生成」改为**新建兄弟分支**（原实现截断列表、删掉旧回复，无法对比两个答案）；
  每条消息显示 `‹ 2/3 ›` 切换，切分支只改显示位置、不删任何消息。
- 老数据 `BackfillMessageParents` 幂等串链并补活跃叶子（不做则升级后打开旧对话只剩最后一条）。

#### 四、AI 工作台：附件 + 公式 + 流式直写

- **图片 / 文档附件**：按需求**不做本地解析**，文件存盘后随对话请求**原样经 API 发给上游**
  （图片 `image_url`、文档 `file` part、Anthropic 转 `document` + `source.base64`）。
  无附件时退化为纯字符串，不给不接受数组的老上游添麻烦。上传接口 `POST /api/ai/attachment`。
- **公式定界符**：Cherry 只认 `$$` / `$`，模型常输出 LaTeX 原生 `\[ … \]` / `\( … \)`，
  新增 `markdown/latex.ts` 在渲染前归一化（代码块 / 行内代码跳过，`\\[` 判为字面反斜杠不动）。
- **浮动可拖拽 AI 写作层**：`components/ai/floating/`，Teleport 到 body、可拖动、**ESC 不关闭**
  （只能点 X 或取消），流式增量直接写进编辑器。

#### 五、话本气泡体专用编辑器

- 语法：`角色：台词`；行首 `#` = 左气泡、`##` = 右气泡。**标记只在去掉后仍能解析出「角色：台词」时才生效**，
  否则 `## 标题` 这类 Markdown 标题会被误伤。同角色连续对话合并进一个气泡。
- 新增作品级角色表 `NovelCharacter`（头像 / 主题色 / 默认左右），写权限仅主笔。
- `components/editor/script/` 五件套；ChapterEditor 新增「气泡」模式，与 Markdown / 富文本三态互转。

#### 六、内容治理两个独立入口

- **AI 审核** `/admin/ai-moderation`：功能配置（开关 / 模型 / 送检策略 / 批次间隔 / 处置阈值 /
  恶意举报者 / 概览）。
- **审核** `/admin/moderation`：待办处置（放行 / 删除留证 / 封禁留证，后两者必填原因）。
- 补齐后端缺口：`ModerationRecord` 增人工处置字段；`POST /api/admin/moderation/records/:id/decide`；
  `auto_hide_on_fail` 由「写了不用」接线为 `SetOnFailHook`（避免 moderation 反向依赖 handlers）；
  软删核心拆出可为 nil 的 `gin.Context` 版本，系统自动处置不伪造请求上下文。

#### 七、厂商 / 模型体系

- `AiVendor.Prefix`（厂商 = 模型 ID 前缀）、`AiModel.Version/SubVersion`（`前缀-版本-亚版本`）。
- 新增 `aiapi/vendor`：`ParseModelID`、`SubVersionFreshness`、`NewestSubVersion`；
  新旧判定优先级 **日期型 > 尾部数字 > 词表档位**。
- `AiCategoryPrice`：chat / image / video 三类聚合分类定价，导入模型未给价时套用。
- 新增接口 `GET/PUT /api/admin/ai/category-prices`、`GET /api/admin/ai/vendors/matrix`
  （矩阵含「谁最新」标记，**仅管理端可见**；用户侧只显示主名字）。

#### 八、本版修掉的三个「静默失效」类真 bug

1. **AI 流式直写一直是假的**：`cherry-markdown@0.11.9` 已是 **CodeMirror 6**，`editor.codemirror`
   根本不存在，按 CM5 写的 `setBookmark/replaceRange` 全 undefined → 增量只追加到变量，编辑器里
   一个字都看不到。已重写为 CM6（`editor.editor` 的 `state` / `dispatch`，偏移量自行前进）。
2. **点「保存章节」必失败**：`@click="saveChapter"` 把 MouseEvent 当 `force` 传进请求体 →
   后端 `bool` 绑定失败 → 400，用户视角就是「点了没反应」。
3. **老库升级后按前缀匹配厂商永久失效**：老站 `ai_vendors` 早已存在（无 `prefix` 列），
   AutoMigrate 只加列、存量前缀全空。已补回填（不覆盖手填值）；顺带修 `derivePrefix`
   先过滤后 `ToLower` 导致 `OpenAI → "pen"` 的丢失大写问题。

#### 九、其它

- 管理端 API Key 连通性 / 有效性体检接口。
- 模型别名 / 分类 / 渠道拉取与开放控制（嵌套子包，无硬编码名单）。
- 警方内容治理镜像（与站长同口径，PII 已脱敏）。

> 版本号四处一致：`server/version/version.go` / `web/package.json` /
> `web/package-lock.json` / README 顶部徽章。

### v4.0.0 新主线分支起点（2026-10-25）

**性质**：这是 `v4` 分支的起点版本，**代码与 `v3.21.2` 完全一致**（同一个提交），
仅版本号从 `3.21.2` 顺延为 `4.0.0`，用于把「新主线」与「v3 维护线」在发布物上区分开。

- `v3.21.2`：v3 线的最后一个发布（也是本次并入 `master` / `main` 的版本）
- `v4.0.0`：v4 线的起点（内容同上，便于后续在 v4 上做破坏性演进而不影响 v3 的补丁发布）
- 版本号四处一致：`server/version/version.go` / `web/package.json` /
  `web/package-lock.json` / README 顶部徽章

> 后续 v4 的功能开发不再回灌 v3；v3 只接受 bugfix 与安全修补，各自独立打 tag。

### v3.21.2 修 CI 后端 job（双编译轨）+ README 全量补全（2026-10-25）

本版两块：**CI 从「每次必挂」修成绿**，以及**把落后于代码的 README 全量补齐**。运行期行为零变更
（`nodist` 只影响编译方式，不进入发布物）。

#### 一、修 CI 后端 job：`//go:embed all:dist` 在全新 clone 里无目录可嵌

**症状**：push 后 `CI / 后端 build / vet / test` **16 秒**就红，注释只有一句
`pattern all:dist: no matching files found`；前端 job 反而正常。

**根因**：`server/dist` 是构建产物、不入库（`be82047` 起 `.gitignore` 掉并在提交里删除）。
`//go:embed all:dist` 要求目录**至少一个文件**，于是全新 clone 里 `cd server && go build ./...`
在编译期就失败 —— 与代码质量无关，是「业务编译依赖前端产物」的结构问题。
更麻烦的是同一段逻辑也卡住了**新贡献者**：clone 下来 `go test ./...` 直接不可能通过。

**改法（两条编译轨，不改发布链路）**
- 新增 `server/embed_nodist.go`（`//go:build nodist`）：嵌入 `nodist_placeholder/index.html` 一个提示页，
  让 `fs.Sub` 成功、页面明确写着「这是占位页，请先 npm run build → 同步 server/dist → 不带标签重新编译」；
  `server/embed.go` 加 `//go:build !nodist`，`main.go` 改用 `getFrontendFS()` 并标注 `nodistBuild`，
  该轨启动时打印醒目警告（避免被误当发布物）
- CI 后端 job 的 `build` / `vet` / `test` / `gitvcs` 全部加 `-tags nodist`（只验证业务代码）；
  **另加一道防回归**：`mkdir dist 占位 + 不带标签 go build`，确保 nodist 开关没把发布轨弄坏
- `make check` / `make test` / `.githooks/pre-push` 同步走 nodist 轨（新克隆不必先构建前端即可跑测试）；
  新增 `make test-embed` 用真实产物验证 embed 本身
- 发布链路完全不变：`make build`、`release.yml`、CI 的「内嵌产物冒烟」仍是不带标签 + 先准备 `server/dist`

**验证**：把 `server/dist` 挪走模拟全新 clone —— 裸 `go build ./...` 复现原报错；
`-tags nodist` 下 `build` / `vet` / `go test ./... -count=1` 全绿；`-tags "nodist gitvcs"` 可编译；
恢复 `server/dist` 后发布轨 `go build ./...` 仍通过。

#### 二、README 全量补全（此前「特性」停在旧范围）

**背景**：功能迭代快于文档，README 里「特性」停在旧范围（AI 工作台 / 会员 / 群组 / 话本协作 / 精读朗读
只在 changelog 里有，特性区看不到），「项目结构」还是 70 个 handler、21 个后台页的旧规模，
配置表缺短信 / 微信 / 国密 / 实名加密整段，且有几处**指路已失效**。

**改动**
- 顶部加 **version 徽章**（与 `version.go` / `package.json` 三处一致，`make version` 校验）
- 特性区新增 6 个板块：**AI 工作台**（对话/绘画/视频/智能体/模型池/路由降级/思考型模型/BYOK/成本台）、
  **阅读体验**（字号/进度/书签/划线批注/语音朗读；楼层评论 + 排序 + 密度 + 评论网 + 加密楼）、
  **群组与实时消息**（SSE 事件、ID 优先的未读计数、私信、消息中心）、
  **话本·协作·版本控制·OC 模式**、**会员与额度**（套餐快照 + 同事务延长 + 每日免费额度 + 权益对比表）
- 安全合规补 5 条：**蜜罐四档**、**隔离墙访问仪表盘**、**网络诊断与攻击流量面板**、**警方取管台**、**AI 内容可追溯**
- 修指路失效：`模板设置` 入口（头像下拉 → **个人中心 → 我的服务**）；用户分级补 **police 角色**；
  页脚版本号不再写死 `v2.2.2`
- 「技术栈」补 AI / 实时 / 质量三行，Go 版本对齐 `go.mod`（1.25），前端版本对齐 `package.json`
- 新增 **「质量门禁与验收脚本」** 段落：8 个脚本各查什么、何时跑，附真浏览器验收的 cookie 取得方式与
  「为什么必须真浏览器」的踩坑说明
- 「项目结构」重写：补 `scripts/` 8 个脚本、server 的 14 个功能子包（aiclient/aiapi/aicall/chaptervcs/
  collab/groups/oc/prompttpl/realtime/honeypot/gate/risk/payment/police）、models 80 文件 / handlers 139 文件、
  web 的 117 条路由 / 50 页面 / 48 后台页 / 70 组件 / 5 个 views 子目录
- 「配置参考」补齐：Redis、短信网关、微信登录、存储与自定义目录、实名加密密钥、国密 TLS 双轨；
  修正 JWT / 日志脱敏的默认值（与 `config.go` 模板一致），并补 `DB_PATH` / `DB_MEMORY` / `JWT_REFRESH_EXPIRE_HOURS`
- **新增 `.env.example`**（仓库根，6691 字节）—— 此前 README 引用它但文件并不存在。
  由 `server/config/config.go` 的 `defaultEnvTemplate` 生成，密钥项留空（避免示例值被当真值使用），
  并在文件头注明「改这里请同步改 Go 模板」

**顺带修正**：`web/package-lock.json` 的根版本长期停在 `3.8.0`（与 `package.json` 自相矛盾），
本次一并对齐到 `3.21.2`；版本号现在是「四处一致」——`version.go` / `package.json` /
`package-lock.json` / README 徽章。

### v3.21.1 个人中心收拢入口，头像下拉瘦身（2026-10-24）

**问题**：头像下拉越挂越长（个人中心 / 更换头像 / 模板设置 / 我的举报 / 格式使用文档 /
API Key 爬虫指南 / 我的 API Key / 我的钱包 / AI 工作台 / 黑名单管理 / 特别关注 /
特别关注动态 / 我的私信 / 账号设置 / 管理员面板 / 警方取管台 / 退出登录，共 17 项），
越往下越难找，且与「更多」菜单的职责重叠。

**改法**
- 下拉只留 5 项：**个人中心 / 我的主页 / （管理员）管理员面板 / （警方）警方取管台 / 退出登录**
  （`popper-class="user-menu-popper"` 便于定位与测试）
- 14 个入口全部搬进 **个人中心 `/me` 概况页**的「我的服务」，按语义分 4 组：
  账号与资料（头像与资料 / 账号设置 / 我的主页）、互动与关注（我的私信 / 特别关注 /
  特别关注动态 / 黑名单管理 / 我的举报）、资产与开放能力（我的钱包 / 我的 API Key /
  API Key 爬虫指南 / AI 工作台）、外观与帮助（模板设置 / 格式使用文档 / 我的精读）
- **更换头像**迁到「个人中心 → 装扮 → 资料卡」：本地选图（≤5MB）→ `/api/author/avatar` 上传 →
  即时写回 store（导航栏头像同步刷新，无需刷新页面）；也可直接填站内路径或图片直链
- 个人中心头部头像可点击直达装扮页（tooltip 提示「更换头像 / 编辑资料」，比藏在下拉里更容易发现）
- 移动端抽屉同步瘦身（账号组只剩个人中心 / 我的主页 / 管理员 / 警方 / 退出）
- 顺带：`MeLayout` / `MeOverview` 的 `#fff` + 各自深色覆盖改为 `var(--bg-card)` 令牌；
  `tab-change` 处理函数签名对齐 Element Plus 的 `string | number`

**验收**：`scripts/browser-me-center.mjs`（8 项）——下拉 ≤5 项、13 个入口在下拉中消失且
在 `/me` 全部可见、15 个服务链接逐个直达不 404 不弹登录、装扮页有头像上传、头像可点跳转、无未捕获异常。

### v3.21.0 前台精细阅读 / 个人主页 / 警方台深化（2026-10-24）

**主题**：把「能用」推到「好用」——评论区可排序可调密度、阅读器朗读真的能用、
个人主页有资料卡、他人主页分栏展示、后台清理死代码并收敛高危操作。

**评论区（`components/CommentSection.vue` + 后端 `GET /api/comments`）**
- 新增 `sort=new|old|hot`（白名单归一化）：`hot` 按楼内回复数排序（子查询，排除已删除回复），
  楼内回复恒按时间正序；非法值回落 `new`。附 `models/comment_sort_test.go`（三种排序 + 删除后热度）。
- 工具条：总数/楼层数 + 排序切换 + 紧凑/舒适密度（`localStorage` 记住偏好）。
- 补齐四态：骨架屏、空态引导、错误态可重试；「加载更多」显示 `已显示 X / 共 Y`。
- 修 **P0 合规门禁**：`crossDomainCheck` 漏 `await`，`shouldShowGuard` 返回 Promise，
  `guard?.needed` 恒 undefined → 敏感分区的两级确认从未生效。

**阅读器（`views/Reader.vue`）**
- 「朗读」按钮此前是空壳（`ttsLoading/ttsPlaying/toggleTTS/stopTTS` 全未定义，
  卸载时 `stopTTS()` 直接 ReferenceError 中断后续清理）。现按 `POST /api/ai/speech` 真正实现：
  按句切段（≤1800 字/段、单次≤8 段）、顺序播放、可随时停止、离开页面释放音频。
- 清理 `[NVS]` 调试日志。

**个人主页**
- `me/MeDecorate`：新增资料卡（头像/昵称/签名/简介）+「看看别人怎么看到我」，
  装扮与公开主页两段保留；后端 `PUT /api/author/profile` 支持 `bio`（≤500 字）与
  `avatar_url`（白名单：`/uploads/` `/static/` `http(s)://`，拒绝 `javascript:`/`data:`）。
- `views/AuthorHome.vue` 重写为「封面 + 资料头 + 分栏（作品/博客/数据/留言）」：
  修 **粉丝数张冠李戴**（原来用 `/follow/stats`＝浏览者自己的口径），新增公开接口
  `GET /api/author/:id/follow-stats`；新增私信直达（`/user/messages?peer=`）、举报、编辑入口；
  数据页去掉了重复渲染的两个图表实例。
- `views/user/Messages.vue`：支持 `?peer=` 直达会话；修「首次私信时轮询把会话关掉」。

**小说详情（`views/NovelDetail.vue`）**
- 目录工具条：章节搜索 + 正序/倒序 + 「跳到上次阅读」；上次阅读章节整行高亮（读 `reading-progress-*`）。
- 头部补齐状态/体裁/来源/AI 参与/单价标签与标签跳转；空目录改用区块级空态。

**后台**
- 新增 `scripts/audit-unresolved-components.mjs`：揪「模板用了、脚本没导入」的组件。
  本次共修 8 处静默失效图标（NavBar 搜索/编辑/下拉箭头、NovelDetail 折叠箭头、
  Editor 删除、UserManagement 搜索、RealNameCard）。已接入 `make check` 与 CI 前端 job。
- 新增 `scripts/fix-table-empty-slots.mjs`：清理 **114 处列级 `#empty` 插槽**（`el-table-column`
  没有 empty 插槽，这些块永远不渲染，涉及 18 个后台页）。
- `AdminPoliceConsole`：概览类型 chips（点击筛选）、关键词过滤、客户端分页、错误态；
  全量取证包导出加二次确认并写明「含个人信息、仅限依法调取」。
- `police/PoliceForumPatrol`：空态/错误态改用 `StateBlock`（去掉 297px 的 `el-empty`）。
- `UserManagement`：封禁/提权/撤销警方收进「更多」下拉（仍各自二次确认），
  编辑弹窗补 `police` 选项与审计路径提示。
- 修 `AdminAiPool` 缺 `adminApi` 导入、`AdminDatabase` 残留的不存在函数 `loadHealth()`、
  `AdminAiBot` 调错接口名（`adminListAgents`→`adminAgents`）、`AiAssistPanel` emits 缺 `meta` 参数。
- `GET /api/ai/bot/status`：「AI 未开启」不再返回 400，改 200 + `enabled:false`
  （原实现让每个带评论区的页面都在控制台留一条 4xx，且会被后台异常横幅统计）。

**验收脚本**
- `scripts/browser-smoke.mjs`：真浏览器四档宽度（375/768/1024/1440）回归，
  断言 `scrollWidth` 无横向溢出、关键元素存在且可见、`/api/*` 无失败、console 无错误，
  截图与 `report.json` 落盘。非 `/api` 的 404 记为「提示」（test_go 里存有攻击性测试数据）。
- `scripts/browser-func.mjs`：功能级回归——资料卡保存并回读落库、作者主页四分栏可切、
  评论排序真的带 `sort=` 参数、密度切换真的改 DOM 高度、朗读真的请求 `/api/ai/speech`
  且失败时有可读提示、按钮不会永久转圈。

### v3.20.1 蜜罐收回归内嵌主程序（2026-10-24）

**口径修正**：蜜罐**只存在于主程序**，不产出第二个二进制。此前 v3.16.0 提供的独立进程形态
（`nvs-honeypot.exe` / 端口 18081）已移除，站长只需部署一个 `nvs-server.exe`。

- 挂载点（全部在主 gin 引擎，`main.go` 注册）：诱饵 `honeypot.Register`、SQLi 假入口
  `honeypot.RegisterSQLi`、影子认证 `honeypot.RegisterShadow`、验证页 `honeypot.RegisterVerify`、
  可疑 404 探路 `honeypot.ProbeNotFound`、迁移/引导 `honeypot.Bootstrap`
- 等级 `off` 时这些路径与普通 404 无差别，不暴露蜜罐存在
- **不提供公开 health 端点**（内嵌形态下 `/__hp/health` 等于自报家门）；
  运行状态走管理端：`GET /api/admin/honeypot/db`（是否独立库/路径）、
  `GET /api/admin/honeypot/overview`（会话/事件/高风险计数）
- **数据隔离仍可选**：`NVS_HONEYPOT_DB_PATH=data/honeypot/honeypot.db` →
  蜜罐数据与业务库物理隔离（启动断言「蜜罐库 ≠ 主库」），后台照旧可读；
  不设则表落主库，零配置
- 移除：`server/cmd/honeypot`、`nvs-honeypot.exe`；文档与对账表同步改口径

**验证**：`go build ./...` 通过；单测（honeypot/oc/models/handlers/scriptmode）全绿；
单 exe 启动冒烟：首页 200、内嵌资源 200、`/.env` 在 off 档返回 404、无 panic、无路由冲突。

### v3.20.0 收口剩余 3 项：出图附件与历史导出 / 气泡长按互动 / 直播入口（2026-10-24）

> 设计与边界见 `docs/anti-abuse-cost-ops.md` §五；对账见 `docs/gap-audit.md`（**缺口已清零**）。

**AI 出图附件 + 历史导出（gap #17）**
- `ai_media_tasks.attachments`：参考图以前只传 data URL 不落库，历史里看不到"当时用了什么图"；
  现在出图接口收 `attach_urls`，前端上传参考图时同时存一份站内副本
- `POST /api/ai/media/attach`（10MB、png/jpg/webp/gif）
- `GET /api/ai/export/media.zip?kind=&month=`：`items.json` + `files/` + `manifest.json`（每文件 SHA256）
- `GET /api/ai/media/:id/export`（JSON，`?files=1` 打 zip）
- 安全：站内读盘做**目录穿越校验**；远程图走 **SSRF 白名单抓取**（拒私网/回环/元数据、
  重定向逐跳校验、限 10MB/15s）；抓不到的图记入 manifest 的 skip 原因
- 顺手修 bug：`ListAiMediaTasks` 原在分页后才按 kind 过滤，会出现"第一页 5 条、total 20"错位

**气泡长按互动（gap #22）**
- `scriptmode` 给气泡打 `data-bubble/data-speaker/data-side/data-text`（全部转义）
- 阅读器：按住 480ms（移动 >10px 取消）或右键唤出动作面板 → **复制台词 / 引用评论 / 收藏本章**
- 引用评论会预填 Markdown 引用块并滚动聚焦评论区

**直播入口（gap #23）**
- `platform_configs.live_setting`：开关/标题/状态/`embed_url`/`video_url`/海报/开播时间/公告/关联群组/需登录
- `embed_url` **仅 https**；iframe 强制 `sandbox`（不含 `allow-top-navigation`）
- 前台 `/live`：直播中挂 iframe 或 video；未开播展示海报与倒计时；关联群组给"进入直播聊天群"
- **明确边界**：本站不做推流/转码/HLS，推流交给 SRS/云直播，这边只做入口与嵌入

**验证**：`go test ./scriptmode ./models ./handlers` 全绿（新增气泡属性转义、直播配置校验、
SSRF 拒绝、目录穿越拒绝、zip manifest、result JSON 兼容）；`go build ./...` 通过；
启动冒烟无路由冲突无 panic；前端 `vite build` 通过并镜像 `server/dist`。

### v3.19.0 安全防护 / 成本闸门 / 内容玩法：15 项缺口一次收口（2026-10-24）

> 详细设计、命令与边界见 `docs/anti-abuse-cost-ops.md`；逐项对账见 `docs/gap-audit.md`。

**安全防护**
- **SQL 注入蜜罐**（`honeypot/sqli.go`）：假 SQL 入口 + 预生成响应库（假表/假用户/金丝雀），
  10 类注入特征识别；`off`/`low` 与普通 404 无差别。真实链路全参数化，"打不穿"是事实，
  这里的目的是拿 payload、拖时间、反哺规则
- **JA3/JA4 + WebRTC**：采信反代 `X-JA3-Hash`/`X-JA4`（缺失记 `missing` **不伪造**），
  指纹表加 `ja3/ja4/tls_source/tls_version/webrtc_ips`；验证页用 RTCPeerConnection 收内网 IP；
  后台给采集率诊断与 nginx/caddy 配置模板
- **注册门槛**（`server/gate`）：邀请码（限次/限时/可作废）+ 设备指纹（`X-Device-Id` 优先，ip+ua 兜底）
  + 账号年龄 + 新号每日上限 + 临时邮箱 + 蜜罐 IP；注册/评论/发文三处生效
- **充值风控**（`server/risk`）：入账前打分（蜜罐 IP/同设备多账号/高频/退款率/新号大额），
  `allow` / `hold`（入账打标）/ `block`（不入账）三处置；**绝不自动扣回已到账余额**
- **警方联动工单**（`server/police`）：建单/派单/进展/结案（**结单必须写结论**），
  超期与未派单口径；工单只存证据引用，不存证据本体
- **API Key 告警口径**：HMAC 审计指纹（不存明文也能对号）+ 多 IP 提醒（只提醒不封禁）
  + 解密失败 + 封禁后仍使用

**成本闸门**
- **全站总预算 + 高峰排队**（`models/ai_budget.go`）：日/月代币上限（`HardStop` 可软可硬）、
  并发闸门与排队超时（排不上返回"忙"，不打上游 429）、峰时上限（高于全站上限自动钳制）
- **三维交叉限流**（`models/ai_throttle.go`）：用户×模型 / 用户 / 模型 三窗口
- **转化漏斗**：触达→使用→额度用尽→进收银台→付费→复购，含"最弱一跳"洞察
- **TTS/音乐**（`aiclient/audio.go` + `POST /api/ai/speech`）：OpenAI 兼容 `/audio/speech`，
  2000 字上限、必须 `kind=audio` 模型（没有就明说未配置）、产物随机名落 uploads；阅读器「朗读」

**内容玩法**
- **自定义导航菜单**：两级树 + 协议白名单（拒 `javascript:`/`data:`/`//host`）+ 登录/角色可见性
- **轮播 / 签到 / 访客**：轮播后端按时段过滤；签到幂等（唯一索引 + 先落记录后发币）与连签加成；
  访客设置 + 14 天 PV/UV
- **自定义表情包**：键全局唯一（服务端渲染无用户上下文）、URL 白名单，渲染为 `<img>`
- **OC 角色卡/世界书互通**：酒馆 V2/V1 双向、`character_book` 互通、`extensions.nvs` 保留自家字段、
  未知字段留存不丢数据
- **思维导图**：AI 动作 `mindmap`，强制输出 Mermaid `mindmap` 代码块（阅读器已能渲染）

**验证**：`go test ./gate ./risk ./police ./honeypot ./oc ./models ./aicall ./scriptmode ./collab ./chaptervcs` 全绿；
`go build ./...` 通过；启动冒烟无路由冲突无 panic；前端 `vite build` 通过并同步 `server/dist`。

**仍未做（明确边界）**：AI 出图附件上传与历史导出、气泡长按互动、直播入口、进程内 JA3 计算、集群级精确限流。

### v3.18.0 主笔转让 + 角色设定 AI 动作（2026-10-24）

**主笔转让（`collab.Transfer`，补齐 gap-audit #21）**
- `POST /api/novels/:id/transfer { to_user_id }`：仅现任主笔或平台可发起
- 受让人**必须是已接受的协作者**（陌生人先邀请，防转错人）
- 事务内完成三步：换 `Novel.AuthorID`、受让人退出成员表（主笔身份由作品字段体现）、
  原主笔降为 `editor` 且保留 `can_edit_others`（接手即失去权限是常见的坑）
- 不可逆：前端二次确认要**输入作品标题**；审计记 `transfer_owner`（before/after）
- 历史收益不追溯（分成按流水归属）

**角色设定 AI 动作（补齐 gap-audit #16）**
- `AI_ACTIONS` 新增 `character`：输出结构化角色卡（姓名/外貌 3 个视觉细节/性格与致命缺陷/处境/
  欲望与需求/秘密/关系张力/说话方式/三个行动选项），流水标签记「角色设定」

### v3.17.0 AI 路由策略 + 故障自动降级（2026-10-24）

**路由策略（`models/ai_router.go`，补齐 gap-audit #10）**
- `manual`（默认，保持既有语义：按 priority）｜`cheapest`（输入价+输出价，免费优先）｜
  `fastest`（近 7 天实测平均延迟，无样本排最后并退化为价格序）｜`scene`（按场景覆盖）
- 默认场景映射：`oc_turn`/`group_bot`/`oc_publish` → cheapest，`ai_hub` → fastest
- 合规开关 `prefer_domestic`：境内厂商优先（只影响排序，不硬性排除境外）
- 后台「AI 模型池 → 路由策略」可配置，并显示各场景**当前生效策略**；保存即时生效
- `AiCallLog` 新增 `latency_ms`：fastest 的数据来源，同时便于排查慢模型

**故障自动降级（`server/aicall`，补齐 gap-audit #11）**
- 统一调用入口 `aicall.Complete`：选模型 → 免费额度/余额预检 → 密钥租约 → 调用 → 结算，失败按
  `max_fallback`（1~3）自动切下一个候选，**绝不静默换模型**：`Result.Notice()` 生成
  「模型已降级：A → B（原因）」，没有备用时明确写「无可用备用模型」
- 已接入 OC 发言（`oc_turn`）与群内 AI（`group_bot`）：降级提示直接写进正文尾部
- 计费与既有账本一致（免费额度优先、失败不扣款、价格快照随流水）

### v3.16.0 蜜罐独立进程/独立库/冷归档 + 合规材料一键导出（2026-10-24）

**蜜罐隔离与留存（补齐 `gap-audit` #1/#2/#5）**
- 独立进程（**v3.20.1 起已收回归内嵌主程序**，此处保留历史记录）：`cmd/honeypot` → `nvs-honeypot.exe`（默认端口 18081），只挂诱饵 / 影子认证 / 验证页，
  不加载任何业务路由；`GET /__hp/health` 自检等级与库路径；优雅停机
- 独立库：`NVS_HONEYPOT_DB_PATH`（如 `data/honeypot/honeypot.db`）；主站设同一路径则**后台照旧可读**，
  蜜罐数据与业务库物理隔离；`SetDB` 断言「蜜罐库 ≠ 主库」，同路径拒绝启动；未设则退回内嵌
- 冷归档：`Cleanup` 改为 `ArchiveAndCleanup` —— 超期事件先写
  `data/honeypot/archive/honeypot-events-YYYYMM.jsonl.gz`（fsync 成功）再分批删库；
  新增 `GET /api/admin/honeypot/archives` 与 `GET /api/admin/honeypot/db`

**合规材料一键导出（补齐 `gap-audit` #7）**
- `GET /api/admin/compliance/bundle.zip`：自查报告 + 算法说明 + 等保定级 + 漏洞整改 + 用户规模汇总
  （不含 PII）+ 蜜罐与日志策略摘要 + 30 天日志清单（含 SHA256 前 8 字节），并附 `00-manifest.json` 文件哈希清单
- 导出动作写用户行为审计；下载后删除临时包（不堆敏感材料）
- 后台「合规中心 → 检查应对工具箱」新增「一键导出材料包 ZIP」

**文档订正**
- `docs/gap-audit.md`：撤回一条误判 —— 「公开算法透明度页」其实早已实现
  （首页 `HomeNoticeBar.vue`「📊 推荐说明」弹窗 + `GET /api/algorithms/description`），第一版只查了路由与后台页

### v3.15.0 蜜罐 M2/M3/M4：影子认证 + 验证页与指纹 + 取证包 + 规则反哺（2026-10-24）

**影子认证（`server/honeypot/jwt.go` + `shadowauth.go`）**
- 蜜罐**独立密钥** `data/honeypot/secret.key`（0600，首次启动生成），与真实 `JWT_SECRET` 物理隔离
- 诱饵 `.env` 暴露的 `JWT_SECRET` 就是这把密钥 → 攻击者伪造的 token **在蜜罐内验签通过**（先甜）
- `/api/v2/*`：读接口返回合成用户/统计/用户列表；写入与提权返回 403「检测到异地登录，请重新验证」+ 验证页地址（卡）
- `alg=none` 直接拒绝并记录；记录 role/is_admin 篡改特征，卡口**没有成功分支**

**验证页与指纹（`verify.go` + `fingerprint.go`）**
- `/hp/verify`：内联采集脚本 `DOMContentLoaded` 即采；`/hp/fp` 上报恒 `ok:false`；
  `/hp/verify` 提交恒失败且 `retry:true`（诱导继续，但永不放行）
- 采集项：canvas / webgl / fonts / 时区 / 语言 / 屏幕 / DPR / 核数 / 内存 / Client Hints / 自动化痕迹
- 服务端另做 headless 标记（UA 关键字、缺 Sec-Fetch/Accept-Language/Referer），**只标记不拒绝**

**取证包（`forensics.go`）**
- `GET /api/admin/honeypot/export.zip` → `session.json` / `events.jsonl` / `fingerprints.json` / `manifest.json`
- manifest 内含每个文件的 SHA256，响应头带 `X-Bundle-SHA256`；下载后删临时包；导出写 `export` 事件留痕

**规则反哺（`feedback.go`）**
- 从风险 ≥60 的事件提炼 UA（sqlmap/nuclei/…）与路径（`/.env`、`/.git/`…）特征
- 一键写入爬虫阻断名单并 `security.ReloadBotRules()` 即时生效；长度 <4 与重复项自动跳过，写 `feedback` 事件留痕

**后台**：新增「规则反哺」面板（勾选后反哺）、会话详情新增「浏览器指纹」区块、会话/全部取证包下载

### v3.14.0 蜜罐等级 + 日志记录字段勾选（2026-10-24）

**蜜罐（`server/honeypot`，`docs/honeypot-design.md` §5.1）**
- 站长/管理员控制等级：`off` / `low`（仅 `.env`）/ `medium`（+`.git`、后台入口、路径清单）/ `high`（全量 + 延迟 + 命中即封）
- 诱饵：`/.env`（每实例唯一金丝雀）、`/.git/config`、`/admin.php`、`/phpinfo.php`、`/actuator/env`、
  `/wp-login.php`、`/backup/`、`/robots-disallow.txt`
- 诱饵路径无条件注册，命中后按等级判定；**等级不覆盖时与普通 404 无差别**（不暴露蜜罐存在），等级热更新
- 探路记录：常见扫描路径与 NoRoute 可疑 404（`.php/.env/.git/admin/backup/actuator/swagger`）静默记 `decoy_probe`
- 会话聚合 + 事件时间轴 + 取证导出（导出动作留痕）+ 按留存天数清理 + 封禁/解封

**日志记录字段（管理员勾选，同时作用于访问日志）**
- 可勾选：User-Agent / Referer / X-Forwarded-For / Query / 请求头（白名单）/ Body 摘要
- `Cookie`、`Authorization`、`X-API-Key` **强制永不记录**；Body 含 password/token/secret 字样整段省略
- 关掉的字段**不落库**（不只是脱敏）：`middleware/access_log.go` 按策略裁剪后再入队
- 与「日志脱敏」分工：脱敏管「记了之后遮」，本策略管「要不要记」

**后台**：`/admin/honeypot`（概览卡片 / 等级 / 自动封禁与延迟 / 留存 / 六个记录字段复选框 / 会话与事件详情）

**修复**：未建站（DB 未初始化）访问诱饵会 nil panic → 现按 off 静默失效，返回普通 404

### v3.13.0 OC 模式群组：角色卡 / 世界书 / 多智能体角色扮演 / 一键发布话本（2026-10-24）

**定位**：OC（Original Character）是群组的**玩法层**，群组该有的治理（额度、先审后发、举报、取证、软删除留证）全部照旧适用。

**数据与玩法（`server/oc`，`docs/oc-mode.md`）**
- 角色卡：人设/外貌/说话风格 + 场景/开场白/对话示例/系统提示/创作者备注/自带世界书；
  公开需平台审核，删除为软删除留证
- 养成：`mood` / `affinity` / `energy`；AI 发言首行 `[情绪:词]` 解析后调整状态并驱动立绘（`emote_map`）
- 世界书：条目化 Lorebook（主/次触发词、`selective`、`logic`、常驻、概率、插入顺序、优先级、位置、包含组），
  书级 `scan_depth` / `token_budget` / `recursive_scan`；官方世界标记 `is_official`
- 世界书激活引擎：按键触发（不整本塞上下文）+ 包含组互斥 + token 预算截断 + 一轮递归 + 位置装配 + `{{char}}/{{user}}` 宏
- 群组模式：`mixed`（人 + AI 角色）/ `ai_only`（纯 AI 互聊，用户当导演，可单步/暂停/喊停）
- 发言调度：`round_robin` / `random` / `trigger` / `manual`；硬约束 `max_turns` 防无限循环
- 预算：群日预算 + 单流预算 + 角色预算三道闸门，超额自动暂停并提示
- 审核：**多条一起审**的窗口审核（按 N 条/N 轮），等级 `strict` / `loose` / `redteam`（红队需授权说明、隔离只记录、可导出报告）
- 一键发布：对话流 → 话本章节（对话体/叙述体/设定集/AI 润色），AI 标识 + `source_flow_id` 来源追溯；
  「一键」= 一键导入 + 提交审核，不是绕审

**借鉴来源**
- SillyTavern（酒馆）角色卡 V2 与 Lorebook 触发机制（`docs/oc-mode.md` §11.1 逐项对照）
- 小团子/咔伴的养成玩法（互动改变 OC、情绪立绘、官方世界）

**前端**
- 群详情「OC 模式」入口 → `/group/:id/oc`：玩法设置 / 角色卡 / 世界书 / AI 角色 / 对话流五个页签
- 对话流气泡展示（角色 + 模型 + 情绪 + 审核状态），支持推进、暂停、结束、真人以角色发言
- 发布对话框：目标作品、正文形态、轮次范围、设定集、AI 润色、是否立即发布

### v3.12.0 话本增强（表情/图片/章节级混排/AI 模板） + 版本控制 git 后端（2026-10-24）

**话本气泡体增强（`docs/novel-script-mode.md` v2）**
- 气泡内表情：行内 `[笑]` 等标记渲染为表情字符（内置约 40 个词），只在已转义文本上替换；
  编辑器预览可点击插入
- 气泡内图片：整行 `![说明](地址)` 渲染为图片气泡，URL 白名单（http/https/站内相对），
  `javascript:` / `data:` / `file:` 一律拒绝，非法地址降级提示而不拼进 `src`
- 章节级混排：`chapters.mode`（继承作品 / 散文 / 话本 / 混排），`mixed` 下连续非对话行
  聚成散文块走 Markdown，与对话气泡同章共存；阅读器按「章节形态优先」渲染
- 话本专用 AI 模板：新增场景 `novel_script`，内置话本续写、台词口吻校准、场景提示生成、
  对话去水检查 4 个模板；编辑器在话本形态下自动切换场景

**章节版本控制：真实 git 仓库后端（`docs/novel-version-control.md` §8）**
- `chaptervcs.Backend` 抽象：默认自研版本链（零依赖），可选真实 git 仓库（go-git）
- git 后端：`make build-gitvcs`（`-tags gitvcs`）+ 运行时 `NVS_VCS_BACKEND=git`；
  仓库在 `作品目录/.vcsrepo`，分支 `ch{章}/{分支名}`，提交即版本
- 冲突检测 / 三方合并 / 回滚在两个后端上语义一致；差异：git 历史不可删除，引导用回滚
- 未带标签构建不引入 go-git 依赖；后端不可用时自动回落自研版本链，不阻断启动

### v3.11.0 多人协作 + 话本气泡体 + 章节版本控制（2026-10-24）

**多人协作写作（规则先定：`docs/novel-collaboration.md`）**
- 角色：主笔（唯一）/ 协作作者 / 编辑 / 审稿；细粒度能力 `can_publish` / `can_review` / `can_edit_others`
- 邀请需对方接受才生效（pending 不授予任何权限）；收益分成用万分比登记（主笔拿剩余），署名顺序主笔固定第一
- 章节状态机 `draft → pending → published / rejected` + 撤回；未发布章节仅创作成员可见，章节按 `author_id` 署名
- 后端 `server/collab` + `server/handlers/collab`；前端：作品卡片「协作」面板、`/collab`「我的协作」、编辑器工作流条
- 所有协作变更落 `novel_collab_audits`（append-only）

**话本气泡体（script mode，`docs/novel-script-mode.md`）**
- 作品新增叙事形态 `prose` / `script`；话本正文按「角色：台词」逐行解析为气泡，同角色连句合并、左右分侧
- 【场景】为舞台提示、其余为旁白；解析与渲染在服务端完成（`server/scriptmode`），全部转义输出
- 阅读器默认气泡视图可切「原文」；编辑器提供「气泡预览」抽屉；`POST /api/script-preview` 实时预览

**章节版本控制（git 语义，`docs/novel-version-control.md`）**
- 每次保存 = 一次提交（快照落作品目录 `.versions/{章}/{分支}/{版本}.md`，DB 只存元数据 + 哈希）
- 保存携带 `base_version`，落后于服务端 head 即返回 409 冲突现场（base / server / mine），前端并排对比后
  选择「以我的为准」（force，仍生成新版本留痕）或「放弃修改重载」
- 分支与三方合并：以分叉点为 base 做行级三方合并，同处冲突不写库、回传三段内容由作者确认
- 历史：版本时间线、并排 diff、回滚（再提交不抹历史）、删除快照（只清正文，元数据保留，禁删 head）

### v3.10.0 群组实时消息 + 模板上下文自动注入（2026-10-24）

**群组 G4：实时消息（SSE）**
- 后端 `GET /api/groups/:id/stream`：复用 `realtime.Hub` 批量下发，心跳 15s，`X-Accel-Buffering: no`
- 事件：新帖 / 审核结果 / 新回复 / 群公告 / 成员加入与离开；AI 回复同样广播（`ai=true`）
- 群组使用独立 Hub（800ms 批量窗口），不受「狂暴模式」开关影响
- 前端 `api/group/stream.ts` + `composables/useGroupStream.ts`：fetch 流式解析、指数退避重连、页面隐藏暂停
- 群详情头部「实时」状态徽标；他人动态弹通知，自己的操作不打扰

**模板上下文自动注入（§13.2）**
- `prompttpl.BuildNovelVars`：按作者作品自动填充 作品名/类型/简介/大纲/角色设定/前文摘要/前文/正文/作者样本
- 新增记忆类型 `characters`（角色设定），编辑器「管理记忆」可直接编辑保存
- 新接口 `POST /api/ai/templates/:id/render-context`（归属校验：作者本人或管理员）
- 编辑器内容优先：`vars` 里的正文不会被旧章节覆盖；只回传模板实际用到且自动注入的变量，避免大纲整段回显
- 编辑器套用模板时提示「已自动注入：大纲、角色信息…」

### v3.9.0 群组功能 + 提示词模板引擎（2026-10-24）

**群组（独立领域包 server/groups + 接口层 server/handlers/group）**
- 群号（7 位唯一数字）、可见性（公开/半公开/私密）、加入方式（直接/申请/邀请码）、角色（群主/管理员/成员）
- 群管理：拉人、申请审核、踢人、任命管理员、禁言、群公告（变更推送）、解散；配额（总开关/人数硬顶/每日发帖/按角色/按人覆盖）
- 群内帖子与回复：复用违禁词库与「先审后发」；AI 辅助标识接入 `ai_content_marks`
- 群文件：`data/groups` 独立存储、扩展名白名单、50MB/512MB 限额、ZIP 炸弹检测、鉴权下载 + 路径穿越防护
- 群内 `@ai`：绑定智能体（AiEnabled/AiAgentID），复用机器人身份与计费账本，输出过违禁词并带 AI 标识
- 前端：`api/group`、`components/group`（QrCard/QrScanDialog/Posts/Members/Files/Settings）、`views/group`（列表/详情）、`views/admin/group`
- 二维码：本地 qrcode 生成；jsQR 纯前端解析（PC 粘贴/拖入/选择图片，手机摄像头扫码），**图片不上传服务器**；支持手输群号/邀请码

**提示词模板引擎（独立功能包 server/prompttpl + 接口层 server/handlers/prompts）**
- `ai_prompt_templates` + `ai_prompt_template_versions`：变量声明、版本快照与回滚、场景挂载、优先级、启停
- 渲染引擎：`{{变量}}` 纯文本替换、必填/默认值/长度截断、missing/unused 诊断；不执行任何模板指令
- 内置 9 条类型文学默认框架（续写/硬科幻/悬疑/架空历史/实验文学/角色卡/摘要/润色/博客），不含爽文模板
- 管理端 `/admin/ai-templates`：变量编辑、实时渲染预览、版本历史与一键回滚
- 编辑区：AI 助手新增「场景模板」下拉（章节/博客两种场景），套用后自动切「自定义」并提示待补占位

**其他**
- 修复：版本号三处同步（server/version、web/package.json、theme store）

### v3.8.0 AI 管理台与多厂商协议落地（2026-10-23）

**AI 工作台（前后端）**
- 渲染安全：新增 `renderAiMarkdown`（ALLOWED_TAGS 白名单、URI 仅 http(s)/mailto、外链 noopener/nofollow、图片 no-referrer、异常全量转义），流式输出同源渲染（120ms 节流）
- 会话：重命名、分享（公开只读快照 `/ai/share/:token`，不含余额与费用）
- 提示词库：平台预设 19 条 + 用户自定义（上限 200），变量占位，插入输入框/保存当前输入
- 多厂商协议族：`AiModel.api_kind` = openai / mj（含 Niji）/ gemini / claude；Claude 走 `/v1/messages`（x-api-key + anthropic-version）
- 绘画：模型卡片墙、五档比例、混图/换脸（参考图仅 data URL，防 SSRF）、作品发布到灵感广场
- 视频：视频广场（复用作品流 kind=video）
- 智能体：扣子 Coze 接入（v3/chat 提交 → 轮询 → 取 answer，`third_conv_id` 续接多轮）、分类与使用量
- 思维导图：LLM 出层级大纲 → mermaid 渲染
- 体验：厂商维度筛选、`Ctrl+K` 新建对话、沉浸模式、灵感广场独立页 `/ai/gallery`
- 编辑器内 AI：章节编辑器 + 博客编辑器共用 `AiAssistPanel`（续写/润色/扩写/摘要），回填后保存走审核

**AI 管理台（前后端）**
- 数据审计 `/admin/ai-logs`：会话 / 作品 / 调用流水三视图 + 会话详情
- 功能配置 `/admin/ai-features`：BYOK、混图、换脸、上下文条数、每日对话上限、广场公开（均真实生效）、厂商字典、风格字典、扣子默认值
- API 卡池：`ai_model_keys` 多密钥权重轮询、失败计数、连续 3 次自动停用、探活测试、池空回退
- 模型池：`api_kind` / `vendor` / `provider_hint` + 15 个厂商预设一键填充

**国密双轨（默认关闭）**
- `config/crypto.go`（intl/gm/dual + fail-fast）、`crypto/`（Provider + 国际轨 + 纯 Go gmtls + Tongsuo CGO）、`cert/`（SM2 双证书）、`tlsconfig/`（握手 JSONL）、`crypto/secret`、`aiapi`（ProviderHint 合规提示 + 错误分类）、`audit/`（gm-audit / credential-age）、`gmcmd`
- 构建：根 `Makefile`（build / build-gm / build-gm-cgo / test-gm / gm-audit / scan-secrets）、`go.gm.mod`（默认 go.mod 零国密依赖）
- 文档：`docs/gm-mapping.md`（GB/T 39786 映射 + 免责声明）、`scripts/scan-frontend-secrets.sh`

**修复**
- SPA 深链白名单补 12 个根段（`/ai` `/wallet` `/me` `/user` 等直接访问不再 404）
- 导航栏栏目被挤压成竖排（flex-shrink / nowrap / 分档收紧间距）
- 个人中心菜单归置：个人项收进下拉分组，管理员面板 / 警方取管台 / 退出登录独立在外

### v3.7.0 经济系统收尾：免费 AI 额度 + 用户自备密钥 + 长篇续写上下文 + @ai 评论机器人 + 在线支付通道（2026-10-23）

> 1. **免费 AI 额度（§5.3，用尽才扣代币）**：平台默认 `calls_per_day=5`/`max_tokens_perday=0 不限`，按人覆盖（0=禁止/-1=不限）；`UserAiFreeUsage` 按日计数，`AiCallLog.FreeQuota` 列命中即 `cost=0` 不碰账本。编辑区补全、工作台 SSE、绘画、视频四条计费路径统一为「免费优先 → 代币预检」分支；管理端 `/admin/ai-free-quota` 三态单选 + resolveUser 按人覆盖。
> 2. **用户自备 API Key（§5.5 BYOK，不扣平台代币）**：`UserAiKey` AES-256-GCM 加密存储、**只写不读**（添加后不可查看）、上限 5、首个自动默认；调用时以伪模型 `{ID:0}` 零费结算，绘画/视频任务按 `ModelID==0` 解析用户上游。钱包页「我的上游密钥」卡 + 各 AI 面板「使用我的密钥」勾选项（模型可临时改）。
> 3. **长篇 AI 续写上下文（§13.5）**：两级记忆 `AiNovelMemory`（novel 级大纲 + 按 chapter 摘要，唯一索引 upsert）；`BuildNovelContext` 预算组装——尾部窗口 1200 runes 固定保留 → 最近 5 章摘要新→旧填充 → 有余量附大纲（默认 3000 tokens）；上下文 tokens 计入 `tokensIn` 保证预检/计费准确。编辑区「长篇上下文」折叠面板（开关/预算/存摘要/删摘要）。
> 4. **@ai 评论区机器人（§5.6）**：系统用户 `ai_assistant`（随机密码哈希等价禁登录），评论中 `@ai`/`@智能体名`（词边界防 `@aiko` 误触）异步召唤回复——goroutine + recover 故障隔离、每日限流（默认 3 次/人）、同楼 60s 去重、按楼层模型挂靠（ParentID/RootID 继承）；费用由召唤人承担（免费额度→代币预检），回复带「AI」徽标与落款。管理端 `/admin/ai-bot`。
> 5. **在线支付通道（§7.4 微信/支付宝聚合）**：`RechargeOrder` 状态机（pending/paid/closed）+ 档位 KV（固定面额+赠送，拒绝非档位金额）；`MarkOrderPaid` 条件更新 + **同事务入账**，渠道重推/管理端补单均幂等；`server/payment/` 提供者抽象——MockProvider（站内模拟收银台，演示不扣款）与 AggregateProvider（易支付/码支付类 MD5 签名下单验签，金额字符串解析无浮点）。公开回调 `/api/payment/notify/:channel` 验签准入（**验签 + 金额核对 + 幂等**三道闸门）；钱包页档位充值浮层（倒计时 + 2.5s 轮询到账）；管理端 `/admin/payment` 统计卡 + 网关三件套（密钥只写不读）+ 兑换比例/限额/TTL + 档位编辑器 + 订单筛选/补单。
> 6. **版本号 3.6.0 → 3.7.0**（语义化顺延）：`server/version/version.go` / `web/package.json` / `web/src/stores/theme.ts`（window.NVS.version 桥）同步更新；`go test ./...` 全绿（新增 17 组支付/额度/密钥/记忆/机器人单测）、前端构建通过、`nvs-server.exe` 已重建（`orders/mark-paid`、`payment/mock/confirm`、`pay/mock/` 等串已验证嵌入）。

| 类别 | 文件 | 说明 |
|---|---|---|
| 免费额度 | `server/models/ai_free_quota.go` | 配置 KV + 按人覆盖（SQL upsert 保零值）+ 每日用量 + Check/Record/Summarize |
| 免费额度 | `server/models/ai_billing.go` / `ai_media.go` | AiCallLog.FreeQuota 列；结算命中 cost=0；MarkAiMediaFree |
| 免费额度 | `server/handlers/ai_free_quota.go` | 用户状态 + 管理端配置/按人覆盖增删查 |
| BYOK | `server/models/user_ai_key.go` | AES-256-GCM 加密、CRUD/默认/解析（删默认自动提升） |
| BYOK | `server/handlers/user_ai_key.go` + `ai_write.go` / `ai_hub.go` / `ai_hub_media.go` | 我的密钥 CRUD；补全/SSE/绘画/视频 BYOK 零费分支 |
| 长篇上下文 | `server/models/ai_memory.go` | 大纲/章节摘要 upsert + ListRecentChapterSummaries + BuildNovelContext 预算组装 |
| 长篇上下文 | `server/handlers/ai_memory.go` + `ai_write.go` | 记忆 CRUD/上下文预览；system prompt 注入 + tokensIn 计入 |
| @ai 机器人 | `server/models/ai_bot.go` | 机器人用户/限流/召唤解析（词边界防误触） |
| @ai 机器人 | `server/handlers/ai_bot.go` + `comment.go` | 异步触发（recover/去重/楼层挂靠/通知广播）；CreateComment 挂点 |
| 在线支付 | `server/models/payment.go` | 订单状态机、配置/档位 KV、MarkOrderPaid 同事务幂等到账、关单/统计 |
| 在线支付 | `server/payment/{provider,mock,aggregate}.go` | Provider 接口；mock 站内收银台；聚合网关 MD5 签名下单/验签 |
| 在线支付 | `server/handlers/{payment,payment_notify,admin_payment}.go` | 下单/轮询/订单/mock 确认；公开回调验签准入；管理端配置/档位/订单/补单 |
| 路由/迁移 | `server/main.go` / `server/models/db.go` | payment 三组路由 + pay 根段 SPA 放行；5 张新表注册 |
| 前端 | `web/src/api/{ai,aiHub,payment}.ts` | 免费额度/我的密钥/记忆/机器人/支付全量 API + 类型 |
| 前端 | `web/src/views/Wallet.vue` / `PayMock.vue` / `PayResult.vue` | 档位充值浮层（倒计时轮询）；模拟收银台；网关跳转结果页 |
| 前端 | `web/src/views/admin/{AdminAiFreeQuota,AdminAiBot,AdminPayment}.vue` | 三个管理端面板（默认关闭、开启即生效） |
| 前端 | `web/src/components/ai/*` + `editor/AiAssistPanel.vue` + `FloorComments.vue` | 「我的密钥」勾选、免费标记；长篇上下文面板；@ai 召唤提示与 AI 徽标 |

### v3.6.0 实名五提供商 + 配置 .env 自动同步 + Web 项目双维度额度 + 登录方式/实名独立页（2026-10-17）

> 1. **实名核验提供商抽象层（`server/realname/`）**：Provider 接口 + 注册表 + `Verify(ctx,cfg,name,idCard)` 统一路由，五通道落地——`national`（通用 JSON 代理，宽松解析 `{"code":0}`/`{"status":"match"}`/`{"result":true}`/嵌套 `data` 等形态）、`tencent`（TC3-HMAC-SHA256，`faceid.tencentcloudapi.com` IdCardVerification，结果码 0 一致 / -1 不一致 / 异常码抛错不收费）、`aliyun`（云市场 AppCode）、`huawei`（AK/SK + SDK-HMAC-SHA256）、`netease`（MD5 签名网关）。敏感凭据后台脱敏回显（`has_secret` 标记，secret 留空沿用旧值），「测试连接」用非法身份证探测链路（厂商对非法输入不收费）。
> 2. **平台配置保存后自动同步 `.env`（`server/config/envfile.go`）**：原位替换 / 末尾追加 / 引号处理 / CRLF 兼容 / 临时文件原子 rename / 互斥锁；`PlatformConfigEnvKeys` 映射（site_name→SITE_NAME、email_verify→EMAIL_VERIFY_ENABLED、captcha_enabled→CAPTCHA_ENABLED、smtp_*→SMTP_*、log_redact_*→LOG_REDACT_*）。DB 为运行时真相、`.env` 保证重启生效；站点配置保存、脱敏配置保存、安装向导三处挂接，后台保存响应回显 `env_synced/env_note/env_sync_error`。
> 3. **Web 项目公开双维度额度（数量 + 总大小 MB，可同时开启）**：全局 `platform_configs.web_publish_quota_size_mb`（0=不限）+ 按人 `user_web_quotas.quota_size_mb`（-1 跟随 / 0 不限 / >0 专属）；`CheckAndApplyWebPublish` 双维度判定，任一超标转 `review_status=quota_review` 待审并生成审核单，reason 区分维度；大小统计 `UserHTML.TotalSize` + `UserFrame.FilePath` 实时 stat（目录递归）。
> 4. **公开链路收口（堵绕过口子）**：上传 ZIP 勾选「广场展示」、新建 UI 模板勾选「公开分享」不再直写 `is_public`，建记录后统一走 `CheckAndApplyWebPublish`（超额转待审 + `public_note` 提示）；前端 HTML 扩展 / UI 模板 / 作者模板设置的公开开关改调 `/api/web-quota/html-public`、`/api/web-quota/frame-public`。
> 5. **登录方式管理独立页（`/admin/login-methods`）**：微信/邮箱/手机号三开关卡（微信登录附 env 配置指引）、注册验证安全（邮箱验证/滑块）、SMTP 通道配置 + 诊断测试、域名黑白名单、保存后 .env 同步回显；修复存量 bug（`login_methods` 为 JSON 字符串需 `JSON.parse` 后读取）。
> 6. **实名与未成年保护独立页（`/admin/realname`）**：统计卡（总数/未成年/成年/当前提供商）+ RealNameProviderForm 五提供商卡片配置与测试 + 保护策略（realname_required/minor_mode）+ 记录列表（`minor_only` 筛选）；AdminSettings 瘦身为两张快速导航卡。
> 7. **热词面板前端打磨**：三类型（内容/搜索/标签）渐变切换卡 + 统计卡 + 分类型彩色条形 + 排名奖牌色 + 暗色适配。
> 8. **自定义主页（alist 单框 + 双作用域 + 站长审核）**：「我的主页装扮」(`/me`) 仅本人登录态可见，允许原始 HTML/CSS/JS（自 XSS，等同 alist 个人自定义）；「作者公开主页」(`/author/:id`) 他人可见，提交即 `utils.SanitizeCustomHTML` 消毒（剥离 script/iframe/on\*/危险协议，保留布局/样式，「不过度激进」）并置 pending 进审核队列，自动化预检发现被移除项（had_issues）时原文 raw_html 一并送站长复核（按 `role='admin'` 查询、不硬编码），通过则 `public_html = safe_html` 对外可见；前端 `MeDecorate.vue` 双区块（自看原始 / 公开送审）+ `AdminHomeCustomizeReview.vue` 审核台（原文 vs 消毒预览）。
> 9. **编辑器快捷键**：评论区/回复框支持 `Ctrl/Cmd+B/I/U`（粗体/斜体/下划线）+ `Ctrl/Cmd+S` 保存并 `preventDefault` 拦截浏览器「网页另存为」；Tiptap 富文本补 `Ctrl+S`→save；通用工具 `web/src/utils/editorShortcuts.ts`。
> 10. **版本号 3.5.1 → 3.6.0**（语义化顺延）：`server/version/version.go` / `web/package.json` / `web/src/stores/theme.ts`（window.NVS.version 桥）同步更新，`go test ./...` 全绿、`npm run build` 通过、`nvs-server.exe` 已重建（新功能字符串已验证嵌入）。

| 类别 | 文件 | 说明 |
|---|---|---|
| 实名提供商 | `server/realname/provider.go` | Provider 接口 + 注册表 + Verify 路由 + 包级 httpClient（测试注入）+ 宽松 JSON 解析辅助 |
| 实名提供商 | `server/realname/national.go` | 通用代理 + parseLooseResult 多形态兼容 |
| 实名提供商 | `server/realname/tencent.go` | TC3-HMAC-SHA256 签名（CanonicalRequest→StringToSign→密钥派生）+ parseTencentResp |
| 实名提供商 | `server/realname/aliyun.go` | AppCode 简单认证，GET/POST 与参数名可配 |
| 实名提供商 | `server/realname/huawei.go` | SDK-HMAC-SHA256 + regionFromEndpoint + verify_result 兼容解析 |
| 实名提供商 | `server/realname/netease.go` | 参数 ASCII 排序拼 MD5 签名 + 网关 code≠200 先拦 |
| 实名提供商 | `server/realname/realname_test.go` | 9 组离线单测（签名计算/响应解析/httptest 全链路/未配置哨兵） |
| .env 同步 | `server/config/envfile.go` | SyncEnvFile 原位替换/原子写/互斥锁 + PlatformConfigEnvKeys + jsonArrayToCSV |
| .env 同步 | `server/config/config.go` | Init 记录实际加载路径（SetEnvFilePath/GetEnvFilePath） |
| .env 同步 | `server/handlers/admin_site.go` | UpdatePlatformConfig 保存后同步 + 派生字段（wechat_login_available 等） |
| .env 同步 | `server/handlers/admin_redact.go` / `setup.go` | 脱敏配置三键同步；安装向导 site_name 同步 |
| 双维度额度 | `server/models/web_quota.go` | 大小额度常量/配置/按人覆盖（map upsert 保零值）+ pathSizeBytes/CountPublicWebSize + CheckAndApplyWebPublish 双维度 reason |
| 双维度额度 | `server/handlers/web_quota.go` | MyWebQuota 大小字段；Admin 配置端点 max_size_mb/quota_size_mb；未提交维度沿用旧值 |
| 双维度额度 | `server/models/web_quota_test.go` | 5 组测试（零值 upsert 回归/目录递归/按人豁免/双维度判定） |
| 公开收口 | `server/handlers/userhtml.go` / `userframe.go` | 上传/新建公开勾选建记录后走 CheckAndApplyWebPublish + public_note |
| 前端 | `web/src/components/admin/RealNameProviderForm.vue` | 新增：五提供商卡片 + 动态凭据表单 + 保存/测试连接 |
| 前端 | `web/src/views/admin/AdminLoginMethods.vue` | 新增：登录方式/SMTP/域名名单独立页 |
| 前端 | `web/src/views/admin/AdminRealName.vue` | 重写：统计卡 + 提供商表单 + 保护策略 + minor_only 筛选 |
| 前端 | `web/src/views/admin/AdminSettings.vue` | 瘦身：移除搬迁四区，改快速导航卡 |
| 前端 | `web/src/views/admin/AdminWebQuota.vue` / `web/src/views/user/WebQuota.vue` | 双维度额度 UI（全局/按人 MB 配置、数量+大小双卡、大小 tag） |
| 前端 | `web/src/views/frame/HTMLManager.vue` / `FrameManager.vue` / `AuthorTemplateSettings.vue` | 公开开关改走额度接口 + 超额待审 badge + need_review 提示 |
| 前端 | `web/src/views/admin/AdminHotWords.vue` | 三类型切换卡/统计卡/彩色条形/奖牌色打磨 |
| 自定义主页消毒 | `server/utils/htmlcustom_sanitize.go` | `SanitizeCustomHTML`（保留结构/样式、剥离脚本/危险标签/事件/危险协议，style 走 `SanitizeCSS`；8 组单测） |
| 自定义主页模型 | `server/models/home_customize.go` | `UserHomeCustomize`（自看原始）/ `AuthorHomeCustomize`（公开消毒+审核，raw/safe/public 三态 + had_issues） |
| 自定义主页接口 | `server/handlers/home_customize.go` | `/me/home-customize`、`/author/:id/home-customize`、`/admin/home-customize/reviews`；预检出问题按 role 通知站长（不硬编码） |
| 前端 | `web/src/views/me/MeDecorate.vue` / `web/src/views/admin/AdminHomeCustomizeReview.vue` | 双区块装扮页 + 站长审核台（原文 vs 消毒预览） |
| 前端 | `web/src/utils/editorShortcuts.ts` | 编辑器快捷键工具（B/I/U/S，拦截浏览器保存）；CommentSection/FloorComments/RichTextEditor/BlogEditor/ChapterEditor 接入 |

### v3.5.1 管理员用户搜索/筛选（按角色分类 + 状态）（2026-08-30）

> 1. **后端多维筛选**：`GET /api/admin/users` 新增 `role`（reader/author/vip_author/arbitrator/police/admin）与 `status`（active/muted/banned）查询维度，与既有 `search` 关键字（用户名/昵称/邮箱）自由组合。
> 2. **封禁用户可见**：`status=banned` 走 `Unscoped()` 查询，使被软删除封禁的账号也能在用户管理中检索出来（普通状态默认排除软删除，符合「看到软删除留证」的合规诉求）；`active` 排除软删除且排除在禁言期内记录，`muted` 仅含禁言未到期用户。
> 3. **前端筛选栏**：「用户管理」页搜索栏新增「角色分类」「状态」下拉（`el-select` + `clearable`），任意维度变更即触发检索（`@change="doSearch"`），`adminApi.getUsers` 签名升级为接收 `{ page, search, role, status }` 参数对象。

### v3.5.0 警方用户角色与取管台（2026-08-30）

> 1. **新增 police 角色（准管理员）**：权限模型扩展 `police` 角色。警方账号**不可自注册**（注册恒为 reader，且无 role 注入路径），只能由管理员在「用户管理」中通过专用提权接口（`POST /api/admin/users/:id/promote-police`）显式提权，防止越权/漏洞提权；撤销走 `POST /api/admin/users/:id/revoke-police`（回退为 reader）。
> 2. **提权审计留痕**：提权/撤销均写入 emergency 审计日志（`user_promote_police` / `user_revoke_police`），并在 `users` 表记录 `police_granted_by` / `police_granted_at`；通用「编辑用户」接口拒绝直接设置 police 角色，强制走审计通道。
> 3. **警方取管台（`/police-console`）**：独立取证台，仅 admin / police 可访问（`/api/police/*`）。可查看全部软删除留证（`softdel_*` 表，满足「看到软删除」），按类型筛选，单条查看完整快照并**下载取证凭证（JSON）**，以及一键「导出全部取证包」（含软删除留证 + 隔离墙访问 + Web 访问 + 用户审计），便于提前准备取证材料。
> 4. **权限边界**：police 仅能进取管台，看不到 `/api/admin` 普通管理功能（用户封禁、内容治理等）；前端后台大屏与导航新增「警方取管台」入口。

### v3.4.1 网络诊断与攻击流量面板（2026-08-30）

> 1. **独立诊断面板**：新增「网络诊断与攻击流量」入口（/admin/net-diag），名义上用于查看用户分布、诊断网络，同时识别防爬与攻击流量。
> 2. **全量 Web 访问日志入库**：新增 `WebAccessLog` 表，由 `AccessLog` 中间件异步批量写入（非静态资源）；记录 IP、X-Forwarded-For 跳数、UA 家族、身份、状态码、耗时，并复用 `security` 包做轻量 bot 判定。
> 3. **后端接口**：`GET /api/admin/web-access`（IP/UA/家族/身份/状态/爬虫/日期多维筛选 + 分页）、`GET /api/admin/web-access/stats`（累计/今日/爬虫/近24h攻击流量、Top IP、Top 路径、状态码分布、UA 家族分布、爬虫原因分布）、`POST /api/admin/web-access/cleanup`（清空，合规证据由文件日志承担）。
> 4. **TTL 说明**：HTTP 应用层无法读取 IP 层 TTL，面板以 X-Forwarded-For 跳数近似代理深度，并在详情抽屉标注该限制。

### v3.4.0 隔离墙访问仪表盘（176号令合规可视化）（2026-08-30）

> 1. **隔离墙进入记录前台可视化**：新增「隔离墙访问记录」面板，把原本只能在数据库看到的敏感分区进入明细（谁、何时、进入哪个分区、IP、User-Agent）以仪表盘形式呈现。
> 2. **后端接口升级**：`GET /api/admin/zone-access` 支持 `user_id`/`zone`/`from`/`to` 筛选与分页，联表带出用户名/昵称；新增 `GET /api/admin/zone-access/stats` 聚合统计（累计/今日/去重用户/分区分布）。
> 3. **后台入口**：管理大屏新增「隔离墙访问记录」卡片，跳转 `/admin/wall-access`。

### v3.3.2 管理员 API Key 封禁与按人限流 + 自建论坛额度 + 楼层式折叠评论 + 扩展安全预检封存（2026-08-30）

> 1. **管理员对 API Key 的「按把封禁」与「按人限流」**：
>    - 按把：`POST /api/admin/ratelimit/keys/:id/ban|unban`，封禁后该 Key 请求直接 403（`api_keys` 新增 `is_banned/ban_reason/banned_at/banned_by`）。
>    - 按人（新增 `user_api_policies` 表）：`keys_banned` 封禁某人全部 Key（含以后新建的，支持 `duration_hours` 到期自动解封）；`quota_per_min/window_secs` 专属配额，按 `min(角色上限, 专属值)` 生效；`max_keys` 限制持有 Key 数。管理接口 `GET/PUT/DELETE /api/admin/ratelimit/users/:id/quota`、`POST /api/admin/ratelimit/users/:id/keys/ban|unban`。
>    - 生效链路：`models.ResolveAPIKey` 叠加用户策略并置 `KeyIdentity.Banned` → `APIKeyGuard` 命中封禁即 403（记日志 `key_banned`）；`CreateAPIKey` 建前校验封禁与 Key 数上限。前端 `AdminRateLimit.vue` 新增封禁/解封、封禁状态列、「按人管控」面板（用户名解析 → 设配额 → 封禁/解封 → 已设置用户表）。
> 2. **管理员限制每人自建论坛数量（全局默认 + 按人削减/放宽）**：全局 `platform_configs.forum_quota`（`max_per_user`，0=不限制，默认 3）；按人 `user_forum_quotas` 表（`max_forums`：-1=跟随全局 / 0=完全剥夺建版权 / >0=专属上限）。`CreateUserForum` 建版前走 `ForumQuotaFor` 校验；`GET /api/forums/mine` 升级为 `{ list, quota }`。前端 `Forums.vue` 展示「已创建 X / 上限 Y」、达上限禁用创建按钮；`AdminForums.vue` 新增额度管控面板。额度接口独立于 `/admin/forum-quota` 子树（`/forums/:id` 已占用通配，注册 `/forums/quota` 会路由 panic）。
> 3. **楼层式折叠评论（公众号评论区 + Discuz 楼中楼），论坛/博客/作品/作者/章评全量统一**：
>    - 语义：甲发主内容 → 乙评论甲 = 开 1 楼；丙评论乙 → 挂 1 楼显示「丙 回复 乙」；丁评论丙 → 仍在 1 楼；戊评论甲（楼主）→ 脱离开 2 楼。即「回复主内容开新楼，回复楼内任意一条挂靠该楼」。
>    - 后端：`comments` 与 `posts` 均新增 `root_id/floor_no`（posts 另加 `parent_id/reply_count`），`resolveFloor()` 落库前推导；分页改为「按楼层分页」并新增 `floor_total/has_more`；楼内回复回填 `reply_to_name`；删楼层主评/主回帖级联删楼内回复（回帖走 `softdel_posts` 留证）；楼内互评新增 `post_reply` 通知；`AutoMigrateAll` 幂等回填存量。
>    - 前端：新增共用组件 `FloorComments.vue`（平铺列表按 `root_id` 还原成楼，楼内默认显示 3 条、其余折叠，楼主徽标/举报/删除/版主禁言按位透出）；`ThreadDetail`、`CommentSection`（作品/博客/作者/章评）统一改用它，各入口补传 `owner-id`。
> 4. **应用扩展安全预检未通过 → ZIP 原样封存不解压，管理员审核后可公布**：
>    - 上传链路改为「解压前纯内存预检」（`precheckZip`，单文件限 8MB，ZIP 炸弹检测前置）：命中 critical/high → 原样封存到 `data/userhtml/_quarantine/<uid>/`，建 `review_status=quarantine` 记录（`is_extracted=false/is_active=false/is_public=false`），**不解压、不落盘任何扩展文件**，并通知全部管理员；仅中/低危告警 → 已解压但强制不公开、挂 pending。
>    - 管理员审核：`AdminReviewHTML`（`/api/admin/html-reviews/html/:id/review` 与 `/api/admin/userhtmls/:id/review` 共用）——放行（approve）对封存件按封存的 ZIP 补一次解压（再验 ZIP 炸弹）、锁定目录、置 active，`publish=true` 同步上广场；驳回（reject）置 rejected，`purge_file=true` 连封存件删除。详情页对封存件回显「未解压 + 阻断原因」。
>    - 前端 `AdminHtmlReview.vue` 新增「已封存」筛选与封存告警弹窗（阻断原因 + 放行将补解压提示）。
> 5. **版本号 3.2.8 → 3.3.2**（语义化顺延）：`server/version/version.go` / `web/package.json` 同步更新，前端 `npm run build` + Go `go build` 通过，`nvs-server.exe` 已重建。

| 类别 | 文件 | 说明 |
|---|---|---|
| API Key 管控 | `server/models/api_key.go` | `APIKey` 增加 `is_banned/ban_reason/banned_at/banned_by`；新增 `UserAPIPolicy` 表模型（按人封禁/专属配额/Key 数上限）；`ResolveAPIKey` 叠加用户策略并置 `Banned`；`SaveUserAPIPolicy` 用 map 显式 upsert（规避 GORM 零值/默认值坑） |
| API Key 管控 | `server/handlers/admin_apikey_policy.go` | 新增：按把封禁/解封、按人封禁/解封、专属配额读写、策略列表与撤销 |
| API Key 管控 | `server/handlers/api_key.go` | `CreateAPIKey` 建前校验用户封禁与 Key 数上限；`ListMyAPIKeys` 返回实际生效配额；管理员 Key 列表附 `user_banned/user_quota/effective_req` |
| API Key 管控 | `server/security/ratelimit.go` | `APIKeyGuard` 命中封禁即 403（`key_banned` 入访问日志） |
| 论坛额度 | `server/models/forum.go` | 新增 `UserForumQuota`（按人建版额度）、`ForumQuotaConfig`（全局默认）、`ForumQuotaFor` 计算、`SaveUserForumQuota` map upsert（`max_forums=0` 曾被 GORM 静默回落成默认 -1） |
| 论坛额度 | `server/handlers/admin_forum_quota.go` | 新增：全局额度读写、按人设置/撤销/列表 |
| 论坛额度 | `server/handlers/forum_custom.go` | `CreateUserForum` 额度校验（区分「被管理员限制」与「已达上限」）；`/api/forums/mine` 返回额度 |
| 楼层评论 | `server/models/comment.go` / `forum.go` | `root_id/floor_no`（posts 加 `parent_id/reply_count`）、`resolveFloor`、楼层分页、`reply_to_name` 回填、级联删除、存量回填 |
| 楼层评论 | `server/handlers/comment.go` / `forum.go` / `forum_custom.go` / `emergency.go` | 楼层分页响应（`floor_total/has_more`）、`CreatePost` 支持 `parent_id`、删楼级联删回复、删除按楼数扣计数 |
| 楼层评论 | `web/src/components/FloorComments.vue` | 新增共用楼层组件（折叠回复、楼主徽标、回复/删除/禁言） |
| 楼层评论 | `web/src/views/ThreadDetail.vue` / `CommentSection.vue` / `BlogDetail.vue` / `NovelDetail.vue` / `AuthorHome.vue` / `Reader.vue` | 统一楼层渲染 + 各入口补传楼主 `owner-id` |
| 扩展预检 | `server/handlers/userhtml.go` | 上传改「解压前纯内存预检」：命中高危 → 原样封存（`_quarantine`）、不解压；通过才解压；pending 强制不公开 |
| 扩展预检 | `server/handlers/admin_html_review.go` | 新增：管理员列表/详情/审核（放行补解压 + 公布，驳回可删封存件） |
| 扩展预检 | `server/handlers/html_review.go` | 封存件详情回显阻断原因；复核委托提前到 body 解析之前（避免二次读 body 报参数错误） |
| 扩展预检 | `server/models/userframe.go` | `is_extracted/quarantine_path/scan_blocked/scan_block_reason/reviewed_by/reviewed_at`；列默认 false + 存量回填 |
| 扩展预检 | `web/src/views/admin/AdminHtmlReview.vue` | 「已封存」筛选、封存告警弹窗、公布/删件勾选 |
| 版本 | `server/version/version.go` / `web/package.json` | 版本号 3.2.8 → **3.3.2** |

### v3.2.8 已登录用户免限流 + 聚合 API 真正落地 + 限流三连修（2026-08-28）

> 1. **已登录浏览器用户免限流（访客仍受限）**：`APIKeyGuard` 此前只识别 `X-API-Key`，从不识别浏览器登录态，导致已登录用户被当作匿名按 IP 限流。现于 `APIKeyGuard` 内解析 `token` cookie（JWT），命中有效登录态即设置 `CtxKeySkipIP=true` 并放行——既绕过匿名/爬虫 IP 限速，又因为路由级 `RateLimit` 同样读取 `CtxKeySkipIP` 而同步豁免；**访客（无有效 token）仍按匿名/爬虫配额限流**。仅针对浏览器会话，API Key 程序化访问走独立 Key 配额、不受影响。
> 2. **聚合 API 真正落地（一次请求拿整页）**：后端 `/api/batch` 聚合中间层早已完整实现（并发 goroutine + 内存内转发 + 统一响应），但前端仅有客户端、`无任何页面调用`，属半成品。本版真正打通：
>    - 新增 `batchGet()` 便捷封装（`web/src/api/batch.ts`），一次调用批量 GET 并按 `id` 返回映射。
>    - **新增应用级聚合 store** `web/src/stores/app.ts`：导航栏/页脚等所有页面共享的"外壳"数据（站点信息、分类、互联站点、未读数、安装状态）与**首页内容**（公告、算法说明、分类统计、作品列表）在**首屏落地页时并入同一批** → **落地页首屏严格 1 次 `/api/batch`**；`NavBar` / `SiteFooter` / `Home.vue` 全部消费该 store（SPA 内后进入首页由 `ensureHomeLoaded()` 幂等兜底）。首屏落地页的初始路由判定用 `location.pathname` + 站点前缀（仅对初始加载成立，无竞态）。
>    - 实测（无头 Chromium 直连真实服务，已登录用户落地 `/`）：**1 次 `/api/batch` + 2 次框架级请求（`/api/auth/me`、`/api/setup/status`）= 共 3 次请求完成整页首屏**，**页面数据直连请求数 = 0**，控制台 0 错误，导航栏/页脚/主容器全部正常渲染。原首屏约 13 次独立往返。
> 3. **限流三连修（真实服务压测发现的三个预置 bug）**：
>    - **静态资源不再被 API 限流**：`APIKeyGuard` 挂在根引擎且未跳过非 `/api/` 路径，一次页面加载的 ~110 个静态资源请求会瞬间耗尽匿名配额（60/min）→ CSS/JS 中途 429 → **页面裸奔（即此前「CSS UI 全乱掉」的根因）**。现改为仅对 `/api/` 接口限流，静态资源与 SPA 页面路由一律放行。
>    - **路由级限流按「IP+路由」分桶**：`RateLimit` 原用纯 IP 桶，不同路由互相挤占配额（聚合子请求并发时低配额路由必 429）。现桶键为 `IP|路由`，各接口独立计数。
>    - **`/api/batch` 子请求免重复限流**：子请求经 `engine.ServeHTTP` 会重新走 APIKeyGuard，把 1 次聚合放大成 N 次配额消耗。现子请求携带进程级随机密钥头（`X-NVS-Int-Batch`，客户端无法伪造），`APIKeyGuard` 识别后跳过重复计数并置 `CtxKeySkipIP`——**1 次 batch 只计 1 个限流单位**。真实服务实测：修复前 6 个子请求中 announcements / categories-stats 必 429，修复后全部 200 且连续 3 轮稳定。
> 4. **版本号 3.2.5 → 3.2.8**（语义化顺延）：`server/version/version.go` / `web/package.json` 同步更新，前端 `npm run build` + Go `go build` 通过，`nvs-server.exe` 已重建。

| 类别 | 文件 | 说明 |
|---|---|---|
| 限流 | `server/security/ratelimit.go` | 新增 `isLoggedInBrowser()`（解析 `token` cookie 判定登录态）、`isInternalBatchSub()`（进程级密钥校验）；APIKeyGuard 静态资源放行 / 已登录免限流 / batch 子请求免重复计数；路由级 `RateLimit` 改按 `IP\|路由` 分桶 |
| 限流 | `server/handlers/batch.go` | 子请求携带 `X-NVS-Int-Batch` 内部密钥头（防伪造，1 次聚合只计 1 个限流单位） |
| 限流 | `server/security/login_exempt_test.go` | 新增测试：登录判定四态、已登录 50 连发不 429、内部 batch 子请求放行 + 伪造头被拒 |
| 聚合 | `web/src/api/batch.ts` | 新增 `batchGet(specs[])` 便捷封装（GET 批量 → 以 id 为键的结果映射） |
| 聚合 | `web/src/stores/app.ts` | 应用级 store：外壳数据 + 首页内容，首屏落地页并入同一批 → 严格 1 次 `/api/batch`；`ensureHomeLoaded()` 幂等兜底 |
| 聚合 | `web/src/App.vue` | bootstrap 提前到 setup 同步启动（子组件挂载前发起，无竞态） |
| 聚合 | `web/src/components/NavBar.vue` | 改为消费 `useAppStore`，移除三个独立请求；未读数首屏不再单独拉取 |
| 聚合 | `web/src/components/SiteFooter.vue` | 改为消费 `useAppStore`，备案号/站点名不再单独请求 |
| 聚合 | `web/src/views/Home.vue` | 首页内容统一由 store 聚合通道提供（`ensureHomeLoaded()`）；删除重复的「标签横排/大方块」切换器（cz-header 排版） |
| 聚合 | `server/handlers/batch.go` | **gzip 修复**：内存内子请求转发时删除 `Accept-Encoding`——否则 gzip 中间件压缩子响应，`json.Unmarshal` 失败、`data` 退化为乱码字符串，前端聚合数据全空（首页分类/小说消失的根因，仅真实浏览器触发） |
| 版本 | `server/version/version.go` / `web/package.json` | 版本号 3.2.5 → **3.2.8** |

### v3.2.5 API Key 管理下沉：作者工作台 + 管理员可管（2026-08-28）

> 1. **作者工作台新增 API Key 管理**（`AuthorDashboard.vue`）：作者可在「创作数据大屏」直接创建 / 启停 / 重生成 / 删除自己的 API Key，并查看每个 Key 的调用次数，附「🕷️ API Key 爬虫指南」入口。逻辑与 `Settings` 完全一致（复用 `apiKeyApi`，在角色配额上限内自设更小配额）。
> 2. **管理员面板升级为可管理**：`AdminRateLimit.vue` 的「全站 API Key 一览」由只读升级为可管理——新增「重生成 / 停用·启用 / 删除」操作列（重生成后一次性展示完整 Key）。
> 3. **后端新增管理员 Key 管理端点**（绕过归属校验，仅 `ensureAdmin`）：`POST /api/admin/ratelimit/keys/:id/regenerate`、`PATCH /api/admin/ratelimit/keys/:id`、`DELETE /api/admin/ratelimit/keys/:id`；前端 `adminApi` 补充 `regenerateApiKey / updateApiKey / deleteApiKey`。
> 4. **普通用户独立管理页**（`MyApiKeys.vue`）：所有登录用户均可从用户菜单进入「🔑 我的 API Key」（`/my-keys`），拥有与作者工作台同等的完整 API Key 获取/管理能力（创建 / 启停 / 重生成 / 删除 / 调用次数），并直达「🕷️ API Key 爬虫指南」。解决此前 API Key 管理深埋于「账号设置」、普通用户难以发现的问题。
> 5. **版本号 2.3.4 → 3.2.5**（语义化顺延，本次为功能性版本跃迁）：`server/version/version.go` / `web/package.json` 同步更新，前端 `npm run build` + Go `go build` 通过，`nvs-server.exe` 已重建。

### v2.3.4 API Key 爬虫指南内置为独立页面（2026-08-28）

> 将 v2.3.3 的 `docs/api-key-crawler.md` 由「仓库 Markdown」升级为**随二进制部署的应用内独立页面**，运行时可直接打开，无需读源码：
> 1. **新增独立页面** `web/src/views/APIKeySpider.vue`，路由 `/api-spider`（公开，与 `/docs` 同级），内容覆盖申请 Key、Python `requests` 调用、公开接口表、分页遍历、429 退避重试、调用次数查看、安全注意事项，并内置「🧪 在线测试」区——可粘贴自己的 Key 同源实测一个请求，自动带上反向代理前缀。
> 2. **导航入口**：用户下拉菜单与移动端抽屉新增「API Key 爬虫指南」；`Settings` 的 API Key 卡片提示改为指向该页的 `router-link`。
> 3. **仓库 Markdown 保留**为源码级参考，顶部注明已内置同名应用内页面。

| 类别 | 文件 | 说明 |
|---|---|---|
| 页面 | `web/src/views/APIKeySpider.vue` | 新增：API Key 爬虫接入指南独立页面（含在线测试） |
| 路由 | `web/src/router/index.ts` | 新增 `/api-spider` 公开路由 |
| 导航 | `web/src/components/NavBar.vue` | 桌面下拉 + 移动抽屉新增入口（Connection 图标） |
| 设置 | `web/src/views/Settings.vue` | API Key 卡片提示指向 `/api-spider` |
| 文档 | `docs/api-key-crawler.md` | 顶部注明已内置同名应用内页面 |
| 版本 | `server/version/version.go` / `web/package.json` | 版本号 2.3.3 → **2.3.4** |

### v2.3.3 管理子页居中补全 + API Key 调用次数 + 爬虫接入文档（2026-08-28）

> 1. **管理子页居中**：`/admin/users`（用户管理）此前使用独立 `.page-container`，风格与其他 `/admin/*` 子页不一致；统一改为 `AdminHeader` + `.admin-page-body` 布局，与限流/设置等页保持一致的居中列。其余 `/admin/*` 子页已由 v2.3.2 前后若干 commit 完成居中，本次补齐最后一块。
> 2. **API Key 累计调用次数**：`api_keys` 与 `federated_peer_keys` 新增 `total_calls` 字段，每次携带有效 Key 的请求通过后原子 +1；重新生成 Key 时归零。用户侧「API Key 管理」与管理端「限流与 API Key」均新增「调用次数」列，便于审计与配额规划。
> 3. **爬虫接入文档**：新增 `docs/api-key-crawler.md`，说明如何申请 Key、如何用 Python `requests` 携带 `X-API-Key` 爬取公开接口、分页遍历与 429 退避重试示例。

| 类别 | 文件 | 说明 |
|---|---|---|
| 管理页 | `web/src/views/UserManagement.vue` | 统一使用 `admin-page`/`AdminHeader`/`admin-page-body`，继承全局居中规则 |
| API Key | `server/models/api_key.go` | `APIKey`/`FederatedPeerKey` 新增 `total_calls int64` |
| API Key | `server/security/ratelimit.go` | 每次有效 Key 请求原子累加 `total_calls`，`last_used_at` 保持分钟级去重 |
| API Key | `server/handlers/api_key.go` | 重生成 Key 时 `total_calls` 归零 |
| 前端 | `web/src/views/Settings.vue` | Key 列表增加「调用次数」列 |
| 前端 | `web/src/views/admin/AdminRateLimit.vue` | 管理员 Key 一览增加「调用次数」列 |
| 文档 | `docs/api-key-crawler.md` | Python requests 爬虫接入指南 |
| 版本 | `server/version/version.go` / `web/package.json` | 版本号 2.3.2 → **2.3.3** |

### v2.3.2 修复「清空限流」对 IP 行为降级失效（管理员自锁死）（2026-08-28）

> 真实浏览器端到端验证中发现两处连环缺陷，均位于管理员「限流与 API Key → 清空限流」解封链路上：
> 1. **清空不解除降级**：`ClearRateLimits()` 只重建了配额表 `rateLimitStore`，未重置 IP 行为统计表 `botIPStore`。被「单 IP 高频」降级为爬虫的 IP，点清空后仍被判 bot，只能等 TTL 窗口自然滚动，管理员无法立即解封。
> 2. **管理员自锁死**：管理端接口（含清空按钮本身）同样参与爬虫降级判定，当管理员自身 IP 已降级时，清空请求会先被 429 挡掉、handler 根本不执行 —— 用来解封的按钮被它要解封的配额锁死，第 1 点的修复也因此无法生效。
>
> 修复后：清空操作同时重置 IP 行为统计；管理端接口（`/api/admin/*`，本身已有 JWT 鉴权，爬虫降级对其无防护价值）不再参与爬虫降级，仅走匿名配额，保证管理员在任何限流状态下都能进后台自救。

| 类别 | 文件 | 说明 |
|---|---|---|
| 安全 | `security/bot.go` | 新增 `ResetBotIPStats()`：在 `botIPMu` 保护下重建 `botIPStore`，供清空限流调用 |
| 限流 | `security/ratelimit.go` | `ClearRateLimits()` 调用 `ResetBotIPStats()`，解封立即生效；`APIKeyGuard` 对 `/api/admin/*` 跳过爬虫降级（JWT 已鉴权，避免自锁死） |
| 安全 | `security/bot_test.go` | 新增 `TestClearRateLimits_ResetsBotIPStats` 回归测试（先打阈值触发降级 → 清空 → 断言恢复为正常访问）；已用「临时回退修复」验证该测试确实会失败（`reason=ip_high_frequency`），确保测试有效 |
| 版本 | `version/version.go` / `web/package.json` | 版本号 2.3.1 → **2.3.2** |

> 验证方式：真实启动服务（SQLite）后以 curl 逐项实测 —— 无头爬虫 UA 精确放行 5/min、伪装头（浏览器 UA 缺 `Sec-Fetch-*`）5/min、普通匿名 >9/min、爬虫 UA 携带有效 API Key 突破至 Key 配额、无效 Key 打回 5/min、已知名头爬虫（GPTBot/Meta/Bytespider）403 且响应头带 `X-Nvs-Blocked`、主流搜索引擎（Googlebot）放行 200。

### v2.3.1 已知名头爬虫直接阻断 + 配额层级修正 + 双数据库字段加固（2026-08-28）

> 爬虫处置由「一律限流」升级为「**三级分诊**」：① 携带有效 API Key → 高配额（不判爬虫）；② 无 Key 普通匿名 → 低配额；③ 无 Key 且识别为爬虫 → **更低**配额；④ 命中「已知名头爬虫」名单 → **直接 403 阻断**，不消耗配额。
> **配额关系修正**：此前「爬虫 10/min、远低于匿名 120/min」的定位有误，正确约束是「匿名本身就很低（120 → 60/min）、爬虫比匿名更低（10 → 5/min）」，从而把提速的唯一通道收敛到 API Key —— 爬虫脚本想加快速率，必须申请并在请求中携带 Key。
> **阻断名单可配置**：内置覆盖 AI 训练/生成爬虫与商业 SEO 采集器，管理员可在后台追加（含某些搜索引擎爬虫）；主流搜索引擎（Googlebot/Bingbot/Baiduspider）默认不阻断以免误伤收录。
> **双数据库兼容**：按 MySQL 严格模式逐字段复核列长，避免 `Data too long`；SQLite 侧同步建表验证。

| 类别 | 文件 | 说明 |
|---|---|---|
| 模型 | `models/api_key.go` | `APIRateLimitConfig` 新增 `BotBlockKeywords`（自定义阻断名单，与内置名单取并集）；默认配额修正：匿名 120 → **60**/min、爬虫 10 → **5**/min；`FederatedPeerKey.KeyCipher` 512 → **1024**（AES-256-GCM hex = 2×明文+56，容纳最长 484 字符的对端 Key）；`ApiAccessLog.Path` 256 → **512**（长 URL + 查询串避免 MySQL `Data too long`） |
| 模型 | `models/api_key_test.go` | 新增配额层级断言（Key > 匿名 > 爬虫、爬虫 ≤ 匿名/4）；**MySQL 列类型契约测试**（用 MySQL 方言推导新增三表列类型：string 必须落在 `varchar(N)`、建索引列不超 InnoDB 索引长度上限、单行 utf8mb4 不超 65535）；字段长度契约测试（逐字段锁定最小长度与推导依据）；SQLite 上界写入测试（独立内存库，避免测试间共享污染） |
| 安全 | `security/bot.go` | 拆分内置名单：`builtinBotKeywords`（无头/脚本库 → 低配额）与 `builtinBlockKeywords`（AI 训练爬虫 + 商业 SEO 采集器 → 403）；新增 `IsBlockedBot()` 阻断判定、`BuiltinBlockKeywords()` 管理端展示；`ReloadBotRules` 加载阻断名单 |
| 限流 | `security/ratelimit.go` | `APIKeyGuard` 前置阻断分支：命中阻断名单直接 403 + 响应头 `X-NVS-Blocked`，以 `identity_type=blocked` 落库审计（按 IP 每分钟去重），不消耗配额计数 |
| 安全 | `security/bot_test.go` | 新增 `TestIsBlockedBot`（AI 爬虫命中 / 主流搜索引擎放行 / 自定义关键词大小写不敏感）、`TestIsBlockedBot_NotBlockMainstreamSearchEngines`（防止误把主流搜索引擎写进内置阻断名单） |
| 处理 | `handlers/api_key.go` | `GetRateLimitConfig` 一并下发 `builtin_block_keywords`，管理端可展示内置名单 |
| 管理端 | `views/admin/AdminRateLimit.vue` | 新增「已知名头爬虫：直接阻断（403）」配置区：自定义阻断关键词 + 内置名单标签墙 + 与「低配额识别规则」的差异说明；配额说明改为四档层级；日志筛选新增「已阻断」类型 |
| 用户端 | `views/Settings.vue` | API Key 提示改为四档层级表述，明确「批量抓取/自动化脚本想提速请带 Key」 |
| 版本 | `version/version.go` / `web/package.json` | 版本号 2.3.0 → **2.3.1** |

### v2.3.0 四层身份 API Key 体系 + 爬虫识别限流 + 联邦双向 Key（2026-08-28）

> 将限流从「一刀切 IP 计数」升级为「**基于身份的多层级流量管理**」：站长/作者/读者/联邦四层身份 Key + 匿名/爬虫通道各自独立配额（管理员后台统一配置）。**爬虫识别**（无头 UA / 伪装浏览器 UA / 单 IP 高频行为）自动走远低于匿名的爬虫配额，Meta 等商业爬虫无法通过任何"无 Key"方式绕过。**联邦 Key 双向可见**：两站互换 Key 后，被链接的站能识别对方用的是哪个 Key、是谁签发的。

| 类别 | 文件 | 说明 |
|---|---|---|
| 模型 | `models/api_key.go`（新）+ `api_key_test.go`（新） | 三张新表：`api_keys`（用户级 Key，仅存 SHA-256 哈希）/ `federated_peer_keys`（联邦双向 Key，出站明文 AES-256-GCM 加密）/ `api_access_logs`（按身份访问日志，限流/携带 Key/爬虫才落库）；配额配置 JSON（`api_key_quota`）读写与夹取 |
| 模型 | `models/db.go` | `AllModels` 注册三张新表（AutoMigrate 自动建表） |
| 模型 | `models/platform_federated.go` | `federated_inbound` 入站记录扩展 `key_prefix` / `issuer` 字段 —— 被链接的站可见对方所用 Key 与签发方 |
| 限流 | `security/ratelimit.go` | 新增 `APIKeyGuard` 身份感知限流中间件（Key 配额滑动窗口 / 匿名与爬虫低配额 / 敏感认证接口保留 IP 限流防撞库 / 访问日志与 last_used 节流防写放大）；`RateLimit` 对「有效 Key + 读取请求」放行 IP 限流；`RateLimitStats` / `ClearRateLimits` 管理接口 |
| 限流 | `security/bot.go`（新）+ `bot_test.go`（新） | 爬虫识别：内置 + 自定义 UA 关键词（HeadlessChrome/curl/GPTBot/MetaWebIndexer 等）、浏览器 UA 缺 Sec-Fetch-* 伪装检测（可关）、单 IP 窗口内高频自动降级（阈值/窗口可配） |
| 处理 | `handlers/api_key.go`（新） | 用户侧 Key CRUD（`/api/apikeys`，配额自设 ≤ 角色上限，支持 IP 白名单/过期/重生成）；管理员侧限流配置 / 日志 / 统计 / 全站 Key 一览（`/api/admin/ratelimit/*`） |
| 联邦 | `handlers/admin_site.go` | 联邦 Key 管理（生成本站签发 Key 一次性展示 / 保存对方签发 Key / 启停 / 吊销，`/api/admin/sites/:id/keys/*`）；`SyncFederatedSite` 出站自动携带对方 Key；`recordInbound` 携带联邦身份写入入站记录 |
| 路由 | `main.go` | `APIKeyGuard` 挂全局中间件链；挂载 apikeys / admin ratelimit / sites keys 路由；启动时重载爬虫规则 + 清理过期日志 |
| 加密 | `config/secure.go` | 导出 `EncryptSecret` / `DecryptSecret`（机器密钥 AES-256-GCM，供联邦出站 Key 落库） |
| 管理端 | `views/admin/AdminRateLimit.vue`（新） | 各身份配额配置、爬虫规则、限流状态统计与清空、访问日志（身份/Key 前缀/命中原因）、全站 Key 一览 |
| 管理端 | `views/admin/AdminSites.vue` | 「联邦 Key」弹窗：生成/转交/保存/启停/吊销，双向可见提示 |
| 用户端 | `views/Settings.vue` + `api/apikey.ts`（新） | 个人 API Key 管理：创建（配额/过期/IP 白名单）、列表、启停、重生成、删除 |
| 路由 | `router/index.ts` / `views/AdminDashboard.vue` | 新增 `/admin/ratelimit` 页面与入口菜单 |
| 版本 | `version/version.go` / `web/package.json` | 版本号 2.2.2 → **2.3.0** |

### v2.2.2 修复公告栏渲染 + 版本号顺延（2026-08-25）

> 公告栏 CSS 误写在 `<style scoped>` 标签之外（Vue SFC 顶层裸文本被编译器丢弃），导致首页公告栏**无样式裸渲染**；另公告详情弹窗原以纯文本显示，无法展示 Markdown 格式的公告内容。同时将实际运行版本号顺延至 **v2.2.2**（此前 AppVersion/package.json 停留在 2.2.0）。

| 类别 | 文件 | 说明 |
|---|---|---|
| 修复 | `views/Home.vue` | ① 公告栏 CSS（`.notice-bar` / `.notice-item` / `.notice-more` 等）从 SFC 顶层移入 `<style scoped>`，恢复首页公告条样式（黄色渐变横幅 / 置顶标签 / 查看全部）；② 公告详情弹窗改用 `renderMarkdown`（Cherry 引擎 + DOMPurify 兜底，与阅读器/博客一致）渲染 Markdown 内容 |
| 版本 | `version/version.go` / `web/package.json` | 版本号顺延：`AppVersion` 与 `package.json version` 2.2.0 → **2.2.2**（与版本记录最新版本一致，页脚/健康接口/建站向导/启动日志同步） |

### v2.2.1 版本号统一管理 + 页脚开源署名（2026-08-25）

> 全站版本号不再散落硬编码 `1.0.0`：新增 Go `version` 包集中管理（可 `-ldflags` 覆盖），健康接口 / 建站向导 / 数据库管理 / 健康页 / 启动日志全部同步为 **v2.2.0**；前端页脚参考 alist 增加「由 NVS 驱动 + GitHub 仓库链接」开源署名，版本号取自 `package.json`（与后端单一来源同步）。

| 类别 | 文件 | 说明 |
|---|---|---|
| 版本 | `version/version.go`（新） | `version.AppVersion = "2.2.0"` 集中管理，支持 `-ldflags "-X nvs-server/version.AppVersion=vX.Y.Z"` 构建时覆盖 |
| 版本 | `handlers/health.go` / `handlers/setup.go` / `handlers/db_admin.go` | `/api/health` `version`、`/api/setup/status` `server_ver`、`/api/admin/db/health` `version` 改引用 `version.AppVersion` |
| 版本 | `main.go` | 健康页「版本 1.0.0」与启动日志 `NVS Server v1.0.0` 改引用 `version.AppVersion` |
| 版本 | `web/package.json` | `version` 1.0.0 → 2.2.0（前端版本单一来源） |
| 署名 | `web/src/components/SiteFooter.vue` | 页脚新增「由 NVS 驱动 v2.2.0」+ GitHub 仓库链接（`HerryABU/nvs-community-web`，alist 风格），与站点名/ICP/公安备案同排常驻显示 |

### v2.2.0 站点路径前缀挂载（反向代理）+ 静态缓存 + 管理面板 UI（2026-08-24）

> 新增 **URL_PREFIX 站点路径前缀挂载**：整站可挂到任意子路径（如 `:8088/go-nvs/login`）供反向代理/网关使用，js/css/API/路由/跳转全自动感知前缀，**前缀名任意配置、严禁硬编码**；同时修复静态资源缓存（解决刷新重复全量下载）与管理面板菜单布局。

| 类别 | 文件 | 说明 |
|---|---|---|
| 路由 | `config/config.go` | 新增 `URL_PREFIX` 环境变量 + `normalizePrefix` 规范化（必须以 `/` 开头、拒绝 `..`/`//`） |
| 路由 | `middleware/urlprefix.go`（新）+ `urlprefix_test.go`（新） | ① 前缀剥离：gin 外层将 `/go-nvs/xxx` → `/xxx`；② HTML 注入：相对 `./assets/` 改写为 `{prefix}/assets/` 绝对前缀（不依赖浏览器 `<base>` 解析）+ `<base href>` 兜底 + `window.__NVS_PREFIX__` |
| 路由 | `main.go` | `StripURLPrefixHandler` 包裹 gin 引擎；SPA 回退白名单（已知前端根段才回退 index.html，杜绝前缀/组件穿越） |
| 前端 | `vite.config.ts` | `base: './'` 相对路径产物（首屏 js/css、动态 chunk、modulepreload 全相对化） |
| 前端 | `utils/prefix.ts`（新） | `getAppPrefix()` 智能解析：后端注入的 `__NVS_PREFIX__` 优先；为空时从 `document.baseURI`（受代理改写 `<base href>` 影响）推导浏览器实际挂载前缀——兼容自建反向代理只改写 base 的场景 |
| 前端 | `api/index.ts` / `router/index.ts` / `stores/auth.ts` | axios baseURL、Vue Router base、401 跳转、退出跳转全部带前缀 |
| 前端 | `api/frame.ts` / `NavBar.vue` / `Home.vue` / `HtmlPlaza.vue` / `HTMLManager.vue` / `AdminCompliance.vue` / `AdminDatabase.vue` | 全部 `window.open` / `<a :href>` / fetch 裸 URL 统一拼前缀（删净裸 `/api`、`/login`） |
| 缓存 | `main.go` | 静态资源缓存策略：`assets/*`（文件名带内容 hash）→ `immutable` 一年；其余静态 → 1h；index.html → `no-cache`（发布新 hash 后浏览器立即拿到） |
| 日志 | `middleware/header_logger.go` | 静态资源请求只打一行 `(static)` 摘要，API 保留完整审计头（日志去刷屏） |
| UI | `views/AdminDashboard.vue` | 管理面板 21 项功能菜单改为 flex 弹性换行，修复 4 列网格最后一行空位歪斜 |

### v2.1.9 176号令合规补全 + 全站安全审计闭环 + 日志脱敏 + 公告栏（2026-08-24）

> 围绕《互联网信息服务深度合成管理规定》等 176 号令开展**合规补全**（算法透明度 / 备案引导 / 等保自测 / 漏洞管理 / 检查工具箱 / 公告栏）与**两轮全站安全审计**（JWT 密钥 / 路径穿越 LFI / CSRF / SSRF / IDOR / 信息泄露 9+ 项全部闭环），并新增**日志敏感信息脱敏**。

| 类别 | 文件 | 说明 |
|---|---|---|
| 脱敏 | `config/redact_config.go`（新）+ `middleware/redact.go`（新） | 日志敏感信息脱敏引擎：Header/Body JSON/Query 三类脱敏，密码类固定 `******`，其余可配置掩码；`.env` 种子化 + DB 持久化 + 运行时热更新 |
| 脱敏 | `middleware/access_log.go` / `config/config.go` | 访问日志接入环境感知脱敏（生产自动脱敏，开发保留原始）；新增 `LOG_REDACT_SENSITIVE` / `LOG_REDACT_MASK` / `LOG_SENSITIVE_FIELDS` |
| 脱敏 | `handlers/admin_redact.go`（新）+ `views/admin/AdminLogRedact.vue`（新） | 管理后台脱敏配置页：开关 + 敏感字段列表 + 掩码 + 预览工具（GET/PUT `/api/admin/log-redact`、POST `preview`） |
| 合规 | `handlers/public_compliance.go`（新）+ `handlers/admin_vuln.go`（新） | 算法透明度（公开 `/api/algorithms/description` + 用户偏好 `/api/users/me/preferences` 关闭权）；漏洞整改闭环管理（CRUD + 导出 + AdminVulnerabilities.vue） |
| 合规 | `handlers/admin_compliance.go`（新）+ `handlers/admin_toolbox.go`（新） | 等保/关基自测问卷 + 定级记录（AdminCompliance.vue）；检查工具箱：日志按时间导出 ZIP / 用户清单 CSV / 合规自查报告 |
| 合规 | `handlers/admin_announce.go`（新）+ `models/announcement.go`（新）+ `AdminAnnouncements.vue`（新） | 公告栏：管理端 CRUD + 首页公告条 + 列表/详情弹窗（置顶优先） |
| 合规 | `docs/compliance/`（新） | 三份合规制度模板（内容审核 / 应急处置 / 安全策略）+ README |
| 安全 | `config/secure.go` / `config.go` | S1 JWT 默认密钥 → 自动生成随机密钥持久化 `data/.jwt_secret`（防伪造管理员 Token） |
| 安全 | `middleware/csrf.go`（新）+ `csrf_test.go`（新） | CSRF 防护：登录态写操作强制 `X-Requested-With` 头，前端 axios 拦截器统一附加 |
| 安全 | `handlers/userhtml.go` / `wasm.go` | S2/H1 路径穿越 LFI → `pathWithin`（filepath.Rel 强校验，保存端+读取端）；上传白名单移除 `.svg` |
| 安全 | `handlers/auth_login.go` / `auth_register.go` | M1 登录账号级失败锁定（5 次/15 分钟）；M2 注册接口限流 RateLimit(10,60) |
| 安全 | `handlers/health.go` / `utils/gorm_logger.go` | L1 `/api/health` 信息收敛（移除驱动/原始错误/计数）；L2 DB 审计日志参数化（密码/邮箱/Token 不落盘） |
| 安全 | `handlers/port.go` / `sandbox_vfs.go` / `forum.go` / `emergency.go` | L4 端口分配 IDOR 校验归属；M4 VFS write/delete 登录 + 归属校验；L5 隐藏论坛元数据（`X-Forum-Room`/`X-Forum-Password` 头）；敏感信息移出 URL（应急预览/删除改 POST body） |
| 安全 | `handlers/proxy.go` + `nginx/nginx.conf` | M5 联邦同步 SSRF：仅 http/https + 拒绝回环/内网/链路本地 + DNS 解析校验 + 10s 超时；nginx 加固 CSP/Referrer-Policy + HTTPS 443 示例 |
| 前端 | 全部沙盒 iframe | 移除 `allow-same-origin`（opaque origin 隔离）；mermaid `securityLevel` 改 strict（存储型 XSS 防护） |
| 审计 | `docs/security-audit.md`（新） | 两轮审计全部发现项修复记录与运维层方案 |

### v2.1.8 话题标签 + 搜索扩展（2026-08-23）

> 新增**话题（#话题）**：博客可参与多个话题、全站话题列表/详情聚合页；**搜索升级**：除作品/作者/博客/扩展外，新增**专栏 / 话题 / 论坛**；博客广场移除分类筛选（保留标签云）；作者主页明确展示**作者专栏**。

| 类别 | 文件 | 说明 |
|---|---|---|
| 话题 | `models/blog_topic.go`(+`blog_topic_test.go`) | `blog_topics` + `blog_topic_links` 表；SyncBlogTopics、ListBlogTopics（文章数统计）、FillBlogTopics |
| 话题 | `handlers/blog_topic.go` + `main.go` | `GET /api/topics`、`GET /api/topics/:id`、`GET /api/topics/:id/blogs`、`POST /api/topics`（登录） |
| 话题 | `handlers/social_blog.go` | CreateBlog/UpdateBlog 支持 `topics`（null=不修改）；列表/详情返回 topics |
| 话题 | `views/Topics.vue` + 路由 | `/topics` 话题列表（卡片）+ `/topics/:id` 话题详情（参与博客分页） |
| 话题 | `views/BlogEditor.vue` / `BlogDetail.vue` | 编辑器话题多选（可创建）；详情页「参与话题」pill 跳话题页 |
| 搜索 | `handlers/novel_public.go` | UnifiedSearch 新增 categories（已公开专栏）/ topics / forums（公开论坛） |
| 搜索 | `components/HomeSearchResults.vue` + `views/Home.vue` | 结果区新增「📚 专栏 / 🔥 话题 / 💬 论坛」三栏展示 |
| 广场 | `views/BlogList.vue` | 移除左侧分类筛选栏，保留「🏷 热门标签」侧栏；tag 参数筛选保留 |
| 主页 | `views/BlogHome.vue` | 作者分类栏标题明确为「📚 作者专栏」 |

### v2.1.7 分类独立路由页（CSDN 式文章管理）（2026-08-23）

> 「迁移」与「分类设置」从弹窗升级为**独立路由页面**；分类文章管理仿 CSDN 创作中心：批量新增/移动文章、未分类筛选；导入博客可指定目标分类。

| 路由 | 页面 | 说明 |
|---|---|---|
| `/author/blog/{用户名}/category/{id}` | `BlogCategoryManage.vue` | 分类文章管理：Tab「本分类文章」（勾选批量移动到其他分类/移出未分类 + 单篇移动）+ Tab「新增其他文章」（未分类/其他分类勾选批量加入本分类，CSDN「批量操作：新增」式） |
| `/author/blog/{用户名}/category/{id}/settings` | `BlogCategorySettings.vue` | 分类设置：改名 / 挂靠父分类 / 排序 / 删除 / 状态与拒绝原因 |
| 后端 | `models/blog.go` | `GetBlogsByAuthorFilter`：category_id=nil 全部 / *0 未分类 / *N 指定分类；exclude_category_id 排除 |
| 后端 | `handlers/social_blog.go` + `main.go` | `POST /api/blogs/batch-category` 批量设置分类（归属校验、≤200 篇）；`ImportBlogs` 支持 `category_id`（导入到指定分类）+ 返回 `created_ids` |
| 前端 | `api/social/index.ts` | `batchSetCategory`；`listByAuthor` 支持 category_id=0 未分类 / exclude_category_id；`importBatch` 支持分类 |
| 前端 | `views/BlogManager.vue` | 分类行按钮改为跳独立页「📂 管理文章 / ⚙ 设置」，移除原弹窗 |
| 前端 | `views/BlogEditor.vue` | 批量导入弹窗新增「导入到分类」下拉（个人专栏） |

### v2.1.6 博客个人专栏 + 标签 + 媒体（2026-08-23）

> 分类体系改为**个人专栏**（CSDN 式：每个作者自己的分类，去"平台分类"概念）；分类可点进去查看并**批量迁移博客**；博客支持 **#标签**（标签云/筛选/编辑）；新增**通用上传接口** + 编辑器**图片上传 / 音视频嵌入**。

| 类别 | 文件 | 说明 |
|---|---|---|
| 标签 | `models/blog_tag.go`(+`blog_tag_test.go`) | `blog_tags` + `blog_tag_links` 表；SyncBlogTags（去重/去#）、GetBlogTags、标签云、按标签查博客 |
| 标签 | `handlers/social_blog.go` | CreateBlog/UpdateBlog 支持 `tags`；列表/详情返回 tags；`GET /api/blog-tags` 标签云；`GET /api/blogs?tag=` 筛选 |
| 个人专栏 | `models/blog_category.go` | `GetAuthorCategoryTree`：某作者已公开分类树（仅 approved，他人父分类上提） |
| 个人专栏 | `handlers/blog_category.go` + `main.go` | `GET /api/author/:id/blog-categories` 公开接口 |
| 个人专栏 | `views/BlogManager.vue` | 分类管理只显示**我的分类**（个人专栏），去掉"平台分类"区；每分类「📂 博客」→ 弹窗查看该分类博客并**批量迁移** |
| 个人专栏 | `views/BlogEditor.vue` | 分类下拉改自己的分类（含待审核标注） |
| 个人专栏 | `views/BlogHome.vue` / `BlogList.vue` | 作者主页分类栏=该作者个人专栏；广场列表支持 tag 参数 + 标签云 |
| 上传 | `handlers/upload.go` + `main.go` | `POST /api/upload` 通用上传（图片≤10MB / 音视频≤50MB，白名单扩展名） |
| 编辑器 | `views/BlogEditor.vue` | Cherry `imageUpload` 走统一上传；「🎬 视频 / 🎵 音频」按钮（上传或粘贴 URL 插入） |
| 详情 | `views/BlogDetail.vue` | 显示 #标签（点击跳全站标签筛选） |
| API | `api/social/index.ts` | listByAuthor 支持 category 筛选；getTags / getAuthorCategories / uploadApi |

### v2.1.3 统一用户索引 + 博客分类导入（2026-08-23）

> 修复 `/author/{用户名}/blogs/{id}` 无法访问的 NaN bug；博客管理支持把已有博客导入/归入分类（单篇 + 批量）。

| 功能 | 涉及文件 |
|---|---|
| **统一用户索引方法** `resolveUserKey`（数字=主键直接使用；用户名→`/api/users/resolve` 大小写不敏感；带缓存；`verifyNumeric` 供区分"数字=用户还是博客id"） | `web/src/utils/userKey.ts`（新） |
| 博客详情页修复：`Number("qbz95")=NaN` 导致 `/author/{用户名}/blogs/{id}` 404 → 改统一索引 | `web/src/views/BlogDetail.vue` |
| 博客主页/管理页改用统一索引（替代各自直调） | `web/src/views/BlogHome.vue`、`web/src/views/BlogManager.vue` |
| **博客分类导入**：每篇博客「设分类」下拉（归入一/二/三级分类或未分类）+「批量分类」勾选多篇一键移动 | `web/src/views/BlogManager.vue` |
| 冒烟测试扩展至 44 项（作者博客列表/详情用户名兼容） | `server/smoke_test.sh` |

### v2.1.2 全面审查补遗（2026-08-23）

> 全站核查"功能遗漏/模块干扰/前端接口路径"：无跨模块干扰；修复 4 处遗漏。

| 功能 | 涉及文件 |
|---|---|
| `ListAuthorBlogs`（`/api/author/:id/blogs`）兼容用户名 | `server/handlers/social_blog.go` |
| BlogHome 公开主页分类栏升级三级（语义修正：不再用全站计数过滤） | `web/src/views/BlogHome.vue` |
| BlogEditor 编辑分类路径恢复竞态修复 | `web/src/views/BlogEditor.vue` |
| BlogList 移动端三级分类样式适配 | `web/src/views/BlogList.vue` |

### v2.1.1 分类管理下放给普通作者（2026-08-23）

> 作者在「我的博客管理」的 **分类管理** 标签页即可：新增分类（一级/二级/三级）、编辑名称、**设置挂靠父分类**、**拖拽挂靠**（拖到另一分类上成为其子分类）、删除分类；管理员后台同样保留入口。

| 功能 | 涉及文件 |
|---|---|
| 分类 CRUD 鉴权从管理员放宽为**登录用户**（作者/管理员均可；层级/同名/防成环校验不变） | `server/handlers/blog_category.go`（`Create/Update/DeleteBlogCategory` + `requireLogin`）、`server/main.go` |
| 普通路由 `POST/PUT/DELETE /api/blog-categories`（admin 组路由保留，指向同一实现） | `server/main.go` |
| **作者端分类管理 UI**：Tab 标签页 + 新增/编辑/删除 + 拖拽挂靠 + 挂靠父分类下拉 | `web/src/views/BlogManager.vue` |
| 前端 API 封装 | `web/src/api/social/index.ts`（`createCategory/updateCategory/deleteCategory`） |
| 单元测试适配新函数名 | `server/handlers/recycle_softdel_test.go` |
| 冒烟测试扩展至 41 项（普通作者新增/挂靠/删除分类） | `server/smoke_test.sh` |

### v2.1.0 URL 规范化 + 博客管理/主页 + 三级分类（2026-08-23）

| 功能 | 涉及文件 |
|---|---|
| **作者博客管理页** `/author/blog/{用户名}`（仅本人，数字 id 兼容；他人访问自动转公开主页） | `web/src/views/BlogManager.vue`（新）、`web/src/router/index.ts`、`web/src/views/AuthorDashboard.vue` |
| **公开博客主页** `/blog/{用户名}`（数字 id 兼容；数字非用户时兼容旧单篇链接自动重定向） | `web/src/views/BlogHome.vue`（新）、`web/src/router/index.ts` |
| **用户标识解析**：数字=主键 id，用户名兼容（大小写不敏感）；作者主页/博客列表/博客详情/阅读页全部支持 | `server/handlers/author_profile.go`（`ResolveUserKey`）、`server/handlers/social_blog.go`、`server/main.go`、`web/src/api/admin.ts`、`web/src/views/AuthorHome.vue`、`AuthorBlogs.vue` |
| **/novel/:id 规范重定向** → `/author/{authorId}/novel/:id`（数字主键）；阅读链接规范化 | `web/src/views/NovelDetail.vue` |
| **分类支持三级**（一级→二级→三级；四级拒绝；删除/成环校验；树递归组装） | `server/models/blog_category.go`、`server/handlers/blog_category.go`、`server/models/blog.go`、`web/src/views/BlogList.vue`、`BlogEditor.vue`（级联选择器）、`admin/AdminBlogCategories.vue` |
| 单元测试：用户解析 + 三级分类层级 | `server/handlers/recycle_softdel_test.go` |
| 冒烟测试扩展至 36 项（用户名解析/三级分类/一级过滤含三级） | `server/smoke_test.sh` |

### v2.0.0 功能更新（2026-08-23）

> 本次更新：2 个严重 bug 修复 + 博客体系升级 + 软删除治理完善 + 测试体系。

#### 1️⃣ 严重 bug 修复

| 改动 | 涉及文件 |
|---|---|
| **访问日志中间件 64KB 截断请求体**（长章节/长博客 JSON 被截断 → 保存报「没有标题/参数错误」，根因） | `server/middleware/access_log.go` |
| **批量导入永远新建小说**（不追加到当前作品） | `server/handlers/import_export.go`、`web/src/api/novel.ts`、`web/src/views/Editor.vue` |
| 创建作品标题 trim 必填校验（空标题友好提示） | `server/handlers/novel.go` |
| 博客创建绑定错误信息更明确（「请填写博客标题和内容」） | `server/handlers/social_blog.go` |
| 章节保存后重复建章（id 标记为 -1 而非真实 id） | `web/src/views/ChapterEditor.vue` |

#### 2️⃣ 博客体系升级（分类 / 点赞 / 评论 / 目录）

| 功能 | 涉及文件 |
|---|---|
| **CSDN 式一/二级分类**（新表 `blog_categories` + `author_blogs.category_id`） | `server/models/blog_category.go`（新）、`server/models/blog.go`、`server/models/db.go`、`server/handlers/blog_category.go`（新）、`server/handlers/social_blog.go`、`server/main.go` |
| 分类管理（管理员任意增删改 + 同名/层级约束） | `web/src/views/admin/AdminBlogCategories.vue`（新）、`web/src/api/admin.ts`、`web/src/views/AdminDashboard.vue`、`web/src/router/index.ts` |
| 博客列表左侧分类栏 + 一级过滤含二级 | `web/src/views/BlogList.vue` |
| 博客编辑器分类下拉 | `web/src/views/BlogEditor.vue` |
| **点赞**（新表 `blog_likes` + `like_count` 冗余计数，toggle） | `server/models/blog_category.go`、`server/handlers/blog_category.go`、`server/main.go`、`web/src/views/BlogDetail.vue`、`web/src/api/social/index.ts` |
| 评论（复用 Comment.blog_id 链路 + 总数展示） | `web/src/components/CommentSection.vue`、`web/src/views/BlogDetail.vue` |
| **MD 可折叠目录**（#~###### 目录树、节点折叠、滚动高亮） | `web/src/views/BlogDetail.vue` |
| **目录面板折叠 + 全屏阅读** | `web/src/views/BlogDetail.vue` |

#### 3️⃣ 软删除治理（用户自删 vs 管理员封禁）

| 功能 | 涉及文件 |
|---|---|
| 新增 `deleted_by` 字段（user=自删 / admin=封禁） | `server/models/novel.go`、`server/models/comment.go`、`server/models/blog.go` |
| 用户删除标记 user；管理员封禁标记 admin（含级联章节） | `server/handlers/comment.go`、`server/handlers/admin_softdel.go` |
| 解封/恢复清除 deleted_by | `server/handlers/admin_ban.go` |
| **作者回收站**（自删内容可恢复，封禁内容拒绝） | `server/handlers/recycle.go`（新）、`server/main.go`、`web/src/api/novel.ts`、`web/src/views/AuthorDashboard.vue` |
| **管理员回收站监管**（全站用户自删列表 + 恢复） | `server/handlers/recycle.go`、`web/src/views/admin/AdminBlogCategories.vue` |

#### 4️⃣ 测试体系

| 类型 | 涉及文件 |
|---|---|
| 单元测试：访问日志 body 还原/脱敏/ multipart | `server/middleware/access_log_test.go`（新） |
| 单元测试：导入解析/标题清理 | `server/handlers/import_export_test.go`（新） |
| 单元测试：博客分类/点赞模型 | `server/models/blog_category_test.go`（新） |
| 集成测试：deleted_by 区分/回收站权限/分类管理 API | `server/handlers/recycle_softdel_test.go`（新） |
| 端到端冒烟测试（30 项） | `server/smoke_test.sh`（新） |

---

## 🌐 项目结构

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
│   ├── browser-func.mjs           # 功能级回归（排序参数/密度/朗读/落库）
│   └── browser-me-center.mjs      # 个人中心与下拉菜单专项
│
├── server/                        # Go 后端
│   ├── main.go                    # 入口：配置初始化 → 数据库 → 子包 Migrate → 路由 → SPA fallback → 启动
│   ├── embed.go                   # 发布轨：//go:embed all:dist（需先 make dist）
│   ├── embed_nodist.go            # 业务轨（-tags nodist）：嵌入提示页，不依赖前端产物即可编译/测试
│   ├── nodist_placeholder/        # 上面那轨嵌的提示页（告诉你怎么构建真正的发布物）
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
│   ├── models/                    # 数据层（80 个文件 / 120+ 结构体）
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
│   ├── handlers/                  # API 处理器（139 个文件，按功能拆分）
│   │   ├── auth_*.go              # 注册 / 登录 / 验证码与改密（含账号锁定与审计）
│   │   ├── novel*.go / chapter*.go# 作品与章节读写（SHA256 + HMAC 签名、软删除归档）
│   │   ├── comment.go             # 评论（楼层分页 + sort=new/old/hot + 加密楼 + 取证视图）
│   │   ├── ai_*.go                # AI 对话（流式）/ 绘画 / 视频 / 智能体 / 模型池 / 路由 / 成本 / 语音
│   │   ├── admin_*.go             # 后台：用户 / 举报 / 论坛 / 站点 / 合规 / 漏洞 / 额度 / AI
│   │   ├── police.go              # 警方取管台：软删除查看 / 凭证下载 / 全量取证包 / 论坛巡查
│   │   ├── social_*.go            # 关注 / 博客 / 话题 / 私信 / 黑名单 / 特别关注
│   │   ├── home_customize.go      # 个人主页装扮（双作用域 + 消毒 + 站长审核）
│   │   ├── emergency.go           # 内容抢救台（二级密码 + 硬删除 + 审计）
│   │   ├── setup.go / db_admin.go # 建站向导 / 数据库健康·热迁移·备份恢复
│   │   └── context.go             # 鉴权上下文统一封装（currentUserID / UserRole 等）
│   │
│   ├── aiclient/  aiapi/  aicall/ # 多厂商模型协议（OpenAI 兼容 / Anthropic / Gemini / MJ / Kling / Coze）
│   ├── chaptervcs/  collab/       # 章节版本控制后端 / 协作
│   ├── groups/  oc/  prompttpl/  realtime/   # 群组 / OC 模式 / 提示词模板 / SSE 推送
│   ├── honeypot/  gate/  risk/    # 蜜罐（诱饵·影子认证·指纹·取证）/ 准入闸门 / 充值风控
│   ├── payment/  police/  audit/  # 支付通道 / 警方取证 / 审计
│   ├── middleware/                # 鉴权 / CSRF / 访问日志 / 脱敏 / 前缀剥离 / 沙盒头 / 端口代理
│   ├── security/                  # 限流 / XSS 净化 / 内容签名 / 内容扫描 / 违禁词 / 滑块验证码 / 正则安全
│   ├── markdown/  scriptmode/     # Markdown 渲染（Mermaid/chemfig）/ 话本气泡体解析
│   ├── realname/  crypto/  tlsconfig/  utils/  version/
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
│       ├── utils/                 # prefix / sensitiveZone / userKey / adminHealth / editorShortcuts / turndown
│       ├── styles/                # global.css（设计令牌）/ scale.css（间距·字号·容器档位）/ admin.css
│       ├── views/                 # 50 个顶层页面 + 5 个子目录
│       │   ├── Home / Login / Register / SetupWizard
│       │   ├── NovelDetail / Reader（精读·朗读·划线）/ Editor / ChapterEditor
│       │   ├── AuthorDashboard / AuthorHome（封面+分栏）/ AuthorBlogs / AuthorTemplateSettings
│       │   ├── BlogList / BlogDetail / BlogEditor / BlogHome / BlogManager / BlogCategory*
│       │   ├── Forums / ForumDetail / ThreadDetail / Topics / Feed / Bookshelf / ReadingNotes
│       │   ├── AiHub（竖列：对话/广场/绘画/视频/灵感/智能体/思维导图）/ Membership / PayMock / PayResult
│       │   ├── CollabWorks / Live / MessageCenter / Wallet / MyApiKeys / APIKeySpider / DocsPage
│       │   ├── admin/ — 48 个后台页（AdminAiPool / AdminPoliceConsole / AdminWallAccess / AdminNetDiag …）
│       │   ├── me/    — MeLayout + 概况/作品/博客/留言/数据/装扮（「我的服务」四组入口）
│       │   ├── user/  — 私信 / 黑名单 / 特别关注 / 微信绑定 / 网页额度
│       │   ├── group/ — 群组列表 / 详情 / 发帖
│       │   └── frame/ — FrameManager / HTMLManager / HtmlRunner / WasmRunner / SandboxPreview
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

---

## 🗄️ 数据字典与数据库改进

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

---

## 🔌 API 一览

所有 API 均在端口 `8080` 的 `/api` 路径下。

### 认证
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/auth/register` | — | 注册 |
| POST | `/api/auth/login` | — | 登录（限流 5次/60s） |
| POST | `/api/auth/logout` | — | 登出 |
| POST | `/api/auth/send-code` | — | 发送邮箱验证码 |
| POST | `/api/auth/verify-code` | — | 验证邮箱验证码 |
| GET | `/api/auth/me` | JWT | 当前用户信息 |

### API 聚合
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/batch` | — | 一次请求批量并发获取多个接口数据（限流 30次/60s） |

### 作品
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/novels` | — | 作品列表（分页/搜索/排序） |
| GET | `/api/novels/:id` | — | 作品详情 |
| POST | `/api/novels` | 作者 | 创建作品 |
| PUT | `/api/novels/:id` | 作者 | 更新作品 |
| DELETE | `/api/novels/:id` | 作者 | 删除作品 |
| GET | `/api/categories` | — | 分类列表 |
| GET | `/api/categories/stats` | — | 分类作品统计 |

### 章节
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/novels/:id/chapters` | — | 章节目录 |
| GET | `/api/novels/:id/chapters/:num` | — | 章节内容 |
| GET | `/api/novels/:id/chapters/:num/verify` | — | 章节完整性验证（哈希+签名） |
| POST | `/api/novels/:id/chapters` | 作者 | 创建章节 |
| PUT | `/api/novels/:id/chapters/:num` | 作者 | 更新章节 |
| DELETE | `/api/novels/:id/chapters/:num` | 作者 | 删除章节 |

### 评论
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/comments` | — | 评论列表（支持 `novel_id` / `blog_id`） |
| POST | `/api/comments` | JWT | 创建评论（限流 20次/60s，支持小说和博客） |
| DELETE | `/api/comments/:id` | JWT | 删除评论 |

### 博客
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/blogs` | — | 博客列表（`category_id` 过滤，一级自动含三级子孙） |
| GET | `/api/blogs/:id` | — | 博客详情 |
| GET | `/api/blogs/:id/likes` | — | 点赞数 + 当前用户是否已赞 |
| POST | `/api/blogs/:id/like` | JWT | 点赞 / 取消点赞（toggle） |
| GET | `/api/blog-categories` | — | 博客分类树（一/二/三级 + 博客数） |
| GET | `/api/author/:id/blogs` | — | 作者博客列表 |
| POST | `/api/blogs` | JWT | 创建博客（可带 `category_id`） |
| PUT | `/api/blogs/:id` | JWT | 更新博客（可改分类） |
| DELETE | `/api/blogs/:id` | JWT | 删除博客（用户自删，进回收站） |

### 博客分类管理（管理员）
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/admin/blog-categories` | JWT(admin) | 分类树 |
| POST | `/api/admin/blog-categories` | JWT(admin) | 新建分类（`parent_id` 指定二级/三级） |
| PUT | `/api/admin/blog-categories/:id` | JWT(admin) | 修改分类（最多三级，防成环） |
| DELETE | `/api/admin/blog-categories/:id` | JWT(admin) | 删除分类（其下博客回未分类） |

### 回收站（软删除区分用户自删 / 管理员封禁）
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/author/recycle` | JWT | 作者回收站（自己删除的小说/博客） |
| POST | `/api/author/recycle/restore` | JWT | 恢复自己删除的内容（仅 `deleted_by=user`） |
| GET | `/api/admin/recycle` | JWT(admin) | 全站用户自删内容监管（可配合 `/admin/softdel/restore` 恢复） |

### 搜索与关注
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/search/authors` | — | 搜索作者 |
| POST | `/api/follow/:id` | JWT | 关注/取关 |
| GET | `/api/follow/stats` | JWT | 关注统计 |

### 隔离墙
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/wall-zone/:zone` | — | 分区隔离详情（三步确认） |

### 评分
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/ratings` | JWT | 提交/更新五维度评分 |
| GET | `/api/ratings` | JWT | 我的评分 |
| GET | `/api/novels/:id/rating` | — | 作品评分统计 |

### 书架
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/bookshelf` | JWT | 书架列表（含作品信息和进度） |
| POST | `/api/bookshelf` | JWT | 添加到书架 |
| DELETE | `/api/bookshelf/:id` | JWT | 从书架移除 |
| GET | `/api/bookshelf/check/:id` | JWT | 检查是否在书架 |
| POST | `/api/bookshelf/progress` | JWT | 更新阅读进度 |

### 论坛
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/forums` | — | 论坛列表 |
| GET | `/api/forums/:id` | — | 论坛详情 |
| GET | `/api/novels/:id/forum` | — | 作品专属论坛 |
| POST | `/api/forums/:id/threads` | JWT | 发帖 |
| GET | `/api/threads/:id` | — | 帖子详情 |
| POST | `/api/threads/:id/posts` | JWT | 回帖 |

### 作者
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/author/novels` | JWT | 我的作品列表 |
| GET | `/api/author/novels/:id/stats` | JWT | 作品统计 |
| GET | `/api/author/dashboard` | JWT | 作者仪表盘 |
| GET | `/api/author/profile/:id` | — | 作者公开主页（`:id` 兼容用户名，数字是主键） |
| GET | `/api/author/forum/:id` | — | 作者专属论坛（兼容用户名） |
| GET | `/api/author/earnings` | JWT | 收益记录 |
| POST | `/api/author/withdraw` | JWT | 申请提现 |
| POST | `/api/author/apply-vip` | JWT | 申请 VIP 作者 |

### 用户标识解析（数字=主键 id，用户名兼容访问，大小写不敏感）
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/users/resolve?key=xxx` | — | 轻量解析用户标识 → `{id, username, nickname, role}` |

### 导入导出
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/novels/import/preview` | JWT | 导入预览 |
| POST | `/api/novels/import` | JWT | 导入（支持追加模式） |
| POST | `/api/novels/import/batch` | JWT | 批量导入（一个文件一章 / 自定义正则 / 自动识别） |
| POST | `/api/blogs/import/batch` | JWT | 博客批量导入（每个文件一篇，标题取文件名） |
| POST | `/api/novels/:id/export` | JWT | 导出 ZIP |
| POST | `/api/novels/:id/export/epub` | JWT | 导出 EPUB |
| POST | `/api/novels/:id/export/markdown` | JWT | 导出 Markdown |
| POST | `/api/novels/:id/export/txt` | JWT | 导出 TXT |

### 举报
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/reports` | JWT | 提交举报 |

### 管理后台
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/admin/stats` | 管理员 | 基础统计 |
| GET | `/api/admin/dashboard` | 管理员 | 数据大屏（7天趋势、2×2 图表） |
| GET | `/api/admin/community` | 管理员 | 社区动态（最新用户/作品/评论/帖子） |
| GET | `/api/admin/users` | 管理员 | 用户列表 |
| GET | `/api/admin/users/audit` | 管理员 | 用户行为审计日志（可按 user_id/action 筛选） |
| PUT | `/api/admin/users/:id` | 管理员 | 修改用户 |
| DELETE | `/api/admin/users/:id` | 管理员 | 删除/封禁用户 |
| POST | `/api/admin/users/:id/mute` | 管理员 | 禁言用户 |
| POST | `/api/admin/users/:id/unmute` | 管理员 | 解除禁言 |
| GET | `/api/admin/vip-applications` | 管理员 | VIP 申请列表 |
| POST | `/api/admin/vip-applications/:id/approve` | 管理员 | 批准 VIP |
| GET | `/api/admin/reports` | 管理员 | 举报列表 |
| POST | `/api/admin/reports/:id/handle` | 管理员 | 处理举报 |
| GET | `/api/admin/finance` | 管理员 | 财务总览 |
| GET | `/api/admin/config` | 管理员 | 站长配置 |
| PUT | `/api/admin/config` | 管理员 | 更新站长配置 |
| GET | `/api/admin/wall-config` | 管理员 | 隔离墙配置 |
| PUT | `/api/admin/wall-config` | 管理员 | 更新隔离墙配置 |
| GET | `/api/admin/sites` | 管理员 | 远程站点列表 |
| GET | `/api/admin/sites/inbound` | 管理员 | 入站连接记录（谁连了我） |
| POST | `/api/admin/sites` | 管理员 | 添加远程站点 |
| PUT | `/api/admin/sites/:id` | 管理员 | 更新远程站点 |
| DELETE | `/api/admin/sites/:id` | 管理员 | 删除远程站点 |
| POST | `/api/admin/sites/:id/sync` | 管理员 | 同步远程站点 |
| GET | `/api/admin/forums` | 管理员 | 论坛管理列表 |
| POST | `/api/admin/forums` | 管理员 | 创建论坛 |
| PUT | `/api/admin/forums/:id` | 管理员 | 编辑论坛 |
| DELETE | `/api/admin/forums/:id` | 管理员 | 删除论坛 |
| POST | `/api/admin/forums/:id/review` | 管理员 | 自建论坛审查 |
| GET | `/api/admin/xss-rules` | 管理员 | XSS 动态规则库 |
| POST | `/api/admin/xss-rules` | 管理员 | 新增 XSS 规则 |
| PUT | `/api/admin/xss-rules/:id` | 管理员 | 更新 XSS 规则 |
| DELETE | `/api/admin/xss-rules/:id` | 管理员 | 删除 XSS 规则 |
| POST | `/api/admin/xss-rules/test` | 管理员 | 测试规则正则 |
| GET | `/api/admin/bad-words` | 管理员 | 违禁词库规则列表 |
| POST | `/api/admin/bad-words` | 管理员 | 新增违禁词规则 |
| PUT | `/api/admin/bad-words/:id` | 管理员 | 更新违禁词规则 |
| DELETE | `/api/admin/bad-words/:id` | 管理员 | 删除违禁词规则 |
| POST | `/api/admin/bad-words/test` | 管理员 | 测试违禁词规则（关键词/正则 × 标记/删除/替换） |
| GET | `/api/admin/bad-word-flags` | 管理员 | 违禁词标记待审列表（status 过滤 + 分页） |
| PUT | `/api/admin/bad-word-flags/:id` | 管理员 | 处理标记（确认违规[可选删除内容留证] / 放行） |
| GET | `/api/admin/html-reviews` | 管理员 | HTML 内容审查列表 |
| GET | `/api/admin/html-reviews/:type/:id` | 管理员 | 审查详情 |
| POST | `/api/admin/html-reviews/:type/:id/review` | 管理员 | 复核 HTML 内容 |
| POST | `/api/admin/smtp/test` | 管理员 | SMTP 测试与逐阶段诊断 |
| GET | `/api/admin/softdel` | 管理员 | 内容治理：软删除列表（含留证记录） |
| POST | `/api/admin/softdel` | 管理员 | 删除/封禁任意内容（软删除 + `softdel_` 留证） |
| POST | `/api/admin/softdel/restore` | 管理员 | 恢复被封禁内容 |
| GET | `/api/admin/recycle` | 管理员 | 回收站监管：全站用户自删内容 |
| GET | `/api/admin/novels` | 管理员 | 小说封禁管理列表 |
| GET | `/api/admin/novels/:id/chapters` | 管理员 | 章节封禁管理列表 |
| GET | `/api/admin/blog-categories` | 管理员 | 博客分类管理列表（用户分组 + 待审核徽标） |
| GET | `/api/admin/blog-categories/mode` | 管理员 | 分类审核模式（post_review 事后审核 / pre_public 先公开） |
| PUT | `/api/admin/blog-categories/mode` | 管理员 | 切换分类审核模式 |
| POST | `/api/admin/blog-categories` | 管理员 | 创建分类（管理员直通） |
| PUT | `/api/admin/blog-categories/:id` | 管理员 | 更新分类 |
| DELETE | `/api/admin/blog-categories/:id` | 管理员 | 删除分类 |
| POST | `/api/admin/blog-categories/:id/review` | 管理员 | 审核分类（approve / reject[附原因]） |
| GET | `/api/admin/algorithms` | 管理员 | 算法推荐配置（176号令 算法透明度） |
| PUT | `/api/admin/algorithms` | 管理员 | 更新算法推荐配置 |
| GET | `/api/admin/announcements` | 管理员 | 公告列表 |
| POST | `/api/admin/announcements` | 管理员 | 发布公告 |
| PUT | `/api/admin/announcements/:id` | 管理员 | 更新公告 |
| DELETE | `/api/admin/announcements/:id` | 管理员 | 删除公告 |
| GET | `/api/admin/log-redact` | 管理员 | 日志脱敏配置 |
| PUT | `/api/admin/log-redact` | 管理员 | 更新脱敏配置（热更新） |
| POST | `/api/admin/log-redact/preview` | 管理员 | 脱敏效果预览工具 |
| GET | `/api/admin/security-grade` | 管理员 | 等保/关基识别记录 |
| PUT | `/api/admin/security-grade` | 管理员 | 更新定级记录 |
| GET | `/api/admin/vulnerabilities` | 管理员 | 漏洞整改清单（status/severity 筛选 + 统计） |
| POST | `/api/admin/vulnerabilities` | 管理员 | 新建漏洞记录 |
| PUT | `/api/admin/vulnerabilities/:id` | 管理员 | 更新漏洞（状态/整改时间） |
| DELETE | `/api/admin/vulnerabilities/:id` | 管理员 | 删除漏洞记录 |
| GET | `/api/admin/vulnerabilities/export` | 管理员 | 漏洞清单导出 |
| GET | `/api/admin/compliance/report` | 管理员 | 合规自查报告（一键生成） |
| GET | `/api/admin/compliance/export-logs` | 管理员 | 按时间范围导出访问日志（ZIP） |
| GET | `/api/admin/compliance/export-users` | 管理员 | 导出用户注册信息清单（CSV） |

### 🔧 内容抢救台（应急窗口）
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/admin/emergency/set-password` | 管理员 | 设置/修改二级密码 |
| POST | `/api/admin/emergency/unlock` | 管理员 | 二级密码解锁（限流 10次/60s，失败锁定 15 分钟） |
| POST | `/api/admin/emergency/preview` | 二级密码 token | 拉取内容预览（body 传 token，小说/博客/帖子返回全部评论/回复；token 不落 URL） |
| POST | `/api/admin/emergency/content` | 二级密码 token | 物理硬删除（body 传 token，不可恢复） |
| GET | `/api/admin/emergency/audit` | 管理员 | 抢救台审计日志 |

### 🗄️ 数据库管理
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/admin/db/health` | 管理员 | 数据库健康检查 |
| GET | `/api/admin/db/current` | 管理员 | 当前连接摘要（密码脱敏） |
| POST | `/api/admin/db/migrate` | 管理员 | SQLite ↔ MySQL 在线热迁移（无需重启） |
| GET | `/api/admin/db/job/:id` | 管理员 | 迁移任务进度 |
| POST | `/api/admin/db/backup` | 管理员 | 立即备份 |
| GET | `/api/admin/db/backups` | 管理员 | 备份列表 |
| DELETE | `/api/admin/db/backups/:name` | 管理员 | 删除备份 |
| GET | `/api/admin/db/backups/:name/download` | 管理员 | 下载备份 |
| POST | `/api/admin/db/restore` | 管理员 | 恢复备份（仅 SQLite） |
| GET | `/api/admin/db/backup-config` | 管理员 | 定时备份配置 |
| PUT | `/api/admin/db/backup-config` | 管理员 | 更新定时备份配置 |

### 🧭 建站向导
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/setup/status` | — | 安装状态 |
| GET | `/api/setup/env-check` | 安装密钥 | 环境检查 |
| POST | `/api/setup/db-test` | 安装密钥 | 测试数据库连通 |
| POST | `/api/setup/install` | 安装密钥 | 执行安装（装后路由注销） |

### 远程站点
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/federated/novels` | — | 联邦作品列表 |
| GET | `/api/federated/sites` | — | 公开站点列表 |
| GET | `/api/site-info` | — | 本站信息 |

> 📡 上述联邦公开接口会记录「谁连接了我」（入站连接：对方站点名/地址/IP/User-Agent/访问次数），被连方可在后台「远程站点互通 → 谁连了我」查看。联邦同步时本站会向对方自报身份（`source_name`/`source_url`）。

### 合规公开接口（176号令）
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/algorithms/description` | — | 算法推荐说明（公开，算法透明度） |
| GET | `/api/announcements` | — | 站点公告列表（仅已发布） |
| GET | `/api/users/me/preferences` | JWT | 用户偏好设置（个性化推荐开关，关闭权） |
| PUT | `/api/users/me/preferences` | JWT | 更新偏好设置 |

### 文件服务
| 路径 | 说明 |
|------|------|
| `/novels/*` | 小说正文静态文件（文件系统） |
| `/uploads/*` | 上传资源静态文件 |
| `/health` | 健康检查（HTML/JSON 双格式） |
| `/*` | SPA fallback → Vue Router |

### 🎨 自定义 UI 模板 (UserFrame)
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/userframes` | JWT | 我的模板列表 |
| POST | `/api/userframes` | JWT | 创建模板（含 HTML 内容） |
| GET | `/api/userframes/:id` | JWT | 模板详情+内容 |
| PUT | `/api/userframes/:id` | JWT | 更新模板 |
| DELETE | `/api/userframes/:id` | JWT | 删除模板 |
| GET | `/api/userframes/public` | — | 浏览公开模板 |
| GET | `/api/userframes/:id/preview` | — | 模板沙盒预览（HTML 页面） |
| GET | `/api/novels/:id/frames` | — | 作品关联的阅读模板 |
| GET | `/api/author/:id/frames` | — | 作者展现模板 |
| GET | `/api/template/novel/:id` | — | 小说数据 API（模板内 JS 调用） |

### 📦 扩展 HTML 托管 (UserHTML)
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/userhtmls` | JWT | 我的扩展列表 |
| POST | `/api/userhtmls/upload` | JWT | 上传 ZIP（multipart/form-data，含安全扫描） |
| GET | `/api/userhtmls/:id` | JWT | 扩展详情+文件列表 |
| PUT | `/api/userhtmls/:id` | JWT | 更新扩展元数据（名称/入口/公开/可下载） |
| DELETE | `/api/userhtmls/:id` | JWT | 删除扩展及全部文件 |
| GET | `/api/userhtmls/:id/preview` | — | 扩展沙盒预览 |
| GET | `/api/userhtmls/:id/download` | — | 下载原始 ZIP（需作者允许） |
| GET | `/api/novels/:id/htmls` | — | 作品关联的扩展 |
| GET | `/api/author/:id/htmls` | — | 作者的公开扩展 |
| GET | `/api/htmls/public` | — | 广场：所有公开扩展（分页） |

### ⚙️ 端口代理 & VFS
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/port/allocate` | JWT | 分配代理端口（需项目名） |
| POST | `/api/port/release` | JWT | 释放端口 |
| GET | `/api/port/list` | JWT | 已分配端口列表 |
| GET | `/api/sandbox/info` | — | 沙盒系统信息 |
| POST | `/api/sandbox-vfs/:id/write` | — | VFS 写入 {key, value} |
| GET | `/api/sandbox-vfs/:id/read?key=` | — | VFS 读取 |
| GET | `/api/sandbox-vfs/:id/list` | — | VFS 键列表 |

### 🔒 虚拟沙盒
| 方法 | 端点 | 认证 | 说明 |
|------|------|------|------|
| GET | `/app/:htmlId` | — | 虚拟沙盒入口（全屏运行，隐藏真实路径） |
| GET | `/app/:htmlId/*` | — | 虚拟沙盒静态资源 |
| ANY | `/sandbox/proxy/:uid/:name/*` | — | 命名端口反向代理 |

---

## 🗺 开发路线图

- [x] **Phase 1 MVP** — 注册登录、作品 CRUD、章节读写（含哈希签名）、分类体系、评论、多维度评分、论坛、导入预览/追加/导出、作者后台、管理后台
- [x] **Phase 1.5 安全与合规** — 敏感区隔离墙（可配置 3~5 步确认）、HMAC-SHA256 内容签名、章节完整性验证、举报系统、IP 黑名单、跨域评论限速
- [x] **Phase 2 生态增强** — 书架收藏与阅读进度、邮箱验证码、滑块验证码、站点联邦互通
- [x] **Phase 2.5 部署体验** — 自动创建 .env、IPv4/IPv6 网络配置、网卡 IP 自动扫描、Windows 防火墙自动配置、代码模块化拆分、社区动态仪表盘、作者博客分页、收藏/阅读数据图表
- [x] **Phase 2.6 创作增强** — Cherry Markdown 统一渲染引擎、图表切换、搜索增强、隔离墙三级体系、字数趋势、博客 MD 渲染
- [x] **Phase 2.7 个性化 & 扩展** — 自定义 HTML-UI 模板（阅读+作者展现）、ZIP 扩展托管、HTML 应用广场、WASM 沙盒支持、VFS 虚拟文件系统、端口代理、23条内容安全扫描、虚拟路径隐藏、展开/收起控件
- [x] **Phase 2.8 部署与合规（176号令）** — 建站向导（一次性安装密钥 + 数据库加密配置）、SQLite ↔ MySQL 在线热迁移、自动备份/恢复、注册/登录 IP 留存、内容软删除（8 表）、用户行为审计日志、访问日志关联 user_id、日志完整性保护（SHA256 归档 + 6 个月保留）、管理员审核日志规范、联邦入站记录
- [x] **Phase 2.9 流量治理** — 四层身份 API Key 体系、爬虫识别（无头/伪装/高频降级）、**已知名头爬虫直接 403 阻断（名单可配）**、联邦 Key 双向可见、按身份访问日志审计
- [ ] **Phase 3 高级功能** — 付费阅读闭环、打赏 UI、EPUB 封面增强
- [x] **Phase 3.5 移动端浏览** — 全站响应式布局、汉堡菜单导航、阅读器移动优化、弹窗限宽、表格横滚、触控友好
- [x] **Phase 3.6 渲染统一与统计准确性** — Cherry 0.11.9 完整版统一引擎（阅读器/博客/评论/论坛与编辑器预览同源同主题）、章节标题大字（h1-h6 !important 全阶统一）、浏览端 Cherry 主题副作用修复（.cherry-previewer 分屏 50% 宽度回归）、全量统计排除软删除数据（分类分布 / 章节 / 字数 / 评论趋势）、删除章节编号自动补齐（renumberChapters）
- [x] **Phase 3.7 阅读器完整版渲染 + 标题大字 + 日志小时切分** — 渲染器/阅读器切换为 Cherry 完整版构建产物（cherry-markdown.esm.js headless 实例，与编辑器 100% 同源同产物）、阅读器正文自动注入「第N章 标题」一级标题大字（参照上传 md 已有标题则不重复；存量数据库章节渲染期自动生效，不改文件不破坏签名）、日志按小时切分且 request/database 分文件夹（log/requests/<日期>/<小时>.log）、归档与 SHA256 适配小时文件、社区最近帖子统计排除软删除
- [x] **Phase 3.8 内容治理：违禁词库** — 动态违禁词规则引擎（关键词+正则双匹配、标记/删除/替换三动作、章节/评论/论坛/博客可配置范围）、章节/评论/帖子/博客 7 处写入点自动净化 + mark 命中进待审队列（去重）、管理员后台「违禁词库」页面（规则 CRUD + 实时测试 + 待审处理[确认违规可连带软删留证/放行] + 分页）、引擎单测覆盖
- [ ] **Phase 4 治理与扩展** — 社区仲裁员选举、财务公开面板、API 开放平台、移动端 PWA（离线缓存 / 安装到主屏）

---

## 📄 许可证

本项目采用 **[GNU Affero General Public License v3.0 (AGPL-3.0)](https://www.gnu.org/licenses/agpl-3.0.html)**。

这意味着：
- ✅ 你可以自由使用、修改和分发本项目
- ✅ 你可以用于商业用途
- ⚠️ **如果你修改了代码并通过网络提供服务，必须公开你的修改**
- ⚠️ 分发本软件（包括修改版）时必须保留相同的 AGPL-3.0 许可证

> 选择 AGPL v3 是为了确保平台永远对社区开放。即使有人 fork 后作为 SaaS 服务运行，其修改也必须回馈社区。

---

<p align="center">
  <sub>全平台 · 全CPU · 8080 端口 · 永远开源 🌌</sub>
</p>